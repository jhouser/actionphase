-- Staged (timed multi-part) action result chains
--
-- A staged chain is one result split into N parts revealed on a timer. Part 1
-- is an ordinary result; parts 2..N carry parent_result_id + reveal_delay_minutes
-- and stay invisible to the recipient until the release worker sets released_at.
--
-- WRITE-PATH FIDELITY (per the lesson in 06_results.sql): the head and the
-- follow-up parts are created by DIFFERENT queries with DIFFERENT column lists,
-- and this fixture must mirror both.
--
--   Head       (CreateActionResult):     sets released_at alongside sent_at.
--   Parts 2..N (CreateStagedResultPart): OMIT released_at so it defaults NULL,
--                                        and set parent_result_id +
--                                        reveal_delay_minutes.
--
-- Do not "tidy" this by giving every part a released_at. A published part with
-- released_at NULL is not a bug here — it is the entire feature, and it is the
-- only case where that combination is legitimate.
--
-- Constraints these rows must satisfy (migration 20260812214302):
--   action_results_delay_requires_parent — parent and delay are both-or-neither
--   action_results_delay_bounds          — 1 <= reveal_delay_minutes <= 1440
--   action_results_no_self_parent        — a part cannot be its own parent
--
-- Coverage:
--   Game #3 (in_progress) — a PENDING chain: part 1 revealed, part 2 due soon,
--     part 3 with no knowable unlock time. This is what the player-facing
--     countdown and "Pending" placeholder render against.
--   Game #3 (in_progress) — a FULLY RELEASED chain, so the "Part N of M" labels
--     have a case where every part reads normally.
--   Game #9 (completed)   — a chain in the public archive, half released and
--     half never released, which is what the export gate is written against.

BEGIN;

DO $$
DECLARE
  gm_id INTEGER;
  p1_id INTEGER;
  p2_id INTEGER;
  game3_id INTEGER;
  game9_id INTEGER;
  phase3_id INTEGER;
  game9_phase7_id INTEGER;
  head_id INTEGER;
  part2_id INTEGER;
