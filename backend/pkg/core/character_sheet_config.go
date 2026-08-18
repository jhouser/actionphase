package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxCharacterSheetLabelLength bounds a GM-supplied tab label. These render as
// tab labels in a horizontal strip that already has to fit on a phone, so the
// limit is a layout constraint rather than a storage one.
const MaxCharacterSheetLabelLength = 24

// CharacterSheetConfig is a game's per-game character sheet configuration,
// stored as JSONB on games.character_sheet.
//
// It is deliberately SPARSE: a field is present only when the GM has actually
// overridden it. An absent field means "use the default", and the defaults live
// in the frontend so exactly one place knows them. Persisting a default here
// would fork that knowledge and freeze today's wording into the stored row.
//
// Unknown keys are rejected on write (see ValidateCharacterSheetConfig). That is
// what keeps this blob from accumulating junk before the planned tab-composition
// feature gives it a fuller schema.
type CharacterSheetConfig struct {
	// Pointer, not a value: encoding/json's omitempty has no effect on a struct
	// field, so a value here would serialize an all-defaults game as
	// {"labels":{}} instead of {}. Nil means "no overrides at all".
	Labels *CharacterSheetLabels `json:"labels,omitempty"`
}

// CharacterSheetLabels holds GM overrides for the stat tab names. The keys match
// the storage module_type of each tab, which is also the React symbol and the
// default label — see the character sheet refactor plan's naming invariant.
type CharacterSheetLabels struct {
	Skills    string `json:"skills,omitempty"`
	Inventory string `json:"inventory,omitempty"`
	Numbers   string `json:"numbers,omitempty"`
}

// IsZero reports whether no override is set, so the config can be stored as a
// bare `{}` rather than `{"labels":{}}`.
func (l CharacterSheetLabels) IsZero() bool {
	return l.Skills == "" && l.Inventory == "" && l.Numbers == ""
}

// ValidateCharacterSheetConfig normalizes and validates a config supplied by a
// client, returning the cleaned value to store.
//
// Normalization is part of validation here on purpose: a label that is only
// whitespace is not an error, it is an unset label, and must be stored as an
// absent key rather than as "" so that "no override" has exactly one
// representation on the wire and in the database.
func ValidateCharacterSheetConfig(config CharacterSheetConfig) (CharacterSheetConfig, error) {
	if config.Labels == nil {
		return config, nil
	}

	labels := []struct {
		name  string
		value *string
	}{
		{"skills", &config.Labels.Skills},
		{"inventory", &config.Labels.Inventory},
		{"numbers", &config.Labels.Numbers},
	}

	for _, label := range labels {
		trimmed := strings.TrimSpace(*label.value)

		// Whitespace-only is "no override", not an error.
		if trimmed == "" {
			*label.value = ""
			continue
		}

		// U+FFFD rather than utf8.ValidString: encoding/json replaces invalid
		// bytes with the replacement character during decode, so by the time a
		// label reaches here it is always valid UTF-8 and a ValidString check
		// can never fire. Its presence means the client sent bytes that were not
		// valid UTF-8, which is worth rejecting rather than storing as mojibake.
		if strings.ContainsRune(trimmed, utf8.RuneError) {
			return config, fmt.Errorf("character sheet label %q is not valid UTF-8", label.name)
		}

		// Counted in runes, not bytes: the limit exists to keep the label
		// readable in a tab strip, and a byte limit would silently allow fewer
		// characters for non-Latin scripts.
		if utf8.RuneCountInString(trimmed) > MaxCharacterSheetLabelLength {
			return config, fmt.Errorf(
				"character sheet label %q must be %d characters or fewer",
				label.name, MaxCharacterSheetLabelLength,
			)
		}

		// Control characters and newlines would break the tab strip's layout and
		// are never meaningful in a label.
		for _, r := range trimmed {
			if r == '\n' || r == '\r' || unicode.IsControl(r) {
				return config, fmt.Errorf(
					"character sheet label %q must not contain control characters or newlines",
					label.name,
				)
			}
		}

		*label.value = trimmed
	}

	// Every override trimmed away to nothing: collapse to the same shape an
	// untouched game has, so "no overrides" is one value rather than two.
	if config.Labels.IsZero() {
		config.Labels = nil
	}

	return config, nil
}

// UnmarshalCharacterSheetConfig parses a stored or client-supplied config,
// rejecting unknown keys at every level.
//
// Strictness is the point. Without it a typo'd or speculative key is accepted
// silently, persists forever, and is indistinguishable from a real setting by
// the time the tab-composition feature tries to define this document properly.
// A nil or empty input is a valid empty config, not an error — that is what
// every pre-existing game's column holds.
func UnmarshalCharacterSheetConfig(data []byte) (CharacterSheetConfig, error) {
	var config CharacterSheetConfig

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return config, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return config, fmt.Errorf("invalid character sheet config: %w", err)
	}

	return config, nil
}

// MarshalCharacterSheetConfig renders a config for storage.
//
// Round-trips through the typed struct rather than storing client bytes, so
// whatever lands in the column is exactly what the type can express — the
// guarantee that makes DisallowUnknownFields worth anything on read.
func MarshalCharacterSheetConfig(config CharacterSheetConfig) ([]byte, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal character sheet config: %w", err)
	}
	return data, nil
}
