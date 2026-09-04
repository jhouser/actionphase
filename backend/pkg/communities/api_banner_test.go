package communities

// HTTP-level tests for community banner upload and removal (Phase 7).
//
// A banner is an uploaded OBJECT plus a column, and the bugs worth guarding
// against are the ones where those two drift apart. A status code cannot see
// any of them, so these tests assert against the fake storage backend as well
// as the response:
//
//   - replacing a banner deletes the old object rather than orphaning it
//   - a failed column write rolls the newly stored object back
//   - deleting removes the object AND clears the column
//
// The permission boundary is the other half: banners are moderator-tier, unlike
// game banners which are primary-GM-only.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"actionphase/pkg/core"
)

// bannerWriteFailureService fails only UpdateCommunityBannerURL, embedding the
// real service for everything else so the handler reaches the write through
// normal resolution -- an all-failing fake would 404 before the upload.
type bannerWriteFailureService struct {
	core.CommunityServiceInterface
	err error
}

func (s *bannerWriteFailureService) UpdateCommunityBannerURL(
	_ context.Context, _ int32, _ *string,
) (*core.Community, error) {
	return nil, s.err
}

// swapCommunityService rebuilds the router around a substitute service.
//
// The router captures its handler at construction, so replacing the service
// means rebuilding both -- the same constraint that makes testWebhookSender
// package-level.
func (h *harness) swapCommunityService(t *testing.T, svc core.CommunityServiceInterface) {
	t.Helper()

	original := h.router
	h.router = setupCommunityTestRouterWithService(h.app, h.testDB, svc)
	t.Cleanup(func() { h.router = original })
}

func bannerPath(slug string) string {
	return "/api/v1/communities/" + slug + "/banner"
}

// uploadBanner issues a multipart banner upload as the given user.
//
// filename and contentType are set independently so a test can send a filename
// with no extension, or omit the content type entirely, which is what forces
// the handler's MIME-inference fallback to run.
func (h *harness) uploadBanner(
	t *testing.T, u *core.User, slug, filename, contentType string, content []byte,
) *httptest.ResponseRecorder {
	t.Helper()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	hdr := make(map[string][]string)
	hdr["Content-Disposition"] = []string{
		fmt.Sprintf(`form-data; name="banner"; filename=%q`, filename),
	}
	// An empty contentType omits the part header entirely rather than sending a
	// blank one, matching a client that does not declare a type.
	if contentType != "" {
		hdr["Content-Type"] = []string{contentType}
	}
	part, err := w.CreatePart(hdr)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	req := httptest.NewRequest(http.MethodPost, bannerPath(slug), &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+h.token(t, u))

	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

// pngBytes is a plausible image body. Content is never decoded -- the handlers
// validate the declared MIME type and the size, not the pixels.
func pngBytes(n int) []byte {
	return append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte("x"), n)...)
}

// bannerURLOf reads the column straight from the database, so an assertion
// cannot be satisfied by a handler that returns the right JSON without writing.
func (h *harness) bannerURLOf(t *testing.T, communityID int32) *string {
	t.Helper()
	var url *string
	err := h.testDB.Pool.QueryRow(context.Background(),
		"SELECT banner_url FROM communities WHERE id = $1", communityID).Scan(&url)
	require.NoError(t, err)
	return url
}

// ------------------------------------------------------------------ upload

func TestCommunityBanner_UploadStoresObjectAndStampsColumn(t *testing.T) {
	h := newHarness(t)

	rec := h.uploadBanner(t, h.owner, "midnight-ravens", "banner.png", "image/png", pngBytes(64))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var got core.Community
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.NotNil(t, got.BannerURL, "response should carry the new banner URL")

	// The response and the column must agree; returning a URL that was never
	// persisted is precisely the failure this endpoint exists to avoid.
	stored := h.bannerURLOf(t, h.community.ID)
	require.NotNil(t, stored, "banner_url should be set in the database")
	assert.Equal(t, *stored, *got.BannerURL)

	assert.Equal(t, 1, testStorage.uploadCount(), "exactly one object should be stored")
	assert.True(t, testStorage.stored(core.ExtractBannerPathFromURL(*stored)),
		"the URL in the column should point at a stored object")
}

