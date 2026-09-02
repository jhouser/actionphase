package db

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	"actionphase/pkg/discord"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testHookURL = "https://discord.com/api/webhooks/123/dispatchtoken"

// webhookFixture builds a community, a game inside it, and a webhook row
// subscribed to the given events.
func webhookFixture(t *testing.T, testDB *core.TestDatabase, events []string) (*models.Game, models.CommunityWebhook) {
	t.Helper()
	ctx := context.Background()
	queries := models.New(testDB.Pool)

	owner := testDB.CreateTestUser(t, "wh-owner-"+t.Name(), "wh-owner-"+t.Name()+"@example.com")
	community := testDB.CreateTestCommunity(t, int32(owner.ID))
	game := testDB.CreateTestGame(t, int32(owner.ID), "Webhook Game")

	_, err := testDB.Pool.Exec(ctx, "UPDATE games SET community_id = $1 WHERE id = $2", community.ID, game.ID)
	require.NoError(t, err)

	hook, err := queries.CreateCommunityWebhook(ctx, models.CreateCommunityWebhookParams{
		CommunityID: community.ID,
		Url:         testHookURL,
		Label:       pgtype.Text{String: "#general", Valid: true},
		IsEnabled:   true,
		Events:      events,
	})
	require.NoError(t, err)

	return game, hook
}

func webhookCleanup(t *testing.T, testDB *core.TestDatabase) {
	testDB.CleanupTables(t, "community_webhooks", "games", "communities", "users")
}

// waitForSends polls until the detached dispatch goroutine has recorded n sends,
// or the deadline passes. Polling rather than a fixed sleep: the assertion is
// about what eventually happens, and a fixed sleep either flakes or is slow.
func waitForSends(t *testing.T, mock *discord.MockWebhookClient, n int, within time.Duration) []discord.SentWebhook {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if sent := mock.Sent(); len(sent) >= n {
			return sent
		}
		time.Sleep(5 * time.Millisecond)
	}
	return mock.Sent()
}

func TestWebhookDispatch_FiresForSubscribedState(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer webhookCleanup(t, testDB)

	app := core.NewTestApp(testDB.Pool)
	mock := &discord.MockWebhookClient{}
	game, _ := webhookFixture(t, testDB, []string{core.GameStateRecruitment})

	gs := &GameService{DB: testDB.Pool, Logger: app.ObsLogger, WebhookNotifier: mock}
	_, err := gs.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	sent := waitForSends(t, mock, 1, 2*time.Second)
	require.Len(t, sent, 1, "one webhook subscribed to recruitment should fire once")
	assert.Equal(t, testHookURL, sent[0].URL)
	assert.Equal(t, "Webhook Game", sent[0].Embed.Title)
	assert.Contains(t, sent[0].Embed.Description, "RECRUITMENT")
}

func TestWebhookDispatch_SilentWhenNotSubscribed(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer webhookCleanup(t, testDB)

	app := core.NewTestApp(testDB.Pool)
	mock := &discord.MockWebhookClient{}
	// Subscribed to completed only; the transition below is to recruitment.
	game, _ := webhookFixture(t, testDB, []string{core.GameStateCompleted})

	gs := &GameService{DB: testDB.Pool, Logger: app.ObsLogger, WebhookNotifier: mock}
	_, err := gs.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)
	assert.Empty(t, mock.Sent(), "a recruitment-only channel must stay quiet for other states")
}

func TestWebhookDispatch_SilentWhenDisabled(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer webhookCleanup(t, testDB)

	app := core.NewTestApp(testDB.Pool)
	mock := &discord.MockWebhookClient{}
	game, hook := webhookFixture(t, testDB, []string{core.GameStateRecruitment})

	_, err := testDB.Pool.Exec(context.Background(),
		"UPDATE community_webhooks SET is_enabled = false WHERE id = $1", hook.ID)
	require.NoError(t, err)

	gs := &GameService{DB: testDB.Pool, Logger: app.ObsLogger, WebhookNotifier: mock}
	_, err = gs.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)
	assert.Empty(t, mock.Sent(), "a disabled webhook must not fire")
}

