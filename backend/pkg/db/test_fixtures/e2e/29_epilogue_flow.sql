-- E2E Test Fixture: Epilogue Game State
--
-- Game IDs: 348, 349 (offset by worker: Worker 1 = 10348/10349, etc.)
--
-- Backs epilogue-flow.spec.ts. Two games, because the spec asks two different
-- kinds of question:
--
--   * Game 348 (TRANSITION, starts in_progress) — driven through
--     in_progress -> epilogue -> completed by the transition tests. It exists
--     to prove the *change*: content hidden before the move is visible after.
--     It must start in_progress, since seeding it already open would assert
--     only that the open state renders, which is the half that never broke.
--
--   * Game 349 (STEADY-STATE, seeded in epilogue) — never mutated. The
--     read-side tests point here so they assert what an epilogue game looks
--     like without depending on another test having run first.
--
-- Splitting them is what makes the suite retry-safe. Playwright retries a
-- failed test in isolation (retries=2 under CI), so a single flake in a chained
-- serial suite re-runs a test against a game some later test has already
-- advanced — a failure that can never pass and has nothing to do with the code.
-- Only the two transition tests are order-dependent now.
--
-- Every seeded row is deliberately addressed to someone OTHER than TestPlayer1,
-- who is the viewer the spec logs in as. A player can always read their own
-- messages, their own results, and their own character sheet, so a fixture that
-- pointed at TestPlayer1 would pass identically in in_progress and prove
-- nothing about disclosure:
--
--   * The seeded conversation (9948 / 9949, one per game) is TestPlayer2 <->
--     TestPlayer3. TestPlayer1 is not a participant, so it is invisible until
--     the archive opens.
--   * The published action result belongs to TestPlayer2. /results/mine serves
--     TestPlayer1 only their own, so this appears only via the all-results
--     endpoint that archive access unlocks.
--   * Epilogue Char 2 belongs to TestPlayer2, so its private sheet modules are
--     gated behind canViewPrivate.
--
-- The active phase is COMMON_ROOM, not action: epilogue deliberately hides the
-- Actions tab (play is over), while the Common Room stays writable so the GM
-- can run epilogue threads. Phase 1 is a closed action phase carrying the
-- archived submissions and results that History surfaces.
--
-- IDEMPOTENT: Safe to run multiple times - deletes existing data before recreating

BEGIN;

-- Deleted inside the loop instead of up front: the IDs are worker-offset, and a
-- literal DELETE here would only ever clear worker 0's rows.

DO $$
DECLARE
  gm_id             INTEGER;
  p1_id             INTEGER;
  p2_id             INTEGER;
  p3_id             INTEGER;
  game_id           INTEGER;
  game_state        TEXT;
  game_title        TEXT;
  convo_id          INTEGER;
  action_phase_id   INTEGER;
  common_phase_id   INTEGER;
  char1_id          INTEGER;
  char2_id          INTEGER;
  char3_id          INTEGER;
  submission2_id    INTEGER;
  spec              RECORD;
  -- Rewritten per worker by apply_e2e_worker.sh (worker N -> N*10000). The IDs
  -- below are built from it rather than written as literals because the
  -- offsetting script only recognises a handful of shapes, and a bare integer
  -- inside a VALUES tuple list is not one of them — it would silently leave
  -- every worker pointing at the same two games.
  worker_game_id_offset INTEGER := 0;
