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
