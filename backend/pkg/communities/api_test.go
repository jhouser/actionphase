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
	"time"

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
	t.Cleanup(func() {
		testDB.CleanupTables(t,
			"community_ban_events", "community_bans",
			"community_moderators", "communities", "users")
	})

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

// --------------------------------------------------------------------- bans

// banPath builds the collection path for the harness community.
func banPath() string { return "/api/v1/communities/midnight-ravens/bans" }

// listBans reads the banlist directly from the service, for asserting that a
// refused request changed nothing. Going through the API would only re-test the
// endpoint under scrutiny.
func (h *harness) listBans(t *testing.T) []*core.CommunityBan {
	t.Helper()
	svc := &communitysvc.CommunityService{DB: h.testDB.Pool, Logger: h.app.ObsLogger}
	bans, err := svc.ListBans(context.Background(), h.community.ID)
	require.NoError(t, err)
	return bans
}

func (h *harness) banEvents(t *testing.T) []*core.CommunityBanEvent {
	t.Helper()
	svc := &communitysvc.CommunityService{DB: h.testDB.Pool, Logger: h.app.ObsLogger}
	events, err := svc.ListBanEvents(context.Background(), h.community.ID, 0, 0)
	require.NoError(t, err)
	return events
}

func TestCommunitiesAPI_BanUser_AsModerator(t *testing.T) {
	h := newHarness(t)

	body := []byte(fmt.Sprintf(`{"user_id":%d,"reason":"spoilers"}`, h.outsider.ID))
	rec := h.request(t, h.moderator, http.MethodPost, banPath(), body, false)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var got core.CommunityBan
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, int32(h.outsider.ID), got.UserID)
	require.NotNil(t, got.Reason)
	assert.Equal(t, "spoilers", *got.Reason)
	assert.True(t, got.IsActive, "a ban with no expiry is permanent and active")
	assert.Nil(t, got.ExpiresAt, "omitting expires_at must mean permanent")
	require.NotNil(t, got.BannedByUserID)
	assert.Equal(t, int32(h.moderator.ID), *got.BannedByUserID,
		"the ban must record which moderator issued it")
}

// Banning is moderator-tier, not owner-tier (req 4) -- it is the routine work
// the feature exists for, and reserving it to owners would defeat the point of
// having moderators at all.
func TestCommunitiesAPI_BanUser_AsOwner(t *testing.T) {
	h := newHarness(t)

	body := []byte(fmt.Sprintf(`{"user_id":%d}`, h.outsider.ID))
	rec := h.request(t, h.owner, http.MethodPost, banPath(), body, false)
	assert.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
}

func TestCommunitiesAPI_BanUser_OutsiderForbidden(t *testing.T) {
	h := newHarness(t)

	// A non-moderator banning someone would be the whole access-control model
	// inverted, so assert the refusal took effect rather than trusting the code.
	body := []byte(fmt.Sprintf(`{"user_id":%d}`, h.admin.ID))
	rec := h.request(t, h.outsider, http.MethodPost, banPath(), body, false)
	require.Equal(t, http.StatusForbidden, rec.Code)

	assert.Empty(t, h.listBans(t), "a refused ban must not be written")
}

func TestCommunitiesAPI_BanUser_AdminWithoutModeForbidden(t *testing.T) {
	h := newHarness(t)

	body := []byte(fmt.Sprintf(`{"user_id":%d}`, h.outsider.ID))
	rec := h.request(t, h.admin, http.MethodPost, banPath(), body, false)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"a site admin browsing normally must not moderate by accident")
}

func TestCommunitiesAPI_BanUser_AdminWithModeAllowed(t *testing.T) {
	h := newHarness(t)

	body := []byte(fmt.Sprintf(`{"user_id":%d}`, h.outsider.ID))
	rec := h.request(t, h.admin, http.MethodPost, banPath(), body, true)
	assert.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
}

