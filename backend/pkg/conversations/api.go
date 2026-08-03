package conversations

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"actionphase/pkg/core"
	db "actionphase/pkg/db/models"
	models "actionphase/pkg/db/models"
	"actionphase/pkg/validation"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/jackc/pgx/v5"
)

// Handler handles HTTP requests for conversations
type Handler struct {
	App                 *core.App
	GameService         core.GameServiceInterface
	CharacterService    core.CharacterServiceInterface
	ConversationService core.ConversationServiceInterface
	PhaseService        core.PhaseServiceInterface
}

// RegisterRoutes registers all conversation routes
// Note: This is called from within the games router, so gameId is already in the path context
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/conversations", func(r chi.Router) {
		r.Post("/", h.CreateConversation)                                   // Create new conversation
		r.Get("/", h.GetUserConversations)                                  // Get user's conversations
		r.Get("/{conversationId}", h.GetConversation)                       // Get conversation details
		r.Delete("/{conversationId}", h.DeleteConversation)                 // Delete an empty conversation
		r.Get("/{conversationId}/messages", h.GetConversationMessages)      // Get messages
		r.Post("/{conversationId}/messages", h.SendMessage)                 // Send message
		r.Delete("/{conversationId}/messages/{messageId}", h.DeleteMessage) // Delete message
		r.Patch("/{conversationId}/messages/{messageId}", h.UpdateMessage)  // Edit message
		r.Post("/{conversationId}/read", h.MarkAsRead)                      // Mark as read
		r.Post("/{conversationId}/participants", h.AddParticipant)          // Add participant
	})
}

// CreateConversationRequest represents the request body for creating a conversation
type CreateConversationRequest struct {
	Title        string  `json:"title"`
	CharacterIDs []int32 `json:"character_ids"` // Characters participating
}

func (r *CreateConversationRequest) Bind(req *http.Request) error {
	return nil
}

// CreateConversation creates a new conversation
func (h *Handler) CreateConversation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(r.Context())
	if authUser == nil {
		h.App.Logger.Error("No authenticated user in context")
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "Unauthorized")
		return
	}

	userID := int32(authUser.ID)

	gameID := ctx.Value("gameID").(int32)

	data := &CreateConversationRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid create conversation request", "error", err)
		return
	}

	if data.Title == "" {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("conversation title is required")), "Invalid create conversation request")
		return
	}

	if len(data.CharacterIDs) < 2 {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("at least 2 characters required for a conversation")), "Invalid create conversation request")
		return
	}

	game := ctx.Value("game").(*db.Game)
	if !game.AllowGroupConversations && len(data.CharacterIDs) > 2 {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("group conversations are not allowed in this game")), "Invalid create conversation request")
		return
	}

	conv, err := h.ConversationService.CreateConversation(ctx, core.CreateConversationRequest{
		GameID:          int32(gameID),
		Title:           data.Title,
		CreatedByUserID: userID,
		ParticipantIDs:  data.CharacterIDs,
	})
	if err != nil {
		h.App.Logger.Error("Failed to create conversation", "error", err, "game_id", gameID, "user_id", userID)
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to create conversation", "error", err)
		return
	}

	h.App.Logger.Info("Conversation created successfully", "conversation_id", conv.ID, "game_id", gameID, "user_id", userID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(conv)
}

// GetUserConversations gets all conversations for the current user in a game
func (h *Handler) GetUserConversations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(r.Context())
	if authUser == nil {
		h.App.Logger.Error("No authenticated user in context")
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "Unauthorized")
		return
	}

	userID := int32(authUser.ID)

	gameID := ctx.Value("gameID").(int32)

	conversationService := h.ConversationService

	unreadOnly := r.URL.Query().Get("unread_only") == "true"

	if unreadOnly {
		limitStr := r.URL.Query().Get("limit")
		limit := int32(10)
		if n, err := strconv.ParseInt(limitStr, 10, 32); err == nil && n > 0 {
			limit = int32(n)
		}
		unread, err := conversationService.GetUserUnreadConversations(ctx, int32(gameID), userID, limit)
		if err != nil {
			h.App.Logger.Error("Failed to get unread conversations", "error", err, "game_id", gameID, "user_id", userID)
			h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get unread conversations", "error", err)
			return
		}
		if unread == nil {
			unread = []models.GetUserUnreadConversationsRow{}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{"conversations": unread}); err != nil {
			h.App.Logger.Error("Failed to encode unread conversations response", "error", err)
		}
		return
	}

	conversations, err := conversationService.GetUserConversations(ctx, int32(gameID), userID)
	if err != nil {
		h.App.Logger.Error("Failed to get user conversations", "error", err, "game_id", gameID, "user_id", userID)
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get user conversations", "error", err)
		return
	}

	if conversations == nil {
		conversations = []models.GetUserConversationsRow{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"conversations": conversations,
	})
}

