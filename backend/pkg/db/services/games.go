package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	"actionphase/pkg/observability"
)

// GameService provides database operations for game management.
// It implements core.GameServiceInterface and handles all game-related
// database interactions including CRUD operations, state management,
// and participant management.
type GameService struct {
	DB     *pgxpool.Pool
	Logger *observability.Logger
}

// Ensure GameService implements the interface at compile time
var _ core.GameServiceInterface = (*GameService)(nil)

// GameWithDetails represents a game enriched with additional metadata
// including GM information, participant count, and user's role in the game.
// This is used for detailed game views that require aggregated data.
type GameWithDetails struct {
	Game             models.Game
	GMUsername       string // Username of the Game Master
	ParticipantCount int64  // Number of current participants
	UserRole         string // User's role in this game, empty if not participating
}

// CreateGame creates a new game with the specified parameters.
// The game is initially created in "setup" state and requires the GM to
// transition it to "recruitment" when ready to accept players.
//
// Parameters:
//   - ctx: Context for the database operation
//   - req: Game creation request with title, description, GM user ID, and optional settings
//
// Returns:
//   - *models.Game: The created game with generated ID and default state
//   - error: Database error or validation failure
//
// Business Rules:
//   - GMUserID must reference a valid user
//   - Title and description are required
//   - Optional dates must be in logical order (recruitment < start < end)
//   - MaxPlayers defaults to 6 if not specified
//   - Game starts in "setup" state
func (gs *GameService) CreateGame(ctx context.Context, req core.CreateGameRequest) (*models.Game, error) {
	gs.Logger.Info(ctx, "Creating new game",
		"title", req.Title,
		"gm_user_id", req.GMUserID,
		"genre", req.Genre,
		"is_public", req.IsPublic,
		"is_anonymous", req.IsAnonymous,
		"auto_accept_audience", req.AutoAcceptAudience,
		"has_schedule", req.CommonRoomOpenDay != nil,
	)

	queries := models.New(gs.DB)

	var startDate, endDate, recruitmentDeadline pgtype.Timestamptz

	if req.StartDate != nil {
		startDate = pgtype.Timestamptz{Time: *req.StartDate, Valid: true}
	}
	if req.EndDate != nil {
		endDate = pgtype.Timestamptz{Time: *req.EndDate, Valid: true}
	}
	if req.RecruitmentDeadline != nil {
		recruitmentDeadline = pgtype.Timestamptz{Time: *req.RecruitmentDeadline, Valid: true}
	}

	var bannerURL pgtype.Text
	if req.BannerURL != nil {
		bannerURL = pgtype.Text{String: *req.BannerURL, Valid: true}
	}

	openDay, closeDay, openTime, closeTime, scheduleTimezone, err := scheduleFieldsToPgtype(
		req.CommonRoomOpenDay, req.CommonRoomCloseDay,
		req.CommonRoomOpenTime, req.CommonRoomCloseTime,
		req.ScheduleTimezone,
	)
	if err != nil {
		return nil, err
	}

	game, err := queries.CreateGame(ctx, models.CreateGameParams{
		Title:                   req.Title,
		Description:             pgtype.Text{String: req.Description, Valid: req.Description != ""},
		GmUserID:                req.GMUserID,
		Genre:                   pgtype.Text{String: req.Genre, Valid: req.Genre != ""},
		StartDate:               startDate,
		EndDate:                 endDate,
		RecruitmentDeadline:     recruitmentDeadline,
		MaxPlayers:              pgtype.Int4{Int32: req.MaxPlayers, Valid: req.MaxPlayers > 0},
		IsPublic:                pgtype.Bool{Bool: req.IsPublic, Valid: true},
		IsAnonymous:             req.IsAnonymous,
		AutoAcceptAudience:      req.AutoAcceptAudience,
		AllowGroupConversations: req.AllowGroupConversations,
		PortraitAvatars:         req.PortraitAvatars,
		BannerUrl:               bannerURL,
		CommonRoomOpenDay:       openDay,
		CommonRoomOpenTime:      openTime,
		CommonRoomCloseDay:      closeDay,
		CommonRoomCloseTime:     closeTime,
		ScheduleTimezone:        scheduleTimezone,
	})

	if err != nil {
		gs.Logger.LogError(ctx, err, "Failed to create game",
			"title", req.Title,
			"gm_user_id", req.GMUserID,
		)
		return nil, err
	}

	gs.Logger.Info(ctx, "Game created successfully",
		"game_id", game.ID,
		"title", game.Title,
		"gm_user_id", game.GmUserID,
		"state", game.State,
	)

	return &game, nil
}

func (gs *GameService) GetGame(ctx context.Context, gameID int32) (*models.Game, error) {
	queries := models.New(gs.DB)
	game, err := queries.GetGame(ctx, gameID)
	return &game, err
}

func (gs *GameService) GetGamesByUser(ctx context.Context, userID int32) ([]models.GetGamesByUserRow, error) {
	queries := models.New(gs.DB)
	return queries.GetGamesByUser(ctx, userID)
}

