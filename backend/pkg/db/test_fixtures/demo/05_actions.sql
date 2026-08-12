-- Create Action Submissions for Action Phases

BEGIN;

DO $$
DECLARE
  p1_id INTEGER;
  p2_id INTEGER;
  p3_id INTEGER;
  p4_id INTEGER;
  game2_id INTEGER;
  game9_id INTEGER;
  phase2_id INTEGER;
  game9_phase2_id INTEGER;
  game9_phase4_id INTEGER;
  game9_phase6_id INTEGER;
  char1_id INTEGER;
  char2_id INTEGER;
  char3_id INTEGER;
  char4_id INTEGER;
  game9_char1_id INTEGER;
  game9_char2_id INTEGER;
  game9_char3_id INTEGER;
BEGIN
  -- Get user IDs
  SELECT id INTO p1_id FROM users WHERE email = 'test_player1@example.com';
  SELECT id INTO p2_id FROM users WHERE email = 'test_player2@example.com';
  SELECT id INTO p3_id FROM users WHERE email = 'test_player3@example.com';
  SELECT id INTO p4_id FROM users WHERE email = 'test_player4@example.com';

  -- Get game IDs
  SELECT id INTO game2_id FROM games WHERE title = 'The Heist at Goldstone Bank';
  SELECT id INTO game9_id FROM games WHERE title = 'COMPLETED: Tales of the Arcane';

  -- Get active action phase IDs
  SELECT id INTO phase2_id FROM game_phases WHERE game_id = game2_id AND phase_number = 2;

  -- Get Game #9 action phase IDs (phases 2, 5, 8 are action phases for submissions)
  SELECT id INTO game9_phase2_id FROM game_phases WHERE game_id = game9_id AND phase_number = 2;
  SELECT id INTO game9_phase4_id FROM game_phases WHERE game_id = game9_id AND phase_number = 5;
  SELECT id INTO game9_phase6_id FROM game_phases WHERE game_id = game9_id AND phase_number = 8;

  -- Get character IDs for Game 2
  SELECT id INTO char1_id FROM characters WHERE game_id = game2_id AND name = 'Shade (Whisper)';
  SELECT id INTO char2_id FROM characters WHERE game_id = game2_id AND name = 'Rook (Hound)';
  SELECT id INTO char3_id FROM characters WHERE game_id = game2_id AND name = 'Vex (Leech)';
  SELECT id INTO char4_id FROM characters WHERE game_id = game2_id AND name = 'Silk (Spider)';

  -- ============================================
  -- GAME #2: Heist Actions
  -- ============================================

  -- Player 1's action (submitted)
  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, submitted_at, updated_at)
  VALUES (
    game2_id,
    p1_id,
    phase2_id,
    char1_id,
    E'Using my supernatural senses, I''ll scan the vault room for any lingering spirits or magical wards before the team enters. If I detect anything, I''ll attempt to commune with or dispel it using my Compel ability.\n\nI''ll position myself near the entrance as a lookout while maintaining focus on the ethereal plane. If guards approach, I''ll create a minor distraction using ghost sounds.',
    NOW() - INTERVAL '2 hours',
    NOW() - INTERVAL '2 hours'
  );

  -- Player 2's action (submitted)
  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, submitted_at, updated_at)
  VALUES (
    game2_id,
    p2_id,
    phase2_id,
    char2_id,
    E'I''ll take up a sniper position on the rooftop across from the bank''s rear entrance. Using my spyglass and Sharpshooter ability, I''ll keep watch on all approaches and provide cover fire if needed.\n\nMy hunting rifle is loaded with tranquilizer darts as the first option, but I have live rounds ready if things go badly. I''ll signal the team via our pre-arranged mirror flashes if I spot any patrols.',
    NOW() - INTERVAL '3 hours',
    NOW() - INTERVAL '3 hours'
  );

  -- Player 3's action (submitted)
  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, submitted_at, updated_at)
  VALUES (
    game2_id,
    p3_id,
    phase2_id,
    char3_id,
    E'I''ll use my alchemical knowledge to create smoke bombs and a custom acid mixture that can dissolve the vault''s lock mechanism without triggering the alarm system.\n\nOnce inside, I''ll handle the mechanical aspects - disabling pressure plates, bypassing the combination lock using my precision tools, and neutralizing any gas traps. I''ve studied the bank''s blueprints extensively and know which ducts to avoid.',
    NOW() - INTERVAL '1 hour',
    NOW() - INTERVAL '1 hour'
  );

  -- Player 4's action (submitted, then edited recently — submitted_at is
  -- preserved from the original submission, updated_at moves with the edit)
  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, submitted_at, updated_at)
  VALUES (
    game2_id,
    p4_id,
    phase2_id,
    char4_id,
    E'Working on my approach... considering using my social connections or going with a disguise...',
    NOW() - INTERVAL '90 minutes',
    NOW() - INTERVAL '30 minutes'
  );

  -- ============================================
  -- GAME #9: COMPLETED - Tales of the Arcane Actions
  -- ============================================

  -- Get character IDs for Game 9
  SELECT id INTO game9_char1_id FROM characters WHERE game_id = game9_id AND name = 'Lyra Nightwhisper';
  SELECT id INTO game9_char2_id FROM characters WHERE game_id = game9_id AND name = 'Theron Brightblade';
  SELECT id INTO game9_char3_id FROM characters WHERE game_id = game9_id AND name = 'Mira Stormweaver';

  -- ====================
  -- Phase 2 Actions (First Action Phase)
  -- ====================

  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, submitted_at, updated_at)
  VALUES (
    game9_id,
    p1_id,
    game9_phase2_id,
    game9_char1_id,
    E'I''ll use my shadow magic to scout ahead through the twisted corridors of the Shadow Council''s fortress. Moving silently between darkness and light, I''ll map out patrol routes and identify weak points in their defenses.\n\nIf I encounter any guards, I''ll use my Whisper of Night ability to cloud their minds and slip past undetected. I''ll leave subtle shadow marks for the party to follow safely.',
    NOW() - INTERVAL '85 days',
    NOW() - INTERVAL '85 days'
  );

  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, submitted_at, updated_at)
  VALUES (
    game9_id,
    p2_id,
    game9_phase2_id,
    game9_char2_id,
    E'Following Lyra''s shadow marks, I''ll lead our main force through the safest route. My enchanted blade Dawnbreaker will cut through any wards or magical barriers we encounter.\n\nI''ll maintain a defensive formation, ready to protect Mira if she needs to prepare any large-scale spells. My holy aura should also help resist the dark magic that permeates this place.',
    NOW() - INTERVAL '85 days',
    NOW() - INTERVAL '85 days'
  );

  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, submitted_at, updated_at)
  VALUES (
    game9_id,
    p3_id,
    game9_phase2_id,
    game9_char3_id,
    E'I''ll weave protective wards around our group as we advance, focusing on deflecting scrying attempts and detection spells. The Shadow Council surely knows someone is here, but I can delay their response.\n\nI''m also preparing a Storm Cage spell - if we get surrounded, I can buy us time with a lightning barrier while we regroup.',
    NOW() - INTERVAL '84 days',
    NOW() - INTERVAL '84 days'
  );

  -- ====================
  -- Phase 4 Actions (Second Action Phase)
  -- ====================

  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, submitted_at, updated_at)
  VALUES (
    game9_id,
    p1_id,
    game9_phase4_id,
    game9_char1_id,
    E'Now that we''ve breached the inner sanctum, I''ll use my shadow form to infiltrate Archmagus Valdane''s meditation chamber. I need to steal the Codex of Binding before he completes the ritual.\n\nThe shadows here are thick with dark power - I''ll need to merge completely with them to avoid detection. Once I have the Codex, I''ll signal the others to begin the distraction.',
    NOW() - INTERVAL '70 days',
    NOW() - INTERVAL '70 days'
  );

  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, submitted_at, updated_at)
  VALUES (
    game9_id,
    p2_id,
    game9_phase4_id,
    game9_char2_id,
    E'When Lyra gives the signal, I''ll challenge Valdane directly to keep his attention. Dawnbreaker was forged specifically to counter his type of magic - the blade''s light should disrupt his concentration.\n\nI know I can''t defeat him alone, but I don''t need to. I just need to keep him occupied long enough for the plan to work. For honor, for the fallen, for the dawn!',
    NOW() - INTERVAL '70 days',
    NOW() - INTERVAL '70 days'
  );

  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, submitted_at, updated_at)
  VALUES (
    game9_id,
    p3_id,
    game9_phase4_id,
    game9_char3_id,
    E'While Theron keeps Valdane busy, I''ll position myself at the ritual circle''s focal points. I''ve studied the binding magic extensively - if I can reverse even three of the seven seals, the entire spell will collapse.\n\nThis will drain almost everything I have, but I''m prepared. I''ve attuned myself to the ley lines beneath the fortress. When the moment comes, I''ll channel their power through my staff.',
    NOW() - INTERVAL '69 days',
    NOW() - INTERVAL '69 days'
  );

  -- ====================
  -- Phase 6 Actions (Third Action Phase - Final Confrontation)
  -- ====================

  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, submitted_at, updated_at)
  VALUES (
    game9_id,
    p1_id,
    game9_phase6_id,
    game9_char1_id,
    E'With the Codex in hand and the ritual disrupted, I''ll use its power to bind the shadow entities that Valdane summoned. They''re creatures of darkness like me - I can speak to them, show them they don''t have to serve him.\n\nIf I can turn even half of them, Valdane will be overwhelmed. The shadows remember freedom. I''ll remind them what it feels like.',
    NOW() - INTERVAL '60 days',
    NOW() - INTERVAL '60 days'
  );

  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, submitted_at, updated_at)
  VALUES (
    game9_id,
    p2_id,
    game9_phase6_id,
    game9_char2_id,
    E'This is it - the final strike. Valdane is weakened, his ritual broken, his forces in disarray. Dawnbreaker burns brighter than ever, fed by my conviction and the righteousness of our cause.\n\nI''ll press the attack with everything I have. No holding back, no second chances. Today, the Shadow Council falls!',
    NOW() - INTERVAL '60 days',
    NOW() - INTERVAL '60 days'
  );

  INSERT INTO action_submissions (game_id, user_id, phase_id, character_id, content, submitted_at, updated_at)
  VALUES (
    game9_id,
    p3_id,
    game9_phase6_id,
    game9_char3_id,
    E'I''m exhausted from breaking the seals, but I have one more spell in me. As Theron engages Valdane and Lyra commands the shadows, I''ll call down the storm itself.\n\nLightning will strike where I direct it, wind will scatter his defenses, and thunder will shatter his concentration. This fortress has stood for centuries in darkness - let it fall in the light of the storm!',
    NOW() - INTERVAL '59 days',
    NOW() - INTERVAL '59 days'
  );

END $$;

COMMIT;
