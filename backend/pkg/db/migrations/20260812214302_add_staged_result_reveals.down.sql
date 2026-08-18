-- Reverses add_staged_result_reveals.
--
-- Dropping the columns would take the constraints and the index with them, but
-- they are dropped explicitly (IF EXISTS) so this still runs cleanly if the up
-- migration failed partway and left only some objects behind.
--
-- Data loss on the way down is total and intended: any staged chain reverts to
-- a set of unrelated results, and an unreleased part becomes an ordinary
-- published-or-draft row with no schedule. There is no way to preserve a
-- schedule in a schema that has no columns for one.

BEGIN;

DROP INDEX IF EXISTS idx_action_results_pending_release;

ALTER TABLE action_results
    DROP CONSTRAINT IF EXISTS action_results_no_self_parent,
    DROP CONSTRAINT IF EXISTS action_results_delay_bounds,
    DROP CONSTRAINT IF EXISTS action_results_delay_requires_parent;

ALTER TABLE action_results
    DROP COLUMN IF EXISTS released_at,
    DROP COLUMN IF EXISTS reveal_delay_minutes,
    DROP COLUMN IF EXISTS parent_result_id;

COMMIT;
