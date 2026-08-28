package http

import (
	"testing"

	"actionphase/pkg/core"

	"gopkg.in/yaml.v3"
)

// specDocument builds the router and renders the OpenAPI document it describes.
//
// No database is touched: Router only wires handlers and registers huma
// operations, and rendering the spec reflects over Go types rather than calling
// anything.
func specDocument(t *testing.T) map[string]any {
	t.Helper()

	h := &Handler{App: core.NewTestApp(nil)}
	_, docsHandler := h.Router()

	raw, err := docsHandler.Spec()
	if err != nil {
		t.Fatalf("failed to render spec: %v", err)
	}

	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("rendered spec is not valid YAML: %v", err)
	}
	return doc
}

// TestSpecTagsCoverEveryOperation fails when an operation is grouped under a
// tag the document never describes.
//
// This is the half TestSpecTagsAreWellFormed cannot do: pkg/docs owns the tag
// list but has no idea what routes exist, and importing pkg/http there would be
// an import cycle. Here both are visible.
//
// The hand-written spec had drifted ten tags behind by the time it was retired
// — every package converted after it was written introduced tags nobody
// declared, and nothing noticed because an undeclared tag still renders, just
// without a heading.
func TestSpecTagsCoverEveryOperation(t *testing.T) {
	doc := specDocument(t)

	declared := map[string]bool{}
	tags, _ := doc["tags"].([]any)
	for _, raw := range tags {
		if tag, ok := raw.(map[string]any); ok {
			if name, ok := tag["name"].(string); ok {
				declared[name] = true
			}
		}
	}

	paths, _ := doc["paths"].(map[string]any)
	if len(paths) == 0 {
		t.Fatal("no paths in the rendered spec")
	}

	missing := map[string][]string{}
	for path, rawItem := range paths {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		for method, rawOp := range item {
			op, ok := rawOp.(map[string]any)
			if !ok {
				continue
			}
			opTags, _ := op["tags"].([]any)
			for _, rawTag := range opTags {
				name, _ := rawTag.(string)
				if name != "" && !declared[name] {
					missing[name] = append(missing[name], method+" "+path)
				}
			}
		}
	}

	for name, ops := range missing {
		t.Errorf("tag %q is used by %d operation(s) but not described in specTags(); "+
			"add it to backend/pkg/docs/spec_metadata.go (e.g. %s)",
			name, len(ops), ops[0])
	}
}

// TestSpecCoversEveryOperationWithASummary catches an operation registered
// without a Summary, which renders in Swagger UI as a bare method and path.
func TestSpecCoversEveryOperationWithASummary(t *testing.T) {
	doc := specDocument(t)
	paths, _ := doc["paths"].(map[string]any)

	for path, rawItem := range paths {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		for method, rawOp := range item {
			op, ok := rawOp.(map[string]any)
			if !ok {
				continue
			}
			if summary, _ := op["summary"].(string); summary == "" {
				t.Errorf("%s %s has no summary", method, path)
			}
		}
	}
}
