package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
)

func TestGameService_CreateGame(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "games", "sessions", "users")

	// Setup test fixtures
	fixtures := testDB.SetupFixtures(t)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	testCases := []struct {
		name        string
		request     core.CreateGameRequest
		expectError bool
		checkState  string
	}{
		{
			name: "valid game creation",
			request: core.CreateGameRequest{
				Title:       "Test Game",
				Description: "A test game for our new system",
				GMUserID:    int32(fixtures.TestUser.ID),
				Genre:       "Fantasy",
				StartDate:   core.TimePtr(time.Now().Add(24 * time.Hour)),
				EndDate:     core.TimePtr(time.Now().Add(7 * 24 * time.Hour)),
				MaxPlayers:  6,
				IsPublic:    true,
			},
			expectError: false,
			checkState:  "setup",
		},
		{
			name: "minimum valid game",
			request: core.CreateGameRequest{
				Title:       "Minimal Game",
				Description: "Minimal test game",
				GMUserID:    int32(fixtures.TestUser.ID),
				IsPublic:    false,
			},
			expectError: false,
			checkState:  "setup",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			game, err := gameService.CreateGame(context.Background(), tc.request)

			if tc.expectError {
				core.AssertError(t, err, "Expected error for invalid game creation")
				return
			}

			core.AssertNoError(t, err, "Failed to create game")
			core.AssertEqual(t, tc.request.Title, game.Title, "Game title mismatch")
			core.AssertEqual(t, tc.checkState, game.State.String, "Game state mismatch")
			core.AssertEqual(t, tc.request.GMUserID, game.GmUserID, "GM user ID mismatch")

			t.Logf("Successfully created game with ID: %d", game.ID)
		})
	}
}

func TestGameService_UpdateGameState(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "games", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create a test game first
	req := core.CreateGameRequest{
		Title:       "State Test Game",
		Description: "Testing state transitions",
		GMUserID:    int32(fixtures.TestUser.ID),
		IsPublic:    false,
	}

	game, err := gameService.CreateGame(context.Background(), req)
	core.AssertNoError(t, err, "Failed to create game")

	validTransitions := []struct {
		fromState string
		toState   string
		expected  bool
	}{
		{"setup", "recruitment", true},
		{"recruitment", "character_creation", true},
		{"character_creation", "in_progress", true},
		{"in_progress", "paused", true},
		{"paused", "in_progress", true},
		{"in_progress", "completed", true},
		{"setup", "cancelled", true},
		{"setup", "invalid_state", false},
	}

	currentState := "setup"
	for _, tt := range validTransitions {
		if tt.fromState == currentState {
			t.Run("transition_to_"+tt.toState, func(t *testing.T) {
				updatedGame, err := gameService.UpdateGameState(context.Background(), game.ID, tt.toState)

				if !tt.expected {
					core.AssertError(t, err, "Expected error for invalid state transition")
					return
				}

				core.AssertNoError(t, err, "Failed to update game state")
				core.AssertEqual(t, tt.toState, updatedGame.State.String, "Game state not updated correctly")
				currentState = tt.toState

				t.Logf("Successfully updated game state to: %s", updatedGame.State.String)
			})
		}
	}
}

func TestGameService_UpdateGameState_InvalidTransitions(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "games", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Helper to create a game at a specific state by walking the state machine.
	// States reachable via the forward path (cancelled is reached from in_progress).
	forwardPath := []string{"recruitment", "character_creation", "in_progress", "paused", "in_progress", "completed"}
	cancelledPath := []string{"recruitment", "character_creation", "in_progress", "cancelled"}

	createGameAtState := func(t *testing.T, targetState string) *models.Game {
		t.Helper()
		req := core.CreateGameRequest{
			Title:       "Transition Test Game " + targetState,
			Description: "Test game for invalid transition tests",
			GMUserID:    int32(fixtures.TestUser.ID),
		}
		game, err := gameService.CreateGame(context.Background(), req)
		core.AssertNoError(t, err, "setup: create game")

		path := forwardPath
		if targetState == "cancelled" {
			path = cancelledPath
		}

		for _, s := range path {
			if game.State.String == targetState {
				break
			}
			updated, err := gameService.UpdateGameState(context.Background(), game.ID, s)
			core.AssertNoError(t, err, "setup: advance to "+s)
			game = updated
		}
		core.AssertEqual(t, targetState, game.State.String, "setup: reached target state")
		return game
	}

	cases := []struct {
		name      string
		fromState string
		toState   string
	}{
		{"recruitment to setup (backward)", "recruitment", "setup"},
		{"in_progress to recruitment (skip back)", "in_progress", "recruitment"},
		{"completed to in_progress (reopen completed)", "completed", "in_progress"},
		{"completed to cancelled (terminal state)", "completed", "cancelled"},
		{"cancelled to recruitment (reopen cancelled)", "cancelled", "recruitment"},
		{"paused to completed (skip paused→in_progress)", "paused", "completed"},
		{"in_progress to character_creation (backward)", "in_progress", "character_creation"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			game := createGameAtState(t, tc.fromState)
			_, err := gameService.UpdateGameState(context.Background(), game.ID, tc.toState)
			core.AssertErrorContains(t, err, "invalid game state transition", "expected error for invalid transition "+tc.fromState+" → "+tc.toState)
		})
	}
}

// NOTE: TestGameService_JoinGame has been removed because direct joining is no longer supported.
// All game participation now goes through the application system (GameApplicationService).
// See game_applications_test.go for tests of the new application-based joining process.

func TestGameService_LeaveGame(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_applications", "game_participants", "games", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create a player
	player := testDB.CreateTestUser(t, "player2", "player2@example.com")

	// Create and setup game
	req := core.CreateGameRequest{
		Title:       "Leave Test Game",
		Description: "Testing game leaving",
		GMUserID:    int32(fixtures.TestUser.ID),
		IsPublic:    true,
	}

	game, err := gameService.CreateGame(context.Background(), req)
	core.AssertNoError(t, err, "Failed to create game")

	_, err = gameService.UpdateGameState(context.Background(), game.ID, "recruitment")
	core.AssertNoError(t, err, "Failed to set game to recruitment")

	// Add player as participant directly (since we removed JoinGame method)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	core.AssertNoError(t, err, "Failed to add game participant")

	testCases := []struct {
		name        string
		gameID      int32
		userID      int32
		expectError bool
		reason      string
	}{
		{
			name:        "valid player leave",
			gameID:      game.ID,
			userID:      int32(player.ID),
			expectError: false,
			reason:      "Player should be able to leave",
		},
		{
			name:        "GM cannot leave their own game",
			gameID:      game.ID,
			userID:      int32(fixtures.TestUser.ID),
			expectError: true,
			reason:      "GM should not be able to leave their own game",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := gameService.LeaveGame(context.Background(), tc.gameID, tc.userID)

			if tc.expectError {
				core.AssertError(t, err, tc.reason)
				return
			}

			core.AssertNoError(t, err, tc.reason)

			// Verify user is no longer in game (if they were a participant, not GM)
			if tc.userID != int32(fixtures.TestUser.ID) {
				inGame, err := gameService.IsUserInGame(context.Background(), tc.gameID, tc.userID)
				core.AssertNoError(t, err, "Failed to check if user is in game")
				core.AssertEqual(t, false, inGame, "User should not be in game after leaving")
			}
		})
	}
}

