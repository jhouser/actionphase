package docs

// Document-level OpenAPI metadata.
//
// Huma generates paths and schemas from Go types, but nothing above them: the
// info block, server list, security scheme, tag descriptions and the one
// operation that is not a huma handler all have to come from somewhere. They
// used to live in a hand-written openapi.yaml that the generated half was
// merged over; with every package converted, that file held only this metadata
// and was deleted (see .claude/planning/huma-migration.md, "Cutover").
//
// Keeping it in Go rather than a YAML stub means the served document has one
// source and `just gen-openapi` can render it without reading anything else.

// Two routes are served but deliberately undocumented, and no check will
// notice — the spec is generated from huma operations, so a chi route is
// invisible to it:
//
//   - GET /api/v1/auth/discord/callback — an OAuth2 redirect target. Answers a
//     302 with plain-text errors, so it has no JSON contract to describe, and
//     only Discord ever calls it.
//   - GET /uploads/* — a static file server for locally-stored avatars and
//     banners, present only when Storage.Backend is "local". In production
//     these are served from S3 and the route does not exist at all.
//
// Both were tracked in scripts/api-docs-baseline.txt until that ledger was
// retired; neither is worth a hand-written operation.

// specBase returns the document that generated paths and schemas are merged
// into. Every call returns a fresh map, since the merge mutates it.
func specBase() map[string]any {
	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title": "ActionPhase API",
			"description": "ActionPhase is a specialized platform for running play-by-post RPG games " +
				"that alternate between two distinct phases:\n" +
				"- **Common Room Phase**: Asynchronous, Reddit-style threaded discussions where players interact in-character\n" +
				"- **Action Phase**: Players submit private moves to the Game Master, who processes and publishes results\n\n" +
				"This API provides access to game management, character creation, and phase management functionality.\n",
			"version": "1.0.0",
			"contact": map[string]any{
				"name": "ActionPhase Development Team",
				"url":  "https://github.com/actionphase/actionphase",
			},
			"license": map[string]any{
				"name": "MIT",
				"url":  "https://opensource.org/licenses/MIT",
			},
		},
		"servers": []any{
			map[string]any{"url": "http://localhost:3000/api/v1", "description": "Local development server"},
			map[string]any{"url": "https://api.action-phase.com/api/v1", "description": "Production server"},
		},
		"security": []any{
			map[string]any{"BearerAuth": []any{}},
		},
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"BearerAuth": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
			},
			"schemas": map[string]any{},
		},
		"paths": map[string]any{
			"/ping": pingOperation(),
		},
		"tags": specTags(),
	}
}

// pingOperation documents the one route that is deliberately not a huma
// handler.
//
// /ping writes the bare string "ponger" rather than JSON, so converting it
// would change the response body for no documentation gain; it stays on chi
// (see the migration plan). Because it is registered on the root router rather
// than under /api/v1, it needs the per-operation `servers` override below —
// without it the documented URL resolves to /api/v1/ping, which 404s.
func pingOperation() map[string]any {
	return map[string]any{
		"get": map[string]any{
			"servers": []any{
				map[string]any{"url": "http://localhost:3000", "description": "Local development server (root, not /api/v1)"},
				map[string]any{"url": "https://api.action-phase.com", "description": "Production server (root, not /api/v1)"},
			},
			"tags":    []any{"Health"},
			"summary": "Health check endpoint",
			"description": "Simple health check to verify the API is running. Served at the root\n" +
				"(`/ping`), not under the `/api/v1` base used by every other operation.\n",
			"security": []any{},
			"responses": map[string]any{
				"200": map[string]any{
					"description": "API is healthy",
					"content": map[string]any{
						"text/plain": map[string]any{
							"schema": map[string]any{"type": "string", "example": "ponger"},
						},
					},
				},
				"503": map[string]any{"description": "API is unavailable"},
			},
		},
	}
}

// specTags describes every tag the registered operations use.
//
// The list is asserted complete by TestSpecTagsCoverEveryOperation: a package
// that introduces a new tag fails that test rather than shipping an operation
// grouped under an undescribed heading. The hand-written spec had drifted ten
// tags behind by the time it was retired, which is what motivated the check.
func specTags() []any {
	type tag struct{ name, desc string }
	tags := []tag{
		{"Health", "API health and status endpoints"},
		{"Authentication", "User authentication and session management"},
		{"Admin", "Administrative user, ban and session management"},
		{"Users", "User profiles, avatars and preferences"},
		{"Dashboard", "Aggregated dashboard data for user overview"},
		{"Communities", "Community profiles and moderator rosters"},
		{"Notifications", "User notification system for game events and updates"},
		{"Games", "Game creation, management, and participation"},
		{"Game Applications", "Application system for joining games"},
		{"Game Logs", "Game logs to view the game timeline"},
		{"Participants", "Player, co-GM and audience membership within a game"},
		{"Audience", "Spectator access to a game's private messages and submissions"},
		{"Characters", "Character creation, management, and character sheet data"},
		{"Phases", "Game phase management (action and common room phases)"},
		{"Actions", "Player action submission during action phases"},
		{"Action Results", "GM responses to player actions"},
		{"Draft Character Updates", "Character sheet changes staged against an unpublished action result"},
		{"Common Room", "Threaded discussion posts and comments in the common room"},
		{"Conversations", "Private in-character messaging between players"},
		{"Handouts", "Game reference documents and lore provided by Game Masters"},
		{"Polls", "In-game polls and voting"},
		{"Deadlines", "Phase deadlines and upcoming-deadline listings"},
		{"Loot Tables", "GM-authored item tables and random loot rolls"},
		{"Exports", "Game archive generation and download"},
	}

	out := make([]any, 0, len(tags))
	for _, t := range tags {
		out = append(out, map[string]any{"name": t.name, "description": t.desc})
	}
	return out
}
