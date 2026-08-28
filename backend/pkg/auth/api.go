package auth

import (
	"actionphase/pkg/core"
	"fmt"
	"net/http"
)

type Request struct {
	*core.User
	HCaptchaToken string `json:"hcaptcha_token"`
	HoneypotValue string `json:"honeypot_value"`
	Fingerprint   string `json:"fingerprint"`
}

func (r *Request) Bind(req *http.Request) error {
	if r.User == nil {
		return fmt.Errorf("missing required User fields")
	}
	if len(r.Fingerprint) > 512 {
		return fmt.Errorf("fingerprint exceeds maximum length")
	}
	return nil
}

type Handler struct {
	App                    *core.App
	UserService            core.UserServiceInterface
	SessionService         core.SessionServiceInterface
	UserPreferencesService core.UserPreferencesServiceInterface
	IPBanService           core.IPBanServiceInterface
	FingerprintBanService  core.FingerprintBanServiceInterface
	DiscordService         core.DiscordAccountServiceInterface
}

type Response struct {
	*core.User
	Token string
}

func (rd *Response) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

// PreferencesRequest represents a request to update user preferences
type PreferencesRequest struct {
	Preferences *core.PreferencesData `json:"preferences"`
}

func (r *PreferencesRequest) Bind(req *http.Request) error {
	if r.Preferences == nil {
		return fmt.Errorf("missing required preferences field")
	}
	return nil
}

// PreferencesResponse represents the preferences response
type PreferencesResponse struct {
	Preferences *core.PreferencesData `json:"preferences"`
}

func (rd *PreferencesResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

// UserSearchResult represents a single user in search results
type UserSearchResult struct {
	ID        int32  `json:"id"`
	Username  string `json:"username"`
	CreatedAt string `json:"created_at"`
}

// SearchUsersResponse represents the search results
type SearchUsersResponse struct {
	Users []UserSearchResult `json:"users"`
}

func (rd *SearchUsersResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}
