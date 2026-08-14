package phases

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	dbsvc "actionphase/pkg/db/services"
	actionsvc "actionphase/pkg/db/services/actions"
	phasesvc "actionphase/pkg/db/services/phases"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stagedChainBody builds a POST /results/staged payload for the given delays.
// delays[0] is the head and must be 0.
func stagedChainBody(userID int, delays ...int32) map[string]interface{} {
	parts := make([]map[string]interface{}, 0, len(delays))
	for i, delay := range delays {
		parts = append(parts, map[string]interface{}{
			"content":       fmt.Sprintf("Part %d content.", i+1),
			"delay_minutes": delay,
		})
	}
	return map[string]interface{}{
		"user_id": userID,
		"parts":   parts,
	}
}

// createDraftResult posts an ordinary single unstaged draft — the state a GM is
// in when they have written the opening beat and nothing else. Appending to one
// of these is what converts it into a staged chain.
func createDraftResult(t *testing.T, router http.Handler, gameID int32, token string, userID int, content string) *httptest.ResponseRecorder {
	t.Helper()
	bodyJSON, err := json.Marshal(map[string]interface{}{
		"user_id": userID,
		"content": content,
	})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/results", gameID), bytes.NewBuffer(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func postStagedChain(t *testing.T, router http.Handler, gameID int32, token string, body map[string]interface{}) *httptest.ResponseRecorder {
	t.Helper()
	bodyJSON, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/results/staged", gameID), bytes.NewBuffer(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// TestPhaseAPI_CreateStagedResultChain tests POST /api/v1/games/{gameId}/results/staged
func TestPhaseAPI_CreateStagedResultChain(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_results", "action_submissions", "phases", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	_, player, gmToken, playerToken, game, _ := setupResultsTestState(t, testDB, app)

	t.Run("GM creates a chain and gets every part back in order", func(t *testing.T) {
		rec := postStagedChain(t, router, game.ID, gmToken, stagedChainBody(player.ID, 0, 15, 30))

		require.Equal(t, http.StatusCreated, rec.Code)

		var response []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		require.Len(t, response, 3)

		for i, part := range response {
			assert.Equal(t, fmt.Sprintf("Part %d content.", i+1), part["content"])
			assert.Equal(t, float64(i+1), part["part_number"], "parts should be numbered in chain order")
			assert.Equal(t, float64(3), part["part_count"])
			assert.Equal(t, float64(player.ID), part["user_id"], "every part shares the chain's recipient")
		}
	})

	t.Run("an unpublished chain releases nothing", func(t *testing.T) {
		rec := postStagedChain(t, router, game.ID, gmToken, stagedChainBody(player.ID, 0, 15))
		require.Equal(t, http.StatusCreated, rec.Code)

		var response []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		require.Len(t, response, 2)

		for i, part := range response {
			assert.Equal(t, false, part["is_published"])
			assert.Nil(t, part["released_at"], "part %d should not be released before publishing", i+1)
		}
	})

	t.Run("publishing a chain releases only the head", func(t *testing.T) {
		body := stagedChainBody(player.ID, 0, 15)
		body["is_published"] = true

		rec := postStagedChain(t, router, game.ID, gmToken, body)
		require.Equal(t, http.StatusCreated, rec.Code)

		var response []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		require.Len(t, response, 2)

		assert.NotNil(t, response[0]["released_at"], "the head should be released on publish")
		assert.Nil(t, response[1]["released_at"], "part 2 must wait for the release worker — this NULL is the feature")
	})

	t.Run("non-GM player cannot create a staged chain", func(t *testing.T) {
		rec := postStagedChain(t, router, game.ID, playerToken, stagedChainBody(player.ID, 0, 15))
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("chain-shape violations are 400, not 500", func(t *testing.T) {
		cases := []struct {
			name   string
			delays []int32
		}{
			{"a single part is not a chain", []int32{0}},
			{"delay below the minimum", []int32{0, 0}},
			{"delay beyond 24 hours", []int32{0, 1441}},
			{"head carrying a delay", []int32{5, 15}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				rec := postStagedChain(t, router, game.ID, gmToken, stagedChainBody(player.ID, tc.delays...))
				assert.Equal(t, http.StatusBadRequest, rec.Code,
					"a malformed chain is the caller's mistake; body: %s", rec.Body.String())
			})
		}
	})

	t.Run("a chain longer than the maximum is rejected", func(t *testing.T) {
		delays := make([]int32, core.MaxStagedChainLength+1)
		for i := 1; i < len(delays); i++ {
			delays[i] = 5
		}

		rec := postStagedChain(t, router, game.ID, gmToken, stagedChainBody(player.ID, delays...))
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestPhaseAPI_CancelPendingStagedPart tests DELETE /api/v1/games/{gameId}/results/{resultId}/pending
func TestPhaseAPI_CancelPendingStagedPart(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_results", "action_submissions", "phases", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	_, player, gmToken, playerToken, game, _ := setupResultsTestState(t, testDB, app)

	// createChain posts a published chain and returns the created part IDs.
	createChain := func(t *testing.T, delays ...int32) []int32 {
		t.Helper()
		body := stagedChainBody(player.ID, delays...)
		body["is_published"] = true

		rec := postStagedChain(t, router, game.ID, gmToken, body)
		require.Equal(t, http.StatusCreated, rec.Code)

		var response []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))

		ids := make([]int32, 0, len(response))
		for _, part := range response {
			ids = append(ids, int32(part["id"].(float64)))
		}
		return ids
	}

	cancelPart := func(t *testing.T, resultID int32, token string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest("DELETE",
			fmt.Sprintf("/api/v1/games/%d/results/%d/pending", game.ID, resultID), nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	t.Run("GM cancels a pending part", func(t *testing.T) {
		ids := createChain(t, 0, 15)

		rec := cancelPart(t, ids[1], gmToken)
		require.Equal(t, http.StatusNoContent, rec.Code)

		// The row is gone, not merely hidden — a cancelled part must never
		// become due later.
		_, err := models.New(testDB.Pool).GetActionResult(context.Background(), ids[1])
		assert.Error(t, err, "cancelled part should no longer exist")
	})

	t.Run("cancelling an already-released part is 400", func(t *testing.T) {
		ids := createChain(t, 0, 15)

		// The head is released the moment the chain is published.
		rec := cancelPart(t, ids[0], gmToken)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("non-GM player cannot cancel a pending part", func(t *testing.T) {
		ids := createChain(t, 0, 15)

		rec := cancelPart(t, ids[1], playerToken)
		assert.Equal(t, http.StatusForbidden, rec.Code)

		// And the part survives the attempt.
		_, err := models.New(testDB.Pool).GetActionResult(context.Background(), ids[1])
		assert.NoError(t, err, "a forbidden cancel must not delete the part")
	})

	t.Run("invalid result ID is rejected", func(t *testing.T) {
		req := httptest.NewRequest("DELETE",
			fmt.Sprintf("/api/v1/games/%d/results/not-a-number/pending", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestPhaseAPI_AppendStagedPart tests POST /api/v1/games/{gameId}/results/{resultId}/parts
func TestPhaseAPI_AppendStagedPart(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_results", "action_submissions", "phases", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	_, player, gmToken, playerToken, game, _ := setupResultsTestState(t, testDB, app)

	// createDraftChain posts an unpublished chain and returns its part IDs.
	createDraftChain := func(t *testing.T, delays ...int32) []int32 {
		t.Helper()
		rec := postStagedChain(t, router, game.ID, gmToken, stagedChainBody(player.ID, delays...))
		require.Equal(t, http.StatusCreated, rec.Code)

		var response []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))

		ids := make([]int32, 0, len(response))
		for _, part := range response {
			ids = append(ids, int32(part["id"].(float64)))
		}
		return ids
	}

	appendPart := func(t *testing.T, resultID int32, token string, body map[string]interface{}) *httptest.ResponseRecorder {
		t.Helper()
		bodyJSON, err := json.Marshal(body)
		require.NoError(t, err)

		req := httptest.NewRequest("POST",
			fmt.Sprintf("/api/v1/games/%d/results/%d/parts", game.ID, resultID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	t.Run("GM appends to a draft chain and gets the new part back", func(t *testing.T) {
		ids := createDraftChain(t, 0, 15)

		rec := appendPart(t, ids[0], gmToken, map[string]interface{}{
			"content":       "The payoff.",
			"delay_minutes": 30,
		})
		require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

		var part map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &part))

		assert.Equal(t, "The payoff.", part["content"])
		assert.Equal(t, float64(30), part["reveal_delay_minutes"],
			"the GM's editor reads this back to populate its delay selector")
		assert.Equal(t, false, part["is_published"])
		assert.Nil(t, part["released_at"])

		// Anchored on the head, but it must land after the current tail.
		assert.Equal(t, float64(ids[1]), part["parent_result_id"],
			"a part appended via the head still follows the tail")
	})

	t.Run("appending to an unstaged draft turns it into a chain", func(t *testing.T) {
		// The headline case: a GM saved a plain draft and comes back later.
		rec := createDraftResult(t, router, game.ID, gmToken, player.ID, "The sword swings...")
		require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

		var draft map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &draft))
		draftID := int32(draft["id"].(float64))

		rec = appendPart(t, draftID, gmToken, map[string]interface{}{
			"content":       "...and misses!",
			"delay_minutes": 15,
		})
		require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

		var part map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &part))
		assert.Equal(t, float64(draftID), part["parent_result_id"])
	})

	t.Run("appending to a published chain is 409, not 400", func(t *testing.T) {
		body := stagedChainBody(player.ID, 0, 15)
		body["is_published"] = true
		rec := postStagedChain(t, router, game.ID, gmToken, body)
		require.Equal(t, http.StatusCreated, rec.Code)

		var response []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		headID := int32(response[0]["id"].(float64))

		rec = appendPart(t, headID, gmToken, map[string]interface{}{
			"content":       "Too late.",
			"delay_minutes": 5,
		})
		// The request is well formed; the chain has simply moved on. Rewriting
		// the body cannot help, which is what distinguishes 409 from 400.
		assert.Equal(t, http.StatusConflict, rec.Code, "body: %s", rec.Body.String())
	})

	t.Run("non-GM player cannot append a part", func(t *testing.T) {
		ids := createDraftChain(t, 0, 15)

		rec := appendPart(t, ids[0], playerToken, map[string]interface{}{
			"content":       "Not mine to write.",
			"delay_minutes": 15,
		})
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("a delay out of range is 400", func(t *testing.T) {
		ids := createDraftChain(t, 0, 15)

		for _, delay := range []int32{0, core.MaxStagedDelayMinutes + 1} {
			rec := appendPart(t, ids[0], gmToken, map[string]interface{}{
				"content":       "Bad timing.",
				"delay_minutes": delay,
			})
			assert.Equal(t, http.StatusBadRequest, rec.Code, "delay %d must be rejected", delay)
		}
	})

	t.Run("appending to a nonexistent result is 404", func(t *testing.T) {
		rec := appendPart(t, 999999, gmToken, map[string]interface{}{
			"content":       "Nowhere to go.",
			"delay_minutes": 15,
		})
		assert.Equal(t, http.StatusNotFound, rec.Code, "body: %s", rec.Body.String())
	})
}

// TestPhaseAPI_UpdateStagedPartDelay tests PUT /api/v1/games/{gameId}/results/{resultId}/delay
func TestPhaseAPI_UpdateStagedPartDelay(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_results", "action_submissions", "phases", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	_, player, gmToken, playerToken, game, _ := setupResultsTestState(t, testDB, app)

	createChain := func(t *testing.T, published bool, delays ...int32) []int32 {
		t.Helper()
		body := stagedChainBody(player.ID, delays...)
		body["is_published"] = published

		rec := postStagedChain(t, router, game.ID, gmToken, body)
		require.Equal(t, http.StatusCreated, rec.Code)

		var response []map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))

		ids := make([]int32, 0, len(response))
		for _, part := range response {
			ids = append(ids, int32(part["id"].(float64)))
		}
		return ids
	}

	setDelay := func(t *testing.T, resultID int32, token string, delay int32) *httptest.ResponseRecorder {
		t.Helper()
		bodyJSON, err := json.Marshal(map[string]interface{}{"delay_minutes": delay})
		require.NoError(t, err)

		req := httptest.NewRequest("PUT",
			fmt.Sprintf("/api/v1/games/%d/results/%d/delay", game.ID, resultID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	t.Run("GM retimes a draft part", func(t *testing.T) {
		ids := createChain(t, false, 0, 15)

		rec := setDelay(t, ids[1], gmToken, 45)
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

		var part map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &part))
		assert.Equal(t, float64(45), part["reveal_delay_minutes"])

		// And it actually persisted, not merely echoed.
		stored, err := models.New(testDB.Pool).GetActionResult(context.Background(), ids[1])
		require.NoError(t, err)
		assert.Equal(t, int32(45), stored.RevealDelayMinutes.Int32)
	})

	t.Run("GM retimes a published pending part", func(t *testing.T) {
		// The live-scene case: the countdown is on the player's screen and the
		// GM needs more time.
		ids := createChain(t, true, 0, 15)

		rec := setDelay(t, ids[1], gmToken, 60)
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

		stored, err := models.New(testDB.Pool).GetActionResult(context.Background(), ids[1])
		require.NoError(t, err)
		assert.Equal(t, int32(60), stored.RevealDelayMinutes.Int32)
		assert.False(t, stored.ReleasedAt.Valid, "retiming must not release the part")
	})

	t.Run("retiming a released part is 409", func(t *testing.T) {
		ids := createChain(t, true, 0, 15)

		// The head released on publish, so it stands in for any released part.
		rec := setDelay(t, ids[0], gmToken, 60)
		assert.Equal(t, http.StatusConflict, rec.Code, "body: %s", rec.Body.String())
	})

	t.Run("retiming an unreleased chain head is 409", func(t *testing.T) {
		ids := createChain(t, false, 0, 15)

		// A head has no parent to measure a delay from.
		rec := setDelay(t, ids[0], gmToken, 60)
		assert.Equal(t, http.StatusConflict, rec.Code, "body: %s", rec.Body.String())
	})

	t.Run("non-GM player cannot retime a part", func(t *testing.T) {
		ids := createChain(t, false, 0, 15)

		rec := setDelay(t, ids[1], playerToken, 60)
		assert.Equal(t, http.StatusForbidden, rec.Code)

		// And the schedule survives the attempt.
		stored, err := models.New(testDB.Pool).GetActionResult(context.Background(), ids[1])
		require.NoError(t, err)
		assert.Equal(t, int32(15), stored.RevealDelayMinutes.Int32)
	})

	t.Run("a delay out of range is 400", func(t *testing.T) {
		ids := createChain(t, false, 0, 15)

		for _, delay := range []int32{0, core.MaxStagedDelayMinutes + 1} {
			rec := setDelay(t, ids[1], gmToken, delay)
			assert.Equal(t, http.StatusBadRequest, rec.Code, "delay %d must be rejected", delay)
		}
	})

	t.Run("retiming a nonexistent result is 404", func(t *testing.T) {
		rec := setDelay(t, 999999, gmToken, 60)
		assert.Equal(t, http.StatusNotFound, rec.Code, "body: %s", rec.Body.String())
	})
}

// TestPhaseAPI_StagedPart_CrossGameMismatch verifies that the staged chain edit
// endpoints refuse a result belonging to a different game.
//
// All three take {resultId} in the URL but authorize against {gameId}, so
// without a check binding the two, any GM can pass their own game ID — passing
// the permission check — while naming a result in someone else's game. The
// service methods take only a result ID and cannot catch this.
//
// The same guard already exists for draft character updates
// (validateGMAccessAndResult, api_draft_updates.go) with its own cross-game
// test; these endpoints are the ones that were missed.
func TestPhaseAPI_StagedPart_CrossGameMismatch(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "notifications", "action_results", "action_submissions", "phases", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	gameService := &dbsvc.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	phaseService := &phasesvc.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	actionService := &actionsvc.ActionSubmissionService{
		DB:                  testDB.Pool,
		Logger:              app.ObsLogger,
		NotificationService: &dbsvc.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger},
	}

	// Victim: gm1's game, holding a draft staged chain.
	gm1 := testDB.CreateTestUser(t, "xg_gm1", "xg_gm1@example.com")
	player1 := testDB.CreateTestUser(t, "xg_player1", "xg_player1@example.com")
	game1 := testDB.CreateTestGame(t, int32(gm1.ID), "Victim Game")
	_, err := gameService.AddGameParticipant(context.Background(), game1.ID, int32(player1.ID), "player")
	require.NoError(t, err)

	phase1, err := phaseService.TransitionToNextPhase(context.Background(), game1.ID, int32(gm1.ID), core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Victim Action Phase",
	})
	require.NoError(t, err)

	chain, err := actionService.CreateStagedResultChain(context.Background(), core.CreateStagedResultChainRequest{
		GameID:   game1.ID,
		PhaseID:  phase1.ID,
		UserID:   int32(player1.ID),
		GMUserID: int32(gm1.ID),
		Parts: []core.StagedResultPart{
			{Content: "The victim game's opening beat."},
			{Content: "The victim game's payoff.", DelayMinutes: 10},
		},
		IsPublished: false,
	})
	require.NoError(t, err)
	require.Len(t, chain, 2)

	// Attacker: gm2 is a legitimate GM, but of their own unrelated game.
	gm2 := testDB.CreateTestUser(t, "xg_gm2", "xg_gm2@example.com")
	gm2Token, err := core.CreateTestJWTTokenForUser(app, gm2)
	require.NoError(t, err)
	game2 := testDB.CreateTestGame(t, int32(gm2.ID), "Attacker Game")
	_, err = phaseService.TransitionToNextPhase(context.Background(), game2.ID, int32(gm2.ID), core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Attacker Action Phase",
	})
	require.NoError(t, err)

	authedRequest := func(t *testing.T, method, path string, body map[string]interface{}) *httptest.ResponseRecorder {
		t.Helper()
		var reader *bytes.Buffer
		if body != nil {
			bodyJSON, err := json.Marshal(body)
			require.NoError(t, err)
			reader = bytes.NewBuffer(bodyJSON)
		} else {
			reader = bytes.NewBuffer(nil)
		}
		req := httptest.NewRequest(method, path, reader)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gm2Token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	t.Run("cannot append a part to another game's chain", func(t *testing.T) {
		rec := authedRequest(t, "POST",
			fmt.Sprintf("/api/v1/games/%d/results/%d/parts", game2.ID, chain[1].ID),
			map[string]interface{}{"content": "Injected by another GM.", "delay_minutes": 5})

		assert.Equal(t, http.StatusNotFound, rec.Code,
			"a result in another game must not be reachable via one's own game ID; body: %s", rec.Body.String())

		var partCount int
		require.NoError(t, testDB.Pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM action_results WHERE game_id = $1`, game1.ID).Scan(&partCount))
		assert.Equal(t, 2, partCount, "the victim game's chain must be unchanged")
	})

	t.Run("cannot retime another game's part", func(t *testing.T) {
		rec := authedRequest(t, "PUT",
			fmt.Sprintf("/api/v1/games/%d/results/%d/delay", game2.ID, chain[1].ID),
			map[string]interface{}{"delay_minutes": 1440})

		assert.Equal(t, http.StatusNotFound, rec.Code,
			"retiming across games must be refused; body: %s", rec.Body.String())

		var delay int32
		require.NoError(t, testDB.Pool.QueryRow(context.Background(),
			`SELECT reveal_delay_minutes FROM action_results WHERE id = $1`, chain[1].ID).Scan(&delay))
		assert.Equal(t, int32(10), delay, "the victim part's delay must be unchanged")
	})

	t.Run("cannot cancel another game's pending part", func(t *testing.T) {
		rec := authedRequest(t, "DELETE",
			fmt.Sprintf("/api/v1/games/%d/results/%d/pending", game2.ID, chain[1].ID), nil)

		assert.Equal(t, http.StatusNotFound, rec.Code,
			"cancelling across games must be refused; body: %s", rec.Body.String())

		var stillThere bool
		require.NoError(t, testDB.Pool.QueryRow(context.Background(),
			`SELECT EXISTS(SELECT 1 FROM action_results WHERE id = $1)`, chain[1].ID).Scan(&stillThere))
		assert.True(t, stillThere, "the victim part must not be deleted by another game's GM")
	})
}
