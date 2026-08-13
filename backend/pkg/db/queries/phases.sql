-- name: CreateGamePhase :one
INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, end_time, deadline)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetActivePhase :one
SELECT * FROM game_phases
WHERE game_id = $1 AND is_active = true;

-- name: GetActivePhaseActivatedAt :one
-- Returns the activated_at timestamp of the currently active phase for a game.
-- Used by the scheduler to detect if a manual activation superseded a scheduled one.
SELECT activated_at FROM game_phases
WHERE game_id = $1 AND is_active = true;

-- name: GetScheduledPhasesToActivate :many
-- Returns inactive phases whose start_time has arrived, for games that are in_progress.
-- Excludes phases with end_time set — those are completed/historical and should never be re-activated.
-- Used by the scheduler to auto-activate phases.
SELECT gp.*
FROM game_phases gp
JOIN games g ON gp.game_id = g.id
WHERE gp.is_active = false
  AND gp.start_time IS NOT NULL
  AND gp.start_time <= NOW()
  AND gp.end_time IS NULL
  AND g.state = 'in_progress'
ORDER BY gp.start_time ASC;

-- name: GetGamePhases :many
SELECT * FROM game_phases
WHERE game_id = $1
ORDER BY phase_number;

-- name: GetPhase :one
SELECT * FROM game_phases WHERE id = $1;

-- name: ActivatePhase :one
UPDATE game_phases
SET is_active = true, activated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeactivatePhase :one
UPDATE game_phases
SET is_active = false, end_time = NOW()
WHERE id = $1
RETURNING *;

-- name: DeactivateAllGamePhases :exec
UPDATE game_phases
SET is_active = false
WHERE game_id = $1;

-- name: ClearStaleScheduledStartTimes :exec
-- Clears start_time on inactive phases in a game whose start_time is in the past,
-- excluding the phase just activated. Called during phase activation to prevent
-- the scheduler from overriding a manual activation with an overdue scheduled phase.
UPDATE game_phases
SET start_time = NULL
WHERE game_id = $1
  AND id != $2
  AND is_active = false
  AND start_time IS NOT NULL
  AND start_time <= NOW();

-- name: UpdatePhaseDeadline :one
UPDATE game_phases
SET deadline = $2
WHERE id = $1
RETURNING *;

-- name: GetLatestPhaseNumber :one
SELECT COALESCE(MAX(phase_number), 0)
FROM game_phases
WHERE game_id = $1;

-- name: SubmitAction :one
-- submitted_at is stamped once on insert and preserved across edits: it marks
-- when the player first submitted, not when they last touched the text.
-- updated_at carries the latter, and the two being equal is how the handler
-- detects a first-time submission for GM notification.
INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, submitted_at)
VALUES ($1, $2, $3, $4, $5, NOW())
ON CONFLICT (game_id, user_id, phase_id)
DO UPDATE SET content = $5, character_id = $4,
              submitted_at = COALESCE(action_submissions.submitted_at, NOW()),
              updated_at = NOW()
RETURNING *;

-- name: GetUserAction :one
SELECT acts.*, c.name as character_name
FROM action_submissions acts
LEFT JOIN characters c ON acts.character_id = c.id
WHERE acts.game_id = $1 AND acts.user_id = $2 AND acts.phase_id = $3;

-- name: GetUserActions :many
SELECT acts.*, gp.phase_type, gp.phase_number, c.name as character_name
FROM action_submissions acts
JOIN game_phases gp ON acts.phase_id = gp.id
LEFT JOIN characters c ON acts.character_id = c.id
WHERE acts.game_id = $1 AND acts.user_id = $2
ORDER BY gp.phase_number DESC;

-- name: GetPhaseActions :many
SELECT acts.*, u.username, c.name as character_name
FROM action_submissions acts
JOIN users u ON acts.user_id = u.id
LEFT JOIN characters c ON acts.character_id = c.id
WHERE acts.phase_id = $1
ORDER BY acts.submitted_at;

-- name: GetGameActions :many
SELECT acts.*, u.username, c.name as character_name, gp.phase_type, gp.phase_number
FROM action_submissions acts
JOIN users u ON acts.user_id = u.id
JOIN game_phases gp ON acts.phase_id = gp.id
LEFT JOIN characters c ON acts.character_id = c.id
WHERE acts.game_id = $1
ORDER BY gp.phase_number, acts.submitted_at;

