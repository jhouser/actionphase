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
	db "actionphase/pkg/db/services"
	dbservices "actionphase/pkg/db/services"
	dbactions "actionphase/pkg/db/services/actions"
	dbmessages "actionphase/pkg/db/services/messages"
	"actionphase/pkg/games"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupVoteRouter(app *core.App, testDB *core.TestDatabase) *chi.Mux {
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &dbservices.UserService{DB: testDB.Pool, Logger: app.ObsLogger}

	gameHandler := games.Handler{
		App:                     app,
		UserService:             &db.UserService{DB: app.Pool, Logger: app.ObsLogger},
		GameService:             &db.GameService{DB: app.Pool, Logger: app.ObsLogger},
		GameApplicationService:  &db.GameApplicationService{DB: app.Pool, Logger: app.ObsLogger},
		CharacterService:        &db.CharacterService{DB: app.Pool, Logger: app.ObsLogger},
		NotificationService:     db.NewNotificationService(app.Pool, app.ObsLogger),
		MessageService:          &dbmessages.MessageService{DB: app.Pool, Logger: app.ObsLogger, Metrics: app.Observability.OTELMetrics},
		ActionSubmissionService: &dbactions.ActionSubmissionService{DB: app.Pool, Logger: app.ObsLogger, NotificationService: db.NewNotificationService(app.Pool, app.ObsLogger)},
	}
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
			r.With(gameHandler.GameMiddleware()).Post("/games/{gameID}/polls", handler.CreatePoll)
			r.Post("/polls/{pollId}/vote", handler.SubmitVote)
			r.Get("/polls/{pollId}", handler.GetPoll)
		})
	})

	return router
}

func setupPollVoteData(t *testing.T, testDB *core.TestDatabase, app *core.App, router *chi.Mux) (
	gm *core.User, player *core.User, outsider *core.User,
	gameID int32,
	pollID int32, optionID int32,
	gmToken, playerToken, outsiderToken string,
) {
	t.Helper()

	gm = testDB.CreateTestUser(t, "vote_gm", "vote_gm@example.com")
	player = testDB.CreateTestUser(t, "vote_player", "vote_player@example.com")
	outsider = testDB.CreateTestUser(t, "vote_outsider", "vote_outsider@example.com")

	gameRecord := testDB.CreateTestGame(t, int32(gm.ID), "Poll Vote Test Game")

	gameService := &dbservices.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err := gameService.AddGameParticipant(context.Background(), gameRecord.ID, int32(player.ID), "player")
	require.NoError(t, err)

	// Player needs an approved character to vote
	factory := core.NewTestDataFactory(testDB, t)
	factory.NewCharacter().ForGame(gameRecord.ID).WithUserID(int32(player.ID)).PlayerCharacter().Approved().WithName("Vote Player Hero").Create()

	// Create poll via service directly
	pollService := &dbservices.PollService{DB: testDB.Pool, Logger: app.ObsLogger}
	created, err := pollService.CreatePollWithOptions(context.Background(), core.CreatePollRequest{
		GameID:          gameRecord.ID,
		CreatedByUserID: int32(gm.ID),
		Question:        "Which option do you prefer?",
		Deadline:        time.Now().Add(24 * time.Hour),
		Options: []core.PollOptionInput{
			{Text: "Option A", DisplayOrder: 1},
			{Text: "Option B", DisplayOrder: 2},
		},
	})
	require.NoError(t, err)

	optionID = created.Options[0].ID
	gameID = gameRecord.ID

	gmToken, err = core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err = core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)
	outsiderToken, err = core.CreateTestJWTTokenForUser(app, outsider)
	require.NoError(t, err)

	return gm, player, outsider, gameRecord.ID, created.Poll.ID, optionID, gmToken, playerToken, outsiderToken
}

func submitVoteReq(t *testing.T, router *chi.Mux, pollID int32, optionID int32, token string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(SubmitVoteRequest{SelectedOptionID: &optionID})
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/polls/%d/vote", pollID), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestPollVote_Player_Succeeds_AndStateUpdated(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "poll_votes", "poll_options", "common_room_polls", "game_participants", "characters", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupVoteRouter(app, testDB)

	_, _, _, _, pollID, optionID, _, playerToken, _ := setupPollVoteData(t, testDB, app, router)

	rec := submitVoteReq(t, router, pollID, optionID, playerToken)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Verify DB state: response recorded
	var count int
	err := testDB.Pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM poll_votes WHERE poll_id = $1", pollID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "vote should be persisted")
}

