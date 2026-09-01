-- Communities, moderators, and bans for the E2E dataset.
--
-- Deliberately mirrors demo/01b_communities.sql: same three communities, same
-- slugs, same three ban scenarios. Specs that assert on community names should
-- not have to care which dataset is loaded.
--
-- Loaded as 00_ (after common users, before any game fixture) because
-- games.community_id references these rows.
--
-- 🔴 WORKER ISOLATION. apply_e2e_worker.sh runs every file once PER WORKER,
-- rewriting test_gm@ -> test_gm_N@. Communities are keyed by a UNIQUE slug, so
-- a fixed slug would have all six workers fight over the same three rows and
-- the last worker would win -- leaving communities owned by TestGM_5 and every
-- other worker's moderator/ban specs looking at someone else's data.
--
-- The slug therefore carries the worker's suffix, derived from the GM's email
-- (which the worker script has already rewritten) rather than hardcoded.
-- Worker 0 keeps the bare slugs, so specs that do not care about workers can
-- still use /communities/midnight-ravens.

BEGIN;

DO $$
DECLARE
  gm_id        INTEGER;
  player1_id   INTEGER;
  player2_id   INTEGER;
  player3_id   INTEGER;
  player4_id   INTEGER;
  player5_id   INTEGER;
  audience_id  INTEGER;
  ravens_id    INTEGER;
  harbor_id    INTEGER;
  road_id      INTEGER;
  -- '' for worker 0, '-w1'..'-w5' for the rest. Read off the rewritten email
  -- so this file needs no worker parameter of its own.
  suffix       TEXT := '';
  ravens_slug  TEXT;
  harbor_slug  TEXT;
  road_slug    TEXT;
BEGIN
  SELECT id INTO gm_id       FROM users WHERE email = 'test_gm@example.com';
  SELECT id INTO player1_id  FROM users WHERE email = 'test_player1@example.com';
  SELECT id INTO player2_id  FROM users WHERE email = 'test_player2@example.com';
  SELECT id INTO player3_id  FROM users WHERE email = 'test_player3@example.com';
  SELECT id INTO player4_id  FROM users WHERE email = 'test_player4@example.com';
  SELECT id INTO player5_id  FROM users WHERE email = 'test_player5@example.com';
  SELECT id INTO audience_id FROM users WHERE email = 'test_audience@example.com';

  -- The worker script rewrote the literal above to test_gm_N@example.com for
  -- worker N; recover N from it so slugs are unique per worker.
  SELECT COALESCE(substring('test_gm@example.com' FROM 'test_gm_([0-9])@'), '')
    INTO suffix;
  IF suffix <> '' THEN
    suffix := '-w' || suffix;
  END IF;

  ravens_slug := 'midnight-ravens' || suffix;
  harbor_slug := 'harbor-lights'   || suffix;
  road_slug   := 'the-long-road'   || suffix;

  INSERT INTO communities (name, slug, description, banner_url, owner_user_id, is_active)
  VALUES
    ('Midnight Ravens', ravens_slug,
     E'A long-running horror and mystery group.\n\nWe favour **slow burn** investigation over combat.',
     NULL, gm_id, TRUE),
    ('Harbor Lights', harbor_slug,
     'Nautical adventure and exploration games.',
     NULL, player2_id, TRUE),
    -- Inactive: the create-game flow must refuse this one.
    ('The Long Road', road_slug,
     'A retired westmarches campaign. Archived, no longer running new games.',
     NULL, player3_id, FALSE)
  ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name, description = EXCLUDED.description,
    owner_user_id = EXCLUDED.owner_user_id, is_active = EXCLUDED.is_active;

  SELECT id INTO ravens_id FROM communities WHERE slug = ravens_slug;
  SELECT id INTO harbor_id FROM communities WHERE slug = harbor_slug;
  SELECT id INTO road_id   FROM communities WHERE slug = road_slug;

  -- Owner is NOT a moderator row; ownership lives in communities.owner_user_id.
  INSERT INTO community_moderators (community_id, user_id, granted_by_user_id)
  VALUES
    (ravens_id, player1_id, gm_id),
    (road_id,   player4_id, player3_id)
  ON CONFLICT (community_id, user_id) DO NOTHING;

  -- The three ban scenarios E2E exercises:
  --
  --   TestPlayer5  / Midnight Ravens -- ACTIVE permanent. Must be refused at
  --     every join path and at game creation in that community.
  --   TestPlayer4  / Midnight Ravens -- EXPIRED. Must be able to join freely,
  --     while still appearing on the moderator's ban list as inactive. This is
  --     the guard for "a row's presence never means banned".
  --   TestAudience / Harbor Lights   -- ACTIVE, future-dated. Banned here but
  --     must stay free to play in Midnight Ravens: proves bans do not leak
  --     across communities.
  INSERT INTO community_bans (community_id, user_id, reason, banned_by_user_id, banned_at, expires_at)
  VALUES
    (ravens_id, player5_id,
     'Repeated harassment of other players after multiple warnings.',
     gm_id, NOW() - INTERVAL '30 days', NULL),
    (ravens_id, player4_id,
     'Two-week cooldown after an argument in the common room.',
     player1_id, NOW() - INTERVAL '30 days', NOW() - INTERVAL '16 days'),
    (harbor_id, audience_id,
     'Spoiling ongoing games in the audience chat.',
     player2_id, NOW() - INTERVAL '2 days', NOW() + INTERVAL '7 days')
  ON CONFLICT (community_id, user_id) DO UPDATE SET
    reason = EXCLUDED.reason, banned_by_user_id = EXCLUDED.banned_by_user_id,
    banned_at = EXCLUDED.banned_at, expires_at = EXCLUDED.expires_at;

  -- Audit log. The player3 sequence has NO community_bans row -- a ban that was
  -- issued, extended, then lifted. It is what the separate audit table exists
  -- for, and it gives the audit log view a lifted ban to render.
  INSERT INTO community_ban_events (community_id, target_user_id, actor_user_id, action, reason, expires_at, created_at)
  VALUES
    (ravens_id, player5_id, gm_id, 'banned',
     'Repeated harassment of other players after multiple warnings.', NULL, NOW() - INTERVAL '30 days'),
    (ravens_id, player4_id, player1_id, 'banned',
     'Two-week cooldown after an argument in the common room.', NOW() - INTERVAL '16 days', NOW() - INTERVAL '30 days'),
    (harbor_id, audience_id, player2_id, 'banned',
     'Spoiling ongoing games in the audience chat.', NOW() + INTERVAL '7 days', NOW() - INTERVAL '2 days'),
    (ravens_id, player3_id, gm_id, 'banned',
     'Disputed action result, cooling-off period.', NOW() - INTERVAL '50 days', NOW() - INTERVAL '60 days'),
    (ravens_id, player3_id, player1_id, 'modified',
     'Extended after a second incident.', NOW() - INTERVAL '40 days', NOW() - INTERVAL '55 days'),
    (ravens_id, player3_id, gm_id, 'unbanned',
     'Appeal accepted; matter resolved.', NULL, NOW() - INTERVAL '45 days');
END $$;

COMMIT;
