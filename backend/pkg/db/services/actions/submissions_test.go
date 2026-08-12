package actions

import (
	"context"
	"fmt"
	"testing"
	"time"

	core "actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	db "actionphase/pkg/db/services"
	phases "actionphase/pkg/db/services/phases"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionSubmissionService_SubmitAction(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test data
	user := testDB.CreateTestUser(t, "testuser", "test@example.com")
	game := testDB.CreateTestGame(t, int32(user.ID), "Test Game")

	// Create game participant
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(user.ID), "player")
	require.NoError(t, err)

	// Create and activate an action phase
	transitionReq := core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
		Deadline:  core.TimePtr(time.Now().Add(72 * time.Hour)),
	}
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(user.ID), transitionReq)
	require.NoError(t, err)

	t.Run("submits action successfully", func(t *testing.T) {
		user1 := testDB.CreateTestUser(t, "testuser1", "test1@example.com")
		_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(user1.ID), "player")
		require.NoError(t, err)

		req := core.SubmitActionRequest{
			GameID:  game.ID,
			PhaseID: phase.ID,
			UserID:  int32(user1.ID),
			Content: "I search the room for clues.",
		}

		action, err := actionService.SubmitAction(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, req.GameID, action.GameID)
		assert.Equal(t, req.PhaseID, action.PhaseID)
		assert.Equal(t, req.UserID, action.UserID)
		assert.Equal(t, "I search the room for clues.", action.Content)
		assert.True(t, action.SubmittedAt.Valid, "submission should be stamped on insert")
	})

	t.Run("returns error when user is not a participant", func(t *testing.T) {
		outsider := testDB.CreateTestUser(t, "outsider", "outsider@example.com")

		req := core.SubmitActionRequest{
			GameID:  game.ID,
			PhaseID: phase.ID,
			UserID:  int32(outsider.ID),
			Content: "Trying to submit without permission",
		}

		_, err := actionService.SubmitAction(context.Background(), req)
		require.Error(t, err) // Verify error occurs (permission denied)
	})

	t.Run("blocks action submission in completed game", func(t *testing.T) {
		// Create a completed game with a phase
		completedGame := testDB.CreateTestGameWithState(t, int32(user.ID), "Completed Game", core.GameStateCompleted)

		// Create participant (using test helper to bypass validation)
		testDB.AddTestGameParticipant(t, completedGame.ID, int32(user.ID), "player")

		// Create a phase (this would have been created before completion)
		// Need to manually create phase bypassing validation for test setup
		testPhase := testDB.CreateTestPhase(t, completedGame.ID, "action", "Old Phase")

		req := core.SubmitActionRequest{
			GameID:  completedGame.ID,
			PhaseID: testPhase.ID,
			UserID:  int32(user.ID),
			Content: "Should fail",
		}

		_, err = actionService.SubmitAction(context.Background(), req)
		require.Error(t, err, "Expected error when submitting action to completed game")
		assert.Contains(t, err.Error(), "archived", "Error should mention game is archived")
	})

	t.Run("blocks action submission in cancelled game", func(t *testing.T) {
		// Create a cancelled game with a phase
		cancelledGame := testDB.CreateTestGameWithState(t, int32(user.ID), "Cancelled Game", core.GameStateCancelled)

		// Create participant (using test helper to bypass validation)
		testDB.AddTestGameParticipant(t, cancelledGame.ID, int32(user.ID), "player")

		// Create a phase
		testPhase := testDB.CreateTestPhase(t, cancelledGame.ID, "action", "Old Phase")

		req := core.SubmitActionRequest{
			GameID:  cancelledGame.ID,
			PhaseID: testPhase.ID,
			UserID:  int32(user.ID),
			Content: "Should fail",
		}

		_, err = actionService.SubmitAction(context.Background(), req)
		require.Error(t, err, "Expected error when submitting action to cancelled game")
		assert.Contains(t, err.Error(), "archived", "Error should mention game is archived")
	})

	t.Run("validates character ownership", func(t *testing.T) {
		// Create character service to create test characters
		queries := models.New(testDB.Pool)

		// Create two players
		player1 := testDB.CreateTestUser(t, "char_player1", "char_player1@example.com")
		player2 := testDB.CreateTestUser(t, "char_player2", "char_player2@example.com")

		_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
		require.NoError(t, err)
		_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
		require.NoError(t, err)

		// Create character owned by player1
		char1, err := queries.CreateCharacter(context.Background(), models.CreateCharacterParams{
			GameID:        game.ID,
			UserID:        pgtype.Int4{Int32: int32(player1.ID), Valid: true},
			Name:          "Player 1's Character",
			CharacterType: "player_character",
			Status:        pgtype.Text{String: "approved", Valid: true},
		})
		require.NoError(t, err)

		// Try to submit action with player2 using player1's character
		req := core.SubmitActionRequest{
			GameID:      game.ID,
			PhaseID:     phase.ID,
			UserID:      int32(player2.ID),
			CharacterID: &char1.ID,
			Content:     "Action with wrong character",
		}

		_, err = actionService.SubmitAction(context.Background(), req)
		require.Error(t, err, "Should error when using another user's character")
		assert.Contains(t, err.Error(), "you can only submit actions for characters you own")
	})

	t.Run("validates character belongs to game", func(t *testing.T) {
		queries := models.New(testDB.Pool)

		// Create another game
		game2 := testDB.CreateTestGame(t, int32(user.ID), "Another Game")
		player := testDB.CreateTestUser(t, "cross_game_player", "cross_game_player@example.com")

		_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
		require.NoError(t, err)

		// Create character in game2
		charInGame2, err := queries.CreateCharacter(context.Background(), models.CreateCharacterParams{
			GameID:        game2.ID,
			UserID:        pgtype.Int4{Int32: int32(player.ID), Valid: true},
			Name:          "Character in Game 2",
			CharacterType: "player_character",
			Status:        pgtype.Text{String: "approved", Valid: true},
		})
		require.NoError(t, err)

		// Try to submit action in game1 with character from game2
		req := core.SubmitActionRequest{
			GameID:      game.ID,
			PhaseID:     phase.ID,
			UserID:      int32(player.ID),
			CharacterID: &charInGame2.ID,
			Content:     "Cross-game action",
		}

		_, err = actionService.SubmitAction(context.Background(), req)
		require.Error(t, err, "Should error when using character from different game")
		assert.Contains(t, err.Error(), "character does not belong to this game")
	})

	t.Run("returns error for non-existent character", func(t *testing.T) {
		player := testDB.CreateTestUser(t, "nonexist_char_player", "nonexist_char_player@example.com")
		_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
		require.NoError(t, err)

		nonExistentCharID := int32(999999)
		req := core.SubmitActionRequest{
			GameID:      game.ID,
			PhaseID:     phase.ID,
			UserID:      int32(player.ID),
			CharacterID: &nonExistentCharID,
			Content:     "Action with non-existent character",
		}

		_, err = actionService.SubmitAction(context.Background(), req)
		require.Error(t, err, "Should error when character doesn't exist")
		assert.Contains(t, err.Error(), "character not found")
	})
}

