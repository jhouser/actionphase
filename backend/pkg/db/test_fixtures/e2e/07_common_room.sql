-- Create Common Room Test Games (Isolated for Parallel E2E Testing)
-- This fixture creates 4 dedicated games for testing Common Room functionality
-- Each game has an active common_room phase and identical structure
-- Games are ISOLATED to prevent test interference when running in parallel
--
-- Game #164: For common-room.spec.ts tests
-- Game #165: For character-mentions.spec.ts tests
-- Game #166: For notification-flow.spec.ts tests
-- Game #167: For character-avatar.spec.ts and misc tests

BEGIN;

-- Delete existing games to prevent duplicates (Python regex offsets these IDs per worker)
DELETE FROM games WHERE id IN (164, 165, 166, 167, 168, 605, 606, 607, 608, 609, 610);

DO $$
DECLARE
  gm_id INTEGER;
  p1_id INTEGER;
  p2_id INTEGER;
  phase165_id INTEGER;
  gm_char165_id INTEGER;
  phase610_id INTEGER;
  char610_p1_id INTEGER;
  char610_p2_id INTEGER;
  post610_root INTEGER;
  post610_l1 INTEGER;
  post610_l2 INTEGER;
  post610_l3 INTEGER;
  post610_l4 INTEGER;
  post610_l5 INTEGER;
  post610_l6 INTEGER;
  post610_l7 INTEGER;
  post610_l8 INTEGER;
  post610_l9 INTEGER;
  post610_l10 INTEGER;
  p3_id INTEGER;
  p4_id INTEGER;
  p5_id INTEGER;
  phase_id INTEGER;
