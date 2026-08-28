package games

import "strings"

// splitCommaSeparated splits a comma-separated query parameter, dropping empty
// entries.
//
// The chi version of this file carried hand-rolled splitString and trimString
// helpers reimplementing strings.Split and strings.TrimSpace; they went with the
// handlers that used them.
func splitCommaSeparated(s string) []string {
	var result []string
	for _, item := range strings.Split(s, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
