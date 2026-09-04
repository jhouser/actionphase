package handouts

// Huma (type-first) implementation of the handout API.
//
// Two registration functions, because handouts are mounted at two prefixes:
// the per-game routes under /games/{gameID}, and the cross-game list at
// /handouts. See .claude/planning/huma-migration.md gotcha 10.

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"actionphase/pkg/core"
	"actionphase/pkg/observability"
)

// Input / output types

type gameIDInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
}

type createHandoutInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
	Body   *CreateHandoutRequest
}

type handoutIDInput struct {
	GameID    int32 `path:"gameID" doc:"Game ID"`
	HandoutID int32 `path:"handoutId" doc:"Handout ID"`
}

type updateHandoutInput struct {
	GameID    int32 `path:"gameID" doc:"Game ID"`
	HandoutID int32 `path:"handoutId" doc:"Handout ID"`
	Body      *UpdateHandoutRequest
}

type createCommentInput struct {
	GameID    int32 `path:"gameID" doc:"Game ID"`
	HandoutID int32 `path:"handoutId" doc:"Handout ID"`
	Body      *CreateHandoutCommentRequest
}

type updateCommentInput struct {
	GameID    int32 `path:"gameID" doc:"Game ID"`
	HandoutID int32 `path:"handoutId" doc:"Handout ID"`
	CommentID int32 `path:"commentId" doc:"Comment ID"`
	Body      *UpdateHandoutCommentRequest
}

type deleteCommentInput struct {
	GameID    int32 `path:"gameID" doc:"Game ID"`
	HandoutID int32 `path:"handoutId" doc:"Handout ID"`
	CommentID int32 `path:"commentId" doc:"Comment ID"`
}

type handoutOutput struct {
	Body *HandoutResponse
}

type handoutListOutput struct {
	Body []*HandoutResponse
}

type handoutWithGameListOutput struct {
	Body []*HandoutWithGameResponse
}

type commentOutput struct {
	Body *HandoutCommentResponse
}

type commentListOutput struct {
	Body []*HandoutCommentResponse
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

func (h *Handler) authUser(ctx context.Context) (int32, error) {
	userID, errResp := core.GetUserIDFromJWT(ctx, h.UserService)
	if errResp != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to authenticate user from JWT")
		return 0, humaErr(errResp)
	}
	return userID, nil
}

// loadHandoutAsGM resolves a handout and confirms the caller may manage it.
//
// The lookup runs as the caller, so a player asking about a draft gets the same
// "not found" a nonexistent id would produce, rather than a 403 that confirms
// the draft exists.
func (h *Handler) loadHandoutAsGM(ctx context.Context, handoutID, userID int32) (*core.Handout, error) {
	handout, err := h.HandoutService.GetHandout(ctx, handoutID, userID)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get handout", "error", err, "handout_id", handoutID)
		return nil, huma.Error404NotFound("Handout not found")
	}

	game, err := h.GameService.GetGame(ctx, handout.GameID)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get game", "error", err, "game_id", handout.GameID)
		return nil, huma.Error404NotFound("Handout not found")
	}

	if errResp := h.verifyUserIsGM(ctx, game, userID); errResp != nil {
		return nil, humaErr(errResp)
	}
	return handout, nil
}

// notifyPublished fans out the "handout published" notification off the request
// path, matching the chi handlers: a notification failure must not fail the
// publish that already committed.
func (h *Handler) notifyPublished(handout *core.Handout, userID int32) {
	notifService := h.NotificationService
	observability.SafeGo(context.Background(), h.App.ObsLogger, "notify-handout-published", func() {
		notifCtx := context.Background()
		if err := notifService.NotifyHandoutPublished(notifCtx, handout.GameID, handout.ID, handout.Title, userID); err != nil {
			h.App.ObsLogger.Warn(notifCtx, "Failed to send handout published notifications", "error", err, "handout_id", handout.ID)
		}
	})
}

func toHandoutResponse(handout *core.Handout) *HandoutResponse {
	return &HandoutResponse{
		ID:        handout.ID,
		GameID:    handout.GameID,
		Title:     handout.Title,
		Content:   handout.Content,
		Status:    handout.Status,
		CreatedAt: handout.CreatedAt,
		UpdatedAt: handout.UpdatedAt,
	}
}

