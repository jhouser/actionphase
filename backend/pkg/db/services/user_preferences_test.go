package db

import (
	"context"
	"testing"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserPreferencesService_GetUserPreferences_Default(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "user_preferences", "users")

	service := NewUserPreferencesService(testDB.Pool)
	ctx := context.Background()

	// Test getting preferences for user that has never set any
	prefs, err := service.GetUserPreferences(ctx, 99999) // Non-existent user ID
	require.NoError(t, err)
	assert.Equal(t, "auto", prefs.Theme, "should return default theme")
}

func TestUserPreferencesService_UpdateUserPreferences_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "user_preferences", "users")

	service := NewUserPreferencesService(testDB.Pool)
	ctx := context.Background()

	// Create a test user
	queries := models.New(testDB.Pool)
	user, err := queries.CreateUser(ctx, models.CreateUserParams{
		Username: "prefs_test_user",
		Email:    "prefs@example.com",
		Password: "testpass",
	})
	require.NoError(t, err)

	// Update preferences (should create new record)
	prefs := PreferencesData{Theme: "dark"}
	updated, err := service.UpdateUserPreferences(ctx, user.ID, prefs)
	require.NoError(t, err)
	assert.Equal(t, "dark", updated.Theme)

	// Verify preferences were saved
	retrieved, err := service.GetUserPreferences(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "dark", retrieved.Theme)
}

func TestUserPreferencesService_UpdateUserPreferences_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "user_preferences", "users")

	service := NewUserPreferencesService(testDB.Pool)
	ctx := context.Background()

	// Create a test user
	queries := models.New(testDB.Pool)
	user, err := queries.CreateUser(ctx, models.CreateUserParams{
		Username: "prefs_update_user",
		Email:    "prefs_update@example.com",
		Password: "testpass",
	})
	require.NoError(t, err)

	// Set initial preferences
	_, err = service.UpdateUserPreferences(ctx, user.ID, PreferencesData{Theme: "light"})
	require.NoError(t, err)

	// Update to dark mode
	updated, err := service.UpdateUserPreferences(ctx, user.ID, PreferencesData{Theme: "dark"})
	require.NoError(t, err)
	assert.Equal(t, "dark", updated.Theme)

	// Verify update persisted
	retrieved, err := service.GetUserPreferences(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "dark", retrieved.Theme)
}

func TestUserPreferencesService_UpdateUserPreferences_InvalidTheme(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "user_preferences", "users")

	service := NewUserPreferencesService(testDB.Pool)
	ctx := context.Background()

	// Test validation for invalid theme values
	invalidThemes := []string{"invalid", "DARK", "Light", "system"}

	for _, theme := range invalidThemes {
		t.Run(theme, func(t *testing.T) {
			prefs := PreferencesData{Theme: theme}
			_, err := service.UpdateUserPreferences(ctx, 1, prefs)
			assert.Error(t, err, "should reject invalid theme: %s", theme)
			assert.Contains(t, err.Error(), "invalid theme value", "error should mention invalid theme")
		})
	}
}

func TestUserPreferencesService_UpdateUserPreferences_ValidThemes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "user_preferences", "users")

	service := NewUserPreferencesService(testDB.Pool)
	ctx := context.Background()

	// Create a test user
	queries := models.New(testDB.Pool)
	user, err := queries.CreateUser(ctx, models.CreateUserParams{
		Username: "prefs_valid_user",
		Email:    "prefs_valid@example.com",
		Password: "testpass",
	})
	require.NoError(t, err)

	// Test all valid theme values
	validThemes := []string{"light", "dark", "highContrast", "highContrastDark", "colorblind", "auto"}

	for _, theme := range validThemes {
		t.Run(theme, func(t *testing.T) {
			prefs := PreferencesData{Theme: theme, CommentReadMode: "auto"}
			updated, err := service.UpdateUserPreferences(ctx, user.ID, prefs)
			require.NoError(t, err, "should accept valid theme: %s", theme)
			assert.Equal(t, theme, updated.Theme)

			// Verify it persisted
			retrieved, err := service.GetUserPreferences(ctx, user.ID)
			require.NoError(t, err)
			assert.Equal(t, theme, retrieved.Theme)
		})
	}
}

func TestUserPreferencesService_CommentReadMode_Default(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "user_preferences", "users")

	service := NewUserPreferencesService(testDB.Pool)
	ctx := context.Background()

	queries := models.New(testDB.Pool)
	user, err := queries.CreateUser(ctx, models.CreateUserParams{
		Username: "readmode_default_user",
		Email:    "readmode_default@example.com",
		Password: "testpass",
	})
	require.NoError(t, err)

	// Preferences never set — should default to "manual"
	prefs, err := service.GetUserPreferences(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "manual", prefs.CommentReadMode, "default comment_read_mode should be 'manual'")
}

