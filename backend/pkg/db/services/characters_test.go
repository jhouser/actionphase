package db

import (
	"context"
	"testing"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCharacterService_CreateCharacter(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "games", "sessions", "users")

	// Setup test fixtures
	fixtures := testDB.SetupFixtures(t)
	characterService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	testCases := []struct {
		name        string
		request     CreateCharacterRequest
		expectError bool
		reason      string
	}{
		{
			name: "valid player character creation",
			request: CreateCharacterRequest{
				GameID:        fixtures.TestGame.ID,
				UserID:        core.Int32Ptr(int32(fixtures.TestUser.ID)),
				Name:          "Aragorn",
				CharacterType: "player_character",
			},
			expectError: false,
		},
		{
			name: "valid GM NPC creation",
			request: CreateCharacterRequest{
				GameID:        fixtures.TestGame.ID,
				UserID:        nil, // GM-controlled NPC
				Name:          "Gandalf",
				CharacterType: "npc",
			},
			expectError: false,
		},
		{
			name: "valid audience NPC creation",
			request: CreateCharacterRequest{
				GameID:        fixtures.TestGame.ID,
				UserID:        nil,
				Name:          "Boromir",
				CharacterType: "npc",
			},
			expectError: false,
		},
		{
			name: "invalid character type",
			request: CreateCharacterRequest{
				GameID:        fixtures.TestGame.ID,
				UserID:        core.Int32Ptr(int32(fixtures.TestUser.ID)),
				Name:          "Invalid Character",
				CharacterType: "invalid_type",
			},
			expectError: true,
			reason:      "should reject invalid character type",
		},
		{
			name: "player character without user ID",
			request: CreateCharacterRequest{
				GameID:        fixtures.TestGame.ID,
				UserID:        nil,
				Name:          "Orphan Character",
				CharacterType: "player_character",
			},
			expectError: true,
			reason:      "player character requires user ID",
		},
		{
			name: "nonexistent game",
			request: CreateCharacterRequest{
				GameID:        99999,
				UserID:        core.Int32Ptr(int32(fixtures.TestUser.ID)),
				Name:          "Lost Character",
				CharacterType: "player_character",
			},
			expectError: true,
			reason:      "should reject nonexistent game",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			character, err := characterService.CreateCharacter(context.Background(), tc.request)

			if tc.expectError {
				core.AssertError(t, err, tc.reason)
			} else {
				core.AssertNoError(t, err, "Failed to create character")
				core.AssertEqual(t, tc.request.Name, character.Name, "Character name mismatch")
				core.AssertEqual(t, tc.request.CharacterType, character.CharacterType, "Character type mismatch")
				core.AssertEqual(t, "pending", character.Status.String, "Character should start with pending status")

				if tc.request.UserID != nil {
					core.AssertEqual(t, true, character.UserID.Valid, "Character should have user ID")
					core.AssertEqual(t, *tc.request.UserID, character.UserID.Int32, "User ID mismatch")
				} else {
					core.AssertEqual(t, false, character.UserID.Valid, "Character should not have user ID")
				}
			}
		})
	}
}

func TestCharacterService_GetCharacter(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "games", "sessions", "users")

	// Setup test fixtures
	fixtures := testDB.SetupFixtures(t)
	characterService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test character
	character, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        fixtures.TestGame.ID,
		UserID:        core.Int32Ptr(int32(fixtures.TestUser.ID)),
		Name:          "Test Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create test character")

	testCases := []struct {
		name        string
		characterID int32
		expectError bool
		reason      string
	}{
		{
			name:        "valid character retrieval",
			characterID: character.ID,
			expectError: false,
		},
		{
			name:        "nonexistent character",
			characterID: 99999,
			expectError: true,
			reason:      "should fail for nonexistent character",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			retrieved, err := characterService.GetCharacter(context.Background(), tc.characterID)

			if tc.expectError {
				core.AssertError(t, err, tc.reason)
			} else {
				core.AssertNoError(t, err, "Failed to get character")
				core.AssertEqual(t, character.ID, retrieved.ID, "Character ID mismatch")
				core.AssertEqual(t, character.Name, retrieved.Name, "Character name mismatch")
			}
		})
	}
}

func TestCharacterService_GetCharactersByGame(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "games", "sessions", "users")

	// Setup test fixtures
	fixtures := testDB.SetupFixtures(t)
	characterService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test characters
	_, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        fixtures.TestGame.ID,
		UserID:        core.Int32Ptr(int32(fixtures.TestUser.ID)),
		Name:          "Player Character 1",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create player character")

	_, err = characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        fixtures.TestGame.ID,
		UserID:        nil,
		Name:          "GM NPC 1",
		CharacterType: "npc",
	})
	core.AssertNoError(t, err, "Failed to create GM NPC")

	// Test retrieval
	characters, err := characterService.GetCharactersByGame(context.Background(), fixtures.TestGame.ID)
	core.AssertNoError(t, err, "Failed to get characters by game")

	// Should have 2 characters
	core.AssertEqual(t, 2, len(characters), "Expected 2 characters")

	// Test empty game
	emptyGame := testDB.CreateTestGame(t, int32(fixtures.TestUser.ID), "Empty Game")
	emptyCharacters, err := characterService.GetCharactersByGame(context.Background(), emptyGame.ID)
	core.AssertNoError(t, err, "Failed to get characters for empty game")
	core.AssertEqual(t, 0, len(emptyCharacters), "Expected 0 characters for empty game")
}

func TestCharacterService_ApproveRejectCharacter(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "games", "sessions", "users")

	// Setup test fixtures
	fixtures := testDB.SetupFixtures(t)
	characterService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test character
	character, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        fixtures.TestGame.ID,
		UserID:        core.Int32Ptr(int32(fixtures.TestUser.ID)),
		Name:          "Test Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create test character")

	// Test approval
	t.Run("approve character", func(t *testing.T) {
		approved, err := characterService.ApproveCharacter(context.Background(), character.ID)
		core.AssertNoError(t, err, "Failed to approve character")
		core.AssertEqual(t, "approved", approved.Status.String, "Character should be approved")
	})

	// Test approval of nonexistent character
	t.Run("approve nonexistent character", func(t *testing.T) {
		_, err := characterService.ApproveCharacter(context.Background(), 99999)
		core.AssertError(t, err, "Should fail for nonexistent character")
	})
}

func TestCharacterService_AssignNPCToUser(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "games", "sessions", "users")

	// Setup test fixtures
	fixtures := testDB.SetupFixtures(t)
	characterService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create additional user for assignment
	assignedUser := testDB.CreateTestUser(t, "assigneduser", "assigned@example.com")

	// Create NPC character
	npcCharacter, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        fixtures.TestGame.ID,
		UserID:        nil,
		Name:          "Test NPC",
		CharacterType: "npc",
	})
	core.AssertNoError(t, err, "Failed to create NPC character")

	testCases := []struct {
		name             string
		characterID      int32
		assignedUserID   int32
		assignedByUserID int32
		expectError      bool
		reason           string
	}{
		{
			name:             "valid NPC assignment",
			characterID:      npcCharacter.ID,
			assignedUserID:   int32(assignedUser.ID),
			assignedByUserID: int32(fixtures.TestUser.ID),
			expectError:      false,
		},
		{
			name:             "assign nonexistent NPC",
			characterID:      99999,
			assignedUserID:   int32(assignedUser.ID),
			assignedByUserID: int32(fixtures.TestUser.ID),
			expectError:      true,
			reason:           "should fail for nonexistent character",
		},
		{
			name:             "assign to nonexistent user",
			characterID:      npcCharacter.ID,
			assignedUserID:   99999,
			assignedByUserID: int32(fixtures.TestUser.ID),
			expectError:      true,
			reason:           "should fail for nonexistent user",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := characterService.AssignNPCToUser(context.Background(), tc.characterID, tc.assignedUserID, tc.assignedByUserID)

			if tc.expectError {
				core.AssertError(t, err, tc.reason)
			} else {
				core.AssertNoError(t, err, "Failed to assign NPC to user")
			}
		})
	}

	// Test assigning player character (should fail)
	t.Run("assign player character", func(t *testing.T) {
		playerCharacter, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
			GameID:        fixtures.TestGame.ID,
			UserID:        core.Int32Ptr(int32(fixtures.TestUser.ID)),
			Name:          "Player Character",
			CharacterType: "player_character",
		})
		core.AssertNoError(t, err, "Failed to create player character")

		err = characterService.AssignNPCToUser(context.Background(), playerCharacter.ID, int32(assignedUser.ID), int32(fixtures.TestUser.ID))
		core.AssertError(t, err, "Should not be able to assign player character")
	})
}

