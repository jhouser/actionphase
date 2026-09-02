package communities

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
)

// Community Discord webhooks (req 9) -- configuration CRUD.
//
// Delivery lives elsewhere (services/webhook_dispatch.go, hung off
// GameService.UpdateGameState); this file only manages the rows.
//
// Two invariants hold throughout:
//
//  1. THE URL IS A CREDENTIAL AND NEVER LEAVES HERE UNMASKED. Every return path
//     goes through webhookFromDB, which masks unconditionally. Nothing in this
//     file returns a raw models.CommunityWebhook.
//
//  2. EVERY LOOKUP IS COMMUNITY-SCOPED. As with documents, a bare id would
//     resolve a webhook in a different community than the request path names --
//     here allowing a moderator of A to read or overwrite B's channel
//     credential. The queries take (id, community_id) so that misses instead.

// validateWebhookEvents rejects any event that is not a notifiable game state.
//
// A nil slice is valid and means "no events": a webhook that fires for nothing
// is a moderator staging a configuration, not an error. An unrecognised event
// IS an error rather than being silently dropped -- a typo'd "in progress" that
// quietly vanished would look like the webhook was simply broken.
func validateWebhookEvents(events []string) ([]string, error) {
	if events == nil {
		return nil, nil
	}

	out := make([]string, 0, len(events))
	for _, e := range events {
		trimmed := strings.TrimSpace(e)
		if !core.IsValidWebhookEvent(trimmed) {
			return nil, fmt.Errorf("%w: %q", core.ErrInvalidWebhookEvent, e)
		}
		out = append(out, trimmed)
	}
	return out, nil
}

// CreateWebhook registers a Discord webhook on a community.
//
// The URL is validated before it is stored. That check is an SSRF control, not
// a formatting preference -- see core.ValidateWebhookURL.
func (s *CommunityService) CreateWebhook(
	ctx context.Context, communityID int32, req *core.CreateCommunityWebhookRequest,
) (*core.CommunityWebhook, error) {
	if req == nil {
		return nil, fmt.Errorf("webhook request is required")
	}

	url := strings.TrimSpace(req.URL)
	if err := core.ValidateWebhookURL(url); err != nil {
		return nil, err
	}

	events, err := validateWebhookEvents(req.Events)
	if err != nil {
		return nil, err
	}
	if events == nil {
		// The column is NOT NULL; an absent list is an empty one, not a NULL.
		events = []string{}
	}

	// Absent means ENABLED: a moderator who just pasted a URL and picked events
	// wants it live. The disabled state is for switching an existing webhook
	// off, not for staging a new one.
	isEnabled := true
	if req.IsEnabled != nil {
		isEnabled = *req.IsEnabled
	}

	params := models.CreateCommunityWebhookParams{
		CommunityID: communityID,
		Url:         url,
		IsEnabled:   isEnabled,
		Events:      events,
	}
	if req.Label != nil {
		params.Label = pgtype.Text{String: strings.TrimSpace(*req.Label), Valid: true}
	}

	row, err := models.New(s.DB).CreateCommunityWebhook(ctx, params)
	if err != nil {
		// No URL in the log line: it is a credential.
		s.Logger.LogError(ctx, err, "Failed to create community webhook",
			"community_id", communityID)
		return nil, fmt.Errorf("create community webhook: %w", err)
	}

	s.Logger.Info(ctx, "Community webhook created",
		"community_id", communityID,
		"webhook_id", row.ID,
		"is_enabled", row.IsEnabled,
		"event_count", len(row.Events),
	)

	return webhookFromDB(row), nil
}

// ListWebhooks returns a community's webhooks, DISABLED ROWS INCLUDED, with
// every URL masked.
//
// Moderator-only: callers must gate on CanModerateCommunity. Disabled rows are
// included because the whole point of the screen is to repair or re-enable
// them.
func (s *CommunityService) ListWebhooks(
	ctx context.Context, communityID int32,
) ([]*core.CommunityWebhook, error) {
	rows, err := models.New(s.DB).ListCommunityWebhooks(ctx, communityID)
	if err != nil {
		s.Logger.LogError(ctx, err, "Failed to list community webhooks",
			"community_id", communityID)
		return nil, fmt.Errorf("list community webhooks: %w", err)
	}

	out := make([]*core.CommunityWebhook, 0, len(rows))
	for _, row := range rows {
		out = append(out, webhookFromDB(row))
	}
	return out, nil
}

// GetWebhook reads one webhook, scoped to its community, with a masked URL.
func (s *CommunityService) GetWebhook(
	ctx context.Context, communityID, webhookID int32,
) (*core.CommunityWebhook, error) {
	row, err := models.New(s.DB).GetCommunityWebhook(ctx, models.GetCommunityWebhookParams{
		ID:          webhookID,
		CommunityID: communityID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrCommunityWebhookNotFound
		}
		return nil, fmt.Errorf("get community webhook: %w", err)
	}
	return webhookFromDB(row), nil
}

