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

// ErrCommunityNotFound is returned when a community lookup by ID or slug finds
// no row. Handlers translate it to 404; without it a missing community reads as
// a 500.
var ErrCommunityNotFound = errors.New("community not found")

// ErrCommunitySlugTaken is returned when a create would collide with an
// existing slug. Handlers translate it to 400 rather than surfacing the raw
// unique-violation.
var ErrCommunitySlugTaken = errors.New("community slug already taken")

// ErrCommunityInactive is returned when a game would be created in, or moved
// into, a community that is no longer active. An inactive community accepts no
// new games -- that is what deactivating one means.
var ErrCommunityInactive = errors.New("community is not active")

// ErrCommunityDocumentNotFound is returned when a document lookup finds no row.
//
// Also returned when the document exists but belongs to a DIFFERENT community
// than the one in the request path: the queries are scoped by
// (id, community_id), so a cross-community id misses rather than resolving.
// Answering 404 rather than 403 is deliberate -- confirming "that document
// exists, just not here" would leak another community's drafts.
var ErrCommunityDocumentNotFound = errors.New("community document not found")

// ErrInvalidDocumentStatus is returned when a document status is neither
// "draft" nor "published". Handlers translate it to 400.
var ErrInvalidDocumentStatus = errors.New("document status must be draft or published")

// ErrGameCommunityLocked is returned when a GM tries to move a game between
// communities after it has left `setup` (decision 4).
//
// Reassignment changes which banlist applies, so it is moderation-relevant once
// players have joined. There is no admin override.
//
// Only an actual MOVE trips this. Resending the community the game already has
// is not a move and is allowed, so a client that round-trips a GET into a PUT
// can still edit a recruiting game.
var ErrGameCommunityLocked = errors.New("game community can only be changed during setup")

// ErrOwnerCannotBeModerator is returned when adding the community owner to the
// moderator roster.
//
// The owner is intentionally NOT a row in community_moderators -- ownership
// lives in communities.owner_user_id, and the permission helpers treat owner as
// a superset of moderator. Allowing a duplicate owner row would create a
// community whose owner could be "demoted" by removing a moderator row while
// still owning it.
var ErrOwnerCannotBeModerator = errors.New("the community owner already has full moderation powers")

// ErrAlreadyModerator is returned when adding a user who already moderates the
// community.
var ErrAlreadyModerator = errors.New("user is already a moderator of this community")

// ErrUserBannedFromCommunity is returned when a banned user tries to enter or
// create a game in the community that banned them.
//
// The message names the community rather than the ban's reason: the reason is
// moderator-facing context in the audit log, and per the scope decisions no
// notification is sent to the banned user -- telling them is the owner's job.
var ErrUserBannedFromCommunity = errors.New("you are banned from this community")

// ErrCannotBanCommunityStaff is returned when banning a community's own owner
// or one of its moderators.
//
// A ban is enforced against people ENTERING games; a moderator who is also
// banned is a contradictory state that no enforcement path knows how to read.
// Remove them from the roster first, which is a deliberate owner-only act.
var ErrCannotBanCommunityStaff = errors.New("remove this user from the moderator roster before banning them")

// ErrBanNotFound is returned when lifting a ban that does not exist.
var ErrBanNotFound = errors.New("no ban found for that user in this community")

// ErrCommunityWebhookNotFound is returned when a webhook lookup finds no row.
//
// Also returned when the webhook exists but belongs to a DIFFERENT community
// than the request path names -- the queries are scoped by (id, community_id).
// 404 rather than 403 for the same reason as documents, with more at stake: a
// response that distinguished "exists elsewhere" from "does not exist" would
// let a moderator of one community probe for another's webhook rows.
var ErrCommunityWebhookNotFound = errors.New("community webhook not found")

// ErrWebhookURLRequired is returned when a webhook is created with no URL.
// Separate from ErrInvalidWebhookURL so the moderator is told which mistake
// they made: an empty box reads differently from a rejected one.
var ErrWebhookURLRequired = errors.New("webhook URL is required")

// ErrInvalidWebhookURL is returned when a URL is not an https Discord webhook
// endpoint. Handlers translate it to 400.
//
// This is an SSRF control, not a formatting preference: the server POSTs to
// this URL from inside the network, so accepting an arbitrary host would make
// the feature an outbound request proxy. See core.ValidateWebhookURL.
var ErrInvalidWebhookURL = errors.New("webhook URL must be an https Discord webhook endpoint")

// ErrInvalidWebhookEvent is returned when a webhook subscribes to something
// that is not a notifiable game state. Handlers translate it to 400.
var ErrInvalidWebhookEvent = errors.New("webhook event must be a valid game state")
