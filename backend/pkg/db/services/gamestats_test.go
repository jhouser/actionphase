package db

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"actionphase/pkg/core"
)

// gameStatsFixture builds a game with a GM topic post and returns the ids
// needed to attach comments and private messages to it.
type gameStatsFixture struct {
	gameID int32
	gmID   int32
	postID int32
}

// newGameStatsFixture creates a game plus the GM-authored `post` row that
// comments must hang off (the messages table enforces
// parent_required_for_comments).
func newGameStatsFixture(t *testing.T, testDB *core.TestDatabase, suffix string) gameStatsFixture {
	t.Helper()
	ctx := context.Background()

	gm := testDB.CreateTestUser(t, "gstats_gm_"+suffix, "gstats_gm_"+suffix+"@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Game Stats "+suffix)

	var postID int32
	err := testDB.Pool.QueryRow(ctx, `
		INSERT INTO messages (game_id, author_id, character_id, content, message_type, visibility, thread_depth)
		VALUES ($1, $2, $3, 'GM topic post', 'post', 'game', 0) RETURNING id
	`, game.ID, gm.ID, mustCharacterID(t, testDB, game.ID, int32(gm.ID), "GMChar_"+suffix)).Scan(&postID)
	require.NoError(t, err)

	return gameStatsFixture{gameID: game.ID, gmID: int32(gm.ID), postID: postID}
}

// mustCharacterID creates a character directly and returns its id.
func mustCharacterID(t *testing.T, testDB *core.TestDatabase, gameID, userID int32, name string) int32 {
	t.Helper()
	var id int32
	err := testDB.Pool.QueryRow(context.Background(), `
		INSERT INTO characters (game_id, user_id, name, character_type, status, is_active)
		VALUES ($1, $2, $3, 'player_character', 'approved', true) RETURNING id
	`, gameID, userID, name).Scan(&id)
	require.NoError(t, err)
	return id
}

// insertComment adds a comment replying to parentID and returns its id.
//
// thread_depth is deliberately not passed: a BEFORE INSERT trigger
// (set_message_thread_depth) derives it from the parent, so any value supplied
// here would be silently overwritten. Nesting is therefore expressed by
// chaining parentID, not by asserting a depth.
func insertComment(t *testing.T, testDB *core.TestDatabase, f gameStatsFixture, charID, parentID int32, content string) int32 {
	t.Helper()
	var id int32
	err := testDB.Pool.QueryRow(context.Background(), `
		INSERT INTO messages (game_id, author_id, character_id, content, message_type, visibility, parent_id)
		VALUES ($1, $2, $3, $4, 'comment', 'game', $5) RETURNING id
	`, f.gameID, f.gmID, charID, content, parentID).Scan(&id)
	require.NoError(t, err)
	return id
}

func insertConversation(t *testing.T, testDB *core.TestDatabase, gameID, creatorID int32, convType, title string) int32 {
	t.Helper()
	var id int32
	err := testDB.Pool.QueryRow(context.Background(), `
		INSERT INTO conversations (game_id, created_by_user_id, conversation_type, title)
		VALUES ($1, $2, $3, $4) RETURNING id
	`, gameID, creatorID, convType, title).Scan(&id)
	require.NoError(t, err)
	return id
}

func insertPM(t *testing.T, testDB *core.TestDatabase, convID, senderUserID, senderCharID int32, content string) {
	t.Helper()
	_, err := testDB.Pool.Exec(context.Background(), `
		INSERT INTO private_messages (conversation_id, sender_user_id, sender_character_id, content)
		VALUES ($1, $2, $3, $4)
	`, convID, senderUserID, senderCharID, content)
	require.NoError(t, err)
}

