package exports

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	core "actionphase/pkg/core"
	models "actionphase/pkg/db/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memStorage keeps archives in memory so job runs need no filesystem, and can
// simulate write/delete faults that are awkward to provoke on real disk.
type memStorage struct {
	mu           sync.Mutex
	files        map[string][]byte
	failOn       string // path substring that should trigger a write error
	failDeleteOn string // exact path whose deletion should fail
}

var _ ArchiveStorer = (*memStorage)(nil)

func newMemStorage() *memStorage {
	return &memStorage{files: map[string][]byte{}}
}

func (m *memStorage) Put(_ context.Context, path string, r io.Reader) error {
	if m.failOn != "" && path != "" && contains(path, m.failOn) {
		return errors.New("simulated write failure")
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[path] = b
	return nil
}

func (m *memStorage) Open(_ context.Context, path string) (io.ReadCloser, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.files[path]
	if !ok {
		return nil, 0, ErrArchiveMissing
	}
	return io.NopCloser(bytes.NewReader(b)), int64(len(b)), nil
}

func (m *memStorage) Delete(_ context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failDeleteOn != "" && path == m.failDeleteOn {
		return errors.New("simulated delete failure")
	}
	delete(m.files, path)
	return nil
}

func (m *memStorage) get(path string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.files[path]
	return b, ok
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

// testPool returns a pool against this package's private, migrated test
// database clone. Connecting to TEST_DATABASE_URL directly would land on the
// shared base DB, which has no game_exports table.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if os.Getenv("SKIP_DB_TESTS") == "true" {
		t.Skip("SKIP_DB_TESTS=true")
	}
	db := core.NewTestDatabase(t)
	t.Cleanup(db.Close)
	return db.Pool
}

