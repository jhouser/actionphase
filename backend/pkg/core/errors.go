package core

import "errors"

// ErrNotImplemented is returned by mock methods that have not been implemented
var ErrNotImplemented = errors.New("not implemented")

// ErrCharacterNotControlled is returned when a user attempts to use a character they don't control
var ErrCharacterNotControlled = errors.New("you do not control this character")

// ErrDraftPostExists is returned when attempting to create a draft post for a phase that already has one
var ErrDraftPostExists = errors.New("a draft post already exists for this phase")

// ErrInvalidStagedChain is returned when a staged result chain violates its
// shape rules: too few parts, too many, a delay out of range, or a head
// carrying a delay. Wrapped so handlers can answer 400 rather than 500 — these
// are all the caller's mistake, and the composer needs to show the GM which.
var ErrInvalidStagedChain = errors.New("invalid staged result chain")

// ErrCannotCancelPart is returned when a staged part cannot be cancelled
// because it has already been released or is a chain head. Both are states a
// GM can reach by racing the release worker, so they are not server faults.
var ErrCannotCancelPart = errors.New("cannot cancel staged part")

// ErrCannotEditChain is returned when a staged chain cannot be edited in the
// way asked: appending to a chain that is already published, or retiming a part
// that has already been released.
//
// Like ErrCannotCancelPart these are reachable by racing the release worker
// rather than by sending a malformed request, so handlers answer 409 rather
// than 400 — the request was well formed, the world moved.
var ErrCannotEditChain = errors.New("cannot edit staged chain")

// ErrInvalidStateTransition is returned when a game state change is rejected by
// the state machine (allowedTransitions, pkg/db/services/games.go) — for
// example epilogue → in_progress, which is a deliberate one-way door.
//
// The request is well formed and the caller is authorized; the transition is
// simply not legal from where the game currently is. That makes it a 409, not a
// 500: nothing failed, and retrying verbatim will never succeed. Handlers
// should surface it with ErrCodeInvalidGameState so a client can distinguish it
// from a genuine server fault.
var ErrInvalidStateTransition = errors.New("invalid game state transition")
