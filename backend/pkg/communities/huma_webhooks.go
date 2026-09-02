package communities

// Community Discord webhook endpoints (req 9).
//
// EVERY endpoint here is moderator-gated, with no public read at all -- unlike
// documents, which have a published half anyone may read. There is nothing
// here an ordinary member has any business seeing: the rows carry a channel
// credential, and even the masked form plus delivery status is operational
// detail belonging to the people who run the community.
//
// The URL travels INBOUND ONLY. Responses are built from the service's masked
// converter, and no handler in this file has access to a raw URL -- the
// interface deliberately offers no method that returns one.

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"actionphase/pkg/core"
)

// ---------------------------------------------------------------- I/O types

type webhookListOutput struct {
	Body []*core.CommunityWebhook
}

type webhookOutput struct {
	Body *core.CommunityWebhook
}

type webhookPathInput struct {
	Slug      string `path:"slug" doc:"Community URL slug"`
	WebhookID int32  `path:"webhookID" doc:"Webhook ID"`
}

// createWebhookInput mirrors core.CreateCommunityWebhookRequest rather than
// reusing it, matching the documents pattern: huma derives request validation
// from struct tags, and the core type is a service-layer contract carrying
// none.
type createWebhookInput struct {
	Slug string `path:"slug" doc:"Community URL slug"`
	Body struct {
		// Format and host are NOT enforced by tags -- core.ValidateWebhookURL is
		// the authority, because this is an SSRF control and it must run
		// identically on create, update, and dispatch. A looser tag here would
		// simply be redundant; a stricter one would disagree with the validator
		// and reject URLs the dispatcher would accept.
		URL   string  `json:"url" required:"true" doc:"Discord webhook URL (https://discord.com/api/webhooks/...)"`
		Label *string `json:"label,omitempty" maxLength:"100" doc:"Name for this channel, e.g. #recruitment"`
		// Omitted means enabled: a moderator who just pasted a URL wants it live.
		IsEnabled *bool `json:"is_enabled,omitempty" doc:"Whether this webhook fires; defaults to true"`
		// Omitted means no events -- a webhook that fires for everything by
		// default would spam a channel the first time any game moved.
		Events []string `json:"events,omitempty" doc:"Game states to announce"`
	}
}

// updateWebhookInput is a partial update: an omitted field is left unchanged.
//
// Omitting `url` KEEPS the stored credential. That is what lets the config form
// save a label or event change without ever holding the secret -- the client
// only ever received a mask, and it must not send that mask back as the URL.
type updateWebhookInput struct {
	Slug      string `path:"slug" doc:"Community URL slug"`
	WebhookID int32  `path:"webhookID" doc:"Webhook ID"`
	Body      struct {
		URL       *string  `json:"url,omitempty" doc:"Replacement Discord webhook URL; omit to keep the current one"`
		Label     *string  `json:"label,omitempty" maxLength:"100" doc:"Name for this channel"`
		IsEnabled *bool    `json:"is_enabled,omitempty" doc:"Whether this webhook fires"`
		Events    []string `json:"events,omitempty" doc:"Game states to announce"`
	}
}

type webhookTestOutput struct {
	Body struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
}

// ----------------------------------------------------------------- handlers

// webhookError maps the service's sentinels onto HTTP.
//
// Not-found answers 404 for a webhook in another community as well as one that
// does not exist -- 403 would confirm the row exists and let a moderator of one
// community probe another's configuration.
func webhookError(err error) error {
	switch {
	case errors.Is(err, core.ErrCommunityWebhookNotFound):
		return huma.Error404NotFound("webhook not found")
	case errors.Is(err, core.ErrWebhookURLRequired):
		return huma.Error400BadRequest("a webhook URL is required")
	case errors.Is(err, core.ErrInvalidWebhookURL):
		return huma.Error400BadRequest(
			"webhook URL must be an https Discord webhook endpoint")
	case errors.Is(err, core.ErrInvalidWebhookEvent):
		return huma.Error400BadRequest("events must be valid game states")
	default:
		return nil
	}
}