func TestCharacterService_CharacterData(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "games", "sessions", "users")

	// Setup test fixtures
	fixtures := testDB.SetupFixtures(t)
	characterService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test character
	character, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        fixtures.TestGame.ID,
		UserID:        core.Int32Ptr(int32(fixtures.TestUser.ID)),
		Name:          "Test Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create test character")

	// Test setting character data
	t.Run("set character data", func(t *testing.T) {
		err := characterService.SetCharacterData(context.Background(), CharacterDataRequest{
			CharacterID: character.ID,
			ModuleType:  "bio",
			FieldName:   "background",
			FieldValue:  "A brave warrior from the north",
			FieldType:   "text",
			IsPublic:    true,
		})
		core.AssertNoError(t, err, "Failed to set character data")

		// Set private data
		err = characterService.SetCharacterData(context.Background(), CharacterDataRequest{
			CharacterID: character.ID,
			ModuleType:  "notes",
			FieldName:   "private_notes",
			FieldValue:  "Secret weakness: afraid of spiders",
			FieldType:   "text",
			IsPublic:    false,
		})
		core.AssertNoError(t, err, "Failed to set private character data")
	})

	// Test getting all character data
	t.Run("get all character data", func(t *testing.T) {
		data, err := characterService.GetCharacterData(context.Background(), character.ID)
		core.AssertNoError(t, err, "Failed to get character data")
		core.AssertEqual(t, 2, len(data), "Expected 2 data entries")
	})

	// Test getting public character data only
	t.Run("get public character data", func(t *testing.T) {
		data, err := characterService.GetPublicCharacterData(context.Background(), character.ID)
		core.AssertNoError(t, err, "Failed to get public character data")
		core.AssertEqual(t, 1, len(data), "Expected 1 public data entry")
		core.AssertEqual(t, true, data[0].IsPublic.Bool, "Data should be public")
	})

	// Test getting character data by module
	t.Run("get character data by module", func(t *testing.T) {
		data, err := characterService.GetCharacterDataByModule(context.Background(), character.ID, "bio")
		core.AssertNoError(t, err, "Failed to get character data by module")
		core.AssertEqual(t, 1, len(data), "Expected 1 bio data entry")
		core.AssertEqual(t, "bio", data[0].ModuleType, "Module type mismatch")
	})

	// Test updating existing character data (upsert behavior)
	t.Run("update character data", func(t *testing.T) {
		err := characterService.SetCharacterData(context.Background(), CharacterDataRequest{
			CharacterID: character.ID,
			ModuleType:  "bio",
			FieldName:   "background",
			FieldValue:  "An experienced warrior from the far north",
			FieldType:   "text",
			IsPublic:    true,
		})
		core.AssertNoError(t, err, "Failed to update character data")

		// Verify the data was updated
		data, err := characterService.GetCharacterData(context.Background(), character.ID)
		core.AssertNoError(t, err, "Failed to get updated character data")

		var backgroundEntry *models.CharacterDatum
		for _, entry := range data {
			if entry.ModuleType == "bio" && entry.FieldName == "background" {
				backgroundEntry = &entry
				break
			}
		}

		if backgroundEntry == nil {
			t.Fatal("Background entry not found")
		}

		core.AssertEqual(t, "An experienced warrior from the far north", backgroundEntry.FieldValue.String, "Data should be updated")
	})
}

func TestCharacterService_CanUserEditCharacter(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "games", "sessions", "users")

	// Setup test fixtures
	fixtures := testDB.SetupFixtures(t)
	characterService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create additional users
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	otherPlayer := testDB.CreateTestUser(t, "otherplayer", "other@example.com")

	// Create player character owned by 'player'
	playerCharacter, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        fixtures.TestGame.ID,
		UserID:        core.Int32Ptr(int32(player.ID)),
		Name:          "Player Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create player character")

	// Create NPC and assign to otherPlayer
	npcCharacter, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        fixtures.TestGame.ID,
		UserID:        nil,
		Name:          "Assigned NPC",
		CharacterType: "npc",
	})
	core.AssertNoError(t, err, "Failed to create NPC character")

	err = characterService.AssignNPCToUser(context.Background(), npcCharacter.ID, int32(otherPlayer.ID), int32(fixtures.TestUser.ID))
	core.AssertNoError(t, err, "Failed to assign NPC")

	testCases := []struct {
		name        string
		characterID int32
		userID      int32
		canEdit     bool
		reason      string
	}{
		{
			name:        "character owner can edit",
			characterID: playerCharacter.ID,
			userID:      int32(player.ID),
			canEdit:     true,
			reason:      "character owner should be able to edit",
		},
		{
			name:        "GM can edit any character",
			characterID: playerCharacter.ID,
			userID:      int32(fixtures.TestUser.ID), // GM
			canEdit:     true,
			reason:      "GM should be able to edit any character",
		},
		{
			name:        "assigned user can edit NPC",
			characterID: npcCharacter.ID,
			userID:      int32(otherPlayer.ID),
			canEdit:     true,
			reason:      "assigned user should be able to edit NPC",
		},
		{
			name:        "other users cannot edit",
			characterID: playerCharacter.ID,
			userID:      int32(otherPlayer.ID),
			canEdit:     false,
			reason:      "other users should not be able to edit",
		},
		{
			name:        "unassigned user cannot edit NPC",
			characterID: npcCharacter.ID,
			userID:      int32(player.ID),
			canEdit:     false,
			reason:      "unassigned user should not be able to edit NPC",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			canEdit, err := characterService.CanUserEditCharacter(context.Background(), tc.characterID, tc.userID)
			core.AssertNoError(t, err, "Failed to check edit permission")
			core.AssertEqual(t, tc.canEdit, canEdit, tc.reason)
		})
	}

	// Test nonexistent character
	t.Run("nonexistent character", func(t *testing.T) {
		_, err := characterService.CanUserEditCharacter(context.Background(), 99999, int32(player.ID))
		core.AssertError(t, err, "Should fail for nonexistent character")
	})
}

func TestCharacterService_GetPlayerCharacters(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "games", "sessions", "users")

	// Setup test fixtures
	fixtures := testDB.SetupFixtures(t)
	characterService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test users
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")

	// Create player characters
	_, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        fixtures.TestGame.ID,
		UserID:        core.Int32Ptr(int32(player1.ID)),
		Name:          "Player Character 1",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create player character 1")

	_, err = characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        fixtures.TestGame.ID,
		UserID:        core.Int32Ptr(int32(player2.ID)),
		Name:          "Player Character 2",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create player character 2")

	// Create NPCs (should not be included)
	_, err = characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        fixtures.TestGame.ID,
		UserID:        nil,
		Name:          "GM NPC",
		CharacterType: "npc",
	})
	core.AssertNoError(t, err, "Failed to create GM NPC")

	_, err = characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        fixtures.TestGame.ID,
		UserID:        nil,
		Name:          "Audience NPC",
		CharacterType: "npc",
	})
	core.AssertNoError(t, err, "Failed to create audience NPC")

	t.Run("returns only player characters", func(t *testing.T) {
		playerChars, err := characterService.GetPlayerCharacters(context.Background(), fixtures.TestGame.ID)
		core.AssertNoError(t, err, "Failed to get player characters")
		core.AssertEqual(t, 2, len(playerChars), "Expected 2 player characters")

		// Verify all are player_character type
		for _, char := range playerChars {
			core.AssertEqual(t, "player_character", char.CharacterType, "All characters should be player_character type")
		}
	})

	t.Run("returns empty list for game with no player characters", func(t *testing.T) {
		emptyGame := testDB.CreateTestGame(t, int32(fixtures.TestUser.ID), "Empty Game")
		playerChars, err := characterService.GetPlayerCharacters(context.Background(), emptyGame.ID)
		core.AssertNoError(t, err, "Failed to get player characters for empty game")
		core.AssertEqual(t, 0, len(playerChars), "Expected 0 player characters")
	})
}

func TestCharacterService_GetNPCs(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "games", "sessions", "users")

	// Setup test fixtures
	fixtures := testDB.SetupFixtures(t)
	characterService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create player characters (should not be included)
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	_, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        fixtures.TestGame.ID,
		UserID:        core.Int32Ptr(int32(player.ID)),
		Name:          "Player Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create player character")

	// Create NPCs
	_, err = characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        fixtures.TestGame.ID,
		UserID:        nil,
		Name:          "GM NPC 1",
		CharacterType: "npc",
	})
	core.AssertNoError(t, err, "Failed to create GM NPC 1")

	_, err = characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        fixtures.TestGame.ID,
		UserID:        nil,
		Name:          "GM NPC 2",
		CharacterType: "npc",
	})
	core.AssertNoError(t, err, "Failed to create GM NPC 2")

	_, err = characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        fixtures.TestGame.ID,
		UserID:        nil,
		Name:          "Audience NPC 1",
		CharacterType: "npc",
	})
	core.AssertNoError(t, err, "Failed to create audience NPC 1")

	t.Run("returns only NPCs", func(t *testing.T) {
		npcs, err := characterService.GetNPCs(context.Background(), fixtures.TestGame.ID)
		core.AssertNoError(t, err, "Failed to get NPCs")
		core.AssertEqual(t, 3, len(npcs), "Expected 3 NPCs")

		// Verify all are NPC types
		for _, npc := range npcs {
			core.AssertEqual(t, "npc", npc.CharacterType, "All characters should be NPC type")
		}
	})

	t.Run("returns empty list for game with no NPCs", func(t *testing.T) {
		emptyGame := testDB.CreateTestGame(t, int32(fixtures.TestUser.ID), "Empty Game")
		npcs, err := characterService.GetNPCs(context.Background(), emptyGame.ID)
		core.AssertNoError(t, err, "Failed to get NPCs for empty game")
		core.AssertEqual(t, 0, len(npcs), "Expected 0 NPCs")
	})
}

