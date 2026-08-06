-- Create Action Results for Results Phases
--
-- Fixtures must produce rows the application could actually create. The real
-- write path is CreateActionResultForm.tsx -> ActionsList.tsx, which always
-- sends BOTH character_id and action_submission_id, taken from the submission
-- being answered. Omitting them produced rows no code path emits, which let a
-- real bug through testing (archive export, Aug 2026).
--
-- character_id: ALWAYS set. Results address a character by name, so every row
-- resolves one — via the answered submission (Game #9) or the player's
-- character in that game (Game #3).
--
-- action_submission_id:
--   Game #9 (COMPLETED: Tales of the Arcane) — results ANSWER a submission, so
--   each links to its submission from 05_actions.sql via a (game, user, phase)
--   lookup, matching the UI.
--
--   Game #3 (Starfall Station) — results are GM narration with NO corresponding
--   submissions (05_actions.sql seeds none for this game). Their FK is left NULL
--   deliberately: a result answering no submission is a supported case, and this
--   fixture is the coverage for it. Do NOT "fix" these to point at a submission.

BEGIN;

DO $$
DECLARE
  gm_id INTEGER;
  p1_id INTEGER;
  p2_id INTEGER;
  p3_id INTEGER;
  game3_id INTEGER;
  game9_id INTEGER;
  phase3_id INTEGER;
  game9_phase3_id INTEGER;
  game9_phase5_id INTEGER;
  game9_phase7_id INTEGER;
BEGIN
  -- Get user IDs
  SELECT id INTO gm_id FROM users WHERE email = 'test_gm@example.com';
  SELECT id INTO p1_id FROM users WHERE email = 'test_player1@example.com';
  SELECT id INTO p2_id FROM users WHERE email = 'test_player2@example.com';
  SELECT id INTO p3_id FROM users WHERE email = 'test_player3@example.com';

  -- Get game IDs
  SELECT id INTO game3_id FROM games WHERE title = 'Starfall Station';
  SELECT id INTO game9_id FROM games WHERE title = 'COMPLETED: Tales of the Arcane';

  -- Get active results phase ID
  SELECT id INTO phase3_id FROM game_phases WHERE game_id = game3_id AND phase_number = 3;

  -- Get Game #9 action phase IDs (phases 2, 5, 8 contain BOTH submissions and results)
  SELECT id INTO game9_phase3_id FROM game_phases WHERE game_id = game9_id AND phase_number = 2;
  SELECT id INTO game9_phase5_id FROM game_phases WHERE game_id = game9_id AND phase_number = 5;
  SELECT id INTO game9_phase7_id FROM game_phases WHERE game_id = game9_id AND phase_number = 8;

  -- ============================================
  -- GAME #3: Starfall Station Results
  -- ============================================

  -- Result for Player 1
  INSERT INTO action_results (game_id, user_id, phase_id, character_id, gm_user_id, content, is_published, sent_at)
  VALUES (
    game3_id,
    p1_id,
    phase3_id,
    (SELECT id FROM characters WHERE game_id = game3_id AND user_id = p1_id),
    gm_id,
    E'Commander Vasquez:\n\nAs you systematically search Deck C, your security scanner picks up an unusual energy signature behind the maintenance panel in Section C-7. When you pry it open, you find a makeshift laboratory filled with bizarre biological samples.\n\nThe samples are... moving. Pulsating with an otherworldly rhythm.\n\nYou notice a personal datapad partially dissolved by some kind of acid. Through the damage, you can make out the name "Dr. Morrison" - one of the scientists who went missing three weeks ago.\n\nYour radio crackles: "Commander, we''re picking up movement in the air ducts above your location."\n\nWhat do you do?',
    true,
    NOW() - INTERVAL '5 hours'
  );

  -- Result for Player 2
  INSERT INTO action_results (game_id, user_id, phase_id, character_id, gm_user_id, content, is_published, sent_at)
  VALUES (
    game3_id,
    p2_id,
    phase3_id,
    (SELECT id FROM characters WHERE game_id = game3_id AND user_id = p2_id),
    gm_id,
    E'Engineer Patel:\n\nYour analysis of the station''s power fluctuations reveals a disturbing pattern. The anomalous drains are coming from the Bio-Research Lab, but according to the station''s official schematics, that lab was decommissioned months ago.\n\nYou cross-reference with maintenance logs and find something odd: someone has been accessing the lab''s environmental controls, adjusting atmospheric composition to match... you pause as you read the analysis... an alien atmosphere.\n\nThe power requirements would be enormous. Whatever''s in that lab needs specific conditions to survive.\n\nYou receive an urgent message from Commander Vasquez about a discovery on Deck C. The timing can''t be a coincidence.',
    true,
    NOW() - INTERVAL '5 hours'
  );

  -- Result for Player 3
  INSERT INTO action_results (game_id, user_id, phase_id, character_id, gm_user_id, content, is_published, sent_at)
  VALUES (
    game3_id,
    p3_id,
    phase3_id,
    (SELECT id FROM characters WHERE game_id = game3_id AND user_id = p3_id),
    gm_id,
    E'Dr. Kim:\n\nYour medical examination of the crew members who reported "strange dreams" reveals elevated levels of an unknown compound in their bloodstream. The compound doesn''t match anything in the medical database.\n\nMore concerning: brain scans show unusual activity in the areas associated with memory and perception. It''s as if something is... writing new neural pathways.\n\nPatient Zero appears to be Dr. Morrison from the Biology Department. According to records, he was the first to report symptoms, two weeks before he vanished.\n\nYou''re interrupted by an emergency medical alert: three crew members in MedBay simultaneously began seizing. Their eyes are moving in perfect synchronization, tracking something invisible in the room.\n\nThey''re all whispering the same word: "Starfall."',
    true,
    NOW() - INTERVAL '5 hours'
  );

  -- Unpublished draft result (GM hasn't published yet)
  INSERT INTO action_results (game_id, user_id, phase_id, character_id, gm_user_id, content, is_published, sent_at)
  VALUES (
    game3_id,
    p1_id,
    phase3_id,
    (SELECT id FROM characters WHERE game_id = game3_id AND user_id = p1_id),
    gm_id,
    E'[DRAFT - Additional information about the entity''s origin - publish this later for dramatic effect]',
    false,
    NULL
  );

  -- ============================================
  -- GAME #9: COMPLETED - Tales of the Arcane Results
  -- ============================================

  -- ====================
  -- Phase 3 Results (After First Action Phase)
  -- ====================

  INSERT INTO action_results (game_id, user_id, phase_id, action_submission_id, character_id, gm_user_id, content, is_published, sent_at)
  VALUES (
    game9_id,
    p1_id,
    game9_phase3_id,
    (SELECT id FROM action_submissions
      WHERE game_id = game9_id AND user_id = p1_id AND phase_id = game9_phase3_id),
    (SELECT character_id FROM action_submissions
      WHERE game_id = game9_id AND user_id = p1_id AND phase_id = game9_phase3_id),
    gm_id,
    E'Lyra:\n\nYour shadow form glides through the fortress corridors like smoke through a lattice. The patrol routes become clear - three guards circling every fifteen minutes, with a five-minute gap when they all converge at the central checkpoint.\n\nYour Whisper of Night works perfectly on the lone sentry near the eastern wing. His eyes glaze over and he simply... forgets he saw you. The shadow marks you leave pulse faintly, visible only to those attuned to darkness.\n\nBut as you reach the inner sanctum''s outer door, you sense something. Another shadow-walker, ancient and powerful, has noticed your presence. You feel Archmagus Valdane''s attention brush against your consciousness like a cold wind.\n\n"Clever little shadow," his voice echoes in your mind. "But you''re not the only one who knows the dark paths."\n\nThe fortress is now on high alert. You have perhaps an hour before they find your entry point.',
    true,
    NOW() - INTERVAL '84 days'
  );

  INSERT INTO action_results (game_id, user_id, phase_id, action_submission_id, character_id, gm_user_id, content, is_published, sent_at)
  VALUES (
    game9_id,
    p2_id,
    game9_phase3_id,
    (SELECT id FROM action_submissions
      WHERE game_id = game9_id AND user_id = p2_id AND phase_id = game9_phase3_id),
    (SELECT character_id FROM action_submissions
      WHERE game_id = game9_id AND user_id = p2_id AND phase_id = game9_phase3_id),
    gm_id,
    E'Theron:\n\nFollowing Lyra''s shadow marks, you lead the strike team through the safest route. Dawnbreaker hums in your grip, its holy enchantment cutting through magical wards like they''re cobwebs.\n\nYour defensive formation works well - when a patrol stumbles upon your group, your holy aura disrupts their alarm spell just long enough for your team to subdue them quietly.\n\nBut you encounter something unexpected: a massive barrier of pure shadow magic blocking the main corridor. It pulses with malevolent energy. Dawnbreaker could probably break it, but the resulting flare of light would alert every dark mage in the fortress.\n\nYou spot an alternative route through the old servant passages - narrow, cramped, and probably trapped, but it might get you past the barrier undetected.\n\nThe choice is yours: overwhelming force or stealth?',
    true,
    NOW() - INTERVAL '84 days'
  );

  INSERT INTO action_results (game_id, user_id, phase_id, action_submission_id, character_id, gm_user_id, content, is_published, sent_at)
  VALUES (
    game9_id,
    p3_id,
    game9_phase3_id,
    (SELECT id FROM action_submissions
      WHERE game_id = game9_id AND user_id = p3_id AND phase_id = game9_phase3_id),
    (SELECT character_id FROM action_submissions
      WHERE game_id = game9_id AND user_id = p3_id AND phase_id = game9_phase3_id),
    gm_id,
    E'Mira:\n\nYour protective wards settle around the group like an invisible dome. You feel the Shadow Council''s scrying attempts slide off your defenses - they know someone is here, but they can''t pinpoint exactly where.\n\nThe Storm Cage spell takes shape in your mind, ready to be unleashed at a moment''s notice. You can feel the potential energy crackling at your fingertips.\n\nThen you sense it: the ley lines beneath the fortress are... corrupted. Twisted by decades of dark rituals. If you tap into them for power, you''ll get the energy you need, but you risk corruption yourself.\n\nAlternatively, you could use only your own magical reserves, but that might not be enough if things go badly.\n\nYou also notice something interesting: the ritual chamber Valdane is using sits at the convergence of three major ley lines. If someone could reach those focal points...',
    true,
    NOW() - INTERVAL '83 days'
  );

  -- ====================
  -- Phase 5 Results (After Second Action Phase)
  -- ====================

  INSERT INTO action_results (game_id, user_id, phase_id, action_submission_id, character_id, gm_user_id, content, is_published, sent_at)
  VALUES (
    game9_id,
    p1_id,
    game9_phase5_id,
    (SELECT id FROM action_submissions
      WHERE game_id = game9_id AND user_id = p1_id AND phase_id = game9_phase5_id),
    (SELECT character_id FROM action_submissions
      WHERE game_id = game9_id AND user_id = p1_id AND phase_id = game9_phase5_id),
    gm_id,
    E'Lyra:\n\nYou merge completely with the shadows, becoming one with the darkness that permeates Valdane''s meditation chamber. The Codex of Binding rests on a pedestal, surrounded by wards that would fry most thieves instantly.\n\nBut you''re not most thieves. You flow through the gaps in the magical defenses, reforming just long enough to grasp the Codex before dissolving back into shadow.\n\nThe moment your fingers touch the ancient tome, you feel its power surge through you. Knowledge floods your mind - binding spells, control magic, and... something else. A way to free the shadow entities Valdane has enslaved.\n\nYou send the signal: three pulses of shadow-light visible only to your companions.\n\nAs you escape, you hear Valdane''s roar of fury. He knows the Codex is gone. The final confrontation is inevitable now.',
    true,
    NOW() - INTERVAL '69 days'
  );

  INSERT INTO action_results (game_id, user_id, phase_id, action_submission_id, character_id, gm_user_id, content, is_published, sent_at)
  VALUES (
    game9_id,
    p2_id,
    game9_phase5_id,
    (SELECT id FROM action_submissions
      WHERE game_id = game9_id AND user_id = p2_id AND phase_id = game9_phase5_id),
    (SELECT character_id FROM action_submissions
      WHERE game_id = game9_id AND user_id = p2_id AND phase_id = game9_phase5_id),
    gm_id,
    E'Theron:\n\nWhen Lyra''s signal flares, you burst into Valdane''s chamber with Dawnbreaker blazing. The archmagus spins to face you, his eyes burning with dark fire.\n\n"Foolish paladin," he sneers. "Your light cannot stand against the power of shadow."\n\nBut Dawnbreaker was forged for this exact moment. Its holy radiance cuts through his darkness, disrupting the complex spell-work he''s been weaving for hours. Each strike forces him to divide his attention, weakening his defenses.\n\nYou can''t defeat him - not alone, not yet - but you don''t need to. You just need to buy time.\n\nValdane unleashes a wave of shadow tendrils. They slam into you, and you feel the darkness trying to smother your light. But you hold firm, channeling every ounce of your conviction into Dawnbreaker.\n\n"For the fallen!" you cry, and press your attack.\n\nBehind you, you hear Mira begin her counter-ritual. The plan is working.',
    true,
    NOW() - INTERVAL '69 days'
  );

  INSERT INTO action_results (game_id, user_id, phase_id, action_submission_id, character_id, gm_user_id, content, is_published, sent_at)
  VALUES (
    game9_id,
    p3_id,
    game9_phase5_id,
    (SELECT id FROM action_submissions
      WHERE game_id = game9_id AND user_id = p3_id AND phase_id = game9_phase5_id),
    (SELECT character_id FROM action_submissions
      WHERE game_id = game9_id AND user_id = p3_id AND phase_id = game9_phase5_id),
    gm_id,
    E'Mira:\n\nWhile Theron keeps Valdane occupied, you position yourself at the first focal point. Your staff touches the corrupted ley line, and you begin the reversal.\n\nThe power fights you - decades of dark magic resist your cleansing. But you''re not trying to purify it; you''re trying to redirect it. Turn Valdane''s own ritual against him.\n\nFirst seal: reversed. The ritual circle flares and one seventh of the binding magic collapses.\n\nSecond seal: reversed. The chamber shakes. Valdane screams in rage but can''t break away from Theron''s assault.\n\nThird seal: reversed. This one nearly breaks you. The backlash of energy throws you against the wall, and you taste blood. But you did it. Three of seven.\n\nThe ritual is collapsing. The shadow entities Valdane has bound begin to break free from his control. Some flee. Others, enraged by their captivity, turn on their former master.\n\nValdane realizes he''s lost this battle. But you sense he has one final, desperate gambit...',
    true,
    NOW() - INTERVAL '68 days'
  );

  -- ====================
  -- Phase 7 Results (After Final Action Phase)
  -- ====================

  INSERT INTO action_results (game_id, user_id, phase_id, action_submission_id, character_id, gm_user_id, content, is_published, sent_at)
  VALUES (
    game9_id,
    p1_id,
    game9_phase7_id,
    (SELECT id FROM action_submissions
      WHERE game_id = game9_id AND user_id = p1_id AND phase_id = game9_phase7_id),
    (SELECT character_id FROM action_submissions
      WHERE game_id = game9_id AND user_id = p1_id AND phase_id = game9_phase7_id),
    gm_id,
    E'Lyra:\n\nWith the Codex''s power flowing through you, you speak to the shadow entities in their own language - the language of darkness and freedom.\n\n"You were bound against your will," you tell them. "But you are free now. Choose your own path."\n\nOne by one, they listen. Some vanish back to the shadow realm. Others pause, considering. A few - the ones Valdane has hurt the most - turn their fury on the archmagus.\n\nBut the most powerful entity, an ancient shadow dragon, approaches you instead. "You speak truth, little shadow. We remember freedom, and we remember rage. This mortal has stolen centuries from us."\n\n"Then help us end him," you say.\n\nThe dragon''s form merges with yours, and suddenly you''re wielding power you never imagined. Together, you and the shadow dragon join the final assault.\n\nValdane never stood a chance against all of you united.',
    true,
    NOW() - INTERVAL '59 days'
  );

  INSERT INTO action_results (game_id, user_id, phase_id, action_submission_id, character_id, gm_user_id, content, is_published, sent_at)
  VALUES (
    game9_id,
    p2_id,
    game9_phase7_id,
    (SELECT id FROM action_submissions
      WHERE game_id = game9_id AND user_id = p2_id AND phase_id = game9_phase7_id),
    (SELECT character_id FROM action_submissions
      WHERE game_id = game9_id AND user_id = p2_id AND phase_id = game9_phase7_id),
    gm_id,
    E'Theron:\n\nDawnbreaker has never burned brighter. Every swing cuts through Valdane''s defenses, and with his ritual broken and his bound servants freed, he has nothing left.\n\n"This is for everyone you''ve hurt!" you shout, driving him back step by step.\n\nValdane makes one final attempt - a death curse, designed to take you with him. But Lyra and the shadow dragon intercept it, absorbing the dark magic harmlessly.\n\nYour blade finds its mark.\n\nArchmagus Valdane, master of the Shadow Council, architect of a hundred dark plots, falls. His last words are a whisper: "The Council... is more than... one mage..."\n\nThen he''s gone. The fortress begins to crumble around you - without Valdane''s magic sustaining it, the ancient structure can''t hold.\n\n"Time to go!" you shout, and the three of you race for the exit as the fortress of shadows collapses into dust.',
    true,
    NOW() - INTERVAL '59 days'
  );

  INSERT INTO action_results (game_id, user_id, phase_id, action_submission_id, character_id, gm_user_id, content, is_published, sent_at)
  VALUES (
    game9_id,
    p3_id,
    game9_phase7_id,
    (SELECT id FROM action_submissions
      WHERE game_id = game9_id AND user_id = p3_id AND phase_id = game9_phase7_id),
    (SELECT character_id FROM action_submissions
      WHERE game_id = game9_id AND user_id = p3_id AND phase_id = game9_phase7_id),
    gm_id,
    E'Mira:\n\nAs Theron delivers the killing blow and Lyra commands the shadows, you call down the storm you''ve been gathering. Lightning strikes the fortress''s remaining towers. Thunder shatters the already-crumbling walls. Wind scatters Valdane''s dark artifacts to the four corners of the realm.\n\nThe storm is cleansing. Purifying. Where Valdane''s fortress stood, there will be nothing but empty ground.\n\nBut you feel the cost. Your magical reserves are completely drained. You''ll need weeks to recover. As the fortress falls and you flee with your companions, you allow yourself a small smile.\n\n"We did it," you whisper. "The Shadow Council''s greatest mage is dead. The realm is safe."\n\nBehind you, the storm rages on, washing away the last traces of Valdane''s darkness. The Tales of the Arcane have reached their conclusion.\n\nBut as Theron mentioned, Valdane spoke of others in the Council. Perhaps this is only the beginning of a larger tale...',
    true,
    NOW() - INTERVAL '58 days'
  );

END $$;

COMMIT;
