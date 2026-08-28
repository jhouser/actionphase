package polls

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"actionphase/pkg/core"
	db "actionphase/pkg/db/models"
)

// Input / output types
//
// The response bodies embed the sqlc models (db.CommonRoomPoll and friends)
// exactly as the chi handlers did. Those marshal to clean scalars at runtime --
// pgtype implements MarshalJSON -- but huma builds schemas by reflecting over
// Go fields, so the *documented* shape of the nullable columns is wrapper-ish.
// Replacing them with hand-written DTOs is tracked separately in
// .claude/planning/sqlc-models-in-api-responses.md; this file is a behaviour-preserving
// port and deliberately does not change any wire format.

type createPollInput struct {
	GameID int32              `path:"gameID" doc:"Game ID"`
	Body   *CreatePollRequest `required:"true"`
}

type pollOutput struct {
	Body *PollResponse
}

type pollListOutput struct {
	// A bare JSON array, matching what the chi handler rendered.
	Body []PollListItem
}

type listGamePollsInput struct {
	GameID         int32 `path:"gameID" doc:"Game ID"`
	IncludeExpired bool  `query:"include_expired" doc:"Include polls whose deadline has passed"`
}

type listPhasePollsInput struct {
	GameID  int32 `path:"gameID" doc:"Game ID"`
	PhaseID int32 `path:"phaseId" doc:"Phase ID"`
}

type pollIDInput struct {
	PollID int32 `path:"pollId" doc:"Poll ID"`
}

type pollResultsOutput struct {
	Body *PollResultsResponse
}

type updatePollInput struct {
	PollID int32              `path:"pollId" doc:"Poll ID"`
	Body   *UpdatePollRequest `required:"true"`
}

type submitVoteInput struct {
	PollID int32              `path:"pollId" doc:"Poll ID"`
	Body   *SubmitVoteRequest `required:"true"`
}

type voteOutput struct {
	Body *db.PollVote
}

// RegisterHumaGamePolls registers the poll operations that hang off the
// /{gameID} subrouter. It must share the game-scoped API with the other
// packages mounted there.
func RegisterHumaGamePolls(api huma.API, h *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "createPoll",
		Method:      http.MethodPost,
		Path:        "/polls",
		Summary:     "Create a poll",
		Description: "Creates a poll in the game's common room. GM or Co-GM only.",
		Tags:        []string{"Polls"},
	}, h.createPoll)

	huma.Register(api, huma.Operation{
		OperationID: "listGamePolls",
		Method:      http.MethodGet,
		Path:        "/polls",
		Summary:     "List game polls",
		Tags:        []string{"Polls"},
	}, h.listGamePolls)

	huma.Register(api, huma.Operation{
		OperationID: "listPhasePolls",
		Method:      http.MethodGet,
		Path:        "/phases/{phaseId}/polls",
		Summary:     "List polls for a phase",
		Tags:        []string{"Polls"},
	}, h.listPhasePolls)
}

// RegisterHumaPolls registers the poll-scoped operations, mounted at /polls.
func RegisterHumaPolls(api huma.API, h *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "getPoll",
		Method:      http.MethodGet,
		Path:        "/{pollId}",
		Summary:     "Get a poll",
		Tags:        []string{"Polls"},
	}, h.getPoll)

	huma.Register(api, huma.Operation{
		OperationID: "getPollResults",
		Method:      http.MethodGet,
		Path:        "/{pollId}/results",
		Summary:     "Get poll results",
		Description: "Results are withheld from players while voting is open, and " +
			"entirely when the GM set hide_results_from_players.",
		Tags: []string{"Polls"},
	}, h.getPollResults)

	huma.Register(api, huma.Operation{
		OperationID: "submitVote",
		Method:      http.MethodPost,
		Path:        "/{pollId}/vote",
		Summary:     "Submit a vote",
		Tags:        []string{"Polls"},
	}, h.submitVote)

	huma.Register(api, huma.Operation{
		OperationID: "updatePoll",
		Method:      http.MethodPut,
		Path:        "/{pollId}",
		Summary:     "Update a poll",
		Tags:        []string{"Polls"},
	}, h.updatePoll)

	huma.Register(api, huma.Operation{
		OperationID: "deletePoll",
		Method:      http.MethodDelete,
		Path:        "/{pollId}",
		Summary:     "Delete a poll",
		Tags:        []string{"Polls"},
		// The chi handler called render.Status(r, 204) without a following
		// write, so it actually answered 200 with an empty body -- a known bug
		// its test pinned rather than fixed. Corrected here to the intended 204.
		DefaultStatus: http.StatusNoContent,
	}, h.deletePoll)
}

