# Test Data Context - Read Before Working with Test Data

**IMPORTANT: Read this file before working with test data and fixtures.**

**Last Updated**: May 2026

## Test Fixture System

ActionPhase uses SQL-based test fixtures for comprehensive test coverage across all game states and phases.

### Fixture Location
**Location**: `/backend/pkg/db/test_fixtures/`

The fixture system is split into four categories:

```
test_fixtures/
├── common/               # Shared base data (users, reset)
│   ├── 00_reset.sql      # Cleans all test data
│   └── 01_users.sql      # Creates test users (TestGM, TestPlayer1-5, TestAudience)
├── demo/                 # Demo/dev data (games, characters, actions, messages)
│   ├── 02_games_recruiting.sql
│   ├── 03_games_running.sql
│   ├── 04_characters.sql
│   ├── 05_actions.sql
│   ├── 06_results.sql
│   ├── 08_private_messages.sql
│   ├── 09_demo_content.sql
│   └── 10_deeply_nested_comments.sql
├── e2e/                  # Isolated fixtures per E2E test file (see below)
│   ├── 07_common_room.sql        # Games #164-169 (common room tests)
│   ├── 08_e2e_dedicated_games.sql # State-modifying test games
│   ├── 09_action_results.sql
│   ├── 10_lifecycle_game.sql
│   ├── 11_character_sheets.sql
│   ├── 12_game_applications.sql
│   ├── 13_game_lifecycle.sql
│   ├── 14_character_workflows.sql
│   ├── 15_deep_thread.sql
│   ├── 16_deep_linking.sql
│   ├── 17_private_message_deletion_w{0-5}.sql  # Worker-specific fixtures
│   ├── 18_co_gm_*.sql            # Co-GM test fixtures (worker-specific)
│   ├── 19_*.sql                  # Player multi-character + polls
│   ├── 20_cogm_npc_messaging.sql
│   ├── 21_audience_private_messages*.sql
│   ├── 22_manual_read_tracking.sql
│   ├── 23_private_message_editing_w{0-5}.sql
│   ├── 24_unread_tracking.sql
│   └── 25_notification_*.sql
├── perf/                 # Performance test fixtures
├── apply_all.sh          # Apply common + demo fixtures
├── apply_e2e.sh          # Apply common + e2e fixtures
└── apply_common.sh       # Apply only common fixtures
```

**Apply commands**:
```bash
just test-fixtures          # Apply demo fixtures (for manual testing)
just load-e2e               # Apply e2e fixtures (done automatically by just e2e)
```

## Quick Commands

### Apply Demo Fixtures (for manual testing / development)
```bash
just test-fixtures
```

### Apply E2E Fixtures (done automatically by `just e2e`)
```bash
just load-e2e
```

### Reset Test Data
```bash
# Reapply from scratch
just test-data reload
```

## Test Users

**All passwords**: `testpassword123`

**IMPORTANT**: Usernames are **case-sensitive** and use **PascalCase** (TestGM, TestPlayer1, etc.)

### User Types
1. **Game Master**: `TestGM` / `test_gm@example.com`
   - Creates and manages games
   - Controls NPCs

2. **Players** (5 total):
   - `TestPlayer1` / `test_player1@example.com`
   - `TestPlayer2` / `test_player2@example.com`
   - `TestPlayer3` / `test_player3@example.com`
   - `TestPlayer4` / `test_player4@example.com`
   - `TestPlayer5` / `test_player5@example.com`

3. **Audience**: `TestAudience` / `test_audience@example.com`
   - Observes games without direct participation

## Test Game Coverage

### 10 Games Covering All States

