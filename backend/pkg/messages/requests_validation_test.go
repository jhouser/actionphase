package messages

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
)

// TestMessageRequestValidation covers the request rules in requests.go: the
// schema tags huma enforces before the handler runs, plus the trimming each
// body does in Resolve.
//
// Whitespace-only content is the case the schema cannot catch: huma's minLength
// counts raw characters, so " " satisfies minLength:"1". Resolve trims first,
// matching what the update handler already did by hand and what the frontend
// composers send.
func TestMessageRequestValidation(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "characters", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupMessageTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	gmToken, err := createTestAuthToken(app, fixtures.TestUser)
	core.AssertNoError(t, err, "GM token creation should succeed")

	gameID := fixtures.TestGame.ID

	charService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	userID := int32(fixtures.TestUser.ID)
	character, err := charService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &userID,
		Name:          "Validation Voice",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Character creation should succeed")

	send := func(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	postsPath := fmt.Sprintf("/api/v1/games/%d/posts", gameID)

	t.Run("rejects post with whitespace-only content", func(t *testing.T) {
		body := fmt.Sprintf(`{"character_id": %d, "content": "   \n\t  "}`, character.ID)
		w := send(t, http.MethodPost, postsPath, body)

		core.AssertEqual(t, http.StatusBadRequest, w.Code, "Whitespace-only content should be rejected")
		if !bytes.Contains(w.Body.Bytes(), []byte("content is required")) {
			t.Errorf("expected 'content is required' in body, got %q", w.Body.String())
		}
	})

	t.Run("reports the JSON field name the client sent", func(t *testing.T) {
		w := send(t, http.MethodPost, postsPath, `{"content": "orphan post"}`)

		core.AssertEqual(t, http.StatusBadRequest, w.Code, "Missing character_id should be rejected")
		// The message names the offending field; the exact wording is huma's
		// ("expected required property character_id to be present") and is not
		// worth pinning, but the field name is the part clients read.
		if !bytes.Contains(w.Body.Bytes(), []byte("character_id")) {
			t.Errorf("expected 'character_id' named in body, got %q", w.Body.String())
		}
		// The Go field name must not leak to clients.
		if bytes.Contains(w.Body.Bytes(), []byte("CharacterID")) {
			t.Errorf("Go field name leaked into error body: %q", w.Body.String())
		}
	})

	t.Run("accepts a valid post and trims its content", func(t *testing.T) {
		body := fmt.Sprintf(`{"character_id": %d, "content": "  a padded post  "}`, character.ID)
		w := send(t, http.MethodPost, postsPath, body)

		core.AssertEqual(t, http.StatusCreated, w.Code, "Valid post should be created")
		if !bytes.Contains(w.Body.Bytes(), []byte(`"content":"a padded post"`)) {
			t.Errorf("expected trimmed content in response, got %q", w.Body.String())
		}
	})

	t.Run("rejects comment update with whitespace-only content", func(t *testing.T) {
		// Seed a post and a comment to edit.
		postBody := fmt.Sprintf(`{"character_id": %d, "content": "root post"}`, character.ID)
		postResp := send(t, http.MethodPost, postsPath, postBody)
		core.AssertEqual(t, http.StatusCreated, postResp.Code, "Seed post should be created")
		postID := decodeMessageID(t, postResp)

		commentPath := fmt.Sprintf("%s/%d/comments", postsPath, postID)
		commentBody := fmt.Sprintf(`{"character_id": %d, "content": "a comment"}`, character.ID)
		commentResp := send(t, http.MethodPost, commentPath, commentBody)
		core.AssertEqual(t, http.StatusCreated, commentResp.Code, "Seed comment should be created")
		commentID := decodeMessageID(t, commentResp)

		updatePath := fmt.Sprintf("%s/%d", commentPath, commentID)
		w := send(t, http.MethodPatch, updatePath, `{"content": "  "}`)

		core.AssertEqual(t, http.StatusBadRequest, w.Code, "Whitespace-only edit should be rejected")
		if !bytes.Contains(w.Body.Bytes(), []byte("content is required")) {
			t.Errorf("expected 'content is required' in body, got %q", w.Body.String())
		}
	})
}

// decodeMessageID pulls the id out of a MessageResponse body.
func decodeMessageID(t *testing.T, w *httptest.ResponseRecorder) int32 {
	t.Helper()
	var resp MessageResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode message response: %v (body: %s)", err, w.Body.String())
	}
	return resp.ID
}
