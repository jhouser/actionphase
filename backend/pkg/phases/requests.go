package phases

// Request payload shapes, retained for the package's tests.
//
// The handlers no longer read these -- huma binds its own body types (see
// huma_api.go), which carry the tags that are actually enforced. These remain
// because the tests marshal them to build request bodies, and they are
// serialization-identical to what the huma types accept.
//
// The `validate:` tags are deliberately gone: nothing runs them any more, and a
// tag that enforces nothing reads like a guarantee. The real constraints live
// on the huma body structs.

import (
	"actionphase/pkg/core"
)

// CreatePhaseRequest represents the request to create a new phase
type CreatePhaseRequest struct {
	PhaseType   string              `json:"phase_type"`
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	StartTime   *core.LocalDateTime `json:"start_time,omitempty"`
	EndTime     *core.LocalDateTime `json:"end_time,omitempty"`
	Deadline    *core.LocalDateTime `json:"deadline,omitempty"`
}

// UpdateDeadlineRequest represents the request to update a phase deadline
type UpdateDeadlineRequest struct {
	Deadline core.LocalDateTime `json:"deadline"`
}

// UpdatePhaseRequest represents the request to update phase details
type UpdatePhaseRequest struct {
	Title       *string             `json:"title,omitempty"`
	Description *string             `json:"description,omitempty"`
	StartTime   *core.LocalDateTime `json:"start_time,omitempty"`
	Deadline    *core.LocalDateTime `json:"deadline,omitempty"`
	// EndTime is intentionally excluded — it is system-managed and set by DeactivatePhase
}

// SubmitActionRequest represents the request to submit an action
type SubmitActionRequest struct {
	CharacterID *int32 `json:"character_id,omitempty"`
	Content     string `json:"content"`
}

// CreateActionResultRequest represents the request to create an action result
type CreateActionResultRequest struct {
	UserID             int32  `json:"user_id"`
	CharacterID        *int32 `json:"character_id,omitempty"`
	ActionSubmissionID *int32 `json:"action_submission_id,omitempty"`
	Content            string `json:"content"`
	IsPublished        bool   `json:"is_published,omitempty"`
}

// StagedResultPartRequest is one part of a staged result chain.
//
// DelayMinutes is how long to wait after the *previous* part becomes visible,
// which is why the first part must carry 0: nothing precedes it.
type StagedResultPartRequest struct {
	Content      string `json:"content"`
	DelayMinutes int32  `json:"delay_minutes"`
}

// CreateStagedResultChainRequest represents the request to create a whole
// staged result chain in one call.
//
// The recipient fields sit on the chain rather than on each part, so a chain
// cannot change recipient midway. That invariant holds by construction and
// needs no validation; see the service layer.
type CreateStagedResultChainRequest struct {
	UserID             int32                     `json:"user_id"`
	CharacterID        *int32                    `json:"character_id,omitempty"`
	ActionSubmissionID *int32                    `json:"action_submission_id,omitempty"`
	Parts              []StagedResultPartRequest `json:"parts"`
	IsPublished        bool                      `json:"is_published,omitempty"`
}

// CreateDraftCharacterUpdateRequest represents the request to create a draft character update
type CreateDraftCharacterUpdateRequest struct {
	CharacterID int32  `json:"character_id"`
	ModuleType  string `json:"module_type"`
	FieldName   string `json:"field_name"`
	FieldValue  string `json:"field_value"`
	FieldType   string `json:"field_type"`
	Operation   string `json:"operation"`
}