-- name: DeleteAction :exec
DELETE FROM action_submissions
WHERE game_id = $1 AND user_id = $2 AND phase_id = $3;

-- name: CreateActionResult :one
-- released_at tracks sent_at exactly: a result created already-published is
-- visible immediately, and a draft is visible to nobody until publish sets both.
--
-- This is load-bearing, not decorative. GetUserResults gates on
-- `released_at IS NOT NULL`, so a published row created without it would be
-- permanently invisible to its recipient — the same failure the migration's
-- backfill fixed for historical rows.
--
-- Creating a *staged* chain does not go through here; see CreateStagedResultPart,
-- which leaves released_at NULL for the release worker to set.
INSERT INTO action_results (game_id, user_id, phase_id, character_id, action_submission_id, gm_user_id, content, is_published, sent_at, released_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8,
        CASE WHEN $8 THEN NOW() ELSE NULL END,
        CASE WHEN $8 THEN NOW() ELSE NULL END)
RETURNING *;

-- name: CreateStagedResultPart :one
-- Append a non-head part to a chain. The head itself is an ordinary result and
-- goes through CreateActionResult; only parts 2..N come through here.
--
-- released_at is deliberately absent from the column list, so it defaults to
-- NULL: a staged part is invisible to its recipient until the release worker
-- sets it, even once is_published is TRUE. That NULL is the entire feature.
-- Contrast CreateActionResult, which sets released_at alongside sent_at.
--
-- sent_at still follows is_published, matching every other write path, so
-- "when did the GM send this" and "when did the player get to see it" stay
-- separate facts. For a staged part they genuinely differ.
--
-- parent_result_id and reveal_delay_minutes are both non-NULL here by
-- construction, which is what action_results_delay_requires_parent demands.
-- Chain length, recipient consistency, and acyclicity are enforced in the
-- service layer, since a CHECK constraint cannot walk the chain.
INSERT INTO action_results (game_id, user_id, phase_id, character_id, action_submission_id, gm_user_id, content, is_published, sent_at, parent_result_id, reveal_delay_minutes)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8,
        CASE WHEN $8 THEN NOW() ELSE NULL END,
        $9, $10)
RETURNING *;

-- name: GetUserResults :many
SELECT results.*, gp.phase_type, gp.phase_number, u.username as gm_username,
       c.name as character_name
FROM action_results results
JOIN game_phases gp ON results.phase_id = gp.id
JOIN users u ON results.gm_user_id = u.id
LEFT JOIN characters c ON results.character_id = c.id
WHERE results.game_id = $1 AND results.user_id = $2 AND results.is_published = true
  -- Staged reveals: a part of a multi-part result is invisible to its recipient
  -- until the release worker sets released_at. This is THE gate the feature
  -- rests on — if an unreleased part's content reaches the client, a player with
  -- devtools defeats the whole thing.
  --
  -- Ordinary unstaged results and every pre-feature row have released_at set at
  -- publish time (and were backfilled by migration 20260812214302), so this
  -- clause is a no-op for everything except pending staged parts.
  --
  -- This is the ONLY read path that gates on released_at besides the archive
  -- export. GM and audience deliberately see unreleased parts — see the
  -- "Who sees unreleased parts" table in
  -- .claude/planning/staged-result-reveals.md.
  AND results.released_at IS NOT NULL
-- Ascending within a phase, unlike the GM-facing queries: a player reading a
-- reveal delivered in several parts wants them in the order the GM sent them.
-- Only published rows are returned here, so sent_at is always populated; id is
-- the tiebreaker for results published in the same transaction.
ORDER BY gp.phase_number DESC, results.sent_at, results.id;

-- name: GetPhaseResults :many
SELECT results.*, u.username, gm.username as gm_username,
       c.name as character_name
FROM action_results results
JOIN users u ON results.user_id = u.id
JOIN users gm ON results.gm_user_id = gm.id
LEFT JOIN characters c ON results.character_id = c.id
WHERE results.phase_id = $1
-- Newest first. sent_at cannot order these: it is NULL until a result is
-- published, so drafts would all tie. Note this disagrees with GetGameResults,
-- which sorts ascending for the History tab — this query currently has no
-- callers, so there is nothing to be consistent with. Match GetGameResults if it
-- ever gets wired to a read path, or delete it.
ORDER BY results.id DESC;

-- name: GetGameResults :many
SELECT results.*, u.username, gp.phase_type, gp.phase_number,
       c.name as character_name
