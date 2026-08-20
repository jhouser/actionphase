package db

import (
	"context"
	"testing"

	"actionphase/pkg/core"
)

// =============================================================================
// HANDOUT CRUD TESTS
// =============================================================================

func TestHandoutService_CreateHandout(t *testing.T) {
	suite := NewTestSuite(t).
		WithTables("handouts", "games", "users").
		Setup()
	defer suite.Cleanup()

	// Create test data
	gm := suite.Factory().NewUser().WithUsername("testgm").Create()
	game := suite.Factory().NewGame().WithGM(gm.ID).Create()
	handoutService := suite.HandoutService()

	testCases := []struct {
		name        string
		gameID      int32
		title       string
		content     string
		status      string
		userID      int32
		expectError bool
	}{
		{
			name:        "create draft handout",
			gameID:      game.ID,
			title:       "Game Rules",
			content:     "# Basic Rules\n\nThis is markdown content.",
			status:      "draft",
			userID:      gm.ID,
			expectError: false,
		},
		{
			name:        "create published handout",
			gameID:      game.ID,
			title:       "World Lore",
			content:     "# The Kingdom\n\nAncient history...",
			status:      "published",
			userID:      gm.ID,
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handout, err := handoutService.CreateHandout(
				context.Background(),
				tc.gameID,
				tc.title,
				tc.content,
				tc.status,
				tc.userID,
			)

			if tc.expectError {
				core.AssertError(t, err, "Expected error for invalid handout creation")
				return
			}

			core.AssertNoError(t, err, "Failed to create handout")
			// Nil check removed - subsequent assertions will fail if nil
			core.AssertEqual(t, tc.title, handout.Title, "Title mismatch")
			core.AssertEqual(t, tc.content, handout.Content, "Content mismatch")
			core.AssertEqual(t, tc.status, handout.Status, "Status mismatch")
			core.AssertEqual(t, tc.gameID, handout.GameID, "Game ID mismatch")
		})
	}
}

func TestHandoutService_GetHandout(t *testing.T) {
	suite := NewTestSuite(t).
		WithTables("handouts", "games", "users").
		Setup()
	defer suite.Cleanup()

	// Create test data
	gm := suite.Factory().NewUser().WithUsername("testgm").Create()
	player := suite.Factory().NewUser().WithUsername("testplayer").Create()
	game := suite.Factory().NewGame().WithGM(gm.ID).Create()
	handoutService := suite.HandoutService()

	// Create a draft handout
	draftHandout, err := handoutService.CreateHandout(
		context.Background(),
		game.ID,
		"Draft Document",
		"Secret GM notes",
		"draft",
		gm.ID,
	)
	core.AssertNoError(t, err, "Failed to create draft handout")

	// Create a published handout
	publishedHandout, err := handoutService.CreateHandout(
		context.Background(),
		game.ID,
		"Published Document",
		"Public lore",
		"published",
		gm.ID,
	)
	core.AssertNoError(t, err, "Failed to create published handout")

	testCases := []struct {
		name        string
		handoutID   int32
		userID      int32
		expectError bool
	}{
		{
			name:        "GM can view draft handout",
			handoutID:   draftHandout.ID,
			userID:      gm.ID,
			expectError: false,
		},
		{
			name:        "GM can view published handout",
			handoutID:   publishedHandout.ID,
			userID:      gm.ID,
			expectError: false,
		},
		{
			name:        "player can view published handout",
			handoutID:   publishedHandout.ID,
			userID:      player.ID,
			expectError: false,
		},
		{
			name:        "player cannot view draft handout",
			handoutID:   draftHandout.ID,
			userID:      player.ID,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handout, err := handoutService.GetHandout(
				context.Background(),
				tc.handoutID,
				tc.userID,
			)

			if tc.expectError {
				core.AssertError(t, err, "Expected error for unauthorized access")
				return
			}

			core.AssertNoError(t, err, "Failed to get handout")
			// Nil check removed - subsequent assertions will fail if nil
			core.AssertEqual(t, tc.handoutID, handout.ID, "Handout ID mismatch")
		})
	}
}

