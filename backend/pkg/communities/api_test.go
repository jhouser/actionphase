package communities

// HTTP-level tests for the member- and moderator-facing community endpoints.
//
// These drive the real router with a real database, so they assert what a
// client actually experiences -- including the permission boundary that defines
// requirement 4: a moderator may do everything an owner can EXCEPT manage the
// moderator roster.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"actionphase/pkg/core"
	dbsvc "actionphase/pkg/db/services"
	communitysvc "actionphase/pkg/db/services/communities"
	"actionphase/pkg/humaconfig"
)

// setupCommunityTestRouter mirrors the production mount in pkg/http/root.go,
// AdminModeMiddleware included -- without it an admin's X-Admin-Mode header
// would never reach the permission helpers and the admin cases below would
// silently test the wrong thing.
func setupCommunityTestRouter(app *core.App, testDB *core.TestDatabase) *chi.Mux {
	tokenAuth := jwtauth.New("HS256", []byte(app.Config.JWT.Secret), nil)
	userService := &dbsvc.UserService{DB: testDB.Pool, Logger: app.ObsLogger}

	r := chi.NewRouter()
	r.Route("/api/v1/communities", func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(jwtauth.Authenticator(tokenAuth))
		r.Use(core.RequireAuthenticationMiddleware(userService))
		r.Use(core.AdminModeMiddleware)

		handler := &Handler{
			App:              app,
			UserService:      userService,
			CommunityService: &communitysvc.CommunityService{DB: testDB.Pool, Logger: app.ObsLogger},
		}
		RegisterHumaCommunities(humaconfig.New(r, "ActionPhase API", "1.0.0"), handler)
	})

	return r
}

// harness holds everything the tests need to act as any of the four callers the
// permission model distinguishes.
type harness struct {
	testDB    *core.TestDatabase
	app       *core.App
	router    *chi.Mux
	community *core.Community

	owner     *core.User
	moderator *core.User
	outsider  *core.User
	admin     *core.User
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	testDB := core.NewTestDatabase(t)
	t.Cleanup(testDB.Close)
	// communities.owner_user_id is ON DELETE RESTRICT, so the community rows
	// must go before the users they point at.
	t.Cleanup(func() { testDB.CleanupTables(t, "community_moderators", "communities", "users") })

	app := core.NewTestApp(testDB.Pool)
	router := setupCommunityTestRouter(app, testDB)

	h := &harness{
		testDB:    testDB,
		app:       app,
		router:    router,
		owner:     testDB.CreateTestUser(t, "cowner", "cowner@example.com"),
		moderator: testDB.CreateTestUser(t, "cmod", "cmod@example.com"),
		outsider:  testDB.CreateTestUser(t, "coutsider", "coutsider@example.com"),
		admin:     testDB.CreateTestUser(t, "cadmin", "cadmin@example.com"),
	}

	_, err := testDB.Pool.Exec(context.Background(),
		"UPDATE users SET is_admin = true WHERE id = $1", h.admin.ID)
	require.NoError(t, err)

	svc := &communitysvc.CommunityService{DB: testDB.Pool, Logger: app.ObsLogger}
	community, err := svc.CreateCommunity(context.Background(), &core.CreateCommunityRequest{
		Name:        "Midnight Ravens",
		Slug:        "midnight-ravens",
		OwnerUserID: int32(h.owner.ID),
	})
	require.NoError(t, err)
	h.community = community

	_, err = svc.AddModerator(context.Background(),
		community.ID, int32(h.moderator.ID), int32(h.owner.ID))
	require.NoError(t, err)

	return h
}

func (h *harness) token(t *testing.T, u *core.User) string {
	t.Helper()
	tok, err := core.CreateTestJWTTokenForUser(h.app, u)
	require.NoError(t, err)
	return tok
}

