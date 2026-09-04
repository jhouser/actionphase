package polls

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"actionphase/pkg/core"
	dbmodels "actionphase/pkg/db/models"
	dbservices "actionphase/pkg/db/services"
	"actionphase/pkg/games"
	"actionphase/pkg/humaconfig"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// visibilityTables lists tables to clean between tests in this file.
var visibilityTables = []string{
	"poll_votes", "poll_options", "common_room_polls",
	"game_participants", "characters", "games", "sessions", "users",
}

func setupVisibilityRouter(app *core.App, testDB *core.TestDatabase) *chi.Mux {
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &dbservices.UserService{DB: testDB.Pool, Logger: app.ObsLogger}

	router := chi.NewRouter()
	router.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(jwtauth.Verifier(tokenAuth))
			r.Use(jwtauth.Authenticator(tokenAuth))
			r.Use(core.RequireAuthenticationMiddleware(userService))

			handler := &Handler{
				App:                 app,
				UserService:         &dbservices.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
				GameService:         &dbservices.GameService{DB: testDB.Pool, Logger: app.ObsLogger},
				PollService:         &dbservices.PollService{DB: testDB.Pool, Logger: app.ObsLogger},
				CharacterService:    &dbservices.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger},
				NotificationService: dbservices.NewNotificationService(testDB.Pool, app.ObsLogger),
			}
			// CreatePoll reads the game from context, so it must sit behind
			// GameMiddleware under a {gameID} route exactly as it does in
			// pkg/http/root.go.
			gameHandler := &games.Handler{
				App:         app,
				UserService: &dbservices.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
				GameService: &dbservices.GameService{DB: testDB.Pool, Logger: app.ObsLogger},
			}
			r.Route("/games/{gameID}", func(r chi.Router) {
				r.Use(gameHandler.GameMiddleware())
				RegisterHumaGamePolls(humaconfig.New(r, "ActionPhase API", "1.0.0"), handler)
			})
			r.Route("/polls", func(r chi.Router) {
				RegisterHumaPolls(humaconfig.New(r, "ActionPhase API", "1.0.0"), handler)
			})
		})
	})

	return router
}

// visibilityFixture is a game with a GM, a player holding an approved character,
// an audience member, and one poll configured by the caller.
type visibilityFixture struct {
	gameID        int32
	pollID        int32
	optionAID     int32
	optionBID     int32
	gmToken       string
	playerToken   string
	audienceToken string
	playerUserID  int32
}

func setupVisibilityFixture(
	t *testing.T, testDB *core.TestDatabase, app *core.App, prefix string,
	pollReq core.CreatePollRequest,
) visibilityFixture {
	t.Helper()

	gm := testDB.CreateTestUser(t, prefix+"_gm", prefix+"_gm@example.com")
	player := testDB.CreateTestUser(t, prefix+"_player", prefix+"_player@example.com")
	audience := testDB.CreateTestUser(t, prefix+"_aud", prefix+"_aud@example.com")

	gameRecord := testDB.CreateTestGame(t, int32(gm.ID), prefix+" Visibility Game")

	gameService := &dbservices.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err := gameService.AddGameParticipant(context.Background(), gameRecord.ID, int32(player.ID), "player")
	require.NoError(t, err)

	queries := dbmodels.New(testDB.Pool)
	_, err = queries.CreateAudienceApplication(context.Background(), dbmodels.CreateAudienceApplicationParams{
		GameID: gameRecord.ID,
		UserID: int32(audience.ID),
		Status: pgtype.Text{String: "active", Valid: true},
	})
	require.NoError(t, err)

	factory := core.NewTestDataFactory(testDB, t)
	factory.NewCharacter().ForGame(gameRecord.ID).WithUserID(int32(player.ID)).
		PlayerCharacter().Approved().WithName(prefix + " Hero").Create()

	pollReq.GameID = gameRecord.ID
	pollReq.CreatedByUserID = int32(gm.ID)
	if pollReq.Question == "" {
		pollReq.Question = "Visibility question?"
	}
	if pollReq.Deadline.IsZero() {
		pollReq.Deadline = time.Now().Add(24 * time.Hour)
	}
	if len(pollReq.Options) == 0 {
		pollReq.Options = []core.PollOptionInput{
			{Text: "Option A", DisplayOrder: 1},
			{Text: "Option B", DisplayOrder: 2},
		}
	}

	pollService := &dbservices.PollService{DB: testDB.Pool, Logger: app.ObsLogger}
	created, err := pollService.CreatePollWithOptions(context.Background(), pollReq)
	require.NoError(t, err)

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)
	audienceToken, err := core.CreateTestJWTTokenForUser(app, audience)
	require.NoError(t, err)

	return visibilityFixture{
		gameID:        gameRecord.ID,
		pollID:        created.Poll.ID,
		optionAID:     created.Options[0].ID,
		optionBID:     created.Options[1].ID,
		gmToken:       gmToken,
		playerToken:   playerToken,
		audienceToken: audienceToken,
		playerUserID:  int32(player.ID),
	}
}

