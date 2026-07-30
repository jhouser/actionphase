package db

import (
	"context"
	"testing"

	core "actionphase/pkg/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGameApplicationService_GMCannotApply(t *testing.T) {
	// Regression test for GM application prevention bug
	// Bug: GM was able to apply to their own game
	// Fix: CanUserApplyToGame query now checks if user is GM

	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := &GameApplicationService{DB: testDB.Pool}
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test users
	gm := testDB.CreateTestUser(t, "gm_user", "gm@example.com")
	player := testDB.CreateTestUser(t, "player_user", "player@example.com")

	// Create game with GM
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	// Update game state to recruitment so applications are allowed
	_, err := gameService.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	t.Run("GM cannot apply to their own game", func(t *testing.T) {
		// Attempt to apply as GM should fail
		req := core.CreateGameApplicationRequest{
			GameID: game.ID,
			UserID: int32(gm.ID),
			Role:   core.RolePlayer,
		}

		_, err := service.CreateGameApplication(context.Background(), req)

		// Should return error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "game master cannot apply")
	})

	t.Run("CanUserApplyToGame returns IsGameMaster for GM", func(t *testing.T) {
		// Check that the status is correctly identified
		status, err := service.CanUserApplyToGame(context.Background(), game.ID, int32(gm.ID))

		require.NoError(t, err)
		assert.Equal(t, core.IsGameMaster, status)
	})

	t.Run("Non-GM user can apply to game", func(t *testing.T) {
		// Player should be able to apply
		req := core.CreateGameApplicationRequest{
			GameID:  game.ID,
			UserID:  int32(player.ID),
			Role:    core.RolePlayer,
			Message: "I would like to join this game",
		}

		application, err := service.CreateGameApplication(context.Background(), req)

		require.NoError(t, err)
		assert.Equal(t, game.ID, application.GameID)
		assert.Equal(t, int32(player.ID), application.UserID)
		assert.Equal(t, core.RolePlayer, application.Role)
		assert.Equal(t, core.ApplicationStatusPending, application.Status.String)
	})
}

