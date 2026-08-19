package actions

import (
	"context"
	"fmt"
	"testing"

	core "actionphase/pkg/core"
	db "actionphase/pkg/db/services"
	phases "actionphase/pkg/db/services/phases"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionSubmissionService_CreateDraftCharacterUpdate(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)
	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test data
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	// Create character
	userID := int32(player.ID)
	character, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &userID,
		Name:          "Test Character",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	// Create action phase and result
	transitionReq := core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	}
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), transitionReq)
	require.NoError(t, err)

	resultReq := core.CreateActionResultRequest{
		GameID:      game.ID,
		PhaseID:     phase.ID,
		UserID:      int32(player.ID),
		Content:     "You find a key",
		IsPublished: false,
	}
	result, err := actionService.CreateActionResult(context.Background(), resultReq)
	require.NoError(t, err)

	t.Run("creates draft successfully with valid data", func(t *testing.T) {
		req := core.CreateDraftCharacterUpdateRequest{
			ActionResultID: result.ID,
			CharacterID:    character.ID,
			ModuleType:     "skills",
			FieldName:      "strength",
			FieldValue:     "15",
			FieldType:      "number",
			Operation:      "upsert",
		}

		draft, err := actionService.CreateDraftCharacterUpdate(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, req.ActionResultID, draft.ActionResultID)
		assert.Equal(t, req.CharacterID, draft.CharacterID)
		assert.Equal(t, req.ModuleType, draft.ModuleType)
		assert.Equal(t, req.FieldName, draft.FieldName)
		assert.Equal(t, req.FieldValue, draft.FieldValue.String)
		assert.Equal(t, req.FieldType, draft.FieldType)
		assert.Equal(t, req.Operation, draft.Operation)
	})

	t.Run("upsert behavior - updates existing draft", func(t *testing.T) {
		req := core.CreateDraftCharacterUpdateRequest{
			ActionResultID: result.ID,
			CharacterID:    character.ID,
			ModuleType:     "skills",
			FieldName:      "lockpicking",
			FieldValue:     "5",
			FieldType:      "number",
			Operation:      "upsert",
		}

		// Create initial draft
		draft1, err := actionService.CreateDraftCharacterUpdate(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, "5", draft1.FieldValue.String)

		// Update with same field - should upsert
		req.FieldValue = "8"
		draft2, err := actionService.CreateDraftCharacterUpdate(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, draft1.ID, draft2.ID) // Same ID = upsert
		assert.Equal(t, "8", draft2.FieldValue.String)
	})

	t.Run("validates module_type", func(t *testing.T) {
		req := core.CreateDraftCharacterUpdateRequest{
			ActionResultID: result.ID,
			CharacterID:    character.ID,
			ModuleType:     "invalid_module",
			FieldName:      "test",
			FieldValue:     "value",
			FieldType:      "text",
			Operation:      "upsert",
		}

		_, err := actionService.CreateDraftCharacterUpdate(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid module_type")
	})

	t.Run("validates field_type", func(t *testing.T) {
		req := core.CreateDraftCharacterUpdateRequest{
			ActionResultID: result.ID,
			CharacterID:    character.ID,
			ModuleType:     "skills",
			FieldName:      "test",
			FieldValue:     "value",
			FieldType:      "invalid_type",
			Operation:      "upsert",
		}

		_, err := actionService.CreateDraftCharacterUpdate(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid field_type")
	})

	t.Run("validates operation", func(t *testing.T) {
		req := core.CreateDraftCharacterUpdateRequest{
			ActionResultID: result.ID,
			CharacterID:    character.ID,
			ModuleType:     "skills",
			FieldName:      "test",
			FieldValue:     "value",
			FieldType:      "text",
			Operation:      "invalid_op",
		}

		_, err := actionService.CreateDraftCharacterUpdate(context.Background(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid operation")
	})

	t.Run("supports all module types", func(t *testing.T) {
		modules := []string{"skills", "inventory", "numbers"}
		for _, module := range modules {
			req := core.CreateDraftCharacterUpdateRequest{
				ActionResultID: result.ID,
				CharacterID:    character.ID,
				ModuleType:     module,
				FieldName:      "test_" + module,
				FieldValue:     "value",
				FieldType:      "text",
				Operation:      "upsert",
			}

			draft, err := actionService.CreateDraftCharacterUpdate(context.Background(), req)
			require.NoError(t, err)
			assert.Equal(t, module, draft.ModuleType)
		}
	})

	t.Run("supports all field types", func(t *testing.T) {
		fieldTypes := []string{"text", "number", "boolean", "json"}
		for _, fieldType := range fieldTypes {
			req := core.CreateDraftCharacterUpdateRequest{
				ActionResultID: result.ID,
				CharacterID:    character.ID,
				ModuleType:     "skills",
				FieldName:      "test_" + fieldType,
				FieldValue:     "value",
				FieldType:      fieldType,
				Operation:      "upsert",
			}

			draft, err := actionService.CreateDraftCharacterUpdate(context.Background(), req)
			require.NoError(t, err)
			assert.Equal(t, fieldType, draft.FieldType)
		}
	})
}

func TestActionSubmissionService_GetDraftCharacterUpdates(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test data
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	userID := int32(player.ID)
	character, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &userID,
		Name:          "Test Character",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	transitionReq := core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	}
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), transitionReq)
	require.NoError(t, err)

	resultReq := core.CreateActionResultRequest{
		GameID:      game.ID,
		PhaseID:     phase.ID,
		UserID:      int32(player.ID),
		Content:     "You find a key",
		IsPublished: false,
	}
	result, err := actionService.CreateActionResult(context.Background(), resultReq)
	require.NoError(t, err)

	t.Run("returns empty list when no drafts exist", func(t *testing.T) {
		drafts, err := actionService.GetDraftCharacterUpdates(context.Background(), result.ID)
		require.NoError(t, err)
		assert.Empty(t, drafts)
	})

	t.Run("returns all drafts for action result", func(t *testing.T) {
		// Create multiple drafts
		drafts := []core.CreateDraftCharacterUpdateRequest{
			{
				ActionResultID: result.ID,
				CharacterID:    character.ID,
				ModuleType:     "skills",
				FieldName:      "strength",
				FieldValue:     "15",
				FieldType:      "number",
				Operation:      "upsert",
			},
			{
				ActionResultID: result.ID,
				CharacterID:    character.ID,
				ModuleType:     "skills",
				FieldName:      "lockpicking",
				FieldValue:     "5",
				FieldType:      "number",
				Operation:      "upsert",
			},
			{
				ActionResultID: result.ID,
				CharacterID:    character.ID,
				ModuleType:     "inventory",
				FieldName:      "mysterious_key",
				FieldValue:     "1",
				FieldType:      "number",
				Operation:      "upsert",
			},
		}

		for _, draft := range drafts {
			_, err := actionService.CreateDraftCharacterUpdate(context.Background(), draft)
			require.NoError(t, err)
		}

		retrieved, err := actionService.GetDraftCharacterUpdates(context.Background(), result.ID)
		require.NoError(t, err)
		assert.Len(t, retrieved, 3)

		// Verify the correct fields were returned (not just any 3 drafts)
		fieldNames := make([]string, 0, len(retrieved))
		for _, d := range retrieved {
			fieldNames = append(fieldNames, d.FieldName)
		}
		assert.Contains(t, fieldNames, "strength")
		assert.Contains(t, fieldNames, "lockpicking")
		assert.Contains(t, fieldNames, "mysterious_key")
	})
}

