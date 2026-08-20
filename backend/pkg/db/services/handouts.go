package db

import (
	"context"
	"errors"
	"fmt"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HandoutService implements the HandoutServiceInterface
type HandoutService struct {
	DB *pgxpool.Pool
}

// Compile-time verification that HandoutService implements HandoutServiceInterface
var _ core.HandoutServiceInterface = (*HandoutService)(nil)

// NewHandoutService creates a new handout service
func NewHandoutService(db *pgxpool.Pool) *HandoutService {
	return &HandoutService{DB: db}
}

// Helper function to convert db.Handout to core.Handout
func handoutToCore(h models.Handout) *core.Handout {
	handout := &core.Handout{
		ID:      h.ID,
		GameID:  h.GameID,
		Title:   h.Title,
		Content: h.Content,
		Status:  h.Status,
	}
	if h.CreatedAt.Valid {
		handout.CreatedAt = &h.CreatedAt.Time
	}
	if h.UpdatedAt.Valid {
		handout.UpdatedAt = &h.UpdatedAt.Time
	}
	return handout
}

// Helper function to convert db.HandoutComment to core.HandoutComment
func handoutCommentToCore(c models.HandoutComment) *core.HandoutComment {
	comment := &core.HandoutComment{
		ID:        c.ID,
		HandoutID: c.HandoutID,
		UserID:    c.UserID,
		Content:   c.Content,
		EditCount: c.EditCount,
	}
	if c.ParentCommentID.Valid {
		parentID := c.ParentCommentID.Int32
		comment.ParentCommentID = &parentID
	}
	if c.CreatedAt.Valid {
		comment.CreatedAt = &c.CreatedAt.Time
	}
	if c.UpdatedAt.Valid {
		comment.UpdatedAt = &c.UpdatedAt.Time
	}
	if c.EditedAt.Valid {
		comment.EditedAt = &c.EditedAt.Time
	}
	if c.DeletedAt.Valid {
		comment.DeletedAt = &c.DeletedAt.Time
	}
	if c.DeletedByUserID.Valid {
		deletedByID := c.DeletedByUserID.Int32
		comment.DeletedByUserID = &deletedByID
	}
	return comment
}

// CreateHandout creates a new handout
func (s *HandoutService) CreateHandout(ctx context.Context, gameID int32, title string, content string, status string, userID int32) (*core.Handout, error) {
	queries := models.New(s.DB)

	// Validate status
	if status != "draft" && status != "published" {
		return nil, fmt.Errorf("invalid status: must be 'draft' or 'published'")
	}

	handout, err := queries.CreateHandout(ctx, models.CreateHandoutParams{
		GameID:  gameID,
		Title:   title,
		Content: content,
		Status:  status,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create handout: %w", err)
	}

	return handoutToCore(handout), nil
}

// GetHandout retrieves a handout by ID
func (s *HandoutService) GetHandout(ctx context.Context, handoutID int32, userID int32) (*core.Handout, error) {
	queries := models.New(s.DB)

	handout, err := queries.GetHandout(ctx, handoutID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("handout not found")
		}
		return nil, fmt.Errorf("failed to get handout: %w", err)
	}

	// Check if user is GM of the game
	game, err := queries.GetGame(ctx, handout.GameID)
	if err != nil {
		return nil, fmt.Errorf("failed to get game: %w", err)
	}

	// If handout is draft and user is not GM, deny access
	if handout.Status == "draft" && game.GmUserID != userID && !core.IsUserCoGM(ctx, s.DB, handout.GameID, userID) {
		return nil, fmt.Errorf("access denied: draft handouts are only visible to GM")
	}

	return handoutToCore(handout), nil
}

// ListHandouts retrieves all handouts for a game
func (s *HandoutService) ListHandouts(ctx context.Context, gameID int32, userID int32, isGM bool) ([]*core.Handout, error) {
	queries := models.New(s.DB)

	handouts, err := queries.ListHandoutsByGame(ctx, models.ListHandoutsByGameParams{
		GameID:  gameID,
		Column2: isGM, // If true, show all handouts; if false, only published
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list handouts: %w", err)
	}

	result := make([]*core.Handout, len(handouts))
	for i, h := range handouts {
		result[i] = handoutToCore(h)
	}

	return result, nil
}

// ListPublishedHandoutsAcrossGames retrieves the published handouts of every
// in_progress game the user takes part in, for surfaces with no game in scope
// (the global Utility Drawer).
//
// Membership and the published-only rule are enforced by the query, so unlike
// ListHandouts this takes no isGM flag: there is no per-game role to resolve
// and nothing further to filter here.
func (s *HandoutService) ListPublishedHandoutsAcrossGames(ctx context.Context, userID int32) ([]*core.HandoutWithGame, error) {
	queries := models.New(s.DB)

	rows, err := queries.ListPublishedHandoutsAcrossGames(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list handouts across games: %w", err)
	}

	result := make([]*core.HandoutWithGame, len(rows))
	for i, r := range rows {
		handout := handoutToCore(models.Handout{
			ID:        r.ID,
			GameID:    r.GameID,
			Title:     r.Title,
			Content:   r.Content,
			Status:    r.Status,
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
		})
		result[i] = &core.HandoutWithGame{
			Handout:   *handout,
			GameTitle: r.GameTitle,
		}
	}

	return result, nil
}

// UpdateHandout updates a handout's title, content, and status
func (s *HandoutService) UpdateHandout(ctx context.Context, handoutID int32, title string, content string, status string, userID int32) (*core.Handout, error) {
	queries := models.New(s.DB)

	// Validate status
	if status != "draft" && status != "published" {
		return nil, fmt.Errorf("invalid status: must be 'draft' or 'published'")
	}

	handout, err := queries.UpdateHandout(ctx, models.UpdateHandoutParams{
		Title:   title,
		Content: content,
		Status:  status,
		ID:      handoutID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("handout not found")
		}
		return nil, fmt.Errorf("failed to update handout: %w", err)
	}

	return handoutToCore(handout), nil
}

// DeleteHandout removes a handout
func (s *HandoutService) DeleteHandout(ctx context.Context, handoutID int32, userID int32) error {
	queries := models.New(s.DB)

	err := queries.DeleteHandout(ctx, handoutID)
	if err != nil {
		return fmt.Errorf("failed to delete handout: %w", err)
	}

	return nil
}

// PublishHandout changes a draft handout to published
func (s *HandoutService) PublishHandout(ctx context.Context, handoutID int32, userID int32) (*core.Handout, error) {
	queries := models.New(s.DB)

	handout, err := queries.PublishHandout(ctx, handoutID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("handout not found")
		}
		return nil, fmt.Errorf("failed to publish handout: %w", err)
	}

	return handoutToCore(handout), nil
}

// UnpublishHandout changes a published handout to draft
func (s *HandoutService) UnpublishHandout(ctx context.Context, handoutID int32, userID int32) (*core.Handout, error) {
	queries := models.New(s.DB)

	handout, err := queries.UnpublishHandout(ctx, handoutID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("handout not found")
		}
		return nil, fmt.Errorf("failed to unpublish handout: %w", err)
	}

	return handoutToCore(handout), nil
}

