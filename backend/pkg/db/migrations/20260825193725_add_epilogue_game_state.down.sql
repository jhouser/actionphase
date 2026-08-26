-- Remove the 'epilogue' game state.
--
-- LOSSY: any game sitting in 'epilogue' must land on a state the narrowed
-- constraint still permits. 'completed' is the only honest target — an
-- epilogue game has already disclosed its entire archive to every player, so
-- rolling it back to 'in_progress' would restore a secrecy that no longer
-- exists. The cost is that the game also becomes read-only.

BEGIN;

UPDATE games SET state = 'completed' WHERE state = 'epilogue';

ALTER TABLE games DROP CONSTRAINT IF EXISTS games_state_check;

ALTER TABLE games ADD CONSTRAINT games_state_check
    CHECK (state IN ('setup', 'recruitment', 'character_creation',
                     'in_progress', 'paused', 'completed', 'cancelled'));

COMMIT;
