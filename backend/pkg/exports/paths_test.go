package exports

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlug(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"simple title", "The Hollow Crown", "the-hollow-crown"},
		{"collapses whitespace", "The   Hollow    Crown", "the-hollow-crown"},
		{"strips punctuation", "The Body in the Library!?", "the-body-in-the-library"},
		{"folds accents", "Café Málaga", "cafe-malaga"},
		{"folds eszett and ligatures", "Straße Œuvre", "strasse-oeuvre"},
		{"trims leading/trailing hyphens", "---hello---", "hello"},
		{"collapses repeated hyphens", "a---b", "a-b"},
		{"lowercases", "ALL CAPS", "all-caps"},
		{"keeps digits", "Chapter 7 Part 2", "chapter-7-part-2"},

		// Path traversal / separator injection — the security-relevant cases.
		{"strips forward slashes", "a/b/c", "a-b-c"},
		{"strips backslashes", `a\b\c`, "a-b-c"},
		{"neutralizes dot-dot", "../../etc/passwd", "etc-passwd"},
		{"neutralizes leading dot", ".hidden", "hidden"},
		{"neutralizes bare dots", "..", "fallback"},
		{"strips null byte", "a\x00b", "a-b"},
		{"strips newlines", "a\nb", "a-b"},
		{"strips colon (windows drive)", "C:file", "c-file"},

		// Degenerate input must still yield a usable name.
		{"empty string", "", "fallback"},
		{"whitespace only", "   ", "fallback"},
		{"punctuation only", "!!!???", "fallback"},
		{"emoji only", "🎲🎲🎲", "fallback"},
		{"CJK only falls back", "日本語", "fallback"},

		// Windows reserved device names.
		{"reserved name con", "CON", "con-file"},
		{"reserved name com1", "com1", "com1-file"},
		{"reserved-like but safe", "console", "console"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Slug(tt.in, "fallback"))
		})
	}
}

func TestSlug_NeverProducesPathSeparators(t *testing.T) {
	// Whatever the input, a slug must be a single path segment.
	inputs := []string{
		"../../../etc/passwd",
		`..\..\windows\system32`,
		"/absolute/path",
		"nested/deep/path/file",
		"mixed/..\\traversal",
		strings.Repeat("../", 50),
	}

	for _, in := range inputs {
		got := Slug(in, "fallback")
		assert.NotContains(t, got, "/", "slug of %q must not contain /", in)
		assert.NotContains(t, got, `\`, "slug of %q must not contain backslash", in)
		assert.NotEqual(t, "..", got)
		assert.NotEmpty(t, got)
	}
}

func TestSlug_LengthCapped(t *testing.T) {
	got := Slug(strings.Repeat("verylongword ", 40), "fallback")
	assert.LessOrEqual(t, len(got), maxSlugLength)
	assert.False(t, strings.HasSuffix(got, "-"), "must not end with a hyphen after truncation")
}

func TestNumberedSlug(t *testing.T) {
	assert.Equal(t, "001-first-post", NumberedSlug(1, "First Post", "post"))
	assert.Equal(t, "042-answer", NumberedSlug(42, "Answer", "post"))
	assert.Equal(t, "007-post", NumberedSlug(7, "", "post"))

	// Duplicate titles stay distinct because the ordinal differs.
	a := NumberedSlug(1, "Same Title", "post")
	b := NumberedSlug(2, "Same Title", "post")
	assert.NotEqual(t, a, b)

	// Zero padding keeps lexical order aligned with numeric order.
	assert.True(t, NumberedSlug(2, "b", "p") > NumberedSlug(1, "a", "p"))
	assert.True(t, NumberedSlug(10, "b", "p") > NumberedSlug(9, "a", "p"))
}

func TestPhaseDirName(t *testing.T) {
	// The underscore in the phase_type value becomes a hyphen in the directory
	// name, which reads better on disk than "common_room".
	assert.Equal(t, "01-common-room-the-gathering",
		PhaseDirName(1, "common_room", "The Gathering"))
	assert.Equal(t, "02-action-descent",
		PhaseDirName(2, "action", "Descent"))
	assert.Equal(t, "03-interlude-untitled",
		PhaseDirName(3, "interlude", ""))
	// Two-digit padding keeps phase 10 sorting after phase 9.
	assert.True(t, PhaseDirName(10, "action", "x") > PhaseDirName(9, "action", "x"))
}

func TestGameDirName(t *testing.T) {
	assert.Equal(t, "game-164-the-hollow-crown", GameDirName(164, "The Hollow Crown"))
	assert.Equal(t, "game-1-untitled", GameDirName(1, ""))
	// A malicious title cannot break out of the root directory.
	assert.Equal(t, "game-9-etc-passwd", GameDirName(9, "../../etc/passwd"))
}
