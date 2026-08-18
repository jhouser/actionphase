# Test Data Management Guide

This document outlines the test data structure for ActionPhase and provides instructions for creating, managing, and resetting test data to ensure comprehensive testing of all features and edge cases.

## Overview

Test data is critical for validating the various states and phases of games in ActionPhase. This guide covers:
- Test data requirements and structure
- SQL scripts for creating test fixtures
- Procedures for resetting to known states
- Edge cases and scenarios to test

## Test Data Requirements

### Users

We need the following user types:

1. **Game Master (GM)** - `test_gm@example.com`
   - Password: `testpassword123`
   - Creates and manages games
   - Controls NPCs

2. **Players (4-5)** - `test_player1@example.com` through `test_player5@example.com`
   - Password: `testpassword123`
   - Each has one player character

3. **Audience Member** - `test_audience@example.com`
   - Password: `testpassword123`
   - Observes games without direct participation

### Games Coverage

We need games in the following states:

#### 1. Game States
- **Recruiting** - Accepting applications
- **Running** - Active gameplay
- **Paused** - Temporarily stopped
- **Completed** - Finished game
- **Cancelled** - Cancelled game

#### 2. Phase Types
- **Common Room** - Active discussion phase
- **Action** - Active action submission phase
- **Results** - Active results publication phase

#### 3. Phase History
- Games with previous Common Room phases
- Games with previous Action phases
- Games with previous Results phases
- Games with mixed phase history

### Required Test Scenarios

#### Minimal Coverage
- ✓ Game #1: Running, Active Common Room Phase
- ✓ Game #2: Running, Active Action Phase (with submitted actions)
- ✓ Game #3: Running, Active Results Phase (with published results)
- ✓ Game #4: Running, Previous Common Room + Active Action Phase
- ✓ Game #5: Running, Previous Action + Previous Results + Active Common Room
- ✓ Game #6: Running, Multiple previous phases of mixed types
- ✓ Game #7: Recruiting (no phases yet)
- ✓ Game #8: Paused (with some phase history)
- ✓ Game #9: Completed (full phase history)

#### Additional Coverage
- Game with deadline approaching (within 1 hour)
- Game with expired deadline
- Game with custom phase titles and descriptions
- Game with many phases (10+) for pagination testing

## Test Data SQL Scripts

### Location
All test data scripts are stored in:
```
backend/pkg/db/test_fixtures/
```

### File Structure
```
test_fixtures/
├── 00_reset.sql           # Cleans all test data
├── 01_users.sql           # Creates test users
├── 02_games_recruiting.sql # Creates recruiting games
├── 03_games_running.sql   # Creates running games with phases
├── 04_characters.sql      # Creates characters and NPCs
├── 05_actions.sql         # Creates action submissions
├── 06_results.sql         # Creates action results
└── apply_all.sh           # Bash script to apply all fixtures
```

### 00_reset.sql - Clean Test Data

```sql
-- Reset Test Data
-- This script removes all test data while preserving the schema

BEGIN;

-- Delete in reverse dependency order
DELETE FROM phase_transitions WHERE game_id IN (SELECT id FROM games WHERE gm_user_id IN (SELECT id FROM users WHERE email LIKE 'test_%@example.com'));
DELETE FROM action_results WHERE game_id IN (SELECT id FROM games WHERE gm_user_id IN (SELECT id FROM users WHERE email LIKE 'test_%@example.com'));
DELETE FROM action_submissions WHERE game_id IN (SELECT id FROM games WHERE gm_user_id IN (SELECT id FROM users WHERE email LIKE 'test_%@example.com'));
DELETE FROM character_data WHERE character_id IN (SELECT id FROM characters WHERE game_id IN (SELECT id FROM games WHERE gm_user_id IN (SELECT id FROM users WHERE email LIKE 'test_%@example.com')));
DELETE FROM characters WHERE game_id IN (SELECT id FROM games WHERE gm_user_id IN (SELECT id FROM users WHERE email LIKE 'test_%@example.com'));
DELETE FROM game_phases WHERE game_id IN (SELECT id FROM games WHERE gm_user_id IN (SELECT id FROM users WHERE email LIKE 'test_%@example.com'));
DELETE FROM game_participants WHERE game_id IN (SELECT id FROM games WHERE gm_user_id IN (SELECT id FROM users WHERE email LIKE 'test_%@example.com'));
DELETE FROM game_applications WHERE game_id IN (SELECT id FROM games WHERE gm_user_id IN (SELECT id FROM users WHERE email LIKE 'test_%@example.com'));
DELETE FROM games WHERE gm_user_id IN (SELECT id FROM users WHERE email LIKE 'test_%@example.com');
DELETE FROM sessions WHERE user_id IN (SELECT id FROM users WHERE email LIKE 'test_%@example.com');
DELETE FROM users WHERE email LIKE 'test_%@example.com';

COMMIT;
```

### 01_users.sql - Create Test Users

