package characters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	services "actionphase/pkg/db/services"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// setupStatsTestRouter creates a test router for character stats
func setupStatsTestRouter(app *core.App, testDB *core.TestDatabase) *chi.Mux {
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &services.UserService{DB: testDB.Pool, Logger: app.ObsLogger}

	router := chi.NewRouter()
	router.Route("/api/v1/characters/{id}", func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(jwtauth.Authenticator(tokenAuth))
		r.Use(core.RequireAuthenticationMiddleware(userService))
		handler := &Handler{
			App:                 app,
			UserService:         &services.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
			CharacterService:    &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger},
			GameService:         &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger},
			NotificationService: services.NewNotificationService(testDB.Pool, app.ObsLogger),
		}
		r.Get("/stats", handler.GetCharacterStats)
	})
	return router
}

// setupGameStatsTestRouter creates a test router for the batch, per-game character stats endpoint.
func setupGameStatsTestRouter(app *core.App, testDB *core.TestDatabase) *chi.Mux {
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &services.UserService{DB: testDB.Pool, Logger: app.ObsLogger}

	router := chi.NewRouter()
	router.Route("/api/v1/games/{gameId}/characters", func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(jwtauth.Authenticator(tokenAuth))
		r.Use(core.RequireAuthenticationMiddleware(userService))
		handler := &Handler{
			App:                 app,
			UserService:         &services.UserService{DB: testDB.Pool, Logger: app.ObsLogger},
			CharacterService:    &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger},
			GameService:         &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger},
			NotificationService: services.NewNotificationService(testDB.Pool, app.ObsLogger),
		}
		r.Get("/stats", handler.GetGameCharacterStats)
	})
	return router
}

// insertPrivateMessage inserts a private message sent by a character (for testing).
// Creates a minimal conversation record first, then inserts the message.
func insertPrivateMessage(t *testing.T, testDB *core.TestDatabase, senderCharID int32, senderUserID int32, gameID int32) {
	t.Helper()
	ctx := context.Background()

	// Create a conversation
	var convID int32
	err := testDB.Pool.QueryRow(ctx,
		`INSERT INTO conversations (game_id, conversation_type, created_by_user_id) VALUES ($1, 'direct', $2) RETURNING id`,
		gameID, senderUserID,
	).Scan(&convID)
	if err != nil {
		t.Fatalf("Failed to insert conversation: %v", err)
	}

	// Insert the private message
	_, err = testDB.Pool.Exec(ctx,
		`INSERT INTO private_messages (conversation_id, sender_user_id, sender_character_id, content, is_deleted)
		 VALUES ($1, $2, $3, 'Test private message', false)`,
		convID, senderUserID, senderCharID,
	)
	if err != nil {
		t.Fatalf("Failed to insert private message: %v", err)
	}
}

func TestGetCharacterStats_NoMessages(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "private_messages", "conversations", "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupStatsTestRouter(app, testDB)

	gmUser := testDB.CreateTestUser(t, "gm_stats1", "gm_stats1@example.com")
	game := testDB.CreateTestGame(t, int32(gmUser.ID), "Stats Test Game")

	// Create a character with no messages
	charService := &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	character, err := charService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &[]int32{int32(gmUser.ID)}[0],
		Name:          "Silent Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Creating character should succeed")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gmUser)
	core.AssertNoError(t, err, "GM token creation should succeed")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/characters/%d/stats", character.ID), nil)
	req.Header.Set("Authorization", "Bearer "+gmToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200")

	var resp CharacterStatsResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	core.AssertNoError(t, err, "Response should be valid JSON")
	core.AssertEqual(t, int64(0), resp.PublicMessages, "Public messages should be 0")
	if resp.PrivateMessages == nil || *resp.PrivateMessages != 0 {
		t.Errorf("Expected private_messages=0, got %v", resp.PrivateMessages)
	}
}

