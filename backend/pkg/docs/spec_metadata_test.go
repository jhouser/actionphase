package docs

import "testing"

// TestSpecTagsAreWellFormed checks the tag list itself: every entry names a tag
// and describes it, and no tag is declared twice.
//
// Completeness against the registered operations cannot be checked here —
// pkg/docs does not know the routes, and importing pkg/http would be a cycle.
// That half lives in pkg/http (TestSpecTagsCoverEveryOperation), which can see
// both.
func TestSpecTagsAreWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for i, raw := range specTags() {
		tag, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("tag %d is not a map: %#v", i, raw)
		}
		name, _ := tag["name"].(string)
		desc, _ := tag["description"].(string)
		if name == "" {
			t.Errorf("tag %d has no name", i)
			continue
		}
		if desc == "" {
			t.Errorf("tag %q has no description", name)
		}
		if seen[name] {
			t.Errorf("tag %q declared twice", name)
		}
		seen[name] = true
	}
}

// TestSpecBaseIsAFreshDocument guards the contract Spec relies on: the merge
// mutates the returned map, so a shared one would accumulate generated paths
// across calls and the second request would differ from the first.
func TestSpecBaseIsAFreshDocument(t *testing.T) {
	first := specBase()
	firstPaths, ok := first["paths"].(map[string]any)
	if !ok {
		t.Fatal("paths missing from specBase")
	}
	firstPaths["/injected"] = map[string]any{}

	second := specBase()
	secondPaths, _ := second["paths"].(map[string]any)
	if _, leaked := secondPaths["/injected"]; leaked {
		t.Fatal("specBase returned a shared map; mutations leak between calls")
	}
	if _, ok := secondPaths["/ping"]; !ok {
		t.Error("expected /ping in the base document")
	}
}