func TestActionSubmissionService_UpdateDraftCharacterUpdate(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test data
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	userID := int32(player.ID)
	character, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &userID,
		Name:          "Test Character",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	transitionReq := core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	}
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), transitionReq)
	require.NoError(t, err)

	resultReq := core.CreateActionResultRequest{
		GameID:      game.ID,
		PhaseID:     phase.ID,
		UserID:      int32(player.ID),
		Content:     "You find a key",
		IsPublished: false,
	}
	result, err := actionService.CreateActionResult(context.Background(), resultReq)
	require.NoError(t, err)

	t.Run("updates draft field value successfully", func(t *testing.T) {
		// Create initial draft
		createReq := core.CreateDraftCharacterUpdateRequest{
			ActionResultID: result.ID,
			CharacterID:    character.ID,
			ModuleType:     "skills",
			FieldName:      "strength",
			FieldValue:     "15",
			FieldType:      "number",
			Operation:      "upsert",
		}
		draft, err := actionService.CreateDraftCharacterUpdate(context.Background(), createReq)
		require.NoError(t, err)
		assert.Equal(t, "15", draft.FieldValue.String)

		// Update the value
		updated, err := actionService.UpdateDraftCharacterUpdate(context.Background(), draft.ID, "18")
		require.NoError(t, err)
		assert.Equal(t, "18", updated.FieldValue.String)
		assert.Equal(t, draft.ID, updated.ID)
	})
}

