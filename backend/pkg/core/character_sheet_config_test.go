package core

import (
	"strings"
	"testing"
)

func TestUnmarshalCharacterSheetConfig(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		errSubstr string
		check     func(t *testing.T, got CharacterSheetConfig)
	}{
		{
			// Every game that predates the feature holds exactly this.
			name:  "empty object is a valid empty config",
			input: `{}`,
			check: func(t *testing.T, got CharacterSheetConfig) {
				if got.Labels != nil {
					t.Errorf("expected nil Labels, got %+v", got.Labels)
				}
			},
		},
		{
			name:  "empty input is a valid empty config",
			input: ``,
			check: func(t *testing.T, got CharacterSheetConfig) {
				if got.Labels != nil {
					t.Errorf("expected nil Labels, got %+v", got.Labels)
				}
			},
		},
		{
			name:  "null is a valid empty config",
			input: `null`,
			check: func(t *testing.T, got CharacterSheetConfig) {
				if got.Labels != nil {
					t.Errorf("expected nil Labels, got %+v", got.Labels)
				}
			},
		},
		{
			name:  "labels parse",
			input: `{"labels":{"skills":"Approaches","numbers":"Resources"}}`,
			check: func(t *testing.T, got CharacterSheetConfig) {
				if got.Labels == nil {
					t.Fatal("expected Labels to be set")
				}
				if got.Labels.Skills != "Approaches" {
					t.Errorf("skills = %q, want %q", got.Labels.Skills, "Approaches")
				}
				if got.Labels.Numbers != "Resources" {
					t.Errorf("numbers = %q, want %q", got.Labels.Numbers, "Resources")
				}
				// Unset override stays empty rather than picking up a default —
				// defaults are the frontend's job.
				if got.Labels.Inventory != "" {
					t.Errorf("inventory = %q, want empty", got.Labels.Inventory)
				}
			},
		},
		{
			// The case that matters: this is what stops the blob accumulating
			// junk before the tab-composition feature defines a real schema.
			name:      "unknown top-level key is rejected",
			input:     `{"labels":{},"tabs":["skills"]}`,
			wantErr:   true,
			errSubstr: "tabs",
		},
		{
			name:      "unknown nested key is rejected",
			input:     `{"labels":{"abilities":"Powers"}}`,
			wantErr:   true,
			errSubstr: "abilities",
		},
		{
			name:    "malformed json is rejected",
			input:   `{"labels":`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnmarshalCharacterSheetConfig([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got config %+v", got)
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not mention %q", err, tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func TestValidateCharacterSheetConfig(t *testing.T) {
	labels := func(skills, inventory, numbers string) CharacterSheetConfig {
		return CharacterSheetConfig{Labels: &CharacterSheetLabels{
			Skills: skills, Inventory: inventory, Numbers: numbers,
		}}
	}

	t.Run("trims surrounding whitespace", func(t *testing.T) {
		got, err := ValidateCharacterSheetConfig(labels("  Approaches  ", "", ""))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Labels.Skills != "Approaches" {
			t.Errorf("skills = %q, want %q", got.Labels.Skills, "Approaches")
		}
	})

	t.Run("whitespace-only label is dropped, not stored as empty string", func(t *testing.T) {
		got, err := ValidateCharacterSheetConfig(labels("   ", "", ""))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Collapses all the way to nil Labels so "no override" has exactly one
		// representation; a stored "" would be a second one.
		if got.Labels != nil {
			t.Fatalf("expected Labels to collapse to nil, got %+v", got.Labels)
		}
		encoded, err := MarshalCharacterSheetConfig(got)
		if err != nil {
			t.Fatalf("unexpected marshal error: %v", err)
		}
		if string(encoded) != "{}" {
			t.Errorf("encoded = %s, want {}", encoded)
		}
	})

	t.Run("accepts a label at exactly the limit", func(t *testing.T) {
		atLimit := strings.Repeat("a", MaxCharacterSheetLabelLength)
		got, err := ValidateCharacterSheetConfig(labels(atLimit, "", ""))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Labels.Skills != atLimit {
			t.Errorf("label at the limit was altered")
		}
	})

	t.Run("rejects an over-long label", func(t *testing.T) {
		tooLong := strings.Repeat("a", MaxCharacterSheetLabelLength+1)
		if _, err := ValidateCharacterSheetConfig(labels(tooLong, "", "")); err == nil {
			t.Fatal("expected an error for an over-long label")
		}
	})

	t.Run("counts length in runes, not bytes", func(t *testing.T) {
		// 24 multi-byte characters is 24 tab-strip characters, and well over 24
		// bytes. A byte limit would reject this and quietly give non-Latin
		// scripts a shorter allowance.
		multibyte := strings.Repeat("あ", MaxCharacterSheetLabelLength)
		if _, err := ValidateCharacterSheetConfig(labels(multibyte, "", "")); err != nil {
			t.Fatalf("unexpected error for a rune-length-valid label: %v", err)
		}
	})

	t.Run("rejects control characters and newlines", func(t *testing.T) {
		for _, bad := range []string{"Ap\nproaches", "Ap\tproaches", "Ap\x00proaches", "Ap\rproaches"} {
			if _, err := ValidateCharacterSheetConfig(labels(bad, "", "")); err == nil {
				t.Errorf("expected an error for label %q", bad)
			}
		}
	})

	t.Run("rejects mojibake from invalid UTF-8 input", func(t *testing.T) {
		// Goes through Unmarshal rather than building the struct directly,
		// because that is the only way this can actually happen: encoding/json
		// silently rewrites invalid bytes to U+FFFD, so a caller cannot hand the
		// validator a genuinely invalid string. Checking utf8.ValidString here
		// instead would be unreachable code that always passes.
		parsed, err := UnmarshalCharacterSheetConfig([]byte("{\"labels\":{\"skills\":\"a\xffb\"}}"))
		if err != nil {
			t.Fatalf("unexpected unmarshal error: %v", err)
		}
		if _, err := ValidateCharacterSheetConfig(parsed); err == nil {
			t.Error("expected an error for a label containing the replacement character")
		}
	})

	t.Run("names the offending label in the error", func(t *testing.T) {
		_, err := ValidateCharacterSheetConfig(labels("", "", strings.Repeat("a", 99)))
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "numbers") {
			t.Errorf("error %q does not name the offending label", err)
		}
	})

	t.Run("nil labels validate as a no-op", func(t *testing.T) {
		got, err := ValidateCharacterSheetConfig(CharacterSheetConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Labels != nil {
			t.Errorf("expected nil Labels, got %+v", got.Labels)
		}
	})
}

func TestCharacterSheetConfigRoundTrip(t *testing.T) {
	t.Run("empty config stores as an empty object", func(t *testing.T) {
		encoded, err := MarshalCharacterSheetConfig(CharacterSheetConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(encoded) != "{}" {
			t.Fatalf("encoded = %s, want {} (a value-typed Labels field would give {\"labels\":{}})", encoded)
		}
	})

	t.Run("only genuine overrides persist", func(t *testing.T) {
		config := CharacterSheetConfig{Labels: &CharacterSheetLabels{Skills: "Approaches"}}
		encoded, err := MarshalCharacterSheetConfig(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(encoded) != `{"labels":{"skills":"Approaches"}}` {
			t.Errorf("encoded = %s, want only the skills override", encoded)
		}
	})

	t.Run("stored bytes parse back to the same config", func(t *testing.T) {
		original := CharacterSheetConfig{Labels: &CharacterSheetLabels{
			Skills: "Approaches", Inventory: "Gear", Numbers: "Resources",
		}}
		encoded, err := MarshalCharacterSheetConfig(original)
		if err != nil {
			t.Fatalf("unexpected marshal error: %v", err)
		}
		decoded, err := UnmarshalCharacterSheetConfig(encoded)
		if err != nil {
			t.Fatalf("unexpected unmarshal error: %v", err)
		}
		if decoded.Labels == nil || *decoded.Labels != *original.Labels {
			t.Errorf("round trip changed the config: %+v", decoded.Labels)
		}
	})
}
