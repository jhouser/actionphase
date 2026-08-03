package messages

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	"actionphase/pkg/validation"
)

// CreatePost creates a new post in the common room
func (h *Handler) CreatePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_create_post")()

	gameID := ctx.Value("gameID").(int32)

	data := &CreatePostRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid create post request", "error", err)
		return
	}

	userID, err := h.getUserIDFromToken(r)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized(err.Error()), "Failed to get user from token", "error", err)
		return
	}

	// Check if user is GM or co-GM (only GM/co-GM can create posts)
	isGMOrCoGM := ctx.Value("is_gm").(bool)

	if !isGMOrCoGM {
		h.renderError(ctx, w, r, core.ErrForbidden("Only the Game Master or co-GM can create posts"), "Non-GM/co-GM user attempted to create post", "user_id", userID, "game_id", gameID)
		return
	}

	messageService := h.MessageService

	post, err := messageService.CreatePost(ctx, core.CreatePostRequest{
		GameID:      int32(gameID),
		PhaseID:     data.PhaseID,
		AuthorID:    userID,
		CharacterID: data.CharacterID,
		Content:     data.Content,
		Visibility:  "game", // Common Room posts are always visible to game
	})

	if err != nil {
		if core.IsArchivedGameError(err) {
			h.renderError(ctx, w, r, core.ErrGameArchived(), "Create post rejected: game is archived", "game_id", gameID, "user_id", userID)
			return
		}
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to create post", "error", err, "game_id", gameID, "user_id", userID)
		return
	}

	h.App.ObsLogger.Info(ctx, "Post created successfully", "post_id", post.ID, "game_id", gameID, "author_id", userID)

	// Fetch full post details to return with metadata
	postDetails, err := messageService.GetPost(ctx, post.ID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to fetch post details", "error", err, "post_id", post.ID)
		return
	}

	response := messageWithDetailsToResponse(postDetails)
	render.Status(r, http.StatusCreated)
	render.Render(w, r, response)
}

// GetGamePosts retrieves all posts for a game
func (h *Handler) GetGamePosts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_game_posts")()

	gameID := ctx.Value("gameID").(int32)

	// Parse optional query parameters
	phaseIDStr := r.URL.Query().Get("phase_id")
	var phaseID *int32
	if phaseIDStr != "" {
		pid, err := strconv.ParseInt(phaseIDStr, 10, 32)
		if err == nil {
			pid32 := int32(pid)
			phaseID = &pid32
		}
	}

	limitStr := r.URL.Query().Get("limit")
	limit := int32(50) // Default limit
	if limitStr != "" {
		l, err := strconv.ParseInt(limitStr, 10, 32)
		if err == nil && l > 0 && l <= 100 {
			limit = int32(l)
		}
	}

	offsetStr := r.URL.Query().Get("offset")
	offset := int32(0)
	if offsetStr != "" {
		o, err := strconv.ParseInt(offsetStr, 10, 32)
		if err == nil && o >= 0 {
			offset = int32(o)
		}
	}

	// Fetch game for anonymous mode check
	queries := models.New(h.App.Pool)
	game, err := queries.GetGame(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game", "error", err, "game_id", gameID)
		return
	}

	userID, _ := h.getUserIDFromToken(r)
	showUsernames := core.CanSeeUsernamesInAnonymousGame(ctx, h.App.Pool, game, userID)

	messageService := h.MessageService
	posts, err := messageService.GetGamePosts(ctx, int32(gameID), phaseID, limit, offset)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game posts", "error", err, "game_id", gameID)
		return
	}

	// Convert to response format
	response := make([]map[string]interface{}, 0)
	for _, post := range posts {
		authorUsername := post.AuthorUsername
		if !showUsernames {
			authorUsername = ""
		}
		postData := map[string]interface{}{
			"id":                   post.ID,
			"game_id":              post.GameID,
			"author_id":            post.AuthorID,
			"character_id":         post.CharacterID,
			"content":              post.Content,
			"message_type":         string(post.MessageType),
			"thread_depth":         post.ThreadDepth,
			"author_username":      authorUsername,
			"character_name":       post.CharacterName,
			"character_avatar_url": post.CharacterAvatarUrl,
			"comment_count":        post.CommentCount,
			"is_edited":            post.IsEdited,
			"is_deleted":           post.IsDeleted,
			"created_at":           post.CreatedAt,
		}

		if post.PhaseID.Valid {
			postData["phase_id"] = post.PhaseID.Int32
		}
		if post.ParentID.Valid {
			postData["parent_id"] = post.ParentID.Int32
		}

		response = append(response, postData)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateComment creates a comment on a post or another comment
func (h *Handler) CreateComment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_create_comment")()

	gameID := ctx.Value("gameID").(int32)

	postIDStr := chi.URLParam(r, "postId")
	postID, err := strconv.ParseInt(postIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid post ID")), "Invalid create comment request")
		return
	}

	data := &CreateCommentRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid create comment request", "error", err)
		return
	}

	userID, err := h.getUserIDFromToken(r)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized(err.Error()), "Failed to get user from token", "error", err)
		return
	}

	messageService := h.MessageService

	// :postId in the URL is the immediate parent (post or comment).
	// root_post_id in the body is the top-level post; required for read tracking.
	// When replying directly to a post they are the same, so postID is the fallback.
	rootPostID := int32(postID)
	if data.RootPostID != nil {
		rootPostID = *data.RootPostID
	}

	comment, err := messageService.CreateComment(ctx, core.CreateCommentRequest{
		GameID:      int32(gameID),
		PhaseID:     data.PhaseID,
		AuthorID:    userID,
		CharacterID: data.CharacterID,
		Content:     data.Content,
		ParentID:    int32(postID),
		RootPostID:  rootPostID,
		Visibility:  "game",
	})

	if err != nil {
		if core.IsArchivedGameError(err) {
			h.renderError(ctx, w, r, core.ErrGameArchived(), "Create comment rejected: game is archived", "game_id", gameID, "post_id", postID, "user_id", userID)
			return
		}
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to create comment", "error", err, "game_id", gameID, "post_id", postID, "user_id", userID)
		return
	}

	h.App.ObsLogger.Info(ctx, "Comment created successfully", "comment_id", comment.ID, "post_id", postID, "author_id", userID)

	// Fetch full comment details
	commentDetails, err := messageService.GetComment(ctx, comment.ID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to fetch comment details", "error", err, "comment_id", comment.ID)
		return
	}

	response := messageWithDetailsToResponse(commentDetails)
	render.Status(r, http.StatusCreated)
	render.Render(w, r, response)
}