func TestHandoutService_ListHandouts(t *testing.T) {
	suite := NewTestSuite(t).
		WithTables("handouts", "games", "users").
		Setup()
	defer suite.Cleanup()

	// Create test data
	gm := suite.Factory().NewUser().WithUsername("testgm").Create()
	player := suite.Factory().NewUser().WithUsername("testplayer").Create()
	game := suite.Factory().NewGame().WithGM(gm.ID).Create()
	handoutService := suite.HandoutService()

	// Create 2 draft and 2 published handouts
	_, err := handoutService.CreateHandout(context.Background(), game.ID, "Draft 1", "Content 1", "draft", gm.ID)
	core.AssertNoError(t, err, "Failed to create draft handout 1")

	_, err = handoutService.CreateHandout(context.Background(), game.ID, "Draft 2", "Content 2", "draft", gm.ID)
	core.AssertNoError(t, err, "Failed to create draft handout 2")

	_, err = handoutService.CreateHandout(context.Background(), game.ID, "Published 1", "Content 3", "published", gm.ID)
	core.AssertNoError(t, err, "Failed to create published handout 1")

	_, err = handoutService.CreateHandout(context.Background(), game.ID, "Published 2", "Content 4", "published", gm.ID)
	core.AssertNoError(t, err, "Failed to create published handout 2")

	testCases := []struct {
		name          string
		userID        int32
		isGM          bool
		expectedCount int
	}{
		{
			name:          "GM sees all handouts",
			userID:        gm.ID,
			isGM:          true,
			expectedCount: 4,
		},
		{
			name:          "player sees only published handouts",
			userID:        player.ID,
			isGM:          false,
			expectedCount: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handouts, err := handoutService.ListHandouts(
				context.Background(),
				game.ID,
				tc.userID,
				tc.isGM,
			)

			core.AssertNoError(t, err, "Failed to list handouts")
			core.AssertEqual(t, tc.expectedCount, len(handouts), "Handout count mismatch")

			// Verify all returned handouts belong to the correct game
			for _, h := range handouts {
				core.AssertEqual(t, game.ID, h.GameID, "Returned handout belongs to wrong game")
				// Non-GM should only see published handouts
				if !tc.isGM {
					core.AssertEqual(t, "published", h.Status, "Non-GM should only see published handouts")
				}
			}
		})
	}
}

func TestHandoutService_ListPublishedHandoutsAcrossGames(t *testing.T) {
	suite := NewTestSuite(t).
		WithTables("handout_comments", "handouts", "game_participants", "games", "users").
		Setup()
	defer suite.Cleanup()

	gm := suite.Factory().NewUser().WithUsername("acrossgm").Create()
	player := suite.Factory().NewUser().WithUsername("acrossplayer").Create()
	outsider := suite.Factory().NewUser().WithUsername("acrossoutsider").Create()

	// Two in_progress games the player belongs to, plus games that must not
	// contribute: one the player has no part in, and one that has finished.
	gameA := suite.Factory().NewGame().WithGM(gm.ID).WithTitle("Alpha Game").WithState("in_progress").Create()
	gameB := suite.Factory().NewGame().WithGM(gm.ID).WithTitle("Beta Game").WithState("in_progress").Create()
	otherGame := suite.Factory().NewGame().WithGM(outsider.ID).WithTitle("Not Mine").WithState("in_progress").Create()
	doneGame := suite.Factory().NewGame().WithGM(gm.ID).WithTitle("Finished Game").WithState("completed").Create()

	suite.Factory().NewGameParticipant().ForGame(gameA.ID).WithUser(player.ID).AsPlayer().Create()
	suite.Factory().NewGameParticipant().ForGame(gameB.ID).WithUser(player.ID).AsPlayer().Create()
	suite.Factory().NewGameParticipant().ForGame(doneGame.ID).WithUser(player.ID).AsPlayer().Create()

	handoutService := suite.HandoutService()
	ctx := context.Background()

	mustCreate := func(gameID int32, title, status string) {
		t.Helper()
		_, err := handoutService.CreateHandout(ctx, gameID, title, "Content of "+title, status, gm.ID)
		core.AssertNoError(t, err, "Failed to create handout "+title)
	}

	mustCreate(gameA.ID, "Alpha Published", "published")
	mustCreate(gameA.ID, "Alpha Draft", "draft")
	mustCreate(gameB.ID, "Beta Published", "published")
	mustCreate(otherGame.ID, "Outsider Published", "published")
	mustCreate(doneGame.ID, "Finished Published", "published")

	titlesOf := func(handouts []*core.HandoutWithGame) map[string]*core.HandoutWithGame {
		byTitle := make(map[string]*core.HandoutWithGame, len(handouts))
		for _, h := range handouts {
			byTitle[h.Title] = h
		}
		return byTitle
	}

	t.Run("player gets published handouts from their in_progress games only", func(t *testing.T) {
		handouts, err := handoutService.ListPublishedHandoutsAcrossGames(ctx, player.ID)
		core.AssertNoError(t, err, "Failed to list handouts across games")

		byTitle := titlesOf(handouts)
		core.AssertEqual(t, 2, len(handouts), "Expected exactly the two published handouts from joined active games")

		alpha, ok := byTitle["Alpha Published"]
		core.AssertTrue(t, ok, "Expected Alpha Published in results")
		if ok {
			// The game title is the whole reason this query exists: the drawer
			// groups by it and has no other way to resolve it.
			core.AssertEqual(t, "Alpha Game", alpha.GameTitle, "Game title should travel with the handout")
			core.AssertEqual(t, gameA.ID, alpha.GameID, "Handout should report its own game")
			core.AssertEqual(t, "Content of Alpha Published", alpha.Content, "Content should be returned for reading")
		}

		_, hasBeta := byTitle["Beta Published"]
		core.AssertTrue(t, hasBeta, "Expected Beta Published in results")

		_, hasDraft := byTitle["Alpha Draft"]
		core.AssertTrue(t, !hasDraft, "Drafts must not appear in the cross-game list")

		_, hasOutsider := byTitle["Outsider Published"]
		core.AssertTrue(t, !hasOutsider, "Handouts from games the user has not joined must not appear")

		_, hasFinished := byTitle["Finished Published"]
		core.AssertTrue(t, !hasFinished, "Handouts from completed games must not appear")
	})

	t.Run("GM sees published handouts but not their own drafts", func(t *testing.T) {
		handouts, err := handoutService.ListPublishedHandoutsAcrossGames(ctx, gm.ID)
		core.AssertNoError(t, err, "Failed to list handouts across games for GM")

		byTitle := titlesOf(handouts)
		core.AssertEqual(t, 2, len(handouts), "GM should get the published handouts of their active games")

		_, hasDraft := byTitle["Alpha Draft"]
		core.AssertTrue(t, !hasDraft, "The drawer is a reading surface; GM drafts stay on the Handouts tab")
	})

	t.Run("user with no games gets an empty list", func(t *testing.T) {
		loner := suite.Factory().NewUser().WithUsername("acrossloner").Create()

		handouts, err := handoutService.ListPublishedHandoutsAcrossGames(ctx, loner.ID)
		core.AssertNoError(t, err, "Failed to list handouts for user with no games")
		core.AssertEqual(t, 0, len(handouts), "A user in no games should receive no handouts")
	})
}

