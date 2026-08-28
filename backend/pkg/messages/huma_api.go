package messages

// Huma (type-first) implementation of the message API.
//
// Three registration functions, because messages are mounted at three prefixes:
// the common-room routes under /games/{gameID}, a character's activity feed
// under /characters, and the phase draft-post routes under /phases.
// See .claude/planning/huma-migration.md gotcha 10.

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
)

// Input types

type gameIDInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
}

type createPostInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
	Body   *CreatePostRequest
}

type listPostsInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
	// Read leniently: the chi handler ignored anything it could not parse and
	// fell back to the default rather than rejecting, so no minimum/maximum
	// tags here -- adding them would start 400ing links that work today. The
	// clamping happens in the handler; see humaGetGamePosts.
	//
	// These are plain values rather than pointers because huma panics on a
	// pointer query parameter. Zero is indistinguishable from omitted, which is
	// harmless here: the handler's guards reject 0 for limit and accept it for
	// offset, exactly as an omitted value would behave.
	PhaseID int32 `query:"phase_id" required:"false" doc:"Restrict to one phase"`
	Limit   int32 `query:"limit" required:"false" doc:"Maximum posts to return, 1-100. Out-of-range values fall back to 50 rather than erroring."`
	Offset  int32 `query:"offset" required:"false" doc:"Posts to skip. A negative value falls back to 0."`
}

type postIDInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
	PostID int32 `path:"postId" doc:"Post ID"`
}

type createCommentInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
	// The immediate parent: a post for a top-level comment, or another comment
	// for a nested reply.
	PostID int32 `path:"postId" doc:"Message being replied to"`
	Body   *CreateCommentRequest
}

type commentsWithThreadsInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
	PostID int32 `path:"postId" doc:"Post ID"`
	Limit  int32 `query:"limit" default:"5" minimum:"1" maximum:"500" doc:"Top-level comments to return"`
	Offset int32 `query:"offset" default:"0" minimum:"0" doc:"Top-level comments to skip"`
	// Defaulted in the handler rather than by a tag, because the fallback is a
	// server config value rather than a constant. -1 is the sentinel for
	// "omitted"; the real range starts at 0.
	MaxDepth int32 `query:"max_depth" default:"-1" minimum:"-1" maximum:"10" doc:"Deepest reply level to include, 0-10. Defaults to the server's configured comment depth."`
}

type updatePostInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
	PostID int32 `path:"postId" doc:"Post ID"`
	Body   *UpdatePostRequest
}

type commentIDInput struct {
	GameID    int32 `path:"gameID" doc:"Game ID"`
	PostID    int32 `path:"postId" doc:"Post the comment belongs to"`
	CommentID int32 `path:"commentId" doc:"Comment ID"`
}

type updateCommentInput struct {
	GameID    int32 `path:"gameID" doc:"Game ID"`
	PostID    int32 `path:"postId" doc:"Post the comment belongs to"`
	CommentID int32 `path:"commentId" doc:"Comment ID"`
	Body      *UpdateCommentRequest
}

type messageIDInput struct {
	GameID    int32 `path:"gameID" doc:"Game ID"`
	MessageID int32 `path:"messageId" doc:"Message ID"`
}

type threadContextInput struct {
	GameID     int32 `path:"gameID" doc:"Game ID"`
	MessageID  int32 `path:"messageId" doc:"Message to centre the chain on"`
	MaxParents int32 `query:"max_parents" default:"3" minimum:"0" maximum:"20" doc:"Ancestor levels to include above the target"`
}

type recentCommentsInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
	// Clamped rather than rejected above 50, but rejected below 1 -- the chi
	// handler's asymmetry, preserved. minimum:"1" now expresses the reject half
	// in the schema; the clamp stays in the handler because huma has no
	// "clamp" tag.
	Limit  int32 `query:"limit" default:"10" minimum:"1" doc:"Comments to return; values above 50 are clamped to 50"`
	Offset int32 `query:"offset" default:"0" minimum:"0" doc:"Comments to skip"`
	// Restricts the list to comments the caller has not manually marked read.
	// Needs an identified caller, unlike the default listing.
	UnreadOnly bool `query:"unread_only" required:"false" doc:"Return only comments the caller has not marked read"`
}

type markPostReadInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
	PostID int32 `path:"postId" doc:"Post being marked read"`
	// Optional: the chi handler tolerated an empty body.
	Body *MarkPostReadRequest
}

type toggleCommentReadInput struct {
	GameID    int32 `path:"gameID" doc:"Game ID"`
	PostID    int32 `path:"postId" doc:"Post the comment belongs to"`
	CommentID int32 `path:"commentId" doc:"Comment to toggle"`
	Body      *ToggleCommentReadRequest
}

type markAllCommentsReadInput struct {
	GameID  int32 `path:"gameID" doc:"Game ID"`
	PhaseID int32 `path:"phaseId" doc:"Phase whose comments to mark read"`
}

type characterCommentsInput struct {
	ID     int32 `path:"id" doc:"Character ID"`
	Limit  int32 `query:"limit" default:"20" minimum:"1" doc:"Messages to return; values above 50 are clamped to 50"`
	Offset int32 `query:"offset" default:"0" minimum:"0" doc:"Messages to skip"`
}

type phaseIDInput struct {
	ID int32 `path:"id" doc:"Phase ID"`
}

type createDraftPostInput struct {
	ID   int32 `path:"id" doc:"Phase ID"`
	Body *CreateDraftPostRequest
}

type updateDraftPostInput struct {
	ID   int32 `path:"id" doc:"Phase ID"`
	Body *UpdateDraftPostRequest
}

// Output types

type messageOutput struct {
	Body *MessageResponse
}

// draftPostOutput carries a nullable body: the chi handler answered 200 with a
// literal `null` when the phase has no draft, rather than 404. The frontend
// branches on the null, so this is preserved -- see humaGetDraftPost.
type draftPostOutput struct {
	Body *MessageResponse
}

type postListOutput struct {
	Body []*PostSummaryResponse
}

type commentListOutput struct {
	Body []*CommentSummaryResponse
}

type paginatedCommentsOutput struct {
	Body *PaginatedCommentsResponse
}

type threadContextOutput struct {
	Body *MessageThreadContextResponse
}

type recentCommentsOutput struct {
	Body *RecentCommentsResponse
}

type characterMessagesOutput struct {
	Body *CharacterMessagesResponse
}

type readMarkerOutput struct {
	Body *ReadMarkerResponse
}

type readMarkerListOutput struct {
	Body []*ReadMarkerResponse
}

