-- Per-spec communities for the community E2E specs.
--
-- WHY THESE EXIST
--
-- 00_communities.sql defines the three SHARED communities (Midnight Ravens,
-- Harbor Lights, The Long Road). They are read by many specs and by the game
-- fixtures, which makes them the wrong place to mutate: a spec that renames a
-- community, promotes a moderator, or issues a ban changes state that another
-- spec is concurrently asserting on. Playwright shards by FILE, so those specs
-- genuinely run at the same time against the same worker's data.
--
-- Two real collisions came from exactly that before these existed:
--   * one spec promoted TestPlayer4 to moderator while another asserted
--     TestPlayer4 was an ordinary member with no controls;
--   * one spec renamed Midnight Ravens and restored it moments later, while
--     another asserted the community card still read "Midnight Ravens".
--
-- Restoring state afterwards cannot fix either one: there is always a window
-- where the value is wrong, and a concurrent reader can land in it.
--
-- So each MUTATING spec gets a community nothing else reads:
--
--   e2e-roster-{suffix}     -- community-moderators.spec.ts. Add/remove
--                              moderators freely; no other spec reads its
--                              roster. Owned by TestGM, TestPlayer1 moderates.
--   e2e-modtools-{suffix}   -- community-moderation.spec.ts. Bans, documents,
--                              and RENAMES. Nothing else asserts on its name.
--                              Owned by TestGM, TestPlayer1 moderates, and it
--                              carries the same three ban scenarios as the
--                              shared fixture so the expiry/audit cases work.
--
-- The shared three stay exactly as they were: ban ENFORCEMENT against real
-- games still uses Midnight Ravens and Harbor Lights, because those are the
-- communities the game fixtures are assigned to, and enforcement specs only
-- READ community state.
--
-- 🔴 WORKER ISOLATION. Same rule as 00_communities.sql: slugs carry the
-- worker's suffix, derived from the GM's rewritten email. Worker 0 keeps bare
-- slugs.

BEGIN;

DO $$
DECLARE
  gm_id         INTEGER;
  player1_id    INTEGER;
  player3_id    INTEGER;
  player4_id    INTEGER;
  player5_id    INTEGER;
  roster_id     INTEGER;
  modtools_id   INTEGER;
  suffix        TEXT := '';
  roster_slug   TEXT;
  modtools_slug TEXT;
BEGIN
  SELECT id INTO gm_id      FROM users WHERE email = 'test_gm@example.com';
  SELECT id INTO player1_id FROM users WHERE email = 'test_player1@example.com';
  SELECT id INTO player3_id FROM users WHERE email = 'test_player3@example.com';
  SELECT id INTO player4_id FROM users WHERE email = 'test_player4@example.com';
  SELECT id INTO player5_id FROM users WHERE email = 'test_player5@example.com';

  SELECT COALESCE(substring('test_gm@example.com' FROM 'test_gm_([0-9])@'), '')
    INTO suffix;
  IF suffix <> '' THEN
    suffix := '-w' || suffix;
  END IF;

  roster_slug   := 'e2e-roster'   || suffix;
  modtools_slug := 'e2e-modtools' || suffix;

  INSERT INTO communities (name, slug, description, banner_url, owner_user_id, is_active)
  VALUES
    ('E2E Roster Community', roster_slug,
     'Dedicated to moderator roster tests. Safe to add and remove moderators.',
     NULL, gm_id, TRUE),
    ('E2E Mod Tools Community', modtools_slug,
     'Dedicated to ban, document, and settings tests. Safe to rename.',
     NULL, gm_id, TRUE)
  ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name, description = EXCLUDED.description,
    owner_user_id = EXCLUDED.owner_user_id, is_active = EXCLUDED.is_active;

  SELECT id INTO roster_id   FROM communities WHERE slug = roster_slug;
  SELECT id INTO modtools_id FROM communities WHERE slug = modtools_slug;

  -- TestPlayer1 moderates both, so the "moderator can do X" and "moderator
  -- cannot change the roster" cases both have a subject. Ownership stays with
  -- TestGM via communities.owner_user_id -- it is not a moderator row.
  INSERT INTO community_moderators (community_id, user_id, granted_by_user_id)
  VALUES
    (roster_id,   player1_id, gm_id),
    (modtools_id, player1_id, gm_id)
  ON CONFLICT (community_id, user_id) DO NOTHING;

  -- The ban scenarios the moderation spec reads, mirroring 00_communities.sql:
  --   TestPlayer5 -- ACTIVE permanent, must render as "Banned"
  --   TestPlayer4 -- EXPIRED, must render as "Expired" while its row remains.
  --                  This is the guard for "a row's presence never means banned".
  INSERT INTO community_bans (community_id, user_id, reason, banned_by_user_id, banned_at, expires_at)
  VALUES
    (modtools_id, player5_id,
     'Repeated harassment of other players after multiple warnings.',
     gm_id, NOW() - INTERVAL '30 days', NULL),
    (modtools_id, player4_id,
     'Two-week cooldown after an argument in the common room.',
     player1_id, NOW() - INTERVAL '30 days', NOW() - INTERVAL '16 days')
  ON CONFLICT (community_id, user_id) DO UPDATE SET
    reason = EXCLUDED.reason, banned_by_user_id = EXCLUDED.banned_by_user_id,
    banned_at = EXCLUDED.banned_at, expires_at = EXCLUDED.expires_at;

  -- Audit log. TestPlayer3 has NO community_bans row here -- banned, extended,
  -- then lifted. It is what the separate audit table exists for, and it gives
  -- the history view a lifted ban to render.
  INSERT INTO community_ban_events (community_id, target_user_id, actor_user_id, action, reason, expires_at, created_at)
  VALUES
    (modtools_id, player5_id, gm_id, 'banned',
     'Repeated harassment of other players after multiple warnings.', NULL, NOW() - INTERVAL '30 days'),
    (modtools_id, player4_id, player1_id, 'banned',
     'Two-week cooldown after an argument in the common room.', NOW() - INTERVAL '16 days', NOW() - INTERVAL '30 days'),
    (modtools_id, player3_id, gm_id, 'banned',
     'Disputed action result, cooling-off period.', NOW() - INTERVAL '50 days', NOW() - INTERVAL '60 days'),
    (modtools_id, player3_id, player1_id, 'modified',
     'Extended after a second incident.', NOW() - INTERVAL '40 days', NOW() - INTERVAL '55 days'),
    (modtools_id, player3_id, gm_id, 'unbanned',
     'Appeal accepted; matter resolved.', NULL, NOW() - INTERVAL '45 days');

  -- One PUBLISHED document, so a spec can assert that an ordinary member SEES
  -- the community's rules without first logging in as a moderator to publish
  -- one. That second login was the point where the member test kept timing out
  -- under parallel load, and it was setup rather than the behaviour under test.
  --
  -- The moderator-side draft/publish flow is exercised live by the spec that
  -- actually owns it; this row exists purely as the member's read target.
  DELETE FROM community_documents
  WHERE community_id = modtools_id AND title = 'E2E Fixture House Rules';

  INSERT INTO community_documents
    (community_id, title, content, status, sort_order, created_by_user_id)
  VALUES
    (modtools_id, 'E2E Fixture House Rules',
     'Be kind. Post in the common room before acting.',
     'published', 0, player1_id);

END $$;

COMMIT;