func (gs *GameService) UpdateGameState(ctx context.Context, gameID int32, newState string) (*models.Game, error) {
	queries := models.New(gs.DB)

	// Get current game state for logging
	currentGame, err := queries.GetGame(ctx, gameID)
	if err != nil {
		gs.Logger.LogError(ctx, err, "Failed to get game for state transition",
			"game_id", gameID,
		)
		return nil, err
	}

	gs.Logger.Info(ctx, "Game state transition requested",
		"game_id", gameID,
		"from_state", currentGame.State,
		"to_state", newState,
	)

	// Validate state transition
	currentState := currentGame.State.String
	if !isValidTransition(currentState, newState) {
		gs.Logger.Warn(ctx, "Invalid game state transition",
			"game_id", gameID,
			"from_state", currentState,
			"to_state", newState,
		)
		return nil, fmt.Errorf("invalid game state transition: %s → %s", currentState, newState)
	}

	game, err := queries.UpdateGameState(ctx, models.UpdateGameStateParams{
		ID:    gameID,
		State: pgtype.Text{String: newState, Valid: true},
	})
	if err != nil {
		gs.Logger.LogError(ctx, err, "Failed to update game state",
			"game_id", gameID,
			"from_state", currentGame.State,
			"to_state", newState,
		)
		return nil, err
	}

	gs.Logger.Info(ctx, "Game state transition completed",
		"game_id", gameID,
		"from_state", currentGame.State,
		"to_state", game.State,
		"title", game.Title,
	)
	queries.CreateLog(ctx, models.CreateLogParams{
		GameID:  gameID,
		Type:    "GAME_STATE_CHANGE",
		Message: pgtype.Text{String: fmt.Sprintf("Game state changed to: %s", core.GetGameStateDescriptionBrief(newState)), Valid: true},
	})

	// When a game is cancelled, automatically reject all pending applications
	if newState == core.GameStateCancelled {
		appService := &GameApplicationService{DB: gs.DB, Logger: gs.Logger}
		// Use the GM's ID as the reviewer (since they're cancelling the game)
		if err := appService.BulkRejectApplications(ctx, gameID, game.GmUserID); err != nil {
			// Log the error but don't fail the state change
			// The game is already cancelled at this point
			if gs.Logger != nil {
				gs.Logger.Warn(ctx, "Failed to reject pending applications for cancelled game",
					"error", err,
					"game_id", gameID)
			}
		}
	}

	// When a game is completed, disable anonymous mode so players can see who played which character
	if newState == core.GameStateCompleted {
		if err := queries.DisableAnonymousMode(ctx, gameID); err != nil {
			// Log the error but don't fail the state change
			// The game is already completed at this point
			if gs.Logger != nil {
				gs.Logger.Warn(ctx, "Failed to disable anonymous mode for completed game",
					"error", err,
					"game_id", gameID)
			}
		}

		// Clean up manual comment read records — they serve no purpose once a game is archived
		if err := queries.DeleteManualCommentReadsForGame(ctx, gameID); err != nil {
			if gs.Logger != nil {
				gs.Logger.Warn(ctx, "Failed to clean up manual comment reads for completed game",
					"error", err,
					"game_id", gameID)
			}
		}
	}

	// When a game transitions to character_creation, automatically create "Gamemaster" NPC
	if newState == core.GameStateCharacterCreation {
		charService := &CharacterService{DB: gs.DB, Logger: gs.Logger}
		if err := charService.CreateGamemasterNPC(ctx, gameID); err != nil {
			// Log the error but don't fail the state change
			// GM can manually create the NPC later if auto-creation fails
			if gs.Logger != nil {
				gs.Logger.Warn(ctx, "Failed to create Gamemaster NPC for character creation state",
					"error", err,
					"game_id", gameID)
			}
		}
	}

	return &game, nil
}

func (gs *GameService) LeaveGame(ctx context.Context, gameID, userID int32) error {
	queries := models.New(gs.DB)

	// Check if user is GM or Co-GM (they cannot leave their own games)
	game, err := queries.GetGame(ctx, gameID)
	if err != nil {
		return err
	}

	if game.GmUserID == userID || core.IsUserCoGM(ctx, gs.DB, gameID, userID) {
		return fmt.Errorf("game master and co-GMs cannot leave their own game")
	}

	return queries.RemoveGameParticipant(ctx, models.RemoveGameParticipantParams{
		GameID: gameID,
		UserID: userID,
	})
}

func (gs *GameService) GetUserRole(ctx context.Context, gameID, userID int32) (string, error) {
	queries := models.New(gs.DB)

	// Check if user is primary GM
	game, err := queries.GetGame(ctx, gameID)
	if err != nil {
		return "", err
	}

	if game.GmUserID == userID {
		return "gm", nil
	}

	// Check if user is Co-GM or participant
	role, err := queries.GetParticipantRole(ctx, models.GetParticipantRoleParams{
		GameID: gameID,
		UserID: userID,
	})
	if err != nil {
		return "", err
	}

	return role, nil
}

func (gs *GameService) IsUserInGame(ctx context.Context, gameID, userID int32) (bool, error) {
	queries := models.New(gs.DB)

	// Check if user is primary GM
	game, err := queries.GetGame(ctx, gameID)
	if err != nil {
		return false, err
	}

	if game.GmUserID == userID {
		return true, nil
	}

	// Check if user is Co-GM or participant
	exists, err := queries.IsUserInGame(ctx, models.IsUserInGameParams{
		GameID: gameID,
		UserID: userID,
	})

	return exists, err
}

// allowedTransitions defines the valid state machine for games.
//
// State Transition Rules:
//
//	setup → recruitment → character_creation → in_progress ↔ paused → completed
//	Any non-terminal state → cancelled (emergency cancellation)
var allowedTransitions = map[string][]string{
	"setup":              {"recruitment", "cancelled"},
	"recruitment":        {"character_creation", "cancelled"},
	"character_creation": {"in_progress", "cancelled"},
	"in_progress":        {"paused", "completed", "cancelled"},
	"paused":             {"in_progress", "cancelled"},
	"completed":          {},
	"cancelled":          {},
}

// isValidTransition returns true if moving from currentState to newState is permitted.
func isValidTransition(currentState, newState string) bool {
	allowed, ok := allowedTransitions[currentState]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == newState {
			return true
		}
	}
	return false
}

