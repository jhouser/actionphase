package characters

import (
	"context"
	"testing"

	"actionphase/pkg/core"
	services "actionphase/pkg/db/services"
)

func TestCharacterWorkflow_CompleteApprovalFlow(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "game_participants", "games", "sessions", "users")

	// Setup test data
	gmUser := testDB.CreateTestUser(t, "gm", "gm@example.com")
	gmCommunity := testDB.CreateTestCommunity(t, int32(gmUser.ID))
	playerUser := testDB.CreateTestUser(t, "player", "player@example.com")

	gameService := &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Workflow Test Game",
		Description: "Testing character approval workflow",
		GMUserID:    int32(gmUser.ID),
		CommunityID: gmCommunity.ID,
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Failed to create test game")

	// Add player as participant
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(playerUser.ID), "player")
	core.AssertNoError(t, err, "Failed to add player participant")

	characterService := &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	t.Run("complete character approval workflow", func(t *testing.T) {
		// Step 1: Player creates character
		character, err := characterService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
			GameID:        game.ID,
			UserID:        core.Int32Ptr(int32(playerUser.ID)),
			Name:          "Workflow Character",
			CharacterType: "player_character",
		})
		core.AssertNoError(t, err, "Failed to create character")
		core.AssertEqual(t, "pending", character.Status.String, "Character should start as pending")

		// Step 2: Player adds character data while pending
		err = characterService.SetCharacterData(context.Background(), services.CharacterDataRequest{
			CharacterID: character.ID,
			ModuleType:  "bio",
			FieldName:   "background",
			FieldValue:  "A heroic character background",
			FieldType:   "text",
			IsPublic:    true,
		})
		core.AssertNoError(t, err, "Player should be able to add character data while pending")

		// Step 3: GM reviews and approves character
		approved, err := characterService.ApproveCharacter(context.Background(), character.ID)
		core.AssertNoError(t, err, "GM should be able to approve character")
		core.AssertEqual(t, "approved", approved.Status.String, "Character should be approved")

		// Step 4: Player can still edit approved character data
		err = characterService.SetCharacterData(context.Background(), services.CharacterDataRequest{
			CharacterID: character.ID,
			ModuleType:  "bio",
			FieldName:   "background",
			FieldValue:  "Updated heroic character background",
			FieldType:   "text",
			IsPublic:    true,
		})
		core.AssertNoError(t, err, "Player should be able to edit approved character data")

		// Step 5: Verify final state
		final, err := characterService.GetCharacter(context.Background(), character.ID)
		core.AssertNoError(t, err, "Should be able to get final character")
		core.AssertEqual(t, "approved", final.Status.String, "Character should remain approved")

		data, err := characterService.GetCharacterData(context.Background(), character.ID)
		core.AssertNoError(t, err, "Should be able to get character data")
		core.AssertEqual(t, 1, len(data), "Should have character data")
		core.AssertEqual(t, "Updated heroic character background", data[0].FieldValue.String, "Data should be updated")
	})

	t.Run("GM deletes unwanted character instead of rejecting", func(t *testing.T) {
		// Create a character that the GM wants to decline
		character, err := characterService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
			GameID:        game.ID,
			UserID:        core.Int32Ptr(int32(playerUser.ID)),
			Name:          "Unwanted Character",
			CharacterType: "player_character",
		})
		core.AssertNoError(t, err, "Failed to create character")

		// GM deletes the character (replaces rejection)
		err = characterService.DeleteCharacter(context.Background(), character.ID)
		core.AssertNoError(t, err, "GM should be able to delete a pending character")

		// Character no longer exists
		_, err = characterService.GetCharacter(context.Background(), character.ID)
		core.AssertError(t, err, "Deleted character should not be retrievable")
	})
}

