package phases

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"actionphase/pkg/core"
	dbsvc "actionphase/pkg/db/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lockedContent is the text of a part that must never reach its recipient
// before release. Distinctive on purpose: the leak test greps the raw response
// bytes for it, so it has to be a string that cannot occur incidentally.
const lockedContent = "SPOILER-XYZZY-the-blow-lands-and-you-die"

// getResults performs a GET against a results endpoint and returns the recorder
// alongside the decoded body, so tests can assert on both the parsed shape and
// the bytes actually sent.
func getResults(t *testing.T, router http.Handler, path, token string) (*httptest.ResponseRecorder, []map[string]interface{}) {
	t.Helper()

	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var decoded []map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &decoded))
	return rec, decoded
}

// TestPhaseAPI_StagedReadShape covers what the read paths report about a chain:
// part numbering, unlock times, and — above all — that a pending part's content
// never reaches the player it is being withheld from.
func TestPhaseAPI_StagedReadShape(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_results", "action_submissions", "game_phases", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	_, player, gmToken, playerToken, game, _ := setupResultsTestState(t, testDB, app)

	// A published three-part chain: part 1 released on publish, parts 2 and 3
	// pending. Part 3 carries the locked string.
	body := map[string]interface{}{
		"user_id":      player.ID,
		"is_published": true,
		"parts": []map[string]interface{}{
			{"content": "The sword whooshes toward your head...", "delay_minutes": 0},
			{"content": "...time seems to slow...", "delay_minutes": 15},
			{"content": lockedContent, "delay_minutes": 30},
		},
	}

	rec := postStagedChain(t, router, game.ID, gmToken, body)
	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

	minePath := fmt.Sprintf("/api/v1/games/%d/results/mine", game.ID)
	allPath := fmt.Sprintf("/api/v1/games/%d/results", game.ID)

	t.Run("player receives a placeholder row for each pending part", func(t *testing.T) {
		_, results := getResults(t, router, minePath, playerToken)

		// All three rows are present: the countdown placeholder needs the row to
		// exist. Withholding the row instead of the text would leave the client
		// with nothing to render and no unlock time to count toward.
		require.Len(t, results, 3, "pending parts are returned as placeholders, not hidden")

		for i, part := range results {
			assert.Equal(t, float64(i+1), part["part_number"])
			assert.Equal(t, float64(3), part["part_count"])
		}
	})

	t.Run("pending content is blanked for the player", func(t *testing.T) {
		_, results := getResults(t, router, minePath, playerToken)

		assert.NotEmpty(t, results[0]["content"], "the released head keeps its content")
		assert.Equal(t, "", results[1]["content"], "a pending part must arrive with no text")
		assert.Equal(t, "", results[2]["content"], "a pending part must arrive with no text")
	})

	// The assertion the entire feature rests on. Checking the decoded field is
	// not enough — this greps the bytes on the wire, so it would catch the
	// content leaking through any other field, a nested object, or a serializer
	// change that reintroduces the raw column.
	t.Run("locked text appears nowhere in the raw player response", func(t *testing.T) {
		rec, _ := getResults(t, router, minePath, playerToken)

		assert.NotContains(t, rec.Body.String(), lockedContent,
			"a player with devtools would defeat the whole feature")
		assert.NotContains(t, rec.Body.String(), "time seems to slow",
			"part 2 is pending too and must be withheld just the same")
	})

	t.Run("released_at marks which parts are readable", func(t *testing.T) {
		_, results := getResults(t, router, minePath, playerToken)

		assert.NotNil(t, results[0]["released_at"], "the head released on publish")
		assert.Nil(t, results[1]["released_at"], "a pending part has no release time")
		assert.Nil(t, results[2]["released_at"])
	})

	t.Run("only the next part due carries an unlock time", func(t *testing.T) {
		_, results := getResults(t, router, minePath, playerToken)

		assert.Nil(t, results[0]["unlocks_at"], "an already-released part is not counting down")
		assert.NotNil(t, results[1]["unlocks_at"],
			"part 2's parent has released, so its unlock time is known and drives the countdown")
		assert.Nil(t, results[2]["unlocks_at"],
			"part 3 cannot have a knowable unlock time until part 2 releases")
	})

	t.Run("GM reads pending content in full", func(t *testing.T) {
		rec, results := getResults(t, router, allPath, gmToken)

		require.Len(t, results, 3)
		assert.Equal(t, lockedContent, results[2]["content"],
			"the GM wrote this part and composes against it")
		assert.Contains(t, rec.Body.String(), lockedContent)

		// Same chain description as the player gets — the roles differ on
		// content, not on numbering.
		assert.Equal(t, float64(3), results[2]["part_number"])
		assert.Equal(t, float64(3), results[2]["part_count"])
	})
}