// The storage key must be namespaced by community ID rather than slug: the id
// is immutable, so a renamed community's objects stay addressable.
func TestCommunityBanner_StoragePathUsesCommunityID(t *testing.T) {
	h := newHarness(t)

	rec := h.uploadBanner(t, h.owner, "midnight-ravens", "banner.png", "image/png", pngBytes(16))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	path := core.ExtractBannerPathFromURL(*h.bannerURLOf(t, h.community.ID))
	assert.True(t, strings.HasPrefix(path, fmt.Sprintf("banners/communities/%d/", h.community.ID)),
		"unexpected storage path %q", path)
	assert.True(t, strings.HasSuffix(path, ".png"), "extension should be preserved: %q", path)
}

// A filename with no extension must still produce a typed storage path, taken
// from the declared MIME. Otherwise the object is stored extensionless and is
// served with the wrong type by static hosts that sniff from the path.
func TestCommunityBanner_ExtensionInferredFromMimeWhenFilenameHasNone(t *testing.T) {
	h := newHarness(t)

	rec := h.uploadBanner(t, h.owner, "midnight-ravens", "banner", "image/webp", pngBytes(16))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	path := core.ExtractBannerPathFromURL(*h.bannerURLOf(t, h.community.ID))
	assert.True(t, strings.HasSuffix(path, ".webp"), "expected .webp from the MIME type, got %q", path)
}

// A client that declares no content type falls back to inference from the
// filename, rather than being rejected as untyped.
func TestCommunityBanner_MimeInferredFromFilenameWhenAbsent(t *testing.T) {
	h := newHarness(t)

	rec := h.uploadBanner(t, h.owner, "midnight-ravens", "banner.jpg", "", pngBytes(16))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	path := core.ExtractBannerPathFromURL(*h.bannerURLOf(t, h.community.ID))
	assert.True(t, strings.HasSuffix(path, ".jpg"), "expected .jpg inferred from filename, got %q", path)
}

// THE ordering rule: replacing a banner must delete the object it replaces.
// Without this a community accumulates one orphaned file per edit, invisible
// through the API and only discovered as a storage bill.
func TestCommunityBanner_ReplacingDeletesThePreviousObject(t *testing.T) {
	h := newHarness(t)

	first := h.uploadBanner(t, h.owner, "midnight-ravens", "one.png", "image/png", pngBytes(16))
	require.Equal(t, http.StatusOK, first.Code, "body: %s", first.Body.String())
	firstPath := core.ExtractBannerPathFromURL(*h.bannerURLOf(t, h.community.ID))

	second := h.uploadBanner(t, h.owner, "midnight-ravens", "two.png", "image/png", pngBytes(16))
	require.Equal(t, http.StatusOK, second.Code, "body: %s", second.Body.String())
	secondPath := core.ExtractBannerPathFromURL(*h.bannerURLOf(t, h.community.ID))

	require.NotEqual(t, firstPath, secondPath, "the replacement should be a distinct object")
	assert.False(t, testStorage.stored(firstPath), "the replaced object should have been deleted")
	assert.True(t, testStorage.stored(secondPath), "the new object should be stored")
}

// The rollback: if stamping the column fails, the object just uploaded must be
// removed, or the bucket keeps a file no row references.
//
// Forced through a service stub rather than by corrupting data, because there
// is no HTTP-reachable state that makes the UPDATE fail AFTER the handler has
// already resolved the community and stored the file -- which is precisely the
// window the rollback exists to cover.
func TestCommunityBanner_RollsBackObjectWhenColumnWriteFails(t *testing.T) {
	h := newHarness(t)

	failing := &bannerWriteFailureService{
		CommunityServiceInterface: h.communityService(),
		err:                       errors.New("column write failed"),
	}
	h.swapCommunityService(t, failing)

	rec := h.uploadBanner(t, h.owner, "midnight-ravens", "b.png", "image/png", pngBytes(16))
	require.Equal(t, http.StatusInternalServerError, rec.Code, "body: %s", rec.Body.String())

	assert.Nil(t, h.bannerURLOf(t, h.community.ID), "a failed write must leave the column unset")

	// The object was stored, then rolled back. Asserting both halves matters:
	// "never uploaded" would also satisfy a bare stored()==false check, and
	// would mean the rollback was never exercised at all.
	require.Equal(t, 1, testStorage.uploadCount(), "the object should have been stored first")
	assert.False(t, testStorage.stored(testStorage.uploads[0]),
		"the stored object should have been rolled back after the failed write")
}

