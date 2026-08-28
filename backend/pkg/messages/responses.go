package messages

import (
	"time"
)

// Response bodies
//
// Most of these replace `map[string]interface{}` literals the chi handlers
// encoded straight to the ResponseWriter. They reproduce those maps key for
// key, including two behaviours worth stating explicitly:
//
//   - Anonymous games blank the author's username to "" rather than omitting
//     the key. The key is always present; only its value changes. Callers read
//     it as a string, so an absent key would be a different contract.
//   - Nullable columns that the map set unconditionally stay present-and-null
//     here; ones the map added inside an `if` are pointers with `omitempty`.

// PostSummaryResponse is one entry of the common-room post list.
//
// Deliberately narrower than MessageResponse: the list omits is_draft,
// updated_at, edit_count and the edit/delete audit fields, which the detail
// endpoints return. Widening it would be a change to what the list ships, so
// the shape is preserved as-is.
type PostSummaryResponse struct {
	ID                 int32   `json:"id" doc:"Post ID"`
	GameID             int32   `json:"game_id" doc:"Game this post belongs to"`
	AuthorID           int32   `json:"author_id" doc:"User who wrote the post"`
	CharacterID        int32   `json:"character_id" doc:"Character the post is attributed to"`
	Content            string  `json:"content" doc:"Post body, as markdown"`
	MessageType        string  `json:"message_type" doc:"Always \"post\" for this endpoint"`
	ThreadDepth        int32   `json:"thread_depth" doc:"Always 0 for a top-level post"`
	AuthorUsername     string  `json:"author_username" doc:"Blank in an anonymous game when the caller may not see it"`
	CharacterName      string  `json:"character_name" doc:"Character's display name"`
	CharacterAvatarURL *string `json:"character_avatar_url" doc:"Character portrait URL"`
	CommentCount       int64   `json:"comment_count" doc:"Direct and nested comments on this post"`
	IsEdited           bool    `json:"is_edited"`
	IsDeleted          bool    `json:"is_deleted"`

	CreatedAt time.Time `json:"created_at"`

	// Added by the chi handler only when the column was non-NULL.
	PhaseID  *int32 `json:"phase_id,omitempty" required:"false" doc:"Phase the post belongs to, absent for phase-less posts"`
	ParentID *int32 `json:"parent_id,omitempty" required:"false" doc:"Parent message, absent for top-level posts"`
}

// CommentSummaryResponse is one entry of the flat per-post comment list.
type CommentSummaryResponse struct {
	ID                    int32   `json:"id" doc:"Comment ID"`
	GameID                int32   `json:"game_id" doc:"Game this comment belongs to"`
	AuthorID              int32   `json:"author_id" doc:"User who wrote the comment"`
	CharacterID           int32   `json:"character_id" doc:"Character the comment is attributed to"`
	Content               string  `json:"content" doc:"Comment body, as markdown"`
	MessageType           string  `json:"message_type" doc:"Always \"comment\" for this endpoint"`
	ThreadDepth           int32   `json:"thread_depth" doc:"Nesting level under the root post"`
	AuthorUsername        string  `json:"author_username" doc:"Blank in an anonymous game when the caller may not see it"`
	CharacterName         string  `json:"character_name" doc:"Character's display name"`
	CharacterAvatarURL    *string `json:"character_avatar_url" doc:"Character portrait URL"`
	ReplyCount            int64   `json:"reply_count" doc:"Direct replies to this comment"`
	IsEdited              bool    `json:"is_edited"`
	IsDeleted             bool    `json:"is_deleted"`
	MentionedCharacterIDs []int32 `json:"mentioned_character_ids" doc:"Characters @-mentioned in the body"`

	CreatedAt time.Time `json:"created_at"`

	PhaseID  *int32 `json:"phase_id,omitempty" required:"false" doc:"Phase the comment belongs to"`
	ParentID *int32 `json:"parent_id,omitempty" required:"false" doc:"Message being replied to"`
}

