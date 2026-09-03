package communities

import (
	"testing"

	"actionphase/pkg/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newDocFixture builds a community with an owner to hang documents on.
func newDocFixture(t *testing.T) (*CommunityService, *core.TestDatabase, *core.Community, *core.User) {
	t.Helper()

	svc, testDB, ctx := newService(t)
	owner := testDB.CreateTestUser(t, "docowner", "docowner@example.com")

	community, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Docs", Slug: uniqueSlug(t, "docs"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	return svc, testDB, community, owner
}

func strPtr(s string) *string { return &s }

func TestCreateDocument(t *testing.T) {
	svc, _, community, owner := newDocFixture(t)
	ctx := t.Context()

	doc, err := svc.CreateDocument(ctx, community.ID, int32(owner.ID), &core.CreateCommunityDocumentRequest{
		Title:   "House Rules",
		Content: "# Be excellent\n\nTo each other.",
	})
	require.NoError(t, err)
	require.NotNil(t, doc)

	assert.Equal(t, "House Rules", doc.Title)
	assert.Equal(t, community.ID, doc.CommunityID)
	require.NotNil(t, doc.CreatedByUserID)
	assert.Equal(t, int32(owner.ID), *doc.CreatedByUserID)
}

// The default has to be the one that shows the document to NOBODY. A moderator
// part-way through writing rules must not have them binding players already.
func TestCreateDocument_DefaultsToDraft(t *testing.T) {
	svc, _, community, owner := newDocFixture(t)
	ctx := t.Context()

	doc, err := svc.CreateDocument(ctx, community.ID, int32(owner.ID), &core.CreateCommunityDocumentRequest{
		Title: "Work in progress", Content: "half written",
	})
	require.NoError(t, err)
	assert.Equal(t, core.DocumentStatusDraft, doc.Status)
}

// A typo'd status must not silently become a draft: that would look exactly
// like the publish button being broken, with nothing to point at.
func TestCreateDocument_RejectsUnknownStatus(t *testing.T) {
	svc, _, community, owner := newDocFixture(t)
	ctx := t.Context()

	_, err := svc.CreateDocument(ctx, community.ID, int32(owner.ID), &core.CreateCommunityDocumentRequest{
		Title: "Rules", Content: "text", Status: strPtr("publish"),
	})
	require.ErrorIs(t, err, core.ErrInvalidDocumentStatus)
}

// THE visibility invariant. If the two list queries ever collapse into one with
// a flag, this is what catches it.
func TestListDocuments_DraftsAreModeratorOnly(t *testing.T) {
	svc, _, community, owner := newDocFixture(t)
	ctx := t.Context()

	_, err := svc.CreateDocument(ctx, community.ID, int32(owner.ID), &core.CreateCommunityDocumentRequest{
		Title: "Published rules", Content: "visible", Status: strPtr(core.DocumentStatusPublished),
	})
	require.NoError(t, err)
	_, err = svc.CreateDocument(ctx, community.ID, int32(owner.ID), &core.CreateCommunityDocumentRequest{
		Title: "Secret draft", Content: "not ready",
	})
	require.NoError(t, err)

	all, err := svc.ListDocuments(ctx, community.ID)
	require.NoError(t, err)
	assert.Len(t, all, 2, "the moderator list includes drafts")

	published, err := svc.ListPublishedDocuments(ctx, community.ID)
	require.NoError(t, err)
	require.Len(t, published, 1, "the public list must omit drafts")
	assert.Equal(t, "Published rules", published[0].Title)
}

// sort_order defaults to 0, so without the id tiebreak in ORDER BY the order of
// an unsorted community's documents is arbitrary and can change per request.
func TestListDocuments_OrdersBySortOrderThenID(t *testing.T) {
	svc, _, community, owner := newDocFixture(t)
	ctx := t.Context()

	mk := func(title string, sortOrder int32) {
		_, err := svc.CreateDocument(ctx, community.ID, int32(owner.ID), &core.CreateCommunityDocumentRequest{
			Title: title, Content: "text", SortOrder: &sortOrder,
		})
		require.NoError(t, err)
	}
	// Created deliberately out of display order.
	mk("Third", 20)
	mk("First", 10)
	mk("Second", 10) // ties with First; the id tiebreak decides

	docs, err := svc.ListDocuments(ctx, community.ID)
	require.NoError(t, err)
	require.Len(t, docs, 3)

	titles := []string{docs[0].Title, docs[1].Title, docs[2].Title}
	assert.Equal(t, []string{"First", "Second", "Third"}, titles)
}

// Documents are addressed by a bare id in the URL, so the community scope is
// the only thing stopping a moderator of A from reading A's path into B's
// draft. Answering "not found" rather than "forbidden" avoids confirming the
// document exists at all.
func TestGetDocument_IsScopedToItsCommunity(t *testing.T) {
	svc, testDB, communityA, owner := newDocFixture(t)
	ctx := t.Context()

	communityB, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Other", Slug: uniqueSlug(t, "other"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)
	_ = testDB

	doc, err := svc.CreateDocument(ctx, communityA.ID, int32(owner.ID), &core.CreateCommunityDocumentRequest{
		Title: "A's draft", Content: "private",
	})
	require.NoError(t, err)

	// Same id, wrong community.
	_, err = svc.GetDocument(ctx, communityB.ID, doc.ID)
	require.ErrorIs(t, err, core.ErrCommunityDocumentNotFound)

	// Sanity: it resolves under its own community.
	found, err := svc.GetDocument(ctx, communityA.ID, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, doc.ID, found.ID)
}

func TestUpdateDocument_LeavesOmittedFieldsAlone(t *testing.T) {
	svc, _, community, owner := newDocFixture(t)
	ctx := t.Context()

	doc, err := svc.CreateDocument(ctx, community.ID, int32(owner.ID), &core.CreateCommunityDocumentRequest{
		Title: "Original", Content: "body text",
	})
	require.NoError(t, err)

	updated, err := svc.UpdateDocument(ctx, community.ID, doc.ID, &core.UpdateCommunityDocumentRequest{
		Status: strPtr(core.DocumentStatusPublished),
	})
	require.NoError(t, err)

	assert.Equal(t, core.DocumentStatusPublished, updated.Status)
	assert.Equal(t, "Original", updated.Title, "title was not sent and must survive")
	assert.Equal(t, "body text", updated.Content, "content was not sent and must survive")
}

// content is NOT NULL, so "" is a blank page rather than a clear. Unlike a
// community description, there is no tri-state here -- and an empty string must
// actually reach the column instead of being read as "unchanged".
func TestUpdateDocument_EmptyContentIsStored(t *testing.T) {
	svc, _, community, owner := newDocFixture(t)
	ctx := t.Context()

	doc, err := svc.CreateDocument(ctx, community.ID, int32(owner.ID), &core.CreateCommunityDocumentRequest{
		Title: "Rules", Content: "to be emptied",
	})
	require.NoError(t, err)

	updated, err := svc.UpdateDocument(ctx, community.ID, doc.ID, &core.UpdateCommunityDocumentRequest{
		Content: strPtr(""),
	})
	require.NoError(t, err)
	assert.Empty(t, updated.Content)
}

func TestUpdateDocument_IsScopedToItsCommunity(t *testing.T) {
	svc, _, communityA, owner := newDocFixture(t)
	ctx := t.Context()

	communityB, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Other", Slug: uniqueSlug(t, "other"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	doc, err := svc.CreateDocument(ctx, communityA.ID, int32(owner.ID), &core.CreateCommunityDocumentRequest{
		Title: "A's rules", Content: "original",
	})
	require.NoError(t, err)

	_, err = svc.UpdateDocument(ctx, communityB.ID, doc.ID, &core.UpdateCommunityDocumentRequest{
		Title: strPtr("hijacked"),
	})
	require.ErrorIs(t, err, core.ErrCommunityDocumentNotFound)

	// The document must be untouched.
	found, err := svc.GetDocument(ctx, communityA.ID, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, "A's rules", found.Title)
}

