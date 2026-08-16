package polls

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"actionphase/pkg/core"
	db "actionphase/pkg/db/models"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/jackc/pgx/v5/pgtype"
)

// anonymousVoterName is the display name used for audience votes, which are never
// attributed to a specific user or character.
const anonymousVoterName = "Anonymous"

// Request and Response Types

// CreatePollRequest is the API request for creating a poll
type CreatePollRequest struct {
	Question               string    `json:"question"`
	Description            *string   `json:"description,omitempty"`
	Deadline               time.Time `json:"deadline"`
	PhaseID                *int32    `json:"phase_id,omitempty"`
	ShowIndividualVotes    bool      `json:"show_individual_votes"`
	AllowOtherOption       bool      `json:"allow_other_option"`
	HideResultsFromPlayers bool      `json:"hide_results_from_players"`
	AllowAudienceVoting    bool      `json:"allow_audience_voting"`
	// ShowRunningTotalsToPlayers lets players see results before the deadline.
	ShowRunningTotalsToPlayers bool                `json:"show_running_totals_to_players"`
	Options                    []PollOptionRequest `json:"options"`
}

// PollOptionRequest represents a poll option in the API request
type PollOptionRequest struct {
	Text         string `json:"text"`
	DisplayOrder int32  `json:"display_order"`
}

// Bind validates the CreatePollRequest
func (req *CreatePollRequest) Bind(r *http.Request) error {
	if req.Question == "" {
		return fmt.Errorf("question is required")
	}
	if req.Deadline.Before(time.Now()) {
		return fmt.Errorf("deadline must be in the future")
	}
	if len(req.Options) < 2 {
		return fmt.Errorf("at least 2 options are required")
	}
	if req.HideResultsFromPlayers && req.ShowIndividualVotes {
		return fmt.Errorf("hide_results_from_players cannot be combined with show_individual_votes")
	}
	if req.HideResultsFromPlayers && req.ShowRunningTotalsToPlayers {
		return fmt.Errorf("hide_results_from_players cannot be combined with show_running_totals_to_players")
	}
	return nil
}

// UpdatePollRequest is the API request for updating a poll
type UpdatePollRequest struct {
	Question                   string    `json:"question"`
	Description                *string   `json:"description,omitempty"`
	Deadline                   time.Time `json:"deadline"`
	ShowIndividualVotes        bool      `json:"show_individual_votes"`
	AllowOtherOption           bool      `json:"allow_other_option"`
	HideResultsFromPlayers     bool      `json:"hide_results_from_players"`
	AllowAudienceVoting        bool      `json:"allow_audience_voting"`
	ShowRunningTotalsToPlayers bool      `json:"show_running_totals_to_players"`
}

// Bind validates the UpdatePollRequest
func (req *UpdatePollRequest) Bind(r *http.Request) error {
	if req.Question == "" {
		return fmt.Errorf("question is required")
	}
	if req.Deadline.Before(time.Now()) {
		return fmt.Errorf("deadline must be in the future")
	}
	if req.HideResultsFromPlayers && req.ShowIndividualVotes {
		return fmt.Errorf("hide_results_from_players cannot be combined with show_individual_votes")
	}
	if req.HideResultsFromPlayers && req.ShowRunningTotalsToPlayers {
		return fmt.Errorf("hide_results_from_players cannot be combined with show_running_totals_to_players")
	}
	return nil
}

// SubmitVoteRequest is the API request for submitting a vote
type SubmitVoteRequest struct {
	SelectedOptionID *int32  `json:"selected_option_id,omitempty"`
	OtherResponse    *string `json:"other_response,omitempty"`
}

// Bind validates the SubmitVoteRequest
func (req *SubmitVoteRequest) Bind(r *http.Request) error {
	if req.SelectedOptionID == nil && req.OtherResponse == nil {
		return fmt.Errorf("either selected_option_id or other_response is required")
	}
	return nil
}

