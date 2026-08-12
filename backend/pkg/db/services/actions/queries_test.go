package actions

import (
	"context"
	"fmt"
	"testing"
	"time"

	core "actionphase/pkg/core"
	db "actionphase/pkg/db/services"
	phases "actionphase/pkg/db/services/phases"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionSubmissionService_GetUserActions(t *testing.T) {
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

	t.Run("returns user's actions for a game", func(t *testing.T) {
		// Create phase 1 and submit
		phase1, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Phase 1",
			Deadline:  core.TimePtr(time.Now().Add(72 * time.Hour)),
		})
		require.NoError(t, err)

		// Submit action to phase 1 while it's active
		_, err = actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
			GameID:  game.ID,
			PhaseID: phase1.ID,
			UserID:  int32(player.ID),
			Content: "Action in phase 1",
			IsDraft: false,
		})
		require.NoError(t, err)

		// Create phase 2 and submit
		phase2, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Phase 2",
			Deadline:  core.TimePtr(time.Now().Add(72 * time.Hour)),
		})
		require.NoError(t, err)

		// Submit action to phase 2 while it's active
		_, err = actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
			GameID:  game.ID,
			PhaseID: phase2.ID,
			UserID:  int32(player.ID),
			Content: "Action in phase 2",
			IsDraft: false,
		})
		require.NoError(t, err)

		// Get user's actions - should return both
		actions, err := actionService.GetUserActions(context.Background(), game.ID, int32(player.ID))
		require.NoError(t, err)
		assert.Len(t, actions, 2, "Should return 2 actions")
	})

	t.Run("returns empty list when user has no actions", func(t *testing.T) {
		// Create new player who hasn't submitted
		newPlayer := testDB.CreateTestUser(t, "newplayer", "newplayer@example.com")
		_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(newPlayer.ID), "player")
		require.NoError(t, err)

		// Get actions for player with no submissions
		actions, err := actionService.GetUserActions(context.Background(), game.ID, int32(newPlayer.ID))
		require.NoError(t, err)
		assert.Empty(t, actions, "Should return empty slice when user has no actions")
	})

	t.Run("filters by game ID correctly", func(t *testing.T) {
		// Create a second game
		player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
		game2 := testDB.CreateTestGame(t, int32(gm.ID), "Second Game")
		_, err := gameService.AddGameParticipant(context.Background(), game2.ID, int32(player2.ID), "player")
		require.NoError(t, err)

		// Create phase in second game
		phase2Game, err := phaseService.TransitionToNextPhase(context.Background(), game2.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Phase in Game 2",
		})
		require.NoError(t, err)

		// Submit actions to both games
		_, err = actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
			GameID:  game2.ID,
			PhaseID: phase2Game.ID,
			UserID:  int32(player2.ID),
			Content: "Action in game 2",
			IsDraft: false,
		})
		require.NoError(t, err)

		// Get actions should only return from game2
		actions, err := actionService.GetUserActions(context.Background(), game2.ID, int32(player2.ID))
		require.NoError(t, err)
		assert.Len(t, actions, 1, "Should only return actions from specified game")
	})

	t.Run("includes draft and final submissions", func(t *testing.T) {
		player3 := testDB.CreateTestUser(t, "player3", "player3@example.com")
		game3 := testDB.CreateTestGame(t, int32(gm.ID), "Third Game")
		_, err := gameService.AddGameParticipant(context.Background(), game3.ID, int32(player3.ID), "player")
		require.NoError(t, err)

		phase, err := phaseService.TransitionToNextPhase(context.Background(), game3.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Test Phase",
		})
		require.NoError(t, err)

		// Submit one draft
		_, err = actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
			GameID:  game3.ID,
			PhaseID: phase.ID,
			UserID:  int32(player3.ID),
			Content: "Draft action",
			IsDraft: true,
		})
		require.NoError(t, err)

		// Get actions should include the draft
		actions, err := actionService.GetUserActions(context.Background(), game3.ID, int32(player3.ID))
		require.NoError(t, err)
		assert.Len(t, actions, 1, "Should include draft submissions")
	})
}

