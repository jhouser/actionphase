-- Drop action_submissions.is_draft.
--
-- Scaffolding for a player-side draft feature that was never built and is now
-- ruled out. The submission model has no draft state: players edit a submission
-- freely until the phase deadline, and whatever stands at the deadline is what
-- was submitted. Every write path passed an explicit FALSE, so no row in the
-- wild is a draft and this drop changes no observable behavior.
ALTER TABLE action_submissions DROP COLUMN is_draft;
