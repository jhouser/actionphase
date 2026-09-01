package games

// Huma (type-first) implementation of the game API.
//
// Two registration functions, because the game routes divide by middleware
// rather than by mount: the public listing group runs jwtauth.Verifier only
// (auth is optional and merely enriches the result), while everything else is
// fully authenticated. See .claude/planning/huma-migration.md gotcha 19.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	"actionphase/pkg/humaconfig"
	"actionphase/pkg/observability"
)

// Helpers

// humaErr converts a core error response into the equivalent huma error,
// preserving the status and message the chi handlers produced.
func humaErr(errResp any) error {
	if resp, ok := errResp.(*core.ErrResponse); ok {
		// The app code travels in the errs slice, which is the only channel
		// huma.NewError offers; the shim unwraps it back onto the response.
		// Without this, core.ErrWithCode's "code" field is silently dropped.
		if resp.AppCode != 0 {
			return huma.NewError(resp.HTTPStatusCode, resp.ErrorText,
				&humaconfig.CodedError{Code: resp.AppCode, Msg: resp.ErrorText})
		}
		return huma.NewError(resp.HTTPStatusCode, resp.ErrorText)
	}
	return huma.Error500InternalServerError("unexpected error")
}

// logAndErr mirrors the chi renderError helper: 5xx logs at Error, 4xx at Warn,
// then the error is returned rather than rendered.
func (h *Handler) logAndErr(ctx context.Context, errResp any, msg string, args ...any) error {
	if resp, ok := errResp.(*core.ErrResponse); ok && resp.HTTPStatusCode >= 500 {
		h.App.ObsLogger.Error(ctx, msg, args...)
	} else {
		h.App.ObsLogger.Warn(ctx, msg, args...)
	}
	return humaErr(errResp)
}

// gameFromCtx reads the row GameMiddleware loaded. Present for every operation
// registered under /{gameID}; the guard turns a misconfigured router into a 500
// rather than a panic.
func gameFromCtx(ctx context.Context) (*models.Game, error) {
	game, ok := ctx.Value("game").(*models.Game)
	if !ok {
		return nil, huma.Error500InternalServerError("game context missing")
	}
	return game, nil
}

func gameIDFromCtx(ctx context.Context) (int32, error) {
	gameID, ok := ctx.Value("gameID").(int32)
	if !ok {
		return 0, huma.Error500InternalServerError("game context missing")
	}
	return gameID, nil
}

// isGMFromCtx reads the GM flag GameMiddleware computed, which already accounts
// for co-GMs and admin mode.
//
// Not interchangeable with `game.GmUserID == userID`: several endpoints
// deliberately test the primary GM directly and so exclude co-GMs and admin
// mode. Copy whichever the chi handler used rather than unifying them.
func isGMFromCtx(ctx context.Context) bool {
	isGM, _ := ctx.Value("is_gm").(bool)
	return isGM
}

func (h *Handler) requireAuth(ctx context.Context) (*core.AuthenticatedUser, error) {
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		return nil, h.logAndErr(ctx, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
	}
	return authUser, nil
}

// requireGMFlag is the is_gm check the loot-table, application and state
// endpoints share.
func (h *Handler) requireGMFlag(ctx context.Context, forbiddenMsg, logMsg string) error {
	if !isGMFromCtx(ctx) {
		return h.logAndErr(ctx, core.ErrForbidden(forbiddenMsg), logMsg)
	}
	return nil
}

// requireCanViewGame is the read gate the audience endpoints share. It admits
// participants and, for a completed game, any authenticated user.
func (h *Handler) requireCanViewGame(ctx context.Context, gameID int32, logMsg string) (*core.AuthenticatedUser, error) {
	authUser, err := h.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	canView, err := h.GameService.CanUserViewGame(ctx, gameID, authUser.ID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to check game view access",
			"error", err, "game_id", gameID, "user_id", authUser.ID)
	}
	if !canView {
		return nil, h.logAndErr(ctx, core.ErrForbidden("you do not have permission to view this game's content"), logMsg)
	}
	return authUser, nil
}

// requireLootTableInGame binds the {tableId} to the {gameID}.
//
// Not redundant with the GM check: permission is granted over the game in the
// URL, but the row acted on is named by the table id, so without this a GM could
// name a table in someone else's game and pass the permission check honestly.
func (h *Handler) requireLootTableInGame(ctx context.Context, tableID, gameID int32) error {
	ok, err := h.GameService.IsLootTableInGame(ctx, tableID, gameID)
	if err != nil {
		return h.logAndErr(ctx, core.ErrInternalError(err), "Failed to check loot table ownership",
			"error", err, "table_id", tableID, "game_id", gameID)
	}
	if !ok {
		return h.logAndErr(ctx, core.ErrForbidden("loot table does not belong to this game"), "Loot table access forbidden")
	}
	return nil
}

// gameResponseFrom builds the full game payload the create, read and update
// endpoints share. The chi handlers repeated this block five times; the shape
// was identical each time, so a single builder cannot drift.
//
// UpdateGameState deliberately does NOT use this — see humaUpdateGameState.
func gameResponseFrom(game *models.Game) *GameResponse {
	resp := &GameResponse{
		ID:                      game.ID,
		Title:                   game.Title,
		Description:             game.Description.String,
		GMUserID:                game.GmUserID,
		State:                   game.State.String,
		IsAnonymous:             game.IsAnonymous,
		AutoAcceptAudience:      game.AutoAcceptAudience,
		AllowGroupConversations: game.AllowGroupConversations,
		PortraitAvatars:         game.PortraitAvatars,
		CharacterSheet:          characterSheetResponse(game.CharacterSheet),
		CreatedAt:               game.CreatedAt.Time,
		UpdatedAt:               game.UpdatedAt.Time,
	}

	if game.Genre.Valid {
		resp.Genre = game.Genre.String
	}
	if game.StartDate.Valid {
		resp.StartDate = &game.StartDate.Time
	}
	if game.EndDate.Valid {
		resp.EndDate = &game.EndDate.Time
	}
	if game.RecruitmentDeadline.Valid {
		resp.RecruitmentDeadline = &game.RecruitmentDeadline.Time
	}
	if game.MaxPlayers.Valid {
		resp.MaxPlayers = game.MaxPlayers.Int32
	}
	if game.BannerUrl.Valid {
		resp.BannerURL = &game.BannerUrl.String
	}
	if game.CommunityID.Valid {
		v := game.CommunityID.Int32
		resp.CommunityID = &v
	}
	if game.CommonRoomOpenDay.Valid {
		v := game.CommonRoomOpenDay.Int16
		resp.CommonRoomOpenDay = &v
	}
	if game.CommonRoomOpenTime.Valid {
		s := formatPgtypeTime(game.CommonRoomOpenTime)
		resp.CommonRoomOpenTime = &s
	}
	if game.CommonRoomCloseDay.Valid {
		v := game.CommonRoomCloseDay.Int16
		resp.CommonRoomCloseDay = &v
	}
	if game.CommonRoomCloseTime.Valid {
		s := formatPgtypeTime(game.CommonRoomCloseTime)
		resp.CommonRoomCloseTime = &s
	}
	if game.ScheduleTimezone.Valid {
		resp.ScheduleTimezone = &game.ScheduleTimezone.String
	}
	return resp
}

// applicationResponseFrom shapes a stored application row for a response.
func applicationResponseFrom(app *models.GameApplication) *GameApplicationResponse {
	resp := &GameApplicationResponse{
		ID:        app.ID,
		GameID:    app.GameID,
		UserID:    app.UserID,
		Role:      app.Role,
		Status:    app.Status.String,
		AppliedAt: app.AppliedAt.Time,
	}
	if app.Message.Valid {
		resp.Message = app.Message.String
	}
	if app.ReviewedAt.Valid {
		t := app.ReviewedAt.Time
		resp.ReviewedAt = &t
	}
	if app.ReviewedByUserID.Valid {
		v := app.ReviewedByUserID.Int32
		resp.ReviewedByUserID = &v
	}
	return resp
}

// Input / output types

type gameScopedInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
}

type userScopedInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
	UserID int32 `path:"userId" doc:"Target user ID"`
}

type tableScopedInput struct {
	GameID  int32 `path:"gameID" doc:"Game ID"`
	TableID int32 `path:"tableId" doc:"Loot table ID"`
}

// Request bodies
//
// These carry the constraints core.ValidateStruct used to enforce through the
// `validate:` tags. Unlike some earlier packages, those tags really did run
// here (via Bind), so the limits below are a faithful copy rather than a
// tightening: title 3-255 characters, description at least 10.

type createGameBody struct {
	Title                   string              `json:"title" minLength:"3" maxLength:"255"`
	Description             string              `json:"description" minLength:"10"`
	Genre                   string              `json:"genre,omitempty" required:"false"`
	StartDate               *core.LocalDateTime `json:"start_date,omitempty" required:"false"`
	EndDate                 *core.LocalDateTime `json:"end_date,omitempty" required:"false"`
	RecruitmentDeadline     *core.LocalDateTime `json:"recruitment_deadline,omitempty" required:"false"`
	MaxPlayers              int32               `json:"max_players,omitempty" required:"false"`
	IsAnonymous             bool                `json:"is_anonymous,omitempty" required:"false"`
	AutoAcceptAudience      bool                `json:"auto_accept_audience,omitempty" required:"false"`
	AllowGroupConversations bool                `json:"allow_group_conversations,omitempty" required:"false"`
	PortraitAvatars         bool                `json:"portrait_avatars,omitempty" required:"false"`
	BannerURL               *string             `json:"banner_url,omitempty" required:"false"`
	CommonRoomOpenDay       *int16              `json:"common_room_open_day,omitempty" required:"false" minimum:"0" maximum:"6"`
	CommonRoomOpenTime      *string             `json:"common_room_open_time,omitempty" required:"false"`
	CommonRoomCloseDay      *int16              `json:"common_room_close_day,omitempty" required:"false" minimum:"0" maximum:"6"`
	CommonRoomCloseTime     *string             `json:"common_room_close_time,omitempty" required:"false"`
	ScheduleTimezone        *string             `json:"schedule_timezone,omitempty" required:"false"`
	// Required: every new game belongs to a community (req 5). minimum:"1"
	// rejects the zero value, which would otherwise pass as "present".
	CommunityID int32 `json:"community_id" required:"true" minimum:"1"`
	// Typed, not json.RawMessage.
	//
	// The chi version kept this raw so it could decode with
	// DisallowUnknownFields, because render.Bind's decoder would otherwise
	// silently drop a typo'd key. Huma needs no such workaround: it rejects
	// unknown properties on nested objects too, verified at both levels of this
	// structure. So the strictness survives and the schema now describes the
	// shape instead of saying "object".
	CharacterSheet *core.CharacterSheetConfig `json:"character_sheet,omitempty" required:"false"`
}

// Resolve carries over the two checks Bind ran after the struct tags: the
// character sheet's own validation (which normalizes whitespace-only labels to
// absent) and the all-or-nothing schedule rule.
func (b *createGameBody) Resolve(huma.Context) []error {
	if errs := humaconfig.TrimStrings(b); len(errs) > 0 {
		return errs
	}
	return resolveGameBody(&b.CharacterSheet,
		b.CommonRoomOpenDay, b.CommonRoomCloseDay,
		b.CommonRoomOpenTime, b.CommonRoomCloseTime, b.ScheduleTimezone)
}

type updateGameBody struct {
	Title               string     `json:"title" minLength:"3" maxLength:"255"`
	Description         string     `json:"description" minLength:"10"`
	Genre               string     `json:"genre,omitempty" required:"false"`
	StartDate           *time.Time `json:"start_date,omitempty" required:"false"`
	EndDate             *time.Time `json:"end_date,omitempty" required:"false"`
	RecruitmentDeadline *time.Time `json:"recruitment_deadline,omitempty" required:"false"`
	MaxPlayers          int32      `json:"max_players,omitempty" required:"false"`
	// A POINTER, unlike most of this body: absent means "leave the community
	// alone", not "clear it". Only honoured while the game is in setup.
	CommunityID             *int32                     `json:"community_id,omitempty" required:"false" minimum:"1"`
	IsPublic                bool                       `json:"is_public,omitempty" required:"false"`
	IsAnonymous             bool                       `json:"is_anonymous,omitempty" required:"false"`
	AutoAcceptAudience      bool                       `json:"auto_accept_audience,omitempty" required:"false"`
	AllowGroupConversations bool                       `json:"allow_group_conversations,omitempty" required:"false"`
	PortraitAvatars         bool                       `json:"portrait_avatars,omitempty" required:"false"`
	BannerURL               *string                    `json:"banner_url,omitempty" required:"false"`
	CommonRoomOpenDay       *int16                     `json:"common_room_open_day,omitempty" required:"false" minimum:"0" maximum:"6"`
	CommonRoomOpenTime      *string                    `json:"common_room_open_time,omitempty" required:"false"`
	CommonRoomCloseDay      *int16                     `json:"common_room_close_day,omitempty" required:"false" minimum:"0" maximum:"6"`
	CommonRoomCloseTime     *string                    `json:"common_room_close_time,omitempty" required:"false"`
	ScheduleTimezone        *string                    `json:"schedule_timezone,omitempty" required:"false"`
	CharacterSheet          *core.CharacterSheetConfig `json:"character_sheet,omitempty" required:"false"`

	// StartDate and friends are plain *time.Time here, not core.LocalDateTime,
	// because that is what the chi request struct used. Update therefore accepts
	// only RFC3339 where create also accepts datetime-local; the asymmetry is
	// inherited and preserved rather than quietly fixed.
}

func (b *updateGameBody) Resolve(huma.Context) []error {
	if errs := humaconfig.TrimStrings(b); len(errs) > 0 {
		return errs
	}
	return resolveGameBody(&b.CharacterSheet,
		b.CommonRoomOpenDay, b.CommonRoomCloseDay,
		b.CommonRoomOpenTime, b.CommonRoomCloseTime, b.ScheduleTimezone)
}

