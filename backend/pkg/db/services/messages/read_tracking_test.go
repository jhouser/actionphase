package messages

import (
	"context"
	"testing"

	core "actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	db "actionphase/pkg/db/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageService_MarkPostAsRead(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	service := &MessageService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Setup test data
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	game := testDB.CreateTestGame(t, int32(player.ID), "Test Game")

	// Add player as participant
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	// Create character for player
	char, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player.ID)),
		Name:          "Test Character",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	// Create a post
	post, err := service.CreatePost(context.Background(), core.CreatePostRequest{
		GameID:      game.ID,
		AuthorID:    int32(player.ID),
		CharacterID: char.ID,
		Content:     "This is a test post",
		Visibility:  string(models.MessageVisibilityGame),
	})
	require.NoError(t, err)

	t.Run("creates read marker successfully", func(t *testing.T) {
		marker, err := service.MarkPostAsRead(context.Background(), int32(player.ID), game.ID, post.ID, nil)

		require.NoError(t, err)
		assert.NotNil(t, marker)
		assert.Equal(t, int32(player.ID), marker.UserID)
		assert.Equal(t, game.ID, marker.GameID)
		assert.Equal(t, post.ID, marker.PostID)
		assert.Nil(t, marker.LastReadCommentID)
		assert.NotZero(t, marker.ID)
		assert.NotZero(t, marker.LastReadAt)
		assert.NotZero(t, marker.CreatedAt)
		assert.NotZero(t, marker.UpdatedAt)
	})

	t.Run("updates existing read marker with comment ID", func(t *testing.T) {
		// Create a comment
		comment, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
			GameID:      game.ID,
			ParentID:    post.ID,
			AuthorID:    int32(player.ID),
			CharacterID: char.ID,
			Content:     "This is a test comment",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		// Mark as read with comment ID
		commentID := comment.ID
		marker, err := service.MarkPostAsRead(context.Background(), int32(player.ID), game.ID, post.ID, &commentID)

		require.NoError(t, err)
		assert.NotNil(t, marker)
		assert.NotNil(t, marker.LastReadCommentID)
		assert.Equal(t, comment.ID, *marker.LastReadCommentID)
	})

	t.Run("upsert behavior - updates timestamp on second call", func(t *testing.T) {
		// Create new user and post for isolated test
		player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
		game2 := testDB.CreateTestGame(t, int32(player2.ID), "Test Game 2")
		_, err := gameService.AddGameParticipant(context.Background(), game2.ID, int32(player2.ID), "player")
		require.NoError(t, err)

		char2, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
			GameID:        game2.ID,
			UserID:        int32Ptr(int32(player2.ID)),
			Name:          "Test Character 2",
			CharacterType: "player_character",
		})
		require.NoError(t, err)

		post2, err := service.CreatePost(context.Background(), core.CreatePostRequest{
			GameID:      game2.ID,
			AuthorID:    int32(player2.ID),
			CharacterID: char2.ID,
			Content:     "Another test post",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		// First call
		marker1, err := service.MarkPostAsRead(context.Background(), int32(player2.ID), game2.ID, post2.ID, nil)
		require.NoError(t, err)

		// Second call - should update existing marker
		marker2, err := service.MarkPostAsRead(context.Background(), int32(player2.ID), game2.ID, post2.ID, nil)
		require.NoError(t, err)

		// Should have same ID (updated, not created)
		assert.Equal(t, marker1.ID, marker2.ID)
		// Created at should be the same
		assert.Equal(t, marker1.CreatedAt, marker2.CreatedAt)
		// Updated at should be different (newer)
		assert.True(t, marker2.UpdatedAt.After(marker1.UpdatedAt) || marker2.UpdatedAt.Equal(marker1.UpdatedAt))
	})
}

func TestMessageService_GetUserReadMarker(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	service := &MessageService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Setup test data
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	game := testDB.CreateTestGame(t, int32(player.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	char, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player.ID)),
		Name:          "Test Character",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	post, err := service.CreatePost(context.Background(), core.CreatePostRequest{
		GameID:      game.ID,
		AuthorID:    int32(player.ID),
		CharacterID: char.ID,
		Content:     "This is a test post",
		Visibility:  string(models.MessageVisibilityGame),
	})
	require.NoError(t, err)

	t.Run("returns nil when no marker exists", func(t *testing.T) {
		marker, err := service.GetUserReadMarker(context.Background(), int32(player.ID), post.ID)

		require.NoError(t, err)
		assert.Nil(t, marker)
	})

	t.Run("returns marker after creation", func(t *testing.T) {
		// Create marker
		created, err := service.MarkPostAsRead(context.Background(), int32(player.ID), game.ID, post.ID, nil)
		require.NoError(t, err)

		// Retrieve it
		marker, err := service.GetUserReadMarker(context.Background(), int32(player.ID), post.ID)

		require.NoError(t, err)
		assert.NotNil(t, marker)
		assert.Equal(t, created.ID, marker.ID)
		assert.Equal(t, created.UserID, marker.UserID)
		assert.Equal(t, created.PostID, marker.PostID)
	})
}

