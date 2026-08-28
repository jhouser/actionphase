package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"actionphase/pkg/core"

	"github.com/go-chi/jwtauth/v5"
)

// DiscordConnectResponse is returned by GET /api/v1/auth/discord/connect
type DiscordConnectResponse struct {
	URL string `json:"url"`
}

func (r *DiscordConnectResponse) Render(w http.ResponseWriter, req *http.Request) error {
	return nil
}

// DiscordStatusResponse is returned by GET /api/v1/auth/discord/status
type DiscordStatusResponse struct {
	Linked          bool    `json:"linked"`
	DiscordUsername *string `json:"discord_username,omitempty"`
}

func (r *DiscordStatusResponse) Render(w http.ResponseWriter, req *http.Request) error {
	return nil
}

// discordOAuthState encodes the ActionPhase user ID into an HMAC-signed state
// parameter. Format: base64(userID + "." + hmac-sha256-hex).
// This avoids server-side session storage while preventing CSRF.
func (h *Handler) buildDiscordState(userID int32) string {
	payload := strconv.Itoa(int(userID))
	mac := hmac.New(sha256.New, []byte(h.App.Config.JWT.Secret))
	mac.Write([]byte(payload))
	sig := fmt.Sprintf("%x", mac.Sum(nil))
	raw := payload + "." + sig
	return base64.URLEncoding.EncodeToString([]byte(raw))
}

// verifyDiscordState validates the HMAC state and returns the encoded userID.
// Returns an error if the state is tampered or malformed.
func (h *Handler) verifyDiscordState(state string) (int32, error) {
	decoded, err := base64.URLEncoding.DecodeString(state)
	if err != nil {
		return 0, fmt.Errorf("invalid state encoding")
	}

	parts := strings.SplitN(string(decoded), ".", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("malformed state")
	}

	payload, sig := parts[0], parts[1]

	mac := hmac.New(sha256.New, []byte(h.App.Config.JWT.Secret))
	mac.Write([]byte(payload))
	expectedSig := fmt.Sprintf("%x", mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return 0, fmt.Errorf("state HMAC mismatch")
	}

	uid, err := strconv.ParseInt(payload, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid user ID in state")
	}

	return int32(uid), nil
}

// getUserIDFromJWT extracts the user ID from the JWT in the request context.
// Returns an error if the token is missing or the sub claim is invalid.
func getUserIDFromJWT(r *http.Request) (int32, error) {
	token, _, err := jwtauth.FromContext(r.Context())
	if err != nil || token == nil {
		return 0, fmt.Errorf("missing or invalid token")
	}

	userIDStr, ok := token.Get("sub")
	if !ok {
		return 0, fmt.Errorf("sub claim missing")
	}

	var uid int
	if _, err := fmt.Sscanf(userIDStr.(string), "%d", &uid); err != nil || uid == 0 {
		return 0, fmt.Errorf("invalid sub claim")
	}

	return int32(uid), nil
}

// discordTokenResponse models the Discord OAuth2 token endpoint response.
type discordTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// discordUserResponse models the Discord /users/@me endpoint response.
type discordUserResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

// V1DiscordCallback handles the OAuth2 redirect from Discord.
// GET /api/v1/auth/discord/callback (public)
func (h *Handler) V1DiscordCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer h.App.ObsLogger.LogOperation(ctx, "api_discord_callback")()

	frontendBaseURL := os.Getenv("FRONTEND_URL")
	if frontendBaseURL == "" {
		frontendBaseURL = "http://localhost:5173"
	}
	frontendURL := fmt.Sprintf("%s/settings?tab=notifications&discord=linked", frontendBaseURL)

	// 1. Validate state
	state := r.URL.Query().Get("state")
	if state == "" {
		h.App.ObsLogger.Warn(ctx, "Discord callback: missing state parameter")
		http.Error(w, "missing state", http.StatusBadRequest)
		return
	}

	userID, err := h.verifyDiscordState(state)
	if err != nil {
		h.App.ObsLogger.Warn(ctx, "Discord callback: invalid state", "error", err)
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		h.App.ObsLogger.Warn(ctx, "Discord callback: missing code parameter", "user_id", userID)
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	// 2. Exchange code for tokens
	tokens, err := h.exchangeDiscordCode(r, code)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Discord callback: token exchange failed", "user_id", userID)
		http.Error(w, "token exchange failed", http.StatusInternalServerError)
		return
	}

	// 3. Fetch Discord user info
	discordUser, err := h.fetchDiscordUser(r, tokens.AccessToken)
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Discord callback: failed to fetch user info", "user_id", userID)
		http.Error(w, "failed to fetch discord user", http.StatusInternalServerError)
		return
	}

	// 4. Upsert account
	var refreshToken *string
	if tokens.RefreshToken != "" {
		refreshToken = &tokens.RefreshToken
	}

	var expiresAt *time.Time
	if tokens.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)
		expiresAt = &t
	}

	discordSvc := h.DiscordService
	_, err = discordSvc.UpsertDiscordAccount(ctx, &core.UpsertDiscordAccountRequest{
		UserID:          userID,
		DiscordUserID:   discordUser.ID,
		DiscordUsername: discordUser.Username,
		AccessToken:     tokens.AccessToken,
		RefreshToken:    refreshToken,
		TokenExpiresAt:  expiresAt,
	})
	if err != nil {
		h.App.ObsLogger.LogError(ctx, err, "Discord callback: upsert failed", "user_id", userID)
		http.Error(w, "failed to save discord account", http.StatusInternalServerError)
		return
	}

	h.App.ObsLogger.Info(ctx, "Discord account linked",
		"user_id", userID,
		"discord_username", discordUser.Username,
	)

	// 5. Redirect to frontend settings page
	http.Redirect(w, r, frontendURL, http.StatusFound)
}

// discordAPIBase returns the Discord API base URL. Overridable via DISCORD_API_BASE_URL
// for testing; defaults to the real Discord API in production.
func discordAPIBase() string {
	if base := os.Getenv("DISCORD_API_BASE_URL"); base != "" {
		return base
	}
	return "https://discord.com/api"
}

// exchangeDiscordCode exchanges an OAuth2 code for tokens.
func (h *Handler) exchangeDiscordCode(r *http.Request, code string) (*discordTokenResponse, error) {
	data := url.Values{}
	data.Set("client_id", h.App.Config.Discord.OAuthClientID)
	data.Set("client_secret", h.App.Config.Discord.OAuthClientSecret)
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", h.App.Config.Discord.OAuthRedirectURL)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost,
		discordAPIBase()+"/oauth2/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("discord token exchange error %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp discordTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	return &tokenResp, nil
}

// fetchDiscordUser retrieves the authenticated user's Discord profile.
func (h *Handler) fetchDiscordUser(r *http.Request, accessToken string) (*discordUserResponse, error) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet,
		discordAPIBase()+"/users/@me", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("discord user API error %d: %s", resp.StatusCode, string(body))
	}

	var user discordUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decode user response: %w", err)
	}

	return &user, nil
}
