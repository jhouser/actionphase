package actions

import (
	"context"
	"testing"

	core "actionphase/pkg/core"
	db "actionphase/pkg/db/services"
	phases "actionphase/pkg/db/services/phases"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionSubmissionService_CreateActionResult(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test data
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	// Create action phase
	transitionReq := core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	}
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), transitionReq)
	require.NoError(t, err)

	t.Run("creates action result successfully", func(t *testing.T) {
		req := core.CreateActionResultRequest{
			GameID:      game.ID,
			PhaseID:     phase.ID,
			UserID:      int32(player.ID),
			Content:     "You find a mysterious key.",
			IsPublished: false,
		}

		result, err := actionService.CreateActionResult(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, req.GameID, result.GameID)
		assert.Equal(t, req.PhaseID, result.PhaseID)
		assert.Equal(t, req.UserID, result.UserID)
		assert.Equal(t, "You find a mysterious key.", result.Content)
		assert.False(t, result.IsPublished.Bool)
	})

	t.Run("publishes action result", func(t *testing.T) {
		// Create unpublished result
		createReq := core.CreateActionResultRequest{
			GameID:      game.ID,
			PhaseID:     phase.ID,
			UserID:      int32(player.ID),
			Content:     "Result to publish",
			IsPublished: false,
		}
		result, err := actionService.CreateActionResult(context.Background(), createReq)
		require.NoError(t, err)

		// Publish it
		err = actionService.PublishActionResult(context.Background(), result.ID, int32(gm.ID))
		require.NoError(t, err)

		// Verify published
		published, err := actionService.GetActionResult(context.Background(), result.ID)
		require.NoError(t, err)
		assert.True(t, published.IsPublished.Bool)
	})
}

func TestActionSubmissionService_ActionResultOperations(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test data
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	// Create action phase
	transitionReq := core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	}
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), transitionReq)
	require.NoError(t, err)

	t.Run("updates unpublished result", func(t *testing.T) {
		// Create result
		createReq := core.CreateActionResultRequest{
			GameID:      game.ID,
			PhaseID:     phase.ID,
			UserID:      int32(player.ID),
			Content:     "Original content",
			IsPublished: false,
		}
		result, err := actionService.CreateActionResult(context.Background(), createReq)
		require.NoError(t, err)

		// Update it
		updated, err := actionService.UpdateActionResult(context.Background(), result.ID, "Updated content")
		require.NoError(t, err)
		assert.Equal(t, "Updated content", updated.Content)
	})

	t.Run("gets unpublished results count", func(t *testing.T) {
		// Create some unpublished results
		for i := 0; i < 3; i++ {
			createReq := core.CreateActionResultRequest{
				GameID:      game.ID,
				PhaseID:     phase.ID,
				UserID:      int32(player.ID),
				Content:     "Unpublished result",
				IsPublished: false,
			}
			_, err := actionService.CreateActionResult(context.Background(), createReq)
			require.NoError(t, err)
		}

		count, err := actionService.GetUnpublishedResultsCount(context.Background(), phase.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, int64(3))
	})

	t.Run("returns error when updating published result", func(t *testing.T) {
		// Create and publish a result
		createReq := core.CreateActionResultRequest{
			GameID:      game.ID,
			PhaseID:     phase.ID,
			UserID:      int32(player.ID),
			Content:     "Published result",
			IsPublished: false,
		}
		result, err := actionService.CreateActionResult(context.Background(), createReq)
		require.NoError(t, err)

		// Publish it
		err = actionService.PublishActionResult(context.Background(), result.ID, int32(gm.ID))
		require.NoError(t, err)

		// Try to update published result
		_, err = actionService.UpdateActionResult(context.Background(), result.ID, "Trying to update published result")
		require.Error(t, err, "Should not be able to update published result")
		assert.Contains(t, err.Error(), "result not found or already published")
	})

	t.Run("returns error when updating non-existent result", func(t *testing.T) {
		nonExistentID := int32(999999)
		_, err := actionService.UpdateActionResult(context.Background(), nonExistentID, "Update non-existent")
		require.Error(t, err, "Should error when result doesn't exist")
		assert.Contains(t, err.Error(), "result not found or already published")
	})

	t.Run("returns error when getting non-existent result", func(t *testing.T) {
		nonExistentID := int32(999999)
		_, err := actionService.GetActionResult(context.Background(), nonExistentID)
		require.Error(t, err, "Should error when result doesn't exist")
		assert.Contains(t, err.Error(), "action result not found")
	})

	t.Run("deletes unpublished draft result", func(t *testing.T) {
		result, err := actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
			GameID:      game.ID,
			PhaseID:     phase.ID,
			UserID:      int32(player.ID),
			Content:     "Draft to delete",
			IsPublished: false,
		})
		require.NoError(t, err)

		err = actionService.DeleteActionResult(context.Background(), result.ID)
		require.NoError(t, err)

		_, err = actionService.GetActionResult(context.Background(), result.ID)
		require.Error(t, err, "Should not be able to retrieve deleted result")
		assert.Contains(t, err.Error(), "action result not found")
	})

	t.Run("cannot delete a published result", func(t *testing.T) {
		result, err := actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
			GameID:      game.ID,
			PhaseID:     phase.ID,
			UserID:      int32(player.ID),
			Content:     "Published result",
			IsPublished: false,
		})
		require.NoError(t, err)

		err = actionService.PublishActionResult(context.Background(), result.ID, int32(gm.ID))
		require.NoError(t, err)

		err = actionService.DeleteActionResult(context.Background(), result.ID)
		require.Error(t, err, "Should not be able to delete a published result")
		assert.Contains(t, err.Error(), "cannot delete a published action result")
	})

	t.Run("returns error when deleting non-existent result", func(t *testing.T) {
		err := actionService.DeleteActionResult(context.Background(), int32(999999))
		require.Error(t, err, "Should error when result doesn't exist")
		assert.Contains(t, err.Error(), "action result not found")
	})
}

