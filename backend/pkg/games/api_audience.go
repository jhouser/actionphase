package games

import (
	"actionphase/pkg/core"
	db "actionphase/pkg/db/models"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// ============================================================================
// Request/Response Types
// ============================================================================

type AudienceMemberResponse struct {
	ID       int32     `json:"id"`
	GameID   int32     `json:"game_id"`
	UserID   int32     `json:"user_id"`
	Username string    `json:"username"`
	Role     string    `json:"role"`
	Status   string    `json:"status"`
	JoinedAt time.Time `json:"joined_at"`
}

type ListAudienceMembersResponse struct {
	AudienceMembers []AudienceMemberResponse `json:"audience_members"`
}

type UpdateAutoAcceptAudienceRequest struct {
	AutoAcceptAudience bool `json:"auto_accept_audience"`
}

type PrivateConversationResponse struct {
	ConversationID          int32       `json:"conversation_id"`
	Subject                 *string     `json:"subject"`
	ConversationType        string      `json:"conversation_type"`
	CreatedAt               string      `json:"created_at"`
	MessageCount            int64       `json:"message_count"`
	LastMessageAt           interface{} `json:"last_message_at"`
	ParticipantNames        interface{} `json:"participant_names"`
	ParticipantUsernames    interface{} `json:"participant_usernames"`
	ParticipantCharacterIDs interface{} `json:"participant_character_ids"`
	LastMessageContent      *string     `json:"last_message_content"`
	LastSenderName          *string     `json:"last_sender_name"`
	LastSenderUsername      *string     `json:"last_sender_username"`
	LastSenderCharacterID   *int32      `json:"last_sender_character_id"`
}

type ActionSubmissionResponse struct {
	ID             int32   `json:"id"`
	GameID         int32   `json:"game_id"`
	UserID         int32   `json:"user_id"`
	PhaseID        int32   `json:"phase_id"`
	CharacterID    *int32  `json:"character_id"`
	Content        string  `json:"content"`
	SubmittedAt    *string `json:"submitted_at"`
	UpdatedAt      *string `json:"updated_at"`
	Username       string  `json:"username"`
	CharacterName  *string `json:"character_name"`
	PhaseType      string  `json:"phase_type"`
	PhaseNumber    int32   `json:"phase_number"`
	PhaseTitle     string  `json:"phase_title"`
	ActionResultID *int32  `json:"action_result_id"`
	Status         string  `json:"status"`
}

type AudienceMessageResponse struct {
	ID                  int32   `json:"id"`
	ConversationID      int32   `json:"conversation_id"`
	SenderUserID        *int32  `json:"sender_user_id"`
	SenderCharacterID   *int32  `json:"sender_character_id"`
	Content             string  `json:"content"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
	IsDeleted           bool    `json:"is_deleted"`
	SenderUsername      string  `json:"sender_username"`
	SenderCharacterName *string `json:"sender_character_name"`
}

func (a *UpdateAutoAcceptAudienceRequest) Bind(r *http.Request) error {
	return nil
}

// ============================================================================
// Handlers
// ============================================================================

// ListAudienceMembers lists all audience members in a game
// GET /api/v1/games/:id/audience
func (h *Handler) ListAudienceMembers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_list_audience_members")()

	gameID := ctx.Value("gameID").(int32)

	gameService := h.GameService

	// Get audience members
	members, err := gameService.ListAudienceMembers(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to list audience members", "error", err, "game_id", gameID)
		return
	}

	// Convert to response format
	response := &ListAudienceMembersResponse{
		AudienceMembers: make([]AudienceMemberResponse, len(members)),
	}

	for i, member := range members {
		response.AudienceMembers[i] = AudienceMemberResponse{
			ID:       member.ID,
			GameID:   member.GameID,
			UserID:   member.UserID,
			Username: member.Username,
			Role:     member.Role,
			Status:   member.Status.String,
			JoinedAt: member.JoinedAt.Time,
		}
	}

	render.JSON(w, r, response)
}

// UpdateAutoAcceptAudience updates the auto-accept audience setting for a game
// PUT /api/v1/games/:id/settings/auto-accept-audience
func (h *Handler) UpdateAutoAcceptAudience(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_auto_accept_audience")()

	game := ctx.Value("game").(*db.Game)
	gameID := ctx.Value("gameID").(int32)

	data := &UpdateAutoAcceptAudienceRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid update auto-accept audience request", "error", err, "game_id", gameID)
		return
	}

	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(ctx)

	// Check if user is GM
	gameService := h.GameService
	if game.GmUserID != int32(authUser.ID) {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can update this setting"), "Update auto accept audience forbidden")
		return
	}

	// Update the setting
	err := gameService.UpdateGameAutoAcceptAudience(ctx, int32(gameID), data.AutoAcceptAudience)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to update auto-accept audience setting", "error", err, "game_id", gameID)
		return
	}

	render.JSON(w, r, map[string]string{
		"message": "Auto-accept audience setting updated",
	})
}

// ListAudienceNPCs lists all audience-controlled NPCs in a game
// GET /api/v1/games/:id/characters/audience-npcs
func (h *Handler) ListAudienceNPCs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_list_audience_npcs")()

	gameID := ctx.Value("gameID").(int32)

	// Get audience NPCs
	npcs, err := h.CharacterService.ListAudienceNPCs(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to list audience NPCs", "error", err, "game_id", gameID)
		return
	}

	render.JSON(w, r, map[string]interface{}{
		"npcs": npcs,
	})
}

// ListAllPrivateConversations lists all private conversations for GM/audience
// GET /api/v1/games/:id/private-messages/all
func (h *Handler) ListAllPrivateConversations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_list_all_private_conversations")()

	gameID := ctx.Value("gameID").(int32)

	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	// Check if user can view game (includes public archive access for completed games)
	gameService := h.GameService
	canView, err := gameService.CanUserViewGame(ctx, int32(gameID), int32(authUser.ID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check game view access", "error", err, "game_id", gameID, "user_id", authUser.ID)
		return
	}

	if !canView {
		h.renderError(ctx, w, r, core.ErrForbidden("you do not have permission to view this game's content"), "List all private conversations forbidden")
		return
	}

	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	limit := int32(20) // default
	if limitStr != "" {
		limitParsed, err := strconv.ParseInt(limitStr, 10, 32)
		if err == nil && limitParsed > 0 {
			limit = int32(limitParsed)
		}
	}

	offsetStr := r.URL.Query().Get("offset")
	offset := int32(0) // default
	if offsetStr != "" {
		offsetParsed, err := strconv.ParseInt(offsetStr, 10, 32)
		if err == nil && offsetParsed >= 0 {
			offset = int32(offsetParsed)
		}
	}

	// Parse participant_names filter (comma-separated)
	var participantNames []string
	participantNamesStr := r.URL.Query().Get("participant_names")
	if participantNamesStr != "" {
		// Split by comma and trim spaces
		for _, name := range r.URL.Query()["participant_names"] {
			if name != "" {
				participantNames = append(participantNames, name)
			}
		}
	}

	// Get all private conversations with filters
	messageService := h.MessageService
	conversations, err := messageService.ListAllPrivateConversations(ctx, core.ListAllPrivateConversationsParams{
		GameID:           int32(gameID),
		ParticipantNames: participantNames,
		Limit:            limit,
		Offset:           offset,
	})
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to list private conversations", "error", err, "game_id", gameID)
		return
	}

	// Get total count for pagination display
	total, err := messageService.CountAllPrivateConversations(ctx, int32(gameID), participantNames)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to count private conversations", "error", err, "game_id", gameID)
		return
	}

	// Map to response structs with clean Go types (not pgtype wrappers)
	responses := make([]PrivateConversationResponse, len(conversations))
	for i, c := range conversations {
		var subject *string
		if c.Subject.Valid {
			subject = &c.Subject.String
		}
		var lastContent *string
		if c.LastMessageContent.Valid {
			lastContent = &c.LastMessageContent.String
		}
		var lastSenderName *string
		if c.LastSenderName.Valid {
			lastSenderName = &c.LastSenderName.String
		}
		var lastSenderUsername *string
		if c.LastSenderUsername.Valid {
			lastSenderUsername = &c.LastSenderUsername.String
		}
		var lastSenderCharID *int32
		if c.LastSenderCharacterID.Valid {
			lastSenderCharID = &c.LastSenderCharacterID.Int32
		}
		responses[i] = PrivateConversationResponse{
			ConversationID:          c.ConversationID,
			Subject:                 subject,
			ConversationType:        c.ConversationType,
			CreatedAt:               c.CreatedAt.Time.Format(time.RFC3339),
			MessageCount:            c.MessageCount,
			LastMessageAt:           c.LastMessageAt,
			ParticipantNames:        c.ParticipantNames,
			ParticipantUsernames:    c.ParticipantUsernames,
			ParticipantCharacterIDs: c.ParticipantCharacterIds,
			LastMessageContent:      lastContent,
			LastSenderName:          lastSenderName,
			LastSenderUsername:      lastSenderUsername,
			LastSenderCharacterID:   lastSenderCharID,
		}
	}

	render.JSON(w, r, map[string]interface{}{
		"conversations": responses,
		"total":         total,
	})
}

// GetAudienceConversationMessages gets messages for a specific conversation (GM/audience only)
// GET /api/v1/games/:id/private-messages/conversations/:conversationId
func (h *Handler) GetAudienceConversationMessages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_audience_conversation_messages")()

	gameID := ctx.Value("gameID").(int32)

	conversationIDStr := chi.URLParam(r, "conversationId")
	conversationID, err := strconv.ParseInt(conversationIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid conversation ID")), "Invalid get audience conversation messages request")
		return
	}

	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	// Check if user can view game (includes public archive access for completed games)
	gameService := h.GameService
	canView, err := gameService.CanUserViewGame(ctx, int32(gameID), int32(authUser.ID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check game view access", "error", err, "game_id", gameID, "user_id", authUser.ID)
		return
	}

	if !canView {
		h.renderError(ctx, w, r, core.ErrForbidden("you do not have permission to view this game's content"), "Get audience conversation messages forbidden")
		return
	}

	// Get conversation messages
	messageService := h.MessageService
	messages, err := messageService.GetAudienceConversationMessages(ctx, int32(conversationID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get conversation messages", "error", err, "conversation_id", conversationID)
		return
	}

	// Map to response structs with clean Go types (not pgtype wrappers)
	responses := make([]AudienceMessageResponse, len(messages))
	for i, m := range messages {
		senderUserID := m.SenderUserID
		var senderCharID *int32
		if m.SenderCharacterID.Valid {
			senderCharID = &m.SenderCharacterID.Int32
		}
		var senderCharName *string
		if m.SenderCharacterName.Valid {
			senderCharName = &m.SenderCharacterName.String
		}
		responses[i] = AudienceMessageResponse{
			ID:                  m.ID,
			ConversationID:      m.ConversationID,
			SenderUserID:        &senderUserID,
			SenderCharacterID:   senderCharID,
			Content:             m.Content,
			CreatedAt:           m.CreatedAt.Time.Format(time.RFC3339),
			UpdatedAt:           m.UpdatedAt.Time.Format(time.RFC3339),
			IsDeleted:           m.IsDeleted.Bool,
			SenderUsername:      m.SenderUsername,
			SenderCharacterName: senderCharName,
		}
	}

	render.JSON(w, r, map[string]interface{}{
		"messages": responses,
	})
}

// GetConversationParticipants returns participant names for the filter UI.
// When no ?selected[] params are given, returns all names that appear in any conversation.
// When selected names are given, returns only names that share a conversation with ALL of them.
// GET /api/v1/games/:id/private-messages/participants
func (h *Handler) GetConversationParticipants(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_conversation_participants")()

	gameID := ctx.Value("gameID").(int32)

	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	gameService := h.GameService
	canView, err := gameService.CanUserViewGame(ctx, int32(gameID), int32(authUser.ID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check game view access", "error", err, "game_id", gameID, "user_id", authUser.ID)
		return
	}
	if !canView {
		h.renderError(ctx, w, r, core.ErrForbidden("you do not have permission to view this game's content"), "Get conversation participants forbidden")
		return
	}

	selectedNames := r.URL.Query()["selected[]"]

	messageService := h.MessageService
	names, err := messageService.GetConversationParticipantNames(ctx, int32(gameID), selectedNames)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get conversation participants", "error", err, "game_id", gameID)
		return
	}

	render.JSON(w, r, map[string][]string{"participants": names})
}

// ListAllActionSubmissions lists all action submissions for GM/audience
// GET /api/v1/games/:id/action-submissions/all
func (h *Handler) ListAllActionSubmissions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_list_all_action_submissions")()

	gameID := ctx.Value("gameID").(int32)

	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	// Check if user can view game (includes public archive access for completed games)
	gameService := h.GameService
	canView, err := gameService.CanUserViewGame(ctx, int32(gameID), int32(authUser.ID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check game view access", "error", err, "game_id", gameID, "user_id", authUser.ID)
		return
	}

	if !canView {
		h.renderError(ctx, w, r, core.ErrForbidden("you do not have permission to view this game's content"), "List all action submissions forbidden")
		return
	}

	// Parse query parameters
	phaseIDStr := r.URL.Query().Get("phase_id")
	phaseID := int32(0) // 0 means all phases
	if phaseIDStr != "" {
		phaseIDParsed, err := strconv.ParseInt(phaseIDStr, 10, 32)
		if err == nil {
			phaseID = int32(phaseIDParsed)
		}
	}

	limitStr := r.URL.Query().Get("limit")
	limit := int32(10) // default
	if limitStr != "" {
		limitParsed, err := strconv.ParseInt(limitStr, 10, 32)
		if err == nil && limitParsed > 0 {
			limit = int32(limitParsed)
		}
	}

	offsetStr := r.URL.Query().Get("offset")
	offset := int32(0) // default
	if offsetStr != "" {
		offsetParsed, err := strconv.ParseInt(offsetStr, 10, 32)
		if err == nil && offsetParsed >= 0 {
			offset = int32(offsetParsed)
		}
	}

	// Get action submissions
	submissions, err := h.ActionSubmissionService.ListAllActionSubmissions(ctx, int32(gameID), phaseID, limit, offset)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to list action submissions", "error", err, "game_id", gameID)
		return
	}

	// Get total count
	total, err := h.ActionSubmissionService.CountAllActionSubmissions(ctx, int32(gameID), phaseID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to count action submissions", "error", err, "game_id", gameID)
		return
	}

	// Map to response structs with clean Go types (not pgtype wrappers)
	responses := make([]ActionSubmissionResponse, len(submissions))
	for i, s := range submissions {
		var charID *int32
		if s.CharacterID.Valid {
			charID = &s.CharacterID.Int32
		}
		var charName *string
		if s.CharacterName.Valid {
			charName = &s.CharacterName.String
		}
		var submittedAt *string
		if s.SubmittedAt.Valid {
			t := s.SubmittedAt.Time.Format(time.RFC3339)
			submittedAt = &t
		}
		var updatedAt *string
		if s.UpdatedAt.Valid {
			t := s.UpdatedAt.Time.Format(time.RFC3339)
			updatedAt = &t
		}
		var actionResultID *int32
		if s.ActionResultID.Valid {
			actionResultID = &s.ActionResultID.Int32
		}
		responses[i] = ActionSubmissionResponse{
			ID:             s.ID,
			GameID:         s.GameID,
			UserID:         s.UserID,
			PhaseID:        s.PhaseID,
			CharacterID:    charID,
			Content:        s.Content,
			SubmittedAt:    submittedAt,
			UpdatedAt:      updatedAt,
			Username:       s.Username,
			CharacterName:  charName,
			PhaseType:      s.PhaseType,
			PhaseNumber:    s.PhaseNumber,
			PhaseTitle:     s.PhaseTitle,
			ActionResultID: actionResultID,
			Status:         s.Status,
		}
	}

	render.JSON(w, r, map[string]interface{}{
		"action_submissions": responses,
		"total":              total,
	})
}