// PollResponse is the API response for a poll with options
// Returns a flat structure with poll fields at top level and options array
type PollResponse struct {
	// Embed all poll fields at top level
	db.CommonRoomPoll

	// Additional response fields
	Options               []db.PollOption `json:"options"`
	HasVoted              bool            `json:"has_voted,omitempty"`
	UserVoteOptionID      *int32          `json:"user_vote_option_id,omitempty"`
	UserVoteOtherResponse *string         `json:"user_vote_other_response,omitempty"`
}

// PollResultsResponse is the API response for poll results
type PollResultsResponse struct {
	Poll                db.CommonRoomPoll `json:"poll"`
	OptionResults       []OptionResult    `json:"option_results"`
	OtherResponses      []OtherResponse   `json:"other_responses"` // Always include even if empty array
	TotalVotes          int32             `json:"total_votes"`
	ShowIndividualVotes bool              `json:"show_individual_votes"`
}

// OptionResult represents voting results for one option
// Returns flattened structure matching frontend expectations
type OptionResult struct {
	PollOptionID *int32      `json:"poll_option_id,omitempty"`
	OptionText   *string     `json:"option_text,omitempty"`
	VoteCount    int32       `json:"vote_count"`
	Voters       []VoterInfo `json:"voters,omitempty"`
}

// VoterInfo represents information about a voter (only shown if show_individual_votes = true)
type VoterInfo struct {
	UserID        int32  `json:"user_id"`
	CharacterName string `json:"character_name"`
	IsAnonymous   bool   `json:"is_anonymous,omitempty"`
}

// OtherResponse represents a free-text "other" response
type OtherResponse struct {
	VoteID        int32  `json:"vote_id"`
	OtherText     string `json:"other_text"`
	CharacterName string `json:"character_name"`
	IsAnonymous   bool   `json:"is_anonymous,omitempty"`
}

// Helper Functions

// strPtr returns a pointer to a string
func strPtr(s string) *string {
	return &s
}

// verifyUserIsGM checks if a user is the GM or Co-GM of a game
// Uses the unified permission check for GM, Co-GM, and admin mode support
func (h *Handler) verifyUserIsGM(ctx context.Context, gameID int32, userID int32) error {
	game, err := h.GameService.GetGame(ctx, gameID)
	if err != nil {
		return fmt.Errorf("failed to get game: %w", err)
	}

	// Get user to check admin status
	user, err := h.UserService.GetUserByID(int(userID))
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Check if user is GM, Co-GM, or admin with admin mode enabled
	if !core.IsUserGameMasterCtx(ctx, userID, user.IsAdmin, *game, h.App.Pool) {
		return fmt.Errorf("only GM or Co-GM can perform this action")
	}

	return nil
}

