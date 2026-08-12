package games

import (
	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) GetGameLootTables(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	defer h.App.ObsLogger.LogOperation(ctx, "api_loot_tables")()

	gameID := ctx.Value("gameID").(int32)

	if !ctx.Value("is_gm").(bool) {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can see and edit loot tables"), "Loot tables access forbidden")
		return
	}

	gameService := h.GameService

	lootTables, err := gameService.GetGameLootTables(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game loot tables", "error", err, "game_id", gameID)
		return
	}

	// Convert to response format
	// Initialize as empty slice to ensure JSON encodes as [] not null
	response := make([]map[string]interface{}, 0)
	for _, lootTable := range lootTables {
		lootTableData := map[string]interface{}{
			"id":         lootTable.ID,
			"game_id":    lootTable.GameID,
			"name":       lootTable.Name,
			"created_at": lootTable.CreatedAt.Time,
		}
		response = append(response, lootTableData)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) AddGameLootTable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	defer h.App.ObsLogger.LogOperation(ctx, "api_loot_tables_add")()

	gameID := ctx.Value("gameID").(int32)
	if !ctx.Value("is_gm").(bool) {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can see and edit loot tables"), "Loot tables access forbidden")
		return
	}

	data := &UpdateLootTableRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid update loot table request", "error", err)
		return
	}

	gameService := h.GameService
	newLootTable, err := gameService.CreateLootTable(ctx, int32(gameID), data.Name)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to create loot table", "error", err, "game_id", gameID)
		return
	}

	for _, item := range data.Items {
		_, err := gameService.AddLootTableContent(ctx, newLootTable.ID, item.Name, item.Data)
		if err != nil {
			h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to save loot table items", "error", err, "game_id", gameID)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newLootTable)
}

func (h *Handler) UpdateGameLootTable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	defer h.App.ObsLogger.LogOperation(ctx, "api_loot_tables_update")()

	gameID := ctx.Value("gameID").(int32)
	if !ctx.Value("is_gm").(bool) {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can see and edit loot tables"), "Loot tables access forbidden")
		return
	}

	tableIDStr := chi.URLParam(r, "tableId")
	tableID, err := strconv.ParseInt(tableIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid table ID")), "Invalid loot table contents request")
		return
	}

	data := &UpdateLootTableRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid update loot table request", "error", err)
		return
	}

	gameService := h.GameService
	isLootTableInGame, err := gameService.IsLootTableInGame(ctx, int32(tableID), int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check loot table ownership", "error", err, "table_id", tableID, "game_id", gameID)
		return
	}
	if !isLootTableInGame {
		h.renderError(ctx, w, r, core.ErrForbidden("loot table does not belong to this game"), "Loot table access forbidden")
		return
	}

	lootTable, err := gameService.UpdateLootTable(ctx, int32(tableID), data.Name)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to update loot table", "error", err, "table_id", tableID)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lootTable)
}

func (h *Handler) DeleteGameLootTable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	defer h.App.ObsLogger.LogOperation(ctx, "api_loot_tables_delete")()

	gameID := ctx.Value("gameID").(int32)
	if !ctx.Value("is_gm").(bool) {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can see and edit loot tables"), "Loot tables access forbidden")
		return
	}

	tableIDStr := chi.URLParam(r, "tableId")
	tableID, err := strconv.ParseInt(tableIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid table ID")), "Invalid loot table contents request")
		return
	}

	gameService := h.GameService

	isLootTableInGame, err := gameService.IsLootTableInGame(ctx, int32(tableID), int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check loot table ownership", "error", err, "table_id", tableID, "game_id", gameID)
		return
	}
	if !isLootTableInGame {
		h.renderError(ctx, w, r, core.ErrForbidden("loot table does not belong to this game"), "Loot table access forbidden")
		return
	}

	err = gameService.DeleteLootTable(ctx, int32(tableID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to delete loot table", "error", err, "table_id", tableID)
		return
	}
}

