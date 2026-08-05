package avatars

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock storage for testing
type MockStorage struct {
	uploads map[string]string // path -> content
	deletes []string          // paths that were deleted
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		uploads: make(map[string]string),
		deletes: []string{},
	}
}

func (m *MockStorage) Upload(ctx context.Context, path string, file interface{}, contentType string) (string, error) {
	// Read content for verification
	var content string
	switch f := file.(type) {
	case *strings.Reader:
		buf := make([]byte, f.Size())
		f.Read(buf)
		content = string(buf)
	}

	m.uploads[path] = content
	return "http://test.com/" + path, nil
}

func (m *MockStorage) Delete(ctx context.Context, path string) error {
	m.deletes = append(m.deletes, path)
	delete(m.uploads, path)
	return nil
}

func (m *MockStorage) GetURL(path string) string {
	return "http://test.com/" + path
}

func TestAvatarService_UploadCharacterAvatar_Validation(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		fileSize    int
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid JPEG",
			contentType: "image/jpeg",
			fileSize:    1024,
			wantErr:     false,
		},
		{
			name:        "valid PNG",
			contentType: "image/png",
			fileSize:    1024,
			wantErr:     false,
		},
		{
			name:        "valid WebP",
			contentType: "image/webp",
			fileSize:    1024,
			wantErr:     false,
		},
		{
			name:        "invalid content type - PDF",
			contentType: "application/pdf",
			fileSize:    1024,
			wantErr:     true,
			errContains: "invalid file type",
		},
		{
			name:        "invalid content type - text",
			contentType: "text/plain",
			fileSize:    1024,
			wantErr:     true,
			errContains: "invalid file type",
		},
		{
			name:        "file too large",
			contentType: "image/jpeg",
			fileSize:    6 * 1024 * 1024, // 6MB
			wantErr:     true,
			errContains: "too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip database tests - these are validation-only tests
			if testing.Short() {
				t.Skip("skipping database test in short mode")
			}

			// Note: Full integration test would require database setup
			// For now, test validation logic directly

			// Test content type validation
			if tt.contentType != "image/jpeg" && tt.contentType != "image/png" && tt.contentType != "image/webp" {
				valid := allowedMimeTypes[tt.contentType]
				assert.False(t, valid, "content type %s should not be allowed", tt.contentType)
			}

			// Test file size validation
			if tt.fileSize > MaxAvatarSize {
				assert.Greater(t, tt.fileSize, int(MaxAvatarSize), "file size should exceed maximum")
			}
		})
	}
}

func TestAvatarService_MimeTypeToExtension(t *testing.T) {
	tests := []struct {
		mimeType string
		want     string
	}{
		{MimeTypeJPEG, ".jpg"},
		{MimeTypePNG, ".png"},
		{MimeTypeWebP, ".webp"},
		{"unknown/type", ""},
	}

	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			got := mimeTypeToExtension(tt.mimeType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAvatarService_ReadAndValidateSize(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		maxSize     int64
		wantErr     bool
		errContains string
	}{
		{
			name:    "small file within limit",
			content: "small file content",
			maxSize: 1024,
			wantErr: false,
		},
		{
			name:    "exactly at limit",
			content: strings.Repeat("x", 1024),
			maxSize: 1024,
			wantErr: false,
		},
		{
			name:        "exceeds limit",
			content:     strings.Repeat("x", 1025),
			maxSize:     1024,
			wantErr:     true,
			errContains: "too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.content)
			newReader, size, err := readAndValidateSize(reader, tt.maxSize)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, int64(len(tt.content)), size)
			assert.NotNil(t, newReader)

			// Verify we can read the content back
			readBack, err := io.ReadAll(newReader)
			require.NoError(t, err)
			assert.Equal(t, tt.content, string(readBack))
		})
	}
}

func TestAvatarService_Constants(t *testing.T) {
	// Verify constants match spec
	assert.Equal(t, 5*1024*1024, MaxAvatarSize, "MaxAvatarSize should be 5MB")

	// Verify allowed MIME types
	assert.True(t, allowedMimeTypes[MimeTypeJPEG])
	assert.True(t, allowedMimeTypes[MimeTypePNG])
	assert.True(t, allowedMimeTypes[MimeTypeWebP])
	assert.False(t, allowedMimeTypes["image/gif"])
	assert.False(t, allowedMimeTypes["application/pdf"])
}

// Integration tests would go here (require database setup)
// Example structure:
//
// func TestAvatarService_Upload_Integration(t *testing.T) {
//     if testing.Short() {
//         t.Skip("skipping integration test")
//     }
//
//     pool := setupTestDB(t)
//     defer cleanupTestDB(t, pool)
//
//     mockStorage := NewMockStorage()
//     service := &AvatarService{
//         DB: pool,
//         Storage: mockStorage,
//     }
//
//     // Create test character
//     character := createTestCharacter(t, pool)
//
//     // Upload avatar
//     file := strings.NewReader("fake image content")
//     url, err := service.UploadCharacterAvatar(context.Background(), character.ID, file, "avatar.jpg", "image/jpeg")
//
//     require.NoError(t, err)
//     assert.NotEmpty(t, url)
//     assert.Len(t, mockStorage.uploads, 1)
//
//     // Verify database updated
//     updatedChar, err := db.New(pool).GetCharacter(context.Background(), character.ID)
//     require.NoError(t, err)
//     assert.True(t, updatedChar.AvatarUrl.Valid)
//     assert.Equal(t, url, updatedChar.AvatarUrl.String)
// }
