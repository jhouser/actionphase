-- Restore action_submissions.is_draft.
--
-- Deliberately DEFAULT FALSE, not the original DEFAULT TRUE. The old default
-- was a trap: every write passed an explicit false, so an insert that omitted
-- the column would silently create a draft that submission stats would then
-- count as unsubmitted. Rolling back should not reinstate that.
ALTER TABLE action_submissions ADD COLUMN is_draft BOOLEAN NOT NULL DEFAULT FALSE;