// TestGameStatsService_CommentAggregates verifies game-wide comment stats
// compute real averages and thread depth, and that GM `post` rows, drafts, and
// soft-deleted comments are all excluded.
func TestGameStatsService_CommentAggregates(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "private_messages", "conversations", "messages", "characters", "games", "users")

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	svc := &GameStatsService{DB: testDB.Pool, Logger: app.ObsLogger}

	f := newGameStatsFixture(t, testDB, "comments")
	char := mustCharacterID(t, testDB, f.gameID, f.gmID, "CommentChar")

	// Two comments: one a direct reply to the post (depth 1), one nested a
	// further level below it (depth 2). Lengths 10 and 20 -> average 15.
	// Depths 1 and 2 -> max 2, avg 1.5.
	topLevel := insertComment(t, testDB, f, char, f.postID, strings.Repeat("a", 10))
	insertComment(t, testDB, f, char, topLevel, strings.Repeat("b", 20))

	stats, err := svc.GetGameStats(ctx, f.gameID)
	require.NoError(t, err)

	assert.Equal(t, int64(2), stats.Comments.TotalComments,
		"the GM topic post must not be counted as a comment")
	assert.Equal(t, int64(15), stats.Comments.AvgCommentLength,
		"average must be the mean comment length (10 and 20)")
	assert.Equal(t, int64(20), stats.Comments.LongestCommentLength)
	assert.Equal(t, int64(2), stats.Comments.MaxThreadDepth,
		"a reply nested under a top-level comment must register as depth 2")
	assert.InDelta(t, 1.5, stats.Comments.AvgThreadDepth, 0.001)

	t.Run("excludes soft-deleted and draft comments", func(t *testing.T) {
		// A 100-char comment that is deleted, and another that is a draft.
		// Either leaking in would move the average well off 15.
		insertComment(t, testDB, f, char, f.postID, strings.Repeat("c", 100))
		_, err := testDB.Pool.Exec(ctx,
			`UPDATE messages SET is_deleted = TRUE WHERE content = $1`, strings.Repeat("c", 100))
		require.NoError(t, err)

		insertComment(t, testDB, f, char, f.postID, strings.Repeat("d", 100))
		_, err = testDB.Pool.Exec(ctx,
			`UPDATE messages SET is_draft = TRUE WHERE content = $1`, strings.Repeat("d", 100))
		require.NoError(t, err)

		stats, err := svc.GetGameStats(ctx, f.gameID)
		require.NoError(t, err)
		assert.Equal(t, int64(2), stats.Comments.TotalComments,
			"deleted and draft comments must not be counted")
		assert.Equal(t, int64(15), stats.Comments.AvgCommentLength,
			"deleted and draft comments must not affect the average")
	})
}

// TestGameStatsService_LongestComment verifies the deep-link target identifies
// the actual longest comment and is absent when there are no comments.
func TestGameStatsService_LongestComment(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "private_messages", "conversations", "messages", "characters", "games", "users")

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	svc := &GameStatsService{DB: testDB.Pool, Logger: app.ObsLogger}

	f := newGameStatsFixture(t, testDB, "longcomment")

	t.Run("nil when the game has no comments", func(t *testing.T) {
		stats, err := svc.GetGameStats(ctx, f.gameID)
		require.NoError(t, err)
		assert.Nil(t, stats.LongestComment)
	})

	char := mustCharacterID(t, testDB, f.gameID, f.gmID, "Wordy")
	insertComment(t, testDB, f, char, f.postID, strings.Repeat("a", 30))
	winner := insertComment(t, testDB, f, char, f.postID, strings.Repeat("b", 200))
	insertComment(t, testDB, f, char, f.postID, strings.Repeat("c", 50))

	t.Run("identifies the longest comment", func(t *testing.T) {
		stats, err := svc.GetGameStats(ctx, f.gameID)
		require.NoError(t, err)
		require.NotNil(t, stats.LongestComment)

		assert.Equal(t, winner, stats.LongestComment.MessageID,
			"must point at the actual longest comment, not merely report its length")
		assert.Equal(t, int64(200), stats.LongestComment.ContentLength)
		assert.Equal(t, "Wordy", stats.LongestComment.CharacterName)
		assert.Equal(t, int64(200), stats.Comments.LongestCommentLength,
			"the aggregate and the deep-link target must agree")
	})

	t.Run("ignores deleted comments when picking the longest", func(t *testing.T) {
		// A longer comment that is soft-deleted must not win.
		insertComment(t, testDB, f, char, f.postID, strings.Repeat("d", 500))
		_, err := testDB.Pool.Exec(ctx,
			`UPDATE messages SET is_deleted = TRUE WHERE content = $1`, strings.Repeat("d", 500))
		require.NoError(t, err)

		stats, err := svc.GetGameStats(ctx, f.gameID)
		require.NoError(t, err)
		require.NotNil(t, stats.LongestComment)
		assert.Equal(t, winner, stats.LongestComment.MessageID,
			"a deleted comment must not become the longest")
	})
}

