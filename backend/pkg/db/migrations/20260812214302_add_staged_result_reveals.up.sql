-- Staged result reveals: split one action result into several parts separated
-- by a timer, so a GM can build dramatic tension ("...the sword whooshes toward
-- your head..." / 15 minutes / "...and misses!").
--
-- A chain is a linked list: each part points at the one before it, because the
-- delay is always relative to the previous part. That makes the release
-- worker's due-ness check a single comparison against the parent's release
-- time, with no chain order to reconstruct.
--
-- The load-bearing column is released_at. Visibility to the recipient becomes
-- `is_published = TRUE AND released_at IS NOT NULL`, so every existing row MUST
-- be backfilled or it silently vanishes from a live game. See the backfill
-- below.

BEGIN;

ALTER TABLE action_results
    -- The part immediately before this one. NULL = part 1, or an ordinary
    -- unstaged result (which is just a one-part chain).
    --
    -- ON DELETE CASCADE: deleting an unpublished chain head takes the whole
    -- tail with it. A tail with no head has no schedule and would never
    -- release, so an orphan is strictly worse than a deletion.
    ADD COLUMN parent_result_id INTEGER REFERENCES action_results(id) ON DELETE CASCADE,

    -- Minutes to wait after the parent is released. NULL for part 1.
    ADD COLUMN reveal_delay_minutes INTEGER,

    -- When this part became visible to its recipient. Set at publish time for a
    -- chain head; set by the release worker for later parts.
    -- NULL = not yet visible.
    ADD COLUMN released_at TIMESTAMPTZ;

-- Backfill: every already-published result is, by definition, already visible.
--
-- Deliberately unconditional. No game has ever used this feature, so there is
-- no row that legitimately wants released_at IS NULL while published, and
-- nothing to protect by being selective. Any published row this misses becomes
-- invisible to its player, which is the worst outcome available here.
UPDATE action_results
SET released_at = sent_at
WHERE is_published = TRUE;

-- Every write path that publishes also sets sent_at (CreateActionResult's
-- CASE WHEN, PublishActionResult, and PublishAllPhaseResults' COALESCE), so
-- the statement above should already cover everything. This catches a
-- published row with a NULL sent_at anyway rather than trusting that reading
-- of the write paths to be exhaustive.
UPDATE action_results
SET released_at = COALESCE(sent_at, NOW())
WHERE is_published = TRUE AND released_at IS NULL;

-- Structural invariant: a delay is meaningful only relative to a parent.
-- Part 1 has neither; every later part has both.
ALTER TABLE action_results
    ADD CONSTRAINT action_results_delay_requires_parent
        CHECK ((parent_result_id IS NULL) = (reveal_delay_minutes IS NULL));

-- Bounds live here as well as in the service layer: an unbounded delay can
-- strand a reveal past the end of a game. 1 minute floor because the worker
-- ticks every minute; 1440 (24h) ceiling as a sanity limit on a feature whose
-- whole point is a pause measured in minutes.
ALTER TABLE action_results
    ADD CONSTRAINT action_results_delay_bounds
        CHECK (reveal_delay_minutes IS NULL
               OR (reveal_delay_minutes >= 1 AND reveal_delay_minutes <= 1440));

-- A part may not be its own parent. Longer cycles are not expressible: a part
-- can only reference an already-inserted row, so the chain is acyclic by
-- construction and this closes the one-row case.
ALTER TABLE action_results
    ADD CONSTRAINT action_results_no_self_parent
        CHECK (parent_result_id IS NULL OR parent_result_id <> id);

-- The release worker's scan: published parts awaiting release. Partial, so it
-- indexes only the handful of rows in flight rather than the whole table.
CREATE INDEX idx_action_results_pending_release
    ON action_results(parent_result_id)
    WHERE released_at IS NULL AND parent_result_id IS NOT NULL;

COMMIT;
