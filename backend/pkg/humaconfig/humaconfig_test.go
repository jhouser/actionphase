package humaconfig

import (
	"context"
	"net/http"
	"testing"

	"actionphase/pkg/observability"

	"github.com/danielgtaylor/huma/v2"
)

// stubContext is the minimum huma.Context needed to reach the request context.
//
// huma.Context is a wide interface, but the hook under test only ever calls
// Context() on it. Embedding the interface anonymously is not possible -- the
// embedded field would be named Context and collide with the method of the same
// name -- so the interface is embedded through an intermediate alias whose name
// differs, leaving the remaining methods promoted (and nil, which is safe
// precisely because nothing calls them).
type humaCtx = huma.Context

type stubContext struct {
	humaCtx
	requestCtx context.Context
}

func (s stubContext) Context() context.Context { return s.requestCtx }

// TestInstallProblemErrorFormat_StampsCorrelationID covers the support path: an
// error body alone must be enough to find the request in the logs.
func TestInstallProblemErrorFormat_StampsCorrelationID(t *testing.T) {
	original := huma.NewErrorWithContext
	t.Cleanup(func() { huma.NewErrorWithContext = original })
	InstallProblemErrorFormat()

	ctx := observability.WithCorrelationID(context.Background(), "corr-xyz789")
	err := huma.NewErrorWithContext(stubContext{requestCtx: ctx}, http.StatusForbidden, "nope")

	model, ok := err.(*huma.ErrorModel)
	if !ok {
		t.Fatalf("Expected *huma.ErrorModel, got %T", err)
	}
	if model.Instance != "urn:actionphase:correlation:corr-xyz789" {
		t.Errorf("Unexpected instance: %q", model.Instance)
	}
	// Huma's own RFC 7807 fields must survive untouched.
	if model.Title != "Forbidden" {
		t.Errorf("Unexpected title: %q", model.Title)
	}
	if model.Status != http.StatusForbidden {
		t.Errorf("Unexpected status: %d", model.Status)
	}
	if model.Detail != "nope" {
		t.Errorf("Unexpected detail: %q", model.Detail)
	}
}

// TestInstallProblemErrorFormat_NoCorrelationID guards against emitting a bare
// "urn:actionphase:correlation:" prefix with no ID behind it.
func TestInstallProblemErrorFormat_NoCorrelationID(t *testing.T) {
	original := huma.NewErrorWithContext
	t.Cleanup(func() { huma.NewErrorWithContext = original })
	InstallProblemErrorFormat()

	err := huma.NewErrorWithContext(stubContext{requestCtx: context.Background()}, http.StatusNotFound, "missing")

	model, ok := err.(*huma.ErrorModel)
	if !ok {
		t.Fatalf("Expected *huma.ErrorModel, got %T", err)
	}
	if model.Instance != "" {
		t.Errorf("Expected empty instance, got %q", model.Instance)
	}
}

// TestInstallProblemErrorFormat_ValidationDetails verifies field-level failures
// still reach the client as the RFC 7807 errors[] array. This is the payoff the
// old flattened "error" string could not express.
func TestInstallProblemErrorFormat_ValidationDetails(t *testing.T) {
	original := huma.NewErrorWithContext
	t.Cleanup(func() { huma.NewErrorWithContext = original })
	InstallProblemErrorFormat()

	detail := &huma.ErrorDetail{Message: "expected string", Location: "body.name", Value: 5}
	err := huma.NewErrorWithContext(stubContext{requestCtx: context.Background()},
		http.StatusUnprocessableEntity, "validation failed", detail)

	model, ok := err.(*huma.ErrorModel)
	if !ok {
		t.Fatalf("Expected *huma.ErrorModel, got %T", err)
	}
	if len(model.Errors) != 1 {
		t.Fatalf("Expected 1 error detail, got %d", len(model.Errors))
	}
	if model.Errors[0].Message != "expected string" || model.Errors[0].Location != "body.name" {
		t.Errorf("Unexpected detail: %+v", model.Errors[0])
	}
}

func TestCorrelationInstance(t *testing.T) {
	if got := CorrelationInstance(""); got != "" {
		t.Errorf("Expected empty string for empty ID, got %q", got)
	}
	if got := CorrelationInstance("abc"); got != "urn:actionphase:correlation:abc" {
		t.Errorf("Unexpected URI: %q", got)
	}
}