func TestGetCharacterStats_PublicMessageCount(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "private_messages", "conversations", "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupStatsTestRouter(app, testDB)

	gmUser := testDB.CreateTestUser(t, "gm_stats2", "gm_stats2@example.com")
	game := testDB.CreateTestGame(t, int32(gmUser.ID), "Stats Test Game 2")

	charService := &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	character, err := charService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &[]int32{int32(gmUser.ID)}[0],
		Name:          "Active Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Creating character should succeed")

	// Insert 3 public posts for this character
	testDB.CreateTestPost(t, game.ID, int32(gmUser.ID), character.ID, "Post 1")
	testDB.CreateTestPost(t, game.ID, int32(gmUser.ID), character.ID, "Post 2")
	testDB.CreateTestPost(t, game.ID, int32(gmUser.ID), character.ID, "Post 3")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gmUser)
	core.AssertNoError(t, err, "GM token creation should succeed")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/characters/%d/stats", character.ID), nil)
	req.Header.Set("Authorization", "Bearer "+gmToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200")

	var resp CharacterStatsResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	core.AssertNoError(t, err, "Response should be valid JSON")
	core.AssertEqual(t, int64(3), resp.PublicMessages, "Should count 3 public messages")
}

// TestGetCharacterStats_PrivateCountHiddenFromOtherPlayer verifies that a player cannot see
// another player's private message count in an active game.
func TestGetCharacterStats_PrivateCountHiddenFromOtherPlayer(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "private_messages", "conversations", "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupStatsTestRouter(app, testDB)

	gmUser := testDB.CreateTestUser(t, "gm_stats3", "gm_stats3@example.com")
	ownerUser := testDB.CreateTestUser(t, "owner_stats3", "owner_stats3@example.com")
	otherPlayer := testDB.CreateTestUser(t, "other_stats3", "other_stats3@example.com")
	game := testDB.CreateTestGame(t, int32(gmUser.ID), "Stats Test Game 3")

	gameService := &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(ownerUser.ID), "player")
	core.AssertNoError(t, err, "Adding owner to game should succeed")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(otherPlayer.ID), "player")
	core.AssertNoError(t, err, "Adding other player to game should succeed")

	// Create a character owned by ownerUser
	charService := &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	character, err := charService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &[]int32{int32(ownerUser.ID)}[0],
		Name:          "Owner Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Creating character should succeed")

	insertPrivateMessage(t, testDB, character.ID, int32(ownerUser.ID), game.ID)

	// Other player (not the owner) requests stats
	otherToken, err := core.CreateTestJWTTokenForUser(app, otherPlayer)
	core.AssertNoError(t, err, "Other player token creation should succeed")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/characters/%d/stats", character.ID), nil)
	req.Header.Set("Authorization", "Bearer "+otherToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200")

	var resp CharacterStatsResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	core.AssertNoError(t, err, "Response should be valid JSON")

	if resp.PrivateMessages != nil {
		t.Errorf("Other player in active game should not see private_messages, got %v", *resp.PrivateMessages)
	}
}

// TestGetCharacterStats_PrivateCountVisibleToOwner verifies that a player can see
// their own character's private message count.
func TestGetCharacterStats_PrivateCountVisibleToOwner(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "private_messages", "conversations", "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupStatsTestRouter(app, testDB)

	gmUser := testDB.CreateTestUser(t, "gm_stats3b", "gm_stats3b@example.com")
	ownerUser := testDB.CreateTestUser(t, "owner_stats3b", "owner_stats3b@example.com")
	game := testDB.CreateTestGame(t, int32(gmUser.ID), "Stats Test Game 3b")

	gameService := &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(ownerUser.ID), "player")
	core.AssertNoError(t, err, "Adding owner to game should succeed")

	charService := &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	character, err := charService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &[]int32{int32(ownerUser.ID)}[0],
		Name:          "My Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Creating character should succeed")

	insertPrivateMessage(t, testDB, character.ID, int32(ownerUser.ID), game.ID)
	insertPrivateMessage(t, testDB, character.ID, int32(ownerUser.ID), game.ID)

	ownerToken, err := core.CreateTestJWTTokenForUser(app, ownerUser)
	core.AssertNoError(t, err, "Owner token creation should succeed")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/characters/%d/stats", character.ID), nil)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200")

	var resp CharacterStatsResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	core.AssertNoError(t, err, "Response should be valid JSON")

	if resp.PrivateMessages == nil {
		t.Fatal("Character owner should see their own private_messages count")
	}
	core.AssertEqual(t, int64(2), *resp.PrivateMessages, "Owner should see 2 private messages")
}

