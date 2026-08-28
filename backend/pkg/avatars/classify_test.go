package avatars

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// detectContentType is the fallback used when a multipart part arrives with no
// Content-Type header. It picks the stored MIME type, which decides both what
// the browser is later told to render and what the service accepts, so a wrong
// answer here silently mislabels a stored image.
func TestDetectContentType(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{"jpg extension", "portrait.jpg", "image/jpeg"},
		{"jpeg extension", "portrait.jpeg", "image/jpeg"},
		{"png extension", "portrait.png", "image/png"},
		{"webp extension", "portrait.webp", "image/webp"},
		{"unknown extension falls back to octet-stream", "portrait.gif", "application/octet-stream"},
		{"no extension falls back to octet-stream", "portrait", "application/octet-stream"},
		// The function slices the last four bytes, so anything shorter would
		// panic without the length guard.
		{"name shorter than an extension", "a.p", "application/octet-stream"},
		{"empty name", "", "application/octet-stream"},
		// Extension matching is case-sensitive by construction: an uppercase
		// name takes the fallback rather than being treated as a JPEG.
		{"uppercase extension is not matched", "PORTRAIT.JPG", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, detectContentType(tt.filename))
		})
	}
}

// isValidationError decides whether an upload failure renders as 400 or 500.
// Misclassifying a user's bad file as a server fault hides the reason from
// them and pages an on-call for a non-bug, so the mapping is worth pinning.
func TestIsValidationError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"invalid file type is a client problem", errors.New("invalid file type: image/gif"), true},
		{"oversized file is a client problem", errors.New("file too large: 9MB exceeds 5MB"), true},
		{"allowed-types message is a client problem", errors.New("Only JPG, PNG, and WebP images are allowed"), true},
		{"storage failure is a server problem", errors.New("failed to upload to storage: connection refused"), false},
		{"database failure is a server problem", errors.New("failed to update character avatar_url"), false},
		{"empty message is a server problem", errors.New(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isValidationError(tt.err))
		})
	}
}