func TestGameService_GetUserRole(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_applications", "game_participants", "games", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create a player
	player := testDB.CreateTestUser(t, "player3", "player3@example.com")

	// Create game
	game := testDB.CreateTestGame(t, int32(fixtures.TestUser.ID), "Role Test Game")

	// Move to recruitment and add player
	_, err := gameService.UpdateGameState(context.Background(), game.ID, "recruitment")
	core.AssertNoError(t, err, "Failed to set game to recruitment")

	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	core.AssertNoError(t, err, "Failed to add game participant")

	testCases := []struct {
		name         string
		gameID       int32
		userID       int32
		expectedRole string
		expectError  bool
	}{
		{
			name:         "GM role",
			gameID:       game.ID,
			userID:       int32(fixtures.TestUser.ID),
			expectedRole: "gm",
			expectError:  false,
		},
		{
			name:         "Player role",
			gameID:       game.ID,
			userID:       int32(player.ID),
			expectedRole: "player",
			expectError:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			role, err := gameService.GetUserRole(context.Background(), tc.gameID, tc.userID)

			if tc.expectError {
				core.AssertError(t, err, "Expected error getting user role")
				return
			}

			core.AssertNoError(t, err, "Failed to get user role")
			core.AssertEqual(t, tc.expectedRole, role, "User role mismatch")
		})
	}
}

func TestGameService_UpdateGame(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "games", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create a game to update
	game := testDB.CreateTestGame(t, int32(fixtures.TestUser.ID), "Original Title")

	// Update the game
	updateReq := core.UpdateGameRequest{
		ID:          game.ID,
		Title:       "Updated Title",
		Description: "Updated description",
		Genre:       "Updated Genre",
		MaxPlayers:  8,
		IsPublic:    false,
	}

	updatedGame, err := gameService.UpdateGame(context.Background(), updateReq)
	core.AssertNoError(t, err, "Failed to update game")

	core.AssertEqual(t, updateReq.Title, updatedGame.Title, "Title not updated")
	core.AssertEqual(t, updateReq.Description, updatedGame.Description.String, "Description not updated")
	core.AssertEqual(t, updateReq.MaxPlayers, updatedGame.MaxPlayers.Int32, "MaxPlayers not updated")
	core.AssertEqual(t, updateReq.IsPublic, updatedGame.IsPublic.Bool, "IsPublic not updated")
}