// A failed upload stores nothing and leaves the column untouched.
func TestCommunityBanner_UploadFailureStampsNothing(t *testing.T) {
	h := newHarness(t)

	testStorage.UploadErr = errors.New("storage unavailable")
	t.Cleanup(func() { testStorage.UploadErr = nil })

	rec := h.uploadBanner(t, h.owner, "midnight-ravens", "b.png", "image/png", pngBytes(16))
	assert.Equal(t, http.StatusInternalServerError, rec.Code, "body: %s", rec.Body.String())

	assert.Nil(t, h.bannerURLOf(t, h.community.ID), "a failed upload must not stamp the column")
	assert.Equal(t, 0, testStorage.uploadCount(), "nothing should have been stored")
}

// ------------------------------------------------------------- validation

func TestCommunityBanner_RejectsDisallowedType(t *testing.T) {
	h := newHarness(t)

	rec := h.uploadBanner(t, h.owner, "midnight-ravens", "evil.svg", "image/svg+xml", []byte("<svg/>"))
	assert.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())

	assert.Nil(t, h.bannerURLOf(t, h.community.ID), "a rejected upload must not stamp the column")
	assert.Equal(t, 0, testStorage.uploadCount(), "a rejected upload must not store an object")
}

// An unrecognised extension with no declared type infers
// application/octet-stream, which the allowlist rejects -- so an unknown file
// is refused rather than stored untyped.
func TestCommunityBanner_RejectsUnknownExtensionWithNoDeclaredType(t *testing.T) {
	h := newHarness(t)

	rec := h.uploadBanner(t, h.owner, "midnight-ravens", "payload.bin", "", []byte("binary"))
	assert.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
	assert.Equal(t, 0, testStorage.uploadCount())
}

func TestCommunityBanner_RejectsOversizeImage(t *testing.T) {
	h := newHarness(t)

	rec := h.uploadBanner(t, h.owner, "midnight-ravens", "huge.png", "image/png",
		pngBytes(core.MaxBannerSize+1))
	assert.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())

	assert.Nil(t, h.bannerURLOf(t, h.community.ID))
	assert.Equal(t, 0, testStorage.uploadCount(), "an oversize upload must not be stored")
}

// ------------------------------------------------------------- permissions

// Banners are moderator-tier, NOT owner-only: keeping the profile current is
// ordinary upkeep, matching PATCH /communities/{slug}. This is the deliberate
// difference from game banners, which are primary-GM-only.
func TestCommunityBanner_ModeratorMayUpload(t *testing.T) {
	h := newHarness(t)

	rec := h.uploadBanner(t, h.moderator, "midnight-ravens", "b.png", "image/png", pngBytes(16))
	assert.Equal(t, http.StatusOK, rec.Code, "a moderator should be able to set the banner: %s", rec.Body.String())
}

func TestCommunityBanner_OutsiderForbidden(t *testing.T) {
	h := newHarness(t)

	rec := h.uploadBanner(t, h.outsider, "midnight-ravens", "b.png", "image/png", pngBytes(16))
	assert.Equal(t, http.StatusForbidden, rec.Code, "body: %s", rec.Body.String())
	assert.Equal(t, 0, testStorage.uploadCount(), "a forbidden upload must not store an object")

	del := h.request(t, h.outsider, http.MethodDelete, bannerPath("midnight-ravens"), nil, false)
	assert.Equal(t, http.StatusForbidden, del.Code)
}

