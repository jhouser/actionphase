package phases

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// CreateStagedResultChain creates a whole staged result chain in one request (GM only).
//
// The chain is created atomically by the service: a partially-written chain
// would contain parts whose parent does not exist, and those parts could never
// become due.
//
// A chain can also be built up one part at a time while it is still a draft;
// see AppendStagedPart. That path is safe for the same reason this one is
// atomic — it always links the new part to an existing tail, so it cannot
// produce a part whose parent is missing.
func (h *Handler) CreateStagedResultChain(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_create_staged_result_chain")()

	gameID := ctx.Value("gameID").(int32)

	data := &CreateStagedResultChainRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid create staged result chain request", "error", err)
		return
	}

	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	phaseService := h.PhaseService
	canManage, err := phaseService.CanUserManagePhases(ctx, gameID, int32(authUser.ID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check phase management permission", "error", err)
		return
	}

	if !canManage {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can create action results"), "Create staged result chain forbidden")
		return
	}

	activePhase, err := phaseService.GetActivePhase(ctx, gameID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get active phase", "error", err)
		return
	}

	if activePhase == nil {
		h.renderError(ctx, w, r, core.ErrBadRequest(fmt.Errorf("no active phase for this game")), "Bad create staged result chain request")
		return
	}

	parts := make([]core.StagedResultPart, 0, len(data.Parts))
	for _, part := range data.Parts {
		parts = append(parts, core.StagedResultPart{
			Content:      part.Content,
			DelayMinutes: part.DelayMinutes,
		})
	}

	req := core.CreateStagedResultChainRequest{
		GameID:             gameID,
		PhaseID:            activePhase.ID,
		UserID:             data.UserID,
		CharacterID:        data.CharacterID,
		ActionSubmissionID: data.ActionSubmissionID,
		GMUserID:           int32(authUser.ID),
		Parts:              parts,
		IsPublished:        data.IsPublished,
	}

	created, err := h.ActionSubmissionService.CreateStagedResultChain(ctx, req)
	if err != nil {
		// Chain-shape violations (too few parts, too many, delay out of range, a
		// head carrying a delay) are the caller's mistake, not a server fault.
		// They surface as 400 so the composer can show the GM what to fix.
		if errors.Is(err, core.ErrInvalidStagedChain) {
			h.renderError(ctx, w, r, core.ErrBadRequest(err), "Invalid staged result chain", "error", err)
			return
		}
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to create staged result chain", "error", err)
		return
	}

	// Every part is echoed back with its content, including unreleased ones.
	// The GM authored them, and this is the GM-only creation response — the
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

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, response)
}

// AppendStagedPart adds one part to the end of a draft chain (GM only).
//
// This is what lets a GM write a staged result over several sittings rather
// than having to compose the whole thing before saving anything. The result ID
// in the URL may be any member of the chain — the service resolves the tail —
// and an ordinary unstaged draft is a chain of one, so this is also how a plain
// draft becomes a staged chain.
//
// Draft chains only. Once published, the chain is fixed: appending would extend
// a scene the player has already started reading.
func (h *Handler) AppendStagedPart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_append_staged_part")()

	gameID := ctx.Value("gameID").(int32)

	resultID, err := stagedPartIDFromURL(r)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid append staged part request")
		return
	}

	data := &AppendStagedPartRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid append staged part request", "error", err)
		return
	}

	if !h.requireStagedChainManager(ctx, w, r, gameID, "only the GM can add a result part") {
		return
	}

	appended, err := h.ActionSubmissionService.AppendStagedPart(ctx, resultID, core.StagedResultPart{
		Content:      data.Content,
		DelayMinutes: data.DelayMinutes,
	})
	if err != nil {
		h.renderStagedEditError(ctx, w, r, err, "Failed to append staged part")
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, stagedPartResponse(*appended))
}

