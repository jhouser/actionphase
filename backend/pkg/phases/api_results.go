package phases

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"actionphase/pkg/core"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/jackc/pgx/v5/pgtype"
)

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

// CreateActionResult creates a result for a player action (GM only)
func (h *Handler) CreateActionResult(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_create_action_result")()

	gameID := ctx.Value("gameID").(int32)

	data := &CreateActionResultRequest{}
	if err := render.Bind(r, data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid create action result request", "error", err)
		return
	}

	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	gmUser := authUser

	// Check permissions - must be GM
	phaseService := h.PhaseService
	canManage, err := phaseService.CanUserManagePhases(ctx, int32(gameID), int32(gmUser.ID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check phase management permission", "error", err)
		return
	}

	if !canManage {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can create action results"), "Create action result forbidden")
		return
	}

	// Get active phase
	activePhase, err := phaseService.GetActivePhase(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get active phase", "error", err)
		return
	}

	if activePhase == nil {
		h.renderError(ctx, w, r, core.ErrBadRequest(fmt.Errorf("no active phase for this game")), "Bad create action result request")
		return
	}

	// Create action result using ActionSubmissionService
	actionService := h.ActionSubmissionService
	req := core.CreateActionResultRequest{
		GameID:             int32(gameID),
		UserID:             data.UserID,
		CharacterID:        data.CharacterID,
		ActionSubmissionID: data.ActionSubmissionID,
		PhaseID:            activePhase.ID,
		GMUserID:           int32(gmUser.ID),
		Content:            data.Content,
		IsPublished:        data.IsPublished,
	}

	result, err := actionService.CreateActionResult(ctx, req)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to create action result", "error", err)
		return
	}

	// Convert to response format
	response := &ActionResultResponse{
		ID:          result.ID,
		GameID:      result.GameID,
		UserID:      result.UserID,
		PhaseID:     result.PhaseID,
		GMUserID:    result.GmUserID,
		Content:     result.Content,
		IsPublished: result.IsPublished.Bool,
	}

	if result.SentAt.Valid {
		response.SentAt = &result.SentAt.Time
	}

	render.Status(r, http.StatusCreated)
	render.Render(w, r, response)
}