func getResultsReq(t *testing.T, router *chi.Mux, pollID int32, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/polls/%d/results", pollID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func voteReq(t *testing.T, router *chi.Mux, pollID int32, optionID int32, token string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(SubmitVoteRequest{SelectedOptionID: &optionID})
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/polls/%d/vote", pollID), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// updatePollReq issues a PUT against a poll. The handler replaces every
// setting, so callers pass a fully-populated request.
func updatePollReq(t *testing.T, router *chi.Mux, pollID int32, token string, body UpdatePollRequest) *httptest.ResponseRecorder {
	t.Helper()
	if body.Question == "" {
		body.Question = "Visibility question?"
	}
	if body.Deadline.IsZero() {
		body.Deadline = time.Now().Add(24 * time.Hour)
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/polls/%d", pollID), bytes.NewBuffer(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// expirePoll pushes a poll's deadline into the past.
func expirePoll(t *testing.T, testDB *core.TestDatabase, pollID int32) {
	t.Helper()
	_, err := testDB.Pool.Exec(context.Background(),
		"UPDATE common_room_polls SET deadline = NOW() - INTERVAL '1 hour' WHERE id = $1", pollID)
	require.NoError(t, err)
}

// ============================================================
// Hidden results
// ============================================================

func TestHiddenResults_PlayerDeniedAfterDeadline(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, visibilityTables...)

	app := core.NewTestApp(testDB.Pool)
	router := setupVisibilityRouter(app, testDB)

	f := setupVisibilityFixture(t, testDB, app, "hidden", core.CreatePollRequest{
		HideResultsFromPlayers: true,
	})

	// Player votes while the poll is open.
	require.Equal(t, http.StatusOK, voteReq(t, router, f.pollID, f.optionAID, f.playerToken).Code)

	// Closing the poll must NOT reveal results to the player.
	expirePoll(t, testDB, f.pollID)

	rec := getResultsReq(t, router, f.pollID, f.playerToken)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"hidden-results poll must stay hidden from players after the deadline")
	assert.NotContains(t, rec.Body.String(), "Option A",
		"denied response must not leak option tallies")
}

func TestHiddenResults_GMAndAudienceStillSeeResults(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, visibilityTables...)

	app := core.NewTestApp(testDB.Pool)
	router := setupVisibilityRouter(app, testDB)

	f := setupVisibilityFixture(t, testDB, app, "hidgm", core.CreatePollRequest{
		HideResultsFromPlayers: true,
	})

	require.Equal(t, http.StatusOK, voteReq(t, router, f.pollID, f.optionAID, f.playerToken).Code)

	for name, token := range map[string]string{"gm": f.gmToken, "audience": f.audienceToken} {
		rec := getResultsReq(t, router, f.pollID, token)
		require.Equal(t, http.StatusOK, rec.Code, "%s should see hidden results: %s", name, rec.Body.String())

		var results PollResultsResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &results))
		assert.Equal(t, int32(1), results.TotalVotes, "%s should see the real vote total", name)

		var votesForA int32
		for _, opt := range results.OptionResults {
			if opt.PollOptionID != nil && *opt.PollOptionID == f.optionAID {
				votesForA = opt.VoteCount
			}
		}
		assert.Equal(t, int32(1), votesForA, "%s should see the vote counted against option A", name)
	}
}

