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

	t.Run("the recipient cannot read unreleased parts", func(t *testing.T) {
		env := setupStagedTest(t)

		chain, err := env.actionService.CreateStagedResultChain(ctx, env.chainRequest(0, 15))
		require.NoError(t, err)

		queries := models.New(env.testDB.Pool)

		// GetUserResults is the player's read path and the gate the whole
		// feature rests on.
		visible, err := queries.GetUserResults(ctx, models.GetUserResultsParams{
			GameID: env.gameID,
			UserID: env.playerID,
		})
		require.NoError(t, err)
		require.Len(t, visible, 1, "only the head should be readable")
		assert.Equal(t, chain[0].ID, visible[0].ID)

		env.backdateRelease(t, chain[0].ID, 16)
		_, _, err = env.actionService.ReleaseDueStagedParts(ctx)
		require.NoError(t, err)

		visible, err = queries.GetUserResults(ctx, models.GetUserResultsParams{
			GameID: env.gameID,
			UserID: env.playerID,
		})
		require.NoError(t, err)
		assert.Len(t, visible, 2, "part 2 becomes readable only after release")
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