// humaErr converts a core error response into the equivalent huma error,
// preserving the status and message the chi handlers produced.
func humaErr(errResp any) error {
	if resp, ok := errResp.(*core.ErrResponse); ok {
		return huma.NewError(resp.HTTPStatusCode, resp.ErrorText)
	}
	return huma.Error500InternalServerError("unexpected error")
}

func (h *Handler) authUser(ctx context.Context) (int32, error) {
	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to authenticate user from JWT")
		return 0, humaErr(errResp)
	}
	return userID, nil
}

func (h *Handler) createPoll(ctx context.Context, in *createPollInput) (*pollOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_create_poll")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.verifyUserIsGM(ctx, in.GameID, userID); err != nil {
		h.App.ObsLogger.Warn(ctx, "User is not GM of the game", "error", err)
		return nil, huma.Error403Forbidden(err.Error())
	}

	options := make([]core.PollOptionInput, len(in.Body.Options))
	for i, opt := range in.Body.Options {
		options[i] = core.PollOptionInput{
			Text:         opt.Text,
			DisplayOrder: opt.DisplayOrder,
		}
	}

	pollWithOptions, err := h.PollService.CreatePollWithOptions(ctx, core.CreatePollRequest{
		GameID:                     in.GameID,
		PhaseID:                    in.Body.PhaseID,
		CreatedByUserID:            userID,
		Question:                   in.Body.Question,
		Description:                in.Body.Description,
		Deadline:                   in.Body.Deadline,
		ShowIndividualVotes:        in.Body.ShowIndividualVotes,
		AllowOtherOption:           in.Body.AllowOtherOption,
		HideResultsFromPlayers:     in.Body.HideResultsFromPlayers,
		AllowAudienceVoting:        in.Body.AllowAudienceVoting,
		ShowRunningTotalsToPlayers: in.Body.ShowRunningTotalsToPlayers,
		Options:                    options,
	})
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to create poll")
		return nil, huma.Error500InternalServerError("Failed to create poll")
	}

	h.notifyPollCreated(ctx, in.GameID, userID, pollWithOptions)

	return &pollOutput{Body: &PollResponse{
		CommonRoomPoll: pollWithOptions.Poll,
		Options:        pollWithOptions.Options,
	}}, nil
}

// notifyPollCreated tells the other participants a poll went up. Notification
// failures are logged, never fatal: the poll already exists by this point.
func (h *Handler) notifyPollCreated(ctx context.Context, gameID, creatorID int32, pollWithOptions *core.PollWithOptions) {
	participants, err := h.GameService.GetGameParticipants(ctx, gameID)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to get game participants for notification")
		return
	}

	userIDs := make([]int32, 0, len(participants))
	for _, participant := range participants {
		if participant.UserID != creatorID {
			userIDs = append(userIDs, participant.UserID)
		}
	}
	if len(userIDs) == 0 {
		return
	}

	gameIDCopy := gameID
	linkURL := fmt.Sprintf("/games/%d?tab=polls", gameID)
	if err := h.NotificationService.CreateBulkNotifications(ctx, userIDs, &core.CreateNotificationRequest{
		GameID:      &gameIDCopy,
		Type:        "poll_created",
		Title:       fmt.Sprintf("New Poll: %s", pollWithOptions.Poll.Question),
		RelatedType: strPtr("poll"),
		RelatedID:   &pollWithOptions.Poll.ID,
		LinkURL:     &linkURL,
	}); err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to create bulk notifications")
	}
}

// pollListItems decorates polls with the caller's vote status. A failed lookup
// degrades to "not voted" rather than failing the whole list.
func (h *Handler) pollListItems(ctx context.Context, polls []db.CommonRoomPoll, userID int32) []PollListItem {
	items := make([]PollListItem, len(polls))
	for i, poll := range polls {
		hasVoted, err := h.PollService.HasUserVoted(ctx, poll.ID, userID)
		if err != nil {
			h.App.ObsLogger.LogError(ctx, err, "Failed to check if user voted", "poll_id", poll.ID)
			hasVoted = false
		}
		items[i] = PollListItem{CommonRoomPoll: poll, UserHasVoted: hasVoted}
	}
	return items
}