type postsUnreadInfoOutput struct {
	Body []*PostUnreadInfoResponse
}

type unreadCommentIDsOutput struct {
	Body []*PostUnreadCommentsResponse
}

type manualReadCommentIDsOutput struct {
	Body []ManualReadCommentIDsResponse
}

type deletedOutput struct {
	Body *DeletedResponse
}

// Helpers

// humaErr converts a core error response into the equivalent huma error,
// preserving the status and message the chi handlers produced.
func humaErr(errResp any) error {
	if resp, ok := errResp.(*core.ErrResponse); ok {
		return huma.NewError(resp.HTTPStatusCode, resp.ErrorText)
	}
	return huma.Error500InternalServerError("unexpected error")
}

func (h *Handler) authUser(ctx context.Context) (int32, error) {
	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get user from token")
		return 0, humaErr(errResp)
	}
	return userID, nil
}

// showUsernames reports whether the caller may see who is behind each message.
// In an anonymous game regular players may not; the username is then blanked to
// "" rather than omitted, because the field is non-optional to clients.
func (h *Handler) showUsernames(ctx context.Context, gameID int32) (bool, error) {
	game, err := models.New(h.App.Pool).GetGame(ctx, gameID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get game", "error", err, "game_id", gameID)
		return false, huma.Error500InternalServerError(err.Error())
	}
	// Unauthenticated callers get userID 0, which no game grants privilege to.
	userID, _ := core.GetUserIDFromJWT(ctx, h.UserService)
	return core.CanSeeUsernamesInAnonymousGame(ctx, h.App.Pool, game, userID), nil
}

// blankIfHidden applies the anonymity rule to one username.
func blankIfHidden(username string, show bool) string {
	if show {
		return username
	}
	return ""
}

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

// clampedLimit caps a feed limit at 50. Values below 1 are rejected by the
// schema's minimum before the handler runs, so only the clamp is left here --
// the chi handlers clamped rather than rejected at the top end, and a client
// asking for 200 still gets a page rather than a 400.
func clampedLimit(v int32) int {
	if v > 50 {
		return 50
	}
	return int(v)
}

// commentSummary converts a comment row into the flat list shape shared by the
// per-post list and the threaded view.
func commentSummary(c *core.MessageWithDetails, show bool) *CommentSummaryResponse {
	item := &CommentSummaryResponse{
		ID:                    c.ID,
		GameID:                c.GameID,
		AuthorID:              c.AuthorID,
		CharacterID:           c.CharacterID,
		Content:               c.Content,
		MessageType:           string(c.MessageType),
		ThreadDepth:           c.ThreadDepth,
		AuthorUsername:        blankIfHidden(c.AuthorUsername, show),
		CharacterName:         c.CharacterName,
		CharacterAvatarURL:    c.CharacterAvatarUrl,
		ReplyCount:            c.ReplyCount,
		IsEdited:              c.IsEdited,
		IsDeleted:             c.IsDeleted,
		MentionedCharacterIDs: c.MentionedCharacterIds,
		CreatedAt:             c.CreatedAt.Time,
	}
	if c.PhaseID.Valid {
		id := c.PhaseID.Int32
		item.PhaseID = &id
	}
	if c.ParentID.Valid {
		id := c.ParentID.Int32
		item.ParentID = &id
	}
	return item
}

// readMarkerResponse converts a read marker row into its response body.
func readMarkerResponse(m *core.ReadMarker) *ReadMarkerResponse {
	return &ReadMarkerResponse{
		ID:                m.ID,
		UserID:            m.UserID,
		GameID:            m.GameID,
		PostID:            m.PostID,
		LastReadCommentID: m.LastReadCommentID,
		LastReadAt:        m.LastReadAt,
		CreatedAt:         m.CreatedAt,
		UpdatedAt:         m.UpdatedAt,
	}
}

// Common room posts

