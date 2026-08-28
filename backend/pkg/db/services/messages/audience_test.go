package messages

import (
	"actionphase/pkg/core"
	db "actionphase/pkg/db/services"
	"context"
	"testing"
)

// TestListAllPrivateConversations tests the audience feature for viewing all private conversations
func TestListAllPrivateConversations(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	fixtures := testDB.SetupFixtures(t)
	msgService := &MessageService{DB: testDB.Pool, Logger: app.ObsLogger}

	gameID := fixtures.TestGame.ID

	// Create some test users and characters for private conversations
	player1 := testDB.CreateTestUser(t, "player1_conv", "player1_conv@example.com")
	player2 := testDB.CreateTestUser(t, "player2_conv", "player2_conv@example.com")

	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	userID1 := int32(player1.ID)
	userID2 := int32(player2.ID)

	char1, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &userID1,
		Name:          "Player1Char",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create character 1")

	char2, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &userID2,
		Name:          "Player2Char",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create character 2")

	// Create a private conversation using ConversationService
	conversationService := db.NewConversationService(testDB.Pool, nil)
	conversation, err := conversationService.CreateConversation(context.Background(), db.CreateConversationRequest{
		GameID:          gameID,
		Title:           "Private conversation between Player1 and Player2",
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	core.AssertNoError(t, err, "Failed to create conversation")

	// Send a private message in the conversation
	_, err = conversationService.SendMessage(context.Background(), db.SendMessageRequest{
		ConversationID:    conversation.ID,
		SenderUserID:      int32(player1.ID),
		SenderCharacterID: char1.ID,
		Content:           "Private message to player 2",
	})
	core.AssertNoError(t, err, "Failed to send private message")

	t.Run("list_all_private_conversations_success", func(t *testing.T) {
		conversations, err := msgService.ListAllPrivateConversations(context.Background(), core.ListAllPrivateConversationsParams{
			GameID:                  gameID,
			ParticipantCharacterIDs: nil,
			Limit:                   20,
			Offset:                  0,
		})
		core.AssertNoError(t, err, "Should list conversations successfully")

		// Even if no conversations exist, should return empty array, not error
		core.AssertTrue(t, conversations != nil, "Conversations should not be nil")
	})

	t.Run("list_all_private_conversations_nonexistent_game", func(t *testing.T) {
		conversations, err := msgService.ListAllPrivateConversations(context.Background(), core.ListAllPrivateConversationsParams{
			GameID:                  99999,
			ParticipantCharacterIDs: nil,
			Limit:                   20,
			Offset:                  0,
		})

		// Should succeed but return empty list for nonexistent game
		core.AssertNoError(t, err, "Should not error on nonexistent game")
		core.AssertEqual(t, 0, len(conversations), "Should return empty list for nonexistent game")
	})
}

// TestListAllPrivateConversations_ParticipantFilter tests that filtering by multiple
// participant character IDs uses AND semantics (all must be present), not OR (any present).
func TestListAllPrivateConversations_ParticipantFilter(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)
	fixtures := testDB.SetupFixtures(t)
	msgService := &MessageService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameID := fixtures.TestGame.ID

	// Create three users/characters
	u1 := testDB.CreateTestUser(t, "pf_user1", "pf_user1@example.com")
	u2 := testDB.CreateTestUser(t, "pf_user2", "pf_user2@example.com")
	u3 := testDB.CreateTestUser(t, "pf_user3", "pf_user3@example.com")

	charSvc := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	uid1, uid2, uid3 := int32(u1.ID), int32(u2.ID), int32(u3.ID)

	c1, err := charSvc.CreateCharacter(context.Background(), db.CreateCharacterRequest{GameID: gameID, UserID: &uid1, Name: "Alpha", CharacterType: "player_character"})
	core.AssertNoError(t, err, "create char 1")
	c2, err := charSvc.CreateCharacter(context.Background(), db.CreateCharacterRequest{GameID: gameID, UserID: &uid2, Name: "Beta", CharacterType: "player_character"})
	core.AssertNoError(t, err, "create char 2")
	c3, err := charSvc.CreateCharacter(context.Background(), db.CreateCharacterRequest{GameID: gameID, UserID: &uid3, Name: "Gamma", CharacterType: "player_character"})
	core.AssertNoError(t, err, "create char 3")

	convSvc := db.NewConversationService(testDB.Pool, nil)

	// Conversation 1: Alpha + Beta
	_, err = convSvc.CreateConversation(context.Background(), db.CreateConversationRequest{
		GameID: gameID, Title: "Alpha-Beta", CreatedByUserID: uid1,
		ParticipantIDs: []int32{c1.ID, c2.ID},
	})
	core.AssertNoError(t, err, "create conv Alpha-Beta")

	// Conversation 2: Alpha + Gamma
	_, err = convSvc.CreateConversation(context.Background(), db.CreateConversationRequest{
		GameID: gameID, Title: "Alpha-Gamma", CreatedByUserID: uid1,
		ParticipantIDs: []int32{c1.ID, c3.ID},
	})
	core.AssertNoError(t, err, "create conv Alpha-Gamma")

	t.Run("single_participant_filter_returns_both", func(t *testing.T) {
		// Filtering by Alpha alone should return both conversations
		convs, err := msgService.ListAllPrivateConversations(context.Background(), core.ListAllPrivateConversationsParams{
			GameID: gameID, ParticipantCharacterIDs: []int32{c1.ID}, Limit: 20, Offset: 0,
		})
		core.AssertNoError(t, err, "should list successfully")
		core.AssertTrue(t, len(convs) >= 2, "Alpha is in both conversations")
	})

	t.Run("two_participant_filter_uses_AND_not_OR", func(t *testing.T) {
		// Filtering by Alpha + Beta should return ONLY the Alpha-Beta conversation,
		// not Alpha-Gamma (which has Alpha but not Beta).
		convs, err := msgService.ListAllPrivateConversations(context.Background(), core.ListAllPrivateConversationsParams{
			GameID: gameID, ParticipantCharacterIDs: []int32{c1.ID, c2.ID}, Limit: 20, Offset: 0,
		})
		core.AssertNoError(t, err, "should list successfully")
		core.AssertEqual(t, 1, len(convs), "should return only the Alpha-Beta conversation")
		if len(convs) == 1 {
			core.AssertEqual(t, "Alpha-Beta", convs[0].Subject.String, "returned conversation should be Alpha-Beta")
		}
	})
}

// TestGetConversationParticipantCharacters tests the participant filter list endpoint.
func TestGetConversationParticipantCharacters(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)
	fixtures := testDB.SetupFixtures(t)
	msgService := &MessageService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameID := fixtures.TestGame.ID

	u1 := testDB.CreateTestUser(t, "gpn_user1", "gpn1@example.com")
	u2 := testDB.CreateTestUser(t, "gpn_user2", "gpn2@example.com")
	u3 := testDB.CreateTestUser(t, "gpn_user3", "gpn3@example.com")

	charSvc := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	uid1, uid2, uid3 := int32(u1.ID), int32(u2.ID), int32(u3.ID)

	cAlpha, err := charSvc.CreateCharacter(context.Background(), db.CreateCharacterRequest{GameID: gameID, UserID: &uid1, Name: "Alpha", CharacterType: "player_character"})
	core.AssertNoError(t, err, "create Alpha")
	cBeta, err := charSvc.CreateCharacter(context.Background(), db.CreateCharacterRequest{GameID: gameID, UserID: &uid2, Name: "Beta", CharacterType: "player_character"})
	core.AssertNoError(t, err, "create Beta")
	cGamma, err := charSvc.CreateCharacter(context.Background(), db.CreateCharacterRequest{GameID: gameID, UserID: &uid3, Name: "Gamma", CharacterType: "player_character"})
	core.AssertNoError(t, err, "create Gamma")

	convSvc := db.NewConversationService(testDB.Pool, nil)

	// Alpha <-> Beta conversation
	_, err = convSvc.CreateConversation(context.Background(), db.CreateConversationRequest{
		GameID: gameID, Title: "Alpha-Beta", CreatedByUserID: uid1,
		ParticipantIDs: []int32{cAlpha.ID, cBeta.ID},
	})
	core.AssertNoError(t, err, "create Alpha-Beta conv")

	// Alpha <-> Gamma conversation
	_, err = convSvc.CreateConversation(context.Background(), db.CreateConversationRequest{
		GameID: gameID, Title: "Alpha-Gamma", CreatedByUserID: uid1,
		ParticipantIDs: []int32{cAlpha.ID, cGamma.ID},
	})
	core.AssertNoError(t, err, "create Alpha-Gamma conv")

	// No Beta <-> Gamma conversation exists.

	t.Run("no_filter_returns_all_participants", func(t *testing.T) {
		chars, err := msgService.GetConversationParticipantCharacters(context.Background(), gameID, nil)
		core.AssertNoError(t, err, "should succeed")
		core.AssertTrue(t, containsID(chars, cAlpha.ID), "Alpha should be present")
		core.AssertTrue(t, containsID(chars, cBeta.ID), "Beta should be present")
		core.AssertTrue(t, containsID(chars, cGamma.ID), "Gamma should be present")
	})

	t.Run("selecting_alpha_returns_beta_and_gamma", func(t *testing.T) {
		chars, err := msgService.GetConversationParticipantCharacters(context.Background(), gameID, []int32{cAlpha.ID})
		core.AssertNoError(t, err, "should succeed")
		core.AssertTrue(t, containsID(chars, cBeta.ID), "Beta co-appears with Alpha")
		core.AssertTrue(t, containsID(chars, cGamma.ID), "Gamma co-appears with Alpha")
	})

	t.Run("selecting_beta_excludes_gamma", func(t *testing.T) {
		// Beta and Gamma share no conversation, so Gamma must not appear
		chars, err := msgService.GetConversationParticipantCharacters(context.Background(), gameID, []int32{cBeta.ID})
		core.AssertNoError(t, err, "should succeed")
		core.AssertTrue(t, containsID(chars, cAlpha.ID), "Alpha co-appears with Beta")
		core.AssertTrue(t, !containsID(chars, cGamma.ID), "Gamma does NOT co-appear with Beta")
	})

	t.Run("selecting_alpha_and_beta_excludes_gamma", func(t *testing.T) {
		chars, err := msgService.GetConversationParticipantCharacters(context.Background(), gameID, []int32{cAlpha.ID, cBeta.ID})
		core.AssertNoError(t, err, "should succeed")
		core.AssertTrue(t, containsID(chars, cAlpha.ID), "Alpha should be present")
		core.AssertTrue(t, containsID(chars, cBeta.ID), "Beta should be present")
		core.AssertTrue(t, !containsID(chars, cGamma.ID), "Gamma does NOT share a conv with both Alpha and Beta")
	})

	t.Run("renaming_a_character_does_not_change_filter_results", func(t *testing.T) {
		// The regression this fix targets: filtering used to match on display name, so
		// renaming a character silently changed which conversations matched. Filtering
		// by ID must be unaffected by the rename.
		_, err := testDB.Pool.Exec(context.Background(),
			`UPDATE characters SET name = 'Alpha Renamed' WHERE id = $1`, cAlpha.ID)
		core.AssertNoError(t, err, "rename Alpha")

		convs, err := msgService.ListAllPrivateConversations(context.Background(), core.ListAllPrivateConversationsParams{
			GameID: gameID, ParticipantCharacterIDs: []int32{cAlpha.ID, cBeta.ID}, Limit: 20, Offset: 0,
		})
		core.AssertNoError(t, err, "should list successfully after rename")
		core.AssertEqual(t, 1, len(convs), "Alpha-Beta conversation still matches after rename")

		count, err := msgService.CountAllPrivateConversations(context.Background(), gameID, []int32{cAlpha.ID, cBeta.ID})
		core.AssertNoError(t, err, "should count successfully after rename")
		core.AssertEqual(t, int64(1), count, "count agrees with list after rename")
	})
}