// Re-banning must edit in place rather than 400 on the unique constraint --
// otherwise a moderator extending a ban is pushed into unban-then-reban, which
// resets banned_at and loses when the ban actually started.
func TestCommunitiesAPI_BanUser_RebanUpdatesInPlace(t *testing.T) {
	h := newHarness(t)

	first := []byte(fmt.Sprintf(`{"user_id":%d,"reason":"first"}`, h.outsider.ID))
	rec := h.request(t, h.moderator, http.MethodPost, banPath(), first, false)
	require.Equal(t, http.StatusOK, rec.Code)

	var original core.CommunityBan
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &original))

	second := []byte(fmt.Sprintf(`{"user_id":%d,"reason":"second"}`, h.outsider.ID))
	rec = h.request(t, h.moderator, http.MethodPost, banPath(), second, false)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var updated core.CommunityBan
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &updated))
	require.NotNil(t, updated.Reason)
	assert.Equal(t, "second", *updated.Reason, "the reason must be updated in place")
	assert.Equal(t, original.BannedAt.Unix(), updated.BannedAt.Unix(),
		"banned_at must survive a re-ban -- it is when the ban actually began")

	bans := h.listBans(t)
	assert.Len(t, bans, 1, "a re-ban must not create a second row")

	// The log must show the edit as an edit. Recording it as a fresh "banned"
	// would imply the user had been unbanned in between, which never happened.
	events := h.banEvents(t)
	require.Len(t, events, 2)
	assert.Equal(t, core.BanEventModified, events[0].Action,
		"a re-ban logs 'modified', newest first")
	assert.Equal(t, core.BanEventBanned, events[1].Action)
}

// A moderator who is also banned is a state no enforcement path can read, and
// clearing it is owner-only -- so allowing it would also let a moderator
// neutralise a peer.
func TestCommunitiesAPI_BanUser_StaffRejected(t *testing.T) {
	h := newHarness(t)

	body := []byte(fmt.Sprintf(`{"user_id":%d}`, h.moderator.ID))
	rec := h.request(t, h.owner, http.MethodPost, banPath(), body, false)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())

	body = []byte(fmt.Sprintf(`{"user_id":%d}`, h.owner.ID))
	rec = h.request(t, h.moderator, http.MethodPost, banPath(), body, false)
	assert.Equal(t, http.StatusBadRequest, rec.Code,
		"a moderator must not be able to ban the owner")
}

func TestCommunitiesAPI_BanUser_UnknownUser(t *testing.T) {
	h := newHarness(t)

	// Without an existence check this trips a foreign key and renders as a 500,
	// telling the moderator the server broke rather than what they got wrong.
	rec := h.request(t, h.moderator, http.MethodPost, banPath(),
		[]byte(`{"user_id":999999}`), false)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
}

// An expiry in the past would write a ban that is inert on arrival: it lands on
// the list already lapsed and enforces nothing. That is never the intent.
func TestCommunitiesAPI_BanUser_PastExpiryRejected(t *testing.T) {
	h := newHarness(t)

	past := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	body := []byte(fmt.Sprintf(`{"user_id":%d,"expires_at":%q}`, h.outsider.ID, past))
	rec := h.request(t, h.moderator, http.MethodPost, banPath(), body, false)
	require.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())

	assert.Empty(t, h.listBans(t), "a rejected expiry must not write a ban")
}

func TestCommunitiesAPI_BanUser_FutureExpiryAccepted(t *testing.T) {
	h := newHarness(t)

	future := time.Now().Add(14 * 24 * time.Hour).UTC().Format(time.RFC3339)
	body := []byte(fmt.Sprintf(`{"user_id":%d,"expires_at":%q}`, h.outsider.ID, future))
	rec := h.request(t, h.moderator, http.MethodPost, banPath(), body, false)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var got core.CommunityBan
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.NotNil(t, got.ExpiresAt, "a temporary ban must carry its expiry")
	assert.True(t, got.IsActive, "a ban expiring in the future is being enforced now")
}

