package exports

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	core "actionphase/pkg/core"
	models "actionphase/pkg/db/models"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/jackc/pgx/v5"
)

// Handler serves the game archive export endpoints.
type Handler struct {
	App           *core.App
	UserService   core.UserServiceInterface
	GameService   core.GameServiceInterface
	ExportService *Service
}

// ExportResponse is the API view of an export job.
type ExportResponse struct {
	ID          int32  `json:"id"`
	GameID      int32  `json:"game_id"`
	Status      string `json:"status"`
	Progress    string `json:"progress,omitempty"`
	Error       string `json:"error,omitempty"`
	SizeBytes   int64  `json:"size_bytes,omitempty"`
	FileCount   int32  `json:"file_count,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
	// ExpiresAt is when the artifact will be reclaimed; the row survives.
	ExpiresAt string `json:"expires_at,omitempty"`
	// Expired marks a completed export whose artifact has been swept. The
	// archive can be regenerated on request.
	Expired bool `json:"expired,omitempty"`
}

func (e *ExportResponse) Render(http.ResponseWriter, *http.Request) error { return nil }

func toExportResponse(row *models.GameExport) *ExportResponse {
	resp := &ExportResponse{
		ID:     row.ID,
		GameID: row.GameID,
		Status: row.Status,
	}
	if row.ProgressNote.Valid {
		resp.Progress = row.ProgressNote.String
	}
	if row.ErrorMessage.Valid {
		resp.Error = row.ErrorMessage.String
	}
	if row.SizeBytes.Valid {
		resp.SizeBytes = row.SizeBytes.Int64
	}
	if row.FileCount.Valid {
		resp.FileCount = row.FileCount.Int32
	}
	if row.CreatedAt.Valid {
		resp.CreatedAt = row.CreatedAt.Time.UTC().Format(time.RFC3339)
	}
	if row.CompletedAt.Valid {
		resp.CompletedAt = row.CompletedAt.Time.UTC().Format(time.RFC3339)
	}
	// Only a finished, unswept artifact is downloadable. The URL points at our
	// own endpoint rather than storage, so authorization is re-checked per
	// download instead of handing out an unauthenticated direct link.
	//
	// A complete export whose storage_path was cleared has expired: report it
	// as such so the UI offers to regenerate rather than showing a dead link.
	if row.Status == "complete" {
		if row.StoragePath.Valid {
			resp.DownloadURL = fmt.Sprintf("/api/v1/exports/%d/download", row.ID)
		} else {
			resp.Expired = true
		}
	}
	if row.ExpiresAt.Valid {
		resp.ExpiresAt = row.ExpiresAt.Time.UTC().Format(time.RFC3339)
	}
	return resp
}

// authorizeGameRead confirms the caller may read this game, reusing the same
// primitive as the rest of the app: completed games are a public archive, so
// any authenticated user passes.
func (h *Handler) authorizeGameRead(ctx context.Context, gameID int32) (int32, render.Renderer) {
	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		return 0, errResp
	}

	canView, err := h.GameService.CanUserViewGame(ctx, gameID, userID)
	if err != nil {
		return 0, core.ErrInternalError(err)
	}
	if !canView {
		return 0, core.ErrForbidden("you do not have access to this game")
	}
	return userID, nil
}

// RequestExport enqueues (or reuses) an archive export for a completed game.
//
// POST /api/v1/games/{gameID}/exports
//
// Returns 200 with an existing artifact or in-flight job, 202 when a new job
// was queued, and 409 when the game has not completed.
func (h *Handler) RequestExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_request_game_export")()

	gameID, errResp := gameIDFromRequest(r)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Invalid game id in export request")
		return
	}

	userID, errResp := h.authorizeGameRead(ctx, gameID)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Export request not authorized", "game_id", gameID)
		return
	}

	job, err := h.ExportService.RequestExport(ctx, gameID, userID)
	if err != nil {
		// Not-completed is a client-side precondition failure, not a bug.
		if errors.Is(err, ErrGameNotCompleted) {
			h.renderError(ctx, w, r,
				core.ErrConflict("only completed games can be exported"),
				"Export requested for non-completed game", "game_id", gameID)
			return
		}
		h.renderError(ctx, w, r, core.ErrInternalError(err),
			"Failed to request export", "error", err, "game_id", gameID)
		return
	}

	// 202 only when work was actually queued; a served cache hit or joined job
	// is a plain 200 so clients can skip polling.
	if job.Status == "pending" || job.Status == "running" {
		render.Status(r, http.StatusAccepted)
	} else {
		render.Status(r, http.StatusOK)
	}
	render.Render(w, r, toExportResponse(job))
}

// GetLatestExport reports the newest export for a game, for status polling.
//
// GET /api/v1/games/{gameID}/exports/latest
func (h *Handler) GetLatestExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_latest_game_export")()

	gameID, errResp := gameIDFromRequest(r)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Invalid game id in export status request")
		return
	}

	if _, errResp := h.authorizeGameRead(ctx, gameID); errResp != nil {
		h.renderError(ctx, w, r, errResp, "Export status not authorized", "game_id", gameID)
		return
	}

	job, err := h.ExportService.GetLatestExport(ctx, gameID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			h.renderError(ctx, w, r, core.ErrNotFound("no export exists for this game"),
				"No export found", "game_id", gameID)
			return
		}
		h.renderError(ctx, w, r, core.ErrInternalError(err),
			"Failed to load latest export", "error", err, "game_id", gameID)
		return
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, toExportResponse(job))
}

// DownloadExport streams the stored archive to the caller.
//
// GET /api/v1/exports/{exportID}/download
//
// Authorization is resolved from the export's game rather than trusting the
// export id, so a leaked id cannot bypass the game's access rules.
//
// The auth check is deliberately kept even though it grants broadly: an archive
// only exists for a completed game, and CanUserViewGame returns true for any
// authenticated user in that case, so this guards nothing a player could not
// already read in the History tab. It is retained because serving archives from
// a static, unauthenticated path would make them crawlable and enumerable via
// sequential ids — a different exposure than "a logged-in user can read it" —
// and because streaming here is what allows a meaningful download filename.
// Removing it would trade those for no measurable gain at current volumes.
func (h *Handler) DownloadExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_download_game_export")()

	raw := chi.URLParam(r, "exportID")
	id, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || id <= 0 {
		h.renderError(ctx, w, r, core.ErrBadRequest(fmt.Errorf("invalid export id %q", raw)),
			"Invalid export id in download request")
		return
	}

	job, err := h.ExportService.GetExport(ctx, int32(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			h.renderError(ctx, w, r, core.ErrNotFound("export"), "Export not found", "export_id", id)
			return
		}
		h.renderError(ctx, w, r, core.ErrInternalError(err),
			"Failed to load export", "error", err, "export_id", id)
		return
	}

	if _, errResp := h.authorizeGameRead(ctx, job.GameID); errResp != nil {
		h.renderError(ctx, w, r, errResp, "Export download not authorized",
			"export_id", id, "game_id", job.GameID)
		return
	}

	// A complete export with no storage_path was swept after its retention
	// window. Say so specifically: the caller can just request a new one, which
	// is different from "still building" or "failed".
	if job.Status == "complete" && !job.StoragePath.Valid {
		h.renderError(ctx, w, r,
			core.ErrConflict("this archive has expired; request a new export to download it again"),
			"Download requested for expired export", "export_id", id, "game_id", job.GameID)
		return
	}

	if job.Status != "complete" || !job.StoragePath.Valid {
		h.renderError(ctx, w, r,
			core.ErrConflict(fmt.Sprintf("export is not ready (status: %s)", job.Status)),
			"Download requested for unfinished export", "export_id", id, "status", job.Status)
		return
	}

	// Stream through this handler rather than redirecting to a static path.
	// The redirect form made authorizeGameRead above decorative, since the
	// archive was then fetchable by URL alone, and gave no way to set a
	// meaningful download filename.
	body, size, err := h.ExportService.Store.Open(ctx, job.StoragePath.String)
	if err != nil {
		if errors.Is(err, ErrArchiveMissing) {
			// The row still points at a file that is gone (e.g. the volume was
			// replaced). Same remedy as expiry: request a new export.
			h.renderError(ctx, w, r,
				core.ErrConflict("this archive is no longer available; request a new export"),
				"Archive missing from disk", "export_id", id, "path", job.StoragePath.String)
			return
		}
		h.renderError(ctx, w, r, core.ErrInternalError(err),
			"Failed to open archive", "error", err, "export_id", id)
		return
	}
	defer body.Close()

	filename := DownloadFilename(job.GameID, job.GameTitle)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	// Archives are per-game private content with a short life; never let a
	// shared cache hold one.
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)

	h.App.ObsLogger.Info(ctx, "Serving export download",
		"export_id", id, "game_id", job.GameID, "filename", filename, "bytes", size)

	if _, err := io.Copy(w, body); err != nil {
		// Headers are already sent, so this can only be logged — most often a
		// client that disconnected mid-download.
		h.App.ObsLogger.Warn(ctx, "Export download interrupted",
			"error", err, "export_id", id, "game_id", job.GameID)
	}
}

// gameIDFromRequest reads the game id from the GameMiddleware context when
// present, falling back to the URL parameter.
func gameIDFromRequest(r *http.Request) (int32, render.Renderer) {
	if v := r.Context().Value("gameID"); v != nil {
		if id, ok := v.(int32); ok && id > 0 {
			return id, nil
		}
	}
	raw := chi.URLParam(r, "gameID")
	id, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || id <= 0 {
		return 0, core.ErrBadRequest(fmt.Errorf("invalid game id %q", raw))
	}
	return int32(id), nil
}

// renderError logs and renders an error, at Error level for 5xx and Warn for 4xx.
func (h *Handler) renderError(ctx context.Context, w http.ResponseWriter, r *http.Request, errResp render.Renderer, msg string, args ...any) {
	if resp, ok := errResp.(*core.ErrResponse); ok && resp.HTTPStatusCode >= 500 {
		h.App.ObsLogger.Error(ctx, msg, args...)
	} else {
		h.App.ObsLogger.Warn(ctx, msg, args...)
	}
	render.Render(w, r, errResp)
}
