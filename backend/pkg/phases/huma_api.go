package phases

// Huma (type-first) implementation of the phase, action, action-result and
// draft-character-update APIs.
//
// Two registration functions, because phases are mounted at two prefixes:
// the per-game routes under /games/{gameID}, and the phase-id operations at
// /phases. See .claude/planning/huma-migration.md gotcha 10.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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

// gameIDFromCtx reads the id GameMiddleware stashed. The middleware runs before
// every operation registered on the game-scoped API, so the value is always
// present; the comma-ok guard exists so a misconfigured router fails as a 500
// rather than a panic.
func gameIDFromCtx(ctx context.Context) (int32, error) {
	gameID, ok := ctx.Value("gameID").(int32)
	if !ok {
		return 0, huma.Error500InternalServerError("game context missing")
	}
	return gameID, nil
}

// isGMFromCtx reads the GM flag GameMiddleware computed, which already accounts
// for co-GMs and admin mode.
func isGMFromCtx(ctx context.Context) bool {
	isGM, _ := ctx.Value("is_gm").(bool)
	return isGM
}

// requireAuth returns the authenticated user or the 401 the chi handlers sent.
func (h *Handler) requireAuth(ctx context.Context) (*core.AuthenticatedUser, error) {
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		return nil, h.logAndErr(ctx, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
	}
	return authUser, nil
}

// requirePhaseManager checks the CanUserManagePhases permission, which is the
// GM/co-GM test the result and action endpoints use.
func (h *Handler) requirePhaseManager(ctx context.Context, gameID, userID int32, forbiddenMsg, logMsg string) error {
	canManage, err := h.PhaseService.CanUserManagePhases(ctx, gameID, userID)
	if err != nil {
		return h.logAndErr(ctx, core.ErrInternalError(err), "Failed to check phase management permission", "error", err)
	}
	if !canManage {
		return h.logAndErr(ctx, core.ErrForbidden(forbiddenMsg), logMsg)
	}
	return nil
}

// loadPhaseGame resolves a phase and confirms the caller is the game's GM.
//
// Shared by the four /phases/{id} operations, which all begin with the same
// phase -> game -> GM check.
func (h *Handler) loadPhaseGame(ctx context.Context, phaseID int32, authUser *core.AuthenticatedUser, forbiddenMsg, logMsg string) error {
	phase, err := h.PhaseService.GetPhase(ctx, phaseID)
	if err != nil {
		return h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get phase", "error", err)
	}

	game, err := h.GameService.GetGame(ctx, phase.GameID)
	if err != nil {
		return h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get game", "error", err)
	}

	// The Ctx form of the same check the chi handlers made through
	// core.IsUserGameMaster, which is itself only a *http.Request wrapper
	// around this. Admin mode is read from the context either way.
	if !core.IsUserGameMasterCtx(ctx, authUser.ID, authUser.IsAdmin, *game, h.App.Pool) {
		return h.logAndErr(ctx, core.ErrForbidden(forbiddenMsg), logMsg,
			"phase_id", phaseID, "game_id", phase.GameID, "user_id", authUser.ID)
	}
	return nil
}

// withCalculatedFields fills the two fields PhaseResponse.Render used to
// compute at serialization time.
//
// Under chi these were set by the Render method, which huma never calls: it
// marshals the output struct directly. Computing them here keeps time_remaining
// and is_expired in the response, which the countdown UI reads.
func withCalculatedFields(p *PhaseResponse) *PhaseResponse {
	if p == nil {
		return nil
	}
	if p.Deadline != nil {
		remaining := time.Until(*p.Deadline)
		if remaining > 0 {
			seconds := int64(remaining.Seconds())
			p.TimeRemaining = &seconds
			p.IsExpired = false
		} else {
			p.IsExpired = true
		}
	}
	return p
}

// Input / output types

type gameScopedInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
}

type phaseIDInput struct {
	ID int32 `path:"id" doc:"Phase ID"`
}

type resultIDInput struct {
	GameID   int32 `path:"gameID" doc:"Game ID"`
	ResultID int32 `path:"resultId" doc:"Action result ID"`
}

type phaseScopedInput struct {
	GameID  int32 `path:"gameID" doc:"Game ID"`
	PhaseID int32 `path:"phaseId" doc:"Phase ID"`
}

type draftIDInput struct {
	GameID   int32 `path:"gameID" doc:"Game ID"`
	ResultID int32 `path:"resultId" doc:"Action result ID"`
	DraftID  int32 `path:"draftId" doc:"Draft update ID"`
}

type phaseOutput struct {
	Body *PhaseResponse
}

type phaseListOutput struct {
	Body []*PhaseResponse
}

// currentPhaseOutput reproduces the {"phase": ...} envelope the chi handler
// wrote by hand. The value is null when no phase is active, which is how the
// client distinguishes "no active phase" from an error -- this endpoint has
// never returned 404 for that case (gotcha 12).
type currentPhaseOutput struct {
	Body struct {
		Phase *PhaseResponse `json:"phase"`
	}
}

type actionOutput struct {
	Body *ActionResponse
}

// actionListOutput's body is a nil-able slice on purpose: the chi handlers
// appended to a `var response []T`, so an empty result set serialized as null
// rather than []. Preserved rather than corrected, since changing it is a
// frontend-visible change that belongs in its own commit.
type actionListOutput struct {
	Body []ActionWithDetailsResponse
}

type actionResultOutput struct {
	Body *ActionResultResponse
}

type actionResultListOutput struct {
	Body []ActionResultWithDetailsResponse
}

type stagedPartOutput struct {
	Body ActionResultWithDetailsResponse
}

type stagedChainOutput struct {
	Body []ActionResultWithDetailsResponse
}

type draftUpdateOutput struct {
	Body *DraftCharacterUpdateResponse
}

type draftUpdateListOutput struct {
	Body []DraftCharacterUpdateResponse
}

type draftCountOutput struct {
	Body struct {
		Count int64 `json:"count"`
	}
}

// messageOutput carries the {"message": ...} envelope two endpoints answer with.
// Both are 200-with-a-body rather than 204, which is what the chi handlers sent.
type messageOutput struct {
	Body struct {
		Message string `json:"message"`
	}
}

// noContentOutput is empty, so huma infers 204 -- which is what these endpoints
// already sent (gotcha 22 in reverse: here the inference is correct).
type noContentOutput struct{}

// Request bodies
//
// These mirror the structs in requests.go, with huma tags replacing the
// `validate:` ones. They are separate types rather than reused ones because the
// chi versions carry Bind methods and `validate:` tags that huma ignores, and a
// silently-ignored tag enforces nothing.

// Datetime fields carry no format:"date-time" tag on purpose.
//
// core.LocalDateTime accepts RFC3339 *and* the looser shapes an HTML
// datetime-local input produces ("2026-09-30T12:00"), which is the whole reason
// the type exists. Huma's format validator runs on the raw JSON string before
// UnmarshalJSON, so tagging these would 400 every non-RFC3339 form the API has
// always accepted -- a narrowing no test covers, since the tests send RFC3339.
// Found by curling the running server.
type createPhaseBody struct {
	PhaseType   string              `json:"phase_type" enum:"common_room,action,interlude" doc:"Phase type"`
	Title       string              `json:"title,omitempty" required:"false"`
	Description string              `json:"description,omitempty" required:"false"`
	StartTime   *core.LocalDateTime `json:"start_time,omitempty" required:"false"`
	EndTime     *core.LocalDateTime `json:"end_time,omitempty" required:"false"`
	Deadline    *core.LocalDateTime `json:"deadline,omitempty" required:"false"`
}

type updateDeadlineBody struct {
	Deadline core.LocalDateTime `json:"deadline" doc:"New deadline"`
}

type updatePhaseBody struct {
	Title       *string             `json:"title,omitempty" required:"false"`
	Description *string             `json:"description,omitempty" required:"false"`
	StartTime   *core.LocalDateTime `json:"start_time,omitempty" required:"false"`
	Deadline    *core.LocalDateTime `json:"deadline,omitempty" required:"false"`
	// EndTime is intentionally absent -- it is system-managed and set by
	// DeactivatePhase, matching the chi request struct.
}

