package messages

import (
	"fmt"
	"net/http"
	"time"

	"actionphase/pkg/core"
)

type Handler struct {
	App            *core.App
	UserService    core.UserServiceInterface
	MessageService core.MessageServiceInterface
}

// Request Types

type CreatePostRequest struct {
	PhaseID     *int32 `json:"phase_id,omitempty"`
	CharacterID int32  `json:"character_id" validate:"required"`
	Content     string `json:"content" validate:"required,min=1"`
}

func (r *CreatePostRequest) Bind(req *http.Request) error {
	return core.ValidateStruct(r)
}

type CreateCommentRequest struct {
	PhaseID     *int32 `json:"phase_id,omitempty"`
	CharacterID int32  `json:"character_id" validate:"required"`
	Content     string `json:"content" validate:"required,min=1"`
	RootPostID  *int32 `json:"root_post_id,omitempty"`
}

func (r *CreateCommentRequest) Bind(req *http.Request) error {
	return core.ValidateStruct(r)
}

type UpdateCommentRequest struct {
	Content     string `json:"content" validate:"required,min=1"`
	CharacterID *int32 `json:"character_id,omitempty"`
}

func (r *UpdateCommentRequest) Bind(req *http.Request) error {
	return core.ValidateStruct(r)
}

type UpdatePostRequest struct {
	Content string `json:"content" validate:"required,min=1"`
}

func (r *UpdatePostRequest) Bind(req *http.Request) error {
	return core.ValidateStruct(r)
}

type CreateDraftPostRequest struct {
	CharacterID int32  `json:"character_id" validate:"required"`
	Content     string `json:"content" validate:"required,min=1"`
}

func (r *CreateDraftPostRequest) Bind(req *http.Request) error {
	return core.ValidateStruct(r)
}

type UpdateDraftPostRequest struct {
	Content string `json:"content" validate:"required,min=1"`
}

func (r *UpdateDraftPostRequest) Bind(req *http.Request) error {
	return core.ValidateStruct(r)
}

// ManualReadCommentIDsResponse represents the manual read comment IDs for a post
type ManualReadCommentIDsResponse struct {
	PostID         int32   `json:"post_id"`
	ReadCommentIDs []int32 `json:"read_comment_ids"`
}

// Response Types