```sql
-- Create Test Users
-- Password for all: testpassword123
-- Hashed with bcrypt cost 10

BEGIN;

-- Game Master
INSERT INTO users (username, email, password, created_at)
VALUES
  ('TestGM', 'test_gm@example.com', '$2a$10$7LH6DSL0M6Dln50UDtKzY.rs7J3a7S/gAZVONnk6QZvouo0pUx/..', NOW());

-- Players
INSERT INTO users (username, email, password, created_at, updated_at)
VALUES
  ('TestPlayer1', 'test_player1@example.com', '$2a$10$7LH6DSL0M6Dln50UDtKzY.rs7J3a7S/gAZVONnk6QZvouo0pUx/..', NOW(), NOW()),
  ('TestPlayer2', 'test_player2@example.com', '$2a$10$7LH6DSL0M6Dln50UDtKzY.rs7J3a7S/gAZVONnk6QZvouo0pUx/..', NOW(), NOW()),
  ('TestPlayer3', 'test_player3@example.com', '$2a$10$7LH6DSL0M6Dln50UDtKzY.rs7J3a7S/gAZVONnk6QZvouo0pUx/..', NOW(), NOW()),
  ('TestPlayer4', 'test_player4@example.com', '$2a$10$7LH6DSL0M6Dln50UDtKzY.rs7J3a7S/gAZVONnk6QZvouo0pUx/..', NOW(), NOW()),
  ('TestPlayer5', 'test_player5@example.com', '$2a$10$7LH6DSL0M6Dln50UDtKzY.rs7J3a7S/gAZVONnk6QZvouo0pUx/..', NOW());

-- Audience Member
INSERT INTO users (username, email, password, created_at, updated_at)
VALUES
  ('TestAudience', 'test_audience@example.com', '$2a$10$7LH6DSL0M6Dln50UDtKzY.rs7J3a7S/gAZVONnk6QZvouo0pUx/..', NOW());

COMMIT;
```

### 02_games_recruiting.sql - Create Recruiting Games

```sql
-- Create Recruiting Games

BEGIN;

-- Get GM user ID
DO $$
DECLARE
  gm_id INTEGER;
BEGIN
  SELECT id INTO gm_id FROM users WHERE email = 'test_gm@example.com';

  -- Game #7: Recruiting game
  INSERT INTO games (title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    'The Mystery of Blackwood Manor',
    'A gothic horror mystery set in a Victorian mansion. Applications open!',
    'Call of Cthulhu 7e',
    gm_id,
    5,
    'recruitment',
    true,
    NOW() - INTERVAL '3 days',
    NOW()
  );

  -- Game #10: Recruiting with private visibility
  INSERT INTO games (title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    'Secret Campaign',
    'A private game for invited players only.',
    'D&D 5e',
    gm_id,
    4,
    'recruitment',
    false,
    NOW() - INTERVAL '1 day',
    NOW()
  );
END $$;

COMMIT;
```

### 03_games_running.sql - Create Running Games with Phases

