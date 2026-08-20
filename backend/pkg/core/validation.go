package core

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"

	db "actionphase/pkg/db/models"
)

// ValidateGameNotCompleted checks if a game is in a completed or cancelled state
// and returns an error if write operations are not allowed.
//
// Completed and cancelled games are read-only archives. This validation should be
// called before any write operation (create/update/delete) on game content such as:
//   - Posts and comments (MessageService)
//   - Actions and action results (ActionService)
//   - Phases (PhaseService)
//   - Game settings (GameService)
//   - Characters (CharacterService)
//
// Parameters:
//   - ctx: Request context (not currently used but available for future enhancements)
//   - game: The game to validate (must contain State field)
//
// Returns:
//   - error: ErrGameArchived if game is completed/cancelled, nil if writable
//
// Example Usage:
//
//	game, err := gs.GetGame(ctx, gameID)
//	if err != nil {
//	    return nil, err
//	}
//
//	if err := ValidateGameNotCompleted(ctx, &game); err != nil {
//	    return nil, err
//	}
//
//	// Proceed with write operation...
func ValidateGameNotCompleted(ctx context.Context, game *db.Game) error {
	// Check if game is in a terminal/archived state
	if game.State.String == GameStateCompleted || game.State.String == GameStateCancelled {
		return fmt.Errorf("game %d is archived (state: %s) and read-only", game.ID, game.State.String)
	}

	return nil
}

// requestValidator is the shared validator instance used by ValidateStruct.
// Built once: validator.New reflects over every struct it sees and caches the
// result, so a per-call instance throws that cache away on each request.
var requestValidator = newRequestValidator()

func newRequestValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())

	// Report failures under the JSON name the client sent rather than the Go
	// field name, so a caller can find the field in its own payload.
	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		tag, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if tag == "-" {
			return ""
		}
		return tag
	})

	return v
}

// trimStringFields trims leading and trailing whitespace from every settable
// string field on a request struct, in place, before the tags are executed.
//
// Without this, `required` and `min=1` are satisfied by a name of "   ": the
// stock `required` only rejects the zero value, and validator refuses to let
// `required` be overridden (it is a restricted tag). Every hand-written check
// this helper replaces already trims first — see RenameCharacter in
// pkg/db/services/characters.go and UpdatePost in pkg/messages/api_posts.go —
// so trimming here both closes that gap and matches what the services go on to
// store. Handlers read the trimmed value, which is the value that gets
// persisted.
//
// Composed request types are walked all the way down: nested structs, pointers
// (including *string, which several request types use for optional fields),
// slice and array elements, and map values. json.RawMessage and other []byte
// fields are left alone: they are not text fields, and trimming them would
// corrupt the payload.
//
// v must be addressable for trimming to take effect — reflect refuses to set a
// field reached through a non-pointer value. ValidateStruct enforces that by
// rejecting non-pointer arguments, so this helper is never handed an
// unaddressable struct in practice.
func trimStringFields(v reflect.Value, seen map[uintptr]bool) {
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return
		}
		// Guard against cycles, which a self-referential request type could
		// otherwise turn into unbounded recursion.
		if v.Kind() == reflect.Ptr {
			addr := v.Pointer()
			if seen[addr] {
				return
			}
			seen[addr] = true
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.String:
		// Reached for a bare string only via a pointer, slice element, or map
		// value; struct fields are handled in the Struct case below so that
		// unexported fields can be skipped before we try to set them.
		if v.CanSet() {
			v.SetString(strings.TrimSpace(v.String()))
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			if !field.CanSet() {
				continue // unexported
			}
			if field.Kind() == reflect.String {
				field.SetString(strings.TrimSpace(field.String()))
				continue
			}
			trimStringFields(field, seen)
		}
	case reflect.Slice, reflect.Array:
		// []byte and its named forms (json.RawMessage) are payloads, not text.
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return
		}
		for i := 0; i < v.Len(); i++ {
			trimStringFields(v.Index(i), seen)
		}
	case reflect.Map:
		// Map values are not addressable, so they cannot be trimmed in place;
		// they have to be trimmed and written back under the same key. Keys are
		// left alone: they are payload identifiers, not user-facing text.
		for _, key := range v.MapKeys() {
			val := v.MapIndex(key)
			if val.Kind() == reflect.String {
				v.SetMapIndex(key, reflect.ValueOf(strings.TrimSpace(val.String())).Convert(val.Type()))
				continue
			}
			// Anything else needs an addressable copy to recurse into.
			copied := reflect.New(val.Type()).Elem()
			copied.Set(val)
			trimStringFields(copied, seen)
			v.SetMapIndex(key, copied)
		}
	}
}

