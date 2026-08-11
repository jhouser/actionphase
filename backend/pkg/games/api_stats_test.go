package games

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"actionphase/pkg/core"
	db "actionphase/pkg/db/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// completeGame drives a game through the state machine to `completed`, which is
// the only state in which stats are served.
func completeGame(t *testing.T, gameService *db.GameService, gameID int32) {
	t.Helper()
	ctx := context.Background()
	for _, state := range []string{
		core.GameStateRecruitment,
		core.GameStateCharacterCreation,
		core.GameStateInProgress,
		core.GameStateCompleted,
	} {
		_, err := gameService.UpdateGameState(ctx, gameID, state)
		require.NoError(t, err, "failed transitioning game to %s", state)
	}
}

// TestGameAPI_Stats covers GET /api/v1/games/{gameID}/stats: the completed-only
// gate, public-archive readability, and the shape of the payload.
func TestGameAPI_Stats(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "private_messages", "conversations", "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "stats_api_gm", "stats_api_gm@example.com")
	outsider := testDB.CreateTestUser(t, "stats_api_outsider", "stats_api_outsider@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	outsiderToken, err := core.CreateTestJWTTokenForUser(app, outsider)
	require.NoError(t, err)

	t.Run("returns 409 while the game is not completed", func(t *testing.T) {
		game := testDB.CreateTestGame(t, int32(gm.ID), "In Progress Stats Game")

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/stats", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusConflict, rec.Code,
			"stats must not be served for a game that has not completed, even to its GM")
	})

	t.Run("returns 401 without authentication", func(t *testing.T) {
		game := testDB.CreateTestGame(t, int32(gm.ID), "Unauthed Stats Game")
		completeGame(t, gameService, game.ID)

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/stats", game.ID), nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("serves real aggregates for a completed game", func(t *testing.T) {
		game := testDB.CreateTestGame(t, int32(gm.ID), "Completed Stats Game")

		// One GM topic post plus two player comments of length 10 and 20.
		var charID int32
		require.NoError(t, testDB.Pool.QueryRow(context.Background(), `
			INSERT INTO characters (game_id, user_id, name, character_type, status, is_active)
			VALUES ($1, $2, 'StatsAPIChar', 'player_character', 'approved', true) RETURNING id
		`, game.ID, gm.ID).Scan(&charID))

		var postID int32
		require.NoError(t, testDB.Pool.QueryRow(context.Background(), `
			INSERT INTO messages (game_id, author_id, character_id, content, message_type, visibility)
			VALUES ($1, $2, $3, 'GM topic', 'post', 'game') RETURNING id
		`, game.ID, gm.ID, charID).Scan(&postID))

		for _, content := range []string{"aaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbb"} {
			_, err := testDB.Pool.Exec(context.Background(), `
				INSERT INTO messages (game_id, author_id, character_id, content, message_type, visibility, parent_id)
				VALUES ($1, $2, $3, $4, 'comment', 'game', $5)
			`, game.ID, gm.ID, charID, content, postID)
			require.NoError(t, err)
		}

		completeGame(t, gameService, game.ID)

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/stats", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

		var stats core.GameStats
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &stats))

		assert.Equal(t, int64(2), stats.Comments.TotalComments,
			"the GM topic post must not be counted as a comment")
		assert.Equal(t, int64(15), stats.Comments.AvgCommentLength)
		assert.Nil(t, stats.LongestConversation,
			"a game with no private conversations must omit longest_conversation")

		// CreateTestGame also provisions a "Gamemaster" character, so match on
		// name rather than asserting the roster size.
		var authored *core.CharacterWriting
		for i := range stats.Characters {
			if stats.Characters[i].CharacterName == "StatsAPIChar" {
				authored = &stats.Characters[i]
			}
		}
		require.NotNil(t, authored, "the commenting character must appear in the stats")
		assert.Equal(t, int64(2), authored.CommentCount)
		assert.Equal(t, int64(15), authored.AvgCommentLength)

		t.Run("any authenticated user may read a completed game's stats", func(t *testing.T) {
			// Completed games are a public archive (CanUserViewGame), so a user
			// with no relationship to the game still gets the stats.
			req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/stats", game.ID), nil)
			req.Header.Set("Authorization", "Bearer "+outsiderToken)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code,
				"completed games are publicly readable, so stats must be too")
		})
	})
}
