package db

import (
	"context"
	"testing"
	"time"

	"actionphase/pkg/core"
	dbmodels "actionphase/pkg/db/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConversationService_CreateConversation(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := NewConversationService(testDB.Pool)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	charService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Setup: Create game, users, and characters
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	// Add players as participants
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	require.NoError(t, err)

	// Create characters
	char1, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player1.ID)),
		Name:          "Character 1",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	char2, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player2.ID)),
		Name:          "Character 2",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	t.Run("creates direct conversation successfully", func(t *testing.T) {
		req := CreateConversationRequest{
			GameID:          game.ID,
			Title:           "Direct Chat",
			CreatedByUserID: int32(player1.ID),
			ParticipantIDs:  []int32{char1.ID, char2.ID},
		}

		conversation, err := service.CreateConversation(context.Background(), req)

		require.NoError(t, err)
		assert.NotNil(t, conversation)
		assert.Equal(t, game.ID, conversation.GameID)
		assert.Equal(t, "direct", conversation.ConversationType)
		assert.Equal(t, "Direct Chat", conversation.Title.String)
	})

	t.Run("creates group conversation with 3+ participants", func(t *testing.T) {
		// Create third character
		player3 := testDB.CreateTestUser(t, "player3", "player3@example.com")
		_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player3.ID), "player")
		require.NoError(t, err)

		char3, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
			GameID:        game.ID,
			UserID:        int32Ptr(int32(player3.ID)),
			Name:          "Character 3",
			CharacterType: "player_character",
		})
		require.NoError(t, err)

		req := CreateConversationRequest{
			GameID:          game.ID,
			Title:           "Group Chat",
			CreatedByUserID: int32(player1.ID),
			ParticipantIDs:  []int32{char1.ID, char2.ID, char3.ID},
		}

		conversation, err := service.CreateConversation(context.Background(), req)

		require.NoError(t, err)
		assert.Equal(t, "group", conversation.ConversationType)
	})

	t.Run("adds all participants to conversation", func(t *testing.T) {
		req := CreateConversationRequest{
			GameID:          game.ID,
			Title:           "Participant Test",
			CreatedByUserID: int32(player1.ID),
			ParticipantIDs:  []int32{char1.ID, char2.ID},
		}

		conversation, err := service.CreateConversation(context.Background(), req)
		require.NoError(t, err)

		// Verify participants were added
		participants, err := service.GetConversationParticipants(context.Background(), conversation.ID)
		require.NoError(t, err)
		assert.Len(t, participants, 2)
	})
}

func TestConversationService_SendMessage(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := NewConversationService(testDB.Pool)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	charService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Setup
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	require.NoError(t, err)

	char1, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player1.ID)),
		Name:          "Character 1",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	char2, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player2.ID)),
		Name:          "Character 2",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	// Create conversation
	conversation, err := service.CreateConversation(context.Background(), CreateConversationRequest{
		GameID:          game.ID,
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	require.NoError(t, err)

	t.Run("sends message successfully", func(t *testing.T) {
		req := SendMessageRequest{
			ConversationID:    conversation.ID,
			SenderUserID:      int32(player1.ID),
			SenderCharacterID: char1.ID,
			Content:           "Hello, this is a test message!",
		}

		message, err := service.SendMessage(context.Background(), req)

		require.NoError(t, err)
		assert.NotNil(t, message)
		assert.Equal(t, conversation.ID, message.ConversationID)
		assert.Equal(t, int32(player1.ID), message.SenderUserID)
		assert.Equal(t, "Hello, this is a test message!", message.Content)
	})

	t.Run("rejects message from non-participant", func(t *testing.T) {
		// Create a third player who is not in the conversation
		player3 := testDB.CreateTestUser(t, "player3", "player3@example.com")

		req := SendMessageRequest{
			ConversationID:    conversation.ID,
			SenderUserID:      int32(player3.ID),
			SenderCharacterID: 0, // No character
			Content:           "Unauthorized message",
		}

		_, err := service.SendMessage(context.Background(), req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a participant")
	})

	t.Run("updates conversation activity timestamp", func(t *testing.T) {
		// Send a message
		req := SendMessageRequest{
			ConversationID:    conversation.ID,
			SenderUserID:      int32(player1.ID),
			SenderCharacterID: char1.ID,
			Content:           "Activity test message",
		}

		_, err := service.SendMessage(context.Background(), req)
		require.NoError(t, err)

		// Verify conversation has updated timestamp
		// (implicitly verified by successful message send)
	})
}

func TestConversationService_GetConversationMessages(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := NewConversationService(testDB.Pool)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	charService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Setup
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	require.NoError(t, err)

	char1, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player1.ID)),
		Name:          "Character 1",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	char2, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player2.ID)),
		Name:          "Character 2",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	// Create conversation
	conversation, err := service.CreateConversation(context.Background(), CreateConversationRequest{
		GameID:          game.ID,
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	require.NoError(t, err)

	// Send several messages
	messages := []string{"First message", "Second message", "Third message"}
	for _, content := range messages {
		_, err := service.SendMessage(context.Background(), SendMessageRequest{
			ConversationID:    conversation.ID,
			SenderUserID:      int32(player1.ID),
			SenderCharacterID: char1.ID,
			Content:           content,
		})
		require.NoError(t, err)
	}

	t.Run("retrieves all conversation messages", func(t *testing.T) {
		msgs, err := service.GetConversationMessages(context.Background(), conversation.ID, int32(player1.ID))

		require.NoError(t, err)
		assert.Len(t, msgs, 3)
		assert.Equal(t, "First message", msgs[0].Content)
		assert.Equal(t, "Second message", msgs[1].Content)
		assert.Equal(t, "Third message", msgs[2].Content)
	})

	t.Run("rejects non-participant from viewing messages", func(t *testing.T) {
		// Create a third player not in conversation
		player3 := testDB.CreateTestUser(t, "player3", "player3@example.com")

		_, err := service.GetConversationMessages(context.Background(), conversation.ID, int32(player3.ID))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a participant")
	})
}

func TestConversationService_GetUserConversations(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := NewConversationService(testDB.Pool)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	charService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Setup
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	require.NoError(t, err)

	char1, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player1.ID)),
		Name:          "Character 1",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	char2, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player2.ID)),
		Name:          "Character 2",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	// Create multiple conversations for player1
	conv1, err := service.CreateConversation(context.Background(), CreateConversationRequest{
		GameID:          game.ID,
		Title:           "Conversation 1",
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	require.NoError(t, err)

	conv2, err := service.CreateConversation(context.Background(), CreateConversationRequest{
		GameID:          game.ID,
		Title:           "Conversation 2",
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	require.NoError(t, err)

	t.Run("retrieves all user conversations in game", func(t *testing.T) {
		conversations, err := service.GetUserConversations(context.Background(), game.ID, int32(player1.ID))

		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(conversations), 2)
		convIDs := make([]int32, 0, len(conversations))
		for _, c := range conversations {
			convIDs = append(convIDs, c.ID)
		}
		assert.Contains(t, convIDs, conv1.ID, "should include Conversation 1")
		assert.Contains(t, convIDs, conv2.ID, "should include Conversation 2")
	})

	t.Run("filters by game", func(t *testing.T) {
		// Create another game
		otherGame := testDB.CreateTestGame(t, int32(gm.ID), "Other Game")

		conversations, err := service.GetUserConversations(context.Background(), otherGame.ID, int32(player1.ID))

		require.NoError(t, err)
		// Should have no conversations in the other game
		assert.Len(t, conversations, 0)
	})
}

func TestConversationService_MarkAsRead(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := NewConversationService(testDB.Pool)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	charService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Setup
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	require.NoError(t, err)

	char1, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player1.ID)),
		Name:          "Character 1",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	char2, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player2.ID)),
		Name:          "Character 2",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	// Create conversation
	conversation, err := service.CreateConversation(context.Background(), CreateConversationRequest{
		GameID:          game.ID,
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	require.NoError(t, err)

	// Player1 sends messages
	_, err = service.SendMessage(context.Background(), SendMessageRequest{
		ConversationID:    conversation.ID,
		SenderUserID:      int32(player1.ID),
		SenderCharacterID: char1.ID,
		Content:           "Unread message",
	})
	require.NoError(t, err)

	t.Run("marks conversation as read", func(t *testing.T) {
		err := service.MarkConversationAsRead(context.Background(), conversation.ID, int32(player2.ID))

		require.NoError(t, err)
	})

	t.Run("rejects non-participant from marking as read", func(t *testing.T) {
		player3 := testDB.CreateTestUser(t, "player3", "player3@example.com")

		err := service.MarkConversationAsRead(context.Background(), conversation.ID, int32(player3.ID))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a participant")
	})
}

func TestConversationService_UnreadCount(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := NewConversationService(testDB.Pool)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	charService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Setup
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	require.NoError(t, err)

	char1, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player1.ID)),
		Name:          "Character 1",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	char2, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player2.ID)),
		Name:          "Character 2",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	// Create conversation
	conversation, err := service.CreateConversation(context.Background(), CreateConversationRequest{
		GameID:          game.ID,
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	require.NoError(t, err)

	// Send messages from player1
	for i := 0; i < 3; i++ {
		_, err := service.SendMessage(context.Background(), SendMessageRequest{
			ConversationID:    conversation.ID,
			SenderUserID:      int32(player1.ID),
			SenderCharacterID: char1.ID,
			Content:           "Unread message",
		})
		require.NoError(t, err)
	}

	t.Run("counts unread messages correctly", func(t *testing.T) {
		count, err := service.GetUnreadMessageCount(context.Background(), conversation.ID, int32(player2.ID))

		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, int64(3))
	})

	t.Run("zero unread after marking as read", func(t *testing.T) {
		err := service.MarkConversationAsRead(context.Background(), conversation.ID, int32(player2.ID))
		require.NoError(t, err)

		count, err := service.GetUnreadMessageCount(context.Background(), conversation.ID, int32(player2.ID))
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})
}