// ValidateStruct executes the `validate` struct tags on a request type and
// returns a client-readable error, or nil when every field passes. It must be
// passed a pointer — see the guard in the body for why a value is rejected.
//
// Request structs across the API carry `validate` tags, but go-chi/render's
// Bind is the only hook that runs after a body is decoded — so unless a Bind
// method calls this, the tags document intent and enforce nothing. Call it from
// Bind:
//
//	func (r *RenameCharacterRequest) Bind(req *http.Request) error {
//	    return core.ValidateStruct(r)
//	}
//
// Bind failures render as 400 via core.ErrInvalidRequest, which is what a
// malformed payload actually is. Letting the same violation fall through to the
// service layer instead surfaces it as a 500 "unexpected error", telling the
// user the server broke when they simply sent a blank name.
//
// The raw validator.ValidationErrors text is not fit to return to a client
// ("Key: 'RenameCharacterRequest.Name' Error:Field validation for 'Name'
// failed on the 'required' tag"), so errors are reformatted in terms of the
// JSON field name the client actually sent.
//
// Tags alone cannot express cross-field or semantic rules — that all five
// schedule fields be set together, or that a string parse as JSON. Those stay
// as explicit checks in Bind; see validateScheduleFields and
// validateLootTableItems in pkg/games/requests.go. The two approaches coexist,
// and a Bind may run both.
func ValidateStruct(v any) error {
	// A non-pointer argument cannot be trimmed: reflect will not set a field
	// reached through a value copy, so CanSet is false everywhere and every
	// trim silently no-ops. The tags would still run, which is the dangerous
	// part — "   " would sail past `required` and the whitespace bug this
	// function exists to close would quietly reopen. Reject it loudly instead;
	// like the InvalidValidationError below, it is a caller bug, not a 400.
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr {
		return fmt.Errorf("validate: ValidateStruct requires a pointer to a struct, got %T", v)
	}

	// Trim before validating so that a whitespace-only string fails `required`
	// and `min`, and so handlers see the same trimmed value the service stores.
	trimStringFields(rv, map[uintptr]bool{})

	err := requestValidator.Struct(v)
	if err == nil {
		return nil
	}

	// An invalid argument (nil, or a non-struct) is a programming error in the
	// caller, not a bad request. Surface it rather than reporting it as a
	// validation failure against a field that does not exist.
	var invalid *validator.InvalidValidationError
	if errors.As(err, &invalid) {
		return fmt.Errorf("validate: %w", err)
	}

	var validationErrs validator.ValidationErrors
	if !errors.As(err, &validationErrs) {
		return err
	}

	messages := make([]string, 0, len(validationErrs))
	for _, fieldErr := range validationErrs {
		messages = append(messages, describeFieldError(fieldErr))
	}
	return errors.New(strings.Join(messages, "; "))
}

// describeFieldError renders one field failure using the JSON name the client
// sent, so the message names a field the caller can actually find in its
// payload rather than the Go struct field.
func describeFieldError(fieldErr validator.FieldError) string {
	// Field() is the JSON name, courtesy of RegisterTagNameFunc; it falls back
	// to the Go name when a field carries no usable json tag.
	name := fieldErr.Field()
	if name == "" {
		name = fieldErr.StructField()
	}

	switch fieldErr.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", name)
	case "min":
		// min on a string is a length; on a number it is a floor. Saying
		// "must be at least 3" for a title reads as a value, not a length.
		if fieldErr.Kind() == reflect.String {
			return fmt.Sprintf("%s must be at least %s characters", name, fieldErr.Param())
		}
		return fmt.Sprintf("%s must be at least %s", name, fieldErr.Param())
	case "max":
		if fieldErr.Kind() == reflect.String {
			return fmt.Sprintf("%s must be at most %s characters", name, fieldErr.Param())
		}
		return fmt.Sprintf("%s must be at most %s", name, fieldErr.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", name, strings.Join(strings.Fields(fieldErr.Param()), ", "))
	case "email":
		return fmt.Sprintf("%s must be a valid email address", name)
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", name, fieldErr.Param())
	case "gte":
		return fmt.Sprintf("%s must be %s or greater", name, fieldErr.Param())
	case "lt":
		return fmt.Sprintf("%s must be less than %s", name, fieldErr.Param())
	case "lte":
		return fmt.Sprintf("%s must be %s or less", name, fieldErr.Param())
	default:
		return fmt.Sprintf("%s is invalid (%s)", name, fieldErr.Tag())
	}
}
