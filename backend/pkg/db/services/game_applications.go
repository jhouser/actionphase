package db

import (
	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	"actionphase/pkg/observability"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GameApplicationService implements the GameApplicationServiceInterface for database operations
type GameApplicationService struct {
	DB     *pgxpool.Pool
	Logger *observability.Logger
}

// CreateGameApplication creates a new application to join a game
func (gas *GameApplicationService) CreateGameApplication(ctx context.Context, req core.CreateGameApplicationRequest) (*models.GameApplication, error) {
	queries := models.New(gas.DB)

	// Validate role
	if !core.IsValidParticipantRole(req.Role) && req.Role != core.RolePlayer && req.Role != core.RoleAudience {
		return nil, fmt.Errorf("invalid role: %s", req.Role)
	}

	// Check if user can apply
	canApplyStatus, err := queries.CanUserApplyToGame(ctx, models.CanUserApplyToGameParams{
		GameID: req.GameID,
		UserID: req.UserID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to check if user can apply: %w", err)
	}

	if canApplyStatus != core.CanApply {
		switch canApplyStatus {
		case core.IsGameMaster:
			return nil, fmt.Errorf("game master cannot apply to their own game")
		case core.AlreadyParticipant:
			return nil, fmt.Errorf("user is already a participant in this game")
		case core.ApplicationPending:
			return nil, fmt.Errorf("user already has a pending application for this game")
		case core.ApplicationRejected:
			return nil, fmt.Errorf("user's previous application was rejected")
		case core.NotRecruiting:
			// Audience can apply at any time, but players can only apply during recruitment
			if req.Role != core.RoleAudience {
				return nil, fmt.Errorf("game is not currently recruiting")
			}
		default:
			return nil, fmt.Errorf("cannot apply to game: %s", canApplyStatus)
		}
	}

	// Delete any stale rejected application — this happens when a user was rejected,
	// later added directly as a participant (superseding the rejection), and has since
	// been removed. The unique constraint on (game_id, user_id) would otherwise block
	// the new application INSERT.
	if err := queries.DeleteRejectedApplicationForUser(ctx, models.DeleteRejectedApplicationForUserParams{
		GameID: req.GameID,
		UserID: req.UserID,
	}); err != nil {
		return nil, fmt.Errorf("failed to clear stale rejected application: %w", err)
	}

	// Delete any stale approved application — this happens when a user's application was
	// approved (making them a participant), and they later left/were removed from the game.
	// Without this, the unique constraint on (game_id, user_id) permanently blocks re-applying.
	if err := queries.DeleteStaleApprovedApplicationForUser(ctx, models.DeleteStaleApprovedApplicationForUserParams{
		GameID: req.GameID,
		UserID: req.UserID,
	}); err != nil {
		return nil, fmt.Errorf("failed to clear stale approved application: %w", err)
	}

	// Create the application
	var messageText pgtype.Text
	if req.Message != "" {
		messageText = pgtype.Text{String: req.Message, Valid: true}
	}

	appRow, err := queries.CreateGameApplication(ctx, models.CreateGameApplicationParams{
		GameID:  req.GameID,
		UserID:  req.UserID,
		Role:    req.Role,
		Message: messageText,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create game application: %w", err)
	}

	// Convert CreateGameApplicationRow to GameApplication
	application := &models.GameApplication{
		ID:               appRow.ID,
		GameID:           appRow.GameID,
		UserID:           appRow.UserID,
		Role:             appRow.Role,
		Message:          appRow.Message,
		Status:           appRow.Status,
		ReviewedByUserID: appRow.ReviewedByUserID,
		ReviewedAt:       appRow.ReviewedAt,
		AppliedAt:        appRow.AppliedAt,
	}

	return application, nil
}

// GetGameApplication retrieves a specific application by ID
func (gas *GameApplicationService) GetGameApplication(ctx context.Context, applicationID int32) (*models.GameApplication, error) {
	queries := models.New(gas.DB)

	appRow, err := queries.GetGameApplication(ctx, applicationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get game application: %w", err)
	}

	// Convert GetGameApplicationRow to GameApplication
	application := &models.GameApplication{
		ID:               appRow.ID,
		GameID:           appRow.GameID,
		UserID:           appRow.UserID,
		Role:             appRow.Role,
		Message:          appRow.Message,
		Status:           appRow.Status,
		ReviewedByUserID: appRow.ReviewedByUserID,
		ReviewedAt:       appRow.ReviewedAt,
		AppliedAt:        appRow.AppliedAt,
	}

	return application, nil
}

// GetGameApplications retrieves all applications for a game with user details
func (gas *GameApplicationService) GetGameApplications(ctx context.Context, gameID int32) ([]models.GetGameApplicationsRow, error) {
	queries := models.New(gas.DB)

	applications, err := queries.GetGameApplications(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to get game applications: %w", err)
	}

	return applications, nil
}

// GetGameApplicationsByStatus retrieves applications for a game filtered by status
func (gas *GameApplicationService) GetGameApplicationsByStatus(ctx context.Context, gameID int32, status string) ([]models.GetGameApplicationsByStatusRow, error) {
	queries := models.New(gas.DB)

	// Validate status
	if !core.IsValidApplicationStatus(status) {
		return nil, fmt.Errorf("invalid application status: %s", status)
	}

	applications, err := queries.GetGameApplicationsByStatus(ctx, models.GetGameApplicationsByStatusParams{
		GameID: gameID,
		Status: pgtype.Text{String: status, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get game applications by status: %w", err)
	}

	return applications, nil
}

// GetUserGameApplications retrieves all applications submitted by a user
func (gas *GameApplicationService) GetUserGameApplications(ctx context.Context, userID int32) ([]models.GetUserGameApplicationsRow, error) {
	queries := models.New(gas.DB)

	applications, err := queries.GetUserGameApplications(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user game applications: %w", err)
	}

	return applications, nil
}

// ApproveGameApplication approves an application and optionally creates participant
func (gas *GameApplicationService) ApproveGameApplication(ctx context.Context, applicationID, reviewerID int32) error {
	queries := models.New(gas.DB)

	// Get the application to check the role
	application, err := gas.GetGameApplication(ctx, applicationID)
	if err != nil {
		return fmt.Errorf("failed to get game application: %w", err)
	}

	// Update application status
	_, err = queries.UpdateGameApplicationStatus(ctx, models.UpdateGameApplicationStatusParams{
		ID:               applicationID,
		Status:           pgtype.Text{String: core.ApplicationStatusApproved, Valid: true},
		ReviewedByUserID: pgtype.Int4{Int32: reviewerID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to approve game application: %w", err)
	}

	// For audience applications, create participant immediately
	// Audience members can apply at any time (not just during recruitment),
	// so they should become participants as soon as approved.
	//
	// Once the participant exists, the application record has served its purpose and
	// is deleted: approved audience members should exist ONLY as participants, never
	// as a lingering 'approved' application. This matches the two other approval paths
	// (auto-accept in games.ApplyToGame, and recruitment-close in
	// ConvertApprovedApplicationsToParticipants), both of which already delete the
	// audience application after creating the participant. Leaving it behind is what
	// caused stale 'approved' rows that blocked re-applying, broke withdrawal, and
	// showed contradictory UI after a member left.
	if application.Role == core.RoleAudience {
		_, err = queries.AddGameParticipant(ctx, models.AddGameParticipantParams{
			GameID: application.GameID,
			UserID: application.UserID,
			Role:   application.Role,
		})
		if err != nil {
			return fmt.Errorf("failed to create participant from audience application: %w", err)
		}

		if err = queries.DeleteGameApplication(ctx, models.DeleteGameApplicationParams{
			ID:     applicationID,
			UserID: application.UserID,
		}); err != nil {
			return fmt.Errorf("failed to delete audience application after creating participant: %w", err)
		}
	}

	// Note: For player applications during recruitment, the participant entry
	// is created when transitioning out of recruitment. This allows the GM to
	// approve applications without immediately adding them as participants.

	gas.Logger.Info(ctx, "Game application approved",
		"application_id", applicationID,
		"game_id", application.GameID,
		"user_id", application.UserID,
		"role", application.Role,
	)

	return nil
}

// RejectGameApplication rejects an application
func (gas *GameApplicationService) RejectGameApplication(ctx context.Context, applicationID, reviewerID int32) error {
	queries := models.New(gas.DB)

	application, err := gas.GetGameApplication(ctx, applicationID)
	if err != nil {
		return fmt.Errorf("failed to get game application: %w", err)
	}

	_, err = queries.UpdateGameApplicationStatus(ctx, models.UpdateGameApplicationStatusParams{
		ID:               applicationID,
		Status:           pgtype.Text{String: core.ApplicationStatusRejected, Valid: true},
		ReviewedByUserID: pgtype.Int4{Int32: reviewerID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to reject game application: %w", err)
	}

	gas.Logger.Info(ctx, "Game application rejected",
		"application_id", applicationID,
		"game_id", application.GameID,
		"user_id", application.UserID,
		"reviewer_id", reviewerID,
	)

	return nil
}

// DeleteGameApplication removes an application
func (gas *GameApplicationService) DeleteGameApplication(ctx context.Context, applicationID, userID int32) error {
	queries := models.New(gas.DB)

	err := queries.DeleteGameApplication(ctx, models.DeleteGameApplicationParams{
		ID:     applicationID,
		UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete game application: %w", err)
	}

	return nil
}

// DeleteStaleApprovedApplicationForUser removes a leftover 'approved' application for a
// user who is no longer an active participant in the game. It is a no-op if the user is
// still an active participant, so it never removes a live application.
func (gas *GameApplicationService) DeleteStaleApprovedApplicationForUser(ctx context.Context, gameID, userID int32) error {
	queries := models.New(gas.DB)

	err := queries.DeleteStaleApprovedApplicationForUser(ctx, models.DeleteStaleApprovedApplicationForUserParams{
		GameID: gameID,
		UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete stale approved application: %w", err)
	}

	return nil
}

// CanUserApplyToGame checks if a user is eligible to apply to a game
func (gas *GameApplicationService) CanUserApplyToGame(ctx context.Context, gameID, userID int32) (string, error) {
	queries := models.New(gas.DB)

	status, err := queries.CanUserApplyToGame(ctx, models.CanUserApplyToGameParams{
		GameID: gameID,
		UserID: userID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to check if user can apply to game: %w", err)
	}

	return status, nil
}

// HasUserAppliedToGame checks if user has an existing application
func (gas *GameApplicationService) HasUserAppliedToGame(ctx context.Context, gameID, userID int32) (bool, error) {
	queries := models.New(gas.DB)

	hasApplied, err := queries.HasUserAppliedToGame(ctx, models.HasUserAppliedToGameParams{
		GameID: gameID,
		UserID: userID,
	})
	if err != nil {
		return false, fmt.Errorf("failed to check if user has applied to game: %w", err)
	}

	return hasApplied, nil
}

// CountPendingApplicationsForGame returns count of pending applications
func (gas *GameApplicationService) CountPendingApplicationsForGame(ctx context.Context, gameID int32) (int64, error) {
	queries := models.New(gas.DB)

	count, err := queries.CountPendingApplicationsForGame(ctx, gameID)
	if err != nil {
		return 0, fmt.Errorf("failed to count pending applications: %w", err)
	}

	return count, nil
}

// BulkApproveApplications approves all pending applications for a game
func (gas *GameApplicationService) BulkApproveApplications(ctx context.Context, gameID, reviewerID int32) error {
	queries := models.New(gas.DB)

	count, _ := queries.CountPendingApplicationsForGame(ctx, gameID)

	err := queries.BulkApproveApplications(ctx, models.BulkApproveApplicationsParams{
		GameID:           gameID,
		ReviewedByUserID: pgtype.Int4{Int32: reviewerID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to bulk approve applications: %w", err)
	}

	gas.Logger.Info(ctx, "Bulk game applications approved",
		"game_id", gameID,
		"count", count,
	)

	return nil
}

// BulkRejectApplications rejects all pending applications for a game
func (gas *GameApplicationService) BulkRejectApplications(ctx context.Context, gameID, reviewerID int32) error {
	queries := models.New(gas.DB)

	count, _ := queries.CountPendingApplicationsForGame(ctx, gameID)

	err := queries.BulkRejectApplications(ctx, models.BulkRejectApplicationsParams{
		GameID:           gameID,
		ReviewedByUserID: pgtype.Int4{Int32: reviewerID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to bulk reject applications: %w", err)
	}

	gas.Logger.Info(ctx, "Bulk game applications rejected",
		"game_id", gameID,
		"count", count,
	)

	return nil
}

// GetApprovedApplicationsForGame retrieves approved applications for participant creation
func (gas *GameApplicationService) GetApprovedApplicationsForGame(ctx context.Context, gameID int32) ([]models.GetApprovedApplicationsForGameRow, error) {
	queries := models.New(gas.DB)

	applications, err := queries.GetApprovedApplicationsForGame(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to get approved applications: %w", err)
	}

	return applications, nil
}

// GetGameApplicationByUserAndGame retrieves an application by user and game (useful for checking existing applications)
func (gas *GameApplicationService) GetGameApplicationByUserAndGame(ctx context.Context, gameID, userID int32) (*models.GameApplication, error) {
	queries := models.New(gas.DB)

	appRow, err := queries.GetGameApplicationByUserAndGame(ctx, models.GetGameApplicationByUserAndGameParams{
		GameID: gameID,
		UserID: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get game application by user and game: %w", err)
	}

	// Convert GetGameApplicationByUserAndGameRow to GameApplication
	application := &models.GameApplication{
		ID:               appRow.ID,
		GameID:           appRow.GameID,
		UserID:           appRow.UserID,
		Role:             appRow.Role,
		Message:          appRow.Message,
		Status:           appRow.Status,
		ReviewedByUserID: appRow.ReviewedByUserID,
		ReviewedAt:       appRow.ReviewedAt,
		AppliedAt:        appRow.AppliedAt,
	}

	return application, nil
}

// ConvertApprovedApplicationsToParticipants converts approved applications to game participants
// This is typically called when transitioning a game out of recruitment phase
func (gas *GameApplicationService) ConvertApprovedApplicationsToParticipants(ctx context.Context, gameID int32) error {
	// Get approved applications
	approvedApplications, err := gas.GetApprovedApplicationsForGame(ctx, gameID)
	if err != nil {
		return fmt.Errorf("failed to get approved applications: %w", err)
	}

	queries := models.New(gas.DB)

	// Create participants for each approved application and delete the application record
	for _, app := range approvedApplications {
		// Skip audience applications - they were already converted to participants when approved
		// (see ApproveGameApplication in this file)
		if app.Role == core.RoleAudience {
			// Delete the audience application record since they're already a participant
			err := queries.DeleteGameApplication(ctx, models.DeleteGameApplicationParams{
				ID:     app.ID,
				UserID: app.UserID,
			})
			if err != nil {
				return fmt.Errorf("failed to delete audience application %d: %w", app.ID, err)
			}
			continue
		}

		// Create participant for player applications
		_, err := queries.AddGameParticipant(ctx, models.AddGameParticipantParams{
			GameID: app.GameID,
			UserID: app.UserID,
			Role:   app.Role,
		})
		if err != nil {
			return fmt.Errorf("failed to create participant from application %d: %w", app.ID, err)
		}

		// Delete the application record now that user is a participant
		// This prevents confusion - approved applicants should only exist as participants
		err = queries.DeleteGameApplication(ctx, models.DeleteGameApplicationParams{
			ID:     app.ID,
			UserID: app.UserID,
		})
		if err != nil {
			// Log but don't fail - participant is created
			return fmt.Errorf("failed to delete application %d after creating participant: %w", app.ID, err)
		}
	}

	gas.Logger.Info(ctx, "Approved applications converted to participants",
		"game_id", gameID,
		"count", len(approvedApplications),
	)

	return nil
}

// PublishApplicationStatuses marks all application statuses as published for a game
// This is called when GM closes recruitment
func (gas *GameApplicationService) PublishApplicationStatuses(ctx context.Context, gameID int32) error {
	queries := models.New(gas.DB)

	err := queries.PublishApplicationStatuses(ctx, gameID)
	if err != nil {
		return fmt.Errorf("failed to publish application statuses: %w", err)
	}

	return nil
}

// DeleteRejectedApplications deletes all rejected applications for a game
// This is called when transitioning out of recruitment to clean up rejected applications
func (gas *GameApplicationService) DeleteRejectedApplications(ctx context.Context, gameID int32) error {
	queries := models.New(gas.DB)

	err := queries.DeleteRejectedApplications(ctx, gameID)
	if err != nil {
		return fmt.Errorf("failed to delete rejected applications: %w", err)
	}

	return nil
}

// GetPublicGameApplicants retrieves the public list of applicants for a game
// Returns only username and role - NO status or review information
// Available to anyone (no permission check) when game is in recruiting state
func (gas *GameApplicationService) GetPublicGameApplicants(ctx context.Context, gameID int32) ([]models.GetPublicGameApplicantsRow, error) {
	queries := models.New(gas.DB)

	applicants, err := queries.GetPublicGameApplicants(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to get public game applicants: %w", err)
	}

	return applicants, nil
}