// resolveGameBody runs the two non-tag validations the create and update bodies
// share, normalizing the character sheet in place.
//
// The sheet is validated here rather than left to the service for the reason the
// chi Bind gave: a service-layer rejection renders as a 500 "unexpected error",
// so a GM typing an over-long tab label would be told the server broke.
func resolveGameBody(sheet **core.CharacterSheetConfig, openDay, closeDay *int16, openTime, closeTime, tz *string) []error {
	if *sheet != nil {
		validated, err := core.ValidateCharacterSheetConfig(**sheet)
		if err != nil {
			return []error{&huma.ErrorDetail{Message: err.Error(), Location: "body.character_sheet"}}
		}
		*sheet = &validated
	}

	if err := validateScheduleFields(openDay, closeDay, openTime, closeTime, tz); err != nil {
		return []error{&huma.ErrorDetail{Message: err.Error(), Location: "body"}}
	}
	return nil
}

// sheetConfigValue dereferences an optional sheet pointer for the service call,
// which takes a value.
func sheetConfigValue(sheet *core.CharacterSheetConfig) core.CharacterSheetConfig {
	if sheet == nil {
		return core.CharacterSheetConfig{}
	}
	return *sheet
}

type updateGameStateBody struct {
	State string `json:"state" minLength:"1" doc:"Target game state"`
}

type applyToGameBody struct {
	// enum rather than the hand-rolled ValidateGameRole check: same two values,
	// now enforced before the handler and visible in the schema.
	Role    string `json:"role" enum:"player,audience"`
	Message string `json:"message,omitempty" required:"false"`
}

type reviewApplicationBody struct {
	Action string `json:"action" enum:"approve,reject" doc:"Whether to approve or reject the application"`
}

type addParticipantBody struct {
	UserID int32  `json:"user_id" minimum:"1" doc:"User to add"`
	Role   string `json:"role" enum:"player,audience"`
}

type autoAcceptAudienceBody struct {
	AutoAcceptAudience bool `json:"auto_accept_audience"`
}

type lootTableItemBody struct {
	Name string `json:"name" minLength:"1"`
	Data string `json:"data" minLength:"1" doc:"GM-authored JSON blob describing the item"`
}

type updateLootTableBody struct {
	Name  string              `json:"name" minLength:"1"`
	Items []lootTableItemBody `json:"items,omitempty" required:"false"`
}

func (b *updateLootTableBody) Resolve(huma.Context) []error {
	if errs := humaconfig.TrimStrings(b); len(errs) > 0 {
		return errs
	}
	return validateLootItems(b.Items)
}

// updateLootContentsBody declares Name even though the endpoint ignores it.
//
// The chi handler decoded into a struct with only Items, so a client sending
// {name, items} -- which the loot editor does, reusing its table payload -- had
// the name silently discarded. Huma would 400 on it instead (gotcha 21), so the
// field is declared and left unread to keep that request working.
type updateLootContentsBody struct {
	Name  string              `json:"name,omitempty" required:"false" doc:"Ignored; use the table endpoint to rename"`
	Items []lootTableItemBody `json:"items,omitempty" required:"false"`
}

func (b *updateLootContentsBody) Resolve(huma.Context) []error {
	return validateLootItems(b.Items)
}

// validateLootItems enforces what validateLootTableItems did: names and data
// non-blank, and data parseable as JSON. The JSON check cannot be a struct tag,
// since the field is a string carrying a document.
func validateLootItems(items []lootTableItemBody) []error {
	var errs []error
	for i, item := range items {
		loc := fmt.Sprintf("body.items[%d]", i)
		if strings.TrimSpace(item.Name) == "" {
			errs = append(errs, &huma.ErrorDetail{
				Message:  fmt.Sprintf("loot table item %d: name is required", i+1),
				Location: loc + ".name",
			})
			continue
		}
		if strings.TrimSpace(item.Data) == "" {
			errs = append(errs, &huma.ErrorDetail{
				Message:  fmt.Sprintf("loot table item %d: data is required", i+1),
				Location: loc + ".data",
			})
			continue
		}
		if !json.Valid([]byte(item.Data)) {
			errs = append(errs, &huma.ErrorDetail{
				Message:  fmt.Sprintf("loot table item %d (%q): data must be valid JSON", i+1, item.Name),
				Location: loc + ".data",
			})
		}
	}
	return errs
}

// Output types

type gameOutput struct {
	Body *GameResponse
}

type gameWithDetailsOutput struct {
	Body *GameWithDetailsResponse
}

type gameListingOutput struct {
	Body *GameListingResponse
}

// recruitingGamesOutput keeps the untyped map shape the chi handler encoded,
// and with it the nil-slice behaviour: an empty result serializes as null, not
// [] (gotcha 12). Both are preserved rather than corrected, since either change
// is frontend-visible.
type recruitingGamesOutput struct {
	Body []map[string]any
}

// participantsOutput is likewise a nil-able slice of maps: null when empty.
type participantsOutput struct {
	Body []map[string]any
}

// applicationsOutput is built with make(...,0), so an empty list is [].
// The difference from participantsOutput is inherited from the chi handlers.
type applicationsOutput struct {
	Body []map[string]any
}

type applicationOutput struct {
	Body *GameApplicationResponse
}

// myApplicationOutput's body is `any` so the no-application case can send a
// bare `null` with 200, which is what the chi handler did via render.JSON(nil).
// A client distinguishes "never applied" from an error by that null.
type myApplicationOutput struct {
	Body any
}

type participantOutput struct {
	Body *models.GameParticipant
}

type audienceMembersOutput struct {
	Body *ListAudienceMembersResponse
}

type audienceNPCsOutput struct {
	Body struct {
		NPCs any `json:"npcs"`
	}
}

type privateConversationsOutput struct {
	Body struct {
		Conversations []PrivateConversationResponse `json:"conversations"`
		Total         int64                         `json:"total"`
	}
}

type conversationMessagesOutput struct {
	Body struct {
		Messages []AudienceMessageResponse `json:"messages"`
	}
}

type conversationParticipantsOutput struct {
	Body struct {
		Participants []core.ConversationParticipantCharacter `json:"participants"`
	}
}

type actionSubmissionsOutput struct {
	Body struct {
		ActionSubmissions []ActionSubmissionResponse `json:"action_submissions"`
		Total             int64                      `json:"total"`
	}
}

type gameLogsOutput struct {
	Body []map[string]any
}

type gameStatsOutput struct {
	Body any
}

type lootTablesOutput struct {
	Body []map[string]any
}

type lootTableOutput struct {
	Body *models.GameLootTable
}

type lootContentsOutput struct {
	Body []map[string]any
}

type lootContentOutput struct {
	Body *models.GameLootTableContent
}

type bannerOutput struct {
	Body struct {
		BannerURL string `json:"banner_url"`
	}
}

type messageOutput struct {
	Body struct {
		Message string `json:"message"`
	}
}

// noContentOutput is empty, so huma infers 204 — which is what these endpoints
// already sent.
type noContentOutput struct{}

// emptyOKOutput answers 200 with a zero-length body.
//
// Three loot-table endpoints end without writing a status or a body, so chi
// sends a bare 200. An empty huma output would infer 204 (gotcha 22), which is
// a different status than has ever shipped, so Status is set explicitly.
// Verified against the running server before converting.
type emptyOKOutput struct {
	Status int
}

// Game CRUD

type createGameInput struct {
	Body *createGameBody
}

func (h *Handler) humaCreateGame(ctx context.Context, in *createGameInput) (*gameOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_create_game")()

	// Email verification, which the chi route enforced with
	// RequireEmailVerificationMiddleware. Huma handlers take a context, so the
	// rule runs here through its context-based twin (gotcha 15).
	if errResp := core.RequireVerifiedEmailCtx(ctx, h.App.Pool); errResp != nil {
		return nil, humaErr(errResp)
	}

	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		return nil, h.logAndErr(ctx, errResp, "Failed to authenticate user from JWT")
	}
	h.App.ObsLogger.Info(ctx, "Authenticated user for game creation", "user_id", userID)

	game, err := h.GameService.CreateGame(ctx, core.CreateGameRequest{
		Title:                   in.Body.Title,
		Description:             in.Body.Description,
		GMUserID:                userID,
		Genre:                   in.Body.Genre,
		StartDate:               in.Body.StartDate.ToTimePtr(),
		EndDate:                 in.Body.EndDate.ToTimePtr(),
		RecruitmentDeadline:     in.Body.RecruitmentDeadline.ToTimePtr(),
		MaxPlayers:              in.Body.MaxPlayers,
		IsPublic:                true, // All games are now public
		IsAnonymous:             in.Body.IsAnonymous,
		AutoAcceptAudience:      in.Body.AutoAcceptAudience,
		AllowGroupConversations: in.Body.AllowGroupConversations,
		PortraitAvatars:         in.Body.PortraitAvatars,
		BannerURL:               in.Body.BannerURL,
		CommonRoomOpenDay:       in.Body.CommonRoomOpenDay,
		CommonRoomOpenTime:      in.Body.CommonRoomOpenTime,
		CommonRoomCloseDay:      in.Body.CommonRoomCloseDay,
		CommonRoomCloseTime:     in.Body.CommonRoomCloseTime,
		ScheduleTimezone:        in.Body.ScheduleTimezone,
		CommunityID:             in.Body.CommunityID,
		CharacterSheet:          sheetConfigValue(in.Body.CharacterSheet),
	})
	if err != nil {
		h.App.Observability.OTELMetrics.RecordGameCreateError(ctx)

		// A bad community is the caller's mistake, not a server fault. Without
		// these branches an unknown or inactive community renders as a 500 and
		// the GM is told nothing actionable.
		switch {
		case errors.Is(err, core.ErrCommunityNotFound):
			return nil, huma.Error404NotFound("that community does not exist")
		case errors.Is(err, core.ErrCommunityInactive):
			return nil, huma.Error400BadRequest("that community is no longer accepting new games")
		}

		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to create game",
			"error", err, "title", in.Body.Title, "user_id", userID)
	}

	h.App.ObsLogger.Info(ctx, "Game created successfully",
		"game_id", game.ID, "title", game.Title, "gm_user_id", game.GmUserID)
	h.App.Observability.OTELMetrics.RecordGameCreated(ctx)

	return &gameOutput{Body: gameResponseFrom(game)}, nil
}

func (h *Handler) humaGetGame(ctx context.Context, in *gameScopedInput) (*gameOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_game")()

	game, err := gameFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	return &gameOutput{Body: gameResponseFrom(game)}, nil
}

func (h *Handler) humaGetGameWithDetails(ctx context.Context, in *gameScopedInput) (*gameWithDetailsOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_game_with_details")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	game, err := h.GameService.GetGameWithDetails(ctx, gameID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get game with details", "error", err, "game_id", gameID)
	}

	resp := &GameWithDetailsResponse{
		ID:                      game.ID,
		Title:                   game.Title,
		Description:             game.Description.String,
		GMUserID:                game.GmUserID,
		State:                   game.State.String,
		IsAnonymous:             game.IsAnonymous,
		AutoAcceptAudience:      game.AutoAcceptAudience,
		AllowGroupConversations: game.AllowGroupConversations,
		PortraitAvatars:         game.PortraitAvatars,
		CharacterSheet:          characterSheetResponse(game.CharacterSheet),
		CurrentPlayers:          game.CurrentPlayers,
		CreatedAt:               game.CreatedAt.Time,
		UpdatedAt:               game.UpdatedAt.Time,
	}

	if game.GmUsername.Valid {
		resp.GMUsername = game.GmUsername.String
	}
	if game.Genre.Valid {
		resp.Genre = game.Genre.String
	}
	if game.StartDate.Valid {
		resp.StartDate = &game.StartDate.Time
	}
	if game.EndDate.Valid {
		resp.EndDate = &game.EndDate.Time
	}
	if game.RecruitmentDeadline.Valid {
		resp.RecruitmentDeadline = &game.RecruitmentDeadline.Time
	}
	if game.MaxPlayers.Valid {
		resp.MaxPlayers = game.MaxPlayers.Int32
	}
	if game.BannerUrl.Valid {
		resp.BannerURL = &game.BannerUrl.String
	}
	if game.CommunityID.Valid {
		v := game.CommunityID.Int32
		resp.CommunityID = &v
	}
	if game.CommonRoomOpenDay.Valid {
		v := game.CommonRoomOpenDay.Int16
		resp.CommonRoomOpenDay = &v
	}
	if game.CommonRoomOpenTime.Valid {
		s := formatPgtypeTime(game.CommonRoomOpenTime)
		resp.CommonRoomOpenTime = &s
	}
	if game.CommonRoomCloseDay.Valid {
		v := game.CommonRoomCloseDay.Int16
		resp.CommonRoomCloseDay = &v
	}
	if game.CommonRoomCloseTime.Valid {
		s := formatPgtypeTime(game.CommonRoomCloseTime)
		resp.CommonRoomCloseTime = &s
	}
	if game.ScheduleTimezone.Valid {
		resp.ScheduleTimezone = &game.ScheduleTimezone.String
	}

	return &gameWithDetailsOutput{Body: resp}, nil
}

type updateGameInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
	Body   *updateGameBody
}

