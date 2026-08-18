-- Dedicated E2E Test Games
-- These games are specifically for tests that modify state (complete, cancel, etc.)
-- They should be reset between test runs
--
-- IDEMPOTENT: Safe to run multiple times - deletes existing E2E games before recreating
--
-- Game IDs: 350-355 (offset by worker: Worker 1 = 10350-10355, etc.)

BEGIN;

-- Delete existing E2E dedicated games to prevent duplicates
DELETE FROM games WHERE id IN (350, 351, 352, 353, 354, 355);

DO $$
DECLARE
  gm_id INTEGER;
  p1_id INTEGER;
  p2_id INTEGER;
  p3_id INTEGER;
  p4_id INTEGER;
  audience_id INTEGER;
  -- Hardcoded game IDs for worker offset support
  game_complete_id INT := 350;
  game_cancel_id INT := 351;
  game_pause_id INT := 352;
  game_action_id INT := 353;
  game_messages_id INT := 354;
  game_settings_id INT := 355;
  phase_id INTEGER;
  char1_id INTEGER;
  char2_id INTEGER;
  char3_id INTEGER;
  char4_id INTEGER;
  char5_id INTEGER;
  char6_id INTEGER;
  char7_id INTEGER;
BEGIN
  -- Get user IDs
  SELECT id INTO gm_id FROM users WHERE email = 'test_gm@example.com';
  SELECT id INTO p1_id FROM users WHERE email = 'test_player1@example.com';
  SELECT id INTO p2_id FROM users WHERE email = 'test_player2@example.com';
  SELECT id INTO p3_id FROM users WHERE email = 'test_player3@example.com';
  SELECT id INTO p4_id FROM users WHERE email = 'test_player4@example.com';
  SELECT id INTO audience_id FROM users WHERE email = 'test_audience@example.com';

  -- ============================================
  -- GAME #350: For Completion Testing
  -- ============================================
  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    game_complete_id,
    'E2E Test: Game to Complete',
    'This game is dedicated for completion testing in E2E tests.',
    'Test',
    gm_id,
    4,
    'in_progress',
    true,
    NOW() - INTERVAL '5 days',
    NOW()
  );

  -- Add participants
  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES
    (game_complete_id, p1_id, 'player', 'active', NOW() - INTERVAL '4 days'),
    (game_complete_id, p2_id, 'player', 'active', NOW() - INTERVAL '4 days');

  -- Add active phase
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, deadline, is_active, is_published, created_at)
  VALUES (
    game_complete_id,
    'action',
    1,
    'Final Phase',
    'This is the final phase before completion',
    NOW() - INTERVAL '1 hour',
    NOW() + INTERVAL '23 hours',
    true,
    false,
    NOW() - INTERVAL '1 hour'
  );

  -- ============================================
  -- GAME #351: For Cancellation Testing
  -- ============================================
  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    game_cancel_id,
    'E2E Test: Game to Cancel',
    'This game is dedicated for cancellation testing in E2E tests.',
    'Test',
    gm_id,
    4,
    'recruitment',
    true,
    NOW() - INTERVAL '2 days',
    NOW()
  );

  -- Add some pending applications
  INSERT INTO game_applications (game_id, user_id, role, status, applied_at)
  VALUES
    (game_cancel_id, p1_id, 'player', 'pending', NOW() - INTERVAL '1 day'),
    (game_cancel_id, p2_id, 'player', 'pending', NOW() - INTERVAL '1 day');

  -- ============================================
  -- GAME #352: For Pause/Resume Testing
  -- ============================================
  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    game_pause_id,
    'E2E Test: Game to Pause',
    'This game is dedicated for pause/resume testing in E2E tests.',
    'Test',
    gm_id,
    4,
    'in_progress',
    true,
    NOW() - INTERVAL '10 days',
    NOW()
  );

  -- Add participants
  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES
    (game_pause_id, p1_id, 'player', 'active', NOW() - INTERVAL '9 days'),
    (game_pause_id, p2_id, 'player', 'active', NOW() - INTERVAL '9 days'),
    (game_pause_id, p3_id, 'player', 'active', NOW() - INTERVAL '9 days');

  -- Add active phase
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, start_time, deadline, is_active, is_published, created_at)
  VALUES (
    game_pause_id,
    'common_room',
    1,
    'Planning Phase',
    NOW() - INTERVAL '2 hours',
    NOW() + INTERVAL '22 hours',
    true,
    false,
    NOW() - INTERVAL '2 hours'
  );

  -- ============================================
  -- GAME #353: For Action Submission Testing
  -- ============================================
  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    game_action_id,
    'E2E Test: Action Submission',
    'This game is dedicated for action submission testing in E2E tests.',
    'Test',
    gm_id,
    5,
    'in_progress',
    true,
    NOW() - INTERVAL '8 days',
    NOW()
  );

  -- Add participants
  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES
    (game_action_id, p1_id, 'player', 'active', NOW() - INTERVAL '7 days'),
    (game_action_id, p2_id, 'player', 'active', NOW() - INTERVAL '7 days'),
    (game_action_id, p3_id, 'player', 'active', NOW() - INTERVAL '7 days'),
    (game_action_id, p4_id, 'player', 'active', NOW() - INTERVAL '7 days');

  -- Add characters for action submission
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game_action_id, p1_id, 'E2E Test Char 1', 'player_character', 'approved', NOW() - INTERVAL '7 days', NOW()) RETURNING id INTO char1_id;

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game_action_id, p2_id, 'E2E Test Char 2', 'player_character', 'approved', NOW() - INTERVAL '7 days', NOW()) RETURNING id INTO char2_id;

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game_action_id, p3_id, 'E2E Test Char 3', 'player_character', 'approved', NOW() - INTERVAL '7 days', NOW()) RETURNING id INTO char3_id;

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game_action_id, p4_id, 'E2E Test Char 4', 'player_character', 'approved', NOW() - INTERVAL '7 days', NOW()) RETURNING id INTO char4_id;

  -- Add previous common room phase
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, start_time, end_time, deadline, is_active, is_published, created_at)
  VALUES (
    game_action_id,
    'common_room',
    1,
    'Discussion Phase',
    NOW() - INTERVAL '3 days',
    NOW() - INTERVAL '2 days',
    NOW() - INTERVAL '2 days',
    false,
    false,
    NOW() - INTERVAL '3 days'
  );

  -- Add active action phase for submissions
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, deadline, is_active, is_published, created_at)
  VALUES (
    game_action_id,
    'action',
    2,
    'Action Phase',
    'Submit your actions for this phase',
    NOW() - INTERVAL '3 hours',
    NOW() + INTERVAL '21 hours',
    true,
    false,
    NOW() - INTERVAL '3 hours'
  ) RETURNING id INTO phase_id;

  -- Add an existing action for Player 1 (to test viewing existing actions)
  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, submitted_at, updated_at)
  VALUES (
    game_action_id,
    p1_id,
    phase_id,
    char1_id,
    'This is my existing action for testing purposes. I will do something interesting.',
    NOW() - INTERVAL '1 hour',
    NOW() - INTERVAL '1 hour'
  );

  -- Add an in-progress action for Player 4 (to test editing before the deadline)
  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, submitted_at, updated_at)
  VALUES (
    game_action_id,
    p4_id,
    phase_id,
    char4_id,
    'This is an in-progress action that needs to be completed.',
    NOW() - INTERVAL '45 minutes',
    NOW() - INTERVAL '30 minutes'
  );

  RAISE NOTICE 'Created Game #%: E2E Test: Action Submission', game_action_id;

  -- ============================================
  -- GAME #354: For Private Messages Testing
  -- ============================================
  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    game_messages_id,
    'E2E Test: Private Messages',
    'This game is dedicated for private messages testing in E2E tests.',
    'Test',
    gm_id,
    5,
    'in_progress',
    true,
    NOW() - INTERVAL '8 days',
    NOW()
  );

  -- Add participants (including audience for private message viewing tests)
  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES
    (game_messages_id, p1_id, 'player', 'active', NOW() - INTERVAL '7 days'),
    (game_messages_id, p2_id, 'player', 'active', NOW() - INTERVAL '7 days'),
    (game_messages_id, p3_id, 'player', 'active', NOW() - INTERVAL '7 days'),
    (game_messages_id, p4_id, 'player', 'active', NOW() - INTERVAL '7 days'),
    (game_messages_id, audience_id, 'audience', 'active', NOW() - INTERVAL '7 days');

  -- Add characters for messaging
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game_messages_id, p1_id, 'E2E Test Char 1', 'player_character', 'approved', NOW() - INTERVAL '7 days', NOW()) RETURNING id INTO char5_id;

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game_messages_id, p2_id, 'E2E Test Char 2', 'player_character', 'approved', NOW() - INTERVAL '7 days', NOW()) RETURNING id INTO char6_id;

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game_messages_id, p3_id, 'E2E Test Char 3', 'player_character', 'approved', NOW() - INTERVAL '7 days', NOW()) RETURNING id INTO char7_id;

  -- Add active common room phase (for private messaging)
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, deadline, is_active, is_published, created_at)
  VALUES (
    game_messages_id,
    'common_room',
    1,
    'Current Phase',
    'Active phase for testing private messages',
    NOW() - INTERVAL '3 hours',
    NOW() + INTERVAL '21 hours',
    true,
    false,
    NOW() - INTERVAL '3 hours'
  );

  -- ============================================
  -- GAME #355: For Game Settings Testing
  -- ============================================
  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    game_settings_id,
    'E2E Test: Game Settings',
    'This game is dedicated for testing game settings modifications (title, description, genre, etc.).',
    'Test',
    gm_id,
    4,
    'in_progress',
    true,
    NOW() - INTERVAL '5 days',
    NOW()
  );

  -- Add a single participant (minimal setup)
  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES
    (game_settings_id, p1_id, 'player', 'active', NOW() - INTERVAL '4 days');

  -- Add a simple active phase
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, start_time, deadline, is_active, is_published, created_at)
  VALUES (
    game_settings_id,
    'common_room',
    1,
    'Planning Phase',
    NOW() - INTERVAL '1 hour',
    NOW() + INTERVAL '23 hours',
    true,
    false,
    NOW() - INTERVAL '1 hour'
  );

  RAISE NOTICE 'Created 6 E2E dedicated test games (350-355) for worker-specific parallel testing';

END $$;

-- Reset the games sequence to prevent duplicate key errors
-- This ensures new game creations don't collide with hardcoded fixture IDs
SELECT setval('games_id_seq', (SELECT MAX(id) FROM games) + 1);

SELECT 'E2E Dedicated Games fixtures created successfully!' AS message;

COMMIT;
