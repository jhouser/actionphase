package games

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"actionphase/pkg/core"
)

// formatPgtypeTime converts a pgtype.Time (microseconds since midnight) to "HH:MM" string.
// Seconds are intentionally truncated; schedule times are always written as whole minutes by parseHHMM.
func formatPgtypeTime(t pgtype.Time) string {
	total := t.Microseconds / 1e6
	h := total / 3600
	m := (total % 3600) / 60
	return fmt.Sprintf("%02d:%02d", h, m)
}

// characterSheetResponse decodes a stored games.character_sheet column for a
// response body. Thin alias for the core helper, which the cross-game character
// payload also uses so both surfaces render identical labels.
func characterSheetResponse(stored []byte) *core.CharacterSheetConfig {
	return core.CharacterSheetConfigForResponse(stored)
}