func TestActionSubmissionService_GetGameActions(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test data
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	})
	require.NoError(t, err)

	t.Run("returns all actions for a game", func(t *testing.T) {
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
				IsDraft: false,
			})
			require.NoError(t, err)
		}

		// Get all game actions
		actions, err := actionService.GetGameActions(context.Background(), game.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(actions), 3, "Should return at least 3 actions")
	})

	t.Run("returns actions across multiple phases", func(t *testing.T) {
		game2 := testDB.CreateTestGame(t, int32(gm.ID), "Multi-Phase Game")
		player := testDB.CreateTestUser(t, "multiphase", "multiphase@example.com")
		_, err := gameService.AddGameParticipant(context.Background(), game2.ID, int32(player.ID), "player")
		require.NoError(t, err)

		// Create phase 1 and submit while active
		phase1, err := phaseService.TransitionToNextPhase(context.Background(), game2.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Phase 1",
		})
		require.NoError(t, err)

		_, err = actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
			GameID:  game2.ID,
			PhaseID: phase1.ID,
			UserID:  int32(player.ID),
			Content: "Action in phase 1",
			IsDraft: false,
		})
		require.NoError(t, err)

		// Create phase 2 and submit while active
		phase2, err := phaseService.TransitionToNextPhase(context.Background(), game2.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Phase 2",
		})
		require.NoError(t, err)

		_, err = actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
			GameID:  game2.ID,
			PhaseID: phase2.ID,
			UserID:  int32(player.ID),
			Content: "Action in phase 2",
			IsDraft: false,
		})
		require.NoError(t, err)

		// Should get actions from both phases
		actions, err := actionService.GetGameActions(context.Background(), game2.ID)
		require.NoError(t, err)
		assert.Len(t, actions, 2, "Should return actions from all phases")
	})

	t.Run("returns empty list for game with no submissions", func(t *testing.T) {
		emptyGame := testDB.CreateTestGame(t, int32(gm.ID), "Empty Game")

		actions, err := actionService.GetGameActions(context.Background(), emptyGame.ID)
		require.NoError(t, err)
		assert.Empty(t, actions, "Should return empty slice for game with no submissions")
	})

	t.Run("includes metadata (usernames, character names)", func(t *testing.T) {
		game3 := testDB.CreateTestGame(t, int32(gm.ID), "Metadata Test Game")
		player := testDB.CreateTestUser(t, "metaplayer", "metaplayer@example.com")
		_, err := gameService.AddGameParticipant(context.Background(), game3.ID, int32(player.ID), "player")
		require.NoError(t, err)

		phase, err := phaseService.TransitionToNextPhase(context.Background(), game3.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Metadata Phase",
		})
		require.NoError(t, err)

		_, err = actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
			GameID:  game3.ID,
			PhaseID: phase.ID,
			UserID:  int32(player.ID),
			Content: "Test action",
			IsDraft: false,
		})
		require.NoError(t, err)

		actions, err := actionService.GetGameActions(context.Background(), game3.ID)
		require.NoError(t, err)
		require.NotEmpty(t, actions, "Should have at least one action")

		// Verify metadata fields are populated (username should be available)
		assert.NotEmpty(t, actions[0].Username, "Username field should be populated")
		assert.Equal(t, player.Username, actions[0].Username, "Username should match the submitting player")
	})
}