func (h *Handler) humaCreatePost(ctx context.Context, in *createPostInput) (*messageOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_create_post")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	// The chi route was wrapped in RequireEmailVerificationMiddleware. Huma
	// handlers take a context rather than a *http.Request, so the same check
	// runs inline here via its context-based twin.
	if errResp := core.RequireVerifiedEmailCtx(ctx, h.App.Pool); errResp != nil {
		h.App.ObsLogger.Warn(ctx, "Create post blocked by email verification", "user_id", userID)
		return nil, humaErr(errResp)
	}

	// GameMiddleware resolves this for the whole /games/{gameID} subtree.
	if isGM, _ := ctx.Value("is_gm").(bool); !isGM {
		h.App.ObsLogger.Warn(ctx, "Non-GM/co-GM user attempted to create post", "user_id", userID, "game_id", in.GameID)
		return nil, huma.Error403Forbidden("Only the Game Master or co-GM can create posts")
	}

	post, err := h.MessageService.CreatePost(ctx, core.CreatePostRequest{
		GameID:      in.GameID,
		PhaseID:     in.Body.PhaseID,
		AuthorID:    userID,
		CharacterID: in.Body.CharacterID,
		Content:     in.Body.Content,
		Visibility:  "game", // Common Room posts are always visible to game
	})
	if err != nil {
		if core.IsArchivedGameError(err) {
			h.App.ObsLogger.Warn(ctx, "Create post rejected: game is archived", "game_id", in.GameID, "user_id", userID)
			return nil, humaErr(core.ErrGameArchived())
		}
		h.App.ObsLogger.Error(ctx, "Failed to create post", "error", err, "game_id", in.GameID, "user_id", userID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	h.App.ObsLogger.Info(ctx, "Post created successfully", "post_id", post.ID, "game_id", in.GameID, "author_id", userID)

	// Re-read so the response carries the joined author/character metadata the
	// insert does not return.
	postDetails, err := h.MessageService.GetPost(ctx, post.ID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to fetch post details", "error", err, "post_id", post.ID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &messageOutput{Body: messageWithDetailsToResponse(postDetails)}, nil
}

// humaGetGamePosts lists the common room.
//
// The three query parameters are read leniently, matching the chi handler: an
// unparseable phase_id, a limit outside 1-100, or a negative offset all fall
// back to the default instead of erroring. That is deliberate here rather than
// an oversight to fix (contrast the threaded-comments endpoint, which rejects
// the same mistakes) -- tightening it would start 400ing links that work today.
func (h *Handler) humaGetGamePosts(ctx context.Context, in *listPostsInput) (*postListOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_game_posts")()

	limit := int32(50)
	if in.Limit > 0 && in.Limit <= 100 {
		limit = in.Limit
	}
	offset := int32(0)
	if in.Offset >= 0 {
		offset = in.Offset
	}

	// phase_id is optional; 0 is not a valid id, so it doubles as "unset".
	var phaseID *int32
	if in.PhaseID != 0 {
		id := in.PhaseID
		phaseID = &id
	}

	show, err := h.showUsernames(ctx, in.GameID)
	if err != nil {
		return nil, err
	}

	posts, err := h.MessageService.GetGamePosts(ctx, in.GameID, phaseID, limit, offset)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get game posts", "error", err, "game_id", in.GameID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	// Built as an empty slice rather than a nil one so a game with no posts
	// encodes as [] rather than null.
	resp := make([]*PostSummaryResponse, 0, len(posts))
	for _, post := range posts {
		item := &PostSummaryResponse{
			ID:                 post.ID,
			GameID:             post.GameID,
			AuthorID:           post.AuthorID,
			CharacterID:        post.CharacterID,
			Content:            post.Content,
			MessageType:        string(post.MessageType),
			ThreadDepth:        post.ThreadDepth,
			AuthorUsername:     blankIfHidden(post.AuthorUsername, show),
			CharacterName:      post.CharacterName,
			CharacterAvatarURL: post.CharacterAvatarUrl,
			CommentCount:       post.CommentCount,
			IsEdited:           post.IsEdited,
			IsDeleted:          post.IsDeleted,
			CreatedAt:          post.CreatedAt.Time,
		}
		if post.PhaseID.Valid {
			id := post.PhaseID.Int32
			item.PhaseID = &id
		}
		if post.ParentID.Valid {
			id := post.ParentID.Int32
			item.ParentID = &id
		}
		resp = append(resp, item)
	}

	return &postListOutput{Body: resp}, nil
}

func (h *Handler) humaUpdatePost(ctx context.Context, in *updatePostInput) (*messageOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_post")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	canEdit, err := h.MessageService.CanUserEditPost(ctx, in.PostID, userID)
	if err != nil {
		// The service reports a missing post as a row-scan failure rather than a
		// typed error, so the text is all there is to match on.
		if strings.Contains(err.Error(), "no rows") {
			h.App.ObsLogger.Warn(ctx, "Post not found", "post_id", in.PostID)
			return nil, huma.Error404NotFound("post not found")
		}
		h.App.ObsLogger.Error(ctx, "Failed to check edit permission", "error", err, "post_id", in.PostID, "user_id", userID)
		return nil, huma.Error500InternalServerError(err.Error())
	}
	if !canEdit {
		h.App.ObsLogger.Warn(ctx, "User attempted to edit post without permission", "post_id", in.PostID, "user_id", userID)
		return nil, huma.Error403Forbidden("You can only edit your own posts")
	}

	updatedPost, err := h.MessageService.UpdatePost(ctx, in.PostID, in.Body.Content)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to update post", "error", err, "post_id", in.PostID, "user_id", userID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	h.App.ObsLogger.Info(ctx, "Post updated successfully", "post_id", in.PostID, "user_id", userID, "edit_count", updatedPost.EditCount)

	postDetails, err := h.MessageService.GetPost(ctx, updatedPost.ID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to fetch updated post details", "error", err, "post_id", updatedPost.ID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &messageOutput{Body: messageWithDetailsToResponse(postDetails)}, nil
}

// Comments

func (h *Handler) humaCreateComment(ctx context.Context, in *createCommentInput) (*messageOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_create_comment")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	if errResp := core.RequireVerifiedEmailCtx(ctx, h.App.Pool); errResp != nil {
		h.App.ObsLogger.Warn(ctx, "Create comment blocked by email verification", "user_id", userID)
		return nil, humaErr(errResp)
	}

	// {postId} in the path is the immediate parent (post or comment).
	// root_post_id in the body is the top-level post, needed for read tracking.
	// Replying directly to a post makes them the same, so postId is the fallback.
	rootPostID := in.PostID
	if in.Body.RootPostID != nil {
		rootPostID = *in.Body.RootPostID
	}

	comment, err := h.MessageService.CreateComment(ctx, core.CreateCommentRequest{
		GameID:      in.GameID,
		PhaseID:     in.Body.PhaseID,
		AuthorID:    userID,
		CharacterID: in.Body.CharacterID,
		Content:     in.Body.Content,
		ParentID:    in.PostID,
		RootPostID:  rootPostID,
		Visibility:  "game",
	})
	if err != nil {
		if core.IsArchivedGameError(err) {
			h.App.ObsLogger.Warn(ctx, "Create comment rejected: game is archived", "game_id", in.GameID, "post_id", in.PostID, "user_id", userID)
			return nil, humaErr(core.ErrGameArchived())
		}
		h.App.ObsLogger.Error(ctx, "Failed to create comment", "error", err, "game_id", in.GameID, "post_id", in.PostID, "user_id", userID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	h.App.ObsLogger.Info(ctx, "Comment created successfully", "comment_id", comment.ID, "post_id", in.PostID, "author_id", userID)

	commentDetails, err := h.MessageService.GetComment(ctx, comment.ID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to fetch comment details", "error", err, "comment_id", comment.ID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &messageOutput{Body: messageWithDetailsToResponse(commentDetails)}, nil
}

func (h *Handler) humaGetPostComments(ctx context.Context, in *postIDInput) (*commentListOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_post_comments")()

	show, err := h.showUsernames(ctx, in.GameID)
	if err != nil {
		return nil, err
	}

	comments, err := h.MessageService.GetPostComments(ctx, in.PostID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get post comments", "error", err, "post_id", in.PostID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	resp := make([]*CommentSummaryResponse, 0, len(comments))
	for i := range comments {
		resp = append(resp, commentSummary(&comments[i], show))
	}

	return &commentListOutput{Body: resp}, nil
}

// humaGetPostCommentsWithThreads returns top-level comments with their nested
// replies flattened, for the paginated thread view.
//
// Comments come back at depths 0 through max_depth-1 so a Reply button can
// appear on every visible comment; one at the deepest level that still has
// replies is where the client shows "Continue thread".
func (h *Handler) humaGetPostCommentsWithThreads(ctx context.Context, in *commentsWithThreadsInput) (*paginatedCommentsOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_post_comments_with_threads")()

	maxDepth := int32(h.App.Config.App.CommentMaxDepth)
	if in.MaxDepth >= 0 {
		maxDepth = in.MaxDepth
	}

	show, err := h.showUsernames(ctx, in.GameID)
	if err != nil {
		return nil, err
	}

	commentsWithDepth, err := h.MessageService.GetPostCommentsWithThreads(ctx, in.PostID, in.Limit, in.Offset, maxDepth)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get post comments with threads", "error", err, "post_id", in.PostID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	totalCount, err := h.MessageService.CountTopLevelComments(ctx, in.PostID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to count top-level comments", "error", err, "post_id", in.PostID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	comments := make([]*ThreadedCommentResponse, 0, len(commentsWithDepth))
	for i := range commentsWithDepth {
		comments = append(comments, &ThreadedCommentResponse{
			CommentSummaryResponse: *commentSummary(&commentsWithDepth[i].Comment, show),
			Depth:                  commentsWithDepth[i].Depth,
		})
	}

	return &paginatedCommentsOutput{Body: &PaginatedCommentsResponse{
		Comments:      comments,
		TotalTopLevel: totalCount,
		Limit:         in.Limit,
		Offset:        in.Offset,
		HasMore:       totalCount > int64(in.Offset+in.Limit),
		// Counts only depth-0 rows: the page size is measured in top-level
		// comments, but the payload also carries their nested replies.
		ReturnedTopLevel: countTopLevelInResponse(commentsWithDepth),
		ReturnedTotal:    len(comments),
	}}, nil
}

func (h *Handler) humaUpdateComment(ctx context.Context, in *updateCommentInput) (*messageOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_comment")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	canEdit, err := h.MessageService.CanUserEditComment(ctx, in.CommentID, userID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to check edit permission", "error", err, "comment_id", in.CommentID, "user_id", userID)
		return nil, huma.Error500InternalServerError(err.Error())
	}
	if !canEdit {
		h.App.ObsLogger.Warn(ctx, "User attempted to edit comment without permission", "comment_id", in.CommentID, "user_id", userID)
		return nil, huma.Error403Forbidden("You can only edit your own comments")
	}

	updatedComment, err := h.MessageService.UpdateComment(ctx, in.CommentID, in.Body.Content, in.Body.CharacterID)
	if err != nil {
		// Re-attributing to a character the caller does not control is a
		// permission failure, not a server fault.
		if errors.Is(err, core.ErrCharacterNotControlled) {
			h.App.ObsLogger.Warn(ctx, "User attempted to use character they don't control", "comment_id", in.CommentID, "user_id", userID)
			return nil, huma.Error403Forbidden("You do not control this character")
		}
		h.App.ObsLogger.Error(ctx, "Failed to update comment", "error", err, "comment_id", in.CommentID, "user_id", userID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	h.App.ObsLogger.Info(ctx, "Comment updated successfully", "comment_id", in.CommentID, "user_id", userID, "edit_count", updatedComment.EditCount)

	commentDetails, err := h.MessageService.GetComment(ctx, updatedComment.ID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to fetch updated comment details", "error", err, "comment_id", updatedComment.ID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &messageOutput{Body: messageWithDetailsToResponse(commentDetails)}, nil
}

func (h *Handler) humaDeleteComment(ctx context.Context, in *commentIDInput) (*deletedOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_comment")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	user, err := h.UserService.GetUserByID(int(userID))
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get user", "error", err, "user_id", userID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	// Admin mode is opt-in per request, so an admin who has not enabled it is
	// held to the same rules as anyone else.
	isAdmin := core.GetAdminMode(ctx) && user.IsAdmin

	canDelete, err := h.MessageService.CanUserDeleteComment(ctx, in.CommentID, userID, isAdmin)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to check delete permission", "error", err, "comment_id", in.CommentID, "user_id", userID)
		return nil, huma.Error500InternalServerError(err.Error())
	}
	if !canDelete {
		h.App.ObsLogger.Warn(ctx, "User attempted to delete comment without permission",
			"comment_id", in.CommentID, "user_id", userID, "is_admin", isAdmin)
		return nil, huma.Error403Forbidden("You don't have permission to delete this comment")
	}

	if err := h.MessageService.DeleteComment(ctx, in.CommentID, userID); err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to delete comment", "error", err, "comment_id", in.CommentID, "user_id", userID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	h.App.ObsLogger.Info(ctx, "Comment deleted successfully",
		"comment_id", in.CommentID, "game_id", in.GameID, "deleted_by_user_id", userID, "is_admin", isAdmin)

	id := in.CommentID
	return &deletedOutput{Body: &DeletedResponse{
		Message: "Comment deleted successfully",
		ID:      &id,
	}}, nil
}

// Deep linking

func (h *Handler) humaGetMessage(ctx context.Context, in *messageIDInput) (*messageOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_message")()

	show, err := h.showUsernames(ctx, in.GameID)
	if err != nil {
		return nil, err
	}

	message, err := h.MessageService.GetMessage(ctx, in.MessageID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get message", "error", err, "message_id", in.MessageID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	resp := messageWithDetailsToResponse(message)
	resp.AuthorUsername = blankIfHidden(resp.AuthorUsername, show)

	return &messageOutput{Body: resp}, nil
}

// humaGetMessageThreadContext returns a message plus a bounded slice of its
// ancestor chain and the true root post ID, in one request.
//
// The chain is capped so a deep link into a long thread does not have to walk
// the whole ancestry; root_post_id still names the real top-level post even
// when the chain was trimmed short of it.
func (h *Handler) humaGetMessageThreadContext(ctx context.Context, in *threadContextInput) (*threadContextOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_message_thread_context")()

	show, err := h.showUsernames(ctx, in.GameID)
	if err != nil {
		return nil, err
	}

	threadCtx, err := h.MessageService.GetMessageWithParentContext(ctx, in.MessageID, in.MaxParents)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get message thread context", "error", err, "message_id", in.MessageID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	chain := make([]*MessageResponse, len(threadCtx.Chain))
	for i := range threadCtx.Chain {
		resp := messageWithDetailsToResponse(&threadCtx.Chain[i])
		resp.AuthorUsername = blankIfHidden(resp.AuthorUsername, show)
		chain[i] = resp
	}

	return &threadContextOutput{Body: &MessageThreadContextResponse{
		Chain:         chain,
		RootPostID:    threadCtx.RootPostID,
		HasFullThread: threadCtx.HasFullThread,
	}}, nil
}

// Activity feeds

func (h *Handler) humaListRecentComments(ctx context.Context, in *recentCommentsInput) (*recentCommentsOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_list_recent_comments_with_parents")()

	limit := clampedLimit(in.Limit)
	offset := int(in.Offset)

	show, err := h.showUsernames(ctx, in.GameID)
	if err != nil {
		return nil, err
	}

	// The default listing works for any viewer; unread_only is per-user and so
	// needs an identified caller.
	userID, userIDErr := core.GetUserIDFromJWT(ctx, h.UserService)
	if in.UnreadOnly && userIDErr != nil {
		h.App.ObsLogger.Warn(ctx, "Invalid list recent comments with parents request", "game_id", in.GameID)
		return nil, huma.Error401Unauthorized("authentication required for unread_only")
	}

	var comments []core.CommentWithParent
	var totalCount int64
	var svcErr error
	if in.UnreadOnly {
		comments, svcErr = h.MessageService.ListRecentUnreadCommentsWithParents(ctx, in.GameID, userID, int32(limit), int32(offset))
		if svcErr != nil {
			h.App.ObsLogger.Error(ctx, "Failed to list recent unread comments", "error", svcErr, "game_id", in.GameID, "user_id", userID)
			return nil, huma.Error500InternalServerError(svcErr.Error())
		}
		totalCount, svcErr = h.MessageService.GetTotalUnreadCommentCount(ctx, in.GameID, userID)
	} else {
		comments, svcErr = h.MessageService.ListRecentCommentsWithParents(ctx, in.GameID, int32(limit), int32(offset))
		if svcErr != nil {
			h.App.ObsLogger.Error(ctx, "Failed to list recent comments", "error", svcErr, "game_id", in.GameID)
			return nil, huma.Error500InternalServerError(svcErr.Error())
		}
		totalCount, svcErr = h.MessageService.GetTotalCommentCount(ctx, in.GameID)
	}
	if svcErr != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get total comment count", "error", svcErr, "game_id", in.GameID)
		return nil, huma.Error500InternalServerError(svcErr.Error())
	}

	h.App.ObsLogger.Info(ctx, "Listed recent comments with parents",
		"game_id", in.GameID, "limit", limit, "offset", offset,
		"unread_only", in.UnreadOnly, "count", len(comments), "total", totalCount)

	return &recentCommentsOutput{Body: &RecentCommentsResponse{
		Comments:   commentsWithParentsToResponse(comments, show),
		Pagination: PaginationResponse{Limit: limit, Offset: offset, Total: totalCount},
	}}, nil
}

func (h *Handler) humaGetCharacterComments(ctx context.Context, in *characterCommentsInput) (*characterMessagesOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_character_comments")()

	limit := clampedLimit(in.Limit)
	offset := int(in.Offset)

	queries := models.New(h.App.Pool)
	character, err := queries.GetCharacter(ctx, in.ID)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get character", "error", err, "character_id", in.ID)
		return nil, huma.Error404NotFound("character not found")
	}

	show, err := h.showUsernames(ctx, character.GameID)
	if err != nil {
		return nil, err
	}

	messages, err := h.MessageService.ListCharacterPostsAndComments(ctx, in.ID, int32(limit), int32(offset))
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to list character messages", "error", err, "character_id", in.ID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	totalCount, err := h.MessageService.CountCharacterPostsAndComments(ctx, in.ID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to count character messages", "error", err, "character_id", in.ID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	result := make([]*CharacterMessageResponse, len(messages))
	for i := range messages {
		msg := &messages[i]
		item := &CharacterMessageResponse{
			ID:                 msg.ID,
			GameID:             msg.GameID,
			ParentID:           msg.ParentID,
			AuthorID:           msg.AuthorID,
			CharacterID:        msg.CharacterID,
			Content:            msg.Content,
			MessageType:        msg.MessageType,
			CreatedAt:          msg.CreatedAt.Format(time.RFC3339),
			EditedAt:           formatTimePtr(msg.EditedAt),
			EditCount:          msg.EditCount,
			DeletedAt:          formatTimePtr(msg.DeletedAt),
			IsDeleted:          msg.IsDeleted,
			AuthorUsername:     blankIfHidden(msg.AuthorUsername, show),
			CharacterName:      msg.CharacterName,
			CharacterAvatarURL: msg.CharacterAvatarUrl,
		}

		if msg.ParentContent != nil {
			// Note this feed nils the hidden parent username, where the "New
			// Comments" feed blanks it to "". Both are preserved as they were.
			parentAuthor := msg.ParentAuthorUsername
			if !show {
				parentAuthor = nil
			}
			item.Parent = &ParentContextResponse{
				Content:            msg.ParentContent,
				CreatedAt:          formatTimePtr(msg.ParentCreatedAt),
				DeletedAt:          formatTimePtr(msg.ParentDeletedAt),
				IsDeleted:          msg.ParentIsDeleted,
				MessageType:        msg.ParentMessageType,
				AuthorUsername:     parentAuthor,
				CharacterName:      msg.ParentCharacterName,
				CharacterAvatarURL: msg.ParentCharacterAvatarUrl,
			}
		}

		result[i] = item
	}

	return &characterMessagesOutput{Body: &CharacterMessagesResponse{
		Messages:   result,
		Pagination: PaginationResponse{Limit: limit, Offset: offset, Total: totalCount},
	}}, nil
}

// Read tracking

func (h *Handler) humaMarkPostRead(ctx context.Context, in *markPostReadInput) (*readMarkerOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_mark_post_read")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	// The body is optional: marking a post read without naming a comment is a
	// valid request, and the chi handler accepted an empty body via io.EOF.
	var lastReadCommentID *int32
	if in.Body != nil {
		lastReadCommentID = in.Body.LastReadCommentID
	}

	readMarker, err := h.MessageService.MarkPostAsRead(ctx, userID, in.GameID, in.PostID, lastReadCommentID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to mark post as read", "error", err, "game_id", in.GameID, "post_id", in.PostID, "user_id", userID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &readMarkerOutput{Body: readMarkerResponse(readMarker)}, nil
}

func (h *Handler) humaGetGameReadMarkers(ctx context.Context, in *gameIDInput) (*readMarkerListOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_game_read_markers")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	readMarkers, err := h.MessageService.GetUserReadMarkersForGame(ctx, userID, in.GameID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get read markers", "error", err, "game_id", in.GameID, "user_id", userID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	resp := make([]*ReadMarkerResponse, 0, len(readMarkers))
	for i := range readMarkers {
		resp = append(resp, readMarkerResponse(readMarkers[i]))
	}

	return &readMarkerListOutput{Body: resp}, nil
}

func (h *Handler) humaGetPostsUnreadInfo(ctx context.Context, in *gameIDInput) (*postsUnreadInfoOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_posts_unread_info")()

	postsInfo, err := h.MessageService.GetPostsWithUnreadInfo(ctx, in.GameID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get posts unread info", "error", err, "game_id", in.GameID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	resp := make([]*PostUnreadInfoResponse, 0, len(postsInfo))
	for _, info := range postsInfo {
		resp = append(resp, &PostUnreadInfoResponse{
			PostID:        info.PostID,
			PostCreatedAt: info.PostCreatedAt,
			TotalComments: info.TotalComments,
			// Absent, not null, when the post has no comments -- the chi
			// handler added this key only inside an if.
			LatestCommentAt: info.LatestCommentAt,
		})
	}

	return &postsUnreadInfoOutput{Body: resp}, nil
}

func (h *Handler) humaGetUnreadCommentIDs(ctx context.Context, in *gameIDInput) (*unreadCommentIDsOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_unread_comment_ids")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	unreadComments, err := h.MessageService.GetUnreadCommentIDsForPosts(ctx, userID, in.GameID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get unread comment IDs", "error", err, "game_id", in.GameID, "user_id", userID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	resp := make([]*PostUnreadCommentsResponse, 0, len(unreadComments))
	for _, uc := range unreadComments {
		resp = append(resp, &PostUnreadCommentsResponse{
			PostID:           uc.PostID,
			UnreadCommentIDs: uc.UnreadCommentIDs,
		})
	}

	return &unreadCommentIDsOutput{Body: resp}, nil
}

func (h *Handler) humaToggleCommentRead(ctx context.Context, in *toggleCommentReadInput) (*struct{}, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_toggle_comment_read")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	// An omitted body means read:false, matching the chi handler's zero value.
	read := false
	if in.Body != nil {
		read = in.Body.Read
	}

	if err := h.MessageService.ToggleCommentRead(ctx, userID, in.GameID, in.PostID, in.CommentID, read); err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to toggle comment read", "error", err,
			"game_id", in.GameID, "post_id", in.PostID, "comment_id", in.CommentID, "user_id", userID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return nil, nil
}

func (h *Handler) humaMarkAllCommentsRead(ctx context.Context, in *markAllCommentsReadInput) (*struct{}, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_mark_all_comments_read")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.MessageService.MarkAllCommentsReadForPhase(ctx, userID, in.GameID, in.PhaseID); err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to mark all comments read for phase", "error", err,
			"game_id", in.GameID, "phase_id", in.PhaseID, "user_id", userID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return nil, nil
}

func (h *Handler) humaGetManualReadCommentIDs(ctx context.Context, in *gameIDInput) (*manualReadCommentIDsOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_manual_read_comment_ids")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	manualReads, err := h.MessageService.GetManualReadCommentIDsForGame(ctx, userID, in.GameID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get manual read comment IDs", "error", err, "game_id", in.GameID, "user_id", userID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	resp := make([]ManualReadCommentIDsResponse, 0, len(manualReads))
	for _, mr := range manualReads {
		resp = append(resp, ManualReadCommentIDsResponse{
			PostID:         mr.PostID,
			ReadCommentIDs: mr.ReadCommentIDs,
		})
	}

	return &manualReadCommentIDsOutput{Body: resp}, nil
}

// Draft posts

// humaGetDraftPost returns the phase's draft post.
//
// A phase with no draft answers 200 with a literal `null` body, not 404. The
// doc comment on the chi handler claimed 404, but the code has always rendered
// null and the frontend branches on it, so the behaviour is preserved and the
// spec now documents what actually happens.
func (h *Handler) humaGetDraftPost(ctx context.Context, in *phaseIDInput) (*draftPostOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_draft_post")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	if errResp := requireGMForPhase(ctx, h.App, in.ID, userID); errResp != nil {
		h.App.ObsLogger.Warn(ctx, "GM required for draft post access", "phase_id", in.ID, "user_id", userID)
		return nil, humaErr(errResp)
	}

	draft, err := h.MessageService.GetDraftPostForPhase(ctx, in.ID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get draft post", "error", err, "phase_id", in.ID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	if draft == nil {
		return &draftPostOutput{Body: nil}, nil
	}

	return &draftPostOutput{Body: messageWithDetailsToResponse(draft)}, nil
}

func (h *Handler) humaCreateDraftPost(ctx context.Context, in *createDraftPostInput) (*messageOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_create_draft_post")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	if errResp := requireGMForPhase(ctx, h.App, in.ID, userID); errResp != nil {
		h.App.ObsLogger.Warn(ctx, "GM required for draft post creation", "phase_id", in.ID, "user_id", userID)
		return nil, humaErr(errResp)
	}

	gameID, err := getGameIDForPhase(ctx, h.App, in.ID)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Create draft post not found", "phase_id", in.ID)
		return nil, huma.Error404NotFound("phase not found")
	}

	phaseID := in.ID
	draft, err := h.MessageService.CreateDraftPost(ctx, core.CreatePostRequest{
		GameID:      gameID,
		PhaseID:     &phaseID,
		AuthorID:    userID,
		CharacterID: in.Body.CharacterID,
		Content:     in.Body.Content,
		Visibility:  "game",
	})
	if err != nil {
		// One draft per phase, so a second create is a conflict rather than an
		// overwrite.
		if errors.Is(err, core.ErrDraftPostExists) {
			h.App.ObsLogger.Warn(ctx, "Create draft post conflict", "phase_id", in.ID)
			return nil, huma.Error409Conflict("a draft post already exists for this phase")
		}
		if core.IsArchivedGameError(err) {
			h.App.ObsLogger.Warn(ctx, "Error in create draft post", "phase_id", in.ID)
			return nil, humaErr(core.ErrGameArchived())
		}
		h.App.ObsLogger.Error(ctx, "Failed to create draft post", "error", err, "phase_id", in.ID, "user_id", userID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	h.App.ObsLogger.Info(ctx, "Draft post created", "phase_id", in.ID, "post_id", draft.ID, "author_id", userID)

	return &messageOutput{Body: messageWithDetailsToResponse(draft)}, nil
}

func (h *Handler) humaUpdateDraftPost(ctx context.Context, in *updateDraftPostInput) (*messageOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_draft_post")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	if errResp := requireGMForPhase(ctx, h.App, in.ID, userID); errResp != nil {
		h.App.ObsLogger.Warn(ctx, "GM required for draft post update", "phase_id", in.ID, "user_id", userID)
		return nil, humaErr(errResp)
	}

	// The draft is addressed by phase, so its row id has to be looked up first.
	existing, err := h.MessageService.GetDraftPostForPhase(ctx, in.ID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to update draft post", "error", err, "phase_id", in.ID)
		return nil, huma.Error500InternalServerError(err.Error())
	}
	if existing == nil {
		h.App.ObsLogger.Warn(ctx, "Update draft post not found", "phase_id", in.ID)
		return nil, huma.Error404NotFound("no draft post for this phase")
	}

	updated, err := h.MessageService.UpdateDraftPost(ctx, existing.ID, in.Body.Content)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to update draft post", "error", err, "phase_id", in.ID, "user_id", userID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	h.App.ObsLogger.Info(ctx, "Draft post updated", "phase_id", in.ID, "post_id", existing.ID, "user_id", userID)

	return &messageOutput{Body: messageWithDetailsToResponse(updated)}, nil
}

func (h *Handler) humaDeleteDraftPost(ctx context.Context, in *phaseIDInput) (*deletedOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_draft_post")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	if errResp := requireGMForPhase(ctx, h.App, in.ID, userID); errResp != nil {
		h.App.ObsLogger.Warn(ctx, "GM required for draft post deletion", "phase_id", in.ID, "user_id", userID)
		return nil, humaErr(errResp)
	}

	existing, err := h.MessageService.GetDraftPostForPhase(ctx, in.ID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to delete draft post", "error", err, "phase_id", in.ID)
		return nil, huma.Error500InternalServerError(err.Error())
	}
	if existing == nil {
		h.App.ObsLogger.Warn(ctx, "Delete draft post not found", "phase_id", in.ID)
		return nil, huma.Error404NotFound("no draft post for this phase")
	}

	if err := h.MessageService.DeleteDraftPost(ctx, existing.ID); err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to delete draft post", "error", err, "phase_id", in.ID, "user_id", userID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	h.App.ObsLogger.Info(ctx, "Draft post deleted", "phase_id", in.ID, "user_id", userID)

	// No id: a draft is addressed by phase, so the chi handler returned only a
	// message here, unlike the comment delete.
	return &deletedOutput{Body: &DeletedResponse{Message: "Draft post deleted successfully"}}, nil
}

// Registration

// RegisterHumaGameMessages registers the common-room operations. Paths are
// relative to the /games/{gameID} subrouter.
func RegisterHumaGameMessages(api huma.API, h *Handler) {
	bearer := []map[string][]string{{"BearerAuth": {}}}
	commonRoom := []string{"Common Room"}

	huma.Register(api, huma.Operation{
		OperationID: "createPost",
		Method:      http.MethodPost,
		Path:        "/posts",
		Summary:     "Create a common room post",
		Description: "Starts a new thread in the common room. GM and co-GM only, and " +
			"requires a verified email.",
		Tags:          commonRoom,
		Security:      bearer,
		DefaultStatus: http.StatusCreated,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid request body"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not the GM or co-GM, or email not verified"},
			"409": {Description: "Game is archived"},
		},
	}, h.humaCreatePost)

	huma.Register(api, huma.Operation{
		OperationID: "listGamePosts",
		Method:      http.MethodGet,
		Path:        "/posts",
		Summary:     "List common room posts",
		Description: "Lists the game's top-level posts. In an anonymous game the author's " +
			"username comes back blank for players who may not see it.",
		Tags:     commonRoom,
		Security: bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaGetGamePosts)

	huma.Register(api, huma.Operation{
		OperationID: "updatePost",
		Method:      http.MethodPatch,
		Path:        "/posts/{postId}",
		Summary:     "Edit a post",
		Description: "Replaces a post's content. Authors only.",
		Tags:        commonRoom,
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid request body"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not the post's author"},
			"404": {Description: "Post not found"},
		},
	}, h.humaUpdatePost)

	huma.Register(api, huma.Operation{
		OperationID: "createComment",
		Method:      http.MethodPost,
		Path:        "/posts/{postId}/comments",
		Summary:     "Comment on a post or another comment",
		Description: "Adds a comment. The path's postId is the immediate parent; send " +
			"root_post_id in the body when replying below a top-level post, or read " +
			"tracking will key off the wrong thread. Requires a verified email.",
		Tags:          commonRoom,
		Security:      bearer,
		DefaultStatus: http.StatusCreated,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid request body"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Email not verified"},
			"409": {Description: "Game is archived"},
		},
	}, h.humaCreateComment)

	huma.Register(api, huma.Operation{
		OperationID: "listPostComments",
		Method:      http.MethodGet,
		Path:        "/posts/{postId}/comments",
		Summary:     "List a post's comments",
		Description: "Returns the post's comments as a flat list. Use comments-with-threads " +
			"for the paginated, depth-annotated view.",
		Tags:     commonRoom,
		Security: bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaGetPostComments)

	huma.Register(api, huma.Operation{
		OperationID: "listPostCommentsWithThreads",
		Method:      http.MethodGet,
		Path:        "/posts/{postId}/comments-with-threads",
		Summary:     "List a post's comments with nested replies",
		Description: "Pages over top-level comments and returns their nested replies " +
			"alongside them, each annotated with its depth. Paging is measured in " +
			"top-level comments, so a page holds more rows than its limit.",
		Tags:     commonRoom,
		Security: bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid limit, offset or max_depth"},
			"401": {Description: "Not authenticated"},
		},
	}, h.humaGetPostCommentsWithThreads)

	huma.Register(api, huma.Operation{
		OperationID: "updateComment",
		Method:      http.MethodPatch,
		Path:        "/posts/{postId}/comments/{commentId}",
		Summary:     "Edit a comment",
		Description: "Replaces a comment's content, and optionally re-attributes it to " +
			"another character the caller controls. Authors only.",
		Tags:     commonRoom,
		Security: bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid request body"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not the comment's author, or the character is not the caller's"},
		},
	}, h.humaUpdateComment)

	huma.Register(api, huma.Operation{
		OperationID: "deleteComment",
		Method:      http.MethodDelete,
		Path:        "/posts/{postId}/comments/{commentId}",
		Summary:     "Delete a comment",
		Description: "Soft-deletes a comment, so replies to it keep their thread. Answers " +
			"200 with a body, not 204.",
		Tags:     commonRoom,
		Security: bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not allowed to delete this comment"},
		},
	}, h.humaDeleteComment)

	huma.Register(api, huma.Operation{
		OperationID: "getMessage",
		Method:      http.MethodGet,
		Path:        "/messages/{messageId}",
		Summary:     "Get one message",
		Description: "Returns a single post or comment, for deep-linking.",
		Tags:        commonRoom,
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaGetMessage)

	huma.Register(api, huma.Operation{
		OperationID: "getMessageThreadContext",
		Method:      http.MethodGet,
		Path:        "/messages/{messageId}/thread-context",
		Summary:     "Get a message with its ancestor chain",
		Description: "Returns the target message plus up to max_parents ancestors, and the " +
			"true root post ID even when the chain was trimmed short of it.",
		Tags:     commonRoom,
		Security: bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid max_parents"},
			"401": {Description: "Not authenticated"},
		},
	}, h.humaGetMessageThreadContext)

	huma.Register(api, huma.Operation{
		OperationID: "listRecentComments",
		Method:      http.MethodGet,
		Path:        "/comments/recent",
		Summary:     "List recent comments with their parents",
		Description: "Backs the New Comments view. Each entry carries the message it " +
			"replies to, so the list renders context without a request per row.",
		Tags:     commonRoom,
		Security: bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid limit or offset"},
			"401": {Description: "Not authenticated, or unread_only used anonymously"},
		},
	}, h.humaListRecentComments)

	// Read tracking

	huma.Register(api, huma.Operation{
		OperationID: "markPostRead",
		Method:      http.MethodPost,
		Path:        "/posts/{postId}/mark-read",
		Summary:     "Mark a post read",
		Description: "Records how far the caller has read in a thread. The body is optional: " +
			"omit it to mark the post itself read without naming a comment.",
		Tags:     commonRoom,
		Security: bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaMarkPostRead)

	huma.Register(api, huma.Operation{
		OperationID: "listGameReadMarkers",
		Method:      http.MethodGet,
		Path:        "/read-markers",
		Summary:     "List the caller's read markers",
		Description: "Returns one marker per thread the caller has opened in this game.",
		Tags:        commonRoom,
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaGetGameReadMarkers)

	huma.Register(api, huma.Operation{
		OperationID: "getPostsUnreadInfo",
		Method:      http.MethodGet,
		Path:        "/posts-unread-info",
		Summary:     "Per-post comment metadata for unread badges",
		Description: "Returns each post's comment count and newest comment time, which the " +
			"client compares against its read markers.",
		Tags:     commonRoom,
		Security: bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaGetPostsUnreadInfo)

	huma.Register(api, huma.Operation{
		OperationID: "getUnreadCommentIDs",
		Method:      http.MethodGet,
		Path:        "/unread-comment-ids",
		Summary:     "Unread comment IDs per post",
		Description: "Lists which comments are newer than the caller's read marker, per post.",
		Tags:        commonRoom,
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaGetUnreadCommentIDs)

	huma.Register(api, huma.Operation{
		OperationID:   "toggleCommentRead",
		Method:        http.MethodPost,
		Path:          "/posts/{postId}/comments/{commentId}/toggle-read",
		Summary:       "Mark one comment read or unread",
		Description:   "Sets the caller's manual read flag on a single comment.",
		Tags:          commonRoom,
		Security:      bearer,
		DefaultStatus: http.StatusNoContent,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaToggleCommentRead)

	huma.Register(api, huma.Operation{
		OperationID: "getManualReadCommentIDs",
		Method:      http.MethodGet,
		Path:        "/manual-read-comment-ids",
		Summary:     "Comment IDs the caller marked read",
		Description: "Lists the comments the caller explicitly marked read, per post.",
		Tags:        commonRoom,
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaGetManualReadCommentIDs)

	huma.Register(api, huma.Operation{
		OperationID:   "markAllCommentsRead",
		Method:        http.MethodPost,
		Path:          "/phases/{phaseId}/mark-all-comments-read",
		Summary:       "Mark every comment in a phase read",
		Description:   "Sets the caller's manual read flag on all of a phase's comments at once.",
		Tags:          commonRoom,
		Security:      bearer,
		DefaultStatus: http.StatusNoContent,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaMarkAllCommentsRead)
}

// RegisterHumaCharacterMessages registers a character's activity feed.
//
// Paths are relative to the characters router's mount point
// (/api/v1/characters).
func RegisterHumaCharacterMessages(api huma.API, h *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "listCharacterComments",
		Method:      http.MethodGet,
		Path:        "/{id}/comments",
		Summary:     "List a character's posts and comments",
		Description: "Returns the character's public activity, each comment carrying the " +
			"message it replies to.",
		Tags:     []string{"Common Room"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid limit or offset"},
			"401": {Description: "Not authenticated"},
			"404": {Description: "Character not found"},
		},
	}, h.humaGetCharacterComments)
}

// RegisterHumaPhaseDraftPosts registers the draft-post operations.
//
// Paths are relative to the phases router's mount point (/api/v1/phases).
func RegisterHumaPhaseDraftPosts(api huma.API, h *Handler) {
	bearer := []map[string][]string{{"BearerAuth": {}}}
	drafts := []string{"Common Room"}

	huma.Register(api, huma.Operation{
		OperationID: "getDraftPost",
		Method:      http.MethodGet,
		Path:        "/{id}/draft-post",
		Summary:     "Get a phase's draft post",
		Description: "Returns the phase's unpublished draft. A phase with no draft answers " +
			"200 with a null body, not 404. GM and co-GM only.",
		Tags:     drafts,
		Security: bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not the GM or co-GM"},
			"404": {Description: "Phase not found"},
		},
	}, h.humaGetDraftPost)

	huma.Register(api, huma.Operation{
		OperationID:   "createDraftPost",
		Method:        http.MethodPost,
		Path:          "/{id}/draft-post",
		Summary:       "Create a phase's draft post",
		Description:   "Starts a draft for a phase. One draft per phase. GM and co-GM only.",
		Tags:          drafts,
		Security:      bearer,
		DefaultStatus: http.StatusCreated,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid request body"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not the GM or co-GM"},
			"404": {Description: "Phase not found"},
			"409": {Description: "A draft already exists for this phase, or the game is archived"},
		},
	}, h.humaCreateDraftPost)

	huma.Register(api, huma.Operation{
		OperationID: "updateDraftPost",
		Method:      http.MethodPut,
		Path:        "/{id}/draft-post",
		Summary:     "Replace a phase's draft post",
		Description: "Rewrites the draft's content. GM and co-GM only.",
		Tags:        drafts,
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid request body"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not the GM or co-GM"},
			"404": {Description: "Phase has no draft post"},
		},
	}, h.humaUpdateDraftPost)

	huma.Register(api, huma.Operation{
		OperationID: "deleteDraftPost",
		Method:      http.MethodDelete,
		Path:        "/{id}/draft-post",
		Summary:     "Delete a phase's draft post",
		Description: "Hard-deletes the draft, which was never published. Answers 200 with " +
			"a body, not 204. GM and co-GM only.",
		Tags:     drafts,
		Security: bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not the GM or co-GM"},
			"404": {Description: "Phase has no draft post"},
		},
	}, h.humaDeleteDraftPost)
}