func toCommentResponse(comment *core.HandoutComment) *HandoutCommentResponse {
	return &HandoutCommentResponse{
		ID:              comment.ID,
		HandoutID:       comment.HandoutID,
		UserID:          comment.UserID,
		ParentCommentID: comment.ParentCommentID,
		Content:         comment.Content,
		EditCount:       comment.EditCount,
		CreatedAt:       comment.CreatedAt,
		UpdatedAt:       comment.UpdatedAt,
		EditedAt:        comment.EditedAt,
		DeletedAt:       comment.DeletedAt,
		DeletedByUserID: comment.DeletedByUserID,
	}
}

// Handout operations

func (h *Handler) humaCreateHandout(ctx context.Context, in *createHandoutInput) (*handoutOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_create_handout")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	// GameMiddleware resolves this for the whole /games/{gameID} subtree.
	if isGM, _ := ctx.Value("is_gm").(bool); !isGM {
		h.App.ObsLogger.Warn(ctx, "Request rejected in create handout", "user_id", userID, "game_id", in.GameID)
		return nil, huma.Error403Forbidden("Only the GM can create handouts")
	}

	handout, err := h.HandoutService.CreateHandout(ctx, in.GameID, in.Body.Title, in.Body.Content, in.Body.Status, userID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to create handout", "error", err, "game_id", in.GameID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	h.App.ObsLogger.Info(ctx, "Handout created successfully", "handout_id", handout.ID, "game_id", in.GameID)

	// A handout created straight into published state still notifies players.
	if handout.Status == "published" {
		h.notifyPublished(handout, userID)
	}

	return &handoutOutput{Body: toHandoutResponse(handout)}, nil
}

func (h *Handler) humaGetHandout(ctx context.Context, in *handoutIDInput) (*handoutOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_get_handout")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	// The service applies the visibility rules: a player asking for a draft is
	// answered as though it does not exist.
	handout, err := h.HandoutService.GetHandout(ctx, in.HandoutID, userID)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get handout", "error", err, "handout_id", in.HandoutID)
		return nil, huma.Error404NotFound("Handout not found")
	}

	return &handoutOutput{Body: toHandoutResponse(handout)}, nil
}

func (h *Handler) humaListHandouts(ctx context.Context, in *gameIDInput) (*handoutListOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_list_handouts")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	// GMs and Co-GMs additionally see drafts.
	isGM, _ := ctx.Value("is_gm").(bool)

	handouts, err := h.HandoutService.ListHandouts(ctx, in.GameID, userID, isGM)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to list handouts", "error", err, "game_id", in.GameID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	// Built as an empty slice rather than a nil one so a game with no handouts
	// encodes as [] rather than null.
	response := make([]*HandoutResponse, len(handouts))
	for i, handout := range handouts {
		response[i] = toHandoutResponse(handout)
	}

	return &handoutListOutput{Body: response}, nil
}

// humaListHandoutsAcrossGames returns the published handouts of every
// in_progress game the current user takes part in. Unlike humaListHandouts this
// takes no game in the path, so it can back surfaces with no game in scope (the
// global Utility Drawer), and each entry carries its game's title so the client
// can group by game without a request per game.
//
// No membership check is needed here: the query returns only handouts from
// games the user belongs to, and only published ones, so every row is already
// one the caller may read.
func (h *Handler) humaListHandoutsAcrossGames(ctx context.Context, _ *struct{}) (*handoutWithGameListOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_list_handouts_across_games")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	handouts, err := h.HandoutService.ListPublishedHandoutsAcrossGames(ctx, userID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to list handouts across games", "error", err, "user_id", userID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	// Built as an empty slice rather than a nil one so a user with no handouts
	// encodes as [] rather than null.
	response := make([]*HandoutWithGameResponse, len(handouts))
	for i, handout := range handouts {
		response[i] = &HandoutWithGameResponse{
			HandoutResponse: *toHandoutResponse(&handout.Handout),
			GameTitle:       handout.GameTitle,
		}
	}

	return &handoutWithGameListOutput{Body: response}, nil
}

