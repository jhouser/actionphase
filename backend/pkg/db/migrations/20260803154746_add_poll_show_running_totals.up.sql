-- show_running_totals_to_players: when TRUE, players may see the poll's results
-- while voting is still open, rather than having to wait for the deadline — the
-- same live view GMs and audience already get.
--
-- Vote attribution is still governed by show_individual_votes: by default players
-- see only the per-option tallies; with show_individual_votes also set they see
-- who is currently voting for what.

ALTER TABLE common_room_polls
    ADD COLUMN show_running_totals_to_players BOOLEAN NOT NULL DEFAULT FALSE;

-- Showing running totals is the opposite of hiding results: a poll cannot both
-- disclose results early and withhold them permanently.
ALTER TABLE common_room_polls
    ADD CONSTRAINT poll_hidden_results_excludes_running_totals
    CHECK (NOT (hide_results_from_players AND show_running_totals_to_players));