// GetConversation gets details about a specific conversation
func (h *Handler) GetConversation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(r.Context())
	if authUser == nil {
		h.App.Logger.Error("No authenticated user in context")
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "Unauthorized")
		return
	}

	userID := int32(authUser.ID)

	conversationIDStr := chi.URLParam(r, "conversationId")
	conversationID, err := strconv.ParseInt(conversationIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid conversation ID")), "Invalid get conversation request")
		return
	}

	conversationService := h.ConversationService

	// Verify user has valid access (checks current character ownership, not just participant records)
	canAccess, err := conversationService.CanUserAccessConversation(ctx, int32(conversationID), userID, authUser.IsAdmin)
	if err != nil {
		h.App.Logger.Error("Failed to check conversation access", "error", err, "conversation_id", conversationID, "user_id", userID)
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get conversation", "error", err)
		return
	}
	if !canAccess {
		h.renderError(ctx, w, r, core.ErrForbidden("you don't have access to this conversation"), "Get conversation forbidden")
		return
	}

	// Get conversation details
	conv, err := h.ConversationService.GetConversation(ctx, int32(conversationID))
	if err != nil {
		h.App.Logger.Error("Failed to get conversation", "error", err, "conversation_id", conversationID)
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get conversation", "error", err)
		return
	}

	// Get participants
	participants, err := conversationService.GetConversationParticipants(ctx, int32(conversationID))
	if err != nil {
		h.App.Logger.Error("Failed to get participants", "error", err, "conversation_id", conversationID)
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get conversation", "error", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"conversation": conv,
		"participants": participants,
	})
}

// DeleteConversation deletes a conversation that has no messages in it.
// Intended for cleaning up conversations created by accident; the service
// rejects the request if any message exists or the user isn't the creator/GM.
func (h *Handler) DeleteConversation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authUser := core.GetAuthenticatedUser(r.Context())
	if authUser == nil {
		h.App.Logger.Error("No authenticated user in context")
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "Unauthorized")
		return
	}

	userID := int32(authUser.ID)

	conversationIDStr := chi.URLParam(r, "conversationId")
	conversationID, err := strconv.ParseInt(conversationIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid conversation ID")), "Invalid delete conversation request")
		return
	}

	conversationService := h.ConversationService

	// Gate on general conversation access first so this endpoint can't be used
	// to probe for the existence of conversations the user can't see.
	canAccess, err := conversationService.CanUserAccessConversation(ctx, int32(conversationID), userID, authUser.IsAdmin)
	if err != nil {
		// A missing conversation surfaces here as an error rather than
		// canAccess=false, so translate it instead of reporting a server fault.
		if errors.Is(err, pgx.ErrNoRows) {
			h.renderError(ctx, w, r, core.ErrNotFound("conversation not found"), "Delete conversation not found")
			return
		}
		h.App.Logger.Error("Failed to check conversation access", "error", err, "conversation_id", conversationID, "user_id", userID)
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to delete conversation", "error", err)
		return
	}
	if !canAccess {
		h.renderError(ctx, w, r, core.ErrForbidden("you don't have access to this conversation"), "Delete conversation forbidden")
		return
	}

	if err := conversationService.DeleteConversation(ctx, int32(conversationID), userID); err != nil {
		switch {
		case errors.Is(err, core.ErrConversationNotFound):
			// Deleted between the access check and here.
			h.renderError(ctx, w, r, core.ErrNotFound("conversation not found"), "Delete conversation not found")
		case errors.Is(err, core.ErrConversationNotEmpty):
			h.renderError(ctx, w, r, core.ErrConflict("conversations with messages cannot be deleted"), "Delete conversation conflict")
		case errors.Is(err, core.ErrConversationDeleteForbidden):
			h.renderError(ctx, w, r, core.ErrForbidden("only the creator or a GM can delete this conversation"), "Delete conversation forbidden")
		default:
			h.App.Logger.Error("Failed to delete conversation", "error", err, "conversation_id", conversationID, "user_id", userID)
			h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to delete conversation", "error", err)
		}
		return
	}

	h.App.Logger.Info("Conversation deleted successfully", "conversation_id", conversationID, "user_id", userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Conversation deleted successfully",
		"id":      conversationID,
	})
}

