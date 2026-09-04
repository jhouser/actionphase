package games

import (
	"actionphase/pkg/core"
	db "actionphase/pkg/db/services"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// TestCreateGame_AllowGroupConversations tests that allow_group_conversations persists on create and update
func TestCreateGame_AllowGroupConversations(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "games", "sessions", "users")
	defer testDB.CleanupTables(t, "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	accessToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	t.Run("persists true when explicitly set on create", func(t *testing.T) {
		requestBody := CreateGameRequest{
			CommunityID:             int32(fixtures.TestCommunity.ID),
			Title:                   "Test Game With Groups",
			Description:             "Testing allow_group_conversations=true",
			AllowGroupConversations: true,
		}
		bodyBytes, _ := json.Marshal(requestBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/games/", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusCreated, w.Code, "Should return 201")
		var response GameResponse
		json.NewDecoder(w.Body).Decode(&response)
		core.AssertEqual(t, true, response.AllowGroupConversations, "allow_group_conversations should be true in response")

		game, err := gameService.GetGame(context.Background(), response.ID)
		core.AssertNoError(t, err, "Should retrieve game")
		core.AssertEqual(t, true, game.AllowGroupConversations, "allow_group_conversations should be true in DB")
	})

	t.Run("persists false when explicitly set on create", func(t *testing.T) {
		requestBody := CreateGameRequest{
			CommunityID:             int32(fixtures.TestCommunity.ID),
			Title:                   "Test Game No Groups",
			Description:             "Testing allow_group_conversations=false",
			AllowGroupConversations: false,
		}
		bodyBytes, _ := json.Marshal(requestBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/games/", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusCreated, w.Code, "Should return 201")
		var response GameResponse
		json.NewDecoder(w.Body).Decode(&response)
		core.AssertEqual(t, false, response.AllowGroupConversations, "allow_group_conversations should be false in response")

		game, err := gameService.GetGame(context.Background(), response.ID)
		core.AssertNoError(t, err, "Should retrieve game")
		core.AssertEqual(t, false, game.AllowGroupConversations, "allow_group_conversations should be false in DB")
	})

	t.Run("can be toggled via update", func(t *testing.T) {
		game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
			Title:                   "Toggle Test Game",
			Description:             "Testing toggle",
			GMUserID:                int32(fixtures.TestUser.ID),
			CommunityID:             int32(fixtures.TestCommunity.ID),
			IsPublic:                true,
			AllowGroupConversations: true,
		})
		core.AssertNoError(t, err, "Game creation should succeed")

		// Disable group conversations
		requestBody := UpdateGameRequest{
			Title:                   game.Title,
			Description:             game.Description.String,
			IsPublic:                true,
			AllowGroupConversations: false,
		}
		bodyBytes, _ := json.Marshal(requestBody)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/games/"+strconv.Itoa(int(game.ID)), bytes.NewBuffer(bodyBytes))
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200")
		var response GameResponse
		json.NewDecoder(w.Body).Decode(&response)
		core.AssertEqual(t, false, response.AllowGroupConversations, "allow_group_conversations should be false after update")

		updated, err := gameService.GetGame(context.Background(), game.ID)
		core.AssertNoError(t, err, "Should retrieve updated game")
		core.AssertEqual(t, false, updated.AllowGroupConversations, "allow_group_conversations should be false in DB after update")
	})
}

