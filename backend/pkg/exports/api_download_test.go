package exports

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"actionphase/pkg/core"
	db "actionphase/pkg/db/services"
	"actionphase/pkg/humaconfig"
)

func setupDownloadRouter(t *testing.T, app *core.App, testDB *core.TestDatabase, svc *Service) *chi.Mux {
	t.Helper()
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &db.UserService{DB: testDB.Pool, Logger: app.ObsLogger}

	r := chi.NewRouter()
	// Mirrors production: the download route is registered on the /exports
	// router, so the huma path is /{exportID}/download relative to it.
	r.Route("/api/v1/exports", func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(jwtauth.Authenticator(tokenAuth))
		r.Use(core.RequireAuthenticationMiddleware(userService))

		h := &Handler{
			App:           app,
			UserService:   userService,
			GameService:   &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger},
			ExportService: svc,
		}
		RegisterHumaExportDownloads(humaconfig.New(r, "ActionPhase API", "1.0.0"), h)
	})
	return r
}

// seedCompletedExport creates a completed game with a finished archive on disk
// and returns the export id plus the game title it should be named after.
func seedCompletedExport(t *testing.T, testDB *core.TestDatabase, svc *Service, title string) (int32, int32) {
	t.Helper()
	ctx := context.Background()

	gm := testDB.CreateTestUser(t, "exp_dl_gm_"+title, "exp_dl_gm_"+title+"@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), title)
	_, err := testDB.Pool.Exec(ctx, `UPDATE games SET state='completed' WHERE id=$1`, game.ID)
	require.NoError(t, err)

	job, err := svc.RequestExport(ctx, game.ID, int32(gm.ID))
	require.NoError(t, err)
	_, err = svc.RunNextJob(ctx)
	require.NoError(t, err)

	done, err := svc.GetExport(ctx, job.ID)
	require.NoError(t, err)
	require.Equal(t, "complete", done.Status, "fixture export must complete")

	return job.ID, game.ID
}

func TestDownloadExport_ServesNamedArchive(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_exports", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	svc := NewService(testDB.Pool, t.TempDir(), app.ObsLogger)
	router := setupDownloadRouter(t, app, testDB, svc)

	exportID, _ := seedCompletedExport(t, testDB, svc, "Shadows Over Innsmouth")

	user := testDB.CreateTestUser(t, "exp_dl_reader", "exp_dl_reader@example.com")
	token, _ := core.CreateTestJWTTokenForUser(app, user)

	req := httptest.NewRequest("GET", "/api/v1/exports/"+strconv.Itoa(int(exportID))+"/download", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	// The filename is the whole point of streaming rather than redirecting.
	assert.Equal(t, `attachment; filename="shadows-over-innsmouth-archive.zip"`,
		rec.Header().Get("Content-Disposition"))
	assert.Equal(t, "application/zip", rec.Header().Get("Content-Type"))
	assert.Equal(t, "private, no-store", rec.Header().Get("Cache-Control"),
		"archives must never be held by a shared cache")

	// Body must be a readable archive, not a redirect page or an error.
	body := rec.Body.Bytes()
	assert.Equal(t, strconv.Itoa(len(body)), rec.Header().Get("Content-Length"))
	_, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	require.NoError(t, err, "response body must be a valid ZIP")
}

// Verified by mutation to be enforced by the handler itself (authorizeGameRead
// → GetUserIDFromJWT), not only by router middleware: stripping both
// jwtauth.Authenticator and RequireAuthenticationMiddleware still yields 401.
func TestDownloadExport_RequiresAuthentication(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_exports", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	svc := NewService(testDB.Pool, t.TempDir(), app.ObsLogger)
	router := setupDownloadRouter(t, app, testDB, svc)

	exportID, _ := seedCompletedExport(t, testDB, svc, "Unauthenticated Probe")

	req := httptest.NewRequest("GET", "/api/v1/exports/"+strconv.Itoa(int(exportID))+"/download", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code,
		"archives must not be retrievable without a token")
	assert.NotContains(t, rec.Body.String(), "PK", "no archive bytes may leak")
}

// A swept archive leaves a row that still reports 'complete'. The handler must
// distinguish that from a server fault so the UI can offer to regenerate.
func TestDownloadExport_ExpiredArchiveConflicts(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_exports", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	svc := NewService(testDB.Pool, t.TempDir(), app.ObsLogger)
	router := setupDownloadRouter(t, app, testDB, svc)

	exportID, _ := seedCompletedExport(t, testDB, svc, "Swept Game")

	_, err := testDB.Pool.Exec(context.Background(),
		`UPDATE game_exports SET storage_path = NULL WHERE id = $1`, exportID)
	require.NoError(t, err)

	user := testDB.CreateTestUser(t, "exp_dl_exp", "exp_dl_exp@example.com")
	token, _ := core.CreateTestJWTTokenForUser(app, user)

	req := httptest.NewRequest("GET", "/api/v1/exports/"+strconv.Itoa(int(exportID))+"/download", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "expired")
}

// The row can outlive its file if the volume is replaced. That is recoverable
// by regenerating, so it must not surface as a 500.
func TestDownloadExport_MissingFileConflicts(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_exports", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	svc := NewService(testDB.Pool, t.TempDir(), app.ObsLogger)
	router := setupDownloadRouter(t, app, testDB, svc)

	exportID, _ := seedCompletedExport(t, testDB, svc, "Vanished Volume")

	// Row still points at a path, but the artifact is gone from disk.
	done, err := svc.GetExport(context.Background(), exportID)
	require.NoError(t, err)
	require.NoError(t, svc.Store.Delete(context.Background(), done.StoragePath.String))

	user := testDB.CreateTestUser(t, "exp_dl_gone", "exp_dl_gone@example.com")
	token, _ := core.CreateTestJWTTokenForUser(app, user)

	req := httptest.NewRequest("GET", "/api/v1/exports/"+strconv.Itoa(int(exportID))+"/download", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code,
		"a missing artifact is recoverable by regenerating, not a server error")
	assert.Contains(t, rec.Body.String(), "no longer available")
}
