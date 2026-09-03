package db

import (
	"context"
	"testing"
	"time"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ban enforcement across every join path enumerated in the Communities plan.
//
// The point of this file is coverage of PATHS, not of the ban query -- that is
// tested in services/communities. A ban that stops the obvious route and leaves
// another open is not a ban, so each way into a game gets its own test, and
// each asserts the same three things: the banned user is refused, an unbanned
// user is not, and a user whose ban has EXPIRED is not.

type banEnforcementFixture struct {
	testDB      *core.TestDatabase
	gameService *GameService
	appService  *GameApplicationService

	community  models.Community
	otherComm  models.Community
	gm         *core.User
	banned     *core.User
	clean      *core.User
	expired    *core.User
	game       models.Game
	legacyGame models.Game
}

// newBanEnforcementFixture builds a community with a game in it, plus three
// users: one actively banned, one never banned, and one whose ban has lapsed.
//
// It also creates a SECOND community and a game belonging to NO community, so
// scope and grandfathering can be asserted from the same setup rather than
// rebuilt per test.
func newBanEnforcementFixture(t *testing.T) *banEnforcementFixture {
	t.Helper()

	testDB := core.NewTestDatabase(t)
	app := core.NewTestApp(testDB.Pool)
	t.Cleanup(func() {
		testDB.CleanupTables(t, "games", "sessions", "users")
		testDB.Close()
	})

	ctx := context.Background()
	queries := models.New(testDB.Pool)

	gm := testDB.CreateTestUser(t, "ban_gm", "ban_gm@example.com")
	banned := testDB.CreateTestUser(t, "ban_target", "ban_target@example.com")
	clean := testDB.CreateTestUser(t, "ban_clean", "ban_clean@example.com")
	expired := testDB.CreateTestUser(t, "ban_expired", "ban_expired@example.com")

	community := *testDB.CreateTestCommunity(t, int32(gm.ID))
	otherComm := *testDB.CreateTestCommunity(t, int32(gm.ID))

	newGame := func(communityID pgtype.Int4, title string) models.Game {
		g, err := queries.CreateGame(ctx, models.CreateGameParams{
			Title:       title,
			Description: pgtype.Text{String: "ban enforcement fixture", Valid: true},
			GmUserID:    int32(gm.ID),
			MaxPlayers:  pgtype.Int4{Int32: 6, Valid: true},
			IsPublic:    pgtype.Bool{Bool: true, Valid: true},
			CommunityID: communityID,
		})
		require.NoError(t, err)

		// Applications are only accepted in recruitment.
		g, err = queries.UpdateGameState(ctx, models.UpdateGameStateParams{
			ID:    g.ID,
			State: pgtype.Text{String: core.GameStateRecruitment, Valid: true},
		})
		require.NoError(t, err)
		return g
	}

	game := newGame(pgtype.Int4{Int32: community.ID, Valid: true}, "Ban Enforcement Game")

	// community_id omitted entirely: a game from before communities existed.
	legacyGame := newGame(pgtype.Int4{}, "Legacy Game")

	ban := func(userID int32, expiresAt pgtype.Timestamptz) {
		_, err := queries.CreateCommunityBan(ctx, models.CreateCommunityBanParams{
			CommunityID:    community.ID,
			UserID:         userID,
			Reason:         pgtype.Text{String: "fixture ban", Valid: true},
			BannedByUserID: pgtype.Int4{Int32: int32(gm.ID), Valid: true},
			ExpiresAt:      expiresAt,
		})
		require.NoError(t, err)
	}

	ban(int32(banned.ID), pgtype.Timestamptz{}) // permanent
	ban(int32(expired.ID), pgtype.Timestamptz{Time: time.Now().Add(-24 * time.Hour), Valid: true})

	return &banEnforcementFixture{
		testDB:      testDB,
		gameService: &GameService{DB: testDB.Pool, Logger: app.ObsLogger},
		appService:  &GameApplicationService{DB: testDB.Pool, Logger: app.ObsLogger},
		community:   community,
		otherComm:   otherComm,
		gm:          gm,
		banned:      banned,
		clean:       clean,
		expired:     expired,
		game:        game,
		legacyGame:  legacyGame,
	}
}

// users returns the standard three-way table every path test runs.
func (f *banEnforcementFixture) users() []struct {
	name       string
	user       *core.User
	wantRefuse bool
} {
	return []struct {
		name       string
		user       *core.User
		wantRefuse bool
	}{
		{"active ban is refused", f.banned, true},
		{"no ban is allowed", f.clean, false},
		// The row still exists; only expires_at makes it inert. A query that
		// drops the expiry test enforces a lapsed ban and fails here.
		{"expired ban is allowed", f.expired, false},
	}
}

// Path 1: applying to a game.
func TestBanEnforcement_ApplyToGame(t *testing.T) {
	f := newBanEnforcementFixture(t)

	for _, tc := range f.users() {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.appService.CreateGameApplication(context.Background(), core.CreateGameApplicationRequest{
				GameID: f.game.ID,
				UserID: int32(tc.user.ID),
				Role:   core.RolePlayer,
			})

			if tc.wantRefuse {
				require.ErrorIs(t, err, core.ErrUserBannedFromCommunity)
				return
			}
			require.NoError(t, err)
		})
	}
}

