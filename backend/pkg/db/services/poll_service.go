package db

import (
	"context"
	"errors"
	"fmt"

	"actionphase/pkg/core"
	db "actionphase/pkg/db/models"
	"actionphase/pkg/observability"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PollService implements the PollServiceInterface
type PollService struct {
	DB     *pgxpool.Pool
	Logger *observability.Logger
}

// Compile-time verification that PollService implements PollServiceInterface
var _ core.PollServiceInterface = (*PollService)(nil)

// CreatePollWithOptions creates a new poll with its options in a transaction
func (s *PollService) CreatePollWithOptions(ctx context.Context, req core.CreatePollRequest) (*core.PollWithOptions, error) {
	s.Logger.Info(ctx, "Creating poll with options",
		"game_id", req.GameID,
		"created_by_user_id", req.CreatedByUserID,
		"option_count", len(req.Options),
	)

	// Start a transaction
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		s.Logger.LogError(ctx, err, "Failed to begin poll creation transaction",
			"game_id", req.GameID,
		)
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // Rollback if we don't commit

	queries := db.New(tx)

	// Convert deadline to pgtype.Timestamptz
	deadline := pgtype.Timestamptz{}
	if err := deadline.Scan(req.Deadline); err != nil {
		return nil, fmt.Errorf("invalid deadline timestamp: %w", err)
	}

	// Convert optional fields
	phaseID := pgtype.Int4{}
	if req.PhaseID != nil {
		phaseID.Int32 = *req.PhaseID
		phaseID.Valid = true
	}

	createdByCharacterID := pgtype.Int4{}
	if req.CreatedByCharacterID != nil {
		createdByCharacterID.Int32 = *req.CreatedByCharacterID
		createdByCharacterID.Valid = true
	}

	description := pgtype.Text{}
	if req.Description != nil {
		description.String = *req.Description
		description.Valid = true
	}

	showIndividualVotes := pgtype.Bool{Bool: req.ShowIndividualVotes, Valid: true}
	allowOtherOption := pgtype.Bool{Bool: req.AllowOtherOption, Valid: true}

	// Create the poll
	pollParams := db.CreatePollParams{
		GameID:                 req.GameID,
		PhaseID:                phaseID,
		CreatedByUserID:        req.CreatedByUserID,
		CreatedByCharacterID:   createdByCharacterID,
		Question:               req.Question,
		Description:            description,
		Deadline:               deadline,
		ShowIndividualVotes:    showIndividualVotes,
		AllowOtherOption:       allowOtherOption,
		HideResultsFromPlayers: req.HideResultsFromPlayers,
		AllowAudienceVoting:    req.AllowAudienceVoting,
	}

	poll, err := queries.CreatePoll(ctx, pollParams)
	if err != nil {
		s.Logger.LogError(ctx, err, "Failed to create poll",
			"game_id", req.GameID,
			"created_by_user_id", req.CreatedByUserID,
		)
		return nil, fmt.Errorf("failed to create poll: %w", err)
	}

	// Create poll options
	options := make([]db.PollOption, 0, len(req.Options))
	for _, opt := range req.Options {
		optionParams := db.CreatePollOptionParams{
			PollID:       poll.ID,
			OptionText:   opt.Text,
			DisplayOrder: opt.DisplayOrder,
		}
		option, err := queries.CreatePollOption(ctx, optionParams)
		if err != nil {
			s.Logger.LogError(ctx, err, "Failed to create poll option",
				"poll_id", poll.ID,
				"option_text", opt.Text,
			)
			return nil, fmt.Errorf("failed to create poll option: %w", err)
		}
		options = append(options, option)
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		s.Logger.LogError(ctx, err, "Failed to commit poll creation transaction",
			"poll_id", poll.ID,
		)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.Logger.Info(ctx, "Poll created successfully",
		"poll_id", poll.ID,
		"game_id", req.GameID,
		"option_count", len(options),
	)

	return &core.PollWithOptions{
		Poll:    poll,
		Options: options,
	}, nil
}

// isAudienceRole reports whether a poll voter participated as audience. Audience
// votes are always reported anonymously, so identity must be stripped before
// results leave the service.
func isAudienceRole(role pgtype.Text) bool {
	return role.Valid && role.String == "audience"
}

// GetPoll retrieves a specific poll by ID
func (s *PollService) GetPoll(ctx context.Context, pollID int32) (*db.CommonRoomPoll, error) {
	queries := db.New(s.DB)

	poll, err := queries.GetPoll(ctx, pollID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("poll not found: %d", pollID)
		}
		return nil, fmt.Errorf("failed to get poll: %w", err)
	}

	return &poll, nil
}

// GetPollWithOptions retrieves a poll with all its options
func (s *PollService) GetPollWithOptions(ctx context.Context, pollID int32) (*core.PollWithOptions, error) {
	queries := db.New(s.DB)

	// Get poll
	poll, err := queries.GetPoll(ctx, pollID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("poll not found: %d", pollID)
		}
		return nil, fmt.Errorf("failed to get poll: %w", err)
	}

	// Get options
	options, err := queries.GetPollOptions(ctx, pollID)
	if err != nil {
		return nil, fmt.Errorf("failed to get poll options: %w", err)
	}

	if options == nil {
		options = []db.PollOption{}
	}

	return &core.PollWithOptions{
		Poll:    poll,
		Options: options,
	}, nil
}