// hide_results_from_players is a play-time protection, not a permanent one.
// Once a game is COMPLETED it becomes a public archive in which every
// authenticated viewer gets audience-level access to the whole game, so a poll
// the GM hid during play opens up along with everything else. Cancelled games
// are NOT public and keep the play-time rule.
func TestHiddenResults_PlayerSeesResultsAfterGameCompleted(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, visibilityTables...)

	app := core.NewTestApp(testDB.Pool)
	router := setupVisibilityRouter(app, testDB)

	f := setupVisibilityFixture(t, testDB, app, "hidcomp", core.CreatePollRequest{
		HideResultsFromPlayers: true,
	})

	require.Equal(t, http.StatusOK, voteReq(t, router, f.pollID, f.optionAID, f.playerToken).Code)
	expirePoll(t, testDB, f.pollID)

	// While the game is live the poll stays hidden — the archive rule must be
	// what opens it, not the deadline.
	require.Equal(t, http.StatusForbidden, getResultsReq(t, router, f.pollID, f.playerToken).Code,
		"hidden poll must stay hidden while the game is still running")

	_, err := testDB.Pool.Exec(context.Background(),
		"UPDATE games SET state = 'completed' WHERE id = $1", f.gameID)
	require.NoError(t, err)

	rec := getResultsReq(t, router, f.pollID, f.playerToken)
	require.Equal(t, http.StatusOK, rec.Code,
		"completing the game must open a hidden-results poll to players: %s", rec.Body.String())

	var results PollResultsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &results))
	assert.Equal(t, int32(1), results.TotalVotes, "player should see the real vote total in the archive")

	var votesForA int32
	for _, opt := range results.OptionResults {
		if opt.PollOptionID != nil && *opt.PollOptionID == f.optionAID {
			votesForA = opt.VoteCount
		}
	}
	assert.Equal(t, int32(1), votesForA, "player should see the vote counted against option A")
}

// A cancelled game is not a public archive, so the play-time protection holds.
func TestHiddenResults_PlayerDeniedAfterGameCancelled(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, visibilityTables...)

	app := core.NewTestApp(testDB.Pool)
	router := setupVisibilityRouter(app, testDB)

	f := setupVisibilityFixture(t, testDB, app, "hidcanc", core.CreatePollRequest{
		HideResultsFromPlayers: true,
	})

	require.Equal(t, http.StatusOK, voteReq(t, router, f.pollID, f.optionAID, f.playerToken).Code)
	expirePoll(t, testDB, f.pollID)

	_, err := testDB.Pool.Exec(context.Background(),
		"UPDATE games SET state = 'cancelled' WHERE id = $1", f.gameID)
	require.NoError(t, err)

	rec := getResultsReq(t, router, f.pollID, f.playerToken)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"cancelling a game must not unhide a hidden-results poll")
}

func TestVisibleResults_PlayerSeesResultsAfterDeadline(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, visibilityTables...)

	app := core.NewTestApp(testDB.Pool)
	router := setupVisibilityRouter(app, testDB)

	// Control case: without the flag, the existing behaviour must be unchanged.
	f := setupVisibilityFixture(t, testDB, app, "shown", core.CreatePollRequest{
		HideResultsFromPlayers: false,
	})

	require.Equal(t, http.StatusOK, voteReq(t, router, f.pollID, f.optionAID, f.playerToken).Code)
	expirePoll(t, testDB, f.pollID)

	rec := getResultsReq(t, router, f.pollID, f.playerToken)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var results PollResultsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &results))
	assert.Equal(t, int32(1), results.TotalVotes)
}