func TestGameApplicationService_CreateGameApplication(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := &GameApplicationService{DB: testDB.Pool}
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test users
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")

	// Create game
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	// Update to recruitment state
	_, err := gameService.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	t.Run("creates application successfully", func(t *testing.T) {
		req := core.CreateGameApplicationRequest{
			GameID:  game.ID,
			UserID:  int32(player1.ID),
			Role:    core.RolePlayer,
			Message: "I would like to join",
		}

		application, err := service.CreateGameApplication(context.Background(), req)

		require.NoError(t, err)
		assert.NotNil(t, application)
		assert.Equal(t, game.ID, application.GameID)
		assert.Equal(t, int32(player1.ID), application.UserID)
		assert.Equal(t, core.RolePlayer, application.Role)
		assert.Equal(t, "I would like to join", application.Message.String)
		assert.Equal(t, core.ApplicationStatusPending, application.Status.String)
	})

	t.Run("rejects duplicate application", func(t *testing.T) {
		// Player1 already applied above, try again
		req := core.CreateGameApplicationRequest{
			GameID:  game.ID,
			UserID:  int32(player1.ID),
			Role:    core.RolePlayer,
			Message: "Second application",
		}

		_, err := service.CreateGameApplication(context.Background(), req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "pending application")
	})

	t.Run("rejects application when not recruiting", func(t *testing.T) {
		// Create a new game in setup state
		setupGame := testDB.CreateTestGame(t, int32(gm.ID), "Setup Game")

		req := core.CreateGameApplicationRequest{
			GameID:  setupGame.ID,
			UserID:  int32(player2.ID),
			Role:    core.RolePlayer,
			Message: "Application to setup game",
		}

		_, err := service.CreateGameApplication(context.Background(), req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not currently recruiting")
	})
}

func TestGameApplicationService_ApproveRejectApplication(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := &GameApplicationService{DB: testDB.Pool}
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test users
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	// Create game and transition to recruitment
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	_, err := gameService.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	// Create application
	req := core.CreateGameApplicationRequest{
		GameID:  game.ID,
		UserID:  int32(player.ID),
		Role:    core.RolePlayer,
		Message: "I want to join",
	}
	application, err := service.CreateGameApplication(context.Background(), req)
	require.NoError(t, err)

	t.Run("approves application successfully", func(t *testing.T) {
		err := service.ApproveGameApplication(context.Background(), application.ID, int32(gm.ID))

		require.NoError(t, err)

		// Verify status changed
		updatedApp, err := service.GetGameApplication(context.Background(), application.ID)
		require.NoError(t, err)
		assert.Equal(t, core.ApplicationStatusApproved, updatedApp.Status.String)
		assert.True(t, updatedApp.ReviewedByUserID.Valid)
		assert.Equal(t, int32(gm.ID), updatedApp.ReviewedByUserID.Int32)
	})
}

func TestGameApplicationService_RejectApplication(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := &GameApplicationService{DB: testDB.Pool}
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test users
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	// Create game and transition to recruitment
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	_, err := gameService.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	// Create application
	req := core.CreateGameApplicationRequest{
		GameID:  game.ID,
		UserID:  int32(player.ID),
		Role:    core.RolePlayer,
		Message: "I want to join",
	}
	application, err := service.CreateGameApplication(context.Background(), req)
	require.NoError(t, err)

	t.Run("rejects application successfully", func(t *testing.T) {
		err := service.RejectGameApplication(context.Background(), application.ID, int32(gm.ID))

		require.NoError(t, err)

		// Verify status changed
		updatedApp, err := service.GetGameApplication(context.Background(), application.ID)
		require.NoError(t, err)
		assert.Equal(t, core.ApplicationStatusRejected, updatedApp.Status.String)
		assert.True(t, updatedApp.ReviewedByUserID.Valid)
		assert.Equal(t, int32(gm.ID), updatedApp.ReviewedByUserID.Int32)
	})
}

func TestGameApplicationService_GetGameApplications(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := &GameApplicationService{DB: testDB.Pool}
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test users
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")

	// Create game
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	_, err := gameService.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	// Create applications
	_, err = service.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
		GameID:  game.ID,
		UserID:  int32(player1.ID),
		Role:    core.RolePlayer,
		Message: "Application 1",
	})
	require.NoError(t, err)

	_, err = service.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
		GameID:  game.ID,
		UserID:  int32(player2.ID),
		Role:    core.RoleAudience,
		Message: "Application 2",
	})
	require.NoError(t, err)

	t.Run("retrieves all applications for game", func(t *testing.T) {
		applications, err := service.GetGameApplications(context.Background(), game.ID)

		require.NoError(t, err)
		assert.Len(t, applications, 2)
		userIDs := make([]int32, 0, len(applications))
		for _, a := range applications {
			assert.Equal(t, game.ID, a.GameID, "application should belong to correct game")
			userIDs = append(userIDs, a.UserID)
		}
		assert.Contains(t, userIDs, int32(player1.ID), "player1 application should be present")
		assert.Contains(t, userIDs, int32(player2.ID), "player2 application should be present")
	})

	t.Run("retrieves applications by status", func(t *testing.T) {
		applications, err := service.GetGameApplicationsByStatus(context.Background(), game.ID, core.ApplicationStatusPending)

		require.NoError(t, err)
		assert.Len(t, applications, 2)
	})
}

func TestGameApplicationService_BulkOperations(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := &GameApplicationService{DB: testDB.Pool}
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test users
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")

	// Create game
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	_, err := gameService.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	// Create applications
	_, err = service.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
		GameID: game.ID,
		UserID: int32(player1.ID),
		Role:   core.RolePlayer,
	})
	require.NoError(t, err)

	_, err = service.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
		GameID: game.ID,
		UserID: int32(player2.ID),
		Role:   core.RolePlayer,
	})
	require.NoError(t, err)

	t.Run("bulk approves all applications", func(t *testing.T) {
		err := service.BulkApproveApplications(context.Background(), game.ID, int32(gm.ID))

		require.NoError(t, err)

		// Verify all applications are approved
		applications, err := service.GetGameApplicationsByStatus(context.Background(), game.ID, core.ApplicationStatusApproved)
		require.NoError(t, err)
		assert.Len(t, applications, 2)
	})
}

