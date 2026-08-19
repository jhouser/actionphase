package actions

import (
	"context"
	"fmt"

	core "actionphase/pkg/core"
	models "actionphase/pkg/db/models"

	"github.com/jackc/pgx/v5/pgtype"
)

// CreateDraftCharacterUpdate creates or updates a draft character sheet update for an action result.
// Uses upsert behavior - if draft already exists for this field, it updates the value.
func (s *ActionSubmissionService) CreateDraftCharacterUpdate(ctx context.Context, req core.CreateDraftCharacterUpdateRequest) (*models.ActionResultCharacterUpdate, error) {
	queries := models.New(s.DB)

	// Validate module type
	// Mirrors the check_module_type constraint on action_result_character_updates.
	// "abilities" was retired and "currency" renamed to "numbers" in the character
	// sheet refactor; keep this in step with the constraint or inserts fail at the
	// database instead of here, where the error is actionable.
	validModules := map[string]bool{"skills": true, "inventory": true, "numbers": true}
	if !validModules[req.ModuleType] {
		return nil, fmt.Errorf("invalid module_type: %s", req.ModuleType)
	}

	// Validate field type
	validFieldTypes := map[string]bool{"text": true, "number": true, "boolean": true, "json": true}
	if !validFieldTypes[req.FieldType] {
		return nil, fmt.Errorf("invalid field_type: %s", req.FieldType)
	}

	// Validate operation
	validOperations := map[string]bool{"upsert": true, "delete": true}
	if !validOperations[req.Operation] {
		return nil, fmt.Errorf("invalid operation: %s", req.Operation)
	}

	// Create or update the draft
	draft, err := queries.CreateDraftCharacterUpdate(ctx, models.CreateDraftCharacterUpdateParams{
		ActionResultID: req.ActionResultID,
		CharacterID:    req.CharacterID,
		ModuleType:     req.ModuleType,
		FieldName:      req.FieldName,
		FieldValue:     pgtype.Text{String: req.FieldValue, Valid: true},
		FieldType:      req.FieldType,
		Operation:      req.Operation,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create draft character update: %w", err)
	}

	return &draft, nil
}

// GetDraftCharacterUpdates retrieves all draft updates for an action result.
func (s *ActionSubmissionService) GetDraftCharacterUpdates(ctx context.Context, actionResultID int32) ([]models.ActionResultCharacterUpdate, error) {
	queries := models.New(s.DB)

	drafts, err := queries.GetDraftCharacterUpdates(ctx, actionResultID)
	if err != nil {
		return nil, fmt.Errorf("failed to get draft character updates: %w", err)
	}

	return drafts, nil
}

// UpdateDraftCharacterUpdate updates the field value of an existing draft.
func (s *ActionSubmissionService) UpdateDraftCharacterUpdate(ctx context.Context, draftID int32, fieldValue string) (*models.ActionResultCharacterUpdate, error) {
	queries := models.New(s.DB)

	draft, err := queries.UpdateDraftCharacterUpdate(ctx, models.UpdateDraftCharacterUpdateParams{
		ID:         draftID,
		FieldValue: pgtype.Text{String: fieldValue, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update draft character update: %w", err)
	}

	return &draft, nil
}

// DeleteDraftCharacterUpdate removes a draft character update.
func (s *ActionSubmissionService) DeleteDraftCharacterUpdate(ctx context.Context, draftID int32) error {
	queries := models.New(s.DB)

	err := queries.DeleteDraftCharacterUpdate(ctx, draftID)
	if err != nil {
		return fmt.Errorf("failed to delete draft character update: %w", err)
	}

	return nil
}

// GetDraftUpdateCount returns the count of draft updates for an action result.
func (s *ActionSubmissionService) GetDraftUpdateCount(ctx context.Context, actionResultID int32) (int64, error) {
	queries := models.New(s.DB)

	count, err := queries.GetDraftUpdateCount(ctx, actionResultID)
	if err != nil {
		return 0, fmt.Errorf("failed to get draft update count: %w", err)
	}

	return count, nil
}