func TestCreatePoll_HiddenResultsWithIndividualVotes_Rejected(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, visibilityTables...)

	app := core.NewTestApp(testDB.Pool)
	router := setupVisibilityRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "excl_gm", "excl_gm@example.com")
	gameRecord := testDB.CreateTestGame(t, int32(gm.ID), "Exclusivity Game")
	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)

	body, _ := json.Marshal(CreatePollRequest{
		Question:               "Mutually exclusive?",
		Deadline:               time.Now().Add(24 * time.Hour),
		ShowIndividualVotes:    true,
		HideResultsFromPlayers: true,
		Options: []PollOptionRequest{
			{Text: "A", DisplayOrder: 1},
			{Text: "B", DisplayOrder: 2},
		},
	})
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/polls", gameRecord.ID), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code,
		"hidden results and individual vote display are mutually exclusive")

	var count int
	require.NoError(t, testDB.Pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM common_room_polls WHERE game_id = $1", gameRecord.ID).Scan(&count))
	assert.Equal(t, 0, count, "rejected poll must not be persisted")
}

// ============================================================
// Running totals
// ============================================================

func TestRunningTotals_Disabled_PlayerDeniedWhilePollOpen(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, visibilityTables...)

	app := core.NewTestApp(testDB.Pool)
	router := setupVisibilityRouter(app, testDB)

	// Control case: without the flag, players still wait for the deadline.
	f := setupVisibilityFixture(t, testDB, app, "rtoff", core.CreatePollRequest{
		ShowRunningTotalsToPlayers: false,
	})

	require.Equal(t, http.StatusOK, voteReq(t, router, f.pollID, f.optionAID, f.playerToken).Code)

	rec := getResultsReq(t, router, f.pollID, f.playerToken)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"without running totals, an open poll's results stay closed to players")
}

func TestRunningTotals_Enabled_PlayerSeesTotalsWhilePollOpen(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, visibilityTables...)

	app := core.NewTestApp(testDB.Pool)
	router := setupVisibilityRouter(app, testDB)

	f := setupVisibilityFixture(t, testDB, app, "rton", core.CreatePollRequest{
		ShowRunningTotalsToPlayers: true,
	})

	require.Equal(t, http.StatusOK, voteReq(t, router, f.pollID, f.optionAID, f.playerToken).Code)

	// Poll is still open — the player should nonetheless see the live tally.
	rec := getResultsReq(t, router, f.pollID, f.playerToken)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var results PollResultsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &results))
	assert.Equal(t, int32(1), results.TotalVotes)

	var votesForA int32
	for _, opt := range results.OptionResults {
		if opt.PollOptionID != nil && *opt.PollOptionID == f.optionAID {
			votesForA = opt.VoteCount
		}
	}
	assert.Equal(t, int32(1), votesForA, "player should see the running tally for option A")
}

func TestRunningTotals_WithoutIndividualVotes_PlayerSeesNoAttribution(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, visibilityTables...)

	app := core.NewTestApp(testDB.Pool)
	router := setupVisibilityRouter(app, testDB)

	f := setupVisibilityFixture(t, testDB, app, "rtnoattr", core.CreatePollRequest{
		ShowRunningTotalsToPlayers: true,
		ShowIndividualVotes:        false,
	})

	require.Equal(t, http.StatusOK, voteReq(t, router, f.pollID, f.optionAID, f.playerToken).Code)

	rec := getResultsReq(t, router, f.pollID, f.playerToken)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var results PollResultsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &results))

	assert.False(t, results.ShowIndividualVotes,
		"running totals alone must not enable vote attribution")
	for _, opt := range results.OptionResults {
		assert.Empty(t, opt.Voters, "running totals alone must not disclose who voted")
	}
	assert.NotContains(t, rec.Body.String(), "Hero",
		"character names must not leak when individual votes are off")
}