func TestGameService_DeleteGame(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "games", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create a cancelled game that the GM owns
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Test Cancelled Game",
		Description: "A test game to be deleted",
		GMUserID:    int32(fixtures.TestUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Failed to create test game")

	// Cancel the game
	game, err = gameService.UpdateGameState(context.Background(), game.ID, core.GameStateCancelled)
	core.AssertNoError(t, err, "Failed to cancel game")

	// Create an active game for testing state validation
	activeGame, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Active Game",
		Description: "Should not be deletable",
		GMUserID:    int32(fixtures.TestUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Failed to create active game")

	// Create another GM's cancelled game for testing authorization
	otherGM := testDB.CreateTestUser(t, "othergm", "other_gm@example.com")
	otherGMGame, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Other GM's Game",
		Description: "Owned by different GM",
		GMUserID:    int32(otherGM.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Failed to create other GM's game")
	otherGMGame, err = gameService.UpdateGameState(context.Background(), otherGMGame.ID, core.GameStateCancelled)
	core.AssertNoError(t, err, "Failed to cancel other GM's game")

	testCases := []struct {
		name        string
		gameID      int32
		userID      int32
		expectError bool
		errorMsg    string
	}{
		{
			name:        "successful deletion by GM",
			gameID:      game.ID,
			userID:      int32(fixtures.TestUser.ID),
			expectError: false,
		},
		{
			name:        "non-GM cannot delete",
			gameID:      otherGMGame.ID,
			userID:      int32(fixtures.TestUser.ID),
			expectError: true,
			errorMsg:    "only the game master or co-GM can delete this game",
		},
		{
			name:        "cannot delete non-cancelled game",
			gameID:      activeGame.ID,
			userID:      int32(fixtures.TestUser.ID),
			expectError: true,
			errorMsg:    "only cancelled games can be deleted",
		},
		{
			name:        "cannot delete non-existent game",
			gameID:      99999,
			userID:      int32(fixtures.TestUser.ID),
			expectError: true,
			errorMsg:    "game not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := gameService.DeleteGame(context.Background(), tc.gameID, tc.userID)

			if tc.expectError {
				core.AssertError(t, err, "Expected error for: "+tc.name)
				if tc.errorMsg != "" {
					core.AssertErrorContains(t, err, tc.errorMsg, "Error message mismatch")
				}
				return
			}

			core.AssertNoError(t, err, "Failed to delete game")

			// Verify the game was actually deleted
			_, err = gameService.GetGame(context.Background(), tc.gameID)
			core.AssertError(t, err, "Game should not exist after deletion")

			t.Logf("Successfully deleted game ID: %d", tc.gameID)
		})
	}
}

// Benchmark tests for performance monitoring
func BenchmarkGameService_CreateGame(b *testing.B) {
	testDB := core.NewTestDatabase(b)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(b, "games", "users")

	fixtures := testDB.SetupFixtures(b)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := core.CreateGameRequest{
			Title:       "Benchmark Game",
			Description: "Benchmark test game",
			GMUserID:    int32(fixtures.TestUser.ID),
			IsPublic:    true,
		}

		_, err := gameService.CreateGame(context.Background(), req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGameService_GetGame(b *testing.B) {
	testDB := core.NewTestDatabase(b)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(b, "games", "users")

	fixtures := testDB.SetupFixtures(b)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create a game for benchmarking
	game := testDB.CreateTestGame(b, int32(fixtures.TestUser.ID), "Benchmark Lookup Game")

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := gameService.GetGame(context.Background(), game.ID)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestGameService_GetGamesByUser(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_participants", "games", "sessions", "users")

	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create users
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	// Create games for GM - GM must be added as participant to show up in GetGamesByUser
	game1 := testDB.CreateTestGame(t, int32(gm.ID), "GM Game 1")
	_, err := gameService.AddGameParticipant(context.Background(), game1.ID, int32(gm.ID), "player")
	core.AssertNoError(t, err, "Failed to add GM as participant")

	game2 := testDB.CreateTestGame(t, int32(gm.ID), "GM Game 2")
	_, err = gameService.AddGameParticipant(context.Background(), game2.ID, int32(gm.ID), "player")
	core.AssertNoError(t, err, "Failed to add GM as participant")

	// Create game where player is participant
	game3 := testDB.CreateTestGame(t, int32(gm.ID), "Player Game")
	_, err = gameService.AddGameParticipant(context.Background(), game3.ID, int32(gm.ID), "player")
	core.AssertNoError(t, err, "Failed to add GM as participant")
	_, err = gameService.AddGameParticipant(context.Background(), game3.ID, int32(player.ID), "player")
	core.AssertNoError(t, err, "Failed to add player as participant")

	t.Run("returns all games for GM", func(t *testing.T) {
		games, err := gameService.GetGamesByUser(context.Background(), int32(gm.ID))

		core.AssertNoError(t, err, "Failed to get games by user")
		core.AssertTrue(t, len(games) >= 3, "GM should have at least 3 games")

		// Verify our created games are in the list
		gameIDs := make(map[int32]bool)
		for _, g := range games {
			gameIDs[g.ID] = true
		}
		core.AssertTrue(t, gameIDs[game1.ID], "Game1 should be in GM's games")
		core.AssertTrue(t, gameIDs[game2.ID], "Game2 should be in GM's games")
		core.AssertTrue(t, gameIDs[game3.ID], "Game3 should be in GM's games")
	})

	t.Run("returns games where user is participant", func(t *testing.T) {
		games, err := gameService.GetGamesByUser(context.Background(), int32(player.ID))

		core.AssertNoError(t, err, "Failed to get games by user")
		core.AssertTrue(t, len(games) >= 1, "Player should have at least 1 game")

		// Verify player's game is in the list
		foundPlayerGame := false
		for _, g := range games {
			if g.ID == game3.ID {
				foundPlayerGame = true
				break
			}
		}
		core.AssertTrue(t, foundPlayerGame, "Player's game should be in the list")
	})

	t.Run("returns empty list for user with no games", func(t *testing.T) {
		noGameUser := testDB.CreateTestUser(t, "nogames", "nogames@example.com")
		games, err := gameService.GetGamesByUser(context.Background(), int32(noGameUser.ID))

		core.AssertNoError(t, err, "Failed to get games by user")
		core.AssertEqual(t, 0, len(games), "User with no games should have empty list")
	})

	// Verify game1 and game2 are in GM's games
	_ = game1
	_ = game2
}

// TestGameService_GetAllGames removed - GetAllGames method no longer exists.
// Use GetFilteredGames with empty filters instead.

func TestGameService_GetRecruitingGames(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "games", "sessions", "users")

	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test user
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")

	// Create games in different states
	setupGame := testDB.CreateTestGame(t, int32(gm.ID), "Setup Game")
	recruitingGame := testDB.CreateTestGame(t, int32(gm.ID), "Recruiting Game")
	inProgressGame := testDB.CreateTestGame(t, int32(gm.ID), "In Progress Game")

	// Update states
	_, err := gameService.UpdateGameState(context.Background(), recruitingGame.ID, "recruitment")
	core.AssertNoError(t, err, "Failed to set game to recruitment")

	_, err = gameService.UpdateGameState(context.Background(), inProgressGame.ID, "recruitment")
	core.AssertNoError(t, err, "Failed to set game to recruitment")
	_, err = gameService.UpdateGameState(context.Background(), inProgressGame.ID, "character_creation")
	core.AssertNoError(t, err, "Failed to set game to character_creation")
	_, err = gameService.UpdateGameState(context.Background(), inProgressGame.ID, "in_progress")
	core.AssertNoError(t, err, "Failed to set game to in_progress")

	t.Run("returns only games in recruitment state", func(t *testing.T) {
		games, err := gameService.GetRecruitingGames(context.Background())

		core.AssertNoError(t, err, "Failed to get recruiting games")

		// Verify recruiting game is in the list
		foundRecruiting := false
		foundSetup := false
		foundInProgress := false

		for _, g := range games {
			if g.ID == recruitingGame.ID {
				foundRecruiting = true
			}
			if g.ID == setupGame.ID {
				foundSetup = true
			}
			if g.ID == inProgressGame.ID {
				foundInProgress = true
			}
		}

		core.AssertTrue(t, foundRecruiting, "Recruiting game should be in the list")
		core.AssertEqual(t, false, foundSetup, "Setup game should NOT be in the list")
		core.AssertEqual(t, false, foundInProgress, "In-progress game should NOT be in the list")
	})
}

func TestGameService_GetGameWithDetails(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_participants", "games", "sessions", "users")

	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create users
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")

	// Create game
	game := testDB.CreateTestGame(t, int32(gm.ID), "Detailed Game")

	// Add participants
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	core.AssertNoError(t, err, "Failed to add player1")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	core.AssertNoError(t, err, "Failed to add player2")

	t.Run("returns game with GM username and participant count", func(t *testing.T) {
		details, err := gameService.GetGameWithDetails(context.Background(), game.ID)

		core.AssertNoError(t, err, "Failed to get game with details")
		core.AssertEqual(t, game.ID, details.ID, "Game ID should match")
		core.AssertEqual(t, gm.Username, details.GmUsername.String, "GM username should match")
		core.AssertEqual(t, int64(2), details.CurrentPlayers, "Should have 2 participants")
	})

	t.Run("returns error for non-existent game", func(t *testing.T) {
		_, err := gameService.GetGameWithDetails(context.Background(), 99999)

		core.AssertError(t, err, "Should return error for non-existent game")
	})
}

func TestGameService_CanUserJoinGame(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_participants", "games", "sessions", "users")

	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create users
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	// Create game in recruitment state
	game := testDB.CreateTestGame(t, int32(gm.ID), "Join Test Game")
	_, err := gameService.UpdateGameState(context.Background(), game.ID, "recruitment")
	core.AssertNoError(t, err, "Failed to set game to recruitment")

	t.Run("allows user to join game in recruitment", func(t *testing.T) {
		result, err := gameService.CanUserJoinGame(context.Background(), game.ID, int32(player.ID))

		core.AssertNoError(t, err, "Failed to check if user can join")
		core.AssertEqual(t, "can_join", result, "User should be able to join recruiting game")
	})

	t.Run("prevents user from joining game they're already in", func(t *testing.T) {
		// Add player to game
		_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
		core.AssertNoError(t, err, "Failed to add player")

		result, err := gameService.CanUserJoinGame(context.Background(), game.ID, int32(player.ID))

		core.AssertNoError(t, err, "Failed to check if user can join")
		core.AssertEqual(t, "already_joined", result, "User should not be able to join game they're in")
	})
}

func TestGameService_RemoveGameParticipant(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_participants", "games", "sessions", "users")

	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create users
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	// Create game
	game := testDB.CreateTestGame(t, int32(gm.ID), "Remove Test Game")

	// Add participant
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	core.AssertNoError(t, err, "Failed to add participant")

	t.Run("removes participant from game", func(t *testing.T) {
		// Verify player is in game
		inGame, err := gameService.IsUserInGame(context.Background(), game.ID, int32(player.ID))
		core.AssertNoError(t, err, "Failed to check if user is in game")
		core.AssertTrue(t, inGame, "Player should be in game before removal")

		// Remove participant
		err = gameService.RemoveGameParticipant(context.Background(), game.ID, int32(player.ID))
		core.AssertNoError(t, err, "Failed to remove participant")

		// Verify player is no longer in game
		inGame, err = gameService.IsUserInGame(context.Background(), game.ID, int32(player.ID))
		core.AssertNoError(t, err, "Failed to check if user is in game")
		core.AssertEqual(t, false, inGame, "Player should not be in game after removal")
	})

	t.Run("handles removing non-existent participant gracefully", func(t *testing.T) {
		// Try to remove player again (already removed)
		err := gameService.RemoveGameParticipant(context.Background(), game.ID, int32(player.ID))

		// Should not error (idempotent operation)
		core.AssertNoError(t, err, "Removing non-existent participant should not error")
	})
}

// Helper function for time pointers
func timePtr(t time.Time) *time.Time {
	return &t
}

func TestGameService_GetFilteredGames(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_participants", "game_applications", "games", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	ctx := context.Background()

	// Create test users
	gm := testDB.CreateTestUser(t, "testgm", "gm@example.com")
	player := fixtures.TestUser

	// Create test games with different states and genres
	fantasyGame, err := gameService.CreateGame(ctx, core.CreateGameRequest{
		Title:       "Fantasy Adventure",
		Description: "An epic fantasy quest",
		GMUserID:    int32(gm.ID),
		Genre:       "Fantasy",
		IsPublic:    true,
		MaxPlayers:  5,
	})
	core.AssertNoError(t, err, "Failed to create fantasy game")

	scifiGame, err := gameService.CreateGame(ctx, core.CreateGameRequest{
		Title:       "Space Odyssey",
		Description: "A sci-fi adventure",
		GMUserID:    int32(gm.ID),
		Genre:       "Sci-Fi",
		IsPublic:    true,
		MaxPlayers:  4,
	})
	core.AssertNoError(t, err, "Failed to create sci-fi game")

	_, err = gameService.CreateGame(ctx, core.CreateGameRequest{
		Title:       "Haunted Mansion",
		Description: "A horror investigation",
		GMUserID:    int32(gm.ID),
		Genre:       "Horror",
		IsPublic:    true,
		MaxPlayers:  3,
	})
	core.AssertNoError(t, err, "Failed to create horror game")

	// Transition fantasy game to recruitment
	_, err = gameService.UpdateGameState(ctx, fantasyGame.ID, "recruitment")
	core.AssertNoError(t, err, "Failed to update fantasy game state")

	// Transition sci-fi game to in_progress
	_, err = gameService.UpdateGameState(ctx, scifiGame.ID, "recruitment")
	core.AssertNoError(t, err, "Failed to update sci-fi game to recruitment")
	_, err = gameService.UpdateGameState(ctx, scifiGame.ID, "character_creation")
	core.AssertNoError(t, err, "Failed to update sci-fi game to character_creation")
	_, err = gameService.UpdateGameState(ctx, scifiGame.ID, "in_progress")
	core.AssertNoError(t, err, "Failed to update sci-fi game to in_progress")

	// Add player to sci-fi game
	_, err = gameService.AddGameParticipant(ctx, scifiGame.ID, int32(player.ID), "player")
	core.AssertNoError(t, err, "Failed to add player to sci-fi game")

	t.Run("returns all public games when no filters", func(t *testing.T) {
		result, err := gameService.GetFilteredGames(ctx, core.GameListingFilters{
			SortBy: "recent_activity",
		})

		core.AssertNoError(t, err, "Failed to get filtered games")
		core.AssertTrue(t, len(result.Games) >= 3, "Should return at least 3 games")
		core.AssertTrue(t, result.Metadata.TotalCount >= 3, "Total count should be at least 3")
	})

	t.Run("filters by state - recruitment only", func(t *testing.T) {
		result, err := gameService.GetFilteredGames(ctx, core.GameListingFilters{
			States: []string{"recruitment"},
			SortBy: "recent_activity",
		})

		core.AssertNoError(t, err, "Failed to get filtered games")
		core.AssertTrue(t, len(result.Games) >= 1, "Should return at least 1 recruitment game")

		// Verify all returned games are in recruitment
		for _, game := range result.Games {
			core.AssertEqual(t, "recruitment", game.State, "Game should be in recruitment state")
		}
	})

	// Note: Genre filtering removed - not currently supported in GameListingFilters

	// Note: Multiple genre filtering removed - not currently supported in GameListingFilters

	t.Run("filters by participation - my_games", func(t *testing.T) {
		playerID := int32(player.ID)
		participationFilter := "my_games"

		result, err := gameService.GetFilteredGames(ctx, core.GameListingFilters{
			UserID:              &playerID,
			ParticipationFilter: &participationFilter,
			SortBy:              "recent_activity",
		})

		core.AssertNoError(t, err, "Failed to get filtered games")
		core.AssertTrue(t, len(result.Games) >= 1, "Player should have at least 1 game")

		// Verify player is participant in all returned games
		for _, game := range result.Games {
			core.AssertTrue(t, game.UserRelationship != nil, "UserRelationship should not be nil")
			isParticipant := *game.UserRelationship == "participant" || *game.UserRelationship == "gm"
			core.AssertTrue(t, isParticipant, "Game should show user as participant or gm")
		}
	})

	t.Run("filters by participation - not_joined", func(t *testing.T) {
		playerID := int32(player.ID)
		participationFilter := "not_joined"

		result, err := gameService.GetFilteredGames(ctx, core.GameListingFilters{
			UserID:              &playerID,
			ParticipationFilter: &participationFilter,
			SortBy:              "recent_activity",
		})

		core.AssertNoError(t, err, "Failed to get filtered games")

		// Verify player is not in any returned games
		for _, game := range result.Games {
			if game.UserRelationship != nil {
				core.AssertTrue(t, *game.UserRelationship == "none", "User should not be in game")
			}
		}
	})

	t.Run("filters by has_open_spots", func(t *testing.T) {
		hasOpenSpots := true

		result, err := gameService.GetFilteredGames(ctx, core.GameListingFilters{
			HasOpenSpots: &hasOpenSpots,
			SortBy:       "recent_activity",
		})

		core.AssertNoError(t, err, "Failed to get filtered games")

		// Verify all returned games have open spots
		for _, game := range result.Games {
			if game.MaxPlayers != nil {
				hasSpots := game.CurrentPlayers < *game.MaxPlayers
				core.AssertTrue(t, hasSpots, "Game should have open spots")
			}
		}
	})

	t.Run("sorts by alphabetical", func(t *testing.T) {
		result, err := gameService.GetFilteredGames(ctx, core.GameListingFilters{
			SortBy: "alphabetical",
		})

		core.AssertNoError(t, err, "Failed to get filtered games")
		core.AssertTrue(t, len(result.Games) >= 2, "Should have at least 2 games to verify sorting")

		// Verify games are sorted alphabetically
		if len(result.Games) >= 2 {
			for i := 0; i < len(result.Games)-1; i++ {
				current := result.Games[i].Title
				next := result.Games[i+1].Title
				if current > next {
					t.Logf("Found out-of-order games:")
					t.Logf("  [%d] %s", i, current)
					t.Logf("  [%d] %s", i+1, next)
					t.Logf("All titles in order:")
					for j, g := range result.Games {
						t.Logf("  [%d] %s", j, g.Title)
					}
					t.Fatalf("Games should be sorted alphabetically: %s > %s", current, next)
				}
			}
		}
	})

	t.Run("enriches with user relationship when authenticated", func(t *testing.T) {
		playerID := int32(player.ID)

		result, err := gameService.GetFilteredGames(ctx, core.GameListingFilters{
			UserID: &playerID,
			SortBy: "recent_activity",
		})

		core.AssertNoError(t, err, "Failed to get filtered games")

		// Find the sci-fi game (where player is participant)
		var scifiGameResult *core.EnrichedGameListItem
		for _, game := range result.Games {
			if game.ID == scifiGame.ID {
				scifiGameResult = game
				break
			}
		}

		core.AssertTrue(t, scifiGameResult != nil, "Should find sci-fi game in results")
		core.AssertTrue(t, scifiGameResult.UserRelationship != nil, "UserRelationship should not be nil")
		core.AssertEqual(t, "participant", *scifiGameResult.UserRelationship, "Player should be participant")
	})

	t.Run("returns correct metadata", func(t *testing.T) {
		result, err := gameService.GetFilteredGames(ctx, core.GameListingFilters{
			SortBy: "recent_activity",
		})

		core.AssertNoError(t, err, "Failed to get filtered games")

		// Verify metadata structure
		core.AssertTrue(t, result.Metadata.TotalCount > 0, "TotalCount should be positive")
		core.AssertEqual(t, len(result.Games), result.Metadata.FilteredCount, "FilteredCount should match games length")
		core.AssertTrue(t, len(result.Metadata.AvailableStates) > 0, "Should have available states")
	})

	t.Run("combines multiple filters", func(t *testing.T) {
		result, err := gameService.GetFilteredGames(ctx, core.GameListingFilters{
			States: []string{"setup", "recruitment"},
			SortBy: "alphabetical",
		})

		core.AssertNoError(t, err, "Failed to get filtered games")

		// Verify all games match the combined filters
		for _, game := range result.Games {
			// Check state
			stateValid := game.State == "setup" || game.State == "recruitment"
			core.AssertTrue(t, stateValid, "Game should be in setup or recruitment")
		}
	})

	t.Run("defaults to recent_activity sort when not specified", func(t *testing.T) {
		result, err := gameService.GetFilteredGames(ctx, core.GameListingFilters{})

		core.AssertNoError(t, err, "Failed to get filtered games")
		core.AssertTrue(t, len(result.Games) > 0, "Should return games")
		// Default sort is applied internally, just verify it doesn't error
	})
}

// ============================================================================
// Audience Participation Tests
// ============================================================================

func TestGameService_AudienceParticipation(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "games", "game_participants", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	ctx := context.Background()

	// Create a test game
	game, err := gameService.CreateGame(ctx, core.CreateGameRequest{
		Title:              "Audience Test Game",
		Description:        "Testing audience participation",
		GMUserID:           int32(fixtures.TestUser.ID),
		IsPublic:           true,
		AutoAcceptAudience: true,
	})
	core.AssertNoError(t, err, "Failed to create test game")

	t.Run("GetGameAutoAcceptAudience returns default true", func(t *testing.T) {
		autoAccept, err := gameService.GetGameAutoAcceptAudience(ctx, game.ID)
		core.AssertNoError(t, err, "Failed to get auto-accept setting")
		core.AssertEqual(t, true, autoAccept, "Default auto-accept should be true (database default)")
	})

	t.Run("UpdateGameAutoAcceptAudience updates setting", func(t *testing.T) {
		err := gameService.UpdateGameAutoAcceptAudience(ctx, game.ID, true)
		core.AssertNoError(t, err, "Failed to update auto-accept setting")

		autoAccept, err := gameService.GetGameAutoAcceptAudience(ctx, game.ID)
		core.AssertNoError(t, err, "Failed to get auto-accept setting")
		core.AssertEqual(t, true, autoAccept, "Auto-accept should be true after update")
	})

	// Create test users for audience tests
	audienceUser1 := testDB.CreateTestUser(t, "audience1@example.com", "Audience User 1")
	audienceUser2 := testDB.CreateTestUser(t, "audience2@example.com", "Audience User 2")

	t.Run("CreateAudienceApplication with auto-accept creates active participant", func(t *testing.T) {
		// Ensure auto-accept is enabled
		err := gameService.UpdateGameAutoAcceptAudience(ctx, game.ID, true)
		core.AssertNoError(t, err, "Failed to enable auto-accept")

		participant, err := gameService.CreateAudienceApplication(ctx, game.ID, int32(audienceUser1.ID))
		core.AssertNoError(t, err, "Failed to create audience application")
		if participant == nil {
			t.Fatal("Participant should not be nil")
		}
		core.AssertEqual(t, "active", participant.Status.String, "Status should be active with auto-accept")
		core.AssertEqual(t, "audience", participant.Role, "Role should be audience")
	})

	t.Run("CreateAudienceApplication without auto-accept creates inactive participant", func(t *testing.T) {
		// Disable auto-accept
		err := gameService.UpdateGameAutoAcceptAudience(ctx, game.ID, false)
		core.AssertNoError(t, err, "Failed to disable auto-accept")

		participant, err := gameService.CreateAudienceApplication(ctx, game.ID, int32(audienceUser2.ID))
		core.AssertNoError(t, err, "Failed to create audience application")
		if participant == nil {
			t.Fatal("Participant should not be nil")
		}
		core.AssertEqual(t, "inactive", participant.Status.String, "Status should be inactive without auto-accept")
		core.AssertEqual(t, "audience", participant.Role, "Role should be audience")
	})

	t.Run("ListAudienceMembers returns active audience members", func(t *testing.T) {
		// audienceUser1 was added as active audience earlier
		members, err := gameService.ListAudienceMembers(ctx, game.ID)
		core.AssertNoError(t, err, "Failed to list audience members")
		core.AssertTrue(t, len(members) >= 1, "Should have at least one audience member")

		// Verify the active member
		found := false
		for _, member := range members {
			if member.UserID == int32(audienceUser1.ID) && member.Status.Valid && member.Status.String == "active" {
				found = true
				core.AssertEqual(t, "audience", member.Role, "Role should be audience")
			}
		}
		core.AssertTrue(t, found, "Should find the active audience member")
	})

	t.Run("CheckAudienceAccess returns true for GM", func(t *testing.T) {
		hasAccess, err := gameService.CheckAudienceAccess(ctx, game.ID, int32(fixtures.TestUser.ID))
		core.AssertNoError(t, err, "Failed to check audience access")
		core.AssertTrue(t, hasAccess, "GM should have audience access")
	})

	t.Run("CheckAudienceAccess returns true for active audience member", func(t *testing.T) {
		hasAccess, err := gameService.CheckAudienceAccess(ctx, game.ID, int32(audienceUser1.ID))
		core.AssertNoError(t, err, "Failed to check audience access")
		core.AssertTrue(t, hasAccess, "Active audience member should have access")
	})

	t.Run("CheckAudienceAccess returns false for non-participant", func(t *testing.T) {
		testUser3 := testDB.CreateTestUser(t, "random_user@example.com", "Random User")
		hasAccess, err := gameService.CheckAudienceAccess(ctx, game.ID, int32(testUser3.ID))
		core.AssertNoError(t, err, "Failed to check audience access")
		core.AssertEqual(t, false, hasAccess, "Random user should not have audience access")
	})
}

// TestGameService_CanUserViewGame tests the public archive access for completed games
func TestGameService_CanUserViewGame(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	ctx := context.Background()

	// Create test users
	gmUser := testDB.CreateTestUser(t, "gm@example.com", "GM User")
	playerUser := testDB.CreateTestUser(t, "player@example.com", "Player User")
	audienceUser := testDB.CreateTestUser(t, "audience@example.com", "Audience User")
	randomUser := testDB.CreateTestUser(t, "random@example.com", "Random User")

	t.Run("completed game allows ANY user to view (public archive)", func(t *testing.T) {
		// Create a completed game
		completedGame := testDB.CreateTestGameWithState(t, int32(gmUser.ID), "Completed Game", core.GameStateCompleted)

		// Random user (not a participant) should be able to view
		canView, err := gameService.CanUserViewGame(ctx, completedGame.ID, int32(randomUser.ID))
		core.AssertNoError(t, err, "Failed to check view access")
		core.AssertTrue(t, canView, "Random user should be able to view completed game (public archive)")

		// GM should also be able to view (obviously)
		canView, err = gameService.CanUserViewGame(ctx, completedGame.ID, int32(gmUser.ID))
		core.AssertNoError(t, err, "Failed to check GM view access")
		core.AssertTrue(t, canView, "GM should be able to view completed game")
	})

	t.Run("cancelled game does NOT allow non-participants to view (private)", func(t *testing.T) {
		// Create a cancelled game
		cancelledGame := testDB.CreateTestGameWithState(t, int32(gmUser.ID), "Cancelled Game", core.GameStateCancelled)

		// Random user should NOT be able to view cancelled game
		canView, err := gameService.CanUserViewGame(ctx, cancelledGame.ID, int32(randomUser.ID))
		core.AssertNoError(t, err, "Failed to check view access")
		core.AssertEqual(t, false, canView, "Random user should NOT be able to view cancelled game")

		// GM should still be able to view their own cancelled game
		canView, err = gameService.CanUserViewGame(ctx, cancelledGame.ID, int32(gmUser.ID))
		core.AssertNoError(t, err, "Failed to check GM view access")
		core.AssertTrue(t, canView, "GM should be able to view their cancelled game")
	})

	t.Run("active game follows normal permissions (participants only)", func(t *testing.T) {
		// Create an active game
		activeGame := testDB.CreateTestGameWithState(t, int32(gmUser.ID), "Active Game", core.GameStateInProgress)

		// Add player as participant
		testDB.AddTestGameParticipant(t, activeGame.ID, int32(playerUser.ID), "player")

		// Add audience member
		testDB.AddTestGameParticipant(t, activeGame.ID, int32(audienceUser.ID), "audience")

		// GM should be able to view
		canView, err := gameService.CanUserViewGame(ctx, activeGame.ID, int32(gmUser.ID))
		core.AssertNoError(t, err, "Failed to check GM view access")
		core.AssertTrue(t, canView, "GM should be able to view active game")

		// Player participant should be able to view
		canView, err = gameService.CanUserViewGame(ctx, activeGame.ID, int32(playerUser.ID))
		core.AssertNoError(t, err, "Failed to check player view access")
		core.AssertTrue(t, canView, "Player should be able to view active game")

		// Audience member should be able to view
		canView, err = gameService.CanUserViewGame(ctx, activeGame.ID, int32(audienceUser.ID))
		core.AssertNoError(t, err, "Failed to check audience view access")
		core.AssertTrue(t, canView, "Audience member should be able to view active game")

		// Random user should NOT be able to view
		canView, err = gameService.CanUserViewGame(ctx, activeGame.ID, int32(randomUser.ID))
		core.AssertNoError(t, err, "Failed to check random user view access")
		core.AssertEqual(t, false, canView, "Random user should NOT be able to view active game")
	})

	t.Run("recruitment game follows normal permissions", func(t *testing.T) {
		// Create a recruiting game
		recruitingGame := testDB.CreateTestGameWithState(t, int32(gmUser.ID), "Recruiting Game", core.GameStateRecruitment)

		// GM should be able to view
		canView, err := gameService.CanUserViewGame(ctx, recruitingGame.ID, int32(gmUser.ID))
		core.AssertNoError(t, err, "Failed to check GM view access")
		core.AssertTrue(t, canView, "GM should be able to view recruiting game")

		// Random user should NOT be able to view (not public archive yet)
		canView, err = gameService.CanUserViewGame(ctx, recruitingGame.ID, int32(randomUser.ID))
		core.AssertNoError(t, err, "Failed to check random user view access")
		core.AssertEqual(t, false, canView, "Random user should NOT be able to view recruiting game")
	})
}

// TestGameService_CancelledGameRejectsPendingApplications tests Bug #8:
// When a game is cancelled, all pending applications should be automatically rejected
func TestGameService_CancelledGameRejectsPendingApplications(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_applications", "game_participants", "games", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	appService := &GameApplicationService{DB: testDB.Pool}

	// Create test users who will apply to the game
	applicant1 := testDB.CreateTestUser(t, "applicant1", "applicant1@example.com")
	applicant2 := testDB.CreateTestUser(t, "applicant2", "applicant2@example.com")

	// Create a game and set it to recruitment
	req := core.CreateGameRequest{
		Title:       "Bug #8 Test Game",
		Description: "Testing cancelled game application handling",
		GMUserID:    int32(fixtures.TestUser.ID),
		IsPublic:    true,
	}

	game, err := gameService.CreateGame(context.Background(), req)
	core.AssertNoError(t, err, "Failed to create game")

	_, err = gameService.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	core.AssertNoError(t, err, "Failed to set game to recruitment")

	// Submit applications from both users
	_, err = appService.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
		GameID:  game.ID,
		UserID:  int32(applicant1.ID),
		Role:    "player",
		Message: "I want to join!",
	})
	core.AssertNoError(t, err, "Failed to submit application 1")

	_, err = appService.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
		GameID:  game.ID,
		UserID:  int32(applicant2.ID),
		Role:    "audience",
		Message: "I'll watch",
	})
	core.AssertNoError(t, err, "Failed to submit application 2")

	// Verify both applications are pending
	app1, err := appService.GetGameApplicationByUserAndGame(context.Background(), game.ID, int32(applicant1.ID))
	core.AssertNoError(t, err, "Failed to get application 1")
	core.AssertEqual(t, "pending", app1.Status.String, "Application 1 should be pending")

	app2, err := appService.GetGameApplicationByUserAndGame(context.Background(), game.ID, int32(applicant2.ID))
	core.AssertNoError(t, err, "Failed to get application 2")
	core.AssertEqual(t, "pending", app2.Status.String, "Application 2 should be pending")

	// Cancel the game
	_, err = gameService.UpdateGameState(context.Background(), game.ID, core.GameStateCancelled)
	core.AssertNoError(t, err, "Failed to cancel game")

	// Verify both applications are now rejected
	app1After, err := appService.GetGameApplicationByUserAndGame(context.Background(), game.ID, int32(applicant1.ID))
	core.AssertNoError(t, err, "Failed to get application 1 after cancellation")
	core.AssertEqual(t, "rejected", app1After.Status.String, "Application 1 should be rejected after game cancellation")

	app2After, err := appService.GetGameApplicationByUserAndGame(context.Background(), game.ID, int32(applicant2.ID))
	core.AssertNoError(t, err, "Failed to get application 2 after cancellation")
	core.AssertEqual(t, "rejected", app2After.Status.String, "Application 2 should be rejected after game cancellation")

	t.Log("Successfully verified that cancelled game automatically rejects all pending applications")
}

