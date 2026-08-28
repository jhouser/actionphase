package exports

import (
	"context"
	"fmt"
	"net/http"
	"time"

	core "actionphase/pkg/core"
	models "actionphase/pkg/db/models"

	"github.com/go-chi/render"
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