// UpdateGame - Update game details
func (gs *GameService) UpdateGame(ctx context.Context, req core.UpdateGameRequest) (*models.Game, error) {
	queries := models.New(gs.DB)

	// Validate game is not completed/cancelled (archived games are read-only)
	game, err := queries.GetGame(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get game: %w", err)
	}

	if err := core.ValidateGameNotCompleted(ctx, &game); err != nil {
		return nil, err
	}

	var startDate, endDate, recruitmentDeadline pgtype.Timestamptz

	if req.StartDate != nil {
		startDate = pgtype.Timestamptz{Time: *req.StartDate, Valid: true}
	}
	if req.EndDate != nil {
		endDate = pgtype.Timestamptz{Time: *req.EndDate, Valid: true}
	}
	if req.RecruitmentDeadline != nil {
		recruitmentDeadline = pgtype.Timestamptz{Time: *req.RecruitmentDeadline, Valid: true}
	}

	var bannerURL pgtype.Text
	if req.BannerURL != nil {
		bannerURL = pgtype.Text{String: *req.BannerURL, Valid: true}
	}

	openDay, closeDay, openTime, closeTime, scheduleTimezone, err := scheduleFieldsToPgtype(
		req.CommonRoomOpenDay, req.CommonRoomCloseDay,
		req.CommonRoomOpenTime, req.CommonRoomCloseTime,
		req.ScheduleTimezone,
	)
	if err != nil {
		return nil, err
	}

	gs.Logger.Info(ctx, "Updating game",
		"game_id", req.ID,
		"has_schedule", req.CommonRoomOpenDay != nil,
	)

	updatedGame, err := queries.UpdateGame(ctx, models.UpdateGameParams{
		ID:                      req.ID,
		Title:                   req.Title,
		Description:             pgtype.Text{String: req.Description, Valid: req.Description != ""},
		Genre:                   pgtype.Text{String: req.Genre, Valid: req.Genre != ""},
		StartDate:               startDate,
		EndDate:                 endDate,
		RecruitmentDeadline:     recruitmentDeadline,
		MaxPlayers:              pgtype.Int4{Int32: req.MaxPlayers, Valid: req.MaxPlayers > 0},
		IsPublic:                pgtype.Bool{Bool: req.IsPublic, Valid: true},
		IsAnonymous:             req.IsAnonymous,
		AutoAcceptAudience:      req.AutoAcceptAudience,
		AllowGroupConversations: req.AllowGroupConversations,
		PortraitAvatars:         req.PortraitAvatars,
		BannerUrl:               bannerURL,
		CommonRoomOpenDay:       openDay,
		CommonRoomOpenTime:      openTime,
		CommonRoomCloseDay:      closeDay,
		CommonRoomCloseTime:     closeTime,
		ScheduleTimezone:        scheduleTimezone,
	})
	if err != nil {
		return nil, err
	}

	gs.Logger.Info(ctx, "Game updated successfully",
		"game_id", updatedGame.ID,
		"has_schedule", updatedGame.CommonRoomOpenDay.Valid,
	)

	return &updatedGame, nil
}

// UpdateGameBannerURL sets or clears the banner_url for a game.
// Pass nil to clear the banner.
func (gs *GameService) UpdateGameBannerURL(ctx context.Context, gameID int32, bannerURL *string) error {
	queries := models.New(gs.DB)
	var pgBannerURL pgtype.Text
	if bannerURL != nil {
		pgBannerURL = pgtype.Text{String: *bannerURL, Valid: true}
	}
	_, err := queries.UpdateGameBannerURL(ctx, models.UpdateGameBannerURLParams{
		ID:        gameID,
		BannerUrl: pgBannerURL,
	})
	return err
}

// DeleteGame - Delete a cancelled game (GM-only)
// Only games in 'cancelled' state can be deleted, and only by the GM
func (gs *GameService) DeleteGame(ctx context.Context, gameID, userID int32) error {
	gs.Logger.Info(ctx, "Game deletion requested",
		"game_id", gameID,
		"user_id", userID,
	)

	queries := models.New(gs.DB)

	// Fetch the game to verify it exists, check state, and verify GM
	game, err := queries.GetGame(ctx, gameID)
	if err != nil {
		gs.Logger.LogError(ctx, err, "Game not found for deletion",
			"game_id", gameID,
			"user_id", userID,
		)
		return fmt.Errorf("game not found")
	}

	// Verify the user is the GM or Co-GM
	if game.GmUserID != userID && !core.IsUserCoGM(ctx, gs.DB, gameID, userID) {
		gs.Logger.Warn(ctx, "Unauthorized game deletion attempt",
			"game_id", gameID,
			"user_id", userID,
			"gm_user_id", game.GmUserID,
		)
		return fmt.Errorf("only the game master or co-GM can delete this game")
	}

	// Verify the game is in cancelled state
	if game.State.String != core.GameStateCancelled {
		gs.Logger.Warn(ctx, "Cannot delete game: not in cancelled state",
			"game_id", gameID,
			"current_state", game.State.String,
			"required_state", core.GameStateCancelled,
		)
		return fmt.Errorf("only cancelled games can be deleted (current state: %s)", game.State.String)
	}

	// Delete the game (SQL query enforces cancelled state as well)
	err = queries.DeleteGame(ctx, gameID)
	if err != nil {
		gs.Logger.LogError(ctx, err, "Failed to delete game",
			"game_id", gameID,
			"title", game.Title,
		)
		return fmt.Errorf("failed to delete game: %w", err)
	}

	gs.Logger.Info(ctx, "Game deleted successfully",
		"game_id", gameID,
		"title", game.Title,
		"gm_user_id", game.GmUserID,
		"reason", "cancelled",
	)

	return nil
}

// GetGameWithDetails - Get game with additional details like GM username and player count
func (gs *GameService) GetGameWithDetails(ctx context.Context, gameID int32) (*models.GetGameWithDetailsRow, error) {
	queries := models.New(gs.DB)
	game, err := queries.GetGameWithDetails(ctx, gameID)
	return &game, err
}

// GetRecruitingGames - Get games currently accepting players
func (gs *GameService) GetRecruitingGames(ctx context.Context) ([]models.GetRecruitingGamesRow, error) {
	queries := models.New(gs.DB)
	return queries.GetRecruitingGames(ctx)
}

// CanUserJoinGame - Check if user can join a game
func (gs *GameService) CanUserJoinGame(ctx context.Context, gameID, userID int32) (string, error) {
	queries := models.New(gs.DB)
	return queries.CanUserJoinGame(ctx, models.CanUserJoinGameParams{
		GameID: gameID,
		UserID: userID,
	})
}