func TestUpdateDocument_RejectsUnknownStatus(t *testing.T) {
	svc, _, community, owner := newDocFixture(t)
	ctx := t.Context()

	doc, err := svc.CreateDocument(ctx, community.ID, int32(owner.ID), &core.CreateCommunityDocumentRequest{
		Title: "Rules", Content: "text",
	})
	require.NoError(t, err)

	_, err = svc.UpdateDocument(ctx, community.ID, doc.ID, &core.UpdateCommunityDocumentRequest{
		Status: strPtr("live"),
	})
	require.ErrorIs(t, err, core.ErrInvalidDocumentStatus)
}

func TestDeleteDocument(t *testing.T) {
	svc, _, community, owner := newDocFixture(t)
	ctx := t.Context()

	doc, err := svc.CreateDocument(ctx, community.ID, int32(owner.ID), &core.CreateCommunityDocumentRequest{
		Title: "Temporary", Content: "text",
	})
	require.NoError(t, err)

	require.NoError(t, svc.DeleteDocument(ctx, community.ID, doc.ID))

	_, err = svc.GetDocument(ctx, community.ID, doc.ID)
	require.ErrorIs(t, err, core.ErrCommunityDocumentNotFound)
}

// A delete that matched nothing must surface. Reporting success would leave a
// moderator believing a document is gone while it is still published.
func TestDeleteDocument_MissingIsAnError(t *testing.T) {
	svc, _, community, _ := newDocFixture(t)
	ctx := t.Context()

	err := svc.DeleteDocument(ctx, community.ID, 999999)
	require.ErrorIs(t, err, core.ErrCommunityDocumentNotFound)
}

