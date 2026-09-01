package games

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"actionphase/pkg/core"
	db "actionphase/pkg/db/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGameAPI_PromoteToCoGM tests POST /api/v1/games/{id}/participants/{userId}/promote-to-co-gm
func TestGameAPI_PromoteToCoGM(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	audienceMember := testDB.CreateTestUser(t, "audience", "audience@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(audienceMember.ID), "audience")
	require.NoError(t, err)

	t.Run("GM promotes audience member to co-GM", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/participants/%d/promote-to-co-gm", game.ID, audienceMember.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)

		// Verify the participant's role changed to co_gm
		participants, err := gameService.GetGameParticipants(context.Background(), game.ID)
		require.NoError(t, err)
		var audienceRole string
		for _, p := range participants {
			if p.UserID == int32(audienceMember.ID) {
				audienceRole = p.Role
			}
		}
		assert.Equal(t, "co_gm", audienceRole)
	})

	t.Run("non-GM player cannot promote to co-GM", func(t *testing.T) {
		// Add another audience member so there's someone to promote
		otherAudience := testDB.CreateTestUser(t, "audience2", "audience2@example.com")
		_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(otherAudience.ID), "audience")
		require.NoError(t, err)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/participants/%d/promote-to-co-gm", game.ID, otherAudience.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("cannot promote player (non-audience) to co-GM", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/participants/%d/promote-to-co-gm", game.ID, player.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Service returns validation error which maps to 400
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestGameAPI_DemoteFromCoGM tests POST /api/v1/games/{id}/participants/{userId}/demote-from-co-gm
func TestGameAPI_DemoteFromCoGM(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	coGMCandidate := testDB.CreateTestUser(t, "cogm", "cogm@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(coGMCandidate.ID), "audience")
	require.NoError(t, err)

	// Promote coGMCandidate to co-GM first
	err = gameService.PromoteToCoGM(context.Background(), game.ID, int32(coGMCandidate.ID), int32(gm.ID))
	require.NoError(t, err)

	t.Run("GM demotes co-GM back to audience", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/participants/%d/demote-from-co-gm", game.ID, coGMCandidate.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)

		// Verify the participant's role changed back to audience
		participants, err := gameService.GetGameParticipants(context.Background(), game.ID)
		require.NoError(t, err)
		var coGMRole string
		for _, p := range participants {
			if p.UserID == int32(coGMCandidate.ID) {
				coGMRole = p.Role
			}
		}
		assert.Equal(t, "audience", coGMRole)
	})

	t.Run("non-GM player cannot demote co-GM", func(t *testing.T) {
		// Re-promote so there's a co-GM to demote
		otherAudience := testDB.CreateTestUser(t, "audience3", "audience3@example.com")
		_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(otherAudience.ID), "audience")
		require.NoError(t, err)
		err = gameService.PromoteToCoGM(context.Background(), game.ID, int32(otherAudience.ID), int32(gm.ID))
		require.NoError(t, err)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/participants/%d/demote-from-co-gm", game.ID, otherAudience.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestGameAPI_RemovePlayer_AccessControl verifies that non-GMs cannot remove players
// and that the GM cannot remove themselves.
func TestGameAPI_RemovePlayer_AccessControl(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	t.Run("non-GM player cannot remove another player", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/games/%d/participants/%d", game.ID, gm.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("GM cannot remove themselves from the game", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/games/%d/participants/%d", game.ID, gm.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		// GM removing themselves returns 409 Conflict
		assert.Equal(t, http.StatusConflict, rec.Code)
	})
}

// TestGameAPI_LeaveGame_WithPendingApplication tests that a user with only a pending
// application (not a participant) can withdraw by calling leave — the application gets deleted.
func TestGameAPI_LeaveGame_WithPendingApplication(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_applications", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	applicant := testDB.CreateTestUser(t, "applicant", "applicant@example.com")

	applicantToken, err := core.CreateTestJWTTokenForUser(app, applicant)
	require.NoError(t, err)

	// Create a recruiting game so the application service allows applying
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	testDB.SetGameStateDirectly(t, game.ID, "recruitment")

	// Create a pending application directly via service
	appService := &db.GameApplicationService{DB: testDB.Pool}
	application, err := appService.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
		GameID:  game.ID,
		UserID:  int32(applicant.ID),
		Role:    "player",
		Message: "I'd like to join",
	})
	require.NoError(t, err)
	require.NotNil(t, application)

	// Verify application exists
	var countBefore int
	err = testDB.Pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM game_applications WHERE game_id = $1 AND user_id = $2", game.ID, applicant.ID,
	).Scan(&countBefore)
	require.NoError(t, err)
	assert.Equal(t, 1, countBefore)

	// Leave the game (applicant, not participant)
	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/games/%d/leave", game.ID), nil)
	req.Header.Set("Authorization", "Bearer "+applicantToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)

	// Verify application was deleted
	var countAfter int
	err = testDB.Pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM game_applications WHERE game_id = $1 AND user_id = $2", game.ID, applicant.ID,
	).Scan(&countAfter)
	require.NoError(t, err)
	assert.Equal(t, 0, countAfter, "pending application should be deleted after leaving")
}

