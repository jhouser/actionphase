package actions

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

// CreateActionResult creates a new action result (GM response to player action)
func (as *ActionSubmissionService) CreateActionResult(ctx context.Context, req core.CreateActionResultRequest) (*models.ActionResult, error) {
	queries := models.New(as.DB)

	// Get the game to find the GM user ID
	game, err := queries.GetGame(ctx, req.GameID)
	if err != nil {
		return nil, fmt.Errorf("failed to get game: %w", err)
	}

	// Convert content to string
	contentStr := fmt.Sprintf("%v", req.Content)

	// Validate content length
	if err := validation.ValidateActionResult(contentStr); err != nil {
		return nil, err
	}

	// Convert character_id to pgtype.Int4
	var characterID pgtype.Int4
	if req.CharacterID != nil {
		characterID = pgtype.Int4{Int32: *req.CharacterID, Valid: true}
	}

	// Convert action_submission_id to pgtype.Int4
	var actionSubmissionID pgtype.Int4
	if req.ActionSubmissionID != nil {
		actionSubmissionID = pgtype.Int4{Int32: *req.ActionSubmissionID, Valid: true}
	}

	params := models.CreateActionResultParams{
		GameID:             req.GameID,
		UserID:             req.UserID,
		PhaseID:            req.PhaseID,
		CharacterID:        characterID,
		ActionSubmissionID: actionSubmissionID,
		GmUserID:           game.GmUserID,
		Content:            contentStr,
		IsPublished:        pgtype.Bool{Bool: req.IsPublished, Valid: true},
	}

	result, err := queries.CreateActionResult(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create action result: %w", err)
	}

	as.Logger.Info(ctx, "Action result created",
		"result_id", result.ID,
		"submission_id", result.ActionSubmissionID,
		"game_id", result.GameID,
	)

	return &result, nil
}

// GetActionResult retrieves a specific action result by ID
func (as *ActionSubmissionService) GetActionResult(ctx context.Context, resultID int32) (*models.ActionResult, error) {
	queries := models.New(as.DB)

	result, err := queries.GetActionResult(ctx, resultID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("action result not found")
		}
		return nil, fmt.Errorf("failed to get action result: %w", err)
	}

	return &result, nil
}

