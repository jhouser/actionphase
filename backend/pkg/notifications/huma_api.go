package notifications

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"actionphase/pkg/core"
)

// Input / output types
//
// Query parameter names here are the ones the server has always read
// (limit/offset/unread). The hand-written spec documented page/page_size/
// unread_only, which no code path has ever looked at -- a client following
// those docs silently got the defaults back.

type listNotificationsInput struct {
	Limit  int  `query:"limit" default:"20" minimum:"1" maximum:"100" doc:"Maximum notifications to return"`
	Offset int  `query:"offset" default:"0" minimum:"0" doc:"Number of notifications to skip"`
	Unread bool `query:"unread" doc:"Return only unread notifications. Ignores offset."`
}

type notificationOutput struct {
	Body *NotificationResponse
}

type listNotificationsOutput struct {
	Body *NotificationListResponse
}

type unreadCountOutput struct {
	Body *UnreadCountResponse
}

type markAllReadOutput struct {
	Body *MarkAllReadResponse
}

type notificationIDInput struct {
	ID int32 `path:"id" doc:"Notification ID"`
}

// RegisterHumaNotifications registers the notification operations on api, which
// must be mounted at /notifications.
func RegisterHumaNotifications(api huma.API, h *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "listNotifications",
		Method:      http.MethodGet,
		Path:        "/",
		Summary:     "List user notifications",
		Description: "Returns the authenticated user's notifications, most recent first.",
		Tags:        []string{"Notifications"},
	}, h.listNotifications)

	huma.Register(api, huma.Operation{
		OperationID: "getUnreadNotificationCount",
		Method:      http.MethodGet,
		Path:        "/unread-count",
		Summary:     "Get unread notification count",
		Tags:        []string{"Notifications"},
	}, h.getUnreadCount)

	huma.Register(api, huma.Operation{
		OperationID: "markAllNotificationsRead",
		Method:      http.MethodPut,
		Path:        "/mark-all-read",
		Summary:     "Mark all notifications as read",
		Tags:        []string{"Notifications"},
	}, h.markAllAsRead)

	huma.Register(api, huma.Operation{
		OperationID: "getNotification",
		Method:      http.MethodGet,
		Path:        "/{id}",
		Summary:     "Get single notification",
		Tags:        []string{"Notifications"},
	}, h.getNotification)

	huma.Register(api, huma.Operation{
		OperationID: "markNotificationRead",
		Method:      http.MethodPut,
		Path:        "/{id}/mark-read",
		Summary:     "Mark notification as read",
		Tags:        []string{"Notifications"},
	}, h.markAsRead)

	huma.Register(api, huma.Operation{
		OperationID: "markNotificationUnread",
		Method:      http.MethodPut,
		Path:        "/{id}/mark-unread",
		Summary:     "Mark notification as unread",
		Tags:        []string{"Notifications"},
	}, h.markAsUnread)

	huma.Register(api, huma.Operation{
		OperationID:   "deleteNotification",
		Method:        http.MethodDelete,
		Path:          "/{id}",
		Summary:       "Delete notification",
		Tags:          []string{"Notifications"},
		DefaultStatus: http.StatusNoContent,
	}, h.deleteNotification)
}

// authUserID resolves the caller from the request context. The auth middleware
// runs ahead of every route here, so a missing user means the middleware was
// not applied rather than a bad request.
func (h *Handler) authUserID(ctx context.Context) (int32, error) {
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.App.ObsLogger.Warn(ctx, "No authenticated user in context")
		return 0, huma.Error401Unauthorized("authentication required")
	}
	return int32(authUser.ID), nil
}

func (h *Handler) listNotifications(ctx context.Context, in *listNotificationsInput) (*listNotificationsOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_notifications")()

	userID, err := h.authUserID(ctx)
	if err != nil {
		return nil, err
	}

	var notifications []*core.Notification
	if in.Unread {
		notifications, err = h.NotificationService.GetUnreadNotifications(ctx, userID, in.Limit)
	} else {
		notifications, err = h.NotificationService.GetUserNotifications(ctx, userID, in.Limit, in.Offset)
	}
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to fetch notifications")
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Failed to fetch notifications: %v", err))
	}

	// Reported as the pagination total even though it counts only unread rows.
	// Preserved as-is: the list UI reads it as a badge count, not a page count.
	totalCount, err := h.NotificationService.GetUnreadCount(ctx, userID)
	if err != nil {
		// A missing count must not fail the list itself.
		totalCount = 0
	}

	data := make([]*NotificationResponse, len(notifications))
	for i, notif := range notifications {
		data[i] = notificationToResponse(notif)
	}

	return &listNotificationsOutput{Body: &NotificationListResponse{
		Data: data,
		Pagination: &PaginationInfo{
			Total:  totalCount,
			Limit:  in.Limit,
			Offset: in.Offset,
		},
	}}, nil
}

