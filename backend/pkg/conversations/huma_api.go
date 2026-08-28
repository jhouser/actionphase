package conversations

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	"actionphase/pkg/validation"
)

// Input / output types
//
// The chi handlers wrote anonymous map[string]interface{} envelopes straight to
// the ResponseWriter, so the response shapes were never expressible in the
// spec. These named structs reproduce those envelopes key for key -- same keys,
// same nesting, same empty-slice-not-null behaviour.

type conversationsListOutput struct {
	Body struct {
		Conversations any `json:"conversations" doc:"Conversations visible to the caller"`
	}
}

type conversationDetailOutput struct {
	Body struct {
		Conversation *models.Conversation                    `json:"conversation"`
		Participants []models.GetConversationParticipantsRow `json:"participants"`
	}
}

type messagesOutput struct {
	Body struct {
		Messages []models.GetConversationMessagesRow `json:"messages"`
	}
}

// successOutput is the {"success": true} envelope used by the read-marker and
// participant endpoints.
type successOutput struct {
	Body struct {
		Success bool `json:"success" doc:"Always true; the operation failed otherwise"`
	}
}

// deletedOutput is the {"message": ..., "id": ...} envelope returned by both
// delete endpoints. Note it is a 200 with a body, not a 204.
type deletedOutput struct {
	Body struct {
		Message string `json:"message" doc:"Confirmation message"`
		ID      int64  `json:"id" doc:"ID of the deleted record"`
	}
}

type createConversationInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
	Body   *CreateConversationRequest
}

type createConversationOutput struct {
	Body *models.Conversation
}

type listConversationsInput struct {
	GameID     int32 `path:"gameID" doc:"Game ID"`
	UnreadOnly bool  `query:"unread_only" doc:"Return only conversations with unread messages"`
	// Only consulted when unread_only is set.
	Limit int32 `query:"limit" default:"10" doc:"Maximum unread conversations to return"`
}

type conversationIDInput struct {
	GameID         int32 `path:"gameID" doc:"Game ID"`
	ConversationID int32 `path:"conversationId" doc:"Conversation ID"`
}

type getMessagesInput struct {
	GameID         int32 `path:"gameID" doc:"Game ID"`
	ConversationID int32 `path:"conversationId" doc:"Conversation ID"`
	// context_for=<messageID> returns just that message and the one before it,
	// for previewing an unread message without reading the whole conversation.
	ContextFor int32 `query:"context_for" required:"false" doc:"Return only this message and the one preceding it"`
}

type sendMessageInput struct {
	GameID         int32 `path:"gameID" doc:"Game ID"`
	ConversationID int32 `path:"conversationId" doc:"Conversation ID"`
	Body           *SendMessageRequest
}

type messageOutput struct {
	Body *models.PrivateMessage
}

type addParticipantInput struct {
	GameID         int32 `path:"gameID" doc:"Game ID"`
	ConversationID int32 `path:"conversationId" doc:"Conversation ID"`
	Body           *AddParticipantRequest
}

type messageIDInput struct {
	GameID         int32 `path:"gameID" doc:"Game ID"`
	ConversationID int32 `path:"conversationId" doc:"Conversation ID"`
	MessageID      int32 `path:"messageId" doc:"Message ID"`
}

type updateMessageInput struct {
	GameID         int32 `path:"gameID" doc:"Game ID"`
	ConversationID int32 `path:"conversationId" doc:"Conversation ID"`
	MessageID      int32 `path:"messageId" doc:"Message ID"`
	Body           *UpdateMessageRequest
}

