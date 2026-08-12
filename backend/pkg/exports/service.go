package exports

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
	"time"

	core "actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	"actionphase/pkg/observability"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// JobQuerier is the job-lifecycle subset of the generated query API.
type JobQuerier interface {
	CreateGameExport(ctx context.Context, arg models.CreateGameExportParams) (models.GameExport, error)
	GetGameExport(ctx context.Context, id int32) (models.GetGameExportRow, error)
	GetLatestGameExport(ctx context.Context, gameID int32) (models.GameExport, error)
	GetActiveGameExport(ctx context.Context, gameID int32) (models.GameExport, error)
	GetReusableGameExport(ctx context.Context, arg models.GetReusableGameExportParams) (models.GameExport, error)
	ClaimNextGameExport(ctx context.Context) (models.GameExport, error)
	MarkGameExportProgress(ctx context.Context, arg models.MarkGameExportProgressParams) error
	MarkGameExportComplete(ctx context.Context, arg models.MarkGameExportCompleteParams) (models.GameExport, error)
	MarkGameExportFailed(ctx context.Context, arg models.MarkGameExportFailedParams) (models.GameExport, error)
	RequeueStalledGameExports(ctx context.Context, staleAfter pgtype.Interval) (int64, error)
	ListExpiredGameExports(ctx context.Context, resultLimit int32) ([]models.ListExpiredGameExportsRow, error)
	MarkGameExportExpired(ctx context.Context, id int32) error
}

// RetentionPeriod is how long a generated archive is kept before the sweep
// reclaims it.
//
// Completed games are effectively immutable — creates are blocked server-side
// and the History tab is read-only — so an archive never needs rebuilding,
// only expiring. Seven days keeps a shared download link working for a
// reasonable window; after that the row remains as history and the next
// request regenerates the file on demand.
const RetentionPeriod = 7 * 24 * time.Hour

// expirySweepBatch bounds how many artifacts one sweep deletes, so a large
// backlog cannot monopolize a worker tick.
const expirySweepBatch = 100

// Service owns the export job lifecycle: enqueueing requests, running jobs,
// and serving cached artifacts.
type Service struct {
	Pool *pgxpool.Pool
	// Store is local-disk only by design; see ArchiveStore for why archives
	// never use the shared (public, S3-backed) storage backend.
	Store     ArchiveStorer
	Logger    *observability.Logger
	Assembler *Assembler

	// Jobs defaults to a Queries built from Pool when nil.
	Jobs JobQuerier
}

// NewService wires a Service from a pool and an archive root directory.
func NewService(pool *pgxpool.Pool, archivePath string, logger *observability.Logger) *Service {
	q := models.New(pool)
	return &Service{
		Pool:      pool,
		Store:     NewArchiveStore(archivePath),
		Logger:    logger,
		Assembler: &Assembler{Queries: q},
		Jobs:      q,
	}
}

func (s *Service) jobs() JobQuerier {
	if s.Jobs != nil {
		return s.Jobs
	}
	return models.New(s.Pool)
}