func (h *Handler) getUnreadCount(ctx context.Context, _ *struct{}) (*unreadCountOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_unread_count")()

	userID, err := h.authUserID(ctx)
	if err != nil {
		return nil, err
	}

	count, err := h.NotificationService.GetUnreadCount(ctx, userID)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to fetch unread count")
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Failed to fetch unread count: %v", err))
	}

	return &unreadCountOutput{Body: &UnreadCountResponse{UnreadCount: count}}, nil
}

func (h *Handler) getNotification(ctx context.Context, in *notificationIDInput) (*notificationOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_notification")()

	userID, err := h.authUserID(ctx)
	if err != nil {
		return nil, err
	}

	// Ownership is enforced by scanning the caller's own notifications: there is
	// no GetNotificationByID on the service, and fetching by ID alone would let
	// one user read another's row.
	notifications, err := h.NotificationService.GetUserNotifications(ctx, userID, 1000, 0)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to fetch notification")
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Failed to fetch notification: %v", err))
	}

	for _, n := range notifications {
		if n.ID == in.ID {
			return &notificationOutput{Body: notificationToResponse(n)}, nil
		}
	}

	return nil, huma.Error404NotFound("Notification not found")
}

func (h *Handler) markAsRead(ctx context.Context, in *notificationIDInput) (*notificationOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_mark_notification_as_read")()

	userID, err := h.authUserID(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.NotificationService.MarkAsRead(ctx, in.ID, userID); err != nil {
		// A notification the caller doesn't own is reported as missing rather
		// than forbidden, so IDs can't be probed.
		if errors.Is(err, core.ErrNotificationNotFound) {
			return nil, huma.Error404NotFound("Notification not found")
		}
		h.App.ObsLogger.LogError(ctx, err, "Failed to mark notification as read")
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Failed to mark notification as read: %v", err))
	}

	// The service returns nothing, so the response echoes only the fields this
	// call is known to have changed rather than re-reading the row.
	now := time.Now()
	return &notificationOutput{Body: &NotificationResponse{
		ID:     in.ID,
		IsRead: true,
		ReadAt: &now,
	}}, nil
}

func (h *Handler) markAsUnread(ctx context.Context, in *notificationIDInput) (*notificationOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_mark_notification_as_unread")()

	userID, err := h.authUserID(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.NotificationService.MarkAsUnread(ctx, in.ID, userID); err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to mark notification as unread")
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Failed to mark notification as unread: %v", err))
	}

	return &notificationOutput{Body: &NotificationResponse{
		ID:     in.ID,
		IsRead: false,
	}}, nil
}

func (h *Handler) markAllAsRead(ctx context.Context, _ *struct{}) (*markAllReadOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_mark_all_as_read")()

	userID, err := h.authUserID(ctx)
	if err != nil {
		return nil, err
	}

	// Counted before the update, since afterwards there is nothing left to count.
	unreadCount, err := h.NotificationService.GetUnreadCount(ctx, userID)
	if err != nil {
		unreadCount = 0
	}

	if err := h.NotificationService.MarkAllAsRead(ctx, userID); err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to mark all notifications as read")
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Failed to mark all notifications as read: %v", err))
	}

	return &markAllReadOutput{Body: &MarkAllReadResponse{MarkedCount: int(unreadCount)}}, nil
}

func (h *Handler) deleteNotification(ctx context.Context, in *notificationIDInput) (*struct{}, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_notification")()

	userID, err := h.authUserID(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.NotificationService.DeleteNotification(ctx, in.ID, userID); err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to delete notification")
		return nil, huma.Error500InternalServerError(fmt.Sprintf("Failed to delete notification: %v", err))
	}

	return nil, nil
}
