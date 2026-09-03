package games

import (
	"net/http"
	"time"

	"actionphase/pkg/core"
)

// GameResponse represents a basic game response
type GameResponse struct {
	ID                      int32      `json:"id"`
	Title                   string     `json:"title"`
	Description             string     `json:"description"`
	GMUserID                int32      `json:"gm_user_id"`
	State                   string     `json:"state"`
	Genre                   string     `json:"genre,omitempty"`
	StartDate               *time.Time `json:"start_date,omitempty"`
	EndDate                 *time.Time `json:"end_date,omitempty"`
	RecruitmentDeadline     *time.Time `json:"recruitment_deadline,omitempty"`
	MaxPlayers              int32      `json:"max_players,omitempty"`
	IsAnonymous             bool       `json:"is_anonymous"`
	AutoAcceptAudience      bool       `json:"auto_accept_audience"`
	AllowGroupConversations bool       `json:"allow_group_conversations"`
	PortraitAvatars         bool       `json:"portrait_avatars"`
	BannerURL               *string    `json:"banner_url,omitempty"`
	CommonRoomOpenDay       *int16     `json:"common_room_open_day,omitempty"`
	CommonRoomOpenTime      *string    `json:"common_room_open_time,omitempty"`
	CommonRoomCloseDay      *int16     `json:"common_room_close_day,omitempty"`
	CommonRoomCloseTime     *string    `json:"common_room_close_time,omitempty"`
	ScheduleTimezone        *string    `json:"schedule_timezone,omitempty"`
	// A POINTER because legacy games genuinely have no community (req 5). A
	// zero int32 would render as community 0, which no client can distinguish
	// from "unset" -- absent is the honest encoding.
	CommunityID *int32 `json:"community_id,omitempty"`
	// As stored: sparse, containing only genuine GM overrides. Defaults are NOT
	// filled in here — the frontend owns them, so exactly one place knows them.
	CharacterSheet *core.CharacterSheetConfig `json:"character_sheet,omitempty"`
	CreatedAt      time.Time                  `json:"created_at"`
	UpdatedAt      time.Time                  `json:"updated_at"`
}