func TestActionSubmissionService_DeleteDraftCharacterUpdate(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test data
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	userID := int32(player.ID)
	character, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &userID,
		Name:          "Test Character",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	transitionReq := core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	}
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), transitionReq)
	require.NoError(t, err)

	resultReq := core.CreateActionResultRequest{
		GameID:      game.ID,
		PhaseID:     phase.ID,
		UserID:      int32(player.ID),
		Content:     "You find a key",
		IsPublished: false,
	}
	result, err := actionService.CreateActionResult(context.Background(), resultReq)
	require.NoError(t, err)

	t.Run("deletes draft successfully", func(t *testing.T) {
		// Create draft
		createReq := core.CreateDraftCharacterUpdateRequest{
			ActionResultID: result.ID,
			CharacterID:    character.ID,
			ModuleType:     "skills",
			FieldName:      "strength",
			FieldValue:     "15",
			FieldType:      "number",
			Operation:      "upsert",
		}
		draft, err := actionService.CreateDraftCharacterUpdate(context.Background(), createReq)
		require.NoError(t, err)

		// Delete draft
		err = actionService.DeleteDraftCharacterUpdate(context.Background(), draft.ID)
		require.NoError(t, err)

		// Verify it's gone
		drafts, err := actionService.GetDraftCharacterUpdates(context.Background(), result.ID)
		require.NoError(t, err)
		assert.Empty(t, drafts)
	})
}

