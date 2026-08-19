package games

import (
	"actionphase/pkg/core"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// CreateGameRequest represents the request to create a new game
type CreateGameRequest struct {
	Title                   string              `json:"title" validate:"required,min=3,max=255"`
	Description             string              `json:"description" validate:"required,min=10"`
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
	// Sparse per-game character sheet overrides. Absent means "all defaults",
	// which the frontend owns; the server never fills them in.
	//
	// Kept as RawMessage rather than the typed struct because render.Bind decodes
	// with a permissive decoder: an unknown key would be silently discarded here
	// instead of rejected, defeating the whole point of the strict schema. It is
	// parsed by core.UnmarshalCharacterSheetConfig in Bind below.
	CharacterSheet json.RawMessage `json:"character_sheet,omitempty"`

	// Parsed form of CharacterSheet, populated by Bind.
	characterSheet core.CharacterSheetConfig
}

func (r *CreateGameRequest) Bind(req *http.Request) error {
	// Parsed AND validated here, not only in the service. The service validates
	// too (its invariants are its own to hold), but a violation surfacing there
	// renders as a 500 "unexpected error" — so a GM typing a 25-character tab
	// label would be told the server broke. Bind failures render as 400 with the
	// message, which is what a bad label actually is.
	parsed, err := core.UnmarshalCharacterSheetConfig(r.CharacterSheet)
	if err != nil {
		return err
	}
	validated, err := core.ValidateCharacterSheetConfig(parsed)
	if err != nil {
		return err
	}
	r.characterSheet = validated

	return validateScheduleFields(r.CommonRoomOpenDay, r.CommonRoomCloseDay, r.CommonRoomOpenTime, r.CommonRoomCloseTime, r.ScheduleTimezone)
}

// CharacterSheetConfig returns the parsed per-game sheet config.
func (r *CreateGameRequest) CharacterSheetConfig() core.CharacterSheetConfig {
	return r.characterSheet
}

// UpdateGameStateRequest represents the request to update a game's state
type UpdateGameStateRequest struct {
	State string `json:"state" validate:"required"`
}

func (r *UpdateGameStateRequest) Bind(req *http.Request) error {
	return core.ValidateStruct(r)
}

// UpdateGameRequest represents the request to update game details
type UpdateGameRequest struct {
	Title                   string     `json:"title" validate:"required,min=3,max=255"`
	Description             string     `json:"description" validate:"required,min=10"`
	Genre                   string     `json:"genre,omitempty"`
	StartDate               *time.Time `json:"start_date,omitempty"`
	EndDate                 *time.Time `json:"end_date,omitempty"`
	RecruitmentDeadline     *time.Time `json:"recruitment_deadline,omitempty"`
	MaxPlayers              int32      `json:"max_players,omitempty"`
	IsPublic                bool       `json:"is_public"`
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
	// Sparse per-game character sheet overrides. Absent means "all defaults",
	// which the frontend owns; the server never fills them in.
	//
	// Kept as RawMessage rather than the typed struct because render.Bind decodes
	// with a permissive decoder: an unknown key would be silently discarded here
	// instead of rejected, defeating the whole point of the strict schema. It is
	// parsed by core.UnmarshalCharacterSheetConfig in Bind below.
	CharacterSheet json.RawMessage `json:"character_sheet,omitempty"`

	// Parsed form of CharacterSheet, populated by Bind.
	characterSheet core.CharacterSheetConfig
}

func (r *UpdateGameRequest) Bind(req *http.Request) error {
	// Parsed AND validated here, not only in the service. The service validates
	// too (its invariants are its own to hold), but a violation surfacing there
	// renders as a 500 "unexpected error" — so a GM typing a 25-character tab
	// label would be told the server broke. Bind failures render as 400 with the
	// message, which is what a bad label actually is.
	parsed, err := core.UnmarshalCharacterSheetConfig(r.CharacterSheet)
	if err != nil {
		return err
	}
	validated, err := core.ValidateCharacterSheetConfig(parsed)
	if err != nil {
		return err
	}
	r.characterSheet = validated

	return validateScheduleFields(r.CommonRoomOpenDay, r.CommonRoomCloseDay, r.CommonRoomOpenTime, r.CommonRoomCloseTime, r.ScheduleTimezone)
}

// CharacterSheetConfig returns the parsed per-game sheet config.
func (r *UpdateGameRequest) CharacterSheetConfig() core.CharacterSheetConfig {
	return r.characterSheet
}

func validateScheduleFields(openDay, closeDay *int16, openTime, closeTime *string, tz *string) error {
	// All five fields must be set together or all omitted — no partial schedules.
	// The frontend tracks 4 visible fields; schedule_timezone is auto-appended from the browser on submit.
	filledCount := 0
	for _, v := range []bool{openDay != nil, closeDay != nil, openTime != nil, closeTime != nil, tz != nil} {
		if v {
			filledCount++
		}
	}
	if filledCount > 0 && filledCount < 5 {
		return errors.New("all schedule fields (open_day, open_time, close_day, close_time, schedule_timezone) must be set together or all omitted")
	}

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

// ApplyToGameRequest represents the request to apply to join a game
type ApplyToGameRequest struct {
	Role    string `json:"role" validate:"required"`
	Message string `json:"message,omitempty"`
}

func (r *ApplyToGameRequest) Bind(req *http.Request) error {
	return core.ValidateStruct(r)
}

// ReviewApplicationRequest represents the request to review a game application
type ReviewApplicationRequest struct {
	Action string `json:"action" validate:"required"` // "approve" or "reject"
}

func (r *ReviewApplicationRequest) Bind(req *http.Request) error {
	return core.ValidateStruct(r)
}

// LootTableItemRequest is a single item within a loot table. Data is the
// GM-authored JSON blob describing the item; it is stored verbatim and parsed by
// the frontend when the item is granted, so it must be present and well-formed.
type LootTableItemRequest struct {
	Name string `json:"name" validate:"required"`
	Data string `json:"data" validate:"required"`
}

// validateLootTableItems enforces the `validate` tags above. The struct tags are
// not executed anywhere in this package (see .claude/planning/request-validation.md),
// so the checks are explicit.
func validateLootTableItems(items []LootTableItemRequest) error {
	for i, item := range items {
		if strings.TrimSpace(item.Name) == "" {
			return fmt.Errorf("loot table item %d: name is required", i+1)
		}
		if strings.TrimSpace(item.Data) == "" {
			return fmt.Errorf("loot table item %d: data is required", i+1)
		}
		if !json.Valid([]byte(item.Data)) {
			return fmt.Errorf("loot table item %d (%q): data must be valid JSON", i+1, item.Name)
		}
	}
	return nil
}

type UpdateLootTableRequest struct {
	Name  string                 `json:"name" validate:"required"`
	Items []LootTableItemRequest `json:"items"`
}

func (r *UpdateLootTableRequest) Bind(req *http.Request) error {
	if strings.TrimSpace(r.Name) == "" {
		return errors.New("loot table name is required")
	}
	return validateLootTableItems(r.Items)
}

type UpdateLootTableContentsRequest struct {
	Items []LootTableItemRequest `json:"items"`
}

func (r *UpdateLootTableContentsRequest) Bind(req *http.Request) error {
	return validateLootTableItems(r.Items)
}
