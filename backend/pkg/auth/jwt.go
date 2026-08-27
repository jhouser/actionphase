package auth

import (
	"actionphase/pkg/core"
	db "actionphase/pkg/db/services"
	"context"
	"fmt"
	"strconv"

	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Removed global tokenAuth - now using instance method with config secret

func SetJWTCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		HttpOnly: true,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		SameSite: http.SameSiteLaxMode,
		// Uncomment below for HTTPS:
		// Secure: true,
		Name:  "jwt", // Must be named "jwt" or else the token cannot be searched for by jwtauth.Verifier.
		Value: token,
		Path:  "/",
	})
}

// ClearJWTCookie clears the JWT cookie by setting it to expire in the past
func ClearJWTCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		HttpOnly: true,
		Expires:  time.Now().Add(-1 * time.Hour), // Expire in the past
		MaxAge:   -1,                             // Tells browser to delete immediately
		SameSite: http.SameSiteLaxMode,
		// Uncomment below for HTTPS:
		// Secure: true,
		Name:  "jwt",
		Value: "",
		Path:  "/",
	})
}

type JWTHandler struct {
	App *core.App
}

// SessionMetadata carries request-derived metadata to store on the session.
type SessionMetadata struct {
	IPAddress   string
	UserAgent   string
	Fingerprint *string
}

func (j *JWTHandler) CreateToken(user *core.User, meta SessionMetadata) (string, error) {
	// First, create a temporary token to generate the session
	tempToken := jwt.NewWithClaims(jwt.SigningMethodHS256,
		jwt.MapClaims{
			"sub": strconv.Itoa(user.ID),
			"exp": time.Now().Add(core.SessionLifetime).Unix(),
		})

	secretKey := []byte(j.App.Config.JWT.Secret)
	tempTokenString, err := tempToken.SignedString(secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign temporary token: %w", err)
	}

	sessionSvc := db.SessionService{DB: j.App.Pool, Logger: j.App.ObsLogger}
	j.App.Logger.Info("Creating session for user", "user_id", user.ID, "username", user.Username)
	session, err := sessionSvc.CreateSession(&core.Session{User: user, Token: tempTokenString})
	if err != nil {
		return "", fmt.Errorf("failed to create session for user %d: %w", user.ID, err)
	}

	// Create final token with session_id included
	finalToken := jwt.NewWithClaims(jwt.SigningMethodHS256,
		jwt.MapClaims{
			"sub":        strconv.Itoa(user.ID),
			"session_id": session.ID,
			"exp":        time.Now().Add(core.SessionLifetime).Unix(),
		})

	finalTokenString, err := finalToken.SignedString(secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign final token: %w", err)
	}

	if err := sessionSvc.UpdateSessionToken(int32(session.ID), finalTokenString); err != nil {
		return "", fmt.Errorf("failed to update session token: %w", err)
	}

	// Store request metadata on the session now that we have the final token
	if err := sessionSvc.UpdateSessionMetadata(context.Background(), int32(session.ID), meta.IPAddress, meta.UserAgent, meta.Fingerprint); err != nil {
		j.App.Logger.Warn("Failed to store session metadata", "error", err, "session_id", session.ID)
	}

	return finalTokenString, nil
}
