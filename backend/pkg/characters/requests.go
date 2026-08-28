package characters

import (
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// Request bodies
//
// Length and enum rules are struct tags, so huma enforces them before the
// handler runs and publishes them in the spec. Resolve handles only what the
// schema cannot: trimming, since minLength counts raw characters and a name of
// "   " would otherwise satisfy it and store blank.

// requiredName trims *s in place and reports it missing if nothing is left.
func requiredName(s *string, field string) []error {
	*s = strings.TrimSpace(*s)
	if *s != "" {
		return nil
	}
	return []error{&huma.ErrorDetail{
		Message:  field + " is required",
		Location: "body." + field,
	}}
}

// CreateCharacterRequest represents a request to create a new character
type CreateCharacterRequest struct {
	Name string `json:"name" minLength:"1" maxLength:"255" doc:"Character name, unique within the game"`
	// The chi handler validated this against the same two values by hand, after
	// its `validate:"required"` tag had already passed anything non-empty. The
	// enum tag now does both, and matches the characters_character_type_check
	// constraint in the database.
	CharacterType string `json:"character_type" enum:"player_character,npc" doc:"Character kind"`
	// Required when a GM creates a player character, since the GM is naming
	// someone else's character; enforced in the handler because it depends on
	// the caller's role, which the schema cannot see.
	UserID *int32 `json:"user_id,omitempty" required:"false" doc:"Player to own the character. Required when a GM creates a player_character."`
}

func (r *CreateCharacterRequest) Resolve(huma.Context) []error {
	return requiredName(&r.Name, "name")
}

// CharacterDataRequest represents a request to set character data
type CharacterDataRequest struct {
	ModuleType string `json:"module_type" minLength:"1" doc:"Sheet tab, e.g. bio, skills, inventory"`
	FieldName  string `json:"field_name" minLength:"1" doc:"Field key within the module"`
	// Deliberately not required: clearing a field is a legitimate write, and the
	// chi handler's tag allowed an empty value too.
	FieldValue string `json:"field_value" required:"false" doc:"Value to store. Empty clears the field."`
	FieldType  string `json:"field_type" enum:"text,number,boolean,json" doc:"How field_value should be parsed"`
	IsPublic   bool   `json:"is_public" required:"false" doc:"Whether non-editors may read this field"`
}

func (r *CharacterDataRequest) Resolve(huma.Context) []error {
	var errs []error
	errs = append(errs, requiredName(&r.ModuleType, "module_type")...)
	errs = append(errs, requiredName(&r.FieldName, "field_name")...)
	return errs
}

// ApproveCharacterRequest represents a request to approve a character.
//
// The endpoint is approve-only: the chi handler rejected anything but
// "approved", and the database's characters_status_check constraint allows only
// pending and approved, so there is no reject path to express. The field is
// kept (rather than dropped for an empty body) because clients send it.
type ApproveCharacterRequest struct {
	Status string `json:"status" enum:"approved" doc:"Must be \"approved\"; there is no reject path"`
}

// AssignNPCRequest represents a request to assign an NPC to a user
type AssignNPCRequest struct {
	// minimum:"1" replaces the old `validate:"required"`, which on an int32
	// rejected 0 as the zero value. Same effect, but expressed as a bound the
	// spec can publish.
	AssignedUserID int32 `json:"assigned_user_id" minimum:"1" doc:"Audience member to control the NPC, or the GM to take it back"`
}

// ReassignCharacterRequest represents a request to reassign an inactive character
type ReassignCharacterRequest struct {
	NewOwnerUserID int32 `json:"new_owner_user_id" minimum:"1" doc:"Player to become the character's owner"`
}

// RenameCharacterRequest represents a request to rename a character
type RenameCharacterRequest struct {
	Name string `json:"name" minLength:"1" maxLength:"255" doc:"New character name, unique within the game"`
}

func (r *RenameCharacterRequest) Resolve(huma.Context) []error {
	return requiredName(&r.Name, "name")
}
