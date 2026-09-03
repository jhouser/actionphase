package games

import (
	"actionphase/pkg/core"
	db "actionphase/pkg/db/services"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestGetFilteredGames_PaginationDefaults tests that pagination defaults are applied correctly
func TestGetFilteredGames_PaginationDefaults(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "games", "sessions", "users")
	defer testDB.CleanupTables(t, "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// Create auth token for test user
	accessToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	// Make request without pagination parameters
	req := httptest.NewRequest(http.MethodGet, "/api/v1/games/", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200 OK")

	var response GameListingResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	core.AssertNoError(t, err, "Should decode response")

	// Verify default pagination values
	core.AssertEqual(t, 1, response.Metadata.Page, "Default page should be 1")
	core.AssertEqual(t, 20, response.Metadata.PageSize, "Default page size should be 20")
	core.AssertEqual(t, false, response.Metadata.HasPreviousPage, "First page should not have previous")
}

// TestGetFilteredGames_PaginationCustomValues tests custom pagination parameters
func TestGetFilteredGames_PaginationCustomValues(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "games", "sessions", "users")
	defer testDB.CleanupTables(t, "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)

	// Create test user and games to test pagination
	fixtures := testDB.SetupFixtures(t)

	// Create auth token for test user
	accessToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create multiple games for pagination testing
	for i := 1; i <= 25; i++ {
		_, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
			Title:       "Test Game " + string(rune(i)),
			Description: "Testing pagination",
			GMUserID:    int32(fixtures.TestUser.ID),
			CommunityID: int32(fixtures.TestCommunity.ID),
			IsPublic:    true,
		})
		core.AssertNoError(t, err, "Game creation should succeed")
	}

	tests := []struct {
		name             string
		page             string
		pageSize         string
		expectedPage     int
		expectedPageSize int
		expectedCount    int
	}{
		{
			name:             "Page 2 with size 10",
			page:             "2",
			pageSize:         "10",
			expectedPage:     2,
			expectedPageSize: 10,
			expectedCount:    10,
		},
		{
			name:             "Page 3 with size 5",
			page:             "3",
			pageSize:         "5",
			expectedPage:     3,
			expectedPageSize: 5,
			expectedCount:    5,
		},
		{
			name:             "Large page size",
			page:             "1",
			pageSize:         "50",
			expectedPage:     1,
			expectedPageSize: 50,
			expectedCount:    26, // SetupFixtures creates 1 game + we created 25 = 26 total
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/games/?page="+tt.page+"&page_size="+tt.pageSize, nil)
			req.Header.Set("Authorization", "Bearer "+accessToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200 OK")

			var response GameListingResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			core.AssertNoError(t, err, "Should decode response")

			core.AssertEqual(t, tt.expectedPage, response.Metadata.Page, "Page number should match")
			core.AssertEqual(t, tt.expectedPageSize, response.Metadata.PageSize, "Page size should match")
			core.AssertEqual(t, tt.expectedCount, len(response.Games), "Game count should match")

			// For the large page size case, verify the fixture game is in the results
			if tt.name == "Large page size" {
				found := false
				for _, g := range response.Games {
					if g.Title == "Test Game" {
						found = true
						break
					}
				}
				core.AssertTrue(t, found, "Fixture game 'Test Game' should appear in results")
			}
		})
	}
}