type submitActionBody struct {
	CharacterID *int32 `json:"character_id,omitempty" required:"false"`
	Content     string `json:"content" minLength:"1" doc:"Action content"`
}

func (b *submitActionBody) Resolve(huma.Context) []error { return humaconfig.TrimStrings(b) }

type createActionResultBody struct {
	UserID             int32  `json:"user_id" doc:"Recipient user ID"`
	CharacterID        *int32 `json:"character_id,omitempty" required:"false"`
	ActionSubmissionID *int32 `json:"action_submission_id,omitempty" required:"false"`
	Content            string `json:"content" minLength:"1" doc:"Result content"`
	IsPublished        bool   `json:"is_published,omitempty" required:"false"`
}

func (b *createActionResultBody) Resolve(huma.Context) []error { return humaconfig.TrimStrings(b) }

// updateActionResultBody has no minLength on Content.
//
// The chi handler decoded this with json.Decode and a `validate:"required"` tag
// that nothing ever ran, so an empty content string has always been accepted
// and stored. Adding minLength here would be a silent contract narrowing on an
// endpoint whose tests do not cover it; leave the behaviour as shipped and
// track tightening it separately.
type updateActionResultBody struct {
	Content string `json:"content" required:"false"`
}

type stagedPartBody struct {
	Content string `json:"content" minLength:"1" doc:"Part content"`
	// DelayMinutes carries no minimum: the first part of a chain must be 0 and
	// the service owns the range check, answering ErrInvalidStagedChain (400)
	// with a message naming the limit.
	DelayMinutes int32 `json:"delay_minutes" required:"false" doc:"Minutes to wait after the previous part releases"`
}

func (b *stagedPartBody) Resolve(huma.Context) []error { return humaconfig.TrimStrings(b) }

type createStagedChainBody struct {
	UserID             int32            `json:"user_id" doc:"Recipient user ID"`
	CharacterID        *int32           `json:"character_id,omitempty" required:"false"`
	ActionSubmissionID *int32           `json:"action_submission_id,omitempty" required:"false"`
	Parts              []stagedPartBody `json:"parts" doc:"Chain parts, in reveal order"`
	IsPublished        bool             `json:"is_published,omitempty" required:"false"`
}

type appendStagedPartBody struct {
	Content      string `json:"content" minLength:"1" doc:"Part content"`
	DelayMinutes int32  `json:"delay_minutes" doc:"Minutes to wait after the previous part releases"`
}

func (b *appendStagedPartBody) Resolve(huma.Context) []error { return humaconfig.TrimStrings(b) }

type updateStagedDelayBody struct {
	DelayMinutes int32 `json:"delay_minutes" doc:"New delay in minutes"`
}

// createDraftUpdateBody's three enums are closed sets the service already
// rejected outside of -- but it did so with a plain fmt.Errorf, which the
// handler mapped to 500. A bad module_type is the caller's mistake, so huma
// answering 400 before the handler runs is both the right status and the first
// time the accepted values appear in the spec.
//
// The values match the check constraints on action_result_character_updates,
// not just the service's maps; the two were verified to agree.
type createDraftUpdateBody struct {
	CharacterID int32  `json:"character_id" doc:"Character to update"`
	ModuleType  string `json:"module_type" enum:"skills,inventory,numbers"`
	FieldName   string `json:"field_name" minLength:"1"`
	FieldValue  string `json:"field_value" minLength:"1"`
	FieldType   string `json:"field_type" enum:"text,number,boolean,json"`
	Operation   string `json:"operation" enum:"upsert,delete"`
}

func (b *createDraftUpdateBody) Resolve(huma.Context) []error { return humaconfig.TrimStrings(b) }

// updateDraftUpdateBody has no minLength on FieldValue, for the same reason as
// updateActionResultBody: the chi handler's `validate:` tag never ran.
type updateDraftUpdateBody struct {
	FieldValue string `json:"field_value" required:"false"`
}

type createPhaseInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
	Body   *createPhaseBody
}

type updateDeadlineInput struct {
	ID   int32 `path:"id" doc:"Phase ID"`
	Body *updateDeadlineBody
}

type updatePhaseInput struct {
	ID   int32 `path:"id" doc:"Phase ID"`
	Body *updatePhaseBody
}

type submitActionInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
	Body   *submitActionBody
}

type createActionResultInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
	Body   *createActionResultBody
}

type createStagedChainInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
	Body   *createStagedChainBody
}

type updateActionResultInput struct {
	GameID   int32 `path:"gameID" doc:"Game ID"`
	ResultID int32 `path:"resultId" doc:"Action result ID"`
	Body     *updateActionResultBody
}

type appendStagedPartInput struct {
	GameID   int32 `path:"gameID" doc:"Game ID"`
	ResultID int32 `path:"resultId" doc:"Action result ID (any member of the chain)"`
	Body     *appendStagedPartBody
}

type updateStagedDelayInput struct {
	GameID   int32 `path:"gameID" doc:"Game ID"`
	ResultID int32 `path:"resultId" doc:"Staged part ID"`
	Body     *updateStagedDelayBody
}

type createDraftUpdateInput struct {
	GameID   int32 `path:"gameID" doc:"Game ID"`
	ResultID int32 `path:"resultId" doc:"Action result ID"`
	Body     *createDraftUpdateBody
}

type updateDraftUpdateInput struct {
	GameID   int32 `path:"gameID" doc:"Game ID"`
	ResultID int32 `path:"resultId" doc:"Action result ID"`
	DraftID  int32 `path:"draftId" doc:"Draft update ID"`
	Body     *updateDraftUpdateBody
}

// Phase CRUD

func (h *Handler) humaCreatePhase(ctx context.Context, in *createPhaseInput) (*phaseOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_create_phase")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	// The enum tag rejects anything outside ValidPhaseTypes before the handler
	// runs, so the chi loop over ValidPhaseTypes has no equivalent here. Note
	// the chi error message named only two of the three valid types; the enum
	// is generated from the same list the database constrains on.
	if !isGMFromCtx(ctx) {
		return nil, h.logAndErr(ctx, core.ErrForbidden("only the GM can create phases"),
			"Phase create permission denied", "game_id", gameID, "user_id", core.GetAuthenticatedUser(ctx).ID)
	}

	phase, err := h.PhaseService.CreatePhase(ctx, core.CreatePhaseRequest{
		GameID:      gameID,
		PhaseType:   in.Body.PhaseType,
		Title:       in.Body.Title,
		Description: in.Body.Description,
		StartTime:   in.Body.StartTime.ToTimePtr(),
		EndTime:     in.Body.EndTime.ToTimePtr(),
		Deadline:    in.Body.Deadline.ToTimePtr(),
	})
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to create phase", "error", err)
		if core.IsArchivedGameError(err) {
			return nil, h.logAndErr(ctx, core.ErrGameArchived(), "Error in create phase")
		}
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to create phase", "error", err)
	}

	return &phaseOutput{Body: withCalculatedFields(convertPhaseToResponse(phase))}, nil
}

func (h *Handler) humaGetCurrentPhase(ctx context.Context, in *gameScopedInput) (*currentPhaseOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_current_phase")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	phase, err := h.PhaseService.GetActivePhase(ctx, gameID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get active phase", "error", err, "game_id", gameID)
	}

	out := &currentPhaseOutput{}
	if phase != nil {
		out.Body.Phase = withCalculatedFields(convertPhaseToResponse(phase))
	}
	return out, nil
}

func (h *Handler) humaGetGamePhases(ctx context.Context, in *gameScopedInput) (*phaseListOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_game_phases")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	phases, err := h.PhaseService.GetGamePhases(ctx, gameID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get game phases", "error", err, "game_id", gameID)
	}

	// make(..., 0, n) rather than a nil slice: the chi handler built this one
	// the same way, so an empty list has always serialized as [].
	response := make([]*PhaseResponse, 0, len(phases))
	for i := range phases {
		response = append(response, withCalculatedFields(convertPhaseToResponse(&phases[i])))
	}
	return &phaseListOutput{Body: response}, nil
}

