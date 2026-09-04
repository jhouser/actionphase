package deadlines

// Huma (type-first) implementation of the deadline API.
//
// See .claude/planning/huma-migration.md.

import (
	"context"
	"net/http"
	"time"

	"actionphase/pkg/core"
	"actionphase/pkg/humaconfig"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/render"
)

// ------------------------------------------------------------------ I/O types

// deadlineBody is the create/update payload. Both operations take the same
// fields, and the chi handlers used two identical structs for them.
//
// maxLength is 100 because game_deadlines.title is VARCHAR(100); an earlier
// tag said 255, so an over-length title passed validation and then failed on
// INSERT as a 500. requests_validation_test.go pins this.
type deadlineBody struct {
	Title string `json:"title" required:"true" minLength:"1" maxLength:"100" doc:"Deadline title"`
	// Optional, and empty is meaningful: the frontend sends "" when the GM
	// leaves the field blank.
	Description string    `json:"description,omitempty" required:"false" doc:"Optional longer description"`
	Deadline    time.Time `json:"deadline" required:"true" doc:"When the deadline falls due (RFC 3339)"`
}

// Resolve restores the trim-then-validate behaviour core.ValidateStruct gave
// the chi handlers, so a whitespace-only title is still a 400 rather than a
// blank row. See humaconfig.TrimStrings.
func (b *deadlineBody) Resolve(huma.Context) []error {
	return humaconfig.TrimStrings(b)
}

type createDeadlineInput struct {
	GameID int32 `path:"gameID" minimum:"1" doc:"Game ID"`
	Body   deadlineBody
}

type listGameDeadlinesInput struct {
	GameID         int32 `path:"gameID" minimum:"1" doc:"Game ID"`
	IncludeExpired bool  `query:"includeExpired" doc:"Include deadlines that have already passed"`
}

type deadlineIDPathInput struct {
	DeadlineID int32 `path:"deadlineId" minimum:"1" doc:"Deadline ID"`
}

type updateDeadlineInput struct {
	DeadlineID int32 `path:"deadlineId" minimum:"1" doc:"Deadline ID"`
	Body       deadlineBody
}

type upcomingDeadlinesInput struct {
	// The chi handler silently fell back to 10 for anything unparseable or out
	// of range. Huma rejects those with a 400 instead, which is the better
	// contract: a client sending limit=abc was being quietly ignored.
	Limit int32 `query:"limit" default:"10" minimum:"1" maximum:"100" doc:"Maximum deadlines to return"`
}

type deadlineOutput struct {
	Body *DeadlineResponse
}

type unifiedDeadlinesOutput struct {
	Body []*UnifiedDeadlineResponse
}

type upcomingDeadlinesOutput struct {
	Body []*DeadlineWithGameResponse
}

// ------------------------------------------------------------------- helpers

// humaErr converts the shared render.Renderer errors into huma ones, preserving
// status and message.
func humaErr(errResp render.Renderer) error {
	resp, ok := errResp.(*core.ErrResponse)
	if !ok {
		return huma.Error500InternalServerError("request failed")
	}
	return huma.NewError(resp.HTTPStatusCode, resp.Detail)
}

// requireGM resolves the caller and confirms they may manage this game's
// deadlines.
//
// Note this returns 401 (not 403) for a non-GM, matching verifyUserIsGM and the
// existing tests. See the note in the migration plan.
func (h *Handler) requireGM(ctx context.Context, gameID int32) (int32, error) {
	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		return 0, humaErr(errResp)
	}
	if _, errResp := h.verifyUserIsGM(ctx, gameID, userID); errResp != nil {
		return 0, humaErr(errResp)
	}
	return userID, nil
}

// ------------------------------------------------------------------ handlers

func (h *Handler) HumaCreateDeadline(ctx context.Context, in *createDeadlineInput) (*deadlineOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_create_deadline")()

	userID, err := h.requireGM(ctx, in.GameID)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Request rejected in create deadline", "game_id", in.GameID)
		return nil, err
	}

	deadline, svcErr := h.DeadlineService.CreateDeadline(ctx, core.CreateDeadlineRequest{
		GameID:      in.GameID,
		Title:       in.Body.Title,
		Description: in.Body.Description,
		Deadline:    in.Body.Deadline,
		CreatedBy:   userID,
	})
	if svcErr != nil {
		h.App.ObsLogger.Error(ctx, "Failed to create deadline", "error", svcErr, "game_id", in.GameID)
		return nil, huma.Error500InternalServerError(svcErr.Error())
	}

	h.App.ObsLogger.Info(ctx, "Deadline created successfully",
		"deadline_id", deadline.ID, "game_id", in.GameID)
	return &deadlineOutput{Body: toDeadlineResponse(deadline)}, nil
}

// HumaGetGameDeadlines returns every deadline for a game — arbitrary, phase and
// poll deadlines in one unified list. Readable by any authenticated user who
// can see the game, not only the GM.
func (h *Handler) HumaGetGameDeadlines(ctx context.Context, in *listGameDeadlinesInput) (*unifiedDeadlinesOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_game_deadlines")()

	if _, err := h.GameService.GetGame(ctx, in.GameID); err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get game", "error", err, "game_id", in.GameID)
		return nil, huma.Error404NotFound("Game not found")
	}

	deadlines, err := h.DeadlineService.GetAllGameDeadlines(ctx, in.GameID, in.IncludeExpired)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get all game deadlines", "error", err, "game_id", in.GameID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	out := make([]*UnifiedDeadlineResponse, len(deadlines))
	for i := range deadlines {
		out[i] = toUnifiedDeadlineResponse(&deadlines[i])
	}
	return &unifiedDeadlinesOutput{Body: out}, nil
}