// TestCreateGame_WithSettings tests game creation with is_anonymous and auto_accept_audience settings
// This test verifies the fix for Issues 2.1 and 2.2 - settings persistence
func TestCreateGame_WithSettings(t *testing.T) {
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
		name                       string
		isAnonymous                bool
		autoAcceptAudience         bool
		expectedIsAnonymous        bool
		expectedAutoAcceptAudience bool
		description                string
	}{
		{
			name:                       "Both settings false",
			isAnonymous:                false,
			autoAcceptAudience:         false,
			expectedIsAnonymous:        false,
			expectedAutoAcceptAudience: false,
			description:                "Default settings - both false",
		},
		{
			name:                       "Anonymous mode enabled",
			isAnonymous:                true,
			autoAcceptAudience:         false,
			expectedIsAnonymous:        true,
			expectedAutoAcceptAudience: false,
			description:                "Only anonymous mode enabled",
		},
		{
			name:                       "Auto accept audience enabled",
			isAnonymous:                false,
			autoAcceptAudience:         true,
			expectedIsAnonymous:        false,
			expectedAutoAcceptAudience: true,
			description:                "Only auto accept audience enabled",
		},
		{
			name:                       "Both settings enabled",
			isAnonymous:                true,
			autoAcceptAudience:         true,
			expectedIsAnonymous:        true,
			expectedAutoAcceptAudience: true,
			description:                "Both anonymous mode and auto accept audience enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request payload
			requestBody := CreateGameRequest{
				CommunityID:        int32(fixtures.TestCommunity.ID),
				Title:              "Test Game - " + tt.name,
				Description:        "Testing game creation with settings",
				IsAnonymous:        tt.isAnonymous,
				AutoAcceptAudience: tt.autoAcceptAudience,
			}

			bodyBytes, err := json.Marshal(requestBody)
			core.AssertNoError(t, err, "Request marshaling should succeed")

			// Make POST request to create game
			req := httptest.NewRequest(http.MethodPost, "/api/v1/games/", bytes.NewBuffer(bodyBytes))
			req.Header.Set("Authorization", "Bearer "+accessToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Assert response
			core.AssertEqual(t, http.StatusCreated, w.Code, "Should return 201 Created")

			var response GameResponse
			err = json.NewDecoder(w.Body).Decode(&response)
			core.AssertNoError(t, err, "Should decode response")

			// Verify settings were persisted
			core.AssertEqual(t, tt.expectedIsAnonymous, response.IsAnonymous, tt.description+" - is_anonymous should match")

			// Verify in database directly
			gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
			game, err := gameService.GetGame(context.Background(), response.ID)
			core.AssertNoError(t, err, "Should retrieve game from database")

			core.AssertEqual(t, tt.expectedIsAnonymous, game.IsAnonymous, tt.description+" - DB is_anonymous should match")
			core.AssertEqual(t, tt.expectedAutoAcceptAudience, game.AutoAcceptAudience, tt.description+" - DB auto_accept_audience should match")
		})
	}
}