// GetMessage retrieves a single message by ID (for deep linking)
func (h *Handler) GetMessage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_message")()

	gameID := ctx.Value("gameID").(int32)

	messageIDStr := chi.URLParam(r, "messageId")
	messageID, err := strconv.ParseInt(messageIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid message ID")), "Invalid get message request")
		return
	}

	queries := models.New(h.App.Pool)
	game, err := queries.GetGame(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game", "error", err, "game_id", gameID)
		return
	}

	userID, _ := h.getUserIDFromToken(r)
	showUsernames := core.CanSeeUsernamesInAnonymousGame(ctx, h.App.Pool, game, userID)

	messageService := h.MessageService
	message, err := messageService.GetMessage(ctx, int32(messageID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get message", "error", err, "message_id", messageID)
		return
	}

	response := messageWithDetailsToResponse(message)
	if !showUsernames {
		response.AuthorUsername = ""
	}
	render.Render(w, r, response)
}

// defaultThreadContextMaxParents is how many ancestor levels above the target
// comment we include for context when the client does not specify max_parents.
// Matches the depth the deep-link modal renders as parent context.
const defaultThreadContextMaxParents = 3

// maxThreadContextMaxParents caps the requestable parent count to bound query cost.
const maxThreadContextMaxParents = 20

// GetMessageThreadContext retrieves a message plus a bounded slice of its ancestor
// chain (target + up to max_parents nearest ancestors) and the true root post ID,
// in a single request, for deep-linking to nested comments.
// Query param: max_parents (optional, default 3, max 20).
func (h *Handler) GetMessageThreadContext(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_message_thread_context")()

	gameIDStr := chi.URLParam(r, "gameId")
	gameID, err := strconv.ParseInt(gameIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid game ID")), "Invalid get message thread context request")
		return
	}

	messageIDStr := chi.URLParam(r, "messageId")
	messageID, err := strconv.ParseInt(messageIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid message ID")), "Invalid get message thread context request")
		return
	}

	maxParents := int32(defaultThreadContextMaxParents)
	if mp := r.URL.Query().Get("max_parents"); mp != "" {
		parsed, perr := strconv.ParseInt(mp, 10, 32)
		if perr != nil || parsed < 0 || parsed > maxThreadContextMaxParents {
			h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid max_parents parameter (must be 0-%d)", maxThreadContextMaxParents)), "Invalid get message thread context request")
			return
		}
		maxParents = int32(parsed)
	}

	queries := models.New(h.App.Pool)
	game, err := queries.GetGame(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game", "error", err, "game_id", gameID)
		return
	}

	userID, _ := h.getUserIDFromToken(r)
	showUsernames := core.CanSeeUsernamesInAnonymousGame(ctx, h.App.Pool, game, userID)

	threadCtx, err := h.MessageService.GetMessageWithParentContext(ctx, int32(messageID), maxParents)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get message thread context", "error", err, "message_id", messageID)
		return
	}

	chain := make([]*MessageResponse, len(threadCtx.Chain))
	for i := range threadCtx.Chain {
		resp := messageWithDetailsToResponse(&threadCtx.Chain[i])
		if !showUsernames {
			resp.AuthorUsername = ""
		}
		chain[i] = resp
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MessageThreadContextResponse{
		Chain:         chain,
		RootPostID:    threadCtx.RootPostID,
		HasFullThread: threadCtx.HasFullThread,
	})
}

