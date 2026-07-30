package core

import (
	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"
	"time"
)

// PreferencesData represents the structured user preferences object.
type PreferencesData struct {
	Theme                string          `json:"theme"`                           // "light" | "dark" | "auto"
	CommentReadMode      string          `json:"comment_read_mode"`               // "auto" | "manual"
	FontSize             string          `json:"font_size"`                       // "small" | "medium" | "large"
	DiscordNotifications map[string]bool `json:"discord_notifications,omitempty"` // per-type Discord DM toggles
}

// User represents a user account in the ActionPhase system.
// It includes authentication credentials, contact information, and metadata.
// The struct supports JSON serialization and validation tags for API usage.
type User struct {
	ID                   int        `json:"id"`                                        // Unique user identifier
	Username             string     `json:"username" validate:"required"`              // Unique username for login
	Email                string     `json:"email" validate:"required,email"`           // User's email address
	EmailVerified        bool       `json:"email_verified"`                            // Whether user has verified their email
	Password             string     `json:"password" validate:"required,min=8,max=64"` // Hashed password (bcrypt)
	Bio                  *string    `json:"bio,omitempty"`                             // User's bio/about text
	AvatarURL            *string    `json:"avatar_url,omitempty"`                      // URL to user's avatar image
	IsAdmin              bool       `json:"is_admin"`                                  // Whether user has admin privileges
	IsBanned             bool       `json:"is_banned"`                                 // Whether user is banned from platform
	BannedAt             *time.Time `json:"banned_at,omitempty"`                       // When user was banned
	BannedByUserID       *int32     `json:"banned_by_user_id,omitempty"`               // ID of admin who banned user
	CreatedAt            *time.Time `json:"createdAt"`                                 // Account creation timestamp
	PendingApproval      bool       `json:"pending_approval"`                          // Whether account is awaiting admin approval
	PendingApprovalSince *time.Time `json:"pending_approval_since,omitempty"`          // When account was placed into pending state
	DiscordUsername      *string    `json:"discord_username,omitempty"`                // Discord username if account is linked
}

// BannedUser represents a banned user with additional ban information.
// Used for admin listing of banned users.
type BannedUser struct {
	ID               int       `json:"id"`
	Username         string    `json:"username"`
	Email            string    `json:"email"`
	BannedAt         time.Time `json:"banned_at"`
	BannedByUserID   int32     `json:"banned_by_user_id"`
	BannedByUsername string    `json:"banned_by_username"`
	CreatedAt        time.Time `json:"created_at"`
}

// IPBan represents a banned IP address.
type IPBan struct {
	ID             int32      `json:"id"`
	IPAddress      string     `json:"ip_address"`
	CreatedBy      int32      `json:"created_by"`
	CreatedAt      time.Time  `json:"created_at"`
	Reason         *string    `json:"reason,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	BannedUserID   *int32     `json:"banned_user_id,omitempty"`
	BannedUsername *string    `json:"banned_username,omitempty"`
}

// FingerprintBan represents a banned device fingerprint.
type FingerprintBan struct {
	ID             int32     `json:"id"`
	Fingerprint    string    `json:"fingerprint"`
	CreatedBy      int32     `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
	Reason         *string   `json:"reason,omitempty"`
	BannedUserID   *int32    `json:"banned_user_id,omitempty"`
	BannedUsername *string   `json:"banned_username,omitempty"`
}

// SessionWithDetails represents a session with metadata for admin views.
type SessionWithDetails struct {
	ID          int32     `json:"id"`
	IPAddress   *string   `json:"ip_address,omitempty"`
	UserAgent   *string   `json:"user_agent,omitempty"`
	Fingerprint *string   `json:"fingerprint,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	Expires     time.Time `json:"expires"`
}

// validate is the shared validator instance for user validation
var validate *validator.Validate

// HashPassword hashes the user's plaintext password using bcrypt.
// This method modifies the User struct by replacing the plaintext password
// with its bcrypt hash. It should be called before storing the user in the database.
//
// Security Features:
//   - Uses bcrypt.DefaultCost (currently 10) for appropriate security/performance balance
//   - Salt is automatically generated and included in the hash
//   - Resistant to rainbow table and brute force attacks
//
// Returns:
//   - error: bcrypt hashing error, or nil if successful
//
// Usage:
//
//	user := &User{Password: "plaintext_password"}
//	err := user.HashPassword()  // user.Password is now hashed
func (u *User) HashPassword() error {
	bytes, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(bytes)
	return nil
}

// CheckPasswordHash verifies a plaintext password against the user's stored hash.
// This method is used during login to authenticate users.
//
// Parameters:
//   - plaintext: The plaintext password to verify
//
// Returns:
//   - bool: true if password matches, false otherwise
//
// Security Notes:
//   - Uses constant-time comparison to prevent timing attacks
//   - No error information is exposed to prevent enumeration attacks
//   - Hash comparison includes salt verification
//
// Usage:
//
//	if user.CheckPasswordHash("attempted_password") {
//	    // Password is correct, user is authenticated
//	}
func (u *User) CheckPasswordHash(plaintext string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(plaintext))
	return err == nil
}

type UserService interface {
	User(id int) (*User, error)
	UserByUsername(username string) (*User, error)
	Users() ([]*User, error)
	CreateUser(u *User) error
	DeleteUser(id int) error
}

func (u *User) Validate() error {
	validate = validator.New(validator.WithRequiredStructEnabled())
	err := validate.Struct(u)
	if err != nil {
		return err
	}
	return nil
}

type Session struct {
	ID          int `json:"id"`
	User        *User
	Token       string     `json:"token"`
	Expires     *time.Time `json:"expires"`
	IPAddress   *string    `json:"ip_address,omitempty"`
	UserAgent   *string    `json:"user_agent,omitempty"`
	Fingerprint *string    `json:"fingerprint,omitempty"`
}

type SessionService interface {
	Session(id int) (*Session, error)
	SessionByToken(token string) (*Session, error)
	Sessions() ([]*Session, error)
	SessionsByUser() ([]*Session, error)
	CreateSession(us *Session) (*Session, error)
	DeleteSession(id int) error
	DeleteSessionByToken(token string) error
}