// TestGetFilteredGames_PaginationInvalidValues tests handling of invalid pagination parameters
func TestGetFilteredGames_PaginationInvalidValues(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "games", "sessions", "users")
	defer testDB.CleanupTables(t, "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// Create auth token for test user
	accessToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	tests := []struct {
		name             string
		page             string
		pageSize         string
		expectedPage     int
		expectedPageSize int
		description      string
	}{
		{
			name:             "Negative page falls back to default",
			page:             "-1",
			pageSize:         "20",
			expectedPage:     1,
			expectedPageSize: 20,
			description:      "Negative page should default to 1",
		},
		{
			name:             "Zero page falls back to default",
			page:             "0",
			pageSize:         "20",
			expectedPage:     1,
			expectedPageSize: 20,
			description:      "Zero page should default to 1",
		},
		{
			name:             "Invalid page string falls back to default",
			page:             "invalid",
			pageSize:         "20",
			expectedPage:     1,
			expectedPageSize: 20,
			description:      "Invalid page should default to 1",
		},
		{
			name:             "Negative page size falls back to default",
			page:             "1",
			pageSize:         "-10",
			expectedPage:     1,
			expectedPageSize: 20,
			description:      "Negative page size should default to 20",
		},
		{
			name:             "Zero page size falls back to default",
			page:             "1",
			pageSize:         "0",
			expectedPage:     1,
			expectedPageSize: 20,
			description:      "Zero page size should default to 20",
		},
		{
			name:             "Page size exceeding max (100) is capped",
			page:             "1",
			pageSize:         "150",
			expectedPage:     1,
			expectedPageSize: 20,
			description:      "Page size > 100 should default to 20",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/games/?page="+tt.page+"&page_size="+tt.pageSize, nil)
			req.Header.Set("Authorization", "Bearer "+accessToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200 OK")

			var response GameListingResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			core.AssertNoError(t, err, "Should decode response")

			core.AssertEqual(t, tt.expectedPage, response.Metadata.Page, tt.description)
			core.AssertEqual(t, tt.expectedPageSize, response.Metadata.PageSize, tt.description)
		})
	}
}

// TestGetFilteredGames_PaginationMetadata tests pagination metadata calculations
func TestGetFilteredGames_PaginationMetadata(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "games", "sessions", "users")
	defer testDB.CleanupTables(t, "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// Create auth token for test user
	accessToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	// Create exactly 23 games for precise metadata testing
	// Note: SetupFixtures already creates 1 game, so total will be 24
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	for i := 1; i <= 23; i++ {
		_, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
			Title:       "Pagination Test Game " + string(rune(i)),
			Description: "Testing metadata",
			GMUserID:    int32(fixtures.TestUser.ID),
			CommunityID: int32(fixtures.TestCommunity.ID),
			IsPublic:    true,
		})
		core.AssertNoError(t, err, "Game creation should succeed")
	}

	tests := []struct {
		name                string
		page                int
		pageSize            int
		expectedTotalPages  int
		expectedHasNext     bool
		expectedHasPrevious bool
	}{
		{
			name:                "First page of 3 (page_size=10, total=24)",
			page:                1,
			pageSize:            10,
			expectedTotalPages:  3,
			expectedHasNext:     true,
			expectedHasPrevious: false,
		},
		{
			name:                "Middle page of 3",
			page:                2,
			pageSize:            10,
			expectedTotalPages:  3,
			expectedHasNext:     true,
			expectedHasPrevious: true,
		},
		{
			name:                "Last page of 3",
			page:                3,
			pageSize:            10,
			expectedTotalPages:  3,
			expectedHasNext:     false,
			expectedHasPrevious: true,
		},
		{
			name:                "Single page when page_size > total",
			page:                1,
			pageSize:            50,
			expectedTotalPages:  1,
			expectedHasNext:     false,
			expectedHasPrevious: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/games/?page="+strconv.Itoa(tt.page)+"&page_size="+strconv.Itoa(tt.pageSize), nil)
			req.Header.Set("Authorization", "Bearer "+accessToken)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200 OK")

			var response GameListingResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			core.AssertNoError(t, err, "Should decode response")

			core.AssertEqual(t, tt.expectedTotalPages, response.Metadata.TotalPages, "Total pages should match")
			core.AssertEqual(t, tt.expectedHasNext, response.Metadata.HasNextPage, "Has next page should match")
			core.AssertEqual(t, tt.expectedHasPrevious, response.Metadata.HasPreviousPage, "Has previous page should match")
			core.AssertEqual(t, 24, response.Metadata.TotalCount, "Total count should be 24")
			core.AssertEqual(t, 24, response.Metadata.FilteredCount, "Filtered count should be 24")
		})
	}
}