// verifyUserInGame checks if a user is a participant in a game (GM, Co-GM, or player)
func (h *Handler) verifyUserInGame(ctx context.Context, gameID int32, userID int32) error {
	game, err := h.GameService.GetGame(ctx, gameID)
	if err != nil {
		return fmt.Errorf("failed to get game: %w", err)
	}

	// Check if user is GM or Co-GM
	if game.GmUserID == userID || core.IsUserCoGM(ctx, h.App.Pool, gameID, userID) {
		return nil
	}

	// Check if user is a participant or audience member in the game
	queries := db.New(h.App.Pool)
	isParticipant, err := queries.IsUserInGame(ctx, db.IsUserInGameParams{
		GameID: gameID,
		UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("failed to check participant status: %w", err)
	}
	if isParticipant {
		return nil
	}

	// Check if user has any characters in the game
	characters, err := h.CharacterService.GetCharactersByGame(ctx, gameID)
	if err != nil {
		return fmt.Errorf("failed to list characters: %w", err)
	}

	for _, char := range characters {
		if char.UserID.Valid && char.UserID.Int32 == userID {
			return nil
		}
	}

	return fmt.Errorf("user is not a participant in this game")
}

// pollViewAccess holds the result of checking whether a user can view polls for a game.
type pollViewAccess struct {
	allowed               bool
	canSeeIndividualVotes bool // true for GM, Co-GM, audience, or any user viewing a completed game
	// isPrivileged is true for GM, Co-GM and audience — the roles that may see
	// results of a poll flagged hide_results_from_players — and for any viewer of
	// a completed game, which is a public archive granting audience-level access.
	isPrivileged bool
}

// checkPollViewAccess determines what visibility level an authenticated user gets for
// a game's polls. All authenticated users may read polls; the flags control whether
// they see individual vote attribution and results the GM hid from players.
//
// Individual votes and hidden-poll results visible to:
//   - GM / Co-GM: always
//   - Audience: always (spectator role)
//   - Everyone else: only after the game is completed
func (h *Handler) checkPollViewAccess(ctx context.Context, gameID int32, userID int32) (pollViewAccess, error) {
	game, err := h.GameService.GetGame(ctx, gameID)
	if err != nil {
		return pollViewAccess{}, fmt.Errorf("failed to get game: %w", err)
	}

	// GM, Co-GM and audience always see full results, and remain privileged even
	// after the game completes.
	if game.GmUserID == userID ||
		core.IsUserCoGM(ctx, h.App.Pool, gameID, userID) ||
		core.IsUserAudience(ctx, h.App.Pool, gameID, userID) {
		return pollViewAccess{allowed: true, canSeeIndividualVotes: true, isPrivileged: true}, nil
	}

	// Completed games are a public archive: every authenticated viewer gets
	// audience-level access to the whole game, hidden polls included. Anonymity
	// (CanSeeUsernamesInAnonymousGame) lifts on completion for the same reason.
	// Cancelled games are NOT public and keep the play-time rules below.
	if game.State.String == core.GameStateCompleted {
		return pollViewAccess{allowed: true, canSeeIndividualVotes: true, isPrivileged: true}, nil
	}

	// Everyone else (players and non-participants) may see polls exist but not individual votes
	return pollViewAccess{allowed: true, canSeeIndividualVotes: false}, nil
}

// API Handler Methods

// CreatePoll handles POST /games/{gameId}/polls
func (h *Handler) CreatePoll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_create_poll")()

	defer h.App.ObsLogger.LogOperation(ctx, "CreatePoll")()

	// Extract game ID from URL
	gameID := ctx.Value("gameID").(int32)

	// Parse request body
	data := &CreatePollRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Failed to bind create poll request", "error", err)
		return
	}

	// Authenticate user
	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	// Verify user is GM
	if err := h.verifyUserIsGM(ctx, int32(gameID), userID); err != nil {
		h.renderError(ctx, w, r, core.ErrForbidden(err.Error()), "User is not GM of the game", "error", err)
		return
	}

	// Convert API request to service request
	options := make([]core.PollOptionInput, len(data.Options))
	for i, opt := range data.Options {
		options[i] = core.PollOptionInput{
			Text:         opt.Text,
			DisplayOrder: opt.DisplayOrder,
		}
	}

	serviceReq := core.CreatePollRequest{
		GameID:                     int32(gameID),
		PhaseID:                    data.PhaseID,
		CreatedByUserID:            userID,
		Question:                   data.Question,
		Description:                data.Description,
		Deadline:                   data.Deadline,
		ShowIndividualVotes:        data.ShowIndividualVotes,
		AllowOtherOption:           data.AllowOtherOption,
		HideResultsFromPlayers:     data.HideResultsFromPlayers,
		AllowAudienceVoting:        data.AllowAudienceVoting,
		ShowRunningTotalsToPlayers: data.ShowRunningTotalsToPlayers,
		Options:                    options,
	}

	// Create poll
	pollWithOptions, err := h.PollService.CreatePollWithOptions(ctx, serviceReq)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to create poll", "error", err)
		return
	}

	// Send notification to all game participants
	participants, err := h.GameService.GetGameParticipants(ctx, int32(gameID))
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to get game participants for notification")
		// Don't fail the request if notification fails - just log
	} else {
		// Build list of user IDs (exclude the creator)
		userIDs := make([]int32, 0)
		for _, participant := range participants {
			if participant.UserID != userID {
				userIDs = append(userIDs, participant.UserID)
			}
		}

		if len(userIDs) > 0 {
			gameIDInt32 := int32(gameID)
			linkURL := fmt.Sprintf("/games/%d?tab=polls", gameID)

			notifReq := &core.CreateNotificationRequest{
				GameID:      &gameIDInt32,
				Type:        "poll_created",
				Title:       fmt.Sprintf("New Poll: %s", pollWithOptions.Poll.Question),
				RelatedType: strPtr("poll"),
				RelatedID:   &pollWithOptions.Poll.ID,
				LinkURL:     &linkURL,
			}

			err = h.NotificationService.CreateBulkNotifications(ctx, userIDs, notifReq)
			if err != nil {
				h.App.ObsLogger.LogError(ctx, err, "Failed to create bulk notifications")
				// Don't fail the request if notification fails
			}
		}
	}

	// Flatten the response structure
	response := PollResponse{
		CommonRoomPoll: pollWithOptions.Poll,
		Options:        pollWithOptions.Options,
	}

	render.JSON(w, r, response)
}