// seedCompletedGame creates a minimal completed game and returns its id.
func seedCompletedGame(t *testing.T, pool *pgxpool.Pool) int32 {
	t.Helper()
	ctx := context.Background()
	q := models.New(pool)

	suffix := time.Now().UnixNano()
	user, err := q.CreateUser(ctx, models.CreateUserParams{
		Username: "exp_gm_" + itoa(suffix),
		Email:    "exp_gm_" + itoa(suffix) + "@example.com",
		Password: "x",
	})
	require.NoError(t, err)

	game, err := q.CreateGame(ctx, models.CreateGameParams{
		Title:       "Export Integration Game",
		Description: pgtype.Text{String: "Integration fixture", Valid: true},
		GmUserID:    user.ID,
		IsPublic:    pgtype.Bool{Bool: true, Valid: true},
	})
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `UPDATE games SET state='completed' WHERE id=$1`, game.ID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM games WHERE id=$1`, game.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user.ID)
	})
	return game.ID
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// Regression: ListExportPhases must not filter on game_phases.is_published.
// That column means "GM published this ACTION phase's results" and is always
// FALSE for common_room and interlude phases, so filtering on it dropped every
// discussion phase and misfiled its content under "unfiled".
func TestService_ExportsUnpublishedPhases(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	gameID := seedCompletedGame(t, pool)

	// Mirror real data: phases that ran, none with is_published set.
	for i, spec := range []struct {
		num       int32
		phaseType string
		title     string
	}{
		{1, "common_room", "The Beginning"},
		{2, "action", "First Challenge"},
		{3, "interlude", "Quiet Hours"},
	} {
		_, err := pool.Exec(ctx,
			`INSERT INTO game_phases (game_id, phase_type, phase_number, title, start_time,
			                          is_published, activated_at)
			 VALUES ($1, $2, $3, $4, NOW(), FALSE, NOW())`,
			gameID, spec.phaseType, spec.num, spec.title)
		require.NoError(t, err, "seeding phase %d", i)
	}

	q := models.New(pool)
	phases, err := q.ListExportPhases(ctx, gameID)
	require.NoError(t, err)

	require.Len(t, phases, 3, "all phases must be exported regardless of is_published")

	titles := make([]string, 0, len(phases))
	for _, p := range phases {
		titles = append(titles, p.Title)
	}
	assert.Equal(t, []string{"The Beginning", "First Challenge", "Quiet Hours"}, titles,
		"phases must be returned in phase_number order")
}

// Content belonging to a real phase must land in that phase's directory, not
// in the unfiled bucket.
func TestService_ArchiveFilesContentUnderItsPhase(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	gameID := seedCompletedGame(t, pool)
	svc, store := newTestService(t, pool)

	var phaseID int32
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO game_phases (game_id, phase_type, phase_number, title, start_time,
		                          is_published, activated_at)
		 VALUES ($1, 'common_room', 1, 'The Beginning', NOW(), FALSE, NOW())
		 RETURNING id`, gameID).Scan(&phaseID))

	// A character and a post inside that phase.
	var gmID int32
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT gm_user_id FROM games WHERE id=$1`, gameID).Scan(&gmID))
	var charID int32
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO characters (game_id, user_id, name, character_type, status)
		 VALUES ($1, $2, 'The Narrator', 'npc', 'approved') RETURNING id`,
		gameID, gmID).Scan(&charID))
	_, err := pool.Exec(ctx,
		`INSERT INTO messages (game_id, phase_id, author_id, character_id, content,
		                       message_type, visibility)
		 VALUES ($1, $2, $3, $4, 'Opening remarks.', 'post', 'game')`,
		gameID, phaseID, gmID, charID)
	require.NoError(t, err)

	job, err := svc.RequestExport(ctx, gameID, 0)
	require.NoError(t, err)
	_, err = svc.RunNextJob(ctx)
	require.NoError(t, err)

	done, err := svc.GetExport(ctx, job.ID)
	require.NoError(t, err)
	require.Equal(t, "complete", done.Status)

	data, ok := store.get(done.StoragePath.String)
	require.True(t, ok)
	names := zipEntryNames(t, data)

	var posts, unfiled []string
	for _, n := range names {
		if strings.Contains(n, "/posts/") {
			posts = append(posts, n)
		}
		if strings.Contains(n, "00-unfiled") {
			unfiled = append(unfiled, n)
		}
	}

	require.Len(t, posts, 1, "the post must be archived")
	assert.Contains(t, posts[0], "phases/01-common-room-the-beginning/posts/",
		"post must be filed under its phase")
	assert.Empty(t, unfiled, "no content should fall through to unfiled")

	// The phase README must exist too, so the archive documents the phase.
	assert.Contains(t, names, gameDirPrefix(gameID)+"phases/01-common-room-the-beginning/README.md")
}

// Retention: an artifact past its window is deleted from storage, its row is
// kept as history with storage_path cleared, and the next request regenerates.
func TestService_SweepExpiredReclaimsArtifacts(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	gameID := seedCompletedGame(t, pool)
	svc, store := newTestService(t, pool)

	job, err := svc.RequestExport(ctx, gameID, 0)
	require.NoError(t, err)
	_, err = svc.RunNextJob(ctx)
	require.NoError(t, err)

	done, err := svc.GetExport(ctx, job.ID)
	require.NoError(t, err)
	require.Equal(t, "complete", done.Status)
	require.True(t, done.ExpiresAt.Valid, "a completed export must record an expiry")
	storagePath := done.StoragePath.String
	_, present := store.get(storagePath)
	require.True(t, present)

	// Not yet due: the sweep must leave it alone.
	swept, err := svc.SweepExpired(ctx)
	require.NoError(t, err)
	assert.Zero(t, swept, "an unexpired artifact must not be reclaimed")
	_, present = store.get(storagePath)
	assert.True(t, present)

	// Age it past the retention window.
	_, err = pool.Exec(ctx,
		`UPDATE game_exports SET expires_at = NOW() - interval '1 hour' WHERE id=$1`, job.ID)
	require.NoError(t, err)

	swept, err = svc.SweepExpired(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, swept)

	// File gone, row kept, path cleared.
	_, present = store.get(storagePath)
	assert.False(t, present, "artifact must be deleted from storage")

	expired, err := svc.GetExport(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, "complete", expired.Status, "row is retained as history")
	assert.False(t, expired.StoragePath.Valid, "storage path must be cleared")
	assert.True(t, expired.SizeBytes.Valid, "historical metadata is preserved")

	// Re-running the sweep is a no-op rather than an error.
	swept, err = svc.SweepExpired(ctx)
	require.NoError(t, err)
	assert.Zero(t, swept)
}

