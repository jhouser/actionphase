package characters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"actionphase/pkg/core"
	db "actionphase/pkg/db/services"
	dbactions "actionphase/pkg/db/services/actions"
	dbmessages "actionphase/pkg/db/services/messages"
	"actionphase/pkg/games"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func int32Ptr(i int32) *int32 { return &i }

// setupCharacterManagementTestRouter creates a router with character management routes
func setupCharacterManagementTestRouter(app *core.App, testDB *core.TestDatabase) *chi.Mux {
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger}

	gameHandler := games.Handler{
		App:                     app,
		UserService:             &db.UserService{DB: app.Pool, Logger: app.ObsLogger},
		GameService:             &db.GameService{DB: app.Pool, Logger: app.ObsLogger},
		GameApplicationService:  &db.GameApplicationService{DB: app.Pool, Logger: app.ObsLogger},
		CharacterService:        &db.CharacterService{DB: app.Pool, Logger: app.ObsLogger},
		NotificationService:     db.NewNotificationService(app.Pool, app.ObsLogger),
		MessageService:          &dbmessages.MessageService{DB: app.Pool, Logger: app.ObsLogger, Metrics: app.Observability.OTELMetrics},
		ActionSubmissionService: &dbactions.ActionSubmissionService{DB: app.Pool, Logger: app.ObsLogger, NotificationService: db.NewNotificationService(app.Pool, app.ObsLogger)},
	}

	router := chi.NewRouter()
	router.Route("/api/v1/characters", func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(jwtauth.Authenticator(tokenAuth))
		r.Use(core.RequireAuthenticationMiddleware(userService))

		handler := &Handler{
			App:                 app,
			UserService:         &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
			CharacterService:    &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger},
			GameService:         &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger},
			NotificationService: db.NewNotificationService(testDB.Pool, app.ObsLogger),
		}
		r.Put("/{id}/rename", handler.RenameCharacter)
		r.Delete("/{id}", handler.DeleteCharacter)
		r.Post("/{id}/approve", handler.ApproveCharacter)
		r.Post("/{id}/assign", handler.AssignNPC)
		r.Put("/{id}/reassign", handler.ReassignCharacter)
		r.Post("/{id}/data", handler.SetCharacterData)
	})
	router.Route("/api/v1/games/{gameID}/characters", func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(jwtauth.Authenticator(tokenAuth))
		r.Use(core.RequireAuthenticationMiddleware(userService))
		r.Use(gameHandler.GameMiddleware())

		handler := &Handler{
			App:                 app,
			UserService:         &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
			CharacterService:    &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger},
			GameService:         &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger},
			NotificationService: db.NewNotificationService(testDB.Pool, app.ObsLogger),
		}
		r.Get("/inactive", handler.ListInactiveCharacters)
	})
	return router
}

// TestCharacterAPI_RenameCharacter tests PUT /api/v1/characters/{id}/rename
func TestCharacterAPI_RenameCharacter(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupCharacterManagementTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	otherPlayer := testDB.CreateTestUser(t, "other", "other@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)
	otherToken, err := core.CreateTestJWTTokenForUser(app, otherPlayer)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(otherPlayer.ID), "player")
	require.NoError(t, err)

	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	playerChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player.ID)),
		Name:          "Original Name",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	t.Run("owner renames their own character", func(t *testing.T) {
		body := RenameCharacterRequest{Name: "New Name"}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/characters/%d/rename", playerChar.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "New Name", response["name"])
	})

	t.Run("GM renames a player character", func(t *testing.T) {
		body := RenameCharacterRequest{Name: "GM Renamed"}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/characters/%d/rename", playerChar.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "GM Renamed", response["name"])
	})

	t.Run("other player cannot rename someone else's character", func(t *testing.T) {
		body := RenameCharacterRequest{Name: "Stolen Name"}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/characters/%d/rename", playerChar.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+otherToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("duplicate name returns conflict", func(t *testing.T) {
		// Create a second character with a name we'll try to duplicate
		otherChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
			GameID:        game.ID,
			UserID:        int32Ptr(int32(otherPlayer.ID)),
			Name:          "Existing Name",
			CharacterType: "player_character",
		})
		require.NoError(t, err)

		// Try to rename playerChar to the same name as otherChar
		body := RenameCharacterRequest{Name: otherChar.Name}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/characters/%d/rename", playerChar.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusConflict, rec.Code)
	})
}

