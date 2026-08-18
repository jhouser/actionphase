package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	"actionphase/pkg/observability"
)

const (
	// MaxCharacterNameLength is the maximum allowed length for character names
	MaxCharacterNameLength = 255
)

var _ core.CharacterServiceInterface = (*CharacterService)(nil)

type CharacterService struct {
	DB     *pgxpool.Pool
	Logger *observability.Logger
}

// CreateCharacterRequest is an alias kept for callers that used the old db-package type.
type CreateCharacterRequest = core.CreateCharacterRequest

// CharacterDataRequest is an alias kept for callers that used the old db-package type.
type CharacterDataRequest = core.CharacterDataRequest

func (cs *CharacterService) CreateCharacter(ctx context.Context, req CreateCharacterRequest) (*models.Character, error) {
	defer cs.Logger.LogOperation(ctx, "create_character",
		"game_id", req.GameID,
		"character_type", req.CharacterType,
		"character_name", req.Name,
	)()

	queries := models.New(cs.DB)

	// Validate character type
	if !isValidCharacterType(req.CharacterType) {
		cs.Logger.Warn(ctx, "Invalid character type provided",
			"character_type", req.CharacterType,
			"game_id", req.GameID,
		)
		return nil, fmt.Errorf("invalid character type: %s", req.CharacterType)
	}

	// For player characters, user ID is required
	if req.CharacterType == "player_character" && req.UserID == nil {
		cs.Logger.Warn(ctx, "User ID required for player character creation",
			"game_id", req.GameID,
			"character_name", req.Name,
		)
		return nil, fmt.Errorf("user ID required for player characters")
	}

	var userID pgtype.Int4
	if req.UserID != nil {
		userID = pgtype.Int4{Int32: *req.UserID, Valid: true}
	}

	character, err := queries.CreateCharacter(ctx, models.CreateCharacterParams{
		GameID:        req.GameID,
		UserID:        userID,
		Name:          req.Name,
		CharacterType: req.CharacterType,
		Status:        pgtype.Text{String: "pending", Valid: true}, // Default status
	})

	if err != nil {
		cs.Logger.LogError(ctx, err, "Failed to create character",
			"game_id", req.GameID,
			"character_type", req.CharacterType,
		)
		return nil, err
	}

	cs.Logger.Info(ctx, "Character created successfully",
		"character_id", character.ID,
		"game_id", req.GameID,
		"character_type", req.CharacterType,
		"character_name", character.Name,
		"status", character.Status.String,
	)

	return &character, nil
}

// CreateGamemasterNPC creates the default "Gamemaster" NPC for a game.
// This is called automatically when a game transitions to character_creation state.
// The NPC is created with approved status and can be used immediately by the GM.
//
// This method is idempotent - if a character named "Gamemaster" already exists in
// the game, it will skip creation and log an info message.
func (cs *CharacterService) CreateGamemasterNPC(ctx context.Context, gameID int32) error {
	defer cs.Logger.LogOperation(ctx, "create_gamemaster_npc", "game_id", gameID)()

	queries := models.New(cs.DB)

	// Check if Gamemaster NPC already exists for this game
	existingChar, err := queries.GetCharacterByNameAndGame(ctx, models.GetCharacterByNameAndGameParams{
		Name:   "Gamemaster",
		GameID: gameID,
	})

	// If character exists (no error), skip creation
	if err == nil && existingChar.ID > 0 {
		cs.Logger.Info(ctx, "Gamemaster NPC already exists, skipping creation",
			"game_id", gameID,
			"character_id", existingChar.ID,
			"character_type", existingChar.CharacterType,
		)
		return nil
	}

	// Create the Gamemaster NPC with approved status
	character, err := queries.CreateCharacter(ctx, models.CreateCharacterParams{
		GameID:        gameID,
		UserID:        pgtype.Int4{Valid: false}, // NULL for GM NPCs
		Name:          "Gamemaster",
		CharacterType: "npc",
		Status:        pgtype.Text{String: "approved", Valid: true}, // Auto-approved
	})

	if err != nil {
		cs.Logger.LogError(ctx, err, "Failed to create Gamemaster NPC", "game_id", gameID)
		return fmt.Errorf("failed to create Gamemaster NPC: %w", err)
	}

	cs.Logger.Info(ctx, "Gamemaster NPC created successfully",
		"game_id", gameID,
		"character_id", character.ID,
		"character_name", character.Name,
		"status", character.Status.String,
	)

	return nil
}