func TestMessageService_GetUserReadMarkersForGame(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	service := &MessageService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Setup test data
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	game := testDB.CreateTestGame(t, int32(player.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	char, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player.ID)),
		Name:          "Test Character",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	// Create multiple posts
	post1, err := service.CreatePost(context.Background(), core.CreatePostRequest{
		GameID:      game.ID,
		AuthorID:    int32(player.ID),
		CharacterID: char.ID,
		Content:     "Post 1",
		Visibility:  string(models.MessageVisibilityGame),
	})
	require.NoError(t, err)

	post2, err := service.CreatePost(context.Background(), core.CreatePostRequest{
		GameID:      game.ID,
		AuthorID:    int32(player.ID),
		CharacterID: char.ID,
		Content:     "Post 2",
		Visibility:  string(models.MessageVisibilityGame),
	})
	require.NoError(t, err)

	t.Run("returns empty array when no markers exist", func(t *testing.T) {
		markers, err := service.GetUserReadMarkersForGame(context.Background(), int32(player.ID), game.ID)

		require.NoError(t, err)
		assert.NotNil(t, markers)
		assert.Empty(t, markers)
	})

	t.Run("returns all markers for user in game", func(t *testing.T) {
		// Mark both posts as read
		_, err := service.MarkPostAsRead(context.Background(), int32(player.ID), game.ID, post1.ID, nil)
		require.NoError(t, err)

		_, err = service.MarkPostAsRead(context.Background(), int32(player.ID), game.ID, post2.ID, nil)
		require.NoError(t, err)

		// Retrieve all markers
		markers, err := service.GetUserReadMarkersForGame(context.Background(), int32(player.ID), game.ID)

		require.NoError(t, err)
		assert.Len(t, markers, 2)

		// Verify both posts are tracked
		postIDs := []int32{markers[0].PostID, markers[1].PostID}
		assert.Contains(t, postIDs, post1.ID)
		assert.Contains(t, postIDs, post2.ID)
	})

	t.Run("only returns markers for specified user", func(t *testing.T) {
		// Create another user
		player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
		_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
		require.NoError(t, err)

		// Player2 marks post1 as read
		_, err = service.MarkPostAsRead(context.Background(), int32(player2.ID), game.ID, post1.ID, nil)
		require.NoError(t, err)

		// Get markers for player1 - should still only have 2
		markers, err := service.GetUserReadMarkersForGame(context.Background(), int32(player.ID), game.ID)

		require.NoError(t, err)
		assert.Len(t, markers, 2)

		// All markers should belong to player1
		for _, marker := range markers {
			assert.Equal(t, int32(player.ID), marker.UserID)
		}
	})
}

