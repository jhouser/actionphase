package communities

// Community banner upload and removal (Phase 7).
//
// Mirrors the game banner path in pkg/games, with two deliberate differences:
//
//  1. The gate is CanModerateCommunity, not a single-owner check. A game banner
//     is primary-GM-only because a game has one author; keeping a community's
//     profile current is ordinary upkeep, the same tier as bans and documents.
//     This matches PATCH /communities/{slug}, which is also requireModerator.
//
//  2. The response is the refreshed community rather than the bare URL, so the
//     frontend can replace its cached profile from the upload response without
//     a follow-up GET.
//
// The ordering rules the game path established are load-bearing and repeated
// here rather than reinvented: delete the previous object BEFORE storing the
// new one so a community never accumulates orphans, and roll the new object
// back if the column write fails so the bucket never keeps a file no row
// points at.

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"actionphase/pkg/core"
)

// communityBannerUpload declares the multipart field.
//
// No contentType tag, matching the game banner: huma would validate the MIME
// before the handler runs and emit its own message, replacing the friendlier
// "invalid file type X. Only JPG, PNG, and WebP images are allowed" that users
// actually read. Both are 400, so a status-only test cannot tell them apart.
type communityBannerUpload struct {
	Banner huma.FormFile `form:"banner" required:"true" doc:"Banner image: JPG, PNG or WebP, at most 5MB"`
}

type uploadCommunityBannerInput struct {
	Slug    string `path:"slug" doc:"Community URL slug"`
	RawBody huma.MultipartFormFiles[communityBannerUpload]
}

// humaUploadCommunityBanner stores a banner image and stamps its URL on the
// community.
func (h *Handler) humaUploadCommunityBanner(ctx context.Context, in *uploadCommunityBannerInput) (*communityOutput, error) {
	community, userID, err := h.requireModerator(ctx, in.Slug)
	if err != nil {
		return nil, err
	}

	// Storage is optional at the application level, so an unconfigured
	// deployment answers 503 rather than panicking on a nil backend -- the same
	// treatment WebhookSender gets on the test-message endpoint.
	if h.App.Storage == nil {
		return nil, huma.Error503ServiceUnavailable("file storage is not configured")
	}

	file := in.RawBody.Data().Banner

	contentType := file.ContentType
	if contentType == "" {
		contentType = core.BannerMimeTypeFromFilename(file.Filename)
	}
	if !core.AllowedBannerMimeTypes[contentType] {
		return nil, huma.Error400BadRequest(
			fmt.Sprintf("invalid file type %s. Only JPG, PNG, and WebP images are allowed", contentType))
	}

	fileData, err := core.ReadAndValidateBannerSize(file)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}

	// Removed before the replacement is stored, so a community never
	// accumulates orphaned objects. Best-effort: a failed delete of the old
	// file must not block the new upload.
	if community.BannerURL != nil && *community.BannerURL != "" {
		_ = h.App.Storage.Delete(ctx, core.ExtractBannerPathFromURL(*community.BannerURL))
	}

	ext := filepath.Ext(file.Filename)
	if ext == "" {
		ext = core.BannerExtFromMime(contentType)
	}
	// Keyed by community ID, not slug: the id is immutable, so a stored object
	// stays addressable no matter what the profile is renamed to.
	//
	// UnixNano, not Unix. Second granularity collides on a same-second replace,
	// and the collision is not benign: the new object lands on the key the old
	// one occupied, so the public URL never changes and browsers and CDNs keep
	// serving the CACHED PREVIOUS IMAGE. The user re-uploads and sees no change.
	// The delete-then-upload ordering above makes it worse -- it removes the
	// object it is about to overwrite. pkg/games has the same latent bug on
	// banners/games/; left alone here as out of scope for this phase.
	storagePath := fmt.Sprintf("banners/communities/%d/%d%s", community.ID, time.Now().UnixNano(), ext)

	bannerURL, err := h.App.Storage.Upload(ctx, storagePath, fileData, contentType)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to store community banner",
			"community_id", community.ID, "user_id", userID)
		return nil, huma.Error500InternalServerError("failed to upload banner")
	}

	updated, err := h.CommunityService.UpdateCommunityBannerURL(ctx, community.ID, &bannerURL)
	if err != nil {
		// Roll the object back, or the bucket keeps a file no row points at.
		_ = h.App.Storage.Delete(ctx, storagePath)
		h.App.ObsLogger.LogError(ctx, err, "Failed to save community banner URL",
			"community_id", community.ID, "user_id", userID)
		return nil, huma.Error500InternalServerError("failed to save banner")
	}

	h.App.ObsLogger.Info(ctx, "Community banner uploaded",
		"community_id", community.ID, "user_id", userID)

	h.withCallerContext(ctx, userID, updated)
	return &communityOutput{Body: updated}, nil
}

// humaDeleteCommunityBanner removes the stored object and clears the column.
//
// Succeeds when there is no banner: the caller's intended end state already
// holds, the same reasoning removeModerator uses for a non-moderator.
func (h *Handler) humaDeleteCommunityBanner(ctx context.Context, in *communitySlugInput) (*struct{}, error) {
	community, userID, err := h.requireModerator(ctx, in.Slug)
	if err != nil {
		return nil, err
	}

	// Object first, column second. The reverse order would strip the only
	// reference to the file before deleting it, orphaning it permanently on a
	// failure between the two.
	if community.BannerURL != nil && *community.BannerURL != "" {
		if h.App.Storage != nil {
			_ = h.App.Storage.Delete(ctx, core.ExtractBannerPathFromURL(*community.BannerURL))
		}
	}

	if _, err := h.CommunityService.UpdateCommunityBannerURL(ctx, community.ID, nil); err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to clear community banner",
			"community_id", community.ID, "user_id", userID)
		return nil, huma.Error500InternalServerError("failed to remove banner")
	}

	h.App.ObsLogger.Info(ctx, "Community banner removed",
		"community_id", community.ID, "user_id", userID)

	return nil, nil
}

// RegisterHumaCommunityBanner wires the banner endpoints onto the communities
// API, mounted alongside the other /api/v1/communities routes.
func RegisterHumaCommunityBanner(api huma.API, h *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "uploadCommunityBanner",
		Method:      http.MethodPost,
		Path:        "/{slug}/banner",
		Summary:     "Upload a community banner image",
		Description: "Requires moderation rights. JPG, PNG or WebP, at most 5MB. Replaces " +
			"any existing banner, deleting the previous file. Returns the refreshed community.",
		Tags:     []string{"Communities"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"400": {Description: "Missing file, unsupported type, or larger than 5MB"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Caller does not moderate this community"},
			"404": {Description: "Community not found"},
			"503": {Description: "File storage is not configured"},
		},
	}, h.humaUploadCommunityBanner)

	huma.Register(api, huma.Operation{
		OperationID: "deleteCommunityBanner",
		Method:      http.MethodDelete,
		Path:        "/{slug}/banner",
		Summary:     "Remove a community's banner image",
		Description: "Requires moderation rights. Deletes the stored file and clears the " +
			"column. Succeeds when there is no banner set.",
		Tags:          []string{"Communities"},
		Security:      []map[string][]string{{"BearerAuth": {}}},
		DefaultStatus: http.StatusNoContent,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "Caller does not moderate this community"},
			"404": {Description: "Community not found"},
		},
	}, h.humaDeleteCommunityBanner)
}
