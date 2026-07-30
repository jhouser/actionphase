package auth

import (
	"actionphase/pkg/core"
	"actionphase/pkg/email"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	db "actionphase/pkg/db/models"
	dbsvc "actionphase/pkg/db/services"
	"github.com/go-chi/jwtauth/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a test database connection.
//
// Delegates to core.NewTestDatabase so this honors TEST_DATABASE_URL (works
// inside the containerized dev stack where the DB host is `db`, not localhost)
// and respects SKIP_DB_TESTS=true — these are DB tests and must be skipped in
// mock mode, not attempt (and fail) a connection.
func setupTestDB(t *testing.T) *pgxpool.Pool {
	ctx := context.Background()

	testDB := core.NewTestDatabase(t) // t.Skip()s the test when SKIP_DB_TESTS=true
	pool := testDB.Pool

	// Clean up tables for test isolation (in correct order due to foreign keys)
	tables := []string{
		"sessions",
		"email_verification_tokens",
		"password_reset_tokens",
		"registration_attempts",
		"users",
	}

	for _, table := range tables {
		if _, err := pool.Exec(ctx, fmt.Sprintf("DELETE FROM %s", table)); err != nil {
			// Some tables might not exist in all test scenarios, log but continue
			t.Logf("Warning: Failed to clean up %s table: %v", table, err)
		}
	}

	return pool
}

// newTestHandler creates a fully-wired Handler for use in tests.
func newTestHandler(pool *pgxpool.Pool) Handler {
	app := core.NewTestApp(pool)
	return Handler{
		App:                    app,
		UserService:            &dbsvc.UserService{DB: pool, Logger: app.ObsLogger},
		SessionService:         &dbsvc.SessionService{DB: pool, Logger: app.ObsLogger},
		UserPreferencesService: dbsvc.NewUserPreferencesService(pool),
		IPBanService:           &dbsvc.IPBanService{DB: pool, Logger: app.ObsLogger},
		FingerprintBanService:  &dbsvc.FingerprintBanService{DB: pool, Logger: app.ObsLogger},
		DiscordService:         &dbsvc.DiscordAccountService{DB: pool, Logger: app.ObsLogger},
	}
}

// createTestUser creates a test user and returns the user ID
func createTestUser(t *testing.T, pool *pgxpool.Pool, email, username, password string) int32 {
	ctx := context.Background()
	queries := db.New(pool)

	hashedPassword, err := HashPassword(password)
	require.NoError(t, err)

	user, err := queries.CreateUser(ctx, db.CreateUserParams{
		Email:    email,
		Username: username,
		Password: hashedPassword,
	})
	require.NoError(t, err)

	return user.ID
}

// generateTestJWT generates a JWT token for testing
func generateTestJWT(t *testing.T, userID int) string {
	tokenAuth := jwtauth.New("HS256", []byte("test-secret"), nil)

	// Get username from database for the given userID
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "postgres://postgres:example@localhost:5432/actionphase_test")
	require.NoError(t, err)
	defer pool.Close()

	queries := db.New(pool)
	user, err := queries.GetUser(ctx, int32(userID))
	require.NoError(t, err)

	claims := map[string]interface{}{
		"user_id":  float64(userID),
		"username": user.Username,
		"exp":      time.Now().Add(time.Hour).Unix(),
	}

	_, tokenString, err := tokenAuth.Encode(claims)
	require.NoError(t, err)

	return tokenString
}