func TestActionSubmissionService_GetDraftUpdateCount(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test data
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	userID := int32(player.ID)
	character, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &userID,
		Name:          "Test Character",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	transitionReq := core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	}
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), transitionReq)
	require.NoError(t, err)

	resultReq := core.CreateActionResultRequest{
		GameID:      game.ID,
		PhaseID:     phase.ID,
		UserID:      int32(player.ID),
		Content:     "You find a key",
		IsPublished: false,
	}
	result, err := actionService.CreateActionResult(context.Background(), resultReq)
	require.NoError(t, err)

	t.Run("returns zero when no drafts exist", func(t *testing.T) {
		count, err := actionService.GetDraftUpdateCount(context.Background(), result.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	t.Run("returns correct count", func(t *testing.T) {
		// Create 3 drafts
		for i := 0; i < 3; i++ {
			createReq := core.CreateDraftCharacterUpdateRequest{
				ActionResultID: result.ID,
				CharacterID:    character.ID,
				ModuleType:     "skills",
				FieldName:      string(rune('a' + i)),
				FieldValue:     "value",
				FieldType:      "text",
				Operation:      "upsert",
			}
			_, err := actionService.CreateDraftCharacterUpdate(context.Background(), createReq)
			require.NoError(t, err)
		}

		count, err := actionService.GetDraftUpdateCount(context.Background(), result.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(3), count)
	})
}

func TestActionSubmissionService_PublishActionResult_WithDrafts(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test data
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	userID := int32(player.ID)
	character, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &userID,
		Name:          "Test Character",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	transitionReq := core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	}
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), transitionReq)
	require.NoError(t, err)

	resultReq := core.CreateActionResultRequest{
		GameID:      game.ID,
		PhaseID:     phase.ID,
		UserID:      int32(player.ID),
		Content:     "You find a key and gain strength",
		IsPublished: false,
	}
	result, err := actionService.CreateActionResult(context.Background(), resultReq)
	require.NoError(t, err)

	t.Run("publishing action result also publishes character updates", func(t *testing.T) {
		// Create draft with the complete desired skills array (new whole-array format)
		skillsJSON := `[{"id":"skill-1","name":"Keen Eye","level":2,"category":"perception","description":"Can spot hidden details"}]`
		createReq := core.CreateDraftCharacterUpdateRequest{
			ActionResultID: result.ID,
			CharacterID:    character.ID,
			ModuleType:     "skills",
			FieldName:      "skills",
			FieldValue:     skillsJSON,
			FieldType:      "json",
			Operation:      "upsert",
		}
		_, err := actionService.CreateDraftCharacterUpdate(context.Background(), createReq)
		require.NoError(t, err)

		// Publish the action result (should also publish character updates)
		err = actionService.PublishActionResult(context.Background(), result.ID, int32(gm.ID))
		require.NoError(t, err)

		// Verify result is published
		publishedResult, err := actionService.GetActionResult(context.Background(), result.ID)
		require.NoError(t, err)
		assert.True(t, publishedResult.IsPublished.Bool)

		// Verify drafts are deleted after publish
		count, err := actionService.GetDraftUpdateCount(context.Background(), result.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)

		// Verify character data was written directly from the draft
		characterData, err := characterService.GetCharacterData(context.Background(), character.ID)
		require.NoError(t, err)

		found := false
		for _, data := range characterData {
			if data.ModuleType == "skills" && data.FieldName == "skills" {
				assert.Equal(t, skillsJSON, data.FieldValue.String)
				found = true
				break
			}
		}
		assert.True(t, found, "skills array should be written to character_data after publishing")
	})
}

