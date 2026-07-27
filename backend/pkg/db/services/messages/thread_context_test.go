package messages

import (
	"context"
	"testing"

	"actionphase/pkg/core"
	db "actionphase/pkg/db/services"
)

// TestGetMessageWithParentContext verifies the deep-link thread-context query:
// it returns the target plus a bounded slice of its nearest ancestors (ordered
// parent-to-child), resolves the true root post ID even when the slice is trimmed,
// and reports whether the returned chain reaches the root.
func TestGetMessageWithParentContext(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)
	fixtures := testDB.SetupFixtures(t)
	msgService := &MessageService{DB: testDB.Pool, Logger: app.ObsLogger}

	gameID := fixtures.TestGame.ID
	player := testDB.CreateTestUser(t, "player_threadctx", "player_threadctx@example.com")

	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	userID := int32(player.ID)
	character, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &userID,
		Name:          "ThreadCtxChar",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create character")

	// Build a chain: post -> c1 -> c2 -> c3 -> c4 -> c5 (target at thread_depth 5).
	post, err := msgService.CreatePost(context.Background(), core.CreatePostRequest{
		GameID:      gameID,
		AuthorID:    int32(player.ID),
		CharacterID: character.ID,
		Content:     "Root post",
		Visibility:  "game",
	})
	core.AssertNoError(t, err, "Failed to create post")

	parentID := post.ID
	ids := []int32{post.ID}
	for i := 1; i <= 5; i++ {
		comment, err := msgService.CreateComment(context.Background(), core.CreateCommentRequest{
			GameID:      gameID,
			ParentID:    parentID,
			AuthorID:    int32(player.ID),
			CharacterID: character.ID,
			Content:     "comment level " + string(rune('0'+i)),
			Visibility:  "game",
		})
		core.AssertNoError(t, err, "Failed to create comment")
		ids = append(ids, comment.ID)
		parentID = comment.ID
	}
	target := ids[len(ids)-1] // the deepest comment (thread_depth 5)
	rootID := post.ID

	t.Run("trims_to_target_plus_max_parents", func(t *testing.T) {
		ctx, err := msgService.GetMessageWithParentContext(context.Background(), target, 3)
		core.AssertNoError(t, err, "Should retrieve thread context")

		// target + 3 nearest ancestors = 4 messages
		core.AssertEqual(t, 4, len(ctx.Chain), "Chain should be target + 3 parents")
		// Ordered parent-to-child: shallowest first, target last
		core.AssertEqual(t, ids[2], ctx.Chain[0].Message.ID, "First should be the shallowest included ancestor")
		core.AssertEqual(t, target, ctx.Chain[len(ctx.Chain)-1].Message.ID, "Last should be the target")
		// Root post ID resolved even though the chain was trimmed above it
		core.AssertEqual(t, rootID, ctx.RootPostID, "RootPostID should be the true root post")
		core.AssertTrue(t, !ctx.HasFullThread, "Chain does not reach root, so HasFullThread is false")
	})

	t.Run("max_parents_zero_returns_target_only", func(t *testing.T) {
		ctx, err := msgService.GetMessageWithParentContext(context.Background(), target, 0)
		core.AssertNoError(t, err, "Should retrieve thread context")

		core.AssertEqual(t, 1, len(ctx.Chain), "Chain should be just the target")
		core.AssertEqual(t, target, ctx.Chain[0].Message.ID, "Only element should be the target")
		core.AssertEqual(t, rootID, ctx.RootPostID, "RootPostID should still resolve to the root")
		core.AssertTrue(t, !ctx.HasFullThread, "Target is not the root, so HasFullThread is false")
	})

	t.Run("large_max_parents_reaches_root", func(t *testing.T) {
		ctx, err := msgService.GetMessageWithParentContext(context.Background(), target, 20)
		core.AssertNoError(t, err, "Should retrieve thread context")

		core.AssertEqual(t, 6, len(ctx.Chain), "Chain should include the full post->target chain")
		core.AssertEqual(t, rootID, ctx.Chain[0].Message.ID, "First element should be the root post")
		core.AssertEqual(t, rootID, ctx.RootPostID, "RootPostID should be the root post")
		core.AssertTrue(t, ctx.HasFullThread, "Chain reaches root, so HasFullThread is true")
	})

	t.Run("target_is_root_post", func(t *testing.T) {
		ctx, err := msgService.GetMessageWithParentContext(context.Background(), rootID, 3)
		core.AssertNoError(t, err, "Should retrieve thread context")

		core.AssertEqual(t, 1, len(ctx.Chain), "Chain should be just the post")
		core.AssertEqual(t, rootID, ctx.RootPostID, "RootPostID should be itself")
		core.AssertTrue(t, ctx.HasFullThread, "A post is its own full thread")
	})
}
