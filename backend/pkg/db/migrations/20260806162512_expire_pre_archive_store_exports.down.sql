-- Irreversible by design: the up migration discards storage paths whose files
-- are unreachable under the current storage layout, and the original values
-- cannot be reconstructed from the remaining columns.
--
-- Nothing to undo. Affected rows read as expired and regenerate on request,
-- which is the same state a normal retention sweep would leave them in.
SELECT 1;