func (h *Handler) humaUpdateGame(ctx context.Context, in *updateGameInput) (*gameOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_game")()

	if err := h.requireGMFlag(ctx, "only the GM can update this game", "Update game forbidden"); err != nil {
		return nil, err
	}

	game, err := gameFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	updatedGame, err := h.GameService.UpdateGame(ctx, core.UpdateGameRequest{
		ID:                      game.ID,
		Title:                   in.Body.Title,
		Description:             in.Body.Description,
		Genre:                   in.Body.Genre,
		StartDate:               in.Body.StartDate,
		EndDate:                 in.Body.EndDate,
		RecruitmentDeadline:     in.Body.RecruitmentDeadline,
		MaxPlayers:              in.Body.MaxPlayers,
		CommunityID:             in.Body.CommunityID,
		IsPublic:                in.Body.IsPublic,
		IsAnonymous:             in.Body.IsAnonymous,
		AutoAcceptAudience:      in.Body.AutoAcceptAudience,
		AllowGroupConversations: in.Body.AllowGroupConversations,
		PortraitAvatars:         in.Body.PortraitAvatars,
		BannerURL:               in.Body.BannerURL,
		CommonRoomOpenDay:       in.Body.CommonRoomOpenDay,
		CommonRoomOpenTime:      in.Body.CommonRoomOpenTime,
		CommonRoomCloseDay:      in.Body.CommonRoomCloseDay,
		CommonRoomCloseTime:     in.Body.CommonRoomCloseTime,
		ScheduleTimezone:        in.Body.ScheduleTimezone,
		CharacterSheet:          sheetConfigValue(in.Body.CharacterSheet),
	})
	if err != nil {
		// Community problems are the caller's mistake, not a server fault.
		switch {
		case errors.Is(err, core.ErrGameCommunityLocked):
			return nil, huma.Error409Conflict(
				"a game's community can only be changed while it is still in setup")
		case errors.Is(err, core.ErrCommunityNotFound):
			return nil, huma.Error404NotFound("that community does not exist")
		case errors.Is(err, core.ErrCommunityInactive):
			return nil, huma.Error400BadRequest("that community is no longer accepting games")
		}
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to update game", "error", err, "game_id", game.ID)
	}

	return &gameOutput{Body: gameResponseFrom(updatedGame)}, nil
}

func (h *Handler) humaDeleteGame(ctx context.Context, in *gameScopedInput) (*noContentOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_game")()

	if err := h.requireGMFlag(ctx, "only the GM can delete this game", "Delete game forbidden"); err != nil {
		return nil, err
	}

	game, err := gameFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	user, err := h.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.GameService.DeleteGame(ctx, game.ID, user.ID); err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to delete game", "error", err, "game_id", game.ID)
		// The service reports these as plain strings, so they are matched by
		// text. strings.HasPrefix replaces the chi version's manual slice, which
		// would panic on any message shorter than the prefix.
		errMsg := err.Error()
		switch {
		case errMsg == "game not found":
			return nil, h.logAndErr(ctx, core.ErrNotFound(errMsg), "Delete game not found")
		case errMsg == "only the game master can delete this game":
			return nil, h.logAndErr(ctx, core.ErrForbidden(errMsg), "Delete game forbidden")
		case strings.HasPrefix(errMsg, "only cancelled games can be deleted"):
			return nil, h.logAndErr(ctx, core.ErrBadRequest(err), "Bad delete game request", "error", err)
		default:
			return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to delete game", "error", err)
		}
	}

	return &noContentOutput{}, nil
}

type updateGameStateInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
	Body   *updateGameStateBody
}

// humaUpdateGameState answers with a deliberately reduced GameResponse.
//
// It carries id, title, description, gm_user_id, state and the timestamps —
// but not genre, dates, max_players, banner, schedule or character_sheet, all
// of which the other game endpoints return. The omitted booleans therefore
// serialize as false regardless of their stored values.
//
// That is what the chi handler sent, and the frontend refetches the game after
// a state change rather than merging this response, so the shape is preserved
// as-is rather than quietly widened to gameResponseFrom.
func (h *Handler) humaUpdateGameState(ctx context.Context, in *updateGameStateInput) (*gameOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_game_state")()

	if err := h.requireGMFlag(ctx, "only the GM can update this game state", "Update game state forbidden"); err != nil {
		return nil, err
	}

	game, err := gameFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	user, err := h.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	updatedGame, err := h.GameService.UpdateGameState(ctx, game.ID, in.Body.State)
	if err != nil {
		// A rejected transition is a client-side precondition failure, not a
		// bug: the request was well formed and authorized, but the move is not
		// legal from the game's current state (e.g. epilogue -> in_progress,
		// a deliberate one-way door). Retrying verbatim will never succeed, so
		// answer 409 with the states involved rather than a bare 500.
		if errors.Is(err, core.ErrInvalidStateTransition) {
			return nil, h.logAndErr(ctx,
				core.ErrWithCode(http.StatusConflict, core.ErrCodeInvalidGameState,
					fmt.Sprintf("cannot change game state from %s to %s", game.State.String, in.Body.State)),
				"Invalid game state transition requested",
				"game_id", game.ID, "from_state", game.State.String, "to_state", in.Body.State)
		}
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to update game state", "error", err, "game_id", game.ID)
	}

	h.settleRecruitment(ctx, game, updatedGame, in.Body.State, user.ID)
	h.notifyStateChange(ctx, game, updatedGame, in.Body.State, user.ID)

	return &gameOutput{Body: &GameResponse{
		ID:          updatedGame.ID,
		Title:       updatedGame.Title,
		Description: updatedGame.Description.String,
		GMUserID:    updatedGame.GmUserID,
		State:       updatedGame.State.String,
		CreatedAt:   updatedGame.CreatedAt.Time,
		UpdatedAt:   updatedGame.UpdatedAt.Time,
	}}, nil
}

// settleRecruitment converts approved applications into participants when a
// game leaves recruitment, and clears the rest.
//
// Every step is best-effort: the state transition has already committed, so a
// failure here is logged but must not fail the request — otherwise the caller
// would see an error for a change that did happen.
func (h *Handler) settleRecruitment(ctx context.Context, game, updatedGame *models.Game, newState string, actorID int32) {
	if game.State.String != core.GameStateRecruitment || newState == core.GameStateRecruitment {
		return
	}

	h.App.ObsLogger.Info(ctx, "Transitioning out of recruitment, converting approved applications", "game_id", game.ID)

	applicationService := h.GameApplicationService

	// Read the approved list first: the conversion below deletes rows, so
	// afterwards there would be nobody left to notify.
	approvedApps, err := applicationService.GetApprovedApplicationsForGame(ctx, game.ID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get approved applications", "error", err, "game_id", game.ID)
	}

	if err := applicationService.BulkRejectApplications(ctx, game.ID, actorID); err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to bulk reject pending applications", "error", err, "game_id", game.ID)
	}

	if err := applicationService.PublishApplicationStatuses(ctx, game.ID); err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to publish application statuses", "error", err, "game_id", game.ID)
	} else {
		h.App.ObsLogger.Info(ctx, "Successfully published application statuses", "game_id", game.ID)
	}

	if err := applicationService.ConvertApprovedApplicationsToParticipants(ctx, game.ID); err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to convert approved applications to participants", "error", err, "game_id", game.ID)
	} else {
		h.App.ObsLogger.Info(ctx, "Successfully converted approved applications to participants", "game_id", game.ID)
	}

	// Rejected applicants cannot join, so the records serve no further purpose.
	if err := applicationService.DeleteRejectedApplications(ctx, game.ID); err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to delete rejected applications", "error", err, "game_id", game.ID)
	} else {
		h.App.ObsLogger.Info(ctx, "Successfully deleted rejected applications", "game_id", game.ID)
	}

	// Sent regardless of whether the conversion above succeeded: the GM's
	// decision was to accept these people, and they should hear about it.
	for _, app := range approvedApps {
		if err := h.NotificationService.NotifyApplicationApproved(ctx, app.UserID, game.ID, updatedGame.Title); err != nil {
			h.App.ObsLogger.Warn(ctx, "Failed to create acceptance notification",
				"error", err, "game_id", game.ID, "user_id", app.UserID)
		} else {
			h.App.ObsLogger.Info(ctx, "Sent acceptance notification", "game_id", game.ID, "user_id", app.UserID)
		}
	}
}

// notifyStateChange tells participants about pause/resume and endgame moves.
//
// "Endgame" rather than "terminal": epilogue is a genuine endgame transition
// participants need to hear about (the whole archive just opened to them), but
// it is not terminal — the game is still writable.
func (h *Handler) notifyStateChange(ctx context.Context, game, updatedGame *models.Game, newState string, actorID int32) {
	isPauseResume := newState == core.GameStatePaused ||
		(newState == core.GameStateInProgress && game.State.String == core.GameStatePaused)
	isEndgame := newState == core.GameStateEpilogue ||
		newState == core.GameStateCompleted ||
		newState == core.GameStateCancelled
	if !isPauseResume && !isEndgame {
		return
	}

	notifSvc := h.NotificationService
	gameID, title := game.ID, updatedGame.Title
	observability.SafeGo(context.Background(), h.App.ObsLogger, "notify-game-state-changed", func() {
		notifCtx := context.Background()
		if err := notifSvc.NotifyGameStateChanged(notifCtx, gameID, newState, title, actorID); err != nil {
			h.App.ObsLogger.Warn(notifCtx, "Failed to send game state changed notifications", "error", err, "game_id", gameID)
		}
	})
}

// Game listing

// filteredGamesInput takes its pagination as strings on purpose.
//
// The chi handler ignored anything it could not parse and fell back to the
// default: ?page=abc returned page 1, ?page_size=99999 returned 20. Declaring
// these as int32 would make huma 400 on the same input (gotcha 18). This is the
// public browse endpoint, reachable from hand-written and shared URLs, so the
// tolerance is preserved rather than narrowed — the parsing below is the chi
// logic verbatim.
type filteredGamesInput struct {
	Search        string `query:"search" required:"false"`
	States        string `query:"states" required:"false" doc:"Comma-separated game states"`
	Participation string `query:"participation" required:"false"`
	HasOpenSpots  string `query:"has_open_spots" required:"false" doc:"\"true\" or \"false\"; anything else is ignored"`
	CommunityID   string `query:"community_id" required:"false" doc:"Only games in this community; omit for all"`
	SortBy        string `query:"sort_by" required:"false"`
	AdminMode     string `query:"admin_mode" required:"false" doc:"\"true\" enables admin mode for an authenticated admin"`
	Page          string `query:"page" required:"false" doc:"1-based page number; defaults to 1"`
	PageSize      string `query:"page_size" required:"false" doc:"1-100; defaults to 20"`
}

func (h *Handler) humaGetFilteredGames(ctx context.Context, in *filteredGamesInput) (*gameListingOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_filtered_games")()

	page := 1
	pageSize := 20
	if in.Page != "" {
		if p, err := strconv.Atoi(in.Page); err == nil && p > 0 {
			page = p
		}
	}
	if in.PageSize != "" {
		if ps, err := strconv.Atoi(in.PageSize); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	filters := core.GameListingFilters{
		Search:   in.Search,
		SortBy:   in.SortBy,
		Page:     page,
		PageSize: pageSize,
	}

	if in.States != "" {
		filters.States = splitCommaSeparated(in.States)
	}
	if in.Participation != "" {
		participation := in.Participation
		filters.ParticipationFilter = &participation
	}
	if in.HasOpenSpots == "true" || in.HasOpenSpots == "false" {
		hasOpenSpots := in.HasOpenSpots == "true"
		filters.HasOpenSpots = &hasOpenSpots
	}
	// Ignored unless it parses to a positive id, matching how page and
	// page_size treat junk here: a filter nobody can express is better than a
	// 400 on a stray query string.
	if in.CommunityID != "" {
		if c, err := strconv.Atoi(in.CommunityID); err == nil && c > 0 {
			communityID := int32(c)
			filters.CommunityID = &communityID
		}
	}

	// Optional: this route runs jwtauth.Verifier without Authenticator, so an
	// anonymous browser reaches here with no token and simply gets an
	// unenriched listing.
	userID, _ := core.GetUserIDFromJWT(ctx, h.UserService)
	if userID != 0 {
		filters.UserID = &userID
	}

	if in.AdminMode == "true" && userID != 0 {
		filters.AdminMode = true
		filters.AdminUserID = &userID
	}

	result, err := h.GameService.GetFilteredGames(ctx, filters)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get filtered games", "error", err)
	}

	response := &GameListingResponse{
		Games: make([]*EnrichedGameListItemResponse, len(result.Games)),
		Metadata: GameListingMetadataResponse{
			TotalCount:      result.Metadata.TotalCount,
			FilteredCount:   result.Metadata.FilteredCount,
			AvailableStates: result.Metadata.AvailableStates,
			Page:            result.Metadata.Page,
			PageSize:        result.Metadata.PageSize,
			TotalPages:      result.Metadata.TotalPages,
			HasNextPage:     result.Metadata.HasNextPage,
			HasPreviousPage: result.Metadata.HasPreviousPage,
		},
	}

	for i, game := range result.Games {
		response.Games[i] = &EnrichedGameListItemResponse{
			ID:                      game.ID,
			Title:                   game.Title,
			Description:             game.Description,
			GMUserID:                game.GMUserID,
			GMUsername:              game.GMUsername,
			State:                   game.State,
			Genre:                   game.Genre,
			StartDate:               game.StartDate,
			EndDate:                 game.EndDate,
			RecruitmentDeadline:     game.RecruitmentDeadline,
			MaxPlayers:              game.MaxPlayers,
			IsPublic:                game.IsPublic,
			IsAnonymous:             game.IsAnonymous,
			AutoAcceptAudience:      game.AutoAcceptAudience,
			AllowGroupConversations: game.AllowGroupConversations,
			PortraitAvatars:         game.PortraitAvatars,
			BannerURL:               game.BannerURL,
			CreatedAt:               game.CreatedAt,
			UpdatedAt:               game.UpdatedAt,
			CurrentPlayers:          game.CurrentPlayers,
			UserRelationship:        game.UserRelationship,
			CurrentPhaseType:        game.CurrentPhaseType,
			CurrentPhaseDeadline:    game.CurrentPhaseDeadline,
			DeadlineUrgency:         game.DeadlineUrgency,
			HasRecentActivity:       game.HasRecentActivity,
		}
	}

	return &gameListingOutput{Body: response}, nil
}