func TestCommunitiesAPI_ListBans_AsModerator(t *testing.T) {
	h := newHarness(t)

	body := []byte(fmt.Sprintf(`{"user_id":%d,"reason":"griefing"}`, h.outsider.ID))
	require.Equal(t, http.StatusOK,
		h.request(t, h.moderator, http.MethodPost, banPath(), body, false).Code)

	rec := h.request(t, h.moderator, http.MethodGet, banPath(), nil, false)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var got []*core.CommunityBan
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got, 1)
	assert.Equal(t, int32(h.outsider.ID), got[0].UserID)
	// The list is the management view: an id alone leaves the UI unable to
	// render who was banned without a second round-trip per row.
	assert.Equal(t, h.outsider.Username, got[0].Username,
		"the banlist must carry the banned user's username")
}

// The banlist names users and carries a moderator's stated reason, so it is
// moderation-internal rather than public profile information.
func TestCommunitiesAPI_ListBans_OutsiderForbidden(t *testing.T) {
	h := newHarness(t)

	rec := h.request(t, h.outsider, http.MethodGet, banPath(), nil, false)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCommunitiesAPI_ListBans_IncludesExpired(t *testing.T) {
	h := newHarness(t)

	// Written directly: the endpoint refuses a past expiry, and rightly so, but
	// a ban that has since lapsed is exactly what this asserts is still listed.
	_, err := h.testDB.Pool.Exec(context.Background(),
		`INSERT INTO community_bans (community_id, user_id, reason, banned_by_user_id, expires_at)
		 VALUES ($1, $2, 'lapsed', $3, NOW() - INTERVAL '1 day')`,
		h.community.ID, h.outsider.ID, h.moderator.ID)
	require.NoError(t, err)

	rec := h.request(t, h.moderator, http.MethodGet, banPath(), nil, false)
	require.Equal(t, http.StatusOK, rec.Code)

	var got []*core.CommunityBan
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got, 1, "an expired ban must stay on the management list, not vanish")
	assert.False(t, got[0].IsActive,
		"a lapsed ban must be marked inactive -- presence of a row never means banned")
}

func TestCommunitiesAPI_UnbanUser_AsModerator(t *testing.T) {
	h := newHarness(t)

	body := []byte(fmt.Sprintf(`{"user_id":%d,"reason":"griefing"}`, h.outsider.ID))
	require.Equal(t, http.StatusOK,
		h.request(t, h.moderator, http.MethodPost, banPath(), body, false).Code)

	rec := h.request(t, h.moderator, http.MethodDelete,
		fmt.Sprintf("%s/%d", banPath(), h.outsider.ID), nil, false)
	require.Equal(t, http.StatusNoContent, rec.Code, "body: %s", rec.Body.String())

	assert.Empty(t, h.listBans(t), "lifting a ban deletes its row")

	// Which is why the log matters: with the row gone, this is the only
	// evidence the ban ever existed, and it keeps what the ban said.
	events := h.banEvents(t)
	require.Len(t, events, 2)
	assert.Equal(t, core.BanEventUnbanned, events[0].Action)
	require.NotNil(t, events[0].Reason)
	assert.Equal(t, "griefing", *events[0].Reason,
		"the unban event must snapshot what the deleted ban said")
}

func TestCommunitiesAPI_UnbanUser_OutsiderForbidden(t *testing.T) {
	h := newHarness(t)

	body := []byte(fmt.Sprintf(`{"user_id":%d}`, h.admin.ID))
	require.Equal(t, http.StatusOK,
		h.request(t, h.moderator, http.MethodPost, banPath(), body, false).Code)

	rec := h.request(t, h.outsider, http.MethodDelete,
		fmt.Sprintf("%s/%d", banPath(), h.admin.ID), nil, false)
	require.Equal(t, http.StatusForbidden, rec.Code)

	assert.Len(t, h.listBans(t), 1, "a refused unban must leave the ban in place")
}

