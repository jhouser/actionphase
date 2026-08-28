package games

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

const maxBannerSize = 5 * 1024 * 1024 // 5MB

var allowedBannerMimeTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

func readAndValidateBannerSize(file io.Reader) (io.Reader, error) {
	limited := io.LimitReader(file, maxBannerSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	if int64(len(data)) > maxBannerSize {
		return nil, fmt.Errorf("image too large. Maximum size is 5MB")
	}
	return strings.NewReader(string(data)), nil
}

func extractBannerPathFromURL(url string) string {
	if index := strings.LastIndex(url, "banners/"); index != -1 {
		return url[index:]
	}
	if index := strings.LastIndex(url, "avatars/"); index != -1 {
		return url[index:]
	}
	if lastSlash := strings.LastIndex(url, "/"); lastSlash != -1 {
		return url[lastSlash+1:]
	}
	return url
}

func bannerMimeTypeFromFilename(filename string) string {
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

func bannerExtFromMime(mimeType string) string {
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