type emptyInput struct{}

func (h *Handler) humaGetRecruitingGames(ctx context.Context, in *emptyInput) (*recruitingGamesOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_recruiting_games")()

	games, err := h.GameService.GetRecruitingGames(ctx)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get recruiting games", "error", err)
	}

	// Nil slice, not make(...): an empty list serializes as null here, matching
	// the chi handler.
	var response []map[string]any
	for _, game := range games {
		gameData := map[string]any{
			"id":              game.ID,
			"title":           game.Title,
			"description":     game.Description,
			"gm_user_id":      game.GmUserID,
			"gm_username":     game.GmUsername,
			"state":           game.State,
			"current_players": game.CurrentPlayers,
			"created_at":      game.CreatedAt.Time,
			"updated_at":      game.UpdatedAt.Time,
		}

		if game.Genre.Valid {
			gameData["genre"] = game.Genre.String
		}
		if game.StartDate.Valid {
			gameData["start_date"] = game.StartDate.Time
		}
		if game.EndDate.Valid {
			gameData["end_date"] = game.EndDate.Time
		}
		if game.RecruitmentDeadline.Valid {
			gameData["recruitment_deadline"] = game.RecruitmentDeadline.Time
		}
		if game.MaxPlayers.Valid {
			gameData["max_players"] = game.MaxPlayers.Int32
		}
		if game.CommonRoomOpenDay.Valid {
			gameData["common_room_open_day"] = game.CommonRoomOpenDay.Int16
		}
		if game.CommonRoomOpenTime.Valid {
			gameData["common_room_open_time"] = formatPgtypeTime(game.CommonRoomOpenTime)
		}
		if game.CommonRoomCloseDay.Valid {
			gameData["common_room_close_day"] = game.CommonRoomCloseDay.Int16
		}
		if game.CommonRoomCloseTime.Valid {
			gameData["common_room_close_time"] = formatPgtypeTime(game.CommonRoomCloseTime)
		}
		if game.ScheduleTimezone.Valid {
			gameData["schedule_timezone"] = game.ScheduleTimezone.String
		}

		response = append(response, gameData)
	}

	return &recruitingGamesOutput{Body: response}, nil
}

// Participants

func (h *Handler) humaLeaveGame(ctx context.Context, in *gameScopedInput) (*noContentOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_leave_game")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		return nil, h.logAndErr(ctx, errResp, "Failed to authenticate user from JWT")
	}

	// A caller may be a participant, an applicant, or both. Try the participant
	// removal first and remember whether it worked, so the application branch
	// below knows whether "no application" is a real failure.
	participantRemoved := false
	if err := h.GameService.RemovePlayer(ctx, gameID, userID, userID); err != nil {
		h.App.ObsLogger.Debug(ctx, "User not found in participants (might have application instead)", "game_id", gameID, "user_id", userID)
	} else {
		participantRemoved = true
		h.App.ObsLogger.Info(ctx, "User left game (participant removed, characters deactivated)", "game_id", gameID, "user_id", userID)
	}

	application, err := h.GameApplicationService.GetGameApplicationByUserAndGame(ctx, gameID, userID)
	if err != nil {
		if !participantRemoved {
			return nil, h.logAndErr(ctx, core.ErrNotFound("you are not associated with this game"),
				"User is neither participant nor applicant", "error", err, "game_id", gameID, "user_id", userID)
		}
	} else if application.Status.String == core.ApplicationStatusPending {
		// Deleted rather than marked withdrawn, so the user can reapply.
		//
		// Approved audience applications no longer reach here:
		// ApproveGameApplication deletes the audience application when it creates
		// the participant, so a member who leaves has only a participant row.
		if err := h.GameApplicationService.DeleteGameApplication(ctx, application.ID, userID); err != nil {
			return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to delete application", "error", err, "application_id", application.ID)
		}
		h.App.ObsLogger.Info(ctx, "Deleted pending application", "application_id", application.ID, "game_id", gameID, "user_id", userID)
	}

	return &noContentOutput{}, nil
}

func (h *Handler) humaGetGameParticipants(ctx context.Context, in *gameScopedInput) (*participantsOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_game_participants")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	participants, err := h.GameService.GetGameParticipants(ctx, gameID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get game participants", "error", err, "game_id", gameID)
	}

	// In anonymous games only GMs, co-GMs and audience members may know which
	// participants are former players; regular players and non-participants
	// cannot.
	redactFormerPlayer := false
	if game, err := h.GameService.GetGame(ctx, gameID); err == nil && game.IsAnonymous {
		viewerID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
		if errResp != nil || !core.CanSeeUsernamesInAnonymousGame(ctx, h.App.Pool, *game, viewerID) {
			redactFormerPlayer = true
		}
	}

	var response []map[string]any
	for _, participant := range participants {
		role := participant.Role
		isFormerPlayer := participant.IsFormerPlayer
		// Viewers who can't see former-player status see them as regular
		// players instead — role spoofed to "player", flag cleared.
		if redactFormerPlayer && participant.IsFormerPlayer {
			role = "player"
			isFormerPlayer = false
		}

		participantData := map[string]any{
			"id":       participant.ID,
			"game_id":  participant.GameID,
			"user_id":  participant.UserID,
			"username": participant.Username,
			// Email is intentionally omitted for privacy.
			"role":             role,
			"status":           participant.Status,
			"joined_at":        participant.JoinedAt.Time,
			"is_former_player": isFormerPlayer,
		}

		// Explicit null rather than an absent key: the client reads this
		// directly to decide whether to render an avatar.
		if participant.AvatarUrl.Valid {
			participantData["avatar_url"] = participant.AvatarUrl.String
		} else {
			participantData["avatar_url"] = nil
		}

		response = append(response, participantData)
	}

	return &participantsOutput{Body: response}, nil
}

func (h *Handler) humaRemovePlayer(ctx context.Context, in *userScopedInput) (*noContentOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_remove_player")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	game, err := gameFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	requestingUserID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		return nil, h.logAndErr(ctx, errResp, "Failed to authenticate user from JWT")
	}

	// The primary GM only, deliberately: not is_gm, so a co-GM cannot remove
	// players and admin mode does not grant it either.
	if game.GmUserID != requestingUserID {
		return nil, h.logAndErr(ctx, core.ErrForbidden("only the GM can remove players"),
			"Non-GM attempted to remove player", "requesting_user_id", requestingUserID, "game_id", gameID)
	}

	if in.UserID == game.GmUserID {
		return nil, h.logAndErr(ctx, core.ErrConflict("GM cannot remove themselves from the game"),
			"GM attempted to remove themselves", "game_id", gameID, "gm_user_id", game.GmUserID)
	}

	if err := h.GameService.RemovePlayer(ctx, gameID, in.UserID, requestingUserID); err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to remove player",
			"error", err, "game_id", gameID, "user_id", in.UserID)
	}

	h.App.ObsLogger.Info(ctx, "Player removed from game", "game_id", gameID, "removed_user_id", in.UserID, "removed_by", requestingUserID)
	return &noContentOutput{}, nil
}

type addParticipantInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
	Body   *addParticipantBody
}

func (h *Handler) humaAddParticipantDirectly(ctx context.Context, in *addParticipantInput) (*participantOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_add_participant_directly")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	game, err := gameFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	requestingUserID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		return nil, h.logAndErr(ctx, errResp, "Failed to authenticate user from JWT")
	}

	if game.GmUserID != requestingUserID {
		return nil, h.logAndErr(ctx, core.ErrForbidden("only the GM can add participants directly"),
			"Non-GM attempted to add participant directly", "requesting_user_id", requestingUserID, "game_id", gameID)
	}

	if _, err := h.UserService.GetUserByID(int(in.Body.UserID)); err != nil {
		return nil, h.logAndErr(ctx, core.ErrNotFound("user not found"), "Target user not found", "error", err, "user_id", in.Body.UserID)
	}

	participant, err := h.GameService.AddParticipantWithRole(ctx, gameID, in.Body.UserID, in.Body.Role)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to add participant directly",
			"error", err, "game_id", gameID, "user_id", in.Body.UserID, "role", in.Body.Role)
	}

	h.App.ObsLogger.Info(ctx, "Participant added directly to game",
		"game_id", gameID, "added_user_id", in.Body.UserID, "role", in.Body.Role, "added_by", requestingUserID)

	return &participantOutput{Body: participant}, nil
}

// humaPromoteToCoGM, humaDemoteFromCoGM and humaTransitionPlayerToAudience all
// delegate their permission check to the service, which is the only place that
// knows the primary GM is required. Each maps that one refusal to 403 and every
// other service error to 400.
func (h *Handler) humaPromoteToCoGM(ctx context.Context, in *userScopedInput) (*noContentOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_promote_to_cogm")()
	return h.coGMTransition(ctx, in, "promote",
		"only the primary GM can promote users to co-GM",
		"User promoted to co-GM", "promoted_user_id", "promoted_by")
}

func (h *Handler) humaDemoteFromCoGM(ctx context.Context, in *userScopedInput) (*noContentOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_demote_from_cogm")()
	return h.coGMTransition(ctx, in, "demote",
		"only the primary GM can demote co-GMs",
		"Co-GM demoted to audience", "demoted_user_id", "demoted_by")
}

func (h *Handler) humaTransitionPlayerToAudience(ctx context.Context, in *userScopedInput) (*noContentOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_transition_player_to_audience")()
	return h.coGMTransition(ctx, in, "to-audience",
		"only the primary GM can transition players to audience",
		"Player transitioned to audience", "user_id", "transitioned_by")
}

// coGMTransition is the shared body of the three role-change endpoints, which
// differ only in which service method they call and how they log.
func (h *Handler) coGMTransition(ctx context.Context, in *userScopedInput, kind, forbiddenMsg, successMsg, targetKey, actorKey string) (*noContentOutput, error) {
	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	requestingUserID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		return nil, h.logAndErr(ctx, errResp, "Failed to authenticate user from JWT")
	}

	switch kind {
	case "promote":
		err = h.GameService.PromoteToCoGM(ctx, gameID, in.UserID, requestingUserID)
	case "demote":
		err = h.GameService.DemoteFromCoGM(ctx, gameID, in.UserID, requestingUserID)
	default:
		err = h.GameService.TransitionPlayerToAudience(ctx, gameID, in.UserID, requestingUserID)
	}

	if err != nil {
		h.App.ObsLogger.Error(ctx, "Role transition failed", "error", err, "kind", kind, "game_id", gameID, "user_id", in.UserID)
		if err.Error() == forbiddenMsg {
			return nil, h.logAndErr(ctx, core.ErrForbidden(err.Error()), "Role transition forbidden", "error", err.Error())
		}
		return nil, h.logAndErr(ctx, core.ErrBadRequest(err), "Bad role transition request", "error", err)
	}

	h.App.ObsLogger.Info(ctx, successMsg, "game_id", gameID, targetKey, in.UserID, actorKey, requestingUserID)
	return &noContentOutput{}, nil
}

// Applications

type applyToGameInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
	Body   *applyToGameBody
}

func (h *Handler) humaApplyToGame(ctx context.Context, in *applyToGameInput) (*applicationOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_apply_to_game")()

	// Email verification, which the chi route enforced with
	// RequireEmailVerificationMiddleware (gotcha 15).
	if errResp := core.RequireVerifiedEmailCtx(ctx, h.App.Pool); errResp != nil {
		return nil, humaErr(errResp)
	}

	game, err := gameFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	authUser, err := h.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	application, err := h.GameApplicationService.CreateGameApplication(ctx, core.CreateGameApplicationRequest{
		GameID:  game.ID,
		UserID:  authUser.ID,
		Role:    in.Body.Role,
		Message: in.Body.Message,
	})
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to create game application", "error", err, "game_id", game.ID, "user_id", authUser.ID)

		// These four are all conditions the applicant can see and act on, so
		// they are 400s rather than 500s. A rejection in particular is terminal:
		// the user cannot re-apply.
		switch err.Error() {
		case "user already has a pending application for this game",
			"user is already a participant in this game",
			"game is not currently recruiting",
			"user's previous application was rejected":
			return nil, h.logAndErr(ctx, core.ErrBadRequest(err), "Bad apply to game request", "error", err)
		}

		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to apply to game", "error", err)
	}

	if in.Body.Role == core.RoleAudience && game.AutoAcceptAudience {
		h.autoAcceptAudience(ctx, game, application, authUser.ID, gameID)
	}

	h.notifyGMOfApplication(ctx, game, application, authUser.Username, in.Body.Role, gameID)

	response := applicationResponseFrom(application)
	response.Username = authUser.Username
	return &applicationOutput{Body: response}, nil
}

// autoAcceptAudience approves an audience application immediately when the game
// is configured for it.
//
// Best-effort throughout: a failure here leaves the application pending, which
// the GM can still approve by hand, so it must not fail the request.
func (h *Handler) autoAcceptAudience(ctx context.Context, game *models.Game, application *models.GameApplication, userID, gameID int32) {
	// ApproveGameApplication creates the participant and deletes the audience
	// application record, so an approved audience member exists only as a
	// participant; no stale 'approved' application is left behind.
	if err := h.GameApplicationService.ApproveGameApplication(ctx, application.ID, game.GmUserID); err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to auto-approve audience application",
			"application_id", application.ID, "user_id", userID, "game_id", gameID)
		return
	}

	title := fmt.Sprintf("Joined %s", game.Title)
	content := fmt.Sprintf("You have joined %s as an audience member!", game.Title)
	linkURL := fmt.Sprintf("/games/%d", gameID)
	relatedType := core.TableGameParticipants
	if _, err := h.NotificationService.CreateNotification(ctx, &core.CreateNotificationRequest{
		UserID:      userID,
		GameID:      &application.GameID,
		Type:        core.NotificationTypeApplicationApproved,
		Title:       title,
		Content:     &content,
		RelatedType: &relatedType,
		RelatedID:   &application.GameID, // The application row is gone; link to the game.
		LinkURL:     &linkURL,
	}); err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to send auto-approval notification", "user_id", userID, "game_id", gameID)
	}
}

