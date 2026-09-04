package core

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"actionphase/pkg/observability"

	"github.com/go-chi/render"
)

// TestErrResponse_Render tests the Render method sets the correct HTTP status
func TestErrResponse_Render(t *testing.T) {
	tests := []struct {
		name               string
		response           *ErrResponse
		expectedStatusCode int
	}{
		{
			name: "400 Bad Request",
			response: &ErrResponse{
				HTTPStatusCode: 400,
				Title:          "Bad Request",
				Detail:         "Invalid input",
			},
			expectedStatusCode: 400,
		},
		{
			name: "401 Unauthorized",
			response: &ErrResponse{
				HTTPStatusCode: 401,
				Title:          "Unauthorized",
			},
			expectedStatusCode: 401,
		},
		{
			name: "500 Internal Server Error",
			response: &ErrResponse{
				HTTPStatusCode: 500,
				Title:          "Internal Server Error",
			},
			expectedStatusCode: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			rec := httptest.NewRecorder()

			// Render should set the status code
			err := tt.response.Render(rec, req)
			if err != nil {
				t.Errorf("Render() error = %v", err)
			}

			// render.Status was called, verify the recorder has the status
			// Note: We need to actually call render.Render to get the status set
			render.Render(rec, req, tt.response)

			if rec.Code != tt.expectedStatusCode {
				t.Errorf("Expected status %d, got %d", tt.expectedStatusCode, rec.Code)
			}
		})
	}
}

// TestErrResponse_JSONSerialization verifies that Err field is never exposed
func TestErrResponse_JSONSerialization(t *testing.T) {
	internalErr := errors.New("database connection failed")
	response := &ErrResponse{
		Err:            internalErr,
		HTTPStatusCode: 500,
		Title:          "Internal Server Error",
		Detail:         "Something went wrong",
		Status:         500,
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	// Parse JSON to verify structure
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify Err field is NOT in JSON
	if _, exists := parsed["Err"]; exists {
		t.Error("Internal Err field should not be serialized")
	}

	// Verify HTTPStatusCode is NOT in JSON
	if _, exists := parsed["HTTPStatusCode"]; exists {
		t.Error("HTTPStatusCode field should not be serialized")
	}

	// Verify expected fields ARE in JSON
	if parsed["title"] != "Internal Server Error" {
		t.Errorf("status field incorrect: %v", parsed["title"])
	}
	if parsed["detail"] != "Something went wrong" {
		t.Errorf("error field incorrect: %v", parsed["detail"])
	}
	// RFC 7807 mirrors the HTTP status inside the body as a number.
	if parsed["status"] != float64(500) {
		t.Errorf("status field incorrect: %v", parsed["status"])
	}
}

// TestErrInvalidRequest tests the 400 Bad Request constructor
func TestErrInvalidRequest(t *testing.T) {
	err := errors.New("missing required field: email")
	result := ErrInvalidRequest(err).(*ErrResponse)

	if result.HTTPStatusCode != 400 {
		t.Errorf("Expected status 400, got %d", result.HTTPStatusCode)
	}
	if result.Title != "Bad Request" {
		t.Errorf("Expected 'Bad Request', got '%s'", result.Title)
	}
	if result.Detail != "missing required field: email" {
		t.Errorf("Expected error text to match, got '%s'", result.Detail)
	}
	if result.Err != err {
		t.Error("Expected internal error to be preserved")
	}
}

// TestErrInternalError tests the 500 Internal Server Error constructor
func TestErrInternalError(t *testing.T) {
	err := errors.New("database query failed")
	result := ErrInternalError(err).(*ErrResponse)

	if result.HTTPStatusCode != 500 {
		t.Errorf("Expected status 500, got %d", result.HTTPStatusCode)
	}
	if result.Title != "Internal Server Error" {
		t.Errorf("Expected 'Internal Server Error', got '%s'", result.Title)
	}
	if result.Detail != "An unexpected error occurred. Please try again later." {
		t.Errorf("Expected generic error text, got '%s'", result.Detail)
	}
	if result.Err == nil || result.Err.Error() != "database query failed" {
		t.Errorf("Expected internal error to be preserved on Err field, got '%v'", result.Err)
	}
}

// TestErrUnauthorized tests the 401 Unauthorized constructor
func TestErrUnauthorized(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"invalid token", "Invalid or expired token"},
		{"missing credentials", "Missing authentication credentials"},
		{"invalid password", "Invalid username or password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ErrUnauthorized(tt.message).(*ErrResponse)

			if result.HTTPStatusCode != 401 {
				t.Errorf("Expected status 401, got %d", result.HTTPStatusCode)
			}
			if result.Title != "Unauthorized" {
				t.Errorf("Expected 'Unauthorized', got '%s'", result.Title)
			}
			if result.Detail != tt.message {
				t.Errorf("Expected message '%s', got '%s'", tt.message, result.Detail)
			}
		})
	}
}