func (cs *CharacterService) RenameCharacter(ctx context.Context, characterID int32, newName string) (*models.Character, error) {
	defer cs.Logger.LogOperation(ctx, "rename_character",
		"character_id", characterID,
		"new_name", newName)()

	queries := models.New(cs.DB)

	// Trim and validate name
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return nil, fmt.Errorf("character name cannot be empty")
	}
	if len(newName) > MaxCharacterNameLength {
		return nil, fmt.Errorf("character name too long (max %d)", MaxCharacterNameLength)
	}

	// Get current character to check game_id and current name
	character, err := queries.GetCharacter(ctx, characterID)
	if err != nil {
		cs.Logger.LogError(ctx, err, "Failed to get character for rename")
		return nil, err
	}

	// Check if new name is same as current (no-op)
	if character.Name == newName {
		return &character, nil
	}

	// Use existing UpdateCharacter query
	updated, err := queries.UpdateCharacter(ctx, models.UpdateCharacterParams{
		ID:     characterID,
		Name:   newName,
		Status: character.Status, // Keep existing status
	})

	if err != nil {
		// Handle unique constraint violation specifically
		if strings.Contains(err.Error(), "characters_game_name_unique") {
			return nil, fmt.Errorf("a character named '%s' already exists in this game", newName)
		}
		cs.Logger.LogError(ctx, err, "Failed to rename character")
		return nil, err
	}

	cs.Logger.Info(ctx, "Character renamed successfully",
		"character_id", characterID,
		"old_name", character.Name,
		"new_name", newName)

	return &updated, nil
}

func (cs *CharacterService) GetCharacter(ctx context.Context, characterID int32) (*models.Character, error) {
	queries := models.New(cs.DB)
	character, err := queries.GetCharacter(ctx, characterID)
	return &character, err
}

func (cs *CharacterService) GetCharactersByGame(ctx context.Context, gameID int32) ([]models.GetCharactersByGameRow, error) {
	queries := models.New(cs.DB)
	return queries.GetCharactersByGame(ctx, gameID)
}

func (cs *CharacterService) GetPlayerCharacters(ctx context.Context, gameID int32) ([]models.GetPlayerCharactersByGameRow, error) {
	queries := models.New(cs.DB)
	return queries.GetPlayerCharactersByGame(ctx, gameID)
}

func (cs *CharacterService) GetNPCs(ctx context.Context, gameID int32) ([]models.GetNPCsByGameRow, error) {
	queries := models.New(cs.DB)
	return queries.GetNPCsByGame(ctx, gameID)
}

func (cs *CharacterService) GetUserControllableCharacters(ctx context.Context, gameID, userID int32) ([]models.GetUserControllableCharactersRow, error) {
	queries := models.New(cs.DB)
	return queries.GetUserControllableCharacters(ctx, models.GetUserControllableCharactersParams{
		GameID: gameID,
		UserID: userID,
	})
}

// GetUserControllableCharactersAcrossGames returns every character the user can
// control in games that are currently in_progress, along with the game context
// (title, state, flags, and the user's role) each character's sheet needs.
func (cs *CharacterService) GetUserControllableCharactersAcrossGames(ctx context.Context, userID int32) ([]models.GetUserControllableCharactersAcrossGamesRow, error) {
	queries := models.New(cs.DB)
	return queries.GetUserControllableCharactersAcrossGames(ctx, userID)
}

func (cs *CharacterService) ApproveCharacter(ctx context.Context, characterID int32) (*models.Character, error) {
	defer cs.Logger.LogOperation(ctx, "approve_character", "character_id", characterID)()

	queries := models.New(cs.DB)
	character, err := queries.UpdateCharacterStatus(ctx, models.UpdateCharacterStatusParams{
		ID:     characterID,
		Status: pgtype.Text{String: "approved", Valid: true},
	})

	if err != nil {
		cs.Logger.LogError(ctx, err, "Failed to approve character", "character_id", characterID)
		return nil, err
	}

	cs.Logger.Info(ctx, "Character approved",
		"character_id", character.ID,
		"character_name", character.Name,
		"game_id", character.GameID,
		"status", character.Status.String,
	)

	return &character, nil
}