func TestV1ChangePassword(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := setupTestDB(t)
	defer pool.Close()

	handler := newTestHandler(pool)

	tests := []struct {
		name           string
		setupUser      func() int32
		requestBody    ChangePasswordRequest
		setupToken     func(userID int32) string
		expectedStatus int
		expectedError  string
	}{
		{
			name: "successful password change",
			setupUser: func() int32 {
				return createTestUser(t, pool, "change1@test.com", "changeuser1", "OldPass123!")
			},
			requestBody: ChangePasswordRequest{
				CurrentPassword: "OldPass123!",
				NewPassword:     "NewPass456!",
				ConfirmPassword: "NewPass456!",
			},
			setupToken: func(userID int32) string {
				return generateTestJWT(t, int(userID))
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "incorrect current password",
			setupUser: func() int32 {
				return createTestUser(t, pool, "change2@test.com", "changeuser2", "OldPass123!")
			},
			requestBody: ChangePasswordRequest{
				CurrentPassword: "WrongPass123!",
				NewPassword:     "NewPass456!",
				ConfirmPassword: "NewPass456!",
			},
			setupToken: func(userID int32) string {
				return generateTestJWT(t, int(userID))
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "current password is incorrect",
		},
		{
			name: "passwords don't match",
			setupUser: func() int32 {
				return createTestUser(t, pool, "change3@test.com", "changeuser3", "OldPass123!")
			},
			requestBody: ChangePasswordRequest{
				CurrentPassword: "OldPass123!",
				NewPassword:     "NewPass456!",
				ConfirmPassword: "DifferentPass456!",
			},
			setupToken: func(userID int32) string {
				return generateTestJWT(t, int(userID))
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "passwords do not match",
		},
		{
			name: "weak new password",
			setupUser: func() int32 {
				return createTestUser(t, pool, "change4@test.com", "changeuser4", "OldPass123!")
			},
			requestBody: ChangePasswordRequest{
				CurrentPassword: "OldPass123!",
				NewPassword:     "weak",
				ConfirmPassword: "weak",
			},
			setupToken: func(userID int32) string {
				return generateTestJWT(t, int(userID))
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "must be at least 8 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := tt.setupUser()

			bodyBytes, err := json.Marshal(tt.requestBody)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/auth/change-password", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			// Add authenticated user to context (simulates RequireAuthenticationMiddleware)
			req = addAuthContextToRequest(t, req, pool, userID)

			w := httptest.NewRecorder()
			handler.V1ChangePassword(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response["error"], tt.expectedError)
			}

			// Cleanup
			queries := db.New(pool)
			_ = queries.DeleteUser(context.Background(), userID)
		})
	}
}

func TestV1RequestPasswordReset(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := setupTestDB(t)
	defer pool.Close()

	handler := newTestHandler(pool)

	tests := []struct {
		name           string
		setupUser      func() int32
		email          string
		expectedStatus int
	}{
		{
			name: "successful password reset request",
			setupUser: func() int32 {
				return createTestUser(t, pool, "reset1@test.com", "resetuser1", "TestPass123!")
			},
			email:          "reset1@test.com",
			expectedStatus: http.StatusOK,
		},
		{
			name: "non-existent email (still returns success for security)",
			setupUser: func() int32 {
				return 0 // No user created
			},
			email:          "nonexistent@test.com",
			expectedStatus: http.StatusOK,
		},
		{
			name: "invalid email format",
			setupUser: func() int32 {
				return 0
			},
			email:          "not-an-email",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "case-insensitive email match sends reset token",
			setupUser: func() int32 {
				return createTestUser(t, pool, "reset_case@test.com", "resetcase", "TestPass123!")
			},
			email:          "RESET_CASE@TEST.COM",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := tt.setupUser()

			requestBody := RequestPasswordResetRequest{
				Email: tt.email,
			}

			bodyBytes, err := json.Marshal(requestBody)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/auth/request-password-reset", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			handler.V1RequestPasswordReset(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK && userID > 0 {
				// Verify token was created in database
				queries := db.New(pool)
				ctx := context.Background()

				// Query for reset tokens for this user
				var count int
				err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM password_reset_tokens WHERE user_id = $1", userID).Scan(&count)
				require.NoError(t, err)
				assert.Greater(t, count, 0, "Password reset token should be created")

				// Cleanup
				_ = queries.DeleteUser(ctx, userID)
			}
		})
	}
}

