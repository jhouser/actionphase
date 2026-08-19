package games

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/render"
)

// bindCreate runs a JSON body through the same render.Bind path the handler uses.
func bindCreate(t *testing.T, body string) (*CreateGameRequest, error) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/games", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	data := &CreateGameRequest{}
	return data, render.Bind(req, data)
}

// validBody wraps a character_sheet fragment in an otherwise-valid game body, so
// a rejection can only be coming from the sheet config.
func validBody(characterSheet string) string {
	return `{"title":"A Test Game","description":"A description long enough to validate.","character_sheet":` + characterSheet + `}`
}

func TestCreateGameRequestCharacterSheetBinding(t *testing.T) {
	t.Run("absent config binds as an empty config", func(t *testing.T) {
		data, err := bindCreate(t, `{"title":"A Test Game","description":"A description long enough to validate."}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data.CharacterSheetConfig().Labels != nil {
			t.Errorf("expected no labels, got %+v", data.CharacterSheetConfig().Labels)
		}
	})

	t.Run("labels bind through", func(t *testing.T) {
		data, err := bindCreate(t, validBody(`{"labels":{"skills":"Approaches"}}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		labels := data.CharacterSheetConfig().Labels
		if labels == nil || labels.Skills != "Approaches" {
			t.Fatalf("expected skills override, got %+v", labels)
		}
	})

	// The reason CharacterSheet is a json.RawMessage rather than the typed struct.
	// render.Bind decodes permissively, so binding straight into the struct would
	// DISCARD an unknown key silently and report success. If this test fails, the
	// strict decode has been bypassed and the blob can accumulate junk again.
	t.Run("unknown key is rejected, not silently dropped", func(t *testing.T) {
		if _, err := bindCreate(t, validBody(`{"labels":{},"tabs":["skills"]}`)); err == nil {
			t.Fatal("expected unknown key 'tabs' to be rejected")
		}
	})

	t.Run("unknown nested label key is rejected", func(t *testing.T) {
		if _, err := bindCreate(t, validBody(`{"labels":{"abilities":"Powers"}}`)); err == nil {
			t.Fatal("expected unknown label 'abilities' to be rejected")
		}
	})

	// Label validation runs in Bind as well as in the service. The service is
	// the real guard, but an error raised there renders as a 500 "unexpected
	// error" — so a GM typing an over-long tab label would be told the server
	// broke. Failing in Bind makes it the 400 it actually is. Verified over the
	// wire: this returned 500 before the Bind-side check was added.
	t.Run("over-long label is rejected at bind time, not left to the service", func(t *testing.T) {
		long := strings.Repeat("a", 40)
		if _, err := bindCreate(t, validBody(`{"labels":{"skills":"`+long+`"}}`)); err == nil {
			t.Fatal("expected an over-long label to be rejected during Bind")
		}
	})

	t.Run("control characters are rejected at bind time", func(t *testing.T) {
		if _, err := bindCreate(t, validBody(`{"labels":{"skills":"Ap\nproaches"}}`)); err == nil {
			t.Fatal("expected a newline in a label to be rejected during Bind")
		}
	})

	t.Run("whitespace-only label binds as no override", func(t *testing.T) {
		data, err := bindCreate(t, validBody(`{"labels":{"skills":"   "}}`))
		if err != nil {
			t.Fatalf("whitespace-only is an unset label, not an error: %v", err)
		}
		if data.CharacterSheetConfig().Labels != nil {
			t.Errorf("expected the label to collapse away, got %+v", data.CharacterSheetConfig().Labels)
		}
	})

	t.Run("labels are trimmed at bind time", func(t *testing.T) {
		data, err := bindCreate(t, validBody(`{"labels":{"skills":"  Approaches  "}}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		labels := data.CharacterSheetConfig().Labels
		if labels == nil || labels.Skills != "Approaches" {
			t.Fatalf("expected a trimmed label, got %+v", labels)
		}
	})
}

func TestUpdateGameRequestCharacterSheetBinding(t *testing.T) {
	bindUpdate := func(body string) (*UpdateGameRequest, error) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/games/1", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		data := &UpdateGameRequest{}
		return data, render.Bind(req, data)
	}

	t.Run("labels bind through", func(t *testing.T) {
		data, err := bindUpdate(validBody(`{"labels":{"numbers":"Resources"}}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		labels := data.CharacterSheetConfig().Labels
		if labels == nil || labels.Numbers != "Resources" {
			t.Fatalf("expected numbers override, got %+v", labels)
		}
	})

	t.Run("unknown key is rejected", func(t *testing.T) {
		if _, err := bindUpdate(validBody(`{"labels":{},"tabs":[]}`)); err == nil {
			t.Fatal("expected unknown key 'tabs' to be rejected")
		}
	})
}

func TestCharacterSheetResponse(t *testing.T) {
	t.Run("empty stored config omits the key entirely", func(t *testing.T) {
		// Every game predating the feature stores exactly this.
		if got := characterSheetResponse([]byte(`{}`)); got != nil {
			t.Errorf("expected nil so the key is omitted, got %+v", got)
		}
	})

	t.Run("nil stored config omits the key entirely", func(t *testing.T) {
		if got := characterSheetResponse(nil); got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})

	t.Run("stored overrides are carried as stored", func(t *testing.T) {
		got := characterSheetResponse([]byte(`{"labels":{"skills":"Approaches"}}`))
		if got == nil || got.Labels == nil {
			t.Fatal("expected the override to be carried")
		}
		if got.Labels.Skills != "Approaches" {
			t.Errorf("skills = %q", got.Labels.Skills)
		}
		// Defaults are NOT filled in server-side — the frontend owns them, so
		// there is exactly one place that knows what a default label is.
		if got.Labels.Inventory != "" || got.Labels.Numbers != "" {
			t.Errorf("server filled in defaults it should not know: %+v", got.Labels)
		}
	})

	t.Run("a malformed stored value degrades to no config", func(t *testing.T) {
		// Server-written and validated on the way in, so this means a bug or a
		// hand-edited row. Losing a label override beats failing the whole
		// game response over one.
		if got := characterSheetResponse([]byte(`{"labels":`)); got != nil {
			t.Errorf("expected nil for malformed stored JSON, got %+v", got)
		}
	})

	t.Run("omitempty actually omits the key", func(t *testing.T) {
		encoded, err := json.Marshal(&GameResponse{ID: 1, Title: "T"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(string(encoded), "character_sheet") {
			t.Errorf("expected character_sheet to be omitted, got %s", encoded)
		}
	})
}
