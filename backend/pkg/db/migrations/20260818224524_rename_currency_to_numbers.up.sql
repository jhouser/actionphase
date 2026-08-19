-- Rename the `currency` character sheet module to `numbers`, and drop `abilities`
-- from the set of writable modules.
--
-- The tab was promoted out from under Inventory and generalised: it now holds any
-- per-game numeric track (stress, XP, heat, clocks), not money. A label could not
-- carry that change because module_type is an identifier, not display text — GMs
-- rename the *label* per game, so the key underneath had to stop saying "currency"
-- or it would be misleading forever.
--
-- Safe as a single deploy with no dual-write window, verified against production
-- before writing this migration:
--   * action_result_character_updates WHERE module_type='currency'  -> 0 (no drafts in flight)
--   * character_data WHERE module_type='numbers'                    -> 0 (no UNIQUE collision)
--   * abilities rows carry no content (14 rows, all field_value NULL)
--
-- Data moves BEFORE the constraint tightens; the reverse order would reject the
-- rows it is meant to migrate.

UPDATE character_data
   SET module_type = 'numbers', field_name = 'numbers'
 WHERE module_type = 'currency' AND field_name = 'currency';

UPDATE action_result_character_updates
   SET module_type = 'numbers', field_name = 'numbers'
 WHERE module_type = 'currency' AND field_name = 'currency';

-- 'abilities' leaves the allowlist here rather than in a separate migration: the
-- constraint has to be rewritten for the rename anyway, and listing a module the
-- code can no longer produce would invite it back.
--
-- Existing abilities ROWS are deliberately left in character_data. They are never
-- read again (the read branch is gone), they hold no content, and deleting user
-- data is not something a rename migration should quietly do.
ALTER TABLE action_result_character_updates DROP CONSTRAINT check_module_type;
ALTER TABLE action_result_character_updates ADD CONSTRAINT check_module_type
    CHECK (module_type IN ('skills', 'inventory', 'numbers'));
