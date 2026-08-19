-- Reverse of the currency -> numbers rename.
--
-- The constraint is widened FIRST: the UPDATEs below write 'currency' back, which
-- the tightened constraint would reject. This is the mirror of the up migration's
-- ordering constraint, in the opposite direction.
--
-- 'abilities' returns to the allowlist so the schema matches what it was before,
-- even though no code writes it any more. A down migration's job is to restore the
-- prior shape, not to keep a subset of the change.
ALTER TABLE action_result_character_updates DROP CONSTRAINT check_module_type;
ALTER TABLE action_result_character_updates ADD CONSTRAINT check_module_type
    CHECK (module_type IN ('abilities', 'skills', 'inventory', 'currency'));

UPDATE character_data
   SET module_type = 'currency', field_name = 'currency'
 WHERE module_type = 'numbers' AND field_name = 'numbers';

UPDATE action_result_character_updates
   SET module_type = 'currency', field_name = 'currency'
 WHERE module_type = 'numbers' AND field_name = 'numbers';