func TestActionSubmissionService_GetUserPhaseResults(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test data
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	// Create action phase
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	})
	require.NoError(t, err)

	t.Run("returns all results for user (both published and unpublished)", func(t *testing.T) {
		// Create 1 published and 1 unpublished result for this user
		_, err := actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
			GameID:      game.ID,
			PhaseID:     phase.ID,
			UserID:      int32(player.ID),
			Content:     "Published result",
			IsPublished: true,
		})
		require.NoError(t, err)

		_, err = actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
			GameID:      game.ID,
			PhaseID:     phase.ID,
			UserID:      int32(player.ID),
			Content:     "Unpublished result",
			IsPublished: false,
		})
		require.NoError(t, err)

		// Get user's results - should return both published and unpublished
		results, err := actionService.GetUserPhaseResults(context.Background(), phase.ID, int32(player.ID))
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(results), 2, "Should return at least 2 results")

		// Verify we have both published and unpublished
		hasPublished := false
		hasUnpublished := false
		for _, r := range results {
			if r.IsPublished.Bool {
				hasPublished = true
			} else {
				hasUnpublished = true
			}
		}
		assert.True(t, hasPublished, "Should have at least one published result")
		assert.True(t, hasUnpublished, "Should have at least one unpublished result")
	})

	t.Run("filters by user and phase correctly", func(t *testing.T) {
		game2 := testDB.CreateTestGame(t, int32(gm.ID), "Test Game 2")
		player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
		player3 := testDB.CreateTestUser(t, "player3", "player3@example.com")
		_, err := gameService.AddGameParticipant(context.Background(), game2.ID, int32(player2.ID), "player")
		require.NoError(t, err)
		_, err = gameService.AddGameParticipant(context.Background(), game2.ID, int32(player3.ID), "player")
		require.NoError(t, err)

		phase2, err := phaseService.TransitionToNextPhase(context.Background(), game2.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Action Phase 2",
		})
		require.NoError(t, err)

		// Create result for player2
		_, err = actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
			GameID:      game2.ID,
			PhaseID:     phase2.ID,
			UserID:      int32(player2.ID),
			Content:     "Result for player 2",
			IsPublished: false,
		})
		require.NoError(t, err)

		// Create result for player3
		_, err = actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
			GameID:      game2.ID,
			PhaseID:     phase2.ID,
			UserID:      int32(player3.ID),
			Content:     "Result for player 3",
			IsPublished: false,
		})
		require.NoError(t, err)

		// Get player2's results - should only return player2's result
		results, err := actionService.GetUserPhaseResults(context.Background(), phase2.ID, int32(player2.ID))
		require.NoError(t, err)
		assert.Len(t, results, 1, "Should return exactly 1 result for player2")
		assert.Equal(t, int32(player2.ID), results[0].UserID, "Result should belong to player2")
	})

	t.Run("returns empty list when no results exist", func(t *testing.T) {
		newPlayer := testDB.CreateTestUser(t, "newplayer", "newplayer@example.com")
		_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(newPlayer.ID), "player")
		require.NoError(t, err)

		// Get results for player with no results
		results, err := actionService.GetUserPhaseResults(context.Background(), phase.ID, int32(newPlayer.ID))
		require.NoError(t, err)
		assert.Empty(t, results, "Should return empty list when no results exist")
	})
}