func TestGameService_PromoteToCoGM(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_participants", "games", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create a test game
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:              "Co-GM Test Game",
		Description:        "Testing co-GM promotion",
		GMUserID:           int32(fixtures.TestUser.ID),
		IsPublic:           false,
		AutoAcceptAudience: true,
	})
	core.AssertNoError(t, err, "Failed to create game")

	// Add an audience member to promote
	audienceMember := testDB.CreateTestUser(t, "audience@example.com", "Audience Member")

	// Add audience member to game
	participant, err := gameService.CreateAudienceApplication(context.Background(), game.ID, int32(audienceMember.ID))
	core.AssertNoError(t, err, "Failed to add audience member")
	core.AssertEqual(t, "audience", participant.Role, "Initial role should be audience")

	// Add another audience member for testing "only one co-GM" rule
	audienceMember2 := testDB.CreateTestUser(t, "audience2@example.com", "Audience Member 2")
	_, err = gameService.CreateAudienceApplication(context.Background(), game.ID, int32(audienceMember2.ID))
	core.AssertNoError(t, err, "Failed to add second audience member")

	testCases := []struct {
		name             string
		gameID           int32
		targetUserID     int32
		requestingUserID int32
		expectError      bool
		errorContains    string
	}{
		{
			name:             "successful promotion by primary GM",
			gameID:           game.ID,
			targetUserID:     int32(audienceMember.ID),
			requestingUserID: int32(fixtures.TestUser.ID),
			expectError:      false,
		},
		{
			name:             "non-GM cannot promote",
			gameID:           game.ID,
			targetUserID:     int32(audienceMember2.ID),
			requestingUserID: int32(audienceMember.ID), // Not the GM
			expectError:      true,
			errorContains:    "only the primary GM can promote",
		},
		{
			name:             "cannot promote non-participant",
			gameID:           game.ID,
			targetUserID:     9999, // Non-existent user
			requestingUserID: int32(fixtures.TestUser.ID),
			expectError:      true,
			errorContains:    "not a participant",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := gameService.PromoteToCoGM(context.Background(), tc.gameID, tc.targetUserID, tc.requestingUserID)

			if tc.expectError {
				core.AssertError(t, err, "Expected error for "+tc.name)
				if tc.errorContains != "" {
					core.AssertErrorContains(t, err, tc.errorContains, "Error message mismatch")
				}
				return
			}

			core.AssertNoError(t, err, "Failed to promote to co-GM")

			// Verify the participant's role was updated
			participants, err := gameService.GetGameParticipants(context.Background(), tc.gameID)
			core.AssertNoError(t, err, "Failed to get participants")

			// Find the promoted user
			var found bool
			for _, p := range participants {
				if p.UserID == tc.targetUserID {
					core.AssertEqual(t, "co_gm", p.Role, "Role should be co_gm after promotion")
					found = true
					break
				}
			}
			core.AssertTrue(t, found, "Promoted user not found in participants")

			t.Logf("Successfully promoted user %d to co-GM", tc.targetUserID)
		})
	}
}

