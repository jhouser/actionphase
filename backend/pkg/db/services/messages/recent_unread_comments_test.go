package messages

import (
	"context"
	"fmt"
	"testing"

	core "actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	db "actionphase/pkg/db/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMessageService_ListRecentUnreadCommentsWithParents verifies the unread-only
// variant of the "New Comments" listing: it must omit comments the requesting user
// has manually marked as read, while remaining per-user and correctly paginated.
func TestMessageService_ListRecentUnreadCommentsWithParents(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	service := &MessageService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	ctx := context.Background()

	gm := testDB.CreateTestUser(t, "unreadgm", "unreadgm@example.com")
	reader := testDB.CreateTestUser(t, "reader", "reader@example.com")
	other := testDB.CreateTestUser(t, "otherreader", "otherreader@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Unread Filter Game")

	_, err := gameService.AddGameParticipant(ctx, game.ID, int32(reader.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(ctx, game.ID, int32(other.ID), "player")
	require.NoError(t, err)

	char, err := characterService.CreateCharacter(ctx, db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(reader.ID)),
		Name:          "Unread Test Character",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	// One post with three comments on it.
	post, err := service.CreatePost(ctx, core.CreatePostRequest{
		GameID:      game.ID,
		AuthorID:    int32(reader.ID),
		CharacterID: char.ID,
		Content:     "Root post",
		Visibility:  string(models.MessageVisibilityGame),
	})
	require.NoError(t, err)

	comments := make([]*models.Message, 0, 3)
	for i := 0; i < 3; i++ {
		c, err := service.CreateComment(ctx, core.CreateCommentRequest{
			GameID:      game.ID,
			AuthorID:    int32(reader.ID),
			CharacterID: char.ID,
			ParentID:    post.ID,
			Content:     fmt.Sprintf("Comment %d", i+1),
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)
		comments = append(comments, c)
	}

	contentsOf := func(list []core.CommentWithParent) []string {
		out := make([]string, len(list))
		for i, c := range list {
			out[i] = c.Content
		}
		return out
	}

	t.Run("returns all comments when none are marked read", func(t *testing.T) {
		got, err := service.ListRecentUnreadCommentsWithParents(ctx, game.ID, int32(reader.ID), 10, 0)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"Comment 1", "Comment 2", "Comment 3"}, contentsOf(got))

		count, err := service.GetTotalUnreadCommentCount(ctx, game.ID, int32(reader.ID))
		require.NoError(t, err)
		assert.Equal(t, int64(3), count)
	})

	t.Run("omits comments the user marked as read", func(t *testing.T) {
		require.NoError(t, service.ToggleCommentRead(ctx, int32(reader.ID), game.ID, post.ID, comments[1].ID, true))

		got, err := service.ListRecentUnreadCommentsWithParents(ctx, game.ID, int32(reader.ID), 10, 0)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"Comment 1", "Comment 3"}, contentsOf(got),
			"the manually-read comment must not appear")

		count, err := service.GetTotalUnreadCommentCount(ctx, game.ID, int32(reader.ID))
		require.NoError(t, err)
		assert.Equal(t, int64(2), count, "total count must reflect the filter")
	})

	t.Run("read state is per-user", func(t *testing.T) {
		// Comment 2 is read for `reader` (previous subtest) but not for `other`.
		got, err := service.ListRecentUnreadCommentsWithParents(ctx, game.ID, int32(other.ID), 10, 0)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"Comment 1", "Comment 2", "Comment 3"}, contentsOf(got),
			"another user's read marks must not filter this user's list")
	})

	t.Run("unmarking as read restores the comment", func(t *testing.T) {
		require.NoError(t, service.ToggleCommentRead(ctx, int32(reader.ID), game.ID, post.ID, comments[1].ID, false))

		got, err := service.ListRecentUnreadCommentsWithParents(ctx, game.ID, int32(reader.ID), 10, 0)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"Comment 1", "Comment 2", "Comment 3"}, contentsOf(got))
	})

	t.Run("filters before pagination so a full page of unread is returned", func(t *testing.T) {
		// Mark the two newest comments read; a limit of 1 must still return the
		// remaining unread comment rather than an empty page.
		require.NoError(t, service.ToggleCommentRead(ctx, int32(reader.ID), game.ID, post.ID, comments[2].ID, true))
		require.NoError(t, service.ToggleCommentRead(ctx, int32(reader.ID), game.ID, post.ID, comments[1].ID, true))

		got, err := service.ListRecentUnreadCommentsWithParents(ctx, game.ID, int32(reader.ID), 1, 0)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "Comment 1", got[0].Content)
	})

	t.Run("includes parent context and post_id like the unfiltered listing", func(t *testing.T) {
		got, err := service.ListRecentUnreadCommentsWithParents(ctx, game.ID, int32(reader.ID), 10, 0)
		require.NoError(t, err)
		require.NotEmpty(t, got)

		c := got[0]
		require.NotNil(t, c.ParentContent)
		assert.Equal(t, "Root post", *c.ParentContent)
		require.NotNil(t, c.ParentMessageType)
		assert.Equal(t, "post", *c.ParentMessageType)
		require.NotNil(t, c.PostID, "post_id drives the Mark as Read button")
		assert.Equal(t, post.ID, *c.PostID)
	})
}
