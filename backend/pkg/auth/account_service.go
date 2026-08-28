package auth

import (
	"actionphase/pkg/email"
	"actionphase/pkg/observability"
	"context"
	"fmt"
	"os"
	"regexp"
	"time"

	db "actionphase/pkg/db/models"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	MinUsernameLength = 3
	MaxUsernameLength = 50
)

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// validateUsername validates username format and length
func validateUsername(username string) error {
	if len(username) < MinUsernameLength {
		return &PasswordValidationError{
			Field:  "username",
			Reason: fmt.Sprintf("username must be at least %d characters long", MinUsernameLength),
		}
	}

	if len(username) > MaxUsernameLength {
		return &PasswordValidationError{
			Field:  "username",
			Reason: fmt.Sprintf("username must be at most %d characters long", MaxUsernameLength),
		}
	}

	if !usernameRegex.MatchString(username) {
		return &PasswordValidationError{
			Field:  "username",
			Reason: "username can only contain letters, numbers, underscores, and hyphens",
		}
	}

	return nil
}

// AccountService handles account management operations (email verification, username/email changes)
type AccountService struct {
	DB           *pgxpool.Pool
	EmailService *email.EmailService
	Logger       *observability.Logger
}

// SendVerificationEmailRequest represents a request to send a verification email
type SendVerificationEmailRequest struct {
	UserID int
	Email  string
}

// VerifyEmailRequest represents a request to verify an email with a token
type VerifyEmailRequest struct {
	Token string `json:"token"`
}

// ChangeUsernameRequest represents a request to change username
type ChangeUsernameRequest struct {
	NewUsername     string `json:"new_username"`
	CurrentPassword string `json:"current_password"`
}

// ChangeEmailRequest represents a request to change email
type ChangeEmailRequest struct {
	NewEmail        string `json:"new_email"`
	CurrentPassword string `json:"current_password"`
}