// GetPostComments retrieves direct comments for a post
func (h *Handler) GetPostComments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_post_comments")()

	gameID := ctx.Value("gameID").(int32)

	postIDStr := chi.URLParam(r, "postId")
	postID, err := strconv.ParseInt(postIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid post ID")), "Invalid get post comments request")
		return
	}

	// Fetch game for anonymous mode check
	queries := models.New(h.App.Pool)
	game, err := queries.GetGame(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game", "error", err, "game_id", gameID)
		return
	}

	userID, _ := h.getUserIDFromToken(r)
	showUsernames := core.CanSeeUsernamesInAnonymousGame(ctx, h.App.Pool, game, userID)

	messageService := h.MessageService
	comments, err := messageService.GetPostComments(ctx, int32(postID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get post comments", "error", err, "post_id", postID)
		return
	}

	// Convert to response format
	response := make([]map[string]interface{}, 0)
	for _, comment := range comments {
		authorUsername := comment.AuthorUsername
		if !showUsernames {
			authorUsername = ""
		}
		commentData := map[string]interface{}{
			"id":                      comment.ID,
			"game_id":                 comment.GameID,
			"author_id":               comment.AuthorID,
			"character_id":            comment.CharacterID,
			"content":                 comment.Content,
			"message_type":            string(comment.MessageType),
			"thread_depth":            comment.ThreadDepth,
			"author_username":         authorUsername,
			"character_name":          comment.CharacterName,
			"character_avatar_url":    comment.CharacterAvatarUrl,
			"reply_count":             comment.ReplyCount,
			"is_edited":               comment.IsEdited,
			"is_deleted":              comment.IsDeleted,
			"mentioned_character_ids": comment.MentionedCharacterIds,
			"created_at":              comment.CreatedAt,
		}

		if comment.PhaseID.Valid {
			commentData["phase_id"] = comment.PhaseID.Int32
		}
		if comment.ParentID.Valid {
			commentData["parent_id"] = comment.ParentID.Int32
		}

		response = append(response, commentData)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetPostCommentsWithThreads fetches paginated top-level comments with nested replies
// GET /api/v1/games/:gameId/posts/:postId/comments-with-threads?limit=200&offset=0&max_depth=5
// Returns comments at depths 0 through (max_depth - 1) so Reply buttons appear on all visible comments
// "Continue thread" button shows on comments at (max_depth - 1) that have deeper replies
func (h *Handler) GetPostCommentsWithThreads(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_post_comments_with_threads")()

	gameID := ctx.Value("gameID").(int32)

	postIDStr := chi.URLParam(r, "postId")
	postID, err := strconv.ParseInt(postIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid post ID")), "Invalid get post comments with threads request")
		return
	}

	// Parse query parameters with defaults
	limitStr := r.URL.Query().Get("limit")
	limit := int32(5) // Default: 5 top-level comments
	if limitStr != "" {
		limitInt, err := strconv.ParseInt(limitStr, 10, 32)
		if err != nil || limitInt < 1 || limitInt > 500 {
			h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid limit parameter (must be 1-500)")), "Invalid get post comments with threads request")
			return
		}
		limit = int32(limitInt)
	}

	offsetStr := r.URL.Query().Get("offset")
	offset := int32(0)
	if offsetStr != "" {
		offsetInt, err := strconv.ParseInt(offsetStr, 10, 32)
		if err != nil || offsetInt < 0 {
			h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid offset parameter (must be >= 0)")), "Invalid get post comments with threads request")
			return
		}
		offset = int32(offsetInt)
	}

	maxDepthStr := r.URL.Query().Get("max_depth")
	maxDepth := int32(h.App.Config.App.CommentMaxDepth)
	if maxDepthStr != "" {
		maxDepthInt, err := strconv.ParseInt(maxDepthStr, 10, 32)
		if err != nil || maxDepthInt < 0 || maxDepthInt > 10 {
			h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid max_depth parameter (must be 0-10)")), "Invalid get post comments with threads request")
			return
		}
		maxDepth = int32(maxDepthInt)
	}

	// Fetch game for anonymous mode check
	queries := models.New(h.App.Pool)
	game, err := queries.GetGame(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game", "error", err, "game_id", gameID)
		return
	}

	userID, _ := h.getUserIDFromToken(r)
	showUsernames := core.CanSeeUsernamesInAnonymousGame(ctx, h.App.Pool, game, userID)

	messageService := h.MessageService

	commentsWithDepth, err := messageService.GetPostCommentsWithThreads(ctx, int32(postID), limit, offset, maxDepth)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get post comments with threads", "error", err, "post_id", postID)
		return
	}

	totalCount, err := messageService.CountTopLevelComments(ctx, int32(postID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to count top-level comments", "error", err, "post_id", postID)
		return
	}

	// Convert to response format
	comments := make([]map[string]interface{}, 0)
	for _, commentWithDepth := range commentsWithDepth {
		comment := commentWithDepth.Comment
		authorUsername := comment.AuthorUsername
		if !showUsernames {
			authorUsername = ""
		}
		commentData := map[string]interface{}{
			"id":                      comment.ID,
			"game_id":                 comment.GameID,
			"author_id":               comment.AuthorID,
			"character_id":            comment.CharacterID,
			"content":                 comment.Content,
			"message_type":            string(comment.MessageType),
			"thread_depth":            comment.ThreadDepth,
			"author_username":         authorUsername,
			"character_name":          comment.CharacterName,
			"character_avatar_url":    comment.CharacterAvatarUrl,
			"reply_count":             comment.ReplyCount,
			"is_edited":               comment.IsEdited,
			"is_deleted":              comment.IsDeleted,
			"mentioned_character_ids": comment.MentionedCharacterIds,
			"created_at":              comment.CreatedAt,
			"depth":                   commentWithDepth.Depth,
		}

		if comment.PhaseID.Valid {
			commentData["phase_id"] = comment.PhaseID.Int32
		}
		if comment.ParentID.Valid {
			commentData["parent_id"] = comment.ParentID.Int32
		}

		comments = append(comments, commentData)
	}

	response := map[string]interface{}{
		"comments":           comments,
		"total_top_level":    totalCount,
		"limit":              limit,
		"offset":             offset,
		"has_more":           totalCount > int64(offset+limit),
		"returned_top_level": countTopLevelInResponse(commentsWithDepth),
		"returned_total":     len(comments),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdatePost updates the content of an existing post
// PATCH /api/v1/games/:gameId/posts/:postId
func (h *Handler) UpdatePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_post")()

	postIDStr := chi.URLParam(r, "postId")
	postID, err := strconv.ParseInt(postIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid post ID")), "Invalid update post request")
		return
	}

	data := &UpdatePostRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid update post request", "error", err)
		return
	}

	if len(strings.TrimSpace(data.Content)) == 0 {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("content cannot be empty")), "Invalid update post request")
		return
	}

	if err := validation.ValidatePost(data.Content); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid update post request", "error", err)
		return
	}

	userID, err := h.getUserIDFromToken(r)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized(err.Error()), "Failed to get user from token", "error", err)
		return
	}

	messageService := h.MessageService

	canEdit, err := messageService.CanUserEditPost(ctx, int32(postID), userID)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			h.renderError(ctx, w, r, core.ErrNotFound("post not found"), "Post not found", "post_id", postID)
			return
		}
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check edit permission", "error", err, "post_id", postID, "user_id", userID)
		return
	}

	if !canEdit {
		h.renderError(ctx, w, r, core.ErrForbidden("You can only edit your own posts"), "User attempted to edit post without permission", "post_id", postID, "user_id", userID)
		return
	}

	updatedPost, err := messageService.UpdatePost(ctx, int32(postID), data.Content)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to update post", "error", err, "post_id", postID, "user_id", userID)
		return
	}

	h.App.ObsLogger.Info(ctx, "Post updated successfully", "post_id", postID, "user_id", userID, "edit_count", updatedPost.EditCount)

	postDetails, err := messageService.GetPost(ctx, updatedPost.ID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to fetch updated post details", "error", err, "post_id", updatedPost.ID)
		return
	}

	response := messageWithDetailsToResponse(postDetails)
	render.Render(w, r, response)
}

