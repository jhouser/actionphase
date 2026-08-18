package actions

import (
	"context"
	"testing"

	core "actionphase/pkg/core"
	models "actionphase/pkg/db/models"
	db "actionphase/pkg/db/services"
	phases "actionphase/pkg/db/services/phases"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stagedTestEnv is the shared fixture for staged-chain tests: a game with a GM,
// a player, and an active action phase to hang results off.
type stagedTestEnv struct {
	testDB        *core.TestDatabase
	actionService *ActionSubmissionService
	gameID        int32
	phaseID       int32
	playerID      int32
	gmID          int32
}

func setupStagedTest(t *testing.T) *stagedTestEnv {
	t.Helper()

	testDB := core.NewTestDatabase(t)
	t.Cleanup(testDB.Close)

	app := core.NewTestApp(testDB.Pool)

	actionService := &ActionSubmissionService{
		DB:                  testDB.Pool,
		Logger:              app.ObsLogger,
		NotificationService: &db.NotificationService{DB: testDB.Pool, Logger: app.ObsLogger},
	}
	phaseService := &phases.PhaseService{DB: testDB.Pool, Logger: app.ObsLogger}
	gameService := &db.GameService{DB: testDB.Pool, Logger: app.ObsLogger}

	gm := testDB.CreateTestUser(t, "staged_gm", "staged_gm@example.com")
	player := testDB.CreateTestUser(t, "staged_player", "staged_player@example.com")
	game := testDB.CreateTestGame(t, int32(gm.ID), "Staged Reveal Game")

	_, err := gameService.AddGameParticipant(context.Background(), game.ID, int32(player.ID), "player")
	require.NoError(t, err)

	phase, err := phaseService.TransitionToNextPhase(context.Background(), game.ID, int32(gm.ID), core.TransitionPhaseRequest{
		PhaseType: "action",
		Title:     "Action Phase",
	})
	require.NoError(t, err)

	return &stagedTestEnv{
		testDB:        testDB,
		actionService: actionService,
		gameID:        game.ID,
		phaseID:       phase.ID,
		playerID:      int32(player.ID),
		gmID:          int32(gm.ID),
	}
}

// chainRequest builds a request for a chain with the given per-part delays.
// delays[0] is the head and must be 0.
func (e *stagedTestEnv) chainRequest(delays ...int32) core.CreateStagedResultChainRequest {
	parts := make([]core.StagedResultPart, len(delays))
	for i, d := range delays {
		parts[i] = core.StagedResultPart{
			Content:      "Part content",
			DelayMinutes: d,
		}
	}
	return core.CreateStagedResultChainRequest{
		GameID:      e.gameID,
		PhaseID:     e.phaseID,
		UserID:      e.playerID,
		GMUserID:    e.gmID,
		Parts:       parts,
		IsPublished: true,
	}
}

// backdateRelease moves a part's released_at into the past, which is how these
// tests make a delay elapse without actually waiting for it. Due-ness is
// computed as parent.released_at + delay <= NOW(), so rewinding the parent is
// equivalent to time passing.
func (e *stagedTestEnv) backdateRelease(t *testing.T, resultID int32, minutes int) {
	t.Helper()
	_, err := e.testDB.Pool.Exec(context.Background(),
		`UPDATE action_results
		 SET released_at = released_at - make_interval(mins => $2)
		 WHERE id = $1`, resultID, minutes)
	require.NoError(t, err)
}

func (e *stagedTestEnv) getResult(t *testing.T, resultID int32) models.ActionResult {
	t.Helper()
	result, err := models.New(e.testDB.Pool).GetActionResult(context.Background(), resultID)
	require.NoError(t, err)
	return result
}

func TestCreateStagedResultChain(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	ctx := context.Background()

	t.Run("links parts to their predecessors with the given delays", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.chainRequest(0, 15, 30))
		require.NoError(t, err)
		require.Len(t, chain, 3)

		// Head: no parent, no delay.
		assert.False(t, chain[0].ParentResultID.Valid, "head must not have a parent")
		assert.False(t, chain[0].RevealDelayMinutes.Valid, "head must not have a delay")

		// Each later part points at the one before it.
		assert.Equal(t, chain[0].ID, chain[1].ParentResultID.Int32)
		assert.Equal(t, int32(15), chain[1].RevealDelayMinutes.Int32)
		assert.Equal(t, chain[1].ID, chain[2].ParentResultID.Int32)
		assert.Equal(t, int32(30), chain[2].RevealDelayMinutes.Int32)
	})

	t.Run("publishing the chain releases only the head", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.chainRequest(0, 15, 30))
		require.NoError(t, err)

		// This is the core visibility property: all three are published, but
		// only the head is released, so only the head is readable.
		assert.True(t, chain[0].ReleasedAt.Valid, "head should be released immediately")
		assert.False(t, chain[1].ReleasedAt.Valid, "part 2 must wait for the worker")
		assert.False(t, chain[2].ReleasedAt.Valid, "part 3 must wait for the worker")

		for i, part := range chain {
			assert.True(t, part.IsPublished.Bool, "part %d should be published", i+1)
		}
	})

	t.Run("an unpublished chain releases nothing and starts no clock", func(t *testing.T) {
		env := setupStagedTest(t)

		req := env.chainRequest(0, 15)
		req.IsPublished = false

		chain, err := env.actionService.CreateStagedResultChain(ctx, req)
		require.NoError(t, err)

		assert.False(t, chain[0].ReleasedAt.Valid, "draft head must not be released")
		assert.False(t, chain[1].ReleasedAt.Valid)

		// With the head unreleased, part 2 has no clock to run against — it must
		// not become due no matter how much time passes.
		examined, released, err := env.actionService.ReleaseDueStagedParts(ctx)
		require.NoError(t, err)
		assert.Zero(t, examined)
		assert.Zero(t, released)
	})

	t.Run("all parts share one recipient", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.chainRequest(0, 15))
		require.NoError(t, err)

		for i, part := range chain {
			assert.Equal(t, env.playerID, part.UserID, "part %d recipient", i+1)
			assert.Equal(t, env.gameID, part.GameID, "part %d game", i+1)
			assert.Equal(t, env.phaseID, part.PhaseID, "part %d phase", i+1)
		}
	})

	t.Run("rejects invariant violations", func(t *testing.T) {
		env := setupStagedTest(t)

		tests := []struct {
			name   string
			mutate func(*core.CreateStagedResultChainRequest)
			errMsg string
		}{
			{
				name:   "single part is not a chain",
				mutate: func(r *core.CreateStagedResultChainRequest) { r.Parts = r.Parts[:1] },
				errMsg: "at least 2 parts",
			},
			{
				name: "chain longer than the maximum",
				mutate: func(r *core.CreateStagedResultChainRequest) {
					parts := make([]core.StagedResultPart, core.MaxStagedChainLength+1)
					for i := range parts {
						parts[i] = core.StagedResultPart{Content: "x", DelayMinutes: 5}
					}
					parts[0].DelayMinutes = 0
					r.Parts = parts
				},
				errMsg: "at most 10 parts",
			},
			{
				name:   "delay below the minimum",
				mutate: func(r *core.CreateStagedResultChainRequest) { r.Parts[1].DelayMinutes = 0 },
				errMsg: "delay must be between",
			},
			{
				name:   "delay beyond 24 hours",
				mutate: func(r *core.CreateStagedResultChainRequest) { r.Parts[1].DelayMinutes = 1441 },
				errMsg: "delay must be between",
			},
			{
				name:   "head carrying a delay",
				mutate: func(r *core.CreateStagedResultChainRequest) { r.Parts[0].DelayMinutes = 10 },
				errMsg: "chain head and cannot have a delay",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := env.chainRequest(0, 15)
				tt.mutate(&req)

				_, err := env.actionService.CreateStagedResultChain(ctx, req)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			})
		}
	})
}