// RegisterHumaConversations registers the conversation operations. Paths are
// relative to the /{gameID} subrouter this package is mounted under, so the
// game-scoped huma API must be shared with the other packages there.
func RegisterHumaConversations(api huma.API, h *Handler) {
	huma.Register(api, huma.Operation{
		OperationID:   "createConversation",
		Method:        http.MethodPost,
		Path:          "/conversations",
		Summary:       "Create a conversation",
		Description:   "Starts a private conversation between two or more characters.",
		Tags:          []string{"Conversations"},
		DefaultStatus: http.StatusCreated,
	}, h.createConversation)

	huma.Register(api, huma.Operation{
		OperationID: "listConversations",
		Method:      http.MethodGet,
		Path:        "/conversations",
		Summary:     "List the caller's conversations",
		Tags:        []string{"Conversations"},
	}, h.listConversations)

	huma.Register(api, huma.Operation{
		OperationID: "getConversation",
		Method:      http.MethodGet,
		Path:        "/conversations/{conversationId}",
		Summary:     "Get conversation details",
		Tags:        []string{"Conversations"},
	}, h.getConversation)

	huma.Register(api, huma.Operation{
		OperationID: "deleteConversation",
		Method:      http.MethodDelete,
		Path:        "/conversations/{conversationId}",
		Summary:     "Delete an empty conversation",
		Description: "Only the creator or a GM may delete, and only while the conversation has no messages.",
		Tags:        []string{"Conversations"},
	}, h.deleteConversation)

	huma.Register(api, huma.Operation{
		OperationID: "getConversationMessages",
		Method:      http.MethodGet,
		Path:        "/conversations/{conversationId}/messages",
		Summary:     "Get conversation messages",
		Tags:        []string{"Conversations"},
	}, h.getConversationMessages)

	huma.Register(api, huma.Operation{
		OperationID:   "sendConversationMessage",
		Method:        http.MethodPost,
		Path:          "/conversations/{conversationId}/messages",
		Summary:       "Send a message",
		Description:   "Only during common room or interlude phases.",
		Tags:          []string{"Conversations"},
		DefaultStatus: http.StatusCreated,
	}, h.sendMessage)

	huma.Register(api, huma.Operation{
		OperationID: "deleteConversationMessage",
		Method:      http.MethodDelete,
		Path:        "/conversations/{conversationId}/messages/{messageId}",
		Summary:     "Delete a message",
		Tags:        []string{"Conversations"},
	}, h.deleteMessage)

	huma.Register(api, huma.Operation{
		OperationID: "updateConversationMessage",
		Method:      http.MethodPatch,
		Path:        "/conversations/{conversationId}/messages/{messageId}",
		Summary:     "Edit a message",
		Tags:        []string{"Conversations"},
	}, h.updateMessage)

	huma.Register(api, huma.Operation{
		OperationID: "markConversationRead",
		Method:      http.MethodPost,
		Path:        "/conversations/{conversationId}/read",
		Summary:     "Mark a conversation as read",
		Tags:        []string{"Conversations"},
	}, h.markAsRead)

	huma.Register(api, huma.Operation{
		OperationID: "addConversationParticipant",
		Method:      http.MethodPost,
		Path:        "/conversations/{conversationId}/participants",
		Summary:     "Add a participant",
		Tags:        []string{"Conversations"},
	}, h.addParticipant)
}

// authUser resolves the caller. The auth middleware runs ahead of every route
// here, so a missing user means the middleware was not applied.
func (h *Handler) authUser(ctx context.Context) (*core.AuthenticatedUser, error) {
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.App.Logger.Error("No authenticated user in context")
		return nil, huma.Error401Unauthorized("authentication required")
	}
	return authUser, nil
}

// requireAccess gates on conversation access, so these endpoints can't be used
// to probe for the existence of conversations the caller can't see.
func (h *Handler) requireAccess(ctx context.Context, conversationID, userID int32, isAdmin bool, what string) error {
	canAccess, err := h.ConversationService.CanUserAccessConversation(ctx, conversationID, userID, isAdmin)
	if err != nil {
		// A missing conversation surfaces here as an error rather than
		// canAccess=false, so translate it instead of reporting a server fault.
		if errors.Is(err, pgx.ErrNoRows) {
			return huma.Error404NotFound("conversation not found")
		}
		h.App.Logger.Error("Failed to check conversation access", "error", err,
			"conversation_id", conversationID, "user_id", userID)
		return huma.Error500InternalServerError(what)
	}
	if !canAccess {
		return huma.Error403Forbidden("you don't have access to this conversation")
	}
	return nil
}