func TestRunningTotals_WithIndividualVotes_PlayerSeesWhoVoted(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, visibilityTables...)

	app := core.NewTestApp(testDB.Pool)
	router := setupVisibilityRouter(app, testDB)

	f := setupVisibilityFixture(t, testDB, app, "rtattr", core.CreatePollRequest{
		ShowRunningTotalsToPlayers: true,
		ShowIndividualVotes:        true,
	})

	require.Equal(t, http.StatusOK, voteReq(t, router, f.pollID, f.optionAID, f.playerToken).Code)

	rec := getResultsReq(t, router, f.pollID, f.playerToken)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var results PollResultsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &results))

	var voterFound *VoterInfo
	for i := range results.OptionResults {
		opt := &results.OptionResults[i]
		if opt.PollOptionID != nil && *opt.PollOptionID == f.optionAID && len(opt.Voters) > 0 {
			voterFound = &opt.Voters[0]
		}
	}

	require.NotNil(t, voterFound,
		"with individual votes enabled, the player should see who voted for option A")
	assert.Equal(t, f.playerUserID, voterFound.UserID)
	assert.Contains(t, voterFound.CharacterName, "Hero")
}

func TestCreatePoll_RunningTotalsWithHiddenResults_Rejected(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, visibilityTables...)

	app := core.NewTestApp(testDB.Pool)
	router := setupVisibilityRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "rtexcl_gm", "rtexcl_gm@example.com")
	gameRecord := testDB.CreateTestGame(t, int32(gm.ID), "Running Totals Exclusivity Game")
	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)

	body, _ := json.Marshal(CreatePollRequest{
		Question:                   "Both at once?",
		Deadline:                   time.Now().Add(24 * time.Hour),
		HideResultsFromPlayers:     true,
		ShowRunningTotalsToPlayers: true,
		Options: []PollOptionRequest{
			{Text: "A", DisplayOrder: 1},
			{Text: "B", DisplayOrder: 2},
		},
	})
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/games/%d/polls", gameRecord.ID), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gmToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code,
		"hiding results and showing running totals are mutually exclusive")

	var count int
	require.NoError(t, testDB.Pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM common_room_polls WHERE game_id = $1", gameRecord.ID).Scan(&count))
	assert.Equal(t, 0, count, "rejected poll must not be persisted")
}

func TestUpdatePoll_EnablingRunningTotals_OpensResultsToPlayer(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, visibilityTables...)

	app := core.NewTestApp(testDB.Pool)
	router := setupVisibilityRouter(app, testDB)

	f := setupVisibilityFixture(t, testDB, app, "rtupon", core.CreatePollRequest{
		ShowRunningTotalsToPlayers: false,
	})

	require.Equal(t, http.StatusOK, voteReq(t, router, f.pollID, f.optionAID, f.playerToken).Code)
	require.Equal(t, http.StatusForbidden, getResultsReq(t, router, f.pollID, f.playerToken).Code,
		"precondition: results start closed while the poll is open")

	rec := updatePollReq(t, router, f.pollID, f.gmToken, UpdatePollRequest{
		ShowRunningTotalsToPlayers: true,
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var persisted bool
	require.NoError(t, testDB.Pool.QueryRow(context.Background(),
		"SELECT show_running_totals_to_players FROM common_room_polls WHERE id = $1", f.pollID).Scan(&persisted))
	assert.True(t, persisted, "the update must persist the flag")

	assert.Equal(t, http.StatusOK, getResultsReq(t, router, f.pollID, f.playerToken).Code,
		"enabling running totals should open the still-active poll's results to the player")
}

func TestUpdatePoll_DisablingRunningTotals_ReclosesResultsToPlayer(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, visibilityTables...)

	app := core.NewTestApp(testDB.Pool)
	router := setupVisibilityRouter(app, testDB)

	f := setupVisibilityFixture(t, testDB, app, "rtupoff", core.CreatePollRequest{
		ShowRunningTotalsToPlayers: true,
	})

	require.Equal(t, http.StatusOK, voteReq(t, router, f.pollID, f.optionAID, f.playerToken).Code)
	require.Equal(t, http.StatusOK, getResultsReq(t, router, f.pollID, f.playerToken).Code,
		"precondition: running totals start open to the player")

	// Revoking the setting must take effect immediately, not leave the results
	// exposed for the rest of the poll's life.
	rec := updatePollReq(t, router, f.pollID, f.gmToken, UpdatePollRequest{
		ShowRunningTotalsToPlayers: false,
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var persisted bool
	require.NoError(t, testDB.Pool.QueryRow(context.Background(),
		"SELECT show_running_totals_to_players FROM common_room_polls WHERE id = $1", f.pollID).Scan(&persisted))
	assert.False(t, persisted, "the update must persist the cleared flag")

	assert.Equal(t, http.StatusForbidden, getResultsReq(t, router, f.pollID, f.playerToken).Code,
		"disabling running totals should re-close the active poll's results")
}

func TestUpdatePoll_RunningTotalsWithHiddenResults_Rejected(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, visibilityTables...)

	app := core.NewTestApp(testDB.Pool)
	router := setupVisibilityRouter(app, testDB)

	f := setupVisibilityFixture(t, testDB, app, "rtupexcl", core.CreatePollRequest{
		ShowRunningTotalsToPlayers: true,
	})

	rec := updatePollReq(t, router, f.pollID, f.gmToken, UpdatePollRequest{
		HideResultsFromPlayers:     true,
		ShowRunningTotalsToPlayers: true,
	})
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code,
		"the update path must enforce the same exclusivity as creation")

	// A rejected update must not partially apply.
	var hidden, running bool
	require.NoError(t, testDB.Pool.QueryRow(context.Background(),
		"SELECT hide_results_from_players, show_running_totals_to_players FROM common_room_polls WHERE id = $1",
		f.pollID).Scan(&hidden, &running))
	assert.False(t, hidden, "rejected update must not hide results")
	assert.True(t, running, "rejected update must leave the original setting intact")
}

