package handouts

import (
	"net/http"
	"time"
)

// CreateHandoutRequest represents the request to create a new handout
type CreateHandoutRequest struct {
	Title   string `json:"title" validate:"required,min=1,max=255"`
	Content string `json:"content" validate:"required"`
	Status  string `json:"status" validate:"required,oneof=draft published"`
}

func (r *CreateHandoutRequest) Bind(req *http.Request) error {
	return nil
}

// UpdateHandoutRequest represents the request to update a handout
type UpdateHandoutRequest struct {
	Title   string `json:"title" validate:"required,min=1,max=255"`
	Content string `json:"content" validate:"required"`
	Status  string `json:"status" validate:"required,oneof=draft published"`
}

func (r *UpdateHandoutRequest) Bind(req *http.Request) error {
	return nil
}

// CreateHandoutCommentRequest represents the request to create a handout comment
type CreateHandoutCommentRequest struct {
	Content         string `json:"content" validate:"required"`
	ParentCommentID *int32 `json:"parent_comment_id,omitempty"`
}

func (r *CreateHandoutCommentRequest) Bind(req *http.Request) error {
	return nil
}

// UpdateHandoutCommentRequest represents the request to update a handout comment
type UpdateHandoutCommentRequest struct {
	Content string `json:"content" validate:"required"`
}

func (r *UpdateHandoutCommentRequest) Bind(req *http.Request) error {
	return nil
}

// HandoutResponse represents a handout in API responses
type HandoutResponse struct {
	ID        int32      `json:"id"`
	GameID    int32      `json:"game_id"`
	Title     string     `json:"title"`
	Content   string     `json:"content"`
	Status    string     `json:"status"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

// HandoutWithGameResponse is a handout plus the game it belongs to, for the
// cross-game list where the client has no game in scope to resolve the title
// from.
type HandoutWithGameResponse struct {
	HandoutResponse
	GameTitle string `json:"game_title"`
}

// HandoutCommentResponse represents a handout comment in API responses
type HandoutCommentResponse struct {
	ID              int32      `json:"id"`
	HandoutID       int32      `json:"handout_id"`
	UserID          int32      `json:"user_id"`
	ParentCommentID *int32     `json:"parent_comment_id,omitempty"`
	Content         string     `json:"content"`
	EditCount       int32      `json:"edit_count"`
	CreatedAt       *time.Time `json:"created_at,omitempty"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
	EditedAt        *time.Time `json:"edited_at,omitempty"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
	DeletedByUserID *int32     `json:"deleted_by_user_id,omitempty"`
}
