-- Community Discord webhooks (req 9).
--
-- The `url` column is a CREDENTIAL: anyone holding it can post to the channel.
-- These queries return it because the dispatcher needs to replay it, but no
-- handler may pass it to a client unmasked -- see core.MaskWebhookURL and the
-- service's webhook converters, which mask on the way out.

-- name: CreateCommunityWebhook :one
INSERT INTO community_webhooks (
    community_id, url, label, is_enabled, events
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: GetCommunityWebhook :one
-- Scoped by community as well as id, for the same reason as community
-- documents: a bare id would resolve a webhook belonging to a DIFFERENT
-- community than the request path names. Here the stakes are higher than a
-- leaked draft -- it would let a moderator of A read (or overwrite) B's
-- channel credential. The pairing makes that a no-rows 404.
SELECT * FROM community_webhooks
WHERE id = $1 AND community_id = $2;

-- name: ListCommunityWebhooks :many
-- The moderator's config listing: DISABLED ROWS INCLUDED, since the whole point
-- of the screen is to re-enable or repair them.
SELECT * FROM community_webhooks
WHERE community_id = $1
ORDER BY id ASC;

-- name: ListWebhooksForGameState :many
-- The dispatch lookup, run after every game state transition.
--
-- Resolves game -> community -> enabled webhooks subscribed to this state in
-- one round-trip. Three filters matter and all three live HERE rather than in
-- Go, so no dispatch path can forget one:
--
--   1. The join through games.community_id yields NO ROWS for a legacy game
--      whose community is NULL. Grandfathering (req 5) is structural.
--   2. is_enabled excludes webhooks a moderator has switched off.
--   3. $2 = ANY(events) excludes webhooks not subscribed to this state, so a
--      recruitment-only channel stays quiet when a game completes.
SELECT w.* FROM community_webhooks w
JOIN games g ON g.community_id = w.community_id
WHERE g.id = $1
  AND w.is_enabled
  -- Cast is load-bearing: without it sqlc infers $2's type from the array it
  -- is compared against and generates a []string parameter for what is a
  -- single state name.
  AND sqlc.arg('state')::text = ANY(w.events)
ORDER BY w.id ASC;

-- name: UpdateCommunityWebhook :one
-- Partial update: COALESCE leaves an omitted field untouched.
--
-- `url` is included so a moderator can rotate a regenerated Discord URL, but it
-- can only be SET, never cleared -- the column is NOT NULL and a webhook with
-- no URL has nothing to deliver to. Passing NULL keeps the stored credential,
-- which is also what lets the config form save a label change without ever
-- having to echo the secret back to the client and re-submit it.
UPDATE community_webhooks
SET url = COALESCE(sqlc.narg('url'), url),
    label = COALESCE(sqlc.narg('label'), label),
    is_enabled = COALESCE(sqlc.narg('is_enabled'), is_enabled),
    events = COALESCE(sqlc.narg('events'), events),
    updated_at = NOW()
WHERE id = $1 AND community_id = $2
RETURNING *;

-- name: DeleteCommunityWebhook :exec
DELETE FROM community_webhooks
WHERE id = $1 AND community_id = $2;

-- name: MarkCommunityWebhookSuccess :exec
-- Stamps a delivery success and CLEARS the previous error.
--
-- Clearing matters: a webhook that failed yesterday and works today must not
-- keep showing a stale error, or moderators learn to ignore the field.
--
-- Not community-scoped, unlike every read above. The dispatcher already
-- resolved this row through the community join, and it runs on a detached
-- goroutine that has no request path to scope against.
UPDATE community_webhooks
SET last_success_at = NOW(),
    last_error = NULL,
    last_error_at = NULL,
    updated_at = NOW()
WHERE id = $1;

-- name: MarkCommunityWebhookError :exec
-- Stamps a delivery failure after all retries are exhausted.
--
-- Deliberately does NOT clear last_success_at: "worked at 09:00, broken since
-- 14:00" is the diagnosis a moderator needs, and dropping the success time
-- would lose the half that says this configuration ever worked at all.
UPDATE community_webhooks
SET last_error = $2,
    last_error_at = NOW(),
    updated_at = NOW()
WHERE id = $1;
