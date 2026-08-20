package handouts

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"actionphase/pkg/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandoutRequestValidation covers the `validate` struct tags in
// requests.go, which are executed by core.ValidateStruct from each Bind.
//
// Content is the field worth pinning down: the create modal marks it required
// in its label but enforces nothing, because the editor is not a native input
// that the browser will block submit on. Before these tags ran, an empty
// handout was created silently.
func TestHandoutRequestValidation(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "handout_comments", "handouts", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupHandoutTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "valhandoutgm", "valhandoutgm@example.com")
	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Handout Validation Game")

	handoutsPath := fmt.Sprintf("/api/v1/games/%d/handouts", game.ID)

	send := func(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	t.Run("rejects a handout with empty content", func(t *testing.T) {
		rec := send(t, http.MethodPost, handoutsPath,
			`{"title": "Empty", "content": "", "status": "draft"}`)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "content is required")
	})

	t.Run("rejects a handout with whitespace-only content", func(t *testing.T) {
		rec := send(t, http.MethodPost, handoutsPath,
			`{"title": "Blank", "content": "   \n  ", "status": "draft"}`)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "content is required")
	})

	t.Run("rejects a handout with a missing title", func(t *testing.T) {
		rec := send(t, http.MethodPost, handoutsPath,
			`{"content": "Body text", "status": "draft"}`)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "title is required")
	})

	t.Run("rejects a status outside the allowed set", func(t *testing.T) {
		// oneof=draft published — the frontend's own union type matches, so an
		// "archived" here can only come from a non-web client.
		rec := send(t, http.MethodPost, handoutsPath,
			`{"title": "Bad status", "content": "Body text", "status": "archived"}`)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "status must be one of: draft, published")
	})

	t.Run("accepts both allowed statuses", func(t *testing.T) {
		for _, status := range []string{"draft", "published"} {
			rec := send(t, http.MethodPost, handoutsPath,
				fmt.Sprintf(`{"title": "Valid %s", "content": "Body text", "status": %q}`, status, status))

			assert.Equal(t, http.StatusCreated, rec.Code, "status %q should be accepted: %s", status, rec.Body.String())
		}
	})

	t.Run("rejects a comment with empty content", func(t *testing.T) {
		handoutID := createTestHandout(t, router, game.ID, gmToken, "published")
		commentsPath := fmt.Sprintf("%s/%d/comments", handoutsPath, handoutID)

		rec := send(t, http.MethodPost, commentsPath, `{"content": "  "}`)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "content is required")
	})
}