func TestHandoutService_UpdateHandout(t *testing.T) {
	suite := NewTestSuite(t).
		WithTables("handouts", "games", "users").
		Setup()
	defer suite.Cleanup()

	// Create test data
	gm := suite.Factory().NewUser().WithUsername("testgm").Create()
	game := suite.Factory().NewGame().WithGM(gm.ID).Create()
	handoutService := suite.HandoutService()

	// Create a handout
	handout, err := handoutService.CreateHandout(
		context.Background(),
		game.ID,
		"Original Title",
		"Original Content",
		"draft",
		gm.ID,
	)
	core.AssertNoError(t, err, "Failed to create handout")

	// Update the handout
	updated, err := handoutService.UpdateHandout(
		context.Background(),
		handout.ID,
		"Updated Title",
		"Updated Content",
		"published",
		gm.ID,
	)

	core.AssertNoError(t, err, "Failed to update handout")
	// Nil check removed - subsequent assertions will fail if nil
	core.AssertEqual(t, "Updated Title", updated.Title, "Title not updated")
	core.AssertEqual(t, "Updated Content", updated.Content, "Content not updated")
	core.AssertEqual(t, "published", updated.Status, "Status not updated")
}

func TestHandoutService_DeleteHandout(t *testing.T) {
	suite := NewTestSuite(t).
		WithTables("handouts", "games", "users").
		Setup()
	defer suite.Cleanup()

	// Create test data
	gm := suite.Factory().NewUser().WithUsername("testgm").Create()
	game := suite.Factory().NewGame().WithGM(gm.ID).Create()
	handoutService := suite.HandoutService()

	// Create a handout
	handout, err := handoutService.CreateHandout(
		context.Background(),
		game.ID,
		"To Be Deleted",
		"This will be deleted",
		"draft",
		gm.ID,
	)
	core.AssertNoError(t, err, "Failed to create handout")

	// Delete the handout
	err = handoutService.DeleteHandout(context.Background(), handout.ID, gm.ID)
	core.AssertNoError(t, err, "Failed to delete handout")

	// Verify handout is deleted
	_, err = handoutService.GetHandout(context.Background(), handout.ID, gm.ID)
	core.AssertError(t, err, "Should not be able to get deleted handout")
}

