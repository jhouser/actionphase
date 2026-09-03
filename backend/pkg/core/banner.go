package core

// Shared validation and path helpers for banner image uploads.
//
// These live in core rather than in pkg/games because two upload paths now use
// them -- game banners and community banners -- and a banner accepted by one
// endpoint but rejected by the other would be a confusing difference with no
// reason behind it. Keeping the size cap and the MIME allowlist in one place
// means widening either is a single edit.
//
// pkg/users/avatar_service.go still carries its own copy of the URL-to-path
// extraction. Deliberately left alone: avatars are a different size budget and
// a different storage prefix, and folding them in here would be a refactor of
// a path this phase does not otherwise touch.

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// MaxBannerSize caps an uploaded banner at 5MB.
const MaxBannerSize = 5 * 1024 * 1024

// AllowedBannerMimeTypes is the upload allowlist. Formats every browser both
// renders and can produce; notably excludes SVG, which is script-bearing and
// would be served from our own origin.
var AllowedBannerMimeTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// ReadAndValidateBannerSize buffers the upload, rejecting anything over
// MaxBannerSize.
//
// Reads MaxBannerSize+1 bytes so that hitting the cap is distinguishable from
// a file that happens to be exactly at it. Buffering is required regardless:
// the storage backend needs a reader it can consume after the size is known.
func ReadAndValidateBannerSize(file io.Reader) (io.Reader, error) {
	limited := io.LimitReader(file, MaxBannerSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	if int64(len(data)) > MaxBannerSize {
		return nil, fmt.Errorf("image too large. Maximum size is 5MB")
	}
	return strings.NewReader(string(data)), nil
}

// ExtractBannerPathFromURL recovers the storage key from a stored public URL,
// so an old object can be deleted when it is replaced or removed.
//
// The stored column holds a full public URL, whose prefix differs between the
// local and S3 backends; the key is the suffix from "banners/" onward.
// Falls back to the last path segment for a URL without that prefix, which
// keeps a malformed value from turning into a delete of something unrelated.
//
// Banners only. Avatars are not handled here even though their layout is
// parallel: pkg/users/avatar_service.go has its own extractPathFromURL, so a
// branch for them would be unreachable from every caller of this function.
func ExtractBannerPathFromURL(url string) string {
	if index := strings.LastIndex(url, "banners/"); index != -1 {
		return url[index:]
	}
	if lastSlash := strings.LastIndex(url, "/"); lastSlash != -1 {
		return url[lastSlash+1:]
	}
	return url
}

// BannerMimeTypeFromFilename infers a content type when the client sends none.
//
// Returns application/octet-stream for anything unrecognised, which fails the
// AllowedBannerMimeTypes check -- so an unknown extension is rejected rather
// than passed through untyped.
func BannerMimeTypeFromFilename(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// BannerExtFromMime supplies a storage-path extension when the uploaded
// filename has none. Returns "" for an unrecognised type, which cannot occur
// on the upload paths since the MIME allowlist is checked first.
func BannerExtFromMime(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}
