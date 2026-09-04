package characters

import (
	"bytes"
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

// TestCharacterRequestValidation covers the request rules in requests.go: the
// schema tags huma enforces before the handler runs, plus the trimming each
// body does in Resolve.
//
// These pin down both halves: that malformed payloads are rejected with a 400
// rather than falling through to a service and surfacing as a 500, and that the
// payloads the frontend actually sends still pass.
//
// The assertions name the offending field rather than quoting the message,
// because the two layers phrase it differently -- huma's schema says "expected
// required property name to be present", Resolve says "name is required" -- and
// pinning the phrasing would break on a huma upgrade with no behaviour change.
func TestCharacterRequestValidation(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupCharacterManagementTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "valgm", "valgm@example.com")
	player := testDB.CreateTestUser(t, "valplayer", "valplayer@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Validation Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	character, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player.ID)),
		Name:          "Validation Target",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	send := func(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	renamePath := fmt.Sprintf("/api/v1/characters/%d/rename", character.ID)
	dataPath := fmt.Sprintf("/api/v1/characters/%d/data", character.ID)
	approvePath := fmt.Sprintf("/api/v1/characters/%d/approve", character.ID)
	assignPath := fmt.Sprintf("/api/v1/characters/%d/assign", character.ID)
	reassignPath := fmt.Sprintf("/api/v1/characters/%d/reassign", character.ID)

	t.Run("rejects rename with missing name", func(t *testing.T) {
		rec := send(t, http.MethodPut, renamePath, `{}`)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Contains(t, rec.Body.String(), "name")
	})

	t.Run("rejects rename with whitespace-only name", func(t *testing.T) {
		// The regression this closes: a blank name satisfied the stock
		// `required` tag, reached RenameCharacter in the service, and its
		// "character name cannot be empty" error rendered as a 500 telling the
		// user the server had broken.
		rec := send(t, http.MethodPut, renamePath, `{"name": "   "}`)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Contains(t, rec.Body.String(), "name")
	})

	t.Run("rejects rename with over-length name", func(t *testing.T) {
		longName := make([]byte, 256)
		for i := range longName {
			longName[i] = 'a'
		}
		rec := send(t, http.MethodPut, renamePath, fmt.Sprintf(`{"name": %q}`, longName))

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Contains(t, rec.Body.String(), "255")
	})

	t.Run("accepts rename and trims surrounding whitespace", func(t *testing.T) {
		rec := send(t, http.MethodPut, renamePath, `{"name": "  Trimmed Name  "}`)

		require.Equal(t, http.StatusOK, rec.Code)
		var response map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Equal(t, "Trimmed Name", response["name"])
	})

	t.Run("rejects character data with missing module_type", func(t *testing.T) {
		rec := send(t, http.MethodPost, dataPath,
			`{"field_name": "strength", "field_value": "10", "field_type": "number", "is_public": true}`)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Contains(t, rec.Body.String(), "module_type")
	})

	t.Run("accepts character data with an empty field_value", func(t *testing.T) {
		// field_value carries no `required` tag on purpose: clearing a sheet
		// field is a normal edit, and the frontend sends "" to do it.
		rec := send(t, http.MethodPost, dataPath,
			`{"module_type": "basic", "field_name": "notes", "field_value": "", "field_type": "text", "is_public": false}`)

		assert.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("rejects approve with missing status", func(t *testing.T) {
		rec := send(t, http.MethodPost, approvePath, `{}`)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Contains(t, rec.Body.String(), "status")
	})

	t.Run("rejects assign with missing assigned_user_id", func(t *testing.T) {
		rec := send(t, http.MethodPost, assignPath, `{}`)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Contains(t, rec.Body.String(), "assigned_user_id")
	})

	t.Run("rejects reassign with missing new_owner_user_id", func(t *testing.T) {
		rec := send(t, http.MethodPut, reassignPath, `{}`)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Contains(t, rec.Body.String(), "new_owner_user_id")
	})
}
