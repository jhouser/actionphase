package polls

import (
	"context"
	"fmt"
	"time"

	"actionphase/pkg/core"
	db "actionphase/pkg/db/models"

	"github.com/danielgtaylor/huma/v2"
)

// anonymousVoterName is the display name used for audience votes, which are never
// attributed to a specific user or character.
const anonymousVoterName = "Anonymous"

// Request and Response Types

// CreatePollRequest is the API request for creating a poll
// The boolean flags carry required:"false" because huma treats a non-pointer
// field as required by default, whereas the chi handler let every flag default
// to false when omitted. Without these tags a client that sends only the
// question, deadline and options -- which the frontend does -- gets a 400.
type CreatePollRequest struct {
	Question               string    `json:"question" doc:"Poll question"`
	Description            *string   `json:"description,omitempty" required:"false" doc:"Optional longer description"`
	Deadline               time.Time `json:"deadline" doc:"When voting closes; must be in the future"`
	PhaseID                *int32    `json:"phase_id,omitempty" required:"false" doc:"Associate the poll with a phase"`
	ShowIndividualVotes    bool      `json:"show_individual_votes" required:"false" doc:"Attribute votes to voters"`
	AllowOtherOption       bool      `json:"allow_other_option" required:"false" doc:"Allow free-text responses"`
	HideResultsFromPlayers bool      `json:"hide_results_from_players" required:"false" doc:"Hide results from players entirely"`
	AllowAudienceVoting    bool      `json:"allow_audience_voting" required:"false" doc:"Let audience members vote anonymously"`
	// ShowRunningTotalsToPlayers lets players see results before the deadline.
	ShowRunningTotalsToPlayers bool                `json:"show_running_totals_to_players" required:"false" doc:"Show players results while voting is open"`
	Options                    []PollOptionRequest `json:"options" doc:"At least two options"`
}

// PollOptionRequest represents a poll option in the API request
type PollOptionRequest struct {
	Text         string `json:"text" doc:"Option label"`
	DisplayOrder int32  `json:"display_order" required:"false" doc:"Sort order within the poll"`
}

// Resolve validates the CreatePollRequest.
//
// huma runs this after decoding the body. The rules are cross-field or
// time-dependent, so they cannot be expressed as schema tags and stay as code.
func (req *CreatePollRequest) Resolve(huma.Context) []error {
	var errs []error
	if req.Question == "" {
		errs = append(errs, &huma.ErrorDetail{Message: "question is required", Location: "body.question"})
	}
	if req.Deadline.Before(time.Now()) {
		errs = append(errs, &huma.ErrorDetail{Message: "deadline must be in the future", Location: "body.deadline"})
	}
	if len(req.Options) < 2 {
		errs = append(errs, &huma.ErrorDetail{Message: "at least 2 options are required", Location: "body.options"})
	}
	if req.HideResultsFromPlayers && req.ShowIndividualVotes {
		errs = append(errs, &huma.ErrorDetail{Message: "hide_results_from_players cannot be combined with show_individual_votes", Location: "body.hide_results_from_players"})
	}
	if req.HideResultsFromPlayers && req.ShowRunningTotalsToPlayers {
		errs = append(errs, &huma.ErrorDetail{Message: "hide_results_from_players cannot be combined with show_running_totals_to_players", Location: "body.hide_results_from_players"})
	}
	return errs
}

// UpdatePollRequest is the API request for updating a poll
// See CreatePollRequest for why the flags carry required:"false".
type UpdatePollRequest struct {
	Question                   string    `json:"question" doc:"Poll question"`
	Description                *string   `json:"description,omitempty" required:"false" doc:"Optional longer description"`
	Deadline                   time.Time `json:"deadline" doc:"When voting closes; must be in the future"`
	ShowIndividualVotes        bool      `json:"show_individual_votes" required:"false" doc:"Attribute votes to voters"`
	AllowOtherOption           bool      `json:"allow_other_option" required:"false" doc:"Allow free-text responses"`
	HideResultsFromPlayers     bool      `json:"hide_results_from_players" required:"false" doc:"Hide results from players entirely"`
	AllowAudienceVoting        bool      `json:"allow_audience_voting" required:"false" doc:"Let audience members vote anonymously"`
	ShowRunningTotalsToPlayers bool      `json:"show_running_totals_to_players" required:"false" doc:"Show players results while voting is open"`
}

// Resolve validates the UpdatePollRequest. See CreatePollRequest.Resolve.
func (req *UpdatePollRequest) Resolve(huma.Context) []error {
	var errs []error
	if req.Question == "" {
		errs = append(errs, &huma.ErrorDetail{Message: "question is required", Location: "body.question"})
	}
	if req.Deadline.Before(time.Now()) {
		errs = append(errs, &huma.ErrorDetail{Message: "deadline must be in the future", Location: "body.deadline"})
	}
	if req.HideResultsFromPlayers && req.ShowIndividualVotes {
		errs = append(errs, &huma.ErrorDetail{Message: "hide_results_from_players cannot be combined with show_individual_votes", Location: "body.hide_results_from_players"})
	}
	if req.HideResultsFromPlayers && req.ShowRunningTotalsToPlayers {
		errs = append(errs, &huma.ErrorDetail{Message: "hide_results_from_players cannot be combined with show_running_totals_to_players", Location: "body.hide_results_from_players"})
	}
	return errs
}

// SubmitVoteRequest is the API request for submitting a vote
type SubmitVoteRequest struct {
	SelectedOptionID *int32  `json:"selected_option_id,omitempty" required:"false" doc:"Chosen option ID"`
	OtherResponse    *string `json:"other_response,omitempty" required:"false" doc:"Free-text response when the poll allows it"`
}

// Resolve validates the SubmitVoteRequest.
func (req *SubmitVoteRequest) Resolve(huma.Context) []error {
	if req.SelectedOptionID == nil && req.OtherResponse == nil {
		return []error{&huma.ErrorDetail{
			Message:  "either selected_option_id or other_response is required",
			Location: "body",
		}}
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
	canSeeIndividualVotes bool // true for GM, Co-GM, audience, or any user viewing a public-archive game
	// isPrivileged is true for GM, Co-GM and audience — the roles that may see
	// results of a poll flagged hide_results_from_players — and for any viewer of
	// a public-archive game (completed or epilogue), which grants audience-level access.
	isPrivileged bool
}

// checkPollViewAccess determines what visibility level an authenticated user gets for
// a game's polls. All authenticated users may read polls; the flags control whether
// they see individual vote attribution and results the GM hid from players.
//
// Individual votes and hidden-poll results visible to:
//   - GM / Co-GM: always
//   - Audience: always (spectator role)
//   - Everyone else: only once the game is a public archive (completed or
//     epilogue) — see core.IsPublicArchive
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

	// Public-archive games (completed or epilogue) give every authenticated
	// viewer audience-level access to the whole game, hidden polls included.
	// Anonymity (CanSeeUsernamesInAnonymousGame) lifts at the same point and for
	// the same reason. Cancelled games are NOT public and keep the play-time
	// rules below.
	if core.IsPublicArchive(game.State.String) {
		return pollViewAccess{allowed: true, canSeeIndividualVotes: true, isPrivileged: true}, nil
	}

	// Everyone else (players and non-participants) may see polls exist but not individual votes
	return pollViewAccess{allowed: true, canSeeIndividualVotes: false}, nil
}

// API Handler Methods

// PollListItem represents a poll in the list response with vote status
type PollListItem struct {
	db.CommonRoomPoll
	UserHasVoted bool `json:"user_has_voted"`
}