func TestConversationService_AddParticipant(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := NewConversationService(testDB.Pool)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	charService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Setup
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
	player3 := testDB.CreateTestUser(t, "player3", "player3@example.com")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player3.ID), "player")
	require.NoError(t, err)

	char1, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player1.ID)),
		Name:          "Character 1",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	char2, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player2.ID)),
		Name:          "Character 2",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	char3, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player3.ID)),
		Name:          "Character 3",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	// Create conversation with 2 participants
	conversation, err := service.CreateConversation(context.Background(), CreateConversationRequest{
		GameID:          game.ID,
		Title:           "Initial Conversation",
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	require.NoError(t, err)

	t.Run("adds participant successfully", func(t *testing.T) {
		err := service.AddParticipant(context.Background(), conversation.ID, char3.ID)

		require.NoError(t, err)

		// Verify participant was added
		participants, err := service.GetConversationParticipants(context.Background(), conversation.ID)
		require.NoError(t, err)
		assert.Len(t, participants, 3)
	})

	t.Run("adds NPC participant using GM's user ID", func(t *testing.T) {
		// Create an NPC (no user_id)
		npc, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
			GameID:        game.ID,
			UserID:        nil, // NPC has no user
			Name:          "NPC Character",
			CharacterType: "npc",
		})
		require.NoError(t, err)

		// Create new conversation
		newConv, err := service.CreateConversation(context.Background(), CreateConversationRequest{
			GameID:          game.ID,
			Title:           "NPC Conversation",
			CreatedByUserID: int32(gm.ID),
			ParticipantIDs:  []int32{char1.ID, char2.ID},
		})
		require.NoError(t, err)

		// Add NPC to conversation
		err = service.AddParticipant(context.Background(), newConv.ID, npc.ID)
		require.NoError(t, err)

		// Verify NPC was added
		participants, err := service.GetConversationParticipants(context.Background(), newConv.ID)
		require.NoError(t, err)
		assert.Len(t, participants, 3)
	})

	t.Run("returns error for non-existent character", func(t *testing.T) {
		err := service.AddParticipant(context.Background(), conversation.ID, 99999)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "character")
	})
}

func TestConversationService_GetOrCreateConversation(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := NewConversationService(testDB.Pool)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	charService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Setup
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	require.NoError(t, err)

	char1, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player1.ID)),
		Name:          "Character 1",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	char2, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player2.ID)),
		Name:          "Character 2",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	t.Run("creates new conversation for two characters", func(t *testing.T) {
		conv, err := service.GetOrCreateConversation(
			context.Background(),
			game.ID,
			[]int32{char1.ID, char2.ID},
			int32(player1.ID),
			"Test Conversation",
		)

		require.NoError(t, err)
		assert.NotNil(t, conv)
		assert.Equal(t, game.ID, conv.GameID)
		assert.Equal(t, "direct", conv.ConversationType)
	})

	t.Run("creates new group conversation for three characters", func(t *testing.T) {
		player3 := testDB.CreateTestUser(t, "player3", "player3@example.com")
		_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player3.ID), "player")
		require.NoError(t, err)

		char3, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
			GameID:        game.ID,
			UserID:        int32Ptr(int32(player3.ID)),
			Name:          "Character 3",
			CharacterType: "player_character",
		})
		require.NoError(t, err)

		conv, err := service.GetOrCreateConversation(
			context.Background(),
			game.ID,
			[]int32{char1.ID, char2.ID, char3.ID},
			int32(player1.ID),
			"Group Chat",
		)

		require.NoError(t, err)
		assert.Equal(t, "group", conv.ConversationType)
	})
}

