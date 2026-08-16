package actions

import (
	"context"
	"errors"
	"fmt"
	"time"

	core "actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	"actionphase/pkg/validation"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// SubmitAction creates or updates an action submission for a phase
func (as *ActionSubmissionService) SubmitAction(ctx context.Context, req core.SubmitActionRequest) (*models.ActionSubmission, error) {
	as.Logger.Info(ctx, "Action submission attempt",
		"game_id", req.GameID,
		"user_id", req.UserID,
		"phase_id", req.PhaseID,
		"character_id", req.CharacterID,
	)

	queries := models.New(as.DB)

	// Validate game is not completed/cancelled (archived games are read-only)
	game, err := queries.GetGame(ctx, req.GameID)
	if err != nil {
		as.Logger.LogError(ctx, err, "Failed to get game for action submission",
			"game_id", req.GameID,
		)
		return nil, fmt.Errorf("failed to get game: %w", err)
	}

	if err := core.ValidateGameNotCompleted(ctx, &game); err != nil {
		as.Logger.Warn(ctx, "Cannot submit action to completed/cancelled game",
			"game_id", req.GameID,
			"game_state", game.State.String,
		)
		return nil, err
	}

	// Verify phase exists and user can submit
	canSubmit, err := queries.CanUserSubmitToPhase(ctx, models.CanUserSubmitToPhaseParams{
		ID:     req.PhaseID,
		UserID: req.UserID,
	})
	if err != nil {
		as.Logger.LogError(ctx, err, "Failed to check submission permissions",
			"phase_id", req.PhaseID,
			"user_id", req.UserID,
		)
		return nil, fmt.Errorf("failed to check submission permissions: %w", err)
	}

	if !canSubmit {
		as.Logger.Warn(ctx, "User cannot submit to this phase",
			"phase_id", req.PhaseID,
			"user_id", req.UserID,
			"game_id", req.GameID,
		)
		return nil, fmt.Errorf("user cannot submit actions to this phase")
	}

	// Validate character ownership if character_id is provided
	if req.CharacterID != nil {
		character, err := queries.GetCharacter(ctx, *req.CharacterID)
		if err != nil {
			as.Logger.Warn(ctx, "Character not found for action submission",
				"character_id", *req.CharacterID,
				"user_id", req.UserID,
			)
			return nil, fmt.Errorf("character not found")
		}

		// Verify the character belongs to the user submitting the action
		if !character.UserID.Valid || character.UserID.Int32 != req.UserID {
			as.Logger.Warn(ctx, "User attempting to submit action for character they don't own",
				"character_id", *req.CharacterID,
				"character_owner_id", character.UserID.Int32,
				"requesting_user_id", req.UserID,
			)
			return nil, fmt.Errorf("you can only submit actions for characters you own")
		}

		// Verify the character belongs to the same game
		if character.GameID != req.GameID {
			as.Logger.Warn(ctx, "Character does not belong to game",
				"character_id", *req.CharacterID,
				"character_game_id", character.GameID,
				"requested_game_id", req.GameID,
			)
			return nil, fmt.Errorf("character does not belong to this game")
		}
	}

	// Convert content to string (assuming JSON marshaling)
	contentStr := fmt.Sprintf("%v", req.Content)

	// Validate content length
	if err := validation.ValidateActionSubmission(contentStr); err != nil {
		as.Logger.Warn(ctx, "Action submission validation failed",
			"error", err,
			"content_length", len(contentStr),
		)
		return nil, err
	}

	params := models.SubmitActionParams{
		GameID:  req.GameID,
		UserID:  req.UserID,
		PhaseID: req.PhaseID,
		Content: contentStr,
	}

	if req.CharacterID != nil {
		params.CharacterID = pgtype.Int4{Int32: *req.CharacterID, Valid: true}
	}

	action, err := queries.SubmitAction(ctx, params)
	if err != nil {
		as.Logger.LogError(ctx, err, "Failed to submit action",
			"game_id", req.GameID,
			"phase_id", req.PhaseID,
			"user_id", req.UserID,
		)
		return nil, fmt.Errorf("failed to submit action: %w", err)
	}

	as.Logger.Info(ctx, "Action submitted successfully",
		"submission_id", action.ID,
		"game_id", action.GameID,
		"phase_id", action.PhaseID,
		"user_id", action.UserID,
		"character_id", action.CharacterID.Int32,
	)

	return &action, nil
}

// GetActionSubmission retrieves a specific action submission by ID
func (as *ActionSubmissionService) GetActionSubmission(ctx context.Context, submissionID int32) (*models.ActionSubmission, error) {
	queries := models.New(as.DB)

	submission, err := queries.GetActionSubmission(ctx, submissionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("action submission not found")
		}
		return nil, fmt.Errorf("failed to get action submission: %w", err)
	}

	return &submission, nil
}