// TestErrForbidden tests the 403 Forbidden constructor
func TestErrForbidden(t *testing.T) {
	message := "Admin access required"
	result := ErrForbidden(message).(*ErrResponse)

	if result.HTTPStatusCode != 403 {
		t.Errorf("Expected status 403, got %d", result.HTTPStatusCode)
	}
	if result.Title != "Forbidden" {
		t.Errorf("Expected 'Forbidden', got '%s'", result.Title)
	}
	if result.Detail != message {
		t.Errorf("Expected message '%s', got '%s'", message, result.Detail)
	}
}

// TestErrBadRequest tests the 400 Bad Request constructor
func TestErrBadRequest(t *testing.T) {
	err := errors.New("Cannot join completed game")
	result := ErrBadRequest(err).(*ErrResponse)

	if result.HTTPStatusCode != 400 {
		t.Errorf("Expected status 400, got %d", result.HTTPStatusCode)
	}
	if result.Title != "Bad Request" {
		t.Errorf("Expected 'Bad Request', got '%s'", result.Title)
	}
	if result.Detail != "Cannot join completed game" {
		t.Errorf("Expected error text to match, got '%s'", result.Detail)
	}
}

// TestErrNotFound tests the 404 Not Found constructor
func TestErrNotFound(t *testing.T) {
	message := "Game not found"
	result := ErrNotFound(message).(*ErrResponse)

	if result.HTTPStatusCode != 404 {
		t.Errorf("Expected status 404, got %d", result.HTTPStatusCode)
	}
	if result.Title != "Not Found" {
		t.Errorf("Expected 'Not Found', got '%s'", result.Title)
	}
	if result.Detail != message {
		t.Errorf("Expected message '%s', got '%s'", message, result.Detail)
	}
}

// TestErrConflict tests the 409 Conflict constructor
func TestErrConflict(t *testing.T) {
	message := "Username already exists"
	result := ErrConflict(message).(*ErrResponse)

	if result.HTTPStatusCode != 409 {
		t.Errorf("Expected status 409, got %d", result.HTTPStatusCode)
	}
	if result.Title != "Conflict" {
		t.Errorf("Expected 'Conflict', got '%s'", result.Title)
	}
	if result.Detail != message {
		t.Errorf("Expected message '%s', got '%s'", message, result.Detail)
	}
}

// TestErrWithStatus tests error responses for statuses without a dedicated
// constructor.
func TestErrWithStatus(t *testing.T) {
	tests := []struct {
		name          string
		httpStatus    int
		message       string
		expectedTitle string
	}{
		{
			name:          "game not recruiting",
			httpStatus:    400,
			message:       "Game is not accepting new players",
			expectedTitle: "Bad Request",
		},
		{
			name:          "unauthorized",
			httpStatus:    401,
			message:       "Token is invalid",
			expectedTitle: "Unauthorized",
		},
		{
			name:          "unregistered status code",
			httpStatus:    599,
			message:       "Test error",
			expectedTitle: "Unknown Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ErrWithStatus(tt.httpStatus, tt.message).(*ErrResponse)

			if result.HTTPStatusCode != tt.httpStatus {
				t.Errorf("Expected status %d, got %d", tt.httpStatus, result.HTTPStatusCode)
			}
			if result.Title != tt.expectedTitle {
				t.Errorf("Expected title '%s', got '%s'", tt.expectedTitle, result.Title)
			}
			if result.Detail != tt.message {
				t.Errorf("Expected message '%s', got '%s'", tt.message, result.Detail)
			}
		})
	}
}

// TestGetTitle tests the status text mapping helper
func TestGetTitle(t *testing.T) {
	tests := []struct {
		httpStatus   int
		expectedText string
	}{
		{400, "Bad Request"},
		{401, "Unauthorized"},
		{403, "Forbidden"},
		{404, "Not Found"},
		{409, "Conflict"},
		{422, "Unprocessable Entity"},
		{500, "Internal Server Error"},
		{418, "I'm a teapot"},  // registered, so net/http knows it
		{999, "Unknown Error"}, // unregistered
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.httpStatus)), func(t *testing.T) {
			result := getTitle(tt.httpStatus)
			if result != tt.expectedText {
				t.Errorf("Expected '%s', got '%s'", tt.expectedText, result)
			}
		})
	}
}

// TestErrGameArchived tests the game archived error
func TestErrGameArchived(t *testing.T) {
	result := ErrGameArchived().(*ErrResponse)

	if result.HTTPStatusCode != 403 {
		t.Errorf("Expected status 403, got %d", result.HTTPStatusCode)
	}
	if result.Detail != "This game is archived and read-only. No new content can be created." {
		t.Errorf("Unexpected error text: %s", result.Detail)
	}
}