// ListPollsByPhase retrieves all active polls for a specific game phase
func (s *PollService) ListPollsByPhase(ctx context.Context, gameID int32, phaseID int32) ([]db.CommonRoomPoll, error) {
	queries := db.New(s.DB)

	params := db.ListPollsByPhaseParams{
		GameID:  gameID,
		PhaseID: pgtype.Int4{Int32: phaseID, Valid: true},
	}

	polls, err := queries.ListPollsByPhase(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list polls by phase: %w", err)
	}

	if polls == nil {
		return []db.CommonRoomPoll{}, nil
	}

	return polls, nil
}

// ListPollsByGame retrieves all active polls for a game
func (s *PollService) ListPollsByGame(ctx context.Context, gameID int32, includeExpired bool) ([]db.CommonRoomPoll, error) {
	queries := db.New(s.DB)

	params := db.ListPollsByGameParams{
		GameID:  gameID,
		Column2: includeExpired,
	}

	polls, err := queries.ListPollsByGame(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list polls by game: %w", err)
	}

	if polls == nil {
		return []db.CommonRoomPoll{}, nil
	}

	return polls, nil
}

// SubmitVote submits or updates a user's vote for a poll
func (s *PollService) SubmitVote(ctx context.Context, req core.SubmitVoteRequest) (*db.PollVote, error) {
	s.Logger.Info(ctx, "Submitting vote",
		"poll_id", req.PollID,
		"user_id", req.UserID,
	)

	queries := db.New(s.DB)

	selectedOptionID := pgtype.Int4{}
	if req.SelectedOptionID != nil {
		selectedOptionID.Int32 = *req.SelectedOptionID
		selectedOptionID.Valid = true
	}

	otherResponse := pgtype.Text{}
	if req.OtherResponse != nil {
		otherResponse.String = *req.OtherResponse
		otherResponse.Valid = true
	}

	// Check if user has already voted
	existingVoteParams := db.GetVoteByPollAndUserParams{
		PollID: req.PollID,
		UserID: req.UserID,
	}

	existingVote, err := queries.GetVoteByPollAndUser(ctx, existingVoteParams)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		s.Logger.LogError(ctx, err, "Failed to check existing vote",
			"poll_id", req.PollID,
			"user_id", req.UserID,
		)
		return nil, fmt.Errorf("failed to check existing vote: %w", err)
	}

	// If vote exists, update it
	if err == nil {
		s.Logger.Info(ctx, "Updating existing vote",
			"vote_id", existingVote.ID,
			"poll_id", req.PollID,
			"user_id", req.UserID,
		)

		updateParams := db.UpdateVoteParams{
			ID:               existingVote.ID,
			SelectedOptionID: selectedOptionID,
			OtherResponse:    otherResponse,
		}
		vote, err := queries.UpdateVote(ctx, updateParams)
		if err != nil {
			s.Logger.LogError(ctx, err, "Failed to update vote",
				"vote_id", existingVote.ID,
				"poll_id", req.PollID,
			)
			return nil, fmt.Errorf("failed to update vote: %w", err)
		}

		s.Logger.Info(ctx, "Vote updated successfully",
			"vote_id", vote.ID,
			"poll_id", req.PollID,
		)

		return &vote, nil
	}

	// Otherwise, create new vote
	voteParams := db.SubmitVoteParams{
		PollID:           req.PollID,
		UserID:           req.UserID,
		SelectedOptionID: selectedOptionID,
		OtherResponse:    otherResponse,
	}

	vote, err := queries.SubmitVote(ctx, voteParams)
	if err != nil {
		s.Logger.LogError(ctx, err, "Failed to submit new vote",
			"poll_id", req.PollID,
			"user_id", req.UserID,
		)
		return nil, fmt.Errorf("failed to submit vote: %w", err)
	}

	s.Logger.Info(ctx, "Vote submitted successfully",
		"vote_id", vote.ID,
		"poll_id", req.PollID,
		"user_id", req.UserID,
	)

	return &vote, nil
}