// TestPhaseAPI_StagedReadShape_Audience verifies the audience reads a chain
// exactly as the GM does. Stated directly by the product owner: in History the
// audience sees action results as though they were a GM, and staged results are
// no different.
func TestPhaseAPI_StagedReadShape_Audience(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_results", "action_submissions", "game_phases", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	_, player, gmToken, _, game, _ := setupResultsTestState(t, testDB, app)

	audience := testDB.CreateTestUser(t, "audience", "audience@example.com")
	audienceToken, err := core.CreateTestJWTTokenForUser(app, audience)
	require.NoError(t, err)

	gameService := &dbsvc.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(audience.ID), "audience")
	require.NoError(t, err)

	body := map[string]interface{}{
		"user_id":      player.ID,
		"is_published": true,
		"parts": []map[string]interface{}{
			{"content": "The sword whooshes toward your head...", "delay_minutes": 0},
			{"content": lockedContent, "delay_minutes": 15},
		},
	}
	rec := postStagedChain(t, router, game.ID, gmToken, body)
	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

	allPath := fmt.Sprintf("/api/v1/games/%d/results", game.ID)

	audienceRec, audienceResults := getResults(t, router, allPath, audienceToken)
	gmRec, gmResults := getResults(t, router, allPath, gmToken)

	require.Len(t, audienceResults, 2)
	assert.Equal(t, lockedContent, audienceResults[1]["content"],
		"the audience reads unreleased parts, same as the GM")

	// Strongest form of "same view": the two responses are byte-identical, so no
	// viewer-dependent branching can have crept into the serializer.
	assert.Equal(t, gmRec.Body.String(), audienceRec.Body.String(),
		"audience and GM must receive the identical payload")
	assert.Equal(t, len(gmResults), len(audienceResults))
}

// TestPhaseAPI_OrdinaryResultsUnchanged pins the compatibility promise: a
// single-part result must serialize exactly as it did before staged reveals
// existed, so no existing client has to learn about staging.
func TestPhaseAPI_OrdinaryResultsUnchanged(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_results", "action_submissions", "game_phases", "characters", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupFullPhaseAPITestRouter(app, testDB)

	_, player, gmToken, playerToken, game, _ := setupResultsTestState(t, testDB, app)

	// No phase_id: the endpoint derives the phase from the game's active one
	// and has never read a phase_id from the body. Sending it used to be
	// silently ignored; huma rejects unknown body properties, and the frontend
	// does not send it either.
	resultBody := map[string]interface{}{
		"user_id":      player.ID,
		"content":      "An ordinary, unstaged result.",
		"is_published": true,
	}
	bodyJSON, err := json.Marshal(resultBody)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/results", game.ID), strings.NewReader(string(bodyJSON)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

	_, results := getResults(t, router, fmt.Sprintf("/api/v1/games/%d/results/mine", game.ID), playerToken)
	require.Len(t, results, 1)

	assert.Equal(t, "An ordinary, unstaged result.", results[0]["content"])

	// omitempty means these keys are absent entirely, not present-and-null. A
	// client that has never heard of staging sees the object it always saw.
	for _, field := range []string{"part_number", "part_count", "unlocks_at"} {
		_, present := results[0][field]
		assert.False(t, present, "%s must be absent for an unstaged result", field)
	}

	// released_at is the exception: it is set for every published result,
	// staged or not, and is what the player path uses to tell locked from
	// readable.
	assert.NotNil(t, results[0]["released_at"])
}
