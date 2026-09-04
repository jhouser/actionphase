package core

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestHandleDBErrorWithID tests database error conversion with resource ID
func TestHandleDBErrorWithID(t *testing.T) {
	tests := []struct {
		name               string
		err                error
		resourceName       string
		id                 interface{}
		expectedStatusCode int
		expectedTitle      string
		expectedDetail     string
		expectNil          bool
	}{
		{
			name:         "nil error returns nil",
			err:          nil,
			resourceName: "game",
			id:           123,
			expectNil:    true,
		},
		{
			name:               "sql.ErrNoRows includes ID in message",
			err:                sql.ErrNoRows,
			resourceName:       "game",
			id:                 456,
			expectedStatusCode: 404,
			expectedTitle:      "Not Found",
			expectedDetail:     "game with ID 456 not found",
			expectNil:          false,
		},
		{
			name:               "pgx.ErrNoRows includes ID in message",
			err:                pgx.ErrNoRows,
			resourceName:       "user",
			id:                 789,
			expectedStatusCode: 404,
			expectedTitle:      "Not Found",
			expectedDetail:     "user with ID 789 not found",
			expectNil:          false,
		},
		{
			name:               "string ID works correctly",
			err:                sql.ErrNoRows,
			resourceName:       "session",
			id:                 "abc-123-def",
			expectedStatusCode: 404,
			expectedTitle:      "Not Found",
			expectedDetail:     "session with ID abc-123-def not found",
			expectNil:          false,
		},
		{
			name:               "int32 ID works correctly",
			err:                pgx.ErrNoRows,
			resourceName:       "character",
			id:                 int32(999),
			expectedStatusCode: 404,
			expectedTitle:      "Not Found",
			expectedDetail:     "character with ID 999 not found",
			expectNil:          false,
		},
		{
			name:               "wrapped sql.ErrNoRows includes ID",
			err:                errors.Join(sql.ErrNoRows, errors.New("context")),
			resourceName:       "poll",
			id:                 111,
			expectedStatusCode: 404,
			expectedTitle:      "Not Found",
			expectedDetail:     "poll with ID 111 not found",
			expectNil:          false,
		},
		{
			name:               "other database error returns 500 with ID",
			err:                errors.New("constraint violation"),
			resourceName:       "game",
			id:                 222,
			expectedStatusCode: 500,
			expectedTitle:      "Internal Server Error",
			expectedDetail:     "An unexpected error occurred. Please try again later.",
			expectNil:          false,
		},
		{
			name:               "generic error with zero ID",
			err:                errors.New("some error"),
			resourceName:       "item",
			id:                 0,
			expectedStatusCode: 500,
			expectedTitle:      "Internal Server Error",
			expectedDetail:     "An unexpected error occurred. Please try again later.",
			expectNil:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HandleDBErrorWithID(tt.err, tt.resourceName, tt.id)

			if tt.expectNil {
				if result != nil {
					t.Errorf("Expected nil result, got: %v", result)
				}
				return
			}

			if result == nil {
				t.Error("Expected error response, got nil")
				return
			}

			errResponse := result.(*ErrResponse)

			if errResponse.HTTPStatusCode != tt.expectedStatusCode {
				t.Errorf("Expected status %d, got %d", tt.expectedStatusCode, errResponse.HTTPStatusCode)
			}

			if errResponse.Title != tt.expectedTitle {
				t.Errorf("Expected title '%s', got '%s'", tt.expectedTitle, errResponse.Title)
			}

			if errResponse.Detail != tt.expectedDetail {
				t.Errorf("Expected detail '%s', got '%s'", tt.expectedDetail, errResponse.Detail)
			}

			// Verify internal error is preserved for 500 errors
			if tt.expectedStatusCode == 500 && errResponse.Err == nil {
				t.Error("Expected internal error to be preserved for 500 errors")
			}
		})
	}
}

// TestHandleDBErrorWithID_VariousIDTypes tests different ID types
func TestHandleDBErrorWithID_VariousIDTypes(t *testing.T) {
	tests := []struct {
		name         string
		id           interface{}
		expectedText string
	}{
		{"int ID", 123, "resource with ID 123 not found"},
		{"int32 ID", int32(456), "resource with ID 456 not found"},
		{"int64 ID", int64(789), "resource with ID 789 not found"},
		{"string ID", "abc-123", "resource with ID abc-123 not found"},
		{"uint ID", uint(999), "resource with ID 999 not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HandleDBErrorWithID(sql.ErrNoRows, "resource", tt.id)
			errResponse := result.(*ErrResponse)

			if errResponse.Detail != tt.expectedText {
				t.Errorf("Expected '%s', got '%s'", tt.expectedText, errResponse.Detail)
			}
		})
	}
}