// TestGameStatsService_PerCharacterNoFanOut is the regression test for the
// Cartesian-product bug that arises from joining messages and private_messages
// directly onto characters in one query: a character with 2 comments and 3 PMs
// would report 6 of each. Every real player has both kinds of activity, so this
// would corrupt the entire per-character table.
func TestGameStatsService_PerCharacterNoFanOut(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "private_messages", "conversations", "messages", "characters", "games", "users")

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	svc := &GameStatsService{DB: testDB.Pool, Logger: app.ObsLogger}

	f := newGameStatsFixture(t, testDB, "fanout")
	char := mustCharacterID(t, testDB, f.gameID, f.gmID, "BusyChar")

	// 2 comments of length 10 and 20 (avg 15).
	insertComment(t, testDB, f, char, f.postID, strings.Repeat("a", 10))
	insertComment(t, testDB, f, char, f.postID, strings.Repeat("b", 20))

	// 3 PMs of length 4, 6, 8 (avg 6).
	convID := insertConversation(t, testDB, f.gameID, f.gmID, "direct", "")
	insertPM(t, testDB, convID, f.gmID, char, strings.Repeat("x", 4))
	insertPM(t, testDB, convID, f.gmID, char, strings.Repeat("y", 6))
	insertPM(t, testDB, convID, f.gmID, char, strings.Repeat("z", 8))

	stats, err := svc.GetGameStats(ctx, f.gameID)
	require.NoError(t, err)

	var busy *core.CharacterWriting
	for i := range stats.Characters {
		if stats.Characters[i].CharacterID == char {
			busy = &stats.Characters[i]
		}
	}
	require.NotNil(t, busy, "the character must appear in the per-character stats")

	// Under the fan-out bug these would be 6 and 6 rather than 2 and 3.
	assert.Equal(t, int64(2), busy.CommentCount,
		"comment count must not be multiplied by the number of private messages")
	assert.Equal(t, int64(3), busy.PrivateMessageCount,
		"private message count must not be multiplied by the number of comments")
	assert.Equal(t, int64(15), busy.AvgCommentLength)
	assert.Equal(t, int64(6), busy.AvgPrivateMessageLength)
}

// TestGameStatsService_IncludesInactiveCharactersWithZeroActivity verifies the
// roster is complete: characters who never wrote anything still appear, so the
// frontend does not need a separate character list to render the table.
func TestGameStatsService_IncludesInactiveCharactersWithZeroActivity(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "private_messages", "conversations", "messages", "characters", "games", "users")

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	svc := &GameStatsService{DB: testDB.Pool, Logger: app.ObsLogger}

	f := newGameStatsFixture(t, testDB, "silent")
	silent := mustCharacterID(t, testDB, f.gameID, f.gmID, "SilentChar")

	stats, err := svc.GetGameStats(ctx, f.gameID)
	require.NoError(t, err)

	var found *core.CharacterWriting
	for i := range stats.Characters {
		if stats.Characters[i].CharacterID == silent {
			found = &stats.Characters[i]
		}
	}
	require.NotNil(t, found, "a character with no activity must still be listed")
	assert.Equal(t, int64(0), found.CommentCount)
	assert.Equal(t, int64(0), found.AvgCommentLength)
	assert.Equal(t, int64(0), found.PrivateMessageCount)
	assert.Equal(t, "SilentChar", found.CharacterName)
}