func TestV1ResetPassword(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := setupTestDB(t)
	defer pool.Close()

	handler := newTestHandler(pool)
	queries := db.New(pool)
	ctx := context.Background()

	tests := []struct {
		name            string
		setupUser       func() int32
		setupToken      func(userID int32) string
		newPassword     string
		confirmPassword string
		expectedStatus  int
		expectedError   string
	}{
		{
			name: "successful password reset",
			setupUser: func() int32 {
				return createTestUser(t, pool, "resetpw1@test.com", "resetpwuser1", "OldPass123!")
			},
			setupToken: func(userID int32) string {
				token, _ := GenerateSecureToken(64)
				expiresAt := time.Now().Add(1 * time.Hour)
				resetToken, _ := queries.CreatePasswordResetToken(ctx, db.CreatePasswordResetTokenParams{
					UserID:    userID,
					Token:     token,
					ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
				})
				return resetToken.Token
			},
			newPassword:     "NewPass456!",
			confirmPassword: "NewPass456!",
			expectedStatus:  http.StatusOK,
		},
		{
			name: "invalid token",
			setupUser: func() int32 {
				return createTestUser(t, pool, "resetpw2@test.com", "resetpwuser2", "OldPass123!")
			},
			setupToken: func(userID int32) string {
				return "invalid-token-12345"
			},
			newPassword:     "NewPass456!",
			confirmPassword: "NewPass456!",
			expectedStatus:  http.StatusBadRequest,
			expectedError:   "invalid or expired reset token",
		},
		{
			name: "expired token",
			setupUser: func() int32 {
				return createTestUser(t, pool, "resetpw3@test.com", "resetpwuser3", "OldPass123!")
			},
			setupToken: func(userID int32) string {
				token, _ := GenerateSecureToken(64)
				expiresAt := time.Now().Add(-1 * time.Hour) // Expired
				resetToken, _ := queries.CreatePasswordResetToken(ctx, db.CreatePasswordResetTokenParams{
					UserID:    userID,
					Token:     token,
					ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
				})
				return resetToken.Token
			},
			newPassword:     "NewPass456!",
			confirmPassword: "NewPass456!",
			expectedStatus:  http.StatusBadRequest,
			expectedError:   "invalid or expired reset token",
		},
		{
			name: "passwords don't match",
			setupUser: func() int32 {
				return createTestUser(t, pool, "resetpw4@test.com", "resetpwuser4", "OldPass123!")
			},
			setupToken: func(userID int32) string {
				token, _ := GenerateSecureToken(64)
				expiresAt := time.Now().Add(1 * time.Hour)
				resetToken, _ := queries.CreatePasswordResetToken(ctx, db.CreatePasswordResetTokenParams{
					UserID:    userID,
					Token:     token,
					ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
				})
				return resetToken.Token
			},
			newPassword:     "NewPass456!",
			confirmPassword: "DifferentPass456!",
			expectedStatus:  http.StatusBadRequest,
			expectedError:   "passwords do not match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := tt.setupUser()
			token := tt.setupToken(userID)

			requestBody := ResetPasswordRequest{
				Token:           token,
				NewPassword:     tt.newPassword,
				ConfirmPassword: tt.confirmPassword,
			}

			bodyBytes, err := json.Marshal(requestBody)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/auth/reset-password", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			handler.V1ResetPassword(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response["error"], tt.expectedError)
			}

			// Cleanup
			if userID > 0 {
				_ = queries.DeleteUser(ctx, userID)
			}
		})
	}
}