func TestReleaseDueStagedParts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	ctx := context.Background()

	t.Run("releases nothing before the delay elapses", func(t *testing.T) {
		env := setupStagedTest(t)

		_, err := env.actionService.CreateStagedResultChain(ctx, env.chainRequest(0, 15))
		require.NoError(t, err)

		examined, released, err := env.actionService.ReleaseDueStagedParts(ctx)
		require.NoError(t, err)
		assert.Zero(t, examined, "nothing is due 0 minutes into a 15-minute wait")
		assert.Zero(t, released)
	})

	t.Run("releases the part once its delay has elapsed", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.chainRequest(0, 15))
		require.NoError(t, err)

		env.backdateRelease(t, chain[0].ID, 16)

		examined, released, err := env.actionService.ReleaseDueStagedParts(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, examined)
		assert.Equal(t, 1, released)

		assert.True(t, env.getResult(t, chain[1].ID).ReleasedAt.Valid, "part 2 should now be visible")
	})

	t.Run("a three-part chain releases strictly in order, one tick at a time", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.chainRequest(0, 15, 30))
		require.NoError(t, err)

		// Part 2 comes due. Part 3's clock has not started, because its parent
		// is still unreleased — the chain cannot skip ahead.
		env.backdateRelease(t, chain[0].ID, 16)

		_, released, err := env.actionService.ReleaseDueStagedParts(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, released, "only part 2 is due")
		assert.False(t, env.getResult(t, chain[2].ID).ReleasedAt.Valid, "part 3 must not jump the queue")

		// Part 3's 30-minute wait runs from part 2's release, not the head's.
		env.backdateRelease(t, chain[1].ID, 31)

		_, released, err = env.actionService.ReleaseDueStagedParts(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, released)
		assert.True(t, env.getResult(t, chain[2].ID).ReleasedAt.Valid, "part 3 should now be visible")
	})

	// Ordering must hold no matter how stale the chain is. The test above
	// backdates the head by 16m, less than part 3's own 30m delay, so it never
	// distinguishes "waiting for my parent" from "my own timer hasn't elapsed".
	// This backdates far past every delay in the chain, so only the parent rule
	// can hold part 3 back.
	//
	// Worth knowing when reading ReleaseDueStagedParts: the explicit
	// `AND parent.released_at IS NOT NULL` is belt-and-braces, not the load-
	// bearing clause. An unreleased parent makes
	// `parent.released_at + interval <= NOW()` evaluate to NULL, which WHERE
	// treats as false — so the row is excluded by the arithmetic alone.
	// Deleting the explicit clause leaves behaviour unchanged (verified by
	// mutation), which is why no test can fail on its removal. Keep it anyway:
	// it states the intent that the NULL semantics only imply.
	t.Run("a part waits for its parent even when its own delay has long elapsed", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.chainRequest(0, 15, 30))
		require.NoError(t, err)

		// 500 minutes: far beyond part 2's 15m and part 3's 30m combined, so
		// every delay in the chain has "elapsed" by wall clock.
		env.backdateRelease(t, chain[0].ID, 500)

		_, released, err := env.actionService.ReleaseDueStagedParts(ctx)
		require.NoError(t, err)

		assert.Equal(t, 1, released,
			"only part 2 may release: part 3's parent is still unreleased, however old the head is")
		assert.True(t, env.getResult(t, chain[1].ID).ReleasedAt.Valid, "part 2 was due")
		assert.False(t, env.getResult(t, chain[2].ID).ReleasedAt.Valid,
			"part 3 must wait for part 2 to release, not merely for time to pass")
	})

	t.Run("a second tick does not re-release an already-released part", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.chainRequest(0, 15))
		require.NoError(t, err)
		env.backdateRelease(t, chain[0].ID, 16)

		_, released, err := env.actionService.ReleaseDueStagedParts(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, released)

		firstReleaseTime := env.getResult(t, chain[1].ID).ReleasedAt.Time

		// Re-releasing would re-notify the player and move the timestamp.
		examined, released, err := env.actionService.ReleaseDueStagedParts(ctx)
		require.NoError(t, err)
		assert.Zero(t, examined, "a released part is no longer due")
		assert.Zero(t, released)

		assert.Equal(t, firstReleaseTime, env.getResult(t, chain[1].ID).ReleasedAt.Time,
			"release timestamp must not move")
	})

	t.Run("the recipient cannot read the text of unreleased parts", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.chainRequest(0, 15))
		require.NoError(t, err)

		queries := models.New(env.testDB.Pool)

		// GetUserResults is the player's read path and the gate the whole
		// feature rests on. It withholds an unreleased part's *content* while
		// still returning the row, so the client can show a placeholder
		// counting down to the reveal. Withholding the row instead would leave
		// nothing to count down from.
		visible, err := queries.GetUserResults(ctx, models.GetUserResultsParams{
			GameID: env.gameID,
			UserID: env.playerID,
		})
		require.NoError(t, err)
		require.Len(t, visible, 2, "both rows are returned; only the text is withheld")

		assert.Equal(t, chain[0].ID, visible[0].ID)
		assert.NotEmpty(t, visible[0].Content, "the released head is readable")

		assert.Equal(t, chain[1].ID, visible[1].ID)
		assert.Empty(t, visible[1].Content,
			"an unreleased part must carry no text — this is the gate the feature rests on")
		assert.False(t, visible[1].ReleasedAt.Valid,
			"and released_at is how the client knows the part is still locked")

		env.backdateRelease(t, chain[0].ID, 16)
		_, _, err = env.actionService.ReleaseDueStagedParts(ctx)
		require.NoError(t, err)

		visible, err = queries.GetUserResults(ctx, models.GetUserResultsParams{
			GameID: env.gameID,
			UserID: env.playerID,
		})
		require.NoError(t, err)
		require.Len(t, visible, 2)
		assert.NotEmpty(t, visible[1].Content, "part 2's text arrives only after release")
		assert.True(t, visible[1].ReleasedAt.Valid)
	})

	t.Run("a chain keeps its own clock after the phase advances", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.chainRequest(0, 15))
		require.NoError(t, err)
		env.backdateRelease(t, chain[0].ID, 16)

		// Advance to a new phase. Chain independence means a pending part still
		// releases on schedule: the worker never consults phase or game state.
		app := core.NewTestApp(env.testDB.Pool)
		phaseService := &phases.PhaseService{DB: env.testDB.Pool, Logger: app.ObsLogger}
		_, err = phaseService.TransitionToNextPhase(ctx, env.gameID, env.gmID, core.TransitionPhaseRequest{
			PhaseType: "common_room",
			Title:     "Aftermath",
		})
		require.NoError(t, err)

		_, released, err := env.actionService.ReleaseDueStagedParts(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, released, "phase advancement must not strand a pending part")
	})
}

