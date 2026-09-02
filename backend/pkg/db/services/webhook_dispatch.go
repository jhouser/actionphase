package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	"actionphase/pkg/discord"
	"actionphase/pkg/observability"
)

// Community Discord webhook dispatch (req 9).
//
// Best-effort by design: no queue, no delivery table, no redelivery after a
// restart. Two properties are NOT optional, and both are easy to break in ways
// no happy-path test notices:
//
//  1. DISPATCH MUST NOT USE THE REQUEST CONTEXT. A bare `go func()` closing over
//     the handler's ctx is cancelled the moment the HTTP response is written, so
//     deliveries would fail nondeterministically under load and pass in every
//     test. A fresh background context carries the correlation ID forward so the
//     detached work is still traceable to the transition that caused it.
//
//  2. A WEBHOOK FAILURE MUST NEVER FAIL OR BLOCK THE TRANSITION. The game state
//     has already changed and been committed by the time we get here. Discord
//     being down is not a reason to report a failed state change to the GM.

// appWebhookSender is the application-wide webhook sender, set once by main.
//
// Exists for the same reason as appDiscordNotifier: GameService is constructed
// as a bare struct literal in dozens of places across the routing layer, and a
// dependency that must be threaded through every one of them is a dependency
// that will be missing from some of them. A missing sender does not fail
// loudly -- dispatch just silently does nothing -- so the failure mode is a
// feature that works on some routes and not others, with nothing to see.
//
// A GameService with an explicit WebhookNotifier keeps it; this is the fallback
// for the literals that set no notifier at all.
var appWebhookSender core.DiscordWebhookSender

// SetAppWebhookSender registers the application-wide webhook sender.
// Call once from main.go at startup.
func SetAppWebhookSender(s core.DiscordWebhookSender) {
	appWebhookSender = s
}

// AppWebhookSender returns the application-wide sender registered by main.
//
// Used by the routing layer to hand the same sender to the communities handler
// for its synchronous test button. A getter rather than a second registry, so
// there is exactly one answer to "which sender is configured?" -- returns nil
// when Discord is not wired up, which the test endpoint reports as 503.
func AppWebhookSender() core.DiscordWebhookSender {
	return appWebhookSender
}

// webhookSender resolves the sender for this service: an explicitly-set one
// wins, otherwise the application-wide one.
func (gs *GameService) webhookSender() core.DiscordWebhookSender {
	if gs.WebhookNotifier != nil {
		return gs.WebhookNotifier
	}
	return appWebhookSender
}

// dispatchStateChangeWebhooks fires a community's Discord webhooks for a game
// state transition, off the request goroutine.
//
// Returns immediately. Everything below happens on a detached goroutine whose
// failures are logged and stamped onto the webhook row, never propagated.
func (gs *GameService) dispatchStateChangeWebhooks(ctx context.Context, game *models.Game, newState string) {
	gs.dispatchStateChangeWebhooksReporting(ctx, game, newState)
}

// dispatchStateChangeWebhooksReporting is dispatchStateChangeWebhooks with a
// return value saying whether a dispatch goroutine was actually started.
//
// The bool exists ONLY for tests, and it earns its place: SafeGo converts a
// panic on the dispatch goroutine into a log line, so deleting the nil guard
// below leaves "the transition succeeded" and "the webhook row is untouched"
// both true while every transition panics. Verified by mutation -- without this
// return value, removing the guard fails no test at all.
func (gs *GameService) dispatchStateChangeWebhooksReporting(ctx context.Context, game *models.Game, newState string) bool {
	// A nil sender is the normal configuration when webhooks are not wired up
	// (most tests). It must be a no-op, not a panic -- the transition already
	// succeeded.
	if gs.webhookSender() == nil {
		return false
	}

	// Detach from the request. The response returning must not cancel delivery;
	// the correlation ID is copied so the dispatch remains traceable.
	dispatchCtx, cancel := context.WithTimeout(
		observability.WithCorrelationID(context.Background(), observability.GetCorrelationID(ctx)),
		core.WebhookDispatchTimeout,
	)

	// SafeGo, not a bare `go func()`: an unrecovered panic in any goroutine kills
	// the whole process, and nothing sits above this one to catch it.
	observability.SafeGo(dispatchCtx, gs.Logger, "dispatch-community-webhooks", func() {
		defer cancel()
		gs.deliverStateChangeWebhooks(dispatchCtx, game, newState)
	})

	return true
}

// deliverStateChangeWebhooks does the actual work on the detached goroutine.
func (gs *GameService) deliverStateChangeWebhooks(ctx context.Context, game *models.Game, newState string) {
	queries := models.New(gs.DB)

	// One round-trip resolves game -> community -> subscribed enabled webhooks.
	// All three filters live in the SQL, including the grandfathering: a legacy
	// game with a NULL community yields no rows, so req 5 holds structurally
	// rather than by a check this function has to remember.
	hooks, err := queries.ListWebhooksForGameState(ctx, models.ListWebhooksForGameStateParams{
		ID:    game.ID,
		State: newState,
	})
	if err != nil {
		gs.Logger.LogError(ctx, err, "Webhook dispatch: failed to list webhooks",
			"game_id", game.ID, "new_state", newState)
		return
	}
	if len(hooks) == 0 {
		return
	}

	embed := gs.buildGameStateEmbed(ctx, game, newState)

	for _, hook := range hooks {
		gs.deliverOne(ctx, hook, embed)
	}
}

