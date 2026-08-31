package games

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
	db "actionphase/pkg/db/services"
	actionsvc "actionphase/pkg/db/services/actions"
	phasesvc "actionphase/pkg/db/services/phases"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGameAPI_ListAudienceMembers tests GET /api/v1/games/{id}/audience
func TestGameAPI_ListAudienceMembers(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	audienceMember := testDB.CreateTestUser(t, "audience", "audience@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(audienceMember.ID), "audience")
	require.NoError(t, err)

	t.Run("GM lists audience members", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/audience", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response ListAudienceMembersResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Len(t, response.AudienceMembers, 1)
		assert.Equal(t, "audience", response.AudienceMembers[0].Role)
	})

	t.Run("player can list audience members", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/audience", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response ListAudienceMembersResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Len(t, response.AudienceMembers, 1)
	})

	t.Run("empty audience when no audience members", func(t *testing.T) {
		otherGM := testDB.CreateTestUser(t, "othergm", "othergm@example.com")
		otherGMToken, err := core.CreateTestJWTTokenForUser(app, otherGM)
		require.NoError(t, err)
		emptyGame := testDB.CreateTestGame(t, int32(otherGM.ID), "Empty Audience Game")

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/audience", emptyGame.ID), nil)
		req.Header.Set("Authorization", "Bearer "+otherGMToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response ListAudienceMembersResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Len(t, response.AudienceMembers, 0)
	})
}