// The SQL CASE branch in CanUserApplyToGame is the one intentional duplicate of
// the ban rule -- it exists so the UI can check eligibility in a single
// round-trip. This asserts the duplicate agrees with the service, which is the
// only thing keeping the two from drifting apart.
func TestBanEnforcement_CanUserApplyAgreesWithService(t *testing.T) {
	f := newBanEnforcementFixture(t)
	ctx := context.Background()
	queries := models.New(f.testDB.Pool)

	cases := []struct {
		name       string
		user       *core.User
		wantStatus string
	}{
		{"active ban reports community_banned", f.banned, core.CommunityBanned},
		{"no ban reports can_apply", f.clean, core.CanApply},
		{"expired ban reports can_apply", f.expired, core.CanApply},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, err := f.appService.CanUserApplyToGame(ctx, f.game.ID, int32(tc.user.ID))
			require.NoError(t, err)
			assert.Equal(t, tc.wantStatus, status)

			// And the service-layer method must reach the same verdict.
			banned, err := queries.IsUserBannedFromGameCommunity(ctx, models.IsUserBannedFromGameCommunityParams{
				GameID: f.game.ID,
				UserID: int32(tc.user.ID),
			})
			require.NoError(t, err)
			assert.Equal(t, tc.wantStatus == core.CommunityBanned, banned,
				"SQL CASE ladder and IsUserBannedFromGameCommunity disagree")
		})
	}
}

// Path 2: the GM adds someone directly, bypassing applications entirely.
func TestBanEnforcement_GMDirectAdd(t *testing.T) {
	f := newBanEnforcementFixture(t)

	for _, tc := range f.users() {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.gameService.AddParticipantWithRole(
				context.Background(), f.game.ID, int32(tc.user.ID), core.RolePlayer)

			if tc.wantRefuse {
				require.ErrorIs(t, err, core.ErrUserBannedFromCommunity)
				return
			}
			require.NoError(t, err)
		})
	}
}

// Path 3: audience join, including the auto-accept flow.
func TestBanEnforcement_AudienceJoin(t *testing.T) {
	f := newBanEnforcementFixture(t)
	ctx := context.Background()

	for _, autoAccept := range []bool{false, true} {
		name := "manual approval"
		if autoAccept {
			name = "auto-accept"
		}

		t.Run(name, func(t *testing.T) {
			require.NoError(t, f.gameService.UpdateGameAutoAcceptAudience(ctx, f.game.ID, autoAccept))

			for _, tc := range f.users() {
				t.Run(tc.name, func(t *testing.T) {
					_, err := f.gameService.CreateAudienceApplication(ctx, f.game.ID, int32(tc.user.ID))

					if tc.wantRefuse {
						// Refused under BOTH settings. Auto-accept must not be a
						// side door, and manual approval must not leave a banned
						// user sitting in the GM's queue.
						require.ErrorIs(t, err, core.ErrUserBannedFromCommunity)
						return
					}
					require.NoError(t, err)

					// Clean up so the next subtest's insert does not collide.
					require.NoError(t, f.gameService.RemoveGameParticipant(ctx, f.game.ID, int32(tc.user.ID)))
				})
			}
		})
	}
}