BEGIN
  SELECT id INTO gm_id FROM users WHERE email = 'test_gm@example.com';
  SELECT id INTO p1_id FROM users WHERE email = 'test_player1@example.com';
  SELECT id INTO p2_id FROM users WHERE email = 'test_player2@example.com';
  SELECT id INTO p3_id FROM users WHERE email = 'test_player3@example.com';

  -- Both games carry byte-identical content and differ only in state, so any
  -- difference the spec observes between them is attributable to the state and
  -- nothing else.
  FOR spec IN
    SELECT * FROM (VALUES
      (348, 'in_progress', 'E2E Test: Epilogue Transition', 9948),
      (349, 'epilogue',    'E2E Test: Epilogue Steady',     9949)
    ) AS t(gid, gstate, gtitle, cid)
  LOOP
    game_id    := spec.gid + worker_game_id_offset;
    game_state := spec.gstate;
    game_title := spec.gtitle;
    convo_id   := spec.cid + worker_game_id_offset;

    DELETE FROM games WHERE id = game_id;

    INSERT INTO games (id, title, description, genre, gm_user_id, max_players, state, is_public, created_at, updated_at)
    VALUES (
      game_id, game_title,
      'Fixture for the in_progress -> epilogue -> completed disclosure flow.',
      'Test Framework',
      gm_id, 5, game_state, true,
      NOW() - INTERVAL '20 days', NOW()
    );

    INSERT INTO game_participants (game_id, user_id, role, status, joined_at) VALUES
      (game_id, p1_id, 'player', 'active', NOW() - INTERVAL '19 days'),
      (game_id, p2_id, 'player', 'active', NOW() - INTERVAL '19 days'),
      (game_id, p3_id, 'player', 'active', NOW() - INTERVAL '19 days');

    INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
    VALUES (game_id, p1_id, 'Epilogue Char 1', 'player_character', 'approved', NOW() - INTERVAL '19 days', NOW())
    RETURNING id INTO char1_id;

    INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
    VALUES (game_id, p2_id, 'Epilogue Char 2', 'player_character', 'approved', NOW() - INTERVAL '19 days', NOW())
    RETURNING id INTO char2_id;

    INSERT INTO characters (game_id, user_id, name, character_type, status, created_at, updated_at)
    VALUES (game_id, p3_id, 'Epilogue Char 3', 'player_character', 'approved', NOW() - INTERVAL '19 days', NOW())
    RETURNING id INTO char3_id;

    -- Private sheet data for Epilogue Char 2 (TestPlayer2's character).
    -- is_public = false: the payload behind the Private Notes tab that only
    -- canViewPrivate reveals.
    INSERT INTO character_data (character_id, module_type, field_name, field_value, field_type, is_public, created_at, updated_at)
    VALUES
      (char2_id, 'notes', 'private_notes', 'Char 2 secret: the ledger was forged all along.', 'text', false, NOW() - INTERVAL '18 days', NOW()),
      (char2_id, 'bio',   'background',    'A quiet clerk with an unremarkable history.',     'text', true,  NOW() - INTERVAL '18 days', NOW());

    -- Phase 1: closed ACTION phase (the archive material)
    INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, deadline, end_time, is_active, is_published, created_at)
    VALUES (
      game_id, 'action', 1,
      'Epilogue Fixture Action Phase',
      'Closed action phase whose submissions and results the History tab surfaces.',
      NOW() - INTERVAL '10 days', NOW() - INTERVAL '6 days', NOW() - INTERVAL '5 days',
      false, true, NOW() - INTERVAL '10 days'
    ) RETURNING id INTO action_phase_id;

    INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, submitted_at, updated_at)
    VALUES (game_id, p2_id, action_phase_id, char2_id,
            'Char 2 slips into the vault while the guards change shift.',
            NOW() - INTERVAL '7 days', NOW() - INTERVAL '7 days')
    RETURNING id INTO submission2_id;

    -- Published result addressed to TestPlayer2. Invisible to TestPlayer1 while
    -- in_progress (/results/mine is scoped to the caller); visible once archive
    -- access unlocks the all-results endpoint.
    INSERT INTO action_results (game_id, user_id, phase_id, action_submission_id, character_id, content, gm_user_id, is_published, sent_at, released_at, created_at, updated_at)
    VALUES (
      game_id, p2_id, action_phase_id, submission2_id, char2_id,
      'The vault yields a sealed confession naming the magistrate.',
      gm_id, true,
      NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days',
      NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days'
    );

    -- Phase 2: active COMMON_ROOM phase (the writable surface)
    INSERT INTO game_phases (game_id, phase_type, phase_number, title, description, start_time, deadline, is_active, is_published, created_at)
    VALUES (
      game_id, 'common_room', 2,
      'Epilogue Fixture Common Room',
      'Active common room phase - writable through epilogue, closed at completion.',
      NOW() - INTERVAL '1 hour', NOW() + INTERVAL '23 hours',
      true, false, NOW() - INTERVAL '1 hour'
    ) RETURNING id INTO common_phase_id;

    INSERT INTO messages (game_id, phase_id, character_id, author_id, message_type, visibility, content, created_at)
    VALUES (
      game_id, common_phase_id, char1_id, p1_id, 'post', 'game',
      'The dust settles over the magistrate''s courtyard.',
      NOW() - INTERVAL '30 minutes'
    );

    -- Private conversation between TestPlayer2 and TestPlayer3.
    -- TestPlayer1 is deliberately NOT a participant.
    INSERT INTO conversations (id, game_id, title, conversation_type, created_by_user_id, created_at)
    VALUES (convo_id, game_id, 'Epilogue Secret Conversation', 'direct', p2_id, NOW() - INTERVAL '8 days')
    ON CONFLICT (id) DO UPDATE SET title = EXCLUDED.title, game_id = EXCLUDED.game_id;

    INSERT INTO conversation_participants (conversation_id, user_id, character_id, joined_at) VALUES
      (convo_id, p2_id, char2_id, NOW() - INTERVAL '8 days'),
      (convo_id, p3_id, char3_id, NOW() - INTERVAL '8 days')
    ON CONFLICT (conversation_id, user_id, character_id) DO NOTHING;

    INSERT INTO private_messages (id, conversation_id, sender_user_id, sender_character_id, content, created_at, is_deleted)
    VALUES
      (convo_id * 10 + 1, convo_id, p2_id, char2_id, 'Only we know where the confession is hidden.', NOW() - INTERVAL '8 days', false),
      (convo_id * 10 + 2, convo_id, p3_id, char3_id, 'Then we keep it that way until the end.',      NOW() - INTERVAL '7 days', false)
    ON CONFLICT (id) DO UPDATE SET content = EXCLUDED.content, conversation_id = EXCLUDED.conversation_id;

    RAISE NOTICE 'Epilogue fixture created: Game #% (%)', game_id, game_state;
  END LOOP;

END $$;

SELECT setval('games_id_seq', (SELECT MAX(id) FROM games) + 1);

COMMIT;

SELECT 'E2E Epilogue fixture created successfully!' as message;