| # | Name | Status | Phase Status | Purpose |
|---|------|--------|--------------|---------|
| 1 | Shadows Over Innsmouth | Running | Active Common Room | Test common room phase |
| 2 | The Heist at Goldstone Bank | Running | Active Action (with submissions) | Test action submissions |
| 3 | Starfall Station | Running | Active Results (with published results) | Test results display |
| 4 | Court of Shadows | Running | Previous CR + Active Action | Test phase transitions |
| 5 | The Dragon of Mount Krag | Running | 6 previous + Active CR | Test complex history |
| 6 | Chronicles of Westmarch | Running | 11 previous + Active Results | Test pagination |
| 7 | The Mystery of Blackwood Manor | Recruiting | No phases | Test recruitment |
| 8 | On Hold: The Frozen North | Paused | 4 previous phases | Test paused state |
| 9 | COMPLETED: Tales of the Arcane | Completed | 9 completed phases | Test completed |
| 10 | Secret Campaign | Recruiting | No phases (private) | Test private games |

### Game States Covered
- **Recruiting** - Games #7, #10
- **Running** - Games #1-6
- **Paused** - Game #8
- **Completed** - Game #9
- **Cancelled** - (add if needed)

### Phase Types Covered
- **Common Room** - Games #1, #5 (active)
- **Action** - Games #2, #4 (active, with submissions)
- **Results** - Games #3, #6 (active, with published results)

### Phase History Patterns
- **No history** - Games #1, #7, #10
- **Single previous phase** - Games #2, #4
- **Mixed history (3-6 phases)** - Games #3, #5, #8
- **Long history (10+ phases)** - Games #6, #9

## Test Characters

### Character Coverage
- **30+ characters total**
- **1 player character per player per game**
- **Multiple GM NPCs per game**
- **Character data examples** (public and private fields)

### Character Types
- `player_character` - Player-controlled characters
- `npc_gm` - GM-controlled NPCs
- `npc_player` - Player-controlled NPCs (future feature)

### Example Characters

**Game #2 (Heist)**:
- Shade (Whisper) - Player 1
- Rook (Hound) - Player 2
- Vex (Leech) - Player 3
- Silk (Spider) - Player 4
- Inspector Dalton - GM NPC

**Game #1 (Innsmouth)**:
- Detective Marcus Kane - Player 1
- Dr. Sarah Chen - Player 2
- Father O'Brien - Player 3
- Captain Obed Marsh - GM NPC

## Test Actions and Results

### Action Submissions (Game #2)
- **3 submitted actions** (finalized)
- **1 draft action** (work in progress)
- Various submission times and content lengths

### Action Results (Game #3)
- **3 published results** (visible to players)
- **1 draft result** (GM hasn't published yet)
- Demonstrates storytelling and cliffhangers

### Staged Result Chains (`demo/06b_staged_results.sql`)

A staged chain is one result split into parts revealed on a timer.

- **Game #3** — a *pending* chain (part 1 released, part 2 due minutes after
  load, part 3 with no knowable unlock time) and a *fully released* chain
- **Game #9** (completed) — an archive chain: released head and tail, plus one
  part still pending, which is what the export gate is verified against

#### 🔴 Two rules when writing `action_results` fixtures

**1. A published row MUST set `released_at`.** `GetUserResults` blanks the
content of any row where `released_at IS NULL`, so a published fixture row
without it reaches the player as an **empty string** — the result renders blank
with no error anywhere. `CreateActionResult` sets `released_at` and `sent_at`
together; fixtures must too. (Every published fixture result was silently
invisible this way until Aug 2026.)

The *only* legitimate published-with-NULL-`released_at` rows are staged parts
awaiting their timer; those also carry `parent_result_id` and
`reveal_delay_minutes`.

**2. A pending staged part cannot be backdated.** The release worker owns a
chain's clock independently of game state — `ReleaseDueStagedParts` deliberately
does not join `games` or `game_phases` — so a part whose parent released in the
past is overdue and fires on the next tick, even in a completed game. To keep a
part genuinely pending, anchor its **parent's** `released_at` to `NOW()` and give
the part a long `reveal_delay_minutes`.

The head and follow-up parts come from different queries with different column
lists (`CreateActionResult` vs `CreateStagedResultPart`); fixtures must mirror
both. See the header comments in `demo/06b_staged_results.sql`.

## Edge Cases Covered