// GetConversationMessages gets all messages in a conversation
func (h *Handler) GetConversationMessages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(r.Context())
	if authUser == nil {
		h.App.Logger.Error("No authenticated user in context")
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "Unauthorized")
		return
	}

	userID := int32(authUser.ID)

	gameID := ctx.Value("gameID").(int32)

	conversationIDStr := chi.URLParam(r, "conversationId")
	conversationID, err := strconv.ParseInt(conversationIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid conversation ID")), "Invalid get conversation messages request")
		return
	}

	conversationService := h.ConversationService

	// Verify user has valid access (checks current character ownership, not just participant records)
	canAccess, err := conversationService.CanUserAccessConversation(ctx, int32(conversationID), userID, authUser.IsAdmin)
	if err != nil {
		h.App.Logger.Error("Failed to check conversation access", "error", err, "conversation_id", conversationID, "user_id", userID)
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get conversation messages", "error", err)
		return
	}
	if !canAccess {
		h.renderError(ctx, w, r, core.ErrForbidden("you don't have access to this conversation"), "Get conversation messages forbidden")
		return
	}

	messages, err := conversationService.GetConversationMessages(ctx, int32(conversationID), userID)
	if err != nil {
		h.App.Logger.Error("Failed to get conversation messages", "error", err, "conversation_id", conversationID, "user_id", userID)
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get conversation messages", "error", err)
		return
	}

	// Ensure we return an empty array instead of null
	if messages == nil {
		messages = []models.GetConversationMessagesRow{}
	}

	queries := models.New(h.App.Pool)
	game, err := queries.GetGame(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game", "error", err, "game_id", gameID)
		return
	}
	if !core.CanSeeUsernamesInAnonymousGame(ctx, h.App.Pool, game, userID) {
		for i := range messages {
			messages[i].SenderUsername = ""
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"messages": messages,
	})
}

// SendMessageRequest represents the request body for sending a message
type SendMessageRequest struct {
	CharacterID int32  `json:"character_id"` // Character sending the message
	Content     string `json:"content"`
}

func (r *SendMessageRequest) Bind(req *http.Request) error {
	return nil
}

// SendMessage sends a message in a conversation
func (h *Handler) SendMessage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(r.Context())
	if authUser == nil {
		h.App.Logger.Error("No authenticated user in context")
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "Unauthorized")
		return
	}

	userID := int32(authUser.ID)

	conversationIDStr := chi.URLParam(r, "conversationId")
	conversationID, err := strconv.ParseInt(conversationIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid conversation ID")), "Invalid send message request")
		return
	}

	data := &SendMessageRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid send message request", "error", err)
		return
	}

	if data.Content == "" {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("message content is required")), "Invalid send message request")
		return
	}

	if err := validation.ValidatePrivateMessage(data.Content); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid send message request", "error", err)
		return
	}

	conversationService := h.ConversationService

	// Verify the character is a participant in this conversation
	participants, err := conversationService.GetConversationParticipants(ctx, int32(conversationID))
	if err != nil {
		h.App.Logger.Error("Failed to get conversation participants", "error", err, "conversation_id", conversationID)
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to send message", "error", err)
		return
	}

	h.App.Logger.Info("Checking character participation", "character_id", data.CharacterID, "conversation_id", conversationID, "user_id", userID, "participants_count", len(participants))

	// Check if the selected character is in the conversation
	isCharacterInConversation := false
	for _, p := range participants {
		h.App.Logger.Info("Participant check", "participant_user_id", p.UserID, "participant_character_id", p.CharacterID, "target_character_id", data.CharacterID)
		if p.CharacterID.Valid && p.CharacterID.Int32 == data.CharacterID {
			isCharacterInConversation = true
			break
		}
	}

	if !isCharacterInConversation {
		h.App.Logger.Warn("Character not in conversation", "character_id", data.CharacterID, "conversation_id", conversationID)
		h.renderError(ctx, w, r, core.ErrForbidden("character is not a participant in this conversation"), "Send message forbidden")
		return
	}

	// Verify the user can control this character (either owns it or can control it as NPC)
	if !core.CanUserControlNPC(ctx, h.App.Pool, data.CharacterID, userID) {
		h.App.Logger.Warn("User cannot control character", "character_id", data.CharacterID, "user_id", userID)
		h.renderError(ctx, w, r, core.ErrForbidden("you cannot send messages as this character"), "Send message forbidden")
		return
	}

	// Get character to access game_id for phase validation
	character, err := h.CharacterService.GetCharacter(ctx, data.CharacterID)
	if err != nil {
		h.App.Logger.Error("Failed to get character", "error", err, "character_id", data.CharacterID)
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to send message", "error", err)
		return
	}

	// Validate that the game is in a common room phase
	activePhase, err := h.PhaseService.GetActivePhase(ctx, character.GameID)
	if err != nil {
		h.App.Logger.Error("Failed to get active phase", "error", err, "game_id", character.GameID)
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to send message", "error", err)
		return
	}

	if activePhase == nil || (activePhase.PhaseType != core.PhaseTypeCommonRoom && activePhase.PhaseType != core.PhaseTypeInterlude) {
		h.App.Logger.Warn("Cannot send private messages outside common room or interlude phase", "game_id", character.GameID, "phase_type", activePhase.PhaseType)
		h.renderError(ctx, w, r, core.ErrForbidden("private messages can only be sent during common room or interlude phases"), "Send message forbidden")
		return
	}

	message, err := h.ConversationService.SendMessage(ctx, core.SendConversationMessageRequest{
		ConversationID:    int32(conversationID),
		SenderUserID:      userID,
		SenderCharacterID: data.CharacterID,
		Content:           data.Content,
	})
	if err != nil {
		h.App.Logger.Error("Failed to send message", "error", err, "conversation_id", conversationID, "user_id", userID)
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to send message", "error", err)
		return
	}

	h.App.Logger.Info("Message sent successfully", "message_id", message.ID, "conversation_id", conversationID, "author", authUser.Username)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(message)
}

