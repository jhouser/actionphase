-- Communities, moderators, and bans for the demo dataset.
--
-- Three communities, mirroring the real deployment this feature was built for:
-- three separate groups share one install, each maintaining its own banlist.
-- One community is INACTIVE so the "not accepting new games" refusal has
-- something to refuse against.
--
-- Loaded as 01b (after users, before games) because games.community_id
-- references these rows and 99_backfill_communities.sql assigns them.

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
BEGIN
  SELECT id INTO gm_id       FROM users WHERE email = 'test_gm@example.com';
  SELECT id INTO player1_id  FROM users WHERE email = 'test_player1@example.com';
  SELECT id INTO player2_id  FROM users WHERE email = 'test_player2@example.com';
  SELECT id INTO player3_id  FROM users WHERE email = 'test_player3@example.com';
  SELECT id INTO player4_id  FROM users WHERE email = 'test_player4@example.com';
  SELECT id INTO player5_id  FROM users WHERE email = 'test_player5@example.com';
  SELECT id INTO audience_id FROM users WHERE email = 'test_audience@example.com';

  -- Column list matches CreateCommunity's write path exactly (name, slug,
  -- description, banner_url, owner_user_id); is_active is set explicitly where
  -- it differs from the default so the inactive case is not an accident.
  INSERT INTO communities (name, slug, description, banner_url, owner_user_id, is_active)
  VALUES
    ('Midnight Ravens', 'midnight-ravens',
     E'A long-running horror and mystery group.\n\nWe favour **slow burn** investigation over combat. New players welcome.',
     NULL, gm_id, TRUE),
    ('Harbor Lights', 'harbor-lights',
     E'Nautical adventure and exploration games.\n\nSee our [guidelines](/communities/harbor-lights/docs) before applying.',
     NULL, player2_id, TRUE),
    -- Inactive: accepts no new games. Gives the create-game flow a community
    -- that must be refused, which an all-active set could never test.
    ('The Long Road', 'the-long-road',
     'A retired westmarches campaign. Archived, no longer running new games.',
     NULL, player3_id, FALSE)
  ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name, description = EXCLUDED.description,
    owner_user_id = EXCLUDED.owner_user_id, is_active = EXCLUDED.is_active;

  SELECT id INTO ravens_id FROM communities WHERE slug = 'midnight-ravens';
  SELECT id INTO harbor_id FROM communities WHERE slug = 'harbor-lights';
  SELECT id INTO road_id   FROM communities WHERE slug = 'the-long-road';

  -- Moderators. The OWNER IS NOT A ROW HERE -- ownership lives in
  -- communities.owner_user_id and outranks moderator. Adding the owner as a
  -- moderator row would misrepresent the two-tier model the UI renders.
  INSERT INTO community_moderators (community_id, user_id, granted_by_user_id)
  VALUES
    (ravens_id, player1_id, gm_id),
    (road_id,   player4_id, player3_id)
  ON CONFLICT (community_id, user_id) DO NOTHING;

  -- Bans. Three scenarios, each guarding a distinct rule:
  --
  --   1. player5 / Midnight Ravens -- ACTIVE, permanent. The core refusal.
  --   2. player4 / Midnight Ravens -- EXPIRED. Guards the rule that the
  --      presence of a row never means "banned": this user must be able to
  --      join freely while still showing on the moderator's list as inactive.
  --      A query that drops the expires_at test enforces this lapsed ban and
  --      fails loudly.
  --   3. audience / Harbor Lights -- ACTIVE, future-dated. Scoped to ONE
  --      community: this user is banned from Harbor Lights but must remain
  --      free to play in Midnight Ravens. Guards against cross-community leak.
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

  -- Audit log. Written alongside each ban in production (one transaction), so
  -- the fixture mirrors that: every ban above has its 'banned' event. The
  -- unbanned/modified pair belongs to a user whose ban was since lifted --
  -- there is deliberately NO community_bans row for them, which is exactly the
  -- case the separate audit table exists to preserve.
  INSERT INTO community_ban_events (community_id, target_user_id, actor_user_id, action, reason, expires_at, created_at)
  VALUES
    (ravens_id, player5_id, gm_id, 'banned',
     'Repeated harassment of other players after multiple warnings.', NULL, NOW() - INTERVAL '30 days'),
    (ravens_id, player4_id, player1_id, 'banned',
     'Two-week cooldown after an argument in the common room.', NOW() - INTERVAL '16 days', NOW() - INTERVAL '30 days'),
    (harbor_id, audience_id, player2_id, 'banned',
     'Spoiling ongoing games in the audience chat.', NOW() + INTERVAL '7 days', NOW() - INTERVAL '2 days'),
    -- A ban that was issued, extended, then lifted. Survives with no
    -- corresponding community_bans row.
    (ravens_id, player3_id, gm_id, 'banned',
     'Disputed action result, cooling-off period.', NOW() - INTERVAL '50 days', NOW() - INTERVAL '60 days'),
    (ravens_id, player3_id, player1_id, 'modified',
     'Extended after a second incident.', NOW() - INTERVAL '40 days', NOW() - INTERVAL '55 days'),
    (ravens_id, player3_id, gm_id, 'unbanned',
     'Appeal accepted; matter resolved.', NULL, NOW() - INTERVAL '45 days');
END $$;

COMMIT;
