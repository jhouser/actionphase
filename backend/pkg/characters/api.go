package characters

import (
	"actionphase/pkg/core"
)

// Handler handles character-related HTTP requests
type Handler struct {
	App                 *core.App
	UserService         core.UserServiceInterface
	CharacterService    core.CharacterServiceInterface
	GameService         core.GameServiceInterface
	NotificationService core.NotificationServiceInterface
}

// The operations live in huma_api.go (type-first handlers plus their
// registration); authz_stats.go holds the private-stats visibility rules they
// share, and requests.go / responses.go the request and response bodies.
