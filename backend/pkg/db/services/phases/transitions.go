package phases

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	core "actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	db "actionphase/pkg/db/services"
	"actionphase/pkg/observability"

	"github.com/jackc/pgx/v5/pgtype"
)

// TransitionToNextPhase creates and activates a new phase, deactivating the current one if it exists
func (ps *PhaseService) TransitionToNextPhase(ctx context.Context, gameID, userID int32, req core.TransitionPhaseRequest) (*models.GamePhase, error) {
	defer ps.Logger.LogOperation(ctx, "transition_to_next_phase",
		"game_id", gameID,
		"phase_type", req.PhaseType,
		"initiated_by_user_id", userID,
	)()

	// Start transaction for atomic phase transition
	ps.Logger.Debug(ctx, "Starting phase transition transaction", "game_id", gameID)
	tx, err := ps.DB.Begin(ctx)
	if err != nil {
		ps.Logger.LogError(ctx, err, "Failed to begin phase transition transaction", "game_id", gameID)
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil {
			ps.Logger.Debug(ctx, "Transaction already committed (rollback ignored)", "game_id", gameID)
		}
	}()

	txQueries := models.New(tx)

	// Get current active phase
	currentPhase, err := txQueries.GetActivePhase(ctx, gameID)
	var currentPhaseID *int32
	if err == nil {
		currentPhaseID = &currentPhase.ID
		ps.Logger.Info(ctx, "Deactivating current phase",
			"game_id", gameID,
			"phase_id", currentPhase.ID,
			"phase_number", currentPhase.PhaseNumber,
			"phase_type", currentPhase.PhaseType,
		)
		// Deactivate current phase
		_, err = txQueries.DeactivatePhase(ctx, currentPhase.ID)
		if err != nil {
			ps.Logger.LogError(ctx, err, "Failed to deactivate current phase",
				"game_id", gameID,
				"phase_id", currentPhase.ID,
			)
			return nil, fmt.Errorf("failed to deactivate current phase: %w", err)
		}
		ps.Logger.Info(ctx, "Current phase deactivated",
			"game_id", gameID,
			"phase_id", currentPhase.ID,
		)
	} else {
		ps.Logger.Info(ctx, "No active phase to deactivate", "game_id", gameID)
	}

	// Get next phase number
	latestPhaseNum, err := txQueries.GetLatestPhaseNumber(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest phase number: %w", err)
	}

	var phaseNumber int32 = 1
	if latestPhaseNum != nil {
		switch val := latestPhaseNum.(type) {
		case int32:
			phaseNumber = val + 1
		case int64:
			phaseNumber = int32(val) + 1
		}
	}

	// Calculate timing
	startTime := time.Now()
	var endTime *time.Time
	if req.Duration != nil {
		calcEndTime := startTime.Add(*req.Duration)
		endTime = &calcEndTime
	} else if req.EndTime != nil {
		endTime = req.EndTime
	}

	// Create new phase
	createReq := core.CreatePhaseRequest{
		GameID:      gameID,
		PhaseType:   req.PhaseType,
		PhaseNumber: phaseNumber,
		Title:       req.Title,
		Description: req.Description,
		StartTime:   &startTime,
		EndTime:     endTime,
		Deadline:    req.Deadline,
	}

	params := models.CreateGamePhaseParams{
		GameID:      createReq.GameID,
		PhaseType:   createReq.PhaseType,
		PhaseNumber: createReq.PhaseNumber,
		Title:       createReq.Title,
		Description: pgtype.Text{String: createReq.Description, Valid: createReq.Description != ""},
		StartTime:   pgtype.Timestamptz{Time: startTime, Valid: true},
	}

	if createReq.EndTime != nil {
		params.EndTime = pgtype.Timestamptz{Time: *createReq.EndTime, Valid: true}
	}
	if createReq.Deadline != nil {
		params.Deadline = pgtype.Timestamptz{Time: *createReq.Deadline, Valid: true}
	}

	ps.Logger.Info(ctx, "Creating new phase",
		"game_id", gameID,
		"phase_number", phaseNumber,
		"phase_type", req.PhaseType,
		"title", req.Title,
	)
	newPhase, err := txQueries.CreateGamePhase(ctx, params)
	if err != nil {
		ps.Logger.LogError(ctx, err, "Failed to create new phase",
			"game_id", gameID,
			"phase_type", req.PhaseType,
		)
		return nil, fmt.Errorf("failed to create new phase: %w", err)
	}

	ps.Logger.Info(ctx, "Activating new phase",
		"game_id", gameID,
		"phase_id", newPhase.ID,
		"phase_number", newPhase.PhaseNumber,
	)
	// Activate the new phase
	activePhase, err := txQueries.ActivatePhase(ctx, newPhase.ID)
	if err != nil {
		ps.Logger.LogError(ctx, err, "Failed to activate new phase",
			"game_id", gameID,
			"phase_id", newPhase.ID,
		)
		return nil, fmt.Errorf("failed to activate new phase: %w", err)
	}

	_, err = txQueries.CreateLog(ctx, models.CreateLogParams{
		GameID:  gameID,
		Type:    "PHASE_ACTIVATED",
		Message: pgtype.Text{String: fmt.Sprintf("Game phase changed to: %s", newPhase.Title), Valid: true},
	})
	if err != nil {
		ps.Logger.LogError(ctx, err, "Failed to log phase activation record", "game_id", gameID)
		return nil, fmt.Errorf("failed to log phase activation: %w", err)
	}

	// Log the transition
	transitionParams := models.CreatePhaseTransitionParams{
		GameID:      gameID,
		ToPhaseID:   newPhase.ID,
		InitiatedBy: userID,
		Reason:      pgtype.Text{String: req.Reason, Valid: req.Reason != ""},
	}
	if currentPhaseID != nil {
		transitionParams.FromPhaseID = pgtype.Int4{Int32: *currentPhaseID, Valid: true}
	}

	_, err = txQueries.CreatePhaseTransition(ctx, transitionParams)
	if err != nil {
		ps.Logger.LogError(ctx, err, "Failed to log phase transition record", "game_id", gameID)
		return nil, fmt.Errorf("failed to log phase transition: %w", err)
	}

	ps.Logger.Debug(ctx, "Committing phase transition transaction", "game_id", gameID)
	if err := tx.Commit(ctx); err != nil {
		ps.Logger.LogError(ctx, err, "Failed to commit phase transition transaction",
			"game_id", gameID,
			"new_phase_id", newPhase.ID,
		)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	ps.Logger.Info(ctx, "Phase transition completed successfully",
		"game_id", gameID,
		"new_phase_id", activePhase.ID,
		"phase_number", activePhase.PhaseNumber,
		"phase_type", activePhase.PhaseType,
		"title", activePhase.Title,
	)

	return &activePhase, nil
}

// ActivatePhase activates a specific phase for a game
func (ps *PhaseService) ActivatePhase(ctx context.Context, phaseID, userID int32) error {
	_, err := ps.activatePhaseInternal(ctx, phaseID)
	return err
}

// activatePhaseInternal is an internal method to avoid recursion
func (ps *PhaseService) activatePhaseInternal(ctx context.Context, phaseID int32) (*models.GamePhase, error) {
	queries := models.New(ps.DB)

	// Get the phase to find the game ID
	phase, err := queries.GetPhase(ctx, phaseID)
	if err != nil {
		ps.Logger.LogError(ctx, err, "Failed to get phase for activation", "phase_id", phaseID)
		return nil, fmt.Errorf("failed to get phase: %w", err)
	}

	ps.Logger.Debug(ctx, "Starting phase activation transaction",
		"phase_id", phaseID,
		"game_id", phase.GameID,
	)

	// Start transaction to ensure atomicity
	tx, err := ps.DB.Begin(ctx)
	if err != nil {
		ps.Logger.LogError(ctx, err, "Failed to begin phase activation transaction",
			"phase_id", phaseID,
			"game_id", phase.GameID,
		)
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil {
			ps.Logger.Debug(ctx, "Transaction already committed (rollback ignored)",
				"phase_id", phaseID,
				"game_id", phase.GameID,
			)
		}
	}()

	txQueries := models.New(tx)

	// Deactivate all other phases for this game
	err = txQueries.DeactivateAllGamePhases(ctx, phase.GameID)
	if err != nil {
		return nil, fmt.Errorf("failed to deactivate existing phases: %w", err)
	}

	// Clear stale scheduled start times — any inactive phase whose start_time is
	// already in the past would otherwise fire on the next scheduler tick and
	// override this activation. Future-scheduled phases are left untouched.
	err = txQueries.ClearStaleScheduledStartTimes(ctx, models.ClearStaleScheduledStartTimesParams{
		GameID: phase.GameID,
		ID:     phaseID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to clear stale scheduled start times: %w", err)
	}

	// Activate the new phase
	activePhase, err := txQueries.ActivatePhase(ctx, phaseID)
	if err != nil {
		ps.Logger.LogError(ctx, err, "Failed to activate phase",
			"phase_id", phaseID,
			"game_id", phase.GameID,
		)
		return nil, fmt.Errorf("failed to activate phase: %w", err)
	}

	_, err = txQueries.CreateLog(ctx, models.CreateLogParams{
		GameID:  phase.GameID,
		Type:    "PHASE_ACTIVATED",
		Message: pgtype.Text{String: fmt.Sprintf("Game phase changed to: %s", phase.Title), Valid: true},
	})
	if err != nil {
		ps.Logger.LogError(ctx, err, "Failed to log phase activation record", "game_id", phase.GameID)
		return nil, fmt.Errorf("failed to log phase activation: %w", err)
	}

	// Count draft posts before publishing (non-fatal if this fails)
	draftCount, countErr := txQueries.CountDraftPostsByPhase(ctx, pgtype.Int4{Int32: phaseID, Valid: true})
	if countErr != nil {
		ps.Logger.Warn(ctx, "Failed to count draft posts before publishing", "phase_id", phaseID, "error", countErr)
	}

	// Publish any draft posts for this phase atomically with activation
	if err := txQueries.PublishDraftPostsForPhase(ctx, pgtype.Int4{Int32: phaseID, Valid: true}); err != nil {
		ps.Logger.LogError(ctx, err, "Failed to publish draft posts during phase activation",
			"phase_id", phaseID,
		)
		return nil, fmt.Errorf("failed to publish draft posts: %w", err)
	}

	ps.Logger.Debug(ctx, "Committing phase activation transaction",
		"phase_id", phaseID,
		"game_id", phase.GameID,
	)
	if err := tx.Commit(ctx); err != nil {
		ps.Logger.LogError(ctx, err, "Failed to commit phase activation transaction",
			"phase_id", phaseID,
			"game_id", phase.GameID,
		)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	ps.Logger.Info(ctx, "Phase activated successfully",
		"phase_id", activePhase.ID,
		"game_id", phase.GameID,
		"phase_number", activePhase.PhaseNumber,
		"phase_type", activePhase.PhaseType,
	)

	if countErr == nil && draftCount > 0 {
		ps.Logger.Info(ctx, "Draft posts published on phase activation",
			"phase_id", phaseID,
			"game_id", phase.GameID,
			"draft_posts_published", draftCount,
		)
	}

	// Preserve context values (correlation_id, trace_id) without inheriting cancellation
	notifCtx := context.WithoutCancel(ctx)

	// Trigger notifications for phase activation (fire-and-forget)
	observability.SafeGo(notifCtx, ps.Logger, "notify-phase-activated", func() {
		ps.notifyPhaseActivated(notifCtx, phase.GameID, activePhase.ID, activePhase.Title, 0)
	})

	return &activePhase, nil
}

// DeactivatePhase deactivates the currently active phase for a game
func (ps *PhaseService) DeactivatePhase(ctx context.Context, gameID, userID int32) error {
	// Get active phase
	activePhase, err := ps.GetActivePhase(ctx, gameID)
	if err != nil {
		return fmt.Errorf("failed to get active phase: %w", err)
	}
	if activePhase == nil {
		return fmt.Errorf("no active phase to deactivate")
	}

	_, err = ps.deactivatePhaseInternal(ctx, activePhase.ID)
	return err
}

// deactivatePhaseInternal is an internal method to avoid recursion
func (ps *PhaseService) deactivatePhaseInternal(ctx context.Context, phaseID int32) (*models.GamePhase, error) {
	queries := models.New(ps.DB)

	phase, err := queries.DeactivatePhase(ctx, phaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to deactivate phase: %w", err)
	}

	return &phase, nil
}

// notifyPhaseActivated sends notifications to game participants when a phase is activated
func (ps *PhaseService) notifyPhaseActivated(ctx context.Context, gameID, phaseID int32, phaseTitle string, excludeUserID int32) {
	notificationService := db.NewNotificationService(ps.DB, ps.Logger)

	// Notify all participants except the GM who activated the phase
	err := notificationService.NotifyPhaseCreated(
		ctx,
		gameID,
		phaseID,
		phaseTitle,
		excludeUserID,
	)
	if err != nil {
		slog.Error("Failed to send phase activation notifications", "error", err, "game_id", gameID, "phase_id", phaseID)
	}
}
