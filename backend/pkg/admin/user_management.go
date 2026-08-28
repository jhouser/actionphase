package admin

import (
	"actionphase/pkg/core"
)

// userListResponse is the paginated shape returned by the admin user list.
// Referenced by listUsersOutput in huma_api.go.
type userListResponse struct {
	Users    []*core.User `json:"users"`
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}