// After expiry a new request must rebuild rather than hand back the dead row.
func TestService_ExpiredExportRegeneratesOnRequest(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	gameID := seedCompletedGame(t, pool)
	svc, store := newTestService(t, pool)

	first, err := svc.RequestExport(ctx, gameID, 0)
	require.NoError(t, err)
	_, err = svc.RunNextJob(ctx)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`UPDATE game_exports SET expires_at = NOW() - interval '1 hour' WHERE id=$1`, first.ID)
	require.NoError(t, err)
	_, err = svc.SweepExpired(ctx)
	require.NoError(t, err)

	second, err := svc.RequestExport(ctx, gameID, 0)
	require.NoError(t, err)
	assert.NotEqual(t, first.ID, second.ID, "expired export must not be reused")
	assert.Equal(t, "pending", second.Status)

	_, err = svc.RunNextJob(ctx)
	require.NoError(t, err)

	rebuilt, err := svc.GetExport(ctx, second.ID)
	require.NoError(t, err)
	require.Equal(t, "complete", rebuilt.Status)
	_, present := store.get(rebuilt.StoragePath.String)
	assert.True(t, present, "regenerated artifact must be in storage")
}

// A storage failure on one artifact must not abort the whole sweep.
func TestService_SweepContinuesPastDeleteFailure(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	svc, store := newTestService(t, pool)

	var ids []int32
	for i := 0; i < 2; i++ {
		gameID := seedCompletedGame(t, pool)
		job, err := svc.RequestExport(ctx, gameID, 0)
		require.NoError(t, err)
		_, err = svc.RunNextJob(ctx)
		require.NoError(t, err)
		ids = append(ids, job.ID)
	}

	_, err := pool.Exec(ctx,
		`UPDATE game_exports SET expires_at = NOW() - interval '1 hour' WHERE id = ANY($1)`, ids)
	require.NoError(t, err)

	// Make the first artifact undeletable.
	first, err := svc.GetExport(ctx, ids[0])
	require.NoError(t, err)
	store.failDeleteOn = first.StoragePath.String

	swept, err := svc.SweepExpired(ctx)
	require.NoError(t, err, "one bad object must not fail the sweep")
	assert.Equal(t, 1, swept, "the healthy artifact is still reclaimed")

	// The failed one keeps its path so a later sweep retries it.
	stillThere, err := svc.GetExport(ctx, ids[0])
	require.NoError(t, err)
	assert.True(t, stillThere.StoragePath.Valid,
		"a failed delete must leave the row pointing at the artifact for retry")
}

func gameDirPrefix(gameID int32) string {
	return GameDirName(gameID, "Export Integration Game") + "/"
}

func zipEntryNames(t *testing.T, data []byte) []string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	names := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	return names
}

func newTestService(t *testing.T, pool *pgxpool.Pool) (*Service, *memStorage) {
	t.Helper()
	st := newMemStorage()
	svc := NewService(pool, t.TempDir(), nil)
	svc.Store = st
	return svc, st
}

func TestService_FullJobLifecycle(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	gameID := seedCompletedGame(t, pool)
	svc, store := newTestService(t, pool)

	// Request -> pending
	job, err := svc.RequestExport(ctx, gameID, 0)
	require.NoError(t, err)
	assert.Equal(t, "pending", job.Status)
	assert.False(t, job.StoragePath.Valid, "pending job has no artifact yet")

	// Run -> complete
	claimed, err := svc.RunNextJob(ctx)
	require.NoError(t, err)
	assert.True(t, claimed)

	done, err := svc.GetExport(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, "complete", done.Status)
	require.True(t, done.StoragePath.Valid)
	assert.True(t, done.SizeBytes.Valid)
	assert.Positive(t, done.SizeBytes.Int64)
	assert.True(t, done.FileCount.Valid)
	assert.Positive(t, done.FileCount.Int32)
	assert.True(t, done.ContentFingerprint.Valid, "completed job must record a fingerprint")
	assert.False(t, done.ProgressNote.Valid, "progress note cleared on completion")

	// The artifact really landed in storage and is a zip.
	data, ok := store.get(done.StoragePath.String)
	require.True(t, ok, "archive must be uploaded to storage")
	require.Greater(t, len(data), 4)
	assert.Equal(t, []byte("PK"), data[:2], "artifact must be a zip")
}