func (h *Handler) createConversation(ctx context.Context, in *createConversationInput) (*createConversationOutput, error) {
	authUser, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}
	userID := int32(authUser.ID)

	// An empty or whitespace-only title is rejected by the schema and the
	// trimming resolver on CreateConversationRequest before reaching here.
	if len(in.Body.CharacterIDs) < 2 {
		return nil, huma.Error400BadRequest("at least 2 characters required for a conversation")
	}

	game, ok := ctx.Value("game").(*models.Game)
	if !ok || game == nil {
		h.App.Logger.Error("No game in context")
		return nil, huma.Error500InternalServerError("Failed to create conversation")
	}
	if !game.AllowGroupConversations && len(in.Body.CharacterIDs) > 2 {
		return nil, huma.Error400BadRequest("group conversations are not allowed in this game")
	}

	conv, err := h.ConversationService.CreateConversation(ctx, core.CreateConversationRequest{
		GameID:          in.GameID,
		Title:           in.Body.Title,
		CreatedByUserID: userID,
		ParticipantIDs:  in.Body.CharacterIDs,
	})
	if err != nil {
		h.App.Logger.Error("Failed to create conversation", "error", err, "game_id", in.GameID, "user_id", userID)
		return nil, huma.Error500InternalServerError("Failed to create conversation")
	}

	h.App.Logger.Info("Conversation created successfully", "conversation_id", conv.ID, "game_id", in.GameID, "user_id", userID)
	return &createConversationOutput{Body: conv}, nil
}

func (h *Handler) listConversations(ctx context.Context, in *listConversationsInput) (*conversationsListOutput, error) {
	authUser, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}
	userID := int32(authUser.ID)

	out := &conversationsListOutput{}

	if in.UnreadOnly {
		limit := in.Limit
		if limit <= 0 {
			limit = 10
		}
		unread, err := h.ConversationService.GetUserUnreadConversations(ctx, in.GameID, userID, limit)
		if err != nil {
			h.App.Logger.Error("Failed to get unread conversations", "error", err, "game_id", in.GameID, "user_id", userID)
			return nil, huma.Error500InternalServerError("Failed to get unread conversations")
		}
		if unread == nil {
			unread = []models.GetUserUnreadConversationsRow{}
		}
		out.Body.Conversations = unread
		return out, nil
	}

	conversations, err := h.ConversationService.GetUserConversations(ctx, in.GameID, userID)
	if err != nil {
		h.App.Logger.Error("Failed to get user conversations", "error", err, "game_id", in.GameID, "user_id", userID)
		return nil, huma.Error500InternalServerError("Failed to get user conversations")
	}
	if conversations == nil {
		conversations = []models.GetUserConversationsRow{}
	}
	out.Body.Conversations = conversations
	return out, nil
}

func (h *Handler) getConversation(ctx context.Context, in *conversationIDInput) (*conversationDetailOutput, error) {
	authUser, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}
	userID := int32(authUser.ID)

	if err := h.requireAccess(ctx, in.ConversationID, userID, authUser.IsAdmin, "Failed to get conversation"); err != nil {
		return nil, err
	}

	conv, err := h.ConversationService.GetConversation(ctx, in.ConversationID)
	if err != nil {
		h.App.Logger.Error("Failed to get conversation", "error", err, "conversation_id", in.ConversationID)
		return nil, huma.Error500InternalServerError("Failed to get conversation")
	}

	participants, err := h.ConversationService.GetConversationParticipants(ctx, in.ConversationID)
	if err != nil {
		h.App.Logger.Error("Failed to get participants", "error", err, "conversation_id", in.ConversationID)
		return nil, huma.Error500InternalServerError("Failed to get conversation")
	}

	out := &conversationDetailOutput{}
	out.Body.Conversation = conv
	out.Body.Participants = participants
	return out, nil
}

