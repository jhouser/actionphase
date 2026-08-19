-- Create Characters and NPCs

BEGIN;

DO $$
DECLARE
  gm_id INTEGER;
  p1_id INTEGER;
  p2_id INTEGER;
  p3_id INTEGER;
  p4_id INTEGER;
  audience_id INTEGER;
  audience1_id INTEGER;
  game1_id INTEGER;
  game2_id INTEGER;
  game3_id INTEGER;
  game5_id INTEGER;
  game6_id INTEGER;
  game9_id INTEGER;
  char1_id INTEGER;
  char2_id INTEGER;
  npc1_id INTEGER;
  npc2_id INTEGER;
BEGIN
  -- Get user IDs
  SELECT id INTO gm_id FROM users WHERE email = 'test_gm@example.com';
  SELECT id INTO p1_id FROM users WHERE email = 'test_player1@example.com';
  SELECT id INTO p2_id FROM users WHERE email = 'test_player2@example.com';
  SELECT id INTO p3_id FROM users WHERE email = 'test_player3@example.com';
  SELECT id INTO p4_id FROM users WHERE email = 'test_player4@example.com';
  SELECT id INTO audience_id FROM users WHERE email = 'test_audience@example.com';
  SELECT id INTO audience1_id FROM users WHERE email = 'test_audience1@example.com';

  -- Get game IDs
  SELECT id INTO game1_id FROM games WHERE title = 'Shadows Over Innsmouth';
  SELECT id INTO game2_id FROM games WHERE title = 'The Heist at Goldstone Bank';
  SELECT id INTO game3_id FROM games WHERE title = 'Starfall Station';
  SELECT id INTO game5_id FROM games WHERE title = 'The Dragon of Mount Krag';
  SELECT id INTO game6_id FROM games WHERE title = 'Chronicles of Westmarch';
  SELECT id INTO game9_id FROM games WHERE title = 'COMPLETED: Tales of the Arcane';

  -- ============================================
  -- GAME #1: Shadows Over Innsmouth
  -- ============================================

  -- Player Characters
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game1_id, p1_id, 'Detective Marcus Kane', 'player_character', 'approved', NOW() - INTERVAL '6 days', NOW()),
    (game1_id, p2_id, 'Dr. Sarah Chen', 'player_character', 'approved', NOW() - INTERVAL '6 days', NOW()),
    (game1_id, p3_id, 'Father O''Brien', 'player_character', 'approved', NOW() - INTERVAL '6 days', NOW());

  -- GM NPCs (user_id = NULL, controlled by GM/co-GM or via npc_assignments)
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game1_id, NULL, 'Captain Obed Marsh', 'npc', 'approved', NOW() - INTERVAL '5 days', NOW()),
    (game1_id, NULL, 'The Fishmonger', 'npc', 'approved', NOW() - INTERVAL '5 days', NOW()),
    (game1_id, NULL, 'Local Informant', 'npc', 'approved', NOW() - INTERVAL '5 days', NOW());

  -- ============================================
  -- GAME #2: The Heist
  -- ============================================

  -- Player Characters
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game2_id, p1_id, 'Shade (Whisper)', 'player_character', 'approved', NOW() - INTERVAL '9 days', NOW()) RETURNING id INTO char1_id;

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game2_id, p2_id, 'Rook (Hound)', 'player_character', 'approved', NOW() - INTERVAL '9 days', NOW()) RETURNING id INTO char2_id;

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game2_id, p3_id, 'Vex (Leech)', 'player_character', 'approved', NOW() - INTERVAL '9 days', NOW()),
    (game2_id, p4_id, 'Silk (Spider)', 'player_character', 'approved', NOW() - INTERVAL '9 days', NOW());

  -- GM NPCs (user_id = NULL)
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game2_id, NULL, 'Inspector Dalton', 'npc', 'approved', NOW() - INTERVAL '8 days', NOW()),
    (game2_id, NULL, 'Bones (Contact)', 'npc', 'approved', NOW() - INTERVAL '7 days', NOW()),
    (game2_id, NULL, 'Whistle (Lookout)', 'npc', 'approved', NOW() - INTERVAL '7 days', NOW());

  -- Character data examples
  -- Module types and JSON shapes must match what the app actually writes:
  -- module_type is one of bio/notes/skills/inventory/numbers, and each
  -- stat collection is a single row holding a JSON array (see CHARACTER_MODULES in
  -- frontend/src/types/characters.ts). Blades flavor lives in the values, not in
  -- invented module types.
  INSERT INTO character_data (character_id, module_type, field_name, field_value, field_type, is_public, created_at, updated_at)
  VALUES
    (char1_id, 'bio', 'background',
     'A Whisper who keeps company with things better left unspoken. Quiet, watchful, and always paying a debt to someone.',
     'text', true, NOW(), NOW()),
    (char1_id, 'skills', 'skills',
     '[{"id":"a1e8c0d2-3b47-4f19-9c6a-5d2e7f081b34","name":"Compel","rank":"2","description":"Force a ghost or spirit to obey.","category":"Arcane"},{"id":"b2f9d1e3-4c58-4a2b-8d7b-6e3f8a192c45","name":"Attune","rank":"1","description":"Open your mind to the ghost field.","category":"Arcane"}]',
     'json', true, NOW(), NOW()),
    (char1_id, 'inventory', 'items',
     '[{"id":"c3a0e2f4-5d69-4b3c-9e8c-7f4a9b203d56","name":"Spirit Bottle","description":"Holds a captured ghost.","quantity":1,"category":"Arcane"},{"id":"d4b1f3a5-6e7a-4c4d-8f9d-8a5b0c314e67","name":"Fine Cloak","description":"Unremarkable until you look twice.","quantity":1,"category":"Gear"}]',
     'json', true, NOW(), NOW()),
    (char1_id, 'numbers', 'numbers',
     '[{"id":"e5c2a4b6-7f8b-4d5e-9a0e-9b6c1d425f78","name":"Coin","amount":6,"description":"Stashed, not spent."},{"id":"f6d3b5c7-8a9c-4e6f-8b1f-0c7d2e536a89","name":"Stress","amount":4,"max":9,"display":"boxes"}]',
     'json', false, NOW(), NOW()),
    (char2_id, 'bio', 'background',
     'A Hound with a steady hand and a longer memory. Never misses, never forgets who pointed him at the target.',
     'text', true, NOW(), NOW()),
    (char2_id, 'skills', 'skills',
     '[{"id":"a7e4c6d8-9b0d-4f70-9c2a-1d8e3f647b90","name":"Sharpshooter","rank":"3","description":"Push yourself to make a long-range shot.","category":"Combat"},{"id":"b8f5d7e9-0c1e-4a81-8d3b-2e9f4a758c01","name":"Survey","rank":"1","description":"Read a situation before it reads you.","category":"Observation"}]',
     'json', true, NOW(), NOW()),
    (char2_id, 'inventory', 'items',
     '[{"id":"c9a6e8f0-1d2f-4b92-9e4c-3f0a5b869d12","name":"Hunting Rifle","description":"Scoped, well maintained.","quantity":1,"category":"Weapon"},{"id":"d0b7f9a1-2e30-4ca3-8f5d-4a1b6c970e23","name":"Tranquilizer Darts","description":"For when the job wants them breathing.","quantity":6,"category":"Ammunition"}]',
     'json', true, NOW(), NOW()),
    (char2_id, 'numbers', 'numbers',
     '[{"id":"e1c8a0b2-3f41-4db4-9a6e-5b2c7d081f34","name":"Coin","amount":3},{"id":"f2d9b1c3-4a52-4ec5-8b7f-6c3d8e192a45","name":"Stress","amount":2,"max":9,"display":"boxes"}]',
     'json', false, NOW(), NOW());

  -- ============================================
  -- GAME #3: Starfall Station
  -- ============================================

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game3_id, p1_id, 'Commander Vasquez', 'player_character', 'approved', NOW() - INTERVAL '13 days', NOW()),
    (game3_id, p2_id, 'Engineer Patel', 'player_character', 'approved', NOW() - INTERVAL '13 days', NOW()),
    (game3_id, p3_id, 'Dr. Kim', 'player_character', 'approved', NOW() - INTERVAL '13 days', NOW()),
    (game3_id, NULL, 'The Alien Entity', 'npc', 'approved', NOW() - INTERVAL '12 days', NOW());

  -- ============================================
  -- GAME #5: Dragon of Mount Krag
  -- ============================================

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game5_id, p1_id, 'Thorin Ironforge', 'player_character', 'approved', NOW() - INTERVAL '44 days', NOW()),
    (game5_id, p2_id, 'Elara Moonshadow', 'player_character', 'approved', NOW() - INTERVAL '44 days', NOW()),
    (game5_id, p3_id, 'Grimm the Bold', 'player_character', 'approved', NOW() - INTERVAL '44 days', NOW()),
    (game5_id, NULL, 'Vorathax the Ancient', 'npc', 'approved', NOW() - INTERVAL '40 days', NOW());

  -- ============================================
  -- GAME #6: Chronicles of Westmarch
  -- ============================================

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game6_id, p1_id, 'Sir Aldric', 'player_character', 'approved', NOW() - INTERVAL '59 days', NOW()),
    (game6_id, p2_id, 'Zara the Mystic', 'player_character', 'approved', NOW() - INTERVAL '59 days', NOW()),
    (game6_id, p3_id, 'Finn Quickfingers', 'player_character', 'approved', NOW() - INTERVAL '58 days', NOW()),
    (game6_id, p4_id, 'Bronwyn Stormcaller', 'player_character', 'approved', NOW() - INTERVAL '55 days', NOW()),
    (game6_id, NULL, 'The Dark Lord', 'npc', 'approved', NOW() - INTERVAL '50 days', NOW()),
    (game6_id, NULL, 'Merchant Guild Master', 'npc', 'approved', NOW() - INTERVAL '50 days', NOW());

  -- ============================================
  -- GAME #9: COMPLETED - Tales of the Arcane
  -- ============================================

  -- Player Characters
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game9_id, p1_id, 'Lyra Nightwhisper', 'player_character', 'approved', NOW() - INTERVAL '90 days', NOW()),
    (game9_id, p2_id, 'Theron Brightblade', 'player_character', 'approved', NOW() - INTERVAL '90 days', NOW()),
    (game9_id, p3_id, 'Mira Stormweaver', 'player_character', 'approved', NOW() - INTERVAL '89 days', NOW());

  -- GM NPCs (user_id = NULL)
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game9_id, NULL, 'Archmagus Valdane', 'npc', 'approved', NOW() - INTERVAL '85 days', NOW()),
    (game9_id, NULL, 'The Shadow Council', 'npc', 'approved', NOW() - INTERVAL '85 days', NOW());

END $$;

COMMIT;