func TestService_ReusesCachedExportWhenUnchanged(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	gameID := seedCompletedGame(t, pool)
	svc, _ := newTestService(t, pool)

	first, err := svc.RequestExport(ctx, gameID, 0)
	require.NoError(t, err)
	_, err = svc.RunNextJob(ctx)
	require.NoError(t, err)

	// Same content -> same export row, no new job.
	second, err := svc.RequestExport(ctx, gameID, 0)
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID, "unchanged content must reuse the cached export")
	assert.Equal(t, "complete", second.Status)

	claimed, err := svc.RunNextJob(ctx)
	require.NoError(t, err)
	assert.False(t, claimed, "no new job should have been queued")
}

func TestService_RegeneratesWhenContentChanges(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	gameID := seedCompletedGame(t, pool)
	svc, _ := newTestService(t, pool)

	first, err := svc.RequestExport(ctx, gameID, 0)
	require.NoError(t, err)
	_, err = svc.RunNextJob(ctx)
	require.NoError(t, err)

	// A post-completion GM edit must invalidate the cached archive.
	_, err = pool.Exec(ctx, `UPDATE games SET updated_at = NOW() + interval '1 second' WHERE id=$1`, gameID)
	require.NoError(t, err)

	second, err := svc.RequestExport(ctx, gameID, 0)
	require.NoError(t, err)
	assert.NotEqual(t, first.ID, second.ID, "changed content must queue a fresh export")
	assert.Equal(t, "pending", second.Status)
}

func TestService_ConcurrentRequestsCoalesce(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	gameID := seedCompletedGame(t, pool)
	svc, _ := newTestService(t, pool)

	const n = 8
	ids := make([]int32, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			job, err := svc.RequestExport(ctx, gameID, 0)
			if err == nil {
				ids[i] = job.ID
			}
		}(i)
	}
	wg.Wait()

	// Every caller must land on the same job: the partial unique index makes
	// duplicate in-flight exports impossible.
	seen := map[int32]bool{}
	for _, id := range ids {
		require.NotZero(t, id, "every concurrent request should resolve to a job")
		seen[id] = true
	}
	assert.Len(t, seen, 1, "concurrent requests must coalesce onto one job")

	var queued int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM game_exports WHERE game_id=$1`, gameID).Scan(&queued))
	assert.Equal(t, 1, queued)
}

func TestService_RefusesNonCompletedGame(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	gameID := seedCompletedGame(t, pool)
	svc, _ := newTestService(t, pool)

	_, err := pool.Exec(ctx, `UPDATE games SET state='in_progress' WHERE id=$1`, gameID)
	require.NoError(t, err)

	_, err = svc.RequestExport(ctx, gameID, 0)
	assert.ErrorIs(t, err, ErrGameNotCompleted)

	var queued int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM game_exports WHERE game_id=$1`, gameID).Scan(&queued))
	assert.Zero(t, queued, "a refused request must not queue a job")
}

// A job that fails must be recorded as failed, not left running, and must not
// take the worker down with it.
func TestService_FailedJobIsRecorded(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	gameID := seedCompletedGame(t, pool)

	store := newMemStorage()
	store.failOn = "exports/"
	svc := NewService(pool, t.TempDir(), nil)
	svc.Store = store

	job, err := svc.RequestExport(ctx, gameID, 0)
	require.NoError(t, err)

	claimed, err := svc.RunNextJob(ctx)
	require.NoError(t, err, "a job failure must not surface as a worker error")
	assert.True(t, claimed)

	failed, err := svc.GetExport(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, "failed", failed.Status)
	require.True(t, failed.ErrorMessage.Valid)
	assert.Contains(t, failed.ErrorMessage.String, "simulated write failure")

	// A failed job must not block a later retry.
	retry, err := svc.RequestExport(ctx, gameID, 0)
	require.NoError(t, err)
	assert.NotEqual(t, job.ID, retry.ID)
	assert.Equal(t, "pending", retry.Status)
}