func (h *Handler) HumaUpdateDeadline(ctx context.Context, in *updateDeadlineInput) (*deadlineOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_deadline")()

	existing, err := h.DeadlineService.GetDeadline(ctx, in.DeadlineID)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get deadline", "error", err, "deadline_id", in.DeadlineID)
		return nil, huma.Error404NotFound("Deadline not found")
	}

	if _, err := h.requireGM(ctx, existing.GameID); err != nil {
		h.App.ObsLogger.Warn(ctx, "Request rejected in update deadline", "deadline_id", in.DeadlineID)
		return nil, err
	}

	deadline, svcErr := h.DeadlineService.UpdateDeadline(ctx, in.DeadlineID, core.UpdateDeadlineRequest{
		Title:       in.Body.Title,
		Description: in.Body.Description,
		Deadline:    in.Body.Deadline,
	})
	if svcErr != nil {
		h.App.ObsLogger.Error(ctx, "Failed to update deadline", "error", svcErr, "deadline_id", in.DeadlineID)
		return nil, huma.Error500InternalServerError(svcErr.Error())
	}

	h.App.ObsLogger.Info(ctx, "Deadline updated successfully", "deadline_id", deadline.ID)
	return &deadlineOutput{Body: toDeadlineResponse(deadline)}, nil
}

func (h *Handler) HumaDeleteDeadline(ctx context.Context, in *deadlineIDPathInput) (*struct{}, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_deadline")()

	existing, err := h.DeadlineService.GetDeadline(ctx, in.DeadlineID)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get deadline", "error", err, "deadline_id", in.DeadlineID)
		return nil, huma.Error404NotFound("Deadline not found")
	}

	userID, err := h.requireGM(ctx, existing.GameID)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Request rejected in delete deadline", "deadline_id", in.DeadlineID)
		return nil, err
	}

	if svcErr := h.DeadlineService.DeleteDeadline(ctx, in.DeadlineID, userID); svcErr != nil {
		h.App.ObsLogger.Error(ctx, "Failed to delete deadline", "error", svcErr, "deadline_id", in.DeadlineID)
		return nil, huma.Error500InternalServerError(svcErr.Error())
	}

	h.App.ObsLogger.Info(ctx, "Deadline deleted successfully", "deadline_id", in.DeadlineID)
	return nil, nil
}

func (h *Handler) HumaGetUpcomingDeadlines(ctx context.Context, in *upcomingDeadlinesInput) (*upcomingDeadlinesOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_upcoming_deadlines")()

	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to authenticate user from JWT")
		return nil, humaErr(errResp)
	}

	deadlines, err := h.DeadlineService.GetUpcomingDeadlines(ctx, userID, in.Limit)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to get upcoming deadlines", "error", err, "user_id", userID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	out := make([]*DeadlineWithGameResponse, len(deadlines))
	for i := range deadlines {
		out[i] = toDeadlineWithGameResponse(&deadlines[i])
	}
	return &upcomingDeadlinesOutput{Body: out}, nil
}

// ---------------------------------------------------------------- registration

// RegisterHumaGameDeadlines registers the per-game deadline operations.
//
// Paths are relative to the router this is called on — the /{gameID} subrouter.
func RegisterHumaGameDeadlines(api huma.API, h *Handler) {
	huma.Register(api, huma.Operation{
		OperationID:   "createGameDeadline",
		Method:        http.MethodPost,
		Path:          "/deadlines",
		Summary:       "Create a deadline for a game",
		Description:   "GM or Co-GM only.",
		Tags:          []string{"Deadlines"},
		DefaultStatus: http.StatusCreated,
		Security:      []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated, or not a GM of this game"},
			"404": {Description: "Game not found"},
		},
	}, h.HumaCreateDeadline)

	huma.Register(api, huma.Operation{
		OperationID: "listGameDeadlines",
		Method:      http.MethodGet,
		Path:        "/deadlines",
		Summary:     "List all deadlines for a game",
		Description: "Returns arbitrary, phase and poll deadlines in one unified list. " +
			"Readable by anyone who can view the game, not only the GM.",
		Tags:     []string{"Deadlines"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"404": {Description: "Game not found"},
		},
	}, h.HumaGetGameDeadlines)
}

// RegisterHumaDeadlines registers the deadline-addressed operations.
//
// Paths are relative to the deadlines router's mount point (/api/v1/deadlines).
func RegisterHumaDeadlines(api huma.API, h *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "getUpcomingDeadlines",
		Method:      http.MethodGet,
		Path:        "/upcoming",
		Summary:     "List upcoming deadlines across all the user's games",
		Tags:        []string{"Deadlines"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.HumaGetUpcomingDeadlines)

	huma.Register(api, huma.Operation{
		OperationID: "updateDeadline",
		Method:      http.MethodPatch,
		Path:        "/{deadlineId}",
		Summary:     "Update a deadline",
		Description: "GM or Co-GM of the deadline's game only.",
		Tags:        []string{"Deadlines"},
		Security:    []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated, or not a GM of this game"},
			"404": {Description: "Deadline not found"},
		},
	}, h.HumaUpdateDeadline)

	huma.Register(api, huma.Operation{
		OperationID:   "deleteDeadline",
		Method:        http.MethodDelete,
		Path:          "/{deadlineId}",
		Summary:       "Delete a deadline",
		Description:   "GM or Co-GM of the deadline's game only.",
		Tags:          []string{"Deadlines"},
		DefaultStatus: http.StatusNoContent,
		Security:      []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated, or not a GM of this game"},
			"404": {Description: "Deadline not found"},
		},
	}, h.HumaDeleteDeadline)
}