func TestCharacterService_GetUserControllableCharacters(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "games", "sessions", "users")

	// Setup test fixtures
	fixtures := testDB.SetupFixtures(t)
	characterService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test users
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	otherPlayer := testDB.CreateTestUser(t, "otherplayer", "other@example.com")

	// Create player character owned by player
	playerChar, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        fixtures.TestGame.ID,
		UserID:        core.Int32Ptr(int32(player.ID)),
		Name:          "Player Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create player character")

	// Create player character owned by otherPlayer
	_, err = characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        fixtures.TestGame.ID,
		UserID:        core.Int32Ptr(int32(otherPlayer.ID)),
		Name:          "Other Player Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create other player character")

	// Create NPC and assign to player
	assignedNPC, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        fixtures.TestGame.ID,
		UserID:        nil,
		Name:          "Assigned NPC",
		CharacterType: "npc",
	})
	core.AssertNoError(t, err, "Failed to create assigned NPC")

	err = characterService.AssignNPCToUser(context.Background(), assignedNPC.ID, int32(player.ID), int32(fixtures.TestUser.ID))
	core.AssertNoError(t, err, "Failed to assign NPC to player")

	// Create unassigned NPC
	_, err = characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        fixtures.TestGame.ID,
		UserID:        nil,
		Name:          "Unassigned NPC",
		CharacterType: "npc",
	})
	core.AssertNoError(t, err, "Failed to create unassigned NPC")

	t.Run("returns user's characters and assigned NPCs", func(t *testing.T) {
		controllable, err := characterService.GetUserControllableCharacters(context.Background(), fixtures.TestGame.ID, int32(player.ID))
		core.AssertNoError(t, err, "Failed to get user controllable characters")
		core.AssertEqual(t, 2, len(controllable), "Expected 2 controllable characters (owned + assigned)")

		// Verify the correct characters are returned
		hasPlayerChar := false
		hasAssignedNPC := false
		for _, char := range controllable {
			if char.ID == playerChar.ID {
				hasPlayerChar = true
			}
			if char.ID == assignedNPC.ID {
				hasAssignedNPC = true
			}
		}
		core.AssertEqual(t, true, hasPlayerChar, "Should include player's own character")
		core.AssertEqual(t, true, hasAssignedNPC, "Should include assigned NPC")
	})

	t.Run("returns only owned characters for user with no assignments", func(t *testing.T) {
		controllable, err := characterService.GetUserControllableCharacters(context.Background(), fixtures.TestGame.ID, int32(otherPlayer.ID))
		core.AssertNoError(t, err, "Failed to get user controllable characters")
		core.AssertEqual(t, 1, len(controllable), "Expected 1 controllable character (owned only)")
		core.AssertEqual(t, "player_character", controllable[0].CharacterType, "Should be player character")
	})

	t.Run("returns empty list for user with no characters", func(t *testing.T) {
		userWithNoChars := testDB.CreateTestUser(t, "nocharuser", "nochars@example.com")
		controllable, err := characterService.GetUserControllableCharacters(context.Background(), fixtures.TestGame.ID, int32(userWithNoChars.ID))
		core.AssertNoError(t, err, "Failed to get user controllable characters")
		core.AssertEqual(t, 0, len(controllable), "Expected 0 controllable characters")
	})
}

func TestCharacterService_GetUserControllableCharacters_PendingAssignedNPC(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "games", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	characterService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create an in_progress game (audience NPC assignments are only restricted during in_progress)
	game := testDB.CreateTestGameWithState(t, int32(fixtures.TestUser.ID), "In Progress Game", "in_progress")

	audienceUser := testDB.CreateTestUser(t, "audienceuser", "audience@example.com")

	// Create a pending NPC (default status is pending) and assign to audience user
	pendingNPC, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        nil,
		Name:          "Pending Assigned NPC",
		CharacterType: "npc",
	})
	core.AssertNoError(t, err, "Failed to create pending NPC")
	core.AssertEqual(t, "pending", pendingNPC.Status.String, "NPC should start as pending")

	err = characterService.AssignNPCToUser(context.Background(), pendingNPC.ID, int32(audienceUser.ID), int32(fixtures.TestUser.ID))
	core.AssertNoError(t, err, "Failed to assign pending NPC to audience user")

	t.Run("audience user can see assigned NPC even when pending", func(t *testing.T) {
		controllable, err := characterService.GetUserControllableCharacters(context.Background(), game.ID, int32(audienceUser.ID))
		core.AssertNoError(t, err, "Failed to get user controllable characters")

		found := false
		for _, char := range controllable {
			if char.ID == pendingNPC.ID {
				found = true
			}
		}
		core.AssertEqual(t, true, found, "Audience user should see assigned NPC even when NPC status is pending")
	})
}

