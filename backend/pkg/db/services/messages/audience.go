package messages

import (
	"context"
	"fmt"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
)

// ============================================================================
// Audience Participation Methods (Private Conversation Viewing)
// ============================================================================

// ListAllPrivateConversations lists all private conversations in a game (for audience/GM)
// Returns conversation metadata including message counts and latest activity
// Supports pagination and filtering by participant names
func (ms *MessageService) ListAllPrivateConversations(ctx context.Context, params core.ListAllPrivateConversationsParams) ([]models.ListAllPrivateConversationsRow, error) {
	queries := models.New(ms.DB)

	// Convert to sqlc params
	sqlcParams := models.ListAllPrivateConversationsParams{
		GameID:                  params.GameID,
		ParticipantCharacterIds: params.ParticipantCharacterIDs,
		ResultLimit:             params.Limit,
		ResultOffset:            params.Offset,
	}

	conversations, err := queries.ListAllPrivateConversations(ctx, sqlcParams)
	if err != nil {
		return nil, fmt.Errorf("failed to list all private conversations: %w", err)
	}

	return conversations, nil
}

// CountAllPrivateConversations returns the total number of private conversations in a game,
// applying the same participant filter as ListAllPrivateConversations.
func (ms *MessageService) CountAllPrivateConversations(ctx context.Context, gameID int32, participantCharacterIDs []int32) (int64, error) {
	queries := models.New(ms.DB)

	count, err := queries.CountAllPrivateConversations(ctx, models.CountAllPrivateConversationsParams{
		GameID:                  gameID,
		ParticipantCharacterIds: participantCharacterIDs,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to count all private conversations: %w", err)
	}

	return count, nil
}

// GetConversationParticipantCharacters returns the characters that appear in at least
// one conversation in the game, optionally narrowed to those who share a conversation
// with all of the given character IDs.
func (ms *MessageService) GetConversationParticipantCharacters(ctx context.Context, gameID int32, selectedCharacterIDs []int32) ([]core.ConversationParticipantCharacter, error) {
	queries := models.New(ms.DB)

	rows, err := queries.GetConversationParticipantCharacters(ctx, models.GetConversationParticipantCharactersParams{
		GameID:               gameID,
		SelectedCharacterIds: selectedCharacterIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation participant characters: %w", err)
	}

	characters := make([]core.ConversationParticipantCharacter, 0, len(rows))
	for _, r := range rows {
		characters = append(characters, core.ConversationParticipantCharacter{
			ID:   r.CharacterID,
			Name: r.CharacterName,
		})
	}
	return characters, nil
}

// GetAudienceConversationMessages retrieves all messages in a conversation (for audience/GM)
// Returns messages with sender information and character details
func (ms *MessageService) GetAudienceConversationMessages(ctx context.Context, conversationID int32) ([]models.GetAudienceConversationMessagesRow, error) {
	queries := models.New(ms.DB)

	messages, err := queries.GetAudienceConversationMessages(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation messages: %w", err)
	}

	return messages, nil
}
