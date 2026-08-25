package notifications

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"actionphase/pkg/core"
)

type Handler struct {
	App                 *core.App
	NotificationService core.NotificationServiceInterface
}

// Response Types
type NotificationResponse struct {
	ID          int32      `json:"id"`
	UserID      int32      `json:"user_id"`
	GameID      *int32     `json:"game_id,omitempty"`
	Type        string     `json:"type"`
	Title       string     `json:"title"`
	Content     *string    `json:"content,omitempty"`
	RelatedType *string    `json:"related_type,omitempty"`
	RelatedID   *int32     `json:"related_id,omitempty"`
	LinkURL     *string    `json:"link_url,omitempty"`
	ContextType *string    `json:"context_type,omitempty"`
	ContextID   *int32     `json:"context_id,omitempty"`
	IsRead      bool       `json:"is_read"`
	ReadAt      *time.Time `json:"read_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (rd *NotificationResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

type NotificationListResponse struct {
	Data       []*NotificationResponse `json:"data"`
	Pagination *PaginationInfo         `json:"pagination"`
}

func (rd *NotificationListResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

type PaginationInfo struct {
	Total  int64 `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}

type UnreadCountResponse struct {
	UnreadCount int64 `json:"unread_count"`
}

func (rd *UnreadCountResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

type MarkAllReadResponse struct {
	MarkedCount int `json:"marked_count"`
}

func (rd *MarkAllReadResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

// Helper function to convert core.Notification to NotificationResponse
func notificationToResponse(notif *core.Notification) *NotificationResponse {
	return &NotificationResponse{
		ID:          notif.ID,
		UserID:      notif.UserID,
		GameID:      notif.GameID,
		Type:        notif.Type,
		Title:       notif.Title,
		Content:     notif.Content,
		RelatedType: notif.RelatedType,
		RelatedID:   notif.RelatedID,
		LinkURL:     notif.LinkURL,
		ContextType: notif.ContextType,
		ContextID:   notif.ContextID,
		IsRead:      notif.IsRead,
		ReadAt:      notif.ReadAt,
		CreatedAt:   notif.CreatedAt,
	}
}

// GetNotifications - GET /api/v1/notifications
// List user's notifications (paginated)
func (h *Handler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_notification")()

	defer h.App.ObsLogger.LogOperation(ctx, "api_get_notifications")()

	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	userID := int32(authUser.ID)

	// Parse query parameters
	limit := 20 // default
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			if parsedLimit > 100 {
				limit = 100 // max limit
			} else {
				limit = parsedLimit
			}
		}
	}

	offset := 0 // default
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	service := h.NotificationService

	// Check if only unread notifications are requested
	unreadOnly := r.URL.Query().Get("unread") == "true"

	var notifications []*core.Notification
	var err error
	if unreadOnly {
		notifications, err = service.GetUnreadNotifications(ctx, userID, limit)
	} else {
		notifications, err = service.GetUserNotifications(ctx, userID, limit, offset)
	}

	if err != nil {
		render.Render(w, r, &core.ErrResponse{
			HTTPStatusCode: http.StatusInternalServerError,
			StatusText:     "Internal Server Error",
			ErrorText:      fmt.Sprintf("Failed to fetch notifications: %v", err),
		})
		return
	}

	// Get total count for pagination
	totalCount, err := service.GetUnreadCount(ctx, userID)
	if err != nil {
		// Don't fail the request if we can't get the count, just log it
		totalCount = 0
	}

	// Convert to response format
	responseData := make([]*NotificationResponse, len(notifications))
	for i, notif := range notifications {
		responseData[i] = notificationToResponse(notif)
	}

	response := &NotificationListResponse{
		Data: responseData,
		Pagination: &PaginationInfo{
			Total:  totalCount,
			Limit:  limit,
			Offset: offset,
		},
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// GetUnreadCount - GET /api/v1/notifications/unread-count
// Get count of unread notifications
func (h *Handler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_unread_count")()

	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	userID := int32(authUser.ID)

	service := h.NotificationService
	count, err := service.GetUnreadCount(ctx, userID)
	if err != nil {
		render.Render(w, r, &core.ErrResponse{
			HTTPStatusCode: http.StatusInternalServerError,
			StatusText:     "Internal Server Error",
			ErrorText:      fmt.Sprintf("Failed to fetch unread count: %v", err),
		})
		return
	}

	response := &UnreadCountResponse{
		UnreadCount: count,
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// GetNotification - GET /api/v1/notifications/:id
// Get a specific notification
func (h *Handler) GetNotification(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_notification")()

	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	userID := int32(authUser.ID)

	// Parse notification ID from URL
	notificationIDStr := chi.URLParam(r, "id")
	notificationID, err := strconv.ParseInt(notificationIDStr, 10, 32)
	if err != nil {
		render.Render(w, r, &core.ErrResponse{
			HTTPStatusCode: http.StatusBadRequest,
			StatusText:     "Bad Request",
			ErrorText:      "Invalid notification ID",
		})
		return
	}

	service := h.NotificationService

	// Get all user's notifications to check ownership
	// (This is a simple approach; for production, you might want a dedicated GetNotificationByID method)
	notifications, err := service.GetUserNotifications(ctx, userID, 1000, 0)
	if err != nil {
		render.Render(w, r, &core.ErrResponse{
			HTTPStatusCode: http.StatusInternalServerError,
			StatusText:     "Internal Server Error",
			ErrorText:      fmt.Sprintf("Failed to fetch notification: %v", err),
		})
		return
	}

	// Find the notification
	var notification *core.Notification
	for _, n := range notifications {
		if n.ID == int32(notificationID) {
			notification = n
			break
		}
	}

	if notification == nil {
		render.Render(w, r, &core.ErrResponse{
			HTTPStatusCode: http.StatusNotFound,
			StatusText:     "Not Found",
			ErrorText:      "Notification not found",
		})
		return
	}

	response := notificationToResponse(notification)
	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// MarkNotificationAsRead - PUT /api/v1/notifications/:id/mark-read
// Mark a notification as read
func (h *Handler) MarkNotificationAsRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_mark_notification_as_read")()

	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	userID := int32(authUser.ID)

	// Parse notification ID from URL
	notificationIDStr := chi.URLParam(r, "id")
	notificationID, err := strconv.ParseInt(notificationIDStr, 10, 32)
	if err != nil {
		render.Render(w, r, &core.ErrResponse{
			HTTPStatusCode: http.StatusBadRequest,
			StatusText:     "Bad Request",
			ErrorText:      "Invalid notification ID",
		})
		return
	}

	service := h.NotificationService
	err = service.MarkAsRead(ctx, int32(notificationID), userID)
	if err != nil {
		// A notification the caller doesn't own is reported as missing rather
		// than forbidden, so IDs can't be probed.
		if errors.Is(err, core.ErrNotificationNotFound) {
			render.Render(w, r, &core.ErrResponse{
				HTTPStatusCode: http.StatusNotFound,
				StatusText:     "Not Found",
				ErrorText:      "Notification not found",
			})
			return
		}
		render.Render(w, r, &core.ErrResponse{
			HTTPStatusCode: http.StatusInternalServerError,
			StatusText:     "Internal Server Error",
			ErrorText:      fmt.Sprintf("Failed to mark notification as read: %v", err),
		})
		return
	}

	// Return success with minimal data
	now := time.Now()
	response := &NotificationResponse{
		ID:     int32(notificationID),
		IsRead: true,
		ReadAt: &now,
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// MarkNotificationAsUnread - PUT /api/v1/notifications/:id/mark-unread
// Mark a notification as unread
func (h *Handler) MarkNotificationAsUnread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_mark_notification_as_unread")()

	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	userID := int32(authUser.ID)

	notificationIDStr := chi.URLParam(r, "id")
	notificationID, err := strconv.ParseInt(notificationIDStr, 10, 32)
	if err != nil {
		render.Render(w, r, &core.ErrResponse{
			HTTPStatusCode: http.StatusBadRequest,
			StatusText:     "Bad Request",
			ErrorText:      "Invalid notification ID",
		})
		return
	}

	service := h.NotificationService
	err = service.MarkAsUnread(ctx, int32(notificationID), userID)
	if err != nil {
		render.Render(w, r, &core.ErrResponse{
			HTTPStatusCode: http.StatusInternalServerError,
			StatusText:     "Internal Server Error",
			ErrorText:      fmt.Sprintf("Failed to mark notification as unread: %v", err),
		})
		return
	}

	response := &NotificationResponse{
		ID:     int32(notificationID),
		IsRead: false,
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// MarkAllAsRead - PUT /api/v1/notifications/mark-all-read
// Mark all user's notifications as read
func (h *Handler) MarkAllAsRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_mark_all_as_read")()

	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	userID := int32(authUser.ID)

	// Get count before marking all as read (for response)
	service := h.NotificationService
	unreadCount, err := service.GetUnreadCount(ctx, userID)
	if err != nil {
		unreadCount = 0 // Don't fail if we can't get count
	}

	err = service.MarkAllAsRead(ctx, userID)
	if err != nil {
		render.Render(w, r, &core.ErrResponse{
			HTTPStatusCode: http.StatusInternalServerError,
			StatusText:     "Internal Server Error",
			ErrorText:      fmt.Sprintf("Failed to mark all notifications as read: %v", err),
		})
		return
	}

	response := &MarkAllReadResponse{
		MarkedCount: int(unreadCount),
	}

	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// DeleteNotification - DELETE /api/v1/notifications/:id
// Delete a notification
func (h *Handler) DeleteNotification(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_notification")()

	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	userID := int32(authUser.ID)

	// Parse notification ID from URL
	notificationIDStr := chi.URLParam(r, "id")
	notificationID, err := strconv.ParseInt(notificationIDStr, 10, 32)
	if err != nil {
		render.Render(w, r, &core.ErrResponse{
			HTTPStatusCode: http.StatusBadRequest,
			StatusText:     "Bad Request",
			ErrorText:      "Invalid notification ID",
		})
		return
	}

	service := h.NotificationService
	err = service.DeleteNotification(ctx, int32(notificationID), userID)
	if err != nil {
		render.Render(w, r, &core.ErrResponse{
			HTTPStatusCode: http.StatusInternalServerError,
			StatusText:     "Internal Server Error",
			ErrorText:      fmt.Sprintf("Failed to delete notification: %v", err),
		})
		return
	}

	// Return 204 No Content
	w.WriteHeader(http.StatusNoContent)
}