func containsID(chars []core.ConversationParticipantCharacter, id int32) bool {
	for _, c := range chars {
		if c.ID == id {
			return true
		}
	}
	return false
}

// TestGetAudienceConversationMessages tests retrieving messages from a conversation
func TestGetAudienceConversationMessages(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	fixtures := testDB.SetupFixtures(t)
	msgService := &MessageService{DB: testDB.Pool, Logger: app.ObsLogger}

	gameID := fixtures.TestGame.ID
	player1 := testDB.CreateTestUser(t, "player1_msg", "player1_msg@example.com")

	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	userID1 := int32(player1.ID)
	char1, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &userID1,
		Name:          "Player1CharMsg",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create character")

	// Create a conversation and send a message
	conversationService := db.NewConversationService(testDB.Pool, nil)
	conversation, err := conversationService.CreateConversation(context.Background(), db.CreateConversationRequest{
		GameID:          gameID,
		Title:           "Test conversation",
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{char1.ID},
	})
	core.AssertNoError(t, err, "Failed to create conversation")

	// Send a message in the conversation
	_, err = conversationService.SendMessage(context.Background(), db.SendMessageRequest{
		ConversationID:    conversation.ID,
		SenderUserID:      int32(player1.ID),
		SenderCharacterID: char1.ID,
		Content:           "Test conversation message",
	})
	core.AssertNoError(t, err, "Failed to send message")

	t.Run("get_audience_conversation_messages_nonexistent", func(t *testing.T) {
		messages, err := msgService.GetAudienceConversationMessages(context.Background(), 99999)

		// Should succeed but return empty list for nonexistent conversation
		core.AssertNoError(t, err, "Should not error on nonexistent conversation")
		core.AssertEqual(t, 0, len(messages), "Should return empty list for nonexistent conversation")
	})

	t.Run("get_audience_conversation_messages_valid_id", func(t *testing.T) {
		// Use the actual conversation ID
		messages, err := msgService.GetAudienceConversationMessages(context.Background(), conversation.ID)

		// Function should execute without error
		core.AssertNoError(t, err, "Should execute successfully")
		core.AssertTrue(t, messages != nil, "Messages should not be nil")
		// Should have at least one message
		core.AssertTrue(t, len(messages) >= 1, "Should have at least one message")
		// Verify the message content matches what was sent
		if len(messages) > 0 {
			core.AssertEqual(t, "Test conversation message", messages[0].Content, "Message content should match what was sent")
		}
	})
}