func (h *Handler) deleteConversation(ctx context.Context, in *conversationIDInput) (*deletedOutput, error) {
	authUser, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}
	userID := int32(authUser.ID)

	if err := h.requireAccess(ctx, in.ConversationID, userID, authUser.IsAdmin, "Failed to delete conversation"); err != nil {
		return nil, err
	}

	if err := h.ConversationService.DeleteConversation(ctx, in.ConversationID, userID); err != nil {
		switch {
		case errors.Is(err, core.ErrConversationNotFound):
			// Deleted between the access check and here.
			return nil, huma.Error404NotFound("conversation not found")
		case errors.Is(err, core.ErrConversationNotEmpty):
			return nil, huma.Error409Conflict("conversations with messages cannot be deleted")
		case errors.Is(err, core.ErrConversationDeleteForbidden):
			return nil, huma.Error403Forbidden("only the creator or a GM can delete this conversation")
		default:
			h.App.Logger.Error("Failed to delete conversation", "error", err, "conversation_id", in.ConversationID, "user_id", userID)
			return nil, huma.Error500InternalServerError("Failed to delete conversation")
		}
	}

	h.App.Logger.Info("Conversation deleted successfully", "conversation_id", in.ConversationID, "user_id", userID)

	out := &deletedOutput{}
	out.Body.Message = "Conversation deleted successfully"
	out.Body.ID = int64(in.ConversationID)
	return out, nil
}

func (h *Handler) getConversationMessages(ctx context.Context, in *getMessagesInput) (*messagesOutput, error) {
	authUser, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}
	userID := int32(authUser.ID)

	if err := h.requireAccess(ctx, in.ConversationID, userID, authUser.IsAdmin, "Failed to get conversation messages"); err != nil {
		return nil, err
	}

	var messages []models.GetConversationMessagesRow
	if in.ContextFor != 0 {
		messages, err = h.ConversationService.GetMessageWithContext(ctx, in.ConversationID, in.ContextFor, userID)
	} else {
		messages, err = h.ConversationService.GetConversationMessages(ctx, in.ConversationID, userID)
	}
	if err != nil {
		if errors.Is(err, core.ErrNotConversationParticipant) {
			return nil, huma.Error403Forbidden("you don't have access to this conversation")
		}
		h.App.Logger.Error("Failed to get conversation messages", "error", err,
			"conversation_id", in.ConversationID, "user_id", userID)
		return nil, huma.Error500InternalServerError("Failed to get conversation messages")
	}

	// Return an empty array rather than null.
	if messages == nil {
		messages = []models.GetConversationMessagesRow{}
	}

	queries := models.New(h.App.Pool)
	game, err := queries.GetGame(ctx, in.GameID)
	if err != nil {
		h.App.Logger.Error("Failed to get game", "error", err, "game_id", in.GameID)
		return nil, huma.Error500InternalServerError("Failed to get game")
	}
	// Anonymous games hide the account behind each character until the reveal.
	if !core.CanSeeUsernamesInAnonymousGame(ctx, h.App.Pool, game, userID) {
		for i := range messages {
			messages[i].SenderUsername = ""
		}
	}

	out := &messagesOutput{}
	out.Body.Messages = messages
	return out, nil
}