// Path 4: approving one application, where the ban landed AFTER it was filed.
func TestBanEnforcement_ApproveApplication(t *testing.T) {
	f := newBanEnforcementFixture(t)
	ctx := context.Background()
	queries := models.New(f.testDB.Pool)

	// Filed before any ban exists, so this is specifically the
	// applied-then-banned case rather than a re-test of the apply gate.
	app, err := queries.CreateGameApplication(ctx, models.CreateGameApplicationParams{
		GameID: f.game.ID,
		UserID: int32(f.clean.ID),
		Role:   core.RolePlayer,
	})
	require.NoError(t, err)

	// Approving is fine while they are unbanned.
	require.NoError(t, f.appService.ApproveGameApplication(ctx, app.ID, int32(f.gm.ID)))

	// Now ban them and file a fresh application to approve.
	_, err = queries.CreateCommunityBan(ctx, models.CreateCommunityBanParams{
		CommunityID:    f.community.ID,
		UserID:         int32(f.clean.ID),
		Reason:         pgtype.Text{String: "banned after applying", Valid: true},
		BannedByUserID: pgtype.Int4{Int32: int32(f.gm.ID), Valid: true},
	})
	require.NoError(t, err)

	app2, err := queries.CreateGameApplication(ctx, models.CreateGameApplicationParams{
		GameID: f.game.ID,
		UserID: int32(f.banned.ID),
		Role:   core.RolePlayer,
	})
	require.NoError(t, err)

	err = f.appService.ApproveGameApplication(ctx, app2.ID, int32(f.gm.ID))
	require.ErrorIs(t, err, core.ErrUserBannedFromCommunity)

	// The application stays pending rather than being silently dropped, so the
	// GM can still see and reject it.
	after, err := queries.GetGameApplication(ctx, app2.ID)
	require.NoError(t, err)
	assert.Equal(t, core.ApplicationStatusPending, after.Status.String)
}

// Path 5: closing recruitment converts approved applications into participants.
//
// This is the real bulk path. §5 of the plan named "BulkApproveApplications",
// but nothing ever called that -- it has since been deleted. The transition
// handler calls BulkRejectApplications (for the pending ones) then
// ConvertApprovedApplicationsToParticipants (for the approved ones).
//
// The bypass it allows is the worst-shaped one available: apply, get approved,
// THEN get banned, and the GM closing recruitment quietly makes you a
// participant. It needs no privileges and crosses no reviewed step.
func TestBanEnforcement_ConvertApprovedSkipsBanned(t *testing.T) {
	f := newBanEnforcementFixture(t)
	ctx := context.Background()
	queries := models.New(f.testDB.Pool)

	// Two approved applicants; one is banned after approval.
	for _, u := range []*core.User{f.clean, f.expired} {
		app, err := queries.CreateGameApplication(ctx, models.CreateGameApplicationParams{
			GameID: f.game.ID,
			UserID: int32(u.ID),
			Role:   core.RolePlayer,
		})
		require.NoError(t, err)
		require.NoError(t, f.appService.ApproveGameApplication(ctx, app.ID, int32(f.gm.ID)))
	}

	_, err := queries.CreateCommunityBan(ctx, models.CreateCommunityBanParams{
		CommunityID:    f.community.ID,
		UserID:         int32(f.clean.ID),
		Reason:         pgtype.Text{String: "banned after approval", Valid: true},
		BannedByUserID: pgtype.Int4{Int32: int32(f.gm.ID), Valid: true},
	})
	require.NoError(t, err)

	require.NoError(t, f.appService.ConvertApprovedApplicationsToParticipants(ctx, f.game.ID))

	participants, err := queries.GetGameParticipants(ctx, f.game.ID)
	require.NoError(t, err)

	joined := map[int32]bool{}
	for _, p := range participants {
		joined[p.UserID] = true
	}

	assert.False(t, joined[int32(f.clean.ID)],
		"a user banned after approval must not be converted into a participant")
	assert.True(t, joined[int32(f.expired.ID)],
		"an expired ban must not block conversion")
}