// UpdateComment updates the content of an existing comment
// PATCH /api/v1/games/:gameId/posts/:postId/comments/:commentId
func (h *Handler) UpdateComment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_comment")()

	commentIDStr := chi.URLParam(r, "commentId")
	commentID, err := strconv.ParseInt(commentIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid comment ID")), "Invalid update comment request")
		return
	}

	data := &UpdateCommentRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid update comment request", "error", err)
		return
	}

	userID, err := h.getUserIDFromToken(r)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized(err.Error()), "Failed to get user from token", "error", err)
		return
	}

	messageService := h.MessageService

	canEdit, err := messageService.CanUserEditComment(ctx, int32(commentID), userID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check edit permission", "error", err, "comment_id", commentID, "user_id", userID)
		return
	}

	if !canEdit {
		h.renderError(ctx, w, r, core.ErrForbidden("You can only edit your own comments"), "User attempted to edit comment without permission", "comment_id", commentID, "user_id", userID)
		return
	}

	updatedComment, err := messageService.UpdateComment(ctx, int32(commentID), data.Content, data.CharacterID)
	if err != nil {
		if errors.Is(err, core.ErrCharacterNotControlled) {
			h.renderError(ctx, w, r, core.ErrForbidden("You do not control this character"), "User attempted to use character they don't control", "comment_id", commentID, "user_id", userID, "requested_character_id", data.CharacterID)
			return
		}
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to update comment", "error", err, "comment_id", commentID, "user_id", userID)
		return
	}

	h.App.ObsLogger.Info(ctx, "Comment updated successfully", "comment_id", commentID, "user_id", userID, "edit_count", updatedComment.EditCount)

	commentDetails, err := messageService.GetComment(ctx, updatedComment.ID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to fetch updated comment details", "error", err, "comment_id", updatedComment.ID)
		return
	}

	response := messageWithDetailsToResponse(commentDetails)
	render.Render(w, r, response)
}

