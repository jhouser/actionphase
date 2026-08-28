package admin

import (
	"actionphase/pkg/core"
)

// Handler holds the services the admin endpoints depend on. The endpoints
// themselves are type-first (huma) and live in huma_api.go.
type Handler struct {
	App                   *core.App
	UserService           core.UserServiceInterface
	SessionService        core.SessionServiceInterface
	IPBanService          core.IPBanServiceInterface
	FingerprintBanService core.FingerprintBanServiceInterface
	MessageService        core.MessageServiceInterface
}
