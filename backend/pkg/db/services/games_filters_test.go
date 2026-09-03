package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"actionphase/pkg/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGameService_FilterByOpenSpots(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "games", "game_participants", "sessions", "users")

	// Setup test fixtures
	fixtures := testDB.SetupFixtures(t)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	ctx := context.Background()

	// Create games in different states with open spots
	recruitingGame := createTestGameInState(t, testDB, gameService, int32(fixtures.TestUser.ID), int32(fixtures.TestCommunity.ID), "recruitment", 5, 2)
	inProgressGame := createTestGameInState(t, testDB, gameService, int32(fixtures.TestUser.ID), int32(fixtures.TestCommunity.ID), "in_progress", 5, 2)
	pausedGame := createTestGameInState(t, testDB, gameService, int32(fixtures.TestUser.ID), int32(fixtures.TestCommunity.ID), "paused", 5, 1)
	completedGame := createTestGameInState(t, testDB, gameService, int32(fixtures.TestUser.ID), int32(fixtures.TestCommunity.ID), "completed", 5, 3)
	setupGame := createTestGameInState(t, testDB, gameService, int32(fixtures.TestUser.ID), int32(fixtures.TestCommunity.ID), "setup", 5, 0)

	// Create recruiting game with NO open spots (full)
	fullRecruitingGame := createTestGameInState(t, testDB, gameService, int32(fixtures.TestUser.ID), int32(fixtures.TestCommunity.ID), "recruitment", 3, 3)

	testCases := []struct {
		name           string
		hasOpenSpots   *bool
		expectedStates []string
		shouldInclude  []int32
		shouldExclude  []int32
	}{
		{
			name:           "has_open_spots=true returns only recruiting games with spots",
			hasOpenSpots:   boolPtr(true),
			expectedStates: []string{"recruitment"},
			shouldInclude:  []int32{recruitingGame},
			shouldExclude:  []int32{inProgressGame, pausedGame, completedGame, setupGame, fullRecruitingGame},
		},
		{
			name:          "has_open_spots=false returns all games",
			hasOpenSpots:  boolPtr(false),
			shouldInclude: []int32{recruitingGame, inProgressGame, pausedGame, completedGame, setupGame, fullRecruitingGame},
			shouldExclude: []int32{},
		},
		{
			name:          "has_open_spots=nil (not set) returns all games",
			hasOpenSpots:  nil,
			shouldInclude: []int32{recruitingGame, inProgressGame, pausedGame, completedGame, setupGame, fullRecruitingGame},
			shouldExclude: []int32{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Build filter
			userID := int32(fixtures.TestUser.ID)
			filters := core.GameListingFilters{
				UserID:       &userID,
				HasOpenSpots: tc.hasOpenSpots,
				PageSize:     100,
				Page:         1,
			}

			// Execute query
			result, err := gameService.GetFilteredGames(ctx, filters)
			require.NoError(t, err)

			// Extract game IDs
			gameIDs := make(map[int32]bool)
			for _, game := range result.Games {
				gameIDs[game.ID] = true
			}

			// Verify expected games are included
			for _, gameID := range tc.shouldInclude {
				assert.True(t, gameIDs[gameID], "Game ID %d should be included", gameID)
			}

			// Verify excluded games are not included
			for _, gameID := range tc.shouldExclude {
				assert.False(t, gameIDs[gameID], "Game ID %d should be excluded", gameID)
			}

			// Verify all games are in expected states (if specified)
			if len(tc.expectedStates) > 0 {
				expectedStateMap := make(map[string]bool)
				for _, state := range tc.expectedStates {
					expectedStateMap[state] = true
				}

				for _, game := range result.Games {
					// Only check games we created in this test
					if game.ID == recruitingGame || game.ID == inProgressGame ||
						game.ID == pausedGame || game.ID == completedGame ||
						game.ID == setupGame || game.ID == fullRecruitingGame {
						assert.True(t, expectedStateMap[game.State],
							"Game ID %d has unexpected state %s (expected one of %v)",
							game.ID, game.State, tc.expectedStates)
					}
				}
			}
		})
	}
}