```sql
-- Create Running Games with Phase Structures

BEGIN;

-- Get user IDs
DO $$
DECLARE
  gm_id INTEGER;
  p1_id INTEGER;
  p2_id INTEGER;
  p3_id INTEGER;
  p4_id INTEGER;
  game1_id INTEGER;
  game2_id INTEGER;
  game3_id INTEGER;
  game4_id INTEGER;
  game5_id INTEGER;
  game6_id INTEGER;
  game8_id INTEGER;
  game9_id INTEGER;
BEGIN
  SELECT id INTO gm_id FROM users WHERE email = 'test_gm@example.com';
  SELECT id INTO p1_id FROM users WHERE email = 'test_player1@example.com';
  SELECT id INTO p2_id FROM users WHERE email = 'test_player2@example.com';
  SELECT id INTO p3_id FROM users WHERE email = 'test_player3@example.com';
  SELECT id INTO p4_id FROM users WHERE email = 'test_player4@example.com';

  -- ============================================
  -- GAME #1: Active Common Room Phase
  -- ============================================
  INSERT INTO games (title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    'Shadows Over Innsmouth',
    'A Lovecraftian horror investigation in a cursed fishing town.',
    'Call of Cthulhu 7e',
    gm_id,
    4,
    'in_progress',
    true,
    NOW() - INTERVAL '7 days',
    NOW()
  ) RETURNING id INTO game1_id;

  -- Add participants
  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES
    (game1_id, p1_id, 'player', 'active', NOW() - INTERVAL '6 days'),
    (game1_id, p2_id, 'player', 'active', NOW() - INTERVAL '6 days'),
    (game1_id, p3_id, 'player', 'active', NOW() - INTERVAL '6 days');

  -- Phase 1: Active Common Room with custom title
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, deadline, is_active, created_at)
  VALUES (
    game1_id,
    'common_room',
    1,
    'Arrival at the Harbor',
    'The investigators arrive at Innsmouth harbor on a foggy evening. The locals eye them suspiciously.',
    NOW() - INTERVAL '2 hours',
    NOW() + INTERVAL '22 hours',
    true,
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
    (game2_id, p4_id, 'player', 'active', NOW() - INTERVAL '9 days');

  -- Previous Phase 1: Common Room
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, start_time, end_time, deadline, is_active, created_at)
  VALUES (
    game2_id,
    'common_room',
    1,
    'Casing the Bank',
    NOW() - INTERVAL '3 days',
    NOW() - INTERVAL '2 days',
    NOW() - INTERVAL '2 days',
    false,
    NOW() - INTERVAL '3 days'
  );

  -- Active Phase 2: Action Phase
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, deadline, is_active, created_at)
  VALUES (
    game2_id,
    'action',
    2,
    'Execute the Plan',
    'Each crew member executes their part of the heist. Submit your actions now!',
    NOW() - INTERVAL '4 hours',
    NOW() + INTERVAL '20 hours',
    true,
    NOW() - INTERVAL '4 hours'
  );

  -- ============================================
  -- GAME #3: Active Results Phase
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
    (game3_id, p3_id, 'player', 'active', NOW() - INTERVAL '13 days');

  -- Previous phases
  INSERT INTO game_phases (game_id, phase_type, phase_number, start_time, end_time, deadline, is_active, created_at)
  VALUES
    (game3_id, 'common_room', 1, NOW() - INTERVAL '5 days', NOW() - INTERVAL '4 days', NOW() - INTERVAL '4 days', false, NOW() - INTERVAL '5 days'),
    (game3_id, 'action', 2, NOW() - INTERVAL '4 days', NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days', false, NOW() - INTERVAL '4 days');

  -- Active Phase 3: Results
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, start_time, is_active, created_at)
  VALUES (
    game3_id,
    'results',
    3,
    'The Truth Revealed',
    NOW() - INTERVAL '6 hours',
    true,
    NOW() - INTERVAL '6 hours'
  );

  -- ============================================
  -- GAME #4: Previous Common Room + Active Action
  -- ============================================
  INSERT INTO games (title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    'Court of Shadows',
    'Political intrigue in a dark fantasy kingdom.',
    'Vampire: The Masquerade',
    gm_id,
    6,
    'in_progress',
    true,
    NOW() - INTERVAL '20 days',
    NOW()
  ) RETURNING id INTO game4_id;

  -- Add participants
  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES
    (game4_id, p1_id, 'player', 'active', NOW() - INTERVAL '19 days'),
    (game4_id, p2_id, 'player', 'active', NOW() - INTERVAL '19 days'),
    (game4_id, p3_id, 'player', 'active', NOW() - INTERVAL '19 days'),
    (game4_id, p4_id, 'player', 'active', NOW() - INTERVAL '18 days');

  -- Previous Phase 1: Common Room
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, end_time, deadline, is_active, created_at)
  VALUES (
    game4_id,
    'common_room',
    1,
    'The Grand Ball',
    'A lavish party where alliances are formed and secrets traded.',
    NOW() - INTERVAL '8 days',
    NOW() - INTERVAL '6 days',
    NOW() - INTERVAL '6 days',
    false,
    NOW() - INTERVAL '8 days'
  );

  -- Active Phase 2: Action
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, start_time, deadline, is_active, created_at)
  VALUES (
    game4_id,
    'action',
    2,
    'Behind Closed Doors',
    NOW() - INTERVAL '1 hour',
    NOW() + INTERVAL '47 hours',
    true,
    NOW() - INTERVAL '1 hour'
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
    (game5_id, p3_id, 'player', 'active', NOW() - INTERVAL '44 days');

  -- Phase History
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, start_time, end_time, is_active, created_at)
  VALUES
    (game5_id, 'common_room', 1, 'The Tavern Meeting', NOW() - INTERVAL '30 days', NOW() - INTERVAL '28 days', false, NOW() - INTERVAL '30 days'),
    (game5_id, 'action', 2, 'Journey to Krag', NOW() - INTERVAL '28 days', NOW() - INTERVAL '25 days', false, NOW() - INTERVAL '28 days'),
    (game5_id, 'results', 3, 'Ambush on the Road', NOW() - INTERVAL '25 days', NOW() - INTERVAL '23 days', false, NOW() - INTERVAL '25 days'),
    (game5_id, 'common_room', 4, 'Healing and Planning', NOW() - INTERVAL '23 days', NOW() - INTERVAL '20 days', false, NOW() - INTERVAL '23 days'),
    (game5_id, 'action', 5, 'Infiltrate the Caves', NOW() - INTERVAL '20 days', NOW() - INTERVAL '18 days', false, NOW() - INTERVAL '20 days'),
    (game5_id, 'results', 6, 'Discovery of the Lair', NOW() - INTERVAL '18 days', NOW() - INTERVAL '15 days', false, NOW() - INTERVAL '18 days');

  -- Active Phase 7: Common Room
  INSERT INTO game_phases (game_id, phase_type, phase_number, title, start_time, is_active, created_at)
  VALUES (
    game5_id,
    'common_room',
    7,
    'Final Preparations',
    NOW() - INTERVAL '3 hours',
    true,
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
    (game6_id, p4_id, 'player', 'active', NOW() - INTERVAL '55 days');

  -- Many phases (12 total)
  INSERT INTO game_phases (game_id, phase_type, phase_number, start_time, end_time, is_active, created_at)
  VALUES
    (game6_id, 'common_room', 1, NOW() - INTERVAL '55 days', NOW() - INTERVAL '53 days', false, NOW() - INTERVAL '55 days'),
    (game6_id, 'action', 2, NOW() - INTERVAL '53 days', NOW() - INTERVAL '50 days', false, NOW() - INTERVAL '53 days'),
    (game6_id, 'results', 3, NOW() - INTERVAL '50 days', NOW() - INTERVAL '48 days', false, NOW() - INTERVAL '50 days'),
    (game6_id, 'common_room', 4, NOW() - INTERVAL '48 days', NOW() - INTERVAL '45 days', false, NOW() - INTERVAL '48 days'),
    (game6_id, 'action', 5, NOW() - INTERVAL '45 days', NOW() - INTERVAL '42 days', false, NOW() - INTERVAL '45 days'),
    (game6_id, 'results', 6, NOW() - INTERVAL '42 days', NOW() - INTERVAL '40 days', false, NOW() - INTERVAL '42 days'),
    (game6_id, 'common_room', 7, NOW() - INTERVAL '40 days', NOW() - INTERVAL '37 days', false, NOW() - INTERVAL '40 days'),
    (game6_id, 'action', 8, NOW() - INTERVAL '37 days', NOW() - INTERVAL '34 days', false, NOW() - INTERVAL '37 days'),
    (game6_id, 'results', 9, NOW() - INTERVAL '34 days', NOW() - INTERVAL '31 days', false, NOW() - INTERVAL '34 days'),
    (game6_id, 'common_room', 10, NOW() - INTERVAL '31 days', NOW() - INTERVAL '28 days', false, NOW() - INTERVAL '31 days'),
    (game6_id, 'action', 11, NOW() - INTERVAL '28 days', NOW() - INTERVAL '25 days', false, NOW() - INTERVAL '28 days');

  -- Active Phase 12: Results
  INSERT INTO game_phases (game_id, phase_type, phase_number, start_time, is_active, created_at)
  VALUES (
    game6_id,
    'results',
    12,
    NOW() - INTERVAL '12 hours',
    true,
    NOW() - INTERVAL '12 hours'
  );

  -- ============================================
  -- GAME #8: Paused with History
  -- ============================================
  INSERT INTO games (title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
  VALUES (
    'On Hold: The Frozen North',
    'Campaign temporarily paused due to GM availability.',
    'D&D 5e',
    gm_id,
    4,
    'paused',
    true,
    NOW() - INTERVAL '35 days',
    NOW() - INTERVAL '7 days'
  ) RETURNING id INTO game8_id;

  -- Add participants
  INSERT INTO game_participants (game_id, user_id, role, status, joined_at)
  VALUES
    (game8_id, p1_id, 'player', 'active', NOW() - INTERVAL '34 days'),
    (game8_id, p2_id, 'player', 'active', NOW() - INTERVAL '34 days'));

  -- Previous phases before pause
  INSERT INTO game_phases (game_id, phase_type, phase_number, start_time, end_time, is_active, created_at)
  VALUES
    (game8_id, 'common_room', 1, NOW() - INTERVAL '30 days', NOW() - INTERVAL '28 days', false, NOW() - INTERVAL '30 days'),
    (game8_id, 'action', 2, NOW() - INTERVAL '28 days', NOW() - INTERVAL '25 days', false, NOW() - INTERVAL '28 days'),
    (game8_id, 'results', 3, NOW() - INTERVAL '25 days', NOW() - INTERVAL '23 days', false, NOW() - INTERVAL '25 days'),
    (game8_id, 'common_room', 4, NOW() - INTERVAL '23 days', NOW() - INTERVAL '20 days', false, NOW() - INTERVAL '23 days');

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
    (game9_id, p3_id, 'player', 'active', NOW() - INTERVAL '88 days');

  -- Full phase history
  INSERT INTO game_phases (game_id, phase_type, phase_number, start_time, end_time, is_active, created_at)
  VALUES
    (game9_id, 'common_room', 1, NOW() - INTERVAL '85 days', NOW() - INTERVAL '82 days', false, NOW() - INTERVAL '85 days'),
    (game9_id, 'action', 2, NOW() - INTERVAL '82 days', NOW() - INTERVAL '79 days', false, NOW() - INTERVAL '82 days'),
    (game9_id, 'results', 3, NOW() - INTERVAL '79 days', NOW() - INTERVAL '76 days', false, NOW() - INTERVAL '79 days'),
    (game9_id, 'common_room', 4, NOW() - INTERVAL '76 days', NOW() - INTERVAL '73 days', false, NOW() - INTERVAL '76 days'),
    (game9_id, 'action', 5, NOW() - INTERVAL '73 days', NOW() - INTERVAL '70 days', false, NOW() - INTERVAL '73 days'),
    (game9_id, 'results', 6, NOW() - INTERVAL '70 days', NOW() - INTERVAL '67 days', false, NOW() - INTERVAL '70 days'),
    (game9_id, 'common_room', 7, NOW() - INTERVAL '67 days', NOW() - INTERVAL '64 days', false, NOW() - INTERVAL '67 days'),
    (game9_id, 'action', 8, NOW() - INTERVAL '64 days', NOW() - INTERVAL '61 days', false, NOW() - INTERVAL '64 days'),
    (game9_id, 'results', 9, NOW() - INTERVAL '61 days', NOW() - INTERVAL '58 days', false, NOW() - INTERVAL '61 days');

END $$;

COMMIT;
```