// TestCharacterAPI_DeleteCharacter tests DELETE /api/v1/characters/{id}
func TestCharacterAPI_DeleteCharacter(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupCharacterManagementTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	t.Run("non-GM player cannot delete a character", func(t *testing.T) {
		char, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
			GameID:        game.ID,
			UserID:        int32Ptr(int32(player.ID)),
			Name:          "Character To Keep",
			CharacterType: "player_character",
		})
		require.NoError(t, err)

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/characters/%d", char.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("GM deletes a character with no activity", func(t *testing.T) {
		char, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
			GameID:        game.ID,
			UserID:        int32Ptr(int32(player.ID)),
			Name:          "Character To Delete",
			CharacterType: "player_character",
		})
		require.NoError(t, err)

		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/characters/%d", char.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)

		// Verify it's gone
		_, err = characterService.GetCharacter(context.Background(), char.ID)
		assert.Error(t, err, "character should no longer exist")
	})

	t.Run("returns 404 for non-existent character", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/v1/characters/99999", nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

// TestCharacterAPI_ApproveCharacter tests POST /api/v1/characters/{id}/approve
func TestCharacterAPI_ApproveCharacter(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupCharacterManagementTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	t.Run("GM approves character — status becomes approved in DB", func(t *testing.T) {
		char, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
			GameID:        game.ID,
			UserID:        int32Ptr(int32(player.ID)),
			Name:          "Pending Character",
			CharacterType: "player_character",
		})
		require.NoError(t, err)

		body := ApproveCharacterRequest{Status: "approved"}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/characters/%d/approve", char.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "approved", response["status"])

		// Verify DB state changed
		updated, err := characterService.GetCharacter(context.Background(), char.ID)
		require.NoError(t, err)
		assert.Equal(t, "approved", updated.Status.String)
	})

	t.Run("GM sends rejected status — returns 400", func(t *testing.T) {
		char, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
			GameID:        game.ID,
			UserID:        int32Ptr(int32(player.ID)),
			Name:          "Rejected Status Character",
			CharacterType: "player_character",
		})
		require.NoError(t, err)

		body := map[string]string{"status": "rejected"}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/characters/%d/approve", char.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("non-GM player cannot approve a character", func(t *testing.T) {
		char, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
			GameID:        game.ID,
			UserID:        int32Ptr(int32(player.ID)),
			Name:          "Auth Test Character",
			CharacterType: "player_character",
		})
		require.NoError(t, err)

		body := ApproveCharacterRequest{Status: "approved"}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/characters/%d/approve", char.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestCharacterAPI_AssignNPC tests POST /api/v1/characters/{id}/assign
func TestCharacterAPI_AssignNPC(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupCharacterManagementTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	audience := testDB.CreateTestUser(t, "audience", "audience@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(audience.ID), "audience")
	require.NoError(t, err)

	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	npc, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		Name:          "NPC Character",
		CharacterType: "npc",
	})
	require.NoError(t, err)

	t.Run("GM can assign NPC to audience member", func(t *testing.T) {
		body := AssignNPCRequest{AssignedUserID: int32(audience.ID)}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/characters/%d/assign", npc.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("GM cannot assign NPC to a player (not audience)", func(t *testing.T) {
		body := AssignNPCRequest{AssignedUserID: int32(player.ID)}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/characters/%d/assign", npc.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("non-GM player cannot assign NPC", func(t *testing.T) {
		body := AssignNPCRequest{AssignedUserID: int32(audience.ID)}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/characters/%d/assign", npc.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestCharacterAPI_ReassignCharacter tests PUT /api/v1/characters/{id}/reassign
func TestCharacterAPI_ReassignCharacter(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupCharacterManagementTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	newOwner := testDB.CreateTestUser(t, "newowner", "newowner@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(newOwner.ID), "player")
	require.NoError(t, err)

	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	t.Run("GM can reassign an inactive character", func(t *testing.T) {
		char, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
			GameID:        game.ID,
			UserID:        int32Ptr(int32(player.ID)),
			Name:          "Inactive Char",
			CharacterType: "player_character",
		})
		require.NoError(t, err)

		// Deactivate the character so it can be reassigned
		err = characterService.DeactivatePlayerCharacters(context.Background(), game.ID, int32(player.ID))
		require.NoError(t, err)

		body := ReassignCharacterRequest{NewOwnerUserID: int32(newOwner.ID)}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/characters/%d/reassign", char.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, float64(newOwner.ID), response["user_id"])
	})

	t.Run("GM cannot reassign an active character", func(t *testing.T) {
		char, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
			GameID:        game.ID,
			UserID:        int32Ptr(int32(player.ID)),
			Name:          "Active Char",
			CharacterType: "player_character",
		})
		require.NoError(t, err)

		body := ReassignCharacterRequest{NewOwnerUserID: int32(newOwner.ID)}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/characters/%d/reassign", char.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusConflict, rec.Code)
	})

	t.Run("non-GM player cannot reassign", func(t *testing.T) {
		char, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
			GameID:        game.ID,
			UserID:        int32Ptr(int32(player.ID)),
			Name:          "Another Char",
			CharacterType: "player_character",
		})
		require.NoError(t, err)

		body := ReassignCharacterRequest{NewOwnerUserID: int32(newOwner.ID)}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/characters/%d/reassign", char.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestCharacterAPI_ListInactiveCharacters tests GET /api/v1/games/{gameId}/characters/inactive