func TestGameService_FilterByParticipation(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "games", "game_participants", "game_applications", "sessions", "users")

	// Setup test fixtures
	fixtures := testDB.SetupFixtures(t)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	appService := &GameApplicationService{DB: testDB.Pool}
	ctx := context.Background()

	// Create a second test user
	player2 := testDB.CreateTestUser(t, "TestPlayer2", "player2@test.com")

	// Create games with different participation scenarios
	// Game 1: User is GM
	gmGame, err := gameService.CreateGame(ctx, core.CreateGameRequest{
		Title:       "GM Game",
		Description: "Game where test user is GM",
		GMUserID:    int32(fixtures.TestUser.ID),
		CommunityID: int32(fixtures.TestCommunity.ID),
		IsPublic:    true,
	})
	require.NoError(t, err)

	// Game 2: User is participant
	participantGame, err := gameService.CreateGame(ctx, core.CreateGameRequest{
		Title:       "Participant Game",
		Description: "Game where test user is participant",
		GMUserID:    int32(player2.ID),
		CommunityID: int32(fixtures.TestCommunity.ID),
		IsPublic:    true,
	})
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(ctx, participantGame.ID, int32(fixtures.TestUser.ID), "player")
	require.NoError(t, err)

	// Game 3: User has pending application
	appliedGame, err := gameService.CreateGame(ctx, core.CreateGameRequest{
		Title:       "Applied Game",
		Description: "Game where test user has applied",
		GMUserID:    int32(player2.ID),
		CommunityID: int32(fixtures.TestCommunity.ID),
		IsPublic:    true,
	})
	require.NoError(t, err)
	// Update to recruitment state
	_, err = gameService.UpdateGameState(ctx, appliedGame.ID, "recruitment")
	require.NoError(t, err)
	// Create application
	_, err = appService.CreateGameApplication(ctx, core.CreateGameApplicationRequest{
		GameID:  appliedGame.ID,
		UserID:  int32(fixtures.TestUser.ID),
		Role:    "player",
		Message: "Test application",
	})
	require.NoError(t, err)

	// Game 4: User not joined
	notJoinedGame, err := gameService.CreateGame(ctx, core.CreateGameRequest{
		Title:       "Not Joined Game",
		Description: "Game where test user is not involved",
		GMUserID:    int32(player2.ID),
		CommunityID: int32(fixtures.TestCommunity.ID),
		IsPublic:    true,
	})
	require.NoError(t, err)

	testCases := []struct {
		name                string
		participationFilter string
		shouldInclude       []int32
		shouldExclude       []int32
		checkRelationship   string
	}{
		{
			name:                "my_games returns games user is in (GM or participant)",
			participationFilter: "my_games",
			shouldInclude:       []int32{gmGame.ID, participantGame.ID},
			shouldExclude:       []int32{appliedGame.ID, notJoinedGame.ID},
		},
		{
			name:                "applied returns games with pending applications",
			participationFilter: "applied",
			shouldInclude:       []int32{appliedGame.ID},
			shouldExclude:       []int32{gmGame.ID, participantGame.ID, notJoinedGame.ID},
			checkRelationship:   "applied",
		},
		{
			name:                "not_joined returns games user hasn't joined",
			participationFilter: "not_joined",
			shouldInclude:       []int32{appliedGame.ID, notJoinedGame.ID},
			shouldExclude:       []int32{gmGame.ID, participantGame.ID},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Build filter
			userID := int32(fixtures.TestUser.ID)
			filters := core.GameListingFilters{
				UserID:              &userID,
				ParticipationFilter: &tc.participationFilter,
				PageSize:            100,
				Page:                1,
			}

			// Execute query
			result, err := gameService.GetFilteredGames(ctx, filters)
			require.NoError(t, err)

			// Extract game IDs and relationships
			gameIDs := make(map[int32]bool)
			gameRelationships := make(map[int32]string)
			for _, game := range result.Games {
				gameIDs[game.ID] = true
				if game.UserRelationship != nil {
					gameRelationships[game.ID] = *game.UserRelationship
				}
			}

			// Verify expected games are included
			for _, gameID := range tc.shouldInclude {
				assert.True(t, gameIDs[gameID],
					"Game ID %d should be included in %s filter", gameID, tc.participationFilter)
			}

			// Verify excluded games are not included
			for _, gameID := range tc.shouldExclude {
				assert.False(t, gameIDs[gameID],
					"Game ID %d should be excluded from %s filter", gameID, tc.participationFilter)
			}

			// Verify relationship if specified
			if tc.checkRelationship != "" {
				for _, gameID := range tc.shouldInclude {
					assert.Equal(t, tc.checkRelationship, gameRelationships[gameID],
						"Game ID %d should have user_relationship=%s", gameID, tc.checkRelationship)
				}
			}
		})
	}
}