// TestGameAPI_UpdateAutoAcceptAudience tests PUT /api/v1/games/{id}/settings/auto-accept-audience
func TestGameAPI_UpdateAutoAcceptAudience(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	t.Run("GM enables auto-accept audience", func(t *testing.T) {
		body := UpdateAutoAcceptAudienceRequest{AutoAcceptAudience: true}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/games/%d/settings/auto-accept-audience", game.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		assert.Contains(t, response["message"], "updated")

		// Verify the setting was actually changed in DB
		updatedGame, err := gameService.GetGame(context.Background(), game.ID)
		require.NoError(t, err)
		assert.True(t, updatedGame.AutoAcceptAudience)
	})

	t.Run("GM disables auto-accept audience", func(t *testing.T) {
		body := UpdateAutoAcceptAudienceRequest{AutoAcceptAudience: false}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/games/%d/settings/auto-accept-audience", game.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		updatedGame, err := gameService.GetGame(context.Background(), game.ID)
		require.NoError(t, err)
		assert.False(t, updatedGame.AutoAcceptAudience)
	})

	t.Run("non-GM player cannot update auto-accept audience", func(t *testing.T) {
		body := UpdateAutoAcceptAudienceRequest{AutoAcceptAudience: true}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/games/%d/settings/auto-accept-audience", game.ID), bytes.NewBuffer(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+playerToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestGameAPI_ListAllPrivateConversations tests GET /api/v1/games/{id}/private-messages/all
// Validates: response shape, access control (participant vs outsider), data accuracy
func TestGameAPI_ListAllPrivateConversations(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "private_messages", "conversation_participants", "conversations", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
	outsider := testDB.CreateTestUser(t, "outsider", "outsider@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	outsiderToken, err := core.CreateTestJWTTokenForUser(app, outsider)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	require.NoError(t, err)

	// Create characters and a private conversation
	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	p1ID := int32(player1.ID)
	p2ID := int32(player2.ID)
	char1, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: &p1ID, Name: "Char1", CharacterType: "player_character",
	})
	require.NoError(t, err)
	char2, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: &p2ID, Name: "Char2", CharacterType: "player_character",
	})
	require.NoError(t, err)

	conversationService := db.NewConversationService(testDB.Pool, nil)
	_, err = conversationService.CreateConversation(context.Background(), db.CreateConversationRequest{
		GameID:          game.ID,
		Title:           "Secret chat",
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	require.NoError(t, err)

	t.Run("GM can list all private conversations", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/private-messages/all", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		// Validate response shape
		conversations, ok := response["conversations"].([]interface{})
		assert.True(t, ok, "response should have 'conversations' array")
		assert.GreaterOrEqual(t, len(conversations), 1, "should include the created conversation")
		_, hasTotal := response["total"]
		assert.True(t, hasTotal, "response should include 'total' field")
		// Validate conversation fields
		conv := conversations[0].(map[string]interface{})
		assert.Contains(t, conv, "conversation_id")
		assert.Contains(t, conv, "conversation_type")
		assert.Contains(t, conv, "participant_names")
	})

	t.Run("outsider cannot list private conversations", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/private-messages/all", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+outsiderToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestGameAPI_GetConversationParticipants tests GET /api/v1/games/{id}/private-messages/participants
// Validates: response shape, access control, returns participating characters as {id, name}
func TestGameAPI_GetConversationParticipants(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "private_messages", "conversation_participants", "conversations", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
	outsider := testDB.CreateTestUser(t, "outsider", "outsider@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	outsiderToken, err := core.CreateTestJWTTokenForUser(app, outsider)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	require.NoError(t, err)

	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	p1ID := int32(player1.ID)
	p2ID := int32(player2.ID)
	char1, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: &p1ID, Name: "AlphaChar", CharacterType: "player_character",
	})
	require.NoError(t, err)
	char2, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: &p2ID, Name: "BetaChar", CharacterType: "player_character",
	})
	require.NoError(t, err)

	conversationService := db.NewConversationService(testDB.Pool, nil)
	_, err = conversationService.CreateConversation(context.Background(), db.CreateConversationRequest{
		GameID:          game.ID,
		Title:           "A conversation",
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	require.NoError(t, err)

	t.Run("GM gets participant characters", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/private-messages/participants", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		participants, ok := response["participants"].([]interface{})
		assert.True(t, ok, "response should have 'participants' array")
		// Participants are {id, name} objects: the UI displays the name but filters by ID.
		names := make([]string, len(participants))
		ids := make([]int32, len(participants))
		for i, p := range participants {
			obj, ok := p.(map[string]interface{})
			require.True(t, ok, "participant should be an object with id and name")
			names[i] = obj["name"].(string)
			ids[i] = int32(obj["id"].(float64))
		}
		assert.Contains(t, names, "AlphaChar")
		assert.Contains(t, names, "BetaChar")
		assert.Contains(t, ids, char1.ID)
		assert.Contains(t, ids, char2.ID)
	})

	t.Run("outsider cannot get participant characters", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/private-messages/participants", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+outsiderToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestGameAPI_GetAudienceConversationMessages tests GET /api/v1/games/{id}/private-messages/conversations/{conversationId}
// Validates: response shape, access control, message content correctness
func TestGameAPI_GetAudienceConversationMessages(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "private_messages", "conversation_participants", "conversations", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	player2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
	outsider := testDB.CreateTestUser(t, "outsider", "outsider@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	outsiderToken, err := core.CreateTestJWTTokenForUser(app, outsider)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player1.ID), "player")
	require.NoError(t, err)
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player2.ID), "player")
	require.NoError(t, err)

	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	p1ID := int32(player1.ID)
	p2ID := int32(player2.ID)
	char1, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: &p1ID, Name: "Char1", CharacterType: "player_character",
	})
	require.NoError(t, err)
	char2, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
		GameID: game.ID, UserID: &p2ID, Name: "Char2", CharacterType: "player_character",
	})
	require.NoError(t, err)

	conversationService := db.NewConversationService(testDB.Pool, nil)
	conv, err := conversationService.CreateConversation(context.Background(), db.CreateConversationRequest{
		GameID:          game.ID,
		Title:           "Private chat",
		CreatedByUserID: int32(player1.ID),
		ParticipantIDs:  []int32{char1.ID, char2.ID},
	})
	require.NoError(t, err)

	_, err = conversationService.SendMessage(context.Background(), db.SendMessageRequest{
		ConversationID:    conv.ID,
		SenderUserID:      int32(player1.ID),
		SenderCharacterID: char1.ID,
		Content:           "Hello from Char1",
	})
	require.NoError(t, err)

	t.Run("GM can read conversation messages", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/private-messages/conversations/%d", game.ID, conv.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		messages, ok := response["messages"].([]interface{})
		assert.True(t, ok, "response should have 'messages' array")
		assert.Len(t, messages, 1)
		// Validate message shape
		msg := messages[0].(map[string]interface{})
		assert.Equal(t, "Hello from Char1", msg["content"])
		assert.Contains(t, msg, "sender_username")
		assert.Contains(t, msg, "created_at")
		assert.Contains(t, msg, "is_deleted")
	})

	t.Run("outsider cannot read conversation messages", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/private-messages/conversations/%d", game.ID, conv.ID), nil)
		req.Header.Set("Authorization", "Bearer "+outsiderToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestGameAPI_ListAllActionSubmissions tests GET /api/v1/games/{id}/action-submissions/all
// Validates: response shape, access control, correct data returned
func TestGameAPI_ListAllActionSubmissions(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "action_submissions", "game_phases", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")
	outsider := testDB.CreateTestUser(t, "outsider", "outsider@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	outsiderToken, err := core.CreateTestJWTTokenForUser(app, outsider)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	// Create an action phase and submit an action
	phaseService := &phasesvc.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Round 1",
		Deadline:  core.TimePtr(time.Now().Add(72 * time.Hour)),
	})
	require.NoError(t, err)

	actionService := &actionsvc.ActionSubmissionService{
		DB:                  testDB.Pool,
		Logger:              app.ObsLogger,
		NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger},
	}
	_, err = actionService.SubmitAction(context.Background(), core.SubmitActionRequest{
		GameID:  game.ID,
		PhaseID: phase.ID,
		UserID:  int32(player.ID),
		Content: "I search the ancient library.",
	})
	require.NoError(t, err)

	t.Run("GM can list all action submissions", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/action-submissions/all", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		submissions, ok := response["action_submissions"].([]interface{})
		assert.True(t, ok, "response should have 'action_submissions' array")
		assert.Len(t, submissions, 1)
		// Validate submission shape and data correctness
		sub := submissions[0].(map[string]interface{})
		assert.Equal(t, "I search the ancient library.", sub["content"])
		assert.Contains(t, sub, "username")
		assert.Contains(t, sub, "phase_type")
		assert.Contains(t, sub, "phase_number")
		_, hasTotal := response["total"]
		assert.True(t, hasTotal, "response should include 'total' field")
	})

	t.Run("outsider cannot list action submissions", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/action-submissions/all", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+outsiderToken)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestGameAPI_ListAudienceNPCs tests GET /api/v1/games/{id}/characters/audience-npcs
func TestGameAPI_ListAudienceNPCs(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "npc_assignments", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	player := testDB.CreateTestUser(t, "player", "player@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)
	playerToken, err := core.CreateTestJWTTokenForUser(app, player)
	require.NoError(t, err)

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	t.Run("returns 200 with empty list when no audience NPCs exist", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/characters/audience-npcs", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var body map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		_, ok := body["npcs"]
		assert.True(t, ok, "response should contain 'npcs' key")
	})

	t.Run("authenticated player can list audience NPCs", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/characters/audience-npcs", game.ID), nil)
		req.Header.Set("Authorization", "Bearer "+playerToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("invalid game ID returns 400", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/games/notanumber/characters/audience-npcs", nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestGameAPI_ConversationFilter_RepeatedQueryParams covers the HTTP query-parsing
// layer of the audience participant filter, which the service-level AND-semantics
// test in pkg/db/services/messages could not: it calls the service directly and so
// never sees how repeated query params are decoded.
//
// The frontend sends the filter as repeated params (?participant_ids=1&participant_ids=2).
// Huma disables `explode` on query params by default, which silently keeps only the
// FIRST occurrence and drops the rest — so selecting a second character had no effect
// and the list stayed filtered on the first character alone.
func TestGameAPI_ConversationFilter_RepeatedQueryParams(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "private_messages", "conversation_participants", "conversations", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)

	gm := testDB.CreateTestUser(t, "gm", "gm@example.com")
	p1 := testDB.CreateTestUser(t, "player1", "player1@example.com")
	p2 := testDB.CreateTestUser(t, "player2", "player2@example.com")
	p3 := testDB.CreateTestUser(t, "player3", "player3@example.com")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gm)
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(gm.ID), "Test Game")
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	for _, u := range []int32{int32(p1.ID), int32(p2.ID), int32(p3.ID)} {
		_, err = gameService.AddGameParticipant(context.Background(), game.ID, u, "player")
		require.NoError(t, err)
	}

	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	mkChar := func(u int32, name string) int32 {
		c, err := characterService.CreateCharacter(context.Background(), db.CreateCharacterRequest{
			GameID: game.ID, UserID: &u, Name: name, CharacterType: "player_character",
		})
		require.NoError(t, err)
		return c.ID
	}
	alpha := mkChar(int32(p1.ID), "Alpha")
	beta := mkChar(int32(p2.ID), "Beta")
	gamma := mkChar(int32(p3.ID), "Gamma")

	// Three conversations so that filtering on {Alpha,Beta} has something to exclude
	// in both directions: Alpha-Gamma has Alpha but not Beta, Beta-Gamma the reverse.
	conversationService := db.NewConversationService(testDB.Pool, nil)
	mkConv := func(title string, participants ...int32) {
		_, err := conversationService.CreateConversation(context.Background(), db.CreateConversationRequest{
			GameID: game.ID, Title: title, CreatedByUserID: int32(p1.ID), ParticipantIDs: participants,
		})
		require.NoError(t, err)
	}
	mkConv("Alpha-Beta", alpha, beta)
	mkConv("Alpha-Gamma", alpha, gamma)
	mkConv("Beta-Gamma", beta, gamma)

	getConvs := func(t *testing.T, query string) ([]string, float64) {
		t.Helper()
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/games/%d/private-messages/all%s", game.ID, query), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		var response struct {
			Conversations []struct {
				Subject *string `json:"subject"`
			} `json:"conversations"`
			Total float64 `json:"total"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		titles := make([]string, 0, len(response.Conversations))
		for _, c := range response.Conversations {
			if c.Subject != nil {
				titles = append(titles, *c.Subject)
			}
		}
		return titles, response.Total
	}

	t.Run("single participant filter returns every conversation with that character", func(t *testing.T) {
		titles, total := getConvs(t, fmt.Sprintf("?participant_ids=%d", alpha))
		assert.ElementsMatch(t, []string{"Alpha-Beta", "Alpha-Gamma"}, titles)
		assert.Equal(t, float64(2), total, "total must reflect the filter, not the unfiltered count")
	})

	t.Run("two participants as repeated params AND together", func(t *testing.T) {
		titles, total := getConvs(t, fmt.Sprintf("?participant_ids=%d&participant_ids=%d", alpha, beta))
		assert.Equal(t, []string{"Alpha-Beta"}, titles,
			"selecting a second character must narrow the list to conversations involving BOTH")
		assert.Equal(t, float64(1), total)
	})

	t.Run("repeated params AND regardless of order", func(t *testing.T) {
		// Guards specifically against reading only the first occurrence: with that bug
		// this ordering matched on Beta alone and wrongly included Beta-Gamma.
		titles, _ := getConvs(t, fmt.Sprintf("?participant_ids=%d&participant_ids=%d", beta, alpha))
		assert.Equal(t, []string{"Alpha-Beta"}, titles)
	})

	t.Run("three participants with no shared conversation return nothing", func(t *testing.T) {
		titles, total := getConvs(t, fmt.Sprintf("?participant_ids=%d&participant_ids=%d&participant_ids=%d", alpha, beta, gamma))
		assert.Empty(t, titles, "no conversation has all three characters")
		assert.Equal(t, float64(0), total)
	})

	t.Run("participants filter narrows options to co-participants via repeated selected[]", func(t *testing.T) {
		// The chip list must narrow as the selection grows, otherwise the UI offers
		// characters that would produce an empty result.
		req := httptest.NewRequest("GET", fmt.Sprintf(
			"/api/v1/games/%d/private-messages/participants?selected%%5B%%5D=%d&selected%%5B%%5D=%d",
			game.ID, alpha, beta), nil)
		req.Header.Set("Authorization", "Bearer "+gmToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		var response struct {
			Participants []struct {
				Name string `json:"name"`
			} `json:"participants"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		names := make([]string, 0, len(response.Participants))
		for _, p := range response.Participants {
			names = append(names, p.Name)
		}
		assert.ElementsMatch(t, []string{"Alpha", "Beta"}, names,
			"only Alpha-Beta matches both, so Gamma must not be offered")
	})
}