### 04_characters.sql - Create Characters and NPCs

```sql
-- Create Characters and NPCs

BEGIN;

DO $$
DECLARE
  gm_id INTEGER;
  p1_id INTEGER;
  p2_id INTEGER;
  p3_id INTEGER;
  p4_id INTEGER;
  game1_id INTEGER;
  game2_id INTEGER;
  game3_id INTEGER;
  game4_id INTEGER;
  game5_id INTEGER;
  game6_id INTEGER;
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

  -- Get game IDs
  SELECT id INTO game1_id FROM games WHERE title = 'Shadows Over Innsmouth';
  SELECT id INTO game2_id FROM games WHERE title = 'The Heist at Goldstone Bank';
  SELECT id INTO game3_id FROM games WHERE title = 'Starfall Station';
  SELECT id INTO game4_id FROM games WHERE title = 'Court of Shadows';
  SELECT id INTO game5_id FROM games WHERE title = 'The Dragon of Mount Krag';
  SELECT id INTO game6_id FROM games WHERE title = 'Chronicles of Westmarch';

  -- ============================================
  -- GAME #1: Shadows Over Innsmouth
  -- ============================================

  -- Player Characters
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game1_id, p1_id, 'Detective Marcus Kane', 'player_character', 'approved', NOW() - INTERVAL '6 days', NOW()),
    (game1_id, p2_id, 'Dr. Sarah Chen', 'player_character', 'approved', NOW() - INTERVAL '6 days', NOW()),
    (game1_id, p3_id, 'Father O''Brien', 'player_character', 'approved', NOW() - INTERVAL '6 days', NOW());

  -- GM NPCs
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game1_id, NULL, 'Captain Obed Marsh', 'npc_gm', 'approved', NOW() - INTERVAL '5 days', NOW()),
    (game1_id, NULL, 'The Fishmonger', 'npc_gm', 'approved', NOW() - INTERVAL '5 days', NOW());

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

  -- GM NPCs only
  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game2_id, NULL, 'Inspector Dalton', 'npc_gm', 'approved', NOW() - INTERVAL '8 days', NOW()),
    (game2_id, NULL, 'Bones (Contact)', 'npc_gm', 'approved', NOW() - INTERVAL '7 days', NOW()),
    (game2_id, NULL, 'Whistle (Lookout)', 'npc_gm', 'approved', NOW() - INTERVAL '7 days', NOW());

  -- Character data examples
  INSERT INTO character_data (character_id, module_type, field_name, field_value, field_type, is_public, created_at, updated_at)
  VALUES
    (char1_id, 'bio', 'background', 'A Whisper who keeps company with things better left unspoken...', 'text', true, NOW(), NOW()),
    (char1_id, 'skills', 'skills', '[{"id":"...","name":"Compel","level":"2","category":"Arcane"}, ...]', 'json', true, NOW(), NOW()),
    (char1_id, 'inventory', 'items', '[{"id":"...","name":"Spirit Bottle","quantity":1,"equipped":false}, ...]', 'json', true, NOW(), NOW()),
    (char1_id, 'currency', 'currency', '[{"id":"...","type":"Coin","amount":6}, {"id":"...","type":"Stress","amount":4}]', 'json', false, NOW(), NOW());
    -- (char2_id rows follow the same shape; see the fixture for full values)

-- NOTE: module_type is limited to bio/notes/abilities/skills/inventory/currency, and each
-- stat collection is ONE row holding a JSON array whose element shape must match what the
-- app writes (see CHARACTER_MODULES and the Add*Modal components). Fixture rows using other
-- module types or field names are unreadable by every code path and hide real bugs.

  -- ============================================
  -- GAME #3: Starfall Station
  -- ============================================

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game3_id, p1_id, 'Commander Vasquez', 'player_character', 'approved', NOW() - INTERVAL '13 days', NOW()),
    (game3_id, p2_id, 'Engineer Patel', 'player_character', 'approved', NOW() - INTERVAL '13 days', NOW()),
    (game3_id, p3_id, 'Dr. Kim', 'player_character', 'approved', NOW() - INTERVAL '13 days', NOW()),
    (game3_id, NULL, 'The Alien Entity', 'npc_gm', 'approved', NOW() - INTERVAL '12 days', NOW());

  -- ============================================
  -- GAME #4: Court of Shadows
  -- ============================================

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game4_id, p1_id, 'Lord Ravenna', 'player_character', 'approved', NOW() - INTERVAL '19 days', NOW()),
    (game4_id, p2_id, 'Countess Nyx', 'player_character', 'approved', NOW() - INTERVAL '19 days', NOW()),
    (game4_id, p3_id, 'Baron Ash', 'player_character', 'approved', NOW() - INTERVAL '19 days', NOW()),
    (game4_id, p4_id, 'Lady Morgana', 'player_character', 'approved', NOW() - INTERVAL '18 days', NOW()),
    (game4_id, NULL, 'Prince Valdric', 'npc_gm', 'approved', NOW() - INTERVAL '18 days', NOW()),
    (game4_id, NULL, 'The Archbishop', 'npc_gm', 'approved', NOW() - INTERVAL '18 days', NOW());

  -- ============================================
  -- GAME #5: Dragon of Mount Krag
  -- ============================================

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game5_id, p1_id, 'Thorin Ironforge', 'player_character', 'approved', NOW() - INTERVAL '44 days', NOW()),
    (game5_id, p2_id, 'Elara Moonshadow', 'player_character', 'approved', NOW() - INTERVAL '44 days', NOW()),
    (game5_id, p3_id, 'Grimm the Bold', 'player_character', 'approved', NOW() - INTERVAL '44 days', NOW()),
    (game5_id, NULL, 'Vorathax the Ancient', 'npc_gm', 'approved', NOW() - INTERVAL '40 days', NOW());

  -- ============================================
  -- GAME #6: Chronicles of Westmarch
  -- ============================================

  INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
  VALUES
    (game6_id, p1_id, 'Sir Aldric', 'player_character', 'approved', NOW() - INTERVAL '59 days', NOW()),
    (game6_id, p2_id, 'Zara the Mystic', 'player_character', 'approved', NOW() - INTERVAL '59 days', NOW()),
    (game6_id, p3_id, 'Finn Quickfingers', 'player_character', 'approved', NOW() - INTERVAL '58 days', NOW()),
    (game6_id, p4_id, 'Bronwyn Stormcaller', 'player_character', 'approved', NOW() - INTERVAL '55 days', NOW()),
    (game6_id, NULL, 'The Dark Lord', 'npc_gm', 'approved', NOW() - INTERVAL '50 days', NOW()),
    (game6_id, NULL, 'Merchant Guild Master', 'npc_gm', 'approved', NOW() - INTERVAL '50 days', NOW());

END $$;

COMMIT;
```