// request issues an authenticated request. adminMode sets the X-Admin-Mode
// header the permission helpers read.
func (h *harness) request(t *testing.T, u *core.User, method, path string, body []byte, adminMode bool) *httptest.ResponseRecorder {
	t.Helper()

	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Authorization", "Bearer "+h.token(t, u))
	if adminMode {
		req.Header.Set("X-Admin-Mode", "true")
	}

	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

// ------------------------------------------------------------------- reads

func TestCommunitiesAPI_ListActive(t *testing.T) {
	h := newHarness(t)

	rec := h.request(t, h.outsider, http.MethodGet, "/api/v1/communities", nil, false)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var got []*core.Community
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))

	var found bool
	for _, c := range got {
		if c.Slug == "midnight-ravens" {
			found = true
		}
	}
	assert.True(t, found, "an active community must appear in the public listing")
}

// An inactive community accepts no new games, so it must not appear on a
// browsable surface where a user could pick it and hit a dead end.
func TestCommunitiesAPI_ListActive_OmitsInactive(t *testing.T) {
	h := newHarness(t)

	svc := &communitysvc.CommunityService{DB: h.testDB.Pool, Logger: h.app.ObsLogger}
	inactive := false
	_, err := svc.UpdateCommunity(context.Background(), h.community.ID,
		&core.UpdateCommunityRequest{IsActive: &inactive})
	require.NoError(t, err)

	rec := h.request(t, h.outsider, http.MethodGet, "/api/v1/communities", nil, false)
	require.Equal(t, http.StatusOK, rec.Code)

	var got []*core.Community
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	for _, c := range got {
		assert.NotEqual(t, "midnight-ravens", c.Slug,
			"a deactivated community must not appear in the public listing")
	}
}

func TestCommunitiesAPI_GetBySlug(t *testing.T) {
	h := newHarness(t)

	rec := h.request(t, h.outsider, http.MethodGet, "/api/v1/communities/midnight-ravens", nil, false)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var got core.Community
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "Midnight Ravens", got.Name)
	assert.Equal(t, int32(h.owner.ID), got.OwnerUserID)

	// The profile names the owner, so the single-community fetch has to carry
	// the joined username -- an id alone leaves the UI with nothing to render
	// and it falls back to a placeholder.
	assert.Equal(t, h.owner.Username, got.OwnerUsername,
		"the profile must carry the owner's username, not just their id")
}

func TestCommunitiesAPI_GetBySlug_NotFound(t *testing.T) {
	h := newHarness(t)

	rec := h.request(t, h.outsider, http.MethodGet, "/api/v1/communities/no-such-place", nil, false)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// ------------------------------------------------------------- roster reads

func TestCommunitiesAPI_ListModerators_AsOwner(t *testing.T) {
	h := newHarness(t)

	rec := h.request(t, h.owner, http.MethodGet,
		"/api/v1/communities/midnight-ravens/moderators", nil, false)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var got []*core.CommunityModerator
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got, 1, "the owner is NOT a moderator row; only the granted moderator is")
	assert.Equal(t, int32(h.moderator.ID), got[0].UserID)
}

// A moderator may read the roster -- that is ordinary moderation, not roster
// management.
func TestCommunitiesAPI_ListModerators_AsModerator(t *testing.T) {
	h := newHarness(t)

	rec := h.request(t, h.moderator, http.MethodGet,
		"/api/v1/communities/midnight-ravens/moderators", nil, false)
	assert.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
}

func TestCommunitiesAPI_ListModerators_OutsiderForbidden(t *testing.T) {
	h := newHarness(t)

	rec := h.request(t, h.outsider, http.MethodGet,
		"/api/v1/communities/midnight-ravens/moderators", nil, false)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// ------------------------------------------------------------ roster writes

func TestCommunitiesAPI_AddModerator_AsOwner(t *testing.T) {
	h := newHarness(t)

	body := []byte(fmt.Sprintf(`{"user_id":%d}`, h.outsider.ID))
	rec := h.request(t, h.owner, http.MethodPost,
		"/api/v1/communities/midnight-ravens/moderators", body, false)
	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

	var got core.CommunityModerator
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, int32(h.outsider.ID), got.UserID)
	require.NotNil(t, got.GrantedByUserID)
	assert.Equal(t, int32(h.owner.ID), *got.GrantedByUserID,
		"the grant must record who made it")
}