// Req 5's regression guard at the dispatch layer: a legacy game predates
// communities entirely and must never reach a webhook.
func TestWebhookDispatch_LegacyGameWithNoCommunity(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer webhookCleanup(t, testDB)

	app := core.NewTestApp(testDB.Pool)
	mock := &discord.MockWebhookClient{}

	owner := testDB.CreateTestUser(t, "legacy-owner", "legacy-owner@example.com")
	community := testDB.CreateTestCommunity(t, int32(owner.ID))
	// A webhook exists, subscribed to the state -- but the game belongs to no
	// community, so the join must yield nothing.
	_, err := models.New(testDB.Pool).CreateCommunityWebhook(context.Background(),
		models.CreateCommunityWebhookParams{
			CommunityID: community.ID,
			Url:         testHookURL,
			IsEnabled:   true,
			Events:      []string{core.GameStateRecruitment},
		})
	require.NoError(t, err)

	legacyGame := testDB.CreateTestGame(t, int32(owner.ID), "Legacy Game")

	gs := &GameService{DB: testDB.Pool, Logger: app.ObsLogger, WebhookNotifier: mock}
	_, err = gs.UpdateGameState(context.Background(), legacyGame.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)
	assert.Empty(t, mock.Sent(), "a game with no community must dispatch nothing")
}

// THE §6 GOTCHA, asserted directly. A bare `go func()` closing over the request
// context is cancelled the instant the response is written -- deliveries would
// then fail nondeterministically in production and pass in every other test.
func TestWebhookDispatch_SurvivesRequestContextCancellation(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer webhookCleanup(t, testDB)

	app := core.NewTestApp(testDB.Pool)
	mock := &discord.MockWebhookClient{}
	game, _ := webhookFixture(t, testDB, []string{core.GameStateRecruitment})

	// A request context that is cancelled the moment UpdateGameState returns,
	// exactly as the HTTP layer does when it writes the response.
	reqCtx, cancel := context.WithCancel(context.Background())

	gs := &GameService{DB: testDB.Pool, Logger: app.ObsLogger, WebhookNotifier: mock}
	_, err := gs.UpdateGameState(reqCtx, game.ID, core.GameStateRecruitment)
	require.NoError(t, err)
	cancel()

	sent := waitForSends(t, mock, 1, 2*time.Second)
	require.Len(t, sent, 1,
		"delivery must survive the request context being cancelled; the dispatch "+
			"goroutine must not close over the request ctx")
}

// A hanging Discord endpoint must not be felt by the GM making the transition.
func TestWebhookDispatch_DoesNotBlockTransition(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer webhookCleanup(t, testDB)

	app := core.NewTestApp(testDB.Pool)
	// Far longer than any acceptable request; if the transition waits on this,
	// the assertion below fails.
	mock := &discord.MockWebhookClient{Delay: 30 * time.Second}
	game, _ := webhookFixture(t, testDB, []string{core.GameStateRecruitment})

	gs := &GameService{DB: testDB.Pool, Logger: app.ObsLogger, WebhookNotifier: mock}

	start := time.Now()
	_, err := gs.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Less(t, elapsed, 2*time.Second,
		"a hanging webhook endpoint must not delay the state transition")
}

// A failing webhook must not fail the transition, and the state change must
// still be durable.
func TestWebhookDispatch_FailureDoesNotFailTransition(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer webhookCleanup(t, testDB)

	app := core.NewTestApp(testDB.Pool)
	mock := &discord.MockWebhookClient{ShouldFail: true}
	game, _ := webhookFixture(t, testDB, []string{core.GameStateRecruitment})

	gs := &GameService{DB: testDB.Pool, Logger: app.ObsLogger, WebhookNotifier: mock}
	updated, err := gs.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)

	require.NoError(t, err, "a Discord failure must never surface as a failed transition")
	assert.Equal(t, core.GameStateRecruitment, updated.State.String)

	// And the change is committed, not merely returned.
	var state string
	require.NoError(t, testDB.Pool.QueryRow(context.Background(),
		"SELECT state FROM games WHERE id = $1", game.ID).Scan(&state))
	assert.Equal(t, core.GameStateRecruitment, state)
}

