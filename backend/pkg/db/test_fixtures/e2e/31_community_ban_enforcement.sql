-- Games 810-812: dedicated join targets for the community ban enforcement specs.
--
-- Why these exist rather than reusing 329-341: those games are owned by
-- game-application-workflow.spec.ts, and several PRE-SEED an application for
-- the very users the ban specs need to apply as. A ban spec that withdraws such
-- an application to get a clean start destroys another spec's fixture; one that
-- does not withdraw passes on a fresh load and then fails on every re-run,
-- because "already applied" and "blocked by a ban" look identical from the UI.
--
-- So each positive control gets its own recruitment game with NO applications
-- on it. The specs still withdraw at the start of each run, which makes them
-- re-runnable against a single fixture load; because nothing else reads these
-- games, that withdrawal cannot disturb anyone.
--
--   810 -- in Midnight Ravens. The REFUSAL target: TestPlayer5 is banned there.
--   811 -- moved to Harbor Lights by 99_backfill_communities.sql. The scope
--          control: TestPlayer5 is NOT banned there and must get in.
--   812 -- in Midnight Ravens. The expiry control: TestPlayer4's ban there has
--          lapsed, so they must get in despite still having a ban row.
--
-- IDs are bare literals in both the DELETE and the INSERTs so that
-- apply_e2e_worker.sh can rewrite them per worker; a PL/pgSQL variable would
-- hide them from that rewriter and every worker would collide on 810.

BEGIN;

DELETE FROM games WHERE id IN (810, 811, 812);

DO $$
DECLARE
  gm_id INTEGER;
BEGIN
  SELECT id INTO gm_id FROM users WHERE email = 'test_gm@example.com';

  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    810,
    'E2E Test: Ban Enforcement - Blocked',
    'Recruitment game in Midnight Ravens. A user banned there must be refused.',
    'Test', gm_id, 6, 'recruitment', true,
    NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days'
  );

  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    811,
    'E2E Test: Ban Enforcement - Other Community',
    'Recruitment game moved to Harbor Lights. A user banned only in Midnight Ravens must still get in.',
    'Test', gm_id, 6, 'recruitment', true,
    NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days'
  );

  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    812,
    'E2E Test: Ban Enforcement - Expired Ban',
    'Recruitment game in Midnight Ravens. A user whose ban there has expired must still get in.',
    'Test', gm_id, 6, 'recruitment', true,
    NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days'
  );

  PERFORM setval('games_id_seq', GREATEST((SELECT MAX(id) FROM games), 1), true);
END $$;

COMMIT;