### 05_actions.sql - Create Action Submissions

```sql
-- Create Action Submissions for Action Phases

BEGIN;

DO $$
DECLARE
  p1_id INTEGER;
  p2_id INTEGER;
  p3_id INTEGER;
  p4_id INTEGER;
  game2_id INTEGER;
  game4_id INTEGER;
  phase2_id INTEGER;
  phase4_id INTEGER;
  char1_id INTEGER;
  char2_id INTEGER;
  char3_id INTEGER;
  char4_id INTEGER;
  char5_id INTEGER;
  char6_id INTEGER;
BEGIN
  -- Get user IDs
  SELECT id INTO p1_id FROM users WHERE email = 'test_player1@example.com';
  SELECT id INTO p2_id FROM users WHERE email = 'test_player2@example.com';
  SELECT id INTO p3_id FROM users WHERE email = 'test_player3@example.com';
  SELECT id INTO p4_id FROM users WHERE email = 'test_player4@example.com';

  -- Get game IDs
  SELECT id INTO game2_id FROM games WHERE title = 'The Heist at Goldstone Bank';
  SELECT id INTO game4_id FROM games WHERE title = 'Court of Shadows';

  -- Get active action phase IDs
  SELECT id INTO phase2_id FROM game_phases WHERE game_id = game2_id AND phase_number = 2;
  SELECT id INTO phase4_id FROM game_phases WHERE game_id = game4_id AND phase_number = 2;

  -- Get character IDs for Game 2
  SELECT id INTO char1_id FROM characters WHERE game_id = game2_id AND name = 'Shade (Whisper)';
  SELECT id INTO char2_id FROM characters WHERE game_id = game2_id AND name = 'Rook (Hound)';
  SELECT id INTO char3_id FROM characters WHERE game_id = game2_id AND name = 'Vex (Leech)';
  SELECT id INTO char4_id FROM characters WHERE game_id = game2_id AND name = 'Silk (Spider)';

  -- ============================================
  -- GAME #2: Heist Actions
  -- ============================================

  -- Player 1's action (submitted)
  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, is_draft, submitted_at, updated_at)
  VALUES (
    game2_id,
    p1_id,
    phase2_id,
    char1_id,
    E'Using my supernatural senses, I''ll scan the vault room for any lingering spirits or magical wards before the team enters. If I detect anything, I''ll attempt to commune with or dispel it using my Compel ability.\n\nI''ll position myself near the entrance as a lookout while maintaining focus on the ethereal plane. If guards approach, I''ll create a minor distraction using ghost sounds.',
    false,
    NOW() - INTERVAL '2 hours',
    NOW() - INTERVAL '2 hours'
  );

  -- Player 2's action (submitted)
  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, is_draft, submitted_at, updated_at)
  VALUES (
    game2_id,
    p2_id,
    phase2_id,
    char2_id,
    E'I''ll take up a sniper position on the rooftop across from the bank''s rear entrance. Using my spyglass and Sharpshooter ability, I''ll keep watch on all approaches and provide cover fire if needed.\n\nMy hunting rifle is loaded with tranquilizer darts as the first option, but I have live rounds ready if things go badly. I''ll signal the team via our pre-arranged mirror flashes if I spot any patrols.',
    false,
    NOW() - INTERVAL '3 hours',
    NOW() - INTERVAL '3 hours'
  );

  -- Player 3's action (submitted)
  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, is_draft, submitted_at, updated_at)
  VALUES (
    game2_id,
    p3_id,
    phase2_id,
    char3_id,
    E'I''ll use my alchemical knowledge to create smoke bombs and a custom acid mixture that can dissolve the vault''s lock mechanism without triggering the alarm system.\n\nOnce inside, I''ll handle the mechanical aspects - disabling pressure plates, bypassing the combination lock using my precision tools, and neutralizing any gas traps. I''ve studied the bank''s blueprints extensively and know which ducts to avoid.',
    false,
    NOW() - INTERVAL '1 hour',
    NOW() - INTERVAL '1 hour'
  );

  -- Player 4's action (draft)
  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, is_draft, submitted_at, updated_at)
  VALUES (
    game2_id,
    p4_id,
    phase2_id,
    char4_id,
    E'Working on my approach... considering using my social connections or going with a disguise...',
    true,
    NULL,
    NOW() - INTERVAL '30 minutes'
  );

  -- ============================================
  -- GAME #4: Court of Shadows Actions
  -- ============================================

  -- Get character IDs for Game 4
  SELECT id INTO char5_id FROM characters WHERE game_id = game4_id AND name = 'Lord Ravenna';
  SELECT id INTO char6_id FROM characters WHERE game_id = game4_id AND name = 'Countess Nyx';

  -- Player 1's action (just submitted)
  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, is_draft, submitted_at, updated_at)
  VALUES (
    game4_id,
    p1_id,
    phase4_id,
    char5_id,
    E'I will arrange a private meeting with Prince Valdric in his chambers. Using my Dominate discipline, I will carefully probe his mind for information about the Archbishop''s true plans while planting subtle suggestions that the Archbishop may be plotting against him.\n\nI must be careful not to push too hard - Valdric is old and powerful. I''ll use my social graces to make this seem like friendly counsel from a concerned elder.',
    false,
    NOW() - INTERVAL '30 minutes',
    NOW() - INTERVAL '30 minutes'
  );

  -- Player 2's action (just submitted)
  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, is_draft, submitted_at, updated_at)
  VALUES (
    game4_id,
    p2_id,
    phase4_id,
    char6_id,
    E'While Lord Ravenna keeps the Prince occupied, I''ll use my Obfuscate powers to slip into the Archbishop''s private library undetected. I''m searching for any correspondence or documents that might reveal his connections to the city''s mortal authorities.\n\nI''ll photograph anything interesting with my concealed camera and be out before anyone notices. My haven is prepared to develop the photos immediately upon my return.',
    false,
    NOW() - INTERVAL '20 minutes',
    NOW() - INTERVAL '20 minutes'
  );

END $$;

COMMIT;
```