func TestActionSubmissionService_GetUserResults(t *testing.T) {
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

	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	})
	require.NoError(t, err)

	t.Run("returns published results for user", func(t *testing.T) {
		// Create and publish 2 results
		for i := 1; i <= 2; i++ {
			result, err := actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
				GameID:      game.ID,
				PhaseID:     phase.ID,
				UserID:      int32(player.ID),
				Content:     fmt.Sprintf("Result %d", i),
				IsPublished: false,
			})
			require.NoError(t, err)

			// Publish it
			err = actionService.PublishActionResult(context.Background(), result.ID, int32(gm.ID))
			require.NoError(t, err)
		}

		// Get user results
		results, err := actionService.GetUserResults(context.Background(), game.ID, int32(player.ID))
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(results), 2, "Should return at least 2 published results")
	})

	t.Run("excludes unpublished results", func(t *testing.T) {
		player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
		game2 := testDB.CreateTestGame(t, int32(gm.ID), "Test Game 2")
		_, err := gameService.AddGameParticipant(context.Background(), game2.ID, int32(player2.ID), "player")
		require.NoError(t, err)

		phase2, err := phaseService.TransitionToNextPhase(context.Background(), game2.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Action Phase",
		})
		require.NoError(t, err)

		// Create 2 results, publish only 1
		result1, err := actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
			GameID:      game2.ID,
			PhaseID:     phase2.ID,
			UserID:      int32(player2.ID),
			Content:     "Published result",
			IsPublished: false,
		})
		require.NoError(t, err)
		err = actionService.PublishActionResult(context.Background(), result1.ID, int32(gm.ID))
		require.NoError(t, err)

		// Create unpublished result
		_, err = actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
			GameID:      game2.ID,
			PhaseID:     phase2.ID,
			UserID:      int32(player2.ID),
			Content:     "Unpublished result",
			IsPublished: false,
		})
		require.NoError(t, err)

		// Should only get published result
		results, err := actionService.GetUserResults(context.Background(), game2.ID, int32(player2.ID))
		require.NoError(t, err)
		assert.Equal(t, 1, len(results), "Should only return published results")
	})

	t.Run("returns empty list when no results exist", func(t *testing.T) {
		player3 := testDB.CreateTestUser(t, "player3", "player3@example.com")
		game3 := testDB.CreateTestGame(t, int32(gm.ID), "Test Game 3")
		_, err := gameService.AddGameParticipant(context.Background(), game3.ID, int32(player3.ID), "player")
		require.NoError(t, err)

		results, err := actionService.GetUserResults(context.Background(), game3.ID, int32(player3.ID))
		require.NoError(t, err)
		assert.Empty(t, results, "Should return empty slice when no results exist")
	})

	t.Run("filters by game ID", func(t *testing.T) {
		player4 := testDB.CreateTestUser(t, "player4", "player4@example.com")
		game4a := testDB.CreateTestGame(t, int32(gm.ID), "Game 4a")
		game4b := testDB.CreateTestGame(t, int32(gm.ID), "Game 4b")

		_, err := gameService.AddGameParticipant(context.Background(), game4a.ID, int32(player4.ID), "player")
		require.NoError(t, err)
		_, err = gameService.AddGameParticipant(context.Background(), game4b.ID, int32(player4.ID), "player")
		require.NoError(t, err)

		// Create phases in both games
		phase4a, err := phaseService.TransitionToNextPhase(context.Background(), game4a.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Phase 4a",
		})
		require.NoError(t, err)

		phase4b, err := phaseService.TransitionToNextPhase(context.Background(), game4b.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Phase 4b",
		})
		require.NoError(t, err)

		// Create and publish results in both games
		result4a, err := actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
			GameID:      game4a.ID,
			PhaseID:     phase4a.ID,
			UserID:      int32(player4.ID),
			Content:     "Result in game 4a",
			IsPublished: false,
		})
		require.NoError(t, err)
		err = actionService.PublishActionResult(context.Background(), result4a.ID, int32(gm.ID))
		require.NoError(t, err)

		result4b, err := actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
			GameID:      game4b.ID,
			PhaseID:     phase4b.ID,
			UserID:      int32(player4.ID),
			Content:     "Result in game 4b",
			IsPublished: false,
		})
		require.NoError(t, err)
		err = actionService.PublishActionResult(context.Background(), result4b.ID, int32(gm.ID))
		require.NoError(t, err)

		// Get results for game4a only
		results, err := actionService.GetUserResults(context.Background(), game4a.ID, int32(player4.ID))
		require.NoError(t, err)
		assert.Equal(t, 1, len(results), "Should only return results from specified game")
	})
}