// PollListItem represents a poll in the list response with vote status
type PollListItem struct {
	db.CommonRoomPoll
	UserHasVoted bool `json:"user_has_voted"`
}

// ListGamePolls handles GET /games/{gameId}/polls
func (h *Handler) ListGamePolls(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_list_game_polls")()

	defer h.App.ObsLogger.LogOperation(ctx, "ListGamePolls")()

	// Extract game ID from URL
	gameID := ctx.Value("gameID").(int32)

	// Authenticate user
	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	// Check access (completed games are public)
	access, err := h.checkPollViewAccess(ctx, int32(gameID), userID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check poll view access", "error", err)
		return
	}
	if !access.allowed {
		h.renderError(ctx, w, r, core.ErrForbidden("user is not a participant in this game"), "List game polls forbidden")
		return
	}

	// Check for includeExpired query parameter
	includeExpired := r.URL.Query().Get("include_expired") == "true"

	// List polls
	pollService := h.PollService
	polls, err := pollService.ListPollsByGame(ctx, int32(gameID), includeExpired)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to list polls", "error", err)
		return
	}

	// Add user_has_voted field to each poll
	pollListItems := make([]PollListItem, len(polls))
	for i, poll := range polls {
		hasVoted, err := pollService.HasUserVoted(ctx, poll.ID, userID)
		if err != nil {
			h.App.ObsLogger.LogError(ctx, err, "Failed to check if user voted", "poll_id", poll.ID)
			hasVoted = false
		}

		pollListItems[i] = PollListItem{
			CommonRoomPoll: poll,
			UserHasVoted:   hasVoted,
		}
	}

	render.JSON(w, r, pollListItems)
}

// GetPoll handles GET /polls/{pollId}
func (h *Handler) GetPoll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_poll")()

	defer h.App.ObsLogger.LogOperation(ctx, "GetPoll")()

	// Extract poll ID from URL
	pollIDStr := chi.URLParam(r, "pollId")
	pollID, err := strconv.ParseInt(pollIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid poll ID")), "Invalid poll ID", "error", err)
		return
	}

	// Authenticate user
	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	// Get poll with options
	pollService := h.PollService
	pollWithOptions, err := pollService.GetPollWithOptions(ctx, int32(pollID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("poll"), "Failed to get poll", "error", err)
		return
	}

	// Check access (completed games are public)
	if access, err := h.checkPollViewAccess(ctx, pollWithOptions.Poll.GameID, userID); err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check poll view access", "error", err)
		return
	} else if !access.allowed {
		h.renderError(ctx, w, r, core.ErrForbidden("user is not a participant in this game"), "Get poll forbidden")
		return
	}

	hasVoted, err := pollService.HasUserVoted(ctx, int32(pollID), userID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check if user voted", "error", err)
		return
	}

	var userVoteOptionID *int32
	var userVoteOtherResponse *string
	if hasVoted {
		vote, err := pollService.GetVote(ctx, int32(pollID), userID)
		if err == nil && vote != nil {
			if vote.SelectedOptionID.Valid {
				optID := vote.SelectedOptionID.Int32
				userVoteOptionID = &optID
			}
			if vote.OtherResponse.Valid {
				otherResp := vote.OtherResponse.String
				userVoteOtherResponse = &otherResp
			}
		}
	}

	response := PollResponse{
		CommonRoomPoll:        pollWithOptions.Poll,
		Options:               pollWithOptions.Options,
		HasVoted:              hasVoted,
		UserVoteOptionID:      userVoteOptionID,
		UserVoteOtherResponse: userVoteOtherResponse,
	}

	render.JSON(w, r, response)
}

