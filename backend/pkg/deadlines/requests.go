package deadlines

import (
	"net/http"
	"time"

	"actionphase/pkg/core"
)

// CreateDeadlineRequest represents the request to create a new deadline
type CreateDeadlineRequest struct {
	Title       string    `json:"title" validate:"required,min=1,max=255"`
	Description string    `json:"description"`
	Deadline    time.Time `json:"deadline" validate:"required"`
}

func (r *CreateDeadlineRequest) Bind(req *http.Request) error {
	return core.ValidateStruct(r)
}

// UpdateDeadlineRequest represents the request to update a deadline
type UpdateDeadlineRequest struct {
	Title       string    `json:"title" validate:"required,min=1,max=255"`
	Description string    `json:"description"`
	Deadline    time.Time `json:"deadline" validate:"required"`
}

func (r *UpdateDeadlineRequest) Bind(req *http.Request) error {
	return core.ValidateStruct(r)
}

// DeadlineResponse represents a deadline in API responses
type DeadlineResponse struct {
	ID          int32      `json:"id"`
	GameID      int32      `json:"game_id"`
	Title       string     `json:"title"`
	Description *string    `json:"description,omitempty"`
	Deadline    *time.Time `json:"deadline,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

// DeadlineWithGameResponse represents a deadline with game information
type DeadlineWithGameResponse struct {
	ID          int32      `json:"id"`
	GameID      int32      `json:"game_id"`
	GameTitle   string     `json:"game_title"`
	Title       string     `json:"title"`
	Description *string    `json:"description,omitempty"`
	Deadline    *time.Time `json:"deadline,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

// UnifiedDeadlineResponse represents a deadline from any source (arbitrary, phase, or poll)
type UnifiedDeadlineResponse struct {
	DeadlineType     string     `json:"deadline_type"`      // "deadline", "phase", or "poll"
	SourceID         int32      `json:"source_id"`          // ID from the source table
	Title            string     `json:"title"`              // Deadline title or phase/poll question
	Description      string     `json:"description"`        // Deadline description
	Deadline         *time.Time `json:"deadline,omitempty"` // When the deadline expires
	GameID           int32      `json:"game_id"`            // Associated game
	PhaseID          *int32     `json:"phase_id,omitempty"` // NULL for arbitrary deadlines
	PollID           *int32     `json:"poll_id,omitempty"`  // NULL for non-poll deadlines
	IsSystemDeadline bool       `json:"is_system_deadline"` // true for phase deadlines (can't be deleted)
}
