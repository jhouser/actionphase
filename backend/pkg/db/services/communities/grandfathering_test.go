package communities

import (
	"context"
	"testing"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 🔴 REGRESSION GUARD FOR REQUIREMENT 5.
//
// games.community_id is nullable ON PURPOSE: games created before Communities
// existed are grandfathered in without one. New games require a community, but
// that rule lives in the application create path, never in a NOT NULL
// constraint.
//
// If someone "tidies up" the schema by adding NOT NULL, this test fails and
// explains why it must not be done. Every pre-existing game on the live site
// depends on it.
func TestGames_CommunityIDIsNullable(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()

	ctx := context.Background()
	gm := testDB.CreateTestUser(t, "legacygm", "legacygm@example.com")

	// Insert a game the way a pre-Communities row looks: no community at all.
	var gameID int32
	err := testDB.Pool.QueryRow(ctx,
		`INSERT INTO games (title, description, gm_user_id, state)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		"Legacy Game", "Predates communities", int32(gm.ID), core.GameStateInProgress,
	).Scan(&gameID)
	require.NoError(t, err,
		"inserting a game with no community must succeed -- games.community_id "+
			"must stay NULLABLE so pre-Communities games remain valid")

	// And it must still read back through the ordinary query path.
	game, err := models.New(testDB.Pool).GetGame(ctx, gameID)
	require.NoError(t, err, "a legacy game must still be readable")
	assert.False(t, game.CommunityID.Valid, "a legacy game must carry a NULL community_id")
}

// A game CAN carry a community, and the FK resolves. This is the other half of
// the pair: nullable does not mean unused.
func TestGames_CommunityIDLinksWhenSet(t *testing.T) {
	svc, testDB, ctx := newService(t)
	owner := testDB.CreateTestUser(t, "linkowner", "linkowner@example.com")
	gm := testDB.CreateTestUser(t, "linkgm", "linkgm@example.com")

	community, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Linked", Slug: uniqueSlug(t, "linked"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	var gameID int32
	err = testDB.Pool.QueryRow(ctx,
		`INSERT INTO games (title, description, gm_user_id, state, community_id)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		"Community Game", "in a community", int32(gm.ID), core.GameStateRecruitment, community.ID,
	).Scan(&gameID)
	require.NoError(t, err)

	game, err := models.New(testDB.Pool).GetGame(ctx, gameID)
	require.NoError(t, err)
	require.True(t, game.CommunityID.Valid)
	assert.Equal(t, community.ID, game.CommunityID.Int32)
}

// A community that still hosts games must not be deletable out from under
// them: the FK is ON DELETE RESTRICT, so the games survive.
func TestCommunity_DeleteRestrictedWhileGamesExist(t *testing.T) {
	svc, testDB, ctx := newService(t)
	owner := testDB.CreateTestUser(t, "delowner", "delowner@example.com")
	gm := testDB.CreateTestUser(t, "delgm", "delgm@example.com")

	community, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Busy", Slug: uniqueSlug(t, "busy"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	_, err = testDB.Pool.Exec(ctx,
		`INSERT INTO games (title, description, gm_user_id, state, community_id) VALUES ($1, $2, $3, $4, $5)`,
		"Hosted Game", "hosted", int32(gm.ID), core.GameStateRecruitment, community.ID)
	require.NoError(t, err)

	_, err = testDB.Pool.Exec(ctx, `DELETE FROM communities WHERE id = $1`, community.ID)
	assert.Error(t, err,
		"deleting a community that still hosts games must be refused, not silently orphan them")
}

// Deleting the owner must be refused rather than cascading and destroying the
// community. Reassign ownership first.
func TestCommunity_OwnerDeleteRestricted(t *testing.T) {
	svc, testDB, ctx := newService(t)
	owner := testDB.CreateTestUser(t, "restrictowner", "restrictowner@example.com")

	_, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Owned", Slug: uniqueSlug(t, "owned"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	_, err = testDB.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, int32(owner.ID))
	assert.Error(t, err,
		"deleting a community owner must be refused so the community is never orphaned")
}

// Removing a community DOES cascade to its moderator roster -- those rows are
// meaningless without it.
func TestCommunity_ModeratorsCascadeOnDelete(t *testing.T) {
	svc, testDB, ctx := newService(t)
	owner := testDB.CreateTestUser(t, "cascadeowner", "cascadeowner@example.com")
	mod := testDB.CreateTestUser(t, "cascademod", "cascademod@example.com")

	community, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Cascade", Slug: uniqueSlug(t, "cascade"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	_, err = svc.AddModerator(ctx, community.ID, int32(mod.ID), int32(owner.ID))
	require.NoError(t, err)

	// No games attached, so the delete is permitted.
	_, err = testDB.Pool.Exec(ctx, `DELETE FROM communities WHERE id = $1`, community.ID)
	require.NoError(t, err)

	var remaining int
	require.NoError(t, testDB.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM community_moderators WHERE community_id = $1`,
		community.ID).Scan(&remaining))
	assert.Zero(t, remaining, "moderator rows must cascade away with their community")
}
