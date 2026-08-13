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
