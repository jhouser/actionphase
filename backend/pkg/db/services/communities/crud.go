package communities

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	"actionphase/pkg/observability"
)

// CommunityService implements core.CommunityServiceInterface.
type CommunityService struct {
	DB     *pgxpool.Pool
	Logger *observability.Logger
}

// Compile-time verification that CommunityService implements the interface.
var _ core.CommunityServiceInterface = (*CommunityService)(nil)

// pgUniqueViolation is the SQLSTATE for a unique constraint breach.
const pgUniqueViolation = "23505"

// CreateCommunity creates a community and assigns its owner.
//
// Slug uniqueness is enforced by the database, not by a check-then-insert:
// a prior existence check cannot close the race between two concurrent creates.
// The unique violation is the authority, and it is translated to a domain error
// so the handler can answer 400 instead of 500.
func (s *CommunityService) CreateCommunity(ctx context.Context, req *core.CreateCommunityRequest) (*core.Community, error) {
	queries := models.New(s.DB)

	params := models.CreateCommunityParams{
		Name:        req.Name,
		Slug:        req.Slug,
		OwnerUserID: req.OwnerUserID,
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}

	row, err := queries.CreateCommunity(ctx, params)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return nil, core.ErrCommunitySlugTaken
		}
		s.Logger.LogError(ctx, err, "Failed to create community", "slug", req.Slug)
		return nil, fmt.Errorf("create community: %w", err)
	}

	s.Logger.Info(ctx, "Community created",
		"community_id", row.ID,
		"slug", row.Slug,
		"owner_user_id", row.OwnerUserID,
	)

	return communityFromDB(row), nil
}

// GetCommunityByID retrieves a community by primary key.
// Returns core.ErrCommunityNotFound when no row matches.
func (s *CommunityService) GetCommunityByID(ctx context.Context, id int32) (*core.Community, error) {
	queries := models.New(s.DB)

	row, err := queries.GetCommunityByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrCommunityNotFound
		}
		return nil, fmt.Errorf("get community by id: %w", err)
	}

	return communityFromListRow(
		row.ID, row.Name, row.Slug, row.Description, row.BannerUrl,
		row.OwnerUserID, row.IsActive, row.CreatedAt, row.UpdatedAt,
		row.OwnerUsername,
	), nil
}

// GetCommunityBySlug retrieves a community by its URL slug.
// Returns core.ErrCommunityNotFound when no row matches.
func (s *CommunityService) GetCommunityBySlug(ctx context.Context, slug string) (*core.Community, error) {
	queries := models.New(s.DB)

	row, err := queries.GetCommunityBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrCommunityNotFound
		}
		return nil, fmt.Errorf("get community by slug: %w", err)
	}

	return communityFromListRow(
		row.ID, row.Name, row.Slug, row.Description, row.BannerUrl,
		row.OwnerUserID, row.IsActive, row.CreatedAt, row.UpdatedAt,
		row.OwnerUsername,
	), nil
}

// ListCommunities returns every community, active or not, for the admin table.
func (s *CommunityService) ListCommunities(ctx context.Context) ([]*core.Community, error) {
	queries := models.New(s.DB)

	rows, err := queries.ListCommunities(ctx)
	if err != nil {
		return nil, fmt.Errorf("list communities: %w", err)
	}

	out := make([]*core.Community, 0, len(rows))
	for _, row := range rows {
		out = append(out, communityFromListRow(
			row.ID, row.Name, row.Slug, row.Description, row.BannerUrl,
			row.OwnerUserID, row.IsActive, row.CreatedAt, row.UpdatedAt,
			row.OwnerUsername,
		))
	}
	return out, nil
}

// ListActiveCommunities returns only active communities, for public surfaces
// and the game-create picker.
func (s *CommunityService) ListActiveCommunities(ctx context.Context) ([]*core.Community, error) {
	queries := models.New(s.DB)

	rows, err := queries.ListActiveCommunities(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active communities: %w", err)
	}

	out := make([]*core.Community, 0, len(rows))
	for _, row := range rows {
		out = append(out, communityFromListRow(
			row.ID, row.Name, row.Slug, row.Description, row.BannerUrl,
			row.OwnerUserID, row.IsActive, row.CreatedAt, row.UpdatedAt,
			row.OwnerUsername,
		))
	}
	return out, nil
}

// UpdateCommunity applies a partial update. Nil fields are left unchanged.
//
// Slug is not updatable: it appears in URLs communities share externally, so it
// is immutable after creation. See core.UpdateCommunityRequest.
func (s *CommunityService) UpdateCommunity(ctx context.Context, id int32, req *core.UpdateCommunityRequest) (*core.Community, error) {
	queries := models.New(s.DB)

	params := models.UpdateCommunityParams{ID: id}
	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.Description != nil {
		// A present-but-empty description means "remove the blurb", so it maps
		// to SQL NULL rather than to "". SetDescription tells the query to
		// write the column at all -- without it, NULL would be indistinguishable
		// from an omitted field and the clear would silently do nothing.
		trimmed := strings.TrimSpace(*req.Description)
		params.SetDescription = true
		params.Description = pgtype.Text{String: trimmed, Valid: trimmed != ""}
	}
	if req.OwnerUserID != nil {
		params.OwnerUserID = pgtype.Int4{Int32: *req.OwnerUserID, Valid: true}
	}
	if req.IsActive != nil {
		params.IsActive = pgtype.Bool{Bool: *req.IsActive, Valid: true}
	}

	row, err := queries.UpdateCommunity(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrCommunityNotFound
		}
		s.Logger.LogError(ctx, err, "Failed to update community", "community_id", id)
		return nil, fmt.Errorf("update community: %w", err)
	}

	// Reassigning ownership is a moderation-relevant act, so it is logged
	// distinctly from an ordinary profile edit.
	if req.OwnerUserID != nil {
		s.Logger.Info(ctx, "Community owner reassigned",
			"community_id", id,
			"new_owner_user_id", *req.OwnerUserID,
		)
	}

	s.Logger.Info(ctx, "Community updated", "community_id", id)
	return communityFromDB(row), nil
}
