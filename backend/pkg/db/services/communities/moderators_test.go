package communities

import (
	"testing"

	"actionphase/pkg/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddModerator(t *testing.T) {
	svc, testDB, ctx := newService(t)
	owner := testDB.CreateTestUser(t, "owner", "owner@example.com")
	mod := testDB.CreateTestUser(t, "mod", "mod@example.com")

	community, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Mods", Slug: uniqueSlug(t, "mods"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	added, err := svc.AddModerator(ctx, community.ID, int32(mod.ID), int32(owner.ID))
	require.NoError(t, err)
	require.NotNil(t, added)

	assert.Equal(t, community.ID, added.CommunityID)
	assert.Equal(t, int32(mod.ID), added.UserID)
	require.NotNil(t, added.GrantedByUserID)
	assert.Equal(t, int32(owner.ID), *added.GrantedByUserID)
}

// The owner is deliberately not a row in community_moderators: ownership
// already confers every moderator power, and a duplicate owner row would let
// someone strip an owner's standing by deleting a moderator row while
// ownership itself stayed put.
func TestAddModerator_OwnerRejected(t *testing.T) {
	svc, testDB, ctx := newService(t)
	owner := testDB.CreateTestUser(t, "owner", "owner@example.com")

	community, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Self", Slug: uniqueSlug(t, "self"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	_, err = svc.AddModerator(ctx, community.ID, int32(owner.ID), int32(owner.ID))
	require.Error(t, err)
	assert.ErrorIs(t, err, core.ErrOwnerCannotBeModerator)

	// And the roster must remain genuinely empty, not merely un-erroring.
	mods, err := svc.ListModerators(ctx, community.ID)
	require.NoError(t, err)
	assert.Empty(t, mods, "the owner must never appear on the moderator roster")
}

func TestAddModerator_DuplicateRejected(t *testing.T) {
	svc, testDB, ctx := newService(t)
	owner := testDB.CreateTestUser(t, "owner", "owner@example.com")
	mod := testDB.CreateTestUser(t, "mod", "mod@example.com")

	community, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Dupes", Slug: uniqueSlug(t, "dupes"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	_, err = svc.AddModerator(ctx, community.ID, int32(mod.ID), int32(owner.ID))
	require.NoError(t, err)

	_, err = svc.AddModerator(ctx, community.ID, int32(mod.ID), int32(owner.ID))
	require.Error(t, err)
	assert.ErrorIs(t, err, core.ErrAlreadyModerator,
		"a duplicate grant must surface as a domain error, not a raw constraint failure")
}

func TestAddModerator_UnknownCommunity(t *testing.T) {
	svc, testDB, ctx := newService(t)
	mod := testDB.CreateTestUser(t, "mod", "mod@example.com")

	_, err := svc.AddModerator(ctx, 999999, int32(mod.ID), 0)
	assert.ErrorIs(t, err, core.ErrCommunityNotFound)
}

func TestRemoveModerator(t *testing.T) {
	svc, testDB, ctx := newService(t)
	owner := testDB.CreateTestUser(t, "owner", "owner@example.com")
	mod := testDB.CreateTestUser(t, "mod", "mod@example.com")

	community, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Remove", Slug: uniqueSlug(t, "remove"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	_, err = svc.AddModerator(ctx, community.ID, int32(mod.ID), int32(owner.ID))
	require.NoError(t, err)

	require.NoError(t, svc.RemoveModerator(ctx, community.ID, int32(mod.ID)))

	role, err := svc.GetRole(ctx, community.ID, int32(mod.ID))
	require.NoError(t, err)
	assert.Equal(t, core.CommunityRoleNone, role)
}

