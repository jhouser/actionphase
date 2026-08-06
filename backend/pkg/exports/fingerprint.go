package exports

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	models "actionphase/pkg/db/models"

	"github.com/jackc/pgx/v5/pgtype"
)

// Fingerprint reduces a game's current content to a short hash. A completed
// game is mostly immutable, but a GM can still edit afterward, so a cached
// archive is only reusable while this value is unchanged.
//
// Row counts catch insertions and deletions; max-timestamps catch in-place
// edits. Neither alone is sufficient: an edit leaves counts untouched, and a
// delete-plus-insert in the same instant can leave the max timestamp untouched.
func Fingerprint(row models.GetGameContentFingerprintRow) string {
	var b strings.Builder

	// Fixed field order: the hash must be stable across runs and processes.
	writeCount := func(label string, n int64) {
		fmt.Fprintf(&b, "%s=%d;", label, n)
	}
	writeTime := func(label string, ts pgtype.Timestamptz) {
		if !ts.Valid {
			fmt.Fprintf(&b, "%s=null;", label)
			return
		}
		// UnixNano would overflow for far-future dates; RFC3339Nano in UTC is
		// stable and readable in debug output.
		fmt.Fprintf(&b, "%s=%s;", label, ts.Time.UTC().Format("2006-01-02T15:04:05.000000000Z"))
	}

	writeCount("messages", row.MessageCount)
	writeTime("messages_max", row.MessageMaxTs)
	writeCount("submissions", row.SubmissionCount)
	writeTime("submissions_max", row.SubmissionMaxTs)
	writeCount("results", row.ResultCount)
	writeTime("results_max", row.ResultMaxTs)
	writeCount("private_messages", row.PrivateMessageCount)
	writeTime("private_messages_max", row.PrivateMessageMaxTs)
	writeCount("phases", row.PhaseCount)
	writeCount("characters", row.CharacterCount)
	writeTime("characters_max", row.CharacterMaxTs)
	writeCount("handouts", row.HandoutCount)
	writeTime("handouts_max", row.HandoutMaxTs)
	writeCount("polls", row.PollCount)
	writeTime("polls_max", row.PollMaxTs)
	writeTime("game_updated", row.GameUpdatedAt)

	// Version prefix: bumping it invalidates every cached archive, which is the
	// escape hatch when the renderer changes and old output must be rebuilt.
	sum := sha256.Sum256([]byte(fingerprintVersion + "|" + b.String()))
	return hex.EncodeToString(sum[:])
}

// fingerprintVersion must be bumped whenever a change to the rendering or
// archive layout would make previously generated archives stale even though
// the underlying game data is unchanged.
const fingerprintVersion = "v1"