FROM action_results results
JOIN users u ON results.user_id = u.id
JOIN game_phases gp ON results.phase_id = gp.id
LEFT JOIN characters c ON results.character_id = c.id
WHERE results.game_id = $1
-- Oldest first within a phase, matching GetUserResults so the History tab reads
-- chronologically for every role. The GM and audience path returns the whole cast
-- (drafts included) while players get only their own published rows, but the two
-- are read side by side in the same view and disagreeing on order made the same
-- phase look different depending on who was logged in.
--
-- id (SERIAL) rather than sent_at: this query returns unpublished drafts, whose
-- sent_at is NULL until publish, so ordering by it would leave every draft tied
-- and arbitrary. A GM writes results in the order they mean them to be read, so
-- creation order is the chronological order.
--
-- GameResultsManager (the GM composing view) shares this query but wants newest
-- first, so it reverses client-side; see the comment there.
ORDER BY gp.phase_number, results.id;

-- Additional queries for comprehensive phase management

-- name: UpdatePhase :one
UPDATE game_phases
SET title = $2, description = $3, start_time = $4, end_time = $5, deadline = $6
WHERE id = $1
RETURNING *;

-- name: DeletePhase :exec
DELETE FROM game_phases WHERE id = $1;

-- name: GetActionSubmission :one
SELECT * FROM action_submissions WHERE id = $1;

-- name: GetUserPhaseSubmission :one
SELECT * FROM action_submissions
WHERE phase_id = $1 AND user_id = $2;

-- name: GetPhaseSubmissions :many
SELECT acts.*, u.username, c.name as character_name
FROM action_submissions acts
JOIN users u ON acts.user_id = u.id
LEFT JOIN characters c ON acts.character_id = c.id
WHERE acts.phase_id = $1
ORDER BY acts.submitted_at;

-- name: DeleteActionSubmission :exec
DELETE FROM action_submissions
WHERE id = $1 AND user_id = $2;

-- name: GetActionResult :one
SELECT * FROM action_results WHERE id = $1;

-- name: GetUserPhaseResults :many
-- ⚠️ Despite the name, this is NOT a player-facing read. It returns drafts and
-- unreleased staged parts, and it has no non-test callers today — it feeds the
-- GM composing view, which is why it sorts newest-first.
--
-- If you wire this to anything a player sees, add
-- `AND is_published = true AND released_at IS NOT NULL` first, or use
-- GetUserResults, which is the gated player path. Returning an unreleased
-- staged part here would hand the player the content of a reveal that has not
-- happened yet.
SELECT * FROM action_results
WHERE phase_id = $1 AND user_id = $2
-- See GetGameResults: sent_at is NULL for drafts, so it cannot order them.
ORDER BY id DESC;

-- name: PublishActionResult :one
-- Publishing a chain head (or an ordinary single result) makes it visible at
-- once. A non-head staged part keeps released_at NULL so the release worker
-- reveals it when its delay elapses — that is the whole feature, so the
-- parent_result_id check here is load-bearing.
UPDATE action_results
SET is_published = true,
    sent_at = NOW(),
    released_at = CASE WHEN parent_result_id IS NULL THEN NOW() ELSE released_at END
WHERE id = $1
RETURNING *;

-- name: PublishAllPhaseResults :exec
-- Same head-only release rule as PublishActionResult. Bulk-publishing ten
-- chains starts ten independent timers from this instant; the delays are
-- per-chain, so parts fire on their own schedules from here.
UPDATE action_results
SET is_published = true,
    sent_at = COALESCE(sent_at, NOW()),
    released_at = CASE WHEN parent_result_id IS NULL THEN COALESCE(released_at, NOW()) ELSE released_at END
WHERE phase_id = $1 AND is_published = false;

-- Staged result reveals -----------------------------------------------------
--
-- A chain is a linked list: each part points at its predecessor via
-- parent_result_id, and reveal_delay_minutes is measured from the moment that
-- predecessor became visible. See .claude/planning/staged-result-reveals.md.

-- name: GetDueStagedParts :many
-- Parts whose wait has elapsed and which the release worker should now reveal.
--
-- Due-ness is computed entirely in SQL from the parent's release time, which is
-- what makes this restart-safe: a part due while the process was down simply
-- comes back on the next tick, and there is no in-memory timer to lose or
-- double-fire.
--
-- Deliberately does NOT join game_phases or games. A chain owns its own clock —
-- phase advancement and game completion do not force, delay, or cancel a
-- release. Adding a phase or state filter here would break that; see "Chain
-- Independence" in the planning doc.
SELECT r.id, r.game_id, r.user_id, r.phase_id, r.character_id, r.gm_user_id,
       r.parent_result_id, r.reveal_delay_minutes