func (h *Handler) humaUpdatePhaseDeadline(ctx context.Context, in *updateDeadlineInput) (*phaseOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_phase_deadline")()

	authUser, err := h.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.loadPhaseGame(ctx, in.ID, authUser,
		"only the GM can update phase deadlines", "Phase deadline update permission denied"); err != nil {
		return nil, err
	}

	updatedPhase, err := h.PhaseService.ExtendPhaseDeadline(ctx, in.ID, in.Body.Deadline.Time)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to update phase deadline", "error", err)
	}

	return &phaseOutput{Body: withCalculatedFields(convertPhaseToResponse(updatedPhase))}, nil
}

func (h *Handler) humaUpdatePhase(ctx context.Context, in *updatePhaseInput) (*phaseOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_phase")()

	authUser, err := h.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.loadPhaseGame(ctx, in.ID, authUser,
		"only the GM can update phases", "Phase update permission denied"); err != nil {
		return nil, err
	}

	req := core.UpdatePhaseRequest{
		ID:        in.ID,
		StartTime: in.Body.StartTime.ToTimePtr(),
		Deadline:  in.Body.Deadline.ToTimePtr(),
	}
	if in.Body.Title != nil {
		req.Title = *in.Body.Title
	}
	if in.Body.Description != nil {
		req.Description = *in.Body.Description
	}

	updatedPhase, err := h.PhaseService.UpdatePhase(ctx, req)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to update phase", "error", err)
	}

	return &phaseOutput{Body: withCalculatedFields(convertPhaseToResponse(updatedPhase))}, nil
}

func (h *Handler) humaDeletePhase(ctx context.Context, in *phaseIDInput) (*noContentOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_phase")()

	authUser, err := h.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.loadPhaseGame(ctx, in.ID, authUser,
		"only the GM can delete phases", "Phase delete permission denied"); err != nil {
		return nil, err
	}

	// Validation (no associated content) happens in the service layer, and its
	// failures are the caller's to fix, hence 400 rather than 500.
	if err := h.PhaseService.DeletePhase(ctx, in.ID); err != nil {
		return nil, h.logAndErr(ctx, core.ErrBadRequest(err), "Failed to delete phase", "error", err)
	}

	return &noContentOutput{}, nil
}

// Phase lifecycle

func (h *Handler) humaActivatePhase(ctx context.Context, in *phaseIDInput) (*phaseOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_activate_phase")()

	authUser, err := h.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.loadPhaseGame(ctx, in.ID, authUser,
		"only the GM can activate phases", "Activate phase forbidden"); err != nil {
		return nil, err
	}

	if err := h.PhaseService.ActivatePhase(ctx, in.ID, authUser.ID); err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to activate phase", "error", err)
	}

	// Re-read: ActivatePhase returns no row, and the response must carry the
	// activated_at the activation just set.
	activePhase, err := h.PhaseService.GetPhase(ctx, in.ID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get activated phase", "error", err)
	}

	return &phaseOutput{Body: withCalculatedFields(convertPhaseToResponse(activePhase))}, nil
}

func (h *Handler) humaPublishAllPhaseResults(ctx context.Context, in *phaseScopedInput) (*messageOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_publish_all_phase_results")()

	if _, err := h.requireAuth(ctx); err != nil {
		return nil, err
	}

	if !isGMFromCtx(ctx) {
		return nil, h.logAndErr(ctx, core.ErrForbidden("only the GM can publish action results"), "Publish all phase results forbidden")
	}

	if err := h.ActionSubmissionService.PublishAllPhaseResults(ctx, in.PhaseID); err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to publish all phase results", "error", err)
	}

	out := &messageOutput{}
	out.Body.Message = "All results published successfully"
	return out, nil
}

func (h *Handler) humaGetUnpublishedResultsCount(ctx context.Context, in *phaseScopedInput) (*draftCountOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_unpublished_results_count")()

	if _, err := h.requireAuth(ctx); err != nil {
		return nil, err
	}

	if !isGMFromCtx(ctx) {
		return nil, h.logAndErr(ctx, core.ErrForbidden("only the GM can view result counts"), "Get unpublished results count forbidden")
	}

	count, err := h.ActionSubmissionService.GetUnpublishedResultsCount(ctx, in.PhaseID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get unpublished results count", "error", err)
	}

	out := &draftCountOutput{}
	out.Body.Count = count
	return out, nil
}

// Action submissions

func (h *Handler) humaSubmitAction(ctx context.Context, in *submitActionInput) (*actionOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_submit_action")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	authUser, err := h.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	canSubmit, err := h.PhaseService.CanUserSubmitActions(ctx, gameID, authUser.ID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to check action submission permission", "error", err)
	}
	if !canSubmit {
		return nil, h.logAndErr(ctx, core.ErrForbidden("you cannot submit actions for this game"),
			"Action submission permission denied", "game_id", gameID, "user_id", authUser.ID)
	}

	activePhase, err := h.PhaseService.GetActivePhase(ctx, gameID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get active phase", "error", err)
	}
	if activePhase == nil {
		return nil, h.logAndErr(ctx, core.ErrBadRequest(fmt.Errorf("no active phase for this game")), "Bad submit action request")
	}
	if activePhase.PhaseType != core.PhaseTypeAction {
		return nil, h.logAndErr(ctx, core.ErrForbidden("actions can only be submitted during an action phase"),
			"Action submission rejected: not an action phase", "game_id", gameID, "phase_type", activePhase.PhaseType)
	}

	action, err := h.ActionSubmissionService.SubmitAction(ctx, core.SubmitActionRequest{
		GameID:      gameID,
		UserID:      authUser.ID,
		PhaseID:     activePhase.ID,
		CharacterID: in.Body.CharacterID,
		Content:     in.Body.Content,
	})
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to submit action", "error", err)
		if core.IsArchivedGameError(err) {
			return nil, h.logAndErr(ctx, core.ErrGameArchived(), "Error in submit action")
		}
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to submit action", "error", err)
	}

	// Notify GM and co-GMs on first-time submission only.
	// submitted_at == updated_at only on insert; edits leave submitted_at unchanged.
	isFirstSubmission := action.SubmittedAt.Valid && action.UpdatedAt.Valid &&
		action.SubmittedAt.Time.Equal(action.UpdatedAt.Time)
	if isFirstSubmission {
		characterName := "Unknown Character"
		if action.CharacterID.Valid {
			var charName string
			if charErr := h.App.Pool.QueryRow(ctx, `SELECT name FROM characters WHERE id = $1`, action.CharacterID.Int32).Scan(&charName); charErr == nil {
				characterName = charName
			}
		}
		notifSvc := h.NotificationService
		userID := authUser.ID
		observability.SafeGo(context.Background(), h.App.ObsLogger, "notify-action-submitted", func() {
			notifCtx := context.Background()
			if err := notifSvc.NotifyActionSubmitted(notifCtx, action.ID, action.GameID, userID, characterName); err != nil {
				h.App.ObsLogger.LogError(notifCtx, err, "Failed to notify GM of action submission", "action_id", action.ID)
			}
		})
	}

	var characterID *int32
	if action.CharacterID.Valid {
		characterID = &action.CharacterID.Int32
	}

	return &actionOutput{Body: &ActionResponse{
		ID:          action.ID,
		GameID:      action.GameID,
		UserID:      action.UserID,
		PhaseID:     action.PhaseID,
		CharacterID: characterID,
		Content:     action.Content,
		SubmittedAt: action.SubmittedAt.Time,
		UpdatedAt:   action.UpdatedAt.Time,
	}}, nil
}

func (h *Handler) humaGetUserActions(ctx context.Context, in *gameScopedInput) (*actionListOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_user_actions")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	authUser, err := h.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	actions, err := h.ActionSubmissionService.GetUserActions(ctx, gameID, authUser.ID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get user actions", "error", err)
	}

	// Nil slice, not make(...): an empty list serializes as null here, matching
	// the chi handler. See actionListOutput.
	var response []ActionWithDetailsResponse
	for _, action := range actions {
		actionResp := ActionWithDetailsResponse{
			ID:          action.ID,
			GameID:      action.GameID,
			UserID:      action.UserID,
			PhaseID:     action.PhaseID,
			Content:     action.Content,
			SubmittedAt: action.SubmittedAt.Time,
			UpdatedAt:   action.UpdatedAt.Time,
			PhaseType:   &action.PhaseType,
			PhaseNumber: &action.PhaseNumber,
		}
		if action.CharacterID.Valid {
			actionResp.CharacterID = &action.CharacterID.Int32
		}
		if action.CharacterName.Valid {
			actionResp.CharacterName = &action.CharacterName.String
		}
		response = append(response, actionResp)
	}

	return &actionListOutput{Body: response}, nil
}