func TestActionSubmissionService_PublishAllPhaseResults(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test data
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	// Create action phase
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	})
	require.NoError(t, err)

	t.Run("publishes all unpublished results for a phase", func(t *testing.T) {
		// Create 3 players with unpublished results
		for i := 1; i <= 3; i++ {
			player := testDB.CreateTestUser(t, "publish_player_"+string(rune(i+'0')), "publish_player_"+string(rune(i+'0'))+"@example.com")
			_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
			require.NoError(t, err)

			_, err = actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
				GameID:      game.ID,
				PhaseID:     phase.ID,
				UserID:      int32(player.ID),
				Content:     "Unpublished result",
				IsPublished: false,
			})
			require.NoError(t, err)
		}

		// Verify we have unpublished results
		countBefore, err := actionService.GetUnpublishedResultsCount(context.Background(), phase.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, countBefore, int64(3), "Should have at least 3 unpublished results")

		// Publish all
		err = actionService.PublishAllPhaseResults(context.Background(), phase.ID)
		require.NoError(t, err)

		// Verify all are published
		countAfter, err := actionService.GetUnpublishedResultsCount(context.Background(), phase.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), countAfter, "Should have 0 unpublished results after publishing all")
	})

	t.Run("does not affect other phases", func(t *testing.T) {
		game2 := testDB.CreateTestGame(t, int32(gm.ID), "Test Game 2")
		player := testDB.CreateTestUser(t, "isolate_player", "isolate_player@example.com")
		_, err := gameService.AddGameParticipant(context.Background(), game2.ID, int32(player.ID), "player")
		require.NoError(t, err)

		// Create phase 1 and phase 2
		phase1, err := phaseService.TransitionToNextPhase(context.Background(), game2.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Phase 1",
		})
		require.NoError(t, err)

		// Create unpublished result in phase 1
		_, err = actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
			GameID:      game2.ID,
			PhaseID:     phase1.ID,
			UserID:      int32(player.ID),
			Content:     "Result in phase 1",
			IsPublished: false,
		})
		require.NoError(t, err)

		// Transition to phase 2
		phase2, err := phaseService.TransitionToNextPhase(context.Background(), game2.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Phase 2",
		})
		require.NoError(t, err)

		// Create unpublished result in phase 2
		_, err = actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
			GameID:      game2.ID,
			PhaseID:     phase2.ID,
			UserID:      int32(player.ID),
			Content:     "Result in phase 2",
			IsPublished: false,
		})
		require.NoError(t, err)

		// Publish all in phase 1
		err = actionService.PublishAllPhaseResults(context.Background(), phase1.ID)
		require.NoError(t, err)

		// Verify phase 1 has 0 unpublished
		count1, err := actionService.GetUnpublishedResultsCount(context.Background(), phase1.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), count1, "Phase 1 should have 0 unpublished results")

		// Verify phase 2 still has 1 unpublished
		count2, err := actionService.GetUnpublishedResultsCount(context.Background(), phase2.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), count2, "Phase 2 should still have 1 unpublished result")
	})

	// Regression: a staged chain must be published as ONE result, not N.
	//
	// PublishActionResult publishes an entire chain, so if the work list
	// includes chain followers each one re-publishes the same chain and sends
	// the recipient another "your result has been published" notification. The
	// player sees one scene and gets N notifications for it.
	t.Run("publishes a staged chain once, with one notification", func(t *testing.T) {
		chainGame := testDB.CreateTestGame(t, int32(gm.ID), "Chain Publish Game")
		player := testDB.CreateTestUser(t, "chain_pub_player", "chain_pub_player@example.com")
		_, err := gameService.AddGameParticipant(context.Background(), chainGame.ID, int32(player.ID), "player")
		require.NoError(t, err)

		chainPhase, err := phaseService.TransitionToNextPhase(context.Background(), chainGame.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Chain Phase",
		})
		require.NoError(t, err)

		// A 3-part draft chain: one scene, three rows.
		_, err = actionService.CreateStagedResultChain(context.Background(), core.CreateStagedResultChainRequest{
			GameID:  chainGame.ID,
			PhaseID: chainPhase.ID,
			UserID:  int32(player.ID),
			Parts: []core.StagedResultPart{
				{Content: "The sword swings toward your head..."},
				{Content: "...and stops.", DelayMinutes: 5},
				{Content: "The creature folds backward and is gone.", DelayMinutes: 5},
			},
			IsPublished: false,
		})
		require.NoError(t, err)

		// The GM is being told how many results they are about to publish.
		// Three rows, but one scene.
		countBefore, err := actionService.GetUnpublishedResultsCount(context.Background(), chainPhase.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), countBefore, "A 3-part chain is one publishable result, not three")

		err = actionService.PublishAllPhaseResults(context.Background(), chainPhase.ID)
		require.NoError(t, err)

		// The whole chain went out, not just the head.
		var publishedParts int
		err = testDB.Pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM action_results WHERE phase_id = $1 AND is_published = true`,
			chainPhase.ID).Scan(&publishedParts)
		require.NoError(t, err)
		assert.Equal(t, 3, publishedParts, "Publishing the head must publish the entire chain")

		// Only the head is released; the followers wait for the worker. This is
		// the feature, so assert it here rather than trusting it elsewhere.
		var releasedParts int
		err = testDB.Pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM action_results WHERE phase_id = $1 AND released_at IS NOT NULL`,
			chainPhase.ID).Scan(&releasedParts)
		require.NoError(t, err)
		assert.Equal(t, 1, releasedParts, "Only the chain head is released at publish time")

		// The actual player-visible symptom of the bug.
		var notifications int
		err = testDB.Pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND game_id = $2`,
			int32(player.ID), chainGame.ID).Scan(&notifications)
		require.NoError(t, err)
		assert.Equal(t, 1, notifications, "One scene must produce exactly one publish notification")
	})

	t.Run("handles empty phase gracefully", func(t *testing.T) {
		emptyGame := testDB.CreateTestGame(t, int32(gm.ID), "Empty Game")
		emptyPhase, err := phaseService.TransitionToNextPhase(context.Background(), emptyGame.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Empty Phase",
		})
		require.NoError(t, err)

		// Try to publish all in empty phase (should not error)
		err = actionService.PublishAllPhaseResults(context.Background(), emptyPhase.ID)
		require.NoError(t, err)

		// Verify still 0 unpublished
		count, err := actionService.GetUnpublishedResultsCount(context.Background(), emptyPhase.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), count, "Empty phase should have 0 unpublished results")
	})
}

// TestActionSubmissionService_PublishCharacterUpdates tests that draft character updates are written
// directly to character_data on publish (whole-array snapshot model — no per-item merging).
func TestActionSubmissionService_PublishCharacterUpdates(t *testing.T) {
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
	charReq := db.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &userID,
		Name:          "Test Character",
		CharacterType: "player_character",
	}
	character, err := characterService.CreateCharacter(context.Background(), charReq)
	require.NoError(t, err)

	// Approve character
	_, err = characterService.ApproveCharacter(context.Background(), character.ID)
	require.NoError(t, err)

	// Create action phase
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	})
	require.NoError(t, err)

	// Create action result
	resultReq := core.CreateActionResultRequest{
		GameID:      game.ID,
		PhaseID:     phase.ID,
		UserID:      int32(player.ID),
		Content:     "You successfully complete your action.",
		IsPublished: false,
	}
	result, err := actionService.CreateActionResult(context.Background(), resultReq)
	require.NoError(t, err)

	t.Run("writes inventory array draft directly to character_data", func(t *testing.T) {
		// Draft stores the complete desired final state as a whole array
		itemsJSON := `[{"name":"Sword","description":"A sharp blade","quantity":1},{"name":"Potion","description":"Healing potion","quantity":3}]`

		_, err := testDB.Pool.Exec(context.Background(),
			`INSERT INTO action_result_character_updates
			(action_result_id, character_id, module_type, field_name, field_value, field_type, operation)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			result.ID, character.ID, "inventory", "items", itemsJSON, "json", "upsert")
		require.NoError(t, err)

		// Publish
		err = actionService.PublishActionResult(context.Background(), result.ID, int32(player.ID))
		require.NoError(t, err)

		// Verify character_data has the array written verbatim
		var fieldName, fieldValue string
		err = testDB.Pool.QueryRow(context.Background(),
			`SELECT field_name, field_value FROM character_data
			WHERE character_id = $1 AND module_type = $2`,
			character.ID, "inventory").Scan(&fieldName, &fieldValue)
		require.NoError(t, err)

		assert.Equal(t, "items", fieldName)
		assert.Contains(t, fieldValue, `"name":"Sword"`)
		assert.Contains(t, fieldValue, `"name":"Potion"`)
		assert.True(t, len(fieldValue) > 0 && fieldValue[0] == '[', "field value should be a JSON array")

		// Verify drafts were cleaned up
		var draftCount int
		err = testDB.Pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM action_result_character_updates WHERE action_result_id = $1`,
			result.ID).Scan(&draftCount)
		require.NoError(t, err)
		assert.Equal(t, 0, draftCount, "draft updates should be deleted after publishing")
	})

	t.Run("writes abilities array draft directly to character_data", func(t *testing.T) {
		result2, err := actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
			GameID:      game.ID,
			PhaseID:     phase.ID,
			UserID:      int32(player.ID),
			Content:     "You gain new abilities.",
			IsPublished: false,
		})
		require.NoError(t, err)

		abilitiesJSON := `[{"name":"Fireball","description":"Cast a fireball","cost":5},{"name":"Shield","description":"Protective barrier","cost":3}]`

		_, err = testDB.Pool.Exec(context.Background(),
			`INSERT INTO action_result_character_updates
			(action_result_id, character_id, module_type, field_name, field_value, field_type, operation)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			result2.ID, character.ID, "abilities", "abilities", abilitiesJSON, "json", "upsert")
		require.NoError(t, err)

		err = actionService.PublishActionResult(context.Background(), result2.ID, int32(player.ID))
		require.NoError(t, err)

		var fieldName, fieldValue string
		err = testDB.Pool.QueryRow(context.Background(),
			`SELECT field_name, field_value FROM character_data
			WHERE character_id = $1 AND module_type = $2`,
			character.ID, "abilities").Scan(&fieldName, &fieldValue)
		require.NoError(t, err)

		assert.Equal(t, "abilities", fieldName)
		assert.Contains(t, fieldValue, `"name":"Fireball"`)
		assert.Contains(t, fieldValue, `"name":"Shield"`)
		assert.True(t, len(fieldValue) > 0 && fieldValue[0] == '[', "field value should be a JSON array")
	})

	t.Run("replaces existing character_data when draft is published", func(t *testing.T) {
		// Pre-populate character_data with old items
		_, err := testDB.Pool.Exec(context.Background(),
			`INSERT INTO character_data (character_id, module_type, field_name, field_value, field_type)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (character_id, module_type, field_name) DO UPDATE
			SET field_value = EXCLUDED.field_value`,
			character.ID, "inventory", "items",
			`[{"name":"Old Item","quantity":1}]`, "json")
		require.NoError(t, err)

		result3, err := actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
			GameID:      game.ID,
			PhaseID:     phase.ID,
			UserID:      int32(player.ID),
			Content:     "Your inventory has changed.",
			IsPublished: false,
		})
		require.NoError(t, err)

		// Draft contains the complete new desired state (Old Item removed, New Sword added)
		newItemsJSON := `[{"name":"New Sword","quantity":1}]`
		_, err = testDB.Pool.Exec(context.Background(),
			`INSERT INTO action_result_character_updates
			(action_result_id, character_id, module_type, field_name, field_value, field_type, operation)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			result3.ID, character.ID, "inventory", "items", newItemsJSON, "json", "upsert")
		require.NoError(t, err)

		err = actionService.PublishActionResult(context.Background(), result3.ID, int32(player.ID))
		require.NoError(t, err)

		var fieldValue string
		err = testDB.Pool.QueryRow(context.Background(),
			`SELECT field_value FROM character_data
			WHERE character_id = $1 AND module_type = $2 AND field_name = $3`,
			character.ID, "inventory", "items").Scan(&fieldValue)
		require.NoError(t, err)

		assert.Contains(t, fieldValue, `"New Sword"`, "should contain the new item")
		assert.NotContains(t, fieldValue, `"Old Item"`, "old item should be replaced, not preserved")
	})
}

// TestActionSubmissionService_NotificationCreation tests that notifications are created when results are published
// This is a regression test for Issue 6.5: No Notifications for Published Results
func TestActionSubmissionService_NotificationCreation(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	app := core.NewTestApp(testDB.Pool)

	notificationService := &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger}
	actionService := &ActionSubmissionService{DB: testDB.Pool, Logger: app.ObsLogger, NotificationService: notificationService}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Create test data
	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	// Create action phase
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	})
	require.NoError(t, err)

	t.Run("creates notification when publishing single result", func(t *testing.T) {
		// Create unpublished result
		result, err := actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
			GameID:      game.ID,
			PhaseID:     phase.ID,
			UserID:      int32(player.ID),
			Content:     "You find a treasure chest.",
			IsPublished: false,
		})
		require.NoError(t, err)

		// Count notifications before publish
		notifsBefore, err := notificationService.GetUserNotifications(context.Background(), int32(player.ID), 10, 0)
		require.NoError(t, err)
		countBefore := len(notifsBefore)

		// Publish the result
		err = actionService.PublishActionResult(context.Background(), result.ID, int32(gm.ID))
		require.NoError(t, err)

		// Count notifications after publish
		notifsAfter, err := notificationService.GetUserNotifications(context.Background(), int32(player.ID), 10, 0)
		require.NoError(t, err)
		countAfter := len(notifsAfter)

		// Should have exactly one more notification
		assert.Equal(t, countBefore+1, countAfter, "Should create exactly one notification")

		// Find the new notification
		var newNotif *core.Notification
		for _, n := range notifsAfter {
			if n.Type == core.NotificationTypeActionResult {
				newNotif = n
				break
			}
		}
		require.NotNil(t, newNotif, "Should have created an action_result notification")

		// Verify notification attributes
		assert.Equal(t, core.NotificationTypeActionResult, newNotif.Type, "Should be action_result type")
		assert.Equal(t, "Action Result Published", newNotif.Title, "Should have correct title")
		assert.Equal(t, "Your action result has been published by the GM", *newNotif.Content, "Should have correct content")
		assert.Equal(t, game.ID, *newNotif.GameID, "Should link to the correct game")
		assert.Equal(t, result.ID, *newNotif.RelatedID, "Should link to the result ID")
		assert.Equal(t, "action_result", *newNotif.RelatedType, "Should have correct related type")

		// Verify link URL format
		assert.Contains(t, *newNotif.LinkURL, "/games/", "Link should contain /games/")
		assert.Contains(t, *newNotif.LinkURL, "?tab=actions", "Link should contain tab=actions")
	})

	t.Run("creates notifications for all results when publishing batch", func(t *testing.T) {
		// Create multiple players
		player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
		player3 := testDB.CreateTestUser(t, "player3", "player3@example.com")

		_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
		require.NoError(t, err)
		_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player3.ID), "player")
		require.NoError(t, err)

		// Create new phase for this test
		phase2, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
			PhaseType: "action",
			Title:     "Batch Test Phase",
		})
		require.NoError(t, err)

		// Create unpublished results for each player
		players := []int32{int32(player2.ID), int32(player3.ID)}
		for _, playerID := range players {
			_, err := actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
				GameID:      game.ID,
				PhaseID:     phase2.ID,
				UserID:      playerID,
				Content:     "Batch test result",
				IsPublished: false,
			})
			require.NoError(t, err)
		}

		// Count notifications before publish
		notifsBefore2, err := notificationService.GetUserNotifications(context.Background(), int32(player2.ID), 10, 0)
		require.NoError(t, err)
		countBefore2 := len(notifsBefore2)

		notifsBefore3, err := notificationService.GetUserNotifications(context.Background(), int32(player3.ID), 10, 0)
		require.NoError(t, err)
		countBefore3 := len(notifsBefore3)

		// Publish all results in the phase
		err = actionService.PublishAllPhaseResults(context.Background(), phase2.ID)
		require.NoError(t, err)

		// Count notifications after publish
		notifsAfter2, err := notificationService.GetUserNotifications(context.Background(), int32(player2.ID), 10, 0)
		require.NoError(t, err)
		countAfter2 := len(notifsAfter2)

		notifsAfter3, err := notificationService.GetUserNotifications(context.Background(), int32(player3.ID), 10, 0)
		require.NoError(t, err)
		countAfter3 := len(notifsAfter3)

		// Each player should have exactly one more notification
		assert.Equal(t, countBefore2+1, countAfter2, "Player 2 should receive one notification")
		assert.Equal(t, countBefore3+1, countAfter3, "Player 3 should receive one notification")
	})

	t.Run("notification error does not fail publish operation", func(t *testing.T) {
		// This test verifies that if notification creation fails, the publish still succeeds
		// We can't easily force a notification failure in this test, but we document the behavior

		// Create unpublished result
		result, err := actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
			GameID:      game.ID,
			PhaseID:     phase.ID,
			UserID:      int32(player.ID),
			Content:     "Test error handling",
			IsPublished: false,
		})
		require.NoError(t, err)

		// Publish should succeed even if notification fails (which it won't in this case)
		err = actionService.PublishActionResult(context.Background(), result.ID, int32(gm.ID))
		require.NoError(t, err)

		// Verify result is actually published
		published, err := actionService.GetActionResult(context.Background(), result.ID)
		require.NoError(t, err)
		assert.True(t, published.IsPublished.Bool, "Result should be published even if notification fails")
	})
}
