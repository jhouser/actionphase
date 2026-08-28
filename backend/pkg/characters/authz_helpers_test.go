package characters

import (
	"testing"

	"actionphase/pkg/core"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

// canSeeCharacterPrivateStats is the last gate before a private message count
// is disclosed. The game-level half is exercised through the handler tests in
// api_stats_test.go; what is asserted here is the ownership half and, most
// importantly, that an unauthenticated caller is never granted access by it.
func TestCanSeeCharacterPrivateStats(t *testing.T) {
	owner := &core.AuthenticatedUser{ID: 7}
	stranger := &core.AuthenticatedUser{ID: 8}
	ownedBy7 := pgtype.Int4{Int32: 7, Valid: true}
	// A NULL owner column decodes with Int32 left at its zero value. Pairing it
	// with a caller whose ID is also 0 is what makes the Valid check load
	// bearing: without it the two compare equal and every NPC reads as owned by
	// the caller.
	unowned := pgtype.Int4{Int32: 0, Valid: false}
	zeroIDUser := &core.AuthenticatedUser{ID: 0}

	tests := []struct {
		name            string
		gameLevelAccess bool
		authUser        *core.AuthenticatedUser
		ownerUserID     pgtype.Int4
		want            bool
	}{
		{
			name:        "owner sees their own private count without game-level access",
			authUser:    owner,
			ownerUserID: ownedBy7,
			want:        true,
		},
		{
			name:        "stranger without game-level access is denied",
			authUser:    stranger,
			ownerUserID: ownedBy7,
			want:        false,
		},
		{
			name:            "game-level access grants a stranger",
			gameLevelAccess: true,
			authUser:        stranger,
			ownerUserID:     ownedBy7,
			want:            true,
		},
		{
			// An NPC has no owner. A NULL owner column must not compare equal
			// to any user, or every caller would read as the owner.
			name:        "unowned character grants nobody by ownership",
			authUser:    owner,
			ownerUserID: unowned,
			want:        false,
		},
		{
			// The NULL sentinel collides with a zero-valued caller ID unless
			// the Valid flag is checked first.
			name:        "NULL owner does not match a zero-valued user ID",
			authUser:    zeroIDUser,
			ownerUserID: unowned,
			want:        false,
		},
		{
			name:        "unauthenticated caller is denied",
			authUser:    nil,
			ownerUserID: ownedBy7,
			want:        false,
		},
		{
			// Belt and braces: even with the game-level flag set, a nil user
			// must not dereference. The flag alone decides.
			name:            "unauthenticated caller with game-level access is allowed and does not panic",
			gameLevelAccess: true,
			authUser:        nil,
			ownerUserID:     unowned,
			want:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := canSeeCharacterPrivateStats(tt.gameLevelAccess, tt.authUser, tt.ownerUserID)
			assert.Equal(t, tt.want, got)
		})
	}
}

// canSeePlayerNames enforces the anonymity promise of an anonymous game. If it
// returned true for a plain player, the game would silently deanonymise its
// cast — the kind of failure no status code reveals.
func TestCanSeePlayerNames(t *testing.T) {
	tests := []struct {
		name        string
		isAnonymous bool
		role        string
		want        bool
	}{
		{"non-anonymous game shows names to players", false, "player", true},
		{"non-anonymous game shows names to observers", false, "observer", true},
		{"anonymous game hides names from players", true, "player", false},
		{"anonymous game hides names from an unknown role", true, "observer", false},
		{"anonymous game hides names from the empty role", true, "", false},
		{"anonymous game still shows names to the GM", true, "gm", true},
		{"anonymous game still shows names to a co-GM", true, "co_gm", true},
		{"anonymous game still shows names to the audience", true, "audience", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, canSeePlayerNames(tt.isAnonymous, tt.role))
		})
	}
}

// ptrText / ptrInt / ptrBool decide whether a nullable column renders as an
// absent JSON field or as a zero value. Collapsing NULL to "" / 0 / false is a
// silent data change the frontend reads as a real value.
func TestNullableColumnPointers(t *testing.T) {
	t.Run("ptrText", func(t *testing.T) {
		assert.Nil(t, ptrText(pgtype.Text{Valid: false}), "NULL must stay absent")

		got := ptrText(pgtype.Text{String: "approved", Valid: true})
		if assert.NotNil(t, got) {
			assert.Equal(t, "approved", *got)
		}

		// A valid empty string is a real value, distinct from NULL.
		empty := ptrText(pgtype.Text{String: "", Valid: true})
		if assert.NotNil(t, empty) {
			assert.Equal(t, "", *empty)
		}
	})

	t.Run("ptrInt", func(t *testing.T) {
		assert.Nil(t, ptrInt(pgtype.Int4{Valid: false}), "NULL must stay absent")

		got := ptrInt(pgtype.Int4{Int32: 42, Valid: true})
		if assert.NotNil(t, got) {
			assert.Equal(t, int32(42), *got)
		}

		// Zero is a real user id boundary: it must not be confused with NULL.
		zero := ptrInt(pgtype.Int4{Int32: 0, Valid: true})
		if assert.NotNil(t, zero) {
			assert.Equal(t, int32(0), *zero)
		}
	})

	t.Run("ptrBool", func(t *testing.T) {
		assert.Nil(t, ptrBool(pgtype.Bool{Valid: false}), "NULL must stay absent")

		tru := ptrBool(pgtype.Bool{Bool: true, Valid: true})
		if assert.NotNil(t, tru) {
			assert.True(t, *tru)
		}

		// A valid false must render as false, not vanish like a NULL.
		fls := ptrBool(pgtype.Bool{Bool: false, Valid: true})
		if assert.NotNil(t, fls) {
			assert.False(t, *fls)
		}
	})
}