func (h *Handler) humaUpdateHandout(ctx context.Context, in *updateHandoutInput) (*handoutOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_handout")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := h.loadHandoutAsGM(ctx, in.HandoutID, userID); err != nil {
		return nil, err
	}

	handout, err := h.HandoutService.UpdateHandout(ctx, in.HandoutID, in.Body.Title, in.Body.Content, in.Body.Status, userID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to update handout", "error", err, "handout_id", in.HandoutID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	h.App.ObsLogger.Info(ctx, "Handout updated successfully", "handout_id", handout.ID)

	return &handoutOutput{Body: toHandoutResponse(handout)}, nil
}

func (h *Handler) humaDeleteHandout(ctx context.Context, in *handoutIDInput) (*struct{}, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_handout")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := h.loadHandoutAsGM(ctx, in.HandoutID, userID); err != nil {
		return nil, err
	}

	if err := h.HandoutService.DeleteHandout(ctx, in.HandoutID, userID); err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to delete handout", "error", err, "handout_id", in.HandoutID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	h.App.ObsLogger.Info(ctx, "Handout deleted successfully", "handout_id", in.HandoutID)
	return nil, nil
}

func (h *Handler) humaPublishHandout(ctx context.Context, in *handoutIDInput) (*handoutOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_publish_handout")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := h.loadHandoutAsGM(ctx, in.HandoutID, userID); err != nil {
		return nil, err
	}

	handout, err := h.HandoutService.PublishHandout(ctx, in.HandoutID, userID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to publish handout", "error", err, "handout_id", in.HandoutID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	h.App.ObsLogger.Info(ctx, "Handout published successfully", "handout_id", handout.ID)
	h.notifyPublished(handout, userID)

	return &handoutOutput{Body: toHandoutResponse(handout)}, nil
}

func (h *Handler) humaUnpublishHandout(ctx context.Context, in *handoutIDInput) (*handoutOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_unpublish_handout")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := h.loadHandoutAsGM(ctx, in.HandoutID, userID); err != nil {
		return nil, err
	}

	handout, err := h.HandoutService.UnpublishHandout(ctx, in.HandoutID, userID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to unpublish handout", "error", err, "handout_id", in.HandoutID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	h.App.ObsLogger.Info(ctx, "Handout unpublished successfully", "handout_id", handout.ID)

	return &handoutOutput{Body: toHandoutResponse(handout)}, nil
}

// Comment operations

func (h *Handler) humaCreateComment(ctx context.Context, in *createCommentInput) (*commentOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_create_handout_comment")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	handout, err := h.HandoutService.GetHandout(ctx, in.HandoutID, userID)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get handout", "error", err, "handout_id", in.HandoutID)
		return nil, huma.Error404NotFound("Handout or comment not found")
	}

	game, err := h.GameService.GetGame(ctx, handout.GameID)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get game", "error", err, "game_id", handout.GameID)
		return nil, huma.Error404NotFound("Handout or comment not found")
	}

	// Comments on handouts are a GM-only annotation channel. Note this check is
	// GM/Co-GM only -- unlike the manage operations it does not honour admin
	// mode, matching the chi handler it replaces.
	if game.GmUserID != userID && !core.IsUserCoGM(ctx, h.App.Pool, game.ID, userID) {
		h.App.ObsLogger.Warn(ctx, "User is not GM of game", "user_id", userID, "game_id", game.ID)
		return nil, huma.Error401Unauthorized("Only GM can comment on handouts")
	}

	comment, err := h.HandoutService.CreateHandoutComment(ctx, in.HandoutID, userID, in.Body.ParentCommentID, in.Body.Content)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to create comment", "error", err, "handout_id", in.HandoutID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	h.App.ObsLogger.Info(ctx, "Handout comment created successfully", "comment_id", comment.ID, "handout_id", in.HandoutID)

	return &commentOutput{Body: toCommentResponse(comment)}, nil
}