func (h *Handler) listGamePolls(ctx context.Context, in *listGamePollsInput) (*pollListOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_list_game_polls")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	access, err := h.checkPollViewAccess(ctx, in.GameID, userID)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to check poll view access")
		return nil, huma.Error500InternalServerError("Failed to check poll view access")
	}
	if !access.allowed {
		return nil, huma.Error403Forbidden("user is not a participant in this game")
	}

	polls, err := h.PollService.ListPollsByGame(ctx, in.GameID, in.IncludeExpired)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to list polls")
		return nil, huma.Error500InternalServerError("Failed to list polls")
	}

	return &pollListOutput{Body: h.pollListItems(ctx, polls, userID)}, nil
}

func (h *Handler) listPhasePolls(ctx context.Context, in *listPhasePollsInput) (*pollListOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_list_polls_by_phase")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	access, err := h.checkPollViewAccess(ctx, in.GameID, userID)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to check poll view access")
		return nil, huma.Error500InternalServerError("Failed to check poll view access")
	}
	if !access.allowed {
		return nil, huma.Error403Forbidden("user is not a participant in this game")
	}

	polls, err := h.PollService.ListPollsByPhase(ctx, in.GameID, in.PhaseID)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to list polls by phase",
			"game_id", in.GameID, "phase_id", in.PhaseID)
		return nil, huma.Error500InternalServerError("Failed to list polls by phase")
	}

	items := h.pollListItems(ctx, polls, userID)
	h.App.ObsLogger.Info(ctx, "Listed polls by phase",
		"game_id", in.GameID, "phase_id", in.PhaseID, "poll_count", len(polls))
	return &pollListOutput{Body: items}, nil
}

func (h *Handler) getPoll(ctx context.Context, in *pollIDInput) (*pollOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_poll")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	pollWithOptions, err := h.PollService.GetPollWithOptions(ctx, in.PollID)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get poll", "error", err)
		return nil, huma.Error404NotFound("poll not found")
	}

	access, err := h.checkPollViewAccess(ctx, pollWithOptions.Poll.GameID, userID)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to check poll view access")
		return nil, huma.Error500InternalServerError("Failed to check poll view access")
	}
	if !access.allowed {
		return nil, huma.Error403Forbidden("user is not a participant in this game")
	}

	hasVoted, err := h.PollService.HasUserVoted(ctx, in.PollID, userID)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to check if user voted")
		return nil, huma.Error500InternalServerError("Failed to check if user voted")
	}

	// A vote lookup failure is treated as "no vote recorded" rather than an
	// error, matching the chi handler: the poll itself is still returnable.
	var userVoteOptionID *int32
	var userVoteOtherResponse *string
	if hasVoted {
		if vote, err := h.PollService.GetVote(ctx, in.PollID, userID); err == nil && vote != nil {
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

	return &pollOutput{Body: &PollResponse{
		CommonRoomPoll:        pollWithOptions.Poll,
		Options:               pollWithOptions.Options,
		HasVoted:              hasVoted,
		UserVoteOptionID:      userVoteOptionID,
		UserVoteOtherResponse: userVoteOtherResponse,
	}}, nil
}

func (h *Handler) getPollResults(ctx context.Context, in *pollIDInput) (*pollResultsOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_poll_results")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	poll, err := h.PollService.GetPoll(ctx, in.PollID)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get poll", "error", err)
		return nil, huma.Error404NotFound("poll not found")
	}

	access, err := h.checkPollViewAccess(ctx, poll.GameID, userID)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to check poll view access")
		return nil, huma.Error500InternalServerError("Failed to check poll view access")
	}
	if !access.allowed {
		return nil, huma.Error403Forbidden("user is not a participant in this game")
	}

	// Authorize before reading results: both checks answer from the poll row we
	// already have, so an unauthorized request never pays for the results queries.

	// Hidden-results polls do not disclose results to players while the game runs --
	// not even after the deadline. GM, co-GM and audience are exempt, as is every
	// viewer once the game completes and becomes a public archive.
	if poll.HideResultsFromPlayers && !access.isPrivileged {
		return nil, huma.Error403Forbidden("poll results are hidden by the GM")
	}

	// Regular players can only see results after the poll expires, unless the GM
	// opted into running totals; privileged users can always view.
	if !access.canSeeIndividualVotes && !poll.ShowRunningTotalsToPlayers && !poll.Deadline.Time.Before(time.Now()) {
		return nil, huma.Error403Forbidden("poll results not available until voting closes")
	}

	results, err := h.PollService.GetPollResults(ctx, in.PollID, access.canSeeIndividualVotes)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get poll results", "error", err)
		return nil, huma.Error404NotFound("poll not found")
	}

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
			voters[j] = VoterInfo{UserID: voter.UserID, CharacterName: characterName}
		}

		// Flatten option fields to top level. For "Other" responses these are
		// zero values, which is how the frontend tells the two apart.
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

	return &pollResultsOutput{Body: &PollResultsResponse{
		Poll:                results.Poll,
		OptionResults:       optionResults,
		OtherResponses:      otherResponses,
		TotalVotes:          results.TotalVotes,
		ShowIndividualVotes: results.ShowIndividualVotes,
	}}, nil
}