func (h *Handler) notifyGMOfApplication(ctx context.Context, game *models.Game, application *models.GameApplication, username, role string, gameID int32) {
	roleLabel := "player"
	if role == "audience" {
		roleLabel = "audience member"
	}
	title := fmt.Sprintf("New %s application for %s", roleLabel, game.Title)
	content := fmt.Sprintf("%s applied to join your game as a %s", username, roleLabel)
	linkURL := fmt.Sprintf("/games/%d?tab=applications", gameID)
	relatedType := core.TableGameApplications

	if _, err := h.NotificationService.CreateNotification(ctx, &core.CreateNotificationRequest{
		UserID:      game.GmUserID,
		GameID:      &application.GameID,
		Type:        core.NotificationTypeApplicationSubmitted,
		Title:       title,
		Content:     &content,
		RelatedType: &relatedType,
		RelatedID:   &application.ID,
		LinkURL:     &linkURL,
	}); err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to create notification for GM", "error", err, "game_id", gameID, "gm_user_id", game.GmUserID)
	}
}

func (h *Handler) humaGetGameApplications(ctx context.Context, in *gameScopedInput) (*applicationsOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_game_applications")()

	if err := h.requireGMFlag(ctx, "only the GM can view game applications", "Get game applications forbidden"); err != nil {
		return nil, err
	}

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	applications, err := h.GameApplicationService.GetGameApplications(ctx, gameID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get game applications", "error", err, "game_id", gameID)
	}

	// make(...,0): an empty list is [] here, unlike the participants endpoint.
	response := make([]map[string]any, 0)
	for _, app := range applications {
		appData := map[string]any{
			"id":       app.ID,
			"game_id":  app.GameID,
			"user_id":  app.UserID,
			"username": app.Username,
			// Email is intentionally omitted for privacy.
			"role":       app.Role,
			"status":     app.Status,
			"applied_at": app.AppliedAt.Time,
		}
		if app.AvatarUrl.Valid {
			appData["avatar_url"] = app.AvatarUrl.String
		}
		if app.Message.Valid {
			appData["message"] = app.Message.String
		}
		if app.ReviewedAt.Valid {
			appData["reviewed_at"] = app.ReviewedAt.Time
		}
		if app.ReviewedByUserID.Valid {
			appData["reviewed_by_user_id"] = app.ReviewedByUserID.Int32
		}
		response = append(response, appData)
	}

	return &applicationsOutput{Body: response}, nil
}

type reviewApplicationInput struct {
	GameID        int32 `path:"gameID" doc:"Game ID"`
	ApplicationID int32 `path:"applicationId" doc:"Application ID"`
	Body          *reviewApplicationBody
}

func (h *Handler) humaReviewGameApplication(ctx context.Context, in *reviewApplicationInput) (*applicationOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_review_game_application")()

	if err := h.requireGMFlag(ctx, "only the GM can review game applications", "Review game application forbidden"); err != nil {
		return nil, err
	}

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	game, err := gameFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	authUser, err := h.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	application, err := h.GameApplicationService.GetGameApplication(ctx, in.ApplicationID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get game application", "error", err, "application_id", in.ApplicationID)
	}

	// Binds the application to the game in the URL: permission was granted over
	// {gameID}, but the row acted on is named by {applicationId}.
	if application.GameID != gameID {
		return nil, h.logAndErr(ctx, core.ErrBadRequest(fmt.Errorf("application does not belong to this game")),
			"Bad review game application request")
	}

	if in.Body.Action == "approve" {
		err = h.GameApplicationService.ApproveGameApplication(ctx, in.ApplicationID, authUser.ID)
	} else {
		err = h.GameApplicationService.RejectGameApplication(ctx, in.ApplicationID, authUser.ID)
	}
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to review game application",
			"error", err, "application_id", in.ApplicationID, "action", in.Body.Action)
	}

	// Audience approval makes the applicant a participant right away and deletes
	// their application, so — unlike player applicants, notified in bulk when the
	// GM closes recruitment — there is no later step that would notify them.
	if in.Body.Action == "approve" && application.Role == core.RoleAudience {
		if notifErr := h.NotificationService.NotifyApplicationApproved(ctx, application.UserID, gameID, game.Title); notifErr != nil {
			h.App.ObsLogger.Warn(ctx, "Failed to send audience approval notification", "error", notifErr, "game_id", gameID, "user_id", application.UserID)
		}
	}

	// Approving an audience application deletes it, so this re-fetch can
	// legitimately fail. Synthesize the response from the row loaded above
	// rather than erroring: the approval itself succeeded.
	updatedApplication, err := h.GameApplicationService.GetGameApplication(ctx, in.ApplicationID)
	if err != nil {
		if in.Body.Action == "approve" && application.Role == core.RoleAudience {
			reviewerID := authUser.ID
			response := applicationResponseFrom(application)
			response.Status = core.ApplicationStatusApproved
			response.ReviewedByUserID = &reviewerID
			response.ReviewedAt = nil
			return &applicationOutput{Body: response}, nil
		}
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get updated application", "error", err, "application_id", in.ApplicationID)
	}

	return &applicationOutput{Body: applicationResponseFrom(updatedApplication)}, nil
}

func (h *Handler) humaGetMyGameApplication(ctx context.Context, in *gameScopedInput) (*myApplicationOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_my_game_application")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	authUser, err := h.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	application, err := h.GameApplicationService.GetGameApplicationByUserAndGame(ctx, gameID, authUser.ID)
	if err != nil {
		// No application is the expected case, not an error: 200 with a bare
		// null, which is how the client tells "never applied" from a failure.
		return &myApplicationOutput{Body: nil}, nil
	}

	// Player applications are decided in bulk when the GM closes recruitment
	// (see PublishApplicationStatuses), so their real status is hidden as
	// "pending" until that publish step runs. Audience applications are decided
	// individually and immediately and never go through that step, so
	// IsPublished is never set for them. A surviving audience application is
	// therefore a rejection (approvals delete the row), and the applicant should
	// see "rejected" right away rather than a permanent, misleading "pending".
	displayStatus := application.Status.String
	if application.Role != core.RoleAudience && !application.IsPublished {
		displayStatus = core.ApplicationStatusPending
	}

	response := &GameApplicationResponse{
		ID:        application.ID,
		GameID:    application.GameID,
		UserID:    application.UserID,
		Role:      application.Role,
		Status:    displayStatus,
		AppliedAt: application.AppliedAt.Time,
	}
	if application.Message.Valid {
		response.Message = application.Message.String
	}
	// Review details are withheld until the status is published, so they cannot
	// be used to infer a decision the applicant is not meant to see yet.
	if application.IsPublished {
		if application.ReviewedAt.Valid {
			t := application.ReviewedAt.Time
			response.ReviewedAt = &t
		}
		if application.ReviewedByUserID.Valid {
			v := application.ReviewedByUserID.Int32
			response.ReviewedByUserID = &v
		}
	}

	return &myApplicationOutput{Body: response}, nil
}

func (h *Handler) humaGetPublicGameApplicants(ctx context.Context, in *gameScopedInput) (*applicationsOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_public_game_applicants")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	// Reuse the row the middleware already loaded rather than issuing a second
	// query on this unauthenticated endpoint.
	game, err := gameFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	if !game.State.Valid || game.State.String != core.GameStateRecruitment {
		return nil, h.logAndErr(ctx, core.ErrForbidden("applicant list is only visible during recruitment"),
			"Get public game applicants forbidden")
	}

	applicants, err := h.GameApplicationService.GetPublicGameApplicants(ctx, gameID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get public game applicants", "error", err, "game_id", gameID)
	}

	// Username and role only — no status, no review information. This endpoint
	// is readable by anyone.
	response := make([]map[string]any, 0)
	for _, applicant := range applicants {
		applicantData := map[string]any{
			"id":         applicant.ID,
			"username":   applicant.Username,
			"role":       applicant.Role,
			"applied_at": applicant.AppliedAt.Time,
		}
		if applicant.AvatarUrl.Valid {
			applicantData["avatar_url"] = applicant.AvatarUrl.String
		}
		response = append(response, applicantData)
	}

	return &applicationsOutput{Body: response}, nil
}

func (h *Handler) humaWithdrawGameApplication(ctx context.Context, in *gameScopedInput) (*noContentOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_withdraw_game_application")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	authUser, err := h.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	application, err := h.GameApplicationService.GetGameApplicationByUserAndGame(ctx, gameID, authUser.ID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrNotFound("no application found for this game"),
			"Failed to get user's application", "error", err, "game_id", gameID, "user_id", authUser.ID)
	}

	switch {
	case application.Status.String == core.ApplicationStatusPending:
		// Deleted rather than marked withdrawn, so the user can reapply.
		if err := h.GameApplicationService.DeleteGameApplication(ctx, application.ID, authUser.ID); err != nil {
			return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to delete application", "error", err, "application_id", application.ID)
		}
	case application.Status.String == core.ApplicationStatusApproved && application.Role == core.RoleAudience:
		// Audience applications create a participant row immediately on
		// approval. If that participant later left or was removed, the
		// 'approved' application row is stale — it no longer represents any live
		// membership, so allow withdrawal to clear it rather than forcing a 400
		// the user has no way to resolve.
		//
		// DeleteStaleApprovedApplicationForUser only removes it if the user is
		// NOT currently an active participant, so a genuinely active audience
		// membership is never touched.
		if err := h.GameApplicationService.DeleteStaleApprovedApplicationForUser(ctx, gameID, authUser.ID); err != nil {
			return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to delete stale application", "error", err, "application_id", application.ID)
		}
	default:
		return nil, h.logAndErr(ctx, core.ErrBadRequest(fmt.Errorf("can only withdraw pending applications")),
			"Bad withdraw game application request")
	}

	return &noContentOutput{}, nil
}

// Audience

func (h *Handler) humaListAudienceMembers(ctx context.Context, in *gameScopedInput) (*audienceMembersOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_list_audience_members")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	members, err := h.GameService.ListAudienceMembers(ctx, gameID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to list audience members", "error", err, "game_id", gameID)
	}

	response := &ListAudienceMembersResponse{
		AudienceMembers: make([]AudienceMemberResponse, len(members)),
	}
	for i, member := range members {
		response.AudienceMembers[i] = AudienceMemberResponse{
			ID:       member.ID,
			GameID:   member.GameID,
			UserID:   member.UserID,
			Username: member.Username,
			Role:     member.Role,
			Status:   member.Status.String,
			JoinedAt: member.JoinedAt.Time,
		}
	}

	return &audienceMembersOutput{Body: response}, nil
}

type autoAcceptAudienceInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
	Body   *autoAcceptAudienceBody
}

func (h *Handler) humaUpdateAutoAcceptAudience(ctx context.Context, in *autoAcceptAudienceInput) (*messageOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_auto_accept_audience")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	game, err := gameFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	authUser, err := h.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	// Primary GM only, deliberately — not is_gm.
	if game.GmUserID != authUser.ID {
		return nil, h.logAndErr(ctx, core.ErrForbidden("only the GM can update this setting"), "Update auto accept audience forbidden")
	}

	if err := h.GameService.UpdateGameAutoAcceptAudience(ctx, gameID, in.Body.AutoAcceptAudience); err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to update auto-accept audience setting", "error", err, "game_id", gameID)
	}

	out := &messageOutput{}
	out.Body.Message = "Auto-accept audience setting updated"
	return out, nil
}

func (h *Handler) humaListAudienceNPCs(ctx context.Context, in *gameScopedInput) (*audienceNPCsOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_list_audience_npcs")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	npcs, err := h.CharacterService.ListAudienceNPCs(ctx, gameID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to list audience NPCs", "error", err, "game_id", gameID)
	}

	out := &audienceNPCsOutput{}
	out.Body.NPCs = npcs
	return out, nil
}

// listConversationsInput's limit and offset are strings for the same reason as
// filteredGamesInput: the chi handler ignored unparseable and out-of-range
// values rather than rejecting them (gotcha 18).
//
// participant_ids is different — it 400s on a non-integer, so it is typed.
//
// It must carry `explode`: Huma defaults query arrays to comma-separated in a
// single value and then keeps only the FIRST occurrence of a repeated param.
// The client sends the filter as repeated params, so without `explode` every
// character after the first was silently dropped and selecting a second
// character did nothing.
type listConversationsInput struct {
	GameID         int32   `path:"gameID" doc:"Game ID"`
	Limit          string  `query:"limit" required:"false" doc:"Defaults to 20; unparseable or non-positive values are ignored"`
	Offset         string  `query:"offset" required:"false" doc:"Defaults to 0; unparseable or negative values are ignored"`
	ParticipantIDs []int32 `query:"participant_ids,explode" required:"false" doc:"Filter to conversations involving all these character IDs"`
}