func (h *Handler) humaGetGameActions(ctx context.Context, in *gameScopedInput) (*actionListOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_game_actions")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	authUser, err := h.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.requirePhaseManager(ctx, gameID, authUser.ID,
		"only the GM can view all actions", "Get game actions forbidden"); err != nil {
		return nil, err
	}

	actions, err := h.ActionSubmissionService.GetGameActions(ctx, gameID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get game actions", "error", err)
	}

	var response []ActionWithDetailsResponse
	for _, action := range actions {
		actionResp := ActionWithDetailsResponse{
			ID:          action.ID,
			GameID:      action.GameID,
			UserID:      action.UserID,
			PhaseID:     action.PhaseID,
			Content:     action.Content,
			SubmittedAt: action.SubmittedAt.Time,
			UpdatedAt:   action.UpdatedAt.Time,
			Username:    action.Username,
			PhaseType:   &action.PhaseType,
			PhaseNumber: &action.PhaseNumber,
		}
		if action.CharacterID.Valid {
			actionResp.CharacterID = &action.CharacterID.Int32
		}
		if action.CharacterName.Valid {
			actionResp.CharacterName = &action.CharacterName.String
		}
		response = append(response, actionResp)
	}

	return &actionListOutput{Body: response}, nil
}

// Action results

func (h *Handler) humaCreateActionResult(ctx context.Context, in *createActionResultInput) (*actionResultOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_create_action_result")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	gmUser, err := h.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.requirePhaseManager(ctx, gameID, gmUser.ID,
		"only the GM can create action results", "Create action result forbidden"); err != nil {
		return nil, err
	}

	activePhase, err := h.PhaseService.GetActivePhase(ctx, gameID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get active phase", "error", err)
	}
	if activePhase == nil {
		return nil, h.logAndErr(ctx, core.ErrBadRequest(fmt.Errorf("no active phase for this game")), "Bad create action result request")
	}

	result, err := h.ActionSubmissionService.CreateActionResult(ctx, core.CreateActionResultRequest{
		GameID:             gameID,
		UserID:             in.Body.UserID,
		CharacterID:        in.Body.CharacterID,
		ActionSubmissionID: in.Body.ActionSubmissionID,
		PhaseID:            activePhase.ID,
		GMUserID:           gmUser.ID,
		Content:            in.Body.Content,
		IsPublished:        in.Body.IsPublished,
	})
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to create action result", "error", err)
	}

	return &actionResultOutput{Body: actionResultResponse(result)}, nil
}

// actionResultResponse shapes the summary form the create and update endpoints
// answer with -- deliberately narrower than ActionResultWithDetailsResponse,
// which the read and staged-edit paths use.
func actionResultResponse(result *models.ActionResult) *ActionResultResponse {
	resp := &ActionResultResponse{
		ID:          result.ID,
		GameID:      result.GameID,
		UserID:      result.UserID,
		PhaseID:     result.PhaseID,
		GMUserID:    result.GmUserID,
		Content:     result.Content,
		IsPublished: result.IsPublished.Bool,
	}
	if result.SentAt.Valid {
		resp.SentAt = &result.SentAt.Time
	}
	return resp
}

func (h *Handler) humaGetUserActionResults(ctx context.Context, in *gameScopedInput) (*actionResultListOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_user_action_results")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	authUser, err := h.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	results, err := h.ActionSubmissionService.GetUserResults(ctx, gameID, authUser.ID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get user action results", "error", err)
	}

	var response []ActionResultWithDetailsResponse
	for _, result := range results {
		resultResp := ActionResultWithDetailsResponse{
			ID:          result.ID,
			GameID:      result.GameID,
			UserID:      result.UserID,
			PhaseID:     result.PhaseID,
			GMUserID:    result.GmUserID,
			Content:     result.Content,
			IsPublished: result.IsPublished.Bool,
			GMUsername:  result.GmUsername,
			PhaseType:   result.PhaseType,
			PhaseNumber: result.PhaseNumber,
		}

		if result.SentAt.Valid {
			resultResp.SentAt = &result.SentAt.Time
		}

		// The client resolves a character's avatar by looking this id up in the
		// game's character list, so omitting it leaves every result showing an
		// initials fallback instead of the portrait.
		if result.CharacterID.Valid {
			charID := result.CharacterID.Int32
			resultResp.CharacterID = &charID
		}

		if result.CharacterName.Valid {
			resultResp.CharacterName = result.CharacterName.String
		}

		// Staged reveal fields. A pending part is returned to its recipient with
		// its content already blanked in SQL (see GetUserResults) so the client
		// can render a placeholder counting down to the reveal. ReleasedAt being
		// nil is how the client identifies a locked part -- it must not infer
		// lockedness from the content being empty.
		applyStagedFields(&resultResp, result.PartNumber, result.PartCount, result.ReleasedAt, result.UnlocksAt, result.RevealDelayMinutes)

		response = append(response, resultResp)
	}

	return &actionResultListOutput{Body: response}, nil
}

func (h *Handler) humaGetGameActionResults(ctx context.Context, in *gameScopedInput) (*actionResultListOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_game_action_results")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	authUser, err := h.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	canManage, err := h.PhaseService.CanUserManagePhases(ctx, gameID, authUser.ID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to check phase management permission", "error", err)
	}

	game, err := h.GameService.GetGame(ctx, gameID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get game", "error", err)
	}

	isAudience := core.IsUserAudience(ctx, h.App.Pool, gameID, authUser.ID)

	// Allow access if: GM, audience, or the game is a public archive.
	//
	// The archive branch deliberately has no membership check -- a public-archive
	// game (completed OR epilogue) is readable by any authenticated user. It is
	// broader than the other two arms, which are scoped to a role: this one
	// admits non-participants.
	//
	// Epilogue must be included: a player writing an epilogue needs to see what
	// happened to everyone else, which is the whole reason that state exists.
	if !canManage && !isAudience && !core.IsPublicArchive(game.State.String) {
		return nil, h.logAndErr(ctx, core.ErrForbidden("only the GM, audience, or any user of a public archive game can view all action results"),
			"Get game action results forbidden")
	}

	results, err := h.ActionSubmissionService.GetGameResults(ctx, gameID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get game action results", "error", err)
	}

	// No is_published filter: every caller admitted above sees drafts as well as
	// published results. That is deliberate for each of the three, but for
	// different reasons, so changing one arm does not license changing another:
	//
	//   - GM: authors the drafts.
	//   - Audience: a trusted spectator role that already sees every private
	//     message and submission, so it sees unpublished and unreleased content
	//     here too. This is an explicit decision about who the role is for, not
	//     an inference about what drafts happen to contain.
	//   - Completed game: the archive is public, and this arm admits any
	//     authenticated user, not just participants. Unpublished drafts the GM
	//     never sent are therefore readable by anyone once a game completes.
	//
	// Contrast humaGetUserActionResults, which serves players and filters to
	// is_published = true in SQL (see GetUserResults).
	var response []ActionResultWithDetailsResponse
	for _, result := range results {
		resultResp := ActionResultWithDetailsResponse{
			ID:          result.ID,
			GameID:      result.GameID,
			UserID:      result.UserID,
			PhaseID:     result.PhaseID,
			GMUserID:    result.GmUserID,
			Content:     result.Content,
			IsPublished: result.IsPublished.Bool,
			Username:    result.Username,
			PhaseType:   result.PhaseType,
			PhaseNumber: result.PhaseNumber,
		}

		if result.CharacterID.Valid {
			charID := result.CharacterID.Int32
			resultResp.CharacterID = &charID
		}

		if result.ActionSubmissionID.Valid {
			submissionID := result.ActionSubmissionID.Int32
			resultResp.ActionSubmissionID = &submissionID
		}

		if result.CharacterName.Valid {
			resultResp.CharacterName = result.CharacterName.String
		}

		if result.SentAt.Valid {
			resultResp.SentAt = &result.SentAt.Time
		}

		// Staged reveal fields. Identical to the player path -- the GM and
		// audience see the same part numbering and schedule. What differs is
		// upstream in SQL: this query never blanks content, because both roles
		// are entitled to read a part before it releases.
		applyStagedFields(&resultResp, result.PartNumber, result.PartCount, result.ReleasedAt, result.UnlocksAt, result.RevealDelayMinutes)

		response = append(response, resultResp)
	}

	return &actionResultListOutput{Body: response}, nil
}