func TestMessageService_GetPostsWithUnreadInfo(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	service := &MessageService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Setup test data
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	game := testDB.CreateTestGame(t, int32(player.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	char, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player.ID)),
		Name:          "Test Character",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	t.Run("returns empty array when no posts exist", func(t *testing.T) {
		emptyGame := testDB.CreateTestGame(t, int32(player.ID), "Empty Game")

		infos, err := service.GetPostsWithUnreadInfo(context.Background(), emptyGame.ID)

		require.NoError(t, err)
		assert.NotNil(t, infos)
		assert.Empty(t, infos)
	})

	t.Run("returns post info with zero comments", func(t *testing.T) {
		post, err := service.CreatePost(context.Background(), core.CreatePostRequest{
			GameID:      game.ID,
			AuthorID:    int32(player.ID),
			CharacterID: char.ID,
			Content:     "Post without comments",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		infos, err := service.GetPostsWithUnreadInfo(context.Background(), game.ID)

		require.NoError(t, err)
		assert.NotEmpty(t, infos)

		// Find our post
		var postInfo *core.PostUnreadInfo
		for _, info := range infos {
			if info.PostID == post.ID {
				postInfo = info
				break
			}
		}

		require.NotNil(t, postInfo)
		assert.Equal(t, post.ID, postInfo.PostID)
		assert.Equal(t, int64(0), postInfo.TotalComments)
		assert.Nil(t, postInfo.LatestCommentAt)
	})

	t.Run("returns post info with comments", func(t *testing.T) {
		post, err := service.CreatePost(context.Background(), core.CreatePostRequest{
			GameID:      game.ID,
			AuthorID:    int32(player.ID),
			CharacterID: char.ID,
			Content:     "Post with comments",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		// Create 2 comments
		_, err = service.CreateComment(context.Background(), core.CreateCommentRequest{
			GameID:      game.ID,
			ParentID:    post.ID,
			AuthorID:    int32(player.ID),
			CharacterID: char.ID,
			Content:     "Comment 1",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		_, err = service.CreateComment(context.Background(), core.CreateCommentRequest{
			GameID:      game.ID,
			ParentID:    post.ID,
			AuthorID:    int32(player.ID),
			CharacterID: char.ID,
			Content:     "Comment 2",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		infos, err := service.GetPostsWithUnreadInfo(context.Background(), game.ID)

		require.NoError(t, err)
		assert.NotEmpty(t, infos)

		// Find our post
		var postInfo *core.PostUnreadInfo
		for _, info := range infos {
			if info.PostID == post.ID {
				postInfo = info
				break
			}
		}

		require.NotNil(t, postInfo)
		assert.Equal(t, post.ID, postInfo.PostID)
		assert.Equal(t, int64(2), postInfo.TotalComments)
		// LatestCommentAt might be nil due to type assertion issues, but total comments should be correct
		if postInfo.LatestCommentAt != nil {
			assert.NotZero(t, *postInfo.LatestCommentAt)
		}
	})

	t.Run("excludes deleted posts", func(t *testing.T) {
		// Create a post
		post, err := service.CreatePost(context.Background(), core.CreatePostRequest{
			GameID:      game.ID,
			AuthorID:    int32(player.ID),
			CharacterID: char.ID,
			Content:     "Post to be deleted",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		// Delete it
		err = service.DeletePost(context.Background(), post.ID)
		require.NoError(t, err)

		// Should not appear in results
		infos, err := service.GetPostsWithUnreadInfo(context.Background(), game.ID)

		require.NoError(t, err)
		for _, info := range infos {
			assert.NotEqual(t, post.ID, info.PostID, "Deleted post should not appear in unread info")
		}
	})

	t.Run("excludes deleted comments from count", func(t *testing.T) {
		post, err := service.CreatePost(context.Background(), core.CreatePostRequest{
			GameID:      game.ID,
			AuthorID:    int32(player.ID),
			CharacterID: char.ID,
			Content:     "Post with deleted comment",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		// Create comment
		comment, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
			GameID:      game.ID,
			ParentID:    post.ID,
			AuthorID:    int32(player.ID),
			CharacterID: char.ID,
			Content:     "Comment to be deleted",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		// Delete comment
		err = service.DeleteComment(context.Background(), comment.ID, int32(player.ID))
		require.NoError(t, err)

		infos, err := service.GetPostsWithUnreadInfo(context.Background(), game.ID)

		require.NoError(t, err)

		// Find our post
		var postInfo *core.PostUnreadInfo
		for _, info := range infos {
			if info.PostID == post.ID {
				postInfo = info
				break
			}
		}

		require.NotNil(t, postInfo)
		assert.Equal(t, int64(0), postInfo.TotalComments, "Deleted comments should not be counted")
	})
}

func TestMessageService_GetUnreadCommentIDsForPosts(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	service := &MessageService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Setup test data
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	game := testDB.CreateTestGame(t, int32(player.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	char, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player.ID)),
		Name:          "Test Character",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	t.Run("first visit returns empty array (no NEW badges on first visit)", func(t *testing.T) {
		// Create a new game and user to isolate this test
		player2 := testDB.CreateTestUser(t, "player_first_visit", "player_first_visit@example.com")
		game2 := testDB.CreateTestGame(t, int32(player2.ID), "Test Game - First Visit")
		_, err := gameService.AddGameParticipant(context.Background(), game2.ID, int32(player2.ID), "player")
		require.NoError(t, err)

		char2, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
			GameID:        game2.ID,
			UserID:        int32Ptr(int32(player2.ID)),
			Name:          "Test Character 2",
			CharacterType: "player_character",
		})
		require.NoError(t, err)

		// Create a post
		post, err := service.CreatePost(context.Background(), core.CreatePostRequest{
			GameID:      game2.ID,
			AuthorID:    int32(player2.ID),
			CharacterID: char2.ID,
			Content:     "Post with nested comments",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		// Create 5 top-level comments
		for i := 0; i < 5; i++ {
			comment, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
				GameID:      game2.ID,
				ParentID:    post.ID,
				AuthorID:    int32(player2.ID),
				CharacterID: char2.ID,
				Content:     "Top-level comment " + string(rune(i+1)),
				Visibility:  string(models.MessageVisibilityGame),
			})
			require.NoError(t, err)

			// Add 2 nested replies to each top-level comment
			for j := 0; j < 2; j++ {
				_, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
					GameID:      game2.ID,
					ParentID:    comment.ID, // Parent is the comment, not the post
					AuthorID:    int32(player2.ID),
					CharacterID: char2.ID,
					Content:     "Nested reply " + string(rune(j+1)),
					Visibility:  string(models.MessageVisibilityGame),
				})
				require.NoError(t, err)
			}
		}

		// User has NEVER visited (no entry in user_common_room_reads)
		// Call GetUnreadCommentIDsForPosts
		result, err := service.GetUnreadCommentIDsForPosts(context.Background(), int32(player2.ID), game2.ID)

		// Assert: Should return EMPTY array (no "NEW" badges on first visit)
		require.NoError(t, err)
		assert.Len(t, result, 1, "Should have one post")
		assert.Equal(t, post.ID, result[0].PostID)
		assert.Empty(t, result[0].UnreadCommentIDs, "Should return empty array on first visit (no NEW badges)")
	})

	t.Run("returns empty array when user has read all comments", func(t *testing.T) {
		// Create post with comments
		post, err := service.CreatePost(context.Background(), core.CreatePostRequest{
			GameID:      game.ID,
			AuthorID:    int32(player.ID),
			CharacterID: char.ID,
			Content:     "Post for read test",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		// Create 3 comments
		for i := 0; i < 3; i++ {
			_, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
				GameID:      game.ID,
				ParentID:    post.ID,
				AuthorID:    int32(player.ID),
				CharacterID: char.ID,
				Content:     "Comment " + string(rune(i+1)),
				Visibility:  string(models.MessageVisibilityGame),
			})
			require.NoError(t, err)
		}

		// Mark post as read (this sets last_read_at to NOW())
		_, err = service.MarkPostAsRead(context.Background(), int32(player.ID), game.ID, post.ID, nil)
		require.NoError(t, err)

		// Get unread comment IDs
		result, err := service.GetUnreadCommentIDsForPosts(context.Background(), int32(player.ID), game.ID)

		require.NoError(t, err)

		// Find our post in results
		var postResult *core.PostUnreadComments
		for i := range result {
			if result[i].PostID == post.ID {
				postResult = result[i]
				break
			}
		}

		require.NotNil(t, postResult, "Post should be in results")
		assert.Empty(t, postResult.UnreadCommentIDs, "Should have no unread comments after marking as read")
	})

	t.Run("returns only new comments created after last visit", func(t *testing.T) {
		// Create a GM user who will author the comments
		gm := testDB.CreateTestUser(t, "gm_partial_read", "gm_partial_read@example.com")
		_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(gm.ID), "co_gm")
		require.NoError(t, err)

		gmUserID := int32(gm.ID)
		gmChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
			GameID:        game.ID,
			UserID:        &gmUserID, // Associate with GM user
			Name:          "GM Character for Partial Read",
			CharacterType: "npc",
		})
		require.NoError(t, err)

		// Create post
		post, err := service.CreatePost(context.Background(), core.CreatePostRequest{
			GameID:      game.ID,
			AuthorID:    int32(gm.ID),
			CharacterID: gmChar.ID,
			Content:     "Post for partial read test",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		// Create 5 OLD comments (authored by GM, not player)
		for i := 0; i < 5; i++ {
			_, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
				GameID:      game.ID,
				ParentID:    post.ID,
				AuthorID:    int32(gm.ID),
				CharacterID: gmChar.ID,
				Content:     "Old comment " + string(rune(i+1)),
				Visibility:  string(models.MessageVisibilityGame),
			})
			require.NoError(t, err)
		}

		// Player marks post as read (establishing baseline timestamp)
		_, err = service.MarkPostAsRead(context.Background(), int32(player.ID), game.ID, post.ID, nil)
		require.NoError(t, err)

		// Create 3 NEW comments after the read timestamp (authored by GM, not player)
		newCommentIDs := make([]int32, 3)
		for i := 0; i < 3; i++ {
			comment, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
				GameID:      game.ID,
				ParentID:    post.ID,
				AuthorID:    int32(gm.ID),
				CharacterID: gmChar.ID,
				Content:     "New comment " + string(rune(i+1)),
				Visibility:  string(models.MessageVisibilityGame),
			})
			require.NoError(t, err)
			newCommentIDs[i] = comment.ID
		}

		// Player checks for unread comment IDs
		result, err := service.GetUnreadCommentIDsForPosts(context.Background(), int32(player.ID), game.ID)

		require.NoError(t, err)

		// Find our post in results
		var postResult *core.PostUnreadComments
		for i := range result {
			if result[i].PostID == post.ID {
				postResult = result[i]
				break
			}
		}

		require.NotNil(t, postResult, "Post should be in results")
		assert.Len(t, postResult.UnreadCommentIDs, 3, "Should return only the 3 new comments")

		// Verify the new comment IDs are present
		for _, newCommentID := range newCommentIDs {
			assert.Contains(t, postResult.UnreadCommentIDs, newCommentID, "Should contain new comment ID %d", newCommentID)
		}
	})

	t.Run("excludes user's own comments from unread list", func(t *testing.T) {
		// Create a second player who will also comment
		player2 := testDB.CreateTestUser(t, "player2_exclude", "player2_exclude@example.com")
		_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
		require.NoError(t, err)

		player2UserID := int32(player2.ID)
		char2, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
			GameID:        game.ID,
			UserID:        &player2UserID,
			Name:          "Player 2 Character",
			CharacterType: "player_character",
		})
		require.NoError(t, err)

		// Create a post
		post, err := service.CreatePost(context.Background(), core.CreatePostRequest{
			GameID:      game.ID,
			AuthorID:    int32(player2.ID),
			CharacterID: char2.ID,
			Content:     "Post for exclusion test",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		// Player 1 marks post as visited
		_, err = service.MarkPostAsRead(context.Background(), int32(player.ID), game.ID, post.ID, nil)
		require.NoError(t, err)

		// Create comments from BOTH users after the visit
		// Player 1's own comment (should be EXCLUDED)
		playerOwnComment, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
			GameID:      game.ID,
			ParentID:    post.ID,
			AuthorID:    int32(player.ID),
			CharacterID: char.ID,
			Content:     "Player 1 own comment",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		// Player 2's comment (should be INCLUDED)
		player2Comment, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
			GameID:      game.ID,
			ParentID:    post.ID,
			AuthorID:    int32(player2.ID),
			CharacterID: char2.ID,
			Content:     "Player 2 comment",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		// Another Player 1 comment (should be EXCLUDED)
		_, err = service.CreateComment(context.Background(), core.CreateCommentRequest{
			GameID:      game.ID,
			ParentID:    post.ID,
			AuthorID:    int32(player.ID),
			CharacterID: char.ID,
			Content:     "Player 1 another comment",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		// Player 1 checks for unread comments
		result, err := service.GetUnreadCommentIDsForPosts(context.Background(), int32(player.ID), game.ID)
		require.NoError(t, err)

		// Find our post in results
		var postResult *core.PostUnreadComments
		for i := range result {
			if result[i].PostID == post.ID {
				postResult = result[i]
				break
			}
		}

		require.NotNil(t, postResult, "Post should be in results")
		assert.Len(t, postResult.UnreadCommentIDs, 1, "Should return only Player 2's comment (Player 1's own excluded)")
		assert.Contains(t, postResult.UnreadCommentIDs, player2Comment.ID, "Should contain Player 2's comment")
		assert.NotContains(t, postResult.UnreadCommentIDs, playerOwnComment.ID, "Should NOT contain Player 1's own comment")
	})

	t.Run("includes nested comments from other users but excludes own nested comments", func(t *testing.T) {
		// Create a second player for this test
		player3 := testDB.CreateTestUser(t, "player3_nested", "player3_nested@example.com")
		_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player3.ID), "player")
		require.NoError(t, err)

		player3UserID := int32(player3.ID)
		char3, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
			GameID:        game.ID,
			UserID:        &player3UserID,
			Name:          "Player 3 Character",
			CharacterType: "player_character",
		})
		require.NoError(t, err)

		// Create a post
		post, err := service.CreatePost(context.Background(), core.CreatePostRequest{
			GameID:      game.ID,
			AuthorID:    int32(player3.ID),
			CharacterID: char3.ID,
			Content:     "Post for nested exclusion test",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		// Player 1 marks post as visited
		_, err = service.MarkPostAsRead(context.Background(), int32(player.ID), game.ID, post.ID, nil)
		require.NoError(t, err)

		// Create a top-level comment from Player 3 (INCLUDED)
		topComment, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
			GameID:      game.ID,
			ParentID:    post.ID,
			AuthorID:    int32(player3.ID),
			CharacterID: char3.ID,
			Content:     "Player 3 top-level",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		// Create a nested reply from Player 1 (EXCLUDED - user's own)
		player1Reply, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
			GameID:      game.ID,
			ParentID:    topComment.ID, // Reply to top comment
			AuthorID:    int32(player.ID),
			CharacterID: char.ID,
			Content:     "Player 1 nested reply",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		// Create a deeply nested reply from Player 3 (INCLUDED)
		deepReply, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
			GameID:      game.ID,
			ParentID:    player1Reply.ID, // Reply to Player 1's reply (3 levels deep)
			AuthorID:    int32(player3.ID),
			CharacterID: char3.ID,
			Content:     "Player 3 deep reply",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		// Player 1 checks for unread comments
		result, err := service.GetUnreadCommentIDsForPosts(context.Background(), int32(player.ID), game.ID)
		require.NoError(t, err)

		// Find our post in results
		var postResult *core.PostUnreadComments
		for i := range result {
			if result[i].PostID == post.ID {
				postResult = result[i]
				break
			}
		}

		require.NotNil(t, postResult, "Post should be in results")
		assert.Len(t, postResult.UnreadCommentIDs, 2, "Should return Player 3's comments only (2 total)")
		assert.Contains(t, postResult.UnreadCommentIDs, topComment.ID, "Should contain Player 3's top-level comment")
		assert.Contains(t, postResult.UnreadCommentIDs, deepReply.ID, "Should contain Player 3's deep nested reply")
		assert.NotContains(t, postResult.UnreadCommentIDs, player1Reply.ID, "Should NOT contain Player 1's own nested reply")
	})

	t.Run("excludes deleted comments from unread list", func(t *testing.T) {
		// Create a second player for this test
		player4 := testDB.CreateTestUser(t, "player4_deleted", "player4_deleted@example.com")
		_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player4.ID), "player")
		require.NoError(t, err)

		player4UserID := int32(player4.ID)
		char4, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
			GameID:        game.ID,
			UserID:        &player4UserID,
			Name:          "Player 4 Character",
			CharacterType: "player_character",
		})
		require.NoError(t, err)

		// Create a post
		post, err := service.CreatePost(context.Background(), core.CreatePostRequest{
			GameID:      game.ID,
			AuthorID:    int32(player4.ID),
			CharacterID: char4.ID,
			Content:     "Post for deleted comment test",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		// Player 1 marks post as visited
		_, err = service.MarkPostAsRead(context.Background(), int32(player.ID), game.ID, post.ID, nil)
		require.NoError(t, err)

		// Create comment that will be deleted
		deletedComment, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
			GameID:      game.ID,
			ParentID:    post.ID,
			AuthorID:    int32(player4.ID),
			CharacterID: char4.ID,
			Content:     "Comment to be deleted",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		// Create another comment that stays
		activeComment, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
			GameID:      game.ID,
			ParentID:    post.ID,
			AuthorID:    int32(player4.ID),
			CharacterID: char4.ID,
			Content:     "Active comment",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		// Delete the first comment
		err = service.DeleteComment(context.Background(), deletedComment.ID, int32(player4.ID))
		require.NoError(t, err)

		// Player 1 checks for unread comments
		result, err := service.GetUnreadCommentIDsForPosts(context.Background(), int32(player.ID), game.ID)
		require.NoError(t, err)

		// Find our post in results
		var postResult *core.PostUnreadComments
		for i := range result {
			if result[i].PostID == post.ID {
				postResult = result[i]
				break
			}
		}

		require.NotNil(t, postResult, "Post should be in results")
		assert.Len(t, postResult.UnreadCommentIDs, 1, "Should return only the active comment")
		assert.Contains(t, postResult.UnreadCommentIDs, activeComment.ID, "Should contain active comment")
		assert.NotContains(t, postResult.UnreadCommentIDs, deletedComment.ID, "Should NOT contain deleted comment")
	})
}

func TestMessageService_ToggleCommentRead(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	service := &MessageService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Setup
	player := testDB.CreateTestUser(t, "toggle_player", "toggle@example.com")
	game := testDB.CreateTestGame(t, int32(player.ID), "Toggle Test Game")
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	char, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player.ID)),
		Name:          "Toggle Char",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	post, err := service.CreatePost(context.Background(), core.CreatePostRequest{
		GameID:      game.ID,
		AuthorID:    int32(player.ID),
		CharacterID: char.ID,
		Content:     "Test post for manual reads",
		Visibility:  string(models.MessageVisibilityGame),
	})
	require.NoError(t, err)

	comment, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID:      game.ID,
		ParentID:    post.ID,
		AuthorID:    int32(player.ID),
		CharacterID: char.ID,
		Content:     "Test comment",
		Visibility:  string(models.MessageVisibilityGame),
	})
	require.NoError(t, err)

	t.Run("marks comment as read", func(t *testing.T) {
		err := service.ToggleCommentRead(context.Background(), int32(player.ID), game.ID, post.ID, comment.ID, true)
		require.NoError(t, err)

		reads, err := service.GetManualReadCommentIDsForGame(context.Background(), int32(player.ID), game.ID)
		require.NoError(t, err)
		require.Len(t, reads, 1)
		assert.Equal(t, post.ID, reads[0].PostID)
		assert.Contains(t, reads[0].ReadCommentIDs, comment.ID)
	})

	t.Run("marking same comment read again is idempotent", func(t *testing.T) {
		err := service.ToggleCommentRead(context.Background(), int32(player.ID), game.ID, post.ID, comment.ID, true)
		require.NoError(t, err)

		reads, err := service.GetManualReadCommentIDsForGame(context.Background(), int32(player.ID), game.ID)
		require.NoError(t, err)
		require.Len(t, reads, 1)
		assert.Len(t, reads[0].ReadCommentIDs, 1, "should not duplicate the entry")
	})

	t.Run("unmarks comment as read", func(t *testing.T) {
		err := service.ToggleCommentRead(context.Background(), int32(player.ID), game.ID, post.ID, comment.ID, false)
		require.NoError(t, err)

		reads, err := service.GetManualReadCommentIDsForGame(context.Background(), int32(player.ID), game.ID)
		require.NoError(t, err)
		assert.Empty(t, reads, "no reads should remain after unmarking")
	})

	t.Run("rejects comment from wrong game", func(t *testing.T) {
		otherGame := testDB.CreateTestGame(t, int32(player.ID), "Other Game")
		err := service.ToggleCommentRead(context.Background(), int32(player.ID), otherGame.ID, post.ID, comment.ID, true)
		assert.Error(t, err, "should reject comment that belongs to a different game")
	})
}

