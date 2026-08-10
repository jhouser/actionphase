package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	"actionphase/pkg/observability"
)

var _ core.GameStatsServiceInterface = (*GameStatsService)(nil)

// GameStatsService computes post-game statistics for completed games.
type GameStatsService struct {
	DB     *pgxpool.Pool
	Logger *observability.Logger
}

// GetGameStats assembles the full statistics payload for a game.
//
// Applies no authorization — the handler decides who may see this.
func (s *GameStatsService) GetGameStats(ctx context.Context, gameID int32) (*core.GameStats, error) {
	defer s.Logger.LogOperation(ctx, "get_game_stats", "game_id", gameID)()

	queries := models.New(s.DB)

	commentRow, err := queries.GetGameCommentStats(ctx, gameID)
	if err != nil {
		s.Logger.LogError(ctx, err, "Failed to get game comment stats", "game_id", gameID)
		return nil, fmt.Errorf("failed to get comment stats: %w", err)
	}

	pmRow, err := queries.GetGamePrivateMessageStats(ctx, gameID)
	if err != nil {
		s.Logger.LogError(ctx, err, "Failed to get game private message stats", "game_id", gameID)
		return nil, fmt.Errorf("failed to get private message stats: %w", err)
	}

	characterRows, err := queries.GetCharacterStatsByGame(ctx, gameID)
	if err != nil {
		s.Logger.LogError(ctx, err, "Failed to get per-character stats", "game_id", gameID)
		return nil, fmt.Errorf("failed to get character stats: %w", err)
	}

	stats := &core.GameStats{
		Comments: core.CommentStats{
			TotalComments:        commentRow.TotalComments,
			AvgCommentLength:     commentRow.AvgCommentLength,
			LongestCommentLength: commentRow.LongestCommentLength,
			MaxThreadDepth:       commentRow.MaxThreadDepth,
			AvgThreadDepth:       commentRow.AvgThreadDepth,
		},
		PrivateMessages: core.PrivateMessageStats{
			TotalPrivateMessages:        pmRow.TotalPrivateMessages,
			AvgPrivateMessageLength:     pmRow.AvgPrivateMessageLength,
			LongestPrivateMessageLength: pmRow.LongestPrivateMessageLength,
			ActiveConversations:         pmRow.ActiveConversations,
		},
		// Non-nil empty slice so the JSON field serializes as [] rather than
		// null for a game with no characters.
		Characters: make([]core.CharacterWriting, 0, len(characterRows)),
	}

	for _, row := range characterRows {
		stats.Characters = append(stats.Characters, core.CharacterWriting{
			CharacterID:             row.CharacterID,
			CharacterName:           row.CharacterName,
			IsActive:                row.IsActive,
			CommentCount:            row.CommentCount,
			AvgCommentLength:        row.AvgCommentLength,
			PrivateMessageCount:     row.PrivateMessageCount,
			AvgPrivateMessageLength: row.AvgPrivateMessageLength,
		})
	}

	longestComment, err := queries.GetGameLongestComment(ctx, gameID)
	if err != nil {
		// No comments at all is ordinary; leave the field nil.
		if !errors.Is(err, pgx.ErrNoRows) {
			s.Logger.LogError(ctx, err, "Failed to get longest comment", "game_id", gameID)
			return nil, fmt.Errorf("failed to get longest comment: %w", err)
		}
	} else {
		lc := &core.LongestComment{
			MessageID:     longestComment.MessageID,
			ContentLength: longestComment.ContentLength,
		}
		if longestComment.CharacterName.Valid {
			lc.CharacterName = longestComment.CharacterName.String
		}
		if longestComment.PhaseID.Valid {
			lc.PhaseID = longestComment.PhaseID.Int32
		}
		stats.LongestComment = lc
	}

	longest, err := queries.GetGameLongestConversation(ctx, gameID)
	if err != nil {
		// A game with no private conversations (or none carrying messages) is
		// an ordinary outcome, not a failure: leave LongestConversation nil and
		// return the rest of the stats.
		if errors.Is(err, pgx.ErrNoRows) {
			return stats, nil
		}
		s.Logger.LogError(ctx, err, "Failed to get longest conversation", "game_id", gameID)
		return nil, fmt.Errorf("failed to get longest conversation: %w", err)
	}

	conv := &core.LongestConversation{
		ConversationID:   longest.ConversationID,
		ConversationType: longest.ConversationType,
		MessageCount:     longest.MessageCount,
		ParticipantNames: longest.ParticipantNames,
	}
	if longest.Title.Valid {
		conv.Title = longest.Title.String
	}
	if longest.FirstMessageAt.Valid {
		t := longest.FirstMessageAt.Time
		conv.FirstMessageAt = &t
	}
	if longest.LastMessageAt.Valid {
		t := longest.LastMessageAt.Time
		conv.LastMessageAt = &t
	}
	if conv.ParticipantNames == nil {
		conv.ParticipantNames = []string{}
	}
	stats.LongestConversation = conv

	return stats, nil
}