type MessageResponse struct {
	ID                    int32      `json:"id"`
	GameID                int32      `json:"game_id"`
	PhaseID               *int32     `json:"phase_id,omitempty"`
	AuthorID              int32      `json:"author_id"`
	CharacterID           int32      `json:"character_id"`
	Content               string     `json:"content"`
	MessageType           string     `json:"message_type"`
	ParentID              *int32     `json:"parent_id,omitempty"`
	ThreadDepth           int32      `json:"thread_depth"`
	AuthorUsername        string     `json:"author_username"`
	CharacterName         string     `json:"character_name"`
	CharacterAvatarUrl    *string    `json:"character_avatar_url,omitempty"`
	CommentCount          int64      `json:"comment_count"` // Always include, even if 0
	ReplyCount            int64      `json:"reply_count,omitempty"`
	IsEdited              bool       `json:"is_edited"`
	IsDeleted             bool       `json:"is_deleted"`
	IsDraft               bool       `json:"is_draft"`
	MentionedCharacterIds []int32    `json:"mentioned_character_ids,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	DeletedAt             *time.Time `json:"deleted_at,omitempty"`
	DeletedByUserID       *int32     `json:"deleted_by_user_id,omitempty"`
	EditedAt              *time.Time `json:"edited_at,omitempty"`
	EditCount             int32      `json:"edit_count"`
}

func (rd *MessageResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

// MessageThreadContextResponse is the payload for deep-linking to a nested comment:
// the target plus a bounded slice of its ancestors, and the true root post ID.
type MessageThreadContextResponse struct {
	// Chain is the target comment plus up to max_parents nearest ancestors,
	// ordered parent-to-child (nearest included ancestor → target).
	Chain []*MessageResponse `json:"chain"`
	// RootPostID is the top-level post at the head of the full thread, even when
	// Chain is trimmed and does not itself reach the root.
	RootPostID int32 `json:"root_post_id"`
	// HasFullThread is true when Chain reaches the root post (nothing trimmed above).
	HasFullThread bool `json:"has_full_thread"`
}

// getUserIDFromToken extracts the authenticated user ID from the request JWT.
func (h *Handler) getUserIDFromToken(r *http.Request) (int32, error) {
	userID, errResp := core.GetUserIDFromJWT(r.Context(), h.UserService)
	if errResp != nil {
		return 0, fmt.Errorf("authentication failed")
	}
	return userID, nil
}

// messageWithDetailsToResponse converts a MessageWithDetails to a MessageResponse.
func messageWithDetailsToResponse(msg *core.MessageWithDetails) *MessageResponse {
	response := &MessageResponse{
		ID:                    msg.ID,
		GameID:                msg.GameID,
		AuthorID:              msg.AuthorID,
		CharacterID:           msg.CharacterID,
		Content:               msg.Content,
		MessageType:           string(msg.MessageType),
		ThreadDepth:           msg.ThreadDepth,
		AuthorUsername:        msg.AuthorUsername,
		CharacterName:         msg.CharacterName,
		CharacterAvatarUrl:    msg.CharacterAvatarUrl,
		IsEdited:              msg.IsEdited,
		IsDeleted:             msg.IsDeleted,
		IsDraft:               msg.IsDraft,
		MentionedCharacterIds: msg.MentionedCharacterIds,
		CreatedAt:             msg.CreatedAt.Time,
		EditCount:             msg.EditCount,
	}

	if msg.PhaseID.Valid {
		phaseID := msg.PhaseID.Int32
		response.PhaseID = &phaseID
	}

	if msg.ParentID.Valid {
		parentID := msg.ParentID.Int32
		response.ParentID = &parentID
	}

	if msg.DeletedAt.Valid {
		deletedAt := msg.DeletedAt.Time
		response.DeletedAt = &deletedAt
	}

	if msg.DeletedByUserID.Valid {
		deletedByUserID := msg.DeletedByUserID.Int32
		response.DeletedByUserID = &deletedByUserID
	}

	if msg.EditedAt.Valid {
		editedAt := msg.EditedAt.Time
		response.EditedAt = &editedAt
	}

	// Set either CommentCount or ReplyCount depending on message type
	if string(msg.MessageType) == "post" {
		response.CommentCount = msg.CommentCount
	} else {
		response.ReplyCount = msg.ReplyCount
	}

	return response
}

// countTopLevelInResponse counts how many top-level comments (depth=0) are in the response.
func countTopLevelInResponse(comments []core.CommentWithDepth) int {
	count := 0
	for _, c := range comments {
		if c.Depth == 0 {
			count++
		}
	}
	return count
}

// commentsWithParentsToResponse converts a CommentWithParent slice to response format.
func commentsWithParentsToResponse(comments []core.CommentWithParent, showUsernames bool) []map[string]interface{} {
	result := make([]map[string]interface{}, len(comments))
	for i, comment := range comments {
		authorUsername := comment.AuthorUsername
		if !showUsernames {
			authorUsername = ""
		}
		commentData := map[string]interface{}{
			"id":                   comment.ID,
			"game_id":              comment.GameID,
			"parent_id":            comment.ParentID,
			"post_id":              comment.PostID,
			"author_id":            comment.AuthorID,
			"character_id":         comment.CharacterID,
			"content":              comment.Content,
			"created_at":           comment.CreatedAt.Format(time.RFC3339),
			"edited_at":            formatTimePtr(comment.EditedAt),
			"edit_count":           comment.EditCount,
			"deleted_at":           formatTimePtr(comment.DeletedAt),
			"is_deleted":           comment.IsDeleted,
			"author_username":      authorUsername,
			"character_name":       comment.CharacterName,
			"character_avatar_url": comment.CharacterAvatarUrl,
		}

		// Add parent data if exists
		if comment.ParentContent != nil {
			parentAuthorUsername := comment.ParentAuthorUsername
			if !showUsernames {
				emptyStr := ""
				parentAuthorUsername = &emptyStr
			}
			commentData["parent"] = map[string]interface{}{
				"content":              comment.ParentContent,
				"created_at":           formatTimePtr(comment.ParentCreatedAt),
				"deleted_at":           formatTimePtr(comment.ParentDeletedAt),
				"is_deleted":           comment.ParentIsDeleted,
				"message_type":         comment.ParentMessageType,
				"author_username":      parentAuthorUsername,
				"character_name":       comment.ParentCharacterName,
				"character_avatar_url": comment.ParentCharacterAvatarUrl,
			}
		}

		result[i] = commentData
	}
	return result
}

// formatTimePtr formats a *time.Time as RFC3339 string or nil.
func formatTimePtr(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339)
}
