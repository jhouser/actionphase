package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	db "actionphase/pkg/db/models"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BotPreventionService handles bot prevention mechanisms
type BotPreventionService struct {
	DB              *pgxpool.Pool
	HCaptchaSecret  string
	HCaptchaEnabled bool
	IsDevelopment   bool
}

// NewBotPreventionService creates a new bot prevention service
func NewBotPreventionService(pool *pgxpool.Pool) *BotPreventionService {
	hcaptchaSecret := os.Getenv("HCAPTCHA_SECRET")
	hcaptchaEnabled := os.Getenv("HCAPTCHA_ENABLED")
	environment := os.Getenv("ENVIRONMENT")
	return &BotPreventionService{
		DB:              pool,
		HCaptchaSecret:  hcaptchaSecret,
		HCaptchaEnabled: hcaptchaEnabled == "true",
		IsDevelopment:   environment == "development",
	}
}

// RegistrationCheckRequest contains data for bot prevention checks
type RegistrationCheckRequest struct {
	Email         string
	Username      string
	IPAddress     string
	UserAgent     string
	HCaptchaToken string
	HoneypotValue string
}

// RegistrationCheckResult contains the result of bot prevention checks
type RegistrationCheckResult struct {
	Allowed        bool
	BlockedReason  string
	CaptchaPassed  bool
	HoneypotFailed bool
}