// TestGameStatsService_LongestConversation verifies the conversation with the
// most messages wins, that participant names come back, and that an empty game
// yields nil rather than an error.
func TestGameStatsService_LongestConversation(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "conversation_participants", "private_messages", "conversations", "messages", "characters", "games", "users")

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	svc := &GameStatsService{DB: testDB.Pool, Logger: app.ObsLogger}

	f := newGameStatsFixture(t, testDB, "convo")

	t.Run("nil when the game has no conversations", func(t *testing.T) {
		stats, err := svc.GetGameStats(ctx, f.gameID)
		require.NoError(t, err, "a game with no conversations must not be an error")
		assert.Nil(t, stats.LongestConversation)
	})

	alice := mustCharacterID(t, testDB, f.gameID, f.gmID, "Alice")
	bruno := mustCharacterID(t, testDB, f.gameID, f.gmID, "Bruno")

	// Short conversation: 1 message.
	shortConv := insertConversation(t, testDB, f.gameID, f.gmID, "direct", "Short")
	insertPM(t, testDB, shortConv, f.gmID, alice, "hello")

	// Long conversation: 3 messages, with both characters as participants.
	longConv := insertConversation(t, testDB, f.gameID, f.gmID, "direct", "The Long One")
	insertPM(t, testDB, longConv, f.gmID, alice, "one")
	insertPM(t, testDB, longConv, f.gmID, bruno, "two")
	insertPM(t, testDB, longConv, f.gmID, alice, "three")
	for _, c := range []int32{alice, bruno} {
		_, err := testDB.Pool.Exec(ctx, `
			INSERT INTO conversation_participants (conversation_id, user_id, character_id)
			VALUES ($1, $2, $3)
		`, longConv, f.gmID, c)
		require.NoError(t, err)
	}

	t.Run("picks the conversation with the most messages", func(t *testing.T) {
		stats, err := svc.GetGameStats(ctx, f.gameID)
		require.NoError(t, err)
		require.NotNil(t, stats.LongestConversation)

		assert.Equal(t, longConv, stats.LongestConversation.ConversationID)
		assert.Equal(t, int64(3), stats.LongestConversation.MessageCount)
		assert.Equal(t, "The Long One", stats.LongestConversation.Title)
		assert.ElementsMatch(t, []string{"Alice", "Bruno"},
			stats.LongestConversation.ParticipantNames)
		require.NotNil(t, stats.LongestConversation.FirstMessageAt)
		require.NotNil(t, stats.LongestConversation.LastMessageAt)
	})

	t.Run("soft-deleted messages do not count toward length", func(t *testing.T) {
		// Delete two of the long conversation's three messages, dropping it to
		// 1 message and tying with the short one.
		_, err := testDB.Pool.Exec(ctx, `
			UPDATE private_messages SET is_deleted = TRUE
			WHERE conversation_id = $1 AND content IN ('two', 'three')
		`, longConv)
		require.NoError(t, err)

		stats, err := svc.GetGameStats(ctx, f.gameID)
		require.NoError(t, err)
		require.NotNil(t, stats.LongestConversation)
		assert.Equal(t, int64(1), stats.LongestConversation.MessageCount,
			"deleted private messages must not count toward conversation length")
	})
}

// TestGameStatsService_EmptyGame verifies a game with no content returns a
// zeroed payload rather than nulls or an error, so the UI can render it.
func TestGameStatsService_EmptyGame(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "private_messages", "conversations", "messages", "characters", "games", "users")

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	svc := &GameStatsService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "gstats_empty", "gstats_empty@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Empty Stats Game")

	stats, err := svc.GetGameStats(ctx, game.ID)
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, int64(0), stats.Comments.TotalComments)
	assert.Equal(t, int64(0), stats.Comments.AvgCommentLength)
	assert.Equal(t, float64(0), stats.Comments.AvgThreadDepth)
	assert.Equal(t, int64(0), stats.PrivateMessages.TotalPrivateMessages)
	assert.Equal(t, int64(0), stats.PrivateMessages.ActiveConversations)
	assert.Nil(t, stats.LongestConversation)
	assert.NotNil(t, stats.Characters, "characters must serialize as [] not null")
	assert.Empty(t, stats.Characters)
}
