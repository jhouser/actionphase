-- E2E Test Fixture for GM Editing Action Results
-- Creates a dedicated game for testing GM result editing workflow
-- where the action phase is ACTIVE (deadline passed, GM writing results)
--
-- Game IDs: 325 (offset by worker: Worker 1 = 1325, Worker 2 = 2325, etc.)
--
-- IDEMPOTENT: Safe to run multiple times - deletes existing data before recreating

BEGIN;

-- Delete existing GM editing results test game to prevent duplicates
DELETE FROM games WHERE id = 325;

DO $$
DECLARE
  gm_id INTEGER;
  p1_id INTEGER;
  p2_id INTEGER;
  p3_id INTEGER;
  p4_id INTEGER;
  game_id INTEGER := 325;
  active_phase_id INTEGER;
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
  -- E2E Game: For GM Editing Action Results
  -- ============================================
  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    game_id,
    'E2E Test: GM Editing Results',
    'This game is dedicated for testing GM editing unpublished results during active action phase.',
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
    (game_id, p1_id, 'GM Edit Test Char 1', 'player_character', 'approved', NOW() - INTERVAL '9 days', NOW()) RETURNING id INTO char1_id;

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game_id, p2_id, 'GM Edit Test Char 2', 'player_character', 'approved', NOW() - INTERVAL '9 days', NOW()) RETURNING id INTO char2_id;

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game_id, p3_id, 'GM Edit Test Char 3', 'player_character', 'approved', NOW() - INTERVAL '9 days', NOW()) RETURNING id INTO char3_id;

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game_id, p4_id, 'GM Edit Test Char 4', 'player_character', 'approved', NOW() - INTERVAL '9 days', NOW()) RETURNING id INTO char4_id;

  -- Add an ACTIVE action phase (deadline passed, GM writing results)
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, deadline, is_active, is_published, created_at)
  VALUES (
    game_id,
    'action',
    1,
    'Active Action Phase',
    'Action deadline has passed - GM is writing results before ending phase',
    NOW() - INTERVAL '5 days',
    NOW() - INTERVAL '2 days',
    true,
    false,
    NOW() - INTERVAL '5 days'
  ) RETURNING id INTO active_phase_id;

  -- Add action submissions from all players
  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, submitted_at, updated_at)
  VALUES (
    game_id,
    p1_id,
    active_phase_id,
    char1_id,
    'Player 1 investigates the mysterious sounds coming from the basement.',
    NOW() - INTERVAL '3 days',
    NOW() - INTERVAL '3 days'
  ) RETURNING id INTO action1_id;

  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, submitted_at, updated_at)
  VALUES (
    game_id,
    p2_id,
    active_phase_id,
    char2_id,
    'Player 2 searches the library for ancient texts about the cult.',
    NOW() - INTERVAL '3 days',
    NOW() - INTERVAL '3 days'
  ) RETURNING id INTO action2_id;

  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, submitted_at, updated_at)
  VALUES (
    game_id,
    p3_id,
    active_phase_id,
    char3_id,
    'Player 3 attempts to decode the mysterious symbols on the wall.',
    NOW() - INTERVAL '3 days',
    NOW() - INTERVAL '3 days'
  ) RETURNING id INTO action3_id;

  -- Add PUBLISHED action result for Player 1 (to test viewing results)
  INSERT INTO action_results (game_id, user_id, phase_id, character_id, action_submission_id, content, gm_user_id, is_published, sent_at, created_at, updated_at)
  VALUES (
    game_id,
    p1_id,
    active_phase_id,
    char1_id,
    action1_id,
    E'# Basement Investigation Results\n\nYou descend into the basement, flashlight in hand. The sounds grow louder as you approach a hidden door behind some old furniture.\n\n**You discovered:** A secret passage!\n\nMention: @GM Edit Test Char 2 might want to know about this.',
    gm_id,
    true,
    NOW() - INTERVAL '1 day',
    NOW() - INTERVAL '1 day',
    NOW() - INTERVAL '1 day'
  );

  -- Add PUBLISHED action result for Player 2 (to test multiple results)
  INSERT INTO action_results (game_id, user_id, phase_id, character_id, action_submission_id, content, gm_user_id, is_published, sent_at, created_at, updated_at)
  VALUES (
    game_id,
    p2_id,
    active_phase_id,
    char2_id,
    action2_id,
    E'# Library Research Results\n\nYour search through the dusty tomes reveals a reference to the "Order of the Crimson Moon" - a cult that operated in this town 100 years ago.\n\n**Knowledge Gained:** +1 Occult Lore',
    gm_id,
    true,
    NOW() - INTERVAL '1 day',
    NOW() - INTERVAL '1 day',
    NOW() - INTERVAL '1 day'
  );

  -- Add UNPUBLISHED action result for Player 3 (to test GM editing)
  INSERT INTO action_results (game_id, user_id, phase_id, character_id, action_submission_id, content, gm_user_id, is_published, sent_at, created_at, updated_at)
  VALUES (
    game_id,
    p3_id,
    active_phase_id,
    char3_id,
    action3_id,
    'DRAFT: The symbols appear to be a warning... (GM still working on this result)',
    gm_id,
    false,
    NULL,
    NOW() - INTERVAL '6 hours',
    NOW() - INTERVAL '6 hours'
  );

  -- Note: No Phase 2 yet - GM will activate it after finishing/publishing results

END $$;

-- Reset the games sequence to prevent duplicate key errors
-- This ensures new game creations don't collide with hardcoded fixture IDs
SELECT setval('games_id_seq', (SELECT MAX(id) FROM games) + 1);

COMMIT;

-- Success message
SELECT 'E2E GM Editing Results fixture created successfully!' as message;
