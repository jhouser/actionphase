-- Create Running Games with Phase Structures
-- Updated to use new phase type system: only 'common_room' and 'action'
-- Published action phases replace the old 'results' phase type

BEGIN;

-- Get user IDs
DO $$
DECLARE
  gm_id INTEGER;
  p1_id INTEGER;
  p2_id INTEGER;
  p3_id INTEGER;
  p4_id INTEGER;
  audience_id INTEGER;
  audience1_id INTEGER;
  audience2_id INTEGER;
  game1_id INTEGER;
  game2_id INTEGER;
  game3_id INTEGER;
  game5_id INTEGER;
  game6_id INTEGER;
  game9_id INTEGER;
BEGIN
  SELECT id INTO gm_id FROM users WHERE email = 'test_gm@example.com';
  SELECT id INTO p1_id FROM users WHERE email = 'test_player1@example.com';
  SELECT id INTO p2_id FROM users WHERE email = 'test_player2@example.com';
  SELECT id INTO p3_id FROM users WHERE email = 'test_player3@example.com';
  SELECT id INTO p4_id FROM users WHERE email = 'test_player4@example.com';
  SELECT id INTO audience_id FROM users WHERE email = 'test_audience@example.com';
  SELECT id INTO audience1_id FROM users WHERE email = 'test_audience1@example.com';
  SELECT id INTO audience2_id FROM users WHERE email = 'test_audience2@example.com';

  -- ============================================
  -- GAME #1: Active Common Room Phase
  -- ============================================
  INSERT INTO games (title, description, genre, gm_user_id, max_players, state, is_public, is_anonymous, created_at, updated_at)
  VALUES (
    'Shadows Over Innsmouth',
    'A Lovecraftian horror investigation in a cursed fishing town.',
    'Call of Cthulhu 7e',
    gm_id,
    4,
    'in_progress',
    true,
    true,
    NOW() - INTERVAL '7 days',
    NOW()
  ) RETURNING id INTO game1_id;

  -- Add participants
  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES
    (game1_id, p1_id, 'player', 'active', NOW() - INTERVAL '6 days'),
    (game1_id, p2_id, 'player', 'active', NOW() - INTERVAL '6 days'),
    (game1_id, p3_id, 'player', 'active', NOW() - INTERVAL '6 days'),
    (game1_id, audience1_id, 'co_gm', 'active', NOW() - INTERVAL '6 days'),  -- Promoted from audience
    (game1_id, audience_id, 'audience', 'active', NOW() - INTERVAL '5 days');

  -- Phase 1: Active Common Room with custom title
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, deadline, is_active, is_published, activated_at, created_at)
  VALUES (
    game1_id,
    'common_room',
    1,
    'Arrival at the Harbor',
    'The investigators arrive at Innsmouth harbor on a foggy evening. The locals eye them suspiciously.',
    NOW() - INTERVAL '2 hours',
    NOW() + INTERVAL '22 hours',
    true,
    true,  -- Published so demo content is visible
    NOW() - INTERVAL '2 hours',
    NOW() - INTERVAL '2 hours'
  );

  -- ============================================
  -- GAME #2: Active Action Phase (with actions)
  -- ============================================
  INSERT INTO games (title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    'The Heist at Goldstone Bank',
    'A thrilling heist scenario where planning is everything.',
    'Blades in the Dark',
    gm_id,
    5,
    'in_progress',
    true,
    NOW() - INTERVAL '10 days',
    NOW()
  ) RETURNING id INTO game2_id;

  -- Add participants
  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES
    (game2_id, p1_id, 'player', 'active', NOW() - INTERVAL '9 days'),
    (game2_id, p2_id, 'player', 'active', NOW() - INTERVAL '9 days'),
    (game2_id, p3_id, 'player', 'active', NOW() - INTERVAL '9 days'),
    (game2_id, p4_id, 'player', 'active', NOW() - INTERVAL '9 days'),
    (game2_id, audience2_id, 'audience', 'active', NOW() - INTERVAL '8 days'),
    (game2_id, audience_id, 'audience', 'active', NOW() - INTERVAL '8 days');

  -- Previous Phase 1: Common Room
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, start_time, end_time, deadline, is_active, is_published, activated_at, created_at)
  VALUES (
    game2_id,
    'common_room',
    1,
    'Casing the Bank',
    NOW() - INTERVAL '3 days',
    NOW() - INTERVAL '2 days',
    NOW() - INTERVAL '2 days',
    false,
    false,
    NOW() - INTERVAL '3 days',
    NOW() - INTERVAL '3 days'
  );

  -- Active Phase 2: Action Phase (accepting submissions)
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, deadline, is_active, is_published, activated_at, created_at)
  VALUES (
    game2_id,
    'action',
    2,
    'Execute the Plan',
    'Each crew member executes their part of the heist. Submit your actions now!',
    NOW() - INTERVAL '4 hours',
    NOW() + INTERVAL '20 hours',
    true,
    false,
    NOW() - INTERVAL '4 hours',
    NOW() - INTERVAL '4 hours'
  );

  -- ============================================
  -- GAME #3: Active Published Action Phase (Results)
  -- ============================================
  INSERT INTO games (title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    'Starfall Station',
    'A sci-fi mystery on a remote space station.',
    'Mothership',
    gm_id,
    4,
    'in_progress',
    true,
    NOW() - INTERVAL '14 days',
    NOW()
  ) RETURNING id INTO game3_id;

  -- Add participants
  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES
    (game3_id, p1_id, 'player', 'active', NOW() - INTERVAL '13 days'),
    (game3_id, p2_id, 'player', 'active', NOW() - INTERVAL '13 days'),
    (game3_id, p3_id, 'player', 'active', NOW() - INTERVAL '13 days'),
    (game3_id, audience_id, 'audience', 'active', NOW() - INTERVAL '12 days');

  -- Previous phases
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, start_time, end_time, deadline, is_active, is_published, activated_at, created_at)
  VALUES
    (game3_id, 'common_room', 1, 'Initial Planning', NOW() - INTERVAL '5 days', NOW() - INTERVAL '4 days', NOW() - INTERVAL '4 days', false, false, NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days'),
    (game3_id, 'action', 2, 'First Investigation', NOW() - INTERVAL '4 days', NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days', false, false, NOW() - INTERVAL '4 days', NOW() - INTERVAL '4 days');

  -- Active Phase 3: Published Action Phase (results are published)
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, start_time, is_active, is_published, activated_at, created_at)
  VALUES (
    game3_id,
    'action',
    3,
    'The Truth Revealed',
    NOW() - INTERVAL '6 hours',
    true,
    true,
    NOW() - INTERVAL '6 hours',
    NOW() - INTERVAL '6 hours'
  );


  -- ============================================
  -- GAME #5: Complex Phase History
  -- ============================================
  INSERT INTO games (title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    'The Dragon of Mount Krag',
    'An epic fantasy campaign with a long history.',
    'D&D 5e',
    gm_id,
    5,
    'in_progress',
    true,
    NOW() - INTERVAL '45 days',
    NOW()
  ) RETURNING id INTO game5_id;

  -- Add participants
  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES
    (game5_id, p1_id, 'player', 'active', NOW() - INTERVAL '44 days'),
    (game5_id, p2_id, 'player', 'active', NOW() - INTERVAL '44 days'),
    (game5_id, p3_id, 'player', 'active', NOW() - INTERVAL '44 days'),
    (game5_id, audience_id, 'audience', 'active', NOW() - INTERVAL '40 days');

  -- Phase History (with published action phases replacing old results phases)
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, start_time, end_time, deadline, is_active, is_published, activated_at, created_at)
  VALUES
    (game5_id, 'common_room', 1, 'The Tavern Meeting', NOW() - INTERVAL '30 days', NOW() - INTERVAL '28 days', NOW() - INTERVAL '28 days', false, false, NOW() - INTERVAL '30 days', NOW() - INTERVAL '30 days'),
    (game5_id, 'action', 2, 'Journey to Krag', NOW() - INTERVAL '28 days', NOW() - INTERVAL '25 days', NOW() - INTERVAL '25 days', false, false, NOW() - INTERVAL '28 days', NOW() - INTERVAL '28 days'),
    (game5_id, 'action', 3, 'Ambush on the Road', NOW() - INTERVAL '25 days', NOW() - INTERVAL '23 days', NOW() - INTERVAL '23 days', false, true, NOW() - INTERVAL '25 days', NOW() - INTERVAL '25 days'),
    (game5_id, 'common_room', 4, 'Healing and Planning', NOW() - INTERVAL '23 days', NOW() - INTERVAL '20 days', NOW() - INTERVAL '20 days', false, false, NOW() - INTERVAL '23 days', NOW() - INTERVAL '23 days'),
    (game5_id, 'action', 5, 'Infiltrate the Caves', NOW() - INTERVAL '20 days', NOW() - INTERVAL '18 days', NOW() - INTERVAL '18 days', false, false, NOW() - INTERVAL '20 days', NOW() - INTERVAL '20 days'),
    (game5_id, 'action', 6, 'Discovery of the Lair', NOW() - INTERVAL '18 days', NOW() - INTERVAL '15 days', NOW() - INTERVAL '15 days', false, true, NOW() - INTERVAL '18 days', NOW() - INTERVAL '18 days');

  -- Active Phase 7: Common Room
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, start_time, is_active, is_published, activated_at, created_at)
  VALUES (
    game5_id,
    'common_room',
    7,
    'Final Preparations',
    NOW() - INTERVAL '3 hours',
    true,
    true,  -- Published so demo content is visible
    NOW() - INTERVAL '3 hours',
    NOW() - INTERVAL '3 hours'
  );

  -- ============================================
  -- GAME #6: Many Mixed Phases
  -- ============================================
  INSERT INTO games (title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    'Chronicles of Westmarch',
    'A long-running sandbox campaign with rich history.',
    'Pathfinder 2e',
    gm_id,
    6,
    'in_progress',
    true,
    NOW() - INTERVAL '60 days',
    NOW()
  ) RETURNING id INTO game6_id;

  -- Add participants
  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES
    (game6_id, p1_id, 'player', 'active', NOW() - INTERVAL '59 days'),
    (game6_id, p2_id, 'player', 'active', NOW() - INTERVAL '59 days'),
    (game6_id, p3_id, 'player', 'active', NOW() - INTERVAL '58 days'),
    (game6_id, p4_id, 'player', 'active', NOW() - INTERVAL '55 days'),
    (game6_id, audience_id, 'audience', 'active', NOW() - INTERVAL '50 days');

  -- Many phases (12 total) - alternating common room, unpublished action, and published action
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, start_time, end_time, deadline, is_active, is_published, activated_at, created_at)
  VALUES
    (game6_id, 'common_room', 1, 'Phase 1', NOW() - INTERVAL '55 days', NOW() - INTERVAL '53 days', NOW() - INTERVAL '53 days', false, false, NOW() - INTERVAL '55 days', NOW() - INTERVAL '55 days'),
    (game6_id, 'action', 2, 'Phase 2', NOW() - INTERVAL '53 days', NOW() - INTERVAL '50 days', NOW() - INTERVAL '50 days', false, false, NOW() - INTERVAL '53 days', NOW() - INTERVAL '53 days'),
    (game6_id, 'action', 3, 'Phase 3 Results', NOW() - INTERVAL '50 days', NOW() - INTERVAL '48 days', NOW() - INTERVAL '48 days', false, true, NOW() - INTERVAL '50 days', NOW() - INTERVAL '50 days'),
    (game6_id, 'common_room', 4, 'Phase 4', NOW() - INTERVAL '48 days', NOW() - INTERVAL '45 days', NOW() - INTERVAL '45 days', false, false, NOW() - INTERVAL '48 days', NOW() - INTERVAL '48 days'),
    (game6_id, 'action', 5, 'Phase 5', NOW() - INTERVAL '45 days', NOW() - INTERVAL '42 days', NOW() - INTERVAL '42 days', false, false, NOW() - INTERVAL '45 days', NOW() - INTERVAL '45 days'),
    (game6_id, 'action', 6, 'Phase 6 Results', NOW() - INTERVAL '42 days', NOW() - INTERVAL '40 days', NOW() - INTERVAL '40 days', false, true, NOW() - INTERVAL '42 days', NOW() - INTERVAL '42 days'),
    (game6_id, 'common_room', 7, 'Phase 7', NOW() - INTERVAL '40 days', NOW() - INTERVAL '37 days', NOW() - INTERVAL '37 days', false, false, NOW() - INTERVAL '40 days', NOW() - INTERVAL '40 days'),
    (game6_id, 'action', 8, 'Phase 8', NOW() - INTERVAL '37 days', NOW() - INTERVAL '34 days', NOW() - INTERVAL '34 days', false, false, NOW() - INTERVAL '37 days', NOW() - INTERVAL '37 days'),
    (game6_id, 'action', 9, 'Phase 9 Results', NOW() - INTERVAL '34 days', NOW() - INTERVAL '31 days', NOW() - INTERVAL '31 days', false, true, NOW() - INTERVAL '34 days', NOW() - INTERVAL '34 days'),
    (game6_id, 'common_room', 10, 'Phase 10', NOW() - INTERVAL '31 days', NOW() - INTERVAL '28 days', NOW() - INTERVAL '28 days', false, false, NOW() - INTERVAL '31 days', NOW() - INTERVAL '31 days'),
    (game6_id, 'action', 11, 'Phase 11', NOW() - INTERVAL '28 days', NOW() - INTERVAL '25 days', NOW() - INTERVAL '25 days', false, false, NOW() - INTERVAL '28 days', NOW() - INTERVAL '28 days');

  -- Active Phase 12: Published Action (showing results)
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, start_time, is_active, is_published, activated_at, created_at)
  VALUES (
    game6_id,
    'action',
    12,
    'Phase 12 Results',
    NOW() - INTERVAL '12 hours',
    true,
    true,
    NOW() - INTERVAL '12 hours',
    NOW() - INTERVAL '12 hours'
  );


  -- ============================================
  -- GAME #9: Completed Campaign
  -- ============================================
  INSERT INTO games (title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    'COMPLETED: Tales of the Arcane',
    'A completed magical mystery campaign.',
    'Mage: The Ascension',
    gm_id,
    4,
    'completed',
    true,
    NOW() - INTERVAL '90 days',
    NOW() - INTERVAL '5 days'
  ) RETURNING id INTO game9_id;

  -- Add participants
  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES
    (game9_id, p1_id, 'player', 'active', NOW() - INTERVAL '89 days'),
    (game9_id, p2_id, 'player', 'active', NOW() - INTERVAL '89 days'),
    (game9_id, p3_id, 'player', 'active', NOW() - INTERVAL '88 days'),
    (game9_id, audience_id, 'audience', 'active', NOW() - INTERVAL '85 days');

  -- Full phase history - action phases contain both submissions and results
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, start_time, end_time, deadline, is_active, is_published, activated_at, created_at)
  VALUES
    (game9_id, 'common_room', 1, 'The Beginning', NOW() - INTERVAL '85 days', NOW() - INTERVAL '82 days', NOW() - INTERVAL '82 days', false, false, NOW() - INTERVAL '85 days', NOW() - INTERVAL '85 days'),
    (game9_id, 'action', 2, 'First Challenge', NOW() - INTERVAL '82 days', NOW() - INTERVAL '79 days', NOW() - INTERVAL '79 days', false, false, NOW() - INTERVAL '82 days', NOW() - INTERVAL '82 days'),
    (game9_id, 'common_room', 3, 'Reflection After First Challenge', NOW() - INTERVAL '79 days', NOW() - INTERVAL '76 days', NOW() - INTERVAL '76 days', false, false, NOW() - INTERVAL '79 days', NOW() - INTERVAL '79 days'),
    (game9_id, 'common_room', 4, 'Planning Second Trial', NOW() - INTERVAL '76 days', NOW() - INTERVAL '73 days', NOW() - INTERVAL '73 days', false, false, NOW() - INTERVAL '76 days', NOW() - INTERVAL '76 days'),
    (game9_id, 'action', 5, 'Second Trial', NOW() - INTERVAL '73 days', NOW() - INTERVAL '70 days', NOW() - INTERVAL '70 days', false, false, NOW() - INTERVAL '73 days', NOW() - INTERVAL '73 days'),
    (game9_id, 'common_room', 6, 'Reflection After Second Trial', NOW() - INTERVAL '70 days', NOW() - INTERVAL '67 days', NOW() - INTERVAL '67 days', false, false, NOW() - INTERVAL '70 days', NOW() - INTERVAL '70 days'),
    (game9_id, 'common_room', 7, 'Final Planning', NOW() - INTERVAL '67 days', NOW() - INTERVAL '64 days', NOW() - INTERVAL '64 days', false, false, NOW() - INTERVAL '67 days', NOW() - INTERVAL '67 days'),
    (game9_id, 'action', 8, 'Final Challenge', NOW() - INTERVAL '64 days', NOW() - INTERVAL '61 days', NOW() - INTERVAL '61 days', false, false, NOW() - INTERVAL '64 days', NOW() - INTERVAL '64 days'),
    (game9_id, 'common_room', 9, 'Epilogue', NOW() - INTERVAL '61 days', NOW() - INTERVAL '58 days', NOW() - INTERVAL '58 days', false, false, NOW() - INTERVAL '61 days', NOW() - INTERVAL '61 days');

END $$;

COMMIT;
