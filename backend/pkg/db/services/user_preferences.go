package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"actionphase/pkg/core"
	models "actionphase/pkg/db/models"
)

var _ core.UserPreferencesServiceInterface = (*UserPreferencesService)(nil)

// UserPreferencesService handles user preferences operations
type UserPreferencesService struct {
	DB      *pgxpool.Pool
	Queries *models.Queries
}

// NewUserPreferencesService creates a new user preferences service
func NewUserPreferencesService(db *pgxpool.Pool) *UserPreferencesService {
	return &UserPreferencesService{
		DB:      db,
		Queries: models.New(db),
	}
}

// PreferencesData is an alias kept for callers that used the old db-package type.
type PreferencesData = core.PreferencesData

// enumPreference describes a string preference field restricted to a fixed
// set of allowed values, with a default applied when unset.
type enumPreference struct {
	name    string
	allowed []string
	def     string
}

func (p enumPreference) applyDefault(value string) string {
	if value == "" {
		return p.def
	}
	return value
}

func (p enumPreference) validate(value string) error {
	for _, allowed := range p.allowed {
		if value == allowed {
			return nil
		}
	}
	return fmt.Errorf("invalid %s value: must be one of %s", p.name, strings.Join(quoteAll(p.allowed), ", "))
}

func quoteAll(values []string) []string {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = "'" + v + "'"
	}
	return quoted
}

var (
	themePreference = enumPreference{
		name:    "theme",
		allowed: []string{"light", "dark", "highContrast", "highContrastDark", "colorblind", "auto"},
		def:     "auto",
	}

	commentReadModePreference = enumPreference{name: "comment_read_mode", allowed: []string{"auto", "manual"}, def: "manual"}

	fontSizePreference = enumPreference{name: "font_size", allowed: []string{"small", "medium", "large"}, def: "medium"}
)

func applyPreferenceDefaults(data *PreferencesData) {
	data.Theme = themePreference.applyDefault(data.Theme)
	data.CommentReadMode = commentReadModePreference.applyDefault(data.CommentReadMode)
	data.FontSize = fontSizePreference.applyDefault(data.FontSize)
}

// GetUserPreferences gets user preferences, returning defaults if not found
func (s *UserPreferencesService) GetUserPreferences(ctx context.Context, userID int32) (*PreferencesData, error) {
	prefs, err := s.Queries.GetUserPreferences(ctx, userID)
	if err != nil {
		// Return default preferences if not found
		data := &PreferencesData{}
		applyPreferenceDefaults(data)
		return data, nil
	}

	// Parse JSONB into PreferencesData
	var data PreferencesData
	if err := json.Unmarshal(prefs.Preferences, &data); err != nil {
		return nil, fmt.Errorf("failed to parse preferences: %w", err)
	}

	// Apply defaults for any missing fields
	applyPreferenceDefaults(&data)

	return &data, nil
}

// UpdateUserPreferences merges the given fields onto the user's existing
// stored preferences and updates or creates the record. Fields left at their
// zero value in prefs (e.g. an unset DiscordNotifications map) do not
// overwrite previously saved values for those fields.
func (s *UserPreferencesService) UpdateUserPreferences(ctx context.Context, userID int32, prefs PreferencesData) (*PreferencesData, error) {
	existing, err := s.GetUserPreferences(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to load existing preferences: %w", err)
	}
	if prefs.DiscordNotifications == nil {
		prefs.DiscordNotifications = existing.DiscordNotifications
	}

	// Apply defaults before validation so callers only need to specify fields they care about
	applyPreferenceDefaults(&prefs)

	if err := themePreference.validate(prefs.Theme); err != nil {
		return nil, err
	}
	if err := commentReadModePreference.validate(prefs.CommentReadMode); err != nil {
		return nil, err
	}
	if err := fontSizePreference.validate(prefs.FontSize); err != nil {
		return nil, err
	}

	// Validate discord_notifications keys (if provided)
	if prefs.DiscordNotifications != nil {
		for k := range prefs.DiscordNotifications {
			if !core.IsValidNotificationType(k) {
				return nil, fmt.Errorf("invalid discord notification type: %q", k)
			}
		}
	}

	// Marshal preferences to JSONB
	jsonData, err := json.Marshal(prefs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal preferences: %w", err)
	}

	// Upsert preferences
	_, err = s.Queries.UpsertUserPreferences(ctx, models.UpsertUserPreferencesParams{
		UserID:      userID,
		Preferences: jsonData,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upsert preferences: %w", err)
	}

	return &prefs, nil
}