// TestUpdateGame_WithSettings tests game update with is_anonymous and auto_accept_audience settings
// This test verifies the fix for Issue 2.2 - settings persistence on update
func TestUpdateGame_WithSettings(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "games", "sessions", "users")
	defer testDB.CleanupTables(t, "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// Create auth token for test user (GM)
	accessToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	// Create a game with settings initially false
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:              "Test Game for Update",
		Description:        "Testing game update with settings",
		GMUserID:           int32(fixtures.TestUser.ID),
		CommunityID:        int32(fixtures.TestCommunity.ID),
		IsPublic:           true,
		IsAnonymous:        false,
		AutoAcceptAudience: false,
	})
	core.AssertNoError(t, err, "Game creation should succeed")

	tests := []struct {
		name                       string
		isAnonymous                bool
		autoAcceptAudience         bool
		expectedIsAnonymous        bool
		expectedAutoAcceptAudience bool
		description                string
	}{
		{
			name:                       "Enable both settings",
			isAnonymous:                true,
			autoAcceptAudience:         true,
			expectedIsAnonymous:        true,
			expectedAutoAcceptAudience: true,
			description:                "Update to enable both settings",
		},
		{
			name:                       "Enable only anonymous mode",
			isAnonymous:                true,
			autoAcceptAudience:         false,
			expectedIsAnonymous:        true,
			expectedAutoAcceptAudience: false,
			description:                "Update to enable only anonymous mode",
		},
		{
			name:                       "Enable only auto accept audience",
			isAnonymous:                false,
			autoAcceptAudience:         true,
			expectedIsAnonymous:        false,
			expectedAutoAcceptAudience: true,
			description:                "Update to enable only auto accept audience",
		},
		{
			name:                       "Disable both settings",
			isAnonymous:                false,
			autoAcceptAudience:         false,
			expectedIsAnonymous:        false,
			expectedAutoAcceptAudience: false,
			description:                "Update to disable both settings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create update request payload
			requestBody := UpdateGameRequest{
				Title:              game.Title,
				Description:        game.Description.String,
				IsPublic:           true,
				IsAnonymous:        tt.isAnonymous,
				AutoAcceptAudience: tt.autoAcceptAudience,
			}

			bodyBytes, err := json.Marshal(requestBody)
			core.AssertNoError(t, err, "Request marshaling should succeed")

			// Make PUT request to update game
			req := httptest.NewRequest(http.MethodPut, "/api/v1/games/"+strconv.Itoa(int(game.ID)), bytes.NewBuffer(bodyBytes))
			req.Header.Set("Authorization", "Bearer "+accessToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Assert response
			core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200 OK")

			var response GameResponse
			err = json.NewDecoder(w.Body).Decode(&response)
			core.AssertNoError(t, err, "Should decode response")

			// Verify settings were updated
			core.AssertEqual(t, tt.expectedIsAnonymous, response.IsAnonymous, tt.description+" - is_anonymous should match")

			// Verify in database directly
			updatedGame, err := gameService.GetGame(context.Background(), game.ID)
			core.AssertNoError(t, err, "Should retrieve updated game from database")

			core.AssertEqual(t, tt.expectedIsAnonymous, updatedGame.IsAnonymous, tt.description+" - DB is_anonymous should match")
			core.AssertEqual(t, tt.expectedAutoAcceptAudience, updatedGame.AutoAcceptAudience, tt.description+" - DB auto_accept_audience should match")
		})
	}
}

// TestCreateGame_SettingsPersistAfterRefresh tests that settings persist across page refreshes
// This is an additional verification test to ensure database persistence
func TestCreateGame_SettingsPersistAfterRefresh(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "games", "sessions", "users")
	defer testDB.CleanupTables(t, "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	fixtures := testDB.SetupFixtures(t)
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create game with both settings enabled
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:              "Test Persistence Game",
		Description:        "Testing settings persistence",
		GMUserID:           int32(fixtures.TestUser.ID),
		CommunityID:        int32(fixtures.TestCommunity.ID),
		IsPublic:           true,
		IsAnonymous:        true,
		AutoAcceptAudience: true,
	})
	core.AssertNoError(t, err, "Game creation should succeed")

	// Simulate "page refresh" by fetching the game again
	retrievedGame, err := gameService.GetGame(context.Background(), game.ID)
	core.AssertNoError(t, err, "Should retrieve game from database")

	// Verify settings persisted
	core.AssertEqual(t, true, retrievedGame.IsAnonymous, "is_anonymous should persist")
	core.AssertEqual(t, true, retrievedGame.AutoAcceptAudience, "auto_accept_audience should persist")

	// Update with settings disabled
	_, err = gameService.UpdateGame(context.Background(), core.UpdateGameRequest{
		ID:                 game.ID,
		Title:              game.Title,
		Description:        game.Description.String,
		IsPublic:           true,
		IsAnonymous:        false,
		AutoAcceptAudience: false,
	})
	core.AssertNoError(t, err, "Game update should succeed")

	// Retrieve again after update
	updatedGame, err := gameService.GetGame(context.Background(), game.ID)
	core.AssertNoError(t, err, "Should retrieve updated game from database")

	// Verify settings were updated and persist
	core.AssertEqual(t, false, updatedGame.IsAnonymous, "is_anonymous should update and persist")
	core.AssertEqual(t, false, updatedGame.AutoAcceptAudience, "auto_accept_audience should update and persist")
}