### 06_results.sql - Create Action Results

```sql
-- Create Action Results for Results Phases

BEGIN;

DO $$
DECLARE
  gm_id INTEGER;
  p1_id INTEGER;
  p2_id INTEGER;
  p3_id INTEGER;
  game3_id INTEGER;
  phase3_id INTEGER;
BEGIN
  -- Get user IDs
  SELECT id INTO gm_id FROM users WHERE email = 'test_gm@example.com';
  SELECT id INTO p1_id FROM users WHERE email = 'test_player1@example.com';
  SELECT id INTO p2_id FROM users WHERE email = 'test_player2@example.com';
  SELECT id INTO p3_id FROM users WHERE email = 'test_player3@example.com';

  -- Get game ID
  SELECT id INTO game3_id FROM games WHERE title = 'Starfall Station';

  -- Get active results phase ID
  SELECT id INTO phase3_id FROM game_phases WHERE game_id = game3_id AND phase_number = 3;

  -- ============================================
  -- GAME #3: Starfall Station Results
  -- ============================================

  -- Result for Player 1
  INSERT INTO action_results (game_id, user_id, phase_id, gm_user_id, content, is_published, sent_at)
  VALUES (
    game3_id,
    p1_id,
    phase3_id,
    gm_id,
    E'Commander Vasquez:\n\nAs you systematically search Deck C, your security scanner picks up an unusual energy signature behind the maintenance panel in Section C-7. When you pry it open, you find a makeshift laboratory filled with bizarre biological samples.\n\nThe samples are... moving. Pulsating with an otherworldly rhythm.\n\nYou notice a personal datapad partially dissolved by some kind of acid. Through the damage, you can make out the name "Dr. Morrison" - one of the scientists who went missing three weeks ago.\n\nYour radio crackles: "Commander, we''re picking up movement in the air ducts above your location."\n\nWhat do you do?',
    true,
    NOW() - INTERVAL '5 hours'
  );

  -- Result for Player 2
  INSERT INTO action_results (game_id, user_id, phase_id, gm_user_id, content, is_published, sent_at)
  VALUES (
    game3_id,
    p2_id,
    phase3_id,
    gm_id,
    E'Engineer Patel:\n\nYour analysis of the station''s power fluctuations reveals a disturbing pattern. The anomalous drains are coming from the Bio-Research Lab, but according to the station''s official schematics, that lab was decommissioned months ago.\n\nYou cross-reference with maintenance logs and find something odd: someone has been accessing the lab''s environmental controls, adjusting atmospheric composition to match... you pause as you read the analysis... an alien atmosphere.\n\nThe power requirements would be enormous. Whatever''s in that lab needs specific conditions to survive.\n\nYou receive an urgent message from Commander Vasquez about a discovery on Deck C. The timing can''t be a coincidence.',
    true,
    NOW() - INTERVAL '5 hours'
  );

  -- Result for Player 3
  INSERT INTO action_results (game_id, user_id, phase_id, gm_user_id, content, is_published, sent_at)
  VALUES (
    game3_id,
    p3_id,
    phase3_id,
    gm_id,
    E'Dr. Kim:\n\nYour medical examination of the crew members who reported "strange dreams" reveals elevated levels of an unknown compound in their bloodstream. The compound doesn''t match anything in the medical database.\n\nMore concerning: brain scans show unusual activity in the areas associated with memory and perception. It''s as if something is... writing new neural pathways.\n\nPatient Zero appears to be Dr. Morrison from the Biology Department. According to records, he was the first to report symptoms, two weeks before he vanished.\n\nYou''re interrupted by an emergency medical alert: three crew members in MedBay simultaneously began seizing. Their eyes are moving in perfect synchronization, tracking something invisible in the room.\n\nThey''re all whispering the same word: "Starfall."',
    true,
    NOW() - INTERVAL '5 hours'
  );

  -- Unpublished draft result (GM hasn't published yet)
  INSERT INTO action_results (game_id, user_id, phase_id, gm_user_id, content, is_published, sent_at)
  VALUES (
    game3_id,
    p1_id,
    phase3_id,
    gm_id,
    E'[DRAFT - Additional information about the entity''s origin - publish this later for dramatic effect]',
    false,
    NULL
  );

END $$;

COMMIT;
```