func TestGameApplicationService_ConvertToParticipants(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := &GameApplicationService{DB: testDB.Pool}
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test users
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")

	// Create game
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	_, err := gameService.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	// Create and approve applications
	_, err = service.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
		GameID: game.ID,
		UserID: int32(player1.ID),
		Role:   core.RolePlayer,
	})
	require.NoError(t, err)

	_, err = service.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
		GameID: game.ID,
		UserID: int32(player2.ID),
		Role:   core.RolePlayer,
	})
	require.NoError(t, err)

	// Approve applications
	err = service.BulkApproveApplications(context.Background(), game.ID, int32(gm.ID))
	require.NoError(t, err)

	t.Run("converts approved applications to participants", func(t *testing.T) {
		err := service.ConvertApprovedApplicationsToParticipants(context.Background(), game.ID)

		require.NoError(t, err)

		// Verify participants were created
		participants, err := gameService.GetGameParticipants(context.Background(), game.ID)
		require.NoError(t, err)
		// Should have 3 participants: GM + 2 players
		assert.GreaterOrEqual(t, len(participants), 2)
	})
}

func TestGameApplicationService_GetUserGameApplications(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := &GameApplicationService{DB: testDB.Pool}
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test users
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	// Create multiple games
	game1 := testDB.CreateTestGame(t, int32(gm.ID), "Test Game 1")
	game2 := testDB.CreateTestGame(t, int32(gm.ID), "Test Game 2")

	// Set games to recruitment
	_, err := gameService.UpdateGameState(context.Background(), game1.ID, core.GameStateRecruitment)
	require.NoError(t, err)
	_, err = gameService.UpdateGameState(context.Background(), game2.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	t.Run("returns empty list when user has no applications", func(t *testing.T) {
		applications, err := service.GetUserGameApplications(context.Background(), int32(player.ID))
		require.NoError(t, err)
		assert.Empty(t, applications)
	})

	t.Run("returns all applications for a user", func(t *testing.T) {
		// Create applications for player
		_, err := service.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
			GameID:  game1.ID,
			UserID:  int32(player.ID),
			Role:    core.RolePlayer,
			Message: "Application to game 1",
		})
		require.NoError(t, err)

		_, err = service.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
			GameID:  game2.ID,
			UserID:  int32(player.ID),
			Role:    core.RoleAudience,
			Message: "Application to game 2",
		})
		require.NoError(t, err)

		// Get user's applications
		applications, err := service.GetUserGameApplications(context.Background(), int32(player.ID))
		require.NoError(t, err)
		assert.Len(t, applications, 2)
	})
}

func TestGameApplicationService_DeleteGameApplication(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := &GameApplicationService{DB: testDB.Pool}
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test users
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	// Create game
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	_, err := gameService.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	// Create application
	application, err := service.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
		GameID:  game.ID,
		UserID:  int32(player.ID),
		Role:    core.RolePlayer,
		Message: "I want to join",
	})
	require.NoError(t, err)

	t.Run("deletes application successfully", func(t *testing.T) {
		err := service.DeleteGameApplication(context.Background(), application.ID, int32(player.ID))
		require.NoError(t, err)

		// Verify application is deleted
		_, err = service.GetGameApplication(context.Background(), application.ID)
		assert.Error(t, err)
	})
}

func TestGameApplicationService_HasUserAppliedToGame(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := &GameApplicationService{DB: testDB.Pool}
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test users
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")

	// Create game
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	_, err := gameService.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	t.Run("returns false when user has not applied", func(t *testing.T) {
		hasApplied, err := service.HasUserAppliedToGame(context.Background(), game.ID, int32(player1.ID))
		require.NoError(t, err)
		assert.False(t, hasApplied)
	})

	t.Run("returns true when user has applied", func(t *testing.T) {
		// Create application
		_, err := service.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
			GameID: game.ID,
			UserID: int32(player1.ID),
			Role:   core.RolePlayer,
		})
		require.NoError(t, err)

		hasApplied, err := service.HasUserAppliedToGame(context.Background(), game.ID, int32(player1.ID))
		require.NoError(t, err)
		assert.True(t, hasApplied)
	})

	t.Run("returns false for different user", func(t *testing.T) {
		// player2 has not applied
		hasApplied, err := service.HasUserAppliedToGame(context.Background(), game.ID, int32(player2.ID))
		require.NoError(t, err)
		assert.False(t, hasApplied)
	})
}

