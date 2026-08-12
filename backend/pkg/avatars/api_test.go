package avatars

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"actionphase/pkg/core"
	dbmodels "actionphase/pkg/db/models"
	dbsvc "actionphase/pkg/db/services"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testStorage is a minimal in-memory storage backend for handler tests.
type testStorage struct {
	uploads map[string]string
}

func newTestStorage() *testStorage {
	return &testStorage{uploads: make(map[string]string)}
}

func (s *testStorage) Upload(_ context.Context, path string, _ io.Reader, _ string) (string, error) {
	s.uploads[path] = path
	return "http://test.storage/" + path, nil
}

func (s *testStorage) Delete(_ context.Context, path string) error {
	delete(s.uploads, path)
	return nil
}

func (s *testStorage) GetURL(path string) string {
	return "http://test.storage/" + path
}

func setupAvatarTestRouter(app *core.App, testDB *core.TestDatabase) *chi.Mux {
	// Avatar handler uses app.DB; Pool and DB are separate fields in App
	app.DB = testDB.Pool

	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &dbsvc.UserService{DB: testDB.Pool, Logger: app.ObsLogger}

	r := chi.NewRouter()
	r.Route("/api/v1/characters/{id}", func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(jwtauth.Authenticator(tokenAuth))
		r.Use(core.RequireAuthenticationMiddleware(userService))

		handler := &Handler{
			App:              app,
			CharacterService: &dbsvc.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger},
		}
		r.Post("/avatar", handler.UploadCharacterAvatar)
		r.Delete("/avatar", handler.DeleteCharacterAvatar)
	})

	return r
}

// buildAvatarUpload creates a multipart request body with the given file content and content-type.
// The Content-Type header is set on the part so the handler can detect it.
func buildAvatarUpload(t *testing.T, filename, contentType string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="avatar"; filename="%s"`, filename))
	h.Set("Content-Type", contentType)
	part, err := mw.CreatePart(h)
	require.NoError(t, err)
	_, err = io.Copy(part, bytes.NewReader(content))
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	return &buf, mw.FormDataContentType()
}

// smallJPEG returns a minimal valid JPEG-sized byte slice for testing.
func smallJPEG() []byte {
	return []byte(strings.Repeat("x", 1024)) // 1KB placeholder; service validates MIME from header, not magic bytes
}

func setupAvatarTestData(t *testing.T, testDB *core.TestDatabase, app *core.App) (owner *core.User, otherUser *core.User, char dbmodels.Character, ownerToken string, otherToken string) {
	t.Helper()

	owner = testDB.CreateTestUser(t, "avatar_owner", "avatar_owner@example.com")
	otherUser = testDB.CreateTestUser(t, "avatar_other", "avatar_other@example.com")
	game := testDB.CreateTestGame(t, int32(owner.ID), "Avatar Test Game")

	// Add owner as participant
	gameService := &dbsvc.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(owner.ID), "player")
	require.NoError(t, err)

	// Create character owned by owner
	queries := dbmodels.New(testDB.Pool)
	char, err = queries.CreateCharacter(context.Background(), dbmodels.CreateCharacterParams{
		GameID:        game.ID,
		Name:          "Test Character",
		CharacterType: "player_character",
		UserID:        pgtype.Int4{Int32: int32(owner.ID), Valid: true},
		Status:        pgtype.Text{String: "approved", Valid: true},
	})
	require.NoError(t, err)

	ownerToken, err = core.CreateTestJWTTokenForUser(app, owner)
	require.NoError(t, err)
	otherToken, err = core.CreateTestJWTTokenForUser(app, otherUser)
	require.NoError(t, err)

	return
}

func TestAvatarUpload_Success(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	storage := newTestStorage()
	app.Storage = storage
	router := setupAvatarTestRouter(app, testDB)

	owner, _, char, ownerToken, _ := setupAvatarTestData(t, testDB, app)
	_ = owner

	body, contentType := buildAvatarUpload(t, "avatar.jpg", "image/jpeg", smallJPEG())
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/characters/%d/avatar", char.ID), body)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", contentType)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp AvatarUploadResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.AvatarURL, "response must include avatar_url")

	// Verify DB updated
	queries := dbmodels.New(testDB.Pool)
	updated, err := queries.GetCharacter(context.Background(), char.ID)
	require.NoError(t, err)
	assert.True(t, updated.AvatarUrl.Valid)
	assert.Contains(t, updated.AvatarUrl.String, "avatars/characters/")
}

func TestAvatarUpload_NonOwner_403(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	app.Storage = newTestStorage()
	router := setupAvatarTestRouter(app, testDB)

	_, _, char, _, otherToken := setupAvatarTestData(t, testDB, app)

	body, contentType := buildAvatarUpload(t, "avatar.jpg", "image/jpeg", smallJPEG())
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/characters/%d/avatar", char.ID), body)
	req.Header.Set("Authorization", "Bearer "+otherToken)
	req.Header.Set("Content-Type", contentType)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestAvatarUpload_InvalidMIME_400(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	app.Storage = newTestStorage()
	router := setupAvatarTestRouter(app, testDB)

	_, _, char, ownerToken, _ := setupAvatarTestData(t, testDB, app)

	body, contentType := buildAvatarUpload(t, "malware.exe", "application/octet-stream", []byte("notanimage"))
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/characters/%d/avatar", char.ID), body)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", contentType)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAvatarUpload_FileTooLarge_400(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	app.Storage = newTestStorage()
	router := setupAvatarTestRouter(app, testDB)

	_, _, char, ownerToken, _ := setupAvatarTestData(t, testDB, app)

	// 6MB > MaxAvatarSize (5MB)
	largeContent := bytes.Repeat([]byte("x"), 6*1024*1024)
	body, contentType := buildAvatarUpload(t, "big.jpg", "image/jpeg", largeContent)
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/characters/%d/avatar", char.ID), body)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", contentType)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAvatarUpload_MissingField_400(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	app.Storage = newTestStorage()
	router := setupAvatarTestRouter(app, testDB)

	_, _, char, ownerToken, _ := setupAvatarTestData(t, testDB, app)

	// Multipart body with wrong field name (not "avatar")
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="photo"; filename="avatar.jpg"`)
	h.Set("Content-Type", "image/jpeg")
	part, err := mw.CreatePart(h) // wrong field name
	require.NoError(t, err)
	_, err = io.Copy(part, bytes.NewReader(smallJPEG()))
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/characters/%d/avatar", char.ID), &buf)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAvatarDelete_Success(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	storage := newTestStorage()
	app.Storage = storage
	router := setupAvatarTestRouter(app, testDB)

	_, _, char, ownerToken, _ := setupAvatarTestData(t, testDB, app)

	// Give the character an existing avatar URL
	queries := dbmodels.New(testDB.Pool)
	_, err := queries.UpdateCharacterAvatar(context.Background(), dbmodels.UpdateCharacterAvatarParams{
		ID:        char.ID,
		AvatarUrl: pgtype.Text{String: "http://test.storage/avatars/characters/1/old.jpg", Valid: true},
	})
	require.NoError(t, err)

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/characters/%d/avatar", char.ID), nil)
	req.Header.Set("Authorization", "Bearer "+ownerToken)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)

	// Verify DB cleared
	updated, err := queries.GetCharacter(context.Background(), char.ID)
	require.NoError(t, err)
	assert.False(t, updated.AvatarUrl.Valid || updated.AvatarUrl.String != "")
}