FROM action_results r
JOIN action_results parent ON r.parent_result_id = parent.id
WHERE r.is_published = TRUE
  AND r.released_at IS NULL
  AND parent.released_at IS NOT NULL
  AND parent.released_at + make_interval(mins => r.reveal_delay_minutes) <= NOW()
-- Oldest chains first, and parents before children within a chain: id ordering
-- means a chain inserted in part order releases in part order even if several
-- parts come due in the same tick.
ORDER BY r.id;

-- name: ReleaseStagedPart :one
-- Reveal one part. Guarded on released_at IS NULL so a double-tick or a
-- concurrent worker cannot re-release (and re-notify) an already-visible part;
-- the second caller gets no row back.
UPDATE action_results
SET released_at = NOW()
WHERE id = $1 AND released_at IS NULL
RETURNING *;

-- name: GetResultChain :many
-- The whole chain containing a given result, with 1-based part indices, walked
-- from the head. Used for "Part 2 of 3" labelling and for the GM's schedule
-- view.
--
-- Takes any member of the chain: the anchor CTE climbs to the head first, so
-- callers do not need to know whether they hold the head.
WITH RECURSIVE head AS (
    -- Climb from the given result to the chain head.
    SELECT anchor.id, anchor.parent_result_id
    FROM action_results anchor
    WHERE anchor.id = $1
    UNION ALL
    SELECT p.id, p.parent_result_id
    FROM action_results p
    JOIN head h ON h.parent_result_id = p.id
),
chain AS (
    -- Descend from the head, numbering as we go.
    SELECT r.*, 1 AS part_number
    FROM action_results r
    WHERE r.id = (SELECT id FROM head WHERE parent_result_id IS NULL)
    UNION ALL
    SELECT child.*, c.part_number + 1
    FROM action_results child
    JOIN chain c ON child.parent_result_id = c.id
)
SELECT c.*, (SELECT COUNT(*) FROM chain) AS part_count
FROM chain c
ORDER BY c.part_number;

-- name: CountChainLength :one
-- Number of parts already in the chain ending at $1, used to enforce the
-- max-chain-length invariant before appending another part.
WITH RECURSIVE ancestors AS (
    SELECT anchor.id, anchor.parent_result_id
    FROM action_results anchor
    WHERE anchor.id = $1
    UNION ALL
    SELECT p.id, p.parent_result_id
    FROM action_results p
    JOIN ancestors a ON a.parent_result_id = p.id
)
SELECT COUNT(*) AS length FROM ancestors;

-- name: GetUnpublishedResultsCount :one
SELECT COUNT(*) as count
FROM action_results
WHERE phase_id = $1 AND is_published = false;

-- name: GetUnpublishedResultIDs :many
SELECT id
FROM action_results
WHERE phase_id = $1 AND is_published = false;

-- name: UpdateActionResult :one
UPDATE action_results
SET content = $2
WHERE id = $1 AND is_published = false
RETURNING *;

-- name: DeleteActionResult :exec
DELETE FROM action_results
WHERE id = $1 AND is_published = false;

-- name: DeleteStagedPart :exec
-- Cancel a staged part that has not yet been revealed.
--
-- Guarded on released_at rather than is_published, which is the distinction
-- DeleteActionResult cannot make: a scheduled part is *published* but not yet
-- *released*, so DeleteActionResult's `is_published = false` matches nothing
-- and would silently delete zero rows.
--
-- ON DELETE CASCADE removes the parts scheduled after this one. A part whose
-- parent is gone has no release time to measure its delay from and would never
-- fire, so cascading is safer than orphaning.
DELETE FROM action_results
WHERE id = $1 AND released_at IS NULL AND parent_result_id IS NOT NULL;

-- name: GetSubmissionStatsForPhase :one
SELECT
    $1::int as phase_id,
    COUNT(DISTINCT gp.user_id) as total_players,
    COUNT(DISTINCT acts.user_id) as submitted_count,
    COALESCE(
        ROUND(
            (COUNT(DISTINCT acts.user_id) * 100.0) /
            NULLIF(COUNT(DISTINCT gp.user_id), 0),
            2
        ),
        0
    ) as submission_rate,
    MAX(acts.submitted_at) as latest_submission