// GetVote retrieves a user's vote for a poll
func (s *PollService) GetVote(ctx context.Context, pollID int32, userID int32) (*db.PollVote, error) {
	queries := db.New(s.DB)

	params := db.GetVoteByPollAndUserParams{
		PollID: pollID,
		UserID: userID,
	}

	vote, err := queries.GetVoteByPollAndUser(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // User hasn't voted
		}
		return nil, fmt.Errorf("failed to get vote: %w", err)
	}

	return &vote, nil
}

// GetPollResults retrieves aggregated results for a poll
// If canSeeIndividualVotes is true (GM, co-GM, or audience), individual votes are always included regardless of poll settings
func (s *PollService) GetPollResults(ctx context.Context, pollID int32, canSeeIndividualVotes bool) (*core.PollResults, error) {
	queries := db.New(s.DB)

	// Get the poll
	poll, err := queries.GetPoll(ctx, pollID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("poll not found: %d", pollID)
		}
		return nil, fmt.Errorf("failed to get poll: %w", err)
	}

	// Get vote summary
	summary, err := queries.GetPollResultsSummary(ctx, pollID)
	if err != nil {
		return nil, fmt.Errorf("failed to get poll results summary: %w", err)
	}

	// Build option results
	optionResults := make([]core.OptionResult, 0, len(summary))
	totalVotes := int32(0)

	for _, row := range summary {
		optResult := core.OptionResult{
			Option: db.PollOption{
				ID:           row.OptionID,
				OptionText:   row.OptionText,
				DisplayOrder: row.DisplayOrder,
			},
			VoteCount: int32(row.VoteCount),
			Voters:    []core.VoterInfo{},
		}
		totalVotes += int32(row.VoteCount)
		optionResults = append(optionResults, optResult)
	}

	// Get "other" responses count
	otherCount, err := queries.GetOtherVoteCount(ctx, pollID)
	if err != nil {
		return nil, fmt.Errorf("failed to get other vote count: %w", err)
	}
	totalVotes += int32(otherCount)

	// Get detailed votes if show_individual_votes is true OR user has privileges
	// GMs, co-GMs, and audience always see individual votes for game management/observation
	showIndividualVotes := poll.ShowIndividualVotes.Bool || canSeeIndividualVotes
	if showIndividualVotes {
		votes, err := queries.GetPollVotesWithDetails(ctx, pollID)
		if err != nil {
			return nil, fmt.Errorf("failed to get detailed votes: %w", err)
		}

		// Map votes to their options
		for _, vote := range votes {
			if vote.SelectedOptionID.Valid {
				// Find the option result and add voter info
				for i := range optionResults {
					if optionResults[i].Option.ID == vote.SelectedOptionID.Int32 {
						var voterInfo core.VoterInfo
						if isAudienceRole(vote.VoterRole) {
							// Audience votes are anonymous to everyone, including the
							// GM: withhold identity here so it never leaves the service.
							voterInfo = core.VoterInfo{IsAnonymous: true}
						} else {
							voterInfo = core.VoterInfo{
								UserID:   vote.UserID,
								Username: vote.Username,
							}
							if vote.CharacterName.Valid {
								charName := vote.CharacterName.String
								voterInfo.CharacterName = &charName
							}
						}
						optionResults[i].Voters = append(optionResults[i].Voters, voterInfo)
						break
					}
				}
			}
		}
	}

	// Get "other" responses with details
	otherResponses := []core.OtherResponse{}
	if poll.AllowOtherOption.Bool {
		others, err := queries.GetOtherResponses(ctx, pollID)
		if err != nil {
			return nil, fmt.Errorf("failed to get other responses: %w", err)
		}

		for _, other := range others {
			if isAudienceRole(other.VoterRole) {
				// Audience responses keep their text but shed all attribution.
				otherResponses = append(otherResponses, core.OtherResponse{
					VoteID:      other.ID,
					OtherText:   other.OtherResponse.String,
					IsAnonymous: true,
				})
				continue
			}

			otherResp := core.OtherResponse{
				VoteID:    other.ID,
				OtherText: other.OtherResponse.String,
				Username:  other.Username,
			}
			if other.CharacterName.Valid {
				charName := other.CharacterName.String
				otherResp.CharacterName = &charName
			}
			otherResponses = append(otherResponses, otherResp)
		}
	}

	return &core.PollResults{
		Poll:                poll,
		OptionResults:       optionResults,
		OtherResponses:      otherResponses,
		TotalVotes:          totalVotes,
		ShowIndividualVotes: showIndividualVotes,
	}, nil
}

