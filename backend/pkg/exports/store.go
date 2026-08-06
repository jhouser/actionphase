package exports

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ErrArchiveMissing reports an archive that is not on disk. Callers treat this
// as "regenerate", not as a server fault: archives are deliberately ephemeral.
var ErrArchiveMissing = errors.New("archive not found")

// ArchiveStorer is the archive persistence contract. Narrow by intent: it is
// satisfied by local disk only (see ArchiveStore) and exists so tests can
// inject write and delete failures without touching the filesystem.
type ArchiveStorer interface {
	Put(ctx context.Context, key string, r io.Reader) error
	Open(ctx context.Context, key string) (io.ReadCloser, int64, error)
	Delete(ctx context.Context, key string) error
}

// Compile-time verification that ArchiveStore satisfies the contract.
var _ ArchiveStorer = (*ArchiveStore)(nil)

// ArchiveStore is local-disk storage for finished export archives.
//
// Deliberately NOT the shared StorageBackendInterface, which in production is
// backed by a public-read S3 bucket. Everything else that backend holds
// (avatars, banners) is public-by-design imagery; an archive contains every
// private message and action result in a game and must never land there.
// Two concrete hazards that separation avoids:
//
//   - The S3 uploader hardcodes "Cache-Control: public, max-age=31536000,
//     immutable". A CDN would keep serving an archive for a year after the
//     7-day sweep deleted it, silently voiding retention.
//   - Objects in that bucket are served unsigned, so a guessable path would
//     expose a whole game's private content without authentication.
//
// Archives live outside the statically served uploads tree, so the only way to
// obtain one is the download handler, which authorizes the caller first.
//
// Local disk means archives do not survive container replacement and are not
// shared across replicas. That is an accepted tradeoff: a missing archive
// regenerates in seconds and the UI already treats "no artifact" as "offer to
// prepare one".
type ArchiveStore struct {
	root string
}

// NewArchiveStore creates a store rooted at a private directory.
func NewArchiveStore(root string) *ArchiveStore {
	return &ArchiveStore{root: root}
}

// resolve maps a storage key to an absolute path, refusing anything that would
// escape the archive root.
func (s *ArchiveStore) resolve(key string) (string, error) {
	if strings.HasPrefix(key, "/") {
		return "", fmt.Errorf("invalid archive path %q: must be relative", key)
	}
	clean := filepath.Clean(key)
	if clean == "." || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("invalid archive path %q: path traversal detected", key)
	}
	full := filepath.Join(s.root, clean)

	// Belt-and-braces: confirm the joined path is still inside the root, which
	// catches traversal forms Clean alone would not.
	rootAbs, err := filepath.Abs(s.root)
	if err != nil {
		return "", fmt.Errorf("resolve archive root: %w", err)
	}
	fullAbs, err := filepath.Abs(full)
	if err != nil {
		return "", fmt.Errorf("resolve archive path: %w", err)
	}
	if fullAbs != rootAbs && !strings.HasPrefix(fullAbs, rootAbs+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid archive path %q: escapes archive root", key)
	}
	return fullAbs, nil
}

// Put writes an archive, creating parent directories and replacing any file
// already at the key.
func (s *ArchiveStore) Put(ctx context.Context, key string, r io.Reader) error {
	full, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("create archive directory: %w", err)
	}

	// Write to a temp file and rename, so a crash mid-write cannot leave a
	// truncated archive that would download as a corrupt ZIP.
	tmp, err := os.CreateTemp(filepath.Dir(full), ".export-*.tmp")
	if err != nil {
		return fmt.Errorf("create archive temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := io.Copy(tmp, r); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write archive: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close archive temp file: %w", err)
	}
	if err := os.Rename(tmpName, full); err != nil {
		return fmt.Errorf("finalize archive: %w", err)
	}
	return nil
}

// Open returns a reader over an archive plus its size, for streaming to a
// client. The caller closes the reader.
func (s *ArchiveStore) Open(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	full, err := s.resolve(key)
	if err != nil {
		return nil, 0, err
	}
	f, err := os.Open(full)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, 0, ErrArchiveMissing
		}
		return nil, 0, fmt.Errorf("open archive: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, fmt.Errorf("stat archive: %w", err)
	}
	return f, info.Size(), nil
}

// Delete removes an archive. Missing files are not an error: the retention
// sweep clears storage_path only after a successful delete, so a retried pass
// must be able to succeed over an already-removed file.
func (s *ArchiveStore) Delete(ctx context.Context, key string) error {
	full, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete archive: %w", err)
	}
	return nil
}

// DownloadFilename is the name the browser saves an archive as.
//
// Built from the game title rather than the storage key, which is id-based:
// ids keep stored paths stable and collision-free, but "export-19.zip" tells
// the person who downloaded it nothing. Runs through Slug, so the result is
// always safe to place in a Content-Disposition header.
func DownloadFilename(gameID int32, title string) string {
	return Slug(title, fmt.Sprintf("game-%d", gameID)) + "-archive.zip"
}