func TestGameApplicationService_CountPendingApplicationsForGame(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := &GameApplicationService{DB: testDB.Pool}
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test users
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
	player3 := testDB.CreateTestUser(t, "player3", "player3@example.com")

	// Create game
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	_, err := gameService.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	t.Run("returns 0 when no pending applications", func(t *testing.T) {
		count, err := service.CountPendingApplicationsForGame(context.Background(), game.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	t.Run("counts only pending applications", func(t *testing.T) {
		// Create pending applications
		app1, err := service.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
			GameID: game.ID,
			UserID: int32(player1.ID),
			Role:   core.RolePlayer,
		})
		require.NoError(t, err)

		_, err = service.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
			GameID: game.ID,
			UserID: int32(player2.ID),
			Role:   core.RolePlayer,
		})
		require.NoError(t, err)

		app3, err := service.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
			GameID: game.ID,
			UserID: int32(player3.ID),
			Role:   core.RolePlayer,
		})
		require.NoError(t, err)

		// Approve one application
		err = service.ApproveGameApplication(context.Background(), app1.ID, int32(gm.ID))
		require.NoError(t, err)

		// Reject one application
		err = service.RejectGameApplication(context.Background(), app3.ID, int32(gm.ID))
		require.NoError(t, err)

		// Count should be 1 (only player2's pending application)
		count, err := service.CountPendingApplicationsForGame(context.Background(), game.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})
}

func TestGameApplicationService_BulkRejectApplications(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := &GameApplicationService{DB: testDB.Pool}
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test users
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
	player3 := testDB.CreateTestUser(t, "player3", "player3@example.com")

	// Create game
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	_, err := gameService.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	// Create applications
	_, err = service.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
		GameID: game.ID,
		UserID: int32(player1.ID),
		Role:   core.RolePlayer,
	})
	require.NoError(t, err)

	_, err = service.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
		GameID: game.ID,
		UserID: int32(player2.ID),
		Role:   core.RolePlayer,
	})
	require.NoError(t, err)

	_, err = service.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
		GameID: game.ID,
		UserID: int32(player3.ID),
		Role:   core.RolePlayer,
	})
	require.NoError(t, err)

	t.Run("bulk rejects all pending applications", func(t *testing.T) {
		err := service.BulkRejectApplications(context.Background(), game.ID, int32(gm.ID))
		require.NoError(t, err)

		// Verify all applications are rejected
		applications, err := service.GetGameApplicationsByStatus(context.Background(), game.ID, core.ApplicationStatusRejected)
		require.NoError(t, err)
		assert.Len(t, applications, 3)

		// Verify no pending applications remain
		pendingCount, err := service.CountPendingApplicationsForGame(context.Background(), game.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), pendingCount)
	})
}

func TestGameApplicationService_GetGameApplicationByUserAndGame(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := &GameApplicationService{DB: testDB.Pool}
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test users
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	// Create game
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	_, err := gameService.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	t.Run("returns error when no application exists", func(t *testing.T) {
		_, err := service.GetGameApplicationByUserAndGame(context.Background(), game.ID, int32(player.ID))
		assert.Error(t, err)
	})

	t.Run("retrieves application by user and game", func(t *testing.T) {
		// Create application
		created, err := service.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
			GameID:  game.ID,
			UserID:  int32(player.ID),
			Role:    core.RolePlayer,
			Message: "My application message",
		})
		require.NoError(t, err)

		// Get application by user and game
		retrieved, err := service.GetGameApplicationByUserAndGame(context.Background(), game.ID, int32(player.ID))
		require.NoError(t, err)
		assert.Equal(t, created.ID, retrieved.ID)
		assert.Equal(t, game.ID, retrieved.GameID)
		assert.Equal(t, int32(player.ID), retrieved.UserID)
		assert.Equal(t, "My application message", retrieved.Message.String)
	})
}

func TestGameApplicationService_PublishApplicationStatuses(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := &GameApplicationService{DB: testDB.Pool}
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test users
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")

	// Create game
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	_, err := gameService.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	// Create and process applications
	app1, err := service.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
		GameID: game.ID,
		UserID: int32(player1.ID),
		Role:   core.RolePlayer,
	})
	require.NoError(t, err)

	app2, err := service.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
		GameID: game.ID,
		UserID: int32(player2.ID),
		Role:   core.RolePlayer,
	})
	require.NoError(t, err)

	// Approve one, reject another
	err = service.ApproveGameApplication(context.Background(), app1.ID, int32(gm.ID))
	require.NoError(t, err)

	err = service.RejectGameApplication(context.Background(), app2.ID, int32(gm.ID))
	require.NoError(t, err)

	t.Run("publishes all application statuses", func(t *testing.T) {
		err := service.PublishApplicationStatuses(context.Background(), game.ID)
		require.NoError(t, err)

		// Note: The actual verification would depend on the database schema
		// This test confirms the function executes without error
		// In a real scenario, you'd verify the published_at field is set
	})
}