func (h *Handler) sendMessage(ctx context.Context, in *sendMessageInput) (*messageOutput, error) {
	authUser, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}
	userID := int32(authUser.ID)

	if in.Body.Content == "" {
		return nil, huma.Error400BadRequest("message content is required")
	}
	if err := validation.ValidatePrivateMessage(in.Body.Content); err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}

	participants, err := h.ConversationService.GetConversationParticipants(ctx, in.ConversationID)
	if err != nil {
		h.App.Logger.Error("Failed to get conversation participants", "error", err, "conversation_id", in.ConversationID)
		return nil, huma.Error500InternalServerError("Failed to send message")
	}

	isCharacterInConversation := false
	for _, p := range participants {
		if p.CharacterID.Valid && p.CharacterID.Int32 == in.Body.CharacterID {
			isCharacterInConversation = true
			break
		}
	}
	if !isCharacterInConversation {
		h.App.Logger.Warn("Character not in conversation", "character_id", in.Body.CharacterID, "conversation_id", in.ConversationID)
		return nil, huma.Error403Forbidden("character is not a participant in this conversation")
	}

	// Owning the character is not enough: NPCs are controllable by others.
	if !core.CanUserControlNPC(ctx, h.App.Pool, in.Body.CharacterID, userID) {
		h.App.Logger.Warn("User cannot control character", "character_id", in.Body.CharacterID, "user_id", userID)
		return nil, huma.Error403Forbidden("you cannot send messages as this character")
	}

	character, err := h.CharacterService.GetCharacter(ctx, in.Body.CharacterID)
	if err != nil {
		h.App.Logger.Error("Failed to get character", "error", err, "character_id", in.Body.CharacterID)
		return nil, huma.Error500InternalServerError("Failed to send message")
	}

	if err := h.requireConversationPhase(ctx, character.GameID, "sent"); err != nil {
		return nil, err
	}

	message, err := h.ConversationService.SendMessage(ctx, core.SendConversationMessageRequest{
		ConversationID:    in.ConversationID,
		SenderUserID:      userID,
		SenderCharacterID: in.Body.CharacterID,
		Content:           in.Body.Content,
	})
	if err != nil {
		if errors.Is(err, core.ErrNotConversationParticipant) {
			return nil, huma.Error403Forbidden("you don't have access to this conversation")
		}
		h.App.Logger.Error("Failed to send message", "error", err, "conversation_id", in.ConversationID, "user_id", userID)
		return nil, huma.Error500InternalServerError("Failed to send message")
	}

	h.App.Logger.Info("Message sent successfully", "message_id", message.ID, "conversation_id", in.ConversationID, "author", authUser.Username)
	return &messageOutput{Body: message}, nil
}

// requireConversationPhase enforces that private messages are only written
// while the game is in a common room or interlude phase. verb is "sent" or
// "edited", matching the wording of the original error messages.
func (h *Handler) requireConversationPhase(ctx context.Context, gameID int32, verb string) error {
	activePhase, err := h.PhaseService.GetActivePhase(ctx, gameID)
	if err != nil {
		h.App.Logger.Error("Failed to get active phase", "error", err, "game_id", gameID)
		return huma.Error500InternalServerError("Failed to get active phase")
	}
	if activePhase == nil || (activePhase.PhaseType != core.PhaseTypeCommonRoom && activePhase.PhaseType != core.PhaseTypeInterlude) {
		h.App.Logger.Warn("Cannot write private message outside common room or interlude phase", "game_id", gameID)
		return huma.Error403Forbidden("private messages can only be " + verb + " during common room or interlude phases")
	}
	return nil
}

func (h *Handler) markAsRead(ctx context.Context, in *conversationIDInput) (*successOutput, error) {
	authUser, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}
	userID := int32(authUser.ID)

	if err := h.ConversationService.MarkConversationAsRead(ctx, in.ConversationID, userID); err != nil {
		if errors.Is(err, core.ErrNotConversationParticipant) {
			return nil, huma.Error403Forbidden("you don't have access to this conversation")
		}
		h.App.Logger.Error("Failed to mark conversation as read", "error", err, "conversation_id", in.ConversationID, "user_id", userID)
		return nil, huma.Error500InternalServerError("Failed to mark as read")
	}

	out := &successOutput{}
	out.Body.Success = true
	return out, nil
}

