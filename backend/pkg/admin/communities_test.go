package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"actionphase/pkg/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// adminRequest issues an authenticated admin request and returns the recorder.
func adminRequest(t *testing.T, router http.Handler, token, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()

	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, jsonBody(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// newAdminHarness returns a router plus an admin token and the admin user.
func newAdminHarness(t *testing.T) (*core.TestDatabase, http.Handler, string, *core.User) {
	t.Helper()

	testDB := core.NewTestDatabase(t)
	t.Cleanup(testDB.Close)
	// communities.owner_user_id is ON DELETE RESTRICT, so leftover communities
	// would block the shared users cleanup other tests in this package use.
	t.Cleanup(func() { testDB.CleanupTables(t, "community_moderators", "communities") })

	app := core.NewTestApp(testDB.Pool)
	router := setupAdminTestRouter(app, testDB)

	admin := testDB.CreateTestUser(t, "commadmin", "commadmin@example.com")
	_, err := testDB.Pool.Exec(context.Background(),
		"UPDATE users SET is_admin = true WHERE id = $1", admin.ID)
	require.NoError(t, err)

	token, err := core.CreateTestJWTTokenForUser(app, admin)
	require.NoError(t, err)

	return testDB, router, token, admin
}

func TestAdminAPI_CreateCommunity(t *testing.T) {
	testDB, router, token, admin := newAdminHarness(t)
	_ = testDB

	body := []byte(fmt.Sprintf(
		`{"name":"Midnight Ravens","slug":"midnight-ravens","description":"Rules live here","owner_user_id":%d}`,
		admin.ID))

	rec := adminRequest(t, router, token, http.MethodPost, "/api/v1/admin/communities", body)
	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

	var got core.Community
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))

	assert.Equal(t, "Midnight Ravens", got.Name)
	assert.Equal(t, "midnight-ravens", got.Slug)
	assert.Equal(t, int32(admin.ID), got.OwnerUserID)
	assert.True(t, got.IsActive)
}

// The handler lowercases a slug before validating, so mixed case is NORMALIZED
// rather than refused. ValidateCommunitySlug itself is stricter -- this asserts
// the handler's contract, which is what an admin actually experiences.
func TestAdminAPI_CreateCommunity_NormalizesSlugCase(t *testing.T) {
	_, router, token, admin := newAdminHarness(t)

	body := []byte(fmt.Sprintf(`{"name":"Case Test","slug":"  MidNight-Ravens  ","owner_user_id":%d}`, admin.ID))

	rec := adminRequest(t, router, token, http.MethodPost, "/api/v1/admin/communities", body)
	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

	var got core.Community
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "midnight-ravens", got.Slug,
		"surrounding space must be trimmed and case folded, not rejected")
}

func TestAdminAPI_CreateCommunity_ValidationErrors(t *testing.T) {
	_, router, token, admin := newAdminHarness(t)

	tests := []struct {
		name string
		body string
		want int
	}{
		{
			name: "reserved slug rejected",
			body: fmt.Sprintf(`{"name":"Reserved","slug":"admin","owner_user_id":%d}`, admin.ID),
			want: http.StatusBadRequest,
		},
		{
			name: "underscore is not a legal slug character",
			body: fmt.Sprintf(`{"name":"Bad","slug":"under_score","owner_user_id":%d}`, admin.ID),
			want: http.StatusBadRequest,
		},
		{
			name: "leading hyphen rejected",
			body: fmt.Sprintf(`{"name":"Bad","slug":"-leading","owner_user_id":%d}`, admin.ID),
			want: http.StatusBadRequest,
		},
		{
			// Whitespace satisfies the schema's minLength, so this is the
			// handler's own trim check firing, not the generated validation.
			name: "whitespace-only name rejected",
			body: fmt.Sprintf(`{"name":"   ","slug":"blank-name","owner_user_id":%d}`, admin.ID),
			want: http.StatusBadRequest,
		},
		{
			// Too short for the schema, so huma rejects before the handler
			// runs. Schema violations are 422; the handler's own checks above
			// stay 400. This case is why the two codes are worth separating.
			name: "single-character name rejected by schema",
			body: fmt.Sprintf(`{"name":"x","slug":"short-name","owner_user_id":%d}`, admin.ID),
			want: http.StatusUnprocessableEntity,
		},
		{
			name: "unknown owner rejected",
			body: `{"name":"Ghost","slug":"ghost-owner","owner_user_id":999999}`,
			want: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := adminRequest(t, router, token, http.MethodPost, "/api/v1/admin/communities", []byte(tt.body))
			assert.Equal(t, tt.want, rec.Code, "body: %s", rec.Body.String())
		})
	}
}

// A slug collision must read as a client error, not a server fault.
func TestAdminAPI_CreateCommunity_DuplicateSlug(t *testing.T) {
	_, router, token, admin := newAdminHarness(t)

	body := []byte(fmt.Sprintf(`{"name":"First","slug":"taken-slug","owner_user_id":%d}`, admin.ID))
	require.Equal(t, http.StatusCreated,
		adminRequest(t, router, token, http.MethodPost, "/api/v1/admin/communities", body).Code)

	dupe := []byte(fmt.Sprintf(`{"name":"Second","slug":"taken-slug","owner_user_id":%d}`, admin.ID))
	rec := adminRequest(t, router, token, http.MethodPost, "/api/v1/admin/communities", dupe)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
}

