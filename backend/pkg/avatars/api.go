package avatars

import (
	"strings"

	"actionphase/pkg/core"
)

type Handler struct {
	App              *core.App
	CharacterService core.CharacterServiceInterface
}

// AvatarUploadResponse is the success body for an avatar upload.
type AvatarUploadResponse struct {
	AvatarURL string `json:"avatar_url" doc:"Public URL of the stored avatar"`
}

// detectContentType guesses a content type from a filename extension. It is a
// fallback for multipart parts that arrive without a Content-Type header.
func detectContentType(filename string) string {
	if len(filename) < 4 {
		return "application/octet-stream"
	}
	switch filename[len(filename)-4:] {
	case ".jpg", "jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case "webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// isValidationError reports whether err is a client-input problem (400) rather
// than a server fault (500). The avatar service signals both through error
// strings, so the classification is by message.
func isValidationError(err error) bool {
	msg := err.Error()
	for _, phrase := range []string{"invalid file type", "too large", "Only JPG, PNG, and WebP"} {
		if strings.Contains(msg, phrase) {
			return true
		}
	}
	return false
}