func TestActionSubmissionService_GetGameResults(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test data
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	})
	require.NoError(t, err)

	t.Run("returns all results for a game", func(t *testing.T) {
		// Create 3 players and results for each
		for i := 1; i <= 3; i++ {
			player := testDB.CreateTestUser(t, fmt.Sprintf("player%d", i), fmt.Sprintf("player%d@example.com", i))
			_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
			require.NoError(t, err)

			_, err = actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
				GameID:      game.ID,
				PhaseID:     phase.ID,
				UserID:      int32(player.ID),
				Content:     fmt.Sprintf("Result for player %d", i),
				IsPublished: false,
			})
			require.NoError(t, err)
		}

		// Get all results
		results, err := actionService.GetGameResults(context.Background(), game.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(results), 3, "Should return at least 3 results")
	})

	t.Run("includes both published and unpublished results", func(t *testing.T) {
		game2 := testDB.CreateTestGame(t, int32(gm.ID), "Mixed Results Game")
		player := testDB.CreateTestUser(t, "mixedplayer", "mixedplayer@example.com")
		_, err := gameService.AddGameParticipant(context.Background(), game2.ID, int32(player.ID), "player")
		require.NoError(t, err)

		phase2, err := phaseService.TransitionToNextPhase(context.Background(), game2.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Mixed Phase",
		})
		require.NoError(t, err)

		// Create 2 published, 2 unpublished
		for i := 1; i <= 4; i++ {
			result, err := actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
				GameID:      game2.ID,
				PhaseID:     phase2.ID,
				UserID:      int32(player.ID),
				Content:     fmt.Sprintf("Result %d", i),
				IsPublished: false,
			})
			require.NoError(t, err)

			// Publish first 2
			if i <= 2 {
				err = actionService.PublishActionResult(context.Background(), result.ID, int32(gm.ID))
				require.NoError(t, err)
			}
		}

		// Should get all 4
		results, err := actionService.GetGameResults(context.Background(), game2.ID)
		require.NoError(t, err)
		assert.Equal(t, 4, len(results), "Should return both published and unpublished results")
	})

	t.Run("returns empty list for game with no results", func(t *testing.T) {
		emptyGame := testDB.CreateTestGame(t, int32(gm.ID), "Empty Results Game")

		results, err := actionService.GetGameResults(context.Background(), emptyGame.ID)
		require.NoError(t, err)
		assert.Empty(t, results, "Should return empty slice for game with no results")
	})

	t.Run("returns results across multiple phases", func(t *testing.T) {
		game3 := testDB.CreateTestGame(t, int32(gm.ID), "Multi-Phase Results")
		player := testDB.CreateTestUser(t, "multiphaseplayer", "multiphaseplayer@example.com")
		_, err := gameService.AddGameParticipant(context.Background(), game3.ID, int32(player.ID), "player")
		require.NoError(t, err)

		// Create 2 phases
		phase1, err := phaseService.TransitionToNextPhase(context.Background(), game3.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Phase 1",
		})
		require.NoError(t, err)

		phase2, err := phaseService.TransitionToNextPhase(context.Background(), game3.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Phase 2",
		})
		require.NoError(t, err)

		// Create results in both phases
		_, err = actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
			GameID:      game3.ID,
			PhaseID:     phase1.ID,
			UserID:      int32(player.ID),
			Content:     "Result in phase 1",
			IsPublished: false,
		})
		require.NoError(t, err)

		_, err = actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
			GameID:      game3.ID,
			PhaseID:     phase2.ID,
			UserID:      int32(player.ID),
			Content:     "Result in phase 2",
			IsPublished: false,
		})
		require.NoError(t, err)

		// Should get results from both phases
		results, err := actionService.GetGameResults(context.Background(), game3.ID)
		require.NoError(t, err)
		assert.Equal(t, 2, len(results), "Should return results from all phases")
	})
}