func (h *Handler) humaUpdateActionResult(ctx context.Context, in *updateActionResultInput) (*actionResultOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_action_result")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	authUser, err := h.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.requirePhaseManager(ctx, gameID, authUser.ID,
		"only the GM can update action results", "Update action result forbidden"); err != nil {
		return nil, err
	}

	result, err := h.ActionSubmissionService.UpdateActionResult(ctx, in.ResultID, in.Body.Content)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to update action result", "error", err)
	}

	return &actionResultOutput{Body: actionResultResponse(result)}, nil
}

func (h *Handler) humaDeleteActionResult(ctx context.Context, in *resultIDInput) (*noContentOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_action_result")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	authUser, err := h.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.requirePhaseManager(ctx, gameID, authUser.ID,
		"only the GM can delete action results", "Delete action result forbidden"); err != nil {
		return nil, err
	}

	if err := h.ActionSubmissionService.DeleteActionResult(ctx, in.ResultID); err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to delete action result", "error", err)
	}

	return &noContentOutput{}, nil
}

func (h *Handler) humaPublishActionResult(ctx context.Context, in *resultIDInput) (*messageOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_publish_action_result")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	authUser, err := h.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.requirePhaseManager(ctx, gameID, authUser.ID,
		"only the GM can publish action results", "Publish action result forbidden"); err != nil {
		return nil, err
	}

	if err := h.ActionSubmissionService.PublishActionResult(ctx, in.ResultID, authUser.ID); err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to publish action result", "error", err)
	}

	out := &messageOutput{}
	out.Body.Message = "Action result published successfully"
	return out, nil
}

// Staged result chains

func (h *Handler) humaCreateStagedResultChain(ctx context.Context, in *createStagedChainInput) (*stagedChainOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_create_staged_result_chain")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	authUser, err := h.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.requirePhaseManager(ctx, gameID, authUser.ID,
		"only the GM can create action results", "Create staged result chain forbidden"); err != nil {
		return nil, err
	}

	activePhase, err := h.PhaseService.GetActivePhase(ctx, gameID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get active phase", "error", err)
	}
	if activePhase == nil {
		return nil, h.logAndErr(ctx, core.ErrBadRequest(fmt.Errorf("no active phase for this game")), "Bad create staged result chain request")
	}

	parts := make([]core.StagedResultPart, 0, len(in.Body.Parts))
	for _, part := range in.Body.Parts {
		parts = append(parts, core.StagedResultPart{
			Content:      part.Content,
			DelayMinutes: part.DelayMinutes,
		})
	}

	created, err := h.ActionSubmissionService.CreateStagedResultChain(ctx, core.CreateStagedResultChainRequest{
		GameID:             gameID,
		PhaseID:            activePhase.ID,
		UserID:             in.Body.UserID,
		CharacterID:        in.Body.CharacterID,
		ActionSubmissionID: in.Body.ActionSubmissionID,
		GMUserID:           authUser.ID,
		Parts:              parts,
		IsPublished:        in.Body.IsPublished,
	})
	if err != nil {
		// Chain-shape violations (too few parts, too many, delay out of range, a
		// head carrying a delay) are the caller's mistake, not a server fault.
		// They surface as 400 so the composer can show the GM what to fix.
		if errors.Is(err, core.ErrInvalidStagedChain) {
			return nil, h.logAndErr(ctx, core.ErrBadRequest(err), "Invalid staged result chain", "error", err)
		}
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to create staged result chain", "error", err)
	}

	// Every part is echoed back with its content, including unreleased ones.
	// The GM authored them, and this is the GM-only creation response -- the
	// withholding rule applies to the player-facing read path, not here.
	partCount := len(created)
	response := make([]ActionResultWithDetailsResponse, 0, partCount)
	for i, result := range created {
		partNumber := int32(i + 1)
		resultResp := ActionResultWithDetailsResponse{
			ID:          result.ID,
			GameID:      result.GameID,
			UserID:      result.UserID,
			PhaseID:     result.PhaseID,
			GMUserID:    result.GmUserID,
			Content:     result.Content,
			IsPublished: result.IsPublished.Bool,
			PartNumber:  &partNumber,
			PartCount:   int32(partCount),
		}

		if result.CharacterID.Valid {
			charID := result.CharacterID.Int32
			resultResp.CharacterID = &charID
		}

		if result.ActionSubmissionID.Valid {
			submissionID := result.ActionSubmissionID.Int32
			resultResp.ActionSubmissionID = &submissionID
		}

		if result.SentAt.Valid {
			resultResp.SentAt = &result.SentAt.Time
		}

		if result.ReleasedAt.Valid {
			resultResp.ReleasedAt = &result.ReleasedAt.Time
		}

		response = append(response, resultResp)
	}

	return &stagedChainOutput{Body: response}, nil
}

func (h *Handler) humaAppendStagedPart(ctx context.Context, in *appendStagedPartInput) (*stagedPartOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_append_staged_part")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.requireStagedChainManagerCtx(ctx, gameID, in.ResultID, "only the GM can add a result part"); err != nil {
		return nil, err
	}

	appended, err := h.ActionSubmissionService.AppendStagedPart(ctx, in.ResultID, core.StagedResultPart{
		Content:      in.Body.Content,
		DelayMinutes: in.Body.DelayMinutes,
	})
	if err != nil {
		return nil, h.stagedEditErr(ctx, err, "Failed to append staged part")
	}

	return &stagedPartOutput{Body: stagedPartResponse(*appended)}, nil
}

func (h *Handler) humaUpdateStagedPartDelay(ctx context.Context, in *updateStagedDelayInput) (*stagedPartOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_staged_part_delay")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.requireStagedChainManagerCtx(ctx, gameID, in.ResultID, "only the GM can retime a result part"); err != nil {
		return nil, err
	}

	updated, err := h.ActionSubmissionService.UpdateStagedPartDelay(ctx, in.ResultID, in.Body.DelayMinutes)
	if err != nil {
		return nil, h.stagedEditErr(ctx, err, "Failed to update staged part delay")
	}

	return &stagedPartOutput{Body: stagedPartResponse(*updated)}, nil
}

func (h *Handler) humaCancelPendingStagedPart(ctx context.Context, in *resultIDInput) (*noContentOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_cancel_pending_staged_part")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.requireStagedChainManagerCtx(ctx, gameID, in.ResultID, "only the GM can cancel a pending result part"); err != nil {
		return nil, err
	}

	if err := h.ActionSubmissionService.CancelPendingPart(ctx, in.ResultID); err != nil {
		// "Already released" and "not a staged part" are both states the GM can
		// reach by racing the release worker or clicking the wrong control, so
		// they are 400s rather than 500s.
		if errors.Is(err, core.ErrCannotCancelPart) {
			return nil, h.logAndErr(ctx, core.ErrBadRequest(err), "Cannot cancel staged part", "error", err)
		}
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to cancel pending part", "error", err)
	}

	return &noContentOutput{}, nil
}

