# Test Builder Usage Guide

**Date**: 2025-10-19
**Status**: Phase 4 Complete - All Builders + TestSuite Available

---

## Quick Start

```go
import "actionphase/pkg/core"

func TestMyFeature(t *testing.T) {
    testDB := core.NewTestDatabase(t)
    defer testDB.Close()
    defer testDB.CleanupTables(t, "characters", "game_phases", "games", "sessions", "users")

    factory := core.NewTestDataFactory(testDB, t)

    // Create test data
    user := factory.NewUser().Create()
    game := factory.NewGame().WithGM(user.ID).Create()

    // Use builders!
    character := factory.NewCharacter().InGame(game).OwnedBy(user).Create()
    phase := factory.NewPhase().InGame(game).ActionPhase().Create()
}
```

---

## CharacterBuilder

### Basic Usage

```go
// Simple player character
character := factory.NewCharacter().
    InGame(game).
    OwnedBy(user).
    Create()

// With custom name
character := factory.NewCharacter().
    InGame(game).
    OwnedBy(user).
    WithName("Aragorn").
    Create()

// With approved status
character := factory.NewCharacter().
    InGame(game).
    OwnedBy(user).
    Approved().
    Create()
```

### Character Types

```go
// Player Character (default)
player := factory.NewCharacter().
    InGame(game).
    OwnedBy(user).
    PlayerCharacter().  // Optional - this is the default
    Create()

// GM-Controlled NPC
villain := factory.NewCharacter().
    InGame(game).
    NPCGMControlled().  // Sets type AND userID = nil
    WithName("Dark Lord").
    Create()

// Audience NPC (player-controllable)
guide := factory.NewCharacter().
    InGame(game).
    NPCAudience().
    WithName("Wise Elder").
    Create()
```

### Status Options

```go
// Pending (default)
pending := factory.NewCharacter().InGame(game).OwnedBy(user).Create()

// Approved
approved := factory.NewCharacter().InGame(game).OwnedBy(user).Approved().Create()

// Rejected
rejected := factory.NewCharacter().InGame(game).OwnedBy(user).Rejected().Create()

// Custom status
custom := factory.NewCharacter().
    InGame(game).
    OwnedBy(user).
    WithStatus("custom_status").
    Create()
```

### Before/After Comparison

**Before** (8 lines):
```go
character, err := characterService.CreateCharacter(context.Background(), CreateCharacterRequest{
    GameID:        fixtures.TestGame.ID,
    UserID:        core.Int32Ptr(int32(fixtures.TestUser.ID)),
    Name:          "Test Character",
    CharacterType: "player_character",
})
core.AssertNoError(t, err, "Failed to create test character")
```

**After** (5 lines):
```go
character := factory.NewCharacter().
    InGame(fixtures.TestGame).
    OwnedBy(fixtures.TestUser).
    WithName("Test Character").
    Create()
```

---

## PhaseBuilder

### Basic Usage

```go
// Simple common room phase
phase := factory.NewPhase().
    InGame(game).
    Create()

// With title and description
phase := factory.NewPhase().
    InGame(game).
    WithTitle("Opening Scene").
    WithDescription("The adventure begins...").
    Create()
```

### Phase Types

```go
// Common Room (default)
commonRoom := factory.NewPhase().
    InGame(game).
    CommonRoom().  // Optional - this is the default
    Create()

// Action Phase
action := factory.NewPhase().
    InGame(game).
    ActionPhase().
    Create()
```

### Time Helpers

```go
// Set deadline in 48 hours from now
phase := factory.NewPhase().
    InGame(game).
    ActionPhase().
    WithDeadlineIn(48 * time.Hour).
    Create()

// Set start and end time range
phase := factory.NewPhase().
    InGame(game).
    WithTimeRange(72 * time.Hour).  // Starts now, ends in 72 hours
    Create()

// Set specific times
startTime := time.Now().Add(24 * time.Hour)
endTime := time.Now().Add(96 * time.Hour)

phase := factory.NewPhase().
    InGame(game).
    WithStartTime(startTime).
    WithEndTime(endTime).
    Create()
```