// TestGameAPI_TransitionPlayerToAudience tests POST /api/v1/games/{id}/participants/{userId}/to-audience
func TestGameAPI_TransitionPlayerToAudience(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	audienceMember := testDB.CreateTestUser(t, "audience", "audience@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(audienceMember.ID), "audience")
	require.NoError(t, err)

	t.Run("GM transitions player to audience", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/participants/%d/to-audience", game.ID, player.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)

		// Verify the participant's role changed to audience
		participants, err := gameService.GetGameParticipants(context.Background(), game.ID)
		require.NoError(t, err)
		var playerRole string
		for _, p := range participants {
			if p.UserID == int32(player.ID) {
				playerRole = p.Role
			}
		}
		assert.Equal(t, "audience", playerRole)
	})

	t.Run("is_former_player is set to true after transition", func(t *testing.T) {
		// Verify DB flag and that it is included in the participants API response
		participants, err := gameService.GetGameParticipants(context.Background(), game.ID)
		require.NoError(t, err)
		var found bool
		for _, p := range participants {
			if p.UserID == int32(player.ID) {
				found = true
				assert.True(t, p.IsFormerPlayer, "is_former_player should be true after transition")
			}
		}
		assert.True(t, found, "transitioned player should still appear in participants list")

		// Verify the API response JSON includes is_former_player
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/participants", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		var playerEntry map[string]interface{}
		for _, p := range resp {
			if int(p["user_id"].(float64)) == player.ID {
				playerEntry = p
			}
		}
		require.NotNil(t, playerEntry, "player should appear in API response")
		assert.Equal(t, true, playerEntry["is_former_player"])
	})

	t.Run("characters are NOT deactivated after transition", func(t *testing.T) {
		// Use a fresh player so we have a known starting state
		anotherPlayer := testDB.CreateTestUser(t, "player2", "player2@example.com")
		_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(anotherPlayer.ID), "player")
		require.NoError(t, err)

		var charID int32
		err = testDB.Pool.QueryRow(context.Background(),
			`INSERT INTO characters (game_id, user_id, name, character_type, status, is_active)
			 VALUES ($1, $2, 'Test Char', 'player_character', 'approved', true)
			 RETURNING id`,
			game.ID, anotherPlayer.ID,
		).Scan(&charID)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/participants/%d/to-audience", game.ID, anotherPlayer.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)

		var isActive bool
		err = testDB.Pool.QueryRow(context.Background(),
			"SELECT is_active FROM characters WHERE id = $1", charID,
		).Scan(&isActive)
		require.NoError(t, err)
		assert.True(t, isActive, "character should remain active after transitioning player to audience")
	})

	t.Run("non-GM player cannot transition another player to audience", func(t *testing.T) {
		yetAnotherPlayer := testDB.CreateTestUser(t, "player3", "player3@example.com")
		_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(yetAnotherPlayer.ID), "player")
		require.NoError(t, err)

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/participants/%d/to-audience", game.ID, yetAnotherPlayer.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("cannot transition audience member (non-player) to audience", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/participants/%d/to-audience", game.ID, audienceMember.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestGameAPI_GetGameParticipants_AnonymousRedaction verifies that is_former_player
// is redacted for regular players viewing an anonymous game's participant list.
func TestGameAPI_GetGameParticipants_AnonymousRedaction(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm_anon", "gm_anon@example.com")
	player := testDB.CreateTestUser(t, "player_anon", "player_anon@example.com")
	formerPlayer := testDB.CreateTestUser(t, "former_anon", "former_anon@example.com")
	audienceMember := testDB.CreateTestUser(t, "audience_anon", "audience_anon@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)
	audienceToken, err := core.CreateTestJWTTokenForUser(app, audienceMember)
	require.NoError(t, err)

	community := testDB.CreateTestCommunity(t, int32(gm.ID))

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Anonymous Game",
		Description: "Test",
		GMUserID:    int32(gm.ID),
		CommunityID: community.ID,
		IsAnonymous: true,
	})
	require.NoError(t, err)

	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(formerPlayer.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(audienceMember.ID), "audience")
	require.NoError(t, err)

	// Transition formerPlayer to audience (sets is_former_player = true)
	err = gameService.TransitionPlayerToAudience(context.Background(), game.ID, int32(formerPlayer.ID), int32(gm.ID))
	require.NoError(t, err)

	getParticipants := func(token string) []map[string]interface{} {
		t.Helper()
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/participants", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		return resp
	}

	formerPlayerEntry := func(resp []map[string]interface{}) map[string]interface{} {
		for _, p := range resp {
			if int(p["user_id"].(float64)) == formerPlayer.ID {
				return p
			}
		}
		return nil
	}

	t.Run("regular player sees former player as a regular player", func(t *testing.T) {
		resp := getParticipants(playerToken)
		entry := formerPlayerEntry(resp)
		require.NotNil(t, entry, "former player should still appear in the participant list")
		assert.Equal(t, "player", entry["role"], "role should be spoofed to player to hide transition")
		assert.Equal(t, false, entry["is_former_player"], "is_former_player should be false to prevent network inspection revealing identity")
	})

	t.Run("GM sees is_former_player as true", func(t *testing.T) {
		resp := getParticipants(gmToken)
		entry := formerPlayerEntry(resp)
		require.NotNil(t, entry)
		assert.Equal(t, true, entry["is_former_player"])
	})

	t.Run("audience member sees is_former_player as true", func(t *testing.T) {
		resp := getParticipants(audienceToken)
		entry := formerPlayerEntry(resp)
		require.NotNil(t, entry)
		assert.Equal(t, true, entry["is_former_player"])
	})
}

// TestGameAPI_LeaveGame_NotAssociated documents the current behavior when a user
// who has no participant record AND no application tries to leave a game.
// Note: LeaveGame uses UPDATE...WHERE which silently affects zero rows, so the
// 404 branch only fires when GetGameApplicationByUserAndGame also returns an error.
// Currently, when RemovePlayer's underlying UPDATE succeeds (zero rows affected),
// participantRemoved=true and the handler returns 204 even for true outsiders.
// This test documents the actual behavior.
func TestGameAPI_LeaveGame_NotAssociated(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	outsider := testDB.CreateTestUser(t, "outsider", "outsider@example.com")

	outsiderToken, err := core.CreateTestJWTTokenForUser(app, outsider)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/games/%d/leave", game.ID), nil)
	req.Header.Set("Authorization", "Bearer "+outsiderToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Current behavior: RemovePlayer (UPDATE WHERE) affects 0 rows without error,
	// so participantRemoved=true and the handler returns 204. The 404 branch
	// requires both the participant removal AND application lookup to fail.
	assert.Equal(t, http.StatusNoContent, rec.Code)
}