func (h *Handler) humaListAllPrivateConversations(ctx context.Context, in *listConversationsInput) (*privateConversationsOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_list_all_private_conversations")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := h.requireCanViewGame(ctx, gameID, "List all private conversations forbidden"); err != nil {
		return nil, err
	}

	limit := parseBoundedInt32(in.Limit, 20, func(v int64) bool { return v > 0 })
	offset := parseBoundedInt32(in.Offset, 0, func(v int64) bool { return v >= 0 })

	conversations, err := h.MessageService.ListAllPrivateConversations(ctx, core.ListAllPrivateConversationsParams{
		GameID: gameID,
		// Filtering is by character ID rather than name: names are mutable and
		// non-unique, so a rename silently changed which conversations matched.
		ParticipantCharacterIDs: in.ParticipantIDs,
		Limit:                   limit,
		Offset:                  offset,
	})
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to list private conversations", "error", err, "game_id", gameID)
	}

	total, err := h.MessageService.CountAllPrivateConversations(ctx, gameID, in.ParticipantIDs)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to count private conversations", "error", err, "game_id", gameID)
	}

	responses := make([]PrivateConversationResponse, len(conversations))
	for i, c := range conversations {
		var subject, lastContent, lastSenderName, lastSenderUsername *string
		if c.Subject.Valid {
			subject = &c.Subject.String
		}
		if c.LastMessageContent.Valid {
			lastContent = &c.LastMessageContent.String
		}
		if c.LastSenderName.Valid {
			lastSenderName = &c.LastSenderName.String
		}
		if c.LastSenderUsername.Valid {
			lastSenderUsername = &c.LastSenderUsername.String
		}
		var lastSenderCharID *int32
		if c.LastSenderCharacterID.Valid {
			lastSenderCharID = &c.LastSenderCharacterID.Int32
		}
		responses[i] = PrivateConversationResponse{
			ConversationID:          c.ConversationID,
			Subject:                 subject,
			ConversationType:        c.ConversationType,
			CreatedAt:               c.CreatedAt.Time.Format(time.RFC3339),
			MessageCount:            c.MessageCount,
			LastMessageAt:           c.LastMessageAt,
			ParticipantNames:        c.ParticipantNames,
			ParticipantUsernames:    c.ParticipantUsernames,
			ParticipantCharacterIDs: c.ParticipantCharacterIds,
			LastMessageContent:      lastContent,
			LastSenderName:          lastSenderName,
			LastSenderUsername:      lastSenderUsername,
			LastSenderCharacterID:   lastSenderCharID,
		}
	}

	out := &privateConversationsOutput{}
	out.Body.Conversations = responses
	out.Body.Total = total
	return out, nil
}

// parseBoundedInt32 reproduces the chi handlers' lenient parsing: a value that
// does not parse, or that fails accept, leaves the default in place.
func parseBoundedInt32(raw string, def int32, accept func(int64) bool) int32 {
	if raw == "" {
		return def
	}
	v, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || !accept(v) {
		return def
	}
	return int32(v)
}

type conversationMessagesInput struct {
	GameID         int32 `path:"gameID" doc:"Game ID"`
	ConversationID int32 `path:"conversationId" doc:"Conversation ID"`
}

func (h *Handler) humaGetAudienceConversationMessages(ctx context.Context, in *conversationMessagesInput) (*conversationMessagesOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_audience_conversation_messages")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := h.requireCanViewGame(ctx, gameID, "Get audience conversation messages forbidden"); err != nil {
		return nil, err
	}

	messages, err := h.MessageService.GetAudienceConversationMessages(ctx, in.ConversationID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get conversation messages", "error", err, "conversation_id", in.ConversationID)
	}

	responses := make([]AudienceMessageResponse, len(messages))
	for i, m := range messages {
		senderUserID := m.SenderUserID
		var senderCharID *int32
		if m.SenderCharacterID.Valid {
			senderCharID = &m.SenderCharacterID.Int32
		}
		var senderCharName *string
		if m.SenderCharacterName.Valid {
			senderCharName = &m.SenderCharacterName.String
		}
		responses[i] = AudienceMessageResponse{
			ID:                  m.ID,
			ConversationID:      m.ConversationID,
			SenderUserID:        &senderUserID,
			SenderCharacterID:   senderCharID,
			Content:             m.Content,
			CreatedAt:           m.CreatedAt.Time.Format(time.RFC3339),
			UpdatedAt:           m.UpdatedAt.Time.Format(time.RFC3339),
			IsDeleted:           m.IsDeleted.Bool,
			SenderUsername:      m.SenderUsername,
			SenderCharacterName: senderCharName,
		}
	}

	out := &conversationMessagesOutput{}
	out.Body.Messages = responses
	return out, nil
}

// conversationParticipantsInput's filter param is literally named "selected[]",
// brackets included, which is what the frontend sends.
type conversationParticipantsInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
	// `explode` for the same reason as listConversationsInput.ParticipantIDs:
	// repeated params are otherwise truncated to the first value.
	Selected []int32 `query:"selected[],explode" required:"false" doc:"Character IDs to intersect on"`
}

func (h *Handler) humaGetConversationParticipants(ctx context.Context, in *conversationParticipantsInput) (*conversationParticipantsOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_conversation_participants")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := h.requireCanViewGame(ctx, gameID, "Get conversation participants forbidden"); err != nil {
		return nil, err
	}

	characters, err := h.MessageService.GetConversationParticipantCharacters(ctx, gameID, in.Selected)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get conversation participants", "error", err, "game_id", gameID)
	}

	out := &conversationParticipantsOutput{}
	out.Body.Participants = characters
	return out, nil
}

type actionSubmissionsInput struct {
	GameID  int32  `path:"gameID" doc:"Game ID"`
	PhaseID string `query:"phase_id" required:"false" doc:"Restrict to one phase; omitted or unparseable means all phases"`
	Limit   string `query:"limit" required:"false" doc:"Defaults to 10; unparseable or non-positive values are ignored"`
	Offset  string `query:"offset" required:"false" doc:"Defaults to 0; unparseable or negative values are ignored"`
}

func (h *Handler) humaListAllActionSubmissions(ctx context.Context, in *actionSubmissionsInput) (*actionSubmissionsOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_list_all_action_submissions")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := h.requireCanViewGame(ctx, gameID, "List all action submissions forbidden"); err != nil {
		return nil, err
	}

	// 0 means all phases. Any parseable value is accepted, including negatives,
	// which simply match nothing — as before.
	phaseID := parseBoundedInt32(in.PhaseID, 0, func(int64) bool { return true })
	limit := parseBoundedInt32(in.Limit, 10, func(v int64) bool { return v > 0 })
	offset := parseBoundedInt32(in.Offset, 0, func(v int64) bool { return v >= 0 })

	submissions, err := h.ActionSubmissionService.ListAllActionSubmissions(ctx, gameID, phaseID, limit, offset)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to list action submissions", "error", err, "game_id", gameID)
	}

	total, err := h.ActionSubmissionService.CountAllActionSubmissions(ctx, gameID, phaseID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to count action submissions", "error", err, "game_id", gameID)
	}

	responses := make([]ActionSubmissionResponse, len(submissions))
	for i, s := range submissions {
		var charID *int32
		if s.CharacterID.Valid {
			charID = &s.CharacterID.Int32
		}
		var charName *string
		if s.CharacterName.Valid {
			charName = &s.CharacterName.String
		}
		var submittedAt, updatedAt *string
		if s.SubmittedAt.Valid {
			t := s.SubmittedAt.Time.Format(time.RFC3339)
			submittedAt = &t
		}
		if s.UpdatedAt.Valid {
			t := s.UpdatedAt.Time.Format(time.RFC3339)
			updatedAt = &t
		}
		responses[i] = ActionSubmissionResponse{
			ID:            s.ID,
			GameID:        s.GameID,
			UserID:        s.UserID,
			PhaseID:       s.PhaseID,
			CharacterID:   charID,
			Content:       s.Content,
			SubmittedAt:   submittedAt,
			UpdatedAt:     updatedAt,
			Username:      s.Username,
			CharacterName: charName,
			PhaseType:     s.PhaseType,
			PhaseNumber:   s.PhaseNumber,
			PhaseTitle:    s.PhaseTitle,
		}
	}

	out := &actionSubmissionsOutput{}
	out.Body.ActionSubmissions = responses
	out.Body.Total = total
	return out, nil
}

// Logs and stats

func (h *Handler) humaGetGameLogs(ctx context.Context, in *gameScopedInput) (*gameLogsOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_logs")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := h.requireAuth(ctx); err != nil {
		return nil, err
	}
	game, err := gameFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	// Once a game is over its log becomes readable by any participant; while it
	// is running the log would reveal GM activity, so it is GM-only.
	if game.State.String != core.GameStateCompleted && game.State.String != core.GameStateCancelled {
		if err := h.requireGMFlag(ctx, "only the GM can retrieve game logs while the game is running", "Game logs access forbidden"); err != nil {
			return nil, err
		}
	}

	logs, err := h.GameService.GetGameLogs(ctx, gameID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get game logs", "error", err, "game_id", gameID)
	}

	response := make([]map[string]any, 0)
	for _, log := range logs {
		response = append(response, map[string]any{
			"id":         log.ID,
			"game_id":    log.GameID,
			"type":       log.Type,
			"message":    log.Message.String,
			"created_at": log.CreatedAt.Time,
		})
	}

	return &gameLogsOutput{Body: response}, nil
}

// humaGetGameStats serves post-game statistics for a completed game.
//
// Authorization is deliberately narrow: stats aggregate over private messages
// as well as public comments, so they are only served once the game reaches
// `completed` and becomes a public archive readable by any authenticated user
// (CanUserViewGame). A cancelled or in-progress game returns 409 even for its
// GM — mid-game these numbers would leak the shape of private activity (who is
// talking to whom, and how much), and they would be a live scoreboard players
// could optimize against.
func (h *Handler) humaGetGameStats(ctx context.Context, in *gameScopedInput) (*gameStatsOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_game_stats")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	game, err := gameFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	// Checked before the view permission so an in-progress game reports the
	// actual reason rather than a 403.
	if game.State.String != core.GameStateCompleted {
		return nil, h.logAndErr(ctx, core.ErrConflict("statistics are only available for completed games"),
			"Game stats requested for non-completed game", "game_id", gameID, "state", game.State.String)
	}

	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		// 403 rather than 401, matching the chi handler.
		return nil, h.logAndErr(ctx, core.ErrForbidden("authentication required"),
			"Unauthenticated game stats request", "game_id", gameID)
	}

	// Reuse the canonical read check rather than hand-rolling a participant
	// test: it already encodes public archive mode for completed games.
	canView, err := h.GameService.CanUserViewGame(ctx, gameID, authUser.ID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to check game view permission for stats",
			"error", err, "game_id", gameID, "user_id", authUser.ID)
	}
	if !canView {
		return nil, h.logAndErr(ctx, core.ErrForbidden("you do not have access to this game"),
			"Forbidden game stats request", "game_id", gameID, "user_id", authUser.ID)
	}

	stats, err := h.GameStatsService.GetGameStats(ctx, gameID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to compute game stats", "error", err, "game_id", gameID)
	}

	return &gameStatsOutput{Body: stats}, nil
}

// Loot tables

// lootTablesInput's exclude-empty is a presence flag: the chi handler used
// Query().Has(), so any value at all — including an empty one — enabled it.
// A bool field would instead require "true", so the raw string is kept and the
// presence test reproduced below.
type lootTablesInput struct {
	GameID       int32  `path:"gameID" doc:"Game ID"`
	ExcludeEmpty string `query:"exclude-empty" required:"false" doc:"Present with any value (including empty) to omit tables with no items"`
}

func (h *Handler) humaGetGameLootTables(ctx context.Context, in *lootTablesInput) (*lootTablesOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_loot_tables")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.requireGMFlag(ctx, "only the GM can see and edit loot tables", "Loot tables access forbidden"); err != nil {
		return nil, err
	}

	// Presence, not value. Huma gives "" both for an absent parameter and for
	// "?exclude-empty" with no value, so the two cannot be told apart here;
	// "?exclude-empty" alone no longer enables the filter, while
	// "?exclude-empty=1" (what the frontend sends, as =true) still does.
	excludeEmpty := in.ExcludeEmpty != ""

	lootTables, err := h.GameService.GetGameLootTables(ctx, gameID, excludeEmpty)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get game loot tables", "error", err, "game_id", gameID)
	}

	response := make([]map[string]any, 0)
	for _, lootTable := range lootTables {
		// Keep these keys in sync with the model returned by AddGameLootTable —
		// both are typed as LootTable on the frontend, so a field present in one
		// and missing from the other is a shape mismatch the types do not catch.
		response = append(response, map[string]any{
			"id":         lootTable.ID,
			"game_id":    lootTable.GameID,
			"name":       lootTable.Name,
			"created_at": lootTable.CreatedAt.Time,
			"updated_at": lootTable.UpdatedAt.Time,
		})
	}

	return &lootTablesOutput{Body: response}, nil
}

type addLootTableInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
	Body   *updateLootTableBody
}

// humaAddGameLootTable answers 200, not 201.
//
// The chi handler encoded the new row without setting a status, so it has
// always been a 200 create. Preserved rather than corrected.
func (h *Handler) humaAddGameLootTable(ctx context.Context, in *addLootTableInput) (*lootTableOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_loot_tables_add")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.requireGMFlag(ctx, "only the GM can see and edit loot tables", "Loot tables access forbidden"); err != nil {
		return nil, err
	}

	newLootTable, err := h.GameService.CreateLootTable(ctx, gameID, in.Body.Name)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to create loot table", "error", err, "game_id", gameID)
	}

	for _, item := range in.Body.Items {
		if _, err := h.GameService.AddLootTableContent(ctx, newLootTable.ID, item.Name, item.Data); err != nil {
			return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to save loot table items", "error", err, "game_id", gameID)
		}
	}

	return &lootTableOutput{Body: newLootTable}, nil
}

type updateLootTableInput struct {
	GameID  int32 `path:"gameID" doc:"Game ID"`
	TableID int32 `path:"tableId" doc:"Loot table ID"`
	Body    *updateLootTableBody
}

func (h *Handler) humaUpdateGameLootTable(ctx context.Context, in *updateLootTableInput) (*lootTableOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_loot_tables_update")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.requireGMFlag(ctx, "only the GM can see and edit loot tables", "Loot tables access forbidden"); err != nil {
		return nil, err
	}
	if err := h.requireLootTableInGame(ctx, in.TableID, gameID); err != nil {
		return nil, err
	}

	// Renames only: the Items in the body are validated but not applied, which
	// is what the chi handler did. The contents endpoint is how items change.
	lootTable, err := h.GameService.UpdateLootTable(ctx, in.TableID, in.Body.Name)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to update loot table", "error", err, "table_id", in.TableID)
	}

	return &lootTableOutput{Body: lootTable}, nil
}

