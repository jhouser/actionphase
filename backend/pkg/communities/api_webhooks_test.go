package communities

// HTTP-level tests for the community webhook endpoints (req 9).
//
// Two things are asserted here that no lower layer can prove on its own:
//
//   - The URL is a CREDENTIAL, and no response on any path returns it. Service
//     tests can show the converter masks; only these tests show that every
//     handler goes through that converter and none of them leaks.
//   - Every endpoint is moderator-gated. There is no public read at all, unlike
//     documents -- an ordinary member has no business seeing a channel
//     credential or its delivery status.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"actionphase/pkg/core"
)

// The token is distinctive so a leak anywhere is unmistakable in a diff.
const secretToken = "SuperSecretWebhookTokenABCDEF"

var validHookURL = "https://discord.com/api/webhooks/998877/" + secretToken

func webhooksPath(slug string) string {
	return "/api/v1/communities/" + slug + "/webhooks"
}

// createHook adds a webhook as the owner and returns it.
func createHook(t *testing.T, h *harness, events []string) *core.CommunityWebhook {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"url":    validHookURL,
		"label":  "#recruitment",
		"events": events,
	})
	require.NoError(t, err)

	rec := h.request(t, h.owner, http.MethodPost, webhooksPath("midnight-ravens"), body, false)
	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

	var got core.CommunityWebhook
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	return &got
}

// ------------------------------------------------------- credential handling

// The assertion this whole feature turns on: the raw URL must not come back out
// of ANY endpoint. Checked across every response shape rather than once, since
// a single handler forgetting the masked converter is exactly the bug.
func TestWebhooksAPI_NeverReturnsUnmaskedURL(t *testing.T) {
	h := newHarness(t)
	hook := createHook(t, h, []string{core.GameStateRecruitment})

	paths := []struct {
		name   string
		method string
		path   string
		body   []byte
	}{
		{"create", http.MethodPost, webhooksPath("midnight-ravens"),
			[]byte(`{"url":"` + validHookURL + `","events":["recruitment"]}`)},
		{"list", http.MethodGet, webhooksPath("midnight-ravens"), nil},
		{"update", http.MethodPatch,
			fmt.Sprintf("%s/%d", webhooksPath("midnight-ravens"), hook.ID),
			[]byte(`{"label":"#general"}`)},
	}

	for _, p := range paths {
		t.Run(p.name, func(t *testing.T) {
			rec := h.request(t, h.owner, p.method, p.path, p.body, false)
			require.Less(t, rec.Code, 300, "body: %s", rec.Body.String())
			assert.NotContains(t, rec.Body.String(), secretToken,
				"%s leaked the webhook token", p.name)
			assert.Contains(t, rec.Body.String(), "••••",
				"%s should return a masked URL", p.name)
		})
	}
}

// A masked URL echoed back as an update must not overwrite the real credential.
// This is the round trip the config form actually performs: it received a mask,
// and if it sends anything back it must not corrupt the stored URL.
func TestWebhooksAPI_UpdateWithoutURLPreservesCredential(t *testing.T) {
	h := newHarness(t)
	hook := createHook(t, h, []string{core.GameStateRecruitment})

	rec := h.request(t, h.owner, http.MethodPatch,
		fmt.Sprintf("%s/%d", webhooksPath("midnight-ravens"), hook.ID),
		[]byte(`{"label":"#general","is_enabled":false}`), false)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var stored string
	require.NoError(t, h.testDB.Pool.QueryRow(t.Context(),
		"SELECT url FROM community_webhooks WHERE id = $1", hook.ID).Scan(&stored))
	assert.Equal(t, validHookURL, stored,
		"omitting the URL must keep the stored credential, not blank or mask it")
}

// ------------------------------------------------------------ authorization

