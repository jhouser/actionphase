-- E2E Test Fixture for Action Results
-- Creates a dedicated game with action results in various states (published, unpublished)
-- for testing the complete action results workflow
--
-- Game IDs: 326 (offset by worker: Worker 1 = 1326, Worker 2 = 2326, etc.)
--
-- IDEMPOTENT: Safe to run multiple times - deletes existing data before recreating

BEGIN;

-- Delete existing action results test game to prevent duplicates
DELETE FROM games WHERE id = 326;

DO $$
DECLARE
  gm_id INTEGER;
  p1_id INTEGER;
  p2_id INTEGER;
  p3_id INTEGER;
  p4_id INTEGER;
  game_id INTEGER := 326;
  completed_phase_id INTEGER;
  char1_id INTEGER;
  char2_id INTEGER;
  char3_id INTEGER;
  char4_id INTEGER;
  action1_id INTEGER;
  action2_id INTEGER;
  action3_id INTEGER;
BEGIN
  -- Get user IDs
  SELECT id INTO gm_id FROM users WHERE email = 'test_gm@example.com';
  SELECT id INTO p1_id FROM users WHERE email = 'test_player1@example.com';
  SELECT id INTO p2_id FROM users WHERE email = 'test_player2@example.com';
  SELECT id INTO p3_id FROM users WHERE email = 'test_player3@example.com';
  SELECT id INTO p4_id FROM users WHERE email = 'test_player4@example.com';

  -- ============================================
  -- E2E Game: For Action Results Testing
  -- ============================================
  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    game_id,
    'E2E Test: Action Results',
    'This game is dedicated for testing action results creation, viewing, and notifications.',
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
    (game_id, p1_id, 'player', 'active', NOW() - INTERVAL '9 days'),
    (game_id, p2_id, 'player', 'active', NOW() - INTERVAL '9 days'),
    (game_id, p3_id, 'player', 'active', NOW() - INTERVAL '9 days'),
    (game_id, p4_id, 'player', 'active', NOW() - INTERVAL '9 days');

  -- Add characters
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game_id, p1_id, 'Result Test Char 1', 'player_character', 'approved', NOW() - INTERVAL '9 days', NOW()) RETURNING id INTO char1_id;

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game_id, p2_id, 'Result Test Char 2', 'player_character', 'approved', NOW() - INTERVAL '9 days', NOW()) RETURNING id INTO char2_id;

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game_id, p3_id, 'Result Test Char 3', 'player_character', 'approved', NOW() - INTERVAL '9 days', NOW()) RETURNING id INTO char3_id;

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game_id, p4_id, 'Result Test Char 4', 'player_character', 'approved', NOW() - INTERVAL '9 days', NOW()) RETURNING id INTO char4_id;

  -- Add a completed action phase with published results
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, deadline, end_time, is_active, is_published, created_at)
  VALUES (
    game_id,
    'action',
    1,
    'Completed Action Phase',
    'Action phase that has ended - results are published',
    NOW() - INTERVAL '10 days',
    NOW() - INTERVAL '5 days',
    NOW() - INTERVAL '2 days',
    false,
    true,
    NOW() - INTERVAL '10 days'
  ) RETURNING id INTO completed_phase_id;

  -- Add action submissions from all players
  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, is_draft, submitted_at, updated_at)
  VALUES (
    game_id,
    p1_id,
    completed_phase_id,
    char1_id,
    'Player 1 investigates the mysterious sounds coming from the basement.',
    false,
    NOW() - INTERVAL '3 days',
    NOW() - INTERVAL '3 days'
  ) RETURNING id INTO action1_id;

  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, is_draft, submitted_at, updated_at)
  VALUES (
    game_id,
    p2_id,
    completed_phase_id,
    char2_id,
    'Player 2 searches the library for ancient texts about the cult.',
    false,
    NOW() - INTERVAL '3 days',
    NOW() - INTERVAL '3 days'
  ) RETURNING id INTO action2_id;

  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, is_draft, submitted_at, updated_at)
  VALUES (
    game_id,
    p3_id,
    completed_phase_id,
    char3_id,
    'Player 3 attempts to decode the mysterious symbols on the wall.',
    false,
    NOW() - INTERVAL '3 days',
    NOW() - INTERVAL '3 days'
  ) RETURNING id INTO action3_id;

  -- Add PUBLISHED action result for Player 1 (to test viewing results)
  INSERT INTO action_results (game_id, user_id, phase_id, action_submission_id, character_id, content, gm_user_id, is_published, sent_at, created_at, updated_at)
  VALUES (
    game_id,
    p1_id,
    completed_phase_id,
    action1_id,
    (SELECT character_id FROM action_submissions WHERE id = action1_id),
    E'# Basement Investigation Results\n\nYou descend into the basement, flashlight in hand. The sounds grow louder as you approach a hidden door behind some old furniture.\n\n**You discovered:** A secret passage!\n\nMention: @Result Test Char 2 might want to know about this.',
    gm_id,
    true,
    NOW() - INTERVAL '1 day',
    NOW() - INTERVAL '1 day',
    NOW() - INTERVAL '1 day'
  );

  -- Add PUBLISHED action result for Player 2 (to test multiple results)
  INSERT INTO action_results (game_id, user_id, phase_id, action_submission_id, character_id, content, gm_user_id, is_published, sent_at, created_at, updated_at)
  VALUES (
    game_id,
    p2_id,
    completed_phase_id,
    action2_id,
    (SELECT character_id FROM action_submissions WHERE id = action2_id),
    E'# Library Research Results\n\nYour search through the dusty tomes reveals a reference to the \"Order of the Crimson Moon\" - a cult that operated in this town 100 years ago.\n\n**Knowledge Gained:** +1 Occult Lore',
    gm_id,
    true,
    NOW() - INTERVAL '1 day',
    NOW() - INTERVAL '1 day',
    NOW() - INTERVAL '1 day'
  );

  -- Add UNPUBLISHED action result for Player 3 (to test that players can't see unpublished results)
  INSERT INTO action_results (game_id, user_id, phase_id, action_submission_id, character_id, content, gm_user_id, is_published, sent_at, created_at, updated_at)
  VALUES (
    game_id,
    p3_id,
    completed_phase_id,
    action3_id,
    (SELECT character_id FROM action_submissions WHERE id = action3_id),
    'DRAFT: The symbols appear to be a warning... (GM still working on this result)',
    gm_id,
    false,
    NULL,
    NOW() - INTERVAL '6 hours',
    NOW() - INTERVAL '6 hours'
  );

  -- Add active common room phase (so Recent Results Section can display)
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, is_active, is_published, created_at)
  VALUES (
    game_id,
    'common_room',
    2,
    'Discussion Phase',
    'Players discuss the action results',
    NOW() - INTERVAL '1 day',
    true,
    true,
    NOW() - INTERVAL '1 day'
  );

END $$;

-- Reset the games sequence to prevent duplicate key errors
-- This ensures new game creations don't collide with hardcoded fixture IDs
SELECT setval('games_id_seq', (SELECT MAX(id) FROM games) + 1);

COMMIT;

-- Success message
SELECT 'E2E Action Results fixture created successfully!' as message;