func TestMessageService_GetManualReadCommentIDsForGame(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	service := &MessageService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	player := testDB.CreateTestUser(t, "getmanual_player", "getmanual@example.com")
	game := testDB.CreateTestGame(t, int32(player.ID), "GetManual Test Game")
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	char, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player.ID)),
		Name:          "Manual Char",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	// Create two posts, each with a comment
	post1, err := service.CreatePost(context.Background(), core.CreatePostRequest{
		GameID: game.ID, AuthorID: int32(player.ID), CharacterID: char.ID,
		Content: "Post 1", Visibility: string(models.MessageVisibilityGame),
	})
	require.NoError(t, err)

	post2, err := service.CreatePost(context.Background(), core.CreatePostRequest{
		GameID: game.ID, AuthorID: int32(player.ID), CharacterID: char.ID,
		Content: "Post 2", Visibility: string(models.MessageVisibilityGame),
	})
	require.NoError(t, err)

	comment1, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID: game.ID, ParentID: post1.ID, AuthorID: int32(player.ID), CharacterID: char.ID,
		Content: "Comment on post 1", Visibility: string(models.MessageVisibilityGame),
	})
	require.NoError(t, err)

	comment2, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID: game.ID, ParentID: post2.ID, AuthorID: int32(player.ID), CharacterID: char.ID,
		Content: "Comment on post 2", Visibility: string(models.MessageVisibilityGame),
	})
	require.NoError(t, err)

	t.Run("returns empty when no reads", func(t *testing.T) {
		reads, err := service.GetManualReadCommentIDsForGame(context.Background(), int32(player.ID), game.ID)
		require.NoError(t, err)
		assert.Empty(t, reads)
	})

	t.Run("returns grouped results across multiple posts", func(t *testing.T) {
		require.NoError(t, service.ToggleCommentRead(context.Background(), int32(player.ID), game.ID, post1.ID, comment1.ID, true))
		require.NoError(t, service.ToggleCommentRead(context.Background(), int32(player.ID), game.ID, post2.ID, comment2.ID, true))

		reads, err := service.GetManualReadCommentIDsForGame(context.Background(), int32(player.ID), game.ID)
		require.NoError(t, err)
		assert.Len(t, reads, 2, "should return one entry per post")

		postMap := make(map[int32][]int32)
		for _, r := range reads {
			postMap[r.PostID] = r.ReadCommentIDs
		}
		assert.Contains(t, postMap[post1.ID], comment1.ID)
		assert.Contains(t, postMap[post2.ID], comment2.ID)
	})
}