func (cs *CharacterService) AssignNPCToUser(ctx context.Context, characterID, assignedUserID, assignedByUserID int32) error {
	defer cs.Logger.LogOperation(ctx, "assign_npc_to_user",
		"character_id", characterID,
		"assigned_user_id", assignedUserID,
		"assigned_by_user_id", assignedByUserID,
	)()

	queries := models.New(cs.DB)

	// Verify this is an NPC
	character, err := queries.GetCharacter(ctx, characterID)
	if err != nil {
		cs.Logger.LogError(ctx, err, "Failed to get character for NPC assignment", "character_id", characterID)
		return err
	}

	if character.CharacterType != "npc" {
		cs.Logger.Warn(ctx, "Attempted to assign non-NPC character to user",
			"character_id", characterID,
			"character_type", character.CharacterType,
		)
		return fmt.Errorf("character is not an NPC")
	}

	_, err = queries.AssignNPCToUser(ctx, models.AssignNPCToUserParams{
		CharacterID:      characterID,
		AssignedUserID:   assignedUserID,
		AssignedByUserID: assignedByUserID,
	})

	if err != nil {
		cs.Logger.LogError(ctx, err, "Failed to assign NPC to user",
			"character_id", characterID,
			"assigned_user_id", assignedUserID,
		)
		return err
	}

	cs.Logger.Info(ctx, "NPC assigned to user",
		"character_id", characterID,
		"character_name", character.Name,
		"assigned_user_id", assignedUserID,
		"assigned_by_user_id", assignedByUserID,
	)

	return nil
}

func (cs *CharacterService) SetCharacterData(ctx context.Context, req CharacterDataRequest) error {
	queries := models.New(cs.DB)

	var fieldValue pgtype.Text
	if req.FieldValue != "" {
		fieldValue = pgtype.Text{String: req.FieldValue, Valid: true}
	}

	_, err := queries.CreateCharacterData(ctx, models.CreateCharacterDataParams{
		CharacterID: req.CharacterID,
		ModuleType:  req.ModuleType,
		FieldName:   req.FieldName,
		FieldValue:  fieldValue,
		FieldType:   pgtype.Text{String: req.FieldType, Valid: true},
		IsPublic:    pgtype.Bool{Bool: req.IsPublic, Valid: true},
	})
	if err != nil {
		cs.Logger.LogError(ctx, err, "Failed to set character data",
			"character_id", req.CharacterID,
			"module_type", req.ModuleType,
			"field_name", req.FieldName,
		)
		return err
	}

	cs.Logger.Info(ctx, "Character data updated",
		"character_id", req.CharacterID,
		"module_type", req.ModuleType,
		"field_name", req.FieldName,
		"is_public", req.IsPublic,
	)
	return nil
}

func (cs *CharacterService) AddToCharacterData(ctx context.Context, req CharacterDataRequest) error {
	existingData, err := cs.GetCharacterData(ctx, req.CharacterID)
	if err != nil {
		return fmt.Errorf("failed to get character data: %w", err)
	}

	for _, module := range existingData {
		if module.ModuleType == req.ModuleType && module.FieldName == req.FieldName {
			newData, err := addStructureToArray(module.FieldValue.String, req.FieldValue)
			if err != nil {
				return fmt.Errorf("failed to add structure to array: %w", err)
			}
			if err := cs.SetCharacterData(ctx, core.CharacterDataRequest{
				CharacterID: req.CharacterID,
				ModuleType:  req.ModuleType,
				FieldName:   req.FieldName,
				FieldValue:  newData,
				FieldType:   req.FieldType,
				IsPublic:    req.IsPublic,
			}); err != nil {
				return fmt.Errorf("failed to set character data: %w", err)
			}
			return nil
		}
	}

	newData, err := addStructureToArray("[]", req.FieldValue)
	if err != nil {
		return fmt.Errorf("failed to add structure to array: %w", err)
	}
	if err := cs.SetCharacterData(ctx, core.CharacterDataRequest{
		CharacterID: req.CharacterID,
		ModuleType:  req.ModuleType,
		FieldName:   req.FieldName,
		FieldValue:  newData,
		FieldType:   req.FieldType,
		IsPublic:    req.IsPublic,
	}); err != nil {
		return fmt.Errorf("failed to set character data: %w", err)
	}
	return nil
}

