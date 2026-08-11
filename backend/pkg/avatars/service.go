package avatars

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"actionphase/pkg/core"
	db "actionphase/pkg/db/models"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// MaxAvatarSize is the maximum allowed avatar file size (5MB)
	MaxAvatarSize = 5 * 1024 * 1024 // 5MB in bytes

	// Allowed MIME types for avatar uploads
	MimeTypeJPEG = "image/jpeg"
	MimeTypePNG  = "image/png"
	MimeTypeWebP = "image/webp"
)

var allowedMimeTypes = map[string]bool{
	MimeTypeJPEG: true,
	MimeTypePNG:  true,
	MimeTypeWebP: true,
}

// AvatarService implements avatar management for characters.
// Handles file upload validation, storage, and database updates.
type AvatarService struct {
	DB      *pgxpool.Pool
	Storage core.StorageBackendInterface
}

// Compile-time verification that AvatarService implements AvatarServiceInterface
var _ core.AvatarServiceInterface = (*AvatarService)(nil)

// UploadCharacterAvatar uploads an avatar image for a character.
//
// Validation:
//   - File type must be image/jpeg, image/png, or image/webp
//   - File size must be ≤5MB
//
// Process:
//  1. Validate content type
//  2. Read file into memory (to check size)
//  3. Upload new avatar to storage
//  4. Update database with new avatar URL
//
// The previously current avatar file is retained, not deleted — see
// DeleteCharacterAvatar for why.
//
// Returns the public URL of the uploaded avatar.
func (s *AvatarService) UploadCharacterAvatar(
	ctx context.Context,
	characterID int32,
	file io.Reader,
	filename string,
	contentType string,
) (string, error) {
	// Validate content type
	if !allowedMimeTypes[contentType] {
		return "", fmt.Errorf("invalid file type %s. Only JPG, PNG, and WebP images are allowed", contentType)
	}

	// Read file into memory to check size and enable re-reading
	fileData, _, err := readAndValidateSize(file, MaxAvatarSize)
	if err != nil {
		return "", err
	}

	queries := db.New(s.DB)

	// The previous avatar file is deliberately left in storage. Storage paths are
	// timestamped, so each upload creates a new file and superseded ones remain
	// resolvable for any message that pinned them.

	// Generate storage path: avatars/characters/{characterID}/{timestamp}_{filename}
	ext := filepath.Ext(filename)
	if ext == "" {
		// Derive extension from content type
		ext = mimeTypeToExtension(contentType)
	}
	// Superseded avatar files are retained so messages that pinned them keep
	// resolving — which only holds if every upload lands on a distinct path. A
	// timestamp alone is not enough: two uploads can share a millisecond, and the
	// second would silently overwrite the file older posts still point at. The
	// random suffix makes collisions a non-issue regardless of clock granularity.
	storagePath := fmt.Sprintf("avatars/characters/%d/%d_%s%s",
		characterID, time.Now().UnixMilli(), randomSuffix(), ext)

	// Upload to storage
	avatarURL, err := s.Storage.Upload(ctx, storagePath, fileData, contentType)
	if err != nil {
		return "", fmt.Errorf("failed to upload avatar: %w", err)
	}

	// Update database
	_, err = queries.UpdateCharacterAvatar(ctx, db.UpdateCharacterAvatarParams{
		ID:        characterID,
		AvatarUrl: pgtype.Text{String: avatarURL, Valid: true},
	})
	if err != nil {
		// Try to clean up uploaded file
		_ = s.Storage.Delete(ctx, storagePath)
		return "", fmt.Errorf("failed to update character avatar: %w", err)
	}

	return avatarURL, nil
}

// DeleteCharacterAvatar clears a character's avatar so they render as initials
// from now on.
//
// This deliberately does NOT delete the file from storage. Messages pin the
// avatar URL they were authored with (messages.character_avatar_url_at_post), so
// removing the underlying file would retroactively blank the avatar on every post
// the character had already written — silently and irreversibly. A player
// clicking "Remove Avatar" is asking to look different going forward, not to
// rewrite their own history.
func (s *AvatarService) DeleteCharacterAvatar(ctx context.Context, characterID int32) error {
	queries := db.New(s.DB)

	if err := queries.DeleteCharacterAvatar(ctx, characterID); err != nil {
		return fmt.Errorf("failed to delete character avatar from database: %w", err)
	}

	return nil
}

// Helper functions

// readAndValidateSize reads the entire file into memory and validates size.
// Returns a new reader with the data and the size in bytes.
func readAndValidateSize(file io.Reader, maxSize int64) (io.Reader, int64, error) {
	// Use a LimitReader to prevent reading too much
	limitedReader := io.LimitReader(file, maxSize+1)

	// Read into memory
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read file: %w", err)
	}

	size := int64(len(data))

	// Check if file exceeds max size
	if size > maxSize {
		return nil, 0, fmt.Errorf("image too large. Maximum size is %d bytes (%.1fMB)", maxSize, float64(maxSize)/(1024*1024))
	}

	// Return a new reader with the data
	return strings.NewReader(string(data)), size, nil
}

// randomSuffix returns a short random hex string used to keep avatar storage
// paths unique, so a new upload can never overwrite a file that existing
// messages have pinned. Falls back to empty on the (practically impossible)
// read failure; the timestamp alone still separates uploads in that case.
func randomSuffix() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// mimeTypeToExtension converts a MIME type to a file extension.
func mimeTypeToExtension(mimeType string) string {
	switch mimeType {
	case MimeTypeJPEG:
		return ".jpg"
	case MimeTypePNG:
		return ".png"
	case MimeTypeWebP:
		return ".webp"
	default:
		return ""
	}
}
