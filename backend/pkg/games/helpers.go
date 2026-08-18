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
// response body.
//
// Returns nil for an empty or all-defaults config so the key is omitted
// entirely rather than sent as {}, keeping "no overrides" a single shape on the
// wire. A malformed stored value is treated as no config rather than failing the
// request: the column is server-written and validated on the way in, so a parse
// failure here means a bug or a hand-edited row, and dropping a whole game
// response over a cosmetic label override would be the worse outcome.
func characterSheetResponse(stored []byte) *core.CharacterSheetConfig {
	config, err := core.UnmarshalCharacterSheetConfig(stored)
	if err != nil {
		return nil
	}
	if config.Labels == nil {
		return nil
	}
	return &config
}