// GetPollResults handles GET /polls/{pollId}/results
func (h *Handler) GetPollResults(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_poll_results")()

	// Extract poll ID from URL
	pollIDStr := chi.URLParam(r, "pollId")
	pollID, err := strconv.ParseInt(pollIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid poll ID")), "Invalid poll ID", "error", err)
		return
	}

	// Authenticate user
	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	// Verify user is in game first (need game info to determine GM status)
	pollService := h.PollService

	// Get poll to find game ID
	poll, err := pollService.GetPoll(ctx, int32(pollID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("poll"), "Failed to get poll", "error", err)
		return
	}

	// Check access (completed games are public) — also gives us canSeeIndividualVotes
	access, err := h.checkPollViewAccess(ctx, poll.GameID, userID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check poll view access", "error", err)
		return
	}
	if !access.allowed {
		h.renderError(ctx, w, r, core.ErrForbidden("user is not a participant in this game"), "Get poll results forbidden")
		return
	}

	// Authorize before reading results: both checks answer from the poll row we
	// already have, so an unauthorized request never pays for the results queries.

	// Hidden-results polls do not disclose results to players while the game runs —
	// not even after the deadline. GM, co-GM and audience are exempt, as is every
	// viewer once the game completes and becomes a public archive.
	if poll.HideResultsFromPlayers && !access.isPrivileged {
		h.renderError(ctx, w, r, core.ErrForbidden("poll results are hidden by the GM"), "Cannot view results - results hidden from players")
		return
	}

	// Regular players can only see results after the poll expires, unless the GM
	// opted into running totals; privileged users can always view.
	if !access.canSeeIndividualVotes && !poll.ShowRunningTotalsToPlayers && !poll.Deadline.Time.Before(time.Now()) {
		h.renderError(ctx, w, r, core.ErrForbidden("poll results not available until voting closes"), "Cannot view results - poll still active")
		return
	}

	// Get poll results; privileged users (GM, co-GM, audience, completed-game viewers) see individual votes
	results, err := pollService.GetPollResults(ctx, int32(pollID), access.canSeeIndividualVotes)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("poll"), "Failed to get poll results", "error", err)
		return
	}

	// Convert core.PollResults to API response with flattened structure
	optionResults := make([]OptionResult, len(results.OptionResults))
	for i, optRes := range results.OptionResults {
		voters := make([]VoterInfo, len(optRes.Voters))
		for j, voter := range optRes.Voters {
			if voter.IsAnonymous {
				voters[j] = VoterInfo{CharacterName: anonymousVoterName, IsAnonymous: true}
				continue
			}

			characterName := ""
			if voter.CharacterName != nil {
				characterName = *voter.CharacterName
			}
			voters[j] = VoterInfo{
				UserID:        voter.UserID,
				CharacterName: characterName,
			}
		}

		// Flatten option fields to top level
		// For "Other" responses, these will be zero values (0 and "")
		var pollOptionID *int32
		var optionText *string
		if optRes.Option.ID != 0 {
			pollOptionID = &optRes.Option.ID
			optionText = &optRes.Option.OptionText
		}

		optionResults[i] = OptionResult{
			PollOptionID: pollOptionID,
			OptionText:   optionText,
			VoteCount:    optRes.VoteCount,
			Voters:       voters,
		}
	}

	otherResponses := make([]OtherResponse, len(results.OtherResponses))
	for i, other := range results.OtherResponses {
		if other.IsAnonymous {
			otherResponses[i] = OtherResponse{
				VoteID:        other.VoteID,
				OtherText:     other.OtherText,
				CharacterName: anonymousVoterName,
				IsAnonymous:   true,
			}
			continue
		}

		characterName := ""
		if other.CharacterName != nil {
			characterName = *other.CharacterName
		}
		otherResponses[i] = OtherResponse{
			VoteID:        other.VoteID,
			OtherText:     other.OtherText,
			CharacterName: characterName,
		}
	}

	response := PollResultsResponse{
		Poll:                results.Poll,
		OptionResults:       optionResults,
		OtherResponses:      otherResponses,
		TotalVotes:          results.TotalVotes,
		ShowIndividualVotes: results.ShowIndividualVotes,
	}

	render.JSON(w, r, response)
}