// TestCreateGame_ValidationErrors tests validation error scenarios for game creation
func TestCreateGame_ValidationErrors(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// Create auth token for test user
	accessToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	testCases := []struct {
		name           string
		payload        CreateGameRequest
		expectedStatus int
		expectedError  string
		description    string
	}{
		{
			name: "empty_title",
			payload: CreateGameRequest{
				Title:       "",
				Description: "A game without a title",
			},
			expectedStatus: 400,
			expectedError:  "title is required",
			description:    "Should reject game creation with empty title",
		},
		{
			name: "whitespace_only_title",
			payload: CreateGameRequest{
				Title:       "   ",
				Description: "A game with whitespace title",
			},
			expectedStatus: 400,
			expectedError:  "title is required",
			description:    "Should reject whitespace-only title (Bind trims before validating)",
		},
		{
			name: "valid_minimal_game",
			payload: CreateGameRequest{
				Title:       "Valid Game Title",
				Description: "A valid game",
				CommunityID: int32(fixtures.TestCommunity.ID),
			},
			expectedStatus: 201,
			description:    "Should accept game with just title and description",
		},
		{
			name: "valid_game_with_all_fields",
			payload: CreateGameRequest{
				Title:              "Complete Game",
				Description:        "A game with all optional fields",
				CommunityID:        int32(fixtures.TestCommunity.ID),
				Genre:              "Fantasy",
				MaxPlayers:         6,
				IsAnonymous:        true,
				AutoAcceptAudience: true,
			},
			expectedStatus: 201,
			description:    "Should accept game with all fields populated",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload, _ := json.Marshal(tc.payload)
			req := httptest.NewRequest("POST", "/api/v1/games/", bytes.NewBuffer(payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+accessToken)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			core.AssertEqual(t, tc.expectedStatus, w.Code, tc.description)

			if tc.expectedError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				core.AssertNoError(t, err, "Should decode error response")

				if errorText, ok := response["error"].(string); ok {
					if len(errorText) == 0 {
						t.Errorf("Expected error message to contain '%s', but error field was empty", tc.expectedError)
					}
					// Verify error field is present and not empty
				} else {
					t.Errorf("Expected 'error' field in response")
				}
			}
		})
	}
}

// TestSplitCommaSeparated verifies that the custom comma-splitting helpers
// correctly parse comma-separated query params. A silent bug here would cause
// GetFilteredGames to ignore genre/tag filters entirely.
func TestSplitCommaSeparated(t *testing.T) {
	cases := []struct {
		input    string
		expected []string
	}{
		{"fantasy,scifi,horror", []string{"fantasy", "scifi", "horror"}},
		{"fantasy, scifi , horror", []string{"fantasy", "scifi", "horror"}},
		{"single", []string{"single"}},
		{"", nil},
		{",,,", nil},
		{"  spaces  ,  more  ", []string{"spaces", "more"}},
	}

	for _, tc := range cases {
		got := splitCommaSeparated(tc.input)
		if len(got) != len(tc.expected) {
			t.Errorf("splitCommaSeparated(%q): got %v, want %v", tc.input, got, tc.expected)
			continue
		}
		for i := range tc.expected {
			if got[i] != tc.expected[i] {
				t.Errorf("splitCommaSeparated(%q)[%d]: got %q, want %q", tc.input, i, got[i], tc.expected[i])
			}
		}
	}
}

