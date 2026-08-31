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