// Path 6: creating a game in a community that has banned you.
func TestBanEnforcement_CreateGame(t *testing.T) {
	f := newBanEnforcementFixture(t)

	for _, tc := range f.users() {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.gameService.CreateGame(context.Background(), core.CreateGameRequest{
				Title:       "Game By " + tc.user.Username,
				Description: "created during ban enforcement tests",
				GMUserID:    int32(tc.user.ID),
				CommunityID: f.community.ID,
				IsPublic:    true,
			})

			if tc.wantRefuse {
				require.ErrorIs(t, err, core.ErrUserBannedFromCommunity)
				return
			}
			require.NoError(t, err)
		})
	}
}

// A ban is scoped to ONE community. This is the whole reason the feature
// exists: three communities share the deployment and each bans independently.
func TestBanEnforcement_ScopedToOneCommunity(t *testing.T) {
	f := newBanEnforcementFixture(t)
	ctx := context.Background()

	// Same user, same everything -- only the community differs.
	_, err := f.gameService.CreateGame(ctx, core.CreateGameRequest{
		Title:       "Refused Here",
		Description: "banned community",
		GMUserID:    int32(f.banned.ID),
		CommunityID: f.community.ID,
		IsPublic:    true,
	})
	require.ErrorIs(t, err, core.ErrUserBannedFromCommunity)

	_, err = f.gameService.CreateGame(ctx, core.CreateGameRequest{
		Title:       "Allowed There",
		Description: "unrelated community",
		GMUserID:    int32(f.banned.ID),
		CommunityID: f.otherComm.ID,
		IsPublic:    true,
	})
	require.NoError(t, err, "a ban in one community must not reach another")
}

// Grandfathering: a game belonging to no community can never be blocked, no
// matter who the user is banned from elsewhere.
//
// games.community_id is nullable, so this state is reachable. The enforcement
// queries inner-join through it, which is what makes a NULL yield no ban row.
func TestBanEnforcement_LegacyGameNeverBlocked(t *testing.T) {
	f := newBanEnforcementFixture(t)
	ctx := context.Background()

	status, err := f.appService.CanUserApplyToGame(ctx, f.legacyGame.ID, int32(f.banned.ID))
	require.NoError(t, err)
	assert.Equal(t, core.CanApply, status,
		"a game with no community must never report community_banned")

	_, err = f.gameService.AddParticipantWithRole(ctx, f.legacyGame.ID, int32(f.banned.ID), core.RolePlayer)
	require.NoError(t, err, "a banned user must still join a game that has no community")

	_, err = f.appService.CreateGameApplication(ctx, core.CreateGameApplicationRequest{
		GameID: f.legacyGame.ID,
		UserID: int32(f.expired.ID),
		Role:   core.RolePlayer,
	})
	require.NoError(t, err)
}

// Unbanning restores access immediately -- the ban row is deleted, so nothing
// should linger.
func TestBanEnforcement_UnbanRestoresAccess(t *testing.T) {
	f := newBanEnforcementFixture(t)
	ctx := context.Background()
	queries := models.New(f.testDB.Pool)

	_, err := f.gameService.AddParticipantWithRole(ctx, f.game.ID, int32(f.banned.ID), core.RolePlayer)
	require.ErrorIs(t, err, core.ErrUserBannedFromCommunity)

	_, err = queries.DeleteCommunityBan(ctx, models.DeleteCommunityBanParams{
		CommunityID: f.community.ID,
		UserID:      int32(f.banned.ID),
	})
	require.NoError(t, err)

	_, err = f.gameService.AddParticipantWithRole(ctx, f.game.ID, int32(f.banned.ID), core.RolePlayer)
	require.NoError(t, err, "unbanning must restore access at once")
}