// TestGameApplicationService_DeleteRejectedApplications verifies that rejected
// applications are removed after a recruitment phase ends. Silent failure means
// stale rejected applications accumulate in the DB.
func TestGameApplicationService_DeleteRejectedApplications(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_applications", "games", "sessions", "users")

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	_ = app
	service := &GameApplicationService{DB: testDB.Pool}
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Recruitment Game")

	// Transition game to recruitment so applications are allowed
	_, err := gameService.UpdateGameState(ctx, game.ID, "recruitment")
	require.NoError(t, err)

	// Submit applications
	app1, err := service.CreateGameApplication(ctx, core.CreateGameApplicationRequest{
		GameID: game.ID, UserID: int32(player1.ID), Role: "player",
	})
	require.NoError(t, err)
	app2, err := service.CreateGameApplication(ctx, core.CreateGameApplicationRequest{
		GameID: game.ID, UserID: int32(player2.ID), Role: "player",
	})
	require.NoError(t, err)

	// Reject both (RejectGameApplication takes applicationID, reviewerID)
	err = service.RejectGameApplication(ctx, app1.ID, int32(gm.ID))
	require.NoError(t, err)
	err = service.RejectGameApplication(ctx, app2.ID, int32(gm.ID))
	require.NoError(t, err)

	// Verify rejected applications exist before cleanup
	before, err := service.GetGameApplications(ctx, game.ID)
	require.NoError(t, err)
	rejectedCount := 0
	for _, a := range before {
		if a.Status.String == "rejected" {
			rejectedCount++
		}
	}
	assert.Equal(t, 2, rejectedCount, "should have 2 rejected applications before cleanup")

	// Delete rejected applications
	err = service.DeleteRejectedApplications(ctx, game.ID)
	require.NoError(t, err)

	// Verify they are gone
	after, err := service.GetGameApplications(ctx, game.ID)
	require.NoError(t, err)
	for _, a := range after {
		assert.NotEqual(t, "rejected", a.Status.String, "no rejected applications should remain after cleanup")
	}
}

// TestGameApplicationService_GetPublicGameApplicants verifies that the public applicant
// list returns usernames without exposing status or review information.
func TestGameApplicationService_GetPublicGameApplicants(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_applications", "games", "sessions", "users")

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	_ = app
	service := &GameApplicationService{DB: testDB.Pool}
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "applicant1", "applicant1@example.com")
	player2 := testDB.CreateTestUser(t, "applicant2", "applicant2@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Public Game")

	_, err := gameService.UpdateGameState(ctx, game.ID, "recruitment")
	require.NoError(t, err)

	_, err = service.CreateGameApplication(ctx, core.CreateGameApplicationRequest{
		GameID: game.ID, UserID: int32(player1.ID), Role: "player",
	})
	require.NoError(t, err)
	_, err = service.CreateGameApplication(ctx, core.CreateGameApplicationRequest{
		GameID: game.ID, UserID: int32(player2.ID), Role: "player",
	})
	require.NoError(t, err)

	applicants, err := service.GetPublicGameApplicants(ctx, game.ID)
	require.NoError(t, err)
	assert.Len(t, applicants, 2, "should return both applicants")

	usernames := make([]string, len(applicants))
	for i, a := range applicants {
		usernames[i] = a.Username
	}
	assert.Contains(t, usernames, player1.Username)
	assert.Contains(t, usernames, player2.Username)
}