// requireStagedChainManagerCtx answers the GM check and confirms the target
// result belongs to the game in the URL. The context-only twin of the chi
// requireStagedChainManager, which rendered its own failure; this returns it.
//
// The ownership check is not redundant with the permission check. The two
// identifiers come from different parts of the URL: permission is granted over
// {gameID}, but the row acted upon is named by {resultId}. Without binding them,
// any GM can pass their own game ID -- passing the permission check honestly --
// while naming a result in someone else's game. The service methods take only a
// result ID and so cannot catch it.
//
// A mismatch answers 404 rather than 403 or 400: to a caller with no rights over
// the result's actual game, the result does not exist, and distinguishing "wrong
// game" from "no such result" would confirm that a given ID is a real result
// somewhere.
func (h *Handler) requireStagedChainManagerCtx(ctx context.Context, gameID, resultID int32, forbiddenMsg string) error {
	authUser, err := h.requireAuth(ctx)
	if err != nil {
		return err
	}

	if err := h.requirePhaseManager(ctx, gameID, authUser.ID, forbiddenMsg, forbiddenMsg); err != nil {
		return err
	}

	result, err := h.ActionSubmissionService.GetActionResult(ctx, resultID)
	if err != nil {
		return h.logAndErr(ctx, core.ErrNotFound("action result not found"), "Failed to get action result", "error", err)
	}

	if result.GameID != gameID {
		return h.logAndErr(ctx, core.ErrNotFound("action result not found"),
			"Staged part does not belong to this game",
			"result_id", resultID, "url_game_id", gameID, "result_game_id", result.GameID)
	}

	return nil
}

// stagedEditErr maps the service's sentinels onto status codes.
//
// The two are deliberately different: a malformed request (delay out of range,
// chain too long) is 400 and the GM can fix it by changing what they sent,
// while a well-formed request that the world has moved past (already published,
// already released) is 409 and no amount of editing the request will help.
func (h *Handler) stagedEditErr(ctx context.Context, err error, logMsg string) error {
	switch {
	case errors.Is(err, core.ErrInvalidStagedChain):
		return h.logAndErr(ctx, core.ErrBadRequest(err), logMsg, "error", err)
	case errors.Is(err, core.ErrCannotEditChain):
		return h.logAndErr(ctx, core.ErrConflict(err.Error()), logMsg, "error", err)
	case err != nil && err.Error() == "action result not found":
		return h.logAndErr(ctx, core.ErrNotFound("action result not found"), logMsg, "error", err)
	default:
		return h.logAndErr(ctx, core.ErrInternalError(err), logMsg, "error", err)
	}
}

// Draft character updates

// requireDraftUpdateAccess is the context twin of validateGMAccessAndResult:
// GM permission over the URL's game, plus confirmation that the named result
// belongs to it.
//
// Unlike the chi helper it uses the injected services rather than constructing
// its own from h.App.Pool, so a test supplying mocks actually exercises them.
//
// The game mismatch answers 400 here, not the 404 the staged-part guard sends.
// That difference is inherited from the chi handlers and preserved deliberately;
// unifying them is a behaviour change that belongs in its own commit.
func (h *Handler) requireDraftUpdateAccess(ctx context.Context, gameID, resultID int32) (*models.ActionResult, error) {
	authUser, err := h.requireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.requirePhaseManager(ctx, gameID, authUser.ID,
		"only the GM can manage draft character updates", "Validate g m access and result forbidden"); err != nil {
		return nil, err
	}

	result, err := h.ActionSubmissionService.GetActionResult(ctx, resultID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrNotFound("action result not found"), "Failed to get action result", "error", err)
	}

	if result.GameID != gameID {
		return nil, h.logAndErr(ctx, core.ErrBadRequest(fmt.Errorf("action result does not belong to this game")),
			"Bad validate g m access and result request")
	}

	return result, nil
}

func (h *Handler) humaCreateDraftCharacterUpdate(ctx context.Context, in *createDraftUpdateInput) (*draftUpdateOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_create_draft_character_update")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	result, err := h.requireDraftUpdateAccess(ctx, gameID, in.ResultID)
	if err != nil {
		return nil, err
	}

	// SECURITY: the character named in the body must belong to the result's
	// recipient *and* to this game. Without both, a GM could stage an update
	// against any character id in the system.
	var validatedCharacterID int32
	query := `SELECT id FROM characters WHERE id = $1 AND user_id = $2 AND game_id = $3`
	if err := h.App.Pool.QueryRow(ctx, query, in.Body.CharacterID, result.UserID, gameID).Scan(&validatedCharacterID); err != nil {
		return nil, h.logAndErr(ctx, core.ErrBadRequest(fmt.Errorf("character not found or does not belong to this user/game")),
			"Character validation failed", "error", err, "character_id", in.Body.CharacterID, "user_id", result.UserID, "game_id", gameID)
	}

	draft, err := h.ActionSubmissionService.CreateDraftCharacterUpdate(ctx, core.CreateDraftCharacterUpdateRequest{
		ActionResultID: in.ResultID,
		CharacterID:    in.Body.CharacterID,
		ModuleType:     in.Body.ModuleType,
		FieldName:      in.Body.FieldName,
		FieldValue:     in.Body.FieldValue,
		FieldType:      in.Body.FieldType,
		Operation:      in.Body.Operation,
	})
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to create draft character update", "error", err)
	}

	return &draftUpdateOutput{Body: draftUpdateResponse(draft)}, nil
}

func draftUpdateResponse(draft *models.ActionResultCharacterUpdate) *DraftCharacterUpdateResponse {
	return &DraftCharacterUpdateResponse{
		ID:             draft.ID,
		ActionResultID: draft.ActionResultID,
		CharacterID:    draft.CharacterID,
		ModuleType:     draft.ModuleType,
		FieldName:      draft.FieldName,
		FieldValue:     draft.FieldValue.String,
		FieldType:      draft.FieldType,
		Operation:      draft.Operation,
		CreatedAt:      draft.CreatedAt.Time,
		UpdatedAt:      draft.UpdatedAt.Time,
	}
}

func (h *Handler) humaGetDraftCharacterUpdates(ctx context.Context, in *resultIDInput) (*draftUpdateListOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_draft_character_updates")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := h.requireDraftUpdateAccess(ctx, gameID, in.ResultID); err != nil {
		return nil, err
	}

	drafts, err := h.ActionSubmissionService.GetDraftCharacterUpdates(ctx, in.ResultID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get draft character updates", "error", err)
	}

	var response []DraftCharacterUpdateResponse
	for _, draft := range drafts {
		response = append(response, *draftUpdateResponse(&draft))
	}

	return &draftUpdateListOutput{Body: response}, nil
}

func (h *Handler) humaUpdateDraftCharacterUpdate(ctx context.Context, in *updateDraftUpdateInput) (*draftUpdateOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_draft_character_update")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := h.requireDraftUpdateAccess(ctx, gameID, in.ResultID); err != nil {
		return nil, err
	}

	draft, err := h.ActionSubmissionService.UpdateDraftCharacterUpdate(ctx, in.DraftID, in.Body.FieldValue)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to update draft character update", "error", err)
	}

	return &draftUpdateOutput{Body: draftUpdateResponse(draft)}, nil
}

func (h *Handler) humaDeleteDraftCharacterUpdate(ctx context.Context, in *draftIDInput) (*noContentOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_draft_character_update")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := h.requireDraftUpdateAccess(ctx, gameID, in.ResultID); err != nil {
		return nil, err
	}

	if err := h.ActionSubmissionService.DeleteDraftCharacterUpdate(ctx, in.DraftID); err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to delete draft character update", "error", err)
	}

	return &noContentOutput{}, nil
}

func (h *Handler) humaGetDraftUpdateCount(ctx context.Context, in *resultIDInput) (*draftCountOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_draft_update_count")()

	gameID, err := gameIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := h.requireDraftUpdateAccess(ctx, gameID, in.ResultID); err != nil {
		return nil, err
	}

	count, err := h.ActionSubmissionService.GetDraftUpdateCount(ctx, in.ResultID)
	if err != nil {
		return nil, h.logAndErr(ctx, core.ErrInternalError(err), "Failed to get draft update count", "error", err)
	}

	out := &draftCountOutput{}
	out.Body.Count = count
	return out, nil
}

// Registration