### apply_all.sh - Apply All Fixtures

```bash
#!/bin/bash
# Apply all test fixtures to the database

set -e

# Load environment variables
if [ -f .env ]; then
  export $(cat .env | grep -v '^#' | xargs)
fi

# Database connection details
DB_USER="${DB_USER:-postgres}"
DB_PASSWORD="${DB_PASSWORD:-example}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-actionphase}"

FIXTURES_DIR="backend/pkg/db/test_fixtures"

echo "🧹 Resetting test data..."
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f "$FIXTURES_DIR/00_reset.sql"

echo "👥 Creating test users..."
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f "$FIXTURES_DIR/01_users.sql"

echo "🎲 Creating recruiting games..."
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f "$FIXTURES_DIR/02_games_recruiting.sql"

echo "🎮 Creating running games with phases..."
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f "$FIXTURES_DIR/03_games_running.sql"

echo "🧙 Creating characters and NPCs..."
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f "$FIXTURES_DIR/04_characters.sql"

echo "⚔️  Creating action submissions..."
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f "$FIXTURES_DIR/05_actions.sql"

echo "📜 Creating action results..."
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f "$FIXTURES_DIR/06_results.sql"

echo "✅ Test data fixtures applied successfully!"
echo ""
echo "Test Accounts:"
echo "  GM: test_gm@example.com / testpassword123"
echo "  Player 1-5: test_player1@example.com through test_player5@example.com / testpassword123"
echo "  Audience: test_audience@example.com / testpassword123"
```

## Usage Instructions

### Initial Setup

1. **Create the fixtures directory:**
   ```bash
   mkdir -p backend/pkg/db/test_fixtures
   ```

2. **Copy all SQL files** from the sections above into the `test_fixtures/` directory

3. **Make the apply script executable:**
   ```bash
   chmod +x backend/pkg/db/test_fixtures/apply_all.sh
   ```

