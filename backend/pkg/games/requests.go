package games

// Request payload shapes, retained for the package's tests.
//
// The handlers no longer read these -- huma binds its own body types (see
// huma_api.go), which carry the tags that are actually enforced. These remain
// because the tests marshal them to build request bodies, and they are
// serialization-identical to what the huma types accept.
//
// The `validate:` tags are deliberately gone. Unlike most packages, they really
// did run here, via Bind calling core.ValidateStruct -- but nothing calls Bind
// any more, so leaving them would suggest a guarantee that no longer holds. The
// same limits now live on the huma body structs, where huma enforces them.

import (
	"errors"
	"fmt"
	"time"

	"actionphase/pkg/core"
)

// CreateGameRequest represents the request to create a new game
type CreateGameRequest struct {
	Title                   string              `json:"title"`
	Description             string              `json:"description"`
	Genre                   string              `json:"genre,omitempty"`
	StartDate               *core.LocalDateTime `json:"start_date,omitempty"`
	EndDate                 *core.LocalDateTime `json:"end_date,omitempty"`
	RecruitmentDeadline     *core.LocalDateTime `json:"recruitment_deadline,omitempty"`
	MaxPlayers              int32               `json:"max_players,omitempty"`
	IsAnonymous             bool                `json:"is_anonymous"`
	AutoAcceptAudience      bool                `json:"auto_accept_audience"`
	AllowGroupConversations bool                `json:"allow_group_conversations"`
	PortraitAvatars         bool                `json:"portrait_avatars"`
	BannerURL               *string             `json:"banner_url,omitempty"`
	CommonRoomOpenDay       *int16              `json:"common_room_open_day,omitempty"`
	CommonRoomOpenTime      *string             `json:"common_room_open_time,omitempty"`
	CommonRoomCloseDay      *int16              `json:"common_room_close_day,omitempty"`
	CommonRoomCloseTime     *string             `json:"common_room_close_time,omitempty"`
	ScheduleTimezone        *string             `json:"schedule_timezone,omitempty"`
	// Typed here, unlike the chi version's json.RawMessage: that was a
	// workaround for render.Bind's permissive decoder silently dropping unknown
	// keys, and huma rejects them natively.
	CharacterSheet *core.CharacterSheetConfig `json:"character_sheet,omitempty"`
}

// UpdateGameStateRequest represents the request to update a game's state
type UpdateGameStateRequest struct {
	State string `json:"state"`
}

// UpdateGameRequest represents the request to update game details
type UpdateGameRequest struct {
	Title                   string                     `json:"title"`
	Description             string                     `json:"description"`
	Genre                   string                     `json:"genre,omitempty"`
	StartDate               *time.Time                 `json:"start_date,omitempty"`
	EndDate                 *time.Time                 `json:"end_date,omitempty"`
	RecruitmentDeadline     *time.Time                 `json:"recruitment_deadline,omitempty"`
	MaxPlayers              int32                      `json:"max_players,omitempty"`
	IsPublic                bool                       `json:"is_public"`
	IsAnonymous             bool                       `json:"is_anonymous"`
	AutoAcceptAudience      bool                       `json:"auto_accept_audience"`
	AllowGroupConversations bool                       `json:"allow_group_conversations"`
	PortraitAvatars         bool                       `json:"portrait_avatars"`
	BannerURL               *string                    `json:"banner_url,omitempty"`
	CommonRoomOpenDay       *int16                     `json:"common_room_open_day,omitempty"`
	CommonRoomOpenTime      *string                    `json:"common_room_open_time,omitempty"`
	CommonRoomCloseDay      *int16                     `json:"common_room_close_day,omitempty"`
	CommonRoomCloseTime     *string                    `json:"common_room_close_time,omitempty"`
	ScheduleTimezone        *string                    `json:"schedule_timezone,omitempty"`
	CharacterSheet          *core.CharacterSheetConfig `json:"character_sheet,omitempty"`
	// EndTime has no counterpart here — see the huma updateGameBody for why
	// this one takes RFC3339 where create also accepts datetime-local.
}

// ApplyToGameRequest represents the request to apply to join a game
type ApplyToGameRequest struct {
	Role    string `json:"role"`
	Message string `json:"message,omitempty"`
}

// UpdateAutoAcceptAudienceRequest represents the request to toggle whether
// audience applications are accepted without review.
type UpdateAutoAcceptAudienceRequest struct {
	AutoAcceptAudience bool `json:"auto_accept_audience"`
}

// validateScheduleFields enforces the all-or-nothing common room schedule rule.
//
// Still used: the huma create and update bodies call it from Resolve, so this is
// live validation rather than a leftover.
func validateScheduleFields(openDay, closeDay *int16, openTime, closeTime *string, tz *string) error {
	// All five fields must be set together or all omitted — no partial schedules.
	// The frontend tracks 4 visible fields; schedule_timezone is auto-appended
	// from the browser on submit.
	filledCount := 0
	for _, v := range []bool{openDay != nil, closeDay != nil, openTime != nil, closeTime != nil, tz != nil} {
		if v {
			filledCount++
		}
	}
	if filledCount > 0 && filledCount < 5 {
		return errors.New("all schedule fields (open_day, open_time, close_day, close_time, schedule_timezone) must be set together or all omitted")
	}

	// Day range is also declared as minimum/maximum on the huma bodies, so this
	// arm is only reachable through a direct call.
	for _, day := range []*int16{openDay, closeDay} {
		if day != nil && (*day < 0 || *day > 6) {
			return errors.New("common room day must be 0 (Sunday) through 6 (Saturday)")
		}
	}
	for _, t := range []*string{openTime, closeTime} {
		if t != nil {
			if _, err := time.Parse("15:04", *t); err != nil {
				return errors.New("common room time must be in HH:MM format")
			}
		}
	}
	if tz != nil {
		if _, err := time.LoadLocation(*tz); err != nil {
			return fmt.Errorf("schedule_timezone %q is not a valid IANA timezone name", *tz)
		}
	}
	return nil
}