func TestGetCharacterStats_PrivateCountVisibleToAudience(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "private_messages", "conversations", "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupStatsTestRouter(app, testDB)

	gmUser := testDB.CreateTestUser(t, "gm_stats4", "gm_stats4@example.com")
	audienceUser := testDB.CreateTestUser(t, "audience_stats4", "audience_stats4@example.com")
	game := testDB.CreateTestGame(t, int32(gmUser.ID), "Stats Test Game 4")

	// Add audience member
	gameService := &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(audienceUser.ID), "audience")
	core.AssertNoError(t, err, "Adding audience member should succeed")

	charService := &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	character, err := charService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &[]int32{int32(gmUser.ID)}[0],
		Name:          "Watched Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Creating character should succeed")

	insertPrivateMessage(t, testDB, character.ID, int32(gmUser.ID), game.ID)

	audienceToken, err := core.CreateTestJWTTokenForUser(app, audienceUser)
	core.AssertNoError(t, err, "Audience token creation should succeed")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/characters/%d/stats", character.ID), nil)
	req.Header.Set("Authorization", "Bearer "+audienceToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200")

	var resp CharacterStatsResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	core.AssertNoError(t, err, "Response should be valid JSON")

	// Audience should see private_messages
	if resp.PrivateMessages == nil {
		t.Fatal("Audience member should see private_messages")
	}
	core.AssertEqual(t, int64(1), *resp.PrivateMessages, "Should count 1 private message")
}

func TestGetCharacterStats_PrivateCountVisibleInCompletedGame(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "private_messages", "conversations", "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupStatsTestRouter(app, testDB)

	gmUser := testDB.CreateTestUser(t, "gm_stats5", "gm_stats5@example.com")
	playerUser := testDB.CreateTestUser(t, "player_stats5", "player_stats5@example.com")

	// Create game in setup state, set up participants and data, then mark completed.
	// AddGameParticipant and CreateCharacter block on completed games, so we set the
	// state last.
	game := testDB.CreateTestGame(t, int32(gmUser.ID), "Completed Stats Game")

	gameService := &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(playerUser.ID), "player")
	core.AssertNoError(t, err, "Adding player to game should succeed")

	charService := &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	character, err := charService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &[]int32{int32(playerUser.ID)}[0],
		Name:          "Archived Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Creating character should succeed")

	insertPrivateMessage(t, testDB, character.ID, int32(playerUser.ID), game.ID)
	insertPrivateMessage(t, testDB, character.ID, int32(playerUser.ID), game.ID)

	// Now mark the game completed — after all setup is done
	testDB.SetGameStateDirectly(t, game.ID, "completed")

	playerToken, err := core.CreateTestJWTTokenForUser(app, playerUser)
	core.AssertNoError(t, err, "Player token creation should succeed")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/characters/%d/stats", character.ID), nil)
	req.Header.Set("Authorization", "Bearer "+playerToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200")

	var resp CharacterStatsResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	core.AssertNoError(t, err, "Response should be valid JSON")

	// In completed game, any authenticated user sees private_messages
	if resp.PrivateMessages == nil {
		t.Fatal("Player in completed game should see private_messages")
	}
	core.AssertEqual(t, int64(2), *resp.PrivateMessages, "Should count 2 private messages")
}