// RequestExport returns an export for a game, creating a job only when needed.
//
// Resolution order:
//  1. a completed export whose fingerprint still matches current content — served immediately
//  2. an already pending/running job — the caller joins it rather than queueing a duplicate
//  3. otherwise a new pending job
//
// The caller is responsible for authorization; this method enforces only the
// completed-game invariant, which Assemble re-checks at run time.
func (s *Service) RequestExport(ctx context.Context, gameID, userID int32) (*models.GameExport, error) {
	q := s.jobs()

	// Reject early so a doomed job is never queued. Assemble checks again when
	// the worker runs, since a game's state could change in between.
	game, err := models.New(s.Pool).GetGame(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("load game %d: %w", gameID, err)
	}
	if !game.State.Valid || game.State.String != core.GameStateCompleted {
		return nil, fmt.Errorf("%w: game %d is %q",
			ErrGameNotCompleted, gameID, stateOrUnknown(game.State))
	}

	fingerprint, err := s.Assembler.FingerprintFor(ctx, gameID)
	if err != nil {
		return nil, err
	}

	// 1. Reusable cached artifact.
	cached, err := q.GetReusableGameExport(ctx, models.GetReusableGameExportParams{
		GameID:             gameID,
		ContentFingerprint: pgtype.Text{String: fingerprint, Valid: true},
	})
	if err == nil {
		return &cached, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("look up cached export: %w", err)
	}

	// 2. Join an in-flight job.
	if active, err := q.GetActiveGameExport(ctx, gameID); err == nil {
		return &active, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("look up active export: %w", err)
	}

	// 3. Queue a new job. The partial unique index is the real guard against a
	// race here: two simultaneous requests both reaching this point means one
	// insert fails, and that loser joins the winner's job.
	// requested_by_user_id is nullable and ON DELETE SET NULL: it is provenance,
	// not a dependency. A zero id (no authenticated user) is stored as NULL
	// rather than violating the foreign key.
	requestedBy := pgtype.Int4{Int32: userID, Valid: userID > 0}
	created, err := q.CreateGameExport(ctx, models.CreateGameExportParams{
		GameID:            gameID,
		RequestedByUserID: requestedBy,
	})
	if err != nil {
		if active, lookupErr := q.GetActiveGameExport(ctx, gameID); lookupErr == nil {
			return &active, nil
		}
		return nil, fmt.Errorf("create export job: %w", err)
	}
	return &created, nil
}