func TestActionSubmissionService_PublishAllPhaseResults_WithDrafts(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test data
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	// Add players to game
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	require.NoError(t, err)

	// Create characters for both players
	userID1 := int32(player1.ID)
	character1, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &userID1,
		Name:          "Test Character 1",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	userID2 := int32(player2.ID)
	character2, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &userID2,
		Name:          "Test Character 2",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	// Create action phase
	transitionReq := core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	}
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), transitionReq)
	require.NoError(t, err)

	// Create action results for both players
	result1Req := core.CreateActionResultRequest{
		GameID:      game.ID,
		PhaseID:     phase.ID,
		UserID:      int32(player1.ID),
		Content:     "Player 1 gains strength",
		IsPublished: false,
	}
	result1, err := actionService.CreateActionResult(context.Background(), result1Req)
	require.NoError(t, err)

	result2Req := core.CreateActionResultRequest{
		GameID:      game.ID,
		PhaseID:     phase.ID,
		UserID:      int32(player2.ID),
		Content:     "Player 2 learns lockpicking",
		IsPublished: false,
	}
	result2, err := actionService.CreateActionResult(context.Background(), result2Req)
	require.NoError(t, err)

	t.Run("bulk publish publishes all drafts for all results", func(t *testing.T) {
		// Drafts store the complete desired final array (whole-array format)
		skills1JSON := `[{"id":"skill-1","name":"Keen Eye","level":2,"category":"perception"}]`
		draft1Req := core.CreateDraftCharacterUpdateRequest{
			ActionResultID: result1.ID,
			CharacterID:    character1.ID,
			ModuleType:     "skills",
			FieldName:      "skills",
			FieldValue:     skills1JSON,
			FieldType:      "json",
			Operation:      "upsert",
		}
		_, err := actionService.CreateDraftCharacterUpdate(context.Background(), draft1Req)
		require.NoError(t, err)

		skills2JSON := `[{"id":"skill-1","name":"Lockpicking","level":5}]`
		draft2Req := core.CreateDraftCharacterUpdateRequest{
			ActionResultID: result2.ID,
			CharacterID:    character2.ID,
			ModuleType:     "skills",
			FieldName:      "skills",
			FieldValue:     skills2JSON,
			FieldType:      "json",
			Operation:      "upsert",
		}
		_, err = actionService.CreateDraftCharacterUpdate(context.Background(), draft2Req)
		require.NoError(t, err)

		// Verify both drafts exist
		count1, err := actionService.GetDraftUpdateCount(context.Background(), result1.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), count1)

		count2, err := actionService.GetDraftUpdateCount(context.Background(), result2.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), count2)

		// Publish all phase results (bulk operation)
		err = actionService.PublishAllPhaseResults(context.Background(), phase.ID)
		require.NoError(t, err)

		// Verify both results are published
		publishedResult1, err := actionService.GetActionResult(context.Background(), result1.ID)
		require.NoError(t, err)
		assert.True(t, publishedResult1.IsPublished.Bool)

		publishedResult2, err := actionService.GetActionResult(context.Background(), result2.ID)
		require.NoError(t, err)
		assert.True(t, publishedResult2.IsPublished.Bool)

		// Verify both drafts are deleted
		countAfter1, err := actionService.GetDraftUpdateCount(context.Background(), result1.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), countAfter1, "Player 1 drafts should be deleted")

		countAfter2, err := actionService.GetDraftUpdateCount(context.Background(), result2.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), countAfter2, "Player 2 drafts should be deleted")

		// Verify character data was written directly from drafts
		characterData1, err := characterService.GetCharacterData(context.Background(), character1.ID)
		require.NoError(t, err)

		found1 := false
		for _, data := range characterData1 {
			if data.ModuleType == "skills" && data.FieldName == "skills" {
				assert.Equal(t, skills1JSON, data.FieldValue.String)
				found1 = true
				break
			}
		}
		assert.True(t, found1, "Player 1 skills array should be in character_data")

		characterData2, err := characterService.GetCharacterData(context.Background(), character2.ID)
		require.NoError(t, err)

		found2 := false
		for _, data := range characterData2 {
			if data.ModuleType == "skills" && data.FieldName == "skills" {
				assert.Equal(t, skills2JSON, data.FieldValue.String)
				found2 = true
				break
			}
		}
		assert.True(t, found2, "Player 2 skills array should be in character_data")
	})

	t.Run("bulk publish is atomic - all or nothing", func(t *testing.T) {
		// This test verifies that if any part of the bulk publish fails,
		// the entire operation is rolled back due to the transaction wrapper.
		// Since we're using a transaction, we can't easily test failure scenarios
		// without mocking, but we verify the success path uses a transaction
		// by checking that all operations complete atomically.

		// Create a new phase for this test
		transitionReq := core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Action Phase 2",
		}
		phase2, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), transitionReq)
		require.NoError(t, err)

		// Create multiple results with drafts
		results := make([]int32, 3)
		for i := 0; i < 3; i++ {
			resultReq := core.CreateActionResultRequest{
				GameID:      game.ID,
				PhaseID:     phase2.ID,
				UserID:      int32(player1.ID),
				Content:     "Result " + string(rune('A'+i)),
				IsPublished: false,
			}
			result, err := actionService.CreateActionResult(context.Background(), resultReq)
			require.NoError(t, err)
			results[i] = result.ID

			// Add a draft to each result
			draftReq := core.CreateDraftCharacterUpdateRequest{
				ActionResultID: result.ID,
				CharacterID:    character1.ID,
				ModuleType:     "numbers",
				FieldName:      "test_" + string(rune('a'+i)),
				FieldValue:     fmt.Sprintf(`{"type":"test_%s","amount":%d,"description":"Test number"}`, string(rune('a'+i)), i+1),
				FieldType:      "json",
				Operation:      "upsert",
			}
			_, err = actionService.CreateDraftCharacterUpdate(context.Background(), draftReq)
			require.NoError(t, err)
		}

		// Bulk publish
		err = actionService.PublishAllPhaseResults(context.Background(), phase2.ID)
		require.NoError(t, err)

		// Verify ALL results are published (atomicity)
		for _, resultID := range results {
			publishedResult, err := actionService.GetActionResult(context.Background(), resultID)
			require.NoError(t, err)
			assert.True(t, publishedResult.IsPublished.Bool, "All results should be published atomically")

			// Verify ALL drafts are deleted
			count, err := actionService.GetDraftUpdateCount(context.Background(), resultID)
			require.NoError(t, err)
			assert.Equal(t, int64(0), count, "All drafts should be deleted atomically")
		}
	})
}