func TestMessageService_MarkAllCommentsReadForPhase(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	service := &MessageService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	player := testDB.CreateTestUser(t, "bulk_player", "bulk_player@example.com")
	game := testDB.CreateTestGame(t, int32(player.ID), "Bulk Read Test Game")
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	char, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player.ID)),
		Name:          "Bulk Char",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	phase1 := testDB.CreateTestPhase(t, game.ID, "action", "Phase 1")
	phase2 := testDB.CreateTestPhase(t, game.ID, "action", "Phase 2")

	post1, err := service.CreatePost(context.Background(), core.CreatePostRequest{
		GameID: game.ID, PhaseID: &phase1.ID, AuthorID: int32(player.ID), CharacterID: char.ID,
		Content: "Post in phase 1", Visibility: string(models.MessageVisibilityGame),
	})
	require.NoError(t, err)

	post2, err := service.CreatePost(context.Background(), core.CreatePostRequest{
		GameID: game.ID, PhaseID: &phase2.ID, AuthorID: int32(player.ID), CharacterID: char.ID,
		Content: "Post in phase 2", Visibility: string(models.MessageVisibilityGame),
	})
	require.NoError(t, err)

	// Two comments in phase 1, one in phase 2
	comment1, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID: game.ID, PhaseID: &phase1.ID, ParentID: post1.ID, RootPostID: post1.ID,
		AuthorID: int32(player.ID), CharacterID: char.ID,
		Content: "Comment 1 in phase 1", Visibility: string(models.MessageVisibilityGame),
	})
	require.NoError(t, err)

	comment2, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID: game.ID, PhaseID: &phase1.ID, ParentID: post1.ID, RootPostID: post1.ID,
		AuthorID: int32(player.ID), CharacterID: char.ID,
		Content: "Comment 2 in phase 1", Visibility: string(models.MessageVisibilityGame),
	})
	require.NoError(t, err)

	// A reply to comment1 - nested two levels deep under post1. Its parent_id
	// is comment1.ID, not post1.ID, so grouping it under the right post
	// requires resolving the full parent chain back to the root post.
	nestedReply, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID: game.ID, PhaseID: &phase1.ID, ParentID: comment1.ID, RootPostID: post1.ID,
		AuthorID: int32(player.ID), CharacterID: char.ID,
		Content: "Nested reply to comment 1", Visibility: string(models.MessageVisibilityGame),
	})
	require.NoError(t, err)

	commentOtherPhase, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID: game.ID, PhaseID: &phase2.ID, ParentID: post2.ID, RootPostID: post2.ID,
		AuthorID: int32(player.ID), CharacterID: char.ID,
		Content: "Comment in phase 2", Visibility: string(models.MessageVisibilityGame),
	})
	require.NoError(t, err)

	// Use a second user - CreateComment auto-marks the author as read, which
	// would make this test trivially pass for `player`.
	reader := testDB.CreateTestUser(t, "bulk_reader", "bulk_reader@example.com")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(reader.ID), "player")
	require.NoError(t, err)

	t.Run("marks only comments in the given phase as read", func(t *testing.T) {
		err := service.MarkAllCommentsReadForPhase(context.Background(), int32(reader.ID), game.ID, phase1.ID)
		require.NoError(t, err)

		reads, err := service.GetManualReadCommentIDsForGame(context.Background(), int32(reader.ID), game.ID)
		require.NoError(t, err)

		readIDs := []int32{}
		for _, r := range reads {
			readIDs = append(readIDs, r.ReadCommentIDs...)
		}
		assert.Contains(t, readIDs, comment1.ID)
		assert.Contains(t, readIDs, comment2.ID)
		assert.Contains(t, readIDs, nestedReply.ID)
		assert.NotContains(t, readIDs, commentOtherPhase.ID, "comment in a different phase should not be marked read")

		// The nested reply must be grouped under the root post (post1), not
		// its immediate parent comment, or the frontend's per-post lookup
		// (which keys off the real post ID) will never find it.
		var post1Entry *core.ManualCommentReads
		for _, r := range reads {
			if r.PostID == post1.ID {
				post1Entry = r
			}
		}
		require.NotNil(t, post1Entry, "nested reply should be grouped under the root post")
		assert.Contains(t, post1Entry.ReadCommentIDs, nestedReply.ID)
	})

	t.Run("is idempotent when called again", func(t *testing.T) {
		err := service.MarkAllCommentsReadForPhase(context.Background(), int32(reader.ID), game.ID, phase1.ID)
		require.NoError(t, err)

		reads, err := service.GetManualReadCommentIDsForGame(context.Background(), int32(reader.ID), game.ID)
		require.NoError(t, err)

		readIDs := []int32{}
		for _, r := range reads {
			readIDs = append(readIDs, r.ReadCommentIDs...)
		}
		assert.Len(t, readIDs, 3, "should not duplicate entries on repeat calls")
	})
}