### Phase Numbering

```go
// Auto-increment (default)
phase1 := factory.NewPhase().InGame(game).Create()  // Will be phase number 1
phase2 := factory.NewPhase().InGame(game).Create()  // Will be phase number 2
phase3 := factory.NewPhase().InGame(game).Create()  // Will be phase number 3

// Manual phase number
phase := factory.NewPhase().
    InGame(game).
    WithPhaseNumber(10).
    Create()
```

### Active Phases

```go
// Create and activate in one step
activePhase := factory.NewPhase().
    InGame(game).
    ActionPhase().
    Active().
    Create()

// Inactive (default)
inactivePhase := factory.NewPhase().
    InGame(game).
    Inactive().  // Optional - this is the default
    Create()
```

### Before/After Comparison

**Before** (10 lines):
```go
req := core.CreatePhaseRequest{
    GameID:      game.ID,
    PhaseType:   "action",
    PhaseNumber: 1,
    Title:       "Action Phase",
    Description: "Submit your actions",
    StartTime:   core.TimePtr(time.Now()),
    EndTime:     core.TimePtr(time.Now().Add(48 * time.Hour)),
}
phase, err := phaseService.CreatePhase(context.Background(), req)
require.NoError(t, err)
```

**After** (6 lines):
```go
phase := factory.NewPhase().
    InGame(game).
    ActionPhase().
    WithTitle("Action Phase").
    WithDescription("Submit your actions").
    WithTimeRange(48 * time.Hour).
    Create()
```

---

## Common Patterns

### Complete Game Setup

```go
// Create game with GM, players, and characters
gm := factory.NewUser().WithUsername("gamemaster").Create()
game := factory.NewGame().WithGM(gm.ID).WithTitle("Epic Quest").Create()

player1 := factory.NewUser().WithUsername("player1").Create()
player2 := factory.NewUser().WithUsername("player2").Create()

char1 := factory.NewCharacter().
    InGame(game).
    OwnedBy(player1).
    WithName("Warrior").
    Approved().
    Create()

char2 := factory.NewCharacter().
    InGame(game).
    OwnedBy(player2).
    WithName("Mage").
    Approved().
    Create()

villain := factory.NewCharacter().
    InGame(game).
    NPCGMControlled().
    WithName("Dark Lord").
    Create()
```

### Phase Sequence

```go
// Create a series of phases for a game
intro := factory.NewPhase().
    InGame(game).
    CommonRoom().
    WithTitle("Introduction").
    Active().
    Create()

investigation := factory.NewPhase().
    InGame(game).
    ActionPhase().
    WithTitle("Investigation").
    WithDeadlineIn(48 * time.Hour).
    Create()

confrontation := factory.NewPhase().
    InGame(game).
    ActionPhase().
    WithTitle("Confrontation").
    Create()
```

---

## All Builder Methods

### CharacterBuilder Methods

| Method | Description | Example |
|--------|-------------|---------|
| `InGame(game)` | Set game by Game object | `.InGame(game)` |
| `ForGame(gameID)` | Set game by ID | `.ForGame(123)` |
| `OwnedBy(user)` | Set owner by User object | `.OwnedBy(user)` |
| `WithUserID(userID)` | Set owner by ID | `.WithUserID(456)` |
| `GMControlled()` | Make NPC with no owner | `.GMControlled()` |
| `WithName(name)` | Set character name | `.WithName("Gandalf")` |
| `WithCharacterType(type)` | Set raw type string | `.WithCharacterType("npc_gm")` |
| `PlayerCharacter()` | Set as player character | `.PlayerCharacter()` |
| `NPCGMControlled()` | Set as GM NPC | `.NPCGMControlled()` |
| `NPCAudience()` | Set as audience NPC | `.NPCAudience()` |
| `WithStatus(status)` | Set raw status string | `.WithStatus("custom")` |
| `Pending()` | Set status to pending | `.Pending()` |
| `Approved()` | Set status to approved | `.Approved()` |
| `Rejected()` | Set status to rejected | `.Rejected()` |
| `Create()` | Persist to database | `.Create()` |

