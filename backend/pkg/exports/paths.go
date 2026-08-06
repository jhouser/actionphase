package exports

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Archive entry names are built from user-supplied text (game titles, character
// names, post bodies), so they must be reduced to something safe to extract on
// any filesystem. Slug rules:
//
//   - lowercase ASCII alphanumerics and hyphens only
//   - no path separators, so a crafted title cannot escape its directory
//   - length-capped, since names come from free text
//   - never empty, so entries always have a usable name
//
// Uniqueness is not a slug concern: callers prefix a zero-padded ordinal
// (see NumberedSlug), which keeps ordering stable and collisions impossible
// even when two posts share a title.

const maxSlugLength = 60

var (
	nonSlugChars     = regexp.MustCompile(`[^a-z0-9]+`)
	repeatedHyphens  = regexp.MustCompile(`-{2,}`)
	windowsReserved  = regexp.MustCompile(`^(con|prn|aux|nul|com[1-9]|lpt[1-9])$`)
	combiningMarkRxp = regexp.MustCompile(`[\x{0300}-\x{036F}]`)
)

// Slug converts arbitrary text into a filesystem-safe archive path segment.
// Returns fallback when the input has no usable characters (e.g. a title that
// is entirely emoji or CJK, which would otherwise slug to an empty string).
func Slug(text, fallback string) string {
	s := strings.ToLower(strings.TrimSpace(text))

	// Fold common accented Latin letters to ASCII so "Café" -> "cafe" rather
	// than losing the character entirely.
	s = foldAccents(s)
	s = combiningMarkRxp.ReplaceAllString(s, "")

	s = nonSlugChars.ReplaceAllString(s, "-")
	s = repeatedHyphens.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")

	if len(s) > maxSlugLength {
		s = s[:maxSlugLength]
		s = strings.Trim(s, "-")
	}

	// Windows refuses these names even with an extension.
	if windowsReserved.MatchString(s) {
		s += "-file"
	}

	if s == "" {
		return fallback
	}
	return s
}

// NumberedSlug prefixes a zero-padded ordinal, giving entries a stable sort
// order in a file browser and guaranteeing uniqueness across duplicate titles.
func NumberedSlug(n int, text, fallback string) string {
	return fmt.Sprintf("%03d-%s", n, Slug(text, fallback))
}

// PhaseDirName names a phase directory as "<number>-<type>-<title>", e.g.
// "02-action-descent". Phase numbers are unique per game, so this cannot collide.
func PhaseDirName(phaseNumber int32, phaseType, title string) string {
	return fmt.Sprintf("%02d-%s-%s",
		phaseNumber,
		Slug(phaseType, "phase"),
		Slug(title, "untitled"),
	)
}

// GameDirName is the archive's single root directory, so extracting never
// scatters files into the current working directory.
func GameDirName(gameID int32, title string) string {
	return fmt.Sprintf("game-%d-%s", gameID, Slug(title, "untitled"))
}

// foldAccents maps accented Latin-1 letters to their ASCII base letter.
func foldAccents(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if replacement, ok := accentFolding[r]; ok {
			b.WriteString(replacement)
			continue
		}
		if r < unicode.MaxASCII {
			b.WriteRune(r)
			continue
		}
		// Non-ASCII with no folding: drop it; nonSlugChars would anyway.
		b.WriteRune(' ')
	}
	return b.String()
}

var accentFolding = map[rune]string{
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a",
	'è': "e", 'é': "e", 'ê': "e", 'ë': "e",
	'ì': "i", 'í': "i", 'î': "i", 'ï': "i",
	'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ö': "o", 'ø': "o",
	'ù': "u", 'ú': "u", 'û': "u", 'ü': "u",
	'ý': "y", 'ÿ': "y",
	'ñ': "n", 'ç': "c", 'ß': "ss",
	'æ': "ae", 'œ': "oe",
}
