-- E2E Test Fixture for Loot Tables
-- Creates a game with a GM-owned loot table so the random-roll flow can be
-- exercised end to end: GM opens a character sheet, rolls on the table, and the
-- rolled item lands in that character's inventory.
--
-- Game IDs: 342 (offset by worker: Worker 1 = 10342, Worker 2 = 20342, etc.)
--
-- Dedicated game rather than reusing E2E_CHARACTER_SHEETS: rolling loot mutates
-- a character's inventory, which would pollute the character-sheet spec's
-- assertions about how many items a sheet has.
--
-- Pick a game ID no other fixture uses. Files are applied in glob order and
-- each starts by DELETE-ing its own ID, so a duplicate ID means the later file
-- silently destroys the earlier file's game — 340 already belongs to
-- 19_player_multiple_characters_w*.sql.
--
-- IDEMPOTENT: Safe to run multiple times - deletes existing data before recreating

BEGIN;

-- Deleting the game cascades to characters, loot tables and their contents.
DELETE FROM games WHERE id = 342;

DO $$
DECLARE
  gm_id INTEGER;
  p1_id INTEGER;
  game_id INTEGER := 342;
  char_id INTEGER;
  single_table_id INTEGER;
  empty_table_id INTEGER;
BEGIN
  SELECT id INTO gm_id FROM users WHERE email = 'test_gm@example.com';
  SELECT id INTO p1_id FROM users WHERE email = 'test_player1@example.com';

  INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    game_id,
    'E2E Test: Loot Tables',
    'This game tests loot table management and rolling loot onto a character sheet',
    'Test',
    gm_id,
    3,
    'in_progress',
    true,
    NOW() - INTERVAL '7 days',
    NOW()
  );

  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES (game_id, p1_id, 'player', 'active', NOW() - INTERVAL '6 days');

  -- Character with an empty inventory, so any item found after a roll must be
  -- the one that was just rolled.
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES (game_id, p1_id, 'Loot Test Char', 'player_character', 'approved', NOW() - INTERVAL '6 days', NOW())
  RETURNING id INTO char_id;

  -- ============================================
  -- Loot table with exactly one item
  -- ============================================
  -- A single item makes the random roll deterministic: whatever the server
  -- picks, it must be this one, so the test can assert on a specific name.
  INSERT INTO game_loot_tables (game_id, name, created_at, updated_at)
  VALUES (game_id, 'E2E Single Item Table', NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days')
  RETURNING id INTO single_table_id;

  -- `data` is the GM-authored JSON blob the frontend parses into an inventory
  -- item. Keys must match InventoryItem fields or the roll silently adds a
  -- blank item.
  INSERT INTO game_loot_table_contents (loot_table_id, name, data)
  VALUES (
    single_table_id,
    'Enchanted Compass',
    '{"name":"Enchanted Compass","description":"Always points toward what you seek.","quantity":1,"category":"Tool","value":150,"weight":0.5}'
  );

  -- ============================================
  -- Empty loot table
  -- ============================================
  -- Rolling on this returns 400; the UI must surface that as an error toast
  -- rather than failing silently.
  INSERT INTO game_loot_tables (game_id, name, created_at, updated_at)
  VALUES (game_id, 'E2E Empty Table', NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days')
  RETURNING id INTO empty_table_id;

  RAISE NOTICE 'Created E2E loot table game % (character %, tables % and %)',
    game_id, char_id, single_table_id, empty_table_id;
END $$;

COMMIT;
