package phases

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"actionphase/pkg/core"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// ActivatePhase activates a phase (GM only)
func (h *Handler) ActivatePhase(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_activate_phase")()

	phaseIDStr := chi.URLParam(r, "id")
	phaseID, err := strconv.ParseInt(phaseIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid phase ID")), "Invalid activate phase request")
		return
	}

	// Get authenticated user
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user found")
		return
	}

	phaseService := h.PhaseService

	// Get phase to check game ID
	phase, err := phaseService.GetPhase(ctx, int32(phaseID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get phase", "error", err)
		return
	}

	// Get game and check GM permissions (considers admin mode)
	gameService := h.GameService
	game, err := gameService.GetGame(ctx, phase.GameID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game", "error", err)
		return
	}

	if !core.IsUserGameMaster(r, authUser.ID, authUser.IsAdmin, *game, h.App.Pool) {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can activate phases"), "Activate phase forbidden")
		return
	}

	// Activate phase
	err = phaseService.ActivatePhase(ctx, int32(phaseID), authUser.ID)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to activate phase", "error", err)
		return
	}

	// Get the updated phase after activation
	activePhase, err := phaseService.GetPhase(ctx, int32(phaseID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get activated phase", "error", err)
		return
	}

	render.Render(w, r, convertPhaseToResponse(activePhase))
}

// PublishAllPhaseResults publishes all unpublished results for a phase (GM only)
func (h *Handler) PublishAllPhaseResults(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_publish_all_phase_results")()

	phaseIDStr := chi.URLParam(r, "phaseId")
	phaseID, err := strconv.ParseInt(phaseIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid phase ID")), "Invalid publish all phase results request")
		return
	}

	// Get authenticated user
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user found")
		return
	}

	// Check GM permissions (considers admin mode)
	if !ctx.Value("is_gm").(bool) {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can publish action results"), "Publish all phase results forbidden")
		return
	}

	// Publish all unpublished results for the phase
	actionService := h.ActionSubmissionService
	err = actionService.PublishAllPhaseResults(ctx, int32(phaseID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to publish all phase results", "error", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "All results published successfully",
	})
}

// GetUnpublishedResultsCount retrieves the count of unpublished results for a phase (GM only)
func (h *Handler) GetUnpublishedResultsCount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_unpublished_results_count")()

	phaseIDStr := chi.URLParam(r, "phaseId")
	phaseID, err := strconv.ParseInt(phaseIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid phase ID")), "Invalid get unpublished results count request")
		return
	}

	// Get authenticated user
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user found")
		return
	}

	// Check GM permissions (considers admin mode)
	if !ctx.Value("is_gm").(bool) {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can view result counts"), "Get unpublished results count forbidden")
		return
	}

	// Get count of unpublished results
	actionService := h.ActionSubmissionService
	count, err := actionService.GetUnpublishedResultsCount(ctx, int32(phaseID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get unpublished results count", "error", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"count": count,
	})
}