func addStructureToArray(arrayJSON string, newStructureJSON string) (string, error) {
	// Validate that the new structure is at least well-formed JSON
	// (RawMessage just stores bytes, so we check validity explicitly)
	newElement := json.RawMessage(newStructureJSON)
	if !json.Valid(newElement) {
		return "", fmt.Errorf("new structure is not valid JSON")
	}

	// Unmarshal the array into a slice of raw messages, preserving
	// each element's original bytes/shape untouched
	var elements []json.RawMessage
	if err := json.Unmarshal([]byte(arrayJSON), &elements); err != nil {
		return "", fmt.Errorf("existing array is not valid JSON array: %w", err)
	}

	// Append the new element
	elements = append(elements, newElement)

	// Marshal back to a JSON array string
	result, err := json.Marshal(elements)
	if err != nil {
		return "", fmt.Errorf("failed to re-marshal array: %w", err)
	}

	return string(result), nil
}

func (cs *CharacterService) GetCharacterData(ctx context.Context, characterID int32) ([]models.CharacterDatum, error) {
	queries := models.New(cs.DB)
	return queries.GetCharacterData(ctx, characterID)
}

func (cs *CharacterService) GetCharacterDataByModule(ctx context.Context, characterID int32, moduleType string) ([]models.CharacterDatum, error) {
	queries := models.New(cs.DB)
	return queries.GetCharacterDataByModule(ctx, models.GetCharacterDataByModuleParams{
		CharacterID: characterID,
		ModuleType:  moduleType,
	})
}

func (cs *CharacterService) GetPublicCharacterData(ctx context.Context, characterID int32) ([]models.CharacterDatum, error) {
	queries := models.New(cs.DB)
	return queries.GetPublicCharacterData(ctx, characterID)
}

func (cs *CharacterService) CanUserEditCharacter(ctx context.Context, characterID, userID int32) (bool, error) {
	queries := models.New(cs.DB)

	character, err := queries.GetCharacter(ctx, characterID)
	if err != nil {
		cs.Logger.LogError(ctx, err, "Failed to get character for permission check",
			"character_id", characterID,
			"user_id", userID,
		)
		return false, err
	}

	// Character owner can edit
	if character.UserID.Valid && character.UserID.Int32 == userID {
		cs.Logger.Debug(ctx, "Authorization granted: character owner",
			"character_id", characterID,
			"user_id", userID,
			"reason", "character_owner",
		)
		return true, nil
	}

	// GM can edit any character in their game
	game, err := queries.GetGame(ctx, character.GameID)
	if err != nil {
		cs.Logger.LogError(ctx, err, "Failed to get game for permission check",
			"character_id", characterID,
			"game_id", character.GameID,
		)
		return false, err
	}

	// Check if user is GM or Co-GM
	if game.GmUserID == userID || core.IsUserCoGM(ctx, cs.DB, character.GameID, userID) {
		cs.Logger.Debug(ctx, "Authorization granted: game GM or Co-GM",
			"character_id", characterID,
			"user_id", userID,
			"game_id", character.GameID,
			"reason", "game_gm_or_co_gm",
		)
		return true, nil
	}

	// Check if user is assigned to this NPC
	if character.CharacterType == "npc" {
		assignment, err := queries.GetNPCAssignment(ctx, characterID)
		// Ignore "no rows" error - just means NPC is not assigned
		if err == nil && assignment.AssignedUserID == userID {
			cs.Logger.Debug(ctx, "Authorization granted: NPC assigned user",
				"character_id", characterID,
				"user_id", userID,
				"reason", "npc_assignment",
			)
			return true, nil
		}
		// If there's an error other than "no rows", it's a real problem
		// But we should still allow GM and owner permissions to work
		// So we don't return the error here, just continue to return false
	}

	return false, nil
}

func isValidCharacterType(characterType string) bool {
	validTypes := []string{"player_character", "npc"}
	for _, validType := range validTypes {
		if characterType == validType {
			return true
		}
	}
	return false
}

// Player Management Methods

// ReassignCharacter reassigns a character to a new owner (used when removing players)
func (cs *CharacterService) ReassignCharacter(ctx context.Context, characterID, newOwnerUserID int32) (*models.Character, error) {
	defer cs.Logger.LogOperation(ctx, "reassign_character",
		"character_id", characterID,
		"new_owner_user_id", newOwnerUserID,
	)()

	queries := models.New(cs.DB)

	character, err := queries.ReassignCharacter(ctx, models.ReassignCharacterParams{
		ID:     characterID,
		UserID: pgtype.Int4{Int32: newOwnerUserID, Valid: true},
	})

	if err != nil {
		cs.Logger.LogError(ctx, err, "Failed to reassign character",
			"character_id", characterID,
			"new_owner_user_id", newOwnerUserID,
		)
		return nil, err
	}

	cs.Logger.Info(ctx, "Character reassigned to new owner",
		"character_id", character.ID,
		"character_name", character.Name,
		"new_owner_user_id", newOwnerUserID,
		"game_id", character.GameID,
	)

	return &character, nil
}

