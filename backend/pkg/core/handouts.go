package core

import "time"

// Handout represents a GM-created informational document for a game.
// Handouts are persistent reference materials (rules, world info) that exist across all game phases.
// Only GMs can create/update/delete handouts. Players can view published handouts.
type Handout struct {
	ID        int32      `json:"id"`         // Unique handout identifier
	GameID    int32      `json:"game_id"`    // Associated game ID
	Title     string     `json:"title"`      // Handout title
	Content   string     `json:"content"`    // Markdown content
	Status    string     `json:"status"`     // "draft" or "published"
	CreatedAt *time.Time `json:"created_at"` // Creation timestamp
	UpdatedAt *time.Time `json:"updated_at"` // Last content edit timestamp
}

// HandoutComment represents a comment on a handout.
// Only GMs can comment on handouts. Supports threaded replies.
type HandoutComment struct {
	ID              int32      `json:"id"`                           // Unique comment identifier
	HandoutID       int32      `json:"handout_id"`                   // Associated handout ID
	UserID          int32      `json:"user_id"`                      // Comment author user ID
	ParentCommentID *int32     `json:"parent_comment_id"`            // Parent comment ID for threaded replies (null for top-level)
	Content         string     `json:"content"`                      // Comment content
	CreatedAt       *time.Time `json:"created_at"`                   // Creation timestamp
	UpdatedAt       *time.Time `json:"updated_at"`                   // Last update timestamp
	EditedAt        *time.Time `json:"edited_at,omitempty"`          // Last edit timestamp (null if never edited)
	EditCount       int32      `json:"edit_count"`                   // Number of times comment has been edited
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`         // Soft delete timestamp (null if not deleted)
	DeletedByUserID *int32     `json:"deleted_by_user_id,omitempty"` // ID of user who deleted the comment
}

// HandoutCommentWithAuthor represents a handout comment with author information.
// Used for API responses that include the author's username.
type HandoutCommentWithAuthor struct {
	HandoutComment
	AuthorUsername string `json:"author_username"` // Username of comment author
}

// HandoutWithGame is a handout carrying the game it belongs to, for surfaces
// that list handouts with no game in scope (the global Utility Drawer). The
// game title travels with each row so the client can group by game without a
// request per game.
type HandoutWithGame struct {
	Handout
	GameTitle string `json:"game_title"` // Title of the game the handout belongs to
}