func TestCancelPendingPart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	ctx := context.Background()

	t.Run("cancels a pending part and the parts behind it", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.chainRequest(0, 15, 30))
		require.NoError(t, err)

		require.NoError(t, env.actionService.CancelPendingPart(ctx, chain[1].ID))

		// Part 3 cascades: with part 2 gone it has no clock and would never fire.
		_, err = models.New(env.testDB.Pool).GetActionResult(ctx, chain[2].ID)
		assert.Error(t, err, "part 3 should cascade with its parent")

		// The head is untouched and still readable.
		assert.True(t, env.getResult(t, chain[0].ID).ReleasedAt.Valid)
	})

	t.Run("refuses to cancel an already-released part", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.chainRequest(0, 15))
		require.NoError(t, err)
		env.backdateRelease(t, chain[0].ID, 16)
		_, _, err = env.actionService.ReleaseDueStagedParts(ctx)
		require.NoError(t, err)

		err = env.actionService.CancelPendingPart(ctx, chain[1].ID)
		require.Error(t, err, "the player has already read it")
		assert.Contains(t, err.Error(), "already been released")
	})

	t.Run("refuses to cancel a chain head", func(t *testing.T) {
		env := setupStagedTest(t)

		// An unpublished chain, so the head is unreleased. A published head is
		// also refused, but by the already-released check — this exercises the
		// head check specifically.
		req := env.chainRequest(0, 15)
		req.IsPublished = false
		chain, err := env.actionService.CreateStagedResultChain(ctx, req)
		require.NoError(t, err)
		require.False(t, chain[0].ReleasedAt.Valid)

		// Routing a head here would cascade the entire chain under a name that
		// does not suggest it.
		err = env.actionService.CancelPendingPart(ctx, chain[0].ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a staged part")
	})

	t.Run("refuses to cancel a published head, which is already visible", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.chainRequest(0, 15))
		require.NoError(t, err)

		err = env.actionService.CancelPendingPart(ctx, chain[0].ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already been released")
	})
}