// Unlike removing a moderator, this 404s: the moderator reached it from a
// banlist, so a missing row means their view is stale and someone else already
// lifted it. Reporting success would hide that.
func TestCommunitiesAPI_UnbanUser_NotBanned(t *testing.T) {
	h := newHarness(t)

	rec := h.request(t, h.moderator, http.MethodDelete,
		fmt.Sprintf("%s/%d", banPath(), h.outsider.ID), nil, false)
	assert.Equal(t, http.StatusNotFound, rec.Code, "body: %s", rec.Body.String())
}

func TestCommunitiesAPI_ListBanEvents_AsModerator(t *testing.T) {
	h := newHarness(t)

	body := []byte(fmt.Sprintf(`{"user_id":%d,"reason":"griefing"}`, h.outsider.ID))
	require.Equal(t, http.StatusOK,
		h.request(t, h.moderator, http.MethodPost, banPath(), body, false).Code)

	rec := h.request(t, h.moderator, http.MethodGet,
		"/api/v1/communities/midnight-ravens/ban-events", nil, false)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var got []*core.CommunityBanEvent
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got, 1)
	assert.Equal(t, core.BanEventBanned, got[0].Action)
	assert.Equal(t, int32(h.outsider.ID), got[0].TargetUserID)
	// The log exists to settle "who banned whom", so both names must be on it
	// without the reader having to resolve ids themselves.
	require.NotNil(t, got[0].ActorUsername)
	assert.Equal(t, h.moderator.Username, *got[0].ActorUsername)
	require.NotNil(t, got[0].TargetUsername)
	assert.Equal(t, h.outsider.Username, *got[0].TargetUsername)
}

func TestCommunitiesAPI_ListBanEvents_OutsiderForbidden(t *testing.T) {
	h := newHarness(t)

	rec := h.request(t, h.outsider, http.MethodGet,
		"/api/v1/communities/midnight-ravens/ban-events", nil, false)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCommunitiesAPI_ListBanEvents_Paged(t *testing.T) {
	h := newHarness(t)

	// Three events on one user: banned, modified, unbanned.
	body := []byte(fmt.Sprintf(`{"user_id":%d,"reason":"one"}`, h.outsider.ID))
	require.Equal(t, http.StatusOK,
		h.request(t, h.moderator, http.MethodPost, banPath(), body, false).Code)
	body = []byte(fmt.Sprintf(`{"user_id":%d,"reason":"two"}`, h.outsider.ID))
	require.Equal(t, http.StatusOK,
		h.request(t, h.moderator, http.MethodPost, banPath(), body, false).Code)
	require.Equal(t, http.StatusNoContent,
		h.request(t, h.moderator, http.MethodDelete,
			fmt.Sprintf("%s/%d", banPath(), h.outsider.ID), nil, false).Code)

	rec := h.request(t, h.moderator, http.MethodGet,
		"/api/v1/communities/midnight-ravens/ban-events?limit=2", nil, false)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var page []*core.CommunityBanEvent
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
	require.Len(t, page, 2, "limit must bound the page")
	assert.Equal(t, core.BanEventUnbanned, page[0].Action, "newest first")

	rec = h.request(t, h.moderator, http.MethodGet,
		"/api/v1/communities/midnight-ravens/ban-events?limit=2&offset=2", nil, false)
	require.Equal(t, http.StatusOK, rec.Code)

	var second []*core.CommunityBanEvent
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &second))
	require.Len(t, second, 1, "offset must skip into the tail of the log")
	assert.Equal(t, core.BanEventBanned, second[0].Action, "the oldest event is the original ban")
}