// humaListWebhooks returns a community's webhooks with MASKED URLs.
//
// Moderator-only, and includes disabled rows: the point of the screen is to
// repair or re-enable them.
func (h *Handler) humaListWebhooks(ctx context.Context, in *communitySlugInput) (*webhookListOutput, error) {
	community, _, err := h.requireModerator(ctx, in.Slug)
	if err != nil {
		return nil, err
	}

	hooks, err := h.CommunityService.ListWebhooks(ctx, community.ID)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Failed to list community webhooks",
			"community_id", community.ID)
		return nil, huma.Error500InternalServerError("Failed to list webhooks")
	}
	return &webhookListOutput{Body: hooks}, nil
}

// humaCreateWebhook registers a webhook. Moderator-only.
func (h *Handler) humaCreateWebhook(ctx context.Context, in *createWebhookInput) (*webhookOutput, error) {
	community, _, err := h.requireModerator(ctx, in.Slug)
	if err != nil {
		return nil, err
	}

	hook, err := h.CommunityService.CreateWebhook(ctx, community.ID,
		&core.CreateCommunityWebhookRequest{
			URL:       in.Body.URL,
			Label:     in.Body.Label,
			IsEnabled: in.Body.IsEnabled,
			Events:    in.Body.Events,
		})
	if err != nil {
		if mapped := webhookError(err); mapped != nil {
			return nil, mapped
		}
		// No URL in the log: it is a credential.
		h.App.ObsLogger.LogError(ctx, err, "Failed to create community webhook",
			"community_id", community.ID)
		return nil, huma.Error500InternalServerError("Failed to create webhook")
	}
	return &webhookOutput{Body: hook}, nil
}

// humaUpdateWebhook applies a partial update. Moderator-only.
func (h *Handler) humaUpdateWebhook(ctx context.Context, in *updateWebhookInput) (*webhookOutput, error) {
	community, _, err := h.requireModerator(ctx, in.Slug)
	if err != nil {
		return nil, err
	}

	hook, err := h.CommunityService.UpdateWebhook(ctx, community.ID, in.WebhookID,
		&core.UpdateCommunityWebhookRequest{
			URL:       in.Body.URL,
			Label:     in.Body.Label,
			IsEnabled: in.Body.IsEnabled,
			Events:    in.Body.Events,
		})
	if err != nil {
		if mapped := webhookError(err); mapped != nil {
			return nil, mapped
		}
		h.App.ObsLogger.LogError(ctx, err, "Failed to update community webhook",
			"community_id", community.ID, "webhook_id", in.WebhookID)
		return nil, huma.Error500InternalServerError("Failed to update webhook")
	}
	return &webhookOutput{Body: hook}, nil
}

// humaDeleteWebhook removes a webhook. Moderator-only.
func (h *Handler) humaDeleteWebhook(ctx context.Context, in *webhookPathInput) (*struct{}, error) {
	community, _, err := h.requireModerator(ctx, in.Slug)
	if err != nil {
		return nil, err
	}

	if err := h.CommunityService.DeleteWebhook(ctx, community.ID, in.WebhookID); err != nil {
		if mapped := webhookError(err); mapped != nil {
			return nil, mapped
		}
		h.App.ObsLogger.LogError(ctx, err, "Failed to delete community webhook",
			"community_id", community.ID, "webhook_id", in.WebhookID)
		return nil, huma.Error500InternalServerError("Failed to delete webhook")
	}
	return nil, nil
}

// humaTestWebhook sends a test embed and reports the result SYNCHRONOUSLY.
//
// Synchronous unlike state-change dispatch, and deliberately so: the moderator
// clicked this button and is waiting for the answer, which is the entire point
// of the endpoint. It is also why a failure here returns 502 with Discord's
// reason rather than being swallowed -- the diagnosis IS the response.
func (h *Handler) humaTestWebhook(ctx context.Context, in *webhookPathInput) (*webhookTestOutput, error) {
	community, _, err := h.requireModerator(ctx, in.Slug)
	if err != nil {
		return nil, err
	}

	if h.WebhookSender == nil {
		return nil, huma.Error503ServiceUnavailable(
			"Discord webhook delivery is not configured on this server")
	}

	if err := h.CommunityService.TestWebhook(ctx, community.ID, in.WebhookID, h.WebhookSender); err != nil {
		if mapped := webhookError(err); mapped != nil {
			return nil, mapped
		}
		// 502, not 500: the failure is Discord's, and the moderator needs the
		// reason to fix their configuration. The transport guarantees this
		// message carries no URL.
		return nil, huma.Error502BadGateway(err.Error())
	}

	out := &webhookTestOutput{}
	out.Body.Success = true
	out.Body.Message = "Test message delivered to Discord"
	return out, nil
}