// THE DEFINING ASSERTION FOR REQUIREMENT 4.
//
// A moderator holds every other community power, so this is the single check
// that keeps the two tiers apart. If it ever passes, moderators can appoint
// each other and the owner tier stops meaning anything.
func TestCommunitiesAPI_AddModerator_ModeratorForbidden(t *testing.T) {
	h := newHarness(t)

	body := []byte(fmt.Sprintf(`{"user_id":%d}`, h.outsider.ID))
	rec := h.request(t, h.moderator, http.MethodPost,
		"/api/v1/communities/midnight-ravens/moderators", body, false)
	require.Equal(t, http.StatusForbidden, rec.Code,
		"a moderator must not be able to appoint moderators (req 4)")

	// The rejection must be real, not merely a status code: nobody was added.
	svc := &communitysvc.CommunityService{DB: h.testDB.Pool, Logger: h.app.ObsLogger}
	mods, err := svc.ListModerators(context.Background(), h.community.ID)
	require.NoError(t, err)
	assert.Len(t, mods, 1, "the roster must be unchanged after a refused grant")
}

func TestCommunitiesAPI_AddModerator_OutsiderForbidden(t *testing.T) {
	h := newHarness(t)

	body := []byte(fmt.Sprintf(`{"user_id":%d}`, h.outsider.ID))
	rec := h.request(t, h.outsider, http.MethodPost,
		"/api/v1/communities/midnight-ravens/moderators", body, false)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// The owner already holds every moderator power, and a duplicate row would let
// someone "demote" them by deleting it while ownership stayed put.
func TestCommunitiesAPI_AddModerator_OwnerRejected(t *testing.T) {
	h := newHarness(t)

	body := []byte(fmt.Sprintf(`{"user_id":%d}`, h.owner.ID))
	rec := h.request(t, h.owner, http.MethodPost,
		"/api/v1/communities/midnight-ravens/moderators", body, false)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
}

func TestCommunitiesAPI_AddModerator_DuplicateRejected(t *testing.T) {
	h := newHarness(t)

	body := []byte(fmt.Sprintf(`{"user_id":%d}`, h.moderator.ID))
	rec := h.request(t, h.owner, http.MethodPost,
		"/api/v1/communities/midnight-ravens/moderators", body, false)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
}

// An unknown target would otherwise fail on the foreign key and surface as a
// 500, telling the owner nothing about what they got wrong.
func TestCommunitiesAPI_AddModerator_UnknownUser(t *testing.T) {
	h := newHarness(t)

	rec := h.request(t, h.owner, http.MethodPost,
		"/api/v1/communities/midnight-ravens/moderators", []byte(`{"user_id":999999}`), false)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
}

func TestCommunitiesAPI_RemoveModerator_AsOwner(t *testing.T) {
	h := newHarness(t)

	path := fmt.Sprintf("/api/v1/communities/midnight-ravens/moderators/%d", h.moderator.ID)
	rec := h.request(t, h.owner, http.MethodDelete, path, nil, false)
	require.Equal(t, http.StatusNoContent, rec.Code, "body: %s", rec.Body.String())

	svc := &communitysvc.CommunityService{DB: h.testDB.Pool, Logger: h.app.ObsLogger}
	mods, err := svc.ListModerators(context.Background(), h.community.ID)
	require.NoError(t, err)
	assert.Empty(t, mods, "the removal must actually land, not just return 204")
}

// Removing is roster management too, so it sits on the owner side of req 4 --
// otherwise a moderator could remove every peer and the tier would be hollow.
func TestCommunitiesAPI_RemoveModerator_ModeratorForbidden(t *testing.T) {
	h := newHarness(t)

	path := fmt.Sprintf("/api/v1/communities/midnight-ravens/moderators/%d", h.moderator.ID)
	rec := h.request(t, h.moderator, http.MethodDelete, path, nil, false)
	require.Equal(t, http.StatusForbidden, rec.Code)

	svc := &communitysvc.CommunityService{DB: h.testDB.Pool, Logger: h.app.ObsLogger}
	mods, err := svc.ListModerators(context.Background(), h.community.ID)
	require.NoError(t, err)
	assert.Len(t, mods, 1, "a refused removal must leave the roster intact")
}

// The caller's intent -- that this user does not moderate -- already holds.
func TestCommunitiesAPI_RemoveModerator_NonMemberSucceeds(t *testing.T) {
	h := newHarness(t)

	path := fmt.Sprintf("/api/v1/communities/midnight-ravens/moderators/%d", h.outsider.ID)
	rec := h.request(t, h.owner, http.MethodDelete, path, nil, false)
	assert.Equal(t, http.StatusNoContent, rec.Code, "body: %s", rec.Body.String())
}

// ------------------------------------------------------------- admin access

// A site admin gets roster powers, but ONLY with admin mode on -- the same
// convention as GM override, so admins do not moderate by accident.
func TestCommunitiesAPI_AddModerator_AdminWithModeAllowed(t *testing.T) {
	h := newHarness(t)

	body := []byte(fmt.Sprintf(`{"user_id":%d}`, h.outsider.ID))
	rec := h.request(t, h.admin, http.MethodPost,
		"/api/v1/communities/midnight-ravens/moderators", body, true)
	assert.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())
}