func TestPollVote_GM_CannotVote_403(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "poll_votes", "poll_options", "common_room_polls", "game_participants", "characters", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupVoteRouter(app, testDB)

	_, _, _, _, pollID, optionID, gmToken, _, _ := setupPollVoteData(t, testDB, app, router)

	rec := submitVoteReq(t, router, pollID, optionID, gmToken)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestPollVote_Outsider_CannotVote_403(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "poll_votes", "poll_options", "common_room_polls", "game_participants", "characters", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupVoteRouter(app, testDB)

	_, _, _, _, pollID, optionID, _, _, outsiderToken := setupPollVoteData(t, testDB, app, router)

	rec := submitVoteReq(t, router, pollID, optionID, outsiderToken)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestPollVote_AudienceMember_CannotVote_403(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "poll_votes", "poll_options", "common_room_polls", "game_audience", "game_participants", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupVoteRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "vote_gm_a", "vote_gm_a@example.com")
	audience := testDB.CreateTestUser(t, "vote_audience", "vote_audience@example.com")
	gameRecord := testDB.CreateTestGame(t, int32(gm.ID), "Audience Vote Test Game")

	// Add audience member
	queries := dbmodels.New(testDB.Pool)
	_, err := queries.CreateAudienceApplication(context.Background(), dbmodels.CreateAudienceApplicationParams{
		GameID: gameRecord.ID,
		UserID: int32(audience.ID),
		Status: pgtype.Text{String: "active", Valid: true},
	})
	require.NoError(t, err)

	pollService := &dbservices.PollService{DB: testDB.Pool, Logger: app.ObsLogger}
	created, err := pollService.CreatePollWithOptions(context.Background(), core.CreatePollRequest{
		GameID:          gameRecord.ID,
		CreatedByUserID: int32(gm.ID),
		Question:        "Audience poll test",
		Deadline:        time.Now().Add(24 * time.Hour),
		Options: []core.PollOptionInput{
			{Text: "A", DisplayOrder: 1},
			{Text: "B", DisplayOrder: 2},
		},
	})
	require.NoError(t, err)

	audienceToken, err := core.CreateTestJWTTokenForUser(app, audience)
	require.NoError(t, err)

	optionID := created.Options[0].ID
	rec := submitVoteReq(t, router, created.Poll.ID, optionID, audienceToken)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestPollVote_AfterDeadline_400(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "poll_votes", "poll_options", "common_room_polls", "game_participants", "characters", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupVoteRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "vote_gm_d", "vote_gm_d@example.com")
	player := testDB.CreateTestUser(t, "vote_player_d", "vote_player_d@example.com")
	gameRecord := testDB.CreateTestGame(t, int32(gm.ID), "Deadline Test Game")

	gameService := &dbservices.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err := gameService.AddGameParticipant(context.Background(), gameRecord.ID, int32(player.ID), "player")
	require.NoError(t, err)

	// Player needs an approved character to reach the deadline check
	factory := core.NewTestDataFactory(testDB, t)
	factory.NewCharacter().ForGame(gameRecord.ID).WithUserID(int32(player.ID)).PlayerCharacter().Approved().WithName("Deadline Player Hero").Create()

	pollService := &dbservices.PollService{DB: testDB.Pool, Logger: app.ObsLogger}
	created, err := pollService.CreatePollWithOptions(context.Background(), core.CreatePollRequest{
		GameID:          gameRecord.ID,
		CreatedByUserID: int32(gm.ID),
		Question:        "Expired poll",
		Deadline:        time.Now().Add(-1 * time.Hour), // past deadline
		Options: []core.PollOptionInput{
			{Text: "A", DisplayOrder: 1},
			{Text: "B", DisplayOrder: 2},
		},
	})
	require.NoError(t, err)

	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	optionID := created.Options[0].ID
	rec := submitVoteReq(t, router, created.Poll.ID, optionID, playerToken)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPollVote_ChangeVote_UpdatesRecord(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "poll_votes", "poll_options", "common_room_polls", "game_participants", "characters", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupVoteRouter(app, testDB)

	_, _, _, _, pollID, optionID, _, playerToken, _ := setupPollVoteData(t, testDB, app, router)

	// First vote
	rec1 := submitVoteReq(t, router, pollID, optionID, playerToken)
	assert.Equal(t, http.StatusOK, rec1.Code, rec1.Body.String())

	// Second vote (change) — should succeed and not create duplicate
	rec2 := submitVoteReq(t, router, pollID, optionID, playerToken)
	assert.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())

	// Verify only one response record exists for this player
	var count int
	err := testDB.Pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM poll_votes WHERE poll_id = $1", pollID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "re-voting should update existing record, not create duplicate")
}
