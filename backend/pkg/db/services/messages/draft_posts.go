package messages

import (
	"context"
	"errors"
	"fmt"

	core "actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	"actionphase/pkg/validation"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// GetDraftPostForPhase retrieves the draft post for a pending phase.
// Returns nil if no draft exists.
func (s *MessageService) GetDraftPostForPhase(ctx context.Context, phaseID int32) (*core.MessageWithDetails, error) {
	queries := models.New(s.DB)

	row, err := queries.GetDraftPostForPhase(ctx, pgtype.Int4{Int32: phaseID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get draft post for phase: %w", err)
	}

	var avatarURL *string
	if row.CharacterAvatarUrl.Valid {
		avatarURL = &row.CharacterAvatarUrl.String
	}

	return &core.MessageWithDetails{
		Message: models.Message{
			ID:                    row.ID,
			GameID:                row.GameID,
			PhaseID:               row.PhaseID,
			AuthorID:              row.AuthorID,
			CharacterID:           row.CharacterID,
			Content:               row.Content,
			MessageType:           row.MessageType,
			ParentID:              row.ParentID,
			ThreadDepth:           row.ThreadDepth,
			Visibility:            row.Visibility,
			MentionedCharacterIds: row.MentionedCharacterIds,
			IsEdited:              row.IsEdited,
			IsDeleted:             row.IsDeleted,
			IsDraft:               row.IsDraft,
			CreatedAt:             row.CreatedAt,
			DeletedAt:             row.DeletedAt,
			DeletedByUserID:       row.DeletedByUserID,
			EditedAt:              row.EditedAt,
			EditCount:             row.EditCount,
		},
		AuthorUsername:     row.AuthorUsername,
		CharacterName:      row.CharacterName.String,
		CharacterAvatarUrl: avatarURL,
		CommentCount:       0,
	}, nil
}

// CreateDraftPost creates a draft post for a pending phase.
// Enforces one-draft-per-phase constraint.
func (s *MessageService) CreateDraftPost(ctx context.Context, req core.CreatePostRequest) (*core.MessageWithDetails, error) {
	if req.PhaseID == nil {
		return nil, fmt.Errorf("phase_id is required for draft posts")
	}

	queries := models.New(s.DB)

	// Validate game is not completed/cancelled
	game, err := queries.GetGame(ctx, req.GameID)
	if err != nil {
		return nil, fmt.Errorf("failed to get game: %w", err)
	}
	if err := core.ValidateGameNotCompleted(ctx, &game); err != nil {
		return nil, err
	}

	// Validate the phase is not already active — drafts only make sense for pending phases
	phase, err := queries.GetPhase(ctx, *req.PhaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to get phase: %w", err)
	}
	if phase.IsActive.Bool {
		return nil, fmt.Errorf("cannot create a draft post for an already-active phase")
	}

	// Validate character ownership
	if err := s.ValidateCharacterOwnership(ctx, req.CharacterID, req.AuthorID, req.GameID); err != nil {
		return nil, fmt.Errorf("character validation failed: %w", err)
	}

	// Validate content length
	if err := validation.ValidatePost(req.Content); err != nil {
		return nil, err
	}

	// Enforce one-draft-per-phase constraint
	existingCount, err := queries.CountDraftPostsByPhase(ctx, pgtype.Int4{Int32: *req.PhaseID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("failed to check existing draft: %w", err)
	}
	if existingCount > 0 {
		return nil, core.ErrDraftPostExists
	}

	// Extract character mentions
	mentionedIDs, err := s.extractCharacterMentions(ctx, req.Content, req.GameID)
	if err != nil {
		mentionedIDs = []int32{}
	}

	_, err = queries.CreateDraftPost(ctx, models.CreateDraftPostParams{
		GameID:                req.GameID,
		PhaseID:               pgtype.Int4{Int32: *req.PhaseID, Valid: true},
		AuthorID:              req.AuthorID,
		CharacterID:           req.CharacterID,
		Content:               req.Content,
		Visibility:            models.MessageVisibility(req.Visibility),
		MentionedCharacterIds: mentionedIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create draft post: %w", err)
	}

	return s.GetDraftPostForPhase(ctx, *req.PhaseID)
}

// UpdateDraftPost replaces the content of an existing draft post.
func (s *MessageService) UpdateDraftPost(ctx context.Context, postID int32, content string) (*core.MessageWithDetails, error) {
	if err := validation.ValidatePost(content); err != nil {
		return nil, err
	}

	queries := models.New(s.DB)

	// Extract mentions from updated content — we need the game ID for this
	// Get the draft post first to find the game ID
	post, err := queries.GetPost(ctx, postID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("draft post not found")
		}
		return nil, fmt.Errorf("failed to get draft post: %w", err)
	}

	mentionedIDs, err := s.extractCharacterMentions(ctx, content, post.GameID)
	if err != nil {
		mentionedIDs = []int32{}
	}

	_, err = queries.UpdateDraftPost(ctx, models.UpdateDraftPostParams{
		ID:                    postID,
		Content:               content,
		MentionedCharacterIds: mentionedIDs,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("draft post not found or is not a draft")
		}
		return nil, fmt.Errorf("failed to update draft post: %w", err)
	}

	// Fetch updated details
	if !post.PhaseID.Valid {
		return nil, fmt.Errorf("draft post has no phase")
	}
	return s.GetDraftPostForPhase(ctx, post.PhaseID.Int32)
}

// DeleteDraftPost hard-deletes a draft post.
func (s *MessageService) DeleteDraftPost(ctx context.Context, postID int32) error {
	queries := models.New(s.DB)

	// Verify it's actually a draft post before deleting
	post, err := queries.GetPost(ctx, postID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("draft post not found")
		}
		return fmt.Errorf("failed to get draft post: %w", err)
	}
	if !post.IsDraft {
		return fmt.Errorf("post is not a draft")
	}

	if err := queries.DeleteDraftPost(ctx, postID); err != nil {
		return fmt.Errorf("failed to delete draft post: %w", err)
	}

	return nil
}

// PublishDraftPostsForPhase clears is_draft on all draft posts for a phase.
// Called atomically during phase activation.
func (s *MessageService) PublishDraftPostsForPhase(ctx context.Context, phaseID int32) error {
	queries := models.New(s.DB)

	if err := queries.PublishDraftPostsForPhase(ctx, pgtype.Int4{Int32: phaseID, Valid: true}); err != nil {
		return fmt.Errorf("failed to publish draft posts for phase %d: %w", phaseID, err)
	}

	return nil
}

// DeleteDraftPostsForPhase hard-deletes all draft posts for a phase.
// Called when a phase is deleted.
func (s *MessageService) DeleteDraftPostsForPhase(ctx context.Context, phaseID int32) error {
	queries := models.New(s.DB)

	if err := queries.DeleteDraftPostsForPhase(ctx, pgtype.Int4{Int32: phaseID, Valid: true}); err != nil {
		return fmt.Errorf("failed to delete draft posts for phase %d: %w", phaseID, err)
	}

	return nil
}