func TestCharacterService_GetUserControllableCharactersAcrossGames(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "game_participants", "games", "sessions", "users")

	characterService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	ctx := context.Background()

	player := testDB.CreateTestUser(t, "xgplayer", "xgplayer@example.com")
	gm := testDB.CreateTestUser(t, "xggm", "xggm@example.com")

	// Two in_progress games the player has a character in, plus a completed one
	// that must be excluded, and a game the player has nothing to do with.
	gameA := testDB.CreateTestGameWithState(t, int32(gm.ID), "Alpha Game", "in_progress")
	gameB := testDB.CreateTestGameWithState(t, int32(gm.ID), "Beta Game", "in_progress")
	completedGame := testDB.CreateTestGameWithState(t, int32(gm.ID), "Finished Game", "completed")

	charA, err := characterService.CreateCharacter(ctx, CreateCharacterRequest{
		GameID:        gameA.ID,
		UserID:        core.Int32Ptr(int32(player.ID)),
		Name:          "Kael Vance",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create character in game A")

	charB, err := characterService.CreateCharacter(ctx, CreateCharacterRequest{
		GameID:        gameB.ID,
		UserID:        core.Int32Ptr(int32(player.ID)),
		Name:          "Mira Oduya",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create character in game B")

	completedChar, err := characterService.CreateCharacter(ctx, CreateCharacterRequest{
		GameID:        completedGame.ID,
		UserID:        core.Int32Ptr(int32(player.ID)),
		Name:          "Retired Hero",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create character in completed game")

	t.Run("returns characters from all in_progress games, ordered by game title", func(t *testing.T) {
		rows, err := characterService.GetUserControllableCharactersAcrossGames(ctx, int32(player.ID))
		core.AssertNoError(t, err, "Failed to get cross-game controllable characters")
		core.AssertEqual(t, 2, len(rows), "Expected characters from both in_progress games")

		// ORDER BY game_title puts Alpha before Beta.
		core.AssertEqual(t, charA.ID, rows[0].ID, "First row should be the Alpha Game character")
		core.AssertEqual(t, "Alpha Game", rows[0].GameTitle, "Should carry the game title")
		core.AssertEqual(t, "Kael Vance", rows[0].Name, "Should carry the character name")
		core.AssertEqual(t, "in_progress", rows[0].GameState.String, "Should carry the game state")

		core.AssertEqual(t, charB.ID, rows[1].ID, "Second row should be the Beta Game character")
		core.AssertEqual(t, "Beta Game", rows[1].GameTitle, "Should carry the game title")
	})

	t.Run("excludes characters in completed games", func(t *testing.T) {
		rows, err := characterService.GetUserControllableCharactersAcrossGames(ctx, int32(player.ID))
		core.AssertNoError(t, err, "Failed to get cross-game controllable characters")

		for _, row := range rows {
			if row.ID == completedChar.ID {
				t.Fatalf("character %d from completed game should not be returned", completedChar.ID)
			}
		}
	})

	t.Run("reports the user's role in each game", func(t *testing.T) {
		// The player is a plain player in game A; the GM owns it.
		playerRows, err := characterService.GetUserControllableCharactersAcrossGames(ctx, int32(player.ID))
		core.AssertNoError(t, err, "Failed to get cross-game controllable characters for player")
		core.AssertEqual(t, "player", playerRows[0].UserRole, "Player should be reported as 'player'")

		// The GM controls NPCs in their games, so give them one to be reported on.
		gmNPC, err := characterService.CreateCharacter(ctx, CreateCharacterRequest{
			GameID:        gameA.ID,
			UserID:        nil,
			Name:          "Role Check NPC",
			CharacterType: "npc",
		})
		core.AssertNoError(t, err, "Failed to create NPC for GM role check")

		gmRows, err := characterService.GetUserControllableCharactersAcrossGames(ctx, int32(gm.ID))
		core.AssertNoError(t, err, "Failed to get cross-game controllable characters for GM")
		require.NotEmpty(t, gmRows, "GM should control NPCs in their own games")

		foundGMNPC := false
		for _, row := range gmRows {
			core.AssertEqual(t, "gm", row.UserRole, "GM should be reported as 'gm' in their own games")
			if row.ID == gmNPC.ID {
				foundGMNPC = true
			}
		}
		core.AssertEqual(t, true, foundGMNPC, "GM should see the NPC they control")
	})

	t.Run("GM sees unassigned NPCs, players do not", func(t *testing.T) {
		npc, err := characterService.CreateCharacter(ctx, CreateCharacterRequest{
			GameID:        gameA.ID,
			UserID:        nil,
			Name:          "Tavern Keeper",
			CharacterType: "npc",
		})
		core.AssertNoError(t, err, "Failed to create unassigned NPC")

		gmRows, err := characterService.GetUserControllableCharactersAcrossGames(ctx, int32(gm.ID))
		core.AssertNoError(t, err, "Failed to get GM's cross-game characters")
		gmHasNPC := false
		for _, row := range gmRows {
			if row.ID == npc.ID {
				gmHasNPC = true
			}
		}
		core.AssertEqual(t, true, gmHasNPC, "GM should control unassigned NPCs in their game")

		playerRows, err := characterService.GetUserControllableCharactersAcrossGames(ctx, int32(player.ID))
		core.AssertNoError(t, err, "Failed to get player's cross-game characters")
		for _, row := range playerRows {
			if row.ID == npc.ID {
				t.Fatal("player should not control an unassigned NPC")
			}
		}
	})

	// The drawer's cross-game list is the GM's cast reference for the game they're
	// running, exactly as the in-game list is. Limiting them to NPCs left a GM off
	// a game page seeing only a fraction of their game.
	t.Run("GM gets every character in their game, not just NPCs", func(t *testing.T) {
		gmRows, err := characterService.GetUserControllableCharactersAcrossGames(ctx, int32(gm.ID))
		core.AssertNoError(t, err, "Failed to get GM's cross-game characters")

		seen := make(map[int32]bool, len(gmRows))
		for _, row := range gmRows {
			seen[row.ID] = true
		}

		// Player characters owned by someone else, in games this GM runs.
		core.AssertEqual(t, true, seen[charA.ID], "GM should see a player's character in game A")
		core.AssertEqual(t, true, seen[charB.ID], "GM should see a player's character in game B")
	})

	// Who is playing each character is the whole value of the cast list — without
	// it the GM gets a wall of names. Sourced here because there is no game route
	// to fetch it from.
	t.Run("carries owner and assignee usernames for the GM's cast", func(t *testing.T) {
		assignedNPC, err := characterService.CreateCharacter(ctx, CreateCharacterRequest{
			GameID:        gameB.ID,
			UserID:        nil,
			Name:          "Assigned Herald",
			CharacterType: "npc",
		})
		core.AssertNoError(t, err, "Failed to create NPC for assignment")
		err = characterService.AssignNPCToUser(ctx, assignedNPC.ID, int32(player.ID), int32(gm.ID))
		core.AssertNoError(t, err, "Failed to assign NPC")

		gmRows, err := characterService.GetUserControllableCharactersAcrossGames(ctx, int32(gm.ID))
		core.AssertNoError(t, err, "Failed to get GM's cross-game characters")

		var pc, npc *models.GetUserControllableCharactersAcrossGamesRow
		for i := range gmRows {
			switch gmRows[i].ID {
			case charA.ID:
				pc = &gmRows[i]
			case assignedNPC.ID:
				npc = &gmRows[i]
			}
		}
		require.NotNil(t, pc, "GM should see the player character")
		require.NotNil(t, npc, "GM should see the assigned NPC")

		core.AssertEqual(t, player.Username, pc.OwnerUsername.String, "Player character should credit its owner")
		core.AssertEqual(t, player.Username, npc.AssignedUsername.String, "Assigned NPC should credit its assignee")
	})

	// A player must not gain a window into the rest of the cast just because the
	// GM's list widened.
	t.Run("players still see only what they control", func(t *testing.T) {
		playerRows, err := characterService.GetUserControllableCharactersAcrossGames(ctx, int32(player.ID))
		core.AssertNoError(t, err, "Failed to get player's cross-game characters")

		for _, row := range playerRows {
			isOwn := row.UserID.Valid && row.UserID.Int32 == int32(player.ID)
			isAssigned := row.AssignedUsername.Valid && row.AssignedUsername.String == player.Username
			if !isOwn && !isAssigned {
				t.Fatalf("player should not see character %d (%q) they neither own nor are assigned", row.ID, row.Name)
			}
		}
	})

	t.Run("co-GM gets NPCs with the co_gm role", func(t *testing.T) {
		coGM := testDB.CreateTestUser(t, "xgcogm", "xgcogm@example.com")
		testDB.AddTestGameParticipant(t, gameB.ID, int32(coGM.ID), "co_gm")

		npc, err := characterService.CreateCharacter(ctx, CreateCharacterRequest{
			GameID:        gameB.ID,
			UserID:        nil,
			Name:          "Beta NPC",
			CharacterType: "npc",
		})
		core.AssertNoError(t, err, "Failed to create NPC in game B")

		rows, err := characterService.GetUserControllableCharactersAcrossGames(ctx, int32(coGM.ID))
		core.AssertNoError(t, err, "Failed to get co-GM's cross-game characters")

		found := false
		for _, row := range rows {
			if row.ID == npc.ID {
				found = true
				core.AssertEqual(t, "co_gm", row.UserRole, "Co-GM should be reported as 'co_gm'")
			}
		}
		core.AssertEqual(t, true, found, "Co-GM should control NPCs in their game")
	})

	// An audience member controls nothing by default, but a GM can assign them
	// an NPC — at which point it's a character they play, and the drawer must
	// offer it like any other. The role travels with it so the sheet resolves
	// permissions correctly (audience is not GM, so no stat editing).
	t.Run("audience member gets NPCs assigned to them, with the audience role", func(t *testing.T) {
		audience := testDB.CreateTestUser(t, "xgaudience", "xgaudience@example.com")
		testDB.AddTestGameParticipant(t, gameA.ID, int32(audience.ID), "audience")

		npc, err := characterService.CreateCharacter(ctx, CreateCharacterRequest{
			GameID:        gameA.ID,
			UserID:        nil,
			Name:          "Audience NPC",
			CharacterType: "npc",
		})
		core.AssertNoError(t, err, "Failed to create NPC for audience assignment")

		// Before assignment the audience member controls nothing.
		before, err := characterService.GetUserControllableCharactersAcrossGames(ctx, int32(audience.ID))
		core.AssertNoError(t, err, "Failed to get audience characters before assignment")
		core.AssertEqual(t, 0, len(before), "Audience should control nothing before assignment")

		err = characterService.AssignNPCToUser(ctx, npc.ID, int32(audience.ID), int32(gm.ID))
		core.AssertNoError(t, err, "Failed to assign NPC to audience member")

		after, err := characterService.GetUserControllableCharactersAcrossGames(ctx, int32(audience.ID))
		core.AssertNoError(t, err, "Failed to get audience characters after assignment")
		core.AssertEqual(t, 1, len(after), "Audience should control the NPC assigned to them")
		core.AssertEqual(t, npc.ID, after[0].ID, "Should be the assigned NPC")
		core.AssertEqual(t, "audience", after[0].UserRole, "Role should be reported as 'audience'")
	})

	t.Run("excludes deactivated characters", func(t *testing.T) {
		leaver := testDB.CreateTestUser(t, "xgleaver", "xgleaver@example.com")
		leaverChar, err := characterService.CreateCharacter(ctx, CreateCharacterRequest{
			GameID:        gameA.ID,
			UserID:        core.Int32Ptr(int32(leaver.ID)),
			Name:          "Departed Soul",
			CharacterType: "player_character",
		})
		core.AssertNoError(t, err, "Failed to create character for leaving player")

		before, err := characterService.GetUserControllableCharactersAcrossGames(ctx, int32(leaver.ID))
		core.AssertNoError(t, err, "Failed to get characters before deactivation")
		core.AssertEqual(t, 1, len(before), "Character should be returned while active")
		core.AssertEqual(t, leaverChar.ID, before[0].ID, "Should be the leaver's character")

		err = characterService.DeactivatePlayerCharacters(ctx, gameA.ID, int32(leaver.ID))
		core.AssertNoError(t, err, "Failed to deactivate characters")

		after, err := characterService.GetUserControllableCharactersAcrossGames(ctx, int32(leaver.ID))
		core.AssertNoError(t, err, "Failed to get characters after deactivation")
		core.AssertEqual(t, 0, len(after), "Deactivated character should not be returned")
	})

	t.Run("returns empty list for a user with no characters", func(t *testing.T) {
		stranger := testDB.CreateTestUser(t, "xgstranger", "xgstranger@example.com")
		rows, err := characterService.GetUserControllableCharactersAcrossGames(ctx, int32(stranger.ID))
		core.AssertNoError(t, err, "Failed to get cross-game controllable characters")
		core.AssertEqual(t, 0, len(rows), "Expected no characters")
	})
}

func TestCharacterService_AssignNPCToUser_AudienceNPC(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "games", "sessions", "users")

	// Setup test fixtures
	fixtures := testDB.SetupFixtures(t)
	characterService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create user for assignment
	assignedUser := testDB.CreateTestUser(t, "assigneduser", "assigned@example.com")

	t.Run("assign audience NPC to user", func(t *testing.T) {
		// Create audience NPC
		audienceNPC, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
			GameID:        fixtures.TestGame.ID,
			UserID:        nil,
			Name:          "Audience NPC",
			CharacterType: "npc",
		})
		core.AssertNoError(t, err, "Failed to create audience NPC")

		// Assign to user
		err = characterService.AssignNPCToUser(context.Background(), audienceNPC.ID, int32(assignedUser.ID), int32(fixtures.TestUser.ID))
		core.AssertNoError(t, err, "Failed to assign audience NPC to user")

		// Verify user can edit the NPC
		canEdit, err := characterService.CanUserEditCharacter(context.Background(), audienceNPC.ID, int32(assignedUser.ID))
		core.AssertNoError(t, err, "Failed to check edit permission")
		core.AssertEqual(t, true, canEdit, "Assigned user should be able to edit audience NPC")
	})
}

func TestCharacterService_DeleteCharacter(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "messages", "action_submissions", "character_data", "npc_assignments", "characters", "game_phases", "games", "sessions", "users")

	// Setup test fixtures
	fixtures := testDB.SetupFixtures(t)
	characterService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	queries := models.New(testDB.Pool)

	t.Run("successfully delete character with no activity", func(t *testing.T) {
		// Create character with no activity
		character, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
			GameID:        fixtures.TestGame.ID,
			UserID:        core.Int32Ptr(int32(fixtures.TestUser.ID)),
			Name:          "Clean Character",
			CharacterType: "player_character",
		})
		core.AssertNoError(t, err, "Failed to create character")

		// Delete character
		err = characterService.DeleteCharacter(context.Background(), character.ID)
		core.AssertNoError(t, err, "Failed to delete character with no activity")

		// Verify character is deleted
		_, err = queries.GetCharacter(context.Background(), character.ID)
		core.AssertError(t, err, "Character should be deleted")
	})

	t.Run("prevent deletion when character has messages", func(t *testing.T) {
		// Create character
		character, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
			GameID:        fixtures.TestGame.ID,
			UserID:        core.Int32Ptr(int32(fixtures.TestUser.ID)),
			Name:          "Character With Messages",
			CharacterType: "player_character",
		})
		core.AssertNoError(t, err, "Failed to create character")

		// Create a message from this character using raw SQL
		_, err = testDB.Pool.Exec(context.Background(), `
			INSERT INTO messages (game_id, author_id, character_id, content, message_type, visibility)
			VALUES ($1, $2, $3, $4, 'post', 'game')
		`, fixtures.TestGame.ID, fixtures.TestUser.ID, character.ID, "Test message")
		core.AssertNoError(t, err, "Failed to create message")

		// Attempt to delete character - should fail
		err = characterService.DeleteCharacter(context.Background(), character.ID)
		core.AssertError(t, err, "Should not allow deletion of character with messages")

		// Verify character still exists
		retrieved, err := queries.GetCharacter(context.Background(), character.ID)
		core.AssertNoError(t, err, "Character should still exist")
		core.AssertEqual(t, character.ID, retrieved.ID, "Character should not be deleted")
	})

	t.Run("prevent deletion when character has action submissions", func(t *testing.T) {
		// Create character
		character, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
			GameID:        fixtures.TestGame.ID,
			UserID:        core.Int32Ptr(int32(fixtures.TestUser.ID)),
			Name:          "Character With Actions",
			CharacterType: "player_character",
		})
		core.AssertNoError(t, err, "Failed to create character")

		// Create a phase using raw SQL
		var phaseID int32
		err = testDB.Pool.QueryRow(context.Background(), `
			INSERT INTO game_phases (game_id, phase_type, phase_number, title, start_time)
			VALUES ($1, 'action', 1, 'Test Phase', NOW())
			RETURNING id
		`, fixtures.TestGame.ID).Scan(&phaseID)
		core.AssertNoError(t, err, "Failed to create phase")

		// Create action submission for this character using raw SQL
		_, err = testDB.Pool.Exec(context.Background(), `
			INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content)
			VALUES ($1, $2, $3, $4, $5)
		`, fixtures.TestGame.ID, fixtures.TestUser.ID, phaseID, character.ID, "Test action")
		core.AssertNoError(t, err, "Failed to create action submission")

		// Attempt to delete character - should fail
		err = characterService.DeleteCharacter(context.Background(), character.ID)
		core.AssertError(t, err, "Should not allow deletion of character with action submissions")

		// Verify character still exists
		retrieved, err := queries.GetCharacter(context.Background(), character.ID)
		core.AssertNoError(t, err, "Character should still exist")
		core.AssertEqual(t, character.ID, retrieved.ID, "Character should not be deleted")
	})

	t.Run("delete character with character_data", func(t *testing.T) {
		// Create character
		character, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
			GameID:        fixtures.TestGame.ID,
			UserID:        core.Int32Ptr(int32(fixtures.TestUser.ID)),
			Name:          "Character With Data",
			CharacterType: "player_character",
		})
		core.AssertNoError(t, err, "Failed to create character")

		// Add character data
		err = characterService.SetCharacterData(context.Background(), CharacterDataRequest{
			CharacterID: character.ID,
			ModuleType:  "bio",
			FieldName:   "background",
			FieldValue:  "Test background",
			FieldType:   "text",
			IsPublic:    true,
		})
		core.AssertNoError(t, err, "Failed to set character data")

		// Delete character - should succeed (character_data CASCADE deletes)
		err = characterService.DeleteCharacter(context.Background(), character.ID)
		core.AssertNoError(t, err, "Should allow deletion of character with character_data (CASCADE)")

		// Verify character is deleted
		_, err = queries.GetCharacter(context.Background(), character.ID)
		core.AssertError(t, err, "Character should be deleted")
	})

	t.Run("delete NPC with assignment", func(t *testing.T) {
		// Create NPC
		npc, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
			GameID:        fixtures.TestGame.ID,
			UserID:        nil,
			Name:          "Assigned NPC",
			CharacterType: "npc",
		})
		core.AssertNoError(t, err, "Failed to create NPC")

		// Assign NPC
		player := testDB.CreateTestUser(t, "npcplayer", "npcplayer@example.com")
		err = characterService.AssignNPCToUser(context.Background(), npc.ID, int32(player.ID), int32(fixtures.TestUser.ID))
		core.AssertNoError(t, err, "Failed to assign NPC")

		// Delete NPC - should succeed (npc_assignments CASCADE deletes)
		err = characterService.DeleteCharacter(context.Background(), npc.ID)
		core.AssertNoError(t, err, "Should allow deletion of NPC with assignment (CASCADE)")

		// Verify NPC is deleted
		_, err = queries.GetCharacter(context.Background(), npc.ID)
		core.AssertError(t, err, "NPC should be deleted")
	})

	t.Run("delete nonexistent character", func(t *testing.T) {
		err := characterService.DeleteCharacter(context.Background(), 99999)
		core.AssertError(t, err, "Should fail for nonexistent character")
	})
}