// AddGameParticipant - Add a user as a participant to a game
func (gs *GameService) AddGameParticipant(ctx context.Context, gameID, userID int32, role string) (*models.GameParticipant, error) {
	gs.Logger.Info(ctx, "Adding participant to game",
		"game_id", gameID,
		"user_id", userID,
		"role", role,
	)

	queries := models.New(gs.DB)

	// Validate game is not completed/cancelled (archived games are read-only)
	game, err := queries.GetGame(ctx, gameID)
	if err != nil {
		gs.Logger.LogError(ctx, err, "Failed to get game for participant addition",
			"game_id", gameID,
		)
		return nil, fmt.Errorf("failed to get game: %w", err)
	}

	if err := core.ValidateGameNotCompleted(ctx, &game); err != nil {
		gs.Logger.Warn(ctx, "Cannot add participant to completed/cancelled game",
			"game_id", gameID,
			"game_state", game.State.String,
		)
		return nil, err
	}

	participant, err := queries.AddGameParticipant(ctx, models.AddGameParticipantParams{
		GameID: gameID,
		UserID: userID,
		Role:   role,
	})

	if err != nil {
		gs.Logger.LogError(ctx, err, "Failed to add participant to game",
			"game_id", gameID,
			"user_id", userID,
			"role", role,
		)
		return nil, err
	}

	gs.Logger.Info(ctx, "Participant added to game successfully",
		"game_id", gameID,
		"user_id", userID,
		"role", participant.Role,
		"participant_id", participant.ID,
	)

	return &participant, nil
}

// RemoveGameParticipant - Remove a user from game participants
func (gs *GameService) RemoveGameParticipant(ctx context.Context, gameID, userID int32) error {
	queries := models.New(gs.DB)
	return queries.RemoveGameParticipant(ctx, models.RemoveGameParticipantParams{
		GameID: gameID,
		UserID: userID,
	})
}

// GetGameParticipants - Get all participants for a game
func (gs *GameService) GetGameParticipants(ctx context.Context, gameID int32) ([]models.GetGameParticipantsRow, error) {
	queries := models.New(gs.DB)
	return queries.GetGameParticipants(ctx, gameID)
}