BEGIN
  -- Get user IDs
  SELECT id INTO gm_id FROM users WHERE email = 'test_gm@example.com';
  SELECT id INTO p1_id FROM users WHERE email = 'test_player1@example.com';
  SELECT id INTO p2_id FROM users WHERE email = 'test_player2@example.com';
  SELECT id INTO p3_id FROM users WHERE email = 'test_player3@example.com';
  SELECT id INTO p4_id FROM users WHERE email = 'test_player4@example.com';
  SELECT id INTO p5_id FROM users WHERE email = 'test_player5@example.com';

  -- ============================================
  -- GAME #164: Common Room Posts (common-room.spec.ts)
  -- ============================================
  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    164,
    'E2E Common Room - Posts',
    'Isolated game for common-room.spec.ts E2E tests (post creation and commenting).',
    'Test Framework',
    gm_id,
    5,
    'in_progress',
    true,
    NOW() - INTERVAL '5 days',
    NOW()
  );

  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES
    (164, p1_id, 'player', 'active', NOW() - INTERVAL '4 days'),
    (164, p2_id, 'player', 'active', NOW() - INTERVAL '4 days');

  INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, deadline, is_active, is_published, created_at)
  VALUES (
    164,
    'common_room',
    1,
    'Discussion and Planning',
    'Active common room phase for testing post creation.',
    NOW() - INTERVAL '1 hour',
    NOW() + INTERVAL '23 hours',
    true,
    true,
    NOW() - INTERVAL '1 hour'
  );

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (164, gm_id, 'GM Test Character', 'npc', 'approved', NOW() - INTERVAL '4 days', NOW()),
    (164, p1_id, 'Test Player 1 Character', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW()),
    (164, p2_id, 'Test Player 2 Character', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW());

  RAISE NOTICE 'Created Game #164: E2E Common Room - Posts';

  -- ============================================
  -- GAME #165: Common Room Mentions (character-mentions.spec.ts)
  -- ============================================
  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    165,
    'E2E Common Room - Mentions',
    'Isolated game for character-mentions.spec.ts E2E tests (character mention functionality).',
    'Test Framework',
    gm_id,
    5,
    'in_progress',
    true,
    NOW() - INTERVAL '5 days',
    NOW()
  );

  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES
    (165, p1_id, 'player', 'active', NOW() - INTERVAL '4 days'),
    (165, p2_id, 'player', 'active', NOW() - INTERVAL '4 days');

  INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, deadline, is_active, is_published, created_at)
  VALUES (
    165,
    'common_room',
    1,
    'Discussion and Planning',
    'Active common room phase for testing character mentions.',
    NOW() - INTERVAL '1 hour',
    NOW() + INTERVAL '23 hours',
    true,
    true,
    NOW() - INTERVAL '1 hour'
  )
  RETURNING id INTO phase165_id;

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES (165, gm_id, 'GM Test Character', 'npc', 'approved', NOW() - INTERVAL '4 days', NOW())
  RETURNING id INTO gm_char165_id;

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (165, p1_id, 'Test Player 1 Character', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW()),
    (165, p2_id, 'Test Player 2 Character', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW());

  -- Pre-seed a GM post so mention tests don't need to create one at runtime
  INSERT INTO messages (game_id, phase_id, author_id, character_id, content, message_type, visibility, mentioned_character_ids, created_at)
  VALUES (165, phase165_id, gm_id, gm_char165_id, 'Mission Briefing: Everyone report in!', 'post', 'game', '{}', NOW() - INTERVAL '30 minutes');

  RAISE NOTICE 'Created Game #165: E2E Common Room - Mentions';

  -- ============================================
  -- GAME #166: Common Room Notifications (notification-flow.spec.ts)
  -- ============================================
  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    166,
    'E2E Common Room - Notifications',
    'Isolated game for notification-flow.spec.ts E2E tests (notification functionality).',
    'Test Framework',
    gm_id,
    5,
    'in_progress',
    true,
    NOW() - INTERVAL '5 days',
    NOW()
  );

  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES
    (166, p1_id, 'player', 'active', NOW() - INTERVAL '4 days'),
    (166, p2_id, 'player', 'active', NOW() - INTERVAL '4 days'),
    (166, p3_id, 'player', 'active', NOW() - INTERVAL '4 days'),
    (166, p4_id, 'player', 'active', NOW() - INTERVAL '4 days');

  INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, deadline, is_active, is_published, created_at)
  VALUES (
    166,
    'common_room',
    1,
    'Discussion and Planning',
    'Active common room phase for testing notifications.',
    NOW() - INTERVAL '1 hour',
    NOW() + INTERVAL '23 hours',
    true,
    true,
    NOW() - INTERVAL '1 hour'
  );

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (166, gm_id, 'GM Test Character', 'npc', 'approved', NOW() - INTERVAL '4 days', NOW()),
    (166, p1_id, 'Test Player 1 Character', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW()),
    (166, p2_id, 'Test Player 2 Character', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW()),
    (166, p3_id, 'Test Player 3 Character', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW()),
    (166, p4_id, 'Test Player 4 Character', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW());

  RAISE NOTICE 'Created Game #166: E2E Common Room - Notifications';

  -- ============================================
  -- GAME #167: Common Room Misc (character-avatar.spec.ts and other tests)
  -- ============================================
  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    167,
    'E2E Common Room - Misc',
    'Isolated game for character-avatar.spec.ts and other miscellaneous E2E tests.',
    'Test Framework',
    gm_id,
    5,
    'in_progress',
    true,
    NOW() - INTERVAL '5 days',
    NOW()
  );

  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES
    (167, p5_id, 'player', 'active', NOW() - INTERVAL '4 days');

  INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, deadline, is_active, is_published, created_at)
  VALUES (
    167,
    'common_room',
    1,
    'Discussion and Planning',
    'Active common room phase for miscellaneous tests.',
    NOW() - INTERVAL '1 hour',
    NOW() + INTERVAL '23 hours',
    true,
    true,
    NOW() - INTERVAL '1 hour'
  );

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (167, gm_id, 'GM Test Character', 'npc', 'approved', NOW() - INTERVAL '4 days', NOW()),
    (167, p5_id, 'Test Player 5 Character', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW());

  -- Unassigned NPCs (user_id=NULL kept separate so the worker ID-offset script handles it correctly)
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES (167, NULL, 'Mysterious Stranger', 'npc', 'approved', NOW() - INTERVAL '4 days', NOW());
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES (167, NULL, 'Town Guard', 'npc', 'approved', NOW() - INTERVAL '4 days', NOW());

  RAISE NOTICE 'Created Game #167: E2E Common Room - Misc';

  -- ============================================
  -- GAME #168: Character Avatars (character-avatar.spec.ts)
  -- ============================================
  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    168,
    'E2E Character Avatars',
    'Dedicated game for character-avatar.spec.ts E2E tests (avatar upload, delete, permissions).',
    'Test Framework',
    gm_id,
    5,
    'in_progress',
    true,
    NOW() - INTERVAL '5 days',
    NOW()
  );

  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES
    (168, p1_id, 'player', 'active', NOW() - INTERVAL '4 days'),
    (168, p2_id, 'player', 'active', NOW() - INTERVAL '4 days'),
    (168, p3_id, 'player', 'active', NOW() - INTERVAL '4 days'),
    (168, p4_id, 'player', 'active', NOW() - INTERVAL '4 days');

  INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, deadline, is_active, is_published, created_at)
  VALUES (
    168,
    'common_room',
    1,
    'Discussion and Planning',
    'Active common room phase for character avatar testing.',
    NOW() - INTERVAL '1 hour',
    NOW() + INTERVAL '23 hours',
    true,
    true,
    NOW() - INTERVAL '1 hour'
  );

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (168, gm_id, 'GM Test Character', 'npc', 'approved', NOW() - INTERVAL '4 days', NOW()),
    (168, p1_id, 'E2E Test Char 1', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW()),
    (168, p2_id, 'E2E Test Char 2', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW()),
    (168, p3_id, 'E2E Test Char 3', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW()),
    (168, p4_id, 'E2E Test Char 4', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW());

  RAISE NOTICE 'Created Game #168: E2E Character Avatars';

  -- ============================================
  -- ISOLATED GAMES FOR common-room.spec.ts PARALLEL TESTING (605-610)
  -- ============================================
  -- Each test in common-room.spec.ts needs its own game to avoid race conditions

  -- Game #605: "GM can create a post in Common Room"
  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (605, 'E2E Common Room - Create Post', 'Test GM creating posts.', 'Test', gm_id, 5, 'in_progress', true, NOW() - INTERVAL '5 days', NOW());
  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES (605, p1_id, 'player', 'active', NOW() - INTERVAL '4 days'), (605, p2_id, 'player', 'active', NOW() - INTERVAL '4 days');
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, deadline, is_active, is_published, created_at)
  VALUES (605, 'common_room', 1, 'Discussion', 'Common room for testing.', NOW() - INTERVAL '1 hour', NOW() + INTERVAL '23 hours', true, true, NOW() - INTERVAL '1 hour');
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES (605, gm_id, 'GM', 'npc', 'approved', NOW() - INTERVAL '4 days', NOW()), (605, p1_id, 'Player 1', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW()), (605, p2_id, 'Player 2', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW());

  -- Game #606: "Player can view GM posts in Common Room"
  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (606, 'E2E Common Room - View Posts', 'Test player viewing GM posts.', 'Test', gm_id, 5, 'in_progress', true, NOW() - INTERVAL '5 days', NOW());
  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES (606, p1_id, 'player', 'active', NOW() - INTERVAL '4 days'), (606, p2_id, 'player', 'active', NOW() - INTERVAL '4 days');
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, deadline, is_active, is_published, created_at)
  VALUES (606, 'common_room', 1, 'Discussion', 'Common room for testing.', NOW() - INTERVAL '1 hour', NOW() + INTERVAL '23 hours', true, true, NOW() - INTERVAL '1 hour');
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES (606, gm_id, 'GM', 'npc', 'approved', NOW() - INTERVAL '4 days', NOW()), (606, p1_id, 'Player 1', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW()), (606, p2_id, 'Player 2', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW());
  SELECT id INTO phase_id FROM game_phases WHERE game_id = 606 LIMIT 1;
  INSERT INTO messages (game_id, phase_id, author_id, character_id, content, message_type, visibility, mentioned_character_ids, created_at)
  VALUES (606, phase_id, gm_id, (SELECT id FROM characters WHERE game_id = 606 AND user_id = gm_id LIMIT 1), 'Mission update for all crew', 'post', 'game', '{}', NOW() - INTERVAL '30 minutes');

  -- Game #607: "Player can comment on GM post"
  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (607, 'E2E Common Room - Comment', 'Test player commenting on posts.', 'Test', gm_id, 5, 'in_progress', true, NOW() - INTERVAL '5 days', NOW());
  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES (607, p1_id, 'player', 'active', NOW() - INTERVAL '4 days'), (607, p2_id, 'player', 'active', NOW() - INTERVAL '4 days');
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, deadline, is_active, is_published, created_at)
  VALUES (607, 'common_room', 1, 'Discussion', 'Common room for testing.', NOW() - INTERVAL '1 hour', NOW() + INTERVAL '23 hours', true, true, NOW() - INTERVAL '1 hour');
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES (607, gm_id, 'GM', 'npc', 'approved', NOW() - INTERVAL '4 days', NOW()), (607, p1_id, 'Player 1', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW()), (607, p2_id, 'Player 2', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW());
  SELECT id INTO phase_id FROM game_phases WHERE game_id = 607 LIMIT 1;
  INSERT INTO messages (game_id, phase_id, author_id, character_id, content, message_type, visibility, mentioned_character_ids, created_at)
  VALUES (607, phase_id, gm_id, (SELECT id FROM characters WHERE game_id = 607 AND user_id = gm_id LIMIT 1), 'Let''s plan our approach', 'post', 'game', '{}', NOW() - INTERVAL '30 minutes');

  -- Game #608: "Players can reply to each others comments (nested replies)"
  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (608, 'E2E Common Room - Nested Replies', 'Test nested comment replies.', 'Test', gm_id, 5, 'in_progress', true, NOW() - INTERVAL '5 days', NOW());
  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES (608, p1_id, 'player', 'active', NOW() - INTERVAL '4 days'), (608, p2_id, 'player', 'active', NOW() - INTERVAL '4 days');
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, deadline, is_active, is_published, created_at)
  VALUES (608, 'common_room', 1, 'Discussion', 'Common room for testing.', NOW() - INTERVAL '1 hour', NOW() + INTERVAL '23 hours', true, true, NOW() - INTERVAL '1 hour');
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES (608, gm_id, 'GM', 'npc', 'approved', NOW() - INTERVAL '4 days', NOW()), (608, p1_id, 'Player 1', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW()), (608, p2_id, 'Player 2', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW());
  SELECT id INTO phase_id FROM game_phases WHERE game_id = 608 LIMIT 1;
  INSERT INTO messages (game_id, phase_id, author_id, character_id, content, message_type, visibility, mentioned_character_ids, created_at)
  VALUES (608, phase_id, gm_id, (SELECT id FROM characters WHERE game_id = 608 AND user_id = gm_id LIMIT 1), 'What should we do next?', 'post', 'game', '{}', NOW() - INTERVAL '30 minutes');

  -- Game #609: "Multiple players can reply to the same comment"
  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (609, 'E2E Common Room - Multiple Replies', 'Test multiple players replying.', 'Test', gm_id, 5, 'in_progress', true, NOW() - INTERVAL '5 days', NOW());
  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES (609, p1_id, 'player', 'active', NOW() - INTERVAL '4 days'), (609, p2_id, 'player', 'active', NOW() - INTERVAL '4 days');
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, deadline, is_active, is_published, created_at)
  VALUES (609, 'common_room', 1, 'Discussion', 'Common room for testing.', NOW() - INTERVAL '1 hour', NOW() + INTERVAL '23 hours', true, true, NOW() - INTERVAL '1 hour');
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES (609, gm_id, 'GM', 'npc', 'approved', NOW() - INTERVAL '4 days', NOW()), (609, p1_id, 'Player 1', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW()), (609, p2_id, 'Player 2', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW());
  SELECT id INTO phase_id FROM game_phases WHERE game_id = 609 LIMIT 1;
  INSERT INTO messages (game_id, phase_id, author_id, character_id, content, message_type, visibility, mentioned_character_ids, created_at)
  VALUES (609, phase_id, gm_id, (SELECT id FROM characters WHERE game_id = 609 AND user_id = gm_id LIMIT 1), 'Who wants to go north?', 'post', 'game', '{}', NOW() - INTERVAL '30 minutes');

  -- Game #610: "Deep nesting shows Continue this thread button at max depth"
  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (610, 'E2E Common Room - Deep Nesting', 'Test deep nested comments.', 'Test', gm_id, 5, 'in_progress', true, NOW() - INTERVAL '5 days', NOW());
  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES (610, p1_id, 'player', 'active', NOW() - INTERVAL '4 days'), (610, p2_id, 'player', 'active', NOW() - INTERVAL '4 days');
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, deadline, is_active, is_published, created_at)
  VALUES (610, 'common_room', 1, 'Discussion', 'Common room for testing.', NOW() - INTERVAL '1 hour', NOW() + INTERVAL '23 hours', true, true, NOW() - INTERVAL '1 hour');
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES (610, gm_id, 'GM', 'npc', 'approved', NOW() - INTERVAL '4 days', NOW()), (610, p1_id, 'Player 1', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW()), (610, p2_id, 'Player 2', 'player_character', 'approved', NOW() - INTERVAL '4 days', NOW());

  -- Capture IDs needed for pre-created nested post chain
  SELECT id INTO phase610_id FROM game_phases WHERE game_id = 610 LIMIT 1;
  SELECT id INTO char610_p1_id FROM characters WHERE game_id = 610 AND user_id = p1_id LIMIT 1;
  SELECT id INTO char610_p2_id FROM characters WHERE game_id = 610 AND user_id = p2_id LIMIT 1;

  -- Pre-create an 11-level nested comment chain in the messages table so the deep-nesting
  -- test only needs to navigate and assert, not create data at runtime.
  -- Root post (GM post, message_type='post', visibility='game')
  INSERT INTO messages (game_id, phase_id, author_id, character_id, content, message_type, visibility, mentioned_character_ids, created_at)
  VALUES (610, phase610_id, gm_id, (SELECT id FROM characters WHERE game_id = 610 AND user_id = gm_id LIMIT 1), 'Deep Thread Test Post', 'post', 'game', '{}', NOW() - INTERVAL '7 days')
  RETURNING id INTO post610_root;

  -- Level 1 comment (Player 1)
  INSERT INTO messages (game_id, phase_id, author_id, character_id, content, message_type, parent_id, visibility, mentioned_character_ids, created_at)
  VALUES (610, phase610_id, p1_id, char610_p1_id, 'Nested Reply Level 1', 'comment', post610_root, 'game', '{}', NOW() - INTERVAL '6 days')
  RETURNING id INTO post610_l1;

  -- Level 2 (Player 2)
  INSERT INTO messages (game_id, phase_id, author_id, character_id, content, message_type, parent_id, visibility, mentioned_character_ids, created_at)
  VALUES (610, phase610_id, p2_id, char610_p2_id, 'Nested Reply Level 2', 'comment', post610_l1, 'game', '{}', NOW() - INTERVAL '5 days')
  RETURNING id INTO post610_l2;

  -- Level 3 (Player 1)
  INSERT INTO messages (game_id, phase_id, author_id, character_id, content, message_type, parent_id, visibility, mentioned_character_ids, created_at)
  VALUES (610, phase610_id, p1_id, char610_p1_id, 'Nested Reply Level 3', 'comment', post610_l2, 'game', '{}', NOW() - INTERVAL '4 days')
  RETURNING id INTO post610_l3;

  -- Level 4 (Player 2)
  INSERT INTO messages (game_id, phase_id, author_id, character_id, content, message_type, parent_id, visibility, mentioned_character_ids, created_at)
  VALUES (610, phase610_id, p2_id, char610_p2_id, 'Nested Reply Level 4', 'comment', post610_l3, 'game', '{}', NOW() - INTERVAL '3 days')
  RETURNING id INTO post610_l4;

  -- Level 5 (Player 1)
  INSERT INTO messages (game_id, phase_id, author_id, character_id, content, message_type, parent_id, visibility, mentioned_character_ids, created_at)
  VALUES (610, phase610_id, p1_id, char610_p1_id, 'Nested Reply Level 5', 'comment', post610_l4, 'game', '{}', NOW() - INTERVAL '2 days')
  RETURNING id INTO post610_l5;

  -- Level 6 (Player 2)
  INSERT INTO messages (game_id, phase_id, author_id, character_id, content, message_type, parent_id, visibility, mentioned_character_ids, created_at)
  VALUES (610, phase610_id, p2_id, char610_p2_id, 'Nested Reply Level 6', 'comment', post610_l5, 'game', '{}', NOW() - INTERVAL '1 day')
  RETURNING id INTO post610_l6;

  -- Levels 7-11 extend the chain past the deepest limit COMMENT_MAX_DEPTH allows
  -- (validation caps it at 10). The deep-nesting specs derive their targets from
  -- the configured depth (see e2e/fixtures/comment-depth.ts), so the chain must
  -- stay deeper than any valid setting: at maxDepth = N the spec asserts that
  -- "Level N-1" renders inline and "Level N" hides behind "Continue this thread".
  -- A chain of 11 leaves a hidden reply even at the maximum of 10.
  INSERT INTO messages (game_id, phase_id, author_id, character_id, content, message_type, parent_id, visibility, mentioned_character_ids, created_at)
  VALUES (610, phase610_id, p1_id, char610_p1_id, 'Nested Reply Level 7', 'comment', post610_l6, 'game', '{}', NOW() - INTERVAL '22 hours')
  RETURNING id INTO post610_l7;

  INSERT INTO messages (game_id, phase_id, author_id, character_id, content, message_type, parent_id, visibility, mentioned_character_ids, created_at)
  VALUES (610, phase610_id, p2_id, char610_p2_id, 'Nested Reply Level 8', 'comment', post610_l7, 'game', '{}', NOW() - INTERVAL '20 hours')
  RETURNING id INTO post610_l8;

  INSERT INTO messages (game_id, phase_id, author_id, character_id, content, message_type, parent_id, visibility, mentioned_character_ids, created_at)
  VALUES (610, phase610_id, p1_id, char610_p1_id, 'Nested Reply Level 9', 'comment', post610_l8, 'game', '{}', NOW() - INTERVAL '18 hours')
  RETURNING id INTO post610_l9;

  INSERT INTO messages (game_id, phase_id, author_id, character_id, content, message_type, parent_id, visibility, mentioned_character_ids, created_at)
  VALUES (610, phase610_id, p2_id, char610_p2_id, 'Nested Reply Level 10', 'comment', post610_l9, 'game', '{}', NOW() - INTERVAL '16 hours')
  RETURNING id INTO post610_l10;

  INSERT INTO messages (game_id, phase_id, author_id, character_id, content, message_type, parent_id, visibility, mentioned_character_ids, created_at)
  VALUES (610, phase610_id, p1_id, char610_p1_id, 'Nested Reply Level 11', 'comment', post610_l10, 'game', '{}', NOW() - INTERVAL '14 hours');

  -- ============================================
  -- Summary
  -- ============================================
  RAISE NOTICE 'Created 5 isolated Common Room test games (164-168) for parallel E2E testing';
  RAISE NOTICE 'Created 6 isolated games (605-610) for common-room.spec.ts parallel testing';

END $$;

-- Reset the games sequence to prevent duplicate key errors
-- This ensures new game creations don't collide with hardcoded fixture IDs
SELECT setval('games_id_seq', (SELECT MAX(id) FROM games) + 1);

COMMIT;
