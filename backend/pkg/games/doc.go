/*
Package games provides HTTP handlers and business logic for game management in ActionPhase.

This package handles the game lifecycle including creation, state management,
player recruitment, and game participation through RESTful API endpoints.

Game Lifecycle States:

	setup         - Initial state when game is created
	recruitment   - Game is accepting new players
	character_creation - Players are creating their characters
	in_progress   - Game is actively being played
	paused        - Game is temporarily suspended
	completed     - Game has finished successfully
	cancelled     - Game was cancelled before completion

Key Features:

Game Management:
  - Create new games with customizable settings
  - Update game details (title, description, settings)
  - Manage game state transitions
  - Delete games (with proper authorization)

Player Recruitment:
  - Join games as player or observer
  - Leave games (except Game Master)
  - Check game capacity and availability
  - Role-based participation (GM, player, observer)

Game Discovery:
  - List all public games
  - Filter games by state (recruiting, in-progress, etc.)
  - Search games by user participation
  - Get detailed game information with participant counts

Authorization:
  - Game Masters have full control over their games
  - Players can join/leave based on game rules
  - Role-based access control for operations
  - Public/private game visibility

HTTP API Endpoints:

	POST   /api/games          - Create new game (authenticated)
	GET    /api/games          - List all public games
	GET    /api/games/user/:id - List games for specific user
	GET    /api/games/:id      - Get game details
	PUT    /api/games/:id      - Update game (GM only)
	DELETE /api/games/:id      - Delete game (GM only)
	POST   /api/games/:id/join - Join game as player
	POST   /api/games/:id/leave - Leave game
	PUT    /api/games/:id/state - Update game state (GM only)

Business Rules:
  - Only Game Masters can modify their games
  - Players cannot join full games
  - Games must be in 'recruitment' state to accept new players
  - Game Masters cannot leave their own games
  - State transitions must follow valid progression
  - Public games are visible to all users
  - Private games are only visible to participants

Routes:

Game routes are registered in backend/pkg/http/root.go, mounted at /api/v1/games.
That file is the only authoritative list — this comment deliberately does not
reproduce it. An inlined copy here drifted for months (it showed a /api/games
mount and POST /join and /leave endpoints that never existed), and a wrong route
table next to the code is worse than none.

For the documented HTTP contract see backend/pkg/docs/openapi.yaml;
`just check-api-docs` verifies the two agree.
*/
package games