// CheckRegistrationAttempt performs all bot prevention checks
func (s *BotPreventionService) CheckRegistrationAttempt(ctx context.Context, req *RegistrationCheckRequest) (*RegistrationCheckResult, error) {
	queries := db.New(s.DB)

	result := &RegistrationCheckResult{
		Allowed: true,
	}

	// 1. Honeypot check (must be empty)
	if req.HoneypotValue != "" {
		result.Allowed = false
		result.BlockedReason = "honeypot"
		result.HoneypotFailed = true

		// Log the attempt
		_, _ = queries.CreateRegistrationAttempt(ctx, db.CreateRegistrationAttemptParams{
			Email:             req.Email,
			Username:          req.Username,
			IpAddress:         req.IPAddress,
			UserAgent:         pgtype.Text{String: req.UserAgent, Valid: req.UserAgent != ""},
			CaptchaPassed:     false,
			HoneypotTriggered: true,
			BlockedReason:     pgtype.Text{String: "honeypot", Valid: true},
			Successful:        false,
		})

		return result, nil
	}

	// 2. hCaptcha verification (if enabled)
	if s.HCaptchaEnabled {
		captchaPassed, err := s.VerifyHCaptcha(ctx, req.HCaptchaToken, req.IPAddress)
		if err != nil {
			return nil, fmt.Errorf("failed to verify hCaptcha: %w", err)
		}

		result.CaptchaPassed = captchaPassed

		if !captchaPassed {
			result.Allowed = false
			result.BlockedReason = "captcha_failed"

			// Log the attempt
			_, _ = queries.CreateRegistrationAttempt(ctx, db.CreateRegistrationAttemptParams{
				Email:             req.Email,
				Username:          req.Username,
				IpAddress:         req.IPAddress,
				UserAgent:         pgtype.Text{String: req.UserAgent, Valid: req.UserAgent != ""},
				CaptchaPassed:     false,
				HoneypotTriggered: false,
				BlockedReason:     pgtype.Text{String: "captcha_failed", Valid: true},
				Successful:        false,
			})

			return result, nil
		}
	} else {
		result.CaptchaPassed = true // No captcha configured
	}

	// 3. Rate limiting by IP (max 5 attempts per hour) - Skip in development
	if !s.IsDevelopment {
		oneHourAgo := time.Now().Add(-1 * time.Hour)
		ipAttempts, err := queries.CountRecentRegistrationAttemptsByIP(ctx, db.CountRecentRegistrationAttemptsByIPParams{
			IpAddress: req.IPAddress,
			CreatedAt: pgtype.Timestamptz{Time: oneHourAgo, Valid: true},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to count IP attempts: %w", err)
		}

		if ipAttempts >= 5 {
			result.Allowed = false
			result.BlockedReason = "rate_limit_ip"

			// Log the attempt
			_, _ = queries.CreateRegistrationAttempt(ctx, db.CreateRegistrationAttemptParams{
				Email:             req.Email,
				Username:          req.Username,
				IpAddress:         req.IPAddress,
				UserAgent:         pgtype.Text{String: req.UserAgent, Valid: req.UserAgent != ""},
				CaptchaPassed:     result.CaptchaPassed,
				HoneypotTriggered: false,
				BlockedReason:     pgtype.Text{String: "rate_limit_ip", Valid: true},
				Successful:        false,
			})

			return result, nil
		}
	}

	// 4. Rate limiting by email (max 3 attempts per day) - Skip in development
	if !s.IsDevelopment {
		oneDayAgo := time.Now().Add(-24 * time.Hour)
		emailAttempts, err := queries.CountRecentRegistrationAttemptsByEmail(ctx, db.CountRecentRegistrationAttemptsByEmailParams{
			Email:     req.Email,
			CreatedAt: pgtype.Timestamptz{Time: oneDayAgo, Valid: true},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to count email attempts: %w", err)
		}

		if emailAttempts >= 3 {
			result.Allowed = false
			result.BlockedReason = "rate_limit_email"

			// Log the attempt
			_, _ = queries.CreateRegistrationAttempt(ctx, db.CreateRegistrationAttemptParams{
				Email:             req.Email,
				Username:          req.Username,
				IpAddress:         req.IPAddress,
				UserAgent:         pgtype.Text{String: req.UserAgent, Valid: req.UserAgent != ""},
				CaptchaPassed:     result.CaptchaPassed,
				HoneypotTriggered: false,
				BlockedReason:     pgtype.Text{String: "rate_limit_email", Valid: true},
				Successful:        false,
			})

			return result, nil
		}
	}

	// 5. Disposable email detection
	if IsDisposableEmail(req.Email) {
		result.Allowed = false
		result.BlockedReason = "disposable_email"

		// Log the attempt
		_, _ = queries.CreateRegistrationAttempt(ctx, db.CreateRegistrationAttemptParams{
			Email:             req.Email,
			Username:          req.Username,
			IpAddress:         req.IPAddress,
			UserAgent:         pgtype.Text{String: req.UserAgent, Valid: req.UserAgent != ""},
			CaptchaPassed:     result.CaptchaPassed,
			HoneypotTriggered: false,
			BlockedReason:     pgtype.Text{String: "disposable_email", Valid: true},
			Successful:        false,
		})

		return result, nil
	}

	// All checks passed - log the attempt as pending (not successful yet, user not created)
	// This is important for rate limiting to work correctly
	_, _ = queries.CreateRegistrationAttempt(ctx, db.CreateRegistrationAttemptParams{
		Email:             req.Email,
		Username:          req.Username,
		IpAddress:         req.IPAddress,
		UserAgent:         pgtype.Text{String: req.UserAgent, Valid: req.UserAgent != ""},
		CaptchaPassed:     result.CaptchaPassed,
		HoneypotTriggered: false,
		BlockedReason:     pgtype.Text{Valid: false}, // No block reason
		Successful:        false,                     // Not successful yet - will be marked successful after user creation
	})

	return result, nil
}

// LogSuccessfulRegistration logs a successful registration attempt
func (s *BotPreventionService) LogSuccessfulRegistration(ctx context.Context, req *RegistrationCheckRequest) error {
	queries := db.New(s.DB)

	_, err := queries.CreateRegistrationAttempt(ctx, db.CreateRegistrationAttemptParams{
		Email:             req.Email,
		Username:          req.Username,
		IpAddress:         req.IPAddress,
		UserAgent:         pgtype.Text{String: req.UserAgent, Valid: req.UserAgent != ""},
		CaptchaPassed:     true,
		HoneypotTriggered: false,
		BlockedReason:     pgtype.Text{Valid: false},
		Successful:        true,
	})

	return err
}

// VerifyHCaptcha verifies an hCaptcha token
func (s *BotPreventionService) VerifyHCaptcha(ctx context.Context, token string, remoteIP string) (bool, error) {
	if token == "" {
		return false, nil
	}

	// Prepare verification request
	data := url.Values{}
	data.Set("secret", s.HCaptchaSecret)
	data.Set("response", token)
	data.Set("remoteip", remoteIP)

	// Send verification request to hCaptcha
	resp, err := http.PostForm("https://hcaptcha.com/siteverify", data)
	if err != nil {
		return false, fmt.Errorf("failed to send hCaptcha verification request: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var result struct {
		Success bool `json:"success"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("failed to decode hCaptcha response: %w", err)
	}

	return result.Success, nil
}

// IsDisposableEmail checks if an email is from a known disposable email provider
// This is a basic implementation - in production, use a comprehensive list or service
func IsDisposableEmail(email string) bool {
	// Extract domain from email
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}

	domain := strings.ToLower(parts[1])

	// List of common disposable email domains
	// In production, this should be a comprehensive list or external service
	disposableDomains := map[string]bool{
		"tempmail.com":      true,
		"guerrillamail.com": true,
		"10minutemail.com":  true,
		"mailinator.com":    true,
		"throwaway.email":   true,
		"temp-mail.org":     true,
		"getairmail.com":    true,
		"fakeinbox.com":     true,
		"trashmail.com":     true,
		"dispostable.com":   true,
	}

	return disposableDomains[domain]
}

// CleanupOldRegistrationAttempts removes registration attempts older than 90 days
func (s *BotPreventionService) CleanupOldRegistrationAttempts(ctx context.Context) error {
	queries := db.New(s.DB)
	ninetyDaysAgo := time.Now().Add(-90 * 24 * time.Hour)
	return queries.DeleteOldRegistrationAttempts(ctx, pgtype.Timestamptz{Time: ninetyDaysAgo, Valid: true})
}