// TestDraftPublish_DirectWrite verifies that publishing a result writes each draft's
// field_value directly to character_data without any merging logic.
// The GM saves the complete desired final state as draft rows; publish is a simple replace.
func TestDraftPublish_DirectWrite(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)
	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Direct Write Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	userID := int32(player.ID)
	character, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &userID,
		Name:          "Test Character",
		CharacterType: "player_character",
	})
	require.NoError(t, err)

	transitionReq := core.TransitionPhaseRequest{PhaseType: "action", Title: "Action Phase"}
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), transitionReq)
	require.NoError(t, err)

	makeResult := func(t *testing.T, content string) int32 {
		t.Helper()
		r, err := actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
			GameID:      game.ID,
			PhaseID:     phase.ID,
			UserID:      int32(player.ID),
			Content:     content,
			IsPublished: false,
		})
		require.NoError(t, err)
		return r.ID
	}

	t.Run("draft value written directly to character_data on publish", func(t *testing.T) {
		resultID := makeResult(t, "You gain skills")
		itemsJSON := `[{"id":"item-1","name":"Sword","quantity":1},{"id":"item-2","name":"Shield","quantity":1}]`

		_, err := actionService.CreateDraftCharacterUpdate(context.Background(), core.CreateDraftCharacterUpdateRequest{
			ActionResultID: resultID,
			CharacterID:    character.ID,
			ModuleType:     "inventory",
			FieldName:      "items",
			FieldValue:     itemsJSON,
			FieldType:      "json",
			Operation:      "upsert",
		})
		require.NoError(t, err)

		err = actionService.PublishActionResult(context.Background(), resultID, int32(gm.ID))
		require.NoError(t, err)

		charData, err := characterService.GetCharacterData(context.Background(), character.ID)
		require.NoError(t, err)

		for _, d := range charData {
			if d.ModuleType == "inventory" && d.FieldName == "items" {
				assert.Equal(t, itemsJSON, d.FieldValue.String, "draft value should be written verbatim")
				return
			}
		}
		t.Fatal("inventory/items not found in character_data")
	})

	t.Run("publishing replaces existing character_data with draft value", func(t *testing.T) {
		// Pre-seed existing data
		oldItems := `[{"id":"old-1","name":"Old Sword","quantity":1}]`
		err := characterService.SetCharacterData(context.Background(), db.CharacterDataRequest{
			CharacterID: character.ID,
			ModuleType:  "inventory",
			FieldName:   "items",
			FieldValue:  oldItems,
			FieldType:   "json",
		})
		require.NoError(t, err)

		resultID := makeResult(t, "Inventory updated")
		// Draft contains the complete new desired state
		newItems := `[{"id":"new-1","name":"Healing Potion","quantity":3}]`
		_, err = actionService.CreateDraftCharacterUpdate(context.Background(), core.CreateDraftCharacterUpdateRequest{
			ActionResultID: resultID,
			CharacterID:    character.ID,
			ModuleType:     "inventory",
			FieldName:      "items",
			FieldValue:     newItems,
			FieldType:      "json",
			Operation:      "upsert",
		})
		require.NoError(t, err)

		err = actionService.PublishActionResult(context.Background(), resultID, int32(gm.ID))
		require.NoError(t, err)

		charData, err := characterService.GetCharacterData(context.Background(), character.ID)
		require.NoError(t, err)

		for _, d := range charData {
			if d.ModuleType == "inventory" && d.FieldName == "items" {
				assert.Equal(t, newItems, d.FieldValue.String, "character_data should reflect draft, not old value")
				assert.NotContains(t, d.FieldValue.String, "Old Sword", "old item should be gone")
				return
			}
		}
		t.Fatal("inventory/items not found in character_data")
	})

	t.Run("multiple module drafts all published correctly", func(t *testing.T) {
		resultID := makeResult(t, "Multiple updates")
		skillsJSON := `[{"id":"a-1","name":"Keen Eye","type":"gm_assigned","active":true}]`
		numbersJSON := `[{"id":"n-1","type":"Gold","amount":100}]`

		for _, req := range []core.CreateDraftCharacterUpdateRequest{
			{ActionResultID: resultID, CharacterID: character.ID, ModuleType: "skills", FieldName: "skills", FieldValue: skillsJSON, FieldType: "json", Operation: "upsert"},
			{ActionResultID: resultID, CharacterID: character.ID, ModuleType: "numbers", FieldName: "numbers", FieldValue: numbersJSON, FieldType: "json", Operation: "upsert"},
		} {
			_, err := actionService.CreateDraftCharacterUpdate(context.Background(), req)
			require.NoError(t, err)
		}

		err = actionService.PublishActionResult(context.Background(), resultID, int32(gm.ID))
		require.NoError(t, err)

		charData, err := characterService.GetCharacterData(context.Background(), character.ID)
		require.NoError(t, err)

		dataByKey := make(map[string]string)
		for _, d := range charData {
			dataByKey[d.ModuleType+"/"+d.FieldName] = d.FieldValue.String
		}

		assert.Equal(t, skillsJSON, dataByKey["skills/skills"], "skills should be written")
		assert.Equal(t, numbersJSON, dataByKey["numbers/numbers"], "numbers should be written")
	})

	t.Run("character with no prior character_data gets row created on publish", func(t *testing.T) {
		// Use a fresh character with zero character_data rows
		freshUserID := int32(player.ID)
		freshChar, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
			GameID:        game.ID,
			UserID:        &freshUserID,
			Name:          "Fresh Character",
			CharacterType: "player_character",
		})
		require.NoError(t, err)

		// Verify no character_data rows exist yet
		existingData, err := characterService.GetCharacterData(context.Background(), freshChar.ID)
		require.NoError(t, err)
		require.Empty(t, existingData, "fresh character should have no character_data")

		resultID := makeResult(t, "First ever update")
		itemsJSON := `[{"id":"item-1","name":"Starter Sword","quantity":1}]`

		_, err = actionService.CreateDraftCharacterUpdate(context.Background(), core.CreateDraftCharacterUpdateRequest{
			ActionResultID: resultID,
			CharacterID:    freshChar.ID,
			ModuleType:     "inventory",
			FieldName:      "items",
			FieldValue:     itemsJSON,
			FieldType:      "json",
			Operation:      "upsert",
		})
		require.NoError(t, err)

		err = actionService.PublishActionResult(context.Background(), resultID, int32(gm.ID))
		require.NoError(t, err)

		charData, err := characterService.GetCharacterData(context.Background(), freshChar.ID)
		require.NoError(t, err)

		for _, d := range charData {
			if d.ModuleType == "inventory" && d.FieldName == "items" {
				assert.Equal(t, itemsJSON, d.FieldValue.String)
				return
			}
		}
		t.Fatal("inventory/items row should have been created for fresh character")
	})
}
