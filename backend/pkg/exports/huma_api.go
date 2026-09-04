package exports

// Huma (type-first) implementation of the game archive export API.
//
// This is the migration's reference for two patterns:
//
//   - a dynamic status code (200 vs 202), via a `Status` field on the output
//   - a streamed file download with custom headers, via huma.StreamResponse
//
// See .claude/planning/huma-migration.md.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	core "actionphase/pkg/core"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
)

type gameIDPathInput struct {
	GameID int32 `path:"gameID" minimum:"1" doc:"Game ID"`
}

// requestExportOutput carries an explicit Status because the same operation
// answers 200 (an artifact or in-flight job already exists) or 202 (a new job
// was queued). Huma reads the Status field when present, so clients can still
// skip polling on a cache hit.
type requestExportOutput struct {
	Status int
	Body   *ExportResponse
}

type exportOutput struct {
	Body *ExportResponse
}

type downloadExportInput struct {
	ExportID int32 `path:"exportID" minimum:"1" doc:"Export job ID"`
}

// authorize confirms the caller may read this game, translating the shared
// render.Renderer errors into huma ones.
//
// Completed games are a public archive, so CanUserViewGame passes for any
// authenticated user; the check still guards enumeration by export id.
func (h *Handler) authorize(ctx context.Context, gameID int32) (int32, error) {
	userID, errResp := h.authorizeGameRead(ctx, gameID)
	if errResp == nil {
		return userID, nil
	}
	resp, ok := errResp.(*core.ErrResponse)
	if !ok {
		return 0, huma.Error500InternalServerError("authorization failed")
	}
	return 0, huma.NewError(resp.HTTPStatusCode, resp.ErrorText)
}

