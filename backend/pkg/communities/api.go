package communities

// Member- and moderator-facing community endpoints, mounted at
// /api/v1/communities.
//
// Site-admin endpoints (creating a community, assigning its owner, listing
// every community including inactive ones) live in pkg/admin instead, behind
// RequireAdmin. The split follows the permission model: creation is a site-admin
// act, while everything here is gated per-community on the caller's standing.

import (
	"actionphase/pkg/core"
)

// Handler carries the dependencies the community endpoints need.
//
// CommunityService is the interface rather than the concrete type so handler
// tests can substitute a fake, matching the other huma packages.
type Handler struct {
	App              *core.App
	UserService      core.UserServiceInterface
	CommunityService core.CommunityServiceInterface
}