// UpdateStagedPartDelay retimes a staged part that has not yet been released
// (GM only).
//
// Works on drafts and on published-but-pending parts alike: the common case is
// a scene already running where the players need longer than the GM first
// guessed. Refused once the part has released, since the player has read it.
func (h *Handler) UpdateStagedPartDelay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_staged_part_delay")()

	gameID := ctx.Value("gameID").(int32)

	resultID, err := stagedPartIDFromURL(r)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid update staged part delay request")
		return
	}

	data := &UpdateStagedPartDelayRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid update staged part delay request", "error", err)
		return
	}

	if !h.requireStagedChainManager(ctx, w, r, gameID, "only the GM can retime a result part") {
		return
	}

	updated, err := h.ActionSubmissionService.UpdateStagedPartDelay(ctx, resultID, data.DelayMinutes)
	if err != nil {
		h.renderStagedEditError(ctx, w, r, err, "Failed to update staged part delay")
		return
	}

	render.JSON(w, r, stagedPartResponse(*updated))
}

// stagedPartIDFromURL parses the {resultId} path parameter.
func stagedPartIDFromURL(r *http.Request) (int32, error) {
	resultID, err := strconv.ParseInt(chi.URLParam(r, "resultId"), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid result ID")
	}
	return int32(resultID), nil
}

// requireStagedChainManager answers the GM check, rendering the failure itself.
// Returns false when the caller has already been answered.
func (h *Handler) requireStagedChainManager(ctx context.Context, w http.ResponseWriter, r *http.Request, gameID int32, forbiddenMsg string) bool {
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return false
	}

	canManage, err := h.PhaseService.CanUserManagePhases(ctx, gameID, int32(authUser.ID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check phase management permission", "error", err)
		return false
	}

	if !canManage {
		h.renderError(ctx, w, r, core.ErrForbidden(forbiddenMsg), forbiddenMsg)
		return false
	}

	return true
}

// renderStagedEditError maps the service's sentinels onto status codes.
//
// The two are deliberately different: a malformed request (delay out of range,
// chain too long) is 400 and the GM can fix it by changing what they sent,
// while a well-formed request that the world has moved past (already published,
// already released) is 409 and no amount of editing the request will help.
func (h *Handler) renderStagedEditError(ctx context.Context, w http.ResponseWriter, r *http.Request, err error, logMsg string) {
	switch {
	case errors.Is(err, core.ErrInvalidStagedChain):
		h.renderError(ctx, w, r, core.ErrBadRequest(err), logMsg, "error", err)
	case errors.Is(err, core.ErrCannotEditChain):
		h.renderError(ctx, w, r, core.ErrConflict(err.Error()), logMsg, "error", err)
	case err != nil && err.Error() == "action result not found":
		h.renderError(ctx, w, r, core.ErrNotFound("action result not found"), logMsg, "error", err)
	default:
		h.renderError(ctx, w, r, core.ErrInternalError(err), logMsg, "error", err)
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

// CancelPendingStagedPart cancels a staged part that has not yet been released (GM only).
//
// Distinct from DeleteActionResult, which is guarded on is_published = false. A
// scheduled part is published but not released, so deleting it through that
// endpoint would match zero rows and report success.
func (h *Handler) CancelPendingStagedPart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_cancel_pending_staged_part")()

	gameID := ctx.Value("gameID").(int32)

	resultIDStr := chi.URLParam(r, "resultId")
	resultID, err := strconv.ParseInt(resultIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid result ID")), "Invalid cancel pending part request")
		return
	}

	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	phaseService := h.PhaseService
	canManage, err := phaseService.CanUserManagePhases(ctx, gameID, int32(authUser.ID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check phase management permission", "error", err)
		return
	}

	if !canManage {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can cancel a pending result part"), "Cancel pending part forbidden")
		return
	}

	if err := h.ActionSubmissionService.CancelPendingPart(ctx, int32(resultID)); err != nil {
		// "Already released" and "not a staged part" are both states the GM can
		// reach by racing the release worker or clicking the wrong control, so
		// they are 400s rather than 500s.
		if errors.Is(err, core.ErrCannotCancelPart) {
			h.renderError(ctx, w, r, core.ErrBadRequest(err), "Cannot cancel staged part", "error", err)
			return
		}
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to cancel pending part", "error", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
