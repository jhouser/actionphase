package core

import (
	"errors"
	"time"

	"github.com/go-playground/validator/v10"
)

// ErrNotificationNotFound is returned when a notification does not exist or is
// not owned by the requesting user. The two cases are deliberately
// indistinguishable so a caller cannot probe for other users' notification IDs.
var ErrNotificationNotFound = errors.New("notification not found")

// Notification represents a user notification.
// It contains information about an event that the user should be aware of.
//
// Two reference pairs exist and mean different things:
//
//   - ContextType/ContextID identify the container a user opens (e.g. a
//     conversation). They scope bulk operations such as "mark everything for
//     this conversation read".
//   - RelatedType/RelatedID identify the specific item that triggered the
//     notification (e.g. one message), so the dashboard inbox can preview it.
//
// Both are optional; notifications predating context tracking leave the
// context pair nil and are only ever marked read one row at a time.
type Notification struct {
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

// CreateNotificationRequest contains the data needed to create a notification.
type CreateNotificationRequest struct {
	UserID      int32   `json:"user_id" validate:"required"`
	GameID      *int32  `json:"game_id,omitempty"`
	Type        string  `json:"type" validate:"required"`
	Title       string  `json:"title" validate:"required,min=1,max=255"`
	Content     *string `json:"content,omitempty" validate:"omitempty,max=1000"`
	RelatedType *string `json:"related_type,omitempty"`
	RelatedID   *int32  `json:"related_id,omitempty"`
	LinkURL     *string `json:"link_url,omitempty" validate:"omitempty,max=500"`
	ContextType *string `json:"context_type,omitempty" validate:"omitempty,max=50"`
	ContextID   *int32  `json:"context_id,omitempty"`
}

// Validate validates the CreateNotificationRequest.
func (r *CreateNotificationRequest) Validate() error {
	validate := validator.New(validator.WithRequiredStructEnabled())

	// Register custom validator for notification type
	validate.RegisterValidation("notification_type", func(fl validator.FieldLevel) bool {
		return IsValidNotificationType(fl.Field().String())
	})

	// Add custom validation for notification type
	if !IsValidNotificationType(r.Type) {
		return &validator.ValidationErrors{}
	}

	return validate.Struct(r)
}

// NotificationFilters contains filters for querying notifications.
type NotificationFilters struct {
	Limit  int  `json:"limit"`
	Offset int  `json:"offset"`
	Unread bool `json:"unread"` // Only return unread notifications
}