// TestCharacterService_DatabaseConstraintViolations tests database constraint enforcement
func TestCharacterService_DatabaseConstraintViolations(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "games", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	characterService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	t.Run("fails to create character with non-existent game", func(t *testing.T) {
		userID := int32(fixtures.TestUser.ID)
		req := CreateCharacterRequest{
			GameID:        99999, // Non-existent game ID
			UserID:        &userID,
			Name:          "Character in Nonexistent Game",
			CharacterType: "player_character",
		}

		_, err := characterService.CreateCharacter(context.Background(), req)
		core.AssertError(t, err, "Should fail with FK constraint violation")
		core.AssertErrorContains(t, err, "foreign key constraint", "Should contain FK constraint error message")
	})

	t.Run("fails to create character with non-existent user", func(t *testing.T) {
		invalidUserID := int32(99999) // Non-existent user ID
		req := CreateCharacterRequest{
			GameID:        fixtures.TestGame.ID,
			UserID:        &invalidUserID,
			Name:          "Character for Nonexistent User",
			CharacterType: "player_character",
		}

		_, err := characterService.CreateCharacter(context.Background(), req)
		core.AssertError(t, err, "Should fail with FK constraint violation")
		core.AssertErrorContains(t, err, "foreign key constraint", "Should contain FK constraint error message")
	})

	t.Run("fails to create character with invalid character type", func(t *testing.T) {
		userID := int32(fixtures.TestUser.ID)
		req := CreateCharacterRequest{
			GameID:        fixtures.TestGame.ID,
			UserID:        &userID,
			Name:          "Character with Invalid Type",
			CharacterType: "invalid_type", // Invalid character type
		}

		_, err := characterService.CreateCharacter(context.Background(), req)
		core.AssertError(t, err, "Should fail with validation error")
		// Note: Character service validates type at application level, not database level
		core.AssertErrorContains(t, err, "invalid character type", "Should contain validation error message")
	})

	t.Run("fails to create character with zero game ID", func(t *testing.T) {
		userID := int32(fixtures.TestUser.ID)
		req := CreateCharacterRequest{
			GameID:        0, // Invalid game ID
			UserID:        &userID,
			Name:          "Character with Zero Game",
			CharacterType: "player_character",
		}

		_, err := characterService.CreateCharacter(context.Background(), req)
		core.AssertError(t, err, "Should fail with invalid FK")
	})
}

