package handouts

import (
	"actionphase/pkg/core"
)

// Handler handles HTTP requests for handout-related endpoints
type Handler struct {
	App                 *core.App
	UserService         core.UserServiceInterface
	GameService         core.GameServiceInterface
	HandoutService      core.HandoutServiceInterface
	NotificationService core.NotificationServiceInterface
}

// The operations live in huma_api.go (type-first handlers plus their
// registration); authz.go holds the GM check they share, and requests.go the
// request and response bodies.