// MarkAsRead marks all messages in a conversation as read
func (h *Handler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(r.Context())
	if authUser == nil {
		h.App.Logger.Error("No authenticated user in context")
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "Unauthorized")
		return
	}

	userID := int32(authUser.ID)

	conversationIDStr := chi.URLParam(r, "conversationId")
	conversationID, err := strconv.ParseInt(conversationIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid conversation ID")), "Invalid mark as read request")
		return
	}

	conversationService := h.ConversationService
	if err := conversationService.MarkConversationAsRead(ctx, int32(conversationID), userID); err != nil {
		h.App.Logger.Error("Failed to mark conversation as read", "error", err, "conversation_id", conversationID, "user_id", userID)
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to mark as read", "error", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

// AddParticipantRequest represents the request body for adding a participant
type AddParticipantRequest struct {
	CharacterID int32 `json:"character_id"`
}

func (r *AddParticipantRequest) Bind(req *http.Request) error {
	return nil
}

// AddParticipant adds a character to an existing conversation
func (h *Handler) AddParticipant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(r.Context())
	if authUser == nil {
		h.App.Logger.Error("No authenticated user in context")
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "Unauthorized")
		return
	}

	userID := int32(authUser.ID)

	conversationIDStr := chi.URLParam(r, "conversationId")
	conversationID, err := strconv.ParseInt(conversationIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid conversation ID")), "Invalid add participant request")
		return
	}

	conversationService := h.ConversationService

	// Verify user has valid access (checks current character ownership, not just participant records)
	canAccess, err := conversationService.CanUserAccessConversation(ctx, int32(conversationID), userID, authUser.IsAdmin)
	if err != nil {
		h.App.Logger.Error("Failed to check conversation access", "error", err, "conversation_id", conversationID, "user_id", userID)
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to add participant", "error", err)
		return
	}
	if !canAccess {
		h.renderError(ctx, w, r, core.ErrForbidden("you don't have access to this conversation"), "Add participant forbidden")
		return
	}

	data := &AddParticipantRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid add participant request", "error", err)
		return
	}

	if err := conversationService.AddParticipant(ctx, int32(conversationID), data.CharacterID); err != nil {
		h.App.Logger.Error("Failed to add participant", "error", err, "conversation_id", conversationID, "character_id", data.CharacterID)
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to add participant", "error", err)
		return
	}

	h.App.Logger.Info("Participant added successfully", "conversation_id", conversationID, "character_id", data.CharacterID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

// UpdateMessageRequest represents the request body for editing a message
type UpdateMessageRequest struct {
	Content string `json:"content"`
}

func (r *UpdateMessageRequest) Bind(req *http.Request) error {
	return nil
}

// UpdateMessage edits an existing private message
func (h *Handler) UpdateMessage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authUser := core.GetAuthenticatedUser(r.Context())
	if authUser == nil {
		h.App.Logger.Error("No authenticated user in context")
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "Unauthorized")
		return
	}

	userID := int32(authUser.ID)

	conversationIDStr := chi.URLParam(r, "conversationId")
	conversationID, err := strconv.ParseInt(conversationIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid conversation ID")), "Invalid update message request")
		return
	}

	messageIDStr := chi.URLParam(r, "messageId")
	messageID, err := strconv.ParseInt(messageIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid message ID")), "Invalid update message request")
		return
	}

	data := &UpdateMessageRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid update message request", "error", err)
		return
	}

	if data.Content == "" {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("message content is required")), "Invalid update message request")
		return
	}

	if err := validation.ValidatePrivateMessage(data.Content); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid update message request", "error", err)
		return
	}

	conversationService := h.ConversationService

	// Verify user has valid access to the conversation
	canAccess, err := conversationService.CanUserAccessConversation(ctx, int32(conversationID), userID, authUser.IsAdmin)
	if err != nil {
		h.App.Logger.Error("Failed to check conversation access", "error", err, "conversation_id", conversationID, "user_id", userID)
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to update message", "error", err)
		return
	}
	if !canAccess {
		h.renderError(ctx, w, r, core.ErrForbidden("you don't have access to this conversation"), "Update message forbidden")
		return
	}

	// Get the message to find the character/game for phase validation
	msg, err := h.ConversationService.GetPrivateMessage(ctx, int32(messageID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("message not found"), "Update message not found")
		return
	}

	// Validate that the game is in a common room phase (same gate as sending)
	conv, err := h.ConversationService.GetConversation(ctx, int32(conversationID))
	if err != nil {
		h.App.Logger.Error("Failed to get conversation", "error", err, "conversation_id", conversationID)
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to update message", "error", err)
		return
	}

	activePhase, err := h.PhaseService.GetActivePhase(ctx, conv.GameID)
	if err != nil {
		h.App.Logger.Error("Failed to get active phase", "error", err, "game_id", conv.GameID)
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to update message", "error", err)
		return
	}

	if activePhase == nil || (activePhase.PhaseType != core.PhaseTypeCommonRoom && activePhase.PhaseType != core.PhaseTypeInterlude) {
		h.App.Logger.Warn("Cannot edit private message outside common room or interlude phase", "game_id", conv.GameID, "phase_type", activePhase.PhaseType)
		h.renderError(ctx, w, r, core.ErrForbidden("private messages can only be edited during common room or interlude phases"), "Update message forbidden")
		return
	}

	updated, err := conversationService.UpdatePrivateMessage(ctx, int32(messageID), userID, data.Content)
	if err != nil {
		if err.Error() == "message not found" {
			h.renderError(ctx, w, r, core.ErrNotFound("message not found"), "Update message not found")
			return
		}
		if err.Error() == "forbidden: you can only edit your own messages" {
			h.renderError(ctx, w, r, core.ErrForbidden("you can only edit your own messages"), "Update message forbidden")
			return
		}
		if err.Error() == "cannot edit a deleted message" {
			h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("cannot edit a deleted message")), "Invalid update message request")
			return
		}
		h.App.Logger.Error("Failed to update message", "error", err, "message_id", messageID, "user_id", userID)
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to update message", "error", err)
		return
	}

	h.App.Logger.Info("Message updated successfully", "message_id", msg.ID, "conversation_id", conversationID, "user_id", userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

// DeleteMessage deletes a private message (soft delete)
func (h *Handler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(r.Context())
	if authUser == nil {
		h.App.Logger.Error("No authenticated user in context")
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "Unauthorized")
		return
	}

	userID := int32(authUser.ID)

	conversationIDStr := chi.URLParam(r, "conversationId")
	conversationID, err := strconv.ParseInt(conversationIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid conversation ID")), "Invalid delete message request")
		return
	}

	messageIDStr := chi.URLParam(r, "messageId")
	messageID, err := strconv.ParseInt(messageIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid message ID")), "Invalid delete message request")
		return
	}

	conversationService := h.ConversationService

	// Delete the message (service handles authorization check)
	err = conversationService.DeletePrivateMessage(ctx, int32(messageID), userID)
	if err != nil {
		if err.Error() == "message not found" {
			h.renderError(ctx, w, r, core.ErrNotFound("message not found"), "Delete message not found")
			return
		}
		if err.Error() == "forbidden: you can only delete your own messages" {
			h.renderError(ctx, w, r, core.ErrForbidden("you can only delete your own messages"), "Delete message forbidden")
			return
		}
		h.App.Logger.Error("Failed to delete message", "error", err, "message_id", messageID, "user_id", userID)
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to delete message", "error", err)
		return
	}

	h.App.Logger.Info("Message deleted successfully", "message_id", messageID, "conversation_id", conversationID, "user_id", userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Message deleted successfully",
		"id":      messageID,
	})
}
