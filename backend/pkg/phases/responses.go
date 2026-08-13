package phases

import (
	"net/http"
	"time"
)

// PhaseResponse represents a phase response
type PhaseResponse struct {
	ID          int32      `json:"id"`
	GameID      int32      `json:"game_id"`
	PhaseType   string     `json:"phase_type"`
	PhaseNumber int32      `json:"phase_number"`
	Title       *string    `json:"title,omitempty"`
	Description *string    `json:"description,omitempty"`
	StartTime   *time.Time `json:"start_time,omitempty"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	Deadline    *time.Time `json:"deadline,omitempty"`
	IsActive    bool       `json:"is_active"`
	IsPublished bool       `json:"is_published"`
	ActivatedAt *time.Time `json:"activated_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`

	// Calculated fields for UI
	TimeRemaining *int64 `json:"time_remaining,omitempty"` // seconds until deadline
	IsExpired     bool   `json:"is_expired"`
}

func (rd *PhaseResponse) Render(w http.ResponseWriter, r *http.Request) error {
	// Calculate time remaining and expiry status
	if rd.Deadline != nil {
		remaining := time.Until(*rd.Deadline)
		if remaining > 0 {
			seconds := int64(remaining.Seconds())
			rd.TimeRemaining = &seconds
			rd.IsExpired = false
		} else {
			rd.IsExpired = true
		}
	}
	return nil
}

// ActionResponse represents an action response
type ActionResponse struct {
	ID          int32     `json:"id"`
	GameID      int32     `json:"game_id"`
	UserID      int32     `json:"user_id"`
	PhaseID     int32     `json:"phase_id"`
	CharacterID *int32    `json:"character_id,omitempty"`
	Content     string    `json:"content"`
	SubmittedAt time.Time `json:"submitted_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (rd *ActionResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

// ActionWithDetailsResponse represents an action with additional details
type ActionWithDetailsResponse struct {
	ID            int32     `json:"id"`
	GameID        int32     `json:"game_id"`
	UserID        int32     `json:"user_id"`
	PhaseID       int32     `json:"phase_id"`
	CharacterID   *int32    `json:"character_id,omitempty"`
	Content       string    `json:"content"`
	SubmittedAt   time.Time `json:"submitted_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Username      string    `json:"username"`
	CharacterName *string   `json:"character_name,omitempty"`
	PhaseType     *string   `json:"phase_type,omitempty"`
	PhaseNumber   *int32    `json:"phase_number,omitempty"`
}

func (rd *ActionWithDetailsResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

// ActionResultResponse represents an action result
type ActionResultResponse struct {
	ID          int32      `json:"id"`
	GameID      int32      `json:"game_id"`
	UserID      int32      `json:"user_id"`
	PhaseID     int32      `json:"phase_id"`
	GMUserID    int32      `json:"gm_user_id"`
	Content     string     `json:"content"`
	IsPublished bool       `json:"is_published"`
	SentAt      *time.Time `json:"sent_at,omitempty"`
}

func (rd *ActionResultResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

// ActionResultWithDetailsResponse represents an action result with additional details
type ActionResultWithDetailsResponse struct {
	ID                 int32      `json:"id"`
	GameID             int32      `json:"game_id"`
	UserID             int32      `json:"user_id"`
	PhaseID            int32      `json:"phase_id"`
	CharacterID        *int32     `json:"character_id,omitempty"`         // Character the action/result is for
	ActionSubmissionID *int32     `json:"action_submission_id,omitempty"` // Reference to the original action submission
	GMUserID           int32      `json:"gm_user_id"`
	Content            string     `json:"content"`
	IsPublished        bool       `json:"is_published"`
	SentAt             *time.Time `json:"sent_at,omitempty"`
	Username           string     `json:"username,omitempty"`       // Player username (for GM view)
	CharacterName      string     `json:"character_name,omitempty"` // Character name (for GM view)
	GMUsername         string     `json:"gm_username,omitempty"`    // GM username (for player view)
	PhaseType          string     `json:"phase_type,omitempty"`
	PhaseNumber        int32      `json:"phase_number,omitempty"`

	// Staged reveal fields. All omitempty, so an ordinary single-part result
	// serializes exactly as it did before this feature existed and no client
	// has to learn about staging to keep working.
	//
	// ReleasedAt is when a part became visible to its recipient; nil means
	// scheduled but not yet released. Distinct from SentAt, which only tracks
	// publication — a staged part is published long before it is released.
	PartNumber *int32     `json:"part_number,omitempty"`
	PartCount  int32      `json:"part_count,omitempty"`
	ReleasedAt *time.Time `json:"released_at,omitempty"`
	UnlocksAt  *time.Time `json:"unlocks_at,omitempty"`

	// ParentResultID is the part this one follows. Nil on a chain head and on
	// any unstaged result. Returned by the edit endpoints so a client can place
	// a newly appended part in the chain without re-fetching the whole list.
	ParentResultID *int32 `json:"parent_result_id,omitempty"`

	// RevealDelayMinutes is the configured wait, as distinct from UnlocksAt's
	// resolved wall-clock time. The GM's editor needs the configured value to
	// populate its delay selector; UnlocksAt cannot supply it, since it is NULL
	// until the parent releases and is anyway a timestamp rather than a
	// duration. Nil on a chain head and on any unstaged result.
	RevealDelayMinutes *int32 `json:"reveal_delay_minutes,omitempty"`
}

func (rd *ActionResultWithDetailsResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

// DraftCharacterUpdateResponse represents a draft character update
type DraftCharacterUpdateResponse struct {
	ID             int32     `json:"id"`
	ActionResultID int32     `json:"action_result_id"`
	CharacterID    int32     `json:"character_id"`
	ModuleType     string    `json:"module_type"`
	FieldName      string    `json:"field_name"`
	FieldValue     string    `json:"field_value"`
	FieldType      string    `json:"field_type"`
	Operation      string    `json:"operation"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (rd *DraftCharacterUpdateResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}
