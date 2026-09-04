package games

import (
	"actionphase/pkg/core"
	db "actionphase/pkg/db/services"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type lootTableResponse struct {
	ID        int32  `json:"id"`
	GameID    int32  `json:"game_id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type lootTableContentResponse struct {
	ID   int32  `json:"id"`
	Name string `json:"name"`
	Data string `json:"data"`
}

func TestGetGameLootTablesAndCreateLootTable(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "game_loot_table_contents", "game_loot_tables", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_loot_table_contents", "game_loot_tables", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	accessToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/games/%d/loot-tables", fixtures.TestGame.ID), nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200 OK")

	var emptyTables []lootTableResponse
	err = json.NewDecoder(w.Body).Decode(&emptyTables)
	core.AssertNoError(t, err, "Should decode empty loot tables response")
	core.AssertEqual(t, 0, len(emptyTables), "Should start without loot tables")

	payload := map[string]any{
		"name": "Treasure Chest",
		"items": []map[string]any{
			{"name": "Gold Coins", "data": "{\"value\":100}"},
		},
	}
	body, err := json.Marshal(payload)
	core.AssertNoError(t, err, "Should marshal loot table request")

	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/games/%d/loot-tables", fixtures.TestGame.ID), bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should create loot table successfully")

	var created lootTableResponse
	err = json.NewDecoder(w.Body).Decode(&created)
	core.AssertNoError(t, err, "Should decode created loot table response")
	core.AssertEqual(t, "Treasure Chest", created.Name, "Created loot table name should match")
	core.AssertEqual(t, fixtures.TestGame.ID, created.GameID, "Created loot table game_id should match")

	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/games/%d/loot-tables", fixtures.TestGame.ID), nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200 OK after creating loot table")

	var tables []lootTableResponse
	err = json.NewDecoder(w.Body).Decode(&tables)
	core.AssertNoError(t, err, "Should decode loot tables response")
	core.AssertEqual(t, 1, len(tables), "Should return one loot table")
	core.AssertEqual(t, "Treasure Chest", tables[0].Name, "Loot table name should match created table")
}

func TestUpdateDeleteLootTable(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "game_loot_table_contents", "game_loot_tables", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_loot_table_contents", "game_loot_tables", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	accessToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	lootTable, err := gameService.CreateLootTable(context.Background(), int32(fixtures.TestGame.ID), "Initial Table")
	core.AssertNoError(t, err, "Should create loot table with service")

	reqBody := map[string]any{"name": "Renamed Table"}
	body, err := json.Marshal(reqBody)
	core.AssertNoError(t, err, "Should marshal update request")

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/games/%d/loot-tables/%d", fixtures.TestGame.ID, lootTable.ID), bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should update loot table successfully")

	var updated lootTableResponse
	err = json.NewDecoder(w.Body).Decode(&updated)
	core.AssertNoError(t, err, "Should decode updated loot table response")
	core.AssertEqual(t, "Renamed Table", updated.Name, "Loot table should be renamed")

	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/games/%d/loot-tables/%d", fixtures.TestGame.ID, lootTable.ID), nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should delete loot table successfully")

	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/games/%d/loot-tables", fixtures.TestGame.ID), nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should return 200 OK after deletion")

	var tables []lootTableResponse
	err = json.NewDecoder(w.Body).Decode(&tables)
	core.AssertNoError(t, err, "Should decode loot tables response after deletion")
	core.AssertEqual(t, 0, len(tables), "Deleted loot table should no longer appear")
}

func TestGetAndUpdateLootTableContents(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "game_loot_table_contents", "game_loot_tables", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_loot_table_contents", "game_loot_tables", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	accessToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	payload := map[string]any{
		"name": "Loot Stash",
		"items": []map[string]any{
			{"name": "Potion", "data": "{\"effect\":\"heal\"}"},
			{"name": "Scroll", "data": "{\"spell\":\"fireball\"}"},
		},
	}
	body, err := json.Marshal(payload)
	core.AssertNoError(t, err, "Should marshal loot table request")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/games/%d/loot-tables", fixtures.TestGame.ID), bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should create loot table successfully")

	var created lootTableResponse
	err = json.NewDecoder(w.Body).Decode(&created)
	core.AssertNoError(t, err, "Should decode created loot table response")

	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/games/%d/loot-tables/%d/contents", fixtures.TestGame.ID, created.ID), nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should fetch loot table contents successfully")

	var contents []lootTableContentResponse
	err = json.NewDecoder(w.Body).Decode(&contents)
	core.AssertNoError(t, err, "Should decode loot table contents response")
	core.AssertEqual(t, 2, len(contents), "Should return both initial loot items")

	updatePayload := map[string]any{
		"items": []map[string]any{
			{"name": "Magic Ring", "data": "{\"power\":\"+1\"}"},
		},
	}
	body, err = json.Marshal(updatePayload)
	core.AssertNoError(t, err, "Should marshal loot contents update request")

	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/games/%d/loot-tables/%d/contents", fixtures.TestGame.ID, created.ID), bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should update loot table contents successfully")

	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/games/%d/loot-tables/%d/contents", fixtures.TestGame.ID, created.ID), nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should fetch updated loot contents successfully")

	contents = nil
	err = json.NewDecoder(w.Body).Decode(&contents)
	core.AssertNoError(t, err, "Should decode updated loot table contents response")
	core.AssertEqual(t, 1, len(contents), "Should return only updated loot item")
	core.AssertEqual(t, "Magic Ring", contents[0].Name, "Updated loot contents should replace existing items")
}

func TestGetLootTablesFilterEmptyTable(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "character_data", "characters", "game_loot_table_contents", "game_loot_tables", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "character_data", "characters", "game_loot_table_contents", "game_loot_tables", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	accessToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	// Create a loot table with no items at all — the create endpoint permits this.
	payload := map[string]any{"name": "Empty Cache"}
	body, err := json.Marshal(payload)
	core.AssertNoError(t, err, "Should marshal loot table request")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/games/%d/loot-tables", fixtures.TestGame.ID), bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should create the empty loot table")

	var created lootTableResponse
	err = json.NewDecoder(w.Body).Decode(&created)
	core.AssertNoError(t, err, "Should decode created loot table response")

	// Try to get loot tables filtering out empty tables. Should return no loot tables.
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/games/%d/loot-tables?exclude-empty=true", fixtures.TestGame.ID), nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var emptyTables []lootTableResponse
	err = json.NewDecoder(w.Body).Decode(&emptyTables)
	core.AssertNoError(t, err, "Should decode empty loot tables response")
	core.AssertEqual(t, 0, len(emptyTables), "Should not return empty tables")

	// Now we add one item and try the request again. It should return the previously created loot table.
	updatePayload := map[string]any{
		"items": []map[string]any{
			{"name": "Magic Ring", "data": "{\"power\":\"+1\"}"},
		},
	}
	body, err = json.Marshal(updatePayload)
	core.AssertNoError(t, err, "Should marshal loot contents update request")

	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/games/%d/loot-tables/%d/contents", fixtures.TestGame.ID, created.ID), bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should update loot table contents successfully")

	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/games/%d/loot-tables", fixtures.TestGame.ID), nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var tables []lootTableResponse
	err = json.NewDecoder(w.Body).Decode(&tables)
	core.AssertNoError(t, err, "Should decode loot tables response")
	core.AssertEqual(t, 1, len(tables), "Should return one loot table")
	core.AssertEqual(t, created.ID, tables[0].ID, "Loot table ID should match created table")
}

// Regression: rolling on a loot table with no contents used to reach
// rand.Intn(0), which panics ("invalid argument to Intn"). The panic was caught
// by the recovery middleware and surfaced as an opaque 500. An empty table is
// reachable through normal use — the create endpoint accepts a table with no
// items — so this must be a client error with an actionable message, not a 500.
func TestSetRandomLootForCharacterEmptyTable(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "character_data", "characters", "game_loot_table_contents", "game_loot_tables", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "character_data", "characters", "game_loot_table_contents", "game_loot_tables", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	accessToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	npc, err := characterService.CreateCharacter(context.Background(), core.CreateCharacterRequest{
		GameID:        int32(fixtures.TestGame.ID),
		CharacterType: "npc",
		Name:          "Empty Table Recipient",
	})
	core.AssertNoError(t, err, "Should create NPC character for loot assignment")

	// Create a loot table with no items at all — the create endpoint permits this.
	payload := map[string]any{"name": "Empty Cache"}
	body, err := json.Marshal(payload)
	core.AssertNoError(t, err, "Should marshal loot table request")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/games/%d/loot-tables", fixtures.TestGame.ID), bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should create the empty loot table")

	var created lootTableResponse
	err = json.NewDecoder(w.Body).Decode(&created)
	core.AssertNoError(t, err, "Should decode created loot table response")

	// Confirm the table really has no contents, so the roll below is the empty case.
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/games/%d/loot-tables/%d/contents", fixtures.TestGame.ID, created.ID), nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var contents []lootTableContentResponse
	err = json.NewDecoder(w.Body).Decode(&contents)
	core.AssertNoError(t, err, "Should decode loot table contents response")
	core.AssertEqual(t, 0, len(contents), "Loot table should be empty for this test")

	// Roll on the empty table: must be a 4xx client error, never a 500 panic.
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/games/%d/loot-tables/%d/random/%d", fixtures.TestGame.ID, created.ID, npc.ID), nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusBadRequest, w.Code, "Rolling on an empty loot table should return 400, not panic with a 500")

	// The GM needs to know *why* it failed, so the message must name the cause.
	core.AssertTrue(t, strings.Contains(strings.ToLower(w.Body.String()), "empty"),
		"Error message should explain that the loot table is empty, got: "+w.Body.String())

	// Nothing should have been granted to the character.
	characterData, err := characterService.GetCharacterData(context.Background(), npc.ID)
	core.AssertNoError(t, err, "Should fetch character data after failed loot assignment")
	core.AssertEqual(t, 0, len(characterData), "No inventory data should be written when the roll fails")
}

func TestSetRandomLootForCharacter(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "character_data", "characters", "game_loot_table_contents", "game_loot_tables", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "character_data", "characters", "game_loot_table_contents", "game_loot_tables", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	accessToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	npc, err := characterService.CreateCharacter(context.Background(), core.CreateCharacterRequest{
		GameID:        int32(fixtures.TestGame.ID),
		CharacterType: "npc",
		Name:          "Loot Recipient",
	})
	core.AssertNoError(t, err, "Should create NPC character for loot assignment")

	payload := map[string]any{
		"name": "Random Cache",
		"items": []map[string]any{
			{"name": "Emerald", "data": "{\"value\":200}"},
			{"name": "Sapphire", "data": "{\"value\":150}"},
		},
	}
	body, err := json.Marshal(payload)
	core.AssertNoError(t, err, "Should marshal loot table request")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/games/%d/loot-tables", fixtures.TestGame.ID), bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should create loot table successfully")

	var created lootTableResponse
	err = json.NewDecoder(w.Body).Decode(&created)
	core.AssertNoError(t, err, "Should decode created loot table response")

	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/games/%d/loot-tables/%d/random/%d", fixtures.TestGame.ID, created.ID, npc.ID), nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Should assign random loot successfully")

	var assigned lootTableContentResponse
	err = json.NewDecoder(w.Body).Decode(&assigned)
	core.AssertNoError(t, err, "Should decode assigned loot response")
	core.AssertNotEqual(t, int32(0), assigned.ID, "Assigned loot item should have an ID")
	core.AssertNotEqual(t, "", assigned.Name, "Assigned loot item should have a name")

	characterData, err := characterService.GetCharacterData(context.Background(), npc.ID)
	core.AssertNoError(t, err, "Should fetch character data after loot assignment")
	core.AssertEqual(t, 1, len(characterData), "Character should have one inventory data entry")
	core.AssertEqual(t, "items", characterData[0].FieldName, "Inventory field name should be items")
	core.AssertEqual(t, "inventory", characterData[0].ModuleType, "Inventory module type should be inventory")
	core.AssertEqual(t, false, characterData[0].IsPublic.Bool, "Loot inventory data should be private")
	core.AssertEqual(t, true, strings.Contains(characterData[0].FieldValue.String, assigned.Data), "Character data should include assigned item data")
}

// Loot tables are GM-only: they hold unrevealed rewards, so a player or audience
// member seeing (or editing) them leaks upcoming game content. Every endpoint must
// enforce this, not just the read paths — a missing check here fails silently,
// since nothing in the UI would reveal that a non-GM could reach the data.
func TestLootTableEndpointsRejectNonGM(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "character_data", "characters", "game_loot_table_contents", "game_loot_tables", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "character_data", "characters", "game_loot_table_contents", "game_loot_tables", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	// A second user who is not the GM of fixtures.TestGame.
	outsider := testDB.CreateTestUser(t, "outsider", "outsider@example.com")
	outsiderToken, err := core.CreateTestJWTTokenForUser(app, outsider)
	core.AssertNoError(t, err, "Outsider token creation should succeed")

	gmToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "GM token creation should succeed")

	// Seed a real loot table as the GM so the non-GM requests target a table that
	// genuinely exists — otherwise a 4xx could just mean "not found".
	payload := map[string]any{
		"name":  "Secret Stash",
		"items": []map[string]any{{"name": "Artifact", "data": `{"name":"Artifact"}`}},
	}
	body, err := json.Marshal(payload)
	core.AssertNoError(t, err, "Should marshal loot table request")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/games/%d/loot-tables", fixtures.TestGame.ID), bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+gmToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	core.AssertEqual(t, http.StatusOK, w.Code, "GM should create the loot table")

	var created lootTableResponse
	err = json.NewDecoder(w.Body).Decode(&created)
	core.AssertNoError(t, err, "Should decode created loot table response")

	base := fmt.Sprintf("/api/v1/games/%d/loot-tables", fixtures.TestGame.ID)
	itemsBody := `{"name":"Renamed","items":[{"name":"x","data":"{}"}]}`

	testCases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"list loot tables", http.MethodGet, base, ""},
		{"create loot table", http.MethodPost, base, itemsBody},
		{"update loot table", http.MethodPut, fmt.Sprintf("%s/%d", base, created.ID), itemsBody},
		{"delete loot table", http.MethodDelete, fmt.Sprintf("%s/%d", base, created.ID), ""},
		{"get loot table contents", http.MethodGet, fmt.Sprintf("%s/%d/contents", base, created.ID), ""},
		{"replace loot table contents", http.MethodPost, fmt.Sprintf("%s/%d/contents", base, created.ID), itemsBody},
	}

	for _, tc := range testCases {
		t.Run(tc.name+" is forbidden for a non-GM", func(t *testing.T) {
			var reqBody *bytes.Buffer
			if tc.body != "" {
				reqBody = bytes.NewBufferString(tc.body)
			} else {
				reqBody = bytes.NewBuffer(nil)
			}
			req := httptest.NewRequest(tc.method, tc.path, reqBody)
			req.Header.Set("Authorization", "Bearer "+outsiderToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			core.AssertEqual(t, http.StatusForbidden, w.Code,
				"non-GM should get 403, got "+w.Body.String())
		})
	}

	// The loot table must still be intact: none of the rejected calls may have
	// mutated anything on their way to the permission check.
	req = httptest.NewRequest(http.MethodGet, base, nil)
	req.Header.Set("Authorization", "Bearer "+gmToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var tables []lootTableResponse
	err = json.NewDecoder(w.Body).Decode(&tables)
	core.AssertNoError(t, err, "Should decode loot tables response")
	core.AssertEqual(t, 1, len(tables), "Loot table should survive the rejected non-GM requests")
	core.AssertEqual(t, "Secret Stash", tables[0].Name, "Loot table name should be unchanged")
}

// The update/delete/contents queries are keyed on the loot table ID alone and are
// not game-scoped, so ownership rests entirely on the handler's IsLootTableInGame
// check. A GM of one game must not be able to reach another game's loot tables by
// passing their ID.
func TestLootTableEndpointsRejectCrossGameAccess(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "character_data", "characters", "game_loot_table_contents", "game_loot_tables", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "character_data", "characters", "game_loot_table_contents", "game_loot_tables", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	victimToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "Victim GM token creation should succeed")

	// A second GM with their own game.
	attacker := testDB.CreateTestUser(t, "othergm", "othergm@example.com")
	attackerGame := testDB.CreateTestGame(t, int32(attacker.ID), "Attacker Game")
	attackerToken, err := core.CreateTestJWTTokenForUser(app, attacker)
	core.AssertNoError(t, err, "Attacker GM token creation should succeed")

	// Victim creates a loot table in their own game.
	payload := map[string]any{
		"name":  "Victim Stash",
		"items": []map[string]any{{"name": "Crown", "data": `{"name":"Crown"}`}},
	}
	body, err := json.Marshal(payload)
	core.AssertNoError(t, err, "Should marshal loot table request")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/games/%d/loot-tables", fixtures.TestGame.ID), bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+victimToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	core.AssertEqual(t, http.StatusOK, w.Code, "Victim GM should create their loot table")

	var victimTable lootTableResponse
	err = json.NewDecoder(w.Body).Decode(&victimTable)
	core.AssertNoError(t, err, "Should decode created loot table response")

	// Attacker addresses the victim's table ID through their OWN game's route,
	// where they legitimately hold GM rights.
	attackerBase := fmt.Sprintf("/api/v1/games/%d/loot-tables/%d", attackerGame.ID, victimTable.ID)
	itemsBody := `{"name":"Hijacked","items":[{"name":"x","data":"{}"}]}`

	testCases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"update", http.MethodPut, attackerBase, itemsBody},
		{"delete", http.MethodDelete, attackerBase, ""},
		{"read contents", http.MethodGet, attackerBase + "/contents", ""},
		{"replace contents", http.MethodPost, attackerBase + "/contents", itemsBody},
	}

	for _, tc := range testCases {
		t.Run(tc.name+" of another game's loot table is forbidden", func(t *testing.T) {
			var reqBody *bytes.Buffer
			if tc.body != "" {
				reqBody = bytes.NewBufferString(tc.body)
			} else {
				reqBody = bytes.NewBuffer(nil)
			}
			req := httptest.NewRequest(tc.method, tc.path, reqBody)
			req.Header.Set("Authorization", "Bearer "+attackerToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			core.AssertEqual(t, http.StatusForbidden, w.Code,
				"cross-game access should get 403, got "+w.Body.String())
		})
	}

	// The victim's table and its contents must be untouched.
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/games/%d/loot-tables/%d/contents", fixtures.TestGame.ID, victimTable.ID), nil)
	req.Header.Set("Authorization", "Bearer "+victimToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	core.AssertEqual(t, http.StatusOK, w.Code, "Victim should still read their own loot table")

	var contents []lootTableContentResponse
	err = json.NewDecoder(w.Body).Decode(&contents)
	core.AssertNoError(t, err, "Should decode loot table contents response")
	core.AssertEqual(t, 1, len(contents), "Victim's loot contents should be unchanged")
	core.AssertEqual(t, "Crown", contents[0].Name, "Victim's loot item should be unchanged")
}

// The roll endpoint is the one that actually grants an item, so it carries three
// guards: GM-only, the caller must be able to edit the target character, and the
// loot table must belong to this game. Cover the two authorization ones here —
// the empty-table case is covered by TestSetRandomLootForCharacterEmptyTable.
func TestSetRandomLootForCharacterAuthorization(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "character_data", "characters", "game_loot_table_contents", "game_loot_tables", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "character_data", "characters", "game_loot_table_contents", "game_loot_tables", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	gmToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "GM token creation should succeed")

	outsider := testDB.CreateTestUser(t, "rolloutsider", "rolloutsider@example.com")
	outsiderToken, err := core.CreateTestJWTTokenForUser(app, outsider)
	core.AssertNoError(t, err, "Outsider token creation should succeed")

	characterService := &db.CharacterService{DB: testDB.Pool, Logger: app.ObsLogger}
	npc, err := characterService.CreateCharacter(context.Background(), core.CreateCharacterRequest{
		GameID:        int32(fixtures.TestGame.ID),
		CharacterType: "npc",
		Name:          "Roll Target",
	})
	core.AssertNoError(t, err, "Should create NPC character")

	payload := map[string]any{
		"name":  "Rollable",
		"items": []map[string]any{{"name": "Gem", "data": `{"name":"Gem"}`}},
	}
	body, err := json.Marshal(payload)
	core.AssertNoError(t, err, "Should marshal loot table request")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/games/%d/loot-tables", fixtures.TestGame.ID), bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+gmToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	core.AssertEqual(t, http.StatusOK, w.Code, "GM should create the loot table")

	var created lootTableResponse
	err = json.NewDecoder(w.Body).Decode(&created)
	core.AssertNoError(t, err, "Should decode created loot table response")

	rollPath := fmt.Sprintf("/api/v1/games/%d/loot-tables/%d/random/%d", fixtures.TestGame.ID, created.ID, npc.ID)

	t.Run("non-GM cannot roll loot onto a character", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, rollPath, bytes.NewBuffer(nil))
		req.Header.Set("Authorization", "Bearer "+outsiderToken)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusForbidden, w.Code, "non-GM roll should be forbidden, got "+w.Body.String())

		// Nothing may have been granted.
		data, err := characterService.GetCharacterData(context.Background(), npc.ID)
		core.AssertNoError(t, err, "Should fetch character data")
		core.AssertEqual(t, 0, len(data), "No loot should be granted on a rejected roll")
	})

	t.Run("GM cannot roll from another game's loot table", func(t *testing.T) {
		attacker := testDB.CreateTestUser(t, "rollattacker", "rollattacker@example.com")
		attackerGame := testDB.CreateTestGame(t, int32(attacker.ID), "Roll Attacker Game")
		attackerToken, err := core.CreateTestJWTTokenForUser(app, attacker)
		core.AssertNoError(t, err, "Attacker token creation should succeed")

		attackerNPC, err := characterService.CreateCharacter(context.Background(), core.CreateCharacterRequest{
			GameID:        attackerGame.ID,
			CharacterType: "npc",
			Name:          "Attacker NPC",
		})
		core.AssertNoError(t, err, "Should create attacker's NPC")

		// Attacker rolls their own game's character against the victim's table ID.
		path := fmt.Sprintf("/api/v1/games/%d/loot-tables/%d/random/%d", attackerGame.ID, created.ID, attackerNPC.ID)
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBuffer(nil))
		req.Header.Set("Authorization", "Bearer "+attackerToken)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		core.AssertEqual(t, http.StatusForbidden, w.Code, "cross-game roll should be forbidden, got "+w.Body.String())

		data, err := characterService.GetCharacterData(context.Background(), attackerNPC.ID)
		core.AssertNoError(t, err, "Should fetch attacker NPC data")
		core.AssertEqual(t, 0, len(data), "No loot should be granted on a cross-game roll")
	})
}

// The `validate:"required"` struct tags on the loot request types are not executed
// by anything (Bind is the only hook, and most Bind methods in this codebase return
// a bare nil — see .claude/planning/request-validation.md). Without explicit checks
// the API happily stores unnamed loot tables and items with empty data, and that
// data string is what the frontend JSON.parses when granting the item.
func TestLootTableRequestValidation(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "game_loot_table_contents", "game_loot_tables", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_loot_table_contents", "game_loot_tables", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	accessToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	base := fmt.Sprintf("/api/v1/games/%d/loot-tables", fixtures.TestGame.ID)

	post := func(t *testing.T, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	rejected := []struct {
		name string
		body string
	}{
		{"blank table name", `{"name":"","items":[{"name":"x","data":"{}"}]}`},
		{"whitespace-only table name", `{"name":"   ","items":[{"name":"x","data":"{}"}]}`},
		{"omitted table name", `{"items":[{"name":"x","data":"{}"}]}`},
		{"blank item name", `{"name":"T","items":[{"name":"","data":"{}"}]}`},
		{"blank item data", `{"name":"T","items":[{"name":"x","data":""}]}`},
		{"item data that is not JSON", `{"name":"T","items":[{"name":"x","data":"not json"}]}`},
	}

	for _, tc := range rejected {
		t.Run("create rejects "+tc.name, func(t *testing.T) {
			w := post(t, base, tc.body)
			core.AssertEqual(t, http.StatusUnprocessableEntity, w.Code,
				"should be rejected with 400, got "+w.Body.String())
		})
	}

	t.Run("create still accepts a valid request", func(t *testing.T) {
		w := post(t, base, `{"name":"Valid Table","items":[{"name":"Sword","data":"{\"name\":\"Sword\"}"}]}`)
		core.AssertEqual(t, http.StatusOK, w.Code, "valid request should succeed, got "+w.Body.String())
	})

	t.Run("a table with no items is still allowed at the API layer", func(t *testing.T) {
		// Deliberate: the roll endpoint rejects empty tables (see
		// TestSetRandomLootForCharacterEmptyTable) and the UI blocks creating one,
		// but a GM may legitimately save a table and fill it in later.
		w := post(t, base, `{"name":"Empty For Now"}`)
		core.AssertEqual(t, http.StatusOK, w.Code, "a named table with no items should be allowed")
	})

	t.Run("contents update rejects invalid items", func(t *testing.T) {
		w := post(t, base, `{"name":"Contents Target","items":[{"name":"x","data":"{}"}]}`)
		core.AssertEqual(t, http.StatusOK, w.Code, "setup table should be created")

		var created lootTableResponse
		err := json.NewDecoder(w.Body).Decode(&created)
		core.AssertNoError(t, err, "Should decode created loot table response")

		contentsPath := fmt.Sprintf("%s/%d/contents", base, created.ID)
		w = post(t, contentsPath, `{"items":[{"name":"","data":"{}"}]}`)
		core.AssertEqual(t, http.StatusUnprocessableEntity, w.Code,
			"blank item name should be rejected, got "+w.Body.String())

		// The existing contents must survive a rejected update: the handler deletes
		// all contents before re-inserting, so a validation failure that slipped
		// through would wipe the table.
		req := httptest.NewRequest(http.MethodGet, contentsPath, nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var contents []lootTableContentResponse
		err = json.NewDecoder(w.Body).Decode(&contents)
		core.AssertNoError(t, err, "Should decode contents response")
		core.AssertEqual(t, 1, len(contents), "Original contents should survive a rejected update")
	})
}

// Regression: loot table contents live in a child table, so rewriting them left
// the parent row untouched and updated_at stale. A GM who edited a table's items
// through the UI saw no "Updated" date, because only renames moved the timestamp
// — and item edits are the common operation.
//
// Also pins that the list endpoint returns updated_at at all: it hand-builds its
// response map, so a field can silently go missing there while the create
// endpoint (which returns the model directly) still has it.
func TestUpdateLootTableContentsBumpsUpdatedAt(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	testDB.CleanupTables(t, "game_loot_table_contents", "game_loot_tables", "games", "sessions", "users")
	defer testDB.CleanupTables(t, "game_loot_table_contents", "game_loot_tables", "games", "sessions", "users")

	app := core.NewTestApp(testDB.Pool)
	router := setupGameTestRouter(app, testDB)
	fixtures := testDB.SetupFixtures(t)

	accessToken, err := core.CreateTestJWTTokenForUser(app, fixtures.TestUser)
	core.AssertNoError(t, err, "Test token creation should succeed")

	payload := map[string]any{
		"name":  "Timestamp Table",
		"items": []map[string]any{{"name": "Potion", "data": "{\"effect\":\"heal\"}"}},
	}
	body, err := json.Marshal(payload)
	core.AssertNoError(t, err, "Should marshal loot table request")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/games/%d/loot-tables", fixtures.TestGame.ID), bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	core.AssertEqual(t, http.StatusOK, w.Code, "Should create loot table")

	var created lootTableResponse
	err = json.NewDecoder(w.Body).Decode(&created)
	core.AssertNoError(t, err, "Should decode created loot table")

	// A freshly created table has both timestamps from the same NOW(), which is
	// what lets the UI hide a redundant "Updated" date.
	core.AssertEqual(t, created.CreatedAt, created.UpdatedAt,
		"A newly created loot table should have updated_at equal to created_at")

	// Rewrite the contents — the operation that used to leave updated_at stale.
	contentsPayload := map[string]any{
		"items": []map[string]any{
			{"name": "Potion", "data": "{\"effect\":\"heal\"}"},
			{"name": "Elixir", "data": "{\"effect\":\"restore\"}"},
		},
	}
	body, err = json.Marshal(contentsPayload)
	core.AssertNoError(t, err, "Should marshal contents request")

	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/games/%d/loot-tables/%d/contents", fixtures.TestGame.ID, created.ID), bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	core.AssertEqual(t, http.StatusOK, w.Code, "Should update loot table contents")

	// Read the table back through the list endpoint the UI actually uses.
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/games/%d/loot-tables", fixtures.TestGame.ID), nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var listed []lootTableResponse
	err = json.NewDecoder(w.Body).Decode(&listed)
	core.AssertNoError(t, err, "Should decode loot table list")
	core.AssertEqual(t, 1, len(listed), "Should list the one loot table")

	core.AssertTrue(t, listed[0].UpdatedAt != "",
		"The list endpoint must return updated_at, not omit it from its hand-built response map")
	core.AssertTrue(t, listed[0].UpdatedAt > listed[0].CreatedAt,
		"Editing contents should bump updated_at past created_at, got created="+listed[0].CreatedAt+" updated="+listed[0].UpdatedAt)
}
