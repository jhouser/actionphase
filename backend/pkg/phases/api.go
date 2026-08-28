package phases

import (
	"actionphase/pkg/core"
)

// Handler handles phase-related HTTP requests
type Handler struct {
	App                     *core.App
	PhaseService            core.PhaseServiceInterface
	ActionSubmissionService core.ActionSubmissionServiceInterface
	GameService             core.GameServiceInterface
	NotificationService     core.NotificationServiceInterface
}

// All handler methods live in huma_api.go, registered by
// RegisterHumaGamePhases (the /games/{gameID} routes) and RegisterHumaPhases
// (the /phases/{id} routes).
//
// Supporting types:
// - requests.go:  request payload shapes, kept for the tests
// - responses.go: response types
// - converters.go: model -> response conversion
