package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	"actionphase/pkg/observability"
	"actionphase/pkg/validation"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ core.ConversationServiceInterface = (*ConversationService)(nil)

// ConversationService handles private messaging operations
type ConversationService struct {
	DB      *pgxpool.Pool
	Queries *models.Queries

	// Logger is optional. When unset, logger() supplies a package-local
	// fallback so callers that construct this service directly keep working.
	Logger *observability.Logger
}

// NewConversationService creates a new conversation service.
//
// The logger may be nil; see ConversationService.Logger.
func NewConversationService(db *pgxpool.Pool, logger *observability.Logger) *ConversationService {
	return &ConversationService{
		DB:      db,
		Queries: models.New(db),
		Logger:  logger,
	}
}

// logger returns the service logger, falling back to a package-local one when
// none was supplied. Background work recovers panics through this logger, so it
// must never be nil.
func (s *ConversationService) logger() *observability.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return fallbackConversationLogger
}

// fallbackConversationLogger backs ConversationService.logger() for callers
// that construct the service without one.
var fallbackConversationLogger = observability.NewLogger("conversations", "info")

// CreateConversationRequest represents a request to create a new conversation
// CreateConversationRequest is an alias kept for callers that used the old db-package type.
type CreateConversationRequest = core.CreateConversationRequest

// ConversationWithDetails includes conversation metadata
type ConversationWithDetails struct {
	Conversation  models.Conversation
	Participants  []models.GetConversationParticipantsRow
	MessageCount  int
	UnreadCount   int
	LastMessageAt *time.Time
}