// GetUserActionResults retrieves user's action results for a game
func (h *Handler) GetUserActionResults(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_user_action_results")()

	gameID := ctx.Value("gameID").(int32)

	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	actionService := h.ActionSubmissionService
	results, err := actionService.GetUserResults(ctx, int32(gameID), int32(authUser.ID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get user action results", "error", err)
		return
	}

	// Convert to response format
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
		// nil is how the client identifies a locked part — it must not infer
		// lockedness from the content being empty.
		applyStagedFields(&resultResp, result.PartNumber, result.PartCount, result.ReleasedAt, result.UnlocksAt, result.RevealDelayMinutes)

		response = append(response, resultResp)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetGameActionResults retrieves all action results for a game
// - GM: Always allowed
// - Completed games: All participants can view (public archive)
// - In-progress games: GM only
func (h *Handler) GetGameActionResults(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_game_action_results")()

	gameID := ctx.Value("gameID").(int32)

	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	// Check permissions - must be GM, audience, OR game must be completed
	phaseService := h.PhaseService
	canManage, err := phaseService.CanUserManagePhases(ctx, int32(gameID), int32(authUser.ID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check phase management permission", "error", err)
		return
	}

	// Get game to check state and participant role
	gameService := h.GameService
	game, err := gameService.GetGame(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game", "error", err)
		return
	}

	// Check if user is audience member
	isAudience := core.IsUserAudience(ctx, h.App.Pool, int32(gameID), int32(authUser.ID))

	// Allow access if: GM, audience, or the game is a public archive.
	//
	// The archive branch deliberately has no membership check — a public-archive
	// game (completed OR epilogue) is readable by any authenticated user. It is
	// broader than the other two arms, which are scoped to a role: this one
	// admits non-participants.
	//
	// Epilogue must be included: a player writing an epilogue needs to see what
	// happened to everyone else, which is the whole reason that state exists.
	if !canManage && !isAudience && !core.IsPublicArchive(game.State.String) {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM, audience, or any user of a public archive game can view all action results"), "Get game action results forbidden")
		return
	}

	actionService := h.ActionSubmissionService
	results, err := actionService.GetGameResults(ctx, int32(gameID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to get game action results", "error", err)
		return
	}

	// Convert to response format.
	//
	// No is_published filter: every caller admitted above sees drafts as well as
	// published results. That is deliberate for each of the three, but for
	// different reasons, so changing one arm does not license changing another:
	//
	//   - GM: authors the drafts.
	//   - Audience: a trusted spectator role that already sees every private
	//     message and submission, so it sees unpublished and unreleased content
	//     here too. This is an explicit decision about who the role is for, not
	//     an inference about what drafts happen to contain.
	//
	//     It used to be justified on the grounds that a result can only be
	//     written against the active phase, making a draft "current-phase content
	//     rather than a future reveal". Staged reveals broke that premise: a
	//     pending part is precisely a future reveal, scheduled minutes or hours
	//     out. The conclusion is unchanged — audience still sees everything — but
	//     do not reach for the old argument to justify widening anything else.
	//   - Completed game: the archive is public, and this arm admits any
	//     authenticated user, not just participants. Unpublished drafts the GM
	//     never sent are therefore readable by anyone once a game completes.
	//
	// Contrast GetUserActionResults, which serves players and filters to
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

		// Add character_id if available
		if result.CharacterID.Valid {
			charID := result.CharacterID.Int32
			resultResp.CharacterID = &charID
		}

		// Add action_submission_id if available
		if result.ActionSubmissionID.Valid {
			submissionID := result.ActionSubmissionID.Int32
			resultResp.ActionSubmissionID = &submissionID
		}

		// Add character_name if available
		if result.CharacterName.Valid {
			resultResp.CharacterName = result.CharacterName.String
		}

		if result.SentAt.Valid {
			resultResp.SentAt = &result.SentAt.Time
		}

		// Staged reveal fields. Identical to the player path — the GM and
		// audience see the same part numbering and schedule. What differs is
		// upstream in SQL: this query never blanks content, because both roles
		// are entitled to read a part before it releases.
		applyStagedFields(&resultResp, result.PartNumber, result.PartCount, result.ReleasedAt, result.UnlocksAt, result.RevealDelayMinutes)

		response = append(response, resultResp)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DeleteActionResult deletes an unpublished (draft) action result (GM only)
func (h *Handler) DeleteActionResult(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_action_result")()

	gameID := ctx.Value("gameID").(int32)

	resultIDStr := chi.URLParam(r, "resultId")
	resultID, err := strconv.ParseInt(resultIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid result ID")), "Invalid delete action result request")
		return
	}

	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	phaseService := h.PhaseService
	canManage, err := phaseService.CanUserManagePhases(ctx, int32(gameID), int32(authUser.ID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check phase management permission", "error", err)
		return
	}

	if !canManage {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can delete action results"), "Delete action result forbidden")
		return
	}

	actionService := h.ActionSubmissionService
	if err := actionService.DeleteActionResult(ctx, int32(resultID)); err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to delete action result", "error", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UpdateActionResult updates an unpublished action result (GM only)
func (h *Handler) UpdateActionResult(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_action_result")()

	gameID := ctx.Value("gameID").(int32)

	resultIDStr := chi.URLParam(r, "resultId")
	resultID, err := strconv.ParseInt(resultIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid result ID")), "Invalid update action result request")
		return
	}

	type UpdateResultRequest struct {
		Content string `json:"content" validate:"required"`
	}

	data := &UpdateResultRequest{}
	if err := json.NewDecoder(r.Body).Decode(data); err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(err), "Invalid update action result request", "error", err)
		return
	}

	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	// Check permissions - must be GM
	phaseService := h.PhaseService
	canManage, err := phaseService.CanUserManagePhases(ctx, int32(gameID), int32(authUser.ID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check phase management permission", "error", err)
		return
	}

	if !canManage {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can update action results"), "Update action result forbidden")
		return
	}

	// Update the action result
	actionService := h.ActionSubmissionService
	result, err := actionService.UpdateActionResult(ctx, int32(resultID), data.Content)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to update action result", "error", err)
		return
	}

	// Convert to response format
	response := &ActionResultResponse{
		ID:          result.ID,
		GameID:      result.GameID,
		UserID:      result.UserID,
		PhaseID:     result.PhaseID,
		GMUserID:    result.GmUserID,
		Content:     result.Content,
		IsPublished: result.IsPublished.Bool,
	}

	if result.SentAt.Valid {
		response.SentAt = &result.SentAt.Time
	}

	render.Render(w, r, response)
}

// PublishActionResult publishes a single action result (GM only)
func (h *Handler) PublishActionResult(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_publish_action_result")()

	gameID := ctx.Value("gameID").(int32)

	resultIDStr := chi.URLParam(r, "resultId")
	resultID, err := strconv.ParseInt(resultIDStr, 10, 32)
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInvalidRequest(fmt.Errorf("invalid result ID")), "Invalid publish action result request")
		return
	}

	// Get authenticated user from context (set by middleware)
	authUser := core.GetAuthenticatedUser(ctx)
	if authUser == nil {
		h.renderError(ctx, w, r, core.ErrUnauthorized("authentication required"), "No authenticated user in context")
		return
	}

	// Check permissions - must be GM
	phaseService := h.PhaseService
	canManage, err := phaseService.CanUserManagePhases(ctx, int32(gameID), int32(authUser.ID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to check phase management permission", "error", err)
		return
	}

	if !canManage {
		h.renderError(ctx, w, r, core.ErrForbidden("only the GM can publish action results"), "Publish action result forbidden")
		return
	}

	// Publish the action result
	actionService := h.ActionSubmissionService
	err = actionService.PublishActionResult(ctx, int32(resultID), int32(authUser.ID))
	if err != nil {
		h.renderError(ctx, w, r, core.ErrInternalError(err), "Failed to publish action result", "error", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message":"Action result published successfully"}`))
}