// UpdateWebhook applies a partial update. A nil field is left unchanged.
//
// Omitting the URL keeps the stored credential, which is what lets the config
// form save a label or event change WITHOUT the client ever holding the secret
// -- it only ever received a mask, and echoing that mask back must not
// overwrite the real URL with bullet characters. A supplied URL is validated
// exactly as at create.
func (s *CommunityService) UpdateWebhook(
	ctx context.Context, communityID, webhookID int32, req *core.UpdateCommunityWebhookRequest,
) (*core.CommunityWebhook, error) {
	if req == nil {
		return nil, fmt.Errorf("webhook request is required")
	}

	params := models.UpdateCommunityWebhookParams{
		ID:          webhookID,
		CommunityID: communityID,
	}

	if req.URL != nil {
		url := strings.TrimSpace(*req.URL)
		if err := core.ValidateWebhookURL(url); err != nil {
			return nil, err
		}
		params.Url = pgtype.Text{String: url, Valid: true}
	}
	if req.Label != nil {
		params.Label = pgtype.Text{String: strings.TrimSpace(*req.Label), Valid: true}
	}
	if req.IsEnabled != nil {
		params.IsEnabled = pgtype.Bool{Bool: *req.IsEnabled, Valid: true}
	}
	if req.Events != nil {
		events, err := validateWebhookEvents(req.Events)
		if err != nil {
			return nil, err
		}
		params.Events = events
	}

	row, err := models.New(s.DB).UpdateCommunityWebhook(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrCommunityWebhookNotFound
		}
		s.Logger.LogError(ctx, err, "Failed to update community webhook",
			"community_id", communityID, "webhook_id", webhookID)
		return nil, fmt.Errorf("update community webhook: %w", err)
	}

	s.Logger.Info(ctx, "Community webhook updated",
		"community_id", communityID,
		"webhook_id", webhookID,
		"is_enabled", row.IsEnabled,
		"url_rotated", req.URL != nil,
	)

	return webhookFromDB(row), nil
}

// DeleteWebhook removes a webhook from a community.
//
// A missing webhook is an ERROR, matching DeleteDocument: a delete that quietly
// matched nothing -- wrong id, wrong community -- would report success while
// leaving a live webhook posting to a channel the moderator believes they just
// disconnected.
func (s *CommunityService) DeleteWebhook(ctx context.Context, communityID, webhookID int32) error {
	// Read first: DeleteCommunityWebhook is :exec and reports no row count, so a
	// missing row is otherwise indistinguishable from a successful delete.
	if _, err := s.GetWebhook(ctx, communityID, webhookID); err != nil {
		return err
	}

	if err := models.New(s.DB).DeleteCommunityWebhook(ctx, models.DeleteCommunityWebhookParams{
		ID:          webhookID,
		CommunityID: communityID,
	}); err != nil {
		s.Logger.LogError(ctx, err, "Failed to delete community webhook",
			"community_id", communityID, "webhook_id", webhookID)
		return fmt.Errorf("delete community webhook: %w", err)
	}

	s.Logger.Info(ctx, "Community webhook deleted",
		"community_id", communityID, "webhook_id", webhookID)
	return nil
}

// TestWebhook sends a test embed SYNCHRONOUSLY and reports the result.
//
// Synchronous on purpose, unlike state-change dispatch: a moderator who clicks
// "send test" is waiting for the answer, and that answer IS the feature. The
// asynchronous path exists so a slow Discord never lands on a GM's transition;
// nobody else is waiting on this one.
//
// Stamps the same last_success_at / last_error columns as a real delivery, so a
// test that fails leaves the same diagnosis behind as a real failure.
func (s *CommunityService) TestWebhook(
	ctx context.Context, communityID, webhookID int32, sender core.DiscordWebhookSender,
) error {
	if sender == nil {
		return fmt.Errorf("webhook sender is not configured")
	}

	// Read the RAW row: the sender needs the real URL, which is exactly why this
	// bypasses webhookFromDB rather than reusing GetWebhook.
	row, err := models.New(s.DB).GetCommunityWebhook(ctx, models.GetCommunityWebhookParams{
		ID:          webhookID,
		CommunityID: communityID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.ErrCommunityWebhookNotFound
		}
		return fmt.Errorf("get community webhook: %w", err)
	}

	embed := core.DiscordEmbed{
		Title:       "ActionPhase test message",
		Description: "This channel is connected. Game state changes will appear here.",
		Color:       0x5865F2,
		Footer:      "ActionPhase",
	}

	queries := models.New(s.DB)
	if sendErr := sender.Send(ctx, row.Url, embed); sendErr != nil {
		msg := sendErr.Error()
		const maxStoredError = 500
		if len(msg) > maxStoredError {
			msg = msg[:maxStoredError]
		}
		if err := queries.MarkCommunityWebhookError(ctx, models.MarkCommunityWebhookErrorParams{
			ID:        webhookID,
			LastError: pgtype.Text{String: msg, Valid: true},
		}); err != nil {
			s.Logger.LogError(ctx, err, "Failed to stamp webhook test error", "webhook_id", webhookID)
		}
		// Returned to the moderator, who needs Discord's reason to fix it. The
		// transport guarantees this string carries no URL.
		return fmt.Errorf("webhook test failed: %w", sendErr)
	}

	if err := queries.MarkCommunityWebhookSuccess(ctx, webhookID); err != nil {
		s.Logger.LogError(ctx, err, "Failed to stamp webhook test success", "webhook_id", webhookID)
	}

	s.Logger.Info(ctx, "Community webhook test succeeded",
		"community_id", communityID, "webhook_id", webhookID)
	return nil
}
