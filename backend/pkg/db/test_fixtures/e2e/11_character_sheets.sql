-- E2E Test Fixture for Character Sheet Management
-- Creates a game with characters that have various character data for testing sheet management
-- Tests: Adding/removing skills, inventory items, numbers management
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
    'This game tests character sheet management: adding skills, items, numbers',
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
  -- Character 1: Has some existing skills and inventory
  -- ============================================
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES (game_id, p1_id, 'Sheet Test Char 1', 'player_character', 'approved', NOW() - INTERVAL '6 days', NOW())
  RETURNING id INTO char1_id;

  -- Character 1: Bio data (public)
  INSERT INTO character_data (character_id, module_type, field_name, field_value, field_type, is_public, created_at, updated_at)
  VALUES
    (char1_id, 'bio', 'background', 'A weathered ranger with keen eyes. Former member of the King''s Guard. Cautious but loyal.', 'text', true, NOW(), NOW());

  -- Character 1: Skills data (4 skills)
  -- The abilities rows that used to sit here were dropped when abilities were
  -- retired; their content folded into skills, which is what the tab shows now.
  --
  -- is_public is false, and must stay false: CharacterSheet.saveJsonField writes
  -- every skills/inventory/numbers row with is_public=false, because access to
  -- those tabs is gated per viewer rather than per row. Only free-text bio
  -- fields carry a real public/private toggle. These rows were seeded `true`,
  -- which no code path produces -- it made GetPublicCharacterData return skills
  -- and items to viewers who cannot see them in production.
  INSERT INTO character_data (character_id, module_type, field_name, field_value, field_type, is_public, created_at, updated_at)
  VALUES
    (char1_id, 'skills', 'skills',
     '[{"id":"skill-1","name":"Archery","rank":"Expert","description":"Master archer","category":"Combat"},{"id":"skill-2","name":"Tracking","rank":"Proficient","description":"Can track creatures","category":"Survival"},{"id":"skill-3","name":"Keen Eye","rank":"Proficient","description":"Can spot hidden details","category":"Perception"},{"id":"skill-4","name":"Quick Draw","rank":"Proficient","description":"Fast weapon draw","category":"Combat"}]',
     'json', false, NOW(), NOW());

  -- Character 1: Inventory data (2 items) and numbers
  INSERT INTO character_data (character_id, module_type, field_name, field_value, field_type, is_public, created_at, updated_at)
  VALUES
    (char1_id, 'inventory', 'items',
     '[{"id":"item-1","name":"Longbow","quantity":1,"description":"Masterwork longbow","category":"Weapon"},{"id":"item-2","name":"Arrows","quantity":20,"description":"Steel-tipped arrows","category":"Ammunition"}]',
     'json', false, NOW(), NOW()),
    (char1_id, 'numbers', 'numbers',
     '[{"id":"number-1","name":"Gold","amount":50},{"id":"number-2","name":"Stress","amount":4,"max":9,"display":"boxes"}]',
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

  -- Character 2: Skills data (4 skills)
  -- skill-8 deliberately still uses the pre-rename `level` key: the rename has
  -- no migration (the key is inside a JSON blob and is resolved on read), so
  -- one unmigrated row keeps that fallback exercised against real data.
  INSERT INTO character_data (character_id, module_type, field_name, field_value, field_type, is_public, created_at, updated_at)
  VALUES
    (char2_id, 'skills', 'skills',
     '[{"id":"skill-5","name":"Arcana","rank":"Expert","description":"Knowledge of magical theory","category":"Academic"},{"id":"skill-6","name":"Fireball","rank":"Expert","description":"Launches a ball of fire","category":"Evocation"},{"id":"skill-7","name":"Shield","rank":"Proficient","description":"Creates magical barrier","category":"Abjuration"},{"id":"skill-8","name":"Arcane Knowledge","level":"Expert","description":"Deep understanding of magic","category":"Academic"}]',
     'json', false, NOW(), NOW());

  -- Character 2: Inventory data (different items)
  INSERT INTO character_data (character_id, module_type, field_name, field_value, field_type, is_public, created_at, updated_at)
  VALUES
    (char2_id, 'inventory', 'items',
     '[{"id":"item-3","name":"Spellbook","quantity":1,"description":"Personal grimoire","category":"Arcane"},{"id":"item-4","name":"Spell Components","quantity":10,"description":"Various magical reagents","category":"Consumable"}]',
     'json', false, NOW(), NOW()),
    (char2_id, 'numbers', 'numbers',
     '[{"id":"number-3","name":"Gold","amount":100},{"id":"number-4","name":"Platinum","amount":5}]',
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
