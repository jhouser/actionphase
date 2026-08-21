-- E2E Test Fixture for Co-GM Action Results (Worker 1)
-- Game ID: 349 (worker 1 offset: 10000 → game 349)
-- IDEMPOTENT: Safe to run multiple times

BEGIN;

DO $$
DECLARE
  gm_id INTEGER;
  audience2_id INTEGER;
  player1_id INTEGER;
  game_id INTEGER;
  phase_id INTEGER;
  character_id INTEGER;
  worker_game_id_offset INTEGER := 10000;
BEGIN
  game_id := 349 + worker_game_id_offset;

  DELETE FROM games WHERE id = game_id;

  gm_id        := get_worker_user_id('TestGM', 1);
  audience2_id := get_worker_user_id('TestAudience2', 1);
  player1_id   := get_worker_user_id('TestPlayer1', 1);

  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    game_id,
    'E2E Test: Co-GM Action Results',
    'Game for testing co-GM action result editing. Has an active action phase.',
    'Fantasy',
    gm_id,
    5,
    'in_progress',
    true,
    NOW() - INTERVAL '14 days',
    NOW()
  );

  INSERT INTO game_participants (game_id, user_id, role, status)
  VALUES
    (game_id, audience2_id, 'audience', 'active'),
    (game_id, player1_id,   'player',   'active');

  INSERT INTO game_phases (game_id, phase_number, phase_type, title, description, start_time, end_time, is_active, is_published)
  VALUES (
    game_id, 1, 'action', 'Test Action Phase',
    'An active action phase for co-GM action result testing.',
    NOW() - INTERVAL '7 days', NOW() + INTERVAL '7 days',
    true, true
  )
  RETURNING id INTO phase_id;

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES (game_id, player1_id, 'Test Player Character', 'player_character', 'approved', NOW() - INTERVAL '10 days', NOW())
  RETURNING id INTO character_id;

  -- updated_at must match submitted_at: the backend preserves submitted_at across
  -- edits and only moves updated_at, so leaving updated_at to its NOW() default
  -- would make this never-edited row render an "Edited" badge in the UI.
  INSERT INTO action_submissions (game_id, phase_id, user_id, character_id, content, submitted_at, updated_at)
  VALUES (game_id, phase_id, player1_id, character_id, 'Test action submission for co-GM action results testing', NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days');

  RAISE NOTICE 'Co-GM Action Results fixture created: Game #%', game_id;
END $$;

SELECT 'E2E Co-GM Action Results fixture (worker 1) created successfully!' AS message;

COMMIT;
