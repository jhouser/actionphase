package communities

// Community document endpoints (req 7, 8) -- a community's rules and reference
// pages.
//
// Authorization here has TWO distinct shapes, and conflating them is the main
// hazard in this file:
//
//   - Writes and the draft-inclusive list are gated on COMMUNITY MODERATION.
//   - Reads of PUBLISHED documents are open to any authenticated user, because
//     a community's rules are what a prospective member reads before joining.
//     Gating those on membership would hide the rules from exactly the person
//     they exist to inform -- and membership is open anyway, so there is
//     nothing to be a member of.
//
// The one game-scoped endpoint is gated on GAME visibility instead: a player
// reads their game's community rules from the Info tab without moderating that
// community.

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"actionphase/pkg/core"
)

// ---------------------------------------------------------------- I/O types

type documentListOutput struct {
	Body []*core.CommunityDocument
}

type documentOutput struct {
	Body *core.CommunityDocument
}

type documentPathInput struct {
	Slug       string `path:"slug" doc:"Community URL slug"`
	DocumentID int32  `path:"documentID" doc:"Document ID"`
}

// createDocumentInput mirrors core.CreateCommunityDocumentRequest rather than
// reusing it: huma derives request validation from struct tags, and the core
// type is a service-layer contract carrying none.
type createDocumentInput struct {
	Slug string `path:"slug" doc:"Community URL slug"`
	Body struct {
		Title string `json:"title" required:"true" minLength:"1" maxLength:"255" doc:"Document title"`
		// No minLength: a moderator may create the page first and write it
		// later, which is what draft status is for.
		Content string `json:"content" doc:"Document body (markdown)"`
		// Omitted means draft -- the default that shows the document to nobody
		// until its author says otherwise.
		Status    *string `json:"status,omitempty" enum:"draft,published" doc:"Visibility; omit for draft"`
		SortOrder *int32  `json:"sort_order,omitempty" doc:"Display position, lowest first"`
	}
}

// updateDocumentInput is a partial update: an omitted field is left unchanged.
//
// Content is deliberately NOT tri-state, unlike a community description: the
// column is NOT NULL, so "" is a blank page rather than a clear.
type updateDocumentInput struct {
	Slug       string `path:"slug" doc:"Community URL slug"`
	DocumentID int32  `path:"documentID" doc:"Document ID"`
	Body       struct {
		Title     *string `json:"title,omitempty" minLength:"1" maxLength:"255" doc:"Document title"`
		Content   *string `json:"content,omitempty" doc:"Document body (markdown)"`
		Status    *string `json:"status,omitempty" enum:"draft,published" doc:"Visibility"`
		SortOrder *int32  `json:"sort_order,omitempty" doc:"Display position, lowest first"`
	}
}

type gameDocumentsInput struct {
	GameID int32 `path:"gameID" doc:"Game ID"`
}

// ----------------------------------------------------------------- handlers

// documentError maps the service's sentinels onto HTTP.
//
// ErrCommunityDocumentNotFound covers both "no such document" and "belongs to
// another community" -- answering 404 rather than 403 for the latter is
// deliberate, since confirming a document exists elsewhere would leak the
// existence of another community's drafts.
func documentError(err error) error {
	switch {
	case errors.Is(err, core.ErrCommunityDocumentNotFound):
		return huma.Error404NotFound("document not found")
	case errors.Is(err, core.ErrInvalidDocumentStatus):
		return huma.Error400BadRequest("status must be draft or published")
	default:
		return nil
	}
}

// humaListDocuments returns a community's PUBLISHED documents.
//
// Open to any authenticated user: rules are what someone reads before deciding
// to join. Moderators use the manage endpoint below to see drafts.
func (h *Handler) humaListDocuments(ctx context.Context, in *communitySlugInput) (*documentListOutput, error) {
	if _, err := h.authUser(ctx); err != nil {
		return nil, err
	}

	community, err := h.loadCommunity(ctx, in.Slug)
	if err != nil {
		return nil, err
	}

	docs, err := h.CommunityService.ListPublishedDocuments(ctx, community.ID)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to list community documents",
			"community_id", community.ID)
		return nil, huma.Error500InternalServerError("Failed to list documents")
	}
	return &documentListOutput{Body: docs}, nil
}

// humaListAllDocuments returns every document INCLUDING DRAFTS.
//
// A separate endpoint from the public list rather than a query parameter on it:
// a `?drafts=true` flag would put the visibility rule in the caller's hands,
// and one handler forgetting to check moderation would leak every draft. Here
// the privileged read is a different path with its own gate.
func (h *Handler) humaListAllDocuments(ctx context.Context, in *communitySlugInput) (*documentListOutput, error) {
	community, _, err := h.requireModerator(ctx, in.Slug)
	if err != nil {
		return nil, err
	}

	docs, err := h.CommunityService.ListDocuments(ctx, community.ID)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to list all community documents",
			"community_id", community.ID)
		return nil, huma.Error500InternalServerError("Failed to list documents")
	}
	return &documentListOutput{Body: docs}, nil
}