func TestService_ClaimReturnsFalseWhenQueueEmpty(t *testing.T) {
	pool := testPool(t)
	svc, _ := newTestService(t, pool)

	// Drain anything left by other tests, then confirm the empty signal.
	for i := 0; i < 50; i++ {
		claimed, err := svc.RunNextJob(context.Background())
		require.NoError(t, err)
		if !claimed {
			return
		}
	}
	t.Fatal("queue never drained")
}

func TestService_RequeueStalledRecoversCrashedJob(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	gameID := seedCompletedGame(t, pool)
	svc, _ := newTestService(t, pool)

	job, err := svc.RequestExport(ctx, gameID, 0)
	require.NoError(t, err)

	// Simulate a process that claimed the job and died: running, started long ago.
	_, err = pool.Exec(ctx,
		`UPDATE game_exports SET status='running', started_at = NOW() - interval '1 hour' WHERE id=$1`,
		job.ID)
	require.NoError(t, err)

	// A fresh job cannot be claimed while the stale one holds 'running'.
	claimed, err := svc.RunNextJob(ctx)
	require.NoError(t, err)
	assert.False(t, claimed, "stale running job is not pending")

	n, err := svc.RequeueStalled(ctx, 15*time.Minute)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, int64(1))

	recovered, err := svc.GetExport(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, "pending", recovered.Status)

	// And it now runs to completion.
	claimed, err = svc.RunNextJob(ctx)
	require.NoError(t, err)
	assert.True(t, claimed)

	done, err := svc.GetExport(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, "complete", done.Status)
}

// A job running normally in another replica must not be yanked away.
func TestService_RequeueLeavesFreshRunningJobAlone(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	gameID := seedCompletedGame(t, pool)
	svc, _ := newTestService(t, pool)

	job, err := svc.RequestExport(ctx, gameID, 0)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`UPDATE game_exports SET status='running', started_at = NOW() WHERE id=$1`, job.ID)
	require.NoError(t, err)

	_, err = svc.RequeueStalled(ctx, 15*time.Minute)
	require.NoError(t, err)

	still, err := svc.GetExport(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, "running", still.Status,
		"a recently started job belongs to a live worker and must be left alone")
}

func TestService_GetLatestExport(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	gameID := seedCompletedGame(t, pool)
	svc, _ := newTestService(t, pool)

	_, err := svc.GetLatestExport(ctx, gameID)
	assert.ErrorIs(t, err, pgx.ErrNoRows, "no exports yet")

	job, err := svc.RequestExport(ctx, gameID, 0)
	require.NoError(t, err)

	latest, err := svc.GetLatestExport(ctx, gameID)
	require.NoError(t, err)
	assert.Equal(t, job.ID, latest.ID)
}

// Two workers polling at once must never run the same job twice.
func TestService_ConcurrentWorkersClaimDistinctJobs(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	svc, _ := newTestService(t, pool)

	const games = 4
	for i := 0; i < games; i++ {
		gameID := seedCompletedGame(t, pool)
		_, err := svc.RequestExport(ctx, gameID, 0)
		require.NoError(t, err)
	}

	var mu sync.Mutex
	claims := 0
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				claimed, err := svc.RunNextJob(ctx)
				if err != nil || !claimed {
					return
				}
				mu.Lock()
				claims++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	assert.GreaterOrEqual(t, claims, games,
		"every queued job must be claimed exactly once across workers")

	var running int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM game_exports WHERE status IN ('pending','running')`).Scan(&running))
	assert.Zero(t, running, "no job may be left in flight")
}
