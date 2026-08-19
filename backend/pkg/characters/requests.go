package characters

import (
	"net/http"

	"actionphase/pkg/core"
)

// CreateCharacterRequest represents a request to create a new character
type CreateCharacterRequest struct {
	Name          string `json:"name" validate:"required,min=1,max=255"`
	CharacterType string `json:"character_type" validate:"required"`
	UserID        *int32 `json:"user_id,omitempty"` // Optional: for GMs to assign player characters to specific players
}

func (r *CreateCharacterRequest) Bind(req *http.Request) error {
	return core.ValidateStruct(r)
}

// CharacterDataRequest represents a request to set character data
type CharacterDataRequest struct {
	ModuleType string `json:"module_type" validate:"required"`
	FieldName  string `json:"field_name" validate:"required"`
	FieldValue string `json:"field_value"`
	FieldType  string `json:"field_type" validate:"required"`
	IsPublic   bool   `json:"is_public"`
}

func (r *CharacterDataRequest) Bind(req *http.Request) error {
	return core.ValidateStruct(r)
}

// ApproveCharacterRequest represents a request to approve or reject a character
type ApproveCharacterRequest struct {
	Status string `json:"status" validate:"required"` // "approved" or "rejected"
}

func (r *ApproveCharacterRequest) Bind(req *http.Request) error {
	return core.ValidateStruct(r)
}

// AssignNPCRequest represents a request to assign an NPC to a user
type AssignNPCRequest struct {
	AssignedUserID int32 `json:"assigned_user_id" validate:"required"`
}

func (r *AssignNPCRequest) Bind(req *http.Request) error {
	return core.ValidateStruct(r)
}

// ReassignCharacterRequest represents a request to reassign an inactive character
type ReassignCharacterRequest struct {
	NewOwnerUserID int32 `json:"new_owner_user_id" validate:"required"`
}

func (r *ReassignCharacterRequest) Bind(req *http.Request) error {
	return core.ValidateStruct(r)
}

// RenameCharacterRequest represents a request to rename a character
type RenameCharacterRequest struct {
	Name string `json:"name" validate:"required,min=1,max=255"`
}

func (r *RenameCharacterRequest) Bind(req *http.Request) error {
	return core.ValidateStruct(r)
}