func TestCommunitiesAPI_AddModerator_AdminWithoutModeForbidden(t *testing.T) {
	h := newHarness(t)

	body := []byte(fmt.Sprintf(`{"user_id":%d}`, h.outsider.ID))
	rec := h.request(t, h.admin, http.MethodPost,
		"/api/v1/communities/midnight-ravens/moderators", body, false)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"an admin browsing normally must not moderate by accident")
}

func TestCommunitiesAPI_ListModerators_AdminWithoutModeForbidden(t *testing.T) {
	h := newHarness(t)

	rec := h.request(t, h.admin, http.MethodGet,
		"/api/v1/communities/midnight-ravens/moderators", nil, false)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// ------------------------------------------------------- profile editing

// Editing name and description is MODERATOR-level, unlike the roster. These
// tests pin that boundary from both sides: a moderator succeeds here but is
// still refused on the roster (TestCommunitiesAPI_AddModerator_ModeratorForbidden).

func TestCommunitiesAPI_UpdateCommunity_AsOwner(t *testing.T) {
	h := newHarness(t)

	body := []byte(`{"name":"Midnight Ravens Reborn","description":"# We fly at dusk"}`)
	rec := h.request(t, h.owner, http.MethodPatch, "/api/v1/communities/midnight-ravens", body, false)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var got core.Community
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "Midnight Ravens Reborn", got.Name)
	require.NotNil(t, got.Description)
	assert.Equal(t, "# We fly at dusk", *got.Description)
	// The slug is immutable, so external links keep working.
	assert.Equal(t, "midnight-ravens", got.Slug)
}

func TestCommunitiesAPI_UpdateCommunity_AsModerator(t *testing.T) {
	h := newHarness(t)

	body := []byte(`{"description":"Updated by a moderator"}`)
	rec := h.request(t, h.moderator, http.MethodPatch, "/api/v1/communities/midnight-ravens", body, false)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var got core.Community
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.NotNil(t, got.Description)
	assert.Equal(t, "Updated by a moderator", *got.Description)
	// Omitted fields are left alone.
	assert.Equal(t, "Midnight Ravens", got.Name)
}

