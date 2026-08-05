package users

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"actionphase/pkg/core"
	db "actionphase/pkg/db/models"
	"actionphase/pkg/observability"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
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

// UserAvatarService implements avatar management for users.
// Handles file upload validation, storage, and database updates.
type UserAvatarService struct {
	DB      *pgxpool.Pool
	Storage core.StorageBackendInterface
}

// Compile-time verification that UserAvatarService implements UserAvatarServiceInterface
var _ core.UserAvatarServiceInterface = (*UserAvatarService)(nil)

// UploadUserAvatar uploads an avatar image for a user.
//
// Validation:
//   - File type must be image/jpeg, image/png, or image/webp
//   - File size must be ≤5MB
//
// Process:
//  1. Validate content type
//  2. Read file into memory (to check size)
//  3. Delete old avatar if exists
//  4. Upload new avatar to storage
//  5. Update database with new avatar URL
//
// Note: unlike character avatars, user avatars are not pinned onto messages, so
// superseded files are deleted rather than retained.
//
// Returns the public URL of the uploaded avatar.
func (s *UserAvatarService) UploadUserAvatar(
	ctx context.Context,
	userID int32,
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

	// Get current user to check for existing avatar
	queries := db.New(s.DB)
	user, err := queries.GetUserProfile(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user: %w", err)
	}

	// Delete old avatar if exists
	if user.AvatarUrl.Valid && user.AvatarUrl.String != "" {
		oldPath := extractPathFromURL(user.AvatarUrl.String)
		if err := s.Storage.Delete(ctx, oldPath); err != nil {
			// Old avatar might not exist - log warning but continue with upload
			correlationID := observability.GetCorrelationID(ctx)
			log.Warn().
				Err(err).
				Str("correlation_id", correlationID).
				Int32("user_id", userID).
				Str("old_path", oldPath).
				Msg("Failed to delete old avatar (file may not exist)")
		}
	}

	// Generate storage path: avatars/users/{userID}/{timestamp}_{filename}
	ext := filepath.Ext(filename)
	if ext == "" {
		// Derive extension from content type
		ext = mimeTypeToExtension(contentType)
	}
	timestamp := time.Now().Unix()
	storagePath := fmt.Sprintf("avatars/users/%d/%d%s", userID, timestamp, ext)

	// Upload to storage
	avatarURL, err := s.Storage.Upload(ctx, storagePath, fileData, contentType)
	if err != nil {
		return "", fmt.Errorf("failed to upload avatar: %w", err)
	}

	// Update database
	err = queries.UpdateUserAvatar(ctx, db.UpdateUserAvatarParams{
		ID:        userID,
		AvatarUrl: pgtype.Text{String: avatarURL, Valid: true},
	})
	if err != nil {
		// Try to clean up uploaded file
		_ = s.Storage.Delete(ctx, storagePath)
		return "", fmt.Errorf("failed to update user avatar: %w", err)
	}

	return avatarURL, nil
}

// DeleteUserAvatar removes a user's avatar.
// Deletes the file from storage and updates the database.
func (s *UserAvatarService) DeleteUserAvatar(ctx context.Context, userID int32) error {
	queries := db.New(s.DB)

	// Get current user
	user, err := queries.GetUserProfile(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// If no avatar, nothing to delete
	if !user.AvatarUrl.Valid || user.AvatarUrl.String == "" {
		return nil
	}

	// Delete from storage
	oldPath := extractPathFromURL(user.AvatarUrl.String)
	if err := s.Storage.Delete(ctx, oldPath); err != nil {
		// File might already be gone - log warning but continue
		correlationID := observability.GetCorrelationID(ctx)
		log.Warn().
			Err(err).
			Str("correlation_id", correlationID).
			Int32("user_id", userID).
			Str("old_path", oldPath).
			Msg("Failed to delete avatar file (file may not exist)")
	}

	// Update database
	if err := queries.DeleteUserAvatar(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete user avatar from database: %w", err)
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

// extractPathFromURL extracts the storage path from a public URL.
// Example: "http://localhost:3000/uploads/avatars/users/1/avatar.jpg" -> "avatars/users/1/avatar.jpg"
func extractPathFromURL(url string) string {
	// Simple extraction: find the last occurrence of "avatars/" and take everything after
	// This works for both local and S3 URLs
	index := strings.LastIndex(url, "avatars/")
	if index == -1 {
		// Fallback: return everything after last slash
		lastSlash := strings.LastIndex(url, "/")
		if lastSlash != -1 {
			return url[lastSlash+1:]
		}
		return url
	}
	return url[index:]
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