// GetUserPhaseResults retrieves all action results for a user in a specific phase
func (as *ActionSubmissionService) GetUserPhaseResults(ctx context.Context, phaseID, userID int32) ([]models.ActionResult, error) {
	queries := models.New(as.DB)

	results, err := queries.GetUserPhaseResults(ctx, models.GetUserPhaseResultsParams{
		PhaseID: phaseID,
		UserID:  userID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get user phase results: %w", err)
	}

	return results, nil
}

// publishDraftUpdates applies draft character updates to character_data.
// Each draft row stores the complete desired final state for a (character_id, module_type, field_name)
// combination — no merging required, just write the value directly.
func (as *ActionSubmissionService) publishDraftUpdates(ctx context.Context, queries *models.Queries, resultID int32) error {
	// Get all draft updates for this result
	drafts, err := queries.GetDraftCharacterUpdates(ctx, resultID)
	if err != nil {
		return fmt.Errorf("failed to get draft updates: %w", err)
	}

	if len(drafts) == 0 {
		return nil // Nothing to publish
	}

	// Each draft row is a complete snapshot — write it directly to character_data.
	// Abilities/skills/items/currency access is gated at the tab level, not by is_public.
	for _, draft := range drafts {
		if !draft.FieldValue.Valid {
			continue
		}
		_, err := queries.CreateCharacterData(ctx, models.CreateCharacterDataParams{
			CharacterID: draft.CharacterID,
			ModuleType:  draft.ModuleType,
			FieldName:   draft.FieldName,
			FieldValue:  draft.FieldValue,
			FieldType:   pgtype.Text{String: draft.FieldType, Valid: true},
			IsPublic:    pgtype.Bool{Bool: false, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("failed to publish draft for character %d %s/%s: %w",
				draft.CharacterID, draft.ModuleType, draft.FieldName, err)
		}
	}

	return nil
}

// publishSingleResultWithDrafts is a helper that publishes a single result and its draft updates.
// This is called by both PublishActionResult and PublishAllPhaseResults to ensure consistent behavior.
// The queries parameter must be from a transaction context to ensure atomicity.
func (as *ActionSubmissionService) publishSingleResultWithDrafts(ctx context.Context, queries *models.Queries, resultID int32) error {
	// Step 1: Publish the action result (marks it as published).
	//
	// Publishes the whole chain, so a chain built up part by part goes out
	// intact: an unpublished follower is invisible to the release worker and
	// would otherwise be stranded forever. For an ordinary single result the
	// chain is just itself, and this behaves exactly as it always has.
	published, err := queries.PublishActionResult(ctx, resultID)
	if err != nil {
		return fmt.Errorf("failed to publish action result %d: %w", resultID, err)
	}
	if len(published) == 0 {
		return fmt.Errorf("failed to publish action result %d: no rows updated", resultID)
	}

	// The notification names the result the GM published, not whichever row the
	// UPDATE happened to return last — for a chain the recipient is the same
	// either way, but the link and related ID must point at the head.
	result := published[0]
	for _, row := range published {
		if row.ID == resultID {
			result = row
			break
		}
	}

	// Step 1.5: Create notification for the player
	content := "Your action result has been published by the GM"
	linkURL := fmt.Sprintf("/games/%d?tab=actions", result.GameID)
	relatedType := "action_result"
	_, notifErr := as.NotificationService.CreateNotification(ctx, &core.CreateNotificationRequest{
		UserID:      result.UserID,
		GameID:      &result.GameID,
		Type:        core.NotificationTypeActionResult,
		Title:       "Action Result Published",
		Content:     &content,
		RelatedType: &relatedType,
		RelatedID:   &result.ID,
		LinkURL:     &linkURL,
	})
	if notifErr != nil {
		// Log error but don't fail the publish operation
		as.Logger.LogError(ctx, notifErr, "Failed to create notification for published result",
			"result_id", resultID,
			"user_id", result.UserID,
		)
	}

	// Step 2: Publish draft character updates
	// Each draft row contains the complete desired final state — write directly to character_data
	//
	// Scoped to resultID alone, deliberately, even though step 1 published the
	// whole chain. A chain's sheet updates are staged on its FINAL part and
	// apply when that part is released (see ReleaseDueStagedParts), so applying
	// them here would grant the reward as the scene opens rather than when it
	// resolves. Publishing a chain head therefore applies no follower's drafts.
	err = as.publishDraftUpdates(ctx, queries, resultID)
	if err != nil {
		return fmt.Errorf("failed to publish draft character updates for result %d: %w", resultID, err)
	}

	// Step 3: Delete the published drafts (cleanup)
	err = queries.DeletePublishedDrafts(ctx, resultID)
	if err != nil {
		return fmt.Errorf("failed to delete published drafts for result %d: %w", resultID, err)
	}

	return nil
}

// PublishActionResult publishes a single action result, making it visible to the player.
// This includes publishing any draft character updates associated with the result.
// All operations are performed in a transaction to ensure atomicity.
func (as *ActionSubmissionService) PublishActionResult(ctx context.Context, resultID, userID int32) error {
	err := pgx.BeginFunc(ctx, as.DB, func(tx pgx.Tx) error {
		queries := models.New(tx)
		return as.publishSingleResultWithDrafts(ctx, queries, resultID)
	})
	if err != nil {
		return err
	}

	as.Logger.Info(ctx, "Action result published",
		"result_id", resultID,
		"user_id", userID,
	)

	return nil
}

// PublishAllPhaseResults publishes all unpublished results for a phase.
// This includes publishing draft character updates for each result.
// All operations are performed in a single transaction to ensure atomicity.
func (as *ActionSubmissionService) PublishAllPhaseResults(ctx context.Context, phaseID int32) error {
	var count int
	err := pgx.BeginFunc(ctx, as.DB, func(tx pgx.Tx) error {
		queries := models.New(tx)

		// Get all unpublished result IDs for this phase
		resultIDs, err := queries.GetUnpublishedResultIDs(ctx, phaseID)
		if err != nil {
			return fmt.Errorf("failed to get unpublished result IDs: %w", err)
		}

		// Publish each result and its draft character updates using shared logic
		for _, resultID := range resultIDs {
			if err := as.publishSingleResultWithDrafts(ctx, queries, resultID); err != nil {
				return err // Error already has context from helper
			}
		}

		count = len(resultIDs)
		return nil
	})
	if err != nil {
		return err
	}

	as.Logger.Info(ctx, "All phase results published",
		"phase_id", phaseID,
		"count", count,
	)

	return nil
}

// GetUnpublishedResultsCount retrieves the count of unpublished results for a phase
func (as *ActionSubmissionService) GetUnpublishedResultsCount(ctx context.Context, phaseID int32) (int64, error) {
	queries := models.New(as.DB)

	count, err := queries.GetUnpublishedResultsCount(ctx, phaseID)
	if err != nil {
		return 0, fmt.Errorf("failed to get unpublished results count: %w", err)
	}

	return count, nil
}

// DeleteActionResult deletes an unpublished (draft) action result and its associated draft character updates.
// Returns an error if the result is already published.
func (as *ActionSubmissionService) DeleteActionResult(ctx context.Context, resultID int32) error {
	err := pgx.BeginFunc(ctx, as.DB, func(tx pgx.Tx) error {
		queries := models.New(tx)

		// Verify the result exists and is unpublished
		result, err := queries.GetActionResult(ctx, resultID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("action result not found")
			}
			return fmt.Errorf("failed to get action result: %w", err)
		}
		if result.IsPublished.Bool {
			return fmt.Errorf("cannot delete a published action result")
		}

		// Delete associated draft character updates first
		if err := queries.DeletePublishedDrafts(ctx, resultID); err != nil {
			return fmt.Errorf("failed to delete draft character updates: %w", err)
		}

		// Delete the action result (only if unpublished, enforced by SQL)
		if err := queries.DeleteActionResult(ctx, resultID); err != nil {
			return fmt.Errorf("failed to delete action result: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	as.Logger.Info(ctx, "Draft action result deleted", "result_id", resultID)
	return nil
}

// UpdateActionResult updates the content of an unpublished action result
func (as *ActionSubmissionService) UpdateActionResult(ctx context.Context, resultID int32, content string) (*models.ActionResult, error) {
	queries := models.New(as.DB)

	result, err := queries.UpdateActionResult(ctx, models.UpdateActionResultParams{
		ID:      resultID,
		Content: content,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("result not found or already published")
		}
		return nil, fmt.Errorf("failed to update action result: %w", err)
	}

	return &result, nil
}