func TestCharacterAPI_ListInactiveCharacters(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupCharacterManagementTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	char, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player.ID)),
		Name:          "To Be Deactivated",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	err = characterService.DeactivatePlayerCharacters(context.Background(), game.ID, int32(player.ID))
	require.NoError(t, err)

	listURL := fmt.Sprintf("/api/v1/games/%d/characters/inactive", game.ID)

	t.Run("GM can list inactive characters", func(t *testing.T) {
		req := httptest.NewRequest("GET", listURL, nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		require.Len(t, response, 1)
		assert.Equal(t, float64(char.ID), response[0]["id"])
		assert.Equal(t, false, response[0]["is_active"])
	})

	t.Run("non-GM player cannot list inactive characters", func(t *testing.T) {
		req := httptest.NewRequest("GET", listURL, nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestCharacterAPI_SetCharacterData tests POST /api/v1/characters/{id}/data
func TestCharacterAPI_SetCharacterData(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupCharacterManagementTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	other := testDB.CreateTestUser(t, "other", "other@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)
	otherToken, err := core.CreateTestJWTTokenForUser(app, other)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(other.ID), "player")
	require.NoError(t, err)

	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	playerChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player.ID)),
		Name:          "Player Character",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	dataURL := fmt.Sprintf("/api/v1/characters/%d/data", playerChar.ID)

	t.Run("character owner can set non-stat data", func(t *testing.T) {
		body := CharacterDataRequest{
			ModuleType: "biography",
			FieldName:  "backstory",
			FieldValue: "A long backstory.",
			FieldType:  "text",
			IsPublic:   true,
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", dataURL, bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("non-owner cannot set character data", func(t *testing.T) {
		body := CharacterDataRequest{
			ModuleType: "biography",
			FieldName:  "backstory",
			FieldValue: "Sneaky edit.",
			FieldType:  "text",
			IsPublic:   true,
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", dataURL, bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+otherToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("player cannot set stat fields (abilities)", func(t *testing.T) {
		body := CharacterDataRequest{
			ModuleType: "abilities",
			FieldName:  "abilities",
			FieldValue: `{"strength": 20}`,
			FieldType:  "json",
			IsPublic:   false,
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", dataURL, bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("GM can set stat fields", func(t *testing.T) {
		body := CharacterDataRequest{
			ModuleType: "abilities",
			FieldName:  "abilities",
			FieldValue: `{"strength": 18}`,
			FieldType:  "json",
			IsPublic:   false,
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", dataURL, bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}

// TestCharacterAPI_ApproveCharacter_SendsNotification verifies that approving a character
// creates an in-app notification for the character owner. This guards against the
// notification goroutine being accidentally removed from ApproveCharacter.
func TestCharacterAPI_ApproveCharacter_SendsNotification(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "notifications", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupCharacterManagementTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	char, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player.ID)),
		Name:          "Hero",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	body := ApproveCharacterRequest{Status: "approved"}
	bodyJSON, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/characters/%d/approve", char.ID), bytes.NewBuffer(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	// Allow the notification goroutine to complete
	time.Sleep(200 * time.Millisecond)

	notifSvc := &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}
	notifs, err := notifSvc.GetUserNotifications(context.Background(), int32(player.ID), 10, 0)
	require.NoError(t, err)

	var found bool
	for _, n := range notifs {
		if n.Type == core.NotificationTypeCharacterApproved {
			assert.Contains(t, n.Title, "Hero")
			assert.Contains(t, n.Title, "published")
			found = true
			break
		}
	}
	assert.True(t, found, "player should receive a character_approved notification after GM approves")
}