BEGIN
  SELECT id INTO gm_id FROM users WHERE email = 'test_gm@example.com';
  SELECT id INTO p1_id FROM users WHERE email = 'test_player1@example.com';
  SELECT id INTO p2_id FROM users WHERE email = 'test_player2@example.com';

  SELECT id INTO game3_id FROM games WHERE title = 'Starfall Station';
  SELECT id INTO game9_id FROM games WHERE title = 'COMPLETED: Tales of the Arcane';

  SELECT id INTO phase3_id FROM game_phases WHERE game_id = game3_id AND phase_number = 3;
  SELECT id INTO game9_phase7_id FROM game_phases WHERE game_id = game9_id AND phase_number = 8;

  -- ============================================
  -- GAME #3: PENDING chain (the mockup's case)
  -- ============================================
  -- Part 1 released 2 minutes ago; part 2 is due 5 minutes after that, so it
  -- unlocks ~3 minutes from load. Short on purpose: someone loading fixtures to
  -- look at the countdown should not have to wait a quarter of an hour, and the
  -- worker will genuinely fire it, which exercises the release path end to end.

  INSERT INTO action_results (game_id, user_id, phase_id, character_id, gm_user_id, content, is_published, sent_at, released_at)
  VALUES (
    game3_id,
    p1_id,
    phase3_id,
    (SELECT id FROM characters WHERE game_id = game3_id AND user_id = p1_id),
    gm_id,
    E'Commander Vasquez:\n\nThe thing in the air duct drops behind you without a sound. You spin, sidearm rising, and for one long moment you are looking straight down the length of something that should not exist.\n\nIt lunges. Your shot goes wide. Its claw comes around in a flat arc toward your throat...',
    true,
    NOW() - INTERVAL '2 minutes',
    NOW() - INTERVAL '2 minutes'
  ) RETURNING id INTO head_id;

  -- Part 2: no released_at (the worker will set it). Due 5 minutes after part 1.
  INSERT INTO action_results (game_id, user_id, phase_id, character_id, gm_user_id, content, is_published, sent_at, parent_result_id, reveal_delay_minutes)
  VALUES (
    game3_id,
    p1_id,
    phase3_id,
    (SELECT id FROM characters WHERE game_id = game3_id AND user_id = p1_id),
    gm_id,
    E'...and stops.\n\nNot blocked. Stopped, a handspan from your skin, shivering as though the air itself has gone solid. The creature makes a sound you feel in your teeth rather than hear.\n\nBehind it, the maintenance panel is glowing. The samples are glowing. Every one of them, in the same slow rhythm.',
    true,
    NOW() - INTERVAL '2 minutes',
    head_id,
    5
  ) RETURNING id INTO part2_id;

  -- Part 3: 10 minutes after part 2. Its unlock time is genuinely unknowable
  -- until part 2 releases, so GetUserResults returns unlocks_at NULL for it and
  -- the UI shows "Pending" rather than a countdown. That is the case this row
  -- exists to cover.
  INSERT INTO action_results (game_id, user_id, phase_id, character_id, gm_user_id, content, is_published, sent_at, parent_result_id, reveal_delay_minutes)
  VALUES (
    game3_id,
    p1_id,
    phase3_id,
    (SELECT id FROM characters WHERE game_id = game3_id AND user_id = p1_id),
    gm_id,
    E'The creature folds backward into the duct as though something has taken hold of it, and is gone.\n\nIn the silence, your radio speaks in Dr. Morrison''s voice — three weeks missing, and perfectly calm:\n\n"Don''t shoot the next one, Commander. It was trying to warn you."',
    true,
    NOW() - INTERVAL '2 minutes',
    part2_id,
    10
  );

  -- ============================================
  -- GAME #3: FULLY RELEASED chain
  -- ============================================
  -- Every part released. Exercises "Part N of M" labelling with no placeholder
  -- in sight — the state a chain spends most of its life in.

  INSERT INTO action_results (game_id, user_id, phase_id, character_id, gm_user_id, content, is_published, sent_at, released_at)
  VALUES (
    game3_id,
    p2_id,
    phase3_id,
    (SELECT id FROM characters WHERE game_id = game3_id AND user_id = p2_id),
    gm_id,
    E'Dr. Kim:\n\nThe seizures stop all at once, as though a switch were thrown. Three crew members sit up in perfect unison and look at you.\n\n"You are reading the wrong records," they say together.',
    true,
    NOW() - INTERVAL '3 hours',
    NOW() - INTERVAL '3 hours'
  ) RETURNING id INTO head_id;

  INSERT INTO action_results (game_id, user_id, phase_id, character_id, gm_user_id, content, is_published, sent_at, parent_result_id, reveal_delay_minutes, released_at)
  VALUES (
    game3_id,
    p2_id,
    phase3_id,
    (SELECT id FROM characters WHERE game_id = game3_id AND user_id = p2_id),
    gm_id,
    E'They speak the coordinates of a storage bay that is not on any manifest you have seen, then lie back down and sleep like children.\n\nWhen you check the medical log an hour later, the entry you wrote about the incident is gone.',
    true,
    NOW() - INTERVAL '3 hours',
    head_id,
    15,
    NOW() - INTERVAL '2 hours 45 minutes'
  );

  -- ============================================
  -- GAME #9 (COMPLETED): archive chain
  -- ============================================
  -- Half released, half not. The game is completed and therefore a public
  -- archive, so this is the fixture the export gate is written against: the
  -- released parts belong in an archive of a finished game, the never-released
  -- one does not (exports.sql keeps a row-level released_at filter).

  INSERT INTO action_results (game_id, user_id, phase_id, action_submission_id, character_id, gm_user_id, content, is_published, sent_at, released_at)
  VALUES (
    game9_id,
    p1_id,
    game9_phase7_id,
    (SELECT id FROM action_submissions
      WHERE game_id = game9_id AND user_id = p1_id AND phase_id = game9_phase7_id),
    (SELECT character_id FROM action_submissions
      WHERE game_id = game9_id AND user_id = p1_id AND phase_id = game9_phase7_id),
    gm_id,
    E'Theron:\n\nAs the fortress falls, you see a figure standing calmly amid the collapsing stone — untouched, unhurried, watching you.\n\nIt raises one hand in something that is almost a greeting...',
    true,
    NOW() - INTERVAL '58 days',
    NOW() - INTERVAL '58 days'
  ) RETURNING id INTO head_id;

  -- Part 2 is the tail of the RELEASED portion. Its own released_at is set, but
  -- note its reveal_delay_minutes is large and it is anchored to NOW(), not to
  -- 58 days ago — see the comment on part 3 for why that matters.
  INSERT INTO action_results (game_id, user_id, phase_id, action_submission_id, character_id, gm_user_id, content, is_published, sent_at, parent_result_id, reveal_delay_minutes, released_at)
  VALUES (
    game9_id,
    p1_id,
    game9_phase7_id,
    (SELECT id FROM action_submissions
      WHERE game_id = game9_id AND user_id = p1_id AND phase_id = game9_phase7_id),
    (SELECT character_id FROM action_submissions
      WHERE game_id = game9_id AND user_id = p1_id AND phase_id = game9_phase7_id),
    gm_id,
    E'...and then it is simply not there, and neither is the fortress, and you are standing in an empty field with your friends and no evidence that any of it happened.\n\nThe Shadow Council is more than one mage.',
    true,
    NOW() - INTERVAL '58 days',
    head_id,
    20,
    NOW()
  ) RETURNING id INTO part2_id;

  -- Still pending: the game completed with this reveal outstanding.
  --
  -- ⚠️ Its parent's released_at is deliberately NOW(), not 58 days ago. The
  -- release worker owns a chain's clock independently of game state — it does
  -- NOT join games or game_phases, by design ("Chain Independence") — so a part
  -- whose parent released in the past is overdue and fires on the very next
  -- tick, completed game or not. Backdating the parent here would have this row
  -- released within a minute of loading fixtures, which silently destroys the
  -- state it exists to represent.
  --
  -- Anchoring the parent to NOW() with a 1440-minute (24h) delay keeps this part
  -- genuinely pending for a day after load, so there is a real pending-at-
  -- completion row to exercise the read and export paths against.
  INSERT INTO action_results (game_id, user_id, phase_id, action_submission_id, character_id, gm_user_id, content, is_published, sent_at, parent_result_id, reveal_delay_minutes)
  VALUES (
    game9_id,
    p1_id,
    game9_phase7_id,
    (SELECT id FROM action_submissions
      WHERE game_id = game9_id AND user_id = p1_id AND phase_id = game9_phase7_id),
    (SELECT character_id FROM action_submissions
      WHERE game_id = game9_id AND user_id = p1_id AND phase_id = game9_phase7_id),
    gm_id,
    E'PENDING AT COMPLETION: this reveal never fired. It appears in the archive and in exports by design — see the note on the export query in exports.sql. The path that DOES hide it is GetUserResults, which blanks unreleased content for the recipient mid-game.',
    true,
    NOW() - INTERVAL '58 days',
    part2_id,
    1440
  );

END $$;

COMMIT;
