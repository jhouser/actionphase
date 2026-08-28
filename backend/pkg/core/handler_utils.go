package core

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
)

// GetUserIDFromJWT extracts the user ID from the JWT token in the context.
// Returns the user ID and nil on success, or 0 and an error response on failure.
//
// Example Usage:
//
//	userID, errResp := GetUserIDFromJWT(ctx, app.UserService)
//	if errResp != nil {
//	    render.Render(w, r, errResp)
//	    return
//	}
func GetUserIDFromJWT(ctx context.Context, userService UserServiceInterface) (int32, render.Renderer) {
	token, _, err := jwtauth.FromContext(ctx)
	if err != nil {
		return 0, ErrUnauthorized("no valid token found")
	}

	// Extract user_id from "sub" claim (immutable ID)
	userIDStr, ok := token.Get("sub")
	if !ok {
		return 0, ErrUnauthorized("user id not found in token")
	}

	// Parse user ID string to int
	var userID int
	fmt.Sscanf(userIDStr.(string), "%d", &userID)

	return int32(userID), nil
}

// GetClientIP extracts the real client IP address from an HTTP request.
// It checks headers set by proxies/load balancers in this order:
// 1. X-Real-IP - Set by nginx and other reverse proxies
// 2. X-Forwarded-For - Standard proxy header (uses leftmost/client IP)
// 3. RemoteAddr - Direct connection IP (fallback)
//
// The returned IP has any port suffix stripped.
//
// This matches the IP lookup behavior used in rate limiting middleware
// to ensure consistent IP detection across the application.
//
// Example Usage:
//
//	clientIP := GetClientIP(r)
//	log.Info("Request from IP", "ip", clientIP)
func GetClientIP(r *http.Request) string {
	// Try X-Real-IP first (set by nginx, etc.)
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return stripPort(ip)
	}

	// Try X-Forwarded-For (standard proxy header)
	// Format: "client, proxy1, proxy2"
	// We want the leftmost (original client) IP
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Split by comma and take first IP
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			clientIP := strings.TrimSpace(ips[0])
			return stripPort(clientIP)
		}
	}

	// Fallback to RemoteAddr (direct connection)
	return stripPort(r.RemoteAddr)
}

// stripPort removes the port suffix from an IP address string if present.
// Examples:
//   - "192.168.1.1:8080" -> "192.168.1.1"
//   - "[::1]:8080" -> "[::1]"
//   - "192.168.1.1" -> "192.168.1.1"
func stripPort(addr string) string {
	// Handle IPv6 addresses in brackets
	if strings.HasPrefix(addr, "[") {
		if idx := strings.Index(addr, "]:"); idx != -1 {
			return addr[:idx+1]
		}
		return addr
	}

	// Handle IPv4 addresses
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}

	return addr
}