func TestUpdatePoll_RunningTotals_PlayerForbidden(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, visibilityTables...)

	app := core.NewTestApp(testDB.Pool)
	router := setupVisibilityRouter(app, testDB)

	f := setupVisibilityFixture(t, testDB, app, "rtupauth", core.CreatePollRequest{
		ShowRunningTotalsToPlayers: false,
	})

	// A player must not be able to reveal the running tally to themselves.
	rec := updatePollReq(t, router, f.pollID, f.playerToken, UpdatePollRequest{
		ShowRunningTotalsToPlayers: true,
	})
	assert.Equal(t, http.StatusForbidden, rec.Code, "only the GM may change poll visibility")

	var persisted bool
	require.NoError(t, testDB.Pool.QueryRow(context.Background(),
		"SELECT show_running_totals_to_players FROM common_room_polls WHERE id = $1", f.pollID).Scan(&persisted))
	assert.False(t, persisted, "a forbidden update must not persist")
}

// ============================================================
// Audience voting
// ============================================================

func TestAudienceVoting_Disabled_AudienceDenied(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, visibilityTables...)

	app := core.NewTestApp(testDB.Pool)
	router := setupVisibilityRouter(app, testDB)

	f := setupVisibilityFixture(t, testDB, app, "audoff", core.CreatePollRequest{
		AllowAudienceVoting: false,
	})

	rec := voteReq(t, router, f.pollID, f.optionAID, f.audienceToken)
	assert.Equal(t, http.StatusForbidden, rec.Code)

	var count int
	require.NoError(t, testDB.Pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM poll_votes WHERE poll_id = $1", f.pollID).Scan(&count))
	assert.Equal(t, 0, count, "denied audience vote must not be persisted")
}

func TestAudienceVoting_Enabled_AudienceVoteCounted(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, visibilityTables...)

	app := core.NewTestApp(testDB.Pool)
	router := setupVisibilityRouter(app, testDB)

	f := setupVisibilityFixture(t, testDB, app, "audon", core.CreatePollRequest{
		AllowAudienceVoting: true,
	})

	rec := voteReq(t, router, f.pollID, f.optionAID, f.audienceToken)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var count int
	require.NoError(t, testDB.Pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM poll_votes WHERE poll_id = $1", f.pollID).Scan(&count))
	assert.Equal(t, 1, count, "audience vote should be persisted")

	// The audience vote must count toward the option tally.
	results := getResultsReq(t, router, f.pollID, f.gmToken)
	require.Equal(t, http.StatusOK, results.Code, results.Body.String())

	var parsed PollResultsResponse
	require.NoError(t, json.Unmarshal(results.Body.Bytes(), &parsed))
	assert.Equal(t, int32(1), parsed.TotalVotes)
}

