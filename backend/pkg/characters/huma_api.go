package characters

// Huma (type-first) implementation of the character API.
//
// Two registration functions, because characters are mounted at two prefixes:
// the roster routes under /games/{gameID}, and the per-character operations at
// /characters. See .claude/planning/huma-migration.md gotcha 10.

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	"actionphase/pkg/observability"
)

// Input / output types

type gameIDInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
}

type characterIDInput struct {
	ID int32 `path:"id" doc:"Character ID"`
}

type createCharacterInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
	Body   *CreateCharacterRequest
}

type approveCharacterInput struct {
	ID   int32 `path:"id" doc:"Character ID"`
	Body *ApproveCharacterRequest
}

type assignNPCInput struct {
	ID   int32 `path:"id" doc:"Character ID"`
	Body *AssignNPCRequest
}

type reassignCharacterInput struct {
	ID   int32 `path:"id" doc:"Character ID"`
	Body *ReassignCharacterRequest
}

type renameCharacterInput struct {
	ID   int32 `path:"id" doc:"Character ID"`
	Body *RenameCharacterRequest
}

type setCharacterDataInput struct {
	ID   int32 `path:"id" doc:"Character ID"`
	Body *CharacterDataRequest
}

type characterOutput struct {
	Body *CharacterResponse
}

type gameCharacterListOutput struct {
	Body []*GameCharacterResponse
}

type controllableListOutput struct {
	Body []*ControllableCharacterResponse
}

type controllableWithGameListOutput struct {
	Body []*ControllableCharacterWithGameResponse
}

type inactiveListOutput struct {
	Body []*InactiveCharacterResponse
}

type characterDataListOutput struct {
	Body []*CharacterDataResponse
}

type characterStatsOutput struct {
	Body *CharacterStatsResponse
}

// gameCharacterStatsOutput is the batch stats body: an object keyed by
// character ID as a string, not an array. Kept as a map because the frontend
// indexes into it directly.
type gameCharacterStatsOutput struct {
	Body map[string]*CharacterStatsResponse
}

// gameCharacterDataOutput is the batch sheet body: an object keyed by character
// ID as a string, matching gameCharacterStatsOutput so the frontend indexes
// both the same way. Characters the caller may see nothing of are still
// present, with an empty list, so a consumer can tell "no data" from "not in
// this game".
type gameCharacterDataOutput struct {
	Body map[string][]*CharacterDataResponse
}

// Helpers

// humaErr converts a core error response into the equivalent huma error,
// preserving the status and message the chi handlers produced.
func humaErr(errResp any) error {
	if resp, ok := errResp.(*core.ErrResponse); ok {
		return huma.NewError(resp.HTTPStatusCode, resp.ErrorText)
	}
	return huma.Error500InternalServerError("unexpected error")
}

// authUser returns the authenticated caller, or the 401 the chi handlers sent
// when the middleware had not populated one.
func (h *Handler) authUser(ctx context.Context) (*core.AuthenticatedUser, error) {
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.App.ObsLogger.Warn(ctx, "No authenticated user found")
		return nil, huma.Error401Unauthorized("authentication required")
	}
	return authUser, nil
}

// loadCharacterGame resolves a character and its game together, which every
// per-character operation needs before it can decide anything.
//
// A missing character answers 404. The game lookup underneath deliberately
// stays 500: a character row pointing at a game that does not exist is data
// corruption, not something the caller got wrong, and reporting it as 404
// would tell the client to stop retrying a bug on our side.
func (h *Handler) loadCharacterGame(ctx context.Context, characterID int32) (*models.Character, *models.Game, error) {
	character, err := h.CharacterService.GetCharacter(ctx, characterID)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get character", "error", err, "character_id", characterID)
		return nil, nil, core.NotFoundOr500(err, "character")
	}

	game, err := h.GameService.GetGame(ctx, character.GameID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get game", "error", err, "game_id", character.GameID)
		return nil, nil, huma.Error500InternalServerError(err.Error())
	}
	return character, game, nil
}