// draftChainRequest is chainRequest's unpublished twin. Appending is a
// draft-only operation, so these tests need a chain the GM is still writing.
func (e *stagedTestEnv) draftChainRequest(delays ...int32) core.CreateStagedResultChainRequest {
	req := e.chainRequest(delays...)
	req.IsPublished = false
	return req
}

// createDraftResult makes an ordinary single unstaged draft — the state a GM is
// in when they have written the opening beat and nothing else yet.
func (e *stagedTestEnv) createDraftResult(t *testing.T) models.ActionResult {
	t.Helper()
	result, err := e.actionService.CreateActionResult(context.Background(), core.CreateActionResultRequest{
		GameID:      e.gameID,
		PhaseID:     e.phaseID,
		UserID:      e.playerID,
		GMUserID:    e.gmID,
		Content:     "The sword whooshes toward your head...",
		IsPublished: false,
	})
	require.NoError(t, err)
	return *result
}

func TestAppendStagedPart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	ctx := context.Background()

	// The headline case for this feature: a GM writes the opening beat, saves,
	// and comes back later to add the payoff. Before this existed the only way
	// to stage a result was to have the whole thing ready at create time.
	t.Run("turns an ordinary draft into a staged chain", func(t *testing.T) {
		env := setupStagedTest(t)

		draft := env.createDraftResult(t)
		require.False(t, draft.ParentResultID.Valid, "a plain draft has no parent")

		appended, err := env.actionService.AppendStagedPart(ctx, draft.ID, core.StagedResultPart{
			Content:      "...and misses!",
			DelayMinutes: 15,
		})
		require.NoError(t, err)

		assert.Equal(t, draft.ID, appended.ParentResultID.Int32, "must hang off the draft")
		assert.Equal(t, int32(15), appended.RevealDelayMinutes.Int32)
		assert.False(t, appended.IsPublished.Bool, "an appended part starts as a draft")
		assert.False(t, appended.ReleasedAt.Valid, "and is not released")

		// The chain now reads as two parts, so the GM's schedule view and the
		// "Part N of M" labelling pick it up with no further work.
		chain, err := env.actionService.GetResultChain(ctx, draft.ID)
		require.NoError(t, err)
		require.Len(t, chain, 2)
		assert.Equal(t, int64(2), chain[0].PartCount)
	})

	t.Run("appends to the tail when given any member of the chain", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.draftChainRequest(0, 15, 30))
		require.NoError(t, err)

		// Anchored on the head, not the tail: the GM may be editing any part,
		// and the new part still belongs at the end.
		appended, err := env.actionService.AppendStagedPart(ctx, chain[0].ID, core.StagedResultPart{
			Content:      "Part four",
			DelayMinutes: 5,
		})
		require.NoError(t, err)

		assert.Equal(t, chain[2].ID, appended.ParentResultID.Int32, "must follow part 3, not the anchor")

		full, err := env.actionService.GetResultChain(ctx, chain[0].ID)
		require.NoError(t, err)
		require.Len(t, full, 4)
		assert.Equal(t, appended.ID, full[3].ID)
	})

	t.Run("copies the recipient from the chain", func(t *testing.T) {
		env := setupStagedTest(t)

		draft := env.createDraftResult(t)
		appended, err := env.actionService.AppendStagedPart(ctx, draft.ID, core.StagedResultPart{
			Content:      "...and misses!",
			DelayMinutes: 15,
		})
		require.NoError(t, err)

		// A chain cannot change recipient midway. The caller never supplies
		// these, so the invariant holds by construction rather than by check.
		assert.Equal(t, draft.UserID, appended.UserID)
		assert.Equal(t, draft.GameID, appended.GameID)
		assert.Equal(t, draft.PhaseID, appended.PhaseID)
		assert.Equal(t, draft.GmUserID, appended.GmUserID)
	})

	// The constraint the user set: the whole chain must be complete before
	// publishing. Appending afterwards would extend a scene mid-read.
	t.Run("refuses to append to a published chain", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.chainRequest(0, 15))
		require.NoError(t, err)
		require.True(t, chain[0].IsPublished.Bool)

		_, err = env.actionService.AppendStagedPart(ctx, chain[0].ID, core.StagedResultPart{
			Content:      "Too late",
			DelayMinutes: 5,
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, core.ErrCannotEditChain)
		assert.Contains(t, err.Error(), "already published")

		// And nothing was written despite the tail lookup succeeding.
		full, err := env.actionService.GetResultChain(ctx, chain[0].ID)
		require.NoError(t, err)
		assert.Len(t, full, 2, "the chain must be unchanged")
	})

	t.Run("enforces the max chain length", func(t *testing.T) {
		env := setupStagedTest(t)

		// A full-length draft chain: head plus MaxStagedChainLength-1 follow-ups.
		delays := make([]int32, core.MaxStagedChainLength)
		for i := 1; i < len(delays); i++ {
			delays[i] = 5
		}
		chain, err := env.actionService.CreateStagedResultChain(ctx, env.draftChainRequest(delays...))
		require.NoError(t, err)
		require.Len(t, chain, core.MaxStagedChainLength)

		_, err = env.actionService.AppendStagedPart(ctx, chain[0].ID, core.StagedResultPart{
			Content:      "One too many",
			DelayMinutes: 5,
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, core.ErrInvalidStagedChain)
	})

	t.Run("rejects a delay outside the allowed range", func(t *testing.T) {
		env := setupStagedTest(t)
		draft := env.createDraftResult(t)

		for _, delay := range []int32{0, core.MinStagedDelayMinutes - 1, core.MaxStagedDelayMinutes + 1} {
			_, err := env.actionService.AppendStagedPart(ctx, draft.ID, core.StagedResultPart{
				Content:      "Bad timing",
				DelayMinutes: delay,
			})
			require.Error(t, err, "delay %d must be rejected", delay)
			assert.ErrorIs(t, err, core.ErrInvalidStagedChain)
		}
	})
}