// TestGetMessage tests single message retrieval with character details
func TestGetMessage(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	fixtures := testDB.SetupFixtures(t)
	msgService := &MessageService{DB: testDB.Pool, Logger: app.ObsLogger}

	gameID := fixtures.TestGame.ID
	player := testDB.CreateTestUser(t, "player_getmsg", "player_getmsg@example.com")

	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	userID := int32(player.ID)
	character, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &userID,
		Name:          "GetMsgChar",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create character")

	// Create a test post
	post, err := msgService.CreatePost(context.Background(), core.CreatePostRequest{
		GameID:      gameID,
		AuthorID:    int32(player.ID),
		CharacterID: character.ID,
		Content:     "Test message for GetMessage",
		Visibility:  "game",
	})
	core.AssertNoError(t, err, "Failed to create test post")

	t.Run("get_message_success", func(t *testing.T) {
		message, err := msgService.GetMessage(context.Background(), post.ID)
		core.AssertNoError(t, err, "Should retrieve message successfully")

		core.AssertNotEqual(t, nil, message, "Message should not be nil")
		core.AssertEqual(t, post.ID, message.Message.ID, "Message ID should match")
		core.AssertEqual(t, "Test message for GetMessage", message.Message.Content, "Content should match")
		core.AssertEqual(t, character.ID, message.Message.CharacterID, "Character ID should match")

		// Verify details are populated
		core.AssertEqual(t, player.Username, message.AuthorUsername, "Author username should be populated")
		core.AssertEqual(t, character.Name, message.CharacterName, "Character name should be populated")
	})

	t.Run("get_message_nonexistent", func(t *testing.T) {
		message, err := msgService.GetMessage(context.Background(), 99999)

		core.AssertTrue(t, err != nil, "Should error on nonexistent message")
		core.AssertEqual(t, nil, message, "Message should be nil on error")
	})

	t.Run("get_message_with_comment", func(t *testing.T) {
		// Create a comment to test message retrieval for comments
		comment, err := msgService.CreateComment(context.Background(), core.CreateCommentRequest{
			GameID:      gameID,
			ParentID:    post.ID,
			AuthorID:    int32(player.ID),
			CharacterID: character.ID,
			Content:     "Test comment",
			Visibility:  "game",
		})
		core.AssertNoError(t, err, "Failed to create comment")

		message, err := msgService.GetMessage(context.Background(), comment.ID)
		core.AssertNoError(t, err, "Should retrieve comment message successfully")

		core.AssertEqual(t, comment.ID, message.Message.ID, "Comment ID should match")
		core.AssertEqual(t, "Test comment", message.Message.Content, "Comment content should match")
		core.AssertEqual(t, post.ID, message.Message.ParentID.Int32, "Parent ID should match post")
	})
}