// TestDeleteGame_NonCancelledGame verifies that deleting a game that is not in
// "cancelled" state returns 400. Silent failure here lets GMs destroy active games.
func TestDeleteGame_NonCancelledGame(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	game := testDB.CreateTestGame(t, int32(gm.ID), "Active Game")

	// Game starts in "setup" state, which is not "cancelled"
	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/games/%d", game.ID), nil)
	req.Header.Set("Authorization", "Bearer "+gmToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Must reject deletion of non-cancelled games
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusConflict {
		t.Errorf("expected 400 or 409 for deleting non-cancelled game, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestUpdateGameState_NonGMForbidden verifies that players cannot change game state.
// A broken auth check here lets any participant advance or cancel a game.
func TestUpdateGameState_NonGMForbidden(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	if err != nil {
		t.Fatalf("failed to add participant: %v", err)
	}

	body := `{"state":"recruitment"}`
	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/games/%d/state", game.ID), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+playerToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-GM state update, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestUpdateGameState_InvalidTransitionReturns409 verifies that a rejected state
// transition answers 409 with the invalid-state app code, not a bare 500.
//
// The request is well formed and authorized — the move is simply not legal from
// where the game is. Answering 500 told clients "we broke, retry", which is
// wrong twice: nothing broke, and retrying can never succeed. This mattered
// enough to fix once epilogue introduced a one-way door a GM can actually hit.
func TestUpdateGameState_InvalidTransitionReturns409(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	tests := []struct {
		name    string
		from    string
		to      string
		because string
	}{
		{
			name:    "epilogue back to in_progress",
			from:    core.GameStateEpilogue,
			to:      core.GameStateInProgress,
			because: "the one-way door: players cannot un-see the disclosed archive",
		},
		{
			name:    "completed is terminal",
			from:    core.GameStateCompleted,
			to:      core.GameStateInProgress,
			because: "completed games are immutable",
		},
		{
			name:    "cannot skip recruitment",
			from:    core.GameStateSetup,
			to:      core.GameStateInProgress,
			because: "pre-existing rule, previously also a 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			game := testDB.CreateTestGameWithState(t, int32(gm.ID), "Transition Test Game", tt.from)

			body := fmt.Sprintf(`{"state":%q}`, tt.to)
			req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/games/%d/state", game.ID), bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+gmToken)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusConflict {
				t.Fatalf("expected 409 for %s → %s (%s), got %d (body: %s)",
					tt.from, tt.to, tt.because, rec.Code, rec.Body.String())
			}

			var resp struct {
				Code  int64  `json:"code"`
				Error string `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode error response: %v (body: %s)", err, rec.Body.String())
			}

			if resp.Code != core.ErrCodeInvalidGameState {
				t.Errorf("expected app code %d (ErrCodeInvalidGameState), got %d",
					core.ErrCodeInvalidGameState, resp.Code)
			}

			// The message must name both states — a GM who sees this needs to
			// know what the game's state actually is, not just that it refused.
			if !strings.Contains(resp.Error, tt.from) || !strings.Contains(resp.Error, tt.to) {
				t.Errorf("error message should name both states, got %q", resp.Error)
			}

			// The rejected transition must not have partially applied.
			after, err := testDB.Pool.Query(context.Background(),
				"SELECT state FROM games WHERE id = $1", game.ID)
			if err != nil {
				t.Fatalf("failed to re-read game: %v", err)
			}
			defer after.Close()
			if !after.Next() {
				t.Fatal("game row disappeared")
			}
			var state string
			if err := after.Scan(&state); err != nil {
				t.Fatalf("failed to scan state: %v", err)
			}
			if state != tt.from {
				t.Errorf("game state changed to %q despite rejected transition; want %q", state, tt.from)
			}
		})
	}
}

// TestUpdateGameState_SendsNotificationsToParticipants verifies that changing game state
// creates in-app notifications for all active participants except the GM who made the change.
// This guards against the notification goroutine being accidentally removed from UpdateGameState.
func TestUpdateGameState_SendsNotificationsToParticipants(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "notifications", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	game := testDB.CreateTestGameWithState(t, int32(gm.ID), "Notification Test Game", "in_progress")
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	if err != nil {
		t.Fatalf("failed to add player1: %v", err)
	}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	if err != nil {
		t.Fatalf("failed to add player2: %v", err)
	}

	body := fmt.Sprintf(`{"state":"paused"}`)
	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/games/%d/state", game.ID), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for state update, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	// Allow the notification goroutine to complete
	time.Sleep(200 * time.Millisecond)

	notifSvc := &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Both players should have a game_state_changed notification
	for _, player := range []struct {
		id   int
		name string
	}{{player1.ID, "player1"}, {player2.ID, "player2"}} {
		notifs, err := notifSvc.GetUserNotifications(context.Background(), int32(player.id), 10, 0)
		if err != nil {
			t.Fatalf("failed to get notifications for %s: %v", player.name, err)
		}
		var found bool
		for _, n := range notifs {
			if n.Type == core.NotificationTypeGameStateChanged {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s should receive a game_state_changed notification", player.name)
		}
	}

	// GM should NOT receive a notification (they triggered the change)
	gmNotifs, err := notifSvc.GetUserNotifications(context.Background(), int32(gm.ID), 10, 0)
	if err != nil {
		t.Fatalf("failed to get GM notifications: %v", err)
	}
	for _, n := range gmNotifs {
		if n.Type == core.NotificationTypeGameStateChanged {
			t.Error("GM should not receive a game_state_changed notification for their own action")
		}
	}
}
