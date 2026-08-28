package users

// Huma (type-first) implementation of the user profile and avatar API.
//
// See .claude/planning/huma-migration.md.

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"actionphase/pkg/core"

	"github.com/danielgtaylor/huma/v2"
)

// ------------------------------------------------------------------ I/O types

// pagination is shared by both profile lookups. The chi handlers clamped bad
// values silently; huma rejects them, which is the better contract — but the
// bounds are the same ones the old code enforced.
type pagination struct {
	Page     int `query:"page" default:"1" minimum:"1" doc:"Page of game history"`
	PageSize int `query:"page_size" default:"12" minimum:"1" maximum:"100" doc:"Games per page"`
}

type getUserProfileInput struct {
	ID int32 `path:"id" minimum:"1" doc:"User ID"`
	pagination
}

type getUserProfileByUsernameInput struct {
	Username string `path:"username" minLength:"1" doc:"Username"`
	pagination
}

type profileOutput struct {
	Body *core.UserProfileResponse
}

// updateProfileBody has no minLength: both fields are optional, and an empty
// string is meaningful — it clears the field. Only the upper bounds are
// enforced, matching the limits the chi Bind checked (255 and 10000).
type updateProfileBody struct {
	DisplayName *string `json:"display_name,omitempty" required:"false" maxLength:"255" doc:"Shown instead of the username; empty clears it"`
	Bio         *string `json:"bio,omitempty" required:"false" maxLength:"10000" doc:"Free-text profile bio; empty clears it"`
}

type updateUserProfileInput struct {
	Body updateProfileBody
}

type userAvatarUpload struct {
	// No contentType tag: the service validates MIME and size, and its message
	// is the one users see. See the multipart note in huma-migration.md.
	Avatar huma.FormFile `form:"avatar" required:"true" doc:"Avatar image — JPG, PNG or WebP, max 5MB"`
}

type uploadUserAvatarInput struct {
	RawBody huma.MultipartFormFiles[userAvatarUpload]
}

type uploadUserAvatarOutput struct {
	Body UploadAvatarResponse
}

// deleteUserAvatarOutput keeps the message body the chi handler returned. This
// endpoint answers 200 with a body rather than 204, unlike the character avatar
// delete — preserved so clients reading .message keep working.
type deleteUserAvatarOutput struct {
	Body struct {
		Message string `json:"message" doc:"Confirmation message"`
	}
}

// ------------------------------------------------------------------- helpers

func (h *Handler) profileService() *UserProfileService {
	return &UserProfileService{DB: h.App.Pool}
}

func (h *Handler) avatarService() *UserAvatarService {
	return &UserAvatarService{DB: h.App.Pool, Storage: h.App.Storage}
}

// currentUser resolves the authenticated user's id from the JWT.
func (h *Handler) currentUser(ctx context.Context) (int32, error) {
	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		return 0, huma.Error401Unauthorized("no valid token found")
	}
	return userID, nil
}

// ------------------------------------------------------------------ handlers

// HumaGetUserProfile returns a user's public profile and game history.
// Any authenticated user may view any profile.
func (h *Handler) HumaGetUserProfile(ctx context.Context, in *getUserProfileInput) (*profileOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_user_profile")()

	if _, err := h.currentUser(ctx); err != nil {
		return nil, err
	}

	profile, err := h.profileService().GetUserProfile(ctx, in.ID, in.Page, in.PageSize)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get user profile", "error", err, "user_id", in.ID)
		return nil, huma.Error404NotFound("user profile")
	}
	return &profileOutput{Body: profile}, nil
}

// HumaGetUserProfileByUsername is HumaGetUserProfile addressed by username.
func (h *Handler) HumaGetUserProfileByUsername(ctx context.Context, in *getUserProfileByUsernameInput) (*profileOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_user_profile_by_username")()

	if _, err := h.currentUser(ctx); err != nil {
		return nil, err
	}

	user, err := h.UserService.UserByUsername(in.Username)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to find user", "error", err, "username", in.Username)
		return nil, huma.Error404NotFound("user")
	}

	profile, err := h.profileService().GetUserProfile(ctx, int32(user.ID), in.Page, in.PageSize)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get user profile", "error", err, "user_id", user.ID)
		return nil, huma.Error404NotFound("user profile")
	}
	return &profileOutput{Body: profile}, nil
}

// HumaUpdateUserProfile edits the caller's own display name and/or bio.
func (h *Handler) HumaUpdateUserProfile(ctx context.Context, in *updateUserProfileInput) (*profileOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_user_profile")()

	userID, err := h.currentUser(ctx)
	if err != nil {
		return nil, err
	}

	svc := h.profileService()
	if err := svc.UpdateUserProfile(ctx, userID, in.Body.DisplayName, in.Body.Bio); err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to update user profile", "error", err, "user_id", userID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	// Return the updated profile, first page at the default size.
	profile, err := svc.GetUserProfile(ctx, userID, 1, 12)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get updated profile", "error", err, "user_id", userID)
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return &profileOutput{Body: profile}, nil
}