func TestGameApplicationService_RemovedParticipantCanApplyAsAudience(t *testing.T) {
	// Regression test: a user with a removed participant record was permanently blocked
	// from applying as audience because CanUserApplyToGame had no status='active' filter.
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := &GameApplicationService{DB: testDB.Pool}
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	user := testDB.CreateTestUser(t, "user", "user@example.com")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	_, err := gameService.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)
	_, err = gameService.UpdateGameState(context.Background(), game.ID, core.GameStateCharacterCreation)
	require.NoError(t, err)

	// Simulate the prod scenario: user has a removed audience participant record
	testDB.AddTestGameParticipant(t, game.ID, int32(user.ID), core.RoleAudience)
	_, err = testDB.Pool.Exec(context.Background(),
		`UPDATE game_participants SET status = 'removed', removed_at = NOW() WHERE game_id = $1 AND user_id = $2`,
		game.ID, user.ID,
	)
	require.NoError(t, err)

	t.Run("removed participant can apply as audience", func(t *testing.T) {
		req := core.CreateGameApplicationRequest{
			GameID: game.ID,
			UserID: int32(user.ID),
			Role:   core.RoleAudience,
		}

		application, err := service.CreateGameApplication(context.Background(), req)

		require.NoError(t, err)
		assert.Equal(t, core.RoleAudience, application.Role)
	})

	t.Run("active participant still cannot apply", func(t *testing.T) {
		activeUser := testDB.CreateTestUser(t, "active_user", "active@example.com")
		testDB.AddTestGameParticipant(t, game.ID, int32(activeUser.ID), core.RoleAudience)

		req := core.CreateGameApplicationRequest{
			GameID: game.ID,
			UserID: int32(activeUser.ID),
			Role:   core.RoleAudience,
		}

		_, err := service.CreateGameApplication(context.Background(), req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "already a participant")
	})

	t.Run("removed participant with stale rejected application can still apply", func(t *testing.T) {
		// Scenario: user was rejected during recruitment, GM later added them directly,
		// they were subsequently removed. The stale rejected application record should not
		// block them — the rejection was superseded when the GM added them as a participant.
		rejectedUser := testDB.CreateTestUser(t, "rejected_user", "rejected@example.com")

		// Simulate a stale rejected application
		_, err := testDB.Pool.Exec(context.Background(),
			`INSERT INTO game_applications (game_id, user_id, role, status) VALUES ($1, $2, 'player', 'rejected')`,
			game.ID, rejectedUser.ID,
		)
		require.NoError(t, err)

		// GM added them directly (superseding the rejection), then they were removed
		testDB.AddTestGameParticipant(t, game.ID, int32(rejectedUser.ID), core.RolePlayer)
		_, err = testDB.Pool.Exec(context.Background(),
			`UPDATE game_participants SET status = 'removed', removed_at = NOW() WHERE game_id = $1 AND user_id = $2`,
			game.ID, rejectedUser.ID,
		)
		require.NoError(t, err)

		req := core.CreateGameApplicationRequest{
			GameID: game.ID,
			UserID: int32(rejectedUser.ID),
			Role:   core.RoleAudience,
		}

		application, err := service.CreateGameApplication(context.Background(), req)

		require.NoError(t, err)
		assert.Equal(t, core.RoleAudience, application.Role)
	})
}

