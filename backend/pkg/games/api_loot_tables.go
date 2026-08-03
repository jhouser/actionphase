package games

import (
	"actionphase/pkg/core"
	db "actionphase/pkg/db/services"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) GetGameLootTables(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	defer h.App.ObsLogger.LogOperation(ctx, "api_loot_tables")()

	gameIDStr := chi.URLParam(r, "gameId")
	gameID, err := strconv.ParseInt(gameIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid game ID")), "Invalid loot tables request")
		return
	}

	// Get authenticated user
	user := core.GetAuthenticatedUser(ctx)
	if user == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user found")
		return
	}

	gameService := &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger}
	// Verify user is GM of this game
	game, err := gameService.GetGame(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game for permission check", "error", err, "game_id", gameID)
		return
	}

	if !core.IsUserGameMaster(r, user.ID, user.IsAdmin, *game, h.App.Pool) {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can see and edit loot tables"), "Loot tables access forbidden")
		return
	}

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

}

func (h *Handler) GetGameLootTableContents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	defer h.App.ObsLogger.LogOperation(ctx, "api_loot_tables")()

	gameIDStr := chi.URLParam(r, "gameId")
	gameID, err := strconv.ParseInt(gameIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid game ID")), "Invalid loot tables request")
		return
	}

	tableIDStr := chi.URLParam(r, "tableId")
	tableID, err := strconv.ParseInt(tableIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid table ID")), "Invalid loot table contents request")
		return
	}

	// Get authenticated user
	user := core.GetAuthenticatedUser(ctx)
	if user == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user found")
		return
	}

	gameService := &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger}
	// Verify user is GM of this game
	game, err := gameService.GetGame(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game for permission check", "error", err, "game_id", gameID)
		return
	}

	if !core.IsUserGameMaster(r, user.ID, user.IsAdmin, *game, h.App.Pool) {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can see and edit loot tables"), "Loot tables access forbidden")
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