// TestIsArchivedGameError tests the archived game error detection
func TestIsArchivedGameError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "archived game error",
			err:  errors.New("game is archived"),
			want: true,
		},
		{
			name: "archived in message",
			err:  errors.New("cannot modify archived game"),
			want: true,
		},
		{
			name: "uppercase archived",
			err:  errors.New("Game is ARCHIVED"),
			want: false, // Case-sensitive check
		},
		{
			name: "different error",
			err:  errors.New("game not found"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsArchivedGameError(tt.err)
			if got != tt.want {
				t.Errorf("IsArchivedGameError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestErrorResponseIntegration tests full error response rendering
func TestErrorResponseIntegration(t *testing.T) {
	tests := []struct {
		name         string
		renderer     render.Renderer
		expectedCode int
		checkJSON    func(*testing.T, map[string]interface{})
	}{
		{
			name:         "invalid request renders correctly",
			renderer:     ErrInvalidRequest(errors.New("bad input")),
			expectedCode: 400,
			checkJSON: func(t *testing.T, data map[string]interface{}) {
				if data["title"] != "Bad Request" {
					t.Errorf("Unexpected status: %v", data["title"])
				}
				if data["detail"] != "bad input" {
					t.Errorf("Unexpected error: %v", data["detail"])
				}
			},
		},
		{
			name:         "unauthorized renders correctly",
			renderer:     ErrUnauthorized("invalid token"),
			expectedCode: 401,
			checkJSON: func(t *testing.T, data map[string]interface{}) {
				if data["title"] != "Unauthorized" {
					t.Errorf("Unexpected status: %v", data["title"])
				}
			},
		},
		{
			name:         "game archived renders RFC 7807 fields",
			renderer:     ErrGameArchived(),
			expectedCode: 403,
			checkJSON: func(t *testing.T, data map[string]interface{}) {
				if data["title"] != "Forbidden" {
					t.Errorf("Unexpected title: %v", data["title"])
				}
				// RFC 7807 mirrors the status in the body as a number. The
				// frontend must never render this as a message, which is what
				// the old bespoke format's string "status" invited.
				if data["status"] != float64(403) {
					t.Errorf("Expected status 403, got %v", data["status"])
				}
				// The dropped "code" field must not reappear.
				if _, exists := data["code"]; exists {
					t.Errorf("code should no longer be serialized: %v", data["code"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			rec := httptest.NewRecorder()

			// Render the error
			render.Render(rec, req, tt.renderer)

			// Check status code
			if rec.Code != tt.expectedCode {
				t.Errorf("Expected status %d, got %d", tt.expectedCode, rec.Code)
			}

			// Parse JSON response
			var data map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			// Run custom JSON checks
			if tt.checkJSON != nil {
				tt.checkJSON(t, data)
			}
		})
	}
}

// TestErrResponse_ProblemJSONContentType pins the RFC 7807 media type.
//
// The router installs render.SetContentType(ContentTypeJSON) globally, so
// without the explicit header in Render an error body would go out as plain
// application/json and a client content-negotiating on problem+json would not
// recognize it.
func TestErrResponse_ProblemJSONContentType(t *testing.T) {
	// Restore the responder afterwards: it is package-level state, and leaving
	// a wrapper installed would leak into every later test in this package.
	original := render.Respond
	t.Cleanup(func() { render.Respond = original })
	InstallProblemJSONResponder()

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	render.Render(rec, req, ErrForbidden("nope"))

	got := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(got, "application/problem+json") {
		t.Errorf("Expected application/problem+json, got %q", got)
	}
}

// TestErrResponse_InstanceCarriesCorrelationID verifies the support-ticket path:
// an error body alone must be enough to find the request in the logs.
func TestErrResponse_InstanceCarriesCorrelationID(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req = req.WithContext(observability.WithCorrelationID(req.Context(), "corr-abc123"))
	rec := httptest.NewRecorder()

	render.Render(rec, req, ErrForbidden("nope"))

	var data map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	if data["instance"] != "urn:actionphase:correlation:corr-abc123" {
		t.Errorf("Unexpected instance: %v", data["instance"])
	}
}

// TestErrResponse_InstanceOmittedWithoutCorrelationID guards against emitting a
// meaningless bare "urn:actionphase:correlation:" prefix when no ID is present.
func TestErrResponse_InstanceOmittedWithoutCorrelationID(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	render.Render(rec, req, ErrForbidden("nope"))

	var data map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	if _, exists := data["instance"]; exists {
		t.Errorf("instance should be omitted when no correlation ID: %v", data["instance"])
	}
}