func TestGetCharacterStats_GMSeesPrivateCount(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "private_messages", "conversations", "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupStatsTestRouter(app, testDB)

	gmUser := testDB.CreateTestUser(t, "gm_stats6", "gm_stats6@example.com")
	playerUser := testDB.CreateTestUser(t, "player_stats6", "player_stats6@example.com")
	game := testDB.CreateTestGame(t, int32(gmUser.ID), "Stats Test Game 6")

	gameService := &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(playerUser.ID), "player")
	core.AssertNoError(t, err, "Adding player to game should succeed")

	charService := &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	character, err := charService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &[]int32{int32(playerUser.ID)}[0],
		Name:          "GM Watched Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Creating character should succeed")

	insertPrivateMessage(t, testDB, character.ID, int32(playerUser.ID), game.ID)

	// Approve character (update status) so GM can see it normally
	queries := models.New(testDB.Pool)
	_, err = queries.UpdateCharacterStatus(context.Background(), models.UpdateCharacterStatusParams{
		ID:     character.ID,
		Status: pgtype.Text{String: "approved", Valid: true},
	})
	core.AssertNoError(t, err, "Approving character should succeed")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gmUser)
	core.AssertNoError(t, err, "GM token creation should succeed")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/characters/%d/stats", character.ID), nil)
	req.Header.Set("Authorization", "Bearer "+gmToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200")

	var resp CharacterStatsResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	core.AssertNoError(t, err, "Response should be valid JSON")

	// GM should see private_messages
	if resp.PrivateMessages == nil {
		t.Fatal("GM should see private_messages")
	}
	core.AssertEqual(t, int64(1), *resp.PrivateMessages, "GM should see 1 private message")
}

func TestGetCharacterStats_CoGMSeesPrivateCount(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "private_messages", "conversations", "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupStatsTestRouter(app, testDB)

	gmUser := testDB.CreateTestUser(t, "gm_stats7", "gm_stats7@example.com")
	coGMUser := testDB.CreateTestUser(t, "cogm_stats7", "cogm_stats7@example.com")
	playerUser := testDB.CreateTestUser(t, "player_stats7", "player_stats7@example.com")
	game := testDB.CreateTestGame(t, int32(gmUser.ID), "Stats Test Game 7")

	gameService := &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(coGMUser.ID), "co_gm")
	core.AssertNoError(t, err, "Adding co-GM should succeed")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(playerUser.ID), "player")
	core.AssertNoError(t, err, "Adding player should succeed")

	charService := &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	character, err := charService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &[]int32{int32(playerUser.ID)}[0],
		Name:          "Co-GM Watched Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Creating character should succeed")

	insertPrivateMessage(t, testDB, character.ID, int32(playerUser.ID), game.ID)

	coGMToken, err := core.CreateTestJWTTokenForUser(app, coGMUser)
	core.AssertNoError(t, err, "Co-GM token creation should succeed")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/characters/%d/stats", character.ID), nil)
	req.Header.Set("Authorization", "Bearer "+coGMToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200")

	var resp CharacterStatsResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	core.AssertNoError(t, err, "Response should be valid JSON")

	if resp.PrivateMessages == nil {
		t.Fatal("Co-GM should see private_messages")
	}
	core.AssertEqual(t, int64(1), *resp.PrivateMessages, "Co-GM should see 1 private message")
}