func TestDeleteDocument_IsScopedToItsCommunity(t *testing.T) {
	svc, _, communityA, owner := newDocFixture(t)
	ctx := t.Context()

	communityB, err := svc.CreateCommunity(ctx, &core.CreateCommunityRequest{
		Name: "Other", Slug: uniqueSlug(t, "other"), OwnerUserID: int32(owner.ID),
	})
	require.NoError(t, err)

	doc, err := svc.CreateDocument(ctx, communityA.ID, int32(owner.ID), &core.CreateCommunityDocumentRequest{
		Title: "A's rules", Content: "text",
	})
	require.NoError(t, err)

	require.ErrorIs(t,
		svc.DeleteDocument(ctx, communityB.ID, doc.ID),
		core.ErrCommunityDocumentNotFound)

	_, err = svc.GetDocument(ctx, communityA.ID, doc.ID)
	require.NoError(t, err, "the document must survive a cross-community delete")
}

// req 8: the Info tab list resolves game -> community, published only.
func TestListPublishedDocumentsForGame(t *testing.T) {
	svc, testDB, community, owner := newDocFixture(t)
	ctx := t.Context()

	_, err := svc.CreateDocument(ctx, community.ID, int32(owner.ID), &core.CreateCommunityDocumentRequest{
		Title: "Published", Content: "visible", Status: strPtr(core.DocumentStatusPublished),
	})
	require.NoError(t, err)
	_, err = svc.CreateDocument(ctx, community.ID, int32(owner.ID), &core.CreateCommunityDocumentRequest{
		Title: "Draft", Content: "hidden",
	})
	require.NoError(t, err)

	game := testDB.CreateTestGame(t, int32(owner.ID), "Game in a community")
	_, err = testDB.Pool.Exec(ctx, `UPDATE games SET community_id = $1 WHERE id = $2`,
		community.ID, game.ID)
	require.NoError(t, err)

	docs, err := svc.ListPublishedDocumentsForGame(ctx, game.ID)
	require.NoError(t, err)
	require.Len(t, docs, 1, "drafts must not reach the Info tab")
	assert.Equal(t, "Published", docs[0].Title)

	// Documents only. The community's identity reaches the Info tab through
	// GetGameWithDetails, not through these rows -- see the games service test
	// for that half. Asserted here so re-adding the columns to serve some future
	// caller has to come past this comment first: duplicating identity onto the
	// document rows is what once made the section unable to name a community
	// that had published nothing.
	assert.Equal(t, community.ID, docs[0].CommunityID,
		"the per-game list must still scope its rows to the owning community")
}

// req 5 grandfathering: a game predating communities has community_id NULL. The
// join must yield nothing rather than erroring, so the Info tab simply renders
// no community section.
func TestListPublishedDocumentsForGame_LegacyGameHasNone(t *testing.T) {
	svc, testDB, _, owner := newDocFixture(t)
	ctx := t.Context()

	game := testDB.CreateTestGame(t, int32(owner.ID), "Legacy game")

	docs, err := svc.ListPublishedDocumentsForGame(ctx, game.ID)
	require.NoError(t, err, "a game with no community is not an error")
	assert.Empty(t, docs)
}
