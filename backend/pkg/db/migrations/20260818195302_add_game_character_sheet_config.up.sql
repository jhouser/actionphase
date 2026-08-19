-- Per-game character sheet configuration.
--
-- Holds GM overrides for the character sheet, starting with the tab labels
-- (skills / inventory / numbers). Sparse by design: a key is present only when
-- the GM has actually renamed that tab, so an empty object means "all defaults".
-- Defaults live in the frontend so there is exactly one place that knows them;
-- storing them here would fork that knowledge and freeze today's wording into
-- every existing game's row.
--
-- JSONB rather than columns because the eventual tab-composition feature will
-- add structure here (which tabs exist, in what order), and the Go side rejects
-- unknown keys so the blob cannot silently accumulate junk before then.
--
-- No index: this is only ever read as part of an already-keyed games row.
ALTER TABLE games
    ADD COLUMN character_sheet JSONB NOT NULL DEFAULT '{}'::jsonb;