func TestActionSubmissionService_SubmitAction_PastDeadlineBlocked(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "deadline_gm", "deadline_gm@example.com")
	player := testDB.CreateTestUser(t, "deadline_player", "deadline_player@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Deadline Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	// Create action phase with a future deadline
	futureDeadline := time.Now().Add(72 * time.Hour)
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Deadline Phase",
		Deadline:  &futureDeadline,
	})
	require.NoError(t, err)

	t.Run("submission succeeds before deadline", func(t *testing.T) {
		_, err := actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
			GameID:  game.ID,
			PhaseID: phase.ID,
			UserID:  int32(player.ID),
			Content: "Before deadline action",
		})
		require.NoError(t, err)
	})

	// Move deadline to the past
	pastDeadline := time.Now().Add(-1 * time.Hour)
	_, err = phaseService.ExtendPhaseDeadline(context.Background(), phase.ID, pastDeadline)
	require.NoError(t, err)

	t.Run("submission is blocked after deadline passes", func(t *testing.T) {
		latePlayer := testDB.CreateTestUser(t, "late_player", "late_player@example.com")
		_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(latePlayer.ID), "player")
		require.NoError(t, err)

		_, err = actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
			GameID:  game.ID,
			PhaseID: phase.ID,
			UserID:  int32(latePlayer.ID),
			Content: "Late submission attempt",
		})
		require.Error(t, err, "Should block submission after deadline")
	})
}