// ------------------------------------------------------------ registration

// RegisterHumaCommunityWebhooks wires the webhook endpoints onto the
// communities group.
func RegisterHumaCommunityWebhooks(api huma.API, h *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "listCommunityWebhooks",
		Method:      http.MethodGet,
		Path:        "/{slug}/webhooks",
		Summary:     "List a community's Discord webhooks",
		Description: "Requires moderation rights. URLs are MASKED -- the full webhook URL is a " +
			"credential and is never returned. Disabled webhooks are included, since " +
			"re-enabling them is the point of the screen.",
		Tags:     []string{"Communities"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not a moderator of this community"},
			"404": {Description: "Community not found"},
		},
	}, h.humaListWebhooks)

	huma.Register(api, huma.Operation{
		OperationID:   "createCommunityWebhook",
		Method:        http.MethodPost,
		Path:          "/{slug}/webhooks",
		DefaultStatus: http.StatusCreated,
		Summary:       "Add a Discord webhook to a community",
		Description: "Requires moderation rights. The URL must be an https Discord webhook " +
			"endpoint; anything else is rejected, since the server makes outbound " +
			"requests to it. The response masks the URL.",
		Tags:     []string{"Communities"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"400": {Description: "URL is not a Discord webhook, or an event is not a game state"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not a moderator of this community"},
			"404": {Description: "Community not found"},
		},
	}, h.humaCreateWebhook)

	huma.Register(api, huma.Operation{
		OperationID: "updateCommunityWebhook",
		Method:      http.MethodPatch,
		Path:        "/{slug}/webhooks/{webhookID}",
		Summary:     "Edit a Discord webhook",
		Description: "Requires moderation rights. Omitted fields are unchanged; omitting the " +
			"URL keeps the stored credential, so the config form can save a label or " +
			"event change without ever holding the secret.",
		Tags:     []string{"Communities"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"400": {Description: "URL is not a Discord webhook, or an event is not a game state"},
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not a moderator of this community"},
			"404": {Description: "Community or webhook not found"},
		},
	}, h.humaUpdateWebhook)

	huma.Register(api, huma.Operation{
		OperationID:   "deleteCommunityWebhook",
		Method:        http.MethodDelete,
		Path:          "/{slug}/webhooks/{webhookID}",
		DefaultStatus: http.StatusNoContent,
		Summary:       "Remove a Discord webhook",
		Description:   "Requires moderation rights.",
		Tags:          []string{"Communities"},
		Security:      []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not a moderator of this community"},
			"404": {Description: "Community or webhook not found"},
		},
	}, h.humaDeleteWebhook)

	huma.Register(api, huma.Operation{
		OperationID: "testCommunityWebhook",
		Method:      http.MethodPost,
		Path:        "/{slug}/webhooks/{webhookID}/test",
		Summary:     "Send a test message to a Discord webhook",
		Description: "Requires moderation rights. Sends SYNCHRONOUSLY and reports the outcome -- " +
			"unlike game-state announcements, which are fire-and-forget. Stamps the " +
			"same delivery status columns as a real delivery.",
		Tags:     []string{"Communities"},
		Security: []map[string][]string{{"BearerAuth": {}}},
		Responses: map[string]*huma.Response{
			"401": {Description: "Not authenticated"},
			"403": {Description: "Not a moderator of this community"},
			"404": {Description: "Community or webhook not found"},
			"502": {Description: "Discord rejected the message; the reason is in the error"},
			"503": {Description: "Webhook delivery is not configured on this server"},
		},
	}, h.humaTestWebhook)
}