// GetUserPhaseSubmission retrieves a user's submission for a specific phase
func (as *ActionSubmissionService) GetUserPhaseSubmission(ctx context.Context, phaseID, userID int32) (*models.ActionSubmission, error) {
	queries := models.New(as.DB)

	submission, err := queries.GetUserPhaseSubmission(ctx, models.GetUserPhaseSubmissionParams{
		PhaseID: phaseID,
		UserID:  userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No submission found
		}
		return nil, fmt.Errorf("failed to get user phase submission: %w", err)
	}

	return &submission, nil
}

// GetPhaseSubmissions retrieves all submissions for a specific phase
func (as *ActionSubmissionService) GetPhaseSubmissions(ctx context.Context, phaseID int32) ([]models.ActionSubmission, error) {
	queries := models.New(as.DB)

	submissions, err := queries.GetPhaseSubmissions(ctx, phaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to get phase submissions: %w", err)
	}

	// Convert to models.ActionSubmission
	result := make([]models.ActionSubmission, len(submissions))
	for i, s := range submissions {
		result[i] = models.ActionSubmission{
			ID:        s.ID,
			GameID:    s.GameID,
			PhaseID:   s.PhaseID,
			UserID:    s.UserID,
			Content:   s.Content,
			UpdatedAt: s.UpdatedAt,
		}
		if s.CharacterID.Valid {
			result[i].CharacterID = pgtype.Int4{Int32: s.CharacterID.Int32, Valid: true}
		}
		if s.SubmittedAt.Valid {
			result[i].SubmittedAt = pgtype.Timestamptz{Time: s.SubmittedAt.Time, Valid: true}
		}
	}

	return result, nil
}

// DeleteActionSubmission deletes an action submission
func (as *ActionSubmissionService) DeleteActionSubmission(ctx context.Context, submissionID, userID int32) error {
	tag, err := as.DB.Exec(ctx,
		"DELETE FROM action_submissions WHERE id = $1 AND user_id = $2",
		submissionID, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete action submission: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("action submission not found or not owned by user")
	}

	return nil
}

// GetSubmissionStats retrieves statistics about action submissions for a phase
func (as *ActionSubmissionService) GetSubmissionStats(ctx context.Context, phaseID int32) (*core.ActionSubmissionStats, error) {
	queries := models.New(as.DB)

	// Get all submissions for this phase
	submissions, err := queries.GetPhaseSubmissions(ctx, phaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to get phase submissions: %w", err)
	}

	// Calculate stats manually
	var submittedCount int32
	var latestSubmission *time.Time

	for _, submission := range submissions {
		submittedCount++
		if submission.SubmittedAt.Valid {
			if latestSubmission == nil || submission.SubmittedAt.Time.After(*latestSubmission) {
				latestSubmission = &submission.SubmittedAt.Time
			}
		}
	}

	// Get total players count - for now use a simple approach
	// This should ideally be the number of participants in the game
	totalPlayers := submittedCount
	if totalPlayers == 0 {
		totalPlayers = 1 // Avoid division by zero
	}

	submissionRate := float64(submittedCount) / float64(totalPlayers) * 100

	result := &core.ActionSubmissionStats{
		PhaseID:        phaseID,
		TotalPlayers:   totalPlayers,
		SubmittedCount: submittedCount,
		SubmissionRate: submissionRate,
	}

	if latestSubmission != nil {
		result.LatestSubmission = latestSubmission
	}

	return result, nil
}