### Applying Test Data

**From the project root directory:**

```bash
# Apply all test fixtures
./backend/pkg/db/test_fixtures/apply_all.sh
```

**Or using just:**

Add this to your `justfile`:
```makefile
# Apply test data fixtures
test-fixtures:
    ./backend/pkg/db/test_fixtures/apply_all.sh

# Reset test data only
reset-test-data:
    PGPASSWORD=${DB_PASSWORD} psql -h ${DB_HOST} -p ${DB_PORT} -U ${DB_USER} -d ${DB_NAME} -f backend/pkg/db/test_fixtures/00_reset.sql
```

Then run:
```bash
just test-fixtures
```

### Resetting to Clean State

To reset the database to a clean state with just test data:

```bash
just reset-test-data
just test-fixtures
```

### Individual Fixture Application

To apply specific fixtures:

```bash
# Reset only
PGPASSWORD=example psql -h localhost -p 5432 -U postgres -d actionphase -f backend/pkg/db/test_fixtures/00_reset.sql

# Add just users
PGPASSWORD=example psql -h localhost -p 5432 -U postgres -d actionphase -f backend/pkg/db/test_fixtures/01_users.sql
```

## Test Data Summary

After applying all fixtures, you will have:

### Users (7 total)
- 1 Game Master
- 5 Players
- 1 Audience Member
- **All passwords:** `testpassword123`

### Games (10 total)

| # | Name | Status | Phase Status |
|---|------|--------|--------------|
| 1 | Shadows Over Innsmouth | Running | Active Common Room |
| 2 | The Heist at Goldstone Bank | Running | Active Action (with submissions) |
| 3 | Starfall Station | Running | Active Results (with published results) |
| 4 | Court of Shadows | Running | Previous Common Room + Active Action |
| 5 | The Dragon of Mount Krag | Running | 6 previous phases + Active Common Room |
| 6 | Chronicles of Westmarch | Running | 11 previous phases + Active Results |
| 7 | The Mystery of Blackwood Manor | Recruiting | No phases |
| 8 | On Hold: The Frozen North | Paused | 4 previous phases |
| 9 | COMPLETED: Tales of the Arcane | Completed | 9 completed phases |
| 10 | Secret Campaign | Recruiting | No phases (private) |

### Characters & NPCs
- **30+ characters total**
- Each player has exactly one player character per game
- Multiple GM NPCs per game (controlled by GM only)
- Character data examples included

### Action Submissions
- **Game #2**: 3 submitted actions + 1 draft
- **Game #4**: 2 submitted actions
- Variety of content lengths and styles

### Action Results
- **Game #3**: 3 published results + 1 draft
- Demonstrates GM storytelling and cliffhangers

## Edge Cases Covered

✅ **Phase States:**
- Active phases of all three types
- Previous phases (completed)
- No phases (recruiting games)
- Mixed phase history
- Long phase history (10+ phases)

✅ **Deadline Scenarios:**
- Deadlines in the future
- Deadlines in the near future (< 1 day)
- Deadlines in the far future

✅ **Character Scenarios:**
- One character per player per game (standard case)
- GM-only NPCs
- Character data (public and private)

✅ **Action Submissions:**
- Submitted (final) actions
- Draft actions
- Actions with characters
- Variety of submission times

✅ **Game States:**
- All status types: recruiting, running, paused, completed, cancelled
- Public and private visibility
- Various creation dates (age testing)

✅ **Participation:**
- Active participants
- Games with different player counts
- Audience members (for future features)

## Maintenance

### Updating Fixtures

To update fixtures:

1. Modify the appropriate SQL file in `test_fixtures/`
2. Run `just reset-test-data && just test-fixtures`
3. Verify changes in the application

### Adding New Scenarios

To add new test scenarios:

1. Identify the fixture file to modify (or create new)
2. Add SQL following existing patterns
3. Update this documentation
4. Test the new scenario

### Version Control

**Commit these files to git:**
- All `.sql` files
- The `apply_all.sh` script
- This documentation

**DO NOT commit:**
- Database dumps
- Actual passwords if using different values in production

## Future Enhancements

### Potential Additions

- **Phase transitions:** Track and test phase transition history
- **Game invitations:** Test private game invitation workflows
- **Applications:** Add game application test data
- **Notifications:** Test notification scenarios
- **Performance:** Large dataset fixtures (100+ games, 1000+ phases)

### Automation Ideas

- GitHub Actions workflow to validate fixtures
- Daily fixture refresh in development environments
- Fixture snapshots for specific feature testing
- Randomized test data generation script

## Troubleshooting

### Connection Errors

If you get connection errors:
```bash
# Check database is running
docker ps | grep postgres

# Verify credentials in .env
cat .env | grep DB_
```

### Permission Errors

If you get permission errors:
```bash
# Ensure script is executable
chmod +x backend/pkg/db/test_fixtures/apply_all.sh

# Verify database user has appropriate permissions
```

### Data Conflicts

If you get unique constraint errors:
```bash
# Ensure you reset first
just reset-test-data

# Then apply fixtures
just test-fixtures
```

## Quick Reference

```bash
# Full reset and reload
just reset-test-data && just test-fixtures

# Login as GM
# Email: test_gm@example.com
# Password: testpassword123

# Login as Player
# Email: test_player1@example.com (or 2-5)
# Password: testpassword123

# View game with active action phase
# Game ID: 2 (The Heist at Goldstone Bank)

# View game with long history
# Game ID: 6 (Chronicles of Westmarch)
```

---

**Document Version:** 1.0
**Last Updated:** 2025-10-14
**Maintained By:** Development Team