### ✅ Complete Coverage

**Phase States**:
- Active phases of all three types
- Previous phases (completed)
- No phases (recruiting games)
- Mixed phase history
- Long phase history (10+ phases)

**Deadline Scenarios**:
- Deadlines in near future (< 1 day)
- Deadlines in far future (> 1 day)
- No deadlines (common room, results phases)

**Character Scenarios**:
- Standard case (1 PC per player per game)
- GM-only NPCs
- Character data (public and private)

**Action Submissions**:
- Submitted (final) actions
- Draft actions
- Various submission times

**Game States**:
- All status types covered
- Public and private visibility
- Various creation dates

**Participation**:
- Active participants
- Games with 2-6 players
- Audience members (future)

## Using Test Data in Tests

### Backend Integration Tests

```go
func TestGameService_GetGame(t *testing.T) {
    // Setup: Apply test fixtures first
    db := setupTestDB(t)

    // Game #2 has active action phase with submissions
    game, err := service.GetGame(ctx, 2)

    assert.NoError(t, err)
    assert.Equal(t, "The Heist at Goldstone Bank", game.Title)
    assert.Equal(t, "in_progress", game.State)
}
```

### Frontend Component Tests

```typescript
test('displays recruiting games', async () => {
  // Mock API response using test data structure
  server.use(
    rest.get('/api/v1/games/recruiting', (req, res, ctx) => {
      return res(ctx.json({
        data: [
          { id: 7, title: 'The Mystery of Blackwood Manor', state: 'recruitment' },
          { id: 10, title: 'Secret Campaign', state: 'recruitment', is_public: false }
        ]
      }));
    })
  );

  render(<GamesList showRecruitingOnly={true} />);
  expect(await screen.findByText('The Mystery of Blackwood Manor')).toBeInTheDocument();
});
```

## Common Testing Scenarios

### Testing Phase Transitions
Use **Game #5** (Dragon of Mount Krag):
- Has 6 previous phases (common_room, action, results pattern)
- Active common room phase
- Good for testing phase history and transitions

### Testing Action Submissions
Use **Game #2** (Heist at Goldstone Bank):
- Active action phase
- 3 submitted actions + 1 draft
- Multiple characters with actions

### Testing Results Display
Use **Game #3** (Starfall Station):
- Active results phase
- 3 published results
- 1 unpublished draft result

### Testing Pagination
Use **Game #6** (Chronicles of Westmarch):
- 12 total phases (11 previous + 1 active)
- Tests pagination for phase lists

### Testing Recruitment
Use **Game #7** (Blackwood Manor):
- Recruiting state
- No phases yet
- Public visibility

## E2E Test Fixtures - Test Isolation Strategy

**CRITICAL**: E2E fixtures are designed for **test isolation and parallel execution**.

### Key Principle: Worker-Based Parallelism

E2E tests run with up to 6 workers in parallel. Many fixtures are **worker-specific** (e.g., `17_private_message_deletion_w0.sql` through `w5.sql`), so each parallel worker gets its own isolated copy of test data. The `getFixtureGameId()` helper automatically selects the right worker's data.

### Using Fixtures in Tests

```typescript
import { getFixtureGameId } from '../fixtures/game-helpers';

// In your test:
const gameId = await getFixtureGameId(page, 'COMMON_ROOM_POSTS');
```

Use the `FIXTURE_GAMES` constants in `frontend/e2e/fixtures/game-helpers.ts` — that file is the authoritative list of all available fixture games and their purposes.

### When to Create a New Fixture vs Reuse

**✅ CREATE a new dedicated fixture when:**
- Writing a new E2E test file that will run in parallel
- Test requires specific participant/character setup
- Test will modify game state (upload avatars, post comments, etc.)

**❌ DO NOT reuse a fixture if:**
- The fixture was designed for a specific test file
- Your test's participant needs differ from the fixture's setup

### Creating a New E2E Fixture

**Step 1**: Add fixture SQL file (e.g., `e2e/26_my_feature.sql`):
```sql
-- Game #800: My New Feature Test
-- Purpose: For my-feature.spec.ts
```