func TestAudienceVoting_AudienceIsAnonymousEvenToGM(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, visibilityTables...)

	app := core.NewTestApp(testDB.Pool)
	router := setupVisibilityRouter(app, testDB)

	f := setupVisibilityFixture(t, testDB, app, "audanon", core.CreatePollRequest{
		AllowAudienceVoting: true,
	})

	// Audience votes A, player votes B, so the two are distinguishable in results.
	require.Equal(t, http.StatusOK, voteReq(t, router, f.pollID, f.optionAID, f.audienceToken).Code)
	require.Equal(t, http.StatusOK, voteReq(t, router, f.pollID, f.optionBID, f.playerToken).Code)

	rec := getResultsReq(t, router, f.pollID, f.gmToken)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var results PollResultsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &results))
	assert.Equal(t, int32(2), results.TotalVotes)

	var audienceVoter, playerVoter *VoterInfo
	for i := range results.OptionResults {
		opt := &results.OptionResults[i]
		for j := range opt.Voters {
			if opt.PollOptionID != nil && *opt.PollOptionID == f.optionAID {
				audienceVoter = &opt.Voters[j]
			} else {
				playerVoter = &opt.Voters[j]
			}
		}
	}

	require.NotNil(t, audienceVoter, "audience vote should appear in results")
	assert.True(t, audienceVoter.IsAnonymous, "audience vote must be flagged anonymous")
	assert.Equal(t, "Anonymous", audienceVoter.CharacterName)
	assert.Equal(t, int32(0), audienceVoter.UserID,
		"audience user ID must not be disclosed, even to the GM")

	// The player's own vote must remain attributed.
	require.NotNil(t, playerVoter, "player vote should appear in results")
	assert.False(t, playerVoter.IsAnonymous, "player votes must stay attributed")
	assert.Equal(t, f.playerUserID, playerVoter.UserID)
	assert.Contains(t, playerVoter.CharacterName, "Hero")
}

func TestAudienceVoting_AudienceNeedsNoCharacter(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, visibilityTables...)

	app := core.NewTestApp(testDB.Pool)
	router := setupVisibilityRouter(app, testDB)

	f := setupVisibilityFixture(t, testDB, app, "audnochar", core.CreatePollRequest{
		AllowAudienceVoting: true,
	})

	// The fixture's audience member has no character at all; voting must still work.
	var charCount int
	require.NoError(t, testDB.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM characters c
		 JOIN game_participants gp ON gp.game_id = c.game_id AND gp.user_id = c.user_id
		 WHERE gp.game_id = $1 AND gp.role = 'audience'`, f.gameID).Scan(&charCount))
	require.Equal(t, 0, charCount, "fixture audience member should have no character")

	rec := voteReq(t, router, f.pollID, f.optionAID, f.audienceToken)
	assert.Equal(t, http.StatusOK, rec.Code,
		"audience must be able to vote without an approved character")
}

func TestAudienceVoting_Enabled_GMStillCannotVote(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, visibilityTables...)

	app := core.NewTestApp(testDB.Pool)
	router := setupVisibilityRouter(app, testDB)

	f := setupVisibilityFixture(t, testDB, app, "audgm", core.CreatePollRequest{
		AllowAudienceVoting: true,
	})

	rec := voteReq(t, router, f.pollID, f.optionAID, f.gmToken)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"enabling audience voting must not let the GM vote")
}

func TestAudienceVoting_AfterDeadline_Rejected(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, visibilityTables...)

	app := core.NewTestApp(testDB.Pool)
	router := setupVisibilityRouter(app, testDB)

	f := setupVisibilityFixture(t, testDB, app, "auddead", core.CreatePollRequest{
		AllowAudienceVoting: true,
	})

	expirePoll(t, testDB, f.pollID)

	rec := voteReq(t, router, f.pollID, f.optionAID, f.audienceToken)
	assert.Equal(t, http.StatusBadRequest, rec.Code,
		"audience voting must respect the poll deadline")
}