// Removing someone who does not moderate is a no-op: the caller's intent
// already holds, so it must not error.
func TestRemoveModerator_NonModeratorIsNoOp(t *testing.T) {
	svc, testDB, ctx := newService(t)
	owner := testDB.CreateTestUser(t, "owner", "owner@example.com")
	stranger := testDB.CreateTestUser(t, "stranger", "stranger@example.com")

	community, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "NoOp", Slug: uniqueSlug(t, "noop"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	assert.NoError(t, svc.RemoveModerator(ctx, community.ID, int32(stranger.ID)))
}

func TestListModerators_CarriesUserDetails(t *testing.T) {
	svc, testDB, ctx := newService(t)
	owner := testDB.CreateTestUser(t, "owner", "owner@example.com")
	mod := testDB.CreateTestUser(t, "mod", "mod@example.com")

	community, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Roster", Slug: uniqueSlug(t, "roster"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	_, err = svc.AddModerator(ctx, community.ID, int32(mod.ID), int32(owner.ID))
	require.NoError(t, err)

	mods, err := svc.ListModerators(ctx, community.ID)
	require.NoError(t, err)
	require.Len(t, mods, 1)

	assert.Equal(t, int32(mod.ID), mods[0].UserID)
	assert.Equal(t, mod.Username, mods[0].Username, "the roster must carry the joined username")
	require.NotNil(t, mods[0].GrantedByUsername)
	assert.Equal(t, owner.Username, *mods[0].GrantedByUsername)
}

// The roster lists moderator rows only. Surfaces wanting "everyone who can
// moderate" render the owner separately from Community.OwnerUserID.
func TestListModerators_ExcludesOwner(t *testing.T) {
	svc, testDB, ctx := newService(t)
	owner := testDB.CreateTestUser(t, "owner", "owner@example.com")
	mod := testDB.CreateTestUser(t, "mod", "mod@example.com")

	community, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "OwnerOut", Slug: uniqueSlug(t, "ownerout"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	_, err = svc.AddModerator(ctx, community.ID, int32(mod.ID), int32(owner.ID))
	require.NoError(t, err)

	mods, err := svc.ListModerators(ctx, community.ID)
	require.NoError(t, err)
	require.Len(t, mods, 1)
	assert.NotEqual(t, int32(owner.ID), mods[0].UserID)
}

func TestGetRole(t *testing.T) {
	svc, testDB, ctx := newService(t)
	owner := testDB.CreateTestUser(t, "owner", "owner@example.com")
	mod := testDB.CreateTestUser(t, "mod", "mod@example.com")
	stranger := testDB.CreateTestUser(t, "stranger", "stranger@example.com")

	community, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Roles", Slug: uniqueSlug(t, "roles"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	_, err = svc.AddModerator(ctx, community.ID, int32(mod.ID), int32(owner.ID))
	require.NoError(t, err)

	tests := []struct {
		name   string
		userID int32
		want   core.CommunityRole
	}{
		{name: "owner", userID: int32(owner.ID), want: core.CommunityRoleOwner},
		{name: "moderator", userID: int32(mod.ID), want: core.CommunityRoleModerator},
		{name: "stranger", userID: int32(stranger.ID), want: core.CommunityRoleNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role, err := svc.GetRole(ctx, community.ID, tt.userID)
			require.NoError(t, err)
			assert.Equal(t, tt.want, role)
		})
	}
}

// Owner outranks moderator. If an owner somehow also held a moderator row, the
// role must still resolve to owner -- otherwise they would lose roster powers.
func TestGetRole_OwnerOutranksModerator(t *testing.T) {
	svc, testDB, ctx := newService(t)
	owner := testDB.CreateTestUser(t, "owner", "owner@example.com")
	other := testDB.CreateTestUser(t, "other", "other@example.com")

	community, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Rank", Slug: uniqueSlug(t, "rank"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	// Grant a moderator row, then hand that user ownership. They now hold both.
	_, err = svc.AddModerator(ctx, community.ID, int32(other.ID), int32(owner.ID))
	require.NoError(t, err)

	otherID := int32(other.ID)
	_, err = svc.UpdateCommunity(ctx, community.ID, &core.UpdateCommunityRequest{OwnerUserID: &otherID})
	require.NoError(t, err)

	role, err := svc.GetRole(ctx, community.ID, otherID)
	require.NoError(t, err)
	assert.Equal(t, core.CommunityRoleOwner, role,
		"holding both a moderator row and ownership must resolve to owner")
}

// A community scopes its own roster: moderating one must confer nothing in
// another. This is the multi-tenant property the whole feature rests on.
func TestGetRole_ScopedPerCommunity(t *testing.T) {
	svc, testDB, ctx := newService(t)
	owner := testDB.CreateTestUser(t, "owner", "owner@example.com")
	mod := testDB.CreateTestUser(t, "mod", "mod@example.com")

	first, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "First", Slug: uniqueSlug(t, "scopea"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	second, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Second", Slug: uniqueSlug(t, "scopeb"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	_, err = svc.AddModerator(ctx, first.ID, int32(mod.ID), int32(owner.ID))
	require.NoError(t, err)

	roleFirst, err := svc.GetRole(ctx, first.ID, int32(mod.ID))
	require.NoError(t, err)
	assert.Equal(t, core.CommunityRoleModerator, roleFirst)

	roleSecond, err := svc.GetRole(ctx, second.ID, int32(mod.ID))
	require.NoError(t, err)
	assert.Equal(t, core.CommunityRoleNone, roleSecond,
		"moderating one community must confer nothing in another")
}