func TestActionSubmissionService_ListAllActionSubmissions(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test data
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Pagination Test Game")

	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	})
	require.NoError(t, err)

	// Create 15 submissions
	for i := 1; i <= 15; i++ {
		player := testDB.CreateTestUser(t, fmt.Sprintf("pagplayer%d", i), fmt.Sprintf("pagplayer%d@example.com", i))
		_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
		require.NoError(t, err)

		_, err = actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
			GameID:  game.ID,
			PhaseID: phase.ID,
			UserID:  int32(player.ID),
			Content: fmt.Sprintf("Action %d", i),
			IsDraft: false,
		})
		require.NoError(t, err)
	}

	t.Run("returns paginated submissions", func(t *testing.T) {
		submissions, err := actionService.ListAllActionSubmissions(context.Background(), game.ID, 0, 10, 0)
		require.NoError(t, err)
		assert.Len(t, submissions, 10, "Should return first 10 submissions")
	})

	t.Run("pagination offset works correctly", func(t *testing.T) {
		submissions, err := actionService.ListAllActionSubmissions(context.Background(), game.ID, 0, 10, 10)
		require.NoError(t, err)
		assert.Len(t, submissions, 5, "Should return last 5 submissions")
	})

	t.Run("filters by phase when phaseID provided", func(t *testing.T) {
		game2 := testDB.CreateTestGame(t, int32(gm.ID), "Multi-Phase Filter Game")
		player := testDB.CreateTestUser(t, "filterplayer", "filterplayer@example.com")
		_, err := gameService.AddGameParticipant(context.Background(), game2.ID, int32(player.ID), "player")
		require.NoError(t, err)

		// Create phase 1 and submit while active
		phase1, err := phaseService.TransitionToNextPhase(context.Background(), game2.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Filter Phase 1",
		})
		require.NoError(t, err)

		// Create 3 submissions in phase1 while it's active
		for i := 1; i <= 3; i++ {
			p := testDB.CreateTestUser(t, fmt.Sprintf("fp1_%d", i), fmt.Sprintf("fp1_%d@example.com", i))
			_, err := gameService.AddGameParticipant(context.Background(), game2.ID, int32(p.ID), "player")
			require.NoError(t, err)

			_, err = actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
				GameID:  game2.ID,
				PhaseID: phase1.ID,
				UserID:  int32(p.ID),
				Content: fmt.Sprintf("Phase 1 Action %d", i),
				IsDraft: false,
			})
			require.NoError(t, err)
		}

		// Create phase 2 and submit while active
		phase2, err := phaseService.TransitionToNextPhase(context.Background(), game2.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Filter Phase 2",
		})
		require.NoError(t, err)

		for i := 1; i <= 2; i++ {
			p := testDB.CreateTestUser(t, fmt.Sprintf("fp2_%d", i), fmt.Sprintf("fp2_%d@example.com", i))
			_, err := gameService.AddGameParticipant(context.Background(), game2.ID, int32(p.ID), "player")
			require.NoError(t, err)

			_, err = actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
				GameID:  game2.ID,
				PhaseID: phase2.ID,
				UserID:  int32(p.ID),
				Content: fmt.Sprintf("Phase 2 Action %d", i),
				IsDraft: false,
			})
			require.NoError(t, err)
		}

		// Filter by phase1
		submissions, err := actionService.ListAllActionSubmissions(context.Background(), game2.ID, phase1.ID, 100, 0)
		require.NoError(t, err)
		assert.Equal(t, 3, len(submissions), "Should only return submissions from phase1")
	})

	t.Run("returns all phases when phaseID=0", func(t *testing.T) {
		game3 := testDB.CreateTestGame(t, int32(gm.ID), "All Phases Game")
		player := testDB.CreateTestUser(t, "allphasesplayer", "allphasesplayer@example.com")
		_, err := gameService.AddGameParticipant(context.Background(), game3.ID, int32(player.ID), "player")
		require.NoError(t, err)

		// Create 2 phases with submissions
		for phaseNum := 1; phaseNum <= 2; phaseNum++ {
			ph, err := phaseService.TransitionToNextPhase(context.Background(), game3.ID, int32(gm.ID), core.TransitionPhaseRequest{
				PhaseType: "action",
				Title:     fmt.Sprintf("All Phases %d", phaseNum),
			})
			require.NoError(t, err)

			for i := 1; i <= 2; i++ {
				p := testDB.CreateTestUser(t, fmt.Sprintf("ap_p%d_%d", phaseNum, i), fmt.Sprintf("ap_p%d_%d@example.com", phaseNum, i))
				_, err := gameService.AddGameParticipant(context.Background(), game3.ID, int32(p.ID), "player")
				require.NoError(t, err)

				_, err = actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
					GameID:  game3.ID,
					PhaseID: ph.ID,
					UserID:  int32(p.ID),
					Content: fmt.Sprintf("Action P%d U%d", phaseNum, i),
					IsDraft: false,
				})
				require.NoError(t, err)
			}
		}

		// Get all with phaseID=0
		submissions, err := actionService.ListAllActionSubmissions(context.Background(), game3.ID, 0, 100, 0)
		require.NoError(t, err)
		assert.Equal(t, 4, len(submissions), "Should return submissions from all phases")
	})

	t.Run("returns empty list when no submissions match", func(t *testing.T) {
		emptyGame := testDB.CreateTestGame(t, int32(gm.ID), "Empty Submissions Game")

		submissions, err := actionService.ListAllActionSubmissions(context.Background(), emptyGame.ID, 0, 10, 0)
		require.NoError(t, err)
		assert.Empty(t, submissions, "Should return empty slice when no submissions")
	})
}

