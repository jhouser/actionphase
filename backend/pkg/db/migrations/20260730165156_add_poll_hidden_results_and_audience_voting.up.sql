-- Poll visibility and participation options
--
-- hide_results_from_players: when TRUE, players never see poll results — not the
-- winner, not the counts, not the "other" responses — even after the deadline
-- passes. GMs, co-GMs and audience still see results as normal.
--
-- allow_audience_voting: when TRUE, audience members may vote on the poll. Their
-- votes count toward the totals but are attributed as "Anonymous", since audience
-- members do not necessarily have characters.

ALTER TABLE common_room_polls
    ADD COLUMN hide_results_from_players BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN allow_audience_voting BOOLEAN NOT NULL DEFAULT FALSE;

-- Hiding results from players is mutually exclusive with showing them individual
-- vote attribution: the latter is a strictly larger disclosure than the former hides.
ALTER TABLE common_room_polls
    ADD CONSTRAINT poll_hidden_results_excludes_individual_votes
    CHECK (NOT (hide_results_from_players AND COALESCE(show_individual_votes, FALSE)));