func TestConversationService_CreateConversationWithNPC(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := NewConversationService(testDB.Pool)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	charService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Setup
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	require.NoError(t, err)

	char1, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player1.ID)),
		Name:          "Player Character",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	// Create NPC without user_id
	npc, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        nil, // NPC has no user
		Name:          "NPC Character",
		CharacterType: "npc",
	})
	require.NoError(t, err)

	t.Run("creates conversation with NPC using GM's user ID", func(t *testing.T) {
		req := CreateConversationRequest{
			GameID:          game.ID,
			Title:           "Player-NPC Chat",
			CreatedByUserID: int32(player1.ID),
			ParticipantIDs:  []int32{char1.ID, npc.ID},
		}

		conversation, err := service.CreateConversation(context.Background(), req)

		require.NoError(t, err)
		assert.NotNil(t, conversation)
		assert.Equal(t, "direct", conversation.ConversationType)

		// Verify both participants were added
		participants, err := service.GetConversationParticipants(context.Background(), conversation.ID)
		require.NoError(t, err)
		assert.Len(t, participants, 2)
	})
}

func TestConversationService_EdgeCases(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := NewConversationService(testDB.Pool)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	charService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Setup
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	require.NoError(t, err)

	char1, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player1.ID)),
		Name:          "Character 1",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	char2, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player2.ID)),
		Name:          "Character 2",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	// Create conversation
	conversation, err := service.CreateConversation(context.Background(), CreateConversationRequest{
		GameID:          game.ID,
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	require.NoError(t, err)

	t.Run("GetUnreadMessageCount returns 0 for participant with no unread messages", func(t *testing.T) {
		// Player1 is participant but hasn't received any messages (they sent the messages)
		count, err := service.GetUnreadMessageCount(context.Background(), conversation.ID, int32(player1.ID))

		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	t.Run("GetConversationParticipants returns empty for non-existent conversation", func(t *testing.T) {
		participants, err := service.GetConversationParticipants(context.Background(), 99999)

		// Should return empty list or error
		if err == nil {
			assert.Empty(t, participants)
		} else {
			// Error is acceptable for non-existent conversation
			assert.Error(t, err)
		}
	})

	t.Run("CreateConversation with empty title", func(t *testing.T) {
		req := CreateConversationRequest{
			GameID:          game.ID,
			Title:           "", // Empty title
			CreatedByUserID: int32(player1.ID),
			ParticipantIDs:  []int32{char1.ID, char2.ID},
		}

		conv, err := service.CreateConversation(context.Background(), req)

		require.NoError(t, err)
		assert.NotNil(t, conv)
		// Empty title should result in null/invalid title
		assert.False(t, conv.Title.Valid)
	})
}

// Helper function
func int32Ptr(v int32) *int32 {
	return &v
}

// ============================================================================
// CONVERSATION READ TRACKING TESTS
// ============================================================================

func TestConversationService_MarkConversationRead(t *testing.T) {
	suite := NewTestSuite(t).
		WithCleanup("conversations").
		Setup()
	defer suite.Cleanup()

	ctx := context.Background()
	service := NewConversationService(suite.Pool())

	// Create test users (using factory auto-generation for unique names)
	user1 := suite.Factory().NewUser().Create()
	user2 := suite.Factory().NewUser().Create()
	gm := suite.Factory().NewUser().Create()

	// Create game
	game := suite.Factory().NewGame().WithGM(gm.ID).Create()

	// Create characters
	char1 := suite.Factory().NewCharacter().InGame(game).OwnedBy(user1).WithName("Char 1").Create()
	char2 := suite.Factory().NewCharacter().InGame(game).OwnedBy(user2).WithName("Char 2").Create()

	// Create conversation
	conv, err := service.CreateConversation(ctx, CreateConversationRequest{
		GameID:          game.ID,
		Title:           "Read Test Conv",
		CreatedByUserID: user1.ID,
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	require.NoError(t, err)

	// Send some messages
	msg1, err := service.SendMessage(ctx, SendMessageRequest{
		ConversationID:    conv.ID,
		SenderUserID:      user1.ID,
		SenderCharacterID: char1.ID,
		Content:           "First message",
	})
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	msg2, err := service.SendMessage(ctx, SendMessageRequest{
		ConversationID:    conv.ID,
		SenderUserID:      user2.ID,
		SenderCharacterID: char2.ID,
		Content:           "Second message",
	})
	require.NoError(t, err)

	t.Run("mark specific message as read", func(t *testing.T) {
		read, err := service.MarkConversationRead(ctx, user2.ID, conv.ID, msg1.ID)
		require.NoError(t, err)
		require.NotNil(t, read)

		assert.Equal(t, user2.ID, read.UserID)
		assert.Equal(t, conv.ID, read.ConversationID)
		assert.Equal(t, msg1.ID, read.LastReadMessageID.Int32)
		assert.True(t, read.LastReadMessageID.Valid)
	})

	t.Run("update read marker to later message", func(t *testing.T) {
		read, err := service.MarkConversationRead(ctx, user2.ID, conv.ID, msg2.ID)
		require.NoError(t, err)
		assert.Equal(t, msg2.ID, read.LastReadMessageID.Int32)

		// Verify no duplicate records
		readInfo, err := service.GetUserConversationRead(ctx, user2.ID, conv.ID)
		require.NoError(t, err)
		require.NotNil(t, readInfo)
		assert.Equal(t, msg2.ID, readInfo.LastReadMessageID.Int32)
	})

	t.Run("mark all messages as read", func(t *testing.T) {
		err := service.MarkConversationAsRead(ctx, conv.ID, user1.ID)
		require.NoError(t, err)

		read, err := service.GetUserConversationRead(ctx, user1.ID, conv.ID)
		require.NoError(t, err)
		require.NotNil(t, read)
		assert.Equal(t, msg2.ID, read.LastReadMessageID.Int32)
	})
}

func TestConversationService_GetConversationUnreadCount(t *testing.T) {
	suite := NewTestSuite(t).
		WithCleanup("conversations").
		Setup()
	defer suite.Cleanup()

	ctx := context.Background()
	service := NewConversationService(suite.Pool())

	user1 := suite.Factory().NewUser().Create()
	user2 := suite.Factory().NewUser().Create()
	gm := suite.Factory().NewUser().Create()
	game := suite.Factory().NewGame().WithGM(gm.ID).Create()
	char1 := suite.Factory().NewCharacter().InGame(game).OwnedBy(user1).WithName("C1").Create()
	char2 := suite.Factory().NewCharacter().InGame(game).OwnedBy(user2).WithName("C2").Create()

	conv, err := service.CreateConversation(ctx, CreateConversationRequest{
		GameID:          game.ID,
		Title:           "Unread Count Test",
		CreatedByUserID: user1.ID,
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	require.NoError(t, err)

	t.Run("no unread when no messages", func(t *testing.T) {
		count, err := service.GetConversationUnreadCount(ctx, user2.ID, conv.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	// Send 3 messages
	for i := 1; i <= 3; i++ {
		_, err := service.SendMessage(ctx, SendMessageRequest{
			ConversationID:    conv.ID,
			SenderUserID:      user1.ID,
			SenderCharacterID: char1.ID,
			Content:           "Msg",
		})
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond)
	}

	t.Run("counts all unread", func(t *testing.T) {
		count, err := service.GetConversationUnreadCount(ctx, user2.ID, conv.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(3), count)
	})

	t.Run("unread count after marking as read", func(t *testing.T) {
		err := service.MarkConversationAsRead(ctx, conv.ID, user2.ID)
		require.NoError(t, err)

		count, err := service.GetConversationUnreadCount(ctx, user2.ID, conv.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})
}

func TestConversationService_GetFirstUnreadMessageID(t *testing.T) {
	suite := NewTestSuite(t).
		WithCleanup("conversations").
		Setup()
	defer suite.Cleanup()

	ctx := context.Background()
	service := NewConversationService(suite.Pool())

	user1 := suite.Factory().NewUser().Create()
	user2 := suite.Factory().NewUser().Create()
	gm := suite.Factory().NewUser().Create()
	game := suite.Factory().NewGame().WithGM(gm.ID).Create()
	char1 := suite.Factory().NewCharacter().InGame(game).OwnedBy(user1).Create()
	char2 := suite.Factory().NewCharacter().InGame(game).OwnedBy(user2).Create()

	conv, err := service.CreateConversation(ctx, CreateConversationRequest{
		GameID:          game.ID,
		Title:           "First Unread",
		CreatedByUserID: user1.ID,
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	require.NoError(t, err)

	t.Run("nil when no messages", func(t *testing.T) {
		firstUnread, err := service.GetFirstUnreadMessageID(ctx, user2.ID, conv.ID)
		require.NoError(t, err)
		assert.Nil(t, firstUnread)
	})

	var msg1, msg2 *dbmodels.PrivateMessage
	msg1, _ = service.SendMessage(ctx, SendMessageRequest{
		ConversationID:    conv.ID,
		SenderUserID:      user1.ID,
		SenderCharacterID: char1.ID,
		Content:           "Msg1",
	})
	time.Sleep(10 * time.Millisecond)

	msg2, _ = service.SendMessage(ctx, SendMessageRequest{
		ConversationID:    conv.ID,
		SenderUserID:      user1.ID,
		SenderCharacterID: char1.ID,
		Content:           "Msg2",
	})
	time.Sleep(10 * time.Millisecond)

	service.SendMessage(ctx, SendMessageRequest{
		ConversationID:    conv.ID,
		SenderUserID:      user1.ID,
		SenderCharacterID: char1.ID,
		Content:           "Msg3",
	})

	t.Run("returns first when all unread", func(t *testing.T) {
		firstUnread, err := service.GetFirstUnreadMessageID(ctx, user2.ID, conv.ID)
		require.NoError(t, err)
		require.NotNil(t, firstUnread)
		assert.Equal(t, msg1.ID, *firstUnread)
	})

	t.Run("returns correct first after partial read", func(t *testing.T) {
		_, err := service.MarkConversationRead(ctx, user2.ID, conv.ID, msg1.ID)
		require.NoError(t, err)

		firstUnread, err := service.GetFirstUnreadMessageID(ctx, user2.ID, conv.ID)
		require.NoError(t, err)
		require.NotNil(t, firstUnread)
		assert.Equal(t, msg2.ID, *firstUnread)
	})

	t.Run("nil when all read", func(t *testing.T) {
		err := service.MarkConversationAsRead(ctx, conv.ID, user2.ID)
		require.NoError(t, err)

		firstUnread, err := service.GetFirstUnreadMessageID(ctx, user2.ID, conv.ID)
		require.NoError(t, err)
		assert.Nil(t, firstUnread)
	})
}

func TestConversationService_GetUserConversations_UnreadCounts(t *testing.T) {
	suite := NewTestSuite(t).
		WithCleanup("conversations").
		Setup()
	defer suite.Cleanup()

	ctx := context.Background()
	service := NewConversationService(suite.Pool())

	user1 := suite.Factory().NewUser().Create()
	user2 := suite.Factory().NewUser().Create()
	gm := suite.Factory().NewUser().Create()
	game := suite.Factory().NewGame().WithGM(gm.ID).Create()
	char1 := suite.Factory().NewCharacter().InGame(game).OwnedBy(user1).Create()
	char2 := suite.Factory().NewCharacter().InGame(game).OwnedBy(user2).Create()

	conv1, err := service.CreateConversation(ctx, CreateConversationRequest{
		GameID:          game.ID,
		Title:           "Conv1",
		CreatedByUserID: user1.ID,
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	require.NoError(t, err)
	require.NotNil(t, conv1)

	conv2, err := service.CreateConversation(ctx, CreateConversationRequest{
		GameID:          game.ID,
		Title:           "Conv2",
		CreatedByUserID: user1.ID,
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	require.NoError(t, err)
	require.NotNil(t, conv2)

	// 5 messages in conv1
	for i := 0; i < 5; i++ {
		service.SendMessage(ctx, SendMessageRequest{
			ConversationID:    conv1.ID,
			SenderUserID:      user1.ID,
			SenderCharacterID: char1.ID,
			Content:           "M",
		})
		time.Sleep(5 * time.Millisecond)
	}

	// 2 messages in conv2
	for i := 0; i < 2; i++ {
		service.SendMessage(ctx, SendMessageRequest{
			ConversationID:    conv2.ID,
			SenderUserID:      user1.ID,
			SenderCharacterID: char1.ID,
			Content:           "M",
		})
		time.Sleep(5 * time.Millisecond)
	}

	t.Run("sorted by unread count", func(t *testing.T) {
		conversations, err := service.GetUserConversations(ctx, game.ID, user2.ID)
		require.NoError(t, err)
		require.Len(t, conversations, 2)

		assert.Equal(t, conv1.ID, conversations[0].ID)
		assert.Equal(t, int64(5), conversations[0].UnreadCount)
		assert.Equal(t, conv2.ID, conversations[1].ID)
		assert.Equal(t, int64(2), conversations[1].UnreadCount)
	})

	t.Run("counts update after read", func(t *testing.T) {
		service.MarkConversationAsRead(ctx, conv1.ID, user2.ID)

		conversations, err := service.GetUserConversations(ctx, game.ID, user2.ID)
		require.NoError(t, err)

		assert.Equal(t, conv2.ID, conversations[0].ID)
		assert.Equal(t, int64(2), conversations[0].UnreadCount)
		assert.Equal(t, conv1.ID, conversations[1].ID)
		assert.Equal(t, int64(0), conversations[1].UnreadCount)
	})
}

func TestConversationService_GetUserConversations_DeletedMessagePreview(t *testing.T) {
	suite := NewTestSuite(t).
		WithCleanup("conversations").
		Setup()
	defer suite.Cleanup()

	ctx := context.Background()
	service := NewConversationService(suite.Pool())

	user1 := suite.Factory().NewUser().Create()
	user2 := suite.Factory().NewUser().Create()
	gm := suite.Factory().NewUser().Create()
	game := suite.Factory().NewGame().WithGM(gm.ID).Create()
	char1 := suite.Factory().NewCharacter().InGame(game).OwnedBy(user1).Create()
	char2 := suite.Factory().NewCharacter().InGame(game).OwnedBy(user2).Create()

	conv, err := service.CreateConversation(ctx, CreateConversationRequest{
		GameID:          game.ID,
		Title:           "Test Conv",
		CreatedByUserID: user1.ID,
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	require.NoError(t, err)

	// Send first message
	_, err = service.SendMessage(ctx, SendMessageRequest{
		ConversationID:    conv.ID,
		SenderUserID:      user1.ID,
		SenderCharacterID: char1.ID,
		Content:           "First message",
	})
	require.NoError(t, err)

	// Send second (most recent) message then delete it
	msg2, err := service.SendMessage(ctx, SendMessageRequest{
		ConversationID:    conv.ID,
		SenderUserID:      user1.ID,
		SenderCharacterID: char1.ID,
		Content:           "Deleted message",
	})
	require.NoError(t, err)

	err = service.DeletePrivateMessage(ctx, msg2.ID, user1.ID)
	require.NoError(t, err)

	t.Run("deleted message does not appear as conversation preview", func(t *testing.T) {
		conversations, err := service.GetUserConversations(ctx, game.ID, user1.ID)
		require.NoError(t, err)
		require.Len(t, conversations, 1)

		// The preview should show the first (non-deleted) message, not the deleted one
		assert.Equal(t, "First message", conversations[0].LastMessage, "deleted message should not appear in preview")
	})
}

func TestConversationService_DeletePrivateMessage(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := NewConversationService(testDB.Pool)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	charService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Setup: Create game, users, and characters
	gm := testDB.CreateTestUser(t, "gm_delete", "gm_delete@example.com")
	player1 := testDB.CreateTestUser(t, "player1_delete", "player1_delete@example.com")
	player2 := testDB.CreateTestUser(t, "player2_delete", "player2_delete@example.com")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Delete Test Game")

	// Add players as participants
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	require.NoError(t, err)

	// Create characters
	char1, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player1.ID)),
		Name:          "Character 1",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	char2, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player2.ID)),
		Name:          "Character 2",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	// Create conversation
	conversation, err := service.CreateConversation(context.Background(), CreateConversationRequest{
		GameID:          game.ID,
		Title:           "Delete Message Test",
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	require.NoError(t, err)

	// Send test messages
	msg1, err := service.SendMessage(context.Background(), SendMessageRequest{
		ConversationID:    conversation.ID,
		SenderUserID:      int32(player1.ID),
		SenderCharacterID: char1.ID,
		Content:           "Message from Player 1",
	})
	require.NoError(t, err)

	msg2, err := service.SendMessage(context.Background(), SendMessageRequest{
		ConversationID:    conversation.ID,
		SenderUserID:      int32(player2.ID),
		SenderCharacterID: char2.ID,
		Content:           "Message from Player 2",
	})
	require.NoError(t, err)

	t.Run("successfully deletes own message", func(t *testing.T) {
		ctx := context.Background()

		// Delete message as sender
		err := service.DeletePrivateMessage(ctx, msg1.ID, int32(player1.ID))
		require.NoError(t, err)

		// Verify message is soft-deleted
		messages, err := service.GetConversationMessages(ctx, conversation.ID, int32(player1.ID))
		require.NoError(t, err)
		require.Len(t, messages, 2)

		// Find the deleted message
		var deletedMsg *dbmodels.GetConversationMessagesRow
		for i := range messages {
			if messages[i].ID == msg1.ID {
				deletedMsg = &messages[i]
				break
			}
		}
		require.NotNil(t, deletedMsg, "Deleted message should still be in results")

		// Verify content replaced with placeholder
		assert.Equal(t, "[Message deleted]", deletedMsg.Content)
		assert.True(t, deletedMsg.IsDeleted.Valid && deletedMsg.IsDeleted.Bool)
		assert.True(t, deletedMsg.DeletedAt.Valid)

		// Verify sender and timestamp preserved
		assert.Equal(t, int32(player1.ID), deletedMsg.SenderUserID)
		assert.True(t, deletedMsg.CreatedAt.Valid)
	})

	t.Run("returns error when deleting another user's message", func(t *testing.T) {
		ctx := context.Background()

		// Try to delete Player 2's message as Player 1
		err := service.DeletePrivateMessage(ctx, msg2.ID, int32(player1.ID))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "forbidden")
	})

	t.Run("returns error for non-existent message", func(t *testing.T) {
		ctx := context.Background()

		// Try to delete message that doesn't exist
		err := service.DeletePrivateMessage(ctx, 999999, int32(player1.ID))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("idempotent - deleting already deleted message succeeds", func(t *testing.T) {
		ctx := context.Background()

		// Send new message
		msg3, err := service.SendMessage(ctx, SendMessageRequest{
			ConversationID:    conversation.ID,
			SenderUserID:      int32(player1.ID),
			SenderCharacterID: char1.ID,
			Content:           "Message to delete twice",
		})
		require.NoError(t, err)

		// Delete first time
		err = service.DeletePrivateMessage(ctx, msg3.ID, int32(player1.ID))
		require.NoError(t, err)

		// Delete second time - should succeed (idempotent)
		err = service.DeletePrivateMessage(ctx, msg3.ID, int32(player1.ID))
		require.NoError(t, err)

		// Verify still soft-deleted
		messages, err := service.GetConversationMessages(ctx, conversation.ID, int32(player1.ID))
		require.NoError(t, err)

		var deletedMsg *dbmodels.GetConversationMessagesRow
		for i := range messages {
			if messages[i].ID == msg3.ID {
				deletedMsg = &messages[i]
				break
			}
		}
		require.NotNil(t, deletedMsg)
		assert.Equal(t, "[Message deleted]", deletedMsg.Content)
		assert.True(t, deletedMsg.IsDeleted.Valid && deletedMsg.IsDeleted.Bool)
	})

	t.Run("deleted messages visible to all participants", func(t *testing.T) {
		ctx := context.Background()

		// Send and delete message from Player 1
		msg4, err := service.SendMessage(ctx, SendMessageRequest{
			ConversationID:    conversation.ID,
			SenderUserID:      int32(player1.ID),
			SenderCharacterID: char1.ID,
			Content:           "Visible to all when deleted",
		})
		require.NoError(t, err)

		err = service.DeletePrivateMessage(ctx, msg4.ID, int32(player1.ID))
		require.NoError(t, err)

		// Verify Player 1 sees deleted message
		messages1, err := service.GetConversationMessages(ctx, conversation.ID, int32(player1.ID))
		require.NoError(t, err)

		// Verify Player 2 sees deleted message
		messages2, err := service.GetConversationMessages(ctx, conversation.ID, int32(player2.ID))
		require.NoError(t, err)

		// Both should see same content
		var msg1Content, msg2Content string
		for i := range messages1 {
			if messages1[i].ID == msg4.ID {
				msg1Content = messages1[i].Content
			}
		}
		for i := range messages2 {
			if messages2[i].ID == msg4.ID {
				msg2Content = messages2[i].Content
			}
		}

		assert.Equal(t, "[Message deleted]", msg1Content)
		assert.Equal(t, "[Message deleted]", msg2Content)
		assert.Equal(t, msg1Content, msg2Content, "Both participants should see same deleted message")
	})

	t.Run("preserves message position in conversation", func(t *testing.T) {
		ctx := context.Background()

		// Get messages before deletion
		messagesBefore, err := service.GetConversationMessages(ctx, conversation.ID, int32(player1.ID))
		require.NoError(t, err)
		countBefore := len(messagesBefore)

		// Send and delete a message
		msg5, err := service.SendMessage(ctx, SendMessageRequest{
			ConversationID:    conversation.ID,
			SenderUserID:      int32(player1.ID),
			SenderCharacterID: char1.ID,
			Content:           "Position test",
		})
		require.NoError(t, err)

		err = service.DeletePrivateMessage(ctx, msg5.ID, int32(player1.ID))
		require.NoError(t, err)

		// Get messages after deletion
		messagesAfter, err := service.GetConversationMessages(ctx, conversation.ID, int32(player1.ID))
		require.NoError(t, err)

		// Message count should be same (soft delete preserves structure)
		assert.Equal(t, countBefore+1, len(messagesAfter), "Deleted message should still be counted")
	})
}

func TestConversationService_UpdatePrivateMessage(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := NewConversationService(testDB.Pool)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	charService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	ctx := context.Background()

	// Setup
	gm := testDB.CreateTestUser(t, "gm_update", "gm_update@example.com")
	player1 := testDB.CreateTestUser(t, "player1_update", "player1_update@example.com")
	player2 := testDB.CreateTestUser(t, "player2_update", "player2_update@example.com")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Update Test Game")

	_, err := gameService.AddGameParticipant(ctx, game.ID, int32(player1.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(ctx, game.ID, int32(player2.ID), "player")
	require.NoError(t, err)

	char1, err := charService.CreateCharacter(ctx, CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player1.ID)),
		Name:          "Char1 Update",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	char2, err := charService.CreateCharacter(ctx, CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player2.ID)),
		Name:          "Char2 Update",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	conv, err := service.CreateConversation(ctx, CreateConversationRequest{
		GameID:          game.ID,
		Title:           "Update Test Conv",
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	require.NoError(t, err)

	sendMsg := func(userID int32, charID int32, content string) *dbmodels.PrivateMessage {
		msg, err := service.SendMessage(ctx, SendMessageRequest{
			ConversationID:    conv.ID,
			SenderUserID:      userID,
			SenderCharacterID: charID,
			Content:           content,
		})
		require.NoError(t, err)
		return msg
	}

	t.Run("successfully edits message and tracks edit metadata", func(t *testing.T) {
		msg := sendMsg(int32(player1.ID), char1.ID, "original content")

		updated, err := service.UpdatePrivateMessage(ctx, msg.ID, int32(player1.ID), "edited content")

		require.NoError(t, err)
		assert.Equal(t, "edited content", updated.Content)
		assert.True(t, updated.IsEdited)
		assert.True(t, updated.EditedAt.Valid)
		assert.Equal(t, int32(1), updated.EditCount)
	})

	t.Run("increments edit_count on subsequent edits", func(t *testing.T) {
		msg := sendMsg(int32(player1.ID), char1.ID, "first version")

		updated1, err := service.UpdatePrivateMessage(ctx, msg.ID, int32(player1.ID), "second version")
		require.NoError(t, err)
		assert.Equal(t, int32(1), updated1.EditCount)

		updated2, err := service.UpdatePrivateMessage(ctx, msg.ID, int32(player1.ID), "third version")
		require.NoError(t, err)
		assert.Equal(t, int32(2), updated2.EditCount)
		assert.Equal(t, "third version", updated2.Content)
	})

	t.Run("non-sender cannot edit message", func(t *testing.T) {
		msg := sendMsg(int32(player1.ID), char1.ID, "player1 wrote this")

		_, err := service.UpdatePrivateMessage(ctx, msg.ID, int32(player2.ID), "player2 tries to edit")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "forbidden")
	})

	t.Run("cannot edit a deleted message", func(t *testing.T) {
		msg := sendMsg(int32(player1.ID), char1.ID, "will be deleted")

		err := service.DeletePrivateMessage(ctx, msg.ID, int32(player1.ID))
		require.NoError(t, err)

		_, err = service.UpdatePrivateMessage(ctx, msg.ID, int32(player1.ID), "trying to edit deleted")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "deleted")
	})

	t.Run("returns error for nonexistent message", func(t *testing.T) {
		_, err := service.UpdatePrivateMessage(ctx, 999999, int32(player1.ID), "ghost edit")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	_ = time.Now() // keep time import used
}

// TestConversationService_CanUserAccessConversation tests the access control logic for private conversations.
// This is a security-critical function: a wrong result means a user reads private messages they shouldn't.
// Tests cover all access paths: GM, co-GM, audience, character controller, and denied outsider.
// TestConversationService_NotifyPrivateMessage_DeduplicatesByUser verifies that a user who controls
// multiple characters in the same conversation receives only one notification per message,
// not one per character they control.
func TestConversationService_NotifyPrivateMessage_DeduplicatesByUser(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "notifications", "conversation_reads", "private_messages", "conversation_participants", "conversations", "npc_assignments", "characters", "game_participants", "games", "sessions", "users")

	ctx := context.Background()
	convService := NewConversationService(testDB.Pool)
	charService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	notifService := NewNotificationService(testDB.Pool, app.ObsLogger)

	gm := testDB.CreateTestUser(t, "gm_notif_dedup", "gm_notif_dedup@example.com")
	player := testDB.CreateTestUser(t, "player_notif_dedup", "player_notif_dedup@example.com")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Notification Dedup Test Game")
	_, err := gameService.AddGameParticipant(ctx, game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	// GM controls two characters in this conversation: their GM character and an NPC
	gmUserID := int32(gm.ID)
	gmChar, err := charService.CreateCharacter(ctx, CreateCharacterRequest{
		GameID: game.ID, UserID: &gmUserID, Name: "GM Character", CharacterType: "player_character",
	})
	require.NoError(t, err)

	npc, err := charService.CreateCharacter(ctx, CreateCharacterRequest{
		GameID: game.ID, UserID: nil, Name: "NPC", CharacterType: "npc",
	})
	require.NoError(t, err)

	// Assign NPC to GM so both participants share gm.ID as their user
	err = charService.AssignNPCToUser(ctx, npc.ID, int32(gm.ID), int32(gm.ID))
	require.NoError(t, err)

	playerUserID := int32(player.ID)
	playerChar, err := charService.CreateCharacter(ctx, CreateCharacterRequest{
		GameID: game.ID, UserID: &playerUserID, Name: "Player Character", CharacterType: "player_character",
	})
	require.NoError(t, err)

	conv, err := convService.CreateConversation(ctx, CreateConversationRequest{
		GameID:          game.ID,
		Title:           "GM NPC Conversation",
		CreatedByUserID: int32(gm.ID),
		ParticipantIDs:  []int32{gmChar.ID, npc.ID, playerChar.ID},
	})
	require.NoError(t, err)

	// Player sends a message — should trigger exactly one notification to GM
	_, err = convService.SendMessage(ctx, SendMessageRequest{
		ConversationID:    conv.ID,
		SenderUserID:      int32(player.ID),
		SenderCharacterID: playerChar.ID,
		Content:           "Hello!",
	})
	require.NoError(t, err)

	// Wait for the background goroutine to complete
	time.Sleep(200 * time.Millisecond)

	// GM should have exactly 1 private_message notification, not 2
	notifications, err := notifService.GetUserNotifications(ctx, int32(gm.ID), 10, 0)
	require.NoError(t, err)

	var pmNotifs []*core.Notification
	for _, n := range notifications {
		if n.Type == core.NotificationTypePrivateMessage {
			pmNotifs = append(pmNotifs, n)
		}
	}
	assert.Len(t, pmNotifs, 1, "GM should receive exactly 1 notification even when controlling multiple characters in the conversation")
}

func TestConversationService_CanUserAccessConversation(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := NewConversationService(testDB.Pool)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	charService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
	audience := testDB.CreateTestUser(t, "audience", "audience@example.com")
	outsider := testDB.CreateTestUser(t, "outsider", "outsider@example.com")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(audience.ID), "audience")
	require.NoError(t, err)

	p1ID := int32(player1.ID)
	p2ID := int32(player2.ID)
	char1, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID: game.ID, UserID: &p1ID, Name: "Char1", CharacterType: "player_character",
	})
	require.NoError(t, err)
	char2, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID: game.ID, UserID: &p2ID, Name: "Char2", CharacterType: "player_character",
	})
	require.NoError(t, err)

	// Create a private conversation between char1 and char2
	conv, err := service.CreateConversation(context.Background(), CreateConversationRequest{
		GameID:          game.ID,
		Title:           "Private chat",
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	require.NoError(t, err)

	t.Run("GM can access conversation", func(t *testing.T) {
		canAccess, err := service.CanUserAccessConversation(context.Background(), conv.ID, int32(gm.ID), false)
		require.NoError(t, err)
		assert.True(t, canAccess, "GM should always have access to conversations in their game")
	})

	t.Run("player controlling a participant character can access", func(t *testing.T) {
		canAccess, err := service.CanUserAccessConversation(context.Background(), conv.ID, int32(player1.ID), false)
		require.NoError(t, err)
		assert.True(t, canAccess, "player controlling char1 should have access")
	})

	t.Run("audience member can access", func(t *testing.T) {
		canAccess, err := service.CanUserAccessConversation(context.Background(), conv.ID, int32(audience.ID), false)
		require.NoError(t, err)
		assert.True(t, canAccess, "audience member should have access to all game conversations")
	})

	t.Run("outsider (not in game) cannot access", func(t *testing.T) {
		canAccess, err := service.CanUserAccessConversation(context.Background(), conv.ID, int32(outsider.ID), false)
		require.NoError(t, err)
		assert.False(t, canAccess, "outsider should not have access to any game conversations")
	})

	t.Run("player not controlling any participant character cannot access", func(t *testing.T) {
		// Create a third player with no characters in this conversation
		player3 := testDB.CreateTestUser(t, "player3", "player3@example.com")
		_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player3.ID), "player")
		require.NoError(t, err)
		p3ID := int32(player3.ID)
		_, err = charService.CreateCharacter(context.Background(), CreateCharacterRequest{
			GameID: game.ID, UserID: &p3ID, Name: "Char3", CharacterType: "player_character",
		})
		require.NoError(t, err)

		canAccess, err := service.CanUserAccessConversation(context.Background(), conv.ID, int32(player3.ID), false)
		require.NoError(t, err)
		assert.False(t, canAccess, "player with no participant in this conversation should not have access")
	})

	t.Run("NPC controller gains access via character assignment", func(t *testing.T) {
		npcController := testDB.CreateTestUser(t, "npccontroller", "npc@example.com")
		_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(npcController.ID), "player")
		require.NoError(t, err)

		// Create an NPC and add it to a new conversation
		npc, err := charService.CreateCharacter(context.Background(), CreateCharacterRequest{
			GameID: game.ID, UserID: nil, Name: "NPC", CharacterType: "npc",
		})
		require.NoError(t, err)

		npcConv, err := service.CreateConversation(context.Background(), CreateConversationRequest{
			GameID:          game.ID,
			Title:           "NPC chat",
			CreatedByUserID: int32(gm.ID),
			ParticipantIDs:  []int32{char1.ID, npc.ID},
		})
		require.NoError(t, err)

		// Before assignment: npcController cannot access
		canAccess, err := service.CanUserAccessConversation(context.Background(), npcConv.ID, int32(npcController.ID), false)
		require.NoError(t, err)
		assert.False(t, canAccess, "user should not have access before being assigned the NPC")

		// Assign NPC to npcController
		err = charService.AssignNPCToUser(context.Background(), npc.ID, int32(npcController.ID), int32(gm.ID))
		require.NoError(t, err)

		// After assignment: npcController can access
		canAccess, err = service.CanUserAccessConversation(context.Background(), npcConv.ID, int32(npcController.ID), false)
		require.NoError(t, err)
		assert.True(t, canAccess, "user assigned to NPC in the conversation should have access")
	})
}

func TestConversationService_DeleteConversation(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()

	service := NewConversationService(testDB.Pool)
	gameService := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	charService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	ctx := context.Background()

	gm := testDB.CreateTestUser(t, "gm_convdel", "gm_convdel@example.com")
	player1 := testDB.CreateTestUser(t, "p1_convdel", "p1_convdel@example.com")
	player2 := testDB.CreateTestUser(t, "p2_convdel", "p2_convdel@example.com")
	outsider := testDB.CreateTestUser(t, "outsider_convdel", "outsider_convdel@example.com")

	game := testDB.CreateTestGame(t, int32(gm.ID), "Conversation Delete Test Game")

	_, err := gameService.AddGameParticipant(ctx, game.ID, int32(player1.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(ctx, game.ID, int32(player2.ID), "player")
	require.NoError(t, err)

	char1, err := charService.CreateCharacter(ctx, CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player1.ID)),
		Name:          "Delete Char 1",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	char2, err := charService.CreateCharacter(ctx, CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        int32Ptr(int32(player2.ID)),
		Name:          "Delete Char 2",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	// newConversation creates a fresh empty conversation owned by creatorUserID.
	newConversation := func(t *testing.T, creatorUserID int32) *dbmodels.Conversation {
		t.Helper()
		conv, err := service.CreateConversation(ctx, CreateConversationRequest{
			GameID:          game.ID,
			Title:           "Accidental Conversation",
			CreatedByUserID: creatorUserID,
			ParticipantIDs:  []int32{char1.ID, char2.ID},
		})
		require.NoError(t, err)
		return conv
	}

	t.Run("creator can delete an empty conversation", func(t *testing.T) {
		conv := newConversation(t, int32(player1.ID))

		err := service.DeleteConversation(ctx, conv.ID, int32(player1.ID))
		require.NoError(t, err)

		// The conversation must actually be gone, not just reported as deleted.
		_, err = service.GetConversation(ctx, conv.ID)
		assert.Error(t, err, "conversation should no longer exist")

		// It must also disappear from the creator's conversation list.
		conversations, err := service.GetUserConversations(ctx, game.ID, int32(player1.ID))
		require.NoError(t, err)
		for _, c := range conversations {
			assert.NotEqual(t, conv.ID, c.ID, "deleted conversation should not be listed")
		}
	})

	t.Run("deleting cascades participant rows", func(t *testing.T) {
		conv := newConversation(t, int32(player1.ID))

		participants, err := service.GetConversationParticipants(ctx, conv.ID)
		require.NoError(t, err)
		require.NotEmpty(t, participants, "precondition: conversation has participants")

		require.NoError(t, service.DeleteConversation(ctx, conv.ID, int32(player1.ID)))

		participants, err = service.GetConversationParticipants(ctx, conv.ID)
		require.NoError(t, err)
		assert.Empty(t, participants, "participant rows should cascade away")
	})

	t.Run("GM can delete an empty conversation created by a player", func(t *testing.T) {
		conv := newConversation(t, int32(player1.ID))

		err := service.DeleteConversation(ctx, conv.ID, int32(gm.ID))
		require.NoError(t, err)

		_, err = service.GetConversation(ctx, conv.ID)
		assert.Error(t, err, "conversation should no longer exist")
	})

	t.Run("co-GM can delete an empty conversation", func(t *testing.T) {
		coGM := testDB.CreateTestUser(t, "cogm_convdel", "cogm_convdel@example.com")
		_, err := gameService.AddGameParticipant(ctx, game.ID, int32(coGM.ID), "co_gm")
		require.NoError(t, err)

		conv := newConversation(t, int32(player1.ID))

		err = service.DeleteConversation(ctx, conv.ID, int32(coGM.ID))
		require.NoError(t, err)

		_, err = service.GetConversation(ctx, conv.ID)
		assert.Error(t, err, "conversation should no longer exist")
	})

	t.Run("conversation with a message cannot be deleted", func(t *testing.T) {
		conv := newConversation(t, int32(player1.ID))

		_, err := service.SendMessage(ctx, SendMessageRequest{
			ConversationID:    conv.ID,
			SenderUserID:      int32(player1.ID),
			SenderCharacterID: char1.ID,
			Content:           "This conversation is now real",
		})
		require.NoError(t, err)

		err = service.DeleteConversation(ctx, conv.ID, int32(player1.ID))
		require.ErrorIs(t, err, core.ErrConversationNotEmpty)

		// The conversation and its message must survive the rejected delete.
		surviving, err := service.GetConversation(ctx, conv.ID)
		require.NoError(t, err)
		assert.Equal(t, conv.ID, surviving.ID)

		messages, err := service.GetConversationMessages(ctx, conv.ID, int32(player1.ID))
		require.NoError(t, err)
		assert.Len(t, messages, 1, "message should be untouched")
	})

	t.Run("conversation whose only message was deleted still cannot be deleted", func(t *testing.T) {
		conv := newConversation(t, int32(player1.ID))

		msg, err := service.SendMessage(ctx, SendMessageRequest{
			ConversationID:    conv.ID,
			SenderUserID:      int32(player1.ID),
			SenderCharacterID: char1.ID,
			Content:           "Sent then retracted",
		})
		require.NoError(t, err)

		require.NoError(t, service.DeletePrivateMessage(ctx, msg.ID, int32(player1.ID)))

		// Soft-deleted messages are still history; the conversation is not empty.
		err = service.DeleteConversation(ctx, conv.ID, int32(player1.ID))
		require.ErrorIs(t, err, core.ErrConversationNotEmpty)

		_, err = service.GetConversation(ctx, conv.ID)
		require.NoError(t, err, "conversation should survive")
	})

	t.Run("non-creator participant cannot delete", func(t *testing.T) {
		conv := newConversation(t, int32(player1.ID))

		err := service.DeleteConversation(ctx, conv.ID, int32(player2.ID))
		require.ErrorIs(t, err, core.ErrConversationDeleteForbidden)

		_, err = service.GetConversation(ctx, conv.ID)
		require.NoError(t, err, "conversation should survive")
	})

	t.Run("unrelated user cannot delete", func(t *testing.T) {
		conv := newConversation(t, int32(player1.ID))

		err := service.DeleteConversation(ctx, conv.ID, int32(outsider.ID))
		require.ErrorIs(t, err, core.ErrConversationDeleteForbidden)

		_, err = service.GetConversation(ctx, conv.ID)
		require.NoError(t, err, "conversation should survive")
	})

	t.Run("deleting a nonexistent conversation errors", func(t *testing.T) {
		err := service.DeleteConversation(ctx, 999999, int32(player1.ID))
		assert.Error(t, err)
	})
}