// RegisterHumaGamePhases registers the phase, action, result and
// draft-character-update operations scoped to one game. Paths are relative to
// the /games/{gameID} subrouter, whose GameMiddleware supplies the gameID and
// is_gm context values these handlers read.
func RegisterHumaGamePhases(api huma.API, h *Handler) {
	bearer := []map[string][]string{{"BearerAuth": {}}}

	huma.Register(api, huma.Operation{
		OperationID:   "createPhase",
		Method:        http.MethodPost,
		Path:          "/phases",
		Summary:       "Create a phase",
		Description:   "Creates a phase in the game. Phase number is assigned sequentially and the phase starts inactive. GM only.",
		Tags:          []string{"Phases"},
		Security:      bearer,
		DefaultStatus: http.StatusCreated,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid request body, or the game is archived"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can create phases"},
		},
	}, h.humaCreatePhase)

	huma.Register(api, huma.Operation{
		OperationID: "getCurrentPhase",
		Method:      http.MethodGet,
		Path:        "/current-phase",
		Summary:     "Get the active phase",
		Description: "Returns the game's active phase, or {\"phase\": null} when none is active. Only one phase per game can be active.",
		Tags:        []string{"Phases"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaGetCurrentPhase)

	huma.Register(api, huma.Operation{
		OperationID: "listGamePhases",
		Method:      http.MethodGet,
		Path:        "/phases",
		Summary:     "List game phases",
		Description: "Lists every phase of the game in order.",
		Tags:        []string{"Phases"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaGetGamePhases)

	huma.Register(api, huma.Operation{
		OperationID:   "submitAction",
		Method:        http.MethodPost,
		Path:          "/actions",
		Summary:       "Submit an action",
		Description:   "Submits or updates the caller's action for the active phase. Only valid during an action phase.",
		Tags:          []string{"Actions"},
		Security:      bearer,
		DefaultStatus: http.StatusCreated,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid request body, no active phase, or the game is archived"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not allowed to submit actions, or the active phase is not an action phase"},
		},
	}, h.humaSubmitAction)

	huma.Register(api, huma.Operation{
		OperationID: "listGameActions",
		Method:      http.MethodGet,
		Path:        "/actions",
		Summary:     "List all game actions",
		Description: "Lists every player's action submissions for the game. GM only.",
		Tags:        []string{"Actions"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can view all actions"},
		},
	}, h.humaGetGameActions)

	huma.Register(api, huma.Operation{
		OperationID: "listMyActions",
		Method:      http.MethodGet,
		Path:        "/actions/mine",
		Summary:     "List my action submissions",
		Description: "Lists the caller's own action submissions across every phase of the game.",
		Tags:        []string{"Actions"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaGetUserActions)

	huma.Register(api, huma.Operation{
		OperationID:   "createActionResult",
		Method:        http.MethodPost,
		Path:          "/results",
		Summary:       "Create an action result",
		Description:   "Writes the GM's result for a player action in the active phase. Can be saved as a draft or published immediately.",
		Tags:          []string{"Action Results"},
		Security:      bearer,
		DefaultStatus: http.StatusCreated,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid request body, or no active phase"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can create action results"},
		},
	}, h.humaCreateActionResult)

	huma.Register(api, huma.Operation{
		OperationID:   "createStagedResultChain",
		Method:        http.MethodPost,
		Path:          "/results/staged",
		Summary:       "Create a staged result chain",
		Description:   "Creates a multi-part result whose parts reveal on a timer. Created atomically: a partial chain would contain parts that could never become due.",
		Tags:          []string{"Action Results"},
		Security:      bearer,
		DefaultStatus: http.StatusCreated,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid chain shape, or no active phase"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can create action results"},
		},
	}, h.humaCreateStagedResultChain)

	huma.Register(api, huma.Operation{
		OperationID: "listGameActionResults",
		Method:      http.MethodGet,
		Path:        "/results",
		Summary:     "List all game action results",
		Description: "Lists every result in the game, drafts included. Open to the GM, to audience members, and to any authenticated user once the game is a public archive.",
		Tags:        []string{"Action Results"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not the GM or audience, and the game is not a public archive"},
		},
	}, h.humaGetGameActionResults)

	huma.Register(api, huma.Operation{
		OperationID: "listMyActionResults",
		Method:      http.MethodGet,
		Path:        "/results/mine",
		Summary:     "List my action results",
		Description: "Lists the caller's own published results. Staged parts that have not released yet are returned with their content blanked.",
		Tags:        []string{"Action Results"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaGetUserActionResults)

	huma.Register(api, huma.Operation{
		OperationID: "updateActionResult",
		Method:      http.MethodPut,
		Path:        "/results/{resultId}",
		Summary:     "Update an action result",
		Description: "Replaces the content of an unpublished result. GM only.",
		Tags:        []string{"Action Results"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid request body"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can update action results"},
		},
	}, h.humaUpdateActionResult)

	huma.Register(api, huma.Operation{
		OperationID: "deleteActionResult",
		Method:      http.MethodDelete,
		Path:        "/results/{resultId}",
		Summary:     "Delete a draft action result",
		Description: "Deletes an unpublished result. A published-but-unreleased staged part is not matched by this; cancel it instead.",
		Tags:        []string{"Action Results"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can delete action results"},
		},
	}, h.humaDeleteActionResult)

	huma.Register(api, huma.Operation{
		OperationID: "cancelPendingStagedPart",
		Method:      http.MethodDelete,
		Path:        "/results/{resultId}/pending",
		Summary:     "Cancel a pending staged part",
		Description: "Cancels a staged part that is scheduled but not yet released. Separate from deleting a draft, which is guarded on is_published = false and so matches nothing here.",
		Tags:        []string{"Action Results"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "Already released, or not a staged part"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can cancel a pending result part"},
			"404": {Description: "Result not found, or it belongs to another game"},
		},
	}, h.humaCancelPendingStagedPart)

	huma.Register(api, huma.Operation{
		OperationID:   "appendStagedPart",
		Method:        http.MethodPost,
		Path:          "/results/{resultId}/parts",
		Summary:       "Append a staged part",
		Description:   "Adds one part to the end of a draft chain. The result ID may be any member of the chain; an ordinary draft is a chain of one, so this is also how a plain draft becomes staged.",
		Tags:          []string{"Action Results"},
		Security:      bearer,
		DefaultStatus: http.StatusCreated,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid chain shape, such as a delay out of range"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can add a result part"},
			"404": {Description: "Result not found, or it belongs to another game"},
			"409": {Description: "The chain is already published and cannot be extended"},
		},
	}, h.humaAppendStagedPart)

	huma.Register(api, huma.Operation{
		OperationID: "updateStagedPartDelay",
		Method:      http.MethodPut,
		Path:        "/results/{resultId}/delay",
		Summary:     "Retime a staged part",
		Description: "Changes the delay on a part that has not yet released. Works on drafts and on published-but-pending parts alike.",
		Tags:        []string{"Action Results"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "Delay out of range"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can retime a result part"},
			"404": {Description: "Result not found, or it belongs to another game"},
			"409": {Description: "The part has already released"},
		},
	}, h.humaUpdateStagedPartDelay)

	huma.Register(api, huma.Operation{
		OperationID: "publishActionResult",
		Method:      http.MethodPost,
		Path:        "/results/{resultId}/publish",
		Summary:     "Publish an action result",
		Description: "Publishes one result to its recipient. GM only.",
		Tags:        []string{"Action Results"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can publish action results"},
		},
	}, h.humaPublishActionResult)

	huma.Register(api, huma.Operation{
		OperationID: "publishAllPhaseResults",
		Method:      http.MethodPost,
		Path:        "/phases/{phaseId}/results/publish",
		Summary:     "Publish all results for a phase",
		Description: "Publishes every unpublished result in the phase. GM only.",
		Tags:        []string{"Action Results"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can publish action results"},
		},
	}, h.humaPublishAllPhaseResults)

	huma.Register(api, huma.Operation{
		OperationID: "getUnpublishedResultsCount",
		Method:      http.MethodGet,
		Path:        "/phases/{phaseId}/results/unpublished-count",
		Summary:     "Count unpublished results",
		Description: "Returns how many results in the phase are still unpublished. GM only.",
		Tags:        []string{"Action Results"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can view result counts"},
		},
	}, h.humaGetUnpublishedResultsCount)

	huma.Register(api, huma.Operation{
		OperationID:   "createDraftCharacterUpdate",
		Method:        http.MethodPost,
		Path:          "/results/{resultId}/character-updates",
		Summary:       "Create a draft character update",
		Description:   "Stages a character sheet change to be applied when the result is published. The character must belong to the result's recipient and to this game.",
		Tags:          []string{"Draft Character Updates"},
		Security:      bearer,
		DefaultStatus: http.StatusCreated,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid request body, unknown character, or the result belongs to another game"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can manage draft character updates"},
			"404": {Description: "Action result not found"},
		},
	}, h.humaCreateDraftCharacterUpdate)

	huma.Register(api, huma.Operation{
		OperationID: "listDraftCharacterUpdates",
		Method:      http.MethodGet,
		Path:        "/results/{resultId}/character-updates",
		Summary:     "List draft character updates",
		Description: "Lists the character sheet changes staged against a result. GM only.",
		Tags:        []string{"Draft Character Updates"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "The result belongs to another game"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can manage draft character updates"},
			"404": {Description: "Action result not found"},
		},
	}, h.humaGetDraftCharacterUpdates)

	huma.Register(api, huma.Operation{
		OperationID: "getDraftUpdateCount",
		Method:      http.MethodGet,
		Path:        "/results/{resultId}/character-updates/count",
		Summary:     "Count draft character updates",
		Description: "Returns how many character sheet changes are staged against a result. GM only.",
		Tags:        []string{"Draft Character Updates"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "The result belongs to another game"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can manage draft character updates"},
			"404": {Description: "Action result not found"},
		},
	}, h.humaGetDraftUpdateCount)

	huma.Register(api, huma.Operation{
		OperationID: "updateDraftCharacterUpdate",
		Method:      http.MethodPut,
		Path:        "/results/{resultId}/character-updates/{draftId}",
		Summary:     "Update a draft character update",
		Description: "Changes the staged value of one draft character update. GM only.",
		Tags:        []string{"Draft Character Updates"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid request body, or the result belongs to another game"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can manage draft character updates"},
			"404": {Description: "Action result not found"},
		},
	}, h.humaUpdateDraftCharacterUpdate)

	huma.Register(api, huma.Operation{
		OperationID: "deleteDraftCharacterUpdate",
		Method:      http.MethodDelete,
		Path:        "/results/{resultId}/character-updates/{draftId}",
		Summary:     "Delete a draft character update",
		Description: "Removes one staged character sheet change. GM only.",
		Tags:        []string{"Draft Character Updates"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "The result belongs to another game"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can manage draft character updates"},
			"404": {Description: "Action result not found"},
		},
	}, h.humaDeleteDraftCharacterUpdate)
}