// TestCharacterService_CreateGamemasterNPC tests automatic Gamemaster NPC creation
func TestCharacterService_CreateGamemasterNPC(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "games", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	characterService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	queries := models.New(testDB.Pool)

	t.Run("creates Gamemaster NPC successfully", func(t *testing.T) {
		// Create Gamemaster NPC
		err := characterService.CreateGamemasterNPC(context.Background(), fixtures.TestGame.ID)
		core.AssertNoError(t, err, "Failed to create Gamemaster NPC")

		// Verify NPC was created
		character, err := queries.GetCharacterByNameAndGame(context.Background(), models.GetCharacterByNameAndGameParams{
			Name:   "Gamemaster",
			GameID: fixtures.TestGame.ID,
		})
		core.AssertNoError(t, err, "Failed to get Gamemaster NPC")

		// Verify NPC attributes
		core.AssertEqual(t, "Gamemaster", character.Name, "Character name should be 'Gamemaster'")
		core.AssertEqual(t, "npc", character.CharacterType, "Character type should be 'npc'")
		core.AssertEqual(t, "approved", character.Status.String, "Character status should be 'approved'")
		core.AssertEqual(t, false, character.UserID.Valid, "User ID should be NULL for GM NPCs")
		core.AssertEqual(t, fixtures.TestGame.ID, character.GameID, "Game ID should match")
	})

	t.Run("idempotent - skips creation if Gamemaster already exists", func(t *testing.T) {
		// Create a new game for this test
		newGame := testDB.CreateTestGame(t, int32(fixtures.TestUser.ID), "Idempotency Test Game")

		// Create Gamemaster NPC first time
		err := characterService.CreateGamemasterNPC(context.Background(), newGame.ID)
		core.AssertNoError(t, err, "Failed to create Gamemaster NPC first time")

		// Get the first NPC ID
		firstNPC, err := queries.GetCharacterByNameAndGame(context.Background(), models.GetCharacterByNameAndGameParams{
			Name:   "Gamemaster",
			GameID: newGame.ID,
		})
		core.AssertNoError(t, err, "Failed to get first Gamemaster NPC")

		// Try to create again - should skip
		err = characterService.CreateGamemasterNPC(context.Background(), newGame.ID)
		core.AssertNoError(t, err, "Should not error when Gamemaster already exists")

		// Verify no duplicate was created
		characters, err := queries.GetCharactersByGame(context.Background(), newGame.ID)
		core.AssertNoError(t, err, "Failed to get characters for game")

		// Count how many "Gamemaster" NPCs exist
		gamemasterCount := 0
		for _, char := range characters {
			if char.Name == "Gamemaster" {
				gamemasterCount++
				core.AssertEqual(t, firstNPC.ID, char.ID, "Should be the same Gamemaster NPC, not a duplicate")
			}
		}
		core.AssertEqual(t, 1, gamemasterCount, "Should have exactly 1 Gamemaster NPC")
	})

	t.Run("skips creation if character named Gamemaster already exists", func(t *testing.T) {
		// Create a new game for this test
		anotherGame := testDB.CreateTestGame(t, int32(fixtures.TestUser.ID), "Conflict Test Game")

		// Manually create a player character named "Gamemaster" first
		userID := int32(fixtures.TestUser.ID)
		playerChar, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
			GameID:        anotherGame.ID,
			UserID:        &userID,
			Name:          "Gamemaster",
			CharacterType: "player_character",
		})
		core.AssertNoError(t, err, "Failed to create player character named Gamemaster")

		// Try to create auto NPC - should skip
		err = characterService.CreateGamemasterNPC(context.Background(), anotherGame.ID)
		core.AssertNoError(t, err, "Should not error when character named Gamemaster already exists")

		// Verify only the player character exists (no NPC was created)
		character, err := queries.GetCharacterByNameAndGame(context.Background(), models.GetCharacterByNameAndGameParams{
			Name:   "Gamemaster",
			GameID: anotherGame.ID,
		})
		core.AssertNoError(t, err, "Failed to get Gamemaster character")
		core.AssertEqual(t, playerChar.ID, character.ID, "Should be the original player character, not a new NPC")
		core.AssertEqual(t, "player_character", character.CharacterType, "Should still be a player_character")
	})

	t.Run("fails gracefully with invalid game ID", func(t *testing.T) {
		// Try to create NPC for nonexistent game
		err := characterService.CreateGamemasterNPC(context.Background(), 99999)
		core.AssertError(t, err, "Should fail with invalid game ID")
		core.AssertErrorContains(t, err, "failed to create Gamemaster NPC", "Should contain error message")
	})
}

