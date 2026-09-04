package communities

import (
	"testing"
	"time"

	"actionphase/pkg/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newBanFixture builds a community with an owner, plus a bystander user to ban.
func newBanFixture(t *testing.T) (*CommunityService, *core.TestDatabase, *core.Community, *core.User, *core.User) {
	t.Helper()

	svc, testDB, ctx := newService(t)
	owner := testDB.CreateTestUser(t, "banowner", "banowner@example.com")
	target := testDB.CreateTestUser(t, "bantarget", "bantarget@example.com")

	community, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Bans", Slug: uniqueSlug(t, "bans"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	return svc, testDB, community, owner, target
}

func TestBanUser(t *testing.T) {
	svc, _, community, owner, target := newBanFixture(t)
	ctx := t.Context()

	reason := "repeatedly harassing other players"
	ban, err := svc.BanUser(ctx, community.ID, int32(owner.ID), &core.CreateCommunityBanRequest{
		UserID: int32(target.ID),
		Reason: &reason,
	})
	require.NoError(t, err)
	require.NotNil(t, ban)

	assert.Equal(t, int32(target.ID), ban.UserID)
	require.NotNil(t, ban.Reason)
	assert.Equal(t, reason, *ban.Reason)
	assert.Nil(t, ban.ExpiresAt, "no expiry means a permanent ban")
	assert.True(t, ban.IsActive)

	banned, err := svc.IsUserBanned(ctx, community.ID, int32(target.ID))
	require.NoError(t, err)
	assert.True(t, banned)
}

// A ban that has run out must stop being enforced WITHOUT anything having
// written to the row -- nothing does when the clock passes expires_at. If any
// enforcement query drops the expiry test, this is what catches it.
func TestBanUser_ExpiredBanDoesNotBlock(t *testing.T) {
	svc, _, community, owner, target := newBanFixture(t)
	ctx := t.Context()

	past := time.Now().Add(-1 * time.Hour)
	ban, err := svc.BanUser(ctx, community.ID, int32(owner.ID), &core.CreateCommunityBanRequest{
		UserID:    int32(target.ID),
		ExpiresAt: &past,
	})
	require.NoError(t, err)
	assert.False(t, ban.IsActive, "a ban whose expiry has passed is not being enforced")

	banned, err := svc.IsUserBanned(ctx, community.ID, int32(target.ID))
	require.NoError(t, err)
	assert.False(t, banned, "an expired ban must not block")

	// But the row survives: the management list has to show that the ban lapsed
	// rather than letting it silently disappear.
	bans, err := svc.ListBans(ctx, community.ID)
	require.NoError(t, err)
	require.Len(t, bans, 1)
	assert.False(t, bans[0].IsActive)
}

func TestBanUser_FutureExpiryStillBlocks(t *testing.T) {
	svc, _, community, owner, target := newBanFixture(t)
	ctx := t.Context()

	future := time.Now().Add(24 * time.Hour)
	_, err := svc.BanUser(ctx, community.ID, int32(owner.ID), &core.CreateCommunityBanRequest{
		UserID:    int32(target.ID),
		ExpiresAt: &future,
	})
	require.NoError(t, err)

	banned, err := svc.IsUserBanned(ctx, community.ID, int32(target.ID))
	require.NoError(t, err)
	assert.True(t, banned, "a ban that has not yet expired still blocks")
}

// This is the separation the whole feature exists for: three communities share
// this deployment, and each must be able to exclude a user without touching the
// others.
func TestBanUser_ScopedToOneCommunity(t *testing.T) {
	svc, testDB, communityA, owner, target := newBanFixture(t)
	ctx := t.Context()

	communityB, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Other", Slug: uniqueSlug(t, "other"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)
	_ = testDB

	_, err = svc.BanUser(ctx, communityA.ID, int32(owner.ID), &core.CreateCommunityBanRequest{
		UserID: int32(target.ID),
	})
	require.NoError(t, err)

	bannedA, err := svc.IsUserBanned(ctx, communityA.ID, int32(target.ID))
	require.NoError(t, err)
	assert.True(t, bannedA)

	bannedB, err := svc.IsUserBanned(ctx, communityB.ID, int32(target.ID))
	require.NoError(t, err)
	assert.False(t, bannedB, "a ban in one community must not reach into another")
}

// Re-banning is how a moderator edits a reason or extends an expiry. If it
// errored, they would be pushed into unban-then-reban, which loses banned_at.
func TestBanUser_ReBanUpdatesInPlace(t *testing.T) {
	svc, _, community, owner, target := newBanFixture(t)
	ctx := t.Context()

	first := "first reason"
	_, err := svc.BanUser(ctx, community.ID, int32(owner.ID), &core.CreateCommunityBanRequest{
		UserID: int32(target.ID), Reason: &first,
	})
	require.NoError(t, err)

	second := "second reason"
	updated, err := svc.BanUser(ctx, community.ID, int32(owner.ID), &core.CreateCommunityBanRequest{
		UserID: int32(target.ID), Reason: &second,
	})
	require.NoError(t, err, "re-banning an already-banned user must update, not fail")
	require.NotNil(t, updated.Reason)
	assert.Equal(t, second, *updated.Reason)

	bans, err := svc.ListBans(ctx, community.ID)
	require.NoError(t, err)
	assert.Len(t, bans, 1, "a re-ban must not stack a second row")
}

// The audit log has to distinguish a fresh ban from an edit, or the history
// reads as though the user was unbanned and re-banned in between.
func TestBanUser_ReBanLogsModifiedNotBanned(t *testing.T) {
	svc, _, community, owner, target := newBanFixture(t)
	ctx := t.Context()

	req := &core.CreateCommunityBanRequest{UserID: int32(target.ID)}
	_, err := svc.BanUser(ctx, community.ID, int32(owner.ID), req)
	require.NoError(t, err)
	_, err = svc.BanUser(ctx, community.ID, int32(owner.ID), req)
	require.NoError(t, err)

	events, err := svc.ListBanEvents(ctx, community.ID, 0, 0)
	require.NoError(t, err)
	require.Len(t, events, 2)

	// Newest first.
	assert.Equal(t, core.BanEventModified, events[0].Action)
	assert.Equal(t, core.BanEventBanned, events[1].Action)
}

// A moderator who is also banned is a state no enforcement path knows how to
// read. The roster must be edited first -- and that is owner-only, unlike
// banning, so this is a real escalation guard and not just tidiness.
func TestBanUser_CannotBanCommunityStaff(t *testing.T) {
	svc, testDB, community, owner, _ := newBanFixture(t)
	ctx := t.Context()

	t.Run("owner", func(t *testing.T) {
		_, err := svc.BanUser(ctx, community.ID, int32(owner.ID), &core.CreateCommunityBanRequest{
			UserID: int32(owner.ID),
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, core.ErrCannotBanCommunityStaff)
	})

	t.Run("moderator", func(t *testing.T) {
		mod := testDB.CreateTestUser(t, "staffmod", "staffmod@example.com")
		_, err := svc.AddModerator(ctx, community.ID, int32(mod.ID), int32(owner.ID))
		require.NoError(t, err)

		_, err = svc.BanUser(ctx, community.ID, int32(owner.ID), &core.CreateCommunityBanRequest{
			UserID: int32(mod.ID),
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, core.ErrCannotBanCommunityStaff)

		banned, err := svc.IsUserBanned(ctx, community.ID, int32(mod.ID))
		require.NoError(t, err)
		assert.False(t, banned, "the rejected ban must not have been written")
	})
}

func TestUnbanUser_RestoresAccess(t *testing.T) {
	svc, _, community, owner, target := newBanFixture(t)
	ctx := t.Context()

	_, err := svc.BanUser(ctx, community.ID, int32(owner.ID), &core.CreateCommunityBanRequest{
		UserID: int32(target.ID),
	})
	require.NoError(t, err)

	require.NoError(t, svc.UnbanUser(ctx, community.ID, int32(target.ID), int32(owner.ID)))

	banned, err := svc.IsUserBanned(ctx, community.ID, int32(target.ID))
	require.NoError(t, err)
	assert.False(t, banned)

	bans, err := svc.ListBans(ctx, community.ID)
	require.NoError(t, err)
	assert.Empty(t, bans)
}

// The audit log is the ONLY surviving record once the ban row is deleted. If
// the unban event were not written -- or were written outside the transaction
// and lost -- the history would be silently incomplete in exactly the disputes
// it exists to settle.
func TestUnbanUser_EventSurvivesBanDeletion(t *testing.T) {
	svc, _, community, owner, target := newBanFixture(t)
	ctx := t.Context()

	reason := "spoilers in the common room"
	_, err := svc.BanUser(ctx, community.ID, int32(owner.ID), &core.CreateCommunityBanRequest{
		UserID: int32(target.ID), Reason: &reason,
	})
	require.NoError(t, err)
	require.NoError(t, svc.UnbanUser(ctx, community.ID, int32(target.ID), int32(owner.ID)))

	bans, err := svc.ListBans(ctx, community.ID)
	require.NoError(t, err)
	require.Empty(t, bans, "the ban row is gone")

	events, err := svc.ListBanEvents(ctx, community.ID, 0, 0)
	require.NoError(t, err)
	require.Len(t, events, 2, "both the ban and the unban survive it")

	assert.Equal(t, core.BanEventUnbanned, events[0].Action)
	// The unban event snapshots what the BAN said -- that is the thing a later
	// dispute needs, and it no longer exists anywhere else.
	require.NotNil(t, events[0].Reason)
	assert.Equal(t, reason, *events[0].Reason)
}

func TestUnbanUser_NotFound(t *testing.T) {
	svc, _, community, owner, target := newBanFixture(t)
	ctx := t.Context()

	err := svc.UnbanUser(ctx, community.ID, int32(target.ID), int32(owner.ID))
	require.Error(t, err)
	assert.ErrorIs(t, err, core.ErrBanNotFound)
}

func TestListBans_CarriesUserDetails(t *testing.T) {
	svc, _, community, owner, target := newBanFixture(t)
	ctx := t.Context()

	_, err := svc.BanUser(ctx, community.ID, int32(owner.ID), &core.CreateCommunityBanRequest{
		UserID: int32(target.ID),
	})
	require.NoError(t, err)

	bans, err := svc.ListBans(ctx, community.ID)
	require.NoError(t, err)
	require.Len(t, bans, 1)

	// Without the joined username the UI could only show a numeric id.
	assert.Equal(t, target.Username, bans[0].Username)
	require.NotNil(t, bans[0].BannedByUsername)
	assert.Equal(t, owner.Username, *bans[0].BannedByUsername)
}

func TestListBannedCommunityIDs(t *testing.T) {
	svc, _, communityA, owner, target := newBanFixture(t)
	ctx := t.Context()

	communityB, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "B", Slug: uniqueSlug(t, "b"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	_, err = svc.BanUser(ctx, communityA.ID, int32(owner.ID), &core.CreateCommunityBanRequest{
		UserID: int32(target.ID),
	})
	require.NoError(t, err)

	// An expired ban must NOT filter a community out of the picker -- the user
	// is allowed back in.
	past := time.Now().Add(-time.Hour)
	_, err = svc.BanUser(ctx, communityB.ID, int32(owner.ID), &core.CreateCommunityBanRequest{
		UserID: int32(target.ID), ExpiresAt: &past,
	})
	require.NoError(t, err)

	ids, err := svc.ListBannedCommunityIDs(ctx, int32(target.ID))
	require.NoError(t, err)
	assert.Equal(t, []int32{communityA.ID}, ids)
}

// banIsActive (Go, used for the response field) and the SQL expiry test (used
// for enforcement) are two implementations of one rule. If they ever disagree,
// the UI would show a ban as lapsed while it still blocked, or vice versa.
func TestBanIsActiveAgreesWithSQL(t *testing.T) {
	svc, _, community, owner, target := newBanFixture(t)
	ctx := t.Context()

	cases := []struct {
		name      string
		expiresAt *time.Time
	}{
		{"permanent", nil},
		{"future expiry", ptrTime(time.Now().Add(time.Hour))},
		{"past expiry", ptrTime(time.Now().Add(-time.Hour))},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ban, err := svc.BanUser(ctx, community.ID, int32(owner.ID), &core.CreateCommunityBanRequest{
				UserID: int32(target.ID), ExpiresAt: tc.expiresAt,
			})
			require.NoError(t, err)

			enforced, err := svc.IsUserBanned(ctx, community.ID, int32(target.ID))
			require.NoError(t, err)

			assert.Equal(t, enforced, ban.IsActive,
				"the computed IsActive must match what SQL actually enforces")
		})
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