// Regression: publishing used to update only the row it was given, which was
// fine while every chain was created whole and published together. Appending to
// a draft broke that assumption — the head went out published while its
// follower stayed a draft, and GetDueStagedParts only considers published rows,
// so the follower was stranded permanently with no repair path but manual SQL.
func TestPublishActionResult_PublishesTheWholeChain(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	ctx := context.Background()

	t.Run("an appended follow-up is published along with the head", func(t *testing.T) {
		env := setupStagedTest(t)

		draft := env.createDraftResult(t)
		appended, err := env.actionService.AppendStagedPart(ctx, draft.ID, core.StagedResultPart{
			Content:      "...and misses!",
			DelayMinutes: 15,
		})
		require.NoError(t, err)
		require.False(t, appended.IsPublished.Bool, "follower starts as a draft")

		require.NoError(t, env.actionService.PublishActionResult(ctx, draft.ID, env.gmID))

		head := env.getResult(t, draft.ID)
		assert.True(t, head.IsPublished.Bool)
		assert.True(t, head.ReleasedAt.Valid, "the head is visible at once")

		follower := env.getResult(t, appended.ID)
		assert.True(t, follower.IsPublished.Bool,
			"an unpublished follower is invisible to the release worker and would never fire")
		assert.False(t, follower.ReleasedAt.Valid, "but it still waits for its timer")
	})

	// The end-to-end consequence: the stranded part would never have released.
	t.Run("the appended part actually releases when its delay elapses", func(t *testing.T) {
		env := setupStagedTest(t)

		draft := env.createDraftResult(t)
		appended, err := env.actionService.AppendStagedPart(ctx, draft.ID, core.StagedResultPart{
			Content:      "...and misses!",
			DelayMinutes: 15,
		})
		require.NoError(t, err)
		require.NoError(t, env.actionService.PublishActionResult(ctx, draft.ID, env.gmID))

		env.backdateRelease(t, draft.ID, 16)
		_, released, err := env.actionService.ReleaseDueStagedParts(ctx)
		require.NoError(t, err)

		assert.Equal(t, 1, released)
		assert.True(t, env.getResult(t, appended.ID).ReleasedAt.Valid)
	})

	t.Run("publishing from a follower publishes the head too", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.draftChainRequest(0, 15))
		require.NoError(t, err)

		// Anchored on part 2: the query climbs to the head before descending.
		require.NoError(t, env.actionService.PublishActionResult(ctx, chain[1].ID, env.gmID))

		assert.True(t, env.getResult(t, chain[0].ID).IsPublished.Bool)
		assert.True(t, env.getResult(t, chain[0].ID).ReleasedAt.Valid, "the head releases on publish")
		assert.True(t, env.getResult(t, chain[1].ID).IsPublished.Bool)
		assert.False(t, env.getResult(t, chain[1].ID).ReleasedAt.Valid)
	})

	t.Run("an ordinary single result is unaffected", func(t *testing.T) {
		env := setupStagedTest(t)

		draft := env.createDraftResult(t)
		require.NoError(t, env.actionService.PublishActionResult(ctx, draft.ID, env.gmID))

		published := env.getResult(t, draft.ID)
		assert.True(t, published.IsPublished.Bool)
		assert.True(t, published.ReleasedAt.Valid, "a result with no parent is visible at once")
	})
}

