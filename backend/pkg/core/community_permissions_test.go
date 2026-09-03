package core

import (
	"context"
	"testing"

	models "actionphase/pkg/db/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedCommunity creates a community owned by ownerID and returns its ID.
// The permission helpers live in core, so this reaches for the generated
// queries directly rather than importing the service package (which would be a
// cycle).
func seedCommunity(t *testing.T, td *TestDatabase, ownerID int32, slug string) int32 {
	t.Helper()

	row, err := models.New(td.Pool).CreateCommunity(context.Background(), models.CreateCommunityParams{
		Name:        "Permissions Fixture",
		Slug:        slug,
		OwnerUserID: ownerID,
	})
	require.NoError(t, err)
	return row.ID
}

func seedModerator(t *testing.T, td *TestDatabase, communityID, userID int32) {
	t.Helper()

	_, err := models.New(td.Pool).AddCommunityModerator(context.Background(), models.AddCommunityModeratorParams{
		CommunityID: communityID,
		UserID:      userID,
	})
	require.NoError(t, err)
}

func TestGetCommunityRole_Permissions(t *testing.T) {
	td := NewTestDatabase(t)
	defer td.Close()
	// communities.owner_user_id is ON DELETE RESTRICT, so a leftover community
	// blocks the shared users cleanup other tests in this package rely on.
	defer td.CleanupTables(t, "community_moderators", "communities")

	ctx := context.Background()
	owner := td.CreateTestUser(t, "permowner", "permowner@example.com")
	mod := td.CreateTestUser(t, "permmod", "permmod@example.com")
	stranger := td.CreateTestUser(t, "permstranger", "permstranger@example.com")

	communityID := seedCommunity(t, td, int32(owner.ID), "perm-roles")
	seedModerator(t, td, communityID, int32(mod.ID))

	assert.Equal(t, CommunityRoleOwner, GetCommunityRole(ctx, td.Pool, communityID, int32(owner.ID)))
	assert.Equal(t, CommunityRoleModerator, GetCommunityRole(ctx, td.Pool, communityID, int32(mod.ID)))
	assert.Equal(t, CommunityRoleNone, GetCommunityRole(ctx, td.Pool, communityID, int32(stranger.ID)))
}

// A lookup failure must never read as elevated access.
func TestGetCommunityRole_UnknownCommunityIsNone(t *testing.T) {
	td := NewTestDatabase(t)
	defer td.Close()
	// communities.owner_user_id is ON DELETE RESTRICT, so a leftover community
	// blocks the shared users cleanup other tests in this package rely on.
	defer td.CleanupTables(t, "community_moderators", "communities")

	user := td.CreateTestUser(t, "permghost", "permghost@example.com")
	assert.Equal(t, CommunityRoleNone,
		GetCommunityRole(context.Background(), td.Pool, 999999, int32(user.ID)))
}

func TestCanModerateCommunity(t *testing.T) {
	td := NewTestDatabase(t)
	defer td.Close()
	// communities.owner_user_id is ON DELETE RESTRICT, so a leftover community
	// blocks the shared users cleanup other tests in this package rely on.
	defer td.CleanupTables(t, "community_moderators", "communities")

	owner := td.CreateTestUser(t, "modowner", "modowner@example.com")
	mod := td.CreateTestUser(t, "modmod", "modmod@example.com")
	stranger := td.CreateTestUser(t, "modstranger", "modstranger@example.com")

	communityID := seedCommunity(t, td, int32(owner.ID), "perm-moderate")
	seedModerator(t, td, communityID, int32(mod.ID))

	tests := []struct {
		name      string
		userID    int32
		isAdmin   bool
		adminMode bool
		want      bool
	}{
		{name: "owner may moderate", userID: int32(owner.ID), want: true},
		{name: "moderator may moderate", userID: int32(mod.ID), want: true},
		{name: "stranger may not", userID: int32(stranger.ID), want: false},

		// Admin mode is the same convention GM-override uses: being an admin is
		// not enough, the header must be set, so admins do not moderate by
		// accident while browsing normally.
		{name: "admin with admin mode may moderate", userID: int32(stranger.ID), isAdmin: true, adminMode: true, want: true},
		{name: "admin WITHOUT admin mode may not", userID: int32(stranger.ID), isAdmin: true, adminMode: false, want: false},
		{name: "non-admin with admin mode set may not", userID: int32(stranger.ID), isAdmin: false, adminMode: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := WithAdminMode(context.Background(), tt.adminMode)
			got := CanModerateCommunity(ctx, td.Pool, communityID, tt.userID, tt.isAdmin)
			assert.Equal(t, tt.want, got)
		})
	}
}