// TestCreateGame_CommonRoomSchedule verifies that schedule fields can be set at creation time.
func TestCreateGame_CommonRoomSchedule(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "games", "sessions", "users")
	defer testDB.CleanupTables(t, "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	accessToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	openDay := int16(6)  // Saturday
	closeDay := int16(0) // Sunday
	openTime := "10:00"
	closeTime := "22:00"
	tz := "America/Chicago"

	t.Run("saves schedule fields on create", func(t *testing.T) {
		body := CreateGameRequest{
			CommunityID:         int32(fixtures.TestCommunity.ID),
			Title:               "Schedule Create Test",
			Description:         "Testing schedule fields at creation",
			CommonRoomOpenDay:   &openDay,
			CommonRoomOpenTime:  &openTime,
			CommonRoomCloseDay:  &closeDay,
			CommonRoomCloseTime: &closeTime,
			ScheduleTimezone:    &tz,
		}
		bodyBytes, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/games/", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusCreated, w.Code, "Should return 201")

		var response GameResponse
		json.NewDecoder(w.Body).Decode(&response)
		core.AssertEqual(t, int16(6), *response.CommonRoomOpenDay, "open day should be Saturday (6)")
		core.AssertEqual(t, "10:00", *response.CommonRoomOpenTime, "open time should be 10:00")
		core.AssertEqual(t, int16(0), *response.CommonRoomCloseDay, "close day should be Sunday (0)")
		core.AssertEqual(t, "22:00", *response.CommonRoomCloseTime, "close time should be 22:00")
		core.AssertEqual(t, "America/Chicago", *response.ScheduleTimezone, "timezone should persist")

		saved, err := gameService.GetGame(context.Background(), response.ID)
		core.AssertNoError(t, err, "Should retrieve saved game")
		core.AssertEqual(t, true, saved.CommonRoomOpenDay.Valid, "open day should be set in DB")
		core.AssertEqual(t, int16(6), saved.CommonRoomOpenDay.Int16, "open day value in DB")
		core.AssertEqual(t, "America/Chicago", saved.ScheduleTimezone.String, "timezone in DB")
	})

	t.Run("rejects partial schedule fields on create", func(t *testing.T) {
		body := CreateGameRequest{
			CommunityID:        int32(fixtures.TestCommunity.ID),
			Title:              "Partial Schedule Create Test",
			Description:        "Testing partial schedule rejection on create",
			CommonRoomOpenDay:  &openDay,
			CommonRoomOpenTime: &openTime,
			// Missing close fields and timezone
		}
		bodyBytes, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/games/", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusBadRequest, w.Code, "Should return 400 for partial schedule fields on create")
	})
}

// TestUpdateGame_CommonRoomSchedule_PartialFill verifies that submitting only some schedule fields is rejected.
func TestUpdateGame_CommonRoomSchedule_PartialFill(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "games", "sessions", "users")
	defer testDB.CleanupTables(t, "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	accessToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Partial Schedule Test Game",
		Description: "Testing partial schedule rejection",
		GMUserID:    int32(fixtures.TestUser.ID),
		CommunityID: int32(fixtures.TestCommunity.ID),
	})
	core.AssertNoError(t, err, "Should create game")

	openDay := int16(1)
	closeDay := int16(2)
	openTime := "09:00"
	closeTime := "17:00"

	t.Run("rejects when only open fields present", func(t *testing.T) {
		updateBody := UpdateGameRequest{
			Title:              game.Title,
			Description:        game.Description.String,
			IsPublic:           true,
			CommonRoomOpenDay:  &openDay,
			CommonRoomOpenTime: &openTime,
		}
		bodyBytes, _ := json.Marshal(updateBody)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/games/"+strconv.Itoa(int(game.ID)), bytes.NewBuffer(bodyBytes))
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		core.AssertEqual(t, http.StatusBadRequest, w.Code, "Should return 400 for partial schedule fields")
	})

	t.Run("rejects when all day/time fields present but timezone missing", func(t *testing.T) {
		updateBody := UpdateGameRequest{
			Title:               game.Title,
			Description:         game.Description.String,
			IsPublic:            true,
			CommonRoomOpenDay:   &openDay,
			CommonRoomOpenTime:  &openTime,
			CommonRoomCloseDay:  &closeDay,
			CommonRoomCloseTime: &closeTime,
			// Intentionally omit ScheduleTimezone
		}
		bodyBytes, _ := json.Marshal(updateBody)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/games/"+strconv.Itoa(int(game.ID)), bytes.NewBuffer(bodyBytes))
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		core.AssertEqual(t, http.StatusBadRequest, w.Code, "Should return 400 when timezone is missing")
	})
}

