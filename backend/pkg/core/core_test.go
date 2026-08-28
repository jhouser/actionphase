package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- middleware.go ---

type mockUserService struct {
	user *User
	err  error
}

func (m *mockUserService) GetUserByID(_ int) (*User, error) {
	return m.user, m.err
}

func TestGetAuthenticatedUser_Present(t *testing.T) {
	authUser := &AuthenticatedUser{ID: 42, Username: "alice"}
	ctx := context.WithValue(context.Background(), UserContextKey, authUser)
	result := GetAuthenticatedUser(ctx)
	require.NotNil(t, result)
	assert.Equal(t, int32(42), result.ID)
	assert.Equal(t, "alice", result.Username)
}

func TestGetAuthenticatedUser_Absent(t *testing.T) {
	result := GetAuthenticatedUser(context.Background())
	assert.Nil(t, result)
}

// --- dashboard.go ---

func TestCalculateDeadlineStatus_Critical(t *testing.T) {
	assert.Equal(t, "critical", CalculateDeadlineStatus(time.Now().Add(30*time.Minute)))
}

func TestCalculateDeadlineStatus_Warning(t *testing.T) {
	assert.Equal(t, "warning", CalculateDeadlineStatus(time.Now().Add(2*time.Hour)))
}

func TestCalculateDeadlineStatus_Normal(t *testing.T) {
	assert.Equal(t, "normal", CalculateDeadlineStatus(time.Now().Add(4*time.Hour)))
}

func TestCalculateDeadlineStatus_Overdue(t *testing.T) {
	// Past deadline: hours remaining is negative, < 1 → critical
	assert.Equal(t, "critical", CalculateDeadlineStatus(time.Now().Add(-1*time.Hour)))
}

func TestCalculateDeadlineStatus_BoundaryWarningToNormal(t *testing.T) {
	// Just under 3h → warning; just over 3h → normal
	assert.Equal(t, "warning", CalculateDeadlineStatus(time.Now().Add(179*time.Minute)))
	assert.Equal(t, "normal", CalculateDeadlineStatus(time.Now().Add(181*time.Minute)))
}

func TestIsGameUrgent_NilDeadline(t *testing.T) {
	assert.False(t, IsGameUrgent(true, nil))
}

func TestIsGameUrgent_PendingActionUnder3h(t *testing.T) {
	deadline := time.Now().Add(2 * time.Hour)
	assert.True(t, IsGameUrgent(true, &deadline))
}

func TestIsGameUrgent_PendingActionOver3h(t *testing.T) {
	// 19h should NOT be urgent with new thresholds
	deadline := time.Now().Add(19 * time.Hour)
	assert.False(t, IsGameUrgent(true, &deadline))
}

func TestIsGameUrgent_NoPendingAction(t *testing.T) {
	deadline := time.Now().Add(2 * time.Hour)
	assert.False(t, IsGameUrgent(false, &deadline))
}

func TestIsGameUrgent_DeadlineFarOut(t *testing.T) {
	deadline := time.Now().Add(48 * time.Hour)
	assert.False(t, IsGameUrgent(true, &deadline))
}

func TestTruncateContent_ShortContent(t *testing.T) {
	assert.Equal(t, "hello", TruncateContent("hello", 100))
}

func TestTruncateContent_TruncatesAtWordBoundary(t *testing.T) {
	result := TruncateContent("hello world foo bar", 11)
	assert.True(t, strings.HasSuffix(result, "..."))
	assert.True(t, strings.HasPrefix(result, "hello"))
}

// --- config.go ---

func TestGetEnvBool_TrueVariants(t *testing.T) {
	for _, val := range []string{"true", "1", "yes", "on"} {
		t.Setenv("TEST_BOOL_VAR", val)
		assert.True(t, getEnvBool("TEST_BOOL_VAR", false), "expected true for %q", val)
	}
}

func TestGetEnvBool_FalseVariants(t *testing.T) {
	for _, val := range []string{"false", "0", "no", "off"} {
		t.Setenv("TEST_BOOL_VAR", val)
		assert.False(t, getEnvBool("TEST_BOOL_VAR", true), "expected false for %q", val)
	}
}

func TestGetEnvBool_Default(t *testing.T) {
	t.Setenv("TEST_BOOL_VAR", "")
	assert.True(t, getEnvBool("TEST_BOOL_VAR", true))
	assert.False(t, getEnvBool("TEST_BOOL_VAR", false))
}

func TestGetEnvInt_Valid(t *testing.T) {
	t.Setenv("TEST_INT_VAR", "42")
	assert.Equal(t, 42, getEnvInt("TEST_INT_VAR", 0))
}

func TestGetEnvInt_Invalid(t *testing.T) {
	t.Setenv("TEST_INT_VAR", "notanint")
	assert.Equal(t, 99, getEnvInt("TEST_INT_VAR", 99))
}

func TestGetEnvInt_Default(t *testing.T) {
	t.Setenv("TEST_INT_VAR", "")
	assert.Equal(t, 5, getEnvInt("TEST_INT_VAR", 5))
}

func TestGetEnvDuration_Valid(t *testing.T) {
	t.Setenv("TEST_DUR_VAR", "2h30m")
	assert.Equal(t, 2*time.Hour+30*time.Minute, getEnvDuration("TEST_DUR_VAR", time.Second))
}

func TestGetEnvDuration_Invalid(t *testing.T) {
	t.Setenv("TEST_DUR_VAR", "notaduration")
	assert.Equal(t, time.Minute, getEnvDuration("TEST_DUR_VAR", time.Minute))
}

