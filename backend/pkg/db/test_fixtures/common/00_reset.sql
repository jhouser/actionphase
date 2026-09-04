-- Reset Test Data
-- This script removes all test data while preserving the schema

BEGIN;

-- Identify all test users by username pattern (catches users whose email was changed by tests)
CREATE TEMP TABLE test_user_ids AS
  SELECT id FROM users WHERE username LIKE 'Test%'
     OR email LIKE 'test_%@example.com'
     OR username LIKE 'e2euser_%'
     OR username LIKE 'loadtest_%'
     OR username LIKE 'nocaptcha_%';

-- Delete in reverse dependency order

-- Clean up messaging and notification tables
DELETE FROM message_reactions WHERE message_id IN (SELECT id FROM messages WHERE game_id IN (SELECT id FROM games WHERE gm_user_id IN (SELECT id FROM test_user_ids)));
DELETE FROM message_recipients WHERE message_id IN (SELECT id FROM messages WHERE game_id IN (SELECT id FROM games WHERE gm_user_id IN (SELECT id FROM test_user_ids)));
DELETE FROM messages WHERE game_id IN (SELECT id FROM games WHERE gm_user_id IN (SELECT id FROM test_user_ids));
DELETE FROM private_messages WHERE conversation_id IN (SELECT id FROM conversations WHERE game_id IN (SELECT id FROM games WHERE gm_user_id IN (SELECT id FROM test_user_ids)));
DELETE FROM conversation_participants WHERE conversation_id IN (SELECT id FROM conversations WHERE game_id IN (SELECT id FROM games WHERE gm_user_id IN (SELECT id FROM test_user_ids)));
DELETE FROM conversations WHERE game_id IN (SELECT id FROM games WHERE gm_user_id IN (SELECT id FROM test_user_ids));
DELETE FROM notifications WHERE game_id IN (SELECT id FROM games WHERE gm_user_id IN (SELECT id FROM test_user_ids));

-- Clean up game-related tables
DELETE FROM phase_transitions WHERE game_id IN (SELECT id FROM games WHERE gm_user_id IN (SELECT id FROM test_user_ids));
DELETE FROM action_results WHERE game_id IN (SELECT id FROM games WHERE gm_user_id IN (SELECT id FROM test_user_ids));
DELETE FROM action_submissions WHERE game_id IN (SELECT id FROM games WHERE gm_user_id IN (SELECT id FROM test_user_ids));
DELETE FROM npc_assignments WHERE character_id IN (SELECT id FROM characters WHERE game_id IN (SELECT id FROM games WHERE gm_user_id IN (SELECT id FROM test_user_ids)));
DELETE FROM character_data WHERE character_id IN (SELECT id FROM characters WHERE game_id IN (SELECT id FROM games WHERE gm_user_id IN (SELECT id FROM test_user_ids)));
DELETE FROM characters WHERE game_id IN (SELECT id FROM games WHERE gm_user_id IN (SELECT id FROM test_user_ids));
DELETE FROM game_phases WHERE game_id IN (SELECT id FROM games WHERE gm_user_id IN (SELECT id FROM test_user_ids));
DELETE FROM game_participants WHERE game_id IN (SELECT id FROM games WHERE gm_user_id IN (SELECT id FROM test_user_ids));
DELETE FROM game_applications WHERE game_id IN (SELECT id FROM games WHERE gm_user_id IN (SELECT id FROM test_user_ids));
DELETE FROM games WHERE gm_user_id IN (SELECT id FROM test_user_ids);

-- Clean up ban tables
DELETE FROM ip_bans WHERE TRUE;
DELETE FROM fingerprint_bans WHERE TRUE;

-- Clean up communities. Ordered before the users DELETE because
-- communities.owner_user_id is ON DELETE RESTRICT -- a surviving community
-- would block its owner's deletion and fail the whole reset.
--
-- Bans and moderators would CASCADE from communities, but are deleted
-- explicitly so a ban on a test user in a community owned by someone else
-- (which is not covered by the cascade) is cleaned up too.
DELETE FROM community_ban_events WHERE target_user_id IN (SELECT id FROM test_user_ids)
                                    OR actor_user_id IN (SELECT id FROM test_user_ids);
DELETE FROM community_bans WHERE user_id IN (SELECT id FROM test_user_ids);
DELETE FROM community_moderators WHERE user_id IN (SELECT id FROM test_user_ids);
DELETE FROM communities WHERE owner_user_id IN (SELECT id FROM test_user_ids);

-- Clean up user-related tables
DELETE FROM sessions WHERE user_id IN (SELECT id FROM test_user_ids);
DELETE FROM users WHERE id IN (SELECT id FROM test_user_ids);

COMMIT;