// SendVerificationEmail sends an email verification token to the user
func (s *AccountService) SendVerificationEmail(ctx context.Context, req *SendVerificationEmailRequest) error {
	// Validate email format
	if !IsValidEmail(req.Email) {
		return &PasswordValidationError{
			Field:  "email",
			Reason: "invalid email format",
		}
	}

	queries := db.New(s.DB)

	// Check if user exists
	user, err := queries.GetUser(ctx, int32(req.UserID))
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// If already verified, return success
	if user.EmailVerified {
		return nil
	}

	// Generate secure token
	token, err := GenerateSecureToken(64)
	if err != nil {
		return fmt.Errorf("failed to generate verification token: %w", err)
	}

	// Create email verification token (expires in 24 hours)
	expiresAt := time.Now().Add(24 * time.Hour)
	verificationToken, err := queries.CreateEmailVerificationToken(ctx, db.CreateEmailVerificationTokenParams{
		UserID:    int32(req.UserID),
		Email:     req.Email,
		Token:     token,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to create verification token: %w", err)
	}

	// Send verification email
	if s.EmailService != nil {
		// Construct verification URL using FRONTEND_URL from environment
		frontendURL := os.Getenv("FRONTEND_URL")
		if frontendURL == "" {
			frontendURL = "http://localhost:5173" // Default for development
		}
		verificationURL := fmt.Sprintf("%s/verify-email?token=%s", frontendURL, verificationToken.Token)

		err = s.EmailService.SendEmailVerificationEmail(ctx, req.Email, verificationToken.Token, verificationURL)
		if err != nil {
			// Log error but don't fail the request
			// The token is already created, user can request a new verification email
			if s.Logger != nil {
				s.Logger.Warn(ctx, "Failed to send verification email", "error", err, "email", req.Email)
			}
		}
	}

	return nil
}

// VerifyEmail verifies a user's email using a valid verification token
func (s *AccountService) VerifyEmail(ctx context.Context, req *VerifyEmailRequest) error {
	// Start transaction
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	queries := db.New(tx)

	// Get and validate token
	verificationToken, err := queries.GetEmailVerificationToken(ctx, req.Token)
	if err != nil {
		return &PasswordValidationError{
			Field:  "token",
			Reason: "invalid or expired verification token",
		}
	}

	// Mark user's email as verified
	err = queries.MarkUserEmailVerified(ctx, verificationToken.UserID)
	if err != nil {
		return fmt.Errorf("failed to mark email as verified: %w", err)
	}

	// Mark token as used
	err = queries.MarkEmailVerificationTokenUsed(ctx, verificationToken.ID)
	if err != nil {
		return fmt.Errorf("failed to mark verification token as used: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit email verification transaction: %w", err)
	}

	if s.Logger != nil {
		s.Logger.Info(ctx, "Email verified successfully", "user_id", verificationToken.UserID)
	}

	return nil
}

// ChangeUsername changes a user's username (with cooldown period check)
func (s *AccountService) ChangeUsername(ctx context.Context, userID int, req *ChangeUsernameRequest) error {
	// Validate current password is provided
	if req.CurrentPassword == "" {
		return &PasswordValidationError{
			Field:  "current_password",
			Reason: "Current password is required",
		}
	}

	// Validate username format and length
	if err := validateUsername(req.NewUsername); err != nil {
		return err
	}

	queries := db.New(s.DB)

	// Get user to check cooldown period and verify password
	user, err := queries.GetUser(ctx, int32(userID))
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Verify current password
	err = VerifyPassword(req.CurrentPassword, user.Password)
	if err != nil {
		return &PasswordValidationError{
			Field:  "current_password",
			Reason: "incorrect password",
		}
	}

	// Check if username was changed recently (30-day cooldown)
	if user.UsernameChangedAt.Valid {
		cooldownEnd := user.UsernameChangedAt.Time.Add(30 * 24 * time.Hour)
		if time.Now().Before(cooldownEnd) {
			daysRemaining := int(time.Until(cooldownEnd).Hours() / 24)
			return &PasswordValidationError{
				Field:  "username",
				Reason: fmt.Sprintf("username can only be changed once per 30 days (%d days remaining)", daysRemaining),
			}
		}
	}

	// Check if new username is already taken
	_, err = queries.GetUserByUsername(ctx, req.NewUsername)
	if err == nil {
		// Username exists
		return &PasswordValidationError{
			Field:  "username",
			Reason: "username is already taken",
		}
	}

	// Update username
	err = queries.UpdateUserUsername(ctx, db.UpdateUserUsernameParams{
		ID:       int32(userID),
		Username: req.NewUsername,
	})
	if err != nil {
		return fmt.Errorf("failed to update username: %w", err)
	}

	return nil
}

// RequestEmailChange initiates the email change process
func (s *AccountService) RequestEmailChange(ctx context.Context, userID int, req *ChangeEmailRequest) error {
	// Validate current password is provided
	if req.CurrentPassword == "" {
		return &PasswordValidationError{
			Field:  "current_password",
			Reason: "Current password is required",
		}
	}

	// Validate email format
	if !IsValidEmail(req.NewEmail) {
		return &PasswordValidationError{
			Field:  "email",
			Reason: "invalid email format",
		}
	}

	// Normalize email to lowercase
	req.NewEmail = strings.ToLower(req.NewEmail)

	queries := db.New(s.DB)

	// Get user to verify password
	user, err := queries.GetUser(ctx, int32(userID))
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Verify current password
	err = VerifyPassword(req.CurrentPassword, user.Password)
	if err != nil {
		return &PasswordValidationError{
			Field:  "current_password",
			Reason: "incorrect password",
		}
	}

	// Check if email is already taken
	_, err = queries.GetUserByEmail(ctx, req.NewEmail)
	if err == nil {
		// Email exists
		return &PasswordValidationError{
			Field:  "email",
			Reason: "email is already in use",
		}
	}

	// Set pending email change
	err = queries.SetEmailChangePending(ctx, db.SetEmailChangePendingParams{
		ID:                 int32(userID),
		EmailChangePending: pgtype.Text{String: req.NewEmail, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to set pending email change: %w", err)
	}

	// Send verification email to new address
	verificationReq := &SendVerificationEmailRequest{
		UserID: userID,
		Email:  req.NewEmail,
	}

	return s.SendVerificationEmail(ctx, verificationReq)
}

// CompleteEmailChange completes the email change after verification
func (s *AccountService) CompleteEmailChange(ctx context.Context, req *VerifyEmailRequest) error {
	// Start transaction
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	queries := db.New(tx)

	// Get and validate token
	verificationToken, err := queries.GetEmailVerificationToken(ctx, req.Token)
	if err != nil {
		return &PasswordValidationError{
			Field:  "token",
			Reason: "invalid or expired verification token",
		}
	}

	// Get user to check if they have a pending email change
	user, err := queries.GetUser(ctx, verificationToken.UserID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Verify the token email matches the pending email change
	if !user.EmailChangePending.Valid || user.EmailChangePending.String != verificationToken.Email {
		return &PasswordValidationError{
			Field:  "token",
			Reason: "no matching pending email change",
		}
	}

	// Update email (this also clears email_change_pending and sets email_verified = true)
	err = queries.UpdateUserEmail(ctx, db.UpdateUserEmailParams{
		ID:    verificationToken.UserID,
		Email: strings.ToLower(verificationToken.Email),
	})
	if err != nil {
		return fmt.Errorf("failed to update email: %w", err)
	}

	// Mark token as used
	err = queries.MarkEmailVerificationTokenUsed(ctx, verificationToken.ID)
	if err != nil {
		return fmt.Errorf("failed to mark verification token as used: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit email change transaction: %w", err)
	}

	if s.Logger != nil {
		s.Logger.Info(ctx, "Email change completed successfully",
			"user_id", verificationToken.UserID,
			"new_email", verificationToken.Email)
	}

	if s.EmailService != nil {
		if err := s.EmailService.SendEmailChangedEmail(ctx, user.Email, verificationToken.Email); err != nil {
			if s.Logger != nil {
				s.Logger.Warn(ctx, "Failed to send email change notification", "error", err, "user_id", verificationToken.UserID)
			}
		}
	}

	return nil
}

// CleanupExpiredVerificationTokens removes expired email verification tokens
func (s *AccountService) CleanupExpiredVerificationTokens(ctx context.Context) error {
	queries := db.New(s.DB)
	return queries.DeleteExpiredEmailVerificationTokens(ctx)
}

// ResendVerificationEmail resends a verification email for a user
func (s *AccountService) ResendVerificationEmail(ctx context.Context, userID int) error {
	queries := db.New(s.DB)

	// Get user
	user, err := queries.GetUser(ctx, int32(userID))
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// If already verified, return success
	if user.EmailVerified {
		return nil
	}

	// Determine which email to send to
	email := user.Email
	if user.EmailChangePending.Valid {
		email = user.EmailChangePending.String
	}

	// Send verification email
	return s.SendVerificationEmail(ctx, &SendVerificationEmailRequest{
		UserID: userID,
		Email:  email,
	})
}

// SoftDeleteAccount marks a user account for deletion (30-day recovery window)
func (s *AccountService) SoftDeleteAccount(ctx context.Context, userID int) error {
	queries := db.New(s.DB)

	// Get user to capture email before deletion
	user, err := queries.GetUser(ctx, int32(userID))
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Soft delete the user
	err = queries.SoftDeleteUser(ctx, int32(userID))
	if err != nil {
		return fmt.Errorf("failed to delete user account: %w", err)
	}

	// Invalidate all user sessions
	err = queries.DeleteUserSessions(ctx, int32(userID))
	if err != nil {
		// Log but don't fail - account is already marked as deleted
		if s.Logger != nil {
			s.Logger.Warn(ctx, "Failed to invalidate user sessions after account deletion", "error", err, "user_id", userID)
		}
	}

	if s.EmailService != nil {
		deletionScheduledFor := time.Now().AddDate(0, 0, 30)
		if err := s.EmailService.SendAccountDeletionScheduledEmail(ctx, user.Email, deletionScheduledFor); err != nil {
			if s.Logger != nil {
				s.Logger.Warn(ctx, "Failed to send account deletion notification email", "error", err, "user_id", userID)
			}
		}
	}

	return nil
}

// RevokeAllSessions revokes all sessions for a user (except current session)
func (s *AccountService) RevokeAllSessions(ctx context.Context, userID int, currentSessionID int32) error {
	queries := db.New(s.DB)

	// Get all user sessions
	sessions, err := queries.GetSessionsByUser(ctx, int32(userID))
	if err != nil {
		return fmt.Errorf("failed to get user sessions: %w", err)
	}

	// Delete all sessions except the current one
	for _, session := range sessions {
		if session.ID != currentSessionID {
			err = queries.DeleteSession(ctx, session.ID)
			if err != nil {
				// Log but don't fail - continue revoking other sessions
				if s.Logger != nil {
					s.Logger.Warn(ctx, "Failed to revoke session", "error", err, "session_id", session.ID, "user_id", userID)
				}
			}
		}
	}

	return nil
}