func TestGameService_PromoteToCoGM_OnlyOneCoGMAllowed(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_participants", "games", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create a test game
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:              "Co-GM Limit Test Game",
		Description:        "Testing single co-GM limit",
		GMUserID:           int32(fixtures.TestUser.ID),
		IsPublic:           false,
		AutoAcceptAudience: true,
	})
	core.AssertNoError(t, err, "Failed to create game")

	// Add two audience members
	audienceMember1 := testDB.CreateTestUser(t, "audience1@example.com", "Audience Member 1")
	_, err = gameService.CreateAudienceApplication(context.Background(), game.ID, int32(audienceMember1.ID))
	core.AssertNoError(t, err, "Failed to add audience member 1")

	audienceMember2 := testDB.CreateTestUser(t, "audience2@example.com", "Audience Member 2")
	_, err = gameService.CreateAudienceApplication(context.Background(), game.ID, int32(audienceMember2.ID))
	core.AssertNoError(t, err, "Failed to add audience member 2")

	// Promote first audience member to co-GM
	err = gameService.PromoteToCoGM(context.Background(), game.ID, int32(audienceMember1.ID), int32(fixtures.TestUser.ID))
	core.AssertNoError(t, err, "Failed to promote first audience member to co-GM")

	// Try to promote second audience member to co-GM (should fail)
	err = gameService.PromoteToCoGM(context.Background(), game.ID, int32(audienceMember2.ID), int32(fixtures.TestUser.ID))
	core.AssertError(t, err, "Expected error when promoting second co-GM")
	core.AssertErrorContains(t, err, "already has a co-GM", "Error message should mention existing co-GM")

	t.Log("Successfully verified that only one co-GM is allowed per game")
}

