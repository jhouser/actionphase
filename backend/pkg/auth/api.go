package auth

// The request and response types the chi handlers bound and rendered are gone:
// huma owns request binding and response marshalling now, with its own input
// and output structs in huma_api.go. What remains is the Handler itself and the
// payload shapes those huma types still reference.

import (
	"actionphase/pkg/core"
)

type Handler struct {
	App                    *core.App
	UserService            core.UserServiceInterface
	SessionService         core.SessionServiceInterface
	UserPreferencesService core.UserPreferencesServiceInterface
	IPBanService           core.IPBanServiceInterface
	FingerprintBanService  core.FingerprintBanServiceInterface
	DiscordService         core.DiscordAccountServiceInterface
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