// deliverOne sends to a single webhook, retries a bounded number of times, and
// stamps the outcome onto the row.
//
// The stamped status is the ENTIRE delivery-observability story -- there is no
// deliveries table -- so it is what a moderator sees when they ask why their
// webhook stopped working.
func (gs *GameService) deliverOne(ctx context.Context, hook models.CommunityWebhook, embed core.DiscordEmbed) {
	var lastErr error

	for attempt := 1; attempt <= core.WebhookMaxAttempts; attempt++ {
		// Check before each attempt rather than relying on the HTTP client:
		// once the dispatch budget is spent, further attempts and their sleeps
		// are pure waste.
		if ctx.Err() != nil {
			lastErr = ctx.Err()
			break
		}

		err := gs.webhookSender().Send(ctx, hook.Url, embed)
		if err == nil {
			gs.markWebhookSuccess(ctx, hook)
			return
		}
		lastErr = err

		if attempt == core.WebhookMaxAttempts {
			break
		}

		// Honour Discord's own retry_after when it sent one; retrying sooner
		// than asked just earns another 429.
		delay := core.WebhookRetryDelay
		var rateLimited *discord.RateLimitedError
		if errors.As(err, &rateLimited) && rateLimited.RetryAfter > 0 {
			delay = rateLimited.RetryAfter
		}

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			lastErr = ctx.Err()
			// Jump straight to stamping: the budget is gone, and another
			// attempt would only fail on the ctx.Err() check above.
			gs.markWebhookError(ctx, hook, lastErr)
			return
		}
	}

	gs.markWebhookError(ctx, hook, lastErr)
}

// markWebhookSuccess stamps a delivery success, clearing any previous error.
func (gs *GameService) markWebhookSuccess(ctx context.Context, hook models.CommunityWebhook) {
	if err := models.New(gs.DB).MarkCommunityWebhookSuccess(ctx, hook.ID); err != nil {
		gs.Logger.LogError(ctx, err, "Webhook dispatch: failed to stamp success",
			"webhook_id", hook.ID)
	}
}

// markWebhookError stamps a delivery failure after retries are exhausted.
func (gs *GameService) markWebhookError(ctx context.Context, hook models.CommunityWebhook, cause error) {
	msg := "unknown error"
	if cause != nil {
		msg = cause.Error()
	}

	// The stamped message is shown to a moderator and must not become a wall of
	// text; the transport already caps what it quotes from a response body, and
	// this bounds anything else that reaches here.
	const maxStoredError = 500
	if len(msg) > maxStoredError {
		msg = msg[:maxStoredError]
	}

	gs.Logger.Warn(ctx, "Webhook dispatch failed after retries",
		"webhook_id", hook.ID,
		"community_id", hook.CommunityID,
		"error", msg,
	)

	// Uses a fresh context: the usual reason for arriving here is that ctx has
	// expired, and reusing it would fail the very write whose whole purpose is
	// to record that failure for the moderator.
	stampCtx, cancel := context.WithTimeout(
		observability.WithCorrelationID(context.Background(), observability.GetCorrelationID(ctx)),
		5*time.Second,
	)
	defer cancel()

	if err := models.New(gs.DB).MarkCommunityWebhookError(stampCtx, models.MarkCommunityWebhookErrorParams{
		ID: hook.ID,
		// Always Valid: msg falls back to "unknown error" above, and a NULL here
		// would read as "no error has ever occurred".
		LastError: pgtype.Text{String: msg, Valid: true},
	}); err != nil {
		gs.Logger.LogError(stampCtx, err, "Webhook dispatch: failed to stamp error",
			"webhook_id", hook.ID)
	}
}

// buildGameStateEmbed renders the Discord embed for a state transition.
//
// ANONYMITY IS RESPECTED HERE. An anonymous game hides who plays whom, and a
// public Discord channel is the last place that should leak -- so the GM's name
// is omitted while the game is anonymous. Note that entering a public archive
// state clears is_anonymous (see UpdateGameState), so a completed game does name
// its GM; that is the same disclosure the archive itself makes.
func (gs *GameService) buildGameStateEmbed(ctx context.Context, game *models.Game, newState string) core.DiscordEmbed {
	embed := core.DiscordEmbed{
		Title:     game.Title,
		Color:     webhookColorForState(newState),
		Footer:    "ActionPhase",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Description: fmt.Sprintf("Game state changed to **%s**",
			core.GetGameStateDescriptionBrief(newState)),
	}

	if frontendURL := strings.TrimRight(os.Getenv("FRONTEND_URL"), "/"); frontendURL != "" {
		embed.URL = fmt.Sprintf("%s/games/%d", frontendURL, game.ID)
	}

	if !game.IsAnonymous {
		if gm, err := models.New(gs.DB).GetUser(ctx, game.GmUserID); err == nil {
			embed.Description += fmt.Sprintf("\nGM: %s", gm.Username)
		}
		// A failed GM lookup is not worth abandoning the notification over --
		// the state change is the news, the GM's name is a garnish.
	}

	return embed
}

// webhookColorForState gives the embed a left-border colour matching the
// meaning of the transition, so a busy channel is scannable at a glance.
func webhookColorForState(state string) int {
	switch state {
	case core.GameStateRecruitment:
		return 0x57F287 // green — open, join now
	case core.GameStateCharacterCreation, core.GameStateInProgress:
		return 0x5865F2 // blurple — active play
	case core.GameStatePaused:
		return 0xFEE75C // yellow — attention
	case core.GameStateEpilogue, core.GameStateCompleted:
		return 0x9B59B6 // purple — concluded
	case core.GameStateCancelled:
		return 0xED4245 // red — ended early
	default:
		return 0x95A5A6 // grey
	}
}