func TestGameService_PromoteToCoGM_OnlyAudienceCanBePromoted(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_participants", "games", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create a test game
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Co-GM Role Test Game",
		Description: "Testing role restrictions",
		GMUserID:    int32(fixtures.TestUser.ID),
		MaxPlayers:  5,
		IsPublic:    false,
	})
	core.AssertNoError(t, err, "Failed to create game")

	// Add a player (not audience)
	player := testDB.CreateTestUser(t, "player@example.com", "Test Player")
	_, err = gameService.AddParticipantWithRole(context.Background(), game.ID, int32(player.ID), "player")
	core.AssertNoError(t, err, "Failed to add player")

	// Try to promote player to co-GM (should fail - must be audience first)
	err = gameService.PromoteToCoGM(context.Background(), game.ID, int32(player.ID), int32(fixtures.TestUser.ID))
	core.AssertError(t, err, "Expected error when promoting non-audience member")
	core.AssertErrorContains(t, err, "only promote audience members", "Error message should mention audience requirement")

	t.Log("Successfully verified that only audience members can be promoted to co-GM")
}

func TestGameService_DemoteFromCoGM(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_participants", "games", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create a test game
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:              "Co-GM Demotion Test Game",
		Description:        "Testing co-GM demotion",
		GMUserID:           int32(fixtures.TestUser.ID),
		IsPublic:           false,
		AutoAcceptAudience: true,
	})
	core.AssertNoError(t, err, "Failed to create game")

	// Add an audience member and promote to co-GM
	audienceMember := testDB.CreateTestUser(t, "cogm@example.com", "Test Co-GM")
	_, err = gameService.CreateAudienceApplication(context.Background(), game.ID, int32(audienceMember.ID))
	core.AssertNoError(t, err, "Failed to add audience member")

	err = gameService.PromoteToCoGM(context.Background(), game.ID, int32(audienceMember.ID), int32(fixtures.TestUser.ID))
	core.AssertNoError(t, err, "Failed to promote to co-GM")

	// Add another audience member to test permission checks
	otherUser := testDB.CreateTestUser(t, "other@example.com", "Other User")
	_, err = gameService.CreateAudienceApplication(context.Background(), game.ID, int32(otherUser.ID))
	core.AssertNoError(t, err, "Failed to add other user")

	testCases := []struct {
		name             string
		gameID           int32
		targetUserID     int32
		requestingUserID int32
		expectError      bool
		errorContains    string
	}{
		{
			name:             "successful demotion by primary GM",
			gameID:           game.ID,
			targetUserID:     int32(audienceMember.ID),
			requestingUserID: int32(fixtures.TestUser.ID),
			expectError:      false,
		},
		{
			name:             "non-GM cannot demote",
			gameID:           game.ID,
			targetUserID:     int32(audienceMember.ID),
			requestingUserID: int32(otherUser.ID), // Not the GM
			expectError:      true,
			errorContains:    "only the primary GM can demote",
		},
		{
			name:             "cannot demote non-co-GM",
			gameID:           game.ID,
			targetUserID:     int32(otherUser.ID), // Audience, not co-GM
			requestingUserID: int32(fixtures.TestUser.ID),
			expectError:      true,
			errorContains:    "only demote co-GMs",
		},
	}

	// First test successful demotion
	t.Run(testCases[0].name, func(t *testing.T) {
		tc := testCases[0]
		err := gameService.DemoteFromCoGM(context.Background(), tc.gameID, tc.targetUserID, tc.requestingUserID)
		core.AssertNoError(t, err, "Failed to demote co-GM")

		// Verify the participant's role was updated to audience
		participants, err := gameService.GetGameParticipants(context.Background(), tc.gameID)
		core.AssertNoError(t, err, "Failed to get participants")

		// Find the demoted user
		var found bool
		for _, p := range participants {
			if p.UserID == tc.targetUserID {
				core.AssertEqual(t, "audience", p.Role, "Role should be audience after demotion")
				found = true
				break
			}
		}
		core.AssertTrue(t, found, "Demoted user not found in participants")

		t.Logf("Successfully demoted user %d to audience", tc.targetUserID)
	})

	// Promote again for remaining tests
	err = gameService.PromoteToCoGM(context.Background(), game.ID, int32(audienceMember.ID), int32(fixtures.TestUser.ID))
	core.AssertNoError(t, err, "Failed to re-promote to co-GM")

	// Test error cases
	for _, tc := range testCases[1:] {
		t.Run(tc.name, func(t *testing.T) {
			err := gameService.DemoteFromCoGM(context.Background(), tc.gameID, tc.targetUserID, tc.requestingUserID)
			core.AssertError(t, err, "Expected error for "+tc.name)
			if tc.errorContains != "" {
				core.AssertErrorContains(t, err, tc.errorContains, "Error message mismatch")
			}
		})
	}
}

