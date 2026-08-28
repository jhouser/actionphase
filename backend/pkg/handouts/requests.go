package handouts

import (
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// Request validation
//
// Length and enum rules live in the struct tags rather than in Resolve, because
// huma validates the schema *before* Resolve runs and publishes those rules in
// the spec -- a client can see that status is draft|published without sending a
// request to find out. A duplicate check in Resolve would be unreachable.
//
// What the schema cannot do is trim. huma's minLength counts raw characters, so
// a title of "   " satisfies minLength:"1" and a blank-titled row would reach
// the database. Resolve therefore trims and re-checks the fields the schema
// requires to be non-empty.

// requiredText trims *s in place and reports it missing if nothing is left.
func requiredText(s *string, field string) []error {
	*s = strings.TrimSpace(*s)
	if *s != "" {
		return nil
	}
	return []error{&huma.ErrorDetail{
		Message:  field + " is required",
		Location: "body." + field,
	}}
}

// trimHandoutBody trims and re-checks the text fields shared by create and
// update. Status needs no check here: its enum tag already rejects anything
// outside the set, and no amount of surrounding whitespace makes "  draft  "
// into a valid value.
func trimHandoutBody(title, content *string) []error {
	var errs []error
	errs = append(errs, requiredText(title, "title")...)
	errs = append(errs, requiredText(content, "content")...)
	return errs
}

// CreateHandoutRequest represents the request to create a new handout
type CreateHandoutRequest struct {
	Title   string `json:"title" minLength:"1" maxLength:"255" doc:"Handout title"`
	Content string `json:"content" minLength:"1" doc:"Handout body, as markdown"`
	Status  string `json:"status" enum:"draft,published" doc:"draft is GM-only; published is visible to the game"`
}

// Resolve validates the create body. Huma runs it after decoding.
//
// The content check is the one that matters: the create modal labels content
// required but enforces nothing, because the markdown editor is not a native
// input the browser will block submit on. Before this ran, an empty handout was
// created silently.
func (r *CreateHandoutRequest) Resolve(huma.Context) []error {
	return trimHandoutBody(&r.Title, &r.Content)
}

// UpdateHandoutRequest represents the request to update a handout
type UpdateHandoutRequest struct {
	Title   string `json:"title" minLength:"1" maxLength:"255" doc:"Handout title"`
	Content string `json:"content" minLength:"1" doc:"Handout body, as markdown"`
	Status  string `json:"status" enum:"draft,published" doc:"draft is GM-only; published is visible to the game"`
}

// Resolve validates the update body. See CreateHandoutRequest.Resolve.
func (r *UpdateHandoutRequest) Resolve(huma.Context) []error {
	return trimHandoutBody(&r.Title, &r.Content)
}

// CreateHandoutCommentRequest represents the request to create a handout comment
type CreateHandoutCommentRequest struct {
	Content         string `json:"content" minLength:"1" doc:"Comment text"`
	ParentCommentID *int32 `json:"parent_comment_id,omitempty" required:"false" doc:"Comment being replied to, for threading"`
}

// Resolve trims the content so a whitespace-only comment is rejected rather
// than stored blank.
func (r *CreateHandoutCommentRequest) Resolve(huma.Context) []error {
	return requiredText(&r.Content, "content")
}

// UpdateHandoutCommentRequest represents the request to update a handout comment
type UpdateHandoutCommentRequest struct {
	Content string `json:"content" minLength:"1" doc:"Replacement comment text"`
}

// Resolve trims the content. See CreateHandoutCommentRequest.Resolve.
func (r *UpdateHandoutCommentRequest) Resolve(huma.Context) []error {
	return requiredText(&r.Content, "content")
}

// HandoutResponse represents a handout in API responses
type HandoutResponse struct {
	ID        int32      `json:"id" doc:"Handout ID"`
	GameID    int32      `json:"game_id" doc:"Game the handout belongs to"`
	Title     string     `json:"title" doc:"Handout title"`
	Content   string     `json:"content" doc:"Handout body, as markdown"`
	Status    string     `json:"status" enum:"draft,published" doc:"draft is GM-only; published is visible to the game"`
	CreatedAt *time.Time `json:"created_at,omitempty" required:"false"`
	UpdatedAt *time.Time `json:"updated_at,omitempty" required:"false"`
}

// HandoutWithGameResponse is a handout plus the game it belongs to, for the
// cross-game list where the client has no game in scope to resolve the title
// from.
type HandoutWithGameResponse struct {
	HandoutResponse
	GameTitle string `json:"game_title" doc:"Title of the game this handout belongs to"`
}

// HandoutCommentResponse represents a handout comment in API responses
type HandoutCommentResponse struct {
	ID              int32      `json:"id" doc:"Comment ID"`
	HandoutID       int32      `json:"handout_id" doc:"Handout being commented on"`
	UserID          int32      `json:"user_id" doc:"Author"`
	ParentCommentID *int32     `json:"parent_comment_id,omitempty" required:"false" doc:"Comment this replies to"`
	Content         string     `json:"content" doc:"Comment text"`
	EditCount       int32      `json:"edit_count" doc:"How many times the comment has been edited"`
	CreatedAt       *time.Time `json:"created_at,omitempty" required:"false"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty" required:"false"`
	EditedAt        *time.Time `json:"edited_at,omitempty" required:"false" doc:"When the comment was last edited"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty" required:"false" doc:"Set when the comment is soft-deleted"`
	DeletedByUserID *int32     `json:"deleted_by_user_id,omitempty" required:"false" doc:"Who soft-deleted the comment"`
}
