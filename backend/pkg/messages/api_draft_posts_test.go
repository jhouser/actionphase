package messages

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	db "actionphase/pkg/db/services"
	messagesvc "actionphase/pkg/db/services/messages"
	"actionphase/pkg/humaconfig"
)

func setupDraftPostRouter(app *core.App, testDB *core.TestDatabase) *chi.Mux {
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger}

	r := chi.NewRouter()
	// Mounted at /phases: the huma operations carry the {id}/draft-post
	// segments themselves, matching production.
	r.Route("/api/v1/phases", func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(jwtauth.Authenticator(tokenAuth))
		r.Use(core.RequireAuthenticationMiddleware(userService))

		h := &Handler{
			App:            app,
			UserService:    &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
			MessageService: &messagesvc.MessageService{DB: testDB.Pool, Logger: app.ObsLogger, Metrics: app.Observability.OTELMetrics},
		}
		RegisterHumaPhaseDraftPosts(humaconfig.New(r, "ActionPhase API", "1.0.0"), h)
	})
	return r
}

func TestDraftPostAPI_CreateAndGet(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupDraftPostRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm_draft", "gm_draft@example.com")
	player := testDB.CreateTestUser(t, "player_draft", "player_draft@example.com")
	outsider := testDB.CreateTestUser(t, "outsider_draft", "outsider_draft@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Draft Post Test Game")
	phase := testDB.CreateTestPhase(t, game.ID, "common_room", "Test Phase")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	charService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	gmUserID := int32(gm.ID)
	char, err := charService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &gmUserID,
		Name:          "Narrator",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)
	outsiderToken, err := core.CreateTestJWTTokenForUser(app, outsider)
	require.NoError(t, err)

	phaseURL := "/api/v1/phases/" + strconv.Itoa(int(phase.ID)) + "/draft-post"

	t.Run("GM can create a draft post", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"character_id": char.ID,
			"content":      "The fog which surrounded you dissipates...",
		})
		req := httptest.NewRequest(http.MethodPost, phaseURL, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)

		var resp MessageResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, "The fog which surrounded you dissipates...", resp.Content)
		assert.True(t, resp.IsDraft, "created post should be a draft")
	})

	t.Run("GM can get the draft post", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, phaseURL, nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp MessageResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.True(t, resp.IsDraft)
		assert.Equal(t, "The fog which surrounded you dissipates...", resp.Content)
	})

	t.Run("player gets 403 on GET", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, phaseURL, nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("outsider gets 403 on POST", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"character_id": char.ID,
			"content":      "Sneaky post",
		})
		req := httptest.NewRequest(http.MethodPost, phaseURL, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+outsiderToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("duplicate create returns 409", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"character_id": char.ID,
			"content":      "Second draft",
		})
		req := httptest.NewRequest(http.MethodPost, phaseURL, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusConflict, rec.Code)
	})

	t.Run("unauthenticated gets 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, phaseURL, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestDraftPostAPI_UpdateAndDelete(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupDraftPostRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm_ud", "gm_ud@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Update Delete Test Game")
	phase := testDB.CreateTestPhase(t, game.ID, "common_room", "Test Phase")

	charService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	gmUserID := int32(gm.ID)
	char, err := charService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &gmUserID,
		Name:          "Narrator",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)

	// Seed a draft post
	msgService := &messagesvc.MessageService{DB: testDB.Pool, Logger: app.ObsLogger}
	phaseID := phase.ID
	_, err = msgService.CreateDraftPost(context.Background(), core.CreatePostRequest{
		GameID:      game.ID,
		PhaseID:     &phaseID,
		AuthorID:    int32(gm.ID),
		CharacterID: char.ID,
		Content:     "Original draft",
		Visibility:  string(models.MessageVisibilityGame),
	})
	require.NoError(t, err)

	phaseURL := "/api/v1/phases/" + strconv.Itoa(int(phase.ID)) + "/draft-post"

	t.Run("GM can update draft content", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"content": "Updated draft content",
		})
		req := httptest.NewRequest(http.MethodPut, phaseURL, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp MessageResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, "Updated draft content", resp.Content)
		assert.True(t, resp.IsDraft)
	})

	t.Run("GM can delete draft post", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, phaseURL, nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		// Verify it's gone — 200 with null body (no draft)
		getReq := httptest.NewRequest(http.MethodGet, phaseURL, nil)
		getReq.Header.Set("Authorization", "Bearer "+gmToken)
		getRec := httptest.NewRecorder()
		router.ServeHTTP(getRec, getReq)
		assert.Equal(t, http.StatusOK, getRec.Code)
		assert.Equal(t, "null\n", getRec.Body.String())
	})
}

func TestDraftPostAPI_NotVisibleInGamePosts(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)

	gm := testDB.CreateTestUser(t, "gm_vis", "gm_vis@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Visibility Test Game")
	phase := testDB.CreateTestPhase(t, game.ID, "common_room", "Test Phase")

	charService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	gmUserID := int32(gm.ID)
	char, err := charService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &gmUserID,
		Name:          "Narrator",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	phaseID := phase.ID
	msgService := &messagesvc.MessageService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = msgService.CreateDraftPost(context.Background(), core.CreatePostRequest{
		GameID:      game.ID,
		PhaseID:     &phaseID,
		AuthorID:    int32(gm.ID),
		CharacterID: char.ID,
		Content:     "Secret draft not visible to players",
		Visibility:  string(models.MessageVisibilityGame),
	})
	require.NoError(t, err)

	t.Run("draft post does not appear in GetGamePosts", func(t *testing.T) {
		posts, err := msgService.GetGamePosts(context.Background(), game.ID, &phaseID, 10, 0)
		require.NoError(t, err)
		assert.Empty(t, posts, "draft posts must not appear in game posts list")
	})
}
