package core

import (
	"context"
	"errors"
	"testing"

	"github.com/go-chi/jwtauth/v5"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// TestGetUserIDFromJWT tests extracting user ID from JWT context
func TestGetUserIDFromJWT(t *testing.T) {
	// Create a mock user service (not needed for this function but required by signature)
	mockUserService := &MockUserService{}

	tests := []struct {
		name           string
		setupContext   func() context.Context
		expectUserID   int32
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "valid token with user ID",
			setupContext: func() context.Context {
				token := jwt.New()
				token.Set("sub", "12345")

				ctx := context.Background()
				ctx = context.WithValue(ctx, jwtauth.TokenCtxKey, token)
				ctx = context.WithValue(ctx, jwtauth.ErrorCtxKey, nil)

				return ctx
			},
			expectUserID: 12345,
			expectError:  false,
		},
		{
			name: "missing token in context",
			setupContext: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, jwtauth.ErrorCtxKey, errors.New("no token"))
				return ctx
			},
			expectUserID:   0,
			expectError:    true,
			expectedErrMsg: "no valid token found",
		},
		{
			name: "token without sub claim",
			setupContext: func() context.Context {
				token := jwt.New()
				// No "sub" claim set

				ctx := context.Background()
				ctx = context.WithValue(ctx, jwtauth.TokenCtxKey, token)
				ctx = context.WithValue(ctx, jwtauth.ErrorCtxKey, nil)

				return ctx
			},
			expectUserID:   0,
			expectError:    true,
			expectedErrMsg: "user id not found in token",
		},
		{
			name: "valid token with different user ID",
			setupContext: func() context.Context {
				token := jwt.New()
				token.Set("sub", "999")

				ctx := context.Background()
				ctx = context.WithValue(ctx, jwtauth.TokenCtxKey, token)
				ctx = context.WithValue(ctx, jwtauth.ErrorCtxKey, nil)

				return ctx
			},
			expectUserID: 999,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupContext()
			userID, errResp := GetUserIDFromJWT(ctx, mockUserService)

			if tt.expectError {
				if errResp == nil {
					t.Error("Expected error response, got nil")
					return
				}

				// Check error message
				errResponse := errResp.(*ErrResponse)
				if errResponse.ErrorText != tt.expectedErrMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.expectedErrMsg, errResponse.ErrorText)
				}

				if userID != 0 {
					t.Errorf("Expected userID 0 on error, got %d", userID)
				}
			} else {
				if errResp != nil {
					t.Errorf("Expected no error, got: %v", errResp)
					return
				}

				if userID != tt.expectUserID {
					t.Errorf("Expected userID %d, got %d", tt.expectUserID, userID)
				}
			}
		})
	}
}

func TestStripPort(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		expectedIP string
	}{
		{
			name:       "IPv4 with port",
			input:      "192.168.1.1:8080",
			expectedIP: "192.168.1.1",
		},
		{
			name:       "IPv4 without port",
			input:      "192.168.1.1",
			expectedIP: "192.168.1.1",
		},
		{
			name:       "IPv6 with port",
			input:      "[2001:db8::1]:8080",
			expectedIP: "[2001:db8::1]",
		},
		{
			name:       "IPv6 without port",
			input:      "[2001:db8::1]",
			expectedIP: "[2001:db8::1]",
		},
		{
			name:       "IPv6 with multiple colons",
			input:      "[::1]:8080",
			expectedIP: "[::1]",
		},
		{
			name:       "localhost with port",
			input:      "127.0.0.1:3000",
			expectedIP: "127.0.0.1",
		},
		{
			name:       "Empty string",
			input:      "",
			expectedIP: "",
		},
		{
			name:       "IPv4 with multiple colons (malformed)",
			input:      "192.168.1.1:8080:9090",
			expectedIP: "192.168.1.1:8080", // LastIndex behavior
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripPort(tt.input)
			if result != tt.expectedIP {
				t.Errorf("stripPort(%s) = %s, want %s", tt.input, result, tt.expectedIP)
			}
		})
	}
}

// Using existing MockUserService from mocks.go
// No need to redefine it here
