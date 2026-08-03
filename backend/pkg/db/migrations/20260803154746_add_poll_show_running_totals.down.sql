ALTER TABLE common_room_polls
    DROP CONSTRAINT IF EXISTS poll_hidden_results_excludes_running_totals;

ALTER TABLE common_room_polls
    DROP COLUMN IF EXISTS show_running_totals_to_players;