// ListInactiveCharacters returns all inactive characters for a game
func (cs *CharacterService) ListInactiveCharacters(ctx context.Context, gameID int32) ([]models.ListInactiveCharactersRow, error) {
	queries := models.New(cs.DB)
	return queries.ListInactiveCharacters(ctx, gameID)
}

// DeactivatePlayerCharacters marks all player characters for a user as inactive
func (cs *CharacterService) DeactivatePlayerCharacters(ctx context.Context, gameID, userID int32) error {
	defer cs.Logger.LogOperation(ctx, "deactivate_player_characters",
		"game_id", gameID,
		"user_id", userID,
	)()

	queries := models.New(cs.DB)
	err := queries.DeactivatePlayerCharacters(ctx, models.DeactivatePlayerCharactersParams{
		GameID: gameID,
		UserID: pgtype.Int4{Int32: userID, Valid: true},
	})

	if err != nil {
		cs.Logger.LogError(ctx, err, "Failed to deactivate player characters",
			"game_id", gameID,
			"user_id", userID,
		)
		return err
	}

	cs.Logger.Info(ctx, "Player characters deactivated",
		"game_id", gameID,
		"user_id", userID,
	)

	return nil
}

// DeleteCharacter deletes a character if it has no activity (messages or actions)
// Returns error if character has messages or action submissions
func (cs *CharacterService) DeleteCharacter(ctx context.Context, characterID int32) error {
	defer cs.Logger.LogOperation(ctx, "delete_character", "character_id", characterID)()

	queries := models.New(cs.DB)

	// Verify character exists
	character, err := queries.GetCharacter(ctx, characterID)
	if err != nil {
		cs.Logger.LogError(ctx, err, "Failed to get character for deletion", "character_id", characterID)
		return fmt.Errorf("failed to get character: %w", err)
	}

	// Check if character has any messages
	hasMessages, err := cs.characterHasMessages(ctx, characterID)
	if err != nil {
		cs.Logger.LogError(ctx, err, "Failed to check character messages", "character_id", characterID)
		return fmt.Errorf("failed to check character messages: %w", err)
	}

	if hasMessages {
		cs.Logger.Warn(ctx, "Cannot delete character: has existing messages",
			"character_id", characterID,
			"character_name", character.Name,
			"game_id", character.GameID,
		)
		return fmt.Errorf("cannot delete character with existing messages")
	}

	// Check if character has any action submissions
	hasActions, err := cs.characterHasActionSubmissions(ctx, characterID)
	if err != nil {
		cs.Logger.LogError(ctx, err, "Failed to check character actions", "character_id", characterID)
		return fmt.Errorf("failed to check character actions: %w", err)
	}

	if hasActions {
		cs.Logger.Warn(ctx, "Cannot delete character: has existing action submissions",
			"character_id", characterID,
			"character_name", character.Name,
			"game_id", character.GameID,
		)
		return fmt.Errorf("cannot delete character with existing action submissions")
	}

	// All checks passed - delete character
	// Note: character_data and npc_assignments will CASCADE delete
	err = queries.DeleteCharacter(ctx, character.ID)
	if err != nil {
		cs.Logger.LogError(ctx, err, "Failed to delete character",
			"character_id", characterID,
			"character_name", character.Name,
		)
		return fmt.Errorf("failed to delete character: %w", err)
	}

	cs.Logger.Info(ctx, "Character deleted successfully",
		"character_id", characterID,
		"character_name", character.Name,
		"game_id", character.GameID,
		"character_type", character.CharacterType,
	)

	return nil
}