// ptrText / ptrInt / ptrBool unwrap a nullable column into the pointer the
// response structs use, so a NULL stays absent from the JSON rather than
// becoming a zero value.
func ptrText(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func ptrInt(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	n := v.Int32
	return &n
}

func ptrBool(v pgtype.Bool) *bool {
	if !v.Valid {
		return nil
	}
	b := v.Bool
	return &b
}

// canSeePlayerNames reports whether the caller may see who is behind a
// character. GMs, co-GMs and audience always can; in an anonymous game regular
// players cannot.
func canSeePlayerNames(isAnonymous bool, role string) bool {
	if !isAnonymous {
		return true
	}
	return role == "gm" || role == "co_gm" || role == "audience"
}

// resolveUserRole reports the caller's role in a game, defaulting to "player"
// for an authenticated user who is not listed as a participant.
func (h *Handler) resolveUserRole(ctx context.Context, gameID, userID int32, isGM bool) string {
	if isGM {
		return "gm"
	}
	participants, err := h.GameService.GetGameParticipants(ctx, gameID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get game participants", "error", err, "game_id", gameID)
		// Matching the chi handler: a lookup failure downgrades to the least
		// privileged role rather than failing the request.
		return "player"
	}
	for _, p := range participants {
		if p.UserID == userID {
			return p.Role
		}
	}
	return "player"
}

// Character CRUD

func (h *Handler) humaCreateCharacter(ctx context.Context, in *createCharacterInput) (*characterOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_create_character")()

	authUser, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	// The chi route was wrapped in RequireEmailVerificationMiddleware. Huma
	// handlers take a context rather than a *http.Request, so the same check
	// runs inline here via its context-based twin.
	if errResp := core.RequireVerifiedEmailCtx(ctx, h.App.Pool); errResp != nil {
		h.App.ObsLogger.Warn(ctx, "Create character blocked by email verification", "user_id", authUser.ID)
		return nil, humaErr(errResp)
	}

	// GameMiddleware resolves this for the whole /games/{gameID} subtree.
	isGM, _ := ctx.Value("is_gm").(bool)

	if in.Body.CharacterType == "player_character" {
		// A GM may create a player character for anyone; a player only for
		// themselves, and only if they actually play in this game.
		if !isGM {
			participants, err := h.GameService.GetGameParticipants(ctx, in.GameID)
			if err != nil {
				h.App.ObsLogger.Error(ctx, "Failed to get game participants", "error", err, "game_id", in.GameID)
				return nil, huma.Error500InternalServerError(err.Error())
			}

			isParticipant := false
			for _, participant := range participants {
				if participant.UserID == authUser.ID && participant.Role == "player" {
					isParticipant = true
					break
				}
			}

			if !isParticipant {
				h.App.ObsLogger.Warn(ctx, "Create character forbidden", "user_id", authUser.ID, "game_id", in.GameID)
				return nil, huma.Error403Forbidden("only game participants can create player characters")
			}
		}

		// The GM is naming someone else's character, so they must say whose.
		if isGM && in.Body.UserID == nil {
			h.App.ObsLogger.Warn(ctx, "Invalid create character request", "game_id", in.GameID)
			return nil, huma.Error400BadRequest("user_id is required when GM creates player characters")
		}
	} else if !isGM {
		// NPCs are GM-only.
		h.App.ObsLogger.Warn(ctx, "Create character forbidden", "user_id", authUser.ID, "game_id", in.GameID)
		return nil, huma.Error403Forbidden("only the GM can create NPCs")
	}

	var reqUserID *int32
	if in.Body.CharacterType == "player_character" {
		if isGM {
			reqUserID = in.Body.UserID
		} else {
			reqUserID = &authUser.ID
		}
	}
	// For NPCs, UserID stays nil (GM-controlled) until assigned.

	character, err := h.CharacterService.CreateCharacter(ctx, core.CreateCharacterRequest{
		GameID:        in.GameID,
		UserID:        reqUserID,
		Name:          in.Body.Name,
		CharacterType: in.Body.CharacterType,
	})
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to create character", "error", err, "game_id", in.GameID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	// Create always reports the type, even in an anonymous game: the caller
	// just supplied it.
	charType := character.CharacterType
	resp := &CharacterResponse{
		ID:            character.ID,
		GameID:        character.GameID,
		Name:          character.Name,
		CharacterType: &charType,
		Status:        character.Status.String,
		CreatedAt:     character.CreatedAt.Time,
		UpdatedAt:     character.UpdatedAt.Time,
		UserID:        ptrInt(character.UserID),
	}

	return &characterOutput{Body: resp}, nil
}

func (h *Handler) humaGetCharacter(ctx context.Context, in *characterIDInput) (*characterOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_character")()

	authUser, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	character, game, err := h.loadCharacterGame(ctx, in.ID)
	if err != nil {
		return nil, err
	}

	isGM := core.IsUserGameMasterCtx(ctx, authUser.ID, authUser.IsAdmin, *game, h.App.Pool)
	isOwner := character.UserID.Valid && character.UserID.Int32 == authUser.ID
	userRole := h.resolveUserRole(ctx, character.GameID, authUser.ID, isGM)

	// An audience member assigned an NPC controls it, so they may see it even
	// before approval.
	isAssignedUser := false
	if character.CharacterType == "npc" {
		queries := models.New(h.App.Pool)
		if assignment, err := queries.GetNPCAssignment(ctx, character.ID); err == nil {
			isAssignedUser = assignment.AssignedUserID == authUser.ID
		}
	}

	// Hide other players' unapproved characters once a game is running: their
	// existence is itself information. Owners and assigned controllers still
	// see their own.
	if game.State.String == "in_progress" && !isGM && !isOwner && !isAssignedUser {
		if character.Status.String == "pending" || character.Status.String == "rejected" {
			h.App.ObsLogger.Warn(ctx, "Get character not found", "character_id", in.ID)
			return nil, huma.Error404NotFound("character not found")
		}
	}

	resp := &CharacterResponse{
		ID:        character.ID,
		GameID:    character.GameID,
		Name:      character.Name,
		Status:    character.Status.String,
		CreatedAt: character.CreatedAt.Time,
		UpdatedAt: character.UpdatedAt.Time,
		UserID:    ptrInt(character.UserID),
		AvatarURL: ptrText(character.AvatarUrl),
	}

	// In an anonymous game the type would leak whether a character is a player's
	// or the GM's, so regular players do not get it.
	if !game.IsAnonymous || userRole == "gm" || userRole == "co_gm" || userRole == "audience" {
		charType := character.CharacterType
		resp.CharacterType = &charType
	}

	return &characterOutput{Body: resp}, nil
}

func (h *Handler) humaGetGameCharacters(ctx context.Context, in *gameIDInput) (*gameCharacterListOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_game_characters")()

	authUser, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	game, ok := ctx.Value("game").(*models.Game)
	if !ok {
		return nil, huma.Error500InternalServerError("game not resolved")
	}

	isGM, _ := ctx.Value("is_gm").(bool)
	userRole := h.resolveUserRole(ctx, in.GameID, authUser.ID, isGM)

	characters, err := h.CharacterService.GetCharactersByGame(ctx, in.GameID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get game characters", "error", err, "game_id", in.GameID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	showNames := canSeePlayerNames(game.IsAnonymous, userRole)
	privileged := isGM || userRole == "co_gm" || userRole == "audience"

	// Built as an empty slice rather than a nil one so a game with no
	// characters encodes as [] rather than null.
	resp := make([]*GameCharacterResponse, 0, len(characters))
	for _, char := range characters {
		// Unapproved characters belonging to *other* players stay hidden from
		// regular players; the caller's own are always included.
		if !privileged && (char.Status.String == "pending" || char.Status.String == "rejected") {
			if !char.UserID.Valid || char.UserID.Int32 != authUser.ID {
				continue
			}
		}

		item := &GameCharacterResponse{
			ID:            char.ID,
			GameID:        char.GameID,
			Name:          char.Name,
			CharacterType: char.CharacterType,
			Status:        ptrText(char.Status),
			IsActive:      char.IsActive,
			CreatedAt:     char.CreatedAt.Time,
			UpdatedAt:     char.UpdatedAt.Time,
			// The portrait belongs to the character, not the player, so it
			// survives anonymous mode.
			AvatarURL: ptrText(char.AvatarUrl),
		}

		if showNames {
			item.UserID = ptrInt(char.UserID)
			item.Username = ptrText(char.OwnerUsername)
			item.AssignedUserID = ptrInt(char.AssignedUserID)
			item.AssignedUsername = ptrText(char.AssignedUsername)
		}

		resp = append(resp, item)
	}

	return &gameCharacterListOutput{Body: resp}, nil
}

func (h *Handler) humaGetUserControllableCharacters(ctx context.Context, in *gameIDInput) (*controllableListOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_user_controllable_characters")()

	authUser, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	characters, err := h.CharacterService.GetUserControllableCharacters(ctx, in.GameID, authUser.ID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get user controllable characters", "error", err, "game_id", in.GameID, "user_id", authUser.ID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	resp := make([]*ControllableCharacterResponse, 0, len(characters))
	for _, char := range characters {
		resp = append(resp, &ControllableCharacterResponse{
			ID:            char.ID,
			GameID:        char.GameID,
			Name:          char.Name,
			CharacterType: char.CharacterType,
			CreatedAt:     char.CreatedAt.Time,
			UpdatedAt:     char.UpdatedAt.Time,
			UserID:        ptrInt(char.UserID),
			Status:        ptrText(char.Status),
			AvatarURL:     ptrText(char.AvatarUrl),
		})
	}

	return &controllableListOutput{Body: resp}, nil
}

// humaGetUserControllableCharactersAcrossGames returns every character the
// current user can control across all their in_progress games. Unlike the
// per-game endpoint this takes no game in the path, so it can back surfaces
// with no game in scope (the global Utility Drawer).
//
// Each entry carries its game context (title, state, flags) and the user's role
// in that game, which the client needs to resolve sheet permissions without a
// request per game.
func (h *Handler) humaGetUserControllableCharactersAcrossGames(ctx context.Context, _ *struct{}) (*controllableWithGameListOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_user_controllable_characters_across_games")()

	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get user from token")
		return nil, humaErr(errResp)
	}

	characters, err := h.CharacterService.GetUserControllableCharactersAcrossGames(ctx, userID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get cross-game controllable characters", "error", err, "user_id", userID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	resp := make([]*ControllableCharacterWithGameResponse, 0, len(characters))
	for _, char := range characters {
		item := &ControllableCharacterWithGameResponse{
			ControllableCharacterResponse: ControllableCharacterResponse{
				ID:            char.ID,
				GameID:        char.GameID,
				Name:          char.Name,
				CharacterType: char.CharacterType,
				CreatedAt:     char.CreatedAt.Time,
				UpdatedAt:     char.UpdatedAt.Time,
				UserID:        ptrInt(char.UserID),
				Status:        ptrText(char.Status),
				AvatarURL:     ptrText(char.AvatarUrl),
			},
			GameTitle:           char.GameTitle,
			GameState:           ptrText(char.GameState),
			GameIsAnonymous:     char.GameIsAnonymous,
			GamePortraitAvatars: char.GamePortraitAvatars,
			// Absent means "all defaults", which the frontend owns. The drawer
			// has no game in scope, so this is its only source for the labels --
			// without it a game that renamed a tab would render the default name
			// here and read as a bug.
			GameCharacterSheet: core.CharacterSheetConfigForResponse(char.GameCharacterSheet),
			// Who plays each character, for the GM's cast list. Named `username`
			// to match the per-game payload the drawer's in-game list reads, so
			// one row renderer serves both.
			Username:         ptrText(char.OwnerUsername),
			AssignedUsername: ptrText(char.AssignedUsername),
			UserRole:         char.UserRole,
		}
		resp = append(resp, item)
	}

	return &controllableWithGameListOutput{Body: resp}, nil
}

func (h *Handler) humaDeleteCharacter(ctx context.Context, in *characterIDInput) (*struct{}, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_character")()

	authUser, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	character, err := h.CharacterService.GetCharacter(ctx, in.ID)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get character", "error", err, "character_id", in.ID)
		return nil, huma.Error404NotFound("character not found")
	}

	game, err := h.GameService.GetGame(ctx, character.GameID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get game", "error", err, "game_id", character.GameID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	if !core.IsUserGameMasterCtx(ctx, authUser.ID, authUser.IsAdmin, *game, h.App.Pool) {
		h.App.ObsLogger.Warn(ctx, "Delete character forbidden", "user_id", authUser.ID, "character_id", in.ID)
		return nil, huma.Error403Forbidden("only the GM can delete characters")
	}

	if err := h.CharacterService.DeleteCharacter(ctx, in.ID); err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to delete character", "error", err, "character_id", in.ID)
		// A character that has spoken or acted cannot be deleted; that is the
		// caller's problem to fix, not a server fault.
		if err.Error() == "cannot delete character with existing messages" ||
			err.Error() == "cannot delete character with existing action submissions" {
			return nil, huma.Error400BadRequest(err.Error())
		}
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return nil, nil
}

// Character management

func (h *Handler) humaApproveCharacter(ctx context.Context, in *approveCharacterInput) (*characterOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_approve_character")()

	authUser, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	_, game, err := h.loadCharacterGame(ctx, in.ID)
	if err != nil {
		return nil, err
	}

	if !core.IsUserGameMasterCtx(ctx, authUser.ID, authUser.IsAdmin, *game, h.App.Pool) {
		h.App.ObsLogger.Warn(ctx, "Approve character forbidden", "user_id", authUser.ID, "character_id", in.ID)
		return nil, huma.Error403Forbidden("only the GM can approve characters")
	}

	updated, err := h.CharacterService.ApproveCharacter(ctx, in.ID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to update character status", "error", err, "character_id", in.ID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	// Off the request path: a notification failure must not fail the approval
	// that already committed.
	if updated.UserID.Valid {
		notifSvc := h.NotificationService
		observability.SafeGo(context.Background(), h.App.ObsLogger, "notify-character-approved", func() {
			notifCtx := context.Background()
			if err := notifSvc.NotifyCharacterApproved(notifCtx, updated.UserID.Int32, updated.GameID, updated.ID, updated.Name); err != nil {
				h.App.ObsLogger.Warn(notifCtx, "Failed to send character approved notification", "error", err, "character_id", updated.ID)
			}
		})
	}

	return &characterOutput{Body: characterFromModel(updated)}, nil
}

func (h *Handler) humaAssignNPC(ctx context.Context, in *assignNPCInput) (*struct{}, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_assign_npc")()

	authUser, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	character, game, err := h.loadCharacterGame(ctx, in.ID)
	if err != nil {
		return nil, err
	}

	if !core.IsUserGameMasterCtx(ctx, authUser.ID, authUser.IsAdmin, *game, h.App.Pool) {
		h.App.ObsLogger.Warn(ctx, "Assign NPC forbidden", "user_id", authUser.ID, "character_id", in.ID)
		return nil, huma.Error403Forbidden("only the GM can assign NPCs")
	}

	// NPCs go to audience members. Assigning to oneself is how a GM takes an
	// NPC back, so it skips the audience check.
	if in.Body.AssignedUserID != authUser.ID {
		participants, err := h.GameService.GetGameParticipants(ctx, character.GameID)
		if err != nil {
			h.App.ObsLogger.Error(ctx, "Failed to get game participants", "error", err, "game_id", character.GameID)
			return nil, huma.Error500InternalServerError(err.Error())
		}

		isAudience := false
		for _, participant := range participants {
			if participant.UserID == in.Body.AssignedUserID && participant.Role == "audience" {
				isAudience = true
				break
			}
		}

		if !isAudience {
			h.App.ObsLogger.Warn(ctx, "Bad assign NPC request", "assigned_user_id", in.Body.AssignedUserID, "game_id", character.GameID)
			return nil, huma.Error400BadRequest("NPCs can only be assigned to audience members")
		}
	}

	if err := h.CharacterService.AssignNPCToUser(ctx, in.ID, in.Body.AssignedUserID, authUser.ID); err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to assign NPC", "error", err, "character_id", in.ID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return nil, nil
}

func (h *Handler) humaReassignCharacter(ctx context.Context, in *reassignCharacterInput) (*characterOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_reassign_character")()

	authUser, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	character, err := h.CharacterService.GetCharacter(ctx, in.ID)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get character", "error", err, "character_id", in.ID)
		return nil, huma.Error404NotFound("character not found")
	}

	game, err := h.GameService.GetGame(ctx, character.GameID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get game", "error", err, "game_id", character.GameID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	if !core.IsUserGameMasterCtx(ctx, authUser.ID, authUser.IsAdmin, *game, h.App.Pool) {
		h.App.ObsLogger.Warn(ctx, "Reassign character forbidden", "user_id", authUser.ID, "character_id", in.ID)
		return nil, huma.Error403Forbidden("only the GM can reassign characters")
	}

	// Reassignment is how an abandoned character finds a new player, so it only
	// applies once the current owner has gone inactive.
	if character.IsActive {
		h.App.ObsLogger.Warn(ctx, "Reassign character conflict", "character_id", in.ID)
		return nil, huma.Error409Conflict("can only reassign inactive characters")
	}

	updated, err := h.CharacterService.ReassignCharacter(ctx, in.ID, in.Body.NewOwnerUserID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to reassign character", "error", err, "character_id", in.ID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	h.App.ObsLogger.Info(ctx, "Character reassigned", "character_id", in.ID, "new_owner", in.Body.NewOwnerUserID, "reassigned_by", authUser.ID)

	return &characterOutput{Body: characterFromModel(updated)}, nil
}

func (h *Handler) humaRenameCharacter(ctx context.Context, in *renameCharacterInput) (*characterOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_rename_character")()

	authUser, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	// Renaming is allowed to the owner as well as the GM, so this is an
	// edit-permission check rather than a GM check.
	canEdit, err := h.CharacterService.CanUserEditCharacter(ctx, in.ID, authUser.ID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to check character edit permission", "error", err, "character_id", in.ID)
		return nil, huma.Error500InternalServerError(err.Error())
	}
	if !canEdit {
		h.App.ObsLogger.Warn(ctx, "Character rename permission denied", "character_id", in.ID, "user_id", authUser.ID)
		return nil, huma.Error403Forbidden("you do not have permission to rename this character")
	}

	updated, err := h.CharacterService.RenameCharacter(ctx, in.ID, in.Body.Name)
	if err != nil {
		// Names are unique within a game, so a collision is the caller's to
		// resolve rather than a server fault.
		if err.Error() == fmt.Sprintf("a character named '%s' already exists in this game", in.Body.Name) {
			h.App.ObsLogger.Warn(ctx, "Rename character conflict", "error", err.Error())
			return nil, huma.Error409Conflict(err.Error())
		}
		h.App.ObsLogger.Error(ctx, "Failed to rename character", "error", err, "character_id", in.ID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	h.App.ObsLogger.Info(ctx, "Character renamed successfully", "character_id", in.ID, "new_name", in.Body.Name, "renamed_by", authUser.ID)

	return &characterOutput{Body: characterFromModel(updated)}, nil
}

func (h *Handler) humaListInactiveCharacters(ctx context.Context, in *gameIDInput) (*inactiveListOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_list_inactive_characters")()

	if isGM, _ := ctx.Value("is_gm").(bool); !isGM {
		h.App.ObsLogger.Warn(ctx, "List inactive characters forbidden", "game_id", in.GameID)
		return nil, huma.Error403Forbidden("only the GM can view inactive characters")
	}

	characters, err := h.CharacterService.ListInactiveCharacters(ctx, in.GameID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to list inactive characters", "error", err, "game_id", in.GameID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	resp := make([]*InactiveCharacterResponse, 0, len(characters))
	for _, char := range characters {
		resp = append(resp, &InactiveCharacterResponse{
			ID:                    char.ID,
			GameID:                char.GameID,
			Name:                  char.Name,
			CharacterType:         char.CharacterType,
			Status:                char.Status.String,
			IsActive:              char.IsActive,
			CreatedAt:             char.CreatedAt.Time,
			UpdatedAt:             char.UpdatedAt.Time,
			CurrentOwnerUsername:  ptrText(char.CurrentOwnerUsername),
			OriginalOwnerUsername: ptrText(char.OriginalOwnerUsername),
			UserID:                ptrInt(char.UserID),
			OriginalOwnerUserID:   ptrInt(char.OriginalOwnerUserID),
		})
	}

	return &inactiveListOutput{Body: resp}, nil
}

// Character sheet data

func (h *Handler) humaSetCharacterData(ctx context.Context, in *setCharacterDataInput) (*struct{}, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_set_character_data")()

	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get user from token")
		return nil, humaErr(errResp)
	}

	canEdit, err := h.CharacterService.CanUserEditCharacter(ctx, in.ID, userID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to check character edit permission", "error", err, "character_id", in.ID)
		return nil, huma.Error500InternalServerError(err.Error())
	}
	if !canEdit {
		h.App.ObsLogger.Warn(ctx, "Character edit permission denied", "character_id", in.ID, "user_id", userID)
		return nil, huma.Error403Forbidden("you cannot edit this character")
	}

	// Stats are the GM's to set even on a character the player otherwise owns:
	// they are game balance, not self-description.
	isStatField := (in.Body.ModuleType == "skills" && in.Body.FieldName == "skills") ||
		(in.Body.ModuleType == "inventory" && in.Body.FieldName == "items") ||
		(in.Body.ModuleType == "numbers" && in.Body.FieldName == "numbers")

	if isStatField {
		queries := models.New(h.App.Pool)
		character, err := queries.GetCharacter(ctx, in.ID)
		if err != nil {
			h.App.ObsLogger.Error(ctx, "Failed to get character for GM check", "error", err, "character_id", in.ID)
			return nil, huma.Error500InternalServerError(err.Error())
		}

		game, err := queries.GetGame(ctx, character.GameID)
		if err != nil {
			h.App.ObsLogger.Error(ctx, "Failed to get game for GM check", "error", err, "game_id", character.GameID)
			return nil, huma.Error500InternalServerError(err.Error())
		}

		if game.GmUserID != userID && !core.IsUserCoGM(ctx, h.App.Pool, character.GameID, userID) {
			h.App.ObsLogger.Warn(ctx, "Character stats edit permission denied", "character_id", in.ID, "user_id", userID, "game_id", character.GameID)
			return nil, huma.Error403Forbidden("only GMs and Co-GMs can edit character stats (skills, items, numbers)")
		}
	}

	err = h.CharacterService.SetCharacterData(ctx, core.CharacterDataRequest{
		CharacterID: in.ID,
		ModuleType:  in.Body.ModuleType,
		FieldName:   in.Body.FieldName,
		FieldValue:  in.Body.FieldValue,
		FieldType:   in.Body.FieldType,
		IsPublic:    in.Body.IsPublic,
	})
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to set character data", "error", err, "character_id", in.ID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return nil, nil
}

func (h *Handler) humaGetCharacterData(ctx context.Context, in *characterIDInput) (*characterDataListOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_character_data")()

	// The token is optional here: an unauthenticated caller still gets the
	// public fields.
	var userID *int32
	if id, errResp := core.GetUserIDFromJWT(ctx, h.UserService); errResp == nil {
		userID = &id
	}

	canViewPrivate := h.canViewPrivateCharacterData(ctx, in.ID, userID)

	var characterData []models.CharacterDatum
	var err error
	if canViewPrivate {
		characterData, err = h.CharacterService.GetCharacterData(ctx, in.ID)
		if err != nil {
			h.App.ObsLogger.Error(ctx, "Failed to get character data", "error", err, "character_id", in.ID)
			return nil, huma.Error500InternalServerError(err.Error())
		}
	} else {
		characterData, err = h.CharacterService.GetPublicCharacterData(ctx, in.ID)
		if err != nil {
			h.App.ObsLogger.Error(ctx, "Failed to get public character data", "error", err, "character_id", in.ID)
			return nil, huma.Error500InternalServerError(err.Error())
		}
	}

	resp := make([]*CharacterDataResponse, 0, len(characterData))
	for _, data := range characterData {
		resp = append(resp, &CharacterDataResponse{
			ID:          data.ID,
			CharacterID: data.CharacterID,
			ModuleType:  data.ModuleType,
			FieldName:   data.FieldName,
			FieldType:   ptrText(data.FieldType),
			CreatedAt:   data.CreatedAt.Time,
			UpdatedAt:   data.UpdatedAt.Time,
			FieldValue:  ptrText(data.FieldValue),
			IsPublic:    ptrBool(data.IsPublic),
		})
	}

	return &characterDataListOutput{Body: resp}, nil
}

// canViewPrivateCharacterData reports whether the caller may read a character's
// private sheet fields: editors always, audience members always (they need the
// secrets to spectate meaningfully), and every participant once the game is a
// public archive.
func (h *Handler) canViewPrivateCharacterData(ctx context.Context, characterID int32, userID *int32) bool {
	if userID == nil {
		return false
	}

	if canEdit, err := h.CharacterService.CanUserEditCharacter(ctx, characterID, *userID); err == nil && canEdit {
		return true
	}

	queries := models.New(h.App.Pool)
	character, err := queries.GetCharacter(ctx, characterID)
	if err != nil {
		return false
	}

	game, gameErr := queries.GetGame(ctx, character.GameID)
	userRole, roleErr := h.GameService.GetUserRole(ctx, character.GameID, *userID)
	if roleErr != nil {
		return false
	}

	if userRole == "audience" {
		h.App.ObsLogger.Debug(ctx, "Audience member viewing character data",
			"character_id", characterID, "user_id", *userID, "game_id", character.GameID)
		return true
	}

	// Completed or epilogue: the archive is open to everyone who was there.
	if gameErr == nil && game.State.Valid && core.IsPublicArchive(game.State.String) {
		h.App.ObsLogger.Debug(ctx, "Participant viewing character data in completed game",
			"character_id", characterID, "user_id", *userID, "game_id", character.GameID, "role", userRole)
		return true
	}

	return false
}

// Activity stats

func (h *Handler) humaGetCharacterStats(ctx context.Context, in *characterIDInput) (*characterStatsOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_character_stats")()

	character, err := h.CharacterService.GetCharacter(ctx, in.ID)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get character for stats", "error", err, "character_id", in.ID)
		return nil, huma.Error404NotFound("character not found")
	}

	game, err := h.GameService.GetGame(ctx, character.GameID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get game for stats", "error", err, "game_id", character.GameID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	authUser := core.GetAuthenticatedUser(ctx)
	gameLevelAccess := h.gameLevelPrivateStatsAccess(ctx, authUser, *game)
	canSeePrivate := canSeeCharacterPrivateStats(gameLevelAccess, authUser, character.UserID)

	stats, err := h.CharacterService.GetCharacterActivityStats(ctx, in.ID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get character activity stats", "error", err, "character_id", in.ID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	resp := &CharacterStatsResponse{PublicMessages: stats.PublicMessages}
	if canSeePrivate {
		resp.PrivateMessages = stats.PrivateMessages
	}

	return &characterStatsOutput{Body: resp}, nil
}

func (h *Handler) humaGetGameCharacterStats(ctx context.Context, in *gameIDInput) (*gameCharacterStatsOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_game_character_stats")()

	game, ok := ctx.Value("game").(*models.Game)
	if !ok {
		return nil, huma.Error500InternalServerError("game not resolved")
	}

	characters, err := h.CharacterService.GetCharactersByGame(ctx, in.GameID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get game characters for stats", "error", err, "game_id", in.GameID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	authUser := core.GetAuthenticatedUser(ctx)
	// Game-level access is the same for every character, so compute it once
	// rather than re-running the DB lookups per roster member.
	gameLevelAccess := h.gameLevelPrivateStatsAccess(ctx, authUser, *game)

	statsByCharacterID, err := h.CharacterService.GetCharacterActivityStatsByGame(ctx, in.GameID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get game character activity stats", "error", err, "game_id", in.GameID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	resp := make(map[string]*CharacterStatsResponse, len(characters))
	for _, char := range characters {
		stats, ok := statsByCharacterID[char.ID]
		if !ok {
			// No messages of either kind for this character.
			stats = &core.CharacterActivityStats{}
		}

		charResp := &CharacterStatsResponse{PublicMessages: stats.PublicMessages}
		if canSeeCharacterPrivateStats(gameLevelAccess, authUser, char.UserID) {
			charResp.PrivateMessages = stats.PrivateMessages
		}
		resp[strconv.Itoa(int(char.ID))] = charResp
	}

	return &gameCharacterStatsOutput{Body: resp}, nil
}

// humaGetGameCharacterData returns every character's sheet rows for one game in
// a single response, filtered per character to what the caller may see.
//
// This exists to replace a request per character. A phase drill-down in History
// can show a whole cast's action content, each with [[item]] references that
// need that character's sheet to resolve; fetching them one at a time meant N
// requests fired at once the moment a phase was opened.
//
// Disclosure is decided by canViewPrivateCharacterData, the same helper the
// single-character endpoint uses — deliberately not a batch-specific reimplementation
// of the rule. Two copies of a visibility check are free to drift, and the way that
// failure shows up is a private sheet leaking through whichever endpoint was
// forgotten. Callers that may not see a character's private rows get exactly what
// GET /characters/{id}/data would give them: the is_public rows only.
func (h *Handler) humaGetGameCharacterData(ctx context.Context, in *gameIDInput) (*gameCharacterDataOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_game_character_data")()

	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}
	userID := authUser.ID

	// The game read gate runs first: without it a non-participant could probe a
	// private game's roster, even if every row came back filtered.
	canView, err := h.GameService.CanUserViewGame(ctx, in.GameID, userID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to check game view access", "error", err, "game_id", in.GameID, "user_id", userID)
		return nil, huma.Error500InternalServerError(err.Error())
	}
	if !canView {
		return nil, huma.Error403Forbidden("you do not have permission to view this game's content")
	}

	characters, err := h.CharacterService.GetCharactersByGame(ctx, in.GameID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get game characters", "error", err, "game_id", in.GameID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	// One query for the whole cast's rows, then partitioned in memory. The
	// alternative -- a query per character -- is the N+1 this endpoint exists to
	// remove.
	rows, err := models.New(h.App.Pool).GetCharacterDataByGame(ctx, in.GameID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get game character data", "error", err, "game_id", in.GameID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	rowsByCharacter := make(map[int32][]models.GetCharacterDataByGameRow, len(characters))
	for _, row := range rows {
		rowsByCharacter[row.CharacterID] = append(rowsByCharacter[row.CharacterID], row)
	}

	resp := make(map[string][]*CharacterDataResponse, len(characters))
	for _, char := range characters {
		canViewPrivate := h.canViewPrivateCharacterData(ctx, char.ID, &userID)

		// Always a non-nil slice: an empty JSON array reads as "nothing to show
		// for this character", where null invites a consumer to crash on it.
		out := make([]*CharacterDataResponse, 0, len(rowsByCharacter[char.ID]))
		for _, row := range rowsByCharacter[char.ID] {
			if !canViewPrivate && !(row.IsPublic.Valid && row.IsPublic.Bool) {
				continue
			}
			out = append(out, &CharacterDataResponse{
				ID:          row.ID,
				CharacterID: row.CharacterID,
				ModuleType:  row.ModuleType,
				FieldName:   row.FieldName,
				FieldType:   ptrText(row.FieldType),
				CreatedAt:   row.CreatedAt.Time,
				UpdatedAt:   row.UpdatedAt.Time,
				FieldValue:  ptrText(row.FieldValue),
				IsPublic:    ptrBool(row.IsPublic),
			})
		}
		resp[strconv.Itoa(int(char.ID))] = out
	}

	return &gameCharacterDataOutput{Body: resp}, nil
}

// characterFromModel builds the single-character body from a full character
// row. Used by the operations that always disclose the type, because the caller
// is a GM or the character's owner.
func characterFromModel(c *models.Character) *CharacterResponse {
	charType := c.CharacterType
	return &CharacterResponse{
		ID:            c.ID,
		GameID:        c.GameID,
		Name:          c.Name,
		CharacterType: &charType,
		Status:        c.Status.String,
		CreatedAt:     c.CreatedAt.Time,
		UpdatedAt:     c.UpdatedAt.Time,
		UserID:        ptrInt(c.UserID),
	}
}

// Registration

// RegisterHumaGameCharacters registers the roster operations. Paths are
// relative to the /games/{gameID} subrouter.
func RegisterHumaGameCharacters(api huma.API, h *Handler) {
	bearer := []map[string][]string{{"BearerAuth": {}}}

	huma.Register(api, huma.Operation{
		OperationID: "createCharacter",
		Method:      http.MethodPost,
		Path:        "/characters",
		Summary:     "Create a character",
		Description: "Creates a player character or NPC. Players may create their own player " +
			"characters; NPCs and characters for other players are GM-only. Requires a verified email.",
		Tags:          []string{"Characters"},
		Security:      bearer,
		DefaultStatus: http.StatusCreated,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"400": {Description: "Invalid body, or user_id missing when a GM creates a player character"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not allowed to create this kind of character, or email not verified"},
		},
	}, h.humaCreateCharacter)

	huma.Register(api, huma.Operation{
		OperationID: "listGameCharacters",
		Method:      http.MethodGet,
		Path:        "/characters",
		Summary:     "List a game's characters",
		Description: "Lists the game's roster. Regular players do not see other players' " +
			"unapproved characters, and an anonymous game hides player identity from them.",
		Tags:     []string{"Characters"},
		Security: bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaGetGameCharacters)

	huma.Register(api, huma.Operation{
		OperationID: "getGameCharacterStats",
		Method:      http.MethodGet,
		Path:        "/characters/stats",
		Summary:     "Activity stats for every character in a game",
		Description: "Returns message counts for the whole roster in one response, keyed by " +
			"character ID. Private counts are omitted per character where the caller may not see them.",
		Tags:     []string{"Characters"},
		Security: bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaGetGameCharacterStats)

	huma.Register(api, huma.Operation{
		OperationID: "getGameCharacterData",
		Method:      http.MethodGet,
		Path:        "/characters/data",
		Summary:     "Sheet fields for every character in a game",
		Description: "Returns character sheet rows for the whole roster in one response, keyed by " +
			"character ID. Private fields are included only for the characters the caller may see " +
			"them for; everyone else's are reduced to their public fields.",
		Tags:     []string{"Characters"},
		Security: bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "The caller cannot view this game's content"},
		},
	}, h.humaGetGameCharacterData)

	huma.Register(api, huma.Operation{
		OperationID: "listControllableCharacters",
		Method:      http.MethodGet,
		Path:        "/characters/controllable",
		Summary:     "List characters the caller can control in a game",
		Description: "Returns the caller's own characters plus any NPCs assigned to them. " +
			"Filtered to active characters, so entries carry no is_active field.",
		Tags:     []string{"Characters"},
		Security: bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaGetUserControllableCharacters)

	huma.Register(api, huma.Operation{
		OperationID: "listInactiveCharacters",
		Method:      http.MethodGet,
		Path:        "/characters/inactive",
		Summary:     "List a game's inactive characters",
		Description: "Lists characters whose owner has gone inactive, with the ownership " +
			"history a reassignment decision needs. GM only.",
		Tags:     []string{"Characters"},
		Security: bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can view inactive characters"},
		},
	}, h.humaListInactiveCharacters)
}

// RegisterHumaCharacters registers the per-character operations.
//
// Paths are relative to the characters router's mount point (/api/v1/characters).
func RegisterHumaCharacters(api huma.API, h *Handler) {
	bearer := []map[string][]string{{"BearerAuth": {}}}

	// Registered before /{id} so the static segment is matched first; huma
	// sorts registrations onto chi, which would otherwise read "controllable"
	// as a character ID.
	huma.Register(api, huma.Operation{
		OperationID: "listControllableCharactersAcrossGames",
		Method:      http.MethodGet,
		Path:        "/controllable",
		Summary:     "List controllable characters across all the caller's games",
		Description: "Returns every character the caller can control in their in_progress games, " +
			"each carrying its game's context so a sheet can render with no game in scope.",
		Tags:     []string{"Characters"},
		Security: bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaGetUserControllableCharactersAcrossGames)

	huma.Register(api, huma.Operation{
		OperationID: "getCharacter",
		Method:      http.MethodGet,
		Path:        "/{id}",
		Summary:     "Get a character",
		Description: "Returns one character. In a running game another player's unapproved " +
			"character is reported as not found, and an anonymous game omits character_type.",
		Tags:     []string{"Characters"},
		Security: bearer,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated"},
			"404": {Description: "No such character, or character hidden from the caller (another player's unapproved character in a running game)"},
		},
	}, h.humaGetCharacter)

	huma.Register(api, huma.Operation{
		OperationID:   "deleteCharacter",
		Method:        http.MethodDelete,
		Path:          "/{id}",
		Summary:       "Delete a character",
		Description:   "Deletes a character that has never spoken or acted. GM only.",
		Tags:          []string{"Characters"},
		Security:      bearer,
		DefaultStatus: http.StatusNoContent,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"400": {Description: "Character has messages or action submissions"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can delete characters"},
			"404": {Description: "Character not found"},
		},
	}, h.humaDeleteCharacter)

	huma.Register(api, huma.Operation{
		OperationID: "approveCharacter",
		Method:      http.MethodPost,
		Path:        "/{id}/approve",
		Summary:     "Approve a character",
		Description: "Approves a pending character and notifies its owner. GM only. " +
			"There is no reject path: the database allows only pending and approved.",
		Tags:     []string{"Characters"},
		Security: bearer,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"400": {Description: "status was not \"approved\""},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can approve characters"},
			"404": {Description: "No such character"},
		},
	}, h.humaApproveCharacter)

	huma.Register(api, huma.Operation{
		OperationID: "assignNPC",
		Method:      http.MethodPost,
		Path:        "/{id}/assign",
		Summary:     "Assign an NPC to a user",
		Description: "Hands control of an NPC to an audience member, or back to the GM. GM only.",
		Tags:        []string{"Characters"},
		Security:    bearer,
		// The chi handler answered 204 with no body, which is preserved.
		DefaultStatus: http.StatusNoContent,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"400": {Description: "Target user is not an audience member"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can assign NPCs"},
			"404": {Description: "No such character"},
		},
	}, h.humaAssignNPC)

	huma.Register(api, huma.Operation{
		OperationID: "reassignCharacter",
		Method:      http.MethodPut,
		Path:        "/{id}/reassign",
		Summary:     "Reassign an inactive character",
		Description: "Gives an inactive character a new owner. GM only, and only while the " +
			"character is inactive.",
		Tags:     []string{"Characters"},
		Security: bearer,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can reassign characters"},
			"404": {Description: "Character not found"},
			"409": {Description: "Character is still active"},
		},
	}, h.humaReassignCharacter)

	huma.Register(api, huma.Operation{
		OperationID: "renameCharacter",
		Method:      http.MethodPut,
		Path:        "/{id}/rename",
		Summary:     "Rename a character",
		Description: "Renames a character. Allowed to the GM and to the character's owner. " +
			"Names are unique within a game.",
		Tags:     []string{"Characters"},
		Security: bearer,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"400": {Description: "Invalid body"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not allowed to rename this character"},
			"409": {Description: "A character with that name already exists in the game"},
		},
	}, h.humaRenameCharacter)

	huma.Register(api, huma.Operation{
		OperationID:   "setCharacterData",
		Method:        http.MethodPost,
		Path:          "/{id}/data",
		Summary:       "Set a character sheet field",
		Description:   "Writes one field of the character sheet. Stat modules (skills, inventory, numbers) are GM-only.",
		Tags:          []string{"Characters"},
		Security:      bearer,
		DefaultStatus: http.StatusNoContent,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"400": {Description: "Invalid body"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not allowed to edit this character, or this module"},
		},
	}, h.humaSetCharacterData)

	huma.Register(api, huma.Operation{
		OperationID: "getCharacterData",
		Method:      http.MethodGet,
		Path:        "/{id}/data",
		Summary:     "Get a character's sheet fields",
		Description: "Returns the character sheet. Private fields are included only for editors, " +
			"audience members, and every participant once the game is a public archive.",
		Tags:     []string{"Characters"},
		Security: bearer,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated"},
		},
	}, h.humaGetCharacterData)

	huma.Register(api, huma.Operation{
		OperationID: "getCharacterStats",
		Method:      http.MethodGet,
		Path:        "/{id}/stats",
		Summary:     "Activity stats for a character",
		Description: "Returns the character's message counts. The private count is omitted " +
			"unless the caller is the owner, a GM, an audience member, or the game is an archive.",
		Tags:     []string{"Characters"},
		Security: bearer,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated"},
			"404": {Description: "Character not found"},
			"500": {Description: "Game lookup failed"},
		},
	}, h.humaGetCharacterStats)
}