func TestCharacterWorkflow_NPCAssignmentFlow(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "game_participants", "games", "sessions", "users")

	// Setup test data
	gmUser := testDB.CreateTestUser(t, "gm", "gm@example.com")
	gmCommunity := testDB.CreateTestCommunity(t, int32(gmUser.ID))
	audienceUser1 := testDB.CreateTestUser(t, "audience1", "audience1@example.com")
	audienceUser2 := testDB.CreateTestUser(t, "audience2", "audience2@example.com")

	gameService := &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "NPC Assignment Test Game",
		Description: "Testing NPC assignment workflow",
		GMUserID:    int32(gmUser.ID),
		CommunityID: gmCommunity.ID,
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Failed to create test game")

	// Add audience members as participants
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(audienceUser1.ID), "audience")
	core.AssertNoError(t, err, "Failed to add audience participant 1")

	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(audienceUser2.ID), "audience")
	core.AssertNoError(t, err, "Failed to add audience participant 2")

	characterService := &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	t.Run("complete NPC assignment workflow", func(t *testing.T) {
		// Step 1: GM creates audience NPC
		npc, err := characterService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
			GameID:        game.ID,
			UserID:        nil, // No user assigned initially
			Name:          "Tavern Keeper",
			CharacterType: "npc",
		})
		core.AssertNoError(t, err, "Failed to create NPC")
		core.AssertEqual(t, false, npc.UserID.Valid, "NPC should not have user assigned initially")

		// Step 2: GM assigns NPC to audience member
		err = characterService.AssignNPCToUser(context.Background(), npc.ID, int32(audienceUser1.ID), int32(gmUser.ID))
		core.AssertNoError(t, err, "GM should be able to assign NPC")

		// Step 3: Verify assignment permissions
		canEditUser1, err := characterService.CanUserEditCharacter(context.Background(), npc.ID, int32(audienceUser1.ID))
		core.AssertNoError(t, err, "Should be able to check edit permission")
		core.AssertEqual(t, true, canEditUser1, "Assigned user should be able to edit NPC")

		canEditUser2, err := characterService.CanUserEditCharacter(context.Background(), npc.ID, int32(audienceUser2.ID))
		core.AssertNoError(t, err, "Should be able to check edit permission")
		core.AssertEqual(t, false, canEditUser2, "Unassigned user should not be able to edit NPC")

		gmCanEdit, err := characterService.CanUserEditCharacter(context.Background(), npc.ID, int32(gmUser.ID))
		core.AssertNoError(t, err, "Should be able to check GM edit permission")
		core.AssertEqual(t, true, gmCanEdit, "GM should always be able to edit NPCs")

		// Step 4: Assigned user adds character data
		err = characterService.SetCharacterData(context.Background(), services.CharacterDataRequest{
			CharacterID: npc.ID,
			ModuleType:  "bio",
			FieldName:   "personality",
			FieldValue:  "Gruff but kind-hearted",
			FieldType:   "text",
			IsPublic:    true,
		})
		core.AssertNoError(t, err, "Assigned user should be able to add NPC data")

		// Step 5: Reassign NPC to different user
		err = characterService.AssignNPCToUser(context.Background(), npc.ID, int32(audienceUser2.ID), int32(gmUser.ID))
		core.AssertNoError(t, err, "GM should be able to reassign NPC")

		// Step 6: Verify new assignment
		canEditUser1After, err := characterService.CanUserEditCharacter(context.Background(), npc.ID, int32(audienceUser1.ID))
		core.AssertNoError(t, err, "Should be able to check edit permission after reassignment")
		core.AssertEqual(t, false, canEditUser1After, "Previously assigned user should lose edit permission")

		canEditUser2After, err := characterService.CanUserEditCharacter(context.Background(), npc.ID, int32(audienceUser2.ID))
		core.AssertNoError(t, err, "Should be able to check edit permission after reassignment")
		core.AssertEqual(t, true, canEditUser2After, "Newly assigned user should gain edit permission")
	})
}