// SubmitVote handles POST /polls/{pollId}/vote
func (h *Handler) SubmitVote(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_submit_vote")()

	defer h.App.ObsLogger.LogOperation(ctx, "SubmitVote")()

	// Extract poll ID from URL
	pollIDStr := chi.URLParam(r, "pollId")
	pollID, err := strconv.ParseInt(pollIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid poll ID")), "Invalid poll ID", "error", err)
		return
	}

	// Authenticate user
	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	// Parse request body
	data := &SubmitVoteRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Failed to bind submit vote request", "error", err)
		return
	}

	// Get poll to verify it exists and check game membership
	pollService := h.PollService
	poll, err := pollService.GetPoll(ctx, int32(pollID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("poll"), "Failed to get poll", "error", err)
		return
	}

	// Verify user is in game
	if err := h.verifyUserInGame(ctx, poll.GameID, userID); err != nil {
		h.renderError(ctx, w, r, core.ErrForbidden(err.Error()), "User is not in the game", "error", err)
		return
	}

	// GMs and co-GMs cannot vote on polls
	game, err := h.GameService.GetGame(ctx, poll.GameID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game", "error", err)
		return
	}

	isGM := game.GmUserID == userID
	isCoGM := core.IsUserCoGM(ctx, h.App.Pool, poll.GameID, userID)
	isAudience := core.IsUserAudience(ctx, h.App.Pool, poll.GameID, userID)

	if isGM || isCoGM {
		h.renderError(ctx, w, r, core.ErrForbidden("GMs and co-GMs cannot vote on polls"), "GMs and co-GMs cannot vote on polls")
		return
	}

	if isAudience {
		// Audience may vote only when the GM enabled it for this poll.
		if !poll.AllowAudienceVoting {
			h.renderError(ctx, w, r, core.ErrForbidden("Audience members cannot vote on polls"), "Audience members cannot vote on polls")
			return
		}
	} else {
		// Players must have an approved character to vote. Audience members are
		// exempt: they need not have a character, and their votes are recorded
		// anonymously.
		queries := db.New(h.App.Pool)
		hasCharacter, err := queries.HasApprovedCharacterInGame(ctx, db.HasApprovedCharacterInGameParams{
			GameID: poll.GameID,
			UserID: pgtype.Int4{Int32: userID, Valid: true},
		})
		if err != nil {
			h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check character status", "error", err)
			return
		}
		if !hasCharacter {
			h.renderError(ctx, w, r, core.ErrForbidden("you must have an approved character to vote"), "Player does not have an approved character in this game")
			return
		}
	}

	// Check if deadline has passed
	if poll.Deadline.Time.Before(time.Now()) {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("voting closed - poll deadline has passed")), "Cannot vote - poll deadline has passed")
		return
	}

	// Submit vote
	serviceReq := core.SubmitVoteRequest{
		PollID:           int32(pollID),
		UserID:           userID,
		SelectedOptionID: data.SelectedOptionID,
		OtherResponse:    data.OtherResponse,
	}

	vote, err := pollService.SubmitVote(ctx, serviceReq)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to submit vote", "error", err)
		return
	}

	render.JSON(w, r, vote)
}

