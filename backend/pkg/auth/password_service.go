package auth

import (
	"actionphase/pkg/email"
	"actionphase/pkg/observability"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	db "actionphase/pkg/db/models"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PasswordService handles password management operations
type PasswordService struct {
	DB           *pgxpool.Pool
	EmailService *email.EmailService
	Logger       *observability.Logger
}

// ChangePasswordRequest represents a request to change password for an authenticated user
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

// RequestPasswordResetRequest represents a request to initiate password reset
type RequestPasswordResetRequest struct {
	Email string `json:"email"`
}

// ResetPasswordRequest represents a request to reset password with a token
type ResetPasswordRequest struct {
	Token           string `json:"token"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

// ChangePassword changes the password for an authenticated user
func (s *PasswordService) ChangePassword(ctx context.Context, userID int, req *ChangePasswordRequest) error {
	// Validate passwords match
	if err := ValidatePasswordsMatch(req.NewPassword, req.ConfirmPassword); err != nil {
		return err
	}

	// Validate new password strength
	if err := ValidatePassword(req.NewPassword); err != nil {
		return err
	}

	// Get user from database
	queries := db.New(s.DB)
	user, err := queries.GetUser(ctx, int32(userID))
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Verify current password
	if err := VerifyPassword(req.CurrentPassword, user.Password); err != nil {
		return &PasswordValidationError{
			Field:  "current_password",
			Reason: "current password is incorrect",
		}
	}

	// Hash new password
	hashedPassword, err := HashPassword(req.NewPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password in database
	err = queries.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{
		ID:       int32(userID),
		Password: hashedPassword,
	})
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Send notification email
	if s.EmailService != nil {
		observability.SafeGo(context.Background(), s.Logger, "send-password-changed-email", func() {
			_ = s.EmailService.SendPasswordChangedEmail(context.Background(), user.Email)
		})
	}

	return nil
}

// RequestPasswordReset initiates the password reset process
func (s *PasswordService) RequestPasswordReset(ctx context.Context, req *RequestPasswordResetRequest) error {
	// Validate email format
	if !IsValidEmail(req.Email) {
		return &PasswordValidationError{
			Field:  "email",
			Reason: "invalid email format",
		}
	}

	queries := db.New(s.DB)

	// Find user by email (use silent failure for security - don't reveal if email exists)
	user, err := queries.GetUserByEmail(ctx, strings.ToLower(req.Email))
	if err != nil {
		// Don't reveal that email doesn't exist - return success anyway
		return nil
	}

	// Generate secure token
	token, err := GenerateSecureToken(64)
	if err != nil {
		return fmt.Errorf("failed to generate reset token: %w", err)
	}

	// Create password reset token (expires in 1 hour)
	expiresAt := time.Now().Add(1 * time.Hour)
	resetToken, err := queries.CreatePasswordResetToken(ctx, db.CreatePasswordResetTokenParams{
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to create reset token: %w", err)
	}

	// Send password reset email
	if s.EmailService != nil {
		// Construct reset URL using FRONTEND_URL from environment
		frontendURL := os.Getenv("FRONTEND_URL")
		if frontendURL == "" {
			frontendURL = "http://localhost:5173" // Default for development
		}
		resetURL := fmt.Sprintf("%s/reset-password?token=%s", frontendURL, resetToken.Token)

		err = s.EmailService.SendPasswordResetEmail(ctx, user.Email, resetToken.Token, resetURL)
		if err != nil {
			// Log error but don't fail the request
			// The token is already created, user can try again
			if s.Logger != nil {
				s.Logger.Warn(ctx, "Failed to send password reset email",
					"error", err,
					"email", user.Email)
			}
		}
	}

	return nil
}

// ResetPassword resets a user's password using a valid reset token
func (s *PasswordService) ResetPassword(ctx context.Context, req *ResetPasswordRequest) error {
	// Validate passwords match
	if err := ValidatePasswordsMatch(req.NewPassword, req.ConfirmPassword); err != nil {
		return err
	}

	// Validate new password strength
	if err := ValidatePassword(req.NewPassword); err != nil {
		return err
	}

	queries := db.New(s.DB)

	// Get and validate token
	resetToken, err := queries.GetPasswordResetToken(ctx, req.Token)
	if err != nil {
		return &PasswordValidationError{
			Field:  "token",
			Reason: "invalid or expired reset token",
		}
	}

	// Get user
	user, err := queries.GetUser(ctx, resetToken.UserID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Hash new password
	hashedPassword, err := HashPassword(req.NewPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password
	err = queries.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{
		ID:       user.ID,
		Password: hashedPassword,
	})
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Mark token as used
	err = queries.MarkPasswordResetTokenUsed(ctx, resetToken.ID)
	if err != nil {
		// Log but don't fail - password was already changed
		if s.Logger != nil {
			s.Logger.Warn(ctx, "Failed to mark reset token as used",
				"error", err,
				"token_id", resetToken.ID,
				"user_id", user.ID)
		}
	}

	// Send confirmation email
	if s.EmailService != nil {
		observability.SafeGo(context.Background(), s.Logger, "send-password-changed-email", func() {
			_ = s.EmailService.SendPasswordChangedEmail(context.Background(), user.Email)
		})
	}

	return nil
}

// CleanupExpiredTokens removes expired password reset tokens
func (s *PasswordService) CleanupExpiredTokens(ctx context.Context) error {
	queries := db.New(s.DB)
	return queries.DeleteExpiredPasswordResetTokens(ctx)
}
