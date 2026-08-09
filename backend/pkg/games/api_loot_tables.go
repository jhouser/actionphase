package games

import (
	"actionphase/pkg/core"
	db "actionphase/pkg/db/services"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func (h *Handler) GetGameLootTables(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	defer h.App.ObsLogger.LogOperation(ctx, "api_loot_tables")()

	gameID := ctx.Value("gameID").(int32)

	if !ctx.Value("is_gm").(bool) {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can see and edit loot tables"), "Loot tables access forbidden")
		return
	}

	gameService := &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger}

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

	gameService := &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger}
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

	gameService := &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger}

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

	gameService := &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger}

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

func (h *Handler) AddGameLootTableContent(w http.ResponseWriter, r *http.Request) {
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

	data := &UpdateLootTableContentRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid update loot table content request", "error", err)
		return
	}

	gameService := &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger}

	isLootTableInGame, err := gameService.IsLootTableInGame(ctx, int32(tableID), int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check loot table ownership", "error", err, "table_id", tableID, "game_id", gameID)
		return
	}
	if !isLootTableInGame {
		h.renderError(ctx, w, r, core.ErrForbidden("loot table does not belong to this game"), "Loot table access forbidden")
		return
	}

	_, err = gameService.AddLootTableContent(ctx, int32(tableID), data.Name, data.Data)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to add loot table content", "error", err, "table_id", tableID)
		return
	}
}

func (h *Handler) DeleteGameLootTableContent(w http.ResponseWriter, r *http.Request) {
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

	gameService := &db.GameService{DB: h.App.Pool, Logger: h.App.ObsLogger}

	isLootTableInGame, err := gameService.IsLootTableInGame(ctx, int32(tableID), int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check loot table ownership", "error", err, "table_id", tableID, "game_id", gameID)
		return
	}
	if !isLootTableInGame {
		h.renderError(ctx, w, r, core.ErrForbidden("loot table does not belong to this game"), "Loot table access forbidden")
		return
	}

}