func TestHandoutService_PublishHandout(t *testing.T) {
	suite := NewTestSuite(t).
		WithTables("handouts", "games", "users").
		Setup()
	defer suite.Cleanup()

	// Create test data
	gm := suite.Factory().NewUser().WithUsername("testgm").Create()
	game := suite.Factory().NewGame().WithGM(gm.ID).Create()
	handoutService := suite.HandoutService()

	// Create a draft handout
	handout, err := handoutService.CreateHandout(
		context.Background(),
		game.ID,
		"Draft Document",
		"To be published",
		"draft",
		gm.ID,
	)
	core.AssertNoError(t, err, "Failed to create draft handout")
	core.AssertEqual(t, "draft", handout.Status, "Initial status should be draft")

	// Publish the handout
	published, err := handoutService.PublishHandout(context.Background(), handout.ID, gm.ID)
	core.AssertNoError(t, err, "Failed to publish handout")
	core.AssertEqual(t, "published", published.Status, "Status should be published")
}

func TestHandoutService_UnpublishHandout(t *testing.T) {
	suite := NewTestSuite(t).
		WithTables("handouts", "games", "users").
		Setup()
	defer suite.Cleanup()

	// Create test data
	gm := suite.Factory().NewUser().WithUsername("testgm").Create()
	game := suite.Factory().NewGame().WithGM(gm.ID).Create()
	handoutService := suite.HandoutService()

	// Create a published handout
	handout, err := handoutService.CreateHandout(
		context.Background(),
		game.ID,
		"Published Document",
		"To be unpublished",
		"published",
		gm.ID,
	)
	core.AssertNoError(t, err, "Failed to create published handout")
	core.AssertEqual(t, "published", handout.Status, "Initial status should be published")

	// Unpublish the handout
	unpublished, err := handoutService.UnpublishHandout(context.Background(), handout.ID, gm.ID)
	core.AssertNoError(t, err, "Failed to unpublish handout")
	core.AssertEqual(t, "draft", unpublished.Status, "Status should be draft")
}

// =============================================================================
// HANDOUT COMMENT TESTS
// =============================================================================

func TestHandoutService_CreateHandoutComment(t *testing.T) {
	suite := NewTestSuite(t).
		WithTables("handout_comments", "handouts", "games", "users").
		Setup()
	defer suite.Cleanup()

	// Create test data
	gm := suite.Factory().NewUser().WithUsername("testgm").Create()
	game := suite.Factory().NewGame().WithGM(gm.ID).Create()
	handoutService := suite.HandoutService()

	// Create a handout
	handout, err := handoutService.CreateHandout(
		context.Background(),
		game.ID,
		"Document",
		"Content",
		"published",
		gm.ID,
	)
	core.AssertNoError(t, err, "Failed to create handout")

	testCases := []struct {
		name        string
		handoutID   int32
		userID      int32
		parentID    *int32
		content     string
		expectError bool
		description string
	}{
		{
			name:        "create top-level comment",
			handoutID:   handout.ID,
			userID:      gm.ID,
			parentID:    nil,
			content:     "This is a top-level comment",
			expectError: false,
			description: "Should create a top-level comment",
		},
	}

	var topLevelComment *core.HandoutComment

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			comment, err := handoutService.CreateHandoutComment(
				context.Background(),
				tc.handoutID,
				tc.userID,
				tc.parentID,
				tc.content,
			)

			if tc.expectError {
				core.AssertError(t, err, "Expected error for invalid comment creation")
				return
			}

			core.AssertNoError(t, err, "Failed to create comment")
			// Nil check removed - subsequent assertions will fail if nil
			core.AssertEqual(t, tc.content, comment.Content, "Content mismatch")
			core.AssertEqual(t, tc.handoutID, comment.HandoutID, "Handout ID mismatch")

			if tc.parentID == nil {
				topLevelComment = comment
			}
		})
	}

	// Test threaded reply
	if topLevelComment != nil {
		t.Run("create threaded reply", func(t *testing.T) {
			reply, err := handoutService.CreateHandoutComment(
				context.Background(),
				handout.ID,
				gm.ID,
				&topLevelComment.ID,
				"This is a reply",
			)

			core.AssertNoError(t, err, "Failed to create reply")
			// Nil check removed - subsequent assertions will fail if nil
			// Nil check removed - subsequent assertions will fail if nil
			core.AssertEqual(t, topLevelComment.ID, *reply.ParentCommentID, "Parent ID mismatch")
		})
	}
}