// HumaRequestExport enqueues (or reuses) an archive export for a completed game.
func (h *Handler) HumaRequestExport(ctx context.Context, in *gameIDPathInput) (*requestExportOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_request_game_export")()

	userID, err := h.authorize(ctx, in.GameID)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Export request not authorized", "game_id", in.GameID)
		return nil, err
	}

	job, err := h.ExportService.RequestExport(ctx, in.GameID, userID)
	if err != nil {
		// Not-completed is a client-side precondition failure, not a bug.
		if errors.Is(err, ErrGameNotCompleted) {
			h.App.ObsLogger.Warn(ctx, "Export requested for non-completed game", "game_id", in.GameID)
			return nil, huma.Error409Conflict("only completed games can be exported")
		}
		h.App.ObsLogger.Error(ctx, "Failed to request export", "error", err, "game_id", in.GameID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	// 202 only when work was actually queued; a served cache hit or joined job
	// is a plain 200 so clients can skip polling.
	status := http.StatusOK
	if job.Status == "pending" || job.Status == "running" {
		status = http.StatusAccepted
	}
	return &requestExportOutput{Status: status, Body: toExportResponse(job)}, nil
}

// HumaGetLatestExport reports the newest export for a game, for status polling.
func (h *Handler) HumaGetLatestExport(ctx context.Context, in *gameIDPathInput) (*exportOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_latest_game_export")()

	if _, err := h.authorize(ctx, in.GameID); err != nil {
		h.App.ObsLogger.Warn(ctx, "Export status not authorized", "game_id", in.GameID)
		return nil, err
	}

	job, err := h.ExportService.GetLatestExport(ctx, in.GameID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			h.App.ObsLogger.Warn(ctx, "No export found", "game_id", in.GameID)
			return nil, huma.Error404NotFound("no export exists for this game")
		}
		h.App.ObsLogger.Error(ctx, "Failed to load latest export", "error", err, "game_id", in.GameID)
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return &exportOutput{Body: toExportResponse(job)}, nil
}

// HumaDownloadExport streams the stored archive to the caller.
//
// Authorization is resolved from the export's game rather than trusting the
// export id, so a leaked id cannot bypass the game's access rules. See the
// note on DownloadExport's chi predecessor for why streaming beats redirecting.
func (h *Handler) HumaDownloadExport(ctx context.Context, in *downloadExportInput) (*huma.StreamResponse, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_download_game_export")()

	job, err := h.ExportService.GetExport(ctx, in.ExportID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			h.App.ObsLogger.Warn(ctx, "Export not found", "export_id", in.ExportID)
			return nil, huma.Error404NotFound("export")
		}
		h.App.ObsLogger.Error(ctx, "Failed to load export", "error", err, "export_id", in.ExportID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	if _, err := h.authorize(ctx, job.GameID); err != nil {
		h.App.ObsLogger.Warn(ctx, "Export download not authorized",
			"export_id", in.ExportID, "game_id", job.GameID)
		return nil, err
	}

	// A complete export with no storage_path was swept after its retention
	// window. Say so specifically: the caller can just request a new one, which
	// is different from "still building" or "failed".
	if job.Status == "complete" && !job.StoragePath.Valid {
		h.App.ObsLogger.Warn(ctx, "Download requested for expired export",
			"export_id", in.ExportID, "game_id", job.GameID)
		return nil, huma.Error409Conflict("this archive has expired; request a new export to download it again")
	}

	if job.Status != "complete" || !job.StoragePath.Valid {
		h.App.ObsLogger.Warn(ctx, "Download requested for unfinished export",
			"export_id", in.ExportID, "status", job.Status)
		return nil, huma.Error409Conflict(fmt.Sprintf("export is not ready (status: %s)", job.Status))
	}

	body, size, err := h.ExportService.Store.Open(ctx, job.StoragePath.String)
	if err != nil {
		if errors.Is(err, ErrArchiveMissing) {
			// The row still points at a file that is gone (e.g. the volume was
			// replaced). Same remedy as expiry: request a new export.
			h.App.ObsLogger.Warn(ctx, "Archive missing from disk",
				"export_id", in.ExportID, "path", job.StoragePath.String)
			return nil, huma.Error409Conflict("this archive is no longer available; request a new export")
		}
		h.App.ObsLogger.Error(ctx, "Failed to open archive", "error", err, "export_id", in.ExportID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	filename := DownloadFilename(job.GameID, job.GameTitle)
	exportID, gameID := in.ExportID, job.GameID

	return &huma.StreamResponse{Body: func(hctx huma.Context) {
		// Closed here rather than by a defer above: the body function runs
		// after this handler returns, so an earlier close would empty it.
		defer body.Close()

		hctx.SetHeader("Content-Type", "application/zip")
		hctx.SetHeader("Content-Length", strconv.FormatInt(size, 10))
		hctx.SetHeader("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		// Archives are per-game private content with a short life; never let a
		// shared cache hold one.
		hctx.SetHeader("Cache-Control", "private, no-store")
		hctx.SetStatus(http.StatusOK)

		h.App.ObsLogger.Info(ctx, "Serving export download",
			"export_id", exportID, "game_id", gameID, "filename", filename, "bytes", size)

		if _, err := io.Copy(hctx.BodyWriter(), body); err != nil {
			// Headers are already sent, so this can only be logged — most often
			// a client that disconnected mid-download.
			h.App.ObsLogger.Warn(ctx, "Export download interrupted",
				"error", err, "export_id", exportID, "game_id", gameID)
		}
	}}, nil
}

// RegisterHumaGameExports registers the per-game export operations.
//
// Paths are relative to the router this is called on, which is the /{gameID}
// subrouter — so "/exports", not "/{gameID}/exports". The gameID is still bound
// from the URL by the gameIDPathInput tag; chi matched it one level up.
func RegisterHumaGameExports(api huma.API, h *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "requestGameExport",
		Method:      http.MethodPost,
		Path:        "/exports",
		Summary:     "Request a game archive export",
		Description: "Enqueues an archive export for a completed game, or returns the " +
			"existing artifact or in-flight job. Responds 202 when a new job was " +
			"queued and 200 when an export already existed.",
		Tags:     []string{"Exports"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "No access to this game"},
			"409": {Description: "Game has not completed"},
		},
	}, h.HumaRequestExport)

	huma.Register(api, huma.Operation{
		OperationID: "getLatestGameExport",
		Method:      http.MethodGet,
		Path:        "/exports/latest",
		Summary:     "Get the most recent export for a game",
		Description: "Reports the newest export job, for polling while one builds.",
		Tags:        []string{"Exports"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "No access to this game"},
			"404": {Description: "No export exists for this game"},
		},
	}, h.HumaGetLatestExport)
}

// RegisterHumaExportDownloads registers the download operation.
//
// Paths are relative to the exports router's mount point (/api/v1/exports).
func RegisterHumaExportDownloads(api huma.API, h *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "downloadGameExport",
		Method:      http.MethodGet,
		Path:        "/{exportID}/download",
		Summary:     "Download a completed game archive",
		Description: "Streams the ZIP archive as an attachment. Access is re-checked " +
			"against the export's game, so an export id alone grants nothing.",
		Tags:     []string{"Exports"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"200": {
				Description: "The archive",
				Content: map[string]*huma.MediaType{
					"application/zip": {Schema: &huma.Schema{Type: "string", Format: "binary"}},
				},
			},
			"401": {Description: "Not authenticated"},
			"403": {Description: "No access to this game"},
			"404": {Description: "Export not found"},
			"409": {Description: "Archive expired, missing, or not yet ready"},
		},
	}, h.HumaDownloadExport)
}
