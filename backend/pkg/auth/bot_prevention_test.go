package auth

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"actionphase/pkg/core"
	db "actionphase/pkg/db/models"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setupBotPreventionTest(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	testDB := core.NewTestDatabase(t)

	// Clean up registration_attempts table
	_, err := testDB.Pool.Exec(context.Background(), "DELETE FROM registration_attempts")
	if err != nil {
		t.Fatalf("Failed to clean up registration_attempts: %v", err)
	}

	cleanup := func() {
		testDB.Close()
	}

	return testDB.Pool, cleanup
}

func TestBotPreventionService_HoneypotDetection(t *testing.T) {
	if os.Getenv("SKIP_DB_TESTS") == "true" {
		t.Skip("Skipping database test")
	}

	pool, cleanup := setupBotPreventionTest(t)
	defer cleanup()

	service := NewBotPreventionService(pool)
	ctx := context.Background()

	// Test with honeypot triggered
	req := &RegistrationCheckRequest{
		Email:         "test@example.com",
		Username:      "testuser",
		IPAddress:     "192.168.1.1",
		UserAgent:     "Mozilla/5.0",
		HCaptchaToken: "",
		HoneypotValue: "bot-filled-this", // Honeypot triggered
	}

	result, err := service.CheckRegistrationAttempt(ctx, req)
	if err != nil {
		t.Fatalf("CheckRegistrationAttempt failed: %v", err)
	}

	if result.Allowed {
		t.Error("Expected registration to be blocked, but it was allowed")
	}

	if result.BlockedReason != "honeypot" {
		t.Errorf("Expected blocked reason 'honeypot', got '%s'", result.BlockedReason)
	}

	if !result.HoneypotFailed {
		t.Error("Expected HoneypotFailed to be true")
	}

	// Verify attempt was logged
	queries := db.New(pool)
	attempts, err := queries.CountRecentRegistrationAttemptsByEmail(ctx, db.CountRecentRegistrationAttemptsByEmailParams{
		Email:     req.Email,
		CreatedAt: pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed to count attempts: %v", err)
	}

	if attempts != 1 {
		t.Errorf("Expected 1 logged attempt, got %d", attempts)
	}
}

func TestBotPreventionService_IPRateLimiting(t *testing.T) {
	if os.Getenv("SKIP_DB_TESTS") == "true" {
		t.Skip("Skipping database test")
	}

	pool, cleanup := setupBotPreventionTest(t)
	defer cleanup()

	service := NewBotPreventionService(pool)
	ctx := context.Background()
	ipAddress := "192.168.1.100"

	// Create 5 registration attempts from same IP
	for i := 0; i < 5; i++ {
		req := &RegistrationCheckRequest{
			Email:         "test" + string(rune('1'+i)) + "@example.com",
			Username:      "testuser" + string(rune('1'+i)),
			IPAddress:     ipAddress,
			UserAgent:     "Mozilla/5.0",
			HCaptchaToken: "",
			HoneypotValue: "",
		}

		result, err := service.CheckRegistrationAttempt(ctx, req)
		if err != nil {
			t.Fatalf("CheckRegistrationAttempt failed: %v", err)
		}

		if i < 4 {
			// First 4 attempts should be allowed
			if !result.Allowed {
				t.Errorf("Attempt %d should be allowed, but was blocked: %s", i+1, result.BlockedReason)
			}
		}
	}

	// 6th attempt should be blocked
	req := &RegistrationCheckRequest{
		Email:         "test6@example.com",
		Username:      "testuser6",
		IPAddress:     ipAddress,
		UserAgent:     "Mozilla/5.0",
		HCaptchaToken: "",
		HoneypotValue: "",
	}

	result, err := service.CheckRegistrationAttempt(ctx, req)
	if err != nil {
		t.Fatalf("CheckRegistrationAttempt failed: %v", err)
	}

	if result.Allowed {
		t.Error("Expected 6th attempt to be blocked by IP rate limit")
	}

	if result.BlockedReason != "rate_limit_ip" {
		t.Errorf("Expected blocked reason 'rate_limit_ip', got '%s'", result.BlockedReason)
	}
}

func TestBotPreventionService_EmailRateLimiting(t *testing.T) {
	if os.Getenv("SKIP_DB_TESTS") == "true" {
		t.Skip("Skipping database test")
	}

	pool, cleanup := setupBotPreventionTest(t)
	defer cleanup()

	service := NewBotPreventionService(pool)
	ctx := context.Background()
	email := "test@example.com"

	// Create 3 registration attempts with same email
	for i := 0; i < 3; i++ {
		req := &RegistrationCheckRequest{
			Email:         email,
			Username:      "testuser" + string(rune('1'+i)),
			IPAddress:     "192.168.1." + string(rune('1'+i)),
			UserAgent:     "Mozilla/5.0",
			HCaptchaToken: "",
			HoneypotValue: "",
		}

		result, err := service.CheckRegistrationAttempt(ctx, req)
		if err != nil {
			t.Fatalf("CheckRegistrationAttempt failed: %v", err)
		}

		if i < 2 {
			// First 2 attempts should be allowed
			if !result.Allowed {
				t.Errorf("Attempt %d should be allowed, but was blocked: %s", i+1, result.BlockedReason)
			}
		}
	}

	// 4th attempt should be blocked
	req := &RegistrationCheckRequest{
		Email:         email,
		Username:      "testuser4",
		IPAddress:     "192.168.1.4",
		UserAgent:     "Mozilla/5.0",
		HCaptchaToken: "",
		HoneypotValue: "",
	}

	result, err := service.CheckRegistrationAttempt(ctx, req)
	if err != nil {
		t.Fatalf("CheckRegistrationAttempt failed: %v", err)
	}

	if result.Allowed {
		t.Error("Expected 4th attempt to be blocked by email rate limit")
	}

	if result.BlockedReason != "rate_limit_email" {
		t.Errorf("Expected blocked reason 'rate_limit_email', got '%s'", result.BlockedReason)
	}
}

func TestBotPreventionService_DisposableEmailDetection(t *testing.T) {
	if os.Getenv("SKIP_DB_TESTS") == "true" {
		t.Skip("Skipping database test")
	}

	pool, cleanup := setupBotPreventionTest(t)
	defer cleanup()

	service := NewBotPreventionService(pool)
	ctx := context.Background()

	disposableEmails := []string{
		"test@tempmail.com",
		"user@guerrillamail.com",
		"temp@10minutemail.com",
		"fake@mailinator.com",
		"throw@throwaway.email",
	}

	for i, email := range disposableEmails {
		req := &RegistrationCheckRequest{
			Email:         email,
			Username:      "testuser",
			IPAddress:     fmt.Sprintf("192.168.1.%d", i+10), // Use different IPs to avoid rate limiting
			UserAgent:     "Mozilla/5.0",
			HCaptchaToken: "",
			HoneypotValue: "",
		}

		result, err := service.CheckRegistrationAttempt(ctx, req)
		if err != nil {
			t.Fatalf("CheckRegistrationAttempt failed for %s: %v", email, err)
		}

		if result.Allowed {
			t.Errorf("Expected %s to be blocked as disposable email", email)
		}

		if result.BlockedReason != "disposable_email" {
			t.Errorf("Expected blocked reason 'disposable_email' for %s, got '%s'", email, result.BlockedReason)
		}
	}

	// Test with valid email (use different IP to avoid rate limiting from previous tests)
	req := &RegistrationCheckRequest{
		Email:         "user@gmail.com",
		Username:      "testuser",
		IPAddress:     "192.168.1.99",
		UserAgent:     "Mozilla/5.0",
		HCaptchaToken: "",
		HoneypotValue: "",
	}

	result, err := service.CheckRegistrationAttempt(ctx, req)
	if err != nil {
		t.Fatalf("CheckRegistrationAttempt failed: %v", err)
	}

	if !result.Allowed {
		t.Error("Expected gmail.com to be allowed")
	}
}

func TestBotPreventionService_AllChecksPass(t *testing.T) {
	if os.Getenv("SKIP_DB_TESTS") == "true" {
		t.Skip("Skipping database test")
	}

	pool, cleanup := setupBotPreventionTest(t)
	defer cleanup()

	service := NewBotPreventionService(pool)
	ctx := context.Background()

	req := &RegistrationCheckRequest{
		Email:         "valid@gmail.com",
		Username:      "validuser",
		IPAddress:     "192.168.1.50",
		UserAgent:     "Mozilla/5.0",
		HCaptchaToken: "", // No captcha configured in test
		HoneypotValue: "", // Honeypot not triggered
	}

	result, err := service.CheckRegistrationAttempt(ctx, req)
	if err != nil {
		t.Fatalf("CheckRegistrationAttempt failed: %v", err)
	}

	if !result.Allowed {
		t.Errorf("Expected registration to be allowed, but was blocked: %s", result.BlockedReason)
	}

	if result.BlockedReason != "" {
		t.Errorf("Expected no blocked reason, got '%s'", result.BlockedReason)
	}

	if result.HoneypotFailed {
		t.Error("Expected HoneypotFailed to be false")
	}

	// CaptchaPassed should be true when captcha is not configured
	if !result.CaptchaPassed {
		t.Error("Expected CaptchaPassed to be true when captcha not configured")
	}
}

func TestBotPreventionService_LogSuccessfulRegistration(t *testing.T) {
	if os.Getenv("SKIP_DB_TESTS") == "true" {
		t.Skip("Skipping database test")
	}

	pool, cleanup := setupBotPreventionTest(t)
	defer cleanup()

	service := NewBotPreventionService(pool)
	ctx := context.Background()

	req := &RegistrationCheckRequest{
		Email:         "success@example.com",
		Username:      "successuser",
		IPAddress:     "192.168.1.200",
		UserAgent:     "Mozilla/5.0",
		HCaptchaToken: "",
		HoneypotValue: "",
	}

	err := service.LogSuccessfulRegistration(ctx, req)
	if err != nil {
		t.Fatalf("LogSuccessfulRegistration failed: %v", err)
	}

	// Verify successful registration was logged
	queries := db.New(pool)
	attempt, err := queries.GetRecentSuccessfulRegistrationByIP(ctx, db.GetRecentSuccessfulRegistrationByIPParams{
		IpAddress: req.IPAddress,
		CreatedAt: pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Minute), Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed to get successful registration: %v", err)
	}

	if attempt.Email != req.Email {
		t.Errorf("Expected email %s, got %s", req.Email, attempt.Email)
	}

	if !attempt.Successful {
		t.Error("Expected successful flag to be true")
	}

	if attempt.CaptchaPassed != true {
		t.Error("Expected captcha_passed to be true for successful registration")
	}
}

func TestIsDisposableEmail(t *testing.T) {
	tests := []struct {
		email        string
		isDisposable bool
	}{
		{"user@tempmail.com", true},
		{"test@guerrillamail.com", true},
		{"fake@10minutemail.com", true},
		{"temp@mailinator.com", true},
		{"user@throwaway.email", true},
		{"valid@gmail.com", false},
		{"work@company.com", false},
		{"personal@outlook.com", false},
		{"invalid-email", false}, // Invalid format
	}

	for _, tt := range tests {
		result := IsDisposableEmail(tt.email)
		if result != tt.isDisposable {
			t.Errorf("IsDisposableEmail(%s) = %v, want %v", tt.email, result, tt.isDisposable)
		}
	}
}

func TestBotPreventionService_CleanupOldAttempts(t *testing.T) {
	if os.Getenv("SKIP_DB_TESTS") == "true" {
		t.Skip("Skipping database test")
	}

	pool, cleanup := setupBotPreventionTest(t)
	defer cleanup()

	service := NewBotPreventionService(pool)
	ctx := context.Background()

	// Create a registration attempt
	req := &RegistrationCheckRequest{
		Email:         "cleanup@example.com",
		Username:      "cleanupuser",
		IPAddress:     "192.168.1.99",
		UserAgent:     "Mozilla/5.0",
		HCaptchaToken: "",
		HoneypotValue: "",
	}

	_, err := service.CheckRegistrationAttempt(ctx, req)
	if err != nil {
		t.Fatalf("CheckRegistrationAttempt failed: %v", err)
	}

	// Verify attempt was created
	queries := db.New(pool)
	countBefore, err := queries.CountRecentRegistrationAttemptsByEmail(ctx, db.CountRecentRegistrationAttemptsByEmailParams{
		Email:     req.Email,
		CreatedAt: pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed to count attempts: %v", err)
	}

	if countBefore != 1 {
		t.Errorf("Expected 1 attempt before cleanup, got %d", countBefore)
	}

	// Run cleanup (should not delete recent attempts)
	err = service.CleanupOldRegistrationAttempts(ctx)
	if err != nil {
		t.Fatalf("CleanupOldRegistrationAttempts failed: %v", err)
	}

	// Verify attempt still exists
	countAfter, err := queries.CountRecentRegistrationAttemptsByEmail(ctx, db.CountRecentRegistrationAttemptsByEmailParams{
		Email:     req.Email,
		CreatedAt: pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatalf("Failed to count attempts after cleanup: %v", err)
	}

	if countAfter != 1 {
		t.Errorf("Expected 1 attempt after cleanup (should not delete recent), got %d", countAfter)
	}
}
