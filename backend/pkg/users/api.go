package users

import (
	"actionphase/pkg/core"
)

// Handler holds dependencies for user profile API handlers
type Handler struct {
	App         *core.App
	UserService core.UserServiceInterface
}

// Request and Response Types
//
// The update-profile body and its length limits now live on the huma input
// struct in huma_api.go, which is what the server enforces and what the spec
// publishes.

// UploadAvatarResponse is the response after uploading an avatar
type UploadAvatarResponse struct {
	AvatarURL string `json:"avatar_url"`
}