// TestGetUnreadCommentIDsForPosts tests bulk unread comment retrieval
func TestGetUnreadCommentIDsForPosts(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	fixtures := testDB.SetupFixtures(t)
	msgService := &MessageService{DB: testDB.Pool, Logger: app.ObsLogger}

	gameID := fixtures.TestGame.ID

	// Create test users
	gmUser := fixtures.TestUser
	playerUser := testDB.CreateTestUser(t, "player_unread", "player_unread@example.com")

	// Create characters
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	gmUserID := int32(gmUser.ID)
	playerUserID := int32(playerUser.ID)

	gmChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &gmUserID,
		Name:          "GMCharUnread",
		CharacterType: "npc",
	})
	core.AssertNoError(t, err, "Failed to create GM character")

	_, err = characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        gameID,
		UserID:        &playerUserID,
		Name:          "PlayerCharUnread",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create player character")

	// Create multiple posts
	post1, err := msgService.CreatePost(context.Background(), core.CreatePostRequest{
		GameID:      gameID,
		AuthorID:    int32(gmUser.ID),
		CharacterID: gmChar.ID,
		Content:     "Post 1 for unread testing",
		Visibility:  "game",
	})
	core.AssertNoError(t, err, "Failed to create post 1")

	post2, err := msgService.CreatePost(context.Background(), core.CreatePostRequest{
		GameID:      gameID,
		AuthorID:    int32(gmUser.ID),
		CharacterID: gmChar.ID,
		Content:     "Post 2 for unread testing",
		Visibility:  "game",
	})
	core.AssertNoError(t, err, "Failed to create post 2")

	// Create comments on the posts
	_, err = msgService.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID:      gameID,
		ParentID:    post1.ID,
		AuthorID:    int32(gmUser.ID),
		CharacterID: gmChar.ID,
		Content:     "Comment on post 1",
		Visibility:  "game",
	})
	core.AssertNoError(t, err, "Failed to create comment 1")

	_, err = msgService.CreateComment(context.Background(), core.CreateCommentRequest{
		GameID:      gameID,
		ParentID:    post2.ID,
		AuthorID:    int32(gmUser.ID),
		CharacterID: gmChar.ID,
		Content:     "Comment on post 2",
		Visibility:  "game",
	})
	core.AssertNoError(t, err, "Failed to create comment 2")

	t.Run("get_unread_comment_ids_for_posts_success", func(t *testing.T) {
		// Player user has not marked anything as read, so all comments should be unread
		unreadInfo, err := msgService.GetUnreadCommentIDsForPosts(context.Background(), int32(playerUser.ID), gameID)
		core.AssertNoError(t, err, "Should retrieve unread comment IDs successfully")

		core.AssertTrue(t, unreadInfo != nil, "Unread info should not be nil")

		// Should have information for posts that have comments
		if len(unreadInfo) > 0 {
			// Verify structure
			for _, info := range unreadInfo {
				core.AssertTrue(t, info.PostID > 0, "Post ID should be valid")
				core.AssertTrue(t, info.UnreadCommentIDs != nil, "Unread comment IDs should not be nil")
			}
		}
	})

	t.Run("get_unread_comment_ids_after_marking_read", func(t *testing.T) {
		// First, mark both posts as read to establish initial visit timestamp
		// This simulates the user visiting the page for the first time
		_, err := msgService.MarkPostAsRead(context.Background(), int32(playerUser.ID), gameID, post1.ID, nil)
		core.AssertNoError(t, err, "Should mark post 1 as visited")

		_, err = msgService.MarkPostAsRead(context.Background(), int32(playerUser.ID), gameID, post2.ID, nil)
		core.AssertNoError(t, err, "Should mark post 2 as visited")

		// Now create a NEW comment on post 2 AFTER the user visited
		// This comment should show up as unread on the next check
		newComment, err := msgService.CreateComment(context.Background(), core.CreateCommentRequest{
			GameID:      gameID,
			ParentID:    post2.ID,
			AuthorID:    int32(gmUser.ID),
			CharacterID: gmChar.ID,
			Content:     "New comment after visit",
			Visibility:  "game",
		})
		core.AssertNoError(t, err, "Failed to create new comment")

		// Now check unread - post 2 should have the new comment as unread
		unreadInfo, err := msgService.GetUnreadCommentIDsForPosts(context.Background(), int32(playerUser.ID), gameID)
		core.AssertNoError(t, err, "Should retrieve unread comment IDs after marking read")

		core.AssertTrue(t, unreadInfo != nil, "Unread info should not be nil")

		// Post 2 should have the new comment as unread
		foundPost2 := false
		for _, info := range unreadInfo {
			if info.PostID == post2.ID {
				foundPost2 = true
				// Should have exactly 1 unread comment (the new one)
				core.AssertTrue(t, len(info.UnreadCommentIDs) > 0, "Post 2 should have unread comments")
				core.AssertTrue(t, containsInt32(info.UnreadCommentIDs, newComment.ID), "Should include the new comment ID")
			}
		}
		core.AssertTrue(t, foundPost2, "Should find post 2 with unread comments")
	})

	t.Run("get_unread_comment_ids_nonexistent_game", func(t *testing.T) {
		unreadInfo, err := msgService.GetUnreadCommentIDsForPosts(context.Background(), int32(playerUser.ID), 99999)

		core.AssertNoError(t, err, "Should not error on nonexistent game")
		core.AssertEqual(t, 0, len(unreadInfo), "Should return empty list for nonexistent game")
	})

	t.Run("get_unread_comment_ids_nonexistent_user", func(t *testing.T) {
		unreadInfo, err := msgService.GetUnreadCommentIDsForPosts(context.Background(), 99999, gameID)

		core.AssertNoError(t, err, "Should not error on nonexistent user")
		// Should return results but the user has no read markers, so all comments are unread
		core.AssertTrue(t, unreadInfo != nil, "Should return results")
	})
}

// Helper function to check if a slice contains a specific int32
func containsInt32(slice []int32, value int32) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}