func TestV1ValidateResetToken(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := setupTestDB(t)
	defer pool.Close()

	handler := newTestHandler(pool)
	queries := db.New(pool)
	ctx := context.Background()

	tests := []struct {
		name           string
		setupUser      func() int32
		setupToken     func(userID int32) string
		expectedStatus int
		expectedValid  bool
	}{
		{
			name: "valid token",
			setupUser: func() int32 {
				return createTestUser(t, pool, "validate1@test.com", "validateuser1", "TestPass123!")
			},
			setupToken: func(userID int32) string {
				token, _ := GenerateSecureToken(64)
				expiresAt := time.Now().Add(1 * time.Hour)
				resetToken, _ := queries.CreatePasswordResetToken(ctx, db.CreatePasswordResetTokenParams{
					UserID:    userID,
					Token:     token,
					ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
				})
				return resetToken.Token
			},
			expectedStatus: http.StatusOK,
			expectedValid:  true,
		},
		{
			name: "expired token",
			setupUser: func() int32 {
				return createTestUser(t, pool, "validate2@test.com", "validateuser2", "TestPass123!")
			},
			setupToken: func(userID int32) string {
				token, _ := GenerateSecureToken(64)
				expiresAt := time.Now().Add(-1 * time.Hour) // Expired
				resetToken, _ := queries.CreatePasswordResetToken(ctx, db.CreatePasswordResetTokenParams{
					UserID:    userID,
					Token:     token,
					ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
				})
				return resetToken.Token
			},
			expectedStatus: http.StatusBadRequest,
			expectedValid:  false,
		},
		{
			name: "invalid token",
			setupUser: func() int32 {
				return 0 // No user
			},
			setupToken: func(userID int32) string {
				return "invalid-token-12345"
			},
			expectedStatus: http.StatusBadRequest,
			expectedValid:  false,
		},
		{
			name: "missing token",
			setupUser: func() int32 {
				return 0
			},
			setupToken: func(userID int32) string {
				return ""
			},
			expectedStatus: http.StatusBadRequest,
			expectedValid:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := tt.setupUser()
			token := tt.setupToken(userID)

			url := "/auth/validate-reset-token"
			if token != "" {
				url = fmt.Sprintf("%s?token=%s", url, token)
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()
			handler.V1ValidateResetToken(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var response map[string]bool
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedValid, response["valid"])
			}

			// Cleanup
			if userID > 0 {
				_ = queries.DeleteUser(ctx, userID)
			}
		})
	}
}

func TestPasswordService_CleanupExpiredTokens(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := setupTestDB(t)
	defer pool.Close()

	queries := db.New(pool)
	ctx := context.Background()

	// Create test user
	userID := createTestUser(t, pool, "cleanup@test.com", "cleanupuser", "TestPass123!")
	defer queries.DeleteUser(ctx, userID)

	// Create expired token
	expiredToken, _ := GenerateSecureToken(64)
	expiresAt := time.Now().Add(-2 * time.Hour) // 2 hours ago
	_, err := queries.CreatePasswordResetToken(ctx, db.CreatePasswordResetTokenParams{
		UserID:    userID,
		Token:     expiredToken,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	require.NoError(t, err)

	// Create valid token
	validToken, _ := GenerateSecureToken(64)
	validExpiresAt := time.Now().Add(1 * time.Hour)
	_, err = queries.CreatePasswordResetToken(ctx, db.CreatePasswordResetTokenParams{
		UserID:    userID,
		Token:     validToken,
		ExpiresAt: pgtype.Timestamptz{Time: validExpiresAt, Valid: true},
	})
	require.NoError(t, err)

	// Create password service
	emailService, _ := email.NewEmailServiceFromEnv()
	passwordService := &PasswordService{
		DB:           pool,
		EmailService: emailService,
	}

	// Cleanup expired tokens
	err = passwordService.CleanupExpiredTokens(ctx)
	require.NoError(t, err)

	// Verify expired token was deleted
	_, err = queries.GetPasswordResetToken(ctx, expiredToken)
	assert.Error(t, err, "Expired token should be deleted")

	// Verify valid token still exists
	validResetToken, err := queries.GetPasswordResetToken(ctx, validToken)
	require.NoError(t, err)
	assert.Equal(t, validToken, validResetToken.Token)
}
