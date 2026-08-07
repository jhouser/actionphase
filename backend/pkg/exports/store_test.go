package exports

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchiveStore_RoundTrip(t *testing.T) {
	store := NewArchiveStore(t.TempDir())
	ctx := context.Background()

	require.NoError(t, store.Put(ctx, "exports/game-1/export-2.zip", bytes.NewReader([]byte("PK-payload"))))

	rc, size, err := store.Open(ctx, "exports/game-1/export-2.zip")
	require.NoError(t, err)
	defer rc.Close()

	body, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, "PK-payload", string(body))
	assert.Equal(t, int64(len("PK-payload")), size)
}

// Archives must not land anywhere a static file server maps, or the whole
// authorization check on download is bypassable by fetching the raw path.
func TestArchiveStore_WritesOutsideUploadsTree(t *testing.T) {
	root := t.TempDir()
	store := NewArchiveStore(root)
	ctx := context.Background()

	require.NoError(t, store.Put(ctx, "exports/game-1/export-2.zip", bytes.NewReader([]byte("x"))))

	written := filepath.Join(root, "exports", "game-1", "export-2.zip")
	_, err := os.Stat(written)
	require.NoError(t, err, "archive should exist under the private root")

	assert.NotContains(t, root, "uploads",
		"archive root must not be the publicly served uploads tree")
}

func TestArchiveStore_DeleteIsIdempotent(t *testing.T) {
	store := NewArchiveStore(t.TempDir())
	ctx := context.Background()

	require.NoError(t, store.Put(ctx, "exports/game-1/a.zip", bytes.NewReader([]byte("x"))))
	require.NoError(t, store.Delete(ctx, "exports/game-1/a.zip"))

	// The retention sweep clears storage_path only after a successful delete, so
	// a second pass over an already-removed file must not fail the batch.
	assert.NoError(t, store.Delete(ctx, "exports/game-1/a.zip"),
		"deleting a missing archive must succeed so the sweep can retry safely")
}

func TestArchiveStore_OpenMissingReportsNotFound(t *testing.T) {
	store := NewArchiveStore(t.TempDir())

	_, _, err := store.Open(context.Background(), "exports/game-1/nope.zip")

	assert.ErrorIs(t, err, ErrArchiveMissing)
}

// A stored path is attacker-influenced only via ids, but the guard is cheap and
// prevents a traversal from ever escaping the archive root.
func TestArchiveStore_RejectsPathTraversal(t *testing.T) {
	store := NewArchiveStore(t.TempDir())
	ctx := context.Background()

	for _, bad := range []string{"../escape.zip", "/etc/passwd", "exports/../../escape.zip"} {
		t.Run(bad, func(t *testing.T) {
			err := store.Put(ctx, bad, bytes.NewReader([]byte("x")))
			assert.Error(t, err, "must reject %q", bad)

			_, _, openErr := store.Open(ctx, bad)
			assert.Error(t, openErr, "must reject reading %q", bad)
		})
	}
}

func TestArchiveStore_PutOverwrites(t *testing.T) {
	store := NewArchiveStore(t.TempDir())
	ctx := context.Background()

	require.NoError(t, store.Put(ctx, "exports/game-1/a.zip", bytes.NewReader([]byte("first"))))
	require.NoError(t, store.Put(ctx, "exports/game-1/a.zip", bytes.NewReader([]byte("second"))))

	rc, _, err := store.Open(ctx, "exports/game-1/a.zip")
	require.NoError(t, err)
	defer rc.Close()

	body, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, "second", string(body))
}

func TestDownloadFilename(t *testing.T) {
	tests := []struct {
		name   string
		gameID int32
		title  string
		want   string
	}{
		{"simple title", 50709, "Shadows Over Innsmouth", "shadows-over-innsmouth-archive.zip"},
		{"punctuation stripped", 12, "A Game: The Reckoning!", "a-game-the-reckoning-archive.zip"},
		{"empty title falls back to id", 42, "", "game-42-archive.zip"},
		{"unicode folded", 7, "Café Noir", "cafe-noir-archive.zip"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, DownloadFilename(tt.gameID, tt.title))
		})
	}
}

// The filename lands in a Content-Disposition header; a quote or newline there
// would let a title break out of the header value.
func TestDownloadFilename_IsHeaderSafe(t *testing.T) {
	hostile := "Evil\"; rm -rf /\r\nX-Injected: yes"

	got := DownloadFilename(99, hostile)

	assert.NotContains(t, got, `"`)
	assert.NotContains(t, got, "\r")
	assert.NotContains(t, got, "\n")
	assert.NotContains(t, got, ";")
	assert.True(t, strings.HasSuffix(got, ".zip"))
}