func (h *Handler) addParticipant(ctx context.Context, in *addParticipantInput) (*successOutput, error) {
	authUser, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}
	userID := int32(authUser.ID)

	if err := h.requireAccess(ctx, in.ConversationID, userID, authUser.IsAdmin, "Failed to add participant"); err != nil {
		return nil, err
	}

	if err := h.ConversationService.AddParticipant(ctx, in.ConversationID, in.Body.CharacterID); err != nil {
		h.App.Logger.Error("Failed to add participant", "error", err, "conversation_id", in.ConversationID, "character_id", in.Body.CharacterID)
		return nil, huma.Error500InternalServerError("Failed to add participant")
	}

	h.App.Logger.Info("Participant added successfully", "conversation_id", in.ConversationID, "character_id", in.Body.CharacterID)

	out := &successOutput{}
	out.Body.Success = true
	return out, nil
}

func (h *Handler) updateMessage(ctx context.Context, in *updateMessageInput) (*messageOutput, error) {
	authUser, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}
	userID := int32(authUser.ID)

	if in.Body.Content == "" {
		return nil, huma.Error400BadRequest("message content is required")
	}
	if err := validation.ValidatePrivateMessage(in.Body.Content); err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}

	if err := h.requireAccess(ctx, in.ConversationID, userID, authUser.IsAdmin, "Failed to update message"); err != nil {
		return nil, err
	}

	msg, err := h.ConversationService.GetPrivateMessage(ctx, in.MessageID)
	if err != nil {
		return nil, huma.Error404NotFound("message not found")
	}

	conv, err := h.ConversationService.GetConversation(ctx, in.ConversationID)
	if err != nil {
		h.App.Logger.Error("Failed to get conversation", "error", err, "conversation_id", in.ConversationID)
		return nil, huma.Error500InternalServerError("Failed to update message")
	}

	// Same phase gate as sending.
	if err := h.requireConversationPhase(ctx, conv.GameID, "edited"); err != nil {
		return nil, err
	}

	updated, err := h.ConversationService.UpdatePrivateMessage(ctx, in.MessageID, userID, in.Body.Content)
	if err != nil {
		// The service reports these by message text rather than sentinel error.
		switch err.Error() {
		case "message not found":
			return nil, huma.Error404NotFound("message not found")
		case "forbidden: you can only edit your own messages":
			return nil, huma.Error403Forbidden("you can only edit your own messages")
		case "cannot edit a deleted message":
			return nil, huma.Error400BadRequest("cannot edit a deleted message")
		}
		h.App.Logger.Error("Failed to update message", "error", err, "message_id", in.MessageID, "user_id", userID)
		return nil, huma.Error500InternalServerError("Failed to update message")
	}

	h.App.Logger.Info("Message updated successfully", "message_id", msg.ID, "conversation_id", in.ConversationID, "user_id", userID)
	return &messageOutput{Body: updated}, nil
}

func (h *Handler) deleteMessage(ctx context.Context, in *messageIDInput) (*deletedOutput, error) {
	authUser, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}
	userID := int32(authUser.ID)

	// No access check here: the service authorizes the delete itself, and the
	// chi handler relied on that too.
	if err := h.ConversationService.DeletePrivateMessage(ctx, in.MessageID, userID); err != nil {
		switch err.Error() {
		case "message not found":
			return nil, huma.Error404NotFound("message not found")
		case "forbidden: you can only delete your own messages":
			return nil, huma.Error403Forbidden("you can only delete your own messages")
		}
		h.App.Logger.Error("Failed to delete message", "error", err, "message_id", in.MessageID, "user_id", userID)
		return nil, huma.Error500InternalServerError("Failed to delete message")
	}

	h.App.Logger.Info("Message deleted successfully", "message_id", in.MessageID, "conversation_id", in.ConversationID, "user_id", userID)

	out := &deletedOutput{}
	out.Body.Message = "Message deleted successfully"
	out.Body.ID = int64(in.MessageID)
	return out, nil
}
