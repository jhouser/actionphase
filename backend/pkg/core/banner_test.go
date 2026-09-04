package core

// Unit tests for the shared banner helpers.
//
// These were promoted here from pkg/games in Phase 7 so game and community
// uploads share one size cap and one MIME allowlist. They were covered only
// indirectly, through the two handler suites; a direct test means a change to
// the allowlist or the cap fails here rather than in a distant HTTP test whose
// name says nothing about MIME types.

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractBannerPathFromURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "local backend URL",
			url:  "http://localhost:3000/uploads/banners/communities/7/1699999999.png",
			want: "banners/communities/7/1699999999.png",
		},
		{
			name: "S3 backend URL",
			url:  "https://bucket.s3.us-east-1.amazonaws.com/banners/games/42/1699999999.jpg",
			want: "banners/games/42/1699999999.jpg",
		},
		{
			// The prefix is matched from the LAST occurrence, so a bucket or
			// path segment that happens to contain "banners/" cannot truncate
			// the key early.
			name: "prefix appearing twice takes the last",
			url:  "https://cdn.example.com/banners/mirror/banners/communities/3/1.webp",
			want: "banners/communities/3/1.webp",
		},
		{
			// A value that never went through our upload path. Falling back to
			// the last segment keeps a stray delete scoped to a leaf name
			// rather than a whole prefix.
			name: "no banners prefix falls back to the last segment",
			url:  "https://example.com/legacy/image.png",
			want: "image.png",
		},
		{
			name: "bare key is returned unchanged",
			url:  "image.png",
			want: "image.png",
		},
		{
			name: "empty string",
			url:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ExtractBannerPathFromURL(tt.url))
		})
	}
}

func TestBannerMimeTypeFromFilename(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"banner.jpg", "image/jpeg"},
		{"banner.jpeg", "image/jpeg"},
		{"banner.png", "image/png"},
		{"banner.webp", "image/webp"},
		// Case is normalised: a file straight off a camera or Windows box is
		// commonly upper-cased, and rejecting it would be arbitrary.
		{"BANNER.PNG", "image/png"},
		{"banner.JPG", "image/jpeg"},
		// Unrecognised types resolve to octet-stream, which then FAILS the
		// allowlist. That is the point: an unknown extension is rejected
		// rather than passed through untyped.
		{"banner.gif", "application/octet-stream"},
		{"banner.svg", "application/octet-stream"},
		{"banner", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := BannerMimeTypeFromFilename(tt.filename)
			assert.Equal(t, tt.want, got)
			if tt.want == "application/octet-stream" {
				assert.False(t, AllowedBannerMimeTypes[got],
					"an unrecognised extension must not survive the allowlist")
			}
		})
	}
}

// SVG is excluded deliberately, not by oversight: it can carry script and
// would be served from our own origin. Pinning it in a test keeps a later
// "add more image formats" change from quietly reintroducing the hole.
func TestAllowedBannerMimeTypesExcludesSVG(t *testing.T) {
	assert.False(t, AllowedBannerMimeTypes["image/svg+xml"])
	assert.True(t, AllowedBannerMimeTypes["image/jpeg"])
	assert.True(t, AllowedBannerMimeTypes["image/png"])
	assert.True(t, AllowedBannerMimeTypes["image/webp"])
	assert.Len(t, AllowedBannerMimeTypes, 3)
}

func TestBannerExtFromMime(t *testing.T) {
	assert.Equal(t, ".jpg", BannerExtFromMime("image/jpeg"))
	assert.Equal(t, ".png", BannerExtFromMime("image/png"))
	assert.Equal(t, ".webp", BannerExtFromMime("image/webp"))
	// Unreachable from the upload paths, which check the allowlist first, but
	// an empty extension is the safe answer rather than a guess.
	assert.Equal(t, "", BannerExtFromMime("image/gif"))
}

func TestReadAndValidateBannerSize(t *testing.T) {
	t.Run("returns the full contents for an image under the cap", func(t *testing.T) {
		reader, err := ReadAndValidateBannerSize(strings.NewReader("image bytes"))
		require.NoError(t, err)

		got, err := io.ReadAll(reader)
		require.NoError(t, err)
		// The reader must be re-readable: the caller consumes it AFTER the
		// size is known, so a helper that drained the input without buffering
		// would upload an empty object.
		assert.Equal(t, "image bytes", string(got))
	})

	t.Run("accepts an image exactly at the cap", func(t *testing.T) {
		// The boundary case the LimitReader(+1) is there to distinguish. An
		// off-by-one here would reject a legitimate 5MB upload.
		reader, err := ReadAndValidateBannerSize(strings.NewReader(strings.Repeat("a", MaxBannerSize)))
		require.NoError(t, err)

		got, err := io.ReadAll(reader)
		require.NoError(t, err)
		assert.Len(t, got, MaxBannerSize)
	})

	t.Run("rejects an image one byte over the cap", func(t *testing.T) {
		_, err := ReadAndValidateBannerSize(strings.NewReader(strings.Repeat("a", MaxBannerSize+1)))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "5MB")
	})

	t.Run("empty input is not an error", func(t *testing.T) {
		// Size is the only thing this checks; an empty body is caught upstream
		// by huma's required:"true" on the form field.
		reader, err := ReadAndValidateBannerSize(strings.NewReader(""))
		require.NoError(t, err)

		got, err := io.ReadAll(reader)
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}