// ThreadedCommentResponse is a comment in the paginated tree view, which adds
// the depth the client needs to nest it.
type ThreadedCommentResponse struct {
	CommentSummaryResponse
	Depth int32 `json:"depth" doc:"Nesting depth: 0 is top-level, 1+ is a reply"`
}

// PaginatedCommentsResponse is the body of the threaded comment endpoint.
//
// It reports four counts because they answer different questions: total_top_level
// drives the pager, returned_top_level and returned_total describe this page
// (which holds nested replies as well as top-level comments), and has_more says
// whether paging further is worthwhile.
type PaginatedCommentsResponse struct {
	Comments         []*ThreadedCommentResponse `json:"comments" doc:"Top-level comments and their nested replies, flattened"`
	TotalTopLevel    int64                      `json:"total_top_level" doc:"Top-level comments on the post, across all pages"`
	Limit            int32                      `json:"limit" doc:"Top-level comments requested"`
	Offset           int32                      `json:"offset" doc:"Top-level comments skipped"`
	HasMore          bool                       `json:"has_more" doc:"Whether more top-level comments exist beyond this page"`
	ReturnedTopLevel int                        `json:"returned_top_level" doc:"Top-level comments in this response"`
	ReturnedTotal    int                        `json:"returned_total" doc:"All comments in this response, nested replies included"`
}

// ParentContextResponse is the message a comment replies to, embedded so the
// "New Comments" view can render context without a second request.
//
// Every field is nullable: the parent may be soft-deleted, and its character
// columns are nullable in the database.
type ParentContextResponse struct {
	Content            *string `json:"content" doc:"Parent body; null when the parent was deleted"`
	CreatedAt          *string `json:"created_at" doc:"RFC3339 timestamp"`
	DeletedAt          *string `json:"deleted_at" doc:"RFC3339 timestamp, null when not deleted"`
	IsDeleted          *bool   `json:"is_deleted"`
	MessageType        *string `json:"message_type" doc:"\"post\" or \"comment\""`
	AuthorUsername     *string `json:"author_username" doc:"Blanked in an anonymous game when the caller may not see it"`
	CharacterName      *string `json:"character_name"`
	CharacterAvatarURL *string `json:"character_avatar_url"`
}

// CommentWithParentResponse is one entry of the "New Comments" view.
//
// Timestamps here are pre-formatted RFC3339 strings rather than time values,
// which is what the chi handler emitted; the frontend parses them as strings.
type CommentWithParentResponse struct {
	ID                 int32   `json:"id" doc:"Comment ID"`
	GameID             int32   `json:"game_id"`
	ParentID           *int32  `json:"parent_id" doc:"Message being replied to"`
	PostID             *int32  `json:"post_id" doc:"Top-level post at the head of this thread"`
	AuthorID           int32   `json:"author_id"`
	CharacterID        int32   `json:"character_id"`
	Content            string  `json:"content" doc:"Comment body, as markdown"`
	CreatedAt          string  `json:"created_at" doc:"RFC3339 timestamp"`
	EditedAt           *string `json:"edited_at" doc:"RFC3339 timestamp, null when never edited"`
	EditCount          int32   `json:"edit_count"`
	DeletedAt          *string `json:"deleted_at" doc:"RFC3339 timestamp, null when not deleted"`
	IsDeleted          bool    `json:"is_deleted"`
	AuthorUsername     string  `json:"author_username" doc:"Blank in an anonymous game when the caller may not see it"`
	CharacterName      *string `json:"character_name"`
	CharacterAvatarURL *string `json:"character_avatar_url"`

	// Present only when the comment has a parent to show.
	Parent *ParentContextResponse `json:"parent,omitempty" required:"false" doc:"The message being replied to"`
}