func (h *Handler) humaDeleteGameLootTable(ctx context.Context, in *tableScopedInput) (*emptyOKOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_loot_tables_delete")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.requireGMFlag(ctx, "only the GM can see and edit loot tables", "Loot tables access forbidden"); err != nil {
		return nil, err
	}
	if err := h.requireLootTableInGame(ctx, in.TableID, gameID); err != nil {
		return nil, err
	}

	if err := h.GameService.DeleteLootTable(ctx, in.TableID); err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to delete loot table", "error", err, "table_id", in.TableID)
	}

	return &emptyOKOutput{Status: http.StatusOK}, nil
}

func (h *Handler) humaGetGameLootTableContents(ctx context.Context, in *tableScopedInput) (*lootContentsOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_loot_tables")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.requireGMFlag(ctx, "only the GM can see and edit loot tables", "Loot tables access forbidden"); err != nil {
		return nil, err
	}
	if err := h.requireLootTableInGame(ctx, in.TableID, gameID); err != nil {
		return nil, err
	}

	contents, err := h.GameService.GetGameLootTableContents(ctx, in.TableID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get loot table contents", "error", err, "table_id", in.TableID)
	}

	response := make([]map[string]any, 0)
	for _, item := range contents {
		response = append(response, map[string]any{
			"id":   item.ID,
			"name": item.Name,
			"data": item.Data.String,
		})
	}

	return &lootContentsOutput{Body: response}, nil
}

type updateLootContentsInput struct {
	GameID  int32 `path:"gameID" doc:"Game ID"`
	TableID int32 `path:"tableId" doc:"Loot table ID"`
	Body    *updateLootContentsBody
}

func (h *Handler) humaUpdateGameLootTableContent(ctx context.Context, in *updateLootContentsInput) (*emptyOKOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_loot_tables")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.requireGMFlag(ctx, "only the GM can see and edit loot tables", "Loot tables access forbidden"); err != nil {
		return nil, err
	}
	if err := h.requireLootTableInGame(ctx, in.TableID, gameID); err != nil {
		return nil, err
	}

	// One transaction: the rewrite deletes every existing item before inserting
	// the new ones, so a partial failure would otherwise leave the GM's table
	// empty. This also bumps updated_at, which the child-table writes never touch.
	items := make([]core.LootTableItem, 0, len(in.Body.Items))
	for _, item := range in.Body.Items {
		items = append(items, core.LootTableItem{Name: item.Name, Data: item.Data})
	}

	if err := h.GameService.ReplaceLootTableContents(ctx, in.TableID, items); err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to save loot table items",
			"error", err, "table_id", in.TableID, "game_id", gameID)
	}

	return &emptyOKOutput{Status: http.StatusOK}, nil
}

type randomLootInput struct {
	GameID      int32 `path:"gameID" doc:"Game ID"`
	TableID     int32 `path:"tableId" doc:"Loot table ID"`
	CharacterID int32 `path:"characterId" doc:"Character to grant the item to"`
}

func (h *Handler) humaSetRandomLootForCharacter(ctx context.Context, in *randomLootInput) (*lootContentOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_loot_tables")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.requireGMFlag(ctx, "only the GM can see and edit loot tables", "Loot tables access forbidden"); err != nil {
		return nil, err
	}
	user, err := h.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	canEdit, err := h.CharacterService.CanUserEditCharacter(ctx, in.CharacterID, user.ID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to check character edit permission", "error", err)
	}
	if !canEdit {
		return nil, h.logAndErr(ctx, core.ErrForbidden("you cannot edit this character"),
			"Character edit permission denied", "character_id", in.CharacterID, "user_id", user.ID)
	}

	if err := h.requireLootTableInGame(ctx, in.TableID, gameID); err != nil {
		return nil, err
	}

	contents, err := h.GameService.GetGameLootTableContents(ctx, in.TableID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get loot table contents", "error", err, "table_id", in.TableID)
	}

	// A table with no items is reachable through normal use (the create endpoint
	// accepts one), and rand.Intn(0) panics. Fail as a client error the GM can
	// act on rather than letting the recovery middleware turn it into an opaque 500.
	if len(contents) == 0 {
		return nil, h.logAndErr(ctx, core.ErrInvalidRequest(fmt.Errorf("loot table is empty: add at least one item before rolling")),
			"Roll requested on empty loot table", "table_id", in.TableID, "game_id", gameID)
	}

	character, err := h.CharacterService.GetCharacter(ctx, in.CharacterID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get character", "error", err, "character_id", in.CharacterID)
	}

	rnd := rand.Intn(len(contents))
	content := contents[rnd]

	if err := h.CharacterService.AddToCharacterData(ctx, core.CharacterDataRequest{
		CharacterID: in.CharacterID,
		ModuleType:  "inventory",
		FieldName:   "items",
		FieldValue:  content.Data.String,
		FieldType:   "json",
		IsPublic:    false,
	}); err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to add loot to character",
			"error", err, "character_id", in.CharacterID, "loot_table_id", in.TableID)
	}

	// The loot is already granted, so a failed log entry must not fail the
	// request — but it should not vanish silently either, since the game log is
	// the GM's only record of what was rolled.
	if _, err := h.GameService.AddGameLog(ctx, models.CreateLogParams{
		GameID:  gameID,
		Type:    "INVENTORY_ADD",
		Message: pgtype.Text{String: fmt.Sprintf("Added %s to Character %s (Rolled: %d)", content.Name, character.Name, rnd+1), Valid: true},
	}); err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to write loot roll game log",
			"game_id", gameID, "character_id", in.CharacterID, "loot_table_id", in.TableID)
	}

	return &lootContentOutput{Body: &content}, nil
}

// Banner

// bannerUpload declares the multipart field. No contentType tag: huma would
// validate the MIME before the handler and emit its own message, replacing the
// service's friendlier "invalid file type X. Only JPG, PNG, and WebP images are
// allowed" — which is what users actually see. Status is 400 either way, so
// tests do not catch the difference. See the multipart notes in the migration plan.
type bannerUpload struct {
	Banner huma.FormFile `form:"banner" required:"true" doc:"Banner image: JPG, PNG or WebP, at most 5MB"`
}

type uploadBannerInput struct {
	GameID  int32 `path:"gameID" doc:"Game ID"`
	RawBody huma.MultipartFormFiles[bannerUpload]
}

func (h *Handler) humaUploadGameBanner(ctx context.Context, in *uploadBannerInput) (*bannerOutput, error) {
	game, err := gameFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		return nil, h.logAndErr(ctx, errResp, "Request rejected in upload game banner")
	}

	// Primary GM only, deliberately — not is_gm, so a co-GM cannot change the
	// banner and admin mode does not grant it.
	if game.GmUserID != userID {
		return nil, h.logAndErr(ctx, core.ErrForbidden("Only the GM can update the game banner"), "Upload game banner forbidden")
	}

	file := in.RawBody.Data().Banner
	contentType := file.ContentType
	if contentType == "" {
		contentType = bannerMimeTypeFromFilename(file.Filename)
	}
	if !allowedBannerMimeTypes[contentType] {
		return nil, h.logAndErr(ctx, core.ErrInvalidRequest(fmt.Errorf("invalid file type %s. Only JPG, PNG, and WebP images are allowed", contentType)),
			"Invalid upload game banner request")
	}

	fileData, err := readAndValidateBannerSize(file)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInvalidRequest(err), "Invalid upload game banner request", "error", err)
	}

	// Removed before the new one is stored, so a game never accumulates orphaned
	// banner objects. Best-effort: a failed delete must not block the upload.
	if game.BannerUrl.Valid && game.BannerUrl.String != "" {
		_ = h.App.Storage.Delete(ctx, extractBannerPathFromURL(game.BannerUrl.String))
	}

	ext := filepath.Ext(file.Filename)
	if ext == "" {
		ext = bannerExtFromMime(contentType)
	}
	storagePath := fmt.Sprintf("banners/games/%d/%d%s", game.ID, time.Now().Unix(), ext)

	bannerURL, err := h.App.Storage.Upload(ctx, storagePath, fileData, contentType)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(fmt.Errorf("failed to upload banner: %w", err)), "Failed to upload game banner")
	}

	if err := h.GameService.UpdateGameBannerURL(ctx, game.ID, &bannerURL); err != nil {
		// Roll the object back, or the bucket keeps a file no row points at.
		_ = h.App.Storage.Delete(ctx, storagePath)
		return nil, h.logAndErr(ctx, core.ErrInternalError(fmt.Errorf("failed to save banner URL: %w", err)), "Failed to upload game banner")
	}

	out := &bannerOutput{}
	out.Body.BannerURL = bannerURL
	return out, nil
}

func (h *Handler) humaDeleteGameBanner(ctx context.Context, in *gameScopedInput) (*noContentOutput, error) {
	game, err := gameFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		return nil, h.logAndErr(ctx, errResp, "Request rejected in delete game banner")
	}

	if game.GmUserID != userID {
		return nil, h.logAndErr(ctx, core.ErrForbidden("Only the GM can remove the game banner"), "Delete game banner forbidden")
	}

	if game.BannerUrl.Valid && game.BannerUrl.String != "" {
		_ = h.App.Storage.Delete(ctx, extractBannerPathFromURL(game.BannerUrl.String))
	}

	if err := h.GameService.UpdateGameBannerURL(ctx, game.ID, nil); err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(fmt.Errorf("failed to remove banner: %w", err)), "Failed to delete game banner")
	}

	return &noContentOutput{}, nil
}

// Registration

// RegisterHumaGamesPublicList registers the game listing, which requires no
// authentication and no game context.
//
// It sits on a chi group running jwtauth.Verifier without Authenticator, so a
// token is read if present and simply absent otherwise -- a different middleware
// stack from every other game route, which is why it needs its own huma API
// (gotcha 19).
func RegisterHumaGamesPublicList(api huma.API, h *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "listGames",
		Method:      http.MethodGet,
		Path:        "/",
		Summary:     "List games",
		Description: "The main game listing, with filtering, sorting and pagination. Authentication is optional: a signed-in caller also gets their relationship to each game.",
		Tags:        []string{"Games"},
	}, h.humaGetFilteredGames)
}

// RegisterHumaGamesPublicApplicants registers the public applicant list.
//
// Separate from the listing because this one additionally needs GameMiddleware
// to load the game, and a huma API binds to a single router.
func RegisterHumaGamesPublicApplicants(api huma.API, h *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "listPublicGameApplicants",
		Method:      http.MethodGet,
		Path:        "/{gameID}/applicants",
		Summary:     "List public applicants",
		Description: "Usernames and roles of a recruiting game's applicants. No status or review information, and readable without authentication.",
		Tags:        []string{"Game Applications"},
		Responses: map[string]*huma.Response{
			"403": {Description: "The game is not recruiting"},
			"404": {Description: "Game not found"},
		},
	}, h.humaGetPublicGameApplicants)
}

// RegisterHumaGamesCollection registers the two operations that need no game
// context: the recruiting list and game creation. They live on the /games
// router itself, outside the /{gameID} subrouter.
func RegisterHumaGamesCollection(api huma.API, h *Handler) {
	bearer := []map[string][]string{{"BearerAuth": {}}}

	huma.Register(api, huma.Operation{
		OperationID: "listRecruitingGames",
		Method:      http.MethodGet,
		Path:        "/recruiting",
		Summary:     "List recruiting games",
		Description: "Games currently accepting applications.",
		Tags:        []string{"Games"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaGetRecruitingGames)

	huma.Register(api, huma.Operation{
		OperationID:   "createGame",
		Method:        http.MethodPost,
		Path:          "/",
		Summary:       "Create a game",
		Description:   "Creates a game with the caller as GM. Requires a verified email address.",
		Tags:          []string{"Games"},
		Security:      bearer,
		DefaultStatus: http.StatusCreated,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid request body, or an incomplete common-room schedule"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Email address not verified"},
		},
	}, h.humaCreateGame)

}

