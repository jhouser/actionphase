package core

import "errors"

var (
	// ErrConversationNotEmpty is returned when a delete is attempted on a
	// conversation that has messages in it.
	ErrConversationNotEmpty = errors.New("conversation has messages and cannot be deleted")

	// ErrConversationDeleteForbidden is returned when the requesting user is
	// neither the conversation's creator nor a GM of the game.
	ErrConversationDeleteForbidden = errors.New("forbidden: only the creator or a GM can delete this conversation")
)

// CreateConversationRequest is the domain request for creating a conversation.
type CreateConversationRequest struct {
	GameID          int32
	Title           string
	CreatedByUserID int32
	ParticipantIDs  []int32 // Character IDs
}

// SendConversationMessageRequest is the domain request for sending a private message.
type SendConversationMessageRequest struct {
	ConversationID    int32
	SenderUserID      int32
	SenderCharacterID int32
	Content           string
}