func TestActionSubmissionService_GetActionSubmission(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test data
	user := testDB.CreateTestUser(t, "testuser", "test@example.com")
	game := testDB.CreateTestGame(t, int32(user.ID), "Test Game")
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(user.ID), "player")
	require.NoError(t, err)

	// Create action phase
	transitionReq := core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	}
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(user.ID), transitionReq)
	require.NoError(t, err)

	// Submit an action
	submitReq := core.SubmitActionRequest{
		GameID:  game.ID,
		PhaseID: phase.ID,
		UserID:  int32(user.ID),
		Content: "Test action",
	}
	submission, err := actionService.SubmitAction(context.Background(), submitReq)
	require.NoError(t, err)

	t.Run("retrieves submission by ID", func(t *testing.T) {
		retrieved, err := actionService.GetActionSubmission(context.Background(), submission.ID)
		require.NoError(t, err)
		assert.Equal(t, submission.ID, retrieved.ID)
		assert.Equal(t, submission.Content, retrieved.Content)
	})

	t.Run("returns error for non-existent submission", func(t *testing.T) {
		_, err := actionService.GetActionSubmission(context.Background(), 99999)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestActionSubmissionService_DeleteActionSubmission(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test data
	user := testDB.CreateTestUser(t, "testuser", "test@example.com")
	game := testDB.CreateTestGame(t, int32(user.ID), "Test Game")
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(user.ID), "player")
	require.NoError(t, err)

	// Create action phase
	transitionReq := core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	}
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(user.ID), transitionReq)
	require.NoError(t, err)

	t.Run("deletes submission successfully", func(t *testing.T) {
		submitReq := core.SubmitActionRequest{
			GameID:  game.ID,
			PhaseID: phase.ID,
			UserID:  int32(user.ID),
			Content: "Action to delete",
		}
		submission, err := actionService.SubmitAction(context.Background(), submitReq)
		require.NoError(t, err)

		err = actionService.DeleteActionSubmission(context.Background(), submission.ID, int32(user.ID))
		require.NoError(t, err)

		// Verify it's gone
		_, err = actionService.GetActionSubmission(context.Background(), submission.ID)
		require.Error(t, err)
	})

	t.Run("wrong user cannot delete another user's submission", func(t *testing.T) {
		otherUser := testDB.CreateTestUser(t, "otheruser", "other@example.com")

		submitReq := core.SubmitActionRequest{
			GameID:  game.ID,
			PhaseID: phase.ID,
			UserID:  int32(user.ID),
			Content: "Submission owned by user",
		}
		submission, err := actionService.SubmitAction(context.Background(), submitReq)
		require.NoError(t, err)

		// Other user attempts to delete - must fail
		err = actionService.DeleteActionSubmission(context.Background(), submission.ID, int32(otherUser.ID))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found or not owned")

		// Submission still exists
		_, err = actionService.GetActionSubmission(context.Background(), submission.ID)
		require.NoError(t, err)
	})

	t.Run("returns error for non-existent submission id", func(t *testing.T) {
		err := actionService.DeleteActionSubmission(context.Background(), 99999, int32(user.ID))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found or not owned")
	})

	t.Run("owner can delete finalized submission (no service-layer restriction)", func(t *testing.T) {
		submitReq := core.SubmitActionRequest{
			GameID:  game.ID,
			PhaseID: phase.ID,
			UserID:  int32(user.ID),
			Content: "Finalized submission",
		}
		submission, err := actionService.SubmitAction(context.Background(), submitReq)
		require.NoError(t, err)

		err = actionService.DeleteActionSubmission(context.Background(), submission.ID, int32(user.ID))
		require.NoError(t, err)
	})
}

func TestActionSubmissionService_GetSubmissionStats(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test data
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	// Create action phase
	transitionReq := core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	}
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), transitionReq)
	require.NoError(t, err)

	t.Run("calculates submission stats correctly", func(t *testing.T) {
		// Add players and submissions
		player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
		player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")

		_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
		require.NoError(t, err)
		_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
		require.NoError(t, err)

		// Both players submit. There is no draft state, so every submission
		// counts toward SubmittedCount.
		_, err = actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
			GameID:  game.ID,
			PhaseID: phase.ID,
			UserID:  int32(player1.ID),
			Content: "Player one's action",
		})
		require.NoError(t, err)

		_, err = actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
			GameID:  game.ID,
			PhaseID: phase.ID,
			UserID:  int32(player2.ID),
			Content: "Player two's action",
		})
		require.NoError(t, err)

		// Get stats
		stats, err := actionService.GetSubmissionStats(context.Background(), phase.ID)
		require.NoError(t, err)
		assert.Equal(t, int32(2), stats.SubmittedCount)
		assert.Equal(t, int32(2), stats.TotalPlayers)
	})
}

