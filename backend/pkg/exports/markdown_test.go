package exports

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStripCustomMarkdown(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		// --- color spans ---
		{
			name:    "unwraps allowed color",
			content: "The door is [color:red]locked[/color].",
			want:    "The door is locked.",
		},
		{
			name:    "unwraps multiple colors in one line",
			content: "[color:red]Fire[/color] and [color:blue]ice[/color].",
			want:    "Fire and ice.",
		},
		{
			name:    "unwraps color spanning newlines",
			content: "[color:green]line one\nline two[/color]",
			want:    "line one\nline two",
		},
		{
			name:    "collapses nested colors",
			content: "[color:red]outer [color:blue]inner[/color] tail[/color]",
			want:    "outer inner tail",
		},
		{
			// Frontend leaves unknown colors as literal text; archive must match
			// rather than silently deleting content the app displays.
			name:    "preserves unknown color verbatim",
			content: "[color:chartreuse]unknown[/color]",
			want:    "[color:chartreuse]unknown[/color]",
		},
		{
			name:    "leaves unmatched opening tag alone",
			content: "[color:red]dangling",
			want:    "[color:red]dangling",
		},

		// --- sheet item references ---
		{
			name:    "rewrites skill ref to display name",
			content: "She used [[Lockpicking|skill:abc-123]] on the door.",
			want:    "She used [[Lockpicking]] on the door.",
		},
		{
			name:    "rewrites ability and item refs",
			content: "[[Second Sight|ability:x1]] and [[Rope|item:y2]]",
			want:    "[[Second Sight]] and [[Rope]]",
		},
		{
			name:    "leaves plain wiki-style link alone",
			content: "[[Just A Name]]",
			want:    "[[Just A Name]]",
		},
		{
			name:    "leaves unknown ref type alone",
			content: "[[Thing|spell:z9]]",
			want:    "[[Thing|spell:z9]]",
		},

		// --- character mentions pass through ---
		{
			name:    "leaves character mention unchanged",
			content: "@Ada Lovelace what do you think?",
			want:    "@Ada Lovelace what do you think?",
		},

		// --- code block protection (ports splitByCodeBlocks) ---
		{
			name:    "preserves custom syntax inside fenced block",
			content: "Use this:\n```\n[color:red]text[/color]\n```\ndone",
			want:    "Use this:\n```\n[color:red]text[/color]\n```\ndone",
		},
		{
			name:    "preserves custom syntax inside tilde fence",
			content: "~~~\n[[Skill|skill:1]]\n~~~",
			want:    "~~~\n[[Skill|skill:1]]\n~~~",
		},
		{
			name:    "preserves custom syntax inside inline code",
			content: "Type `[color:red]x[/color]` to color text.",
			want:    "Type `[color:red]x[/color]` to color text.",
		},
		{
			name:    "strips outside fence but not inside",
			content: "[color:red]stripped[/color]\n```\n[color:red]kept[/color]\n```",
			want:    "stripped\n```\n[color:red]kept[/color]\n```",
		},
		{
			name:    "handles text after closing fence",
			content: "```\n[color:red]a[/color]\n```\n[color:blue]b[/color]",
			want:    "```\n[color:red]a[/color]\n```\nb",
		},

		// --- standard GFM untouched ---
		{
			name:    "leaves standard markdown alone",
			content: "# Head\n\n**bold** _em_ [link](http://x)\n\n- a\n- b",
			want:    "# Head\n\n**bold** _em_ [link](http://x)\n\n- a\n- b",
		},
		{
			name:    "handles empty content",
			content: "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, StripCustomMarkdown(tt.content))
		})
	}
}

// A post that mixes prose, both custom syntaxes, and a fenced example — the
// realistic shape that motivated code-block skipping.
func TestStripCustomMarkdown_MixedRealisticPost(t *testing.T) {
	content := strings.Join([]string{
		"@Charles Babbage I checked the [color:red]cellar door[/color].",
		"",
		"I rolled [[Lockpicking|skill:9f2]] and failed.",
		"",
		"To write colored text use:",
		"```markdown",
		"[color:gold]treasure[/color]",
		"```",
		"",
		"[color:teal]Anyone else?[/color]",
	}, "\n")

	got := StripCustomMarkdown(content)

	assert.Contains(t, got, "@Charles Babbage I checked the cellar door.")
	assert.Contains(t, got, "I rolled [[Lockpicking]] and failed.")
	assert.Contains(t, got, "Anyone else?")
	// The fenced example must survive verbatim.
	assert.Contains(t, got, "[color:gold]treasure[/color]")
	assert.NotContains(t, got, "[color:red]")
	assert.NotContains(t, got, "skill:9f2")
}

// Guards against catastrophic backtracking / runaway loops on adversarial input.
func TestStripCustomMarkdown_PathologicalInput(t *testing.T) {
	cases := map[string]string{
		"many unclosed openers": strings.Repeat("[color:red]", 500),
		"many closers":          strings.Repeat("[/color]", 500),
		"deeply nested":         strings.Repeat("[color:red]", 200) + "x" + strings.Repeat("[/color]", 200),
		"unterminated fence":    "```\n" + strings.Repeat("[color:red]a[/color]\n", 200),
	}

	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			done := make(chan string, 1)
			go func() { done <- StripCustomMarkdown(input) }()
			select {
			case out := <-done:
				assert.NotNil(t, out)
			case <-time.After(5 * time.Second):
				t.Fatal("StripCustomMarkdown did not terminate within 5s")
			}
		})
	}
}