// UpdatePoll handles PUT /polls/{pollId}
func (h *Handler) UpdatePoll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_poll")()

	defer h.App.ObsLogger.LogOperation(ctx, "UpdatePoll")()

	// Extract poll ID from URL
	pollIDStr := chi.URLParam(r, "pollId")
	pollID, err := strconv.ParseInt(pollIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid poll ID")), "Invalid poll ID", "error", err)
		return
	}

	// Authenticate user
	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	// Parse request body
	data := &UpdatePollRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Failed to bind update poll request", "error", err)
		return
	}

	// Get poll to verify it exists and get game ID
	pollService := h.PollService
	poll, err := pollService.GetPoll(ctx, int32(pollID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("poll"), "Failed to get poll", "error", err)
		return
	}

	// Verify user is GM
	if err := h.verifyUserIsGM(ctx, poll.GameID, userID); err != nil {
		h.renderError(ctx, w, r, core.ErrForbidden(err.Error()), "User is not GM of the game", "error", err)
		return
	}

	// Update poll
	serviceReq := core.UpdatePollRequest{
		Question:                   data.Question,
		Description:                data.Description,
		Deadline:                   data.Deadline,
		ShowIndividualVotes:        data.ShowIndividualVotes,
		AllowOtherOption:           data.AllowOtherOption,
		HideResultsFromPlayers:     data.HideResultsFromPlayers,
		AllowAudienceVoting:        data.AllowAudienceVoting,
		ShowRunningTotalsToPlayers: data.ShowRunningTotalsToPlayers,
	}

	updatedPoll, err := pollService.UpdatePoll(ctx, int32(pollID), serviceReq)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to update poll", "error", err)
		return
	}

	render.JSON(w, r, updatedPoll)
}

// DeletePoll handles DELETE /polls/{pollId}
func (h *Handler) DeletePoll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_poll")()

	defer h.App.ObsLogger.LogOperation(ctx, "DeletePoll")()

	// Extract poll ID from URL
	pollIDStr := chi.URLParam(r, "pollId")
	pollID, err := strconv.ParseInt(pollIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid poll ID")), "Invalid poll ID", "error", err)
		return
	}

	// Authenticate user
	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	// Get poll to verify it exists and get game ID
	pollService := h.PollService
	poll, err := pollService.GetPoll(ctx, int32(pollID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrNotFound("poll"), "Failed to get poll", "error", err)
		return
	}

	// Verify user is GM
	if err := h.verifyUserIsGM(ctx, poll.GameID, userID); err != nil {
		h.renderError(ctx, w, r, core.ErrForbidden(err.Error()), "User is not GM of the game", "error", err)
		return
	}

	// Delete poll
	if err := pollService.DeletePoll(ctx, int32(pollID)); err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to delete poll", "error", err)
		return
	}

	render.Status(r, http.StatusNoContent)
}

// ListPollsByPhase handles GET /games/{gameId}/phases/{phaseId}/polls
func (h *Handler) ListPollsByPhase(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_list_polls_by_phase")()

	// Extract game ID from URL
	gameID := ctx.Value("gameID").(int32)

	// Extract phase ID from URL
	phaseIDStr := chi.URLParam(r, "phaseId")
	phaseID, err := strconv.ParseInt(phaseIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid phase ID")), "Invalid phase ID", "error", err)
		return
	}

	// Authenticate user
	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		h.renderError(ctx, w, r, errResp, "Failed to authenticate user from JWT")
		return
	}

	// Check access (completed games are public)
	access, err := h.checkPollViewAccess(ctx, int32(gameID), userID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check poll view access", "error", err)
		return
	}
	if !access.allowed {
		h.renderError(ctx, w, r, core.ErrForbidden("user is not a participant in this game"), "List polls by phase forbidden")
		return
	}

	// List polls by phase
	pollService := h.PollService
	polls, err := pollService.ListPollsByPhase(ctx, int32(gameID), int32(phaseID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to list polls by phase", "error", err, "game_id", gameID, "phase_id", phaseID)
		return
	}

	// Add user_has_voted field to each poll
	pollListItems := make([]PollListItem, len(polls))
	for i, poll := range polls {
		hasVoted, err := pollService.HasUserVoted(ctx, poll.ID, userID)
		if err != nil {
			h.App.ObsLogger.LogError(ctx, err, "Failed to check if user voted", "poll_id", poll.ID)
			hasVoted = false
		}

		pollListItems[i] = PollListItem{
			CommonRoomPoll: poll,
			UserHasVoted:   hasVoted,
		}
	}

	h.App.ObsLogger.Info(ctx, "Listed polls by phase", "game_id", gameID, "phase_id", phaseID, "poll_count", len(polls))
	render.JSON(w, r, pollListItems)
}
