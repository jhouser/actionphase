package games

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"actionphase/pkg/humaconfig"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
)

// bindThroughHuma runs a JSON body through huma's real decode-and-resolve path
// and hands back the bound body.
//
// These tests used to call render.Bind on CreateGameRequest. That pipeline is
// gone: huma owns request binding now, and it is huma's strictness -- not a
// json.RawMessage plus DisallowUnknownFields -- that keeps an unknown key from
// being silently dropped. Exercising the real path is the whole point of the
// unknown-key cases below.
func bindThroughHuma[T any](t *testing.T, method, body string) (*T, error) {
	t.Helper()

	type in struct{ Body *T }

	var bound *T
	r := chi.NewRouter()
	api := humaconfig.New(r, "test", "1.0.0")
	huma.Register(api, huma.Operation{
		OperationID: "bind",
		Method:      method,
		Path:        "/bind",
	}, func(ctx context.Context, i *in) (*struct{}, error) {
		bound = i.Body
		return &struct{}{}, nil
	})

	req := httptest.NewRequest(method, "/bind", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code >= 400 {
		return nil, fmt.Errorf("rejected with %d: %s", rec.Code, rec.Body.String())
	}
	return bound, nil
}

func bindCreate(t *testing.T, body string) (*createGameBody, error) {
	t.Helper()
	return bindThroughHuma[createGameBody](t, http.MethodPost, body)
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
		if data.CharacterSheet != nil {
			t.Errorf("expected no sheet, got %+v", data.CharacterSheet)
		}
	})

	t.Run("labels bind through", func(t *testing.T) {
		data, err := bindCreate(t, validBody(`{"labels":{"skills":"Approaches"}}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data.CharacterSheet == nil || data.CharacterSheet.Labels == nil ||
			data.CharacterSheet.Labels.Skills != "Approaches" {
			t.Fatalf("expected skills override, got %+v", data.CharacterSheet)
		}
	})

	// Huma rejects unknown properties on nested objects, which is what replaced
	// the json.RawMessage + DisallowUnknownFields workaround the chi version
	// needed. If this test fails, the strict decode has been bypassed and the
	// blob can accumulate junk again.
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

	// Label validation runs in Resolve as well as in the service. The service is
	// the real guard, but an error raised there renders as a 500 "unexpected
	// error" -- so a GM typing an over-long tab label would be told the server
	// broke. Failing during request binding makes it the 400 it actually is.
	t.Run("over-long label is rejected at bind time, not left to the service", func(t *testing.T) {
		long := strings.Repeat("a", 40)
		if _, err := bindCreate(t, validBody(`{"labels":{"skills":"`+long+`"}}`)); err == nil {
			t.Fatal("expected an over-long label to be rejected during binding")
		}
	})

	t.Run("control characters are rejected at bind time", func(t *testing.T) {
		if _, err := bindCreate(t, validBody(`{"labels":{"skills":"Ap\nproaches"}}`)); err == nil {
			t.Fatal("expected a newline in a label to be rejected during binding")
		}
	})

	t.Run("whitespace-only label binds as no override", func(t *testing.T) {
		data, err := bindCreate(t, validBody(`{"labels":{"skills":"   "}}`))
		if err != nil {
			t.Fatalf("whitespace-only is an unset label, not an error: %v", err)
		}
		if data.CharacterSheet != nil && data.CharacterSheet.Labels != nil {
			t.Errorf("expected the label to collapse away, got %+v", data.CharacterSheet.Labels)
		}
	})

	t.Run("labels are trimmed at bind time", func(t *testing.T) {
		data, err := bindCreate(t, validBody(`{"labels":{"skills":"  Approaches  "}}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data.CharacterSheet == nil || data.CharacterSheet.Labels == nil ||
			data.CharacterSheet.Labels.Skills != "Approaches" {
			t.Fatalf("expected a trimmed label, got %+v", data.CharacterSheet)
		}
	})
}

func TestUpdateGameRequestCharacterSheetBinding(t *testing.T) {
	bindUpdate := func(body string) (*updateGameBody, error) {
		return bindThroughHuma[updateGameBody](t, http.MethodPut, body)
	}

	t.Run("labels bind through", func(t *testing.T) {
		data, err := bindUpdate(validBody(`{"labels":{"numbers":"Resources"}}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data.CharacterSheet == nil || data.CharacterSheet.Labels == nil ||
			data.CharacterSheet.Labels.Numbers != "Resources" {
			t.Fatalf("expected numbers override, got %+v", data.CharacterSheet)
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