FROM game_participants gp
JOIN game_phases ph ON gp.game_id = ph.game_id
LEFT JOIN action_submissions acts ON gp.user_id = acts.user_id AND acts.phase_id = ph.id
WHERE ph.id = $1 AND gp.role = 'player';

-- name: CanUserSubmitToPhase :one
SELECT
    CASE
        WHEN ph.phase_type != 'action' THEN false
        WHEN ph.deadline IS NOT NULL AND ph.deadline < NOW() THEN false
        WHEN NOT ph.is_active THEN false
        WHEN gp.role != 'player' THEN false
        ELSE true
    END as can_submit
FROM game_phases ph
JOIN games g ON ph.game_id = g.id
JOIN game_participants gp ON g.id = gp.game_id
WHERE ph.id = $1 AND gp.user_id = $2;

-- Phase transition queries

-- name: CreatePhaseTransition :one
INSERT INTO phase_transitions (game_id, from_phase_id, to_phase_id, initiated_by, reason)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetPhaseTransitions :many
SELECT pt.*,
       from_phase.phase_type as from_phase_type, from_phase.phase_number as from_phase_number,
       to_phase.phase_type as to_phase_type, to_phase.phase_number as to_phase_number,
       u.username as initiated_by_username
FROM phase_transitions pt
LEFT JOIN game_phases from_phase ON pt.from_phase_id = from_phase.id
JOIN game_phases to_phase ON pt.to_phase_id = to_phase.id
JOIN users u ON pt.initiated_by = u.id
WHERE pt.game_id = $1
ORDER BY pt.created_at;

-- Audience Participation Queries (Action Viewing)

-- name: ListAllActionSubmissions :many
-- List all action submissions for a game (for audience/GM)
--
-- Deliberately does NOT join action_results. Results are meaningful per phase,
-- not per submission: a player submits 0-1 actions but may receive several
-- results (staged reveals), so joining them fanned one submission into a row
-- per result and desynced LIMIT/OFFSET from CountAllActionSubmissions, which
-- counts submissions alone. Consumers read results from the per-phase results
-- endpoints instead.
SELECT acts.*, u.username, c.name as character_name, gp.phase_type, gp.phase_number, gp.title as phase_title
FROM action_submissions acts
JOIN users u ON acts.user_id = u.id
JOIN game_phases gp ON acts.phase_id = gp.id
LEFT JOIN characters c ON acts.character_id = c.id
WHERE acts.game_id = sqlc.arg(game_id)
  AND (CASE WHEN sqlc.arg(phase_id) = 0 THEN TRUE ELSE acts.phase_id = sqlc.arg(phase_id) END)
ORDER BY gp.phase_number DESC, acts.submitted_at DESC
LIMIT sqlc.arg(result_limit) OFFSET sqlc.arg(result_offset);

-- name: CountAllActionSubmissions :one
-- Count total action submissions for a game/phase (for pagination)
SELECT COUNT(*)
FROM action_submissions acts
WHERE acts.game_id = sqlc.arg(game_id)
  AND (CASE WHEN sqlc.arg(phase_id) = 0 THEN TRUE ELSE acts.phase_id = sqlc.arg(phase_id) END);

-- name: CountActionSubmissionsByCharacter :one
-- Count action submissions for a specific character
-- Used to check if character can be deleted
SELECT COUNT(*)
FROM action_submissions
WHERE character_id = $1;

-- Delete validation queries

-- name: CountActionSubmissionsByPhase :one
-- Count action submissions for a specific phase
-- Used to check if phase can be deleted
SELECT COUNT(*)
FROM action_submissions
WHERE phase_id = $1;

-- name: CountActionResultsByPhase :one
-- Count action results for a specific phase
-- Used to check if phase can be deleted
SELECT COUNT(*)
FROM action_results
WHERE phase_id = $1;

-- name: CountPollsByPhase :one
-- Count polls for a specific phase
-- Used to check if phase can be deleted
SELECT COUNT(*)
FROM common_room_polls
WHERE phase_id = $1;

-- name: CountThreadsByPhase :one
-- Count common room threads for a specific phase
-- Used to check if phase can be deleted
SELECT COUNT(*)
FROM threads
WHERE phase_id = $1;

-- name: CountMessagesByPhase :one
-- Count non-draft messages for a specific phase
-- Used to check if phase can be deleted; draft posts are excluded (they're cleaned up separately)
SELECT COUNT(*)
FROM messages
WHERE phase_id = $1
  AND is_draft = false;