// CreateConversation creates a new conversation with participants
// All participants must be characters (or GM)
func (s *ConversationService) CreateConversation(ctx context.Context, req CreateConversationRequest) (*models.Conversation, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.Queries.WithTx(tx)

	// Create conversation
	var title pgtype.Text
	if req.Title != "" {
		title = pgtype.Text{String: req.Title, Valid: true}
	}

	// Determine conversation type based on participant count
	// "direct" for 2 participants, "group" for 3+
	conversationType := "group"
	if len(req.ParticipantIDs) == 2 {
		conversationType = "direct"
	}

	conv, err := qtx.CreateConversation(ctx, models.CreateConversationParams{
		GameID:           req.GameID,
		ConversationType: conversationType,
		Title:            title,
		CreatedByUserID:  req.CreatedByUserID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	// Add all participants (characters)
	for _, charID := range req.ParticipantIDs {
		// Get character to find the user_id
		char, err := qtx.GetCharacter(ctx, charID)
		if err != nil {
			return nil, fmt.Errorf("failed to get character %d: %w", charID, err)
		}

		// Determine the user ID for this participant
		var participantUserID int32
		if char.UserID.Valid {
			// Player character - use the character's user_id
			participantUserID = char.UserID.Int32
			slog.Info("Using player character user ID", "character_id", charID, "user_id", participantUserID)
		} else {
			// NPC - check if assigned to a user (e.g., audience member)
			assignment, err := qtx.GetNPCAssignment(ctx, charID)
			if err == nil {
				// NPC is assigned to a user - use that user's ID
				participantUserID = assignment.AssignedUserID
				slog.Info("Using NPC assignment", "character_id", charID, "assigned_user_id", participantUserID)
			} else {
				// NPC is not assigned - add both GM and all co-GMs as participants
				slog.Info("NPC not assigned, adding GM and co-GMs as participants", "character_id", charID, "assignment_error", err)

				game, err := qtx.GetGame(ctx, req.GameID)
				if err != nil {
					return nil, fmt.Errorf("failed to get game: %w", err)
				}

				// Add primary GM
				_, err = qtx.AddConversationParticipant(ctx, models.AddConversationParticipantParams{
					ConversationID: conv.ID,
					UserID:         game.GmUserID,
					CharacterID:    pgtype.Int4{Int32: charID, Valid: true},
				})
				if err != nil {
					return nil, fmt.Errorf("failed to add GM as participant: %w", err)
				}
				slog.Info("Added primary GM for unassigned NPC", "character_id", charID, "user_id", game.GmUserID)

				// Add all co-GMs
				coGMUserIDs, err := qtx.GetGameCoGMs(ctx, req.GameID)
				if err != nil {
					return nil, fmt.Errorf("failed to get co-GMs: %w", err)
				}

				for _, coGMUserID := range coGMUserIDs {
					_, err = qtx.AddConversationParticipant(ctx, models.AddConversationParticipantParams{
						ConversationID: conv.ID,
						UserID:         coGMUserID,
						CharacterID:    pgtype.Int4{Int32: charID, Valid: true},
					})
					if err != nil {
						return nil, fmt.Errorf("failed to add co-GM %d as participant: %w", coGMUserID, err)
					}
					slog.Info("Added co-GM for unassigned NPC", "character_id", charID, "user_id", coGMUserID)
				}

				// Skip the normal participant addition since we already added GM/co-GMs
				continue
			}
		}

		_, err = qtx.AddConversationParticipant(ctx, models.AddConversationParticipantParams{
			ConversationID: conv.ID,
			UserID:         participantUserID,
			CharacterID:    pgtype.Int4{Int32: charID, Valid: true},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to add participant: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &conv, nil
}

// GetOrCreateConversation finds an existing conversation between characters or creates a new one
func (s *ConversationService) GetOrCreateConversation(ctx context.Context, gameID int32, characterIDs []int32, createdByUserID int32, title string) (*models.Conversation, error) {
	// For now, only support 1-on-1 conversations for finding existing ones
	// Group conversations will always be created new
	if len(characterIDs) == 2 {
		// Try to find existing conversation between these two characters
		char1, err := s.Queries.GetCharacter(ctx, characterIDs[0])
		if err == nil && char1.UserID.Valid {
			char2, err := s.Queries.GetCharacter(ctx, characterIDs[1])
			if err == nil && char2.UserID.Valid {
				// Try to find existing conversation
				// Note: This query needs to be added to communications.sql if not present
				// For now, just create a new one
			}
		}
	}

	// Create new conversation
	return s.CreateConversation(ctx, CreateConversationRequest{
		GameID:          gameID,
		Title:           title,
		CreatedByUserID: createdByUserID,
		ParticipantIDs:  characterIDs,
	})
}

// GetUserConversations gets all conversations for a user in a game
func (s *ConversationService) GetUserConversations(ctx context.Context, gameID int32, userID int32) ([]models.GetUserConversationsRow, error) {
	conversations, err := s.Queries.GetUserConversations(ctx, models.GetUserConversationsParams{
		GmUserID: userID, // Note: This is actually the current user ID, used both for filtering and GM check
		GameID:   gameID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get user conversations: %w", err)
	}

	return conversations, nil
}

// GetUserUnreadConversations gets conversations with unread messages for a user, capped at limit.
func (s *ConversationService) GetUserUnreadConversations(ctx context.Context, gameID int32, userID int32, limit int32) ([]models.GetUserUnreadConversationsRow, error) {
	conversations, err := s.Queries.GetUserUnreadConversations(ctx, models.GetUserUnreadConversationsParams{
		UserID:     userID,
		GameID:     gameID,
		MaxResults: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get unread conversations: %w", err)
	}
	return conversations, nil
}

// GetConversationParticipants gets all participants in a conversation
func (s *ConversationService) GetConversationParticipants(ctx context.Context, conversationID int32) ([]models.GetConversationParticipantsRow, error) {
	participants, err := s.Queries.GetConversationParticipants(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation participants: %w", err)
	}

	return participants, nil
}

// SendMessageRequest is an alias kept for callers that used the old db-package type.
type SendMessageRequest = core.SendConversationMessageRequest

// SendMessage sends a message in a conversation
func (s *ConversationService) SendMessage(ctx context.Context, req SendMessageRequest) (*models.PrivateMessage, error) {
	if err := validation.ValidatePrivateMessage(req.Content); err != nil {
		return nil, err
	}

	// Verify sender is a participant
	isParticipant, err := s.Queries.IsUserInConversation(ctx, models.IsUserInConversationParams{
		ConversationID: req.ConversationID,
		UserID:         req.SenderUserID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to check participation: %w", err)
	}
	if !isParticipant {
		return nil, core.ErrNotConversationParticipant
	}

	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.Queries.WithTx(tx)

	// Send message
	msg, err := qtx.SendPrivateMessage(ctx, models.SendPrivateMessageParams{
		ConversationID:    req.ConversationID,
		SenderUserID:      req.SenderUserID,
		SenderCharacterID: pgtype.Int4{Int32: req.SenderCharacterID, Valid: req.SenderCharacterID != 0},
		Content:           req.Content,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Update conversation activity timestamp
	if err := qtx.UpdateConversationActivity(ctx, req.ConversationID); err != nil {
		return nil, fmt.Errorf("failed to update conversation activity: %w", err)
	}

	// Mark conversation as read for the sender (they just sent a message, so they've "read" it)
	if _, err := qtx.UpsertConversationRead(ctx, models.UpsertConversationReadParams{
		UserID:         req.SenderUserID,
		ConversationID: req.ConversationID,
		LastReadMessageID: pgtype.Int4{
			Int32: msg.ID,
			Valid: true,
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to mark conversation as read for sender: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Trigger notifications for all participants except sender (fire-and-forget)
	observability.SafeGo(context.Background(), s.logger(), "notify-private-message", func() {
		s.notifyPrivateMessage(context.Background(), req.ConversationID, req.SenderUserID, req.SenderCharacterID, msg.ID)
	})

	return &msg, nil
}

// GetConversationMessages gets all messages in a conversation
func (s *ConversationService) GetConversationMessages(ctx context.Context, conversationID int32, userID int32) ([]models.GetConversationMessagesRow, error) {
	return s.getConversationMessages(ctx, conversationID, userID, pgtype.Int4{})
}

// GetMessageWithContext returns a single message along with the one immediately
// preceding it in the conversation, oldest first. Used to preview an unread
// message with enough context to recall the thread without sending the entire
// conversation, which GetConversationMessages does unbounded.
//
// A messageID belonging to another conversation matches nothing and yields no
// rows, so this cannot be used to read a conversation the caller isn't in.
func (s *ConversationService) GetMessageWithContext(ctx context.Context, conversationID int32, messageID int32, userID int32) ([]models.GetConversationMessagesRow, error) {
	return s.getConversationMessages(ctx, conversationID, userID, pgtype.Int4{Int32: messageID, Valid: true})
}

// getConversationMessages backs both the full-thread and scoped-context reads.
// They differ only in contextFor: unset returns the whole conversation, set
// narrows to that message and its predecessor. Sharing one path keeps the
// participant check and deleted-content masking identical for both.
func (s *ConversationService) getConversationMessages(ctx context.Context, conversationID int32, userID int32, contextFor pgtype.Int4) ([]models.GetConversationMessagesRow, error) {
	// Verify user is a participant
	isParticipant, err := s.Queries.IsUserInConversation(ctx, models.IsUserInConversationParams{
		ConversationID: conversationID,
		UserID:         userID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to check participation: %w", err)
	}
	if !isParticipant {
		return nil, core.ErrNotConversationParticipant
	}

	messages, err := s.Queries.GetConversationMessages(ctx, models.GetConversationMessagesParams{
		ConversationID: conversationID,
		ContextFor:     contextFor,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation messages: %w", err)
	}

	// Replace content of deleted messages with placeholder text
	for i := range messages {
		if messages[i].IsDeleted.Valid && messages[i].IsDeleted.Bool {
			messages[i].Content = "[Message deleted]"
		}
	}

	return messages, nil
}

// MarkConversationAsRead marks all messages in a conversation as read for a user
func (s *ConversationService) MarkConversationAsRead(ctx context.Context, conversationID int32, userID int32) error {
	// Verify user is a participant
	isParticipant, err := s.Queries.IsUserInConversation(ctx, models.IsUserInConversationParams{
		ConversationID: conversationID,
		UserID:         userID,
	})
	if err != nil {
		return fmt.Errorf("failed to check participation: %w", err)
	}
	if !isParticipant {
		return core.ErrNotConversationParticipant
	}

	// Only the newest message id is needed here, so ask for exactly that rather
	// than reading every message in the conversation and discarding all but one.
	lastMessageID, err := s.Queries.GetLastConversationMessageID(ctx, conversationID)
	if err != nil {
		// No messages in the conversation, so there is nothing to mark as read.
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("failed to get messages: %w", err)
	}

	// Mark conversation as read up to the latest message
	_, err = s.MarkConversationRead(ctx, userID, conversationID, lastMessageID)
	if err != nil {
		return fmt.Errorf("failed to mark conversation as read: %w", err)
	}

	return nil
}

// GetUnreadMessageCount gets the count of unread messages for a user in a conversation
func (s *ConversationService) GetUnreadMessageCount(ctx context.Context, conversationID int32, userID int32) (int64, error) {
	count, err := s.Queries.GetUnreadMessageCount(ctx, models.GetUnreadMessageCountParams{
		UserID:         userID,
		ConversationID: conversationID,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}

	return count, nil
}

// AddParticipant adds a character to an existing conversation
func (s *ConversationService) AddParticipant(ctx context.Context, conversationID int32, characterID int32) error {
	// Get character to find the user_id
	char, err := s.Queries.GetCharacter(ctx, characterID)
	if err != nil {
		return fmt.Errorf("failed to get character: %w", err)
	}

	// For NPCs without a user_id, use the GM's user_id
	var participantUserID int32
	if !char.UserID.Valid {
		// Get the game via conversation
		conv, err := s.Queries.GetConversation(ctx, conversationID)
		if err != nil {
			return fmt.Errorf("failed to get conversation: %w", err)
		}
		game, err := s.Queries.GetGame(ctx, conv.GameID)
		if err != nil {
			return fmt.Errorf("failed to get game: %w", err)
		}
		participantUserID = game.GmUserID
	} else {
		participantUserID = char.UserID.Int32
	}

	_, err = s.Queries.AddConversationParticipant(ctx, models.AddConversationParticipantParams{
		ConversationID: conversationID,
		UserID:         participantUserID,
		CharacterID:    pgtype.Int4{Int32: characterID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to add participant: %w", err)
	}

	return nil
}

// notifyPrivateMessage triggers notifications for all conversation participants except the sender
// This runs in a goroutine and should not fail the parent operation
func (s *ConversationService) notifyPrivateMessage(ctx context.Context, conversationID, senderUserID, senderCharacterID int32, messageID int32) {
	notificationService := NewNotificationService(s.DB, s.logger())

	// Get conversation details to find game_id
	conv, err := s.Queries.GetConversation(ctx, conversationID)
	if err != nil {
		slog.Error("Failed to get conversation for notifications", "error", err, "conversation_id", conversationID)
		return
	}

	// Get sender character name
	senderCharName := "Unknown"
	if senderCharacterID != 0 {
		senderChar, err := s.Queries.GetCharacter(ctx, senderCharacterID)
		if err == nil {
			senderCharName = senderChar.Name
		}
	}

	// Get all participants
	participants, err := s.Queries.GetConversationParticipants(ctx, conversationID)
	if err != nil {
		slog.Error("Failed to get conversation participants for notifications", "error", err, "conversation_id", conversationID)
		return
	}

	// Notify each participant except the sender, deduplicating by user so that
	// a user controlling multiple characters in the conversation gets at most one notification.
	notifiedUsers := make(map[int32]bool)
	for _, participant := range participants {
		if participant.UserID == senderUserID {
			continue // Don't notify the sender
		}
		if notifiedUsers[participant.UserID] {
			continue // Already notified this user for another character they control
		}
		notifiedUsers[participant.UserID] = true

		err = notificationService.NotifyPrivateMessage(
			ctx,
			participant.UserID,
			messageID,
			conv.GameID,
			conversationID,
			senderCharName,
		)
		if err != nil {
			slog.Error("Failed to send private message notification", "error", err, "recipient_user_id", participant.UserID)
		}
	}
}

// MarkConversationRead marks messages in a conversation as read up to a specific message
func (s *ConversationService) MarkConversationRead(ctx context.Context, userID int32, conversationID int32, lastReadMessageID int32) (*models.ConversationRead, error) {
	read, err := s.Queries.UpsertConversationRead(ctx, models.UpsertConversationReadParams{
		UserID:            userID,
		ConversationID:    conversationID,
		LastReadMessageID: pgtype.Int4{Int32: lastReadMessageID, Valid: true},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to mark conversation as read: %w", err)
	}

	return &read, nil
}

// GetConversationUnreadCount returns the number of unread messages in a conversation for a user
func (s *ConversationService) GetConversationUnreadCount(ctx context.Context, userID int32, conversationID int32) (int64, error) {
	count, err := s.Queries.GetConversationUnreadCount(ctx, models.GetConversationUnreadCountParams{
		ConversationID: conversationID,
		UserID:         userID,
	})

	if err != nil {
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}

	return count, nil
}

// GetFirstUnreadMessageID returns the ID of the first unread message in a conversation for a user
func (s *ConversationService) GetFirstUnreadMessageID(ctx context.Context, userID int32, conversationID int32) (*int32, error) {
	messageID, err := s.Queries.GetFirstUnreadMessageID(ctx, models.GetFirstUnreadMessageIDParams{
		ConversationID: conversationID,
		UserID:         userID,
	})

	if err != nil {
		// No unread messages is not an error
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get first unread message: %w", err)
	}

	return &messageID, nil
}

// GetUserConversationRead returns the read tracking info for a user and conversation
func (s *ConversationService) GetUserConversationRead(ctx context.Context, userID int32, conversationID int32) (*models.ConversationRead, error) {
	read, err := s.Queries.GetUserConversationRead(ctx, models.GetUserConversationReadParams{
		UserID:         userID,
		ConversationID: conversationID,
	})

	if err != nil {
		// No read marker is not an error - user hasn't read anything yet
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get conversation read info: %w", err)
	}

	return &read, nil
}

// UpdatePrivateMessage edits the content of a private message.
// Only the message sender can edit their own messages, and only if not deleted.
func (s *ConversationService) UpdatePrivateMessage(ctx context.Context, messageID int32, userID int32, content string) (*models.PrivateMessage, error) {
	if err := validation.ValidatePrivateMessage(content); err != nil {
		return nil, err
	}

	// Get the message to verify ownership and status
	message, err := s.Queries.GetPrivateMessage(ctx, messageID)
	if err != nil {
		return nil, fmt.Errorf("message not found")
	}

	// Verify the user is the sender
	if message.SenderUserID != userID {
		return nil, fmt.Errorf("forbidden: you can only edit your own messages")
	}

	// Cannot edit a deleted message
	if message.IsDeleted.Valid && message.IsDeleted.Bool {
		return nil, fmt.Errorf("cannot edit a deleted message")
	}

	updated, err := s.Queries.UpdatePrivateMessage(ctx, models.UpdatePrivateMessageParams{
		ID:           messageID,
		Content:      content,
		SenderUserID: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update message: %w", err)
	}

	return &updated, nil
}

// DeletePrivateMessage soft-deletes a private message
// Only the message sender can delete their own messages
func (s *ConversationService) DeletePrivateMessage(ctx context.Context, messageID int32, userID int32) error {
	// Get the message to verify ownership
	message, err := s.Queries.GetPrivateMessage(ctx, messageID)
	if err != nil {
		return fmt.Errorf("message not found")
	}

	// Verify the user is the sender
	if message.SenderUserID != userID {
		return fmt.Errorf("forbidden: you can only delete your own messages")
	}

	// Perform soft delete
	err = s.Queries.SoftDeletePrivateMessage(ctx, models.SoftDeletePrivateMessageParams{
		ID:           messageID,
		SenderUserID: userID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	return nil
}

// CanUserAccessConversation checks if a user has valid access to a conversation.
// This re-validates access by checking current character ownership, not just participant records.
//
// Access is granted if any of these conditions are true:
// 1. User is the GM of the game
// 2. User is an audience member for the game
// 3. User currently controls at least one character in the conversation
//
// This prevents the security issue where a user retains access after their character is reassigned.
func (s *ConversationService) CanUserAccessConversation(ctx context.Context, conversationID int32, userID int32, isAdmin bool) (bool, error) {
	// Get conversation to find the game
	conv, err := s.Queries.GetConversation(ctx, conversationID)
	if err != nil {
		return false, fmt.Errorf("failed to get conversation: %w", err)
	}

	// Get game to check GM and audience status
	game, err := s.Queries.GetGame(ctx, conv.GameID)
	if err != nil {
		return false, fmt.Errorf("failed to get game: %w", err)
	}

	// Check if user is GM (includes primary GM, co-GM, and admin mode)
	// Note: We can't check admin mode here since we don't have the HTTP request
	// The handler should check this separately or pass admin mode as a parameter
	if game.GmUserID == userID || core.IsUserCoGM(ctx, s.DB, conv.GameID, userID) {
		return true, nil
	}

	// Check if user is co-GM
	if core.IsUserCoGM(ctx, s.DB, conv.GameID, userID) {
		return true, nil
	}

	// Check if user is audience member
	if core.IsUserAudience(ctx, s.DB, conv.GameID, userID) {
		return true, nil
	}

	// Check if user currently controls any character in the conversation
	participants, err := s.GetConversationParticipants(ctx, conversationID)
	if err != nil {
		return false, fmt.Errorf("failed to get participants: %w", err)
	}

	for _, participant := range participants {
		if participant.CharacterID.Valid {
			// Check if user currently controls this character
			if core.CanUserControlNPC(ctx, s.DB, participant.CharacterID.Int32, userID) {
				return true, nil
			}
		}
	}

	// User doesn't have valid access
	return false, nil
}

// GetConversation retrieves a single conversation by ID.
func (s *ConversationService) GetConversation(ctx context.Context, conversationID int32) (*models.Conversation, error) {
	conv, err := s.Queries.GetConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	return &conv, nil
}

// DeleteConversation permanently removes a conversation that has no messages.
// This exists so a user who creates a conversation by accident can clean it up;
// once anything has been said the conversation is history and stays put.
//
// Only the creator or the game's GM/co-GM may delete. Participants and read
// markers are removed by ON DELETE CASCADE.
func (s *ConversationService) DeleteConversation(ctx context.Context, conversationID int32, userID int32) error {
	conv, err := s.Queries.GetConversation(ctx, conversationID)
	if err != nil {
		// Distinguish "no such conversation" from a genuine DB failure so the
		// handler can render 404 vs 500, and wrap so the cause survives logging.
		if errors.Is(err, pgx.ErrNoRows) {
			return core.ErrConversationNotFound
		}
		return fmt.Errorf("failed to get conversation: %w", err)
	}

	game, err := s.Queries.GetGame(ctx, conv.GameID)
	if err != nil {
		return fmt.Errorf("failed to get game: %w", err)
	}

	isCreator := conv.CreatedByUserID == userID
	isGM := game.GmUserID == userID || core.IsUserCoGM(ctx, s.DB, conv.GameID, userID)
	if !isCreator && !isGM {
		return core.ErrConversationDeleteForbidden
	}

	// The emptiness check lives inside the DELETE so it cannot race with a
	// message being sent. Affecting zero rows means the conversation was not
	// empty (it existed a moment ago, per GetConversation above). Soft-deleted
	// messages still count: a conversation whose only message was deleted has
	// history and must not be removable.
	rows, err := s.Queries.DeleteEmptyConversation(ctx, conversationID)
	if err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}
	if rows == 0 {
		return core.ErrConversationNotEmpty
	}

	return nil
}

// GetPrivateMessage retrieves a single private message by ID.
func (s *ConversationService) GetPrivateMessage(ctx context.Context, messageID int32) (*models.PrivateMessage, error) {
	msg, err := s.Queries.GetPrivateMessage(ctx, messageID)
	if err != nil {
		return nil, err
	}
	return &msg, nil
}
