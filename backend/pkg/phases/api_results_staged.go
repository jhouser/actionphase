package phases

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"actionphase/pkg/core"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// CreateStagedResultChain creates a whole staged result chain in one request (GM only).
//
// The chain is created atomically by the service: a partially-written chain
// would contain parts whose parent does not exist, and those parts could never
// become due. There is deliberately no endpoint for appending to an existing
// chain, which would reintroduce that hazard one request at a time.
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
