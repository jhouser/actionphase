package dashboard

import (
	"actionphase/pkg/core"
)

// Handler handles HTTP requests for dashboard endpoints.
//
// The operations themselves are type-first huma handlers in huma_api.go.
type Handler struct {
	App              *core.App
	UserService      core.UserServiceInterface
	DashboardService core.DashboardServiceInterface
}
