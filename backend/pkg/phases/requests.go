package phases

import (
	"actionphase/pkg/core"
	"net/http"
)

// CreatePhaseRequest represents the request to create a new phase
type CreatePhaseRequest struct {
	PhaseType   string              `json:"phase_type" validate:"required"`
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	StartTime   *core.LocalDateTime `json:"start_time,omitempty"`
	EndTime     *core.LocalDateTime `json:"end_time,omitempty"`
	Deadline    *core.LocalDateTime `json:"deadline,omitempty"`
}

func (r *CreatePhaseRequest) Bind(req *http.Request) error {
	return nil
}

// UpdateDeadlineRequest represents the request to update a phase deadline
type UpdateDeadlineRequest struct {
	Deadline core.LocalDateTime `json:"deadline" validate:"required"`
}

func (r *UpdateDeadlineRequest) Bind(req *http.Request) error {
	return nil
}

// UpdatePhaseRequest represents the request to update phase details
type UpdatePhaseRequest struct {
	Title       *string             `json:"title,omitempty"`
	Description *string             `json:"description,omitempty"`
	StartTime   *core.LocalDateTime `json:"start_time,omitempty"`
	Deadline    *core.LocalDateTime `json:"deadline,omitempty"`
	// EndTime is intentionally excluded — it is system-managed and set by DeactivatePhase
}

func (r *UpdatePhaseRequest) Bind(req *http.Request) error {
	return nil
}

// SubmitActionRequest represents the request to submit an action
type SubmitActionRequest struct {
	CharacterID *int32 `json:"character_id,omitempty"`
	Content     string `json:"content" validate:"required"`
}

func (r *SubmitActionRequest) Bind(req *http.Request) error {
	return nil
}

// CreateActionResultRequest represents the request to create an action result
type CreateActionResultRequest struct {
	UserID             int32  `json:"user_id" validate:"required"`
	CharacterID        *int32 `json:"character_id,omitempty"`
	ActionSubmissionID *int32 `json:"action_submission_id,omitempty"`
	Content            string `json:"content" validate:"required"`
	IsPublished        bool   `json:"is_published,omitempty"`
}

func (r *CreateActionResultRequest) Bind(req *http.Request) error {
	return nil
}

// StagedResultPartRequest is one part of a staged result chain.
//
// DelayMinutes is how long to wait after the *previous* part becomes visible,
// which is why the first part must carry 0: nothing precedes it.
type StagedResultPartRequest struct {
	Content      string `json:"content" validate:"required"`
	DelayMinutes int32  `json:"delay_minutes"`
}

// CreateStagedResultChainRequest represents the request to create a whole
// staged result chain in one call.
//
// The recipient fields sit on the chain rather than on each part, so a chain
// cannot change recipient midway. That invariant holds by construction and
// needs no validation; see the service layer.
type CreateStagedResultChainRequest struct {
	UserID             int32                     `json:"user_id" validate:"required"`
	CharacterID        *int32                    `json:"character_id,omitempty"`
	ActionSubmissionID *int32                    `json:"action_submission_id,omitempty"`
	Parts              []StagedResultPartRequest `json:"parts" validate:"required"`
	IsPublished        bool                      `json:"is_published,omitempty"`
}

func (r *CreateStagedResultChainRequest) Bind(req *http.Request) error {
	return nil
}

// CreateDraftCharacterUpdateRequest represents the request to create a draft character update
type CreateDraftCharacterUpdateRequest struct {
	CharacterID int32  `json:"character_id" validate:"required"`
	ModuleType  string `json:"module_type" validate:"required"`
	FieldName   string `json:"field_name" validate:"required"`
	FieldValue  string `json:"field_value" validate:"required"`
	FieldType   string `json:"field_type" validate:"required"`
	Operation   string `json:"operation" validate:"required"`
}

func (r *CreateDraftCharacterUpdateRequest) Bind(req *http.Request) error {
	return nil
}
