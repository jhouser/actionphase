package characters

import (
	"time"

	"actionphase/pkg/core"
)

// Response bodies
//
// Several of these replace `map[string]interface{}` literals the chi handlers
// encoded directly. The maps built their keys conditionally -- omitting player
// identity in anonymous games, omitting nullable columns that were NULL -- so
// every such key is a pointer with `omitempty` here. That reproduces the wire
// format exactly: absent stays absent rather than becoming a null or a zero
// value the frontend would render as a real answer.

// CharacterResponse is the single-character body returned by create, get,
// approve, reassign and rename.
type CharacterResponse struct {
	ID     int32  `json:"id" doc:"Character ID"`
	GameID int32  `json:"game_id" doc:"Game the character belongs to"`
	UserID *int32 `json:"user_id,omitempty" required:"false" doc:"Owning player, absent for unassigned NPCs"`
	Name   string `json:"name" doc:"Character name"`
	// Absent when an anonymous game hides it from regular players, which is why
	// it is a pointer rather than a plain string with an enum.
	CharacterType *string   `json:"character_type,omitempty" required:"false" enum:"player_character,npc" doc:"Absent when an anonymous game hides it from the caller"`
	Status        string    `json:"status" enum:"pending,approved" doc:"Approval status"`
	AvatarURL     *string   `json:"avatar_url,omitempty" required:"false" doc:"Character portrait URL"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// GameCharacterResponse is one entry of the per-game roster.
//
// It carries more than CharacterResponse (is_active, the owner's username, NPC
// assignment) and drops nothing, but the identity fields are omitted wholesale
// for regular players in anonymous games -- see canSeePlayerNames.
type GameCharacterResponse struct {
	ID            int32     `json:"id" doc:"Character ID"`
	GameID        int32     `json:"game_id" doc:"Game the character belongs to"`
	Name          string    `json:"name" doc:"Character name"`
	CharacterType string    `json:"character_type" enum:"player_character,npc" doc:"Character kind"`
	Status        *string   `json:"status,omitempty" required:"false" enum:"pending,approved" doc:"Approval status"`
	IsActive      bool      `json:"is_active" doc:"False once the character has been retired"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// Identity fields. All four are omitted together when an anonymous game
	// hides player identity from the caller; individually they are also omitted
	// when the underlying column is NULL.
	UserID           *int32  `json:"user_id,omitempty" required:"false" doc:"Owning player"`
	Username         *string `json:"username,omitempty" required:"false" doc:"Owning player's username"`
	AssignedUserID   *int32  `json:"assigned_user_id,omitempty" required:"false" doc:"User controlling this NPC"`
	AssignedUsername *string `json:"assigned_username,omitempty" required:"false" doc:"Username controlling this NPC"`

	// Avatars stay visible in anonymous games: the portrait belongs to the
	// character, not the player behind it.
	AvatarURL *string `json:"avatar_url,omitempty" required:"false" doc:"Character portrait URL"`
}

