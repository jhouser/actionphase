package deadlines

import (
	"time"
)

// CreateDeadlineRequest is the create-deadline body.
//
// Validation now lives on the huma input struct in huma_api.go (the tags there
// are what the server enforces and what the spec publishes). This type is
// retained as the named, JSON-tagged shape of that body for callers and tests.
type CreateDeadlineRequest struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Deadline    time.Time `json:"deadline"`
}

// UpdateDeadlineRequest is the update-deadline body. See CreateDeadlineRequest.
type UpdateDeadlineRequest struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Deadline    time.Time `json:"deadline"`
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