// GetExport returns a single export job by id.
func (s *Service) GetExport(ctx context.Context, id int32) (*models.GetGameExportRow, error) {
	row, err := s.jobs().GetGameExport(ctx, id)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// GetLatestExport returns the newest export for a game, in any state.
func (s *Service) GetLatestExport(ctx context.Context, gameID int32) (*models.GameExport, error) {
	row, err := s.jobs().GetLatestGameExport(ctx, gameID)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// RunNextJob claims and runs one pending export.
//
// Returns claimed=false when the queue is empty. A job failure is recorded on
// the row and does NOT return an error: the worker loop should keep running.
// Only infrastructure faults (claim query failures) surface as errors.
func (s *Service) RunNextJob(ctx context.Context) (claimed bool, err error) {
	q := s.jobs()

	job, err := q.ClaimNextGameExport(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("claim export job: %w", err)
	}

	if runErr := s.runJob(ctx, q, job); runErr != nil {
		s.logf(ctx, runErr, "Export job failed", "export_id", job.ID, "game_id", job.GameID)

		if _, markErr := q.MarkGameExportFailed(ctx, models.MarkGameExportFailedParams{
			ID:           job.ID,
			ErrorMessage: pgtype.Text{String: truncateError(runErr.Error()), Valid: true},
		}); markErr != nil {
			// The row is stuck in 'running'; startup requeue will recover it.
			return true, fmt.Errorf("record export failure: %w", markErr)
		}
	}
	return true, nil
}

// runJob assembles and stores one archive.
func (s *Service) runJob(ctx context.Context, q JobQuerier, job models.GameExport) error {
	progress := func(note string) {
		if err := q.MarkGameExportProgress(ctx, models.MarkGameExportProgressParams{
			ID:           job.ID,
			ProgressNote: pgtype.Text{String: note, Valid: true},
		}); err != nil {
			s.logf(ctx, err, "Failed to record export progress", "export_id", job.ID)
		}
	}

	// Buffered rather than streamed: zip.Writer needs to seek back to patch
	// entry headers, and archives are text-only (single-digit MB), so holding
	// one in memory is cheaper than the temp-file plumbing streaming requires.
	var buf bytes.Buffer
	result, err := s.Assembler.Assemble(ctx, job.GameID, &buf, progress)
	if err != nil {
		return err
	}

	// Fingerprint AFTER assembly: taking it before would let an edit that lands
	// mid-assembly be recorded as already captured, pinning a stale archive.
	fingerprint, err := s.Assembler.FingerprintFor(ctx, job.GameID)
	if err != nil {
		return err
	}

	storagePath := ExportStoragePath(job.GameID, job.ID)
	if err := s.Store.Put(ctx, storagePath, bytes.NewReader(buf.Bytes())); err != nil {
		return fmt.Errorf("write archive: %w", err)
	}

	if _, err := q.MarkGameExportComplete(ctx, models.MarkGameExportCompleteParams{
		ID:                 job.ID,
		StoragePath:        pgtype.Text{String: storagePath, Valid: true},
		SizeBytes:          pgtype.Int8{Int64: int64(buf.Len()), Valid: true},
		FileCount:          pgtype.Int4{Int32: int32(result.FileCount), Valid: true},
		ContentFingerprint: pgtype.Text{String: fingerprint, Valid: true},
		RetentionPeriod:    durationToInterval(RetentionPeriod),
	}); err != nil {
		return fmt.Errorf("mark export complete: %w", err)
	}

	s.logInfo(ctx, "Export completed",
		"export_id", job.ID, "game_id", job.GameID,
		"files", result.FileCount, "bytes", buf.Len())
	return nil
}

// RequeueStalled returns jobs abandoned by a crashed process to the queue.
// Called on startup; the age bound avoids disturbing a job actively running in
// another replica.
func (s *Service) RequeueStalled(ctx context.Context, staleAfter time.Duration) (int64, error) {
	n, err := s.jobs().RequeueStalledGameExports(ctx, durationToInterval(staleAfter))
	if err != nil {
		return 0, fmt.Errorf("requeue stalled exports: %w", err)
	}
	if n > 0 {
		s.logInfo(ctx, "Requeued stalled export jobs", "count", n)
	}
	return n, nil
}

// SweepExpired deletes artifacts whose retention window has passed and clears
// their storage_path, keeping the row as history.
//
// Storage deletion happens BEFORE the row update so a crash in between leaves
// a row still pointing at a deleted file — which the next sweep retries
// harmlessly, since both backends treat deleting a missing file as success.
// The reverse order could strand a file with no row referencing it, making it
// unreclaimable.
//
// A single artifact that fails to delete is logged and skipped rather than
// aborting the sweep, so one bad object cannot block the rest.
func (s *Service) SweepExpired(ctx context.Context) (deleted int, err error) {
	q := s.jobs()

	rows, err := q.ListExpiredGameExports(ctx, expirySweepBatch)
	if err != nil {
		return 0, fmt.Errorf("list expired exports: %w", err)
	}

	for _, row := range rows {
		if !row.StoragePath.Valid {
			continue
		}
		if delErr := s.Store.Delete(ctx, row.StoragePath.String); delErr != nil {
			s.logf(ctx, delErr, "Failed to delete expired export artifact",
				"export_id", row.ID, "game_id", row.GameID, "path", row.StoragePath.String)
			continue
		}
		if markErr := q.MarkGameExportExpired(ctx, row.ID); markErr != nil {
			// File is gone but the row still points at it; the next sweep
			// retries and the delete is idempotent.
			s.logf(ctx, markErr, "Deleted expired artifact but failed to clear its path",
				"export_id", row.ID)
			continue
		}
		deleted++
	}

	if deleted > 0 {
		s.logInfo(ctx, "Swept expired export artifacts", "deleted", deleted)
	}
	return deleted, nil
}

// ExportStoragePath is the storage key for a finished archive. The export id
// keeps regenerated archives from overwriting one another, so a download URL
// handed out earlier keeps resolving to the bytes it described.
func ExportStoragePath(gameID, exportID int32) string {
	return path.Join("exports", fmt.Sprintf("game-%d", gameID), fmt.Sprintf("export-%d.zip", exportID))
}

// durationToInterval converts a Go duration into a pgtype.Interval.
func durationToInterval(d time.Duration) pgtype.Interval {
	return pgtype.Interval{Microseconds: int64(d / time.Microsecond), Valid: true}
}

// truncateError bounds an error string so a pathological message cannot bloat
// the row. error_message has no length limit, but the API returns it verbatim.
func truncateError(msg string) string {
	const max = 1000
	if len(msg) <= max {
		return msg
	}
	return msg[:max] + "… (truncated)"
}

func (s *Service) logf(ctx context.Context, err error, msg string, args ...any) {
	if s.Logger != nil {
		s.Logger.LogError(ctx, err, msg, args...)
	}
}

func (s *Service) logInfo(ctx context.Context, msg string, args ...any) {
	if s.Logger != nil {
		s.Logger.Info(ctx, msg, args...)
	}
}
