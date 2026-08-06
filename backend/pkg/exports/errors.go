package exports

import "errors"

// ErrGameNotCompleted is returned when an export is requested for a game that
// has not reached the completed state.
//
// This is a hard invariant, not a policy knob: the archive contains private
// conversations, action submissions, and published results, which are only
// disclosable under public archive mode. CanUserViewGame grants read access to
// any authenticated user once a game is completed, and nothing weaker justifies
// producing this artifact. Callers should surface it as a 409, not a 500.
var ErrGameNotCompleted = errors.New("game is not completed")

// ErrExportInProgress is returned when an export already exists for a game in
// pending or running state. The partial unique index idx_game_exports_one_active
// enforces this at the database level so concurrent requests coalesce onto a
// single job rather than assembling the same archive repeatedly.
var ErrExportInProgress = errors.New("an export is already in progress for this game")