func (h *Handler) GetGameLootTableContents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	defer h.App.ObsLogger.LogOperation(ctx, "api_loot_tables")()

	gameID := ctx.Value("gameID").(int32)
	if !ctx.Value("is_gm").(bool) {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can see and edit loot tables"), "Loot tables access forbidden")
		return
	}

	tableIDStr := chi.URLParam(r, "tableId")
	tableID, err := strconv.ParseInt(tableIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid table ID")), "Invalid loot table contents request")
		return
	}

	gameService := h.GameService

	isLootTableInGame, err := gameService.IsLootTableInGame(ctx, int32(tableID), int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check loot table ownership", "error", err, "table_id", tableID, "game_id", gameID)
		return
	}
	if !isLootTableInGame {
		h.renderError(ctx, w, r, core.ErrForbidden("loot table does not belong to this game"), "Loot table access forbidden")
		return
	}

	lootTables, err := gameService.GetGameLootTableContents(ctx, int32(tableID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get loot table contents", "error", err, "table_id", tableID)
		return
	}

	// Convert to response format
	// Initialize as empty slice to ensure JSON encodes as [] not null
	response := make([]map[string]interface{}, 0)
	for _, lootTable := range lootTables {
		lootTableData := map[string]interface{}{
			"id":   lootTable.ID,
			"name": lootTable.Name,
			"data": lootTable.Data.String,
		}
		response = append(response, lootTableData)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) UpdateGameLootTableContent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	defer h.App.ObsLogger.LogOperation(ctx, "api_loot_tables")()

	gameID := ctx.Value("gameID").(int32)
	if !ctx.Value("is_gm").(bool) {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can see and edit loot tables"), "Loot tables access forbidden")
		return
	}

	tableIDStr := chi.URLParam(r, "tableId")
	tableID, err := strconv.ParseInt(tableIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid table ID")), "Invalid loot table contents request")
		return
	}

	data := &UpdateLootTableContentsRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid update loot table content request", "error", err)
		return
	}

	gameService := h.GameService

	isLootTableInGame, err := gameService.IsLootTableInGame(ctx, int32(tableID), int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check loot table ownership", "error", err, "table_id", tableID, "game_id", gameID)
		return
	}
	if !isLootTableInGame {
		h.renderError(ctx, w, r, core.ErrForbidden("loot table does not belong to this game"), "Loot table access forbidden")
		return
	}
	err = gameService.DeleteLootTableContents(ctx, int32(tableID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to delete loot table contents", "error", err, "table_id", tableID)
		return
	}

	for _, item := range data.Items {
		_, err := gameService.AddLootTableContent(ctx, int32(tableID), item.Name, item.Data)
		if err != nil {
			h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to save loot table items", "error", err, "game_id", gameID)
			return
		}
	}
}

func (h *Handler) SetRandomLootForCharacter(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	defer h.App.ObsLogger.LogOperation(ctx, "api_loot_tables")()

	gameID := ctx.Value("gameID").(int32)
	if !ctx.Value("is_gm").(bool) {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can see and edit loot tables"), "Loot tables access forbidden")
		return
	}

	tableIDStr := chi.URLParam(r, "tableId")
	tableID, err := strconv.ParseInt(tableIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid table ID")), "Invalid loot table contents request")
		return
	}

	characterIDStr := chi.URLParam(r, "characterId")
	characterID, err := strconv.ParseInt(characterIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid character ID")), "Invalid request")
		return
	}

	gameService := h.GameService

	// Verify user can edit this character
	characterService := h.CharacterService
	user := core.GetAuthenticatedUser(ctx)
	canEdit, err := characterService.CanUserEditCharacter(ctx, int32(characterID), user.ID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check character edit permission", "error", err)
		return
	}

	if !canEdit {
		h.renderError(ctx, w, r, core.ErrForbidden("you cannot edit this character"), "Character edit permission denied", "character_id", characterID, "user_id", user.ID)
		return
	}

	isLootTableInGame, err := gameService.IsLootTableInGame(ctx, int32(tableID), int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check loot table ownership", "error", err, "table_id", tableID, "game_id", gameID)
		return
	}
	if !isLootTableInGame {
		h.renderError(ctx, w, r, core.ErrForbidden("loot table does not belong to this game"), "Loot table access forbidden")
		return
	}

	contents, err := gameService.GetGameLootTableContents(ctx, int32(tableID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get loot table contents", "error", err, "table_id", tableID)
		return
	}

	// A table with no items is reachable through normal use (the create endpoint
	// accepts one), and rand.Intn(0) panics. Fail as a client error the GM can act
	// on rather than letting the recovery middleware turn it into an opaque 500.
	if len(contents) == 0 {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("loot table is empty: add at least one item before rolling")), "Roll requested on empty loot table", "table_id", tableID, "game_id", gameID)
		return
	}

	character, err := characterService.GetCharacter(ctx, int32(characterID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get character", "error", err, "character_id", characterID)
		return
	}

	rnd := rand.Intn(len(contents))

	content := contents[rnd]

	err = characterService.AddToCharacterData(ctx, core.CharacterDataRequest{
		CharacterID: int32(characterID),
		ModuleType:  "inventory",
		FieldName:   "items",
		FieldValue:  content.Data.String,
		FieldType:   "json",
		IsPublic:    false,
	})
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to add loot to character", "error", err, "character_id", characterID, "loot_table_id", tableID)
		return
	}

	// The loot is already granted, so a failed log entry must not fail the request —
	// but it should not vanish silently either, since the game log is the GM's only
	// record of what was rolled.
	if _, err := gameService.AddGameLog(ctx, models.CreateLogParams{
		GameID:  gameID,
		Type:    "INVENTORY_ADD",
		Message: pgtype.Text{String: fmt.Sprintf("Added %s to Character %s (Rolled: %d)", content.Name, character.Name, rnd+1), Valid: true},
	}); err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to write loot roll game log",
			"game_id", gameID, "character_id", characterID, "loot_table_id", tableID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(content)
}
