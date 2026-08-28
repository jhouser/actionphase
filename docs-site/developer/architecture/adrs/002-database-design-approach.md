# ADR-002: Database Design Approach

## Status
Accepted

## Context
ActionPhase requires a database design that can handle:
- Complex game state with nested data structures
- Character sheets with variable schemas depending on game system
- Phase-based gameplay with temporal data
- User management and session tracking
- Performance for concurrent users
- Flexibility for future game system additions

The decision needed to balance between structured relational data and flexible document storage.

## Decision
We adopted a **Hybrid Relational-Document approach** using PostgreSQL with strategic JSONB usage:

**Core Entities**: Traditional relational tables
- Users, Games, Characters, Phases, Applications - structured as normalized tables
- Foreign key relationships for data integrity
- Proper indexing for query performance

**Flexible Data**: JSONB columns for variable schema data
- Character data sheets in `character_data` JSONB column
- Game-specific configuration in `game_config` JSONB column
- Action submissions in `action_data` JSONB column
- Notification preferences in `preferences` JSONB column

> ⚠️ **Three of these four columns do not exist** (verified 2026-08-26). See
> [Implementation Divergence](#implementation-divergence-verified-2026-08-26).

**Schema Management**: golang-migrate for version control
- All schema changes tracked in migration files
- Up/down migrations for rollback capability
- Environment-specific migration control

## Alternatives Considered

### 1. Pure Relational Approach
**Approach**: Traditional normalized relational schema with separate tables for all entities.

**Pros**:
- Strong consistency and ACID compliance
- Excellent query performance for structured data
- Clear data relationships and constraints
- Familiar to most developers

**Cons**:
- Inflexible for variable character sheet schemas
- Complex joins for game-specific data
- Difficult to add new game systems without schema changes
- Over-normalization leading to query complexity

### 2. Pure Document Approach (MongoDB)
**Approach**: Store all game data as nested JSON documents.

**Pros**:
- Maximum flexibility for game data structures
- Easy to add new game systems
- Natural fit for nested character data
- No impedance mismatch with JSON APIs

**Cons**:
- Limited transaction support across documents
- Eventual consistency issues
- Less mature query capabilities
- Difficult to maintain referential integrity

### 3. EAV (Entity-Attribute-Value) Pattern
**Approach**: Generic key-value storage for flexible attributes.

**Pros**:
- Flexible schema evolution
- Supports arbitrary attributes
- Relational database benefits

**Cons**:
- Poor query performance
- Complex queries for simple operations
- Loss of type safety
- Difficult to maintain and understand

## Consequences

### Positive Consequences

**Data Integrity**:
- Strong ACID guarantees for critical game state
- Foreign key constraints prevent orphaned records
- Transaction support for multi-table operations

**Performance**:
- Optimized queries for structured data access patterns
- JSONB indexing for efficient document queries
- Connection pooling for concurrent access

**Flexibility**:
- JSONB allows arbitrary character sheet schemas
- Easy addition of new game systems without migrations
- Support for complex nested data structures

**Developer Experience**:
- sqlc generates type-safe Go code for structured queries
- Familiar SQL for complex relational queries
- JSON handling for document-style operations

### Negative Consequences

**Complexity**:
- Developers need to understand both relational and document patterns
- Query complexity varies between structured and document data
- Mixed paradigms can be confusing for new team members

**Schema Evolution**:
- JSONB schema changes require application-level validation
- No automatic migration for document structure changes
- Potential for inconsistent document schemas over time

**Query Limitations**:
- Complex JSONB queries can be less readable than SQL
- Limited aggregation capabilities for nested document data
- Indexing strategy becomes more complex

### Migration Strategy

**Existing Data**:
- No breaking changes to current schema
- Additive JSONB columns to existing tables
- Gradual migration of flexible data to JSONB

**Future Considerations**:
- Document schema versioning strategy needed
- JSONB query performance monitoring
- Potential extraction of frequently-queried JSONB fields to columns

## Implementation Details

### Core Tables Structure

> ❌ **The sample originally here did not match the schema** — it put
> `preferences` on `users`, invented `games.game_config`, and made
> `character_data` a JSONB column on `characters`. Corrected below from
> `backend/pkg/db/schema.sql` (abridged).

```sql
-- Users: traditional relational. Preferences live in their OWN table,
-- one row per user.
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE user_preferences (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  preferences JSONB NOT NULL DEFAULT '{}'::jsonb,
  UNIQUE(user_id)
);
CREATE INDEX idx_user_preferences_jsonb ON user_preferences USING GIN (preferences);

-- Games: typed columns, plus ONE JSONB column for sheet configuration.
-- There is no `game_config`.
CREATE TABLE games (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    gm_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    state VARCHAR(50) DEFAULT 'setup',
    genre VARCHAR(100),
    max_players INTEGER DEFAULT 6,
    is_public BOOLEAN DEFAULT TRUE,
    character_sheet JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Characters: fully relational metadata — no JSONB here.
CREATE TABLE characters (
    id SERIAL PRIMARY KEY,
    game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    character_type VARCHAR(50) NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    is_active BOOLEAN DEFAULT TRUE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Character sheet content: an EAV table, NOT a JSONB document.
CREATE TABLE character_data (
    id SERIAL PRIMARY KEY,
    character_id INTEGER NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    module_type VARCHAR(50) NOT NULL,
    field_name VARCHAR(100) NOT NULL,
    field_value TEXT,
    field_type VARCHAR(50) DEFAULT 'text',
    is_public BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

### JSONB Usage Patterns

> ❌ **The queries originally here referenced columns that do not exist**
> (`characters.character_data`, `games.game_config`). Replaced with the real
> access patterns.

```sql
-- Character sheet content is EAV, not JSONB: one row per field.
SELECT field_name, field_value, field_type, is_public
FROM character_data
WHERE character_id = $1
ORDER BY module_type, field_name;

-- Per-game character sheet configuration (real JSONB).
SELECT character_sheet
FROM games
WHERE id = $1;

-- User preferences (real JSONB, GIN-indexed).
SELECT preferences
FROM user_preferences
WHERE user_id = $1;

-- The one JSONB index that exists:
CREATE INDEX idx_user_preferences_jsonb
  ON user_preferences USING GIN (preferences);
```

Note there is **no JSONB index on `games.character_sheet`** — it is read whole
by primary key, so it never needs one.

### Data Validation Strategy
- **Application Layer**: Validate structure before insert — this is where sheet
  validation actually happens, for both the EAV rows and JSONB documents
- **Go types**: `core.CharacterSheetConfig` is the schema for
  `games.character_sheet`
- **Database Constraints**: typed columns and CHECK constraints carry most of
  the invariants, since the schema is largely relational
- **Migration Scripts**: Transform JSONB data when document schemas evolve

## Implementation Divergence (verified 2026-08-26)

The shipped schema is **substantially more relational** than this ADR describes.
`backend/pkg/db/schema.sql` contains exactly **two** JSONB columns, not four.

| ADR claims | Reality |
|---|---|
| `character_data` JSONB column | `character_data` is a **table**, not a column — an EAV store (`module_type`, `field_name`, `field_value`, `field_type`, `is_public`) |
| `game_config` JSONB column | **Does not exist.** Game settings are ordinary typed columns on `games` (`max_players`, `is_public`, `genre`, `common_room_open_day`, …) |
| `action_data` JSONB column | **Does not exist.** `action_submissions.content` is `TEXT` |
| `preferences` JSONB column | ✅ Correct — `user_preferences.preferences`, GIN-indexed |

The other real JSONB column is `games.character_sheet` (per-game character-sheet
config), which this ADR does not mention.

### Why this matters

The hybrid design's stated benefits — "JSONB allows arbitrary character sheet
schemas" and "easy addition of new game systems without migrations" — are
achieved by the **EAV `character_data` table**, not by JSONB. That carries
different trade-offs than the ADR analyses:

- Values are `TEXT` with a `field_type` discriminator, so the database does not
  enforce types; validation is entirely application-level (the ADR anticipated
  this consequence, just for the wrong mechanism).
- Reading a full sheet is a multi-row fetch plus application-side assembly,
  rather than one JSONB read.
- `is_public` per field gives row-level visibility control that a single JSONB
  blob could not express without extra structure — a genuine advantage of the
  chosen design that this ADR never records.

**The core decision — PostgreSQL, sqlc, golang-migrate, relational core with
selective document storage — still holds.** Only the specific columns are wrong.

## References
- [PostgreSQL JSONB Documentation](https://www.postgresql.org/docs/current/datatype-json.html)
- [JSONB Indexing Strategies](https://www.postgresql.org/docs/current/datatype-json.html#JSON-INDEXING)
- [golang-migrate Documentation](https://github.com/golang-migrate/migrate)
- [sqlc JSONB Support](https://docs.sqlc.dev/en/latest/howto/query-json.html)