func (h *Handler) submitVote(ctx context.Context, in *submitVoteInput) (*voteOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_submit_vote")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	poll, err := h.PollService.GetPoll(ctx, in.PollID)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get poll", "error", err)
		return nil, huma.Error404NotFound("poll not found")
	}

	if err := h.verifyUserInGame(ctx, poll.GameID, userID); err != nil {
		h.App.ObsLogger.Warn(ctx, "User is not in the game", "error", err)
		return nil, huma.Error403Forbidden(err.Error())
	}

	game, err := h.GameService.GetGame(ctx, poll.GameID)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to get game")
		return nil, huma.Error500InternalServerError("Failed to get game")
	}

	isGM := game.GmUserID == userID
	isCoGM := core.IsUserCoGM(ctx, h.App.Pool, poll.GameID, userID)
	isAudience := core.IsUserAudience(ctx, h.App.Pool, poll.GameID, userID)

	if isGM || isCoGM {
		return nil, huma.Error403Forbidden("GMs and co-GMs cannot vote on polls")
	}

	if isAudience {
		// Audience may vote only when the GM enabled it for this poll.
		if !poll.AllowAudienceVoting {
			return nil, huma.Error403Forbidden("Audience members cannot vote on polls")
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
			h.App.ObsLogger.LogError(ctx, err, "Failed to check character status")
			return nil, huma.Error500InternalServerError("Failed to check character status")
		}
		if !hasCharacter {
			return nil, huma.Error403Forbidden("you must have an approved character to vote")
		}
	}

	if poll.Deadline.Time.Before(time.Now()) {
		return nil, huma.Error400BadRequest("voting closed - poll deadline has passed")
	}

	vote, err := h.PollService.SubmitVote(ctx, core.SubmitVoteRequest{
		PollID:           in.PollID,
		UserID:           userID,
		SelectedOptionID: in.Body.SelectedOptionID,
		OtherResponse:    in.Body.OtherResponse,
	})
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to submit vote")
		return nil, huma.Error500InternalServerError("Failed to submit vote")
	}

	return &voteOutput{Body: vote}, nil
}

func (h *Handler) updatePoll(ctx context.Context, in *updatePollInput) (*pollOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_poll")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	poll, err := h.PollService.GetPoll(ctx, in.PollID)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get poll", "error", err)
		return nil, huma.Error404NotFound("poll not found")
	}

	if err := h.verifyUserIsGM(ctx, poll.GameID, userID); err != nil {
		h.App.ObsLogger.Warn(ctx, "User is not GM of the game", "error", err)
		return nil, huma.Error403Forbidden(err.Error())
	}

	updatedPoll, err := h.PollService.UpdatePoll(ctx, in.PollID, core.UpdatePollRequest{
		Question:                   in.Body.Question,
		Description:                in.Body.Description,
		Deadline:                   in.Body.Deadline,
		ShowIndividualVotes:        in.Body.ShowIndividualVotes,
		AllowOtherOption:           in.Body.AllowOtherOption,
		HideResultsFromPlayers:     in.Body.HideResultsFromPlayers,
		AllowAudienceVoting:        in.Body.AllowAudienceVoting,
		ShowRunningTotalsToPlayers: in.Body.ShowRunningTotalsToPlayers,
	})
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to update poll")
		return nil, huma.Error500InternalServerError("Failed to update poll")
	}

	// The chi handler rendered the bare updated poll here (no options array),
	// unlike create/get which wrap it in PollResponse.
	return &pollOutput{Body: &PollResponse{CommonRoomPoll: *updatedPoll}}, nil
}

func (h *Handler) deletePoll(ctx context.Context, in *pollIDInput) (*struct{}, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_poll")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	poll, err := h.PollService.GetPoll(ctx, in.PollID)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get poll", "error", err)
		return nil, huma.Error404NotFound("poll not found")
	}

	if err := h.verifyUserIsGM(ctx, poll.GameID, userID); err != nil {
		h.App.ObsLogger.Warn(ctx, "User is not GM of the game", "error", err)
		return nil, huma.Error403Forbidden(err.Error())
	}

	if err := h.PollService.DeletePoll(ctx, in.PollID); err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to delete poll")
		return nil, huma.Error500InternalServerError("Failed to delete poll")
	}

	return nil, nil
}