// TestGetGameCharacterStats_ReturnsStatsForWholeRoster verifies the batch endpoint
// returns one entry per character in a single response, including characters with
// zero messages, replacing what would otherwise be N individual /stats requests.
func TestGetGameCharacterStats_ReturnsStatsForWholeRoster(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "private_messages", "conversations", "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameStatsTestRouter(app, testDB)

	gmUser := testDB.CreateTestUser(t, "gm_gstats1", "gm_gstats1@example.com")
	game := testDB.CreateTestGame(t, int32(gmUser.ID), "Game Stats Test Game")

	charService := &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	active, err := charService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &[]int32{int32(gmUser.ID)}[0],
		Name:          "Active Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Creating active character should succeed")

	silent, err := charService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &[]int32{int32(gmUser.ID)}[0],
		Name:          "Silent Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Creating silent character should succeed")

	testDB.CreateTestPost(t, game.ID, int32(gmUser.ID), active.ID, "Post 1")
	testDB.CreateTestPost(t, game.ID, int32(gmUser.ID), active.ID, "Post 2")

	gmToken, err := core.CreateTestJWTTokenForUser(app, gmUser)
	core.AssertNoError(t, err, "GM token creation should succeed")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/games/%d/characters/stats", game.ID), nil)
	req.Header.Set("Authorization", "Bearer "+gmToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200")

	var resp map[string]CharacterStatsResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	core.AssertNoError(t, err, "Response should be valid JSON")

	core.AssertEqual(t, 2, len(resp), "Should return stats for both characters")

	activeStats, ok := resp[fmt.Sprintf("%d", active.ID)]
	if !ok {
		t.Fatalf("Expected stats entry for active character %d", active.ID)
	}
	core.AssertEqual(t, int64(2), activeStats.PublicMessages, "Active character should have 2 public messages")

	silentStats, ok := resp[fmt.Sprintf("%d", silent.ID)]
	if !ok {
		t.Fatalf("Expected stats entry for silent character %d (zero-message characters must still appear)", silent.ID)
	}
	core.AssertEqual(t, int64(0), silentStats.PublicMessages, "Silent character should have 0 public messages")
}

// TestGetGameCharacterStats_PrivateCountHiddenFromOtherPlayer verifies the batch
// endpoint applies the same per-character private-message visibility rule as the
// single-character endpoint: another active player cannot see it.
func TestGetGameCharacterStats_PrivateCountHiddenFromOtherPlayer(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "private_messages", "conversations", "messages", "characters", "game_participants", "games", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameStatsTestRouter(app, testDB)

	gmUser := testDB.CreateTestUser(t, "gm_gstats2", "gm_gstats2@example.com")
	ownerUser := testDB.CreateTestUser(t, "owner_gstats2", "owner_gstats2@example.com")
	otherPlayer := testDB.CreateTestUser(t, "other_gstats2", "other_gstats2@example.com")
	game := testDB.CreateTestGame(t, int32(gmUser.ID), "Game Stats Test Game 2")

	gameService := &services.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(ownerUser.ID), "player")
	core.AssertNoError(t, err, "Adding owner to game should succeed")
	_, err = gameService.AddGameParticipant(context.Background(), game.ID, int32(otherPlayer.ID), "player")
	core.AssertNoError(t, err, "Adding other player to game should succeed")

	charService := &services.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	character, err := charService.CreateCharacter(context.Background(), services.CreateCharacterRequest{
		GameID:        game.ID,
		UserID:        &[]int32{int32(ownerUser.ID)}[0],
		Name:          "Owner Character",
		CharacterType: "player_character",
	})
	core.AssertNoError(t, err, "Creating character should succeed")

	insertPrivateMessage(t, testDB, character.ID, int32(ownerUser.ID), game.ID)

	otherToken, err := core.CreateTestJWTTokenForUser(app, otherPlayer)
	core.AssertNoError(t, err, "Other player token creation should succeed")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/games/%d/characters/stats", game.ID), nil)
	req.Header.Set("Authorization", "Bearer "+otherToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200")

	var resp map[string]CharacterStatsResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	core.AssertNoError(t, err, "Response should be valid JSON")

	stats, ok := resp[fmt.Sprintf("%d", character.ID)]
	if !ok {
		t.Fatalf("Expected stats entry for character %d", character.ID)
	}
	if stats.PrivateMessages != nil {
		t.Errorf("Other player in active game should not see private_messages, got %v", *stats.PrivateMessages)
	}
}
