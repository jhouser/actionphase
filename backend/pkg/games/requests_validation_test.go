package games

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/render"
)

// bindUpdate runs a JSON body through the same render.Bind path the update
// handler uses, mirroring bindCreate in character_sheet_config_test.go.
func bindUpdate(t *testing.T, body string) (*UpdateGameRequest, error) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/games/1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	data := &UpdateGameRequest{}
	return data, render.Bind(req, data)
}

// TestGameRequestTagValidation covers the `validate` tags on the game request
// types. They sat inert for a while: Bind only parsed the character sheet and
// checked the schedule fields, so a blank title reached the service and
// surfaced to the GM as a 500 "unexpected error" rather than a 400.
func TestGameRequestTagValidation(t *testing.T) {
	const goodDescription = "A description long enough to validate."

	t.Run("create rejects a missing title", func(t *testing.T) {
		_, err := bindCreate(t, `{"description":"`+goodDescription+`"}`)
		if err == nil {
			t.Fatal("expected a missing title to be rejected")
		}
		if !strings.Contains(err.Error(), "title is required") {
			t.Errorf("expected 'title is required', got %q", err.Error())
		}
	})

	t.Run("create rejects a whitespace-only title", func(t *testing.T) {
		_, err := bindCreate(t, `{"title":"   ","description":"`+goodDescription+`"}`)
		if err == nil {
			t.Fatal("expected a whitespace-only title to be rejected")
		}
		if !strings.Contains(err.Error(), "title is required") {
			t.Errorf("expected 'title is required', got %q", err.Error())
		}
	})

	t.Run("create rejects a too-short title", func(t *testing.T) {
		_, err := bindCreate(t, `{"title":"ab","description":"`+goodDescription+`"}`)
		if err == nil {
			t.Fatal("expected a two-character title to be rejected")
		}
		if !strings.Contains(err.Error(), "title must be at least 3 characters") {
			t.Errorf("expected a min-length message, got %q", err.Error())
		}
	})

	t.Run("create rejects a too-short description", func(t *testing.T) {
		_, err := bindCreate(t, `{"title":"A Test Game","description":"short"}`)
		if err == nil {
			t.Fatal("expected a five-character description to be rejected")
		}
		if !strings.Contains(err.Error(), "description must be at least 10 characters") {
			t.Errorf("expected a min-length message, got %q", err.Error())
		}
	})

	t.Run("create trims a padded title", func(t *testing.T) {
		data, err := bindCreate(t, `{"title":"  A Test Game  ","description":"`+goodDescription+`"}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data.Title != "A Test Game" {
			t.Errorf("expected title trimmed, got %q", data.Title)
		}
	})

	t.Run("create accepts a valid body", func(t *testing.T) {
		if _, err := bindCreate(t, `{"title":"A Test Game","description":"`+goodDescription+`"}`); err != nil {
			t.Fatalf("expected a valid body to pass, got %v", err)
		}
	})

	t.Run("update rejects a whitespace-only title", func(t *testing.T) {
		_, err := bindUpdate(t, `{"title":"   ","description":"`+goodDescription+`"}`)
		if err == nil {
			t.Fatal("expected a whitespace-only title to be rejected")
		}
		if !strings.Contains(err.Error(), "title is required") {
			t.Errorf("expected 'title is required', got %q", err.Error())
		}
	})

	t.Run("update accepts a valid body", func(t *testing.T) {
		if _, err := bindUpdate(t, `{"title":"A Test Game","description":"`+goodDescription+`"}`); err != nil {
			t.Fatalf("expected a valid body to pass, got %v", err)
		}
	})

	t.Run("tag validation runs before the schedule check", func(t *testing.T) {
		// A body that is bad in two ways should name the blank title, not the
		// partial schedule: the field the GM actually needs to fix first.
		_, err := bindCreate(t, `{"title":"","description":"`+goodDescription+`","common_room_open_day":1}`)
		if err == nil {
			t.Fatal("expected the body to be rejected")
		}
		if !strings.Contains(err.Error(), "title is required") {
			t.Errorf("expected the title error to win, got %q", err.Error())
		}
	})
}