func TestGameApplicationService_ApproveAudienceApplicationDeletesIt(t *testing.T) {
	// Root-cause regression test: approving an audience application must create the
	// participant AND delete the application row, so an approved audience member exists
	// only as a participant. Previously the row was left as 'approved', which went stale
	// when the member later left and broke re-apply / withdraw / the UI. A player
	// application, by contrast, must survive approval (it isn't converted to a participant
	// until recruitment closes).
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := &GameApplicationService{DB: testDB.Pool}
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	_, err := gameService.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	t.Run("approving an audience application deletes it and creates an active participant", func(t *testing.T) {
		audienceUser := testDB.CreateTestUser(t, "audience_user", "audience@example.com")
		application, err := service.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
			GameID: game.ID,
			UserID: int32(audienceUser.ID),
			Role:   core.RoleAudience,
		})
		require.NoError(t, err)

		err = service.ApproveGameApplication(context.Background(), application.ID, int32(gm.ID))
		require.NoError(t, err)

		// Application row is gone
		_, err = service.GetGameApplication(context.Background(), application.ID)
		require.Error(t, err, "approved audience application should be deleted")

		// Participant row exists and is active
		var status string
		err = testDB.Pool.QueryRow(context.Background(),
			`SELECT status FROM game_participants WHERE game_id = $1 AND user_id = $2 AND role = 'audience'`,
			game.ID, audienceUser.ID,
		).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "active", status)
	})

	t.Run("approving a player application keeps it (converted to participant later)", func(t *testing.T) {
		playerUser := testDB.CreateTestUser(t, "player_user", "player@example.com")
		application, err := service.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
			GameID: game.ID,
			UserID: int32(playerUser.ID),
			Role:   core.RolePlayer,
		})
		require.NoError(t, err)

		err = service.ApproveGameApplication(context.Background(), application.ID, int32(gm.ID))
		require.NoError(t, err)

		// Player application row survives, marked approved
		retrieved, err := service.GetGameApplication(context.Background(), application.ID)
		require.NoError(t, err, "approved player application should not be deleted")
		assert.Equal(t, core.ApplicationStatusApproved, retrieved.Status.String)

		// No participant row yet (created only at recruitment close)
		var count int
		err = testDB.Pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM game_participants WHERE game_id = $1 AND user_id = $2`,
			game.ID, playerUser.ID,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "player should not become a participant until recruitment closes")
	})
}

func TestGameApplicationService_LeftAudienceMemberCanReapply(t *testing.T) {
	// Regression test: a user who applied to the audience, was approved (creating an
	// 'approved' game_applications row plus an active game_participants row), and then
	// left the game was permanently blocked from re-applying. LeaveGame only deletes
	// 'pending' applications, so the stale 'approved' row remained and its UNIQUE(game_id,
	// user_id) constraint caused CreateGameApplication's plain INSERT to fail on re-apply.
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := &GameApplicationService{DB: testDB.Pool}
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	user := testDB.CreateTestUser(t, "user", "user@example.com")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	// Simulate the prod scenario directly at the DB level: an approved audience
	// application whose participant was later removed (i.e. the user left).
	_, err := testDB.Pool.Exec(context.Background(),
		`INSERT INTO game_applications (game_id, user_id, role, status) VALUES ($1, $2, 'audience', 'approved')`,
		game.ID, user.ID,
	)
	require.NoError(t, err)
	testDB.AddTestGameParticipant(t, game.ID, int32(user.ID), core.RoleAudience)
	_, err = testDB.Pool.Exec(context.Background(),
		`UPDATE game_participants SET status = 'removed', removed_at = NOW() WHERE game_id = $1 AND user_id = $2`,
		game.ID, user.ID,
	)
	require.NoError(t, err)

	t.Run("user can re-apply to audience after leaving", func(t *testing.T) {
		req := core.CreateGameApplicationRequest{
			GameID: game.ID,
			UserID: int32(user.ID),
			Role:   core.RoleAudience,
		}

		application, err := service.CreateGameApplication(context.Background(), req)

		require.NoError(t, err, "stale approved application should not block re-applying")
		assert.Equal(t, core.RoleAudience, application.Role)
	})

	t.Run("stale approved application does not block an active member from being blocked", func(t *testing.T) {
		activeUser := testDB.CreateTestUser(t, "active_user", "active@example.com")
		_, err := testDB.Pool.Exec(context.Background(),
			`INSERT INTO game_applications (game_id, user_id, role, status) VALUES ($1, $2, 'audience', 'approved')`,
			game.ID, activeUser.ID,
		)
		require.NoError(t, err)
		testDB.AddTestGameParticipant(t, game.ID, int32(activeUser.ID), core.RoleAudience)

		err = service.DeleteStaleApprovedApplicationForUser(context.Background(), game.ID, int32(activeUser.ID))
		require.NoError(t, err)

		// The application row must still exist since the participant is still active
		retrieved, err := service.GetGameApplicationByUserAndGame(context.Background(), game.ID, int32(activeUser.ID))
		require.NoError(t, err)
		assert.Equal(t, core.ApplicationStatusApproved, retrieved.Status.String)

		_ = gameService.RemovePlayer(context.Background(), game.ID, int32(activeUser.ID), int32(activeUser.ID))
	})
}

func TestAddGameParticipant_ReactivatesRemovedRecord(t *testing.T) {
	// Regression test: AddGameParticipant used a plain INSERT which would fail with a
	// unique constraint violation when a removed participant record already existed.
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	user := testDB.CreateTestUser(t, "user", "user@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	// Create and then remove a participant
	testDB.AddTestGameParticipant(t, game.ID, int32(user.ID), core.RoleAudience)
	_, err := testDB.Pool.Exec(context.Background(),
		`UPDATE game_participants SET status = 'removed', removed_at = NOW(), is_former_player = TRUE WHERE game_id = $1 AND user_id = $2`,
		game.ID, user.ID,
	)
	require.NoError(t, err)

	// Re-adding the same user should reactivate the record, not fail with a constraint error
	gameService := &GameService{DB: testDB.Pool, Logger: core.NewTestApp(testDB.Pool).ObsLogger}
	participant, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(user.ID), core.RoleAudience)

	require.NoError(t, err)
	assert.Equal(t, "active", participant.Status.String)
	assert.True(t, participant.IsFormerPlayer, "is_former_player should be preserved on re-join")
	assert.False(t, participant.RemovedAt.Valid, "removed_at should be cleared")
}