func TestWebhookDispatch_NilNotifierNeverPanics(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer webhookCleanup(t, testDB)

	app := core.NewTestApp(testDB.Pool)
	game, _ := webhookFixture(t, testDB, []string{core.GameStateRecruitment})

	// WebhookNotifier intentionally nil -- the normal configuration wherever
	// Discord is not wired up.
	gs := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}
	updated, err := gs.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)

	require.NoError(t, err)
	assert.Equal(t, core.GameStateRecruitment, updated.State.String)

}

// The nil guard has to be asserted SEPARATELY from "the transition survived".
//
// SafeGo recovers panics on the dispatch goroutine, so deleting the nil check
// leaves the test above perfectly green while every single transition panics
// and is silently swallowed -- verified by mutation: removing the guard does
// not fail it. Both defences are wanted (SafeGo is the last resort, the guard
// is the contract), so this test proves the guard by observing that dispatch
// STOPS before touching the notifier at all.
func TestWebhookDispatch_NilNotifierSkipsDispatchEntirely(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer webhookCleanup(t, testDB)

	app := core.NewTestApp(testDB.Pool)
	game, hook := webhookFixture(t, testDB, []string{core.GameStateRecruitment})

	gs := &GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	// Assert the guard where it is decidable: dispatch must report that it
	// started NOTHING. Checking the webhook row instead would not work -- the
	// panic a missing guard causes happens before any stamp, so an untouched
	// row looks identical either way.
	started := gs.dispatchStateChangeWebhooksReporting(
		context.Background(), &models.Game{ID: game.ID}, core.GameStateRecruitment)
	assert.False(t, started,
		"a nil WebhookNotifier must be caught by the guard, before any goroutine is spawned")

	// And the transition itself is unaffected.
	updated, err := gs.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)
	assert.Equal(t, core.GameStateRecruitment, updated.State.String)

	time.Sleep(200 * time.Millisecond)

	var lastSuccess, lastErrAt *time.Time
	var lastErr *string
	require.NoError(t, testDB.Pool.QueryRow(context.Background(),
		"SELECT last_success_at, last_error, last_error_at FROM community_webhooks WHERE id = $1", hook.ID).
		Scan(&lastSuccess, &lastErr, &lastErrAt))

	assert.Nil(t, lastSuccess, "nothing was delivered, so nothing may be stamped as delivered")
	assert.Nil(t, lastErr, "dispatch must not run at all, so no failure may be recorded")
	assert.Nil(t, lastErrAt)
}

// An unrecovered panic in any goroutine takes down the whole process, and
// nothing sits above the dispatch goroutine to catch one.
func TestWebhookDispatch_PanicInGoroutineDoesNotCrash(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer webhookCleanup(t, testDB)

	app := core.NewTestApp(testDB.Pool)
	game, _ := webhookFixture(t, testDB, []string{core.GameStateRecruitment})

	gs := &GameService{DB: testDB.Pool, Logger: app.ObsLogger, WebhookNotifier: &panicWebhookClient{}}
	_, err := gs.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	// Reaching here at all is most of the assertion: an unrecovered panic would
	// have killed the test binary rather than failed this test.
	time.Sleep(200 * time.Millisecond)
}

type panicWebhookClient struct{}

func (p *panicWebhookClient) Send(_ context.Context, _ string, _ core.DiscordEmbed) error {
	panic("simulated webhook panic")
}

func TestWebhookDispatch_RetriesThenSucceeds(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer webhookCleanup(t, testDB)

	app := core.NewTestApp(testDB.Pool)
	// Fails once, then succeeds: distinguishes real retrying from a single
	// attempt, which the sent list alone cannot show.
	mock := &discord.MockWebhookClient{FailTimes: 1}
	game, hook := webhookFixture(t, testDB, []string{core.GameStateRecruitment})

	gs := &GameService{DB: testDB.Pool, Logger: app.ObsLogger, WebhookNotifier: mock}
	_, err := gs.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	sent := waitForSends(t, mock, 1, 6*time.Second)
	require.Len(t, sent, 1, "the retry must eventually deliver")
	assert.Equal(t, 2, mock.Attempts(), "should have taken two attempts")

	// Poll for the STAMP rather than reading once.
	//
	// waitForSends returns as soon as the mock records the send, but the row is
	// stamped afterwards -- so a single read here races the dispatcher and fails
	// intermittently under load (it did, in a full-suite run, while passing in
	// isolation). The condition being waited on has to be the one asserted.
	var successAt *time.Time
	var lastErr *string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		require.NoError(t, testDB.Pool.QueryRow(context.Background(),
			"SELECT last_success_at, last_error FROM community_webhooks WHERE id = $1", hook.ID).
			Scan(&successAt, &lastErr))
		if successAt != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	assert.NotNil(t, successAt, "a delivered webhook must stamp last_success_at")
	assert.Nil(t, lastErr, "success must clear a previous error")
}