func TestAdminAPI_ListCommunities(t *testing.T) {
	_, router, token, admin := newAdminHarness(t)

	body := []byte(fmt.Sprintf(`{"name":"Listed","slug":"listed-community","owner_user_id":%d}`, admin.ID))
	createRec := adminRequest(t, router, token, http.MethodPost, "/api/v1/admin/communities", body)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var created core.Community
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	listRec := adminRequest(t, router, token, http.MethodGet, "/api/v1/admin/communities", nil)
	require.Equal(t, http.StatusOK, listRec.Code)

	var list []core.Community
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &list))
	require.NotEmpty(t, list)
	assert.NotEmpty(t, list[0].OwnerUsername, "the listing must carry the joined owner username")

	assert.Condition(t, func() bool {
		for _, c := range list {
			if c.ID == created.ID {
				return true
			}
		}
		return false
	}, "the created community must appear in the listing")
}

func TestAdminAPI_UpdateCommunity_NotFound(t *testing.T) {
	_, router, token, _ := newAdminHarness(t)

	rec := adminRequest(t, router, token, http.MethodPatch,
		"/api/v1/admin/communities/999999", []byte(`{"name":"Ghost"}`))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAdminAPI_UpdateCommunity(t *testing.T) {
	testDB, router, token, admin := newAdminHarness(t)

	body := []byte(fmt.Sprintf(`{"name":"Before","slug":"update-me","description":"keep me","owner_user_id":%d}`, admin.ID))
	createRec := adminRequest(t, router, token, http.MethodPost, "/api/v1/admin/communities", body)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var created core.Community
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	newOwner := testDB.CreateTestUser(t, "newowner", "newowner@example.com")
	patch := []byte(fmt.Sprintf(`{"name":"After","owner_user_id":%d,"is_active":false}`, newOwner.ID))

	rec := adminRequest(t, router, token, http.MethodPatch,
		fmt.Sprintf("/api/v1/admin/communities/%d", created.ID), patch)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var updated core.Community
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &updated))

	assert.Equal(t, "After", updated.Name)
	assert.Equal(t, int32(newOwner.ID), updated.OwnerUserID)
	assert.False(t, updated.IsActive)
	assert.Equal(t, "update-me", updated.Slug, "slug must be immutable")

	require.NotNil(t, updated.Description)
	assert.Equal(t, "keep me", *updated.Description,
		"an omitted field must survive a PATCH untouched")
}

// TestAdminAPI_UpdateCommunity_ClearsDescription proves the clear survives the
// full HTTP round-trip, not just the service call: an omitted field and an
// empty one must mean different things all the way through the huma decoding.
func TestAdminAPI_UpdateCommunity_ClearsDescription(t *testing.T) {
	_, router, token, admin := newAdminHarness(t)

	body := []byte(fmt.Sprintf(
		`{"name":"Blurbed","slug":"clear-via-api","description":"remove me","owner_user_id":%d}`, admin.ID))
	createRec := adminRequest(t, router, token, http.MethodPost, "/api/v1/admin/communities", body)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var created core.Community
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	require.NotNil(t, created.Description, "precondition: the description is set")

	rec := adminRequest(t, router, token, http.MethodPatch,
		fmt.Sprintf("/api/v1/admin/communities/%d", created.ID), []byte(`{"description":""}`))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var updated core.Community
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &updated))
	assert.Nil(t, updated.Description,
		"an empty description in the PATCH body must clear the blurb")
	assert.Equal(t, "Blurbed", updated.Name, "clearing one field must not disturb another")
}

func TestAdminAPI_UpdateCommunity_UnknownOwnerRejected(t *testing.T) {
	_, router, token, admin := newAdminHarness(t)

	body := []byte(fmt.Sprintf(`{"name":"Owner Check","slug":"owner-check","owner_user_id":%d}`, admin.ID))
	createRec := adminRequest(t, router, token, http.MethodPost, "/api/v1/admin/communities", body)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var created core.Community
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	rec := adminRequest(t, router, token, http.MethodPatch,
		fmt.Sprintf("/api/v1/admin/communities/%d", created.ID), []byte(`{"owner_user_id":999999}`))
	assert.Equal(t, http.StatusBadRequest, rec.Code,
		"reassigning to a nonexistent user must be a client error, not a 500")
}

// Requirement 1: only site admins may create communities. The admin group's
// RequireAdmin middleware is what enforces it, so this asserts the route is
// genuinely inside that group.
func TestAdminAPI_Communities_ForbiddenForNonAdmin(t *testing.T) {
	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "community_moderators", "communities")

	app := core.NewTestApp(testDB.Pool)
	router := setupAdminTestRouter(app, testDB)

	regular := testDB.CreateTestUser(t, "regularcomm", "regularcomm@example.com")
	token, err := core.CreateTestJWTTokenForUser(app, regular)
	require.NoError(t, err)

	endpoints := []struct {
		method string
		path   string
		body   []byte
	}{
		{http.MethodGet, "/api/v1/admin/communities", nil},
		{http.MethodPost, "/api/v1/admin/communities", []byte(`{"name":"Nope","slug":"nope","owner_user_id":1}`)},
		{http.MethodPatch, "/api/v1/admin/communities/1", []byte(`{"name":"Nope"}`)},
	}

	for _, e := range endpoints {
		t.Run(fmt.Sprintf("%s %s", e.method, e.path), func(t *testing.T) {
			rec := adminRequest(t, router, token, e.method, e.path, e.body)
			assert.Equal(t, http.StatusForbidden, rec.Code,
				"non-admins must not reach community administration")
		})
	}
}
