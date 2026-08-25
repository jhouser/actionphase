# Game States and Transitions

Complete reference for game state machine in ActionPhase.

## Game States

```sql
-- Live constraint (migration 20260825193725). NOTE: schema.sql is STALE and
-- omits it — trust the migrations, not schema.sql.
CHECK (state IN ('setup', 'recruitment', 'character_creation',
                 'in_progress', 'paused', 'epilogue', 'completed', 'cancelled'))
```

⚠️ Four places define this set independently and none check the others at
compile time: `core.ValidGameStates`, `allowedTransitions`, the CHECK
constraint, and the frontend `GameState` union. `scripts/check-game-states.sh`
(run by `just lint`) compares all four.

Canonical values are the `GameState*` constants in
`backend/pkg/core/constants.go`:

```go
GameStateSetup             = "setup"
GameStateRecruitment       = "recruitment"
GameStateCharacterCreation = "character_creation"
GameStateInProgress        = "in_progress"
GameStatePaused            = "paused"
GameStateEpilogue          = "epilogue"
GameStateCompleted         = "completed"
GameStateCancelled         = "cancelled"
```

## State Definitions

### SETUP
- Initial creation state
- GM configures game details
- Not visible to players yet
- Can edit all settings

### RECRUITMENT
- Game listed publicly (if is_public=true)
- Players can apply to join
- GM reviews applications
- Continues until GM closes or deadline

### CHARACTER_CREATION
- Approved players create characters
- GM reviews character sheets
- Approve/reject workflow
- Continues until all characters approved

### IN_PROGRESS
- Active gameplay
- Phase cycles running
- Players posting and acting
- GM managing phases

### PAUSED
- Temporarily suspended
- No phase advancement
- Resumes with same phase

### EPILOGUE
- The game is winding down but **not finished**: a **writable public archive**
- **PUBLIC ARCHIVE**: same read access as completed — `CanUserViewGame` returns
  true for ANY authenticated user
- **STILL WRITABLE**: `ValidateGameNotCompleted` deliberately does NOT reject
  epilogue, so the GM can create phases and post epilogue / meta-discussion
  threads, and players can reply
- Anonymous mode is disabled on entry (identities disclosed)
- ⚠️ **One-way door**: `epilogue → in_progress` is NOT permitted. Entering
  epilogue discloses every private message and action submission, and players
  cannot un-see it
- Exports, stats, and full Game Logs access remain **completed-only**

### COMPLETED
- Game ended normally
- **PUBLIC ARCHIVE**: `CanUserViewGame` returns true for ANY authenticated user
- Common room content, action submissions, private conversations, and published
  results are all readable by non-participants
- Read-only state — this is the difference from epilogue

### CANCELLED
- Game terminated early
- ⚠️ **NOT public** — cancelled games follow normal permission rules; only
  participants can view them
- Marked as cancelled

## State Transition Rules

**SETUP → RECRUITMENT**
- GM ready to accept players
- Game details finalized
- API: `PUT /api/v1/games/{id}/state {"state": "recruitment"}`

**RECRUITMENT → CHARACTER_CREATION**
- GM closes recruitment
- Player slots filled
- API: `PUT /api/v1/games/{id}/state {"state": "character_creation"}`

**CHARACTER_CREATION → IN_PROGRESS**
- All players have approved characters
- GM starts game
- First phase created
- API: `PUT /api/v1/games/{id}/state {"state": "in_progress"}`

**IN_PROGRESS → PAUSED**
- GM pauses game temporarily
- API: `PUT /api/v1/games/{id}/state {"state": "paused"}`

**PAUSED → IN_PROGRESS**
- GM resumes game
- API: `PUT /api/v1/games/{id}/state {"state": "in_progress"}`

**IN_PROGRESS → EPILOGUE**
- GM opens the archive while keeping the game writable, for epilogue and
  meta-discussion threads
- Irreversible: there is no transition back to in_progress
- API: `PUT /api/v1/games/{id}/state {"state": "epilogue"}`

**EPILOGUE → COMPLETED**
- GM finishes; the game becomes read-only
- API: `PUT /api/v1/games/{id}/state {"state": "completed"}`

**IN_PROGRESS → COMPLETED**
- GM ends game normally, skipping epilogue
- API: `PUT /api/v1/games/{id}/state {"state": "completed"}`

### The two gates are separate

Read access and the write gate are **different questions** keyed on different
predicates. Epilogue is the case that proves it:

| State | Public archive (read) | Writable |
|---|---|---|
| `in_progress` | ❌ | ✅ |
| `epilogue` | ✅ | ✅ |
| `completed` | ✅ | ❌ |
| `cancelled` | ❌ | ❌ |

- Read gate: `core.IsPublicArchive` (backend), `isPublicArchive()` (frontend)
- Write gate: `core.ValidateGameNotCompleted` (backend), `isGameWritable()` (frontend)

**ANY STATE → CANCELLED**
- GM cancels game
- API: `PUT /api/v1/games/{id}/state {"state": "cancelled"}`

---

**Back to**: [SKILL.md](../SKILL.md)