func TestWebhookDispatch_ExhaustedRetriesStampError(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer webhookCleanup(t, testDB)

	app := core.NewTestApp(testDB.Pool)
	mock := &discord.MockWebhookClient{ShouldFail: true}
	game, hook := webhookFixture(t, testDB, []string{core.GameStateRecruitment})

	gs := &GameService{DB: testDB.Pool, Logger: app.ObsLogger, WebhookNotifier: mock}
	_, err := gs.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	// This is the whole delivery-observability story: without the stamp, a
	// moderator has no way to learn their webhook is broken.
	var lastErr *string
	var lastErrAt *time.Time
	deadline := time.Now().Add(core.WebhookDispatchTimeout + 3*time.Second)
	for time.Now().Before(deadline) {
		require.NoError(t, testDB.Pool.QueryRow(context.Background(),
			"SELECT last_error, last_error_at FROM community_webhooks WHERE id = $1", hook.ID).
			Scan(&lastErr, &lastErrAt))
		if lastErr != nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	require.NotNil(t, lastErr, "exhausted retries must stamp last_error")
	require.NotNil(t, lastErrAt)
	assert.Equal(t, core.WebhookMaxAttempts, mock.Attempts(), "should exhaust the retry budget")
}

// The stamped error is shown to a moderator and must never contain the token.
func TestWebhookDispatch_StampedErrorNeverLeaksURL(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer webhookCleanup(t, testDB)

	app := core.NewTestApp(testDB.Pool)
	mock := &discord.MockWebhookClient{ShouldFail: true}
	game, hook := webhookFixture(t, testDB, []string{core.GameStateRecruitment})

	gs := &GameService{DB: testDB.Pool, Logger: app.ObsLogger, WebhookNotifier: mock}
	_, err := gs.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	var lastErr *string
	deadline := time.Now().Add(core.WebhookDispatchTimeout + 3*time.Second)
	for time.Now().Before(deadline) {
		require.NoError(t, testDB.Pool.QueryRow(context.Background(),
			"SELECT last_error FROM community_webhooks WHERE id = $1", hook.ID).Scan(&lastErr))
		if lastErr != nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	require.NotNil(t, lastErr)
	assert.NotContains(t, *lastErr, "dispatchtoken",
		"the webhook token must never be stored in a moderator-visible column")
	assert.LessOrEqual(t, len(*lastErr), 500, "stored error must be bounded")
}

// Anonymity must not be broken by a public Discord post.
func TestWebhookDispatch_AnonymousGameHidesGM(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer webhookCleanup(t, testDB)

	app := core.NewTestApp(testDB.Pool)
	mock := &discord.MockWebhookClient{}
	game, _ := webhookFixture(t, testDB, []string{core.GameStateRecruitment})

	_, err := testDB.Pool.Exec(context.Background(),
		"UPDATE games SET is_anonymous = true WHERE id = $1", game.ID)
	require.NoError(t, err)

	gs := &GameService{DB: testDB.Pool, Logger: app.ObsLogger, WebhookNotifier: mock}
	_, err = gs.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	sent := waitForSends(t, mock, 1, 2*time.Second)
	require.Len(t, sent, 1)
	assert.NotContains(t, sent[0].Embed.Description, "GM:",
		"an anonymous game must not name its GM in a public channel")
}

func TestWebhookDispatch_NamedGMForNonAnonymousGame(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer webhookCleanup(t, testDB)

	app := core.NewTestApp(testDB.Pool)
	mock := &discord.MockWebhookClient{}
	game, _ := webhookFixture(t, testDB, []string{core.GameStateRecruitment})

	gs := &GameService{DB: testDB.Pool, Logger: app.ObsLogger, WebhookNotifier: mock}
	_, err := gs.UpdateGameState(context.Background(), game.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	sent := waitForSends(t, mock, 1, 2*time.Second)
	require.Len(t, sent, 1)
	assert.Contains(t, sent[0].Embed.Description, "GM:",
		"a non-anonymous game names its GM, which is what makes the anonymous case meaningful")
}

// Several webhooks on one community each get exactly one delivery.
func TestWebhookDispatch_FiresOncePerMatchingWebhook(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer webhookCleanup(t, testDB)

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	mock := &discord.MockWebhookClient{}
	game, _ := webhookFixture(t, testDB, []string{core.GameStateRecruitment})

	var communityID int32
	require.NoError(t, testDB.Pool.QueryRow(ctx,
		"SELECT community_id FROM games WHERE id = $1", game.ID).Scan(&communityID))

	queries := models.New(testDB.Pool)
	// A second subscribed webhook, and a third subscribed to something else.
	_, err := queries.CreateCommunityWebhook(ctx, models.CreateCommunityWebhookParams{
		CommunityID: communityID,
		Url:         "https://discord.com/api/webhooks/456/second",
		IsEnabled:   true,
		Events:      []string{core.GameStateRecruitment},
	})
	require.NoError(t, err)
	_, err = queries.CreateCommunityWebhook(ctx, models.CreateCommunityWebhookParams{
		CommunityID: communityID,
		Url:         "https://discord.com/api/webhooks/789/third",
		IsEnabled:   true,
		Events:      []string{core.GameStateCompleted},
	})
	require.NoError(t, err)

	gs := &GameService{DB: testDB.Pool, Logger: app.ObsLogger, WebhookNotifier: mock}
	_, err = gs.UpdateGameState(ctx, game.ID, core.GameStateRecruitment)
	require.NoError(t, err)

	sent := waitForSends(t, mock, 2, 2*time.Second)
	require.Len(t, sent, 2, "exactly the two subscribed webhooks should fire")

	urls := []string{sent[0].URL, sent[1].URL}
	assert.Contains(t, strings.Join(urls, " "), "dispatchtoken")
	assert.Contains(t, strings.Join(urls, " "), "second")
	assert.NotContains(t, strings.Join(urls, " "), "third",
		"a webhook subscribed to a different state must not fire")
}

// Guards against a shared-state bug in the dispatcher: concurrent transitions
// must not interleave into lost or duplicated deliveries.
func TestWebhookDispatch_ConcurrentTransitionsAreIndependent(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer webhookCleanup(t, testDB)

	ctx := context.Background()
	app := core.NewTestApp(testDB.Pool)
	mock := &discord.MockWebhookClient{}

	owner := testDB.CreateTestUser(t, "conc-owner", "conc-owner@example.com")
	community := testDB.CreateTestCommunity(t, int32(owner.ID))
	queries := models.New(testDB.Pool)
	_, err := queries.CreateCommunityWebhook(ctx, models.CreateCommunityWebhookParams{
		CommunityID: community.ID,
		Url:         testHookURL,
		IsEnabled:   true,
		Events:      []string{core.GameStateRecruitment},
	})
	require.NoError(t, err)

	const gameCount = 5
	games := make([]*models.Game, gameCount)
	for i := range games {
		g := testDB.CreateTestGame(t, int32(owner.ID), "Concurrent Game")
		_, err := testDB.Pool.Exec(ctx, "UPDATE games SET community_id = $1 WHERE id = $2", community.ID, g.ID)
		require.NoError(t, err)
		games[i] = g
	}

	gs := &GameService{DB: testDB.Pool, Logger: app.ObsLogger, WebhookNotifier: mock}

	var wg sync.WaitGroup
	for _, g := range games {
		wg.Add(1)
		go func(id int32) {
			defer wg.Done()
			_, err := gs.UpdateGameState(ctx, id, core.GameStateRecruitment)
			assert.NoError(t, err)
		}(g.ID)
	}
	wg.Wait()

	sent := waitForSends(t, mock, gameCount, 3*time.Second)
	assert.Len(t, sent, gameCount, "each transition delivers exactly once")
}