// Helper function to create a game in a specific state with participants
func createTestGameInState(t *testing.T, testDB *core.TestDatabase, gameService *GameService, gmUserID, communityID int32, state string, maxPlayers int, currentPlayers int) int32 {
	ctx := context.Background()

	game, err := gameService.CreateGame(ctx, core.CreateGameRequest{
		Title:       "Test Game - " + state,
		Description: "Game in " + state + " state",
		GMUserID:    gmUserID,
		CommunityID: communityID,
		MaxPlayers:  int32(maxPlayers),
		IsPublic:    true,
		StartDate:   core.TimePtr(time.Now().Add(24 * time.Hour)),
		EndDate:     core.TimePtr(time.Now().Add(7 * 24 * time.Hour)),
	})
	require.NoError(t, err)

	// Add participants BEFORE changing state (archived states are read-only)
	for i := 0; i < currentPlayers; i++ {
		// Create test players with unique usernames
		username := fmt.Sprintf("player_%s_%d_%d", state, game.ID, i)
		email := fmt.Sprintf("player_%s_%d_%d@test.com", state, game.ID, i)
		player := testDB.CreateTestUser(t, username, email)
		_, err = gameService.AddGameParticipant(ctx, game.ID, int32(player.ID), "player")
		require.NoError(t, err)
	}

	// Walk the state machine to reach the target state.
	// Valid forward path: setup → recruitment → character_creation → in_progress → paused → in_progress → completed
	// Cancelled can be reached from any non-terminal state.
	statePath := map[string][]string{
		"recruitment":        {"recruitment"},
		"character_creation": {"recruitment", "character_creation"},
		"in_progress":        {"recruitment", "character_creation", "in_progress"},
		"paused":             {"recruitment", "character_creation", "in_progress", "paused"},
		"completed":          {"recruitment", "character_creation", "in_progress", "completed"},
		"cancelled":          {"recruitment", "character_creation", "in_progress", "cancelled"},
	}
	if steps, ok := statePath[state]; ok {
		for _, s := range steps {
			_, err = gameService.UpdateGameState(ctx, game.ID, s)
			require.NoError(t, err, "failed to advance to state %s", s)
		}
	}

	return game.ID
}

func TestGameService_CurrentPlayersExcludesAudience(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "games", "game_participants", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	ctx := context.Background()

	game, err := gameService.CreateGame(ctx, core.CreateGameRequest{
		Title:       "Audience Count Test Game",
		Description: "Test that audience members are excluded from player count",
		GMUserID:    int32(fixtures.TestUser.ID),
		CommunityID: int32(fixtures.TestCommunity.ID),
		MaxPlayers:  5,
		IsPublic:    true,
	})
	require.NoError(t, err)

	player := testDB.CreateTestUser(t, "count_test_player", "count_player@test.com")
	audience := testDB.CreateTestUser(t, "count_test_audience", "count_audience@test.com")

	_, err = gameService.AddGameParticipant(ctx, game.ID, int32(player.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(ctx, game.ID, int32(audience.ID), "audience")
	require.NoError(t, err)

	userID := int32(fixtures.TestUser.ID)
	result, err := gameService.GetFilteredGames(ctx, core.GameListingFilters{
		UserID:   &userID,
		PageSize: 100,
		Page:     1,
	})
	require.NoError(t, err)

	var found *core.EnrichedGameListItem
	for _, g := range result.Games {
		if g.ID == game.ID {
			found = g
			break
		}
	}
	require.NotNil(t, found, "game should appear in listing")
	assert.Equal(t, int32(1), found.CurrentPlayers,
		"current_players should count only players, not audience members")
}

// TestGameService_UserRelationshipDistinguishesRoles verifies that the listing
// query reports the viewer's actual participant role. It previously reported
// every active participant as "participant", which made the games-list badge
// tell an audience member they were playing.
func TestGameService_UserRelationshipDistinguishesRoles(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "games", "game_participants", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	ctx := context.Background()

	game, err := gameService.CreateGame(ctx, core.CreateGameRequest{
		Title:       "Relationship Role Test Game",
		Description: "Test that user_relationship reflects participant role",
		GMUserID:    int32(fixtures.TestUser.ID),
		CommunityID: int32(fixtures.TestCommunity.ID),
		MaxPlayers:  5,
		IsPublic:    true,
	})
	require.NoError(t, err)

	player := testDB.CreateTestUser(t, "rel_test_player", "rel_player@test.com")
	audience := testDB.CreateTestUser(t, "rel_test_audience", "rel_audience@test.com")
	coGM := testDB.CreateTestUser(t, "rel_test_cogm", "rel_cogm@test.com")

	_, err = gameService.AddGameParticipant(ctx, game.ID, int32(player.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(ctx, game.ID, int32(audience.ID), "audience")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(ctx, game.ID, int32(coGM.ID), "co_gm")
	require.NoError(t, err)

	relationshipFor := func(t *testing.T, userID int32) string {
		t.Helper()
		result, err := gameService.GetFilteredGames(ctx, core.GameListingFilters{
			UserID:   &userID,
			PageSize: 100,
			Page:     1,
		})
		require.NoError(t, err)

		for _, g := range result.Games {
			if g.ID == game.ID {
				require.NotNil(t, g.UserRelationship, "user_relationship should be populated")
				return *g.UserRelationship
			}
		}
		t.Fatalf("game %d should appear in listing", game.ID)
		return ""
	}

	assert.Equal(t, "gm", relationshipFor(t, int32(fixtures.TestUser.ID)),
		"the game's GM should be reported as gm")
	assert.Equal(t, "participant", relationshipFor(t, int32(player.ID)),
		"a player should be reported as participant")
	assert.Equal(t, "audience", relationshipFor(t, int32(audience.ID)),
		"an audience member must not be reported as participant")
	assert.Equal(t, "co_gm", relationshipFor(t, int32(coGM.ID)),
		"a co-GM should be reported as co_gm")
}

// Helper function for bool pointers
func boolPtr(b bool) *bool {
	return &b
}
