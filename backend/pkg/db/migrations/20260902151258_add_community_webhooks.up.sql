-- Community Discord webhooks (req 9): a community announces its games' state
-- transitions into a Discord channel it controls.
--
-- Best-effort by design. There is deliberately NO deliveries table, no queue,
-- and no retry scheduler -- see the plan's section 6. A missed Discord ping is
-- not worth durability machinery, so the entire delivery-observability story is
-- the three last_* columns below: they answer "my webhook stopped working,
-- why?" without persisting a delivery history nobody reads.

CREATE TABLE community_webhooks (
    id SERIAL PRIMARY KEY,
    community_id INTEGER NOT NULL REFERENCES communities(id) ON DELETE CASCADE,
    -- THE URL IS A CREDENTIAL. Anyone holding it can post to the channel, so it
    -- is never returned by the API in full -- reads emit a masked form and the
    -- column is write-only from the client's perspective. Stored in plaintext
    -- because it must be replayable on every dispatch; treat it like a password
    -- in logs and responses, not like an identifier.
    url TEXT NOT NULL,
    -- Moderator's own name for the channel ("#recruitment"). Nullable: a
    -- community with one webhook has nothing to disambiguate.
    label VARCHAR(100),
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    -- Game states to notify on, e.g. {recruitment,in_progress}. A TEXT[] rather
    -- than a join table: the set is small, always read whole, and never queried
    -- from the other direction. Empty means this webhook fires for nothing,
    -- which is a valid (if pointless) configuration and not an error.
    events TEXT[] NOT NULL DEFAULT '{}',
    -- The whole delivery-observability story. Success stamps last_success_at and
    -- clears last_error; exhausted retries stamp both last_error columns. A row
    -- with an old last_success_at and a fresh last_error is a broken webhook,
    -- which is the one diagnosis a moderator actually needs.
    last_success_at TIMESTAMPTZ,
    last_error TEXT,
    last_error_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The dispatch lookup: every game state transition in a community with webhooks
-- runs this, so it is the hot path. Partial on is_enabled because a disabled
-- webhook is never dispatched to and only bloats the index.
CREATE INDEX idx_community_webhooks_dispatch
    ON community_webhooks(community_id)
    WHERE is_enabled;

-- The moderator's config list, which includes disabled rows and so cannot use
-- the partial index above.
CREATE INDEX idx_community_webhooks_community
    ON community_webhooks(community_id, id);