// characterHasMessages checks if a character has any messages (posts or comments)
func (cs *CharacterService) characterHasMessages(ctx context.Context, characterID int32) (bool, error) {
	queries := models.New(cs.DB)

	// Count messages by this character
	count, err := queries.CountMessagesByCharacter(ctx, characterID)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// characterHasActionSubmissions checks if a character has any action submissions
func (cs *CharacterService) characterHasActionSubmissions(ctx context.Context, characterID int32) (bool, error) {
	queries := models.New(cs.DB)

	// Count action submissions for this character
	count, err := queries.CountActionSubmissionsByCharacter(ctx, pgtype.Int4{Int32: characterID, Valid: true})
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// ============================================================================
// Audience Participation Methods (NPC Assignment)
// ============================================================================

// ListAudienceNPCs retrieves all audience NPCs for a game with assignment information
// Returns NPCs with owner information and current assignment status
func (cs *CharacterService) ListAudienceNPCs(ctx context.Context, gameID int32) ([]models.ListAudienceNPCsRow, error) {
	queries := models.New(cs.DB)

	npcs, err := queries.ListAudienceNPCs(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to list audience NPCs: %w", err)
	}

	return npcs, nil
}

// GetCharacterActivityStats returns public and private message counts for a character.
func (cs *CharacterService) GetCharacterActivityStats(ctx context.Context, characterID int32) (*core.CharacterActivityStats, error) {
	queries := models.New(cs.DB)
	row, err := queries.GetCharacterActivityStats(ctx, characterID)
	if err != nil {
		cs.Logger.LogError(ctx, err, "Failed to get character activity stats", "character_id", characterID)
		return nil, err
	}
	return &core.CharacterActivityStats{
		PublicMessages:  row.PublicMessages,
		PrivateMessages: &row.PrivateMessages,
	}, nil
}

// GetCharacterActivityStatsByGame returns activity stats for every character in a
// game, keyed by character ID, in a single query. Characters with no messages of
// either kind are still present in the result with zero counts.
func (cs *CharacterService) GetCharacterActivityStatsByGame(ctx context.Context, gameID int32) (map[int32]*core.CharacterActivityStats, error) {
	queries := models.New(cs.DB)
	rows, err := queries.GetCharacterActivityStatsByGame(ctx, gameID)
	if err != nil {
		cs.Logger.LogError(ctx, err, "Failed to get character activity stats by game", "game_id", gameID)
		return nil, err
	}

	statsByCharacterID := make(map[int32]*core.CharacterActivityStats, len(rows))
	for _, row := range rows {
		row := row
		statsByCharacterID[row.CharacterID] = &core.CharacterActivityStats{
			PublicMessages:  row.PublicMessages,
			PrivateMessages: &row.PrivateMessages,
		}
	}
	return statsByCharacterID, nil
}

// AssignNPCToAudience assigns an NPC character to an audience member
// Creates or updates the NPC assignment record
func (cs *CharacterService) AssignNPCToAudience(ctx context.Context, characterID, assignedUserID, assignedByUserID int32) (*models.NpcAssignment, error) {
	defer cs.Logger.LogOperation(ctx, "assign_npc_to_audience",
		"character_id", characterID,
		"assigned_user_id", assignedUserID,
		"assigned_by_user_id", assignedByUserID,
	)()

	queries := models.New(cs.DB)

	// Verify this is an NPC
	character, err := queries.GetCharacter(ctx, characterID)
	if err != nil {
		cs.Logger.LogError(ctx, err, "Failed to get character for audience assignment",
			"character_id", characterID,
		)
		return nil, fmt.Errorf("failed to get character: %w", err)
	}

	if character.CharacterType != "npc" {
		cs.Logger.Warn(ctx, "Attempted to assign non-NPC character to audience",
			"character_id", characterID,
			"character_type", character.CharacterType,
		)
		return nil, fmt.Errorf("character is not an NPC (type: %s)", character.CharacterType)
	}

	// Create or update the assignment
	assignment, err := queries.AssignNPCToAudience(ctx, models.AssignNPCToAudienceParams{
		CharacterID:      characterID,
		AssignedUserID:   assignedUserID,
		AssignedByUserID: assignedByUserID,
	})

	if err != nil {
		cs.Logger.LogError(ctx, err, "Failed to assign NPC to audience",
			"character_id", characterID,
			"assigned_user_id", assignedUserID,
		)
		return nil, fmt.Errorf("failed to assign NPC to audience: %w", err)
	}

	cs.Logger.Info(ctx, "NPC assigned to audience member",
		"character_id", characterID,
		"character_name", character.Name,
		"assigned_user_id", assignedUserID,
		"assigned_by_user_id", assignedByUserID,
	)

	return &assignment, nil
}
