package messages

import (
	"context"
	"fmt"
	"testing"
	"time"

	core "actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	db "actionphase/pkg/db/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetPostCommentsReplyCountIsDescendantCount pins the semantics of
// reply_count on the direct-children endpoint (GetPostComments).
//
// Regression: the "Continue this thread" link renders comment.reply_count as a
// preview of how much thread is hidden behind it. The main Common Room view is
// fed by GetPostCommentsWithThreads, whose reply_count is a *recursive
// descendant* count, so that first link showed the true size of the hidden
// subtree. Opening the thread modal re-fetches each level through
// GetPostComments, which counted only *direct* children. In a linear thread
// (A -> B -> C -> D, one reply each) every comment has exactly one direct
// child, so the second "Continue this thread" always read "(1 reply)" no matter
// how many comments were actually below it.
//
// Both endpoints must report the same number for the same comment.
func TestGetPostCommentsReplyCountIsDescendantCount(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	service := &MessageService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Reply Count Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	char, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player.ID)),
		Name:          "Reply Count Character",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	post, err := service.CreatePost(context.Background(), core.CreatePostRequest{
		GameID:      game.ID,
		AuthorID:    int32(player.ID),
		CharacterID: char.ID,
		Content:     "Post with a deep linear thread",
		Visibility:  string(models.MessageVisibilityGame),
	})
	require.NoError(t, err)

	// Build a linear chain of 6 comments hanging off the post:
	// post -> c0 -> c1 -> c2 -> c3 -> c4 -> c5
	// Each comment has exactly ONE direct child, but c0 has 5 descendants.
	const chainLen = 6
	chain := make([]int32, 0, chainLen)
	parentID := post.ID
	for i := 0; i < chainLen; i++ {
		time.Sleep(1 * time.Millisecond) // distinct created_at ordering
		comment, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
			GameID:      game.ID,
			AuthorID:    int32(player.ID),
			CharacterID: char.ID,
			ParentID:    parentID,
			Content:     fmt.Sprintf("Chain comment %d", i),
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)
		chain = append(chain, comment.ID)
		parentID = comment.ID
	}

	// Expected descendant count for chain[i] is the number of comments below it.
	expected := func(i int) int64 { return int64(chainLen - 1 - i) }

	t.Run("direct-children endpoint reports descendant counts", func(t *testing.T) {
		// Walk the chain the way the thread modal does: fetch each level's
		// children and read the child's reply_count off that response.
		for i := 0; i < chainLen-1; i++ {
			children, err := service.GetPostComments(context.Background(), chain[i])
			require.NoError(t, err)
			require.Len(t, children, 1, "chain comment %d should have exactly one direct child", i)

			child := children[0]
			assert.Equal(t, chain[i+1], child.ID)
			assert.Equal(t, expected(i+1), child.ReplyCount,
				"reply_count for chain comment %d must count all %d descendants, not just direct children",
				i+1, expected(i+1))
		}
	})

	t.Run("matches the threaded endpoint for the same comment", func(t *testing.T) {
		threaded, err := service.GetPostCommentsWithThreads(context.Background(), post.ID, 15, 0, 10)
		require.NoError(t, err)

		threadedCounts := make(map[int32]int64, len(threaded))
		for _, c := range threaded {
			threadedCounts[c.Comment.ID] = c.Comment.ReplyCount
		}

		direct, err := service.GetPostComments(context.Background(), post.ID)
		require.NoError(t, err)
		require.Len(t, direct, 1)

		require.Contains(t, threadedCounts, direct[0].ID)
		assert.Equal(t, threadedCounts[direct[0].ID], direct[0].ReplyCount,
			"both endpoints must agree on reply_count for the same comment")
	})

	t.Run("counts a branching subtree, not just the direct children", func(t *testing.T) {
		// Give the deepest comment two direct children, one of which has its own
		// child, so the branching case is covered too: 3 descendants, 2 direct.
		leaf := chain[chainLen-1]
		branchA, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
			GameID: game.ID, AuthorID: int32(player.ID), CharacterID: char.ID,
			ParentID: leaf, Content: "Branch A", Visibility: string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)
		time.Sleep(1 * time.Millisecond)
		_, err = service.CreateComment(context.Background(), core.CreateCommentRequest{
			GameID: game.ID, AuthorID: int32(player.ID), CharacterID: char.ID,
			ParentID: leaf, Content: "Branch B", Visibility: string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)
		time.Sleep(1 * time.Millisecond)
		_, err = service.CreateComment(context.Background(), core.CreateCommentRequest{
			GameID: game.ID, AuthorID: int32(player.ID), CharacterID: char.ID,
			ParentID: branchA.ID, Content: "Branch A child", Visibility: string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		children, err := service.GetPostComments(context.Background(), chain[chainLen-2])
		require.NoError(t, err)
		require.Len(t, children, 1)
		assert.Equal(t, int64(3), children[0].ReplyCount,
			"leaf now has 2 direct children and 3 total descendants")
	})
}