func TestActionSubmissionService_CountAllActionSubmissions(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test data
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Count Test Game")

	t.Run("counts all submissions in game", func(t *testing.T) {
		// Create phase 1 and submit while active
		phase1, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Count Phase 1",
		})
		require.NoError(t, err)

		// Create 4 submissions in phase1
		for i := 1; i <= 4; i++ {
			player := testDB.CreateTestUser(t, fmt.Sprintf("countplayer%d", i), fmt.Sprintf("countplayer%d@example.com", i))
			_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
			require.NoError(t, err)

			_, err = actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
				GameID:  game.ID,
				PhaseID: phase1.ID,
				UserID:  int32(player.ID),
				Content: fmt.Sprintf("Count Action %d", i),
				IsDraft: false,
			})
			require.NoError(t, err)
		}

		// Create phase 2 and submit while active
		phase2, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Count Phase 2",
		})
		require.NoError(t, err)

		// Create 3 submissions in phase2
		for i := 5; i <= 7; i++ {
			player := testDB.CreateTestUser(t, fmt.Sprintf("countplayer%d", i), fmt.Sprintf("countplayer%d@example.com", i))
			_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
			require.NoError(t, err)

			_, err = actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
				GameID:  game.ID,
				PhaseID: phase2.ID,
				UserID:  int32(player.ID),
				Content: fmt.Sprintf("Count Action %d", i),
				IsDraft: false,
			})
			require.NoError(t, err)
		}

		// Count all
		count, err := actionService.CountAllActionSubmissions(context.Background(), game.ID, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(7), count, "Should count all 7 submissions")
	})

	t.Run("counts submissions for specific phase", func(t *testing.T) {
		game2 := testDB.CreateTestGame(t, int32(gm.ID), "Phase Count Game")

		// Create phase 1 and submit while active
		phase1, err := phaseService.TransitionToNextPhase(context.Background(), game2.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Specific Phase 1",
		})
		require.NoError(t, err)

		// 5 in phase1
		for i := 1; i <= 5; i++ {
			p := testDB.CreateTestUser(t, fmt.Sprintf("sp1_%d", i), fmt.Sprintf("sp1_%d@example.com", i))
			_, err := gameService.AddGameParticipant(context.Background(), game2.ID, int32(p.ID), "player")
			require.NoError(t, err)

			_, err = actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
				GameID:  game2.ID,
				PhaseID: phase1.ID,
				UserID:  int32(p.ID),
				Content: "Phase 1 action",
				IsDraft: false,
			})
			require.NoError(t, err)
		}

		// Create phase 2 and submit while active
		phase2, err := phaseService.TransitionToNextPhase(context.Background(), game2.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Specific Phase 2",
		})
		require.NoError(t, err)

		// 3 in phase2
		for i := 1; i <= 3; i++ {
			p := testDB.CreateTestUser(t, fmt.Sprintf("sp2_%d", i), fmt.Sprintf("sp2_%d@example.com", i))
			_, err := gameService.AddGameParticipant(context.Background(), game2.ID, int32(p.ID), "player")
			require.NoError(t, err)

			_, err = actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
				GameID:  game2.ID,
				PhaseID: phase2.ID,
				UserID:  int32(p.ID),
				Content: "Phase 2 action",
				IsDraft: false,
			})
			require.NoError(t, err)
		}

		// Count phase1 only
		count, err := actionService.CountAllActionSubmissions(context.Background(), game2.ID, phase1.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(5), count, "Should count only phase1 submissions")
	})

	t.Run("returns 0 for game with no submissions", func(t *testing.T) {
		emptyGame := testDB.CreateTestGame(t, int32(gm.ID), "Empty Count Game")

		count, err := actionService.CountAllActionSubmissions(context.Background(), emptyGame.ID, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(0), count, "Should return 0 for game with no submissions")
	})

	t.Run("works with phaseID=0 for all phases", func(t *testing.T) {
		game3 := testDB.CreateTestGame(t, int32(gm.ID), "All Phases Count")

		// Create 2 phases with 5 submissions each
		for phaseNum := 1; phaseNum <= 2; phaseNum++ {
			ph, err := phaseService.TransitionToNextPhase(context.Background(), game3.ID, int32(gm.ID), core.TransitionPhaseRequest{
				PhaseType: "action",
				Title:     fmt.Sprintf("All Count Phase %d", phaseNum),
			})
			require.NoError(t, err)

			for i := 1; i <= 5; i++ {
				p := testDB.CreateTestUser(t, fmt.Sprintf("acp_%d_%d", phaseNum, i), fmt.Sprintf("acp_%d_%d@example.com", phaseNum, i))
				_, err := gameService.AddGameParticipant(context.Background(), game3.ID, int32(p.ID), "player")
				require.NoError(t, err)

				_, err = actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
					GameID:  game3.ID,
					PhaseID: ph.ID,
					UserID:  int32(p.ID),
					Content: "Count action",
					IsDraft: false,
				})
				require.NoError(t, err)
			}
		}

		// Count all with phaseID=0
		count, err := actionService.CountAllActionSubmissions(context.Background(), game3.ID, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(10), count, "Should count all 10 submissions across all phases")
	})
}

