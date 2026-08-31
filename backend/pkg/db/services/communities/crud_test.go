package communities

import (
	"context"
	"fmt"
	"testing"

	"actionphase/pkg/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newService spins up a service against a per-package test database.
func newService(t *testing.T) (*CommunityService, *core.TestDatabase, context.Context) {
	t.Helper()

	testDB := core.NewTestDatabase(t)
	t.Cleanup(testDB.Close)

	app := core.NewTestApp(testDB.Pool)
	return &CommunityService{DB: testDB.Pool, Logger: app.ObsLogger}, testDB, context.Background()
}

// uniqueSlug keeps parallel-safe tests from colliding on the slug unique index.
func uniqueSlug(t *testing.T, prefix string) string {
	t.Helper()
	return fmt.Sprintf("%s-%d", prefix, testCounter())
}

var counter int

func testCounter() int {
	counter++
	return counter
}

func TestCreateCommunity(t *testing.T) {
	svc, testDB, ctx := newService(t)
	owner := testDB.CreateTestUser(t, "owner", "owner@example.com")

	slug := uniqueSlug(t, "ravens")
	desc := "A community for testing"

	community, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name:        "Midnight Ravens",
		Slug:        slug,
		Description: &desc,
		OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)
	require.NotNil(t, community)

	assert.Equal(t, "Midnight Ravens", community.Name)
	assert.Equal(t, slug, community.Slug)
	assert.Equal(t, int32(owner.ID), community.OwnerUserID)
	assert.True(t, community.IsActive, "a new community must be active by default")
	require.NotNil(t, community.Description)
	assert.Equal(t, desc, *community.Description)

	// An omitted optional field must round-trip as nil, not as "".
	assert.Nil(t, community.BannerURL, "an unset banner must be nil, not empty string")
}

// The unique constraint is the authority on slug collisions, and the service
// must translate it to a domain error rather than leaking a 500-shaped failure.
func TestCreateCommunity_DuplicateSlugRejected(t *testing.T) {
	svc, testDB, ctx := newService(t)
	owner := testDB.CreateTestUser(t, "owner", "owner@example.com")

	slug := uniqueSlug(t, "dupe")
	_, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "First", Slug: slug, OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	_, err = svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Second", Slug: slug, OwnerUserID: int32(owner.ID),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, core.ErrCommunitySlugTaken,
		"a slug collision must surface as ErrCommunitySlugTaken so the handler can answer 400")
}

func TestGetCommunity_ByIDAndSlug(t *testing.T) {
	svc, testDB, ctx := newService(t)
	owner := testDB.CreateTestUser(t, "owner", "owner@example.com")

	slug := uniqueSlug(t, "lookup")
	created, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Lookup", Slug: slug, OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	byID, err := svc.GetCommunityByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, byID.ID)

	bySlug, err := svc.GetCommunityBySlug(ctx, slug)
	require.NoError(t, err)
	assert.Equal(t, created.ID, bySlug.ID)
}

// A missing community must be distinguishable from a server fault, or the
// handler cannot answer 404.
func TestGetCommunity_NotFound(t *testing.T) {
	svc, _, ctx := newService(t)

	_, err := svc.GetCommunityByID(ctx, 999999)
	assert.ErrorIs(t, err, core.ErrCommunityNotFound)

	_, err = svc.GetCommunityBySlug(ctx, "no-such-community-anywhere")
	assert.ErrorIs(t, err, core.ErrCommunityNotFound)
}

// PATCH semantics: an omitted field must be left alone, not blanked.
func TestUpdateCommunity_PartialUpdateLeavesOtherFieldsIntact(t *testing.T) {
	svc, testDB, ctx := newService(t)
	owner := testDB.CreateTestUser(t, "owner", "owner@example.com")

	desc := "original description"
	created, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name:        "Original Name",
		Slug:        uniqueSlug(t, "partial"),
		Description: &desc,
		OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	newName := "Renamed"
	updated, err := svc.UpdateCommunity(ctx, created.ID, &core.UpdateCommunityRequest{
		Name: &newName,
	})
	require.NoError(t, err)

	assert.Equal(t, "Renamed", updated.Name)
	require.NotNil(t, updated.Description)
	assert.Equal(t, desc, *updated.Description,
		"an omitted description must be left unchanged, not blanked")
	assert.Equal(t, created.Slug, updated.Slug, "slug must be immutable")
	assert.Equal(t, created.OwnerUserID, updated.OwnerUserID)
	assert.True(t, updated.IsActive)
}

func TestUpdateCommunity_ReassignOwnerAndDeactivate(t *testing.T) {
	svc, testDB, ctx := newService(t)
	owner := testDB.CreateTestUser(t, "owner", "owner@example.com")
	newOwner := testDB.CreateTestUser(t, "newowner", "newowner@example.com")

	created, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Handover", Slug: uniqueSlug(t, "handover"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	newOwnerID := int32(newOwner.ID)
	inactive := false
	updated, err := svc.UpdateCommunity(ctx, created.ID, &core.UpdateCommunityRequest{
		OwnerUserID: &newOwnerID,
		IsActive:    &inactive,
	})
	require.NoError(t, err)

	assert.Equal(t, newOwnerID, updated.OwnerUserID)
	assert.False(t, updated.IsActive)
}

func TestUpdateCommunity_NotFound(t *testing.T) {
	svc, _, ctx := newService(t)

	name := "Ghost"
	_, err := svc.UpdateCommunity(ctx, 999999, &core.UpdateCommunityRequest{Name: &name})
	assert.ErrorIs(t, err, core.ErrCommunityNotFound)
}