// DeleteComment soft-deletes a comment
// DELETE /api/v1/games/:gameId/posts/:postId/comments/:commentId
func (h *Handler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_comment")()

	gameID := ctx.Value("gameID").(int32)

	commentIDStr := chi.URLParam(r, "commentId")
	commentID, err := strconv.ParseInt(commentIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid comment ID")), "Invalid delete comment request")
		return
	}

	userID, err := h.getUserIDFromToken(r)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized(err.Error()), "Failed to get user from token", "error", err)
		return
	}

	user, err := h.UserService.GetUserByID(int(userID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get user", "error", err, "user_id", userID)
		return
	}

	isAdmin := core.GetAdminMode(ctx) && user.IsAdmin

	messageService := h.MessageService

	canDelete, err := messageService.CanUserDeleteComment(ctx, int32(commentID), userID, isAdmin)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check delete permission", "error", err, "comment_id", commentID, "user_id", userID)
		return
	}

	if !canDelete {
		h.App.ObsLogger.Warn(ctx, "User attempted to delete comment without permission",
			"comment_id", commentID,
			"user_id", userID,
			"is_admin", isAdmin)
		h.renderError(ctx, w, r, core.ErrForbidden("You don't have permission to delete this comment"), "Delete comment forbidden")
		return
	}

	err = messageService.DeleteComment(ctx, int32(commentID), userID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to delete comment", "error", err, "comment_id", commentID, "user_id", userID)
		return
	}

	h.App.ObsLogger.Info(ctx, "Comment deleted successfully",
		"comment_id", commentID,
		"game_id", gameID,
		"deleted_by_user_id", userID,
		"is_admin", isAdmin)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Comment deleted successfully",
		"id":      commentID,
	})
}
