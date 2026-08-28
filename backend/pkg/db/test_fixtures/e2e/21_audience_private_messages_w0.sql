-- E2E Test Fixture: Audience Private Messages View (Worker 0)
--
-- Purpose: Pre-seeded conversations for audience-private-messages.spec.ts
-- Game #360 has two conversations with messages from different senders so
-- audience view tests don't need to create conversations at runtime.
--
-- Conversations:
--   ID 9960: "Audience Test Conversation" — 2 messages from Char 1
--   ID 9961: "Preview Test Conversation"  — 1 message ("Last message preview text")
--
-- Game IDs: 360. Unlike transformed fixtures, the 21_* worker files are
-- PRE-OFFSET: each _wN file hardcodes its own IDs (w5 = 50360/59960/59961) and
-- is applied verbatim, with no rewriting by apply_e2e_worker.sh.

BEGIN;

DELETE FROM games WHERE id = 360;

DO $$
DECLARE
  gm_id       INTEGER;
  p1_id       INTEGER;
  p2_id       INTEGER;
  aud_id      INTEGER;
  phase_id    INTEGER;
  char1_id    INTEGER;
  char2_id    INTEGER;
BEGIN
  SELECT id INTO gm_id  FROM users WHERE email = 'test_gm@example.com';
  SELECT id INTO p1_id  FROM users WHERE email = 'test_player1@example.com';
  SELECT id INTO p2_id  FROM users WHERE email = 'test_player2@example.com';
  SELECT id INTO aud_id FROM users WHERE email = 'test_audience@example.com';

  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    360,
    'E2E Test: Audience Private Messages',
    'Isolated game for audience-private-messages.spec.ts',
    'Test Framework',
    gm_id, 5, 'in_progress', true,
    NOW() - INTERVAL '5 days', NOW()
  );

  INSERT INTO game_participants (game_id, user_id, role, status, joined_at) VALUES
    (360, p1_id,  'player',   'active', NOW() - INTERVAL '4 days'),
    (360, p2_id,  'player',   'active', NOW() - INTERVAL '4 days'),
    (360, aud_id, 'audience', 'active', NOW() - INTERVAL '4 days');

  INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, deadline, is_active, is_published, created_at)
  VALUES (360, 'common_room', 1, 'Current Phase', 'Active phase', NOW() - INTERVAL '1 hour', NOW() + INTERVAL '23 hours', true, false, NOW() - INTERVAL '1 hour')
  RETURNING id INTO phase_id;

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES (360, p1_id, 'Audience Test Char 1', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW())
  RETURNING id INTO char1_id;

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES (360, p2_id, 'Audience Test Char 2', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW())
  RETURNING id INTO char2_id;

  -- Conversation 1: two messages from Char 1 then one reply from Char 2
  INSERT INTO conversations (id, game_id, title, conversation_type, created_by_user_id, created_at)
  VALUES (9960, 360, 'Audience Test Conversation', 'direct', p1_id, NOW() - INTERVAL '30 minutes')
  ON CONFLICT (id) DO UPDATE SET title = EXCLUDED.title, game_id = EXCLUDED.game_id;

  INSERT INTO conversation_participants (conversation_id, user_id, character_id, joined_at) VALUES
    (9960, p1_id, char1_id, NOW() - INTERVAL '30 minutes'),
    (9960, p2_id, char2_id, NOW() - INTERVAL '30 minutes')
  ON CONFLICT (conversation_id, user_id, character_id) DO NOTHING;

  -- Used by: test 1 (enhanced UI), test 3 (message grouping + date dividers)
  INSERT INTO private_messages (id, conversation_id, sender_user_id, sender_character_id, content, created_at, is_deleted)
  VALUES
    (99601, 9960, p1_id, char1_id, 'First message from Player 1',  NOW() - INTERVAL '25 minutes', false),
    (99602, 9960, p1_id, char1_id, 'Second message from Player 1', NOW() - INTERVAL '24 minutes', false),
    (99603, 9960, p2_id, char2_id, 'Player 2 reply',               NOW() - INTERVAL '20 minutes', false)
  ON CONFLICT (id) DO UPDATE SET content = EXCLUDED.content;

  -- Conversation 2: last message preview test
  -- Used by: test 5 (last message preview on conversation cards)
  INSERT INTO conversations (id, game_id, title, conversation_type, created_by_user_id, created_at)
  VALUES (9961, 360, 'Preview Test Conversation', 'direct', p1_id, NOW() - INTERVAL '15 minutes')
  ON CONFLICT (id) DO UPDATE SET title = EXCLUDED.title, game_id = EXCLUDED.game_id;

  INSERT INTO conversation_participants (conversation_id, user_id, character_id, joined_at) VALUES
    (9961, p1_id, char1_id, NOW() - INTERVAL '15 minutes'),
    (9961, p2_id, char2_id, NOW() - INTERVAL '15 minutes')
  ON CONFLICT (conversation_id, user_id, character_id) DO NOTHING;

  INSERT INTO private_messages (id, conversation_id, sender_user_id, sender_character_id, content, created_at, is_deleted)
  VALUES (99611, 9961, p1_id, char1_id, 'Last message preview text', NOW() - INTERVAL '10 minutes', false)
  ON CONFLICT (id) DO UPDATE SET content = EXCLUDED.content;

  RAISE NOTICE 'Created Game #360: Audience Private Messages fixture (worker 0)';
END $$;

SELECT setval('games_id_seq', (SELECT MAX(id) FROM games) + 1);

COMMIT;