// The defining requirement: moderators may do everything a community owner can
// EXCEPT manage the moderator roster. CanAdministerCommunity is the gate that
// separates them, so a moderator must fail it while passing CanModerate.
func TestCanAdministerCommunity_ModeratorCannotManageRoster(t *testing.T) {
	td := NewTestDatabase(t)
	defer td.Close()
	// communities.owner_user_id is ON DELETE RESTRICT, so a leftover community
	// blocks the shared users cleanup other tests in this package rely on.
	defer td.CleanupTables(t, "community_moderators", "communities")

	owner := td.CreateTestUser(t, "adminowner", "adminowner@example.com")
	mod := td.CreateTestUser(t, "adminmod", "adminmod@example.com")
	stranger := td.CreateTestUser(t, "adminstranger", "adminstranger@example.com")

	communityID := seedCommunity(t, td, int32(owner.ID), "perm-administer")
	seedModerator(t, td, communityID, int32(mod.ID))

	ctx := WithAdminMode(context.Background(), false)

	// The heart of requirement 4, asserted as a pair so the two gates cannot
	// silently converge.
	assert.True(t, CanModerateCommunity(ctx, td.Pool, communityID, int32(mod.ID), false),
		"a moderator must be able to moderate")
	assert.False(t, CanAdministerCommunity(ctx, td.Pool, communityID, int32(mod.ID), false),
		"a moderator must NOT be able to manage the moderator roster")

	assert.True(t, CanAdministerCommunity(ctx, td.Pool, communityID, int32(owner.ID), false),
		"the owner must be able to manage the roster")
	assert.False(t, CanAdministerCommunity(ctx, td.Pool, communityID, int32(stranger.ID), false))
}

func TestCanAdministerCommunity_AdminMode(t *testing.T) {
	td := NewTestDatabase(t)
	defer td.Close()
	// communities.owner_user_id is ON DELETE RESTRICT, so a leftover community
	// blocks the shared users cleanup other tests in this package rely on.
	defer td.CleanupTables(t, "community_moderators", "communities")

	owner := td.CreateTestUser(t, "aowner", "aowner@example.com")
	admin := td.CreateTestUser(t, "asiteadmin", "asiteadmin@example.com")

	communityID := seedCommunity(t, td, int32(owner.ID), "perm-adminmode")

	withMode := WithAdminMode(context.Background(), true)
	assert.True(t, CanAdministerCommunity(withMode, td.Pool, communityID, int32(admin.ID), true),
		"a site admin in admin mode may manage the roster")

	withoutMode := WithAdminMode(context.Background(), false)
	assert.False(t, CanAdministerCommunity(withoutMode, td.Pool, communityID, int32(admin.ID), true),
		"a site admin without admin mode may not")
}

// Permissions are scoped per community: moderating one must grant nothing in
// another. This is the isolation the whole multi-community feature rests on.
func TestCommunityPermissions_ScopedPerCommunity(t *testing.T) {
	td := NewTestDatabase(t)
	defer td.Close()
	// communities.owner_user_id is ON DELETE RESTRICT, so a leftover community
	// blocks the shared users cleanup other tests in this package rely on.
	defer td.CleanupTables(t, "community_moderators", "communities")

	owner := td.CreateTestUser(t, "scopeowner", "scopeowner@example.com")
	mod := td.CreateTestUser(t, "scopemod", "scopemod@example.com")

	first := seedCommunity(t, td, int32(owner.ID), "perm-scope-a")
	second := seedCommunity(t, td, int32(owner.ID), "perm-scope-b")
	seedModerator(t, td, first, int32(mod.ID))

	ctx := WithAdminMode(context.Background(), false)

	assert.True(t, CanModerateCommunity(ctx, td.Pool, first, int32(mod.ID), false))
	assert.False(t, CanModerateCommunity(ctx, td.Pool, second, int32(mod.ID), false),
		"moderating one community must confer nothing in another")
}

// A context with no admin-mode value at all must behave as "off" rather than
// panicking or defaulting to on.
func TestCanModerateCommunity_MissingAdminModeDefaultsOff(t *testing.T) {
	td := NewTestDatabase(t)
	defer td.Close()
	// communities.owner_user_id is ON DELETE RESTRICT, so a leftover community
	// blocks the shared users cleanup other tests in this package rely on.
	defer td.CleanupTables(t, "community_moderators", "communities")

	owner := td.CreateTestUser(t, "bareowner", "bareowner@example.com")
	admin := td.CreateTestUser(t, "bareadmin", "bareadmin@example.com")

	communityID := seedCommunity(t, td, int32(owner.ID), "perm-bare-ctx")

	// Bare context: WithAdminMode never called.
	assert.False(t, CanModerateCommunity(context.Background(), td.Pool, communityID, int32(admin.ID), true),
		"an absent admin-mode value must read as off")
}