// Bans are per-community -- that separation is the entire reason Communities
// exists. A moderator of one community must not be able to reach another's
// banlist by swapping the slug.
func TestCommunitiesAPI_Bans_ScopedBySlug(t *testing.T) {
	h := newHarness(t)

	svc := &communitysvc.CommunityService{DB: h.testDB.Pool, Logger: h.app.ObsLogger}
	other, err := svc.CreateCommunity(context.Background(), &core.CreateCommunityRequest{
		Name:        "Harbor Lights",
		Slug:        "harbor-lights",
		OwnerUserID: int32(h.outsider.ID),
	})
	require.NoError(t, err)

	body := []byte(fmt.Sprintf(`{"user_id":%d}`, h.admin.ID))
	rec := h.request(t, h.moderator, http.MethodPost,
		"/api/v1/communities/harbor-lights/bans", body, false)
	require.Equal(t, http.StatusForbidden, rec.Code,
		"moderating one community must not confer power over another")

	bans, err := svc.ListBans(context.Background(), other.ID)
	require.NoError(t, err)
	assert.Empty(t, bans, "the other community's banlist must be untouched")
}

// is_banned reports the CALLER's own standing, so it must be computed per
// request rather than shared -- the same reason your_role is.
func TestCommunitiesAPI_IsBanned_IsPerCaller(t *testing.T) {
	h := newHarness(t)

	body := []byte(fmt.Sprintf(`{"user_id":%d}`, h.outsider.ID))
	require.Equal(t, http.StatusOK,
		h.request(t, h.moderator, http.MethodPost, banPath(), body, false).Code)

	// The banned user sees the flag set.
	rec := h.request(t, h.outsider, http.MethodGet, "/api/v1/communities/midnight-ravens", nil, false)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	var mine core.Community
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &mine))
	assert.True(t, mine.IsBanned, "the banned caller must see is_banned true")

	// Nobody else's standing leaks: the admin is not banned and reads the same
	// community with the flag clear.
	rec = h.request(t, h.admin, http.MethodGet, "/api/v1/communities/midnight-ravens", nil, false)
	require.Equal(t, http.StatusOK, rec.Code)
	var theirs core.Community
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &theirs))
	assert.False(t, theirs.IsBanned,
		"is_banned must describe the CALLER, not the community")
}

// The listing is what the game-creation picker actually reads, so the flag has
// to survive the plural path too -- and be stamped on the right row.
func TestCommunitiesAPI_IsBanned_OnListing(t *testing.T) {
	h := newHarness(t)

	svc := &communitysvc.CommunityService{DB: h.testDB.Pool, Logger: h.app.ObsLogger}
	other, err := svc.CreateCommunity(context.Background(), &core.CreateCommunityRequest{
		Name:        "Harbor Lights",
		Slug:        "harbor-lights",
		OwnerUserID: int32(h.owner.ID),
	})
	require.NoError(t, err)

	body := []byte(fmt.Sprintf(`{"user_id":%d}`, h.outsider.ID))
	require.Equal(t, http.StatusOK,
		h.request(t, h.moderator, http.MethodPost, banPath(), body, false).Code)

	rec := h.request(t, h.outsider, http.MethodGet, "/api/v1/communities", nil, false)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var list []core.Community
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))

	seen := map[int32]bool{}
	for _, c := range list {
		seen[c.ID] = c.IsBanned
	}
	assert.True(t, seen[h.community.ID], "the banned community must be flagged")
	assert.False(t, seen[other.ID],
		"a ban in one community must not flag another -- that separation is the whole feature")
}

// An EXPIRED ban must not flag a community: the user may create games there
// again, and the create endpoint would allow it. Never infer "banned" from a
// row's presence.
func TestCommunitiesAPI_IsBanned_ExcludesExpired(t *testing.T) {
	h := newHarness(t)

	_, err := h.testDB.Pool.Exec(context.Background(),
		`INSERT INTO community_bans (community_id, user_id, reason, banned_by_user_id, expires_at)
		 VALUES ($1, $2, 'lapsed', $3, NOW() - INTERVAL '1 day')`,
		h.community.ID, h.outsider.ID, h.moderator.ID)
	require.NoError(t, err)

	rec := h.request(t, h.outsider, http.MethodGet, "/api/v1/communities/midnight-ravens", nil, false)
	require.Equal(t, http.StatusOK, rec.Code)

	var got core.Community
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.False(t, got.IsBanned,
		"a lapsed ban must not flag the community")
}