func TestGetEnvDuration_Default(t *testing.T) {
	t.Setenv("TEST_DUR_VAR", "")
	assert.Equal(t, 30*time.Second, getEnvDuration("TEST_DUR_VAR", 30*time.Second))
}

// --- users.go ---

func TestHashPassword_AndCheckPasswordHash(t *testing.T) {
	u := &User{Password: "my-secret-password"}
	require.NoError(t, u.HashPassword())
	assert.NotEqual(t, "my-secret-password", u.Password, "password should be hashed")
	assert.True(t, u.CheckPasswordHash("my-secret-password"))
	assert.False(t, u.CheckPasswordHash("wrong-password"))
}

func TestCheckPasswordHash_WrongPassword(t *testing.T) {
	u := &User{Password: "correct"}
	require.NoError(t, u.HashPassword())
	assert.False(t, u.CheckPasswordHash("incorrect"))
}

// Verify mockUserService satisfies the MiddlewareUserService interface at compile time.
var _ MiddlewareUserService = (*mockUserService)(nil)

func TestMockUserService_ReturnsUser(t *testing.T) {
	svc := &mockUserService{user: &User{ID: 1, Username: "alice"}}
	u, err := svc.GetUserByID(1)
	require.NoError(t, err)
	assert.Equal(t, "alice", u.Username)
}

func TestMockUserService_ReturnsError(t *testing.T) {
	svc := &mockUserService{err: errors.New("not found")}
	u, err := svc.GetUserByID(999)
	assert.Nil(t, u)
	assert.Error(t, err)
}

// --- constants.go ---

func TestIsValidParticipantRole(t *testing.T) {
	for _, r := range ValidParticipantRoles {
		assert.True(t, IsValidParticipantRole(r), "expected %q to be valid", r)
	}
	assert.False(t, IsValidParticipantRole(""), "empty string should be invalid")
	assert.False(t, IsValidParticipantRole("admin"), "unknown role should be invalid")
	assert.False(t, IsValidParticipantRole("GM"), "case-sensitive: uppercase should be invalid")
}

func TestIsValidApplicationStatus(t *testing.T) {
	for _, s := range ValidApplicationStatuses {
		assert.True(t, IsValidApplicationStatus(s), "expected %q to be valid", s)
	}
	assert.False(t, IsValidApplicationStatus(""), "empty string should be invalid")
	assert.False(t, IsValidApplicationStatus("withdrawn"), "unknown status should be invalid")
}

func TestIsValidNotificationType(t *testing.T) {
	for _, typ := range ValidNotificationTypes {
		assert.True(t, IsValidNotificationType(typ), "expected %q to be valid", typ)
	}
	assert.False(t, IsValidNotificationType(""), "empty string should be invalid")
	assert.False(t, IsValidNotificationType("unknown_event"), "unknown type should be invalid")
	assert.False(t, IsValidNotificationType("ACTION_SUBMITTED"), "case-sensitive: uppercase should be invalid")
}

// --- notifications.go ---

func TestCreateNotificationRequest_Validate(t *testing.T) {
	str := func(s string) *string { return &s }

	t.Run("valid request passes", func(t *testing.T) {
		req := &CreateNotificationRequest{
			UserID: 1,
			Type:   NotificationTypeActionResult,
			Title:  "Your action result is ready",
		}
		assert.NoError(t, req.Validate())
	})

	t.Run("invalid notification type fails", func(t *testing.T) {
		req := &CreateNotificationRequest{
			UserID: 1,
			Type:   "not_a_real_type",
			Title:  "Some title",
		}
		assert.Error(t, req.Validate(), "unknown notification type should fail validation")
	})

	t.Run("empty title fails", func(t *testing.T) {
		req := &CreateNotificationRequest{
			UserID: 1,
			Type:   NotificationTypeActionResult,
			Title:  "",
		}
		assert.Error(t, req.Validate(), "empty title should fail validation")
	})

	t.Run("title too long fails", func(t *testing.T) {
		req := &CreateNotificationRequest{
			UserID: 1,
			Type:   NotificationTypeActionResult,
			Title:  string(make([]byte, 256)), // 256 bytes > max 255
		}
		assert.Error(t, req.Validate(), "title exceeding 255 chars should fail")
	})

	t.Run("link URL too long fails", func(t *testing.T) {
		req := &CreateNotificationRequest{
			UserID:  1,
			Type:    NotificationTypeActionResult,
			Title:   "OK",
			LinkURL: str(string(make([]byte, 501))), // 501 > max 500
		}
		assert.Error(t, req.Validate(), "link URL exceeding 500 chars should fail")
	})
}

// --- email_verification_middleware.go ---

// TestRequireVerifiedEmailCtx_Disabled covers the development escape hatch.
//
// Repointed from RequireEmailVerificationMiddleware, which was deleted with the
// huma migration: handlers take a context now, so the middleware had no callers
// left. The Ctx twin reads the same environment variable, and this is the only
// direct test of that branch.
func TestRequireVerifiedEmailCtx_Disabled(t *testing.T) {
	t.Setenv("REQUIRE_EMAIL_VERIFICATION", "false")

	// A nil pool is safe here precisely because the flag short-circuits before
	// any lookup — if that ever stops being true, this panics rather than
	// silently passing.
	if errResp := RequireVerifiedEmailCtx(context.Background(), nil); errResp != nil {
		t.Fatalf("expected nil when verification is disabled, got %#v", errResp)
	}
}
