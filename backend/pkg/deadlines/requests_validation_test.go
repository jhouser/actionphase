package deadlines

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"actionphase/pkg/core"
	db "actionphase/pkg/db/services"
)

// TestDeadlineRequestValidation covers the `validate` struct tags in
// requests.go, which are executed by core.ValidateStruct from each Bind.
//
// Both request types require a title and a deadline; description is optional
// and the frontend sends it as "" when the GM leaves it blank, so that case is
// pinned down here alongside the rejections.
func TestDeadlineRequestValidation(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_deadlines", "games", "sessions", "users")

	router := setupDeadlineTestRouter(app, testDB)

	gmUser := testDB.CreateTestUser(t, "valdeadlinegm", "valdeadlinegm@example.com")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Deadline Validation Game",
		Description: "Testing deadline request validation",
		GMUserID:    int32(gmUser.ID),
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Game creation should succeed")

	accessToken, err := core.CreateTestJWTTokenForUser(app, gmUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	path := fmt.Sprintf("/api/v1/games/%d/deadlines", game.ID)

	post := func(t *testing.T, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+accessToken)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	future := time.Now().Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339)

	t.Run("rejects a missing title", func(t *testing.T) {
		w := post(t, fmt.Sprintf(`{"description": "d", "deadline": %q}`, future))

		core.AssertEqual(t, http.StatusBadRequest, w.Code, "Missing title should be rejected")
		if !bytes.Contains(w.Body.Bytes(), []byte("title is required")) {
			t.Errorf("expected 'title is required' in body, got %q", w.Body.String())
		}
	})

	t.Run("rejects a whitespace-only title", func(t *testing.T) {
		w := post(t, fmt.Sprintf(`{"title": "   ", "description": "d", "deadline": %q}`, future))

		core.AssertEqual(t, http.StatusBadRequest, w.Code, "Whitespace-only title should be rejected")
	})

	t.Run("rejects a missing deadline", func(t *testing.T) {
		// A zero time.Time fails `required` because the validator is built with
		// WithRequiredStructEnabled, which makes it descend into struct fields.
		w := post(t, `{"title": "No date", "description": "d"}`)

		core.AssertEqual(t, http.StatusBadRequest, w.Code, "Missing deadline should be rejected")
		if !bytes.Contains(w.Body.Bytes(), []byte("deadline is required")) {
			t.Errorf("expected 'deadline is required' in body, got %q", w.Body.String())
		}
	})

	t.Run("accepts a deadline with an empty description", func(t *testing.T) {
		// description carries no `required` tag; the frontend sends "" when the
		// GM leaves the field blank.
		w := post(t, fmt.Sprintf(`{"title": "Valid deadline", "description": "", "deadline": %q}`, future))

		core.AssertEqual(t, http.StatusCreated, w.Code, "Valid deadline should be created")
	})
}
