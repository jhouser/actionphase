package auth

import (
	"net/http"
)

// SessionResponse represents a session in the API response
type SessionResponse struct {
	ID        int32  `json:"id"`
	CreatedAt string `json:"created_at"`
	Expires   string `json:"expires"`
	IsCurrent bool   `json:"is_current"`
}

// SessionsListResponse represents the list of sessions
type SessionsListResponse struct {
	Sessions []SessionResponse `json:"sessions"`
}

func (rd *SessionsListResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}