// ControllableCharacterResponse is one entry of the per-game controllable list.
//
// Note it omits is_active entirely -- the query already filters to active
// characters, so the field was never in this payload and callers must not
// branch on it.
type ControllableCharacterResponse struct {
	ID            int32     `json:"id" doc:"Character ID"`
	GameID        int32     `json:"game_id" doc:"Game the character belongs to"`
	Name          string    `json:"name" doc:"Character name"`
	CharacterType string    `json:"character_type" enum:"player_character,npc" doc:"Character kind"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	UserID    *int32  `json:"user_id,omitempty" required:"false" doc:"Owning player"`
	Status    *string `json:"status,omitempty" required:"false" enum:"pending,approved" doc:"Approval status"`
	AvatarURL *string `json:"avatar_url,omitempty" required:"false" doc:"Character portrait URL"`
}

// ControllableCharacterWithGameResponse adds the game context the cross-game
// list needs, for surfaces rendering a sheet with no game in scope.
type ControllableCharacterWithGameResponse struct {
	ControllableCharacterResponse

	GameTitle           string  `json:"game_title" doc:"Title of the game this character belongs to"`
	GameState           *string `json:"game_state,omitempty" required:"false" doc:"That game's lifecycle state"`
	GameIsAnonymous     bool    `json:"game_is_anonymous" doc:"Whether that game hides player identity"`
	GamePortraitAvatars bool    `json:"game_portrait_avatars" doc:"Whether that game renders avatars as portraits"`

	// Omitted when the GM has set no overrides, which is the common case. The
	// frontend owns the default labels, so absent means "use the defaults" --
	// filling them in here would put the defaults in two places.
	GameCharacterSheet *core.CharacterSheetConfig `json:"game_character_sheet,omitempty" required:"false" doc:"That game's sheet label overrides, absent when it has none"`

	// Present for a GM's cast entries: a GM receives every character in games
	// they run, not just the ones they personally control.
	Username         *string `json:"username,omitempty" required:"false" doc:"Owning player's username"`
	AssignedUsername *string `json:"assigned_username,omitempty" required:"false" doc:"Username controlling this NPC"`

	UserRole string `json:"user_role" enum:"gm,co_gm,player,audience" doc:"The caller's role in that game"`
}

// InactiveCharacterResponse is one entry of the GM's inactive-character list,
// which carries the ownership history a reassignment decision needs.
type InactiveCharacterResponse struct {
	ID            int32     `json:"id" doc:"Character ID"`
	GameID        int32     `json:"game_id" doc:"Game the character belongs to"`
	Name          string    `json:"name" doc:"Character name"`
	CharacterType string    `json:"character_type" enum:"player_character,npc" doc:"Character kind"`
	Status        string    `json:"status" enum:"pending,approved" doc:"Approval status"`
	IsActive      bool      `json:"is_active" doc:"Always false in this list"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// Always present but nullable, matching the chi handler: it put the raw
	// nullable column in the map, which encodes as a string or an explicit
	// null -- never as an absent key.
	CurrentOwnerUsername  *string `json:"current_owner_username" doc:"Who holds the character now; null if that account is gone"`
	OriginalOwnerUsername *string `json:"original_owner_username" doc:"Who created the character; null if that account is gone"`

	UserID              *int32 `json:"user_id,omitempty" required:"false" doc:"Current owner"`
	OriginalOwnerUserID *int32 `json:"original_owner_user_id,omitempty" required:"false" doc:"Original owner"`
}

// CharacterDataResponse is one stored character-sheet field.
type CharacterDataResponse struct {
	ID          int32  `json:"id" doc:"Row ID"`
	CharacterID int32  `json:"character_id" doc:"Character this field belongs to"`
	ModuleType  string `json:"module_type" doc:"Sheet tab, e.g. bio, skills, inventory"`
	FieldName   string `json:"field_name" doc:"Field key within the module"`
	// Nullable in the database, though every row written through the API sets
	// it. Kept nullable rather than defaulted so a legacy NULL row is reported
	// as unknown rather than silently claiming to be text.
	FieldType *string   `json:"field_type" enum:"text,number,boolean,json" doc:"How field_value should be parsed"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	FieldValue *string `json:"field_value,omitempty" required:"false" doc:"Stored value, absent when NULL"`
	IsPublic   *bool   `json:"is_public,omitempty" required:"false" doc:"Whether non-editors may read this field"`
}

// CharacterStatsResponse represents the activity stats response for a character.
//
// PrivateMessages is omitted rather than zeroed when the caller may not see it:
// a 0 would read as "this character has no private messages", which is a
// different and misleading claim.
type CharacterStatsResponse struct {
	PublicMessages  int64  `json:"public_messages" doc:"Messages visible to the whole game"`
	PrivateMessages *int64 `json:"private_messages,omitempty" required:"false" doc:"Omitted when the caller may not see it"`
}