// GetFilteredGames retrieves games with filters, sorting, and user enrichment
func (gs *GameService) GetFilteredGames(ctx context.Context, filters core.GameListingFilters) (*core.GameListingResponse, error) {
	queries := models.New(gs.DB)

	// Convert filters to sqlc parameters
	// Note: sqlc generated Column1-10 parameter names, mapping:
	// Column1 = user_id, Column2 = states, Column3 = participation_filter
	// Column4 = has_open_spots, Column5 = sort_by
	// Column6 = admin_mode, Column7 = admin_user_id, Column8 = search
	// Column9 = limit, Column10 = offset
	var userID int32
	if filters.UserID != nil {
		userID = *filters.UserID
	}

	var participationFilter string
	if filters.ParticipationFilter != nil {
		participationFilter = *filters.ParticipationFilter
	}

	var hasOpenSpots bool
	if filters.HasOpenSpots != nil {
		hasOpenSpots = *filters.HasOpenSpots
	}

	// Default sort to recent_activity
	sortBy := filters.SortBy
	if sortBy == "" {
		sortBy = "recent_activity"
	}

	var adminUserID int32
	if filters.AdminUserID != nil {
		adminUserID = *filters.AdminUserID
	}

	// Set pagination defaults if not provided
	page := filters.Page
	if page == 0 {
		page = 1
	}
	pageSize := filters.PageSize
	if pageSize == 0 {
		pageSize = 20
	}

	// Calculate pagination
	limit := int32(pageSize)
	offset := int32((page - 1) * pageSize)

	// Get filtered count first (for pagination metadata)
	filteredCount, err := queries.CountFilteredGames(ctx, models.CountFilteredGamesParams{
		Column1: userID,
		Column2: filters.States,
		Column3: participationFilter,
		Column4: hasOpenSpots,
		Column5: filters.AdminMode,
		Column6: adminUserID,
		Column7: filters.Search,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to count games: %w", err)
	}

	// Execute query with pagination
	rows, err := queries.GetFilteredGames(ctx, models.GetFilteredGamesParams{
		Column1:  userID,
		Column2:  filters.States,
		Column3:  participationFilter,
		Column4:  hasOpenSpots,
		Column5:  sortBy,
		Column6:  filters.AdminMode,
		Column7:  adminUserID,
		Column8:  filters.Search,
		Column9:  limit,
		Column10: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch games: %w", err)
	}

	// Convert to domain models
	games := make([]*core.EnrichedGameListItem, len(rows))
	for i, row := range rows {
		games[i] = enrichedGameFromRow(row)
	}

	// Fetch base metadata (available filters and total count)
	metadata, err := gs.getListingMetadata(ctx, queries)
	if err != nil {
		// Don't fail the entire request, just use empty metadata
		metadata = core.GameListingMetadata{
			TotalCount:      0,
			FilteredCount:   int(filteredCount),
			AvailableStates: []string{},
		}
	} else {
		metadata.FilteredCount = int(filteredCount)
	}

	// Calculate pagination metadata
	totalPages := (int(filteredCount) + pageSize - 1) / pageSize // Ceiling division
	if totalPages == 0 {
		totalPages = 1
	}

	metadata.Page = page
	metadata.PageSize = pageSize
	metadata.TotalPages = totalPages
	metadata.HasNextPage = page < totalPages
	metadata.HasPreviousPage = page > 1

	return &core.GameListingResponse{
		Games:    games,
		Metadata: metadata,
	}, nil
}

// getListingMetadata fetches available states for filter dropdowns
func (gs *GameService) getListingMetadata(ctx context.Context, queries *models.Queries) (core.GameListingMetadata, error) {
	// Get total count of public games
	totalCount, err := queries.CountPublicGames(ctx)
	if err != nil {
		return core.GameListingMetadata{}, err
	}

	// Get states that have at least one game
	statesDB, err := queries.GetAvailableStates(ctx)
	if err != nil {
		return core.GameListingMetadata{}, err
	}

	// Convert pgtype.Text to []string
	states := make([]string, 0, len(statesDB))
	for _, s := range statesDB {
		if s.Valid {
			states = append(states, s.String)
		}
	}

	return core.GameListingMetadata{
		TotalCount:      int(totalCount),
		AvailableStates: states,
	}, nil
}

// enrichedGameFromRow converts DB row to EnrichedGameListItem
func enrichedGameFromRow(row models.GetFilteredGamesRow) *core.EnrichedGameListItem {
	return &core.EnrichedGameListItem{
		ID:                      row.ID,
		Title:                   row.Title,
		Description:             textToString(row.Description),
		GMUserID:                row.GmUserID,
		GMUsername:              row.GmUsername,
		State:                   textToString(row.State),
		Genre:                   nullTextToStringPtr(row.Genre),
		StartDate:               timestamptzToTimePtr(row.StartDate),
		EndDate:                 timestamptzToTimePtr(row.EndDate),
		RecruitmentDeadline:     timestamptzToTimePtr(row.RecruitmentDeadline),
		MaxPlayers:              nullInt4ToInt32Ptr(row.MaxPlayers),
		IsPublic:                boolToBool(row.IsPublic),
		IsAnonymous:             row.IsAnonymous,
		AutoAcceptAudience:      row.AutoAcceptAudience,
		AllowGroupConversations: row.AllowGroupConversations,
		PortraitAvatars:         row.PortraitAvatars,
		BannerURL:               nullTextToStringPtr(row.BannerUrl),
		CreatedAt:               timestamptzToTime(row.CreatedAt),
		UpdatedAt:               timestamptzToTime(row.UpdatedAt),
		CurrentPlayers:          int32(row.CurrentPlayers),
		UserRelationship:        interfaceToStringPtr(row.UserRelationship),
		CurrentPhaseType:        interfaceToStringPtr(row.CurrentPhaseType),
		CurrentPhaseDeadline:    timestamptzToTimePtr(row.CurrentPhaseDeadline),
		DeadlineUrgency:         row.DeadlineUrgency,
		HasRecentActivity:       row.HasRecentActivity,
	}
}

// Helper conversion functions for pgtype to Go types
func textToString(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}

func nullTextToStringPtr(t pgtype.Text) *string {
	if t.Valid && t.String != "" {
		return &t.String
	}
	return nil
}

func timestamptzToTime(t pgtype.Timestamptz) time.Time {
	if t.Valid {
		return t.Time
	}
	return time.Time{}
}

func timestamptzToTimePtr(t pgtype.Timestamptz) *time.Time {
	if t.Valid {
		return &t.Time
	}
	return nil
}

func nullInt4ToInt32Ptr(i pgtype.Int4) *int32 {
	if i.Valid {
		return &i.Int32
	}
	return nil
}

func boolToBool(b pgtype.Bool) bool {
	if b.Valid {
		return b.Bool
	}
	return false
}

func interfaceToStringPtr(i interface{}) *string {
	if i == nil {
		return nil
	}
	s, ok := i.(string)
	if !ok || s == "" || s == "none" {
		return nil
	}
	return &s
}

// Player Management Methods

// RemovePlayer removes a player from the game and deactivates their characters
func (gs *GameService) RemovePlayer(ctx context.Context, gameID, userID, gmUserID int32) error {
	gs.Logger.Info(ctx, "Removing player from game",
		"game_id", gameID,
		"user_id", userID,
		"removed_by_user_id", gmUserID,
	)

	queries := models.New(gs.DB)

	// Start a transaction
	gs.Logger.Debug(ctx, "Starting player removal transaction",
		"game_id", gameID,
		"user_id", userID,
	)
	tx, err := gs.DB.Begin(ctx)
	if err != nil {
		gs.Logger.LogError(ctx, err, "Failed to begin player removal transaction",
			"game_id", gameID,
			"user_id", userID,
		)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil {
			gs.Logger.Debug(ctx, "Transaction already committed (rollback ignored)",
				"game_id", gameID,
				"user_id", userID,
			)
		}
	}()

	txQueries := queries.WithTx(tx)

	// Remove the participant (soft delete)
	gs.Logger.Debug(ctx, "Removing participant from game",
		"game_id", gameID,
		"user_id", userID,
	)
	err = txQueries.RemoveParticipant(ctx, models.RemoveParticipantParams{
		GameID:          gameID,
		UserID:          userID,
		RemovedByUserID: pgtype.Int4{Int32: gmUserID, Valid: true},
	})
	if err != nil {
		gs.Logger.LogError(ctx, err, "Failed to remove participant",
			"game_id", gameID,
			"user_id", userID,
		)
		return fmt.Errorf("failed to remove participant: %w", err)
	}

	// Deactivate the player's characters
	gs.Logger.Debug(ctx, "Deactivating player characters",
		"game_id", gameID,
		"user_id", userID,
	)
	err = txQueries.DeactivatePlayerCharacters(ctx, models.DeactivatePlayerCharactersParams{
		GameID: gameID,
		UserID: pgtype.Int4{Int32: userID, Valid: true},
	})
	if err != nil {
		gs.Logger.LogError(ctx, err, "Failed to deactivate player characters",
			"game_id", gameID,
			"user_id", userID,
		)
		return fmt.Errorf("failed to deactivate characters: %w", err)
	}

	// Commit the transaction
	gs.Logger.Debug(ctx, "Committing player removal transaction",
		"game_id", gameID,
		"user_id", userID,
	)
	if err = tx.Commit(ctx); err != nil {
		gs.Logger.LogError(ctx, err, "Failed to commit player removal transaction",
			"game_id", gameID,
			"user_id", userID,
		)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	gs.Logger.Info(ctx, "Player removed successfully",
		"game_id", gameID,
		"user_id", userID,
		"removed_by_user_id", gmUserID,
	)

	return nil
}

// AddParticipantWithRole adds a user directly to the game with the given role, bypassing the application process.
// Valid roles: "player", "audience"
func (gs *GameService) AddParticipantWithRole(ctx context.Context, gameID, userID int32, role string) (*models.GameParticipant, error) {
	queries := models.New(gs.DB)

	participant, err := queries.AddParticipantWithRole(ctx, models.AddParticipantWithRoleParams{
		GameID: gameID,
		UserID: userID,
		Role:   role,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to add participant: %w", err)
	}

	return &participant, nil
}

// GetActiveParticipants retrieves all active (non-removed) participants for a game
func (gs *GameService) GetActiveParticipants(ctx context.Context, gameID int32) ([]models.GetActiveParticipantsRow, error) {
	queries := models.New(gs.DB)
	return queries.GetActiveParticipants(ctx, gameID)
}

// ============================================================================
// Audience Participation Methods
// ============================================================================

// GetGameAutoAcceptAudience retrieves the auto-accept audience setting for a game
func (gs *GameService) GetGameAutoAcceptAudience(ctx context.Context, gameID int32) (bool, error) {
	queries := models.New(gs.DB)
	return queries.GetGameAutoAcceptAudience(ctx, gameID)
}

// UpdateGameAutoAcceptAudience updates the auto-accept audience setting for a game
func (gs *GameService) UpdateGameAutoAcceptAudience(ctx context.Context, gameID int32, autoAccept bool) error {
	queries := models.New(gs.DB)
	return queries.UpdateGameAutoAcceptAudience(ctx, models.UpdateGameAutoAcceptAudienceParams{
		ID:                 gameID,
		AutoAcceptAudience: autoAccept,
	})
}

// CreateAudienceApplication allows a user to apply/join as an audience member
// If auto_accept_audience is enabled, user is immediately added as active audience.
// Otherwise, they are added with 'pending' status and require GM approval.
func (gs *GameService) CreateAudienceApplication(ctx context.Context, gameID, userID int32) (*models.GameParticipant, error) {
	queries := models.New(gs.DB)

	// Check if auto-accept is enabled for this game
	autoAccept, err := queries.GetGameAutoAcceptAudience(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to get auto-accept setting: %w", err)
	}

	// Determine status based on auto-accept setting
	// Use 'inactive' for manual approval, 'active' for auto-accept
	status := "active"
	if !autoAccept {
		status = "inactive"
	}

	// Create the audience application/participant
	participant, err := queries.CreateAudienceApplication(ctx, models.CreateAudienceApplicationParams{
		GameID: gameID,
		UserID: userID,
		Status: pgtype.Text{String: status, Valid: true},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create audience application: %w", err)
	}

	gs.Logger.Info(ctx, "Audience application created",
		"game_id", gameID,
		"user_id", userID,
		"auto_accepted", autoAccept,
		"status", status,
	)

	return &participant, nil
}

// ListAudienceMembers retrieves all audience members for a game
func (gs *GameService) ListAudienceMembers(ctx context.Context, gameID int32) ([]models.ListAudienceMembersRow, error) {
	queries := models.New(gs.DB)
	return queries.ListAudienceMembers(ctx, gameID)
}

// CheckAudienceAccess verifies if a user has audience or GM access to a game
// Returns true if the user is:
// - The GM of the game
// - A co-GM participant
// - An active audience member
func (gs *GameService) CheckAudienceAccess(ctx context.Context, gameID, userID int32) (bool, error) {
	queries := models.New(gs.DB)

	result, err := queries.CheckAudienceAccess(ctx, models.CheckAudienceAccessParams{
		GameID: gameID,
		UserID: userID,
	})

	if err != nil {
		return false, fmt.Errorf("failed to check audience access: %w", err)
	}

	return result.Bool, nil
}

// CanUserViewGame checks if a user can view a game's content (read-only access).
// Public Archive Mode: Completed games are viewable by ANY user (not just participants).
// Active Games: Follows normal permission rules (GM, participants, audience).
// Cancelled Games: Follow normal permission rules (NOT public).
func (gs *GameService) CanUserViewGame(ctx context.Context, gameID, userID int32) (bool, error) {
	queries := models.New(gs.DB)

	// Get the game to check its state
	game, err := queries.GetGame(ctx, gameID)
	if err != nil {
		return false, fmt.Errorf("failed to get game: %w", err)
	}

	// Public Archive Mode: Completed games are viewable by anyone
	if game.State.Valid && game.State.String == core.GameStateCompleted {
		return true, nil
	}

	// For non-completed games (including cancelled), use normal permission checks
	// Check if user is GM or any type of participant (player, audience, co_gm)
	isParticipant, err := gs.IsUserInGame(ctx, gameID, userID)
	if err != nil {
		return false, fmt.Errorf("failed to check participation: %w", err)
	}

	return isParticipant, nil
}

// ============================================================================
// Co-GM Management Methods
// ============================================================================

// PromoteToCoGM promotes an audience member to co-GM role.
// Co-GMs have full GM permissions except editing game settings and promoting others.
//
// Business Rules:
//   - Only the primary GM (game.gm_user_id) can promote users
//   - Only audience members can be promoted to co-GM
//   - Only one co-GM allowed per game
//   - Target user must be an active participant
//
// Parameters:
//   - ctx: Context for the database operation
//   - gameID: ID of the game
//   - userID: ID of the user to promote
//   - requestingUserID: ID of the user making the request (must be primary GM)
//
// Returns:
//   - error: Validation error or database error
func (gs *GameService) PromoteToCoGM(ctx context.Context, gameID, userID, requestingUserID int32) error {
	queries := models.New(gs.DB)

	// 1. Verify requester is the primary GM
	game, err := queries.GetGame(ctx, gameID)
	if err != nil {
		return fmt.Errorf("failed to get game: %w", err)
	}

	if game.GmUserID != requestingUserID {
		return fmt.Errorf("only the primary GM can promote users to co-GM")
	}

	// 2. Verify target user is an active participant
	participant, err := queries.GetParticipantByGameAndUser(ctx, models.GetParticipantByGameAndUserParams{
		GameID: gameID,
		UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("user is not a participant in this game: %w", err)
	}

	// 3. Verify target user is currently an audience member
	if participant.Role != "audience" {
		return fmt.Errorf("can only promote audience members to co-GM (current role: %s)", participant.Role)
	}

	// 4. Verify no existing co-GM in the game
	coGMCount, err := queries.CountCoGMsInGame(ctx, gameID)
	if err != nil {
		return fmt.Errorf("failed to count co-GMs: %w", err)
	}

	if coGMCount > 0 {
		return fmt.Errorf("game already has a co-GM, only one co-GM allowed per game")
	}

	// 5. Update participant role to co_gm
	_, err = queries.UpdateParticipantRole(ctx, models.UpdateParticipantRoleParams{
		GameID: gameID,
		UserID: userID,
		Role:   "co_gm",
	})

	if err != nil {
		return fmt.Errorf("failed to update participant role: %w", err)
	}

	gs.Logger.Info(ctx, "User promoted to co-GM",
		"game_id", gameID,
		"promoted_user_id", userID,
		"requested_by_user_id", requestingUserID,
	)

	return nil
}

// DemoteFromCoGM demotes a co-GM back to audience role.
//
// Business Rules:
//   - Only the primary GM (game.gm_user_id) can demote co-GMs
//   - Target user must currently be a co-GM
//   - Target user becomes audience member after demotion
//
// Parameters:
//   - ctx: Context for the database operation
//   - gameID: ID of the game
//   - userID: ID of the co-GM to demote
//   - requestingUserID: ID of the user making the request (must be primary GM)
//
// Returns:
//   - error: Validation error or database error
func (gs *GameService) DemoteFromCoGM(ctx context.Context, gameID, userID, requestingUserID int32) error {
	queries := models.New(gs.DB)

	// 1. Verify requester is the primary GM
	game, err := queries.GetGame(ctx, gameID)
	if err != nil {
		return fmt.Errorf("failed to get game: %w", err)
	}

	if game.GmUserID != requestingUserID {
		return fmt.Errorf("only the primary GM can demote co-GMs")
	}

	// 2. Verify target user is an active participant
	participant, err := queries.GetParticipantByGameAndUser(ctx, models.GetParticipantByGameAndUserParams{
		GameID: gameID,
		UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("user is not a participant in this game: %w", err)
	}

	// 3. Verify target user is currently a co-GM
	if participant.Role != "co_gm" {
		return fmt.Errorf("can only demote co-GMs (current role: %s)", participant.Role)
	}

	// 4. Update participant role to audience
	_, err = queries.UpdateParticipantRole(ctx, models.UpdateParticipantRoleParams{
		GameID: gameID,
		UserID: userID,
		Role:   "audience",
	})

	if err != nil {
		return fmt.Errorf("failed to update participant role: %w", err)
	}

	gs.Logger.Info(ctx, "Co-GM demoted to audience",
		"game_id", gameID,
		"demoted_user_id", userID,
		"requested_by_user_id", requestingUserID,
	)

	return nil
}

// TransitionPlayerToAudience changes a player's role to audience without deactivating their characters.
// Used when a player character dies (permadeath) — the player retains character access and can still
// comment in common rooms, but is no longer an active player.
func (gs *GameService) TransitionPlayerToAudience(ctx context.Context, gameID, userID, requestingUserID int32) error {
	queries := models.New(gs.DB)

	game, err := queries.GetGame(ctx, gameID)
	if err != nil {
		return fmt.Errorf("failed to get game: %w", err)
	}
	if game.GmUserID != requestingUserID {
		return fmt.Errorf("only the primary GM can transition players to audience")
	}

	participant, err := queries.GetParticipantByGameAndUser(ctx, models.GetParticipantByGameAndUserParams{
		GameID: gameID,
		UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("user is not a participant in this game: %w", err)
	}
	if participant.Role != "player" {
		return fmt.Errorf("can only transition players to audience (current role: %s)", participant.Role)
	}

	_, err = queries.TransitionParticipantToAudience(ctx, models.TransitionParticipantToAudienceParams{
		GameID: gameID,
		UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("failed to transition participant to audience: %w", err)
	}

	gs.Logger.Info(ctx, "Player transitioned to audience (permadeath)",
		"game_id", gameID,
		"user_id", userID,
		"requested_by_user_id", requestingUserID,
	)

	return nil
}

// scheduleFieldsToPgtype converts schedule pointer fields from a request into pgtype values.
// Returns an error if any time string is malformed (should not occur when handler validation runs first).
func scheduleFieldsToPgtype(
	openDay, closeDay *int16,
	openTime, closeTime *string,
	tz *string,
) (pgtype.Int2, pgtype.Int2, pgtype.Time, pgtype.Time, pgtype.Text, error) {
	var pgOpenDay, pgCloseDay pgtype.Int2
	if openDay != nil {
		pgOpenDay = pgtype.Int2{Int16: *openDay, Valid: true}
	}
	if closeDay != nil {
		pgCloseDay = pgtype.Int2{Int16: *closeDay, Valid: true}
	}

	var pgOpenTime, pgCloseTime pgtype.Time
	if openTime != nil {
		t, err := parseHHMM(*openTime)
		if err != nil {
			return pgtype.Int2{}, pgtype.Int2{}, pgtype.Time{}, pgtype.Time{}, pgtype.Text{}, fmt.Errorf("invalid open time: %w", err)
		}
		pgOpenTime = t
	}
	if closeTime != nil {
		t, err := parseHHMM(*closeTime)
		if err != nil {
			return pgtype.Int2{}, pgtype.Int2{}, pgtype.Time{}, pgtype.Time{}, pgtype.Text{}, fmt.Errorf("invalid close time: %w", err)
		}
		pgCloseTime = t
	}

	var pgTZ pgtype.Text
	if tz != nil {
		pgTZ = pgtype.Text{String: *tz, Valid: true}
	}

	return pgOpenDay, pgCloseDay, pgOpenTime, pgCloseTime, pgTZ, nil
}

// parseHHMM parses a "HH:MM" string into a pgtype.Time.
func parseHHMM(s string) (pgtype.Time, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return pgtype.Time{}, fmt.Errorf("invalid time %q: %w", s, err)
	}
	microseconds := int64(t.Hour())*3600*1e6 + int64(t.Minute())*60*1e6
	return pgtype.Time{Microseconds: microseconds, Valid: true}, nil
}

// GetGameLogs - Get game logs
func (gs *GameService) GetGameLogs(ctx context.Context, gameID int32) ([]models.GameLog, error) {
	queries := models.New(gs.DB)
	logs, err := queries.GetGameLogs(ctx, gameID)
	return logs, err
}

// AddGameLog - Append a log entry for a game
func (gs *GameService) AddGameLog(ctx context.Context, arg models.CreateLogParams) (models.GameLog, error) {
	queries := models.New(gs.DB)
	log, err := queries.CreateLog(ctx, arg)
	return log, err
}

// GetGameLootTables - Get game loot tables
func (gs *GameService) GetGameLootTables(ctx context.Context, gameID int32, excludeEmpty bool) ([]models.GameLootTable, error) {
	queries := models.New(gs.DB)
	if excludeEmpty {
		lootTables, err := queries.GetNonEmptyLootTables(ctx, gameID)
		return lootTables, err
	}
	lootTables, err := queries.GetLootTables(ctx, gameID)
	return lootTables, err
}

// IsLootTableInGame - Check if a loot table belongs to a specific game
func (gs *GameService) IsLootTableInGame(ctx context.Context, lootTableID, gameID int32) (bool, error) {
	queries := models.New(gs.DB)
	result, err := queries.IsLootTableInGame(ctx, models.IsLootTableInGameParams{
		ID:     lootTableID,
		GameID: gameID,
	})
	if err != nil {
		return false, err
	}
	return result, nil
}

// CreateLootTable - Create a new loot table for a game
func (gs *GameService) CreateLootTable(ctx context.Context, gameID int32, name string) (*models.GameLootTable, error) {
	queries := models.New(gs.DB)
	lootTable, err := queries.CreateLootTable(ctx, models.CreateLootTableParams{
		GameID: gameID,
		Name:   name,
	})
	return &lootTable, err
}

// UpdateLootTable - Update an existing loot table
func (gs *GameService) UpdateLootTable(ctx context.Context, lootTableID int32, name string) (*models.GameLootTable, error) {
	queries := models.New(gs.DB)
	lootTable, err := queries.UpdateLootTable(ctx, models.UpdateLootTableParams{
		ID:   lootTableID,
		Name: name,
	})
	return &lootTable, err
}

// DeleteLootTable - Remove a loot table from a game
func (gs *GameService) DeleteLootTable(ctx context.Context, lootTableID int32) error {
	queries := models.New(gs.DB)
	return queries.DeleteLootTable(ctx, lootTableID)
}

// GetGameLootTableContents - Get contents of a specific loot table
func (gs *GameService) GetGameLootTableContents(ctx context.Context, lootTableID int32) ([]models.GameLootTableContent, error) {
	queries := models.New(gs.DB)
	contents, err := queries.GetLootTableContents(ctx, lootTableID)
	return contents, err
}

// AddLootTableContent - Add an item to a loot table
func (gs *GameService) AddLootTableContent(ctx context.Context, lootTableID int32, itemName string, itemData string) (*models.GameLootTableContent, error) {
	queries := models.New(gs.DB)
	content, err := queries.AddLootTableContent(ctx, models.AddLootTableContentParams{
		LootTableID: lootTableID,
		Name:        itemName,
		Data:        pgtype.Text{String: itemData, Valid: true},
	})
	return &content, err
}

// ReplaceLootTableContents - Atomically swap a loot table's entire item list.
//
// The rewrite is a delete followed by re-inserts, so it MUST be one transaction:
// without it, a failure partway through the inserts leaves the table empty or
// half-populated with the GM's original items already gone. That is silent data
// loss on what the UI presents as a save.
//
// Also bumps updated_at in the same transaction — contents live in a child table,
// so the parent row is otherwise untouched and its timestamp goes stale.
func (gs *GameService) ReplaceLootTableContents(ctx context.Context, lootTableID int32, items []core.LootTableItem) error {
	tx, err := gs.DB.Begin(ctx)
	if err != nil {
		gs.Logger.LogError(ctx, err, "Failed to begin loot table contents transaction",
			"loot_table_id", lootTableID,
		)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil {
			gs.Logger.Debug(ctx, "Transaction already committed (rollback ignored)",
				"loot_table_id", lootTableID,
			)
		}
	}()

	txQueries := models.New(gs.DB).WithTx(tx)

	if err := txQueries.DeleteLootTableContents(ctx, lootTableID); err != nil {
		gs.Logger.LogError(ctx, err, "Failed to delete loot table contents",
			"loot_table_id", lootTableID,
		)
		return fmt.Errorf("failed to delete loot table contents: %w", err)
	}

	for i, item := range items {
		if _, err := txQueries.AddLootTableContent(ctx, models.AddLootTableContentParams{
			LootTableID: lootTableID,
			Name:        item.Name,
			Data:        pgtype.Text{String: item.Data, Valid: true},
		}); err != nil {
			gs.Logger.LogError(ctx, err, "Failed to add loot table content",
				"loot_table_id", lootTableID,
				"item_index", i,
			)
			return fmt.Errorf("failed to add loot table item %d: %w", i+1, err)
		}
	}

	if err := txQueries.TouchLootTable(ctx, lootTableID); err != nil {
		gs.Logger.LogError(ctx, err, "Failed to bump loot table updated_at",
			"loot_table_id", lootTableID,
		)
		return fmt.Errorf("failed to bump loot table updated_at: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		gs.Logger.LogError(ctx, err, "Failed to commit loot table contents transaction",
			"loot_table_id", lootTableID,
		)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}