### PhaseBuilder Methods

| Method | Description | Example |
|--------|-------------|---------|
| `InGame(game)` | Set game by Game object | `.InGame(game)` |
| `ForGame(gameID)` | Set game by ID | `.ForGame(123)` |
| `WithPhaseType(type)` | Set raw type string | `.WithPhaseType("action")` |
| `CommonRoom()` | Set as common room | `.CommonRoom()` |
| `ActionPhase()` | Set as action phase | `.ActionPhase()` |
| `WithPhaseNumber(num)` | Set phase number | `.WithPhaseNumber(5)` |
| `WithTitle(title)` | Set phase title | `.WithTitle("Battle")` |
| `WithDescription(desc)` | Set description | `.WithDescription("...")` |
| `WithStartTime(time)` | Set start time | `.WithStartTime(t)` |
| `WithEndTime(time)` | Set end time | `.WithEndTime(t)` |
| `WithDeadline(time)` | Set deadline | `.WithDeadline(t)` |
| `WithDeadlineIn(dur)` | Set deadline from now | `.WithDeadlineIn(48*time.Hour)` |
| `WithTimeRange(dur)` | Set start=now, end=now+dur | `.WithTimeRange(72*time.Hour)` |
| `Active()` | Activate phase | `.Active()` |
| `Inactive()` | Keep inactive | `.Inactive()` |
| `Create()` | Persist to database | `.Create()` |

---

## TestSuite (Phase 4)

### Basic Usage

The TestSuite reduces setup/teardown boilerplate and provides easy access to services and factories.

**Location**: `actionphase/pkg/db/services` (package `db`)

```go
import db "actionphase/pkg/db/services"

func TestMyFeature(t *testing.T) {
    suite := db.NewTestSuite(t).
        WithCleanup("characters").  // Use cleanup preset
        Setup()
    defer suite.Cleanup()

    // Access services directly
    characterService := suite.CharacterService()
    gameService := suite.GameService()

    // Access factory
    user := suite.Factory().NewUser().Create()
    game := suite.Factory().NewGame().WithGM(user.ID).Create()
}
```

### Cleanup Presets

Common table cleanup combinations are available in `core.CleanupPresets`:

| Preset | Tables Cleaned |
|--------|----------------|
| `"games"` | games, sessions, users |
| `"characters"` | character_data, npc_assignments, characters, games, sessions, users |
| `"phases"` | game_phases, games, sessions, users |
| `"messages"` | messages, characters, games, sessions, users |
| `"actions"` | action_results, action_submissions, game_phases, characters, games, sessions, users |
| `"participants"` | game_participants, game_applications, games, sessions, users |
| `"all"` | All tables in dependency order |

### Custom Table Cleanup

```go
suite := db.NewTestSuite(t).
    WithTables("my_table", "other_table", "users").
    Setup()
```

### With Fixtures

```go
suite := db.NewTestSuite(t).
    WithCleanup("games").
    WithFixtures().  // Creates TestUser and TestGame
    Setup()
defer suite.Cleanup()

user := suite.Fixtures().TestUser
game := suite.Fixtures().TestGame
```

### Service Factory

The TestSuite provides convenient access to all services:

```go
suite := db.NewTestSuite(t).WithCleanup("all").Setup()
defer suite.Cleanup()

// All services available
userService := suite.UserService()
sessionService := suite.SessionService()
gameService := suite.GameService()
applicationService := suite.GameApplicationService()
characterService := suite.CharacterService()
```

### Helper Methods

#### TransitionGameTo

Transition a game to a new state with automatic error handling:

```go
game := suite.Factory().NewGame().WithGM(user.ID).Create()

// Simple state transition
game = suite.TransitionGameTo(game, "recruitment")
assert.Equal(t, "recruitment", game.State.String)
```

#### AddParticipant

Add a participant to a game:

