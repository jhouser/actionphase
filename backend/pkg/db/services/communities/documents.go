package communities

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
)

// Community documents (req 7, 8) -- a community's rules and reference pages.
//
// Two invariants hold across this file:
//
//  1. DRAFTS ARE PRIVILEGED. The moderator read and the public read are separate
//     queries, never one query with a visibility flag, so a caller cannot leak
//     a draft by passing the wrong argument. ListDocuments includes drafts;
//     ListPublishedDocuments does not.
//
//  2. EVERY LOOKUP IS COMMUNITY-SCOPED. Documents are addressed by id, and an id
//     alone would resolve a document in a different community than the request
//     path names -- letting a moderator of A read A's URL into B's draft. The
//     queries take (id, community_id) so that misses instead.

// documentStatusOrDefault resolves an optional status to a stored value.
//
// Absent means DRAFT: the safe default shows a new document to nobody until its
// author says otherwise. An unrecognised status is rejected rather than
// silently coerced -- a typo'd "publish" that quietly became a draft would look
// like the publish button was broken.
func documentStatusOrDefault(status *string) (string, error) {
	if status == nil {
		return core.DocumentStatusDraft, nil
	}
	s := strings.TrimSpace(*status)
	if !core.IsValidDocumentStatus(s) {
		return "", core.ErrInvalidDocumentStatus
	}
	return s, nil
}

// CreateDocument adds a document to a community.
//
// The caller is recorded as author, but the document belongs to the COMMUNITY:
// created_by_user_id is ON DELETE SET NULL, so deleting the moderator who wrote
// the rules does not delete the rules.
func (s *CommunityService) CreateDocument(
	ctx context.Context, communityID, actorID int32, req *core.CreateCommunityDocumentRequest,
) (*core.CommunityDocument, error) {
	if req == nil {
		return nil, fmt.Errorf("document request is required")
	}

	status, err := documentStatusOrDefault(req.Status)
	if err != nil {
		return nil, err
	}

	var sortOrder int32
	if req.SortOrder != nil {
		sortOrder = *req.SortOrder
	}

	queries := models.New(s.DB)
	row, err := queries.CreateCommunityDocument(ctx, models.CreateCommunityDocumentParams{
		CommunityID:     communityID,
		Title:           strings.TrimSpace(req.Title),
		Content:         req.Content,
		Status:          status,
		SortOrder:       sortOrder,
		CreatedByUserID: pgtype.Int4{Int32: actorID, Valid: true},
	})
	if err != nil {
		s.Logger.LogError(ctx, err, "Failed to create community document",
			"community_id", communityID)
		return nil, fmt.Errorf("create community document: %w", err)
	}

	s.Logger.Info(ctx, "Community document created",
		"community_id", communityID,
		"document_id", row.ID,
		"status", row.Status,
		"actor_user_id", actorID,
	)

	return documentFromDB(row), nil
}

// GetDocument reads one document, scoped to its community.
//
// Returns ErrCommunityDocumentNotFound both when no such document exists and
// when it belongs to another community -- see the sentinel's doc comment for
// why that is not a 403.
//
// This does NOT filter drafts: it serves both the moderator editor and the
// public reader, and the caller applies the visibility rule it needs. The
// handler is the only layer that knows whether the requester moderates.
func (s *CommunityService) GetDocument(
	ctx context.Context, communityID, documentID int32,
) (*core.CommunityDocument, error) {
	queries := models.New(s.DB)

	row, err := queries.GetCommunityDocument(ctx, models.GetCommunityDocumentParams{
		ID:          documentID,
		CommunityID: communityID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrCommunityDocumentNotFound
		}
		return nil, fmt.Errorf("get community document: %w", err)
	}

	return documentFromDB(row), nil
}

// ListDocuments returns every document in a community, DRAFTS INCLUDED.
// Moderator-only: callers must gate on CanModerateCommunity.
func (s *CommunityService) ListDocuments(
	ctx context.Context, communityID int32,
) ([]*core.CommunityDocument, error) {
	queries := models.New(s.DB)

	rows, err := queries.ListCommunityDocuments(ctx, communityID)
	if err != nil {
		s.Logger.LogError(ctx, err, "Failed to list community documents",
			"community_id", communityID)
		return nil, fmt.Errorf("list community documents: %w", err)
	}

	out := make([]*core.CommunityDocument, 0, len(rows))
	for _, row := range rows {
		out = append(out, documentFromDB(row))
	}
	return out, nil
}

