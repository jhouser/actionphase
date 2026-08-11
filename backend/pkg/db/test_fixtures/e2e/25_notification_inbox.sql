-- E2E Test Fixture: Dashboard Unread Inbox
-- Purpose: Isolated game for unread-inbox.spec.ts.
--   - Game 706: dashboard inbox inline-reply test
--     Player 1 and Player 2 participate with named characters.
--     Pre-seeded: a GM post + a Player 1 comment, so the test's setup step is
--     just "Player 2 replies" — the reply itself generates the comment_reply
--     notification the inbox is expected to pick up.
--
--     This test MUTATES state (it marks a notification read and posts a
--     comment), so it gets its own game rather than sharing #704 with
--     notification-flow.spec.ts.
--
--     Deliberately NOT pre-seeding the notification row: the point of the test
--     is that a real reply produces a notification whose title/related_id the
--     client can parse back into a repliable inbox item. A hand-written
--     notification row would bypass exactly the backend contract under test.
-- Game ID: 706 (offset by worker via apply_e2e_worker.sh transformation)
-- IDEMPOTENT: Safe to run multiple times

BEGIN;

DELETE FROM games WHERE id = 706;

DO $$
DECLARE
  game_id    INTEGER := 706;
  gm_id      INTEGER;
  p1_id      INTEGER;
  p2_id      INTEGER;
  phase_id   INTEGER;
  post_id    INTEGER;
  gm_char_id INTEGER;
  p1_char_id INTEGER;
  p2_char_id INTEGER;
BEGIN
  SELECT id INTO gm_id FROM users WHERE email = 'test_gm@example.com';
  SELECT id INTO p1_id FROM users WHERE email = 'test_player1@example.com';
  SELECT id INTO p2_id FROM users WHERE email = 'test_player2@example.com';

  INSERT INTO games (
    id, title, description, genre, gm_user_id, max_players,
    state, is_public, created_at, updated_at
  ) VALUES (
    706,
    'E2E Test: Unread Inbox',
    'Stable fixture for the dashboard unread inbox E2E test.',
    'Test',
    gm_id,
    6,
    'in_progress',
    true,
    NOW() - INTERVAL '7 days',
    NOW()
  );

  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES
    (706, p1_id, 'player', 'active', NOW() - INTERVAL '7 days'),
    (706, p2_id, 'player', 'active', NOW() - INTERVAL '7 days');

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES (706, gm_id, 'GM Character', 'npc', 'approved', NOW() - INTERVAL '7 days', NOW())
  RETURNING id INTO gm_char_id;

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES (706, p1_id, 'Inbox Char 1', 'player_character', 'approved', NOW() - INTERVAL '7 days', NOW())
  RETURNING id INTO p1_char_id;

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES (706, p2_id, 'Inbox Char 2', 'player_character', 'approved', NOW() - INTERVAL '7 days', NOW())
  RETURNING id INTO p2_char_id;

  -- Active common_room phase
  INSERT INTO game_phases (
    game_id, phase_type, phase_number, title, description,
    start_time, deadline, is_active, is_published, created_at
  ) VALUES (
    706, 'common_room', 1, 'Discussion', 'Common room for the inbox test.',
    NOW() - INTERVAL '6 days', NOW() + INTERVAL '30 days',
    true, true, NOW() - INTERVAL '6 days'
  ) RETURNING id INTO phase_id;

  -- Pre-seeded GM post.
  -- character_avatar_url_at_post is set from the character to mirror the
  -- production write path (CreatePost in queries/messages.sql), which always
  -- populates it via a subquery rather than leaving it NULL.
  INSERT INTO messages (
    game_id, phase_id, author_id, character_id,
    content, message_type, visibility, mentioned_character_ids,
    character_avatar_url_at_post, created_at
  ) VALUES (
    706, phase_id, gm_id, gm_char_id,
    'Inbox Test Post',
    'post', 'game', '{}',
    (SELECT avatar_url FROM characters WHERE id = gm_char_id),
    NOW() - INTERVAL '5 days'
  ) RETURNING id INTO post_id;

  -- Pre-seeded Player 1 comment on the post (Player 2 replies to this at runtime)
  INSERT INTO messages (
    game_id, phase_id, author_id, character_id,
    content, message_type, parent_id, visibility, mentioned_character_ids,
    character_avatar_url_at_post, created_at
  ) VALUES (
    706, phase_id, p1_id, p1_char_id,
    'Player 1 comment on inbox test post',
    'comment', post_id, 'game', '{}',
    (SELECT avatar_url FROM characters WHERE id = p1_char_id),
    NOW() - INTERVAL '4 days'
  );

  RAISE NOTICE 'Unread Inbox fixture created: Game #706';
END $$;

SELECT setval('games_id_seq', (SELECT MAX(id) FROM games) + 1);

COMMIT;
