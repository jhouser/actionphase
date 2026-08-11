// Package exports assembles completed games into downloadable Markdown archives.
package exports

import (
	"regexp"
	"strings"
)

// ActionPhase stores message bodies as GitHub-flavored Markdown plus three
// custom syntaxes that only the frontend renderer understands
// (frontend/src/components/MarkdownPreview.tsx). Archives are plain Markdown
// meant to outlive the app, so the custom syntaxes are reduced to their
// readable text:
//
//	[color:red]text[/color]        -> text
//	[[Lockpicking|skill:uuid]]     -> [[Lockpicking]]
//	@Ada Lovelace                  -> unchanged (already plain text)
//
// Character mentions need no handling at all: the stored form is the literal
// name, and the frontend only decorates it at render time.
//
// Both transforms skip fenced and inline code spans, matching the frontend's
// splitByCodeBlocks behavior — a post *discussing* the syntax must survive
// with its examples intact.

// allowedColors mirrors ALLOWED_COLORS in MarkdownPreview.tsx. An unrecognized
// color is left untouched there, so it is left untouched here too; stripping it
// would silently discard text the app displays literally.
var allowedColors = map[string]bool{
	"red": true, "green": true, "blue": true, "purple": true, "orange": true,
	"gold": true, "gray": true, "teal": true, "pink": true,
}

var (
	// [color:name]...[/color] — non-greedy, spans newlines.
	colorPattern = regexp.MustCompile(`(?s)\[color:([a-z]+)\](.*?)\[/color\]`)

	// [[Display Name|type:ref]] where type is ability, skill, or item.
	sheetRefPattern = regexp.MustCompile(`\[\[([^\]|]+)\|(?:ability|skill|item):([^\]]+)\]\]`)

	// Fenced blocks (``` or ~~~) and inline code spans. Ordered so fenced
	// blocks win over inline spans that would otherwise match inside them.
	codeSegmentPattern = regexp.MustCompile("(?s)(```.*?```|~~~.*?~~~|`[^`\n]*`)")
)

// StripCustomMarkdown reduces ActionPhase's custom syntaxes to plain Markdown,
// leaving code spans untouched. Standard GFM passes through unchanged.
func StripCustomMarkdown(content string) string {
	return mapOutsideCode(content, func(text string) string {
		return stripColors(stripSheetRefs(text))
	})
}

// mapOutsideCode applies fn to every part of content that is not inside a
// fenced block or inline code span, reassembling the original in order.
func mapOutsideCode(content string, fn func(string) string) string {
	matches := codeSegmentPattern.FindAllStringIndex(content, -1)
	if len(matches) == 0 {
		return fn(content)
	}

	var b strings.Builder
	last := 0
	for _, m := range matches {
		b.WriteString(fn(content[last:m[0]])) // text before the code segment
		b.WriteString(content[m[0]:m[1]])     // the code segment, verbatim
		last = m[1]
	}
	b.WriteString(fn(content[last:]))
	return b.String()
}

// stripColors unwraps [color:x]...[/color] to its inner text. Unknown colors
// are preserved verbatim, matching frontend behavior. Runs repeatedly so that
// nested wrappers collapse fully.
func stripColors(text string) string {
	for {
		replaced := colorPattern.ReplaceAllStringFunc(text, func(match string) string {
			parts := colorPattern.FindStringSubmatch(match)
			if parts == nil || !allowedColors[parts[1]] {
				return match
			}
			return parts[2]
		})
		if replaced == text {
			return text
		}
		text = replaced
	}
}

// stripSheetRefs rewrites [[Display|skill:uuid]] to [[Display]], dropping the
// type and opaque id while keeping the human-readable name.
func stripSheetRefs(text string) string {
	return sheetRefPattern.ReplaceAllString(text, "[[$1]]")
}
