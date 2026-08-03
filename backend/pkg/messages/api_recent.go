package messages

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
)

// ListRecentCommentsWithParents lists recent comments with their parent messages for the "New Comments" view
// GET /api/v1/games/:gameId/comments/recent
func (h *Handler) ListRecentCommentsWithParents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_list_recent_comments_with_parents")()

	gameID := ctx.Value("gameID").(int32)

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 10
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit < 1 {
			h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid limit parameter")), "Invalid list recent comments with parents request")
			return
		}
		if parsedLimit > 50 {
			parsedLimit = 50
		}
		limit = parsedLimit
	}

	offset := 0
	if offsetStr != "" {
		parsedOffset, err := strconv.Atoi(offsetStr)
		if err != nil || parsedOffset < 0 {
			h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid offset parameter")), "Invalid list recent comments with parents request")
			return
		}
		offset = parsedOffset
	}

	queries := models.New(h.App.Pool)
	game, err := queries.GetGame(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game", "error", err, "game_id", gameID)
		return
	}

	userID, userIDErr := h.getUserIDFromToken(r)
	showUsernames := core.CanSeeUsernamesInAnonymousGame(ctx, h.App.Pool, game, userID)

	// unread_only restricts the list to comments this user has not manually
	// marked as read (the "New Comments" filter in manual read mode). It needs
	// an identified user, unlike the default unfiltered listing.
	unreadOnly := r.URL.Query().Get("unread_only") == "true"
	if unreadOnly && userIDErr != nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required for unread_only"), "Invalid list recent comments with parents request")
		return
	}

	messageService := h.MessageService

	var comments []core.CommentWithParent
	var totalCount int64
	if unreadOnly {
		comments, err = messageService.ListRecentUnreadCommentsWithParents(ctx, int32(gameID), userID, int32(limit), int32(offset))
		if err != nil {
			h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to list recent unread comments", "error", err, "game_id", gameID, "user_id", userID)
			return
		}
		totalCount, err = messageService.GetTotalUnreadCommentCount(ctx, int32(gameID), userID)
		if err != nil {
			h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get total unread comment count", "error", err, "game_id", gameID, "user_id", userID)
			return
		}
	} else {
		comments, err = messageService.ListRecentCommentsWithParents(ctx, int32(gameID), int32(limit), int32(offset))
		if err != nil {
			h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to list recent comments", "error", err, "game_id", gameID)
			return
		}
		totalCount, err = messageService.GetTotalCommentCount(ctx, int32(gameID))
		if err != nil {
			h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get total comment count", "error", err, "game_id", gameID)
			return
		}
	}

	h.App.ObsLogger.Info(ctx, "Listed recent comments with parents",
		"game_id", gameID,
		"limit", limit,
		"offset", offset,
		"unread_only", unreadOnly,
		"count", len(comments),
		"total", totalCount)

	response := map[string]interface{}{
		"comments": commentsWithParentsToResponse(comments, showUsernames),
		"pagination": map[string]interface{}{
			"limit":  limit,
			"offset": offset,
			"total":  totalCount,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetCharacterComments retrieves paginated public posts and comments by a specific character
func (h *Handler) GetCharacterComments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_character_comments")()

	characterIDStr := chi.URLParam(r, "id")
	characterID, err := strconv.ParseInt(characterIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid character ID")), "Invalid get character comments request")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 20
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit < 1 {
			h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid limit parameter")), "Invalid get character comments request")
			return
		}
		if parsedLimit > 50 {
			parsedLimit = 50
		}
		limit = parsedLimit
	}

	offset := 0
	if offsetStr != "" {
		parsedOffset, err := strconv.Atoi(offsetStr)
		if err != nil || parsedOffset < 0 {
			h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid offset parameter")), "Invalid get character comments request")
			return
		}
		offset = parsedOffset
	}

	queries := models.New(h.App.Pool)

	character, err := queries.GetCharacter(ctx, int32(characterID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("character not found"), "Failed to get character", "error", err, "character_id", characterID)
		return
	}

	game, err := queries.GetGame(ctx, character.GameID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game for character", "error", err, "game_id", character.GameID)
		return
	}

	userID, _ := h.getUserIDFromToken(r)
	showUsernames := core.CanSeeUsernamesInAnonymousGame(ctx, h.App.Pool, game, userID)

	messageService := h.MessageService

	messages, err := messageService.ListCharacterPostsAndComments(ctx, int32(characterID), int32(limit), int32(offset))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to list character messages", "error", err, "character_id", characterID)
		return
	}

	totalCount, err := messageService.CountCharacterPostsAndComments(ctx, int32(characterID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to count character messages", "error", err, "character_id", characterID)
		return
	}

	result := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		authorUsername := msg.AuthorUsername
		if !showUsernames {
			authorUsername = ""
		}
		msgData := map[string]interface{}{
			"id":                   msg.ID,
			"game_id":              msg.GameID,
			"parent_id":            msg.ParentID,
			"author_id":            msg.AuthorID,
			"character_id":         msg.CharacterID,
			"content":              msg.Content,
			"message_type":         msg.MessageType,
			"created_at":           msg.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			"edited_at":            formatTimePtr(msg.EditedAt),
			"edit_count":           msg.EditCount,
			"deleted_at":           formatTimePtr(msg.DeletedAt),
			"is_deleted":           msg.IsDeleted,
			"author_username":      authorUsername,
			"character_name":       msg.CharacterName,
			"character_avatar_url": msg.CharacterAvatarUrl,
		}

		if msg.ParentContent != nil {
			parentAuthorUsername := msg.ParentAuthorUsername
			if !showUsernames {
				parentAuthorUsername = nil
			}
			msgData["parent"] = map[string]interface{}{
				"content":              msg.ParentContent,
				"created_at":           formatTimePtr(msg.ParentCreatedAt),
				"deleted_at":           formatTimePtr(msg.ParentDeletedAt),
				"is_deleted":           msg.ParentIsDeleted,
				"message_type":         msg.ParentMessageType,
				"author_username":      parentAuthorUsername,
				"character_name":       msg.ParentCharacterName,
				"character_avatar_url": msg.ParentCharacterAvatarUrl,
			}
		}

		result[i] = msgData
	}

	response := map[string]interface{}{
		"messages": result,
		"pagination": map[string]interface{}{
			"limit":  limit,
			"offset": offset,
			"total":  totalCount,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