func TestCommunitiesAPI_UpdateCommunity_OutsiderForbidden(t *testing.T) {
	h := newHarness(t)

	body := []byte(`{"name":"Hijacked"}`)
	rec := h.request(t, h.outsider, http.MethodPatch, "/api/v1/communities/midnight-ravens", body, false)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCommunitiesAPI_UpdateCommunity_AdminWithoutModeForbidden(t *testing.T) {
	h := newHarness(t)

	body := []byte(`{"name":"Hijacked"}`)
	rec := h.request(t, h.admin, http.MethodPatch, "/api/v1/communities/midnight-ravens", body, false)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCommunitiesAPI_UpdateCommunity_AdminWithModeAllowed(t *testing.T) {
	h := newHarness(t)

	body := []byte(`{"name":"Admin Renamed"}`)
	rec := h.request(t, h.admin, http.MethodPatch, "/api/v1/communities/midnight-ravens", body, true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// A blank name would leave the community unnameable in every listing, so it is
// rejected rather than stored. Whitespace-only counts as blank -- it passes a
// minLength check but is not a name.
func TestCommunitiesAPI_UpdateCommunity_BlankNameRejected(t *testing.T) {
	h := newHarness(t)

	body := []byte(`{"name":"   "}`)
	rec := h.request(t, h.owner, http.MethodPatch, "/api/v1/communities/midnight-ravens", body, false)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// An empty description CLEARS the blurb. Without this case a description would
// be write-once-then-permanent, since omission means "unchanged".
func TestCommunitiesAPI_UpdateCommunity_EmptyDescriptionClears(t *testing.T) {
	h := newHarness(t)

	set := []byte(`{"description":"temporary"}`)
	rec := h.request(t, h.owner, http.MethodPatch, "/api/v1/communities/midnight-ravens", set, false)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	clear := []byte(`{"description":""}`)
	rec = h.request(t, h.owner, http.MethodPatch, "/api/v1/communities/midnight-ravens", clear, false)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var got core.Community
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	if got.Description != nil && *got.Description != "" {
		t.Fatalf("description = %q, want it cleared", *got.Description)
	}
}

// A moderator must not be able to seize the community or retire it.
//
// The schema has no owner_user_id or is_active, and huma REJECTS unknown
// properties rather than dropping them -- so the attempt fails loudly with a
// 400 instead of appearing to succeed. That is the stronger outcome: a client
// cannot believe it changed ownership when it did not.
func TestCommunitiesAPI_UpdateCommunity_RejectsOwnerAndActiveFields(t *testing.T) {
	h := newHarness(t)

	for _, tc := range []struct {
		name string
		body string
	}{
		{"owner_user_id", fmt.Sprintf(`{"name":"Still Theirs","owner_user_id":%d}`, h.moderator.ID)},
		{"is_active", `{"name":"Still Theirs","is_active":false}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := h.request(t, h.moderator, http.MethodPatch,
				"/api/v1/communities/midnight-ravens", []byte(tc.body), false)
			assert.Equal(t, http.StatusBadRequest, rec.Code,
				"a moderator must not be able to set %s", tc.name)
		})
	}

	// And the community is untouched by the refused attempts.
	rec := h.request(t, h.moderator, http.MethodGet, "/api/v1/communities/midnight-ravens", nil, false)
	require.Equal(t, http.StatusOK, rec.Code)

	var got core.Community
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, int32(h.owner.ID), got.OwnerUserID, "ownership must be unchanged")
	assert.True(t, got.IsActive, "the community must still be active")
	assert.Equal(t, "Midnight Ravens", got.Name, "a rejected request must not apply its other fields")
}

func TestCommunitiesAPI_UpdateCommunity_NotFound(t *testing.T) {
	h := newHarness(t)

	body := []byte(`{"name":"Nowhere"}`)
	rec := h.request(t, h.owner, http.MethodPatch, "/api/v1/communities/no-such-community", body, false)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// --------------------------------------------------------------- your_role

// your_role tells the client the caller's standing, which the community record
// alone cannot: it names the owner but not the moderators. Without it the only
// signal is whether the moderator-gated roster endpoint 403s -- a request most
// viewers are expected to fail.

func TestCommunitiesAPI_YourRole_Owner(t *testing.T) {
	h := newHarness(t)

	rec := h.request(t, h.owner, http.MethodGet, "/api/v1/communities/midnight-ravens", nil, false)
	require.Equal(t, http.StatusOK, rec.Code)

	var got core.Community
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, core.CommunityRoleOwner, got.YourRole)
}

func TestCommunitiesAPI_YourRole_Moderator(t *testing.T) {
	h := newHarness(t)

	rec := h.request(t, h.moderator, http.MethodGet, "/api/v1/communities/midnight-ravens", nil, false)
	require.Equal(t, http.StatusOK, rec.Code)

	var got core.Community
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, core.CommunityRoleModerator, got.YourRole)
}

func TestCommunitiesAPI_YourRole_Outsider(t *testing.T) {
	h := newHarness(t)

	rec := h.request(t, h.outsider, http.MethodGet, "/api/v1/communities/midnight-ravens", nil, false)
	require.Equal(t, http.StatusOK, rec.Code)

	var got core.Community
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, core.CommunityRoleNone, got.YourRole)

	// An outsider still reads the profile -- it is public. The role is the only
	// thing that changes, so a client can gate edit controls on it.
	assert.Equal(t, "Midnight Ravens", got.Name)
}

// A site admin browsing NORMALLY has no standing here. This is the case a
// cached login payload could never get right, since admin mode is a per-request
// header the user toggles at will.
func TestCommunitiesAPI_YourRole_AdminWithoutMode(t *testing.T) {
	h := newHarness(t)

	rec := h.request(t, h.admin, http.MethodGet, "/api/v1/communities/midnight-ravens", nil, false)
	require.Equal(t, http.StatusOK, rec.Code)

	var got core.Community
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, core.CommunityRoleNone, got.YourRole,
		"an admin browsing normally must not be shown moderation controls")
}

func TestCommunitiesAPI_YourRole_AdminWithMode(t *testing.T) {
	h := newHarness(t)

	rec := h.request(t, h.admin, http.MethodGet, "/api/v1/communities/midnight-ravens", nil, true)
	require.Equal(t, http.StatusOK, rec.Code)

	var got core.Community
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, core.CommunityRoleOwner, got.YourRole,
		"admin mode grants the full power set, which is the owner tier")
}

// The listing carries the role too, so a browsing surface can mark the
// communities a user helps run without a request per row.
func TestCommunitiesAPI_YourRole_OnListing(t *testing.T) {
	h := newHarness(t)

	rec := h.request(t, h.moderator, http.MethodGet, "/api/v1/communities", nil, false)
	require.Equal(t, http.StatusOK, rec.Code)

	var got []core.Community
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.NotEmpty(t, got)

	var found bool
	for _, c := range got {
		if c.Slug == "midnight-ravens" {
			found = true
			assert.Equal(t, core.CommunityRoleModerator, c.YourRole)
		}
	}
	assert.True(t, found, "the seeded community should appear in the active listing")
}

// The role must reflect the CALLER, not the last caller. A cached-by-slug
// response that leaked one user's role to another would hand out moderation
// controls to visitors.
func TestCommunitiesAPI_YourRole_IsPerCaller(t *testing.T) {
	h := newHarness(t)

	first := h.request(t, h.owner, http.MethodGet, "/api/v1/communities/midnight-ravens", nil, false)
	require.Equal(t, http.StatusOK, first.Code)

	second := h.request(t, h.outsider, http.MethodGet, "/api/v1/communities/midnight-ravens", nil, false)
	require.Equal(t, http.StatusOK, second.Code)

	var got core.Community
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &got))
	assert.Equal(t, core.CommunityRoleNone, got.YourRole,
		"the outsider must not inherit the owner's role from a prior request")
}