// Sheet updates belong to the chain's FINAL part and apply when that part is
// released, not when the chain is published. Otherwise publishing hands the
// player their reward while they are still reading whether they survived to
// earn it — the exact spoiler this feature exists to prevent.
func TestStagedChain_SheetUpdatesApplyOnRelease(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	ctx := context.Background()

	// stageReward creates a character and stages a sheet update on the given part.
	stageReward := func(t *testing.T, env *stagedTestEnv, resultID int32) int32 {
		t.Helper()
		characterService := &db.CharacterService{DB: env.testDB.Pool, Logger: env.actionService.Logger}
		userID := env.playerID
		character, err := characterService.CreateCharacter(ctx, db.CreateCharacterRequest{
			GameID:        env.gameID,
			UserID:        &userID,
			Name:          "Staged Reward Character",
			CharacterType: "player_character",
		})
		require.NoError(t, err)

		_, err = env.actionService.CreateDraftCharacterUpdate(ctx, core.CreateDraftCharacterUpdateRequest{
			ActionResultID: resultID,
			CharacterID:    character.ID,
			ModuleType:     "inventory",
			FieldName:      "items",
			FieldValue:     `["Sword of Winning"]`,
			FieldType:      "json",
			Operation:      "upsert",
		})
		require.NoError(t, err)
		return character.ID
	}

	hasReward := func(t *testing.T, env *stagedTestEnv, characterID int32) bool {
		t.Helper()
		var count int
		err := env.testDB.Pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM character_data
			 WHERE character_id = $1 AND module_type = 'inventory' AND field_name = 'items'`,
			characterID).Scan(&count)
		require.NoError(t, err)
		return count > 0
	}

	t.Run("publishing the chain does not apply the tail's sheet updates", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.draftChainRequest(0, 15))
		require.NoError(t, err)
		characterID := stageReward(t, env, chain[1].ID)

		require.NoError(t, env.actionService.PublishActionResult(ctx, chain[0].ID, env.gmID))

		assert.False(t, hasReward(t, env, characterID),
			"the reward must not land while the player is still reading part 1")
	})

	t.Run("releasing the tail applies them", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.draftChainRequest(0, 15))
		require.NoError(t, err)
		characterID := stageReward(t, env, chain[1].ID)

		require.NoError(t, env.actionService.PublishActionResult(ctx, chain[0].ID, env.gmID))
		env.backdateRelease(t, chain[0].ID, 16)

		_, released, err := env.actionService.ReleaseDueStagedParts(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, released)

		assert.True(t, hasReward(t, env, characterID),
			"the reward lands with the beat that earns it")
	})

	t.Run("released drafts are cleaned up so a later tick cannot reapply them", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.draftChainRequest(0, 15))
		require.NoError(t, err)
		stageReward(t, env, chain[1].ID)

		require.NoError(t, env.actionService.PublishActionResult(ctx, chain[0].ID, env.gmID))
		env.backdateRelease(t, chain[0].ID, 16)
		_, _, err = env.actionService.ReleaseDueStagedParts(ctx)
		require.NoError(t, err)

		count, err := env.actionService.GetDraftUpdateCount(ctx, chain[1].ID)
		require.NoError(t, err)
		assert.Zero(t, count, "drafts are consumed by the release, matching the publish path")
	})

	t.Run("an ordinary result still applies its updates at publish", func(t *testing.T) {
		env := setupStagedTest(t)

		draft := env.createDraftResult(t)
		characterID := stageReward(t, env, draft.ID)

		require.NoError(t, env.actionService.PublishActionResult(ctx, draft.ID, env.gmID))

		// A result with no chain releases at publish, so its updates apply then.
		assert.True(t, hasReward(t, env, characterID))
	})
}

func TestUpdateStagedPartDelay(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	ctx := context.Background()

	t.Run("retimes a draft part", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.draftChainRequest(0, 15))
		require.NoError(t, err)

		updated, err := env.actionService.UpdateStagedPartDelay(ctx, chain[1].ID, 45)
		require.NoError(t, err)
		assert.Equal(t, int32(45), updated.RevealDelayMinutes.Int32)
		assert.Equal(t, int32(45), env.getResult(t, chain[1].ID).RevealDelayMinutes.Int32)
	})

	// The case the user asked for specifically: the scene is live, the player is
	// watching a countdown, and the GM needs to move it.
	t.Run("retimes a published pending part", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.chainRequest(0, 15))
		require.NoError(t, err)
		require.True(t, chain[1].IsPublished.Bool, "published...")
		require.False(t, chain[1].ReleasedAt.Valid, "...but not yet released")

		updated, err := env.actionService.UpdateStagedPartDelay(ctx, chain[1].ID, 60)
		require.NoError(t, err)
		assert.Equal(t, int32(60), updated.RevealDelayMinutes.Int32)
		assert.False(t, updated.ReleasedAt.Valid, "retiming must not release it")
	})

	// Nothing reschedules on a retime, so this proves the new delay is actually
	// what the worker consults rather than a value stored and ignored.
	t.Run("the worker honours the new delay on its next tick", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.chainRequest(0, 60))
		require.NoError(t, err)

		// 30 minutes have passed: not due under the original 60.
		env.backdateRelease(t, chain[0].ID, 30)
		_, released, err := env.actionService.ReleaseDueStagedParts(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, released, "still waiting under the original delay")

		// Shorten to 15, which the same elapsed 30 minutes now exceeds.
		_, err = env.actionService.UpdateStagedPartDelay(ctx, chain[1].ID, 15)
		require.NoError(t, err)

		_, released, err = env.actionService.ReleaseDueStagedParts(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, released, "the shortened delay must make it due")
		assert.True(t, env.getResult(t, chain[1].ID).ReleasedAt.Valid)
	})

	// The mirror image: pushing a nearly-due part back out of reach.
	t.Run("extending a delay defers a part that was about to fire", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.chainRequest(0, 15))
		require.NoError(t, err)
		env.backdateRelease(t, chain[0].ID, 20) // Already overdue.

		_, err = env.actionService.UpdateStagedPartDelay(ctx, chain[1].ID, 120)
		require.NoError(t, err)

		_, released, err := env.actionService.ReleaseDueStagedParts(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, released, "the extended delay must hold it back")
		assert.False(t, env.getResult(t, chain[1].ID).ReleasedAt.Valid)
	})

	t.Run("refuses to retime a released part", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.chainRequest(0, 15))
		require.NoError(t, err)
		env.backdateRelease(t, chain[0].ID, 16)
		_, _, err = env.actionService.ReleaseDueStagedParts(ctx)
		require.NoError(t, err)
		require.True(t, env.getResult(t, chain[1].ID).ReleasedAt.Valid)

		_, err = env.actionService.UpdateStagedPartDelay(ctx, chain[1].ID, 60)
		require.Error(t, err, "the player has already read it")
		assert.ErrorIs(t, err, core.ErrCannotEditChain)
		assert.Contains(t, err.Error(), "already been released")
	})

	t.Run("refuses to retime a chain head", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.draftChainRequest(0, 15))
		require.NoError(t, err)

		// A head has no parent to measure a delay from, so there is no
		// schedule to change.
		_, err = env.actionService.UpdateStagedPartDelay(ctx, chain[0].ID, 30)
		require.Error(t, err)
		assert.ErrorIs(t, err, core.ErrCannotEditChain)
		assert.Contains(t, err.Error(), "chain head")
	})

	t.Run("rejects a delay outside the allowed range", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.draftChainRequest(0, 15))
		require.NoError(t, err)

		for _, delay := range []int32{0, core.MaxStagedDelayMinutes + 1} {
			_, err := env.actionService.UpdateStagedPartDelay(ctx, chain[1].ID, delay)
			require.Error(t, err, "delay %d must be rejected", delay)
			assert.ErrorIs(t, err, core.ErrInvalidStagedChain)
		}

		// And the stored value is untouched by a rejected edit.
		assert.Equal(t, int32(15), env.getResult(t, chain[1].ID).RevealDelayMinutes.Int32)
	})
}

func TestGetResultChain(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	ctx := context.Background()

	t.Run("returns the whole chain from any member", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.chainRequest(0, 15, 30))
		require.NoError(t, err)

		// Asking from the middle must yield the same chain as asking from the
		// head: callers should not need to know which part they hold.
		for _, anchor := range chain {
			got, err := env.actionService.GetResultChain(ctx, anchor.ID)
			require.NoError(t, err)
			require.Len(t, got, 3)

			assert.Equal(t, chain[0].ID, got[0].ID, "chain must start at the head")
			assert.Equal(t, int32(1), got[0].PartNumber)
			assert.Equal(t, int32(3), got[2].PartNumber)
			assert.Equal(t, int64(3), got[0].PartCount)
		}
	})
}