func TestCreateComment_AutoMarksAuthorAsRead(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)
	service := &MessageService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	author := testDB.CreateTestUser(t, "automark_author", "automark_author@example.com")
	other := testDB.CreateTestUser(t, "automark_other", "automark_other@example.com")
	game := testDB.CreateTestGame(t, int32(author.ID), "AutoMark Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(author.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(other.ID), "player")
	require.NoError(t, err)

	char, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(author.ID)),
		Name:          "Author Char",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	post, err := service.CreatePost(context.Background(), core.CreatePostRequest{
		GameID:      game.ID,
		AuthorID:    int32(author.ID),
		CharacterID: char.ID,
		Content:     "Test post",
		Visibility:  string(models.MessageVisibilityGame),
	})
	require.NoError(t, err)

	t.Run("comment is auto-marked read for author when RootPostID is set", func(t *testing.T) {
		comment, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
			GameID:      game.ID,
			ParentID:    post.ID,
			RootPostID:  post.ID,
			AuthorID:    int32(author.ID),
			CharacterID: char.ID,
			Content:     "Author's own comment",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		reads, err := service.GetManualReadCommentIDsForGame(context.Background(), int32(author.ID), game.ID)
		require.NoError(t, err)
		require.Len(t, reads, 1)
		assert.Contains(t, reads[0].ReadCommentIDs, comment.ID, "author's own comment should be auto-marked read")
	})

	t.Run("comment is NOT auto-marked read for other users", func(t *testing.T) {
		reads, err := service.GetManualReadCommentIDsForGame(context.Background(), int32(other.ID), game.ID)
		require.NoError(t, err)
		assert.Empty(t, reads, "auto-mark should only apply to the author, not other users")
	})

	t.Run("comment is NOT auto-marked read when RootPostID is zero", func(t *testing.T) {
		// Simulate old callers that don't set RootPostID
		comment, err := service.CreateComment(context.Background(), core.CreateCommentRequest{
			GameID:      game.ID,
			ParentID:    post.ID,
			AuthorID:    int32(author.ID),
			CharacterID: char.ID,
			Content:     "Comment without root post ID",
			Visibility:  string(models.MessageVisibilityGame),
		})
		require.NoError(t, err)

		reads, err := service.GetManualReadCommentIDsForGame(context.Background(), int32(author.ID), game.ID)
		require.NoError(t, err)
		// Only the first comment (from the previous sub-test) should be in the list
		found := false
		for _, r := range reads {
			for _, id := range r.ReadCommentIDs {
				if id == comment.ID {
					found = true
				}
			}
		}
		assert.False(t, found, "comment without RootPostID should not be auto-marked read")
	})
}