// Every endpoint is moderator-gated -- there is no public read here at all.
func TestWebhooksAPI_OutsiderIsRefusedEverywhere(t *testing.T) {
	h := newHarness(t)
	hook := createHook(t, h, []string{core.GameStateRecruitment})
	one := fmt.Sprintf("%s/%d", webhooksPath("midnight-ravens"), hook.ID)

	cases := []struct {
		name   string
		method string
		path   string
		body   []byte
	}{
		{"list", http.MethodGet, webhooksPath("midnight-ravens"), nil},
		{"create", http.MethodPost, webhooksPath("midnight-ravens"),
			[]byte(`{"url":"` + validHookURL + `"}`)},
		{"update", http.MethodPatch, one, []byte(`{"label":"x"}`)},
		{"delete", http.MethodDelete, one, nil},
		{"test", http.MethodPost, one + "/test", nil},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := h.request(t, h.outsider, c.method, c.path, c.body, false)
			assert.Equal(t, http.StatusForbidden, rec.Code,
				"an outsider must not reach %s; body: %s", c.name, rec.Body.String())
			assert.NotContains(t, rec.Body.String(), secretToken)
		})
	}
}

// A plain moderator manages webhooks fully. Unlike the moderator ROSTER, which
// is owner-only (req 4), webhook config is ordinary moderation work.
func TestWebhooksAPI_ModeratorMayManage(t *testing.T) {
	h := newHarness(t)

	body := []byte(`{"url":"` + validHookURL + `","events":["recruitment"]}`)
	rec := h.request(t, h.moderator, http.MethodPost, webhooksPath("midnight-ravens"), body, false)
	assert.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

	rec = h.request(t, h.moderator, http.MethodGet, webhooksPath("midnight-ravens"), nil, false)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// ---------------------------------------------------------------- validation

// The SSRF control, at the HTTP boundary. Without it the server is an open
// outbound request proxy for anyone who can moderate a community.
func TestWebhooksAPI_RejectsNonDiscordURLs(t *testing.T) {
	h := newHarness(t)

	for _, bad := range []string{
		"https://evil.test/api/webhooks/1/t",
		"https://discord.com.evil.test/api/webhooks/1/t",
		"http://discord.com/api/webhooks/1/t",
		"https://169.254.169.254/api/webhooks/1/t",
		"https://discord.com/api/users/@me",
		"",
	} {
		t.Run(bad, func(t *testing.T) {
			body, err := json.Marshal(map[string]any{"url": bad})
			require.NoError(t, err)

			rec := h.request(t, h.owner, http.MethodPost, webhooksPath("midnight-ravens"), body, false)
			assert.Equal(t, http.StatusBadRequest, rec.Code,
				"must reject %q; body: %s", bad, rec.Body.String())
		})
	}
}

func TestWebhooksAPI_RejectsUnknownEvents(t *testing.T) {
	h := newHarness(t)

	body := []byte(`{"url":"` + validHookURL + `","events":["not_a_state"]}`)
	rec := h.request(t, h.owner, http.MethodPost, webhooksPath("midnight-ravens"), body, false)

	assert.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
}

// `setup` is a valid game state but NOT a notifiable one: a game in setup is
// not yet public, and announcing it would leak an unlisted game.
func TestWebhooksAPI_RejectsSetupAsEvent(t *testing.T) {
	h := newHarness(t)

	body := []byte(`{"url":"` + validHookURL + `","events":["setup"]}`)
	rec := h.request(t, h.owner, http.MethodPost, webhooksPath("midnight-ravens"), body, false)

	assert.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
}

// ------------------------------------------------------------------ scoping

// A webhook addressed through the WRONG community must 404, even for someone
// who moderates both. Without the (id, community_id) pairing this would let a
// moderator of A read or overwrite B's channel credential.
func TestWebhooksAPI_CrossCommunityAccessIsNotFound(t *testing.T) {
	h := newHarness(t)
	hook := createHook(t, h, []string{core.GameStateRecruitment})

	// A second community owned by the SAME user, so the moderator gate passes
	// and the scoping check is what is actually under test.
	other, err := h.communityService().CreateCommunity(t.Context(), &core.CreateCommunityRequest{
		Name:        "Harbor Lights",
		Slug:        "harbor-lights",
		OwnerUserID: int32(h.owner.ID),
	})
	require.NoError(t, err)
	require.NotEqual(t, other.ID, hook.CommunityID)

	rec := h.request(t, h.owner, http.MethodPatch,
		fmt.Sprintf("%s/%d", webhooksPath("harbor-lights"), hook.ID),
		[]byte(`{"label":"hijacked"}`), false)

	assert.Equal(t, http.StatusNotFound, rec.Code,
		"a webhook in another community must not resolve; body: %s", rec.Body.String())

	// And nothing was written.
	var label *string
	require.NoError(t, h.testDB.Pool.QueryRow(t.Context(),
		"SELECT label FROM community_webhooks WHERE id = $1", hook.ID).Scan(&label))
	require.NotNil(t, label)
	assert.Equal(t, "#recruitment", *label)
}

// ------------------------------------------------------------------- delete

func TestWebhooksAPI_Delete(t *testing.T) {
	h := newHarness(t)
	hook := createHook(t, h, []string{core.GameStateRecruitment})
	one := fmt.Sprintf("%s/%d", webhooksPath("midnight-ravens"), hook.ID)

	rec := h.request(t, h.owner, http.MethodDelete, one, nil, false)
	require.Equal(t, http.StatusNoContent, rec.Code, "body: %s", rec.Body.String())

	// Deleting again is a 404 rather than a silent success: a delete that
	// matched nothing would otherwise report that a live webhook was
	// disconnected when it was not.
	rec = h.request(t, h.owner, http.MethodDelete, one, nil, false)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// --------------------------------------------------------------- test button

func TestWebhooksAPI_TestButtonReportsSuccess(t *testing.T) {
	h := newHarness(t)
	testWebhookSender.Reset()
	hook := createHook(t, h, []string{core.GameStateRecruitment})

	rec := h.request(t, h.owner, http.MethodPost,
		fmt.Sprintf("%s/%d/test", webhooksPath("midnight-ravens"), hook.ID), nil, false)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	// The send used the REAL URL, not the mask -- a masked URL would 404 at
	// Discord, which is the bug this asserts against.
	sent := testWebhookSender.Sent()
	require.Len(t, sent, 1)
	assert.Equal(t, validHookURL, sent[0].URL)

	// Success stamps the status a moderator reads.
	var successAt *string
	require.NoError(t, h.testDB.Pool.QueryRow(t.Context(),
		"SELECT last_success_at::text FROM community_webhooks WHERE id = $1", hook.ID).Scan(&successAt))
	assert.NotNil(t, successAt)
}

func TestWebhooksAPI_TestButtonReportsFailure(t *testing.T) {
	h := newHarness(t)
	testWebhookSender.Reset()
	testWebhookSender.ShouldFail = true
	defer func() { testWebhookSender.ShouldFail = false }()

	hook := createHook(t, h, []string{core.GameStateRecruitment})

	rec := h.request(t, h.owner, http.MethodPost,
		fmt.Sprintf("%s/%d/test", webhooksPath("midnight-ravens"), hook.ID), nil, false)

	// 502, not 500: the failure is Discord's and the moderator needs the reason.
	assert.Equal(t, http.StatusBadGateway, rec.Code, "body: %s", rec.Body.String())
	assert.NotContains(t, rec.Body.String(), secretToken,
		"a failure message must not carry the credential")

	// A failed test leaves the same diagnosis as a failed delivery.
	var lastErr *string
	require.NoError(t, h.testDB.Pool.QueryRow(t.Context(),
		"SELECT last_error FROM community_webhooks WHERE id = $1", hook.ID).Scan(&lastErr))
	require.NotNil(t, lastErr)
	assert.NotContains(t, *lastErr, secretToken)
}