// humaGetDocument returns one document.
//
// A DRAFT is visible only to a moderator. The service read is deliberately
// status-blind -- it serves both the editor and the public reader -- so the
// visibility rule is applied HERE, and it is the only thing standing between a
// draft and an ordinary user who guesses an id.
//
// A hidden draft answers 404, not 403: 403 would confirm the document exists
// and let an outsider enumerate a community's unpublished work.
func (h *Handler) humaGetDocument(ctx context.Context, in *documentPathInput) (*documentOutput, error) {
	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	community, err := h.loadCommunity(ctx, in.Slug)
	if err != nil {
		return nil, err
	}

	doc, err := h.CommunityService.GetDocument(ctx, community.ID, in.DocumentID)
	if err != nil {
		if mapped := documentError(err); mapped != nil {
			return nil, mapped
		}
		h.App.ObsLogger.LogError(ctx, err, "Failed to get community document",
			"community_id", community.ID, "document_id", in.DocumentID)
		return nil, huma.Error500InternalServerError("Failed to get document")
	}

	if doc.Status != core.DocumentStatusPublished {
		if !core.CanModerateCommunity(ctx, h.App.Pool, community.ID, userID, h.isSiteAdmin(ctx, userID)) {
			return nil, huma.Error404NotFound("document not found")
		}
	}

	return &documentOutput{Body: doc}, nil
}

// humaCreateDocument adds a document. Moderator-only.
func (h *Handler) humaCreateDocument(ctx context.Context, in *createDocumentInput) (*documentOutput, error) {
	community, userID, err := h.requireModerator(ctx, in.Slug)
	if err != nil {
		return nil, err
	}

	doc, err := h.CommunityService.CreateDocument(ctx, community.ID, userID,
		&core.CreateCommunityDocumentRequest{
			Title:     in.Body.Title,
			Content:   in.Body.Content,
			Status:    in.Body.Status,
			SortOrder: in.Body.SortOrder,
		})
	if err != nil {
		if mapped := documentError(err); mapped != nil {
			return nil, mapped
		}
		h.App.ObsLogger.LogError(ctx, err, "Failed to create community document",
			"community_id", community.ID)
		return nil, huma.Error500InternalServerError("Failed to create document")
	}

	return &documentOutput{Body: doc}, nil
}

// humaUpdateDocument applies a partial update. Moderator-only.
//
// Publishing and unpublishing both run through here rather than through
// dedicated endpoints, because a document's status sits on the same form as its
// body -- fixing a typo and publishing is one request, not two.
func (h *Handler) humaUpdateDocument(ctx context.Context, in *updateDocumentInput) (*documentOutput, error) {
	community, _, err := h.requireModerator(ctx, in.Slug)
	if err != nil {
		return nil, err
	}

	doc, err := h.CommunityService.UpdateDocument(ctx, community.ID, in.DocumentID,
		&core.UpdateCommunityDocumentRequest{
			Title:     in.Body.Title,
			Content:   in.Body.Content,
			Status:    in.Body.Status,
			SortOrder: in.Body.SortOrder,
		})
	if err != nil {
		if mapped := documentError(err); mapped != nil {
			return nil, mapped
		}
		h.App.ObsLogger.LogError(ctx, err, "Failed to update community document",
			"community_id", community.ID, "document_id", in.DocumentID)
		return nil, huma.Error500InternalServerError("Failed to update document")
	}

	return &documentOutput{Body: doc}, nil
}

// humaDeleteDocument removes a document. Moderator-only.
func (h *Handler) humaDeleteDocument(ctx context.Context, in *documentPathInput) (*struct{}, error) {
	community, _, err := h.requireModerator(ctx, in.Slug)
	if err != nil {
		return nil, err
	}

	if err := h.CommunityService.DeleteDocument(ctx, community.ID, in.DocumentID); err != nil {
		if mapped := documentError(err); mapped != nil {
			return nil, mapped
		}
		h.App.ObsLogger.LogError(ctx, err, "Failed to delete community document",
			"community_id", community.ID, "document_id", in.DocumentID)
		return nil, huma.Error500InternalServerError("Failed to delete document")
	}

	return nil, nil
}

// humaListGameCommunityDocuments backs the Game Info tab (req 8).
//
// Gated on GAME visibility rather than community standing: a player reading
// their own game's Info tab is not necessarily anything in that community.
//
// A legacy game with no community returns an EMPTY LIST, not a 404 -- the
// grandfathering lives in the SQL join, so the tab renders no community section
// and nothing has to special-case it.
func (h *Handler) humaListGameCommunityDocuments(ctx context.Context, in *gameDocumentsInput) (*documentListOutput, error) {
	userID, err := h.authUser(ctx)
	if err != nil {
		return nil, err
	}

	canView, err := h.GameService.CanUserViewGame(ctx, in.GameID, userID)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to check game visibility",
			"game_id", in.GameID, "user_id", userID)
		return nil, huma.Error500InternalServerError("Failed to check game access")
	}
	if !canView {
		return nil, huma.Error403Forbidden("you do not have access to this game")
	}

	docs, err := h.CommunityService.ListPublishedDocumentsForGame(ctx, in.GameID)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to list community documents for game",
			"game_id", in.GameID)
		return nil, huma.Error500InternalServerError("Failed to list documents")
	}
	return &documentListOutput{Body: docs}, nil
}

