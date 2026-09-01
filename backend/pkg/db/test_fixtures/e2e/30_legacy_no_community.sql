-- Game 800: the permanent grandfathering guard.
--
-- 🔴 THIS GAME MUST KEEP community_id IS NULL. Do not "fix" it, and do not let
-- a backfill assign it a community -- 99_backfill_communities.sql excludes it
-- by ID for exactly this reason.
--
-- Why it exists: games.community_id is nullable, so NULL is reachable. Every
-- ban enforcement path is written to treat a NULL community as "no ban can
-- apply" (the enforcement queries inner-join through games.community_id, so a
-- NULL yields no row). If every fixture game had a community, that branch would
-- ship untested and a NULL game could start 500ing or being wrongly blocked
-- without any test noticing.
--
-- What it guards, concretely:
--   * CanUserApplyToGame returns 'can_apply', never 'community_banned'
--   * IsUserBannedFromGameCommunity returns false, even for a user who IS
--     banned from some other community
--   * the game page renders with no community section rather than erroring
--
-- TestPlayer5 is banned from Midnight Ravens (see 00_communities.sql) and is
-- deliberately the user specs should apply here with: a banned user must still
-- be able to join a game that belongs to no community.

BEGIN;

-- The ID is written as a bare literal in both the DELETE and the INSERT, not
-- via a variable: apply_e2e_worker.sh offsets game IDs by rewriting literals it
-- can see (INSERT INTO games (...id...) VALUES (800, ...), DELETE ... id = 800).
-- Passing a PL/pgSQL variable hides the number from that rewriter, so every
-- worker would insert id 800 and all but the first would silently no-op --
-- leaving workers 1-5 with no grandfathering guard at all.
DO $$
DECLARE
  gm_id           INTEGER;
BEGIN
  SELECT id INTO gm_id FROM users WHERE email = 'test_gm@example.com';

  DELETE FROM games WHERE id = 800;

  -- community_id is omitted rather than written as NULL: this row is meant to
  -- look exactly like a game created before communities existed.
  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    800,
    'E2E Test: Legacy Game (no community)',
    'Predates communities. Belongs to no community and must never be blocked by a community ban.',
    'Test',
    gm_id,
    6,
    'recruitment',
    true,
    NOW() - INTERVAL '400 days',
    NOW() - INTERVAL '400 days'
  );

  PERFORM setval('games_id_seq', GREATEST((SELECT MAX(id) FROM games), 1), true);
END $$;

COMMIT;