// TestGameService_DatabaseConstraintViolations tests database constraint enforcement
func TestGameService_DatabaseConstraintViolations(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "games", "sessions", "users")

	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	t.Run("fails to create game with non-existent GM user", func(t *testing.T) {
		req := core.CreateGameRequest{
			Title:       "Game with Invalid GM",
			Description: "Testing FK constraint",
			GMUserID:    99999, // Non-existent user ID
			IsPublic:    true,
		}

		_, err := gameService.CreateGame(context.Background(), req)
		core.AssertError(t, err, "Should fail with FK constraint violation")
		core.AssertErrorContains(t, err, "foreign key constraint", "Should contain FK constraint error message")
	})

	t.Run("fails to create game with zero GM user ID", func(t *testing.T) {
		req := core.CreateGameRequest{
			Title:       "Game with Zero GM",
			Description: "Testing zero FK",
			GMUserID:    0, // Invalid user ID
			IsPublic:    true,
		}

		_, err := gameService.CreateGame(context.Background(), req)
		core.AssertError(t, err, "Should fail with invalid FK")
	})

	t.Run("fails to create game with negative GM user ID", func(t *testing.T) {
		req := core.CreateGameRequest{
			Title:       "Game with Negative GM",
			Description: "Testing negative FK",
			GMUserID:    -1, // Invalid user ID
			IsPublic:    true,
		}

		_, err := gameService.CreateGame(context.Background(), req)
		core.AssertError(t, err, "Should fail with invalid FK")
	})
}