func (rd *GameResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

// GameWithDetailsResponse represents a game response with additional details
type GameWithDetailsResponse struct {
	ID                      int32      `json:"id"`
	Title                   string     `json:"title"`
	Description             string     `json:"description"`
	GMUserID                int32      `json:"gm_user_id"`
	GMUsername              string     `json:"gm_username,omitempty"`
	State                   string     `json:"state"`
	Genre                   string     `json:"genre,omitempty"`
	StartDate               *time.Time `json:"start_date,omitempty"`
	EndDate                 *time.Time `json:"end_date,omitempty"`
	RecruitmentDeadline     *time.Time `json:"recruitment_deadline,omitempty"`
	MaxPlayers              int32      `json:"max_players,omitempty"`
	IsAnonymous             bool       `json:"is_anonymous"`
	AutoAcceptAudience      bool       `json:"auto_accept_audience"`
	AllowGroupConversations bool       `json:"allow_group_conversations"`
	PortraitAvatars         bool       `json:"portrait_avatars"`
	BannerURL               *string    `json:"banner_url,omitempty"`
	// Same pointer semantics as GameResponse.CommunityID: nil means the game
	// predates communities (req 5), never community 0.
	//
	// This endpoint -- not GET /games/{id} -- is what the game page loads, so
	// the edit form hydrates its community picker from here. Omitting the
	// field made the picker fall back to "-- Choose a community --" on games
	// that plainly had one.
	CommunityID *int32 `json:"community_id,omitempty"`
	// Name and slug of the owning community, joined alongside CommunityID so a
	// game surface can label and link it without a second request. Both nil for
	// a legacy game, exactly like CommunityID.
	//
	// The Info tab's community section needs these even when the community has
	// published NO documents -- naming the community is not conditional on it
	// having written anything.
	CommunityName       *string `json:"community_name,omitempty"`
	CommunitySlug       *string `json:"community_slug,omitempty"`
	CommonRoomOpenDay   *int16  `json:"common_room_open_day,omitempty"`
	CommonRoomOpenTime  *string `json:"common_room_open_time,omitempty"`
	CommonRoomCloseDay  *int16  `json:"common_room_close_day,omitempty"`
	CommonRoomCloseTime *string `json:"common_room_close_time,omitempty"`
	ScheduleTimezone    *string `json:"schedule_timezone,omitempty"`
	// As stored: sparse, containing only genuine GM overrides. See GameResponse.
	CharacterSheet *core.CharacterSheetConfig `json:"character_sheet,omitempty"`
	CurrentPlayers int64                      `json:"current_players"`
	CreatedAt      time.Time                  `json:"created_at"`
	UpdatedAt      time.Time                  `json:"updated_at"`
}

func (rd *GameWithDetailsResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

// GameApplicationResponse represents a game application
type GameApplicationResponse struct {
	ID               int32      `json:"id"`
	GameID           int32      `json:"game_id"`
	UserID           int32      `json:"user_id"`
	Username         string     `json:"username,omitempty"`
	Email            string     `json:"email,omitempty"`
	Role             string     `json:"role"`
	Message          string     `json:"message,omitempty"`
	Status           string     `json:"status"`
	AppliedAt        time.Time  `json:"applied_at"`
	ReviewedAt       *time.Time `json:"reviewed_at,omitempty"`
	ReviewedByUserID *int32     `json:"reviewed_by_user_id,omitempty"`
}

func (rd *GameApplicationResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

// EnrichedGameListItemResponse represents an enriched game list item with user context
type EnrichedGameListItemResponse struct {
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
	UserRelationship        *string    `json:"user_relationship,omitempty"`
	CurrentPhaseType        *string    `json:"current_phase_type,omitempty"`
	CurrentPhaseDeadline    *time.Time `json:"current_phase_deadline,omitempty"`
	DeadlineUrgency         string     `json:"deadline_urgency"`
	HasRecentActivity       bool       `json:"has_recent_activity"`
}

// GameListingMetadataResponse represents metadata about the game listing
type GameListingMetadataResponse struct {
	TotalCount      int      `json:"total_count"`
	FilteredCount   int      `json:"filtered_count"`
	AvailableStates []string `json:"available_states"`
	Page            int      `json:"page"`
	PageSize        int      `json:"page_size"`
	TotalPages      int      `json:"total_pages"`
	HasNextPage     bool     `json:"has_next_page"`
	HasPreviousPage bool     `json:"has_previous_page"`
}

// GameListingResponse represents the full game listing response
type GameListingResponse struct {
	Games    []*EnrichedGameListItemResponse `json:"games"`
	Metadata GameListingMetadataResponse     `json:"metadata"`
}

func (rd *GameListingResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

// Audience response types.
//
// Moved here from api_audience.go when that file's handlers were converted;
// they are response shapes like everything else in this file.

type AudienceMemberResponse struct {
	ID       int32     `json:"id"`
	GameID   int32     `json:"game_id"`
	UserID   int32     `json:"user_id"`
	Username string    `json:"username"`
	Role     string    `json:"role"`
	Status   string    `json:"status"`
	JoinedAt time.Time `json:"joined_at"`
}

type ListAudienceMembersResponse struct {
	AudienceMembers []AudienceMemberResponse `json:"audience_members"`
}

type PrivateConversationResponse struct {
	ConversationID          int32       `json:"conversation_id"`
	Subject                 *string     `json:"subject"`
	ConversationType        string      `json:"conversation_type"`
	CreatedAt               string      `json:"created_at"`
	MessageCount            int64       `json:"message_count"`
	LastMessageAt           interface{} `json:"last_message_at"`
	ParticipantNames        interface{} `json:"participant_names"`
	ParticipantUsernames    interface{} `json:"participant_usernames"`
	ParticipantCharacterIDs interface{} `json:"participant_character_ids"`
	LastMessageContent      *string     `json:"last_message_content"`
	LastSenderName          *string     `json:"last_sender_name"`
	LastSenderUsername      *string     `json:"last_sender_username"`
	LastSenderCharacterID   *int32      `json:"last_sender_character_id"`
}

type ActionSubmissionResponse struct {
	ID            int32   `json:"id"`
	GameID        int32   `json:"game_id"`
	UserID        int32   `json:"user_id"`
	PhaseID       int32   `json:"phase_id"`
	CharacterID   *int32  `json:"character_id"`
	Content       string  `json:"content"`
	SubmittedAt   *string `json:"submitted_at"`
	UpdatedAt     *string `json:"updated_at"`
	Username      string  `json:"username"`
	CharacterName *string `json:"character_name"`
	PhaseType     string  `json:"phase_type"`
	PhaseNumber   int32   `json:"phase_number"`
	PhaseTitle    string  `json:"phase_title"`
}

type AudienceMessageResponse struct {
	ID                  int32   `json:"id"`
	ConversationID      int32   `json:"conversation_id"`
	SenderUserID        *int32  `json:"sender_user_id"`
	SenderCharacterID   *int32  `json:"sender_character_id"`
	Content             string  `json:"content"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
	IsDeleted           bool    `json:"is_deleted"`
	SenderUsername      string  `json:"sender_username"`
	SenderCharacterName *string `json:"sender_character_name"`
}
