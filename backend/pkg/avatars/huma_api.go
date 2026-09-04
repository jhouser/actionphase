package avatars

// Huma (type-first) implementation of the character avatar API.
//
// This is the migration's reference for multipart uploads: the "avatar" file
// field is declared as a struct field, so the request body appears in the
// generated spec as a documented multipart schema rather than as prose.
// See .claude/planning/huma-migration.md.

import (
	"context"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/jwtauth/v5"
)

// avatarUpload is the multipart body. The `form` tag names the field, matching
// the "avatar" the frontend already sends.
type avatarUpload struct {
	// No contentType tag on purpose: huma would reject a bad type before the
	// handler runs, with its own message ("Invalid mime type: got X, expected
	// ..."). The service's message is friendlier and is what users see, so the
	// service stays the single validator for both MIME and size. The accepted
	// types are still published to the spec via doc.
	Avatar huma.FormFile `form:"avatar" required:"true" doc:"Avatar image — JPG, PNG or WebP, max 5MB"`
}

type uploadAvatarInput struct {
	ID      int32 `path:"id" doc:"Character ID"`
	RawBody huma.MultipartFormFiles[avatarUpload]
}

type uploadAvatarOutput struct {
	Body AvatarUploadResponse
}

type deleteAvatarInput struct {
	ID int32 `path:"id" doc:"Character ID"`
}

// userIDFromContext reads the authenticated user's ID from the JWT "sub" claim.
//
// The auth middleware runs before the handler, so a failure here means a
// malformed token rather than a missing one.
func userIDFromContext(ctx context.Context) (int32, error) {
	token, _, err := jwtauth.FromContext(ctx)
	if err != nil {
		return 0, huma.Error401Unauthorized("Unauthorized")
	}
	sub, ok := token.Get("sub")
	if !ok {
		return 0, huma.Error401Unauthorized("User ID not found in token")
	}
	s, ok := sub.(string)
	if !ok {
		return 0, huma.Error401Unauthorized("Invalid user ID in token")
	}
	id, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, huma.Error401Unauthorized("Invalid user ID in token")
	}
	return int32(id), nil
}

// requireEditPermission checks the caller may modify this character's avatar.
func (h *Handler) requireEditPermission(ctx context.Context, characterID int32) error {
	userID, err := userIDFromContext(ctx)
	if err != nil {
		return err
	}
	canEdit, err := h.CharacterService.CanUserEditCharacter(ctx, characterID, userID)
	if err != nil {
		return huma.Error500InternalServerError("Failed to check permissions")
	}
	if !canEdit {
		return huma.Error403Forbidden("You don't have permission to modify this character's avatar")
	}
	return nil
}

func (h *Handler) service() *AvatarService {
	return &AvatarService{DB: h.App.DB, Storage: h.App.Storage}
}

// HumaUploadCharacterAvatar stores a new avatar image for a character.
func (h *Handler) HumaUploadCharacterAvatar(ctx context.Context, in *uploadAvatarInput) (*uploadAvatarOutput, error) {
	if err := h.requireEditPermission(ctx, in.ID); err != nil {
		return nil, err
	}

	file := in.RawBody.Data().Avatar
	if !file.IsSet {
		return nil, huma.Error400BadRequest("Missing 'avatar' file in request")
	}
	defer file.Close()

	h.App.Logger.Info("Avatar file received",
		"character_id", in.ID,
		"filename", file.Filename,
		"size", file.Size,
		"content_type", file.ContentType,
	)

	contentType := file.ContentType
	if contentType == "" {
		contentType = detectContentType(file.Filename)
	}

	// The service owns MIME and size validation so that the rules live beside
	// the storage logic that depends on them; isValidationError maps its
	// rejections to 400 rather than 500.
	avatarURL, err := h.service().UploadCharacterAvatar(ctx, in.ID, file, file.Filename, contentType)
	if err != nil {
		if isValidationError(err) {
			return nil, huma.Error400BadRequest(err.Error())
		}
		h.App.Logger.Error("Failed to upload avatar", "error", err, "character_id", in.ID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &uploadAvatarOutput{Body: AvatarUploadResponse{AvatarURL: avatarURL}}, nil
}

// HumaDeleteCharacterAvatar removes a character's avatar.
func (h *Handler) HumaDeleteCharacterAvatar(ctx context.Context, in *deleteAvatarInput) (*struct{}, error) {
	if err := h.requireEditPermission(ctx, in.ID); err != nil {
		return nil, err
	}

	if err := h.service().DeleteCharacterAvatar(ctx, in.ID); err != nil {
		h.App.Logger.Error("Failed to delete avatar", "error", err, "character_id", in.ID)
		return nil, huma.Error500InternalServerError("Failed to delete avatar")
	}
	return nil, nil
}

// RegisterHumaAvatars registers the avatar operations on api.
//
// Paths are relative to the characters router's mount point (/api/v1/characters).
func RegisterHumaAvatars(api huma.API, h *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "uploadCharacterAvatar",
		Method:      http.MethodPost,
		Path:        "/{id}/avatar",
		Summary:     "Upload a character avatar",
		Description: "Accepts multipart/form-data with an `avatar` file field. " +
			"JPG, PNG and WebP are allowed, up to 5MB.",
		Tags:     []string{"Characters"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"400": {Description: "Missing, oversized, or unsupported image"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not permitted to modify this character"},
		},
	}, h.HumaUploadCharacterAvatar)

	huma.Register(api, huma.Operation{
		OperationID:   "deleteCharacterAvatar",
		Method:        http.MethodDelete,
		Path:          "/{id}/avatar",
		Summary:       "Delete a character avatar",
		Tags:          []string{"Characters"},
		DefaultStatus: http.StatusNoContent,
		Security:      []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not permitted to modify this character"},
		},
	}, h.HumaDeleteCharacterAvatar)
}