// TestActionSubmissionService_ResultOrdering pins down the order results come
// back in.
//
// Regression: these queries ordered by sent_at, which CreateActionResult leaves
// NULL until a result is published. Every draft therefore tied, and Postgres
// returned them in whatever order the executor produced — a GM who wrote four
// drafts saw them shuffled, with no stable order between page loads.
//
// GetGameResults and GetUserResults both sort ascending, because the History tab
// renders them through one shared path and reads chronologically for every role.
// They differ in scope, not order: GM and audience get the whole cast including
// drafts, players get only their own published rows. When the two disagreed on
// order, the same phase looked different depending on who was logged in.
//
// GetUserPhaseResults stays newest-first: it feeds the GM composing view, where a
// freshly written draft belongs at the top so the GM can confirm it landed.
func TestActionSubmissionService_ResultOrdering(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "orderinggm", "orderinggm@example.com")
	player := testDB.CreateTestUser(t, "orderingplayer", "orderingplayer@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Ordering Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Ordering Phase",
		Deadline:  core.TimePtr(time.Now().Add(72 * time.Hour)),
	})
	require.NoError(t, err)

	// Four unpublished drafts, created in a known order. All share sent_at = NULL,
	// which is exactly the case the old ordering could not resolve.
	created := make([]int32, 0, 4)
	for _, content := range []string{"First draft", "Second draft", "Third draft", "Fourth draft"} {
		result, err := actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
			GameID:      game.ID,
			PhaseID:     phase.ID,
			UserID:      int32(player.ID),
			GMUserID:    int32(gm.ID),
			Content:     content,
			IsPublished: false,
		})
		require.NoError(t, err)
		created = append(created, result.ID)
	}

	// The GM and audience History view. Ascending, matching what a player sees in
	// the same tab — see the GetUserResults subtest below for the other half.
	t.Run("GetGameResults returns drafts oldest first", func(t *testing.T) {
		results, err := actionService.GetGameResults(context.Background(), game.ID)
		require.NoError(t, err)
		require.Len(t, results, 4)

		contents := make([]string, 0, len(results))
		for _, r := range results {
			contents = append(contents, r.Content)
			require.False(t, r.SentAt.Valid, "drafts should have no sent_at, the condition that broke ordering")
		}

		assert.Equal(t, []string{"First draft", "Second draft", "Third draft", "Fourth draft"}, contents)
	})

	t.Run("GetUserPhaseResults returns drafts newest first", func(t *testing.T) {
		results, err := actionService.GetUserPhaseResults(context.Background(), phase.ID, int32(player.ID))
		require.NoError(t, err)
		require.Len(t, results, 4)

		ids := make([]int32, 0, len(results))
		for _, r := range results {
			ids = append(ids, r.ID)
		}

		assert.Equal(t, []int32{created[3], created[2], created[1], created[0]}, ids)
	})

	// The player's own view reads the other way: publish the drafts in order and
	// confirm they come back in the order they were sent, not reversed.
	t.Run("GetUserResults returns published results oldest first within a phase", func(t *testing.T) {
		for _, id := range created {
			require.NoError(t, actionService.PublishActionResult(context.Background(), id, int32(player.ID)))
		}

		results, err := actionService.GetUserResults(context.Background(), game.ID, int32(player.ID))
		require.NoError(t, err)
		require.Len(t, results, 4)

		contents := make([]string, 0, len(results))
		for _, r := range results {
			contents = append(contents, r.Content)
		}

		assert.Equal(t, []string{"First draft", "Second draft", "Third draft", "Fourth draft"}, contents)
	})

	// The regression this guards: both roles read the same phase in the same tab,
	// so the two endpoints feeding it must agree on order. Asserting each one's
	// order separately would still pass if a later change flipped one of them, so
	// compare them against each other directly. Scope legitimately differs (the GM
	// list is the whole cast), so intersect on the ids the player can see.
	t.Run("GM and player see the same order in History", func(t *testing.T) {
		gmResults, err := actionService.GetGameResults(context.Background(), game.ID)
		require.NoError(t, err)

		playerResults, err := actionService.GetUserResults(context.Background(), game.ID, int32(player.ID))
		require.NoError(t, err)

		visibleToPlayer := make(map[int32]bool, len(playerResults))
		playerOrder := make([]int32, 0, len(playerResults))
		for _, r := range playerResults {
			visibleToPlayer[r.ID] = true
			playerOrder = append(playerOrder, r.ID)
		}

		gmOrder := make([]int32, 0, len(playerOrder))
		for _, r := range gmResults {
			if visibleToPlayer[r.ID] {
				gmOrder = append(gmOrder, r.ID)
			}
		}

		require.NotEmpty(t, playerOrder, "fixture should leave the player some published results")
		assert.Equal(t, playerOrder, gmOrder,
			"History renders both through one path, so the endpoints must not disagree on order")
	})
}
