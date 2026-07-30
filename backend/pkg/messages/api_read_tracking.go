package messages

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"actionphase/pkg/core"
)

// MarkPostRead marks a post (and optionally a specific comment) as read
// POST /api/v1/games/:gameId/posts/:postId/mark-read
func (h *Handler) MarkPostRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_mark_post_read")()

	gameIDStr := chi.URLParam(r, "gameId")
	gameID, err := strconv.ParseInt(gameIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid game ID")), "Invalid mark post read request")
		return
	}

	postIDStr := chi.URLParam(r, "postId")
	postID, err := strconv.ParseInt(postIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid post ID")), "Invalid mark post read request")
		return
	}

	userID, err := h.getUserIDFromToken(r)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized(err.Error()), "Failed to get user from token", "error", err)
		return
	}

	var requestBody struct {
		LastReadCommentID *int32 `json:"last_read_comment_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil && err != io.EOF {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid request body")), "Invalid mark post read request")
		return
	}

	messageService := h.MessageService
	readMarker, err := messageService.MarkPostAsRead(ctx, userID, int32(gameID), int32(postID), requestBody.LastReadCommentID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to mark post as read", "error", err, "game_id", gameID, "post_id", postID, "user_id", userID)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":                   readMarker.ID,
		"user_id":              readMarker.UserID,
		"game_id":              readMarker.GameID,
		"post_id":              readMarker.PostID,
		"last_read_comment_id": readMarker.LastReadCommentID,
		"last_read_at":         readMarker.LastReadAt,
		"created_at":           readMarker.CreatedAt,
		"updated_at":           readMarker.UpdatedAt,
	})
}

// GetGameReadMarkers gets all read markers for the current user in a game
// GET /api/v1/games/:gameId/read-markers
func (h *Handler) GetGameReadMarkers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_game_read_markers")()

	gameIDStr := chi.URLParam(r, "gameId")
	gameID, err := strconv.ParseInt(gameIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid game ID")), "Invalid get game read markers request")
		return
	}

	userID, err := h.getUserIDFromToken(r)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized(err.Error()), "Failed to get user from token", "error", err)
		return
	}

	messageService := h.MessageService
	readMarkers, err := messageService.GetUserReadMarkersForGame(ctx, userID, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get read markers", "error", err, "game_id", gameID, "user_id", userID)
		return
	}

	response := make([]map[string]interface{}, 0, len(readMarkers))
	for _, marker := range readMarkers {
		response = append(response, map[string]interface{}{
			"id":                   marker.ID,
			"user_id":              marker.UserID,
			"game_id":              marker.GameID,
			"post_id":              marker.PostID,
			"last_read_comment_id": marker.LastReadCommentID,
			"last_read_at":         marker.LastReadAt,
			"created_at":           marker.CreatedAt,
			"updated_at":           marker.UpdatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetPostsUnreadInfo gets post metadata to determine unread status
// GET /api/v1/games/:gameId/posts-unread-info
func (h *Handler) GetPostsUnreadInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_posts_unread_info")()

	gameIDStr := chi.URLParam(r, "gameId")
	gameID, err := strconv.ParseInt(gameIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid game ID")), "Invalid get posts unread info request")
		return
	}

	messageService := h.MessageService
	postsInfo, err := messageService.GetPostsWithUnreadInfo(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get posts unread info", "error", err, "game_id", gameID)
		return
	}

	response := make([]map[string]interface{}, 0, len(postsInfo))
	for _, info := range postsInfo {
		postData := map[string]interface{}{
			"post_id":         info.PostID,
			"post_created_at": info.PostCreatedAt,
			"total_comments":  info.TotalComments,
		}

		if info.LatestCommentAt != nil {
			postData["latest_comment_at"] = info.LatestCommentAt
		}

		response = append(response, postData)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetUnreadCommentIDs gets the specific IDs of unread comments for all posts in a game
// GET /api/v1/games/:gameId/unread-comment-ids
func (h *Handler) GetUnreadCommentIDs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_unread_comment_ids")()

	gameIDStr := chi.URLParam(r, "gameId")
	gameID, err := strconv.ParseInt(gameIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid game ID")), "Invalid get unread comment i ds request")
		return
	}

	userID, err := h.getUserIDFromToken(r)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized(err.Error()), "Failed to get user from token", "error", err)
		return
	}

	messageService := h.MessageService
	unreadComments, err := messageService.GetUnreadCommentIDsForPosts(ctx, userID, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get unread comment IDs", "error", err, "game_id", gameID, "user_id", userID)
		return
	}

	response := make([]map[string]interface{}, 0, len(unreadComments))
	for _, uc := range unreadComments {
		response = append(response, map[string]interface{}{
			"post_id":            uc.PostID,
			"unread_comment_ids": uc.UnreadCommentIDs,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ToggleCommentRead marks or unmarks a single comment as manually read by the current user
// POST /api/v1/games/:gameId/posts/:postId/comments/:commentId/toggle-read
func (h *Handler) ToggleCommentRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_toggle_comment_read")()

	gameIDStr := chi.URLParam(r, "gameId")
	gameID, err := strconv.ParseInt(gameIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid game ID")), "Invalid toggle comment read request")
		return
	}

	postIDStr := chi.URLParam(r, "postId")
	postID, err := strconv.ParseInt(postIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid post ID")), "Invalid toggle comment read request")
		return
	}

	commentIDStr := chi.URLParam(r, "commentId")
	commentID, err := strconv.ParseInt(commentIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid comment ID")), "Invalid toggle comment read request")
		return
	}

	userID, err := h.getUserIDFromToken(r)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized(err.Error()), "Failed to get user from token", "error", err)
		return
	}

	var requestBody struct {
		Read bool `json:"read"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil && err != io.EOF {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid request body")), "Invalid toggle comment read request")
		return
	}

	messageService := h.MessageService
	if err := messageService.ToggleCommentRead(ctx, userID, int32(gameID), int32(postID), int32(commentID), requestBody.Read); err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to toggle comment read", "error", err,
			"game_id", gameID, "post_id", postID, "comment_id", commentID, "user_id", userID)
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to toggle comment read", "error", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// MarkAllCommentsRead marks every comment in a phase as manually read by the current user
// POST /api/v1/games/:gameId/phases/:phaseId/mark-all-comments-read
func (h *Handler) MarkAllCommentsRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_mark_all_comments_read")()

	gameIDStr := chi.URLParam(r, "gameId")
	gameID, err := strconv.ParseInt(gameIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid game ID")), "Invalid mark all comments read request")
		return
	}

	phaseIDStr := chi.URLParam(r, "phaseId")
	phaseID, err := strconv.ParseInt(phaseIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid phase ID")), "Invalid mark all comments read request")
		return
	}

	userID, err := h.getUserIDFromToken(r)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized(err.Error()), "Failed to get user from token", "error", err)
		return
	}

	messageService := h.MessageService
	if err := messageService.MarkAllCommentsReadForPhase(ctx, userID, int32(gameID), int32(phaseID)); err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to mark all comments read for phase", "error", err,
			"game_id", gameID, "phase_id", phaseID, "user_id", userID)
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to mark all comments read", "error", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetManualReadCommentIDs gets all comment IDs manually marked as read by the current user in a game
// GET /api/v1/games/:gameId/manual-read-comment-ids
func (h *Handler) GetManualReadCommentIDs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_manual_read_comment_ids")()

	gameIDStr := chi.URLParam(r, "gameId")
	gameID, err := strconv.ParseInt(gameIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid game ID")), "Invalid get manual read comment i ds request")
		return
	}

	userID, err := h.getUserIDFromToken(r)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized(err.Error()), "Failed to get user from token", "error", err)
		return
	}

	messageService := h.MessageService
	manualReads, err := messageService.GetManualReadCommentIDsForGame(ctx, userID, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get manual read comment IDs", "error", err, "game_id", gameID, "user_id", userID)
		return
	}

	response := make([]ManualReadCommentIDsResponse, 0, len(manualReads))
	for _, mr := range manualReads {
		response = append(response, ManualReadCommentIDsResponse{
			PostID:         mr.PostID,
			ReadCommentIDs: mr.ReadCommentIDs,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
