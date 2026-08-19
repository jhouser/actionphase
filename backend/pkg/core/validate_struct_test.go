package core

import (
	"encoding/json"
	"strings"
	"testing"
)

type sampleRequest struct {
	Name     string          `json:"name" validate:"required,min=1,max=8"`
	Content  string          `json:"content" validate:"required,min=1"`
	CharID   int32           `json:"character_id" validate:"required"`
	Note     string          `json:"note,omitempty"`
	Raw      json.RawMessage `json:"raw,omitempty"`
	Untagged string          `validate:"required"`
}

func validSample() *sampleRequest {
	return &sampleRequest{Name: "Rowan", Content: "hello", CharID: 7, Untagged: "x"}
}

func TestValidateStructAcceptsValidRequest(t *testing.T) {
	if err := ValidateStruct(validSample()); err != nil {
		t.Fatalf("expected valid request to pass, got %v", err)
	}
}

func TestValidateStructOptionalFieldsMayBeOmitted(t *testing.T) {
	// The long tail of request types have optional fields the frontend omits.
	// Validation must not start rejecting payloads that work today.
	req := validSample()
	req.Note = ""
	req.Raw = nil

	if err := ValidateStruct(req); err != nil {
		t.Fatalf("expected omitted optional fields to pass, got %v", err)
	}
}

func TestValidateStructReportsMissingFieldsByJSONName(t *testing.T) {
	err := ValidateStruct(&sampleRequest{})
	if err == nil {
		t.Fatal("expected empty request to fail validation")
	}

	msg := err.Error()
	for _, want := range []string{"name is required", "content is required", "character_id is required"} {
		if !strings.Contains(msg, want) {
			t.Errorf("expected message to contain %q, got %q", want, msg)
		}
	}
	// The Go field name must not leak when a json tag is available.
	if strings.Contains(msg, "CharID") {
		t.Errorf("expected JSON field name, but Go name leaked: %q", msg)
	}
}

func TestValidateStructRejectsWhitespaceOnlyStrings(t *testing.T) {
	// The concrete bug this closes: renaming a character to "   " satisfied the
	// stock `required` tag, reached the service, and rendered as a 500.
	req := validSample()
	req.Name = "   \t\n "

	err := ValidateStruct(req)
	if err == nil {
		t.Fatal("expected whitespace-only name to fail validation")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("expected 'name is required', got %q", err.Error())
	}
}

func TestValidateStructTrimsStringFieldsInPlace(t *testing.T) {
	// Handlers read the struct after Bind, so the trimmed value is the one that
	// gets persisted.
	req := validSample()
	req.Name = "  Rowan  "
	req.Note = "\tpadded\n"

	if err := ValidateStruct(req); err != nil {
		t.Fatalf("expected padded but non-empty values to pass, got %v", err)
	}
	if req.Name != "Rowan" {
		t.Errorf("expected Name trimmed to %q, got %q", "Rowan", req.Name)
	}
	if req.Note != "padded" {
		t.Errorf("expected Note trimmed to %q, got %q", "padded", req.Note)
	}
}

func TestValidateStructLeavesRawJSONUntouched(t *testing.T) {
	// json.RawMessage is a []byte payload, not a text field; trimming it would
	// corrupt bodies that legitimately carry leading whitespace.
	req := validSample()
	req.Raw = json.RawMessage(" {\"a\": 1} ")

	if err := ValidateStruct(req); err != nil {
		t.Fatalf("expected request to pass, got %v", err)
	}
	if string(req.Raw) != " {\"a\": 1} " {
		t.Errorf("expected Raw left untouched, got %q", string(req.Raw))
	}
}

func TestValidateStructReportsLengthViolations(t *testing.T) {
	req := validSample()
	req.Name = "far too long a name"

	err := ValidateStruct(req)
	if err == nil {
		t.Fatal("expected over-length name to fail validation")
	}
	if want := "name must be at most 8 characters"; !strings.Contains(err.Error(), want) {
		t.Errorf("expected %q, got %q", want, err.Error())
	}
}

func TestValidateStructFallsBackToGoNameWithoutJSONTag(t *testing.T) {
	req := validSample()
	req.Untagged = ""

	err := ValidateStruct(req)
	if err == nil {
		t.Fatal("expected missing untagged field to fail validation")
	}
	if want := "Untagged is required"; !strings.Contains(err.Error(), want) {
		t.Errorf("expected %q, got %q", want, err.Error())
	}
}

type nestedRequest struct {
	Title string       `json:"title" validate:"required"`
	Items []nestedItem `json:"items"`
	Child *nestedItem  `json:"child,omitempty"`
}

type nestedItem struct {
	Label string `json:"label"`
}

func TestValidateStructTrimsNestedAndSliceFields(t *testing.T) {
	req := &nestedRequest{
		Title: " Quest ",
		Items: []nestedItem{{Label: "  first  "}},
		Child: &nestedItem{Label: "\tchild\t"},
	}

	if err := ValidateStruct(req); err != nil {
		t.Fatalf("expected request to pass, got %v", err)
	}
	if req.Title != "Quest" {
		t.Errorf("expected Title trimmed, got %q", req.Title)
	}
	if req.Items[0].Label != "first" {
		t.Errorf("expected slice element trimmed, got %q", req.Items[0].Label)
	}
	if req.Child.Label != "child" {
		t.Errorf("expected nested pointer field trimmed, got %q", req.Child.Label)
	}
}

func TestValidateStructRejectsNonStruct(t *testing.T) {
	// A caller passing the wrong thing is a programming error, not a 400.
	if err := ValidateStruct(nil); err == nil {
		t.Fatal("expected nil to produce an error")
	}
}