// CreateHandoutComment adds a comment to a handout
func (s *HandoutService) CreateHandoutComment(ctx context.Context, handoutID int32, userID int32, parentCommentID *int32, content string) (*core.HandoutComment, error) {
	queries := models.New(s.DB)

	var parentID pgtype.Int4
	if parentCommentID != nil {
		parentID = pgtype.Int4{Int32: *parentCommentID, Valid: true}
	}

	comment, err := queries.CreateHandoutComment(ctx, models.CreateHandoutCommentParams{
		HandoutID:       handoutID,
		UserID:          userID,
		ParentCommentID: parentID,
		Content:         content,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}

	return handoutCommentToCore(comment), nil
}

// ListHandoutComments retrieves all comments for a handout
func (s *HandoutService) ListHandoutComments(ctx context.Context, handoutID int32) ([]*core.HandoutComment, error) {
	queries := models.New(s.DB)

	comments, err := queries.ListHandoutComments(ctx, handoutID)
	if err != nil {
		return nil, fmt.Errorf("failed to list comments: %w", err)
	}

	result := make([]*core.HandoutComment, len(comments))
	for i, c := range comments {
		comment := handoutCommentToCore(models.HandoutComment{
			ID:              c.ID,
			HandoutID:       c.HandoutID,
			UserID:          c.UserID,
			ParentCommentID: c.ParentCommentID,
			Content:         c.Content,
			CreatedAt:       c.CreatedAt,
			UpdatedAt:       c.UpdatedAt,
			EditedAt:        c.EditedAt,
			EditCount:       c.EditCount,
			DeletedAt:       c.DeletedAt,
			DeletedByUserID: c.DeletedByUserID,
		})
		result[i] = comment
	}

	return result, nil
}

// UpdateHandoutComment updates a comment's content
func (s *HandoutService) UpdateHandoutComment(ctx context.Context, commentID int32, userID int32, content string) (*core.HandoutComment, error) {
	queries := models.New(s.DB)

	comment, err := queries.UpdateHandoutComment(ctx, models.UpdateHandoutCommentParams{
		Content: content,
		ID:      commentID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("comment not found or already deleted")
		}
		return nil, fmt.Errorf("failed to update comment: %w", err)
	}

	return handoutCommentToCore(comment), nil
}

// DeleteHandoutComment soft-deletes a comment
func (s *HandoutService) DeleteHandoutComment(ctx context.Context, commentID int32, userID int32, isGM bool) error {
	queries := models.New(s.DB)

	err := queries.DeleteHandoutComment(ctx, models.DeleteHandoutCommentParams{
		ID:              commentID,
		DeletedByUserID: pgtype.Int4{Int32: userID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}

	return nil
}