func TestHandoutService_ListHandoutComments(t *testing.T) {
	suite := NewTestSuite(t).
		WithTables("handout_comments", "handouts", "games", "users").
		Setup()
	defer suite.Cleanup()

	// Create test data
	gm := suite.Factory().NewUser().WithUsername("testgm").Create()
	game := suite.Factory().NewGame().WithGM(gm.ID).Create()
	handoutService := suite.HandoutService()

	// Create a handout
	handout, err := handoutService.CreateHandout(
		context.Background(),
		game.ID,
		"Document",
		"Content",
		"published",
		gm.ID,
	)
	core.AssertNoError(t, err, "Failed to create handout")

	// Create 3 comments
	_, err = handoutService.CreateHandoutComment(context.Background(), handout.ID, gm.ID, nil, "Comment 1")
	core.AssertNoError(t, err, "Failed to create comment 1")

	_, err = handoutService.CreateHandoutComment(context.Background(), handout.ID, gm.ID, nil, "Comment 2")
	core.AssertNoError(t, err, "Failed to create comment 2")

	_, err = handoutService.CreateHandoutComment(context.Background(), handout.ID, gm.ID, nil, "Comment 3")
	core.AssertNoError(t, err, "Failed to create comment 3")

	// List comments
	comments, err := handoutService.ListHandoutComments(context.Background(), handout.ID)
	core.AssertNoError(t, err, "Failed to list comments")
	core.AssertEqual(t, 3, len(comments), "Should have 3 comments")
}

func TestHandoutService_UpdateHandoutComment(t *testing.T) {
	suite := NewTestSuite(t).
		WithTables("handout_comments", "handouts", "games", "users").
		Setup()
	defer suite.Cleanup()

	// Create test data
	gm := suite.Factory().NewUser().WithUsername("testgm").Create()
	game := suite.Factory().NewGame().WithGM(gm.ID).Create()
	handoutService := suite.HandoutService()

	// Create a handout
	handout, err := handoutService.CreateHandout(
		context.Background(),
		game.ID,
		"Document",
		"Content",
		"published",
		gm.ID,
	)
	core.AssertNoError(t, err, "Failed to create handout")

	// Create a comment
	comment, err := handoutService.CreateHandoutComment(
		context.Background(),
		handout.ID,
		gm.ID,
		nil,
		"Original comment",
	)
	core.AssertNoError(t, err, "Failed to create comment")
	core.AssertEqual(t, int32(0), comment.EditCount, "Initial edit count should be 0")

	// Update the comment
	updated, err := handoutService.UpdateHandoutComment(
		context.Background(),
		comment.ID,
		gm.ID,
		"Updated comment",
	)

	core.AssertNoError(t, err, "Failed to update comment")
	core.AssertEqual(t, "Updated comment", updated.Content, "Content not updated")
	core.AssertEqual(t, int32(1), updated.EditCount, "Edit count should be 1")
	// Nil check removed - subsequent assertions will fail if nil
}

func TestHandoutService_DeleteHandoutComment(t *testing.T) {
	suite := NewTestSuite(t).
		WithTables("handout_comments", "handouts", "games", "users").
		Setup()
	defer suite.Cleanup()

	// Create test data
	gm := suite.Factory().NewUser().WithUsername("testgm").Create()
	game := suite.Factory().NewGame().WithGM(gm.ID).Create()
	handoutService := suite.HandoutService()

	// Create a handout
	handout, err := handoutService.CreateHandout(
		context.Background(),
		game.ID,
		"Document",
		"Content",
		"published",
		gm.ID,
	)
	core.AssertNoError(t, err, "Failed to create handout")

	// Create a comment
	comment, err := handoutService.CreateHandoutComment(
		context.Background(),
		handout.ID,
		gm.ID,
		nil,
		"To be deleted",
	)
	core.AssertNoError(t, err, "Failed to create comment")

	// Delete the comment
	err = handoutService.DeleteHandoutComment(context.Background(), comment.ID, gm.ID, true)
	core.AssertNoError(t, err, "Failed to delete comment")

	// Verify comment is excluded from list (soft deleted)
	comments, err := handoutService.ListHandoutComments(context.Background(), handout.ID)
	core.AssertNoError(t, err, "Failed to list comments")
	core.AssertEqual(t, 0, len(comments), "Deleted comment should not appear in list")
}