func (h *Handler) humaListComments(ctx context.Context, in *handoutIDInput) (*commentListOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_list_handout_comments")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	// Reading the handout first is the access check: it 404s for a caller who
	// may not see the handout, so comments on a draft stay invisible to players.
	handout, err := h.HandoutService.GetHandout(ctx, in.HandoutID, userID)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Failed to get handout", "error", err, "handout_id", in.HandoutID)
		return nil, huma.Error404NotFound("Handout or comment not found")
	}

	comments, err := h.HandoutService.ListHandoutComments(ctx, in.HandoutID)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to list comments", "error", err, "handout_id", in.HandoutID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	response := make([]*HandoutCommentResponse, len(comments))
	for i, comment := range comments {
		response[i] = toCommentResponse(comment)
	}

	h.App.ObsLogger.Info(ctx, "Listed handout comments", "handout_id", handout.ID, "count", len(comments))

	return &commentListOutput{Body: response}, nil
}

func (h *Handler) humaUpdateComment(ctx context.Context, in *updateCommentInput) (*commentOutput, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_update_handout_comment")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	// Authorship is enforced in the service's UPDATE predicate, not here.
	comment, err := h.HandoutService.UpdateHandoutComment(ctx, in.CommentID, userID, in.Body.Content)
	if err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to update comment", "error", err, "comment_id", in.CommentID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	h.App.ObsLogger.Info(ctx, "Handout comment updated successfully", "comment_id", comment.ID)

	return &commentOutput{Body: toCommentResponse(comment)}, nil
}