func TestAvatarDelete_NonOwner_403(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	app.Storage = newTestStorage()
	router := setupAvatarTestRouter(app, testDB)

	_, _, char, _, otherToken := setupAvatarTestData(t, testDB, app)

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/characters/%d/avatar", char.ID), nil)
	req.Header.Set("Authorization", "Bearer "+otherToken)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// Avatars are narrative content: messages pin the avatar URL they were authored
// with, so a superseded or removed avatar file must stay resolvable in storage.
// These tests guard the two ways a file used to get destroyed.

// TestAvatarUpload_RetainsPreviousFile covers replacement. Uploading a new avatar
// must not delete the old file, or every post that pinned it loses its image.
func TestAvatarUpload_RetainsPreviousFile(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	storage := newTestStorage()
	app.Storage = storage
	router := setupAvatarTestRouter(app, testDB)

	_, _, char, ownerToken, _ := setupAvatarTestData(t, testDB, app)

	// First upload establishes the "old" avatar.
	body, contentType := buildAvatarUpload(t, "first.jpg", "image/jpeg", smallJPEG())
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/characters/%d/avatar", char.ID), body)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	require.Len(t, storage.uploads, 1, "first upload should store one file")
	var firstPath string
	for p := range storage.uploads {
		firstPath = p
	}

	// Storage paths carry a random suffix (avatars/characters/{id}/{ms}_{rand}.jpg),
	// so back-to-back uploads land on distinct keys and "was the old file deleted?"
	// is observable without waiting on the clock.
	// Second upload supersedes it.
	body2, contentType2 := buildAvatarUpload(t, "second.jpg", "image/jpeg", smallJPEG())
	req2 := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/characters/%d/avatar", char.ID), body2)
	req2.Header.Set("Authorization", "Bearer "+ownerToken)
	req2.Header.Set("Content-Type", contentType2)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)

	assert.Contains(t, storage.uploads, firstPath,
		"the superseded avatar file must remain in storage so posts that pinned it still resolve")
	assert.Len(t, storage.uploads, 2, "both the old and new avatar files should exist")
}

// TestAvatarDelete_RetainsFile covers explicit removal. "Remove Avatar" clears the
// character's current avatar but must leave the file, so historical posts keep
// rendering what they were written with.
func TestAvatarDelete_RetainsFile(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	storage := newTestStorage()
	app.Storage = storage
	router := setupAvatarTestRouter(app, testDB)

	_, _, char, ownerToken, _ := setupAvatarTestData(t, testDB, app)

	body, contentType := buildAvatarUpload(t, "avatar.jpg", "image/jpeg", smallJPEG())
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/characters/%d/avatar", char.ID), body)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, storage.uploads, 1)

	var uploadedPath string
	for p := range storage.uploads {
		uploadedPath = p
	}

	delReq := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/characters/%d/avatar", char.ID), nil)
	delReq.Header.Set("Authorization", "Bearer "+ownerToken)
	delRec := httptest.NewRecorder()
	router.ServeHTTP(delRec, delReq)
	require.Equal(t, http.StatusNoContent, delRec.Code)

	// The character now renders as initials...
	queries := dbmodels.New(testDB.Pool)
	updated, err := queries.GetCharacter(context.Background(), char.ID)
	require.NoError(t, err)
	assert.False(t, updated.AvatarUrl.Valid && updated.AvatarUrl.String != "",
		"character's current avatar should be cleared")

	// ...but the file survives for messages that pinned it.
	assert.Contains(t, storage.uploads, uploadedPath,
		"removing an avatar must not delete the file: posts that pinned it would lose their image")
}