func TestCharacterWorkflow_PermissionMatrix(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "game_participants", "games", "sessions", "users")

	// Setup comprehensive test scenario
	gmUser := testDB.CreateTestUser(t, "gm", "gm@example.com")
	gmCommunity := testDB.CreateTestCommunity(t, int32(gmUser.ID))
	playerUser := testDB.CreateTestUser(t, "player", "player@example.com")
	audienceUser := testDB.CreateTestUser(t, "audience", "audience@example.com")
	outsideUser := testDB.CreateTestUser(t, "outside", "outside@example.com")

	gameService := &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game, err := gameService.CreateGame(context.Background(), core.CreateGameRequest{
		Title:       "Permission Matrix Test Game",
		Description: "Testing all permission combinations",
		GMUserID:    int32(gmUser.ID),
		CommunityID: gmCommunity.ID,
		IsPublic:    true,
	})
	core.AssertNoError(t, err, "Failed to create test game")

	// Add participants
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(playerUser.ID), "player")
	core.AssertNoError(t, err, "Failed to add player participant")

	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(audienceUser.ID), "audience")
	core.AssertNoError(t, err, "Failed to add audience participant")

	characterService := &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create different types of characters
	playerCharacter, err := characterService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        core.Int32Ptr(int32(playerUser.ID)),
		Name:          "Player Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Failed to create player character")

	gmNPC, err := characterService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        nil,
		Name:          "GM NPC",
		CharacterType: "npc",
	})
	core.AssertNoError(t, err, "Failed to create GM NPC")

	audienceNPC, err := characterService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        nil,
		Name:          "Audience NPC",
		CharacterType: "npc",
	})
	core.AssertNoError(t, err, "Failed to create audience NPC")

	// Assign audience NPC
	err = characterService.AssignNPCToUser(context.Background(), audienceNPC.ID, int32(audienceUser.ID), int32(gmUser.ID))
	core.AssertNoError(t, err, "Failed to assign audience NPC")

	// Test permission matrix
	testCases := []struct {
		name        string
		characterID int32
		userID      int32
		canEdit     bool
		reason      string
	}{
		{
			name:        "GM can edit player character",
			characterID: playerCharacter.ID,
			userID:      int32(gmUser.ID),
			canEdit:     true,
			reason:      "GM has edit access to all characters",
		},
		{
			name:        "Player can edit own character",
			characterID: playerCharacter.ID,
			userID:      int32(playerUser.ID),
			canEdit:     true,
			reason:      "Player owns the character",
		},
		{
			name:        "Audience cannot edit player character",
			characterID: playerCharacter.ID,
			userID:      int32(audienceUser.ID),
			canEdit:     false,
			reason:      "Audience member doesn't own character",
		},
		{
			name:        "Outside user cannot edit player character",
			characterID: playerCharacter.ID,
			userID:      int32(outsideUser.ID),
			canEdit:     false,
			reason:      "Outside user has no access",
		},
		{
			name:        "GM can edit GM NPC",
			characterID: gmNPC.ID,
			userID:      int32(gmUser.ID),
			canEdit:     true,
			reason:      "GM owns GM NPCs",
		},
		{
			name:        "Player cannot edit GM NPC",
			characterID: gmNPC.ID,
			userID:      int32(playerUser.ID),
			canEdit:     false,
			reason:      "Player has no access to GM NPCs",
		},
		{
			name:        "GM can edit assigned audience NPC",
			characterID: audienceNPC.ID,
			userID:      int32(gmUser.ID),
			canEdit:     true,
			reason:      "GM has access to all NPCs",
		},
		{
			name:        "Assigned audience can edit audience NPC",
			characterID: audienceNPC.ID,
			userID:      int32(audienceUser.ID),
			canEdit:     true,
			reason:      "Audience member is assigned to NPC",
		},
		{
			name:        "Player cannot edit audience NPC",
			characterID: audienceNPC.ID,
			userID:      int32(playerUser.ID),
			canEdit:     false,
			reason:      "Player is not assigned to NPC",
		},
		{
			name:        "Outside user cannot edit audience NPC",
			characterID: audienceNPC.ID,
			userID:      int32(outsideUser.ID),
			canEdit:     false,
			reason:      "Outside user has no access",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			canEdit, err := characterService.CanUserEditCharacter(context.Background(), tc.characterID, tc.userID)
			core.AssertNoError(t, err, "Failed to check edit permission")
			core.AssertEqual(t, tc.canEdit, canEdit, tc.reason)
		})
	}
}