// RegisterHumaPhases registers the phase-id operations mounted at /phases.
//
// These are not game-scoped: the phase ID alone identifies the row, and the
// handlers resolve its game themselves to run the GM check. That is why they
// live at their own mount rather than under /games/{gameID}.
func RegisterHumaPhases(api huma.API, h *Handler) {
	bearer := []map[string][]string{{"BearerAuth": {}}}

	huma.Register(api, huma.Operation{
		OperationID: "activatePhase",
		Method:      http.MethodPost,
		Path:        "/{id}/activate",
		Summary:     "Activate a phase",
		Description: "Makes this phase the game's active one, deactivating any other. GM only.",
		Tags:        []string{"Phases"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can activate phases"},
		},
	}, h.humaActivatePhase)

	huma.Register(api, huma.Operation{
		OperationID: "updatePhaseDeadline",
		Method:      http.MethodPut,
		Path:        "/{id}/deadline",
		Summary:     "Update a phase deadline",
		Description: "Sets or extends the phase's deadline. GM only.",
		Tags:        []string{"Phases"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid request body"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can update phase deadlines"},
		},
	}, h.humaUpdatePhaseDeadline)

	huma.Register(api, huma.Operation{
		OperationID: "updatePhase",
		Method:      http.MethodPut,
		Path:        "/{id}",
		Summary:     "Update a phase",
		Description: "Updates the phase's title, description, start time and deadline. End time is system-managed and cannot be set here.",
		Tags:        []string{"Phases"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "Invalid request body"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can update phases"},
		},
	}, h.humaUpdatePhase)

	huma.Register(api, huma.Operation{
		OperationID: "deletePhase",
		Method:      http.MethodDelete,
		Path:        "/{id}",
		Summary:     "Delete a phase",
		Description: "Deletes a phase that has no associated content. GM only.",
		Tags:        []string{"Phases"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"400": {Description: "The phase has associated content and cannot be deleted"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can delete phases"},
		},
	}, h.humaDeletePhase)
}

// applyStagedFields copies the staged-reveal columns onto a response.
//
// Shared by the player and GM/audience read paths so the two cannot drift on
// how a chain is described. The visibility difference between those roles is
// enforced in SQL (GetUserResults blanks unreleased content), not here: this
// helper is deliberately identical for every viewer, which is what keeps the
// serializer free of viewer-dependent branching.
//
// All four fields are omitempty on the response, so an ordinary single-part
// result — where part_number is NULL because it never entered the chain CTE —
// serializes exactly as it did before staged reveals existed.
func applyStagedFields(resp *ActionResultWithDetailsResponse, partNumber pgtype.Int4, partCount pgtype.Int8, releasedAt, unlocksAt pgtype.Timestamptz, revealDelayMinutes pgtype.Int4) {
	if partNumber.Valid {
		n := partNumber.Int32
		resp.PartNumber = &n
	}
	if partCount.Valid {
		// int64 in SQL (COUNT), int32 on the wire. A chain is capped at
		// core.MaxStagedChainLength, so the narrowing cannot overflow.
		resp.PartCount = int32(partCount.Int64)
	}
	// Present for released parts only. Its absence is how a client identifies a
	// part that is still locked.
	if releasedAt.Valid {
		resp.ReleasedAt = &releasedAt.Time
	}
	// Set only for the next part due out, whose parent has already released.
	// Parts further down the chain have no knowable unlock time yet.
	if unlocksAt.Valid {
		resp.UnlocksAt = &unlocksAt.Time
	}
	// The configured wait, which the GM's delay selector needs to show what the
	// timer currently is. Distinct from UnlocksAt: that is a resolved timestamp
	// and is absent until the parent releases, so it cannot stand in for this.
	//
	// Load-bearing for the GM edit path. Without it the selector receives
	// undefined, matches no option, and silently displays the first preset — so
	// a 30-minute delay reads as 1 minute with nothing to indicate it is wrong.
	if revealDelayMinutes.Valid {
		delay := revealDelayMinutes.Int32
		resp.RevealDelayMinutes = &delay
	}
}

// stagedPartResponse shapes a single part for the GM-facing edit endpoints.
//
// Content is echoed back in full, including for unreleased parts. The GM wrote
// it, and the withholding rule governs the player's read path, not this one.
func stagedPartResponse(result models.ActionResult) ActionResultWithDetailsResponse {
	resp := ActionResultWithDetailsResponse{
		ID:          result.ID,
		GameID:      result.GameID,
		UserID:      result.UserID,
		PhaseID:     result.PhaseID,
		GMUserID:    result.GmUserID,
		Content:     result.Content,
		IsPublished: result.IsPublished.Bool,
	}

	if result.CharacterID.Valid {
		charID := result.CharacterID.Int32
		resp.CharacterID = &charID
	}

	if result.ActionSubmissionID.Valid {
		submissionID := result.ActionSubmissionID.Int32
		resp.ActionSubmissionID = &submissionID
	}

	if result.SentAt.Valid {
		resp.SentAt = &result.SentAt.Time
	}

	if result.ReleasedAt.Valid {
		resp.ReleasedAt = &result.ReleasedAt.Time
	}

	if result.ParentResultID.Valid {
		parentID := result.ParentResultID.Int32
		resp.ParentResultID = &parentID
	}

	if result.RevealDelayMinutes.Valid {
		delay := result.RevealDelayMinutes.Int32
		resp.RevealDelayMinutes = &delay
	}

	return resp
}