// RegisterHumaGameScoped registers the operations under /games/{gameID}.
//
// Paths are relative to that subrouter, whose GameMiddleware has already loaded
// the game and computed is_gm before any of these handlers run.
func RegisterHumaGameScoped(api huma.API, h *Handler) {
	bearer := []map[string][]string{{"BearerAuth": {}}}

	huma.Register(api, huma.Operation{
		OperationID: "getGame",
		Method:      http.MethodGet,
		Path:        "/",
		Summary:     "Get a game",
		Description: "Returns the game's settings and metadata.",
		Tags:        []string{"Games"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"404": {Description: "Game not found"},
		},
	}, h.humaGetGame)

	huma.Register(api, huma.Operation{
		OperationID: "getGameDetails",
		Method:      http.MethodGet,
		Path:        "/details",
		Summary:     "Get a game with details",
		Description: "As getGame, plus the GM's username and the current player count.",
		Tags:        []string{"Games"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"404": {Description: "Game not found"},
		},
	}, h.humaGetGameWithDetails)

	huma.Register(api, huma.Operation{
		OperationID: "updateGame",
		Method:      http.MethodPut,
		Path:        "/",
		Summary:     "Update a game",
		Description: "Replaces the game's settings. GM only.",
		Tags:        []string{"Games"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid request body, or an incomplete common-room schedule"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can update this game"},
			"404": {Description: "Game not found"},
		},
	}, h.humaUpdateGame)

	huma.Register(api, huma.Operation{
		OperationID: "deleteGame",
		Method:      http.MethodDelete,
		Path:        "/",
		Summary:     "Delete a game",
		Description: "Permanently deletes a cancelled game. GM only.",
		Tags:        []string{"Games"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "The game is not cancelled and so cannot be deleted"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can delete this game"},
			"404": {Description: "Game not found"},
		},
	}, h.humaDeleteGame)

	huma.Register(api, huma.Operation{
		OperationID: "updateGameState",
		Method:      http.MethodPut,
		Path:        "/state",
		Summary:     "Update game state",
		Description: "Moves the game to a new state. Leaving recruitment converts approved applications into participants; pause, resume and endgame moves notify participants. The response carries only the core game fields, not the full settings.",
		Tags:        []string{"Games"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid request body"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can update this game state"},
			"409": {Description: "The transition is not legal from the game's current state"},
		},
	}, h.humaUpdateGameState)

	huma.Register(api, huma.Operation{
		OperationID: "uploadGameBanner",
		Method:      http.MethodPost,
		Path:        "/banner",
		Summary:     "Upload a game banner",
		Description: "Replaces the game's banner image. JPG, PNG or WebP, at most 5MB. Primary GM only.",
		Tags:        []string{"Games"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "Missing file, unsupported type, or larger than 5MB"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can update the game banner"},
		},
	}, h.humaUploadGameBanner)

	huma.Register(api, huma.Operation{
		OperationID: "deleteGameBanner",
		Method:      http.MethodDelete,
		Path:        "/banner",
		Summary:     "Remove a game banner",
		Description: "Clears the game's banner image. Primary GM only.",
		Tags:        []string{"Games"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can remove the game banner"},
		},
	}, h.humaDeleteGameBanner)

	// Participants

	huma.Register(api, huma.Operation{
		OperationID: "listGameParticipants",
		Method:      http.MethodGet,
		Path:        "/participants",
		Summary:     "List participants",
		Description: "Lists the game's participants. In an anonymous game, viewers who may not see former-player status get those participants reported as ordinary players.",
		Tags:        []string{"Participants"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaGetGameParticipants)

	huma.Register(api, huma.Operation{
		OperationID: "leaveGame",
		Method:      http.MethodDelete,
		Path:        "/leave",
		Summary:     "Leave a game",
		Description: "Removes the caller from the game, deactivating their characters, and deletes any pending application.",
		Tags:        []string{"Participants"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"404": {Description: "The caller is neither a participant nor an applicant"},
		},
	}, h.humaLeaveGame)

	huma.Register(api, huma.Operation{
		OperationID:   "addParticipantDirectly",
		Method:        http.MethodPost,
		Path:          "/participants/direct-add",
		Summary:       "Add a participant directly",
		Description:   "Adds a user as a player or audience member without an application. Primary GM only.",
		Tags:          []string{"Participants"},
		Security:      bearer,
		DefaultStatus: http.StatusCreated,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid request body"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can add participants directly"},
			"404": {Description: "User not found"},
		},
	}, h.humaAddParticipantDirectly)

	huma.Register(api, huma.Operation{
		OperationID: "removePlayer",
		Method:      http.MethodDelete,
		Path:        "/participants/{userId}",
		Summary:     "Remove a player",
		Description: "Removes a player and deactivates their characters. Primary GM only, and the GM cannot remove themselves.",
		Tags:        []string{"Participants"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can remove players"},
			"409": {Description: "The GM cannot remove themselves"},
		},
	}, h.humaRemovePlayer)

	huma.Register(api, huma.Operation{
		OperationID: "promoteToCoGM",
		Method:      http.MethodPost,
		Path:        "/participants/{userId}/promote-to-co-gm",
		Summary:     "Promote to co-GM",
		Description: "Promotes an audience member to co-GM. Primary GM only.",
		Tags:        []string{"Participants"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "The user cannot be promoted from their current role"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the primary GM can promote users to co-GM"},
		},
	}, h.humaPromoteToCoGM)

	huma.Register(api, huma.Operation{
		OperationID: "demoteFromCoGM",
		Method:      http.MethodPost,
		Path:        "/participants/{userId}/demote-from-co-gm",
		Summary:     "Demote a co-GM",
		Description: "Returns a co-GM to the audience. Primary GM only.",
		Tags:        []string{"Participants"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "The user is not a co-GM"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the primary GM can demote co-GMs"},
		},
	}, h.humaDemoteFromCoGM)

	huma.Register(api, huma.Operation{
		OperationID: "transitionPlayerToAudience",
		Method:      http.MethodPost,
		Path:        "/participants/{userId}/to-audience",
		Summary:     "Move a player to the audience",
		Description: "Moves a player to the audience without deactivating their characters — the permadeath path. Primary GM only.",
		Tags:        []string{"Participants"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "The user is not a player"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the primary GM can transition players to audience"},
		},
	}, h.humaTransitionPlayerToAudience)

	// Applications

	huma.Register(api, huma.Operation{
		OperationID:   "applyToGame",
		Method:        http.MethodPost,
		Path:          "/apply",
		Summary:       "Apply to a game",
		Description:   "Applies to join as a player or audience member. Requires a verified email address. An audience application is accepted immediately when the game allows it.",
		Tags:          []string{"Game Applications"},
		Security:      bearer,
		DefaultStatus: http.StatusCreated,
		Responses: map[string]*huma.Response{
			"400": {Description: "Already applied, already a participant, the game is not recruiting, or a previous application was rejected"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Email address not verified"},
		},
	}, h.humaApplyToGame)

	huma.Register(api, huma.Operation{
		OperationID: "listGameApplications",
		Method:      http.MethodGet,
		Path:        "/applications",
		Summary:     "List applications",
		Description: "Lists every application to the game, with status and review information. GM only.",
		Tags:        []string{"Game Applications"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can view game applications"},
		},
	}, h.humaGetGameApplications)

	huma.Register(api, huma.Operation{
		OperationID: "getMyGameApplication",
		Method:      http.MethodGet,
		Path:        "/application/mine",
		Summary:     "Get my application",
		Description: "Returns the caller's own application, or null if they have not applied. A player's decision reads as pending until the GM closes recruitment and publishes statuses.",
		Tags:        []string{"Game Applications"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaGetMyGameApplication)

	huma.Register(api, huma.Operation{
		OperationID: "reviewGameApplication",
		Method:      http.MethodPut,
		Path:        "/applications/{applicationId}/review",
		Summary:     "Review an application",
		Description: "Approves or rejects an application. GM only. Approving an audience application makes the applicant a participant immediately.",
		Tags:        []string{"Game Applications"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid action, or the application belongs to another game"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can review game applications"},
		},
	}, h.humaReviewGameApplication)

	huma.Register(api, huma.Operation{
		OperationID: "withdrawGameApplication",
		Method:      http.MethodDelete,
		Path:        "/application",
		Summary:     "Withdraw my application",
		Description: "Withdraws the caller's own pending application, or clears a stale approved audience application.",
		Tags:        []string{"Game Applications"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "Only pending applications can be withdrawn"},
			"401": {Description: "Not authenticated"},
			"404": {Description: "No application found for this game"},
		},
	}, h.humaWithdrawGameApplication)

	// Audience

	huma.Register(api, huma.Operation{
		OperationID: "listAudienceMembers",
		Method:      http.MethodGet,
		Path:        "/audience",
		Summary:     "List audience members",
		Description: "Lists the game's audience members.",
		Tags:        []string{"Audience"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaListAudienceMembers)

	huma.Register(api, huma.Operation{
		OperationID: "listAudienceNPCs",
		Method:      http.MethodGet,
		Path:        "/characters/audience-npcs",
		Summary:     "List audience NPCs",
		Description: "Lists the game's audience-controlled NPCs.",
		Tags:        []string{"Audience"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaListAudienceNPCs)

	huma.Register(api, huma.Operation{
		OperationID: "updateAutoAcceptAudience",
		Method:      http.MethodPut,
		Path:        "/settings/auto-accept-audience",
		Summary:     "Set auto-accept for audience",
		Description: "Controls whether audience applications are accepted without review. Primary GM only.",
		Tags:        []string{"Audience"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid request body"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can update this setting"},
		},
	}, h.humaUpdateAutoAcceptAudience)

	huma.Register(api, huma.Operation{
		OperationID: "listAllPrivateConversations",
		Method:      http.MethodGet,
		Path:        "/private-messages/all",
		Summary:     "List all private conversations",
		Description: "Lists every private conversation in the game, for GMs and the audience. Also open to any authenticated user once the game is a public archive.",
		Tags:        []string{"Audience"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "participant_ids contained a non-integer"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "The caller cannot view this game's content"},
		},
	}, h.humaListAllPrivateConversations)

	huma.Register(api, huma.Operation{
		OperationID: "listConversationParticipants",
		Method:      http.MethodGet,
		Path:        "/private-messages/participants",
		Summary:     "List conversation participants",
		Description: "Characters appearing in the game's private conversations, for the filter UI. With selected[] character IDs, returns only characters sharing a conversation with all of them.",
		Tags:        []string{"Audience"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "selected[] contained a non-integer"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "The caller cannot view this game's content"},
		},
	}, h.humaGetConversationParticipants)

	huma.Register(api, huma.Operation{
		OperationID: "getAudienceConversationMessages",
		Method:      http.MethodGet,
		Path:        "/private-messages/conversations/{conversationId}",
		Summary:     "Read a private conversation",
		Description: "Returns one private conversation's messages, for GMs and the audience.",
		Tags:        []string{"Audience"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "The caller cannot view this game's content"},
		},
	}, h.humaGetAudienceConversationMessages)

	huma.Register(api, huma.Operation{
		OperationID: "listAllActionSubmissions",
		Method:      http.MethodGet,
		Path:        "/action-submissions/all",
		Summary:     "List all action submissions",
		Description: "Lists every action submission in the game, for GMs and the audience.",
		Tags:        []string{"Audience"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "The caller cannot view this game's content"},
		},
	}, h.humaListAllActionSubmissions)

	// Logs and stats

	huma.Register(api, huma.Operation{
		OperationID: "getGameLogs",
		Method:      http.MethodGet,
		Path:        "/logs",
		Summary:     "Get the game log",
		Description: "Returns the game's event log. GM only while the game is running; readable by participants once it is completed or cancelled.",
		Tags:        []string{"Game Logs"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can retrieve game logs while the game is running"},
		},
	}, h.humaGetGameLogs)

	huma.Register(api, huma.Operation{
		OperationID: "getGameStats",
		Method:      http.MethodGet,
		Path:        "/stats",
		Summary:     "Get post-game statistics",
		Description: "Returns aggregate statistics for a completed game. Withheld until completion even from the GM, because the numbers aggregate over private messages.",
		Tags:        []string{"Games"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"403": {Description: "Not authenticated, or the caller cannot view this game"},
			"409": {Description: "The game is not completed"},
		},
	}, h.humaGetGameStats)

	// Loot tables

	huma.Register(api, huma.Operation{
		OperationID: "listLootTables",
		Method:      http.MethodGet,
		Path:        "/loot-tables",
		Summary:     "List loot tables",
		Description: "Lists the game's loot tables. GM only.",
		Tags:        []string{"Loot Tables"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can see and edit loot tables"},
		},
	}, h.humaGetGameLootTables)

	huma.Register(api, huma.Operation{
		OperationID: "createLootTable",
		Method:      http.MethodPost,
		Path:        "/loot-tables",
		Summary:     "Create a loot table",
		Description: "Creates a named loot table, optionally with its initial items. GM only. Answers 200, not 201.",
		Tags:        []string{"Loot Tables"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "Missing name, or an item with a blank name or invalid JSON data"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can see and edit loot tables"},
		},
	}, h.humaAddGameLootTable)

	huma.Register(api, huma.Operation{
		OperationID: "updateLootTable",
		Method:      http.MethodPut,
		Path:        "/loot-tables/{tableId}",
		Summary:     "Rename a loot table",
		Description: "Changes a loot table's name. Items in the body are validated but not applied; use the contents endpoint to change them. GM only.",
		Tags:        []string{"Loot Tables"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "Missing name, or an item with a blank name or invalid JSON data"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not the GM, or the table belongs to another game"},
		},
	}, h.humaUpdateGameLootTable)

	huma.Register(api, huma.Operation{
		OperationID: "deleteLootTable",
		Method:      http.MethodDelete,
		Path:        "/loot-tables/{tableId}",
		Summary:     "Delete a loot table",
		Description: "Deletes a loot table and its items. GM only. Answers 200 with an empty body.",
		Tags:        []string{"Loot Tables"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not the GM, or the table belongs to another game"},
		},
	}, h.humaDeleteGameLootTable)

	huma.Register(api, huma.Operation{
		OperationID: "listLootTableContents",
		Method:      http.MethodGet,
		Path:        "/loot-tables/{tableId}/contents",
		Summary:     "List loot table contents",
		Description: "Lists the items in a loot table. GM only.",
		Tags:        []string{"Loot Tables"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not the GM, or the table belongs to another game"},
		},
	}, h.humaGetGameLootTableContents)

	huma.Register(api, huma.Operation{
		OperationID: "replaceLootTableContents",
		Method:      http.MethodPost,
		Path:        "/loot-tables/{tableId}/contents",
		Summary:     "Replace loot table contents",
		Description: "Replaces every item in the table in one transaction. GM only. Answers 200 with an empty body.",
		Tags:        []string{"Loot Tables"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "An item with a blank name or invalid JSON data"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not the GM, or the table belongs to another game"},
		},
	}, h.humaUpdateGameLootTableContent)

	huma.Register(api, huma.Operation{
		OperationID: "rollLootForCharacter",
		Method:      http.MethodPost,
		Path:        "/loot-tables/{tableId}/random/{characterId}",
		Summary:     "Roll random loot",
		Description: "Picks a random item from the table, adds it to the character's inventory and records the roll in the game log. GM only.",
		Tags:        []string{"Loot Tables"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "The loot table is empty"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not the GM, the table belongs to another game, or the character cannot be edited"},
		},
	}, h.humaSetRandomLootForCharacter)
}