func TestCharacterService_RenameCharacter(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "games", "sessions", "users")

	fixtures := testDB.SetupFixtures(t)
	characterService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Setup: Create a character for testing
	character, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
		GameID:        fixtures.TestGame.ID,
		UserID:        core.Int32Ptr(int32(fixtures.TestUser.ID)),
		Name:          "Original Name",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create test character")

	t.Run("successfully renames character", func(t *testing.T) {
		updatedChar, err := characterService.RenameCharacter(context.Background(), character.ID, "New Name")
		core.AssertNoError(t, err, "Should rename character successfully")
		if updatedChar == nil {
			t.Fatal("Expected updated character to be returned")
		}
		core.AssertEqual(t, "New Name", updatedChar.Name, "Name should be updated")
		core.AssertEqual(t, character.ID, updatedChar.ID, "Should be same character")
	})

	t.Run("trims whitespace from new name", func(t *testing.T) {
		updatedChar, err := characterService.RenameCharacter(context.Background(), character.ID, "  Trimmed Name  ")
		core.AssertNoError(t, err, "Should rename character successfully")
		core.AssertEqual(t, "Trimmed Name", updatedChar.Name, "Name should be trimmed")
	})

	t.Run("returns error for empty name", func(t *testing.T) {
		_, err := characterService.RenameCharacter(context.Background(), character.ID, "   ")
		core.AssertError(t, err, "Should fail with empty name")
		core.AssertErrorContains(t, err, "cannot be empty", "Should contain appropriate error message")
	})

	t.Run("returns error for name too long", func(t *testing.T) {
		longName := string(make([]byte, MaxCharacterNameLength+1)) // One more than max
		for i := range longName {
			longName = longName[:i] + "a" + longName[i+1:]
		}
		_, err := characterService.RenameCharacter(context.Background(), character.ID, longName)
		core.AssertError(t, err, "Should fail with name too long")
		core.AssertErrorContains(t, err, "too long", "Should contain appropriate error message")
	})

	t.Run("no-op when new name is same as current", func(t *testing.T) {
		// Get current character state
		currentChar, err := characterService.GetCharacter(context.Background(), character.ID)
		core.AssertNoError(t, err, "Should get character")

		// Rename to same name
		updatedChar, err := characterService.RenameCharacter(context.Background(), character.ID, currentChar.Name)
		core.AssertNoError(t, err, "Should succeed with same name")
		core.AssertEqual(t, currentChar.Name, updatedChar.Name, "Name should be unchanged")
	})

	t.Run("fails with duplicate name in same game", func(t *testing.T) {
		// Create two fresh characters for this test
		char1, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
			GameID:        fixtures.TestGame.ID,
			UserID:        core.Int32Ptr(int32(fixtures.TestUser.ID)),
			Name:          "Duplicate Test Char 1",
			CharacterType: "player_character",
		})
		core.AssertNoError(t, err, "Failed to create first character")

		char2, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
			GameID:        fixtures.TestGame.ID,
			UserID:        core.Int32Ptr(int32(fixtures.TestUser.ID)),
			Name:          "Duplicate Test Char 2",
			CharacterType: "player_character",
		})
		core.AssertNoError(t, err, "Failed to create second character")

		// Try to rename first character to second character's name
		_, err = characterService.RenameCharacter(context.Background(), char1.ID, "Duplicate Test Char 2")
		core.AssertError(t, err, "Should fail with duplicate name")
		core.AssertErrorContains(t, err, "already exists", "Should contain duplicate error message")

		// Verify first character name is unchanged
		verifyChar, err := characterService.GetCharacter(context.Background(), char1.ID)
		core.AssertNoError(t, err, "Should get character")
		core.AssertEqual(t, "Duplicate Test Char 1", verifyChar.Name, "Original character name should be unchanged")

		// Cleanup
		_ = char2 // Avoid unused variable warning
	})

	t.Run("allows same name in different games", func(t *testing.T) {
		// Create a second game and character
		game2 := testDB.CreateTestGame(t, int32(fixtures.TestUser.ID), "Second Game")
		character2, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
			GameID:        game2.ID,
			UserID:        core.Int32Ptr(int32(fixtures.TestUser.ID)),
			Name:          "Unique Name",
			CharacterType: "player_character",
		})
		core.AssertNoError(t, err, "Failed to create character in second game")

		// Rename character in second game to use name from first game (should work)
		sameName := "Shared Name Across Games"
		_, err = characterService.RenameCharacter(context.Background(), character.ID, sameName)
		core.AssertNoError(t, err, "Should rename character in first game")

		updatedChar2, err := characterService.RenameCharacter(context.Background(), character2.ID, sameName)
		core.AssertNoError(t, err, "Should allow same name in different game")
		core.AssertEqual(t, sameName, updatedChar2.Name, "Should have same name as character in other game")
	})

	t.Run("fails with invalid character ID", func(t *testing.T) {
		_, err := characterService.RenameCharacter(context.Background(), 99999, "New Name")
		core.AssertError(t, err, "Should fail with invalid character ID")
	})
}

// TestCharacterService_ReassignCharacter verifies that ownership transfer changes
// the character's user_id in the database. Silent failure here means the character
// still appears under the old owner after a player removal.
func TestCharacterService_ReassignCharacter(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "games", "sessions", "users")

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	characterService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	newOwner := testDB.CreateTestUser(t, "newowner", "newowner@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	char, err := characterService.CreateCharacter(ctx, CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        core.Int32Ptr(int32(player.ID)),
		Name:          "TransferMe",
		CharacterType: "player_character",
	})
	if err != nil {
		t.Fatalf("failed to create character: %v", err)
	}

	t.Run("reassigns character to new owner", func(t *testing.T) {
		updated, err := characterService.ReassignCharacter(ctx, char.ID, int32(newOwner.ID))
		if err != nil {
			t.Fatalf("ReassignCharacter failed: %v", err)
		}
		if !updated.UserID.Valid || updated.UserID.Int32 != int32(newOwner.ID) {
			t.Errorf("expected new owner user_id %d, got %+v", newOwner.ID, updated.UserID)
		}
	})

	t.Run("returns error for non-existent character", func(t *testing.T) {
		_, err := characterService.ReassignCharacter(ctx, 99999, int32(newOwner.ID))
		if err == nil {
			t.Error("expected error for non-existent character, got nil")
		}
	})
}

// TestCharacterService_DeactivatePlayerCharacters verifies that all player characters
// for a user in a game are marked inactive. Silent failure leaves characters active
// after a player is removed, which corrupts game state.
func TestCharacterService_DeactivatePlayerCharacters(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "games", "sessions", "users")

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	characterService := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	otherPlayer := testDB.CreateTestUser(t, "other", "other@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	// Create two characters for the target player
	char1, err := characterService.CreateCharacter(ctx, CreateCharacterRequest{
		GameID: game.ID, UserID: core.Int32Ptr(int32(player.ID)),
		Name: "Char1", CharacterType: "player_character",
	})
	if err != nil {
		t.Fatalf("create char1: %v", err)
	}
	char2, err := characterService.CreateCharacter(ctx, CreateCharacterRequest{
		GameID: game.ID, UserID: core.Int32Ptr(int32(player.ID)),
		Name: "Char2", CharacterType: "player_character",
	})
	if err != nil {
		t.Fatalf("create char2: %v", err)
	}

	// Create a character for a different player — must not be affected
	otherChar, err := characterService.CreateCharacter(ctx, CreateCharacterRequest{
		GameID: game.ID, UserID: core.Int32Ptr(int32(otherPlayer.ID)),
		Name: "OtherChar", CharacterType: "player_character",
	})
	if err != nil {
		t.Fatalf("create otherChar: %v", err)
	}

	err = characterService.DeactivatePlayerCharacters(ctx, game.ID, int32(player.ID))
	if err != nil {
		t.Fatalf("DeactivatePlayerCharacters failed: %v", err)
	}

	queries := models.New(testDB.Pool)

	for _, id := range []int32{char1.ID, char2.ID} {
		c, err := queries.GetCharacter(ctx, id)
		if err != nil {
			t.Fatalf("fetch character %d: %v", id, err)
		}
		if c.IsActive {
			t.Errorf("character %d: expected is_active=false after deactivation", id)
		}
	}

	// Other player's character should still be active
	oc, err := queries.GetCharacter(ctx, otherChar.ID)
	if err != nil {
		t.Fatalf("fetch otherChar: %v", err)
	}
	if !oc.IsActive {
		t.Errorf("other player's character was incorrectly deactivated")
	}
}