// CharacterMessageResponse is one entry of a character's activity feed.
//
// Same shape as CommentWithParentResponse minus post_id, which that query does
// not return. Kept as its own type rather than shared, because the two feeds
// are free to diverge and a shared type would silently couple them.
type CharacterMessageResponse struct {
	ID                 int32   `json:"id" doc:"Message ID"`
	GameID             int32   `json:"game_id"`
	ParentID           *int32  `json:"parent_id" doc:"Message being replied to, null for a post"`
	AuthorID           int32   `json:"author_id"`
	CharacterID        int32   `json:"character_id"`
	Content            string  `json:"content" doc:"Body, as markdown"`
	MessageType        string  `json:"message_type" doc:"\"post\" or \"comment\""`
	CreatedAt          string  `json:"created_at" doc:"RFC3339 timestamp"`
	EditedAt           *string `json:"edited_at" doc:"RFC3339 timestamp, null when never edited"`
	EditCount          int32   `json:"edit_count"`
	DeletedAt          *string `json:"deleted_at" doc:"RFC3339 timestamp, null when not deleted"`
	IsDeleted          bool    `json:"is_deleted"`
	AuthorUsername     string  `json:"author_username" doc:"Blank in an anonymous game when the caller may not see it"`
	CharacterName      *string `json:"character_name"`
	CharacterAvatarURL *string `json:"character_avatar_url"`

	Parent *ParentContextResponse `json:"parent,omitempty" required:"false" doc:"The message being replied to, for comments"`
}

// PaginationResponse is the {limit, offset, total} envelope shared by the two
// paginated feeds.
type PaginationResponse struct {
	Limit  int   `json:"limit" doc:"Rows requested"`
	Offset int   `json:"offset" doc:"Rows skipped"`
	Total  int64 `json:"total" doc:"Rows matching the query, across all pages"`
}

// RecentCommentsResponse is the body of the "New Comments" view.
type RecentCommentsResponse struct {
	Comments   []*CommentWithParentResponse `json:"comments"`
	Pagination PaginationResponse           `json:"pagination"`
}

// CharacterMessagesResponse is the body of a character's activity feed.
type CharacterMessagesResponse struct {
	Messages   []*CharacterMessageResponse `json:"messages"`
	Pagination PaginationResponse          `json:"pagination"`
}

// ReadMarkerResponse records how far a user has read in one post's thread.
type ReadMarkerResponse struct {
	ID                int32     `json:"id" doc:"Read marker ID"`
	UserID            int32     `json:"user_id" doc:"Reader"`
	GameID            int32     `json:"game_id"`
	PostID            int32     `json:"post_id" doc:"Thread this marker tracks"`
	LastReadCommentID *int32    `json:"last_read_comment_id" doc:"Newest comment read; null when only the post was read"`
	LastReadAt        time.Time `json:"last_read_at" doc:"When the thread was last opened"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// PostUnreadInfoResponse is the per-post metadata the client compares against
// its read markers to decide what to badge as unread.
type PostUnreadInfoResponse struct {
	PostID        int32     `json:"post_id"`
	PostCreatedAt time.Time `json:"post_created_at"`
	TotalComments int64     `json:"total_comments" doc:"Comments on the post, nested replies included"`

	// Added by the chi handler only when the post has comments.
	LatestCommentAt *time.Time `json:"latest_comment_at,omitempty" required:"false" doc:"Newest comment's timestamp, absent when the post has no comments"`
}

// PostUnreadCommentsResponse lists which comments in a post are unread.
type PostUnreadCommentsResponse struct {
	PostID           int32   `json:"post_id"`
	UnreadCommentIDs []int32 `json:"unread_comment_ids" doc:"Comments newer than the caller's read marker"`
}

// ManualReadCommentIDsResponse represents the manual read comment IDs for a post
type ManualReadCommentIDsResponse struct {
	PostID         int32   `json:"post_id"`
	ReadCommentIDs []int32 `json:"read_comment_ids" doc:"Comments the caller explicitly marked read"`
}

// DeletedResponse is the {"message": ...} / {"message": ..., "id": ...}
// envelope the delete endpoints return. Note both are 200 with a body, not 204.
type DeletedResponse struct {
	Message string `json:"message" doc:"Confirmation message"`
	ID      *int32 `json:"id,omitempty" required:"false" doc:"ID of the deleted record; absent for draft posts, which are keyed by phase"`
}
