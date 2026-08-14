// Package core provides game-related domain models
package core

import "time"

// GameListingFilters represents filter criteria for game listing
type GameListingFilters struct {
	UserID              *int32   // For participation enrichment (nullable)
	Search              string   // Search text for title/description (case-insensitive)
	States              []string // Filter by game states
	ParticipationFilter *string  // 'my_games', 'applied', 'not_joined'
	HasOpenSpots        *bool    // Only games with available player spots
	SortBy              string   // 'recent_activity', 'created', 'start_date', 'alphabetical'
	AdminMode           bool     // Admin mode: bypasses is_public filter when user is admin
	AdminUserID         *int32   // User ID requesting admin mode (for validation)
	Page                int      // Page number (1-indexed, default: 1)
	PageSize            int      // Number of items per page (default: 20, max: 100)
}

// EnrichedGameListItem extends GameListItem with user context and urgency
type EnrichedGameListItem struct {
	// Base game fields
	ID                      int32      `json:"id"`
	Title                   string     `json:"title"`
	Description             string     `json:"description"`
	GMUserID                int32      `json:"gm_user_id"`
	GMUsername              string     `json:"gm_username"`
	State                   string     `json:"state"`
	Genre                   *string    `json:"genre,omitempty"`
	StartDate               *time.Time `json:"start_date,omitempty"`
	EndDate                 *time.Time `json:"end_date,omitempty"`
	RecruitmentDeadline     *time.Time `json:"recruitment_deadline,omitempty"`
	MaxPlayers              *int32     `json:"max_players,omitempty"`
	IsPublic                bool       `json:"is_public"`
	IsAnonymous             bool       `json:"is_anonymous"`
	AutoAcceptAudience      bool       `json:"auto_accept_audience"`
	AllowGroupConversations bool       `json:"allow_group_conversations"`
	PortraitAvatars         bool       `json:"portrait_avatars"`
	BannerURL               *string    `json:"banner_url,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
	CurrentPlayers          int32      `json:"current_players"`

	// Enrichment fields
	UserRelationship     *string    `json:"user_relationship,omitempty"` // 'gm', 'participant', 'applied', 'none'
	CurrentPhaseType     *string    `json:"current_phase_type,omitempty"`
	CurrentPhaseDeadline *time.Time `json:"current_phase_deadline,omitempty"`
	DeadlineUrgency      string     `json:"deadline_urgency"` // 'critical', 'warning', 'normal'
	HasRecentActivity    bool       `json:"has_recent_activity"`
}

// GameListingMetadata provides context for the listing
type GameListingMetadata struct {
	TotalCount      int      `json:"total_count"`       // Total count of all public games
	FilteredCount   int      `json:"filtered_count"`    // Count of games matching filters
	AvailableStates []string `json:"available_states"`  // Game states with at least one game
	Page            int      `json:"page"`              // Current page number (1-indexed)
	PageSize        int      `json:"page_size"`         // Number of items per page
	TotalPages      int      `json:"total_pages"`       // Total number of pages
	HasNextPage     bool     `json:"has_next_page"`     // Whether there's a next page
	HasPreviousPage bool     `json:"has_previous_page"` // Whether there's a previous page
}

// GameListingResponse is the full response for listing endpoint
type GameListingResponse struct {
	Games    []*EnrichedGameListItem `json:"games"`
	Metadata GameListingMetadata     `json:"metadata"`
}

// LootTableItem is a single item in a loot table, as accepted by the service
// layer. Data is the GM-authored JSON blob describing the item; it is stored
// verbatim and validated at the HTTP boundary.
type LootTableItem struct {
	Name string
	Data string
}