// HumaUploadUserAvatar stores a new avatar for the authenticated user.
func (h *Handler) HumaUploadUserAvatar(ctx context.Context, in *uploadUserAvatarInput) (*uploadUserAvatarOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_upload_user_avatar")()

	userID, err := h.currentUser(ctx)
	if err != nil {
		return nil, err
	}

	file := in.RawBody.Data().Avatar
	if !file.IsSet {
		return nil, huma.Error400BadRequest("avatar file is required")
	}
	defer file.Close()

	contentType := file.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	fileData, readErr := io.ReadAll(file)
	if readErr != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to read file data", "error", readErr, "user_id", userID)
		return nil, huma.Error400BadRequest("failed to read file")
	}

	// Every service error is a 400 here, matching the chi handler: the service
	// only rejects for bad MIME or oversize, both of which are client faults.
	avatarURL, upErr := h.avatarService().UploadUserAvatar(
		ctx, userID, bytes.NewReader(fileData), file.Filename, contentType)
	if upErr != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to upload user avatar", "error", upErr, "user_id", userID)
		return nil, huma.Error400BadRequest(upErr.Error())
	}

	return &uploadUserAvatarOutput{Body: UploadAvatarResponse{AvatarURL: avatarURL}}, nil
}

// HumaDeleteUserAvatar removes the authenticated user's avatar.
func (h *Handler) HumaDeleteUserAvatar(ctx context.Context, _ *struct{}) (*deleteUserAvatarOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_user_avatar")()

	userID, err := h.currentUser(ctx)
	if err != nil {
		return nil, err
	}

	if delErr := h.avatarService().DeleteUserAvatar(ctx, userID); delErr != nil {
		h.App.ObsLogger.Error(ctx, "Failed to delete user avatar", "error", delErr, "user_id", userID)
		return nil, huma.Error500InternalServerError(delErr.Error())
	}

	out := &deleteUserAvatarOutput{}
	out.Body.Message = "Avatar deleted successfully"
	return out, nil
}

// ---------------------------------------------------------------- registration

// RegisterHumaUsers registers the user profile and avatar operations.
//
// Paths are relative to the users router's mount point (/api/v1/users).
func RegisterHumaUsers(api huma.API, h *Handler) {
	auth := []map[string][]string{{"BearerAuth": {}}}

	huma.Register(api, huma.Operation{
		OperationID: "getUserProfile",
		Method:      http.MethodGet,
		Path:        "/{id}/profile",
		Summary:     "Get a user's profile by ID",
		Description: "Public to any authenticated user. Includes a paginated game history.",
		Tags:        []string{"Users"},
		Security:    auth,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"404": {Description: "User profile not found"},
		},
	}, h.HumaGetUserProfile)

	huma.Register(api, huma.Operation{
		OperationID: "getUserProfileByUsername",
		Method:      http.MethodGet,
		Path:        "/username/{username}/profile",
		Summary:     "Get a user's profile by username",
		Description: "Public to any authenticated user. Includes a paginated game history.",
		Tags:        []string{"Users"},
		Security:    auth,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"404": {Description: "User not found"},
		},
	}, h.HumaGetUserProfileByUsername)

	huma.Register(api, huma.Operation{
		OperationID: "updateOwnProfile",
		Method:      http.MethodPatch,
		Path:        "/me/profile",
		Summary:     "Update your own profile",
		Description: "Both fields are optional; sending an empty string clears one.",
		Tags:        []string{"Users"},
		Security:    auth,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.HumaUpdateUserProfile)

	huma.Register(api, huma.Operation{
		OperationID:   "uploadOwnAvatar",
		Method:        http.MethodPost,
		Path:          "/me/avatar",
		Summary:       "Upload your own avatar",
		Description:   "multipart/form-data with an `avatar` file field. JPG, PNG or WebP, max 5MB.",
		Tags:          []string{"Users"},
		DefaultStatus: http.StatusCreated,
		Security:      auth,
		Responses: map[string]*huma.Response{
			"400": {Description: "Missing, oversized, or unsupported image"},
			"401": {Description: "Not authenticated"},
		},
	}, h.HumaUploadUserAvatar)

	huma.Register(api, huma.Operation{
		OperationID: "deleteOwnAvatar",
		Method:      http.MethodDelete,
		Path:        "/me/avatar",
		Summary:     "Delete your own avatar",
		Tags:        []string{"Users"},
		Security:    auth,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.HumaDeleteUserAvatar)
}