func (h *Handler) humaDeleteComment(ctx context.Context, in *deleteCommentInput) (*struct{}, error) {
	defer h.App.ObsLogger.LogOperation(ctx, "api_delete_handout_comment")()

	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	// isGM is passed as true unconditionally: only GMs can create comments in
	// the first place, so the service's own ownership check is what actually
	// guards this. Carried over from the chi handler.
	if err := h.HandoutService.DeleteHandoutComment(ctx, in.CommentID, userID, true); err != nil {
		h.App.ObsLogger.Error(ctx, "Failed to delete comment", "error", err, "comment_id", in.CommentID)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	h.App.ObsLogger.Info(ctx, "Handout comment deleted successfully", "comment_id", in.CommentID)
	return nil, nil
}

// Registration

// RegisterHumaGameHandouts registers the per-game handout and comment
// operations. Paths are relative to the /games/{gameID} subrouter.
func RegisterHumaGameHandouts(api huma.API, h *Handler) {
	bearer := []map[string][]string{{"BearerAuth": {}}}

	huma.Register(api, huma.Operation{
		OperationID:   "createHandout",
		Method:        http.MethodPost,
		Path:          "/handouts",
		Summary:       "Create a handout",
		Description:   "Creates a handout for the game. Creating one directly in published state notifies players.",
		Tags:          []string{"Handouts"},
		Security:      bearer,
		DefaultStatus: http.StatusCreated,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"400": {Description: "Invalid request body"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Only the GM can create handouts"},
		},
	}, h.humaCreateHandout)

	huma.Register(api, huma.Operation{
		OperationID: "listHandouts",
		Method:      http.MethodGet,
		Path:        "/handouts",
		Summary:     "List game handouts",
		Description: "Lists the game's handouts. GMs and Co-GMs also see drafts; everyone else sees only published ones.",
		Tags:        []string{"Handouts"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaListHandouts)

	huma.Register(api, huma.Operation{
		OperationID: "getHandout",
		Method:      http.MethodGet,
		Path:        "/handouts/{handoutId}",
		Summary:     "Get a handout",
		Description: "Returns one handout. A draft is reported as not found to anyone but the GM or Co-GM.",
		Tags:        []string{"Handouts"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated"},
			"404": {Description: "Handout not found, or not visible to the caller"},
		},
	}, h.humaGetHandout)

	huma.Register(api, huma.Operation{
		OperationID: "updateHandout",
		Method:      http.MethodPut,
		Path:        "/handouts/{handoutId}",
		Summary:     "Update a handout",
		Description: "Replaces the handout's title, content and status. GM or Co-GM only.",
		Tags:        []string{"Handouts"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"400": {Description: "Invalid request body"},
			"401": {Description: "Not authenticated, or not the GM"},
			"404": {Description: "Handout not found"},
		},
	}, h.humaUpdateHandout)

	huma.Register(api, huma.Operation{
		OperationID:   "deleteHandout",
		Method:        http.MethodDelete,
		Path:          "/handouts/{handoutId}",
		Summary:       "Delete a handout",
		Description:   "Permanently deletes the handout. GM or Co-GM only.",
		Tags:          []string{"Handouts"},
		Security:      bearer,
		DefaultStatus: http.StatusNoContent,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated, or not the GM"},
			"404": {Description: "Handout not found"},
		},
	}, h.humaDeleteHandout)

	huma.Register(api, huma.Operation{
		OperationID: "publishHandout",
		Method:      http.MethodPost,
		Path:        "/handouts/{handoutId}/publish",
		Summary:     "Publish a handout",
		Description: "Makes a draft handout visible to players and notifies them. GM or Co-GM only.",
		Tags:        []string{"Handouts"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated, or not the GM"},
			"404": {Description: "Handout not found"},
		},
	}, h.humaPublishHandout)

	huma.Register(api, huma.Operation{
		OperationID: "unpublishHandout",
		Method:      http.MethodPost,
		Path:        "/handouts/{handoutId}/unpublish",
		Summary:     "Unpublish a handout",
		Description: "Returns a published handout to draft, hiding it from players. GM or Co-GM only.",
		Tags:        []string{"Handouts"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated, or not the GM"},
			"404": {Description: "Handout not found"},
		},
	}, h.humaUnpublishHandout)

	huma.Register(api, huma.Operation{
		OperationID:   "createHandoutComment",
		Method:        http.MethodPost,
		Path:          "/handouts/{handoutId}/comments",
		Summary:       "Comment on a handout",
		Description:   "Adds a GM-only annotation to a handout. Set parent_comment_id to reply within a thread.",
		Tags:          []string{"Handouts"},
		Security:      bearer,
		DefaultStatus: http.StatusCreated,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"400": {Description: "Invalid request body"},
			"401": {Description: "Not authenticated, or not the GM"},
			"404": {Description: "Handout not found"},
		},
	}, h.humaCreateComment)

	huma.Register(api, huma.Operation{
		OperationID: "listHandoutComments",
		Method:      http.MethodGet,
		Path:        "/handouts/{handoutId}/comments",
		Summary:     "List handout comments",
		Description: "Lists comments on a handout, including soft-deleted ones (marked by deleted_at).",
		Tags:        []string{"Handouts"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated"},
			"404": {Description: "Handout not found, or not visible to the caller"},
		},
	}, h.humaListComments)

	huma.Register(api, huma.Operation{
		OperationID: "updateHandoutComment",
		Method:      http.MethodPatch,
		Path:        "/handouts/{handoutId}/comments/{commentId}",
		Summary:     "Edit a handout comment",
		Description: "Rewrites a comment's content and increments its edit count. Authors only.",
		Tags:        []string{"Handouts"},
		Security:    bearer,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"400": {Description: "Invalid request body"},
			"401": {Description: "Not authenticated"},
		},
	}, h.humaUpdateComment)

	huma.Register(api, huma.Operation{
		OperationID:   "deleteHandoutComment",
		Method:        http.MethodDelete,
		Path:          "/handouts/{handoutId}/comments/{commentId}",
		Summary:       "Delete a handout comment",
		Description:   "Soft-deletes a comment, so replies to it keep their thread.",
		Tags:          []string{"Handouts"},
		Security:      bearer,
		DefaultStatus: http.StatusNoContent,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated"},
		},
	}, h.humaDeleteComment)
}

// RegisterHumaHandouts registers the cross-game handout list.
//
// Paths are relative to the handouts router's mount point (/api/v1/handouts).
func RegisterHumaHandouts(api huma.API, h *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "listHandoutsAcrossGames",
		Method:      http.MethodGet,
		Path:        "/",
		Summary:     "List handouts across all the caller's games",
		Description: "Returns published handouts from every in_progress game the caller takes part in, " +
			"each carrying its game's title so the client can group without a request per game.",
		Tags:     []string{"Handouts"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
		},
	}, h.humaListHandoutsAcrossGames)
}