func TestActionSubmissionService_GetUserPhaseSubmission(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test data
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	// Create action phase
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	})
	require.NoError(t, err)

	t.Run("returns submission when user has submitted", func(t *testing.T) {
		// Submit action
		submitted, err := actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
			GameID:  game.ID,
			PhaseID: phase.ID,
			UserID:  int32(player.ID),
			Content: "My action for this phase",
		})
		require.NoError(t, err)

		// Get user's phase submission
		result, err := actionService.GetUserPhaseSubmission(context.Background(), phase.ID, int32(player.ID))
		require.NoError(t, err)
		require.NotNil(t, result, "Should return submission")
		assert.Equal(t, submitted.ID, result.ID)
		assert.Equal(t, "My action for this phase", result.Content)
	})

	t.Run("returns nil when user has not submitted", func(t *testing.T) {
		newPlayer := testDB.CreateTestUser(t, "newplayer", "newplayer@example.com")
		_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(newPlayer.ID), "player")
		require.NoError(t, err)

		// Get submission for user who hasn't submitted
		result, err := actionService.GetUserPhaseSubmission(context.Background(), phase.ID, int32(newPlayer.ID))
		require.NoError(t, err)
		assert.Nil(t, result, "Should return nil when no submission exists")
	})

	t.Run("editing a submission preserves the original submitted_at", func(t *testing.T) {
		editPlayer := testDB.CreateTestUser(t, "editplayer", "editplayer@example.com")
		_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(editPlayer.ID), "player")
		require.NoError(t, err)

		first, err := actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
			GameID:  game.ID,
			PhaseID: phase.ID,
			UserID:  int32(editPlayer.ID),
			Content: "First draft of my action",
		})
		require.NoError(t, err)
		require.True(t, first.SubmittedAt.Valid)

		// Players edit freely until the deadline; an edit must not restamp
		// submitted_at, which marks when they first submitted.
		_, err = actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
			GameID:  game.ID,
			PhaseID: phase.ID,
			UserID:  int32(editPlayer.ID),
			Content: "Revised action",
		})
		require.NoError(t, err)

		result, err := actionService.GetUserPhaseSubmission(context.Background(), phase.ID, int32(editPlayer.ID))
		require.NoError(t, err)
		require.NotNil(t, result, "Should return the edited submission")
		assert.Equal(t, "Revised action", result.Content)
		assert.Equal(t, first.SubmittedAt.Time, result.SubmittedAt.Time,
			"editing must not restamp submitted_at")
	})
}

func TestActionSubmissionService_GetPhaseSubmissions(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test data
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	// Create action phase
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	})
	require.NoError(t, err)

	t.Run("returns all submissions for a phase", func(t *testing.T) {
		// Create 3 players and have them submit
		for i := 1; i <= 3; i++ {
			player := testDB.CreateTestUser(t, fmt.Sprintf("player%d", i), fmt.Sprintf("player%d@example.com", i))
			_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
			require.NoError(t, err)

			_, err = actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
				GameID:  game.ID,
				PhaseID: phase.ID,
				UserID:  int32(player.ID),
				Content: fmt.Sprintf("Action from player %d", i),
			})
			require.NoError(t, err)
		}

		// Get all phase submissions
		submissions, err := actionService.GetPhaseSubmissions(context.Background(), phase.ID)
		require.NoError(t, err)
		assert.Len(t, submissions, 3, "Should return all 3 submissions")
	})


	t.Run("returns empty list when no submissions exist", func(t *testing.T) {
		emptyGame := testDB.CreateTestGame(t, int32(gm.ID), "Empty Game")
		emptyPhase, err := phaseService.TransitionToNextPhase(context.Background(), emptyGame.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Empty Phase",
		})
		require.NoError(t, err)

		// Get submissions for phase with no submissions
		submissions, err := actionService.GetPhaseSubmissions(context.Background(), emptyPhase.ID)
		require.NoError(t, err)
		assert.Empty(t, submissions, "Should return empty list when no submissions exist")
	})
}