func TestUserPreferencesService_CommentReadMode_ValidValues(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "user_preferences", "users")

	service := NewUserPreferencesService(testDB.Pool)
	ctx := context.Background()

	queries := models.New(testDB.Pool)
	user, err := queries.CreateUser(ctx, models.CreateUserParams{
		Username: "readmode_valid_user",
		Email:    "readmode_valid@example.com",
		Password: "testpass",
	})
	require.NoError(t, err)

	for _, mode := range []string{"auto", "manual"} {
		t.Run(mode, func(t *testing.T) {
			prefs := PreferencesData{Theme: "auto", CommentReadMode: mode}
			updated, err := service.UpdateUserPreferences(ctx, user.ID, prefs)
			require.NoError(t, err)
			assert.Equal(t, mode, updated.CommentReadMode)
		})
	}
}

func TestUserPreferencesService_CommentReadMode_InvalidValue(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "user_preferences", "users")

	service := NewUserPreferencesService(testDB.Pool)
	ctx := context.Background()

	for _, mode := range []string{"none", "Auto", "MANUAL", "highlight"} {
		t.Run(mode, func(t *testing.T) {
			prefs := PreferencesData{Theme: "auto", CommentReadMode: mode}
			_, err := service.UpdateUserPreferences(ctx, 1, prefs)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid comment_read_mode")
		})
	}
}

func TestUserPreferencesService_FontSize_Default(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "user_preferences", "users")

	service := NewUserPreferencesService(testDB.Pool)
	ctx := context.Background()

	queries := models.New(testDB.Pool)
	user, err := queries.CreateUser(ctx, models.CreateUserParams{
		Username: "fontsize_default_user",
		Email:    "fontsize_default@example.com",
		Password: "testpass",
	})
	require.NoError(t, err)

	// Preferences never set — should default to "medium"
	prefs, err := service.GetUserPreferences(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "medium", prefs.FontSize, "default font_size should be 'medium'")
}

func TestUserPreferencesService_FontSize_ValidValues(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "user_preferences", "users")

	service := NewUserPreferencesService(testDB.Pool)
	ctx := context.Background()

	queries := models.New(testDB.Pool)
	user, err := queries.CreateUser(ctx, models.CreateUserParams{
		Username: "fontsize_valid_user",
		Email:    "fontsize_valid@example.com",
		Password: "testpass",
	})
	require.NoError(t, err)

	for _, size := range []string{"small", "medium", "large"} {
		t.Run(size, func(t *testing.T) {
			prefs := PreferencesData{Theme: "auto", CommentReadMode: "manual", FontSize: size}
			updated, err := service.UpdateUserPreferences(ctx, user.ID, prefs)
			require.NoError(t, err, "should accept valid font_size: %s", size)
			assert.Equal(t, size, updated.FontSize)

			// Verify it persisted
			retrieved, err := service.GetUserPreferences(ctx, user.ID)
			require.NoError(t, err)
			assert.Equal(t, size, retrieved.FontSize)
		})
	}
}

func TestUserPreferencesService_FontSize_InvalidValue(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "user_preferences", "users")

	service := NewUserPreferencesService(testDB.Pool)
	ctx := context.Background()

	for _, size := range []string{"huge", "Small", "MEDIUM", "extra-large"} {
		t.Run(size, func(t *testing.T) {
			prefs := PreferencesData{Theme: "auto", CommentReadMode: "manual", FontSize: size}
			_, err := service.UpdateUserPreferences(ctx, 1, prefs)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid font_size")
		})
	}
}

func TestUserPreferencesService_UpdateUserPreferences_PreservesDiscordNotificationsWhenOmitted(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	testDB := core.NewTestDatabase(t)
	defer testDB.Close()
	defer testDB.CleanupTables(t, "user_preferences", "users")

	service := NewUserPreferencesService(testDB.Pool)
	ctx := context.Background()

	queries := models.New(testDB.Pool)
	user, err := queries.CreateUser(ctx, models.CreateUserParams{
		Username: "prefs_merge_user",
		Email:    "prefs_merge@example.com",
		Password: "testpass",
	})
	require.NoError(t, err)

	// Save Discord notification preferences.
	_, err = service.UpdateUserPreferences(ctx, user.ID, PreferencesData{
		Theme:                "auto",
		CommentReadMode:      "manual",
		FontSize:             "medium",
		DiscordNotifications: map[string]bool{"private_message": true},
	})
	require.NoError(t, err)

	// Later, change only the font size (as SettingsPage does) without
	// resending discord_notifications — it must not be wiped out.
	updated, err := service.UpdateUserPreferences(ctx, user.ID, PreferencesData{
		Theme:           "auto",
		CommentReadMode: "manual",
		FontSize:        "large",
	})
	require.NoError(t, err)
	assert.Equal(t, "large", updated.FontSize)
	assert.Equal(t, map[string]bool{"private_message": true}, updated.DiscordNotifications,
		"discord_notifications should be preserved when the update payload omits it")

	retrieved, err := service.GetUserPreferences(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"private_message": true}, retrieved.DiscordNotifications,
		"discord_notifications should still be persisted after an unrelated preference update")
}
