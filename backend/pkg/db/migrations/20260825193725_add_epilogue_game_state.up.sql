-- Add 'epilogue' game state: a writable public archive.
--
-- 'epilogue' sits between 'in_progress' and 'completed'. It grants the READ
-- permissions of 'completed' (the whole game becomes readable by any
-- authenticated user) while keeping the WRITE permissions of 'in_progress'
-- (the GM can still create phases and threads, players can still post), so
-- epilogue and meta-discussion threads can be written with the archive open.
--
-- 'completed' is unchanged: still terminal, still read-only.

BEGIN;

ALTER TABLE games DROP CONSTRAINT IF EXISTS games_state_check;

ALTER TABLE games ADD CONSTRAINT games_state_check
    CHECK (state IN ('setup', 'recruitment', 'character_creation',
                     'in_progress', 'paused', 'epilogue', 'completed', 'cancelled'));

COMMIT;
