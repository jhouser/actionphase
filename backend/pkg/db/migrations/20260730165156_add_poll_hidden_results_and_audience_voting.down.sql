ALTER TABLE common_room_polls
    DROP CONSTRAINT IF EXISTS poll_hidden_results_excludes_individual_votes;

ALTER TABLE common_room_polls
    DROP COLUMN IF EXISTS hide_results_from_players,
    DROP COLUMN IF EXISTS allow_audience_voting;