// -------------------------------------------------------------- registration

// RegisterHumaCommunityDocuments registers the document routes, relative to the
// /api/v1/communities mount.
func RegisterHumaCommunityDocuments(api huma.API, h *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "listCommunityDocuments",
		Method:      http.MethodGet,
		Path:        "/{slug}/documents",
		Summary:     "List a community's published documents",
		Description: "Open to any authenticated user -- a community's rules are what someone " +
			"reads before deciding to join. Drafts are omitted; moderators use the manage " +
			"endpoint to see them.",
		Tags:     []string{"Communities"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated"},
			"404": {Description: "Community not found"},
		},
	}, h.humaListDocuments)

	huma.Register(api, huma.Operation{
		OperationID: "listAllCommunityDocuments",
		Method:      http.MethodGet,
		Path:        "/{slug}/documents/manage",
		Summary:     "List every document including drafts",
		Description: "Requires moderation rights. A separate path from the public list rather " +
			"than a query flag on it, so the privileged read carries its own gate.",
		Tags:     []string{"Communities"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Caller does not moderate this community"},
			"404": {Description: "Community not found"},
		},
	}, h.humaListAllDocuments)

	huma.Register(api, huma.Operation{
		OperationID: "getCommunityDocument",
		Method:      http.MethodGet,
		Path:        "/{slug}/documents/{documentID}",
		Summary:     "Read one community document",
		Description: "Published documents are readable by any authenticated user. A DRAFT is " +
			"visible only to moderators, and answers 404 rather than 403 for everyone else " +
			"so unpublished work cannot be enumerated.",
		Tags:     []string{"Communities"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated"},
			"404": {Description: "Community or document not found"},
		},
	}, h.humaGetDocument)

	huma.Register(api, huma.Operation{
		OperationID: "createCommunityDocument",
		Method:      http.MethodPost,
		Path:        "/{slug}/documents",
		Summary:     "Create a community document",
		Description: "Requires moderation rights. Omit status to create a draft, which is the " +
			"default so a half-written page binds nobody.",
		Tags:          []string{"Communities"},
		Security:      []map[string][]string{{"BearerAuth": {}}},
		DefaultStatus: http.StatusCreated,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"400": {Description: "Title is blank or the status is unrecognised"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Caller does not moderate this community"},
			"404": {Description: "Community not found"},
		},
	}, h.humaCreateDocument)

	huma.Register(api, huma.Operation{
		OperationID: "updateCommunityDocument",
		Method:      http.MethodPatch,
		Path:        "/{slug}/documents/{documentID}",
		Summary:     "Edit a community document",
		Description: "Requires moderation rights. Publishing and unpublishing happen here " +
			"rather than through dedicated endpoints, since status sits on the same form as " +
			"the body. An omitted field is left unchanged; an empty content is a blank page.",
		Tags:     []string{"Communities"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"400": {Description: "Title is blank or the status is unrecognised"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Caller does not moderate this community"},
			"404": {Description: "Community or document not found"},
		},
	}, h.humaUpdateDocument)

	huma.Register(api, huma.Operation{
		OperationID:   "deleteCommunityDocument",
		Method:        http.MethodDelete,
		Path:          "/{slug}/documents/{documentID}",
		Summary:       "Delete a community document",
		Description:   "Requires moderation rights. Deleting a document that does not exist answers 404 rather than succeeding, so a delete that silently matched nothing surfaces.",
		Tags:          []string{"Communities"},
		Security:      []map[string][]string{{"BearerAuth": {}}},
		DefaultStatus: http.StatusNoContent,
		Responses: map[string]*huma.Response{
			"422": {Description: "Request failed validation"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Caller does not moderate this community"},
			"404": {Description: "Community or document not found"},
		},
	}, h.humaDeleteDocument)
}

// RegisterHumaGameCommunityDocuments registers the Game Info tab's document
// list on the shared /games/{gameID} subrouter (req 8).
//
// Separate from RegisterHumaCommunityDocuments because it mounts elsewhere and
// is gated differently: game visibility, not community moderation.
func RegisterHumaGameCommunityDocuments(api huma.API, h *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "listGameCommunityDocuments",
		Method:      http.MethodGet,
		Path:        "/community-documents",
		Summary:     "List the published documents of the game's community",
		Description: "Backs the Info tab. Requires access to the GAME, not standing in the " +
			"community. A game with no community returns an empty list rather than 404, so " +
			"legacy games simply render no community section.",
		Tags:     []string{"Games", "Communities"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "Caller cannot view this game"},
		},
	}, h.humaListGameCommunityDocuments)
}
