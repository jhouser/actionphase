package games

import (
	"net/http"
	"strings"
	"testing"
)

// bindUpdate runs a JSON body through huma's real binding path, mirroring
// bindCreate in character_sheet_config_test.go.
func bindUpdate(t *testing.T, body string) (*updateGameBody, error) {
	t.Helper()
	return bindThroughHuma[updateGameBody](t, http.MethodPut, body)
}

// TestGameRequestTagValidation covers the length and presence constraints on
// the game request bodies. These sat inert for a while before the chi version
// wired them up: Bind only parsed the character sheet and checked the schedule
// fields, so a blank title reached the service and surfaced to the GM as a 500
// "unexpected error" rather than a 400.
//
// The constraints now live as huma tags rather than `validate` tags, so the
// wording differs -- huma says "expected length >= 3" where core.ValidateStruct
// said "title must be at least 3 characters". Every input rejected before is
// still rejected, and with the same 400; only the message text moved.
func TestGameRequestTagValidation(t *testing.T) {
	const goodDescription = "A description long enough to validate."

	t.Run("create rejects a missing title", func(t *testing.T) {
		_, err := bindCreate(t, `{"community_id":1,"description":"`+goodDescription+`"}`)
		if err == nil {
			t.Fatal("expected a missing title to be rejected")
		}
		if !strings.Contains(err.Error(), "expected required property title to be present") {
			t.Errorf("expected a missing-property message, got %q", err.Error())
		}
	})

	t.Run("create rejects a whitespace-only title", func(t *testing.T) {
		_, err := bindCreate(t, `{"community_id":1,"title":"   ","description":"`+goodDescription+`"}`)
		if err == nil {
			t.Fatal("expected a whitespace-only title to be rejected")
		}
		// Trimmed to empty by the Resolve hook, then reported as blank rather
		// than passing a whitespace title through to the database.
		if !strings.Contains(err.Error(), "title must not be blank") {
			t.Errorf("expected a blank-title message, got %q", err.Error())
		}
	})

	t.Run("create rejects a too-short title", func(t *testing.T) {
		_, err := bindCreate(t, `{"community_id":1,"title":"ab","description":"`+goodDescription+`"}`)
		if err == nil {
			t.Fatal("expected a two-character title to be rejected")
		}
		if !strings.Contains(err.Error(), "expected length >= 3") {
			t.Errorf("expected a min-length message, got %q", err.Error())
		}
	})

	t.Run("create rejects a too-short description", func(t *testing.T) {
		_, err := bindCreate(t, `{"community_id":1,"title":"A Test Game","description":"short"}`)
		if err == nil {
			t.Fatal("expected a five-character description to be rejected")
		}
		if !strings.Contains(err.Error(), "expected length >= 10") {
			t.Errorf("expected a min-length message, got %q", err.Error())
		}
	})

	t.Run("create trims a padded title", func(t *testing.T) {
		data, err := bindCreate(t, `{"community_id":1,"title":"  A Test Game  ","description":"`+goodDescription+`"}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data.Title != "A Test Game" {
			t.Errorf("expected title trimmed, got %q", data.Title)
		}
	})

	t.Run("create accepts a valid body", func(t *testing.T) {
		if _, err := bindCreate(t, `{"community_id":1,"title":"A Test Game","description":"`+goodDescription+`"}`); err != nil {
			t.Fatalf("expected a valid body to pass, got %v", err)
		}
	})

	t.Run("update rejects a whitespace-only title", func(t *testing.T) {
		_, err := bindUpdate(t, `{"title":"   ","description":"`+goodDescription+`"}`)
		if err == nil {
			t.Fatal("expected a whitespace-only title to be rejected")
		}
		if !strings.Contains(err.Error(), "title must not be blank") {
			t.Errorf("expected a blank-title message, got %q", err.Error())
		}
	})

	t.Run("update accepts a valid body", func(t *testing.T) {
		if _, err := bindUpdate(t, `{"title":"A Test Game","description":"`+goodDescription+`"}`); err != nil {
			t.Fatalf("expected a valid body to pass, got %v", err)
		}
	})

	t.Run("field validation runs before the schedule check", func(t *testing.T) {
		// A body that is bad in two ways should name the bad title, not the
		// partial schedule: the field the GM actually needs to fix first. Huma
		// runs schema validation before Resolve, which preserves that ordering.
		_, err := bindCreate(t, `{"community_id":1,"title":"","description":"`+goodDescription+`","common_room_open_day":1}`)
		if err == nil {
			t.Fatal("expected the body to be rejected")
		}
		if !strings.Contains(err.Error(), "expected length >= 3") {
			t.Errorf("expected the title error to win, got %q", err.Error())
		}
		if strings.Contains(err.Error(), "schedule fields") {
			t.Errorf("the schedule error should not have been reached: %q", err.Error())
		}
	})

	t.Run("an incomplete schedule is still rejected on its own", func(t *testing.T) {
		_, err := bindCreate(t, `{"community_id":1,"title":"A Test Game","description":"`+goodDescription+`","common_room_open_day":1}`)
		if err == nil {
			t.Fatal("expected a partial schedule to be rejected")
		}
		if !strings.Contains(err.Error(), "must be set together or all omitted") {
			t.Errorf("expected the schedule error, got %q", err.Error())
		}
	})
}