```go
player := suite.Factory().NewUser().Create()
game := suite.Factory().NewGame().WithGM(gm.ID).Create()

participant := suite.AddParticipant(game, player, "player")
assert.Equal(t, "player", participant.Role)
```

### Before/After Comparison

**Before** (manual setup/teardown):
```go
func TestCharacterCreation(t *testing.T) {
    testDB := core.NewTestDatabase(t)
    defer testDB.Close()
    defer testDB.CleanupTables(t, "character_data", "npc_assignments", "characters", "games", "sessions", "users")

    factory := core.NewTestDataFactory(testDB, t)
    characterService := &CharacterService{DB: testDB.Pool}

    user := factory.NewUser().Create()
    game := factory.NewGame().WithGM(user.ID).Create()

    // Test code...
}
```

**After** (with TestSuite):
```go
func TestCharacterCreation(t *testing.T) {
    suite := db.NewTestSuite(t).
        WithCleanup("characters").
        Setup()
    defer suite.Cleanup()

    user := suite.Factory().NewUser().Create()
    game := suite.Factory().NewGame().WithGM(user.ID).Create()

    // Test code...
}
```

**Savings**: 5 lines → 2 lines of boilerplate (60% reduction)

---

## Tips

### When to Use Each Builder

**CharacterBuilder**:
- Creating test characters for game scenarios
- Setting up character permissions tests
- Testing character approval workflows
- Creating NPCs for action submission tests

**PhaseBuilder**:
- Creating phases for phase transition tests
- Setting up action submission scenarios
- Testing phase activation logic
- Creating time-based test scenarios

### Chaining Best Practices

1. **Start with game reference**: Always start with `.InGame(game)` or `.ForGame(gameID)`
2. **Set required fields next**: For characters, set owner; for phases, set type if not default
3. **Customize last**: Add names, descriptions, statuses last
4. **End with Create()**: Always end the chain with `.Create()`

**Good**:
```go
character := factory.NewCharacter().
    InGame(game).           // 1. Game first
    OwnedBy(user).          // 2. Required field
    WithName("Aragorn").    // 3. Customization
    Approved().             // 4. More customization
    Create()                // 5. Create last
```

**Avoid**:
```go
// Don't put Create() in the middle
character := factory.NewCharacter().Create().InGame(game)  // ❌

// Don't forget InGame()
character := factory.NewCharacter().OwnedBy(user).Create()  // ❌
```

### Auto-Generated Defaults

Both builders auto-generate unique sequential values:

```go
// Each call gets a unique name
char1 := factory.NewCharacter().InGame(game).OwnedBy(user).Create()
// Name: "Test Character 1"

char2 := factory.NewCharacter().InGame(game).OwnedBy(user).Create()
// Name: "Test Character 2"

phase1 := factory.NewPhase().InGame(game).Create()
// Title: "Test Phase 1"

phase2 := factory.NewPhase().InGame(game).Create()
// Title: "Test Phase 2"
```

---

## Future Builders (Coming Soon)

### Phase 3 - ActionSubmissionBuilder
```go
action := factory.NewActionSubmission().
    InPhase(phase).
    ByUser(user).
    WithContent("I search for clues").
    Create()
```

### Phase 3 - MessageBuilder
```go
post := factory.NewPost().
    InGame(game).
    ByCharacter(character).
    WithContent("The dragon stirs...").
    Create()
```

---

## Need Help?

- **Full examples**: See `backend/pkg/core/test_builders_test.go`
- **Builder code**: See `backend/pkg/core/test_factories.go`
- **Test suite builder**: See `backend/pkg/db/services/test_suite.go`

---

**Last Updated**: 2025-10-19
**Builders Available**: UserBuilder, GameBuilder, SessionBuilder, GameParticipantBuilder, CharacterBuilder, PhaseBuilder, ActionSubmissionBuilder, MessageBuilder
**Helpers Available**: TestSuite, ServiceFactory, CleanupPresets, TransitionGameTo, AddParticipant
**Status**: Production Ready - Phase 4 Complete
