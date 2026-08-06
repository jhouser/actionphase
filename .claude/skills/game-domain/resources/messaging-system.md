# Messaging System

Message types, storage, and visibility rules.

Source of truth: `backend/pkg/db/schema.sql`,
`backend/pkg/db/queries/messages.sql`, `backend/pkg/db/services/messages/`.

## Two Storage Systems

### 1. `messages` — common room content

⚠️ **There are no separate `posts` or `comments` tables.** Posts, comments, and
some private message records all live in ONE table, discriminated by enums:

```sql
CREATE TYPE message_type AS ENUM ('post', 'comment', 'private_message');
CREATE TYPE message_visibility AS ENUM ('game', 'private');
```

Key columns:

- `parent_id` — self-referential FK; comments nest arbitrarily deep
- `thread_depth` — denormalized depth, maintained on insert
- `mentioned_character_ids INTEGER[]` — GIN-indexed
- `is_draft` — author-only visibility
- `is_deleted` / `deleted_at` / `deleted_by_user_id` — soft delete
- `character_avatar_url_at_post` — avatar pinned at post time; readers COALESCE
  to the live `characters.avatar_url` when NULL

### 2. `conversations` + `private_messages` — direct/group messages

- `conversations` — `conversation_type` is `'direct'` or `'group'`
- `conversation_participants` — who is in it (user + optional character)
- `private_messages` — the message bodies
- ⚠️ `conversations` has **no `phase_id`** — conversations span phases

## Nested Comment Threads

Threads can be very deep (100+ replies across many levels). Fetch a whole tree
in ONE recursive CTE rather than N+1 per level; `idx_messages_thread` covers
`(game_id, parent_id, created_at)`:

```sql
WITH RECURSIVE tree AS (
  SELECT m.*, ARRAY[m.created_at] AS path FROM messages m WHERE m.id = $1
  UNION ALL
  SELECT c.*, t.path || c.created_at FROM messages c JOIN tree t ON c.parent_id = t.id
)
SELECT * FROM tree ORDER BY path;
```

Relevant endpoints:

```
GET /api/v1/games/{gameID}/posts/{postId}/comments-with-threads  -- paginated + nested
GET /api/v1/games/{gameID}/messages/{messageId}/thread-context   -- ancestor chain
```

## Visibility Rules

| Content | Hidden from | Visible to |
|---|---|---|
| Draft (`is_draft`) | everyone else | author |
| Soft-deleted | normal reads | — |
| Action submission | other players | author, GM/co-GM, audience |
| Action result | the player, until `is_published` | GM/co-GM, audience |
| Private conversation | uninvolved *players* | participants, GM/co-GM, audience |

⚠️ **Private conversations are NOT hidden from the GM or audience.**
`ListAllPrivateConversations` and `GetAudienceConversationMessages`
(`backend/pkg/db/queries/messages.sql:673,781`) apply no participant filter.

Once a game is `completed`, all of the above is readable by any authenticated
user via public archive mode. Use `CanUserViewGame` for read authorization.

## Anonymous Games

When `games.is_anonymous = true`, players cannot see each other's usernames.
GMs, co-GMs, and audience always can.

⚠️ **Anonymity is play-time only.** Once the game is `completed`, public archive
mode discloses usernames to everyone, including non-participants — the same way
completion lifts the individual-vote restriction on polls. Cancelled games are
not public and keep the play-time rule.

See `CanSeeUsernamesInAnonymousGame` (`backend/pkg/core/permissions.go`).

---

**Back to**: [SKILL.md](../SKILL.md)