// TestCharacterService_ListInactiveCharacters verifies that inactive characters
// are returned and active ones are excluded. Silent failure here means the GM
// cannot see players who have left the game.
func TestCharacterService_ListInactiveCharacters(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "characters", "games", "users")

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	svc := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "inactive_gm", "inactive_gm@example.com")
	player := testDB.CreateTestUser(t, "inactive_player", "inactive_player@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Inactive Test Game")

	active, err := svc.CreateCharacter(ctx, CreateCharacterRequest{
		GameID: game.ID, UserID: core.Int32Ptr(int32(player.ID)),
		Name: "ActiveChar", CharacterType: "player_character",
	})
	require.NoError(t, err)

	inactive, err := svc.CreateCharacter(ctx, CreateCharacterRequest{
		GameID: game.ID, UserID: core.Int32Ptr(int32(player.ID)),
		Name: "InactiveChar", CharacterType: "player_character",
	})
	require.NoError(t, err)

	// Deactivate the second character directly
	_, err = testDB.Pool.Exec(ctx, "UPDATE characters SET is_active = false WHERE id = $1", inactive.ID)
	require.NoError(t, err)

	t.Run("returns only inactive characters", func(t *testing.T) {
		result, err := svc.ListInactiveCharacters(ctx, game.ID)
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, inactive.ID, result[0].ID)
	})

	t.Run("does not include active characters", func(t *testing.T) {
		result, err := svc.ListInactiveCharacters(ctx, game.ID)
		require.NoError(t, err)
		for _, c := range result {
			assert.NotEqual(t, active.ID, c.ID)
		}
	})

	t.Run("returns empty list for game with no inactive characters", func(t *testing.T) {
		otherGame := testDB.CreateTestGame(t, int32(gm.ID), "Other Game")
		result, err := svc.ListInactiveCharacters(ctx, otherGame.ID)
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// TestCharacterService_GetCharacterActivityStats verifies that public and private
// message counts are returned accurately. Silent failure means the GM sees zero
// activity for all characters in the audience view.
func TestCharacterService_GetCharacterActivityStats(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "private_messages", "conversations", "messages", "characters", "games", "users")

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	svc := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "stats_gm", "stats_gm@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Stats Test Game")

	char, err := svc.CreateCharacter(ctx, CreateCharacterRequest{
		GameID: game.ID, UserID: core.Int32Ptr(int32(gm.ID)),
		Name: "StatsChar", CharacterType: "player_character",
	})
	require.NoError(t, err)

	t.Run("returns zero counts for character with no messages", func(t *testing.T) {
		stats, err := svc.GetCharacterActivityStats(ctx, char.ID)
		require.NoError(t, err)
		require.NotNil(t, stats)
		assert.Equal(t, int64(0), stats.PublicMessages)
		require.NotNil(t, stats.PrivateMessages)
		assert.Equal(t, int64(0), *stats.PrivateMessages)
	})

	t.Run("counts public messages authored by the character", func(t *testing.T) {
		_, err := testDB.Pool.Exec(ctx, `
			INSERT INTO messages (game_id, author_id, character_id, content, message_type, visibility)
			VALUES ($1, $2, $3, 'public post one', 'post', 'game')
		`, game.ID, gm.ID, char.ID)
		require.NoError(t, err)

		_, err = testDB.Pool.Exec(ctx, `
			INSERT INTO messages (game_id, author_id, character_id, content, message_type, visibility)
			VALUES ($1, $2, $3, 'public post two', 'post', 'game')
		`, game.ID, gm.ID, char.ID)
		require.NoError(t, err)

		stats, err := svc.GetCharacterActivityStats(ctx, char.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(2), stats.PublicMessages, "must count all non-deleted public messages for the character")
	})

	t.Run("counts private messages sent as the character", func(t *testing.T) {
		// A private message requires a conversation parent row.
		var convID int32
		err := testDB.Pool.QueryRow(ctx, `
			INSERT INTO conversations (game_id, created_by_user_id, conversation_type)
			VALUES ($1, $2, 'direct') RETURNING id
		`, game.ID, gm.ID).Scan(&convID)
		require.NoError(t, err)

		_, err = testDB.Pool.Exec(ctx, `
			INSERT INTO private_messages (conversation_id, sender_user_id, sender_character_id, content)
			VALUES ($1, $2, $3, 'private message one')
		`, convID, gm.ID, char.ID)
		require.NoError(t, err)

		stats, err := svc.GetCharacterActivityStats(ctx, char.ID)
		require.NoError(t, err)
		require.NotNil(t, stats.PrivateMessages)
		assert.Equal(t, int64(1), *stats.PrivateMessages, "must count non-deleted private messages sent as the character")
	})

	t.Run("does not count soft-deleted messages", func(t *testing.T) {
		// Mark all existing messages as deleted for this character.
		_, err := testDB.Pool.Exec(ctx,
			"UPDATE messages SET is_deleted = TRUE WHERE character_id = $1", char.ID)
		require.NoError(t, err)
		_, err = testDB.Pool.Exec(ctx,
			"UPDATE private_messages SET is_deleted = TRUE WHERE sender_character_id = $1", char.ID)
		require.NoError(t, err)

		stats, err := svc.GetCharacterActivityStats(ctx, char.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), stats.PublicMessages, "deleted public messages must not be counted")
		require.NotNil(t, stats.PrivateMessages)
		assert.Equal(t, int64(0), *stats.PrivateMessages, "deleted private messages must not be counted")
	})
}

// TestCharacterService_GetCharacterActivityStatsByGame verifies the batch
// per-game stats query returns one entry per character (including characters
// with zero messages), aggregates the same way as GetCharacterActivityStats,
// and excludes soft-deleted messages.
func TestCharacterService_GetCharacterActivityStatsByGame(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "private_messages", "conversations", "messages", "characters", "games", "users")

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	svc := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "gamestats_gm", "gamestats_gm@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Game Stats Service Test Game")

	active, err := svc.CreateCharacter(ctx, CreateCharacterRequest{
		GameID: game.ID, UserID: core.Int32Ptr(int32(gm.ID)),
		Name: "ActiveChar", CharacterType: "player_character",
	})
	require.NoError(t, err)

	silent, err := svc.CreateCharacter(ctx, CreateCharacterRequest{
		GameID: game.ID, UserID: core.Int32Ptr(int32(gm.ID)),
		Name: "SilentChar", CharacterType: "player_character",
	})
	require.NoError(t, err)

	t.Run("returns zero counts for a character with no messages, keyed by ID", func(t *testing.T) {
		statsByID, err := svc.GetCharacterActivityStatsByGame(ctx, game.ID)
		require.NoError(t, err)

		require.Contains(t, statsByID, silent.ID, "characters with no messages must still appear in the map")
		assert.Equal(t, int64(0), statsByID[silent.ID].PublicMessages)
		require.NotNil(t, statsByID[silent.ID].PrivateMessages)
		assert.Equal(t, int64(0), *statsByID[silent.ID].PrivateMessages)
	})

	t.Run("counts public and private messages per character and does not cross-contaminate", func(t *testing.T) {
		_, err := testDB.Pool.Exec(ctx, `
			INSERT INTO messages (game_id, author_id, character_id, content, message_type, visibility)
			VALUES ($1, $2, $3, 'public post one', 'post', 'game'), ($1, $2, $3, 'public post two', 'post', 'game')
		`, game.ID, gm.ID, active.ID)
		require.NoError(t, err)

		var convID int32
		err = testDB.Pool.QueryRow(ctx, `
			INSERT INTO conversations (game_id, created_by_user_id, conversation_type)
			VALUES ($1, $2, 'direct') RETURNING id
		`, game.ID, gm.ID).Scan(&convID)
		require.NoError(t, err)

		_, err = testDB.Pool.Exec(ctx, `
			INSERT INTO private_messages (conversation_id, sender_user_id, sender_character_id, content)
			VALUES ($1, $2, $3, 'private message one')
		`, convID, gm.ID, active.ID)
		require.NoError(t, err)

		statsByID, err := svc.GetCharacterActivityStatsByGame(ctx, game.ID)
		require.NoError(t, err)

		require.Contains(t, statsByID, active.ID)
		assert.Equal(t, int64(2), statsByID[active.ID].PublicMessages)
		require.NotNil(t, statsByID[active.ID].PrivateMessages)
		assert.Equal(t, int64(1), *statsByID[active.ID].PrivateMessages)

		require.Contains(t, statsByID, silent.ID, "unrelated character's counts must remain zero")
		assert.Equal(t, int64(0), statsByID[silent.ID].PublicMessages)
	})

	t.Run("does not count soft-deleted messages", func(t *testing.T) {
		_, err := testDB.Pool.Exec(ctx,
			"UPDATE messages SET is_deleted = TRUE WHERE character_id = $1", active.ID)
		require.NoError(t, err)
		_, err = testDB.Pool.Exec(ctx,
			"UPDATE private_messages SET is_deleted = TRUE WHERE sender_character_id = $1", active.ID)
		require.NoError(t, err)

		statsByID, err := svc.GetCharacterActivityStatsByGame(ctx, game.ID)
		require.NoError(t, err)

		assert.Equal(t, int64(0), statsByID[active.ID].PublicMessages, "deleted public messages must not be counted")
		require.NotNil(t, statsByID[active.ID].PrivateMessages)
		assert.Equal(t, int64(0), *statsByID[active.ID].PrivateMessages, "deleted private messages must not be counted")
	})
}

// TestCharacterService_AssignNPCToAudience verifies that the type guard rejects
// player characters and that a valid NPC assignment is persisted.
func TestCharacterService_AssignNPCToAudience(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "npc_assignments", "characters", "games", "users")

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	svc := &CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "npcaud_gm", "npcaud_gm@example.com")
	audience := testDB.CreateTestUser(t, "npcaud_audience", "npcaud_audience@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "NPC Audience Game")

	npc, err := svc.CreateCharacter(ctx, CreateCharacterRequest{
		GameID: game.ID, UserID: nil,
		Name: "TestNPC", CharacterType: "npc",
	})
	require.NoError(t, err)

	playerChar, err := svc.CreateCharacter(ctx, CreateCharacterRequest{
		GameID: game.ID, UserID: core.Int32Ptr(int32(gm.ID)),
		Name: "PlayerChar", CharacterType: "player_character",
	})
	require.NoError(t, err)

	t.Run("rejects assignment of player character type", func(t *testing.T) {
		_, err := svc.AssignNPCToAudience(ctx, playerChar.ID, int32(audience.ID), int32(gm.ID))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "character is not an NPC")
	})

	t.Run("assigns NPC to audience member successfully", func(t *testing.T) {
		assignment, err := svc.AssignNPCToAudience(ctx, npc.ID, int32(audience.ID), int32(gm.ID))
		require.NoError(t, err)
		require.NotNil(t, assignment)
		assert.Equal(t, npc.ID, assignment.CharacterID)
		assert.Equal(t, int32(audience.ID), assignment.AssignedUserID)
		assert.Equal(t, int32(gm.ID), assignment.AssignedByUserID)
	})

	t.Run("returns error for non-existent character", func(t *testing.T) {
		_, err := svc.AssignNPCToAudience(ctx, 999999, int32(audience.ID), int32(gm.ID))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get character")
	})
}
