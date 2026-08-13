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