// TestUpdateGame_CommonRoomSchedule verifies that schedule fields persist and can be cleared.
func TestUpdateGame_CommonRoomSchedule(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "games", "sessions", "users")
	defer testDB.CleanupTables(t, "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	accessToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Schedule Test Game",
		Description: "Testing common room schedule persistence",
		GMUserID:    int32(fixtures.TestUser.ID),
		CommunityID: int32(fixtures.TestCommunity.ID),
	})
	core.AssertNoError(t, err, "Should create game")

	openDay := int16(6)  // Saturday
	closeDay := int16(0) // Sunday
	openTime := "10:00"
	closeTime := "22:00"
	tz := "America/New_York"

	t.Run("saves schedule fields", func(t *testing.T) {
		updateBody := UpdateGameRequest{
			Title:               game.Title,
			Description:         game.Description.String,
			IsPublic:            true,
			CommonRoomOpenDay:   &openDay,
			CommonRoomOpenTime:  &openTime,
			CommonRoomCloseDay:  &closeDay,
			CommonRoomCloseTime: &closeTime,
			ScheduleTimezone:    &tz,
		}
		bodyBytes, _ := json.Marshal(updateBody)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/games/"+strconv.Itoa(int(game.ID)), bytes.NewBuffer(bodyBytes))
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200")

		var response GameResponse
		json.NewDecoder(w.Body).Decode(&response)
		core.AssertEqual(t, int16(6), *response.CommonRoomOpenDay, "open day should be Saturday (6)")
		core.AssertEqual(t, "10:00", *response.CommonRoomOpenTime, "open time should be 10:00")
		core.AssertEqual(t, int16(0), *response.CommonRoomCloseDay, "close day should be Sunday (0)")
		core.AssertEqual(t, "22:00", *response.CommonRoomCloseTime, "close time should be 22:00")
		core.AssertEqual(t, "America/New_York", *response.ScheduleTimezone, "timezone should persist")

		saved, err := gameService.GetGame(context.Background(), game.ID)
		core.AssertNoError(t, err, "Should retrieve saved game")
		core.AssertEqual(t, true, saved.CommonRoomOpenDay.Valid, "open day should be set in DB")
		core.AssertEqual(t, int16(6), saved.CommonRoomOpenDay.Int16, "open day value in DB")
		core.AssertEqual(t, "America/New_York", saved.ScheduleTimezone.String, "timezone in DB")
	})

	t.Run("clears schedule when fields omitted", func(t *testing.T) {
		updateBody := UpdateGameRequest{
			Title:       game.Title,
			Description: game.Description.String,
			IsPublic:    true,
		}
		bodyBytes, _ := json.Marshal(updateBody)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/games/"+strconv.Itoa(int(game.ID)), bytes.NewBuffer(bodyBytes))
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200")

		var response GameResponse
		json.NewDecoder(w.Body).Decode(&response)
		if response.CommonRoomOpenDay != nil {
			t.Errorf("CommonRoomOpenDay should be nil after clearing, got %v", *response.CommonRoomOpenDay)
		}
		if response.ScheduleTimezone != nil {
			t.Errorf("ScheduleTimezone should be nil after clearing, got %v", *response.ScheduleTimezone)
		}
	})
}
