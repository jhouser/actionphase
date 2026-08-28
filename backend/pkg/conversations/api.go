package conversations

import (
	"actionphase/pkg/core"
	"actionphase/pkg/humaconfig"

	"github.com/danielgtaylor/huma/v2"
)

// Handler handles HTTP requests for conversations
type Handler struct {
	App                 *core.App
	GameService         core.GameServiceInterface
	CharacterService    core.CharacterServiceInterface
	ConversationService core.ConversationServiceInterface
	PhaseService        core.PhaseServiceInterface
}

// CreateConversationRequest represents the request body for creating a conversation
type CreateConversationRequest struct {
	Title        string  `json:"title" minLength:"1" maxLength:"255" doc:"Conversation title"`
	CharacterIDs []int32 `json:"character_ids" doc:"Characters participating; at least two"`
}

// Resolve trims the title before validation, so a whitespace-only title is
// rejected rather than stored blank. huma's own checks count raw characters.
func (r *CreateConversationRequest) Resolve(huma.Context) []error {
	return humaconfig.TrimStrings(r)
}

// SendMessageRequest represents the request body for sending a message
type SendMessageRequest struct {
	CharacterID int32  `json:"character_id"` // Character sending the message
	Content     string `json:"content"`
}

// AddParticipantRequest represents the request body for adding a participant
type AddParticipantRequest struct {
	CharacterID int32 `json:"character_id"`
}

// UpdateMessageRequest represents the request body for editing a message
type UpdateMessageRequest struct {
	Content string `json:"content"`
}