// ListCommunities is the admin view: it must include inactive communities,
// because deactivating one must not make it unmanageable.
func TestListCommunities_IncludesInactive(t *testing.T) {
	svc, testDB, ctx := newService(t)
	defer testDB.CleanupTables(t, "communities")
	owner := testDB.CreateTestUser(t, "owner", "owner@example.com")

	active, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Active", Slug: uniqueSlug(t, "active"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	hidden, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Hidden", Slug: uniqueSlug(t, "hidden"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	off := false
	_, err = svc.UpdateCommunity(ctx, hidden.ID, &core.UpdateCommunityRequest{IsActive: &off})
	require.NoError(t, err)

	all, err := svc.ListCommunities(ctx)
	require.NoError(t, err)

	ids := idSet(all)
	assert.Contains(t, ids, active.ID)
	assert.Contains(t, ids, hidden.ID, "the admin listing must include inactive communities")

	// The joined owner username must be populated, or the admin table would
	// need a request per row.
	for _, c := range all {
		assert.NotEmpty(t, c.OwnerUsername, "list rows must carry the joined owner username")
	}
}

// ListActiveCommunities powers the public list and the game-create picker, so
// an inactive community must not appear.
func TestListActiveCommunities_ExcludesInactive(t *testing.T) {
	svc, testDB, ctx := newService(t)
	defer testDB.CleanupTables(t, "communities")
	owner := testDB.CreateTestUser(t, "owner", "owner@example.com")

	active, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Active", Slug: uniqueSlug(t, "pubactive"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	hidden, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Hidden", Slug: uniqueSlug(t, "pubhidden"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	off := false
	_, err = svc.UpdateCommunity(ctx, hidden.ID, &core.UpdateCommunityRequest{IsActive: &off})
	require.NoError(t, err)

	activeOnly, err := svc.ListActiveCommunities(ctx)
	require.NoError(t, err)

	ids := idSet(activeOnly)
	assert.Contains(t, ids, active.ID)
	assert.NotContains(t, ids, hidden.ID, "an inactive community must not appear in the public listing")
}

func idSet(cs []*core.Community) map[int32]bool {
	out := make(map[int32]bool, len(cs))
	for _, c := range cs {
		out[c.ID] = true
	}
	return out
}

// TestUpdateCommunity_EmptyDescriptionClearsIt covers the third state a naive
// COALESCE update cannot express. Omitting the field leaves it alone (see
// TestUpdateCommunity_PartialUpdateLeavesOtherFieldsIntact); sending an empty
// string must remove the blurb entirely rather than store "" or silently no-op.
// Without this, a description would be write-once-then-permanent.
func TestUpdateCommunity_EmptyDescriptionClearsIt(t *testing.T) {
	svc, testDB, ctx := newService(t)
	owner := testDB.CreateTestUser(t, "clearowner", "clearowner@example.com")

	desc := "a blurb the admin later regrets"
	created, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name:        "Clearable",
		Slug:        uniqueSlug(t, "clearable"),
		Description: &desc,
		OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)
	require.NotNil(t, created.Description, "precondition: the description is set")

	blank := ""
	updated, err := svc.UpdateCommunity(ctx, created.ID, &core.UpdateCommunityRequest{
		Description: &blank,
	})
	require.NoError(t, err)
	assert.Nil(t, updated.Description,
		"an empty description must clear the column to NULL, not store an empty string")

	// It must survive a re-read: a clear that only appears in the RETURNING row
	// but never lands in the column would pass the assertion above.
	reread, err := svc.GetCommunityByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, reread.Description, "the cleared description must persist")
}

// TestUpdateCommunity_WhitespaceDescriptionClearsIt guards the near-miss: a
// field the admin blanked by selecting-all and hitting space is intent to
// clear, not intent to store "   ".
func TestUpdateCommunity_WhitespaceDescriptionClearsIt(t *testing.T) {
	svc, testDB, ctx := newService(t)
	owner := testDB.CreateTestUser(t, "wsowner", "wsowner@example.com")

	desc := "original"
	created, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name:        "Whitespace",
		Slug:        uniqueSlug(t, "whitespace"),
		Description: &desc,
		OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	spaces := "   "
	updated, err := svc.UpdateCommunity(ctx, created.ID, &core.UpdateCommunityRequest{
		Description: &spaces,
	})
	require.NoError(t, err)
	assert.Nil(t, updated.Description,
		"a whitespace-only description must clear the column, not store spaces")
}

// TestUpdateCommunity_DescriptionCanBeSetAfterClearing proves the clear is not
// a one-way door -- the field stays editable afterwards.
func TestUpdateCommunity_DescriptionCanBeSetAfterClearing(t *testing.T) {
	svc, testDB, ctx := newService(t)
	owner := testDB.CreateTestUser(t, "recowner", "recowner@example.com")

	created, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name:        "Round Trip",
		Slug:        uniqueSlug(t, "roundtrip"),
		OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)
	require.Nil(t, created.Description, "precondition: starts with no description")

	set := "now it has one"
	updated, err := svc.UpdateCommunity(ctx, created.ID, &core.UpdateCommunityRequest{
		Description: &set,
	})
	require.NoError(t, err)
	require.NotNil(t, updated.Description)
	assert.Equal(t, set, *updated.Description)

	blank := ""
	updated, err = svc.UpdateCommunity(ctx, created.ID, &core.UpdateCommunityRequest{
		Description: &blank,
	})
	require.NoError(t, err)
	assert.Nil(t, updated.Description)

	again := "and again"
	updated, err = svc.UpdateCommunity(ctx, created.ID, &core.UpdateCommunityRequest{
		Description: &again,
	})
	require.NoError(t, err)
	require.NotNil(t, updated.Description)
	assert.Equal(t, again, *updated.Description, "clearing must not lock the field")
}