**Step 2**: Add to `FIXTURE_GAMES` in `frontend/e2e/fixtures/game-helpers.ts`:
```typescript
MY_FEATURE: 'E2E My Feature',  // Game #800 - for my-feature.spec.ts
```

**Step 3**: Use in your test:
```typescript
const gameId = await getFixtureGameId(page, 'MY_FEATURE');
```

**Note**: If tests need parallel isolation (6 workers), create worker-specific SQL files: `26_my_feature_w0.sql` through `w5.sql`.

### Test Isolation Principles

1. **One Purpose Per Fixture**: Each fixture game serves ONE test file
2. **Independent State**: Tests should not depend on other tests' state changes
3. **Parallel Safe**: Fixtures must support parallel test execution
4. **Documented Purpose**: Always comment WHY a fixture exists and WHAT test uses it
5. **Minimal Overlap**: Avoid "generic" fixtures that many tests share

**Why This Matters:**
- Tests run in parallel for speed
- Shared fixtures cause race conditions and flaky tests
- Clear fixture ownership makes debugging easier
- Test isolation prevents cascading failures

## Fixture Maintenance

### Updating Fixtures

1. Modify appropriate SQL file in `test_fixtures/`
2. Run: `just reset-test-data && just test-fixtures`
3. Verify changes in application
4. Update this documentation if needed

### Adding New Scenarios

1. **Think First**: Does an existing fixture serve this purpose?
2. **Check Purpose**: Read fixture comments and game-helpers.ts
3. **Create if Needed**: Make a dedicated fixture for test isolation
4. **Document**: Update SQL comments and this file

## Troubleshooting

### Common Issues

**Schema Drift**: If tests fail with "column does not exist" errors:
```bash
# Apply migrations to test database
just migrate_test

# Then reapply fixtures
just test-fixtures
```

**Connection Errors**: Verify database is running:
```bash
docker ps | grep postgres
```

**Permission Errors**: Make script executable:
```bash
chmod +x backend/pkg/db/test_fixtures/apply_all.sh
```

**Unique Constraint Errors**: Reset first:
```bash
just reset-test-data && just test-fixtures
```

## Integration with Tests

### Backend Tests
When writing integration tests that use the database:

1. **Setup**: Apply test fixtures before test suite
2. **Transaction isolation**: Use transaction rollback pattern
3. **Known IDs**: Reference fixture IDs (Game #2 = Heist, etc.)
4. **Cleanup**: Rollback transactions after each test

### Frontend Tests
When writing component tests that display data:

1. **Mock API**: Use MSW with test data structure
2. **Consistent data**: Mirror fixture structure in mocks
3. **Edge cases**: Test with various fixture scenarios
4. **Loading states**: Use fixture data to test async loading

## Quick Reference

### Login as GM
```
Username: TestGM
Email: test_gm@example.com
Password: testpassword123
```

### Login as Player
```
Username: TestPlayer1
Email: test_player1@example.com
Password: testpassword123
```

### View Active Action Phase
Game #2: "The Heist at Goldstone Bank"

### View Long Phase History
Game #6: "Chronicles of Westmarch" (12 phases)

### View Recruiting Game
Game #7: "The Mystery of Blackwood Manor"

## References

- **Fixture SQL files**: `/backend/pkg/db/test_fixtures/`
- **Fixture game constants**: `frontend/e2e/fixtures/game-helpers.ts` (authoritative list)
- **Testing Strategy ADR**: `/docs-site/developer/architecture/adrs/007-testing-strategy.md`
- **E2E testing guide**: `frontend/e2e/README.md`

## Checklist Before Writing Tests

- [ ] Test fixtures applied and current
- [ ] Know which game/phase to use for test scenario
- [ ] Understand test user credentials
- [ ] Transaction isolation set up (for integration tests)
- [ ] Mock data mirrors fixture structure (for frontend tests)
- [ ] Edge cases identified from fixture coverage
