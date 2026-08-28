package notifications

import (
	"time"

	"actionphase/pkg/core"
)

type Handler struct {
	App                 *core.App
	NotificationService core.NotificationServiceInterface
}

// Response Types
//
// These are the huma output bodies (see huma_api.go); they no longer implement
// render.Renderer, since nothing renders them through go-chi any more.
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

type NotificationListResponse struct {
	Data       []*NotificationResponse `json:"data"`
	Pagination *PaginationInfo         `json:"pagination"`
}

type PaginationInfo struct {
	Total  int64 `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}

type UnreadCountResponse struct {
	UnreadCount int64 `json:"unread_count"`
}

type MarkAllReadResponse struct {
	MarkedCount int `json:"marked_count"`
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
