package games

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"actionphase/pkg/core"
	db "actionphase/pkg/db/models"

	"github.com/go-chi/render"
)

const maxBannerSize = 5 * 1024 * 1024 // 5MB

var allowedBannerMimeTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// UploadGameBanner handles POST /api/v1/games/{id}/banner
func (h *Handler) UploadGameBanner(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	game := ctx.Value("game").(*db.Game)

	userService := h.UserService
	userID, errResp := core.GetUserIDFromJWT(ctx, userService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Request rejected in upload game banner")
		return
	}

	gameService := h.GameService
	if game.GmUserID != userID {
		h.renderError(ctx, w, r, core.ErrForbidden("Only the GM can update the game banner"), "Upload game banner forbidden")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("failed to parse multipart form: %w", err)), "Invalid upload game banner request")
		return
	}

	file, header, err := r.FormFile("banner")
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("missing 'banner' file in request")), "Invalid upload game banner request")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = bannerMimeTypeFromFilename(header.Filename)
	}

	if !allowedBannerMimeTypes[contentType] {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid file type %s. Only JPG, PNG, and WebP images are allowed", contentType)), "Invalid upload game banner request")
		return
	}

	fileData, err := readAndValidateBannerSize(file)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid upload game banner request", "error", err)
		return
	}

	// Delete old banner if exists
	if game.BannerUrl.Valid && game.BannerUrl.String != "" {
		oldPath := extractBannerPathFromURL(game.BannerUrl.String)
		_ = h.App.Storage.Delete(ctx, oldPath)
	}

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = bannerExtFromMime(contentType)
	}
	storagePath := fmt.Sprintf("banners/games/%d/%d%s", game.ID, time.Now().Unix(), ext)

	bannerURL, err := h.App.Storage.Upload(ctx, storagePath, fileData, contentType)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(fmt.Errorf("failed to upload banner: %w", err)), "Failed to upload game banner")
		return
	}

	if err := gameService.UpdateGameBannerURL(ctx, game.ID, &bannerURL); err != nil {
		_ = h.App.Storage.Delete(ctx, storagePath)
		h.renderError(ctx, w, r, core.ErrInternalError(fmt.Errorf("failed to save banner URL: %w", err)), "Failed to upload game banner")
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, map[string]string{"banner_url": bannerURL})
}

// DeleteGameBanner handles DELETE /api/v1/games/{id}/banner
func (h *Handler) DeleteGameBanner(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	game := ctx.Value("game").(*db.Game)

	userService := h.UserService
	userID, errResp := core.GetUserIDFromJWT(ctx, userService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Request rejected in delete game banner")
		return
	}

	gameService := h.GameService
	if game.GmUserID != userID {
		h.renderError(ctx, w, r, core.ErrForbidden("Only the GM can remove the game banner"), "Delete game banner forbidden")
		return
	}

	if game.BannerUrl.Valid && game.BannerUrl.String != "" {
		oldPath := extractBannerPathFromURL(game.BannerUrl.String)
		_ = h.App.Storage.Delete(ctx, oldPath)
	}

	if err := gameService.UpdateGameBannerURL(ctx, game.ID, nil); err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(fmt.Errorf("failed to remove banner: %w", err)), "Failed to delete game banner")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func readAndValidateBannerSize(file io.Reader) (io.Reader, error) {
	limited := io.LimitReader(file, maxBannerSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	if int64(len(data)) > maxBannerSize {
		return nil, fmt.Errorf("image too large. Maximum size is 5MB")
	}
	return strings.NewReader(string(data)), nil
}

func extractBannerPathFromURL(url string) string {
	if index := strings.LastIndex(url, "banners/"); index != -1 {
		return url[index:]
	}
	if index := strings.LastIndex(url, "avatars/"); index != -1 {
		return url[index:]
	}
	if lastSlash := strings.LastIndex(url, "/"); lastSlash != -1 {
		return url[lastSlash+1:]
	}
	return url
}

func bannerMimeTypeFromFilename(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func bannerExtFromMime(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}