func TestCommunityBanner_UnknownCommunityIs404(t *testing.T) {
	h := newHarness(t)

	rec := h.uploadBanner(t, h.owner, "no-such-community", "b.png", "image/png", pngBytes(16))
	assert.Equal(t, http.StatusNotFound, rec.Code, "body: %s", rec.Body.String())
}

// ------------------------------------------------------------------ delete

func TestCommunityBanner_DeleteRemovesObjectAndClearsColumn(t *testing.T) {
	h := newHarness(t)

	up := h.uploadBanner(t, h.owner, "midnight-ravens", "b.png", "image/png", pngBytes(16))
	require.Equal(t, http.StatusOK, up.Code, "body: %s", up.Body.String())
	path := core.ExtractBannerPathFromURL(*h.bannerURLOf(t, h.community.ID))
	require.True(t, testStorage.stored(path))

	rec := h.request(t, h.owner, http.MethodDelete, bannerPath("midnight-ravens"), nil, false)
	require.Equal(t, http.StatusNoContent, rec.Code, "body: %s", rec.Body.String())

	assert.Nil(t, h.bannerURLOf(t, h.community.ID), "banner_url should be cleared")
	assert.False(t, testStorage.stored(path), "the stored object should be deleted")
}

// Deleting when there is no banner succeeds: the caller's intended end state
// already holds. Matches removeModerator's treatment of a non-moderator.
func TestCommunityBanner_DeleteWithNoBannerSucceeds(t *testing.T) {
	h := newHarness(t)

	rec := h.request(t, h.owner, http.MethodDelete, bannerPath("midnight-ravens"), nil, false)
	assert.Equal(t, http.StatusNoContent, rec.Code, "body: %s", rec.Body.String())
	assert.Nil(t, h.bannerURLOf(t, h.community.ID))
	assert.Equal(t, 0, testStorage.deleteCount(), "nothing to delete, so storage should not be called")
}

// ------------------------------------------------------- storage unconfigured

// Storage is optional at the application level, so an unconfigured deployment
// must answer 503 rather than panicking on a nil backend.
func TestCommunityBanner_UploadWithoutStorageIs503(t *testing.T) {
	h := newHarness(t)
	h.app.Storage = nil
	t.Cleanup(func() { h.app.Storage = testStorage })

	rec := h.uploadBanner(t, h.owner, "midnight-ravens", "b.png", "image/png", pngBytes(16))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code, "body: %s", rec.Body.String())
}

// Delete still clears the column with no storage backend: the column is the
// part the API can always fix, and refusing would leave a community stuck with
// a banner it cannot remove.
func TestCommunityBanner_DeleteWithoutStorageStillClearsColumn(t *testing.T) {
	h := newHarness(t)

	up := h.uploadBanner(t, h.owner, "midnight-ravens", "b.png", "image/png", pngBytes(16))
	require.Equal(t, http.StatusOK, up.Code, "body: %s", up.Body.String())

	h.app.Storage = nil
	t.Cleanup(func() { h.app.Storage = testStorage })

	rec := h.request(t, h.owner, http.MethodDelete, bannerPath("midnight-ravens"), nil, false)
	require.Equal(t, http.StatusNoContent, rec.Code, "body: %s", rec.Body.String())
	assert.Nil(t, h.bannerURLOf(t, h.community.ID))
}

// ------------------------------------------------------------- PATCH guard

// banner_url must remain unwritable through the profile PATCH. If huma ever
// starts accepting it, the column could be pointed at a file nothing owns --
// the exact reason this is a dedicated endpoint (see the plan, Phase 7).
func TestCommunityBanner_PatchCannotSetBannerURL(t *testing.T) {
	h := newHarness(t)

	rec := h.request(t, h.owner, http.MethodPatch, "/api/v1/communities/midnight-ravens",
		[]byte(`{"banner_url":"http://evil.test/x.png"}`), false)

	// huma REJECTS the unknown property rather than dropping it, the same
	// treatment that stops a moderator setting owner_user_id.
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, "body: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "banner_url",
		"the rejection should name the offending property")
	assert.Nil(t, h.bannerURLOf(t, h.community.ID), "PATCH must not write banner_url")
}
