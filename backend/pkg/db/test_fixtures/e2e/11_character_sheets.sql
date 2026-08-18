-- E2E Test Fixture for Character Sheet Management
-- Creates a game with characters that have various character data for testing sheet management
-- Tests: Adding/removing abilities, skills, inventory items, currency management
--
-- Game IDs: 328 (offset by worker: Worker 1 = 1328, Worker 2 = 2328, etc.)
--
-- IDEMPOTENT: Safe to run multiple times - deletes existing data before recreating

BEGIN;

-- Delete existing character sheet test game to prevent duplicates
DELETE FROM games WHERE id = 328;

DO $$
DECLARE
  gm_id INTEGER;
  p1_id INTEGER;
  p2_id INTEGER;
  game_id INTEGER := 328;
  char1_id INTEGER;
  char2_id INTEGER;
  char_empty_id INTEGER;
BEGIN
  -- Get user IDs
  SELECT id INTO gm_id FROM users WHERE email = 'test_gm@example.com';
  SELECT id INTO p1_id FROM users WHERE email = 'test_player1@example.com';
  SELECT id INTO p2_id FROM users WHERE email = 'test_player2@example.com';

  -- ============================================
  -- E2E Game: For Character Sheet Testing
  -- ============================================
  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    game_id,
    'E2E Test: Character Sheets',
    'This game tests character sheet management: adding abilities, skills, items, currency',
    'Test',
    gm_id,
    3,
    'in_progress',
    true,
    NOW() - INTERVAL '7 days',
    NOW()
  );

  -- Add participants
  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES
    (game_id, p1_id, 'player', 'active', NOW() - INTERVAL '6 days'),
    (game_id, p2_id, 'player', 'active', NOW() - INTERVAL '6 days');

  -- ============================================
  -- Character 1: Has some existing abilities and inventory
  -- ============================================
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES (game_id, p1_id, 'Sheet Test Char 1', 'player_character', 'approved', NOW() - INTERVAL '6 days', NOW())
  RETURNING id INTO char1_id;

  -- Character 1: Bio data (public)
  INSERT INTO character_data (character_id, module_type, field_name, field_value, field_type, is_public, created_at, updated_at)
  VALUES
    (char1_id, 'bio', 'background', 'A weathered ranger with keen eyes. Former member of the King''s Guard. Cautious but loyal.', 'text', true, NOW(), NOW());

  -- Character 1: Abilities data (2 abilities, 2 skills)
  INSERT INTO character_data (character_id, module_type, field_name, field_value, field_type, is_public, created_at, updated_at)
  VALUES
    (char1_id, 'abilities', 'abilities',
     '[{"id":"ability-1","name":"Keen Eye","description":"Can spot hidden details","type":"innate","active":true},{"id":"ability-2","name":"Quick Draw","description":"Fast weapon draw","type":"learned","active":true}]',
     'json', true, NOW(), NOW()),
    (char1_id, 'skills', 'skills',
     '[{"id":"skill-1","name":"Archery","level":"Expert","description":"Master archer","category":"Combat"},{"id":"skill-2","name":"Tracking","level":"Proficient","description":"Can track creatures","category":"Survival"}]',
     'json', true, NOW(), NOW());

  -- Character 1: Inventory data (2 items, currency)
  INSERT INTO character_data (character_id, module_type, field_name, field_value, field_type, is_public, created_at, updated_at)
  VALUES
    (char1_id, 'inventory', 'items',
     '[{"id":"item-1","name":"Longbow","quantity":1,"description":"Masterwork longbow","category":"Weapon","equipped":true},{"id":"item-2","name":"Arrows","quantity":20,"description":"Steel-tipped arrows","category":"Ammunition","equipped":false}]',
     'json', true, NOW(), NOW()),
    (char1_id, 'currency', 'currency',
     '[{"id":"currency-1","type":"Gold","amount":50},{"id":"currency-2","type":"Silver","amount":25}]',
     'json', false, NOW(), NOW());

  -- ============================================
  -- Character 2: Has different data for comparison
  -- ============================================
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES (game_id, p2_id, 'Sheet Test Char 2', 'player_character', 'approved', NOW() - INTERVAL '6 days', NOW())
  RETURNING id INTO char2_id;

  -- Character 2: Bio data
  INSERT INTO character_data (character_id, module_type, field_name, field_value, field_type, is_public, created_at, updated_at)
  VALUES
    (char2_id, 'bio', 'background', 'A mysterious mage in dark robes. Scholarly and reserved.', 'text', true, NOW(), NOW());

  -- Character 2: Abilities data (3 abilities, 1 skill)
  INSERT INTO character_data (character_id, module_type, field_name, field_value, field_type, is_public, created_at, updated_at)
  VALUES
    (char2_id, 'abilities', 'abilities',
     '[{"id":"ability-3","name":"Fireball","description":"Launches a ball of fire","type":"learned","active":true},{"id":"ability-4","name":"Shield","description":"Creates magical barrier","type":"learned","active":true},{"id":"ability-5","name":"Arcane Knowledge","description":"Deep understanding of magic","type":"innate","active":true}]',
     'json', true, NOW(), NOW()),
    (char2_id, 'skills', 'skills',
     '[{"id":"skill-3","name":"Arcana","level":"Expert","description":"Knowledge of magical theory","category":"Academic"}]',
     'json', true, NOW(), NOW());

  -- Character 2: Inventory data (different items)
  INSERT INTO character_data (character_id, module_type, field_name, field_value, field_type, is_public, created_at, updated_at)
  VALUES
    (char2_id, 'inventory', 'items',
     '[{"id":"item-3","name":"Spellbook","quantity":1,"description":"Personal grimoire","category":"Arcane","equipped":true},{"id":"item-4","name":"Spell Components","quantity":10,"description":"Various magical reagents","category":"Consumable","equipped":false}]',
     'json', true, NOW(), NOW()),
    (char2_id, 'currency', 'currency',
     '[{"id":"currency-3","type":"Gold","amount":100},{"id":"currency-4","type":"Platinum","amount":5}]',
     'json', false, NOW(), NOW());

  -- ============================================
  -- Character 3: Empty sheet for fresh testing
  -- ============================================
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES (game_id, p1_id, 'Empty Sheet Char', 'player_character', 'approved', NOW() - INTERVAL '5 days', NOW())
  RETURNING id INTO char_empty_id;

  -- Character 3: Only minimal bio (rest will be added via E2E tests)
  INSERT INTO character_data (character_id, module_type, field_name, field_value, field_type, is_public, created_at, updated_at)
  VALUES
    (char_empty_id, 'bio', 'background', 'A new adventurer', 'text', true, NOW(), NOW());

END $$;

-- Reset the games sequence to prevent duplicate key errors
-- This ensures new game creations don't collide with hardcoded fixture IDs
SELECT setval('games_id_seq', (SELECT MAX(id) FROM games) + 1);

COMMIT;

-- Success message
SELECT 'E2E Character Sheets fixture created successfully!' as message;
