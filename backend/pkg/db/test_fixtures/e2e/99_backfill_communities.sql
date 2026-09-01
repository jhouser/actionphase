-- Assign this worker's E2E games to this worker's communities, except the one
-- deliberate holdout.
--
-- Done as a backfill rather than a community_id column on each of the ~90
-- INSERT INTO games statements spread across ~54 fixture files: adding the
-- column to each invites a miss, and a missed game would silently exercise the
-- grandfathering path instead of the ban paths without anyone noticing.
--
-- 🔴 WORKER ISOLATION. apply_e2e_worker.sh runs this file once per worker, so
-- it must touch only the games belonging to the worker it is running as --
-- matched by GM, since the GM's email was rewritten to test_gm_N@. A blanket
-- "all games with a NULL community" would let worker 0 hand its games to
-- worker 0's communities and then let worker 1 do nothing, and the final
-- assertion would fail against games that are not this worker's to assign.

BEGIN;

DO $$
DECLARE
  ravens_id      INTEGER;
  harbor_id      INTEGER;
  gm_id          INTEGER;
  -- The loader rewrites this specific declaration name to the worker's offset
  -- (0, 10000, 20000...). Deriving the holdout's ID from it is the only
  -- reliable way to name "this worker's legacy game": a bare 800 would refer to
  -- worker 0's game from every worker, so workers 1-5 would sail past the
  -- exclusion below and assign their own guard game a community -- which is
  -- exactly what happened before this was fixed.
  worker_game_id_offset INTEGER := 0;
  legacy_game_id INTEGER;
  suffix         TEXT := '';
  unassigned     INTEGER;
BEGIN
  legacy_game_id := 800 + worker_game_id_offset;

  SELECT COALESCE(substring('test_gm@example.com' FROM 'test_gm_([0-9])@'), '')
    INTO suffix;
  IF suffix <> '' THEN
    suffix := '-w' || suffix;
  END IF;

  SELECT id INTO ravens_id FROM communities WHERE slug = 'midnight-ravens' || suffix;
  SELECT id INTO harbor_id FROM communities WHERE slug = 'harbor-lights'   || suffix;
  SELECT id INTO gm_id     FROM users       WHERE email = 'test_gm@example.com';

  -- This worker's GM games -> the community that GM owns.
  UPDATE games SET community_id = ravens_id
  WHERE community_id IS NULL
    AND gm_user_id = gm_id
    AND id <> legacy_game_id;

  -- This worker's other games (player-run, e2e worker users) -> Harbor Lights.
  UPDATE games SET community_id = harbor_id
  WHERE community_id IS NULL
    AND id <> legacy_game_id
    AND gm_user_id IN (
      SELECT id FROM users
      WHERE email LIKE 'test_%' || COALESCE(NULLIF(replace(suffix, '-w', '_'), ''), '') || '@example.com'
         OR username LIKE 'e2euser_%'
    );

  -- Every E2E game is GM-run, so the rule above leaves Harbor Lights empty --
  -- and the cross-community ban scenario has nothing to act on. TestAudience is
  -- banned from Harbor Lights but NOT from Midnight Ravens, so proving the ban
  -- does not leak needs one game in each.
  --
  -- 341 is chosen because it is a read-only "public applicant list" fixture:
  -- the application submit/approve/reject games (329-333) are the ones existing
  -- specs mutate, and moving those between communities risks disturbing them.
  UPDATE games SET community_id = harbor_id
  WHERE community_id = ravens_id
    AND id = 341 + worker_game_id_offset;

  -- The holdout must have survived. If a future fixture assigns it a community,
  -- the grandfathering path silently loses its only coverage -- so fail here
  -- rather than let that happen quietly.
  IF EXISTS (SELECT 1 FROM games WHERE id = legacy_game_id AND community_id IS NOT NULL) THEN
    RAISE EXCEPTION
      'game % is the grandfathering guard and must keep community_id IS NULL', legacy_game_id;
  END IF;

  -- Nothing of THIS worker's may be left NULL: an unassigned game exercises the
  -- legacy path instead of the ban paths it was meant to cover. Scoped to this
  -- worker's GM, since other workers' games are not this run's to assign.
  SELECT COUNT(*) INTO unassigned
  FROM games
  WHERE community_id IS NULL
    AND id <> legacy_game_id
    AND gm_user_id = gm_id;

  IF unassigned > 0 THEN
    RAISE EXCEPTION 'e2e backfill left % game(s) with no community for %',
      unassigned, 'test_gm@example.com';
  END IF;
END $$;

COMMIT;