// TestGameService_UpdateGameState_AutoCreateGamemasterNPC tests Gamemaster NPC creation during state transition
func TestGameService_UpdateGameState_AutoCreateGamemasterNPC(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "games", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	queries := models.New(testDB.Pool)

	t.Run("creates Gamemaster NPC when transitioning to character_creation", func(t *testing.T) {
		// Create a game in setup state
		game := testDB.CreateTestGame(t, int32(fixtures.TestUser.ID), "Character Creation Test Game")
		core.AssertEqual(t, core.GameStateSetup, game.State.String, "Game should start in setup state")

		// Verify no Gamemaster NPC exists yet
		characters, err := queries.GetCharactersByGame(context.Background(), game.ID)
		core.AssertNoError(t, err, "Failed to get characters")
		core.AssertEqual(t, 0, len(characters), "Should have no characters initially")

		// Walk through the valid path to character_creation
		_, err = gameService.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
		core.AssertNoError(t, err, "Failed to transition to recruitment")

		// Transition to character_creation state
		updatedGame, err := gameService.UpdateGameState(context.Background(), game.ID, core.GameStateCharacterCreation)
		core.AssertNoError(t, err, "Failed to update game state")
		core.AssertEqual(t, core.GameStateCharacterCreation, updatedGame.State.String, "Game should be in character_creation state")

		// Verify Gamemaster NPC was created
		gamemasterNPC, err := queries.GetCharacterByNameAndGame(context.Background(), models.GetCharacterByNameAndGameParams{
			Name:   "Gamemaster",
			GameID: game.ID,
		})
		core.AssertNoError(t, err, "Gamemaster NPC should exist after state transition")

		// Verify NPC attributes
		core.AssertEqual(t, "Gamemaster", gamemasterNPC.Name, "Character name should be 'Gamemaster'")
		core.AssertEqual(t, "npc", gamemasterNPC.CharacterType, "Character type should be 'npc'")
		core.AssertEqual(t, "approved", gamemasterNPC.Status.String, "Character status should be 'approved'")
		core.AssertEqual(t, false, gamemasterNPC.UserID.Valid, "User ID should be NULL for GM NPCs")
	})

	t.Run("creates exactly one Gamemaster NPC (no duplicates)", func(t *testing.T) {
		// Create a game and walk the valid path to character_creation
		game := testDB.CreateTestGame(t, int32(fixtures.TestUser.ID), "NPC Uniqueness Test Game")

		_, err := gameService.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
		core.AssertNoError(t, err, "Failed to transition to recruitment")
		_, err = gameService.UpdateGameState(context.Background(), game.ID, core.GameStateCharacterCreation)
		core.AssertNoError(t, err, "Failed to transition to character_creation")

		// Count Gamemaster NPCs — must be exactly 1
		characters, err := queries.GetCharactersByGame(context.Background(), game.ID)
		core.AssertNoError(t, err, "Failed to get characters")
		gamemasterCount := 0
		for _, char := range characters {
			if char.Name == "Gamemaster" {
				gamemasterCount++
			}
		}
		core.AssertEqual(t, 1, gamemasterCount, "Should have exactly 1 Gamemaster NPC")
	})

	t.Run("does not create NPC for other state transitions", func(t *testing.T) {
		// Create a game in setup state
		game := testDB.CreateTestGame(t, int32(fixtures.TestUser.ID), "Other State Test Game")

		// Transition to recruitment (not character_creation)
		_, err := gameService.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
		core.AssertNoError(t, err, "Failed to transition to recruitment")

		// Verify no Gamemaster NPC was created
		characters, err := queries.GetCharactersByGame(context.Background(), game.ID)
		core.AssertNoError(t, err, "Failed to get characters")
		gamemasterCount := 0
		for _, char := range characters {
			if char.Name == "Gamemaster" {
				gamemasterCount++
			}
		}
		core.AssertEqual(t, 0, gamemasterCount, "Should have no Gamemaster NPC for non-character_creation state")
	})
}

// TestGameService_RemovePlayer verifies the transactional remove-player operation
// which soft-deletes the participant AND deactivates their characters atomically.
// Silent failure here means a removed player retains their characters in the game.
func TestGameService_RemovePlayer(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "sessions", "users")

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	_, err := gameService.AddGameParticipant(ctx, game.ID, int32(player.ID), "player")
	if err != nil {
		t.Fatalf("failed to add participant: %v", err)
	}

	// Create a character for the player
	char, err := characterService.CreateCharacter(ctx, CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        core.Int32Ptr(int32(player.ID)),
		Name:          "PlayerChar",
		CharacterType: "player_character",
	})
	if err != nil {
		t.Fatalf("failed to create character: %v", err)
	}

	t.Run("removes participant and deactivates characters atomically", func(t *testing.T) {
		err := gameService.RemovePlayer(ctx, game.ID, int32(player.ID), int32(gm.ID))
		if err != nil {
			t.Fatalf("RemovePlayer failed: %v", err)
		}

		// Player should no longer be an active participant
		participants, err := gameService.GetActiveParticipants(ctx, game.ID)
		if err != nil {
			t.Fatalf("GetActiveParticipants failed: %v", err)
		}
		for _, p := range participants {
			if p.UserID == int32(player.ID) {
				t.Errorf("removed player is still listed as an active participant")
			}
		}

		// Player's character should be deactivated
		queries := models.New(testDB.Pool)
		updatedChar, err := queries.GetCharacter(ctx, char.ID)
		if err != nil {
			t.Fatalf("failed to fetch character after removal: %v", err)
		}
		if updatedChar.IsActive {
			t.Errorf("expected character is_active=false after player removal, got true")
		}
	})
}

// The contents rewrite is a delete followed by re-inserts. Without a transaction
// a failure partway through the inserts leaves the table empty or half-populated
// with the GM's original items already gone — silent data loss on what the UI
// presents as a save. These tests pin that the whole swap is atomic.
func TestGameService_ReplaceLootTableContents(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_loot_table_contents", "game_loot_tables", "games", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	ctx := context.Background()

	seed := func(t *testing.T, name string) int32 {
		t.Helper()
		table, err := gameService.CreateLootTable(ctx, int32(fixtures.TestGame.ID), name)
		core.AssertNoError(t, err, "Should create loot table")
		err = gameService.ReplaceLootTableContents(ctx, table.ID, []core.LootTableItem{
			{Name: "Original Potion", Data: `{"effect":"heal"}`},
			{Name: "Original Scroll", Data: `{"spell":"fireball"}`},
		})
		core.AssertNoError(t, err, "Should seed original contents")
		return table.ID
	}

	t.Run("replaces the full item list on success", func(t *testing.T) {
		tableID := seed(t, "Success Table")

		err := gameService.ReplaceLootTableContents(ctx, tableID, []core.LootTableItem{
			{Name: "New Sword", Data: `{"damage":10}`},
		})
		core.AssertNoError(t, err, "Should replace contents")

		contents, err := gameService.GetGameLootTableContents(ctx, tableID)
		core.AssertNoError(t, err, "Should read contents back")
		core.AssertEqual(t, 1, len(contents), "Only the new item should remain")
		core.AssertEqual(t, "New Sword", contents[0].Name, "The new item should have replaced the originals")
	})

	t.Run("rolls back and preserves the originals when an insert fails", func(t *testing.T) {
		tableID := seed(t, "Rollback Table")

		// name is varchar(255); the second item overflows it, so the insert fails
		// after the delete and after the first item was already written.
		tooLong := make([]byte, 256)
		for i := range tooLong {
			tooLong[i] = 'x'
		}

		err := gameService.ReplaceLootTableContents(ctx, tableID, []core.LootTableItem{
			{Name: "Replacement One", Data: `{"ok":true}`},
			{Name: string(tooLong), Data: `{"ok":false}`},
		})
		core.AssertTrue(t, err != nil, "An oversized item name should fail the rewrite")

		// The critical assertion: the original items must survive a failed save.
		contents, err := gameService.GetGameLootTableContents(ctx, tableID)
		core.AssertNoError(t, err, "Should read contents back after the failed rewrite")
		core.AssertEqual(t, 2, len(contents), "The original items must survive a failed rewrite, got "+fmt.Sprint(len(contents)))
		core.AssertEqual(t, "Original Potion", contents[0].Name, "First original item should be intact")
		core.AssertEqual(t, "Original Scroll", contents[1].Name, "Second original item should be intact")
	})

	t.Run("clears every item when given an empty list", func(t *testing.T) {
		tableID := seed(t, "Clear Table")

		err := gameService.ReplaceLootTableContents(ctx, tableID, []core.LootTableItem{})
		core.AssertNoError(t, err, "Should accept an empty item list")

		contents, err := gameService.GetGameLootTableContents(ctx, tableID)
		core.AssertNoError(t, err, "Should read contents back")
		core.AssertEqual(t, 0, len(contents), "An empty list should clear the table")
	})
}