// ListPublishedDocuments returns only published documents -- the public read.
func (s *CommunityService) ListPublishedDocuments(
	ctx context.Context, communityID int32,
) ([]*core.CommunityDocument, error) {
	queries := models.New(s.DB)

	rows, err := queries.ListPublishedCommunityDocuments(ctx, communityID)
	if err != nil {
		s.Logger.LogError(ctx, err, "Failed to list published community documents",
			"community_id", communityID)
		return nil, fmt.Errorf("list published community documents: %w", err)
	}

	out := make([]*core.CommunityDocument, 0, len(rows))
	for _, row := range rows {
		out = append(out, documentFromDB(row))
	}
	return out, nil
}

// ListPublishedDocumentsForGame returns the published documents of the
// community that owns a game -- the Game Info tab's list (req 8).
//
// Returns an EMPTY SLICE, not an error, for a legacy game with no community.
// The grandfathering is in the SQL join rather than a NULL check here, so no
// caller can forget it; the Info tab renders no community section and nothing
// has to special-case it.
func (s *CommunityService) ListPublishedDocumentsForGame(
	ctx context.Context, gameID int32,
) ([]*core.CommunityDocument, error) {
	queries := models.New(s.DB)

	rows, err := queries.ListPublishedCommunityDocumentsForGame(ctx, gameID)
	if err != nil {
		s.Logger.LogError(ctx, err, "Failed to list community documents for game",
			"game_id", gameID)
		return nil, fmt.Errorf("list community documents for game: %w", err)
	}

	out := make([]*core.CommunityDocument, 0, len(rows))
	for _, row := range rows {
		out = append(out, documentFromDB(row))
	}
	return out, nil
}

// UpdateDocument applies a partial update. A nil field is left unchanged.
//
// Publishing and unpublishing both go through here rather than through
// dedicated endpoints: unlike handouts, a document's status is edited on the
// same form as its body, so a moderator who fixes a typo and publishes does it
// in one request.
func (s *CommunityService) UpdateDocument(
	ctx context.Context, communityID, documentID int32, req *core.UpdateCommunityDocumentRequest,
) (*core.CommunityDocument, error) {
	if req == nil {
		return nil, fmt.Errorf("document request is required")
	}

	params := models.UpdateCommunityDocumentParams{
		ID:          documentID,
		CommunityID: communityID,
	}

	if req.Title != nil {
		params.Title = pgtype.Text{String: strings.TrimSpace(*req.Title), Valid: true}
	}
	if req.Content != nil {
		// Not trimmed and not treated as a clear: the column is NOT NULL, so an
		// empty body is a blank page rather than an absent one.
		params.Content = pgtype.Text{String: *req.Content, Valid: true}
	}
	if req.Status != nil {
		status := strings.TrimSpace(*req.Status)
		if !core.IsValidDocumentStatus(status) {
			return nil, core.ErrInvalidDocumentStatus
		}
		params.Status = pgtype.Text{String: status, Valid: true}
	}
	if req.SortOrder != nil {
		params.SortOrder = pgtype.Int4{Int32: *req.SortOrder, Valid: true}
	}

	queries := models.New(s.DB)
	row, err := queries.UpdateCommunityDocument(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrCommunityDocumentNotFound
		}
		s.Logger.LogError(ctx, err, "Failed to update community document",
			"community_id", communityID, "document_id", documentID)
		return nil, fmt.Errorf("update community document: %w", err)
	}

	s.Logger.Info(ctx, "Community document updated",
		"community_id", communityID,
		"document_id", documentID,
		"status", row.Status,
	)

	return documentFromDB(row), nil
}

// DeleteDocument removes a document from a community.
//
// Deleting something already absent is an ERROR here, unlike removing a
// moderator who does not moderate. A document is a body of writing a moderator
// may have spent real effort on, so a delete that quietly matched nothing --
// wrong id, wrong community -- should surface rather than report success and
// leave the document sitting there.
func (s *CommunityService) DeleteDocument(ctx context.Context, communityID, documentID int32) error {
	queries := models.New(s.DB)

	// Read first: DeleteCommunityDocument is :exec and reports no row count, so
	// a missing document is indistinguishable from a successful delete without
	// this. Not a transaction -- a concurrent delete of the same document is a
	// race whose outcomes are both "it is gone", which is what the caller wants.
	if _, err := s.GetDocument(ctx, communityID, documentID); err != nil {
		return err
	}

	if err := queries.DeleteCommunityDocument(ctx, models.DeleteCommunityDocumentParams{
		ID:          documentID,
		CommunityID: communityID,
	}); err != nil {
		s.Logger.LogError(ctx, err, "Failed to delete community document",
			"community_id", communityID, "document_id", documentID)
		return fmt.Errorf("delete community document: %w", err)
	}

	s.Logger.Info(ctx, "Community document deleted",
		"community_id", communityID, "document_id", documentID)
	return nil
}