// UpdatePoll updates poll details
func (s *PollService) UpdatePoll(ctx context.Context, pollID int32, req core.UpdatePollRequest) (*db.CommonRoomPoll, error) {
	queries := db.New(s.DB)

	// Convert deadline to pgtype.Timestamptz
	deadline := pgtype.Timestamptz{}
	if err := deadline.Scan(req.Deadline); err != nil {
		return nil, fmt.Errorf("invalid deadline timestamp: %w", err)
	}

	description := pgtype.Text{}
	if req.Description != nil {
		description.String = *req.Description
		description.Valid = true
	}

	showIndividualVotes := pgtype.Bool{Bool: req.ShowIndividualVotes, Valid: true}
	allowOtherOption := pgtype.Bool{Bool: req.AllowOtherOption, Valid: true}

	params := db.UpdatePollParams{
		ID:                     pollID,
		Question:               req.Question,
		Description:            description,
		Deadline:               deadline,
		ShowIndividualVotes:    showIndividualVotes,
		AllowOtherOption:       allowOtherOption,
		HideResultsFromPlayers: req.HideResultsFromPlayers,
		AllowAudienceVoting:    req.AllowAudienceVoting,
	}

	poll, err := queries.UpdatePoll(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("poll not found: %d", pollID)
		}
		return nil, fmt.Errorf("failed to update poll: %w", err)
	}

	return &poll, nil
}

// DeletePoll soft-deletes a poll by setting is_deleted flag
func (s *PollService) DeletePoll(ctx context.Context, pollID int32) error {
	s.Logger.Info(ctx, "Deleting poll",
		"poll_id", pollID,
	)

	queries := db.New(s.DB)

	// First verify the poll exists
	_, err := queries.GetPoll(ctx, pollID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			s.Logger.Warn(ctx, "Attempted to delete non-existent poll",
				"poll_id", pollID,
			)
			return fmt.Errorf("poll not found: %d", pollID)
		}
		s.Logger.LogError(ctx, err, "Failed to verify poll exists",
			"poll_id", pollID,
		)
		return fmt.Errorf("failed to verify poll: %w", err)
	}

	// Perform soft delete
	if err := queries.SoftDeletePoll(ctx, pollID); err != nil {
		s.Logger.LogError(ctx, err, "Failed to soft delete poll",
			"poll_id", pollID,
		)
		return fmt.Errorf("failed to delete poll: %w", err)
	}

	s.Logger.Info(ctx, "Poll deleted successfully",
		"poll_id", pollID,
	)

	return nil
}

// HasUserVoted checks if a user has already voted in a poll
func (s *PollService) HasUserVoted(ctx context.Context, pollID int32, userID int32) (bool, error) {
	queries := db.New(s.DB)

	params := db.HasUserVotedParams{
		PollID: pollID,
		UserID: userID,
	}

	hasVoted, err := queries.HasUserVoted(ctx, params)
	if err != nil {
		return false, fmt.Errorf("failed to check if user voted: %w", err)
	}

	return hasVoted, nil
}
