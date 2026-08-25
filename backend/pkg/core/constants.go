package core

// GameStates defines all valid game states and their transitions.
// This provides a single source of truth for game state management
// and enables compile-time checking of state values.
const (
	// GameStateSetup - Initial game creation state
	// GM is configuring game settings, not yet accepting players
	GameStateSetup = "setup"

	// GameStateRecruitment - Game is accepting new players
	// Players can join during this state
	GameStateRecruitment = "recruitment"

	// GameStateCharacterCreation - Character creation phase
	// Players create/customize their characters
	GameStateCharacterCreation = "character_creation"

	// GameStateInProgress - Game is actively being played
	// No new players can join (except as audience)
	GameStateInProgress = "in_progress"

	// GameStatePaused - Game temporarily paused
	// Can resume to in_progress
	GameStatePaused = "paused"

	// GameStateCompleted - Game finished successfully
	// No further state changes allowed
	GameStateCompleted = "completed"

	// GameStateCancelled - Game cancelled or abandoned
	// Terminal state - no further changes allowed
	GameStateCancelled = "cancelled"
)

// PhaseTypes defines all valid game phase types.
const (
	PhaseTypeCommonRoom = "common_room"
	PhaseTypeAction     = "action"
	PhaseTypeInterlude  = "interlude"
)

// ValidPhaseTypes contains all valid phase types for validation.
var ValidPhaseTypes = []string{
	PhaseTypeCommonRoom,
	PhaseTypeAction,
	PhaseTypeInterlude,
}

// ParticipantRoles defines all valid participant roles in games.
const (
	// RoleGM - Game Master, controls the game narrative and rules
	RoleGM = "gm"

	// RolePlayer - Active participant who controls a character
	RolePlayer = "player"

	// RoleAudience - Observer who can watch but not participate actively
	RoleAudience = "audience"
)

// ValidParticipantRoles contains all valid participant roles for validation.
var ValidParticipantRoles = []string{
	RoleGM,
	RolePlayer,
	RoleAudience,
}

// ParticipantStatus defines the status of a game participant.
const (
	// StatusActive - Participant is actively involved in the game
	StatusActive = "active"

	// StatusInactive - Participant is temporarily inactive
	StatusInactive = "inactive"

	// StatusBanned - Participant has been banned from the game
	StatusBanned = "banned"
)

// ErrorCodes define application-specific error codes for client handling.
// These supplement HTTP status codes with more specific error categorization.
const (
	// General validation errors (1000-1099)
	ErrCodeValidation     = 1001
	ErrCodeMissingField   = 1002
	ErrCodeInvalidFormat  = 1003
	ErrCodeDuplicateValue = 1004

	// Authentication/Authorization errors (1100-1199)
	ErrCodeUnauthorized       = 1101
	ErrCodeForbidden          = 1102
	ErrCodeInvalidToken       = 1103
	ErrCodeExpiredToken       = 1104
	ErrCodeInvalidCredentials = 1105

	// User management errors (1200-1299)
	ErrCodeUserNotFound      = 1201
	ErrCodeUserAlreadyExists = 1202
	ErrCodeWeakPassword      = 1203
	ErrCodeInvalidEmail      = 1204

	// Game management errors (1300-1399)
	ErrCodeGameNotFound             = 1301
	ErrCodeGameNotRecruiting        = 1302
	ErrCodeGameFull                 = 1303
	ErrCodeGameDeadlinePassed       = 1304
	ErrCodeAlreadyParticipant       = 1305
	ErrCodeNotParticipant           = 1306
	ErrCodeInvalidGameState         = 1307
	ErrCodeInvalidStateTransition   = 1308
	ErrCodeNotGameMaster            = 1309
	ErrCodeApplicationNotFound      = 1310
	ErrCodeApplicationExists        = 1311
	ErrCodeInvalidApplicationStatus = 1312
	ErrCodeGameArchived             = 1313

	// System/Infrastructure errors (1400-1499)
	ErrCodeDatabaseError   = 1401
	ErrCodeExternalService = 1402
	ErrCodeRateLimited     = 1403
)

// JoinGameStatusCodes define the possible results of checking if a user can join a game.
// These are used by the CanUserJoinGame service method.
const (
	// CanJoin - User is eligible to join the game
	CanJoin = "can_join"

	// GameNotRecruiting - Game is not in recruitment state
	GameNotRecruiting = "game_not_recruiting"

	// DeadlinePassed - Recruitment deadline has passed
	DeadlinePassed = "deadline_passed"

	// GameFull - Game has reached maximum player capacity
	GameFull = "game_full"

	// AlreadyJoined - User is already a participant in this game
	AlreadyJoined = "already_joined"

	// GameNotFound - Game does not exist
	GameNotFound = "game_not_found"
)

// ApplicationStatusCodes define the possible results of checking if a user can apply to a game.
// These are used by the CanUserApplyToGame service method.
const (
	// CanApply - User can submit an application to the game
	CanApply = "can_apply"

	// IsGameMaster - User is the Game Master of this game
	IsGameMaster = "is_game_master"

	// ApplicationPending - User already has a pending application
	ApplicationPending = "application_pending"

	// ApplicationRejected - User's previous application was rejected
	ApplicationRejected = "application_rejected"

	// AlreadyParticipant - User is already a participant in the game
	AlreadyParticipant = "already_participant"

	// NotRecruiting - Game is not currently in recruitment state
	NotRecruiting = "not_recruiting"
)

// ApplicationStatuses define all valid application statuses.
const (
	// ApplicationStatusPending - Application submitted, awaiting GM review
	ApplicationStatusPending = "pending"

	// ApplicationStatusApproved - GM approved the application
	ApplicationStatusApproved = "approved"

	// ApplicationStatusRejected - GM rejected the application
	ApplicationStatusRejected = "rejected"
)

// ValidApplicationStatuses contains all valid application statuses for validation.
var ValidApplicationStatuses = []string{
	ApplicationStatusPending,
	ApplicationStatusApproved,
	ApplicationStatusRejected,
}

// NotificationTypes define all valid notification types.
const (
	// NotificationTypePrivateMessage - User received a private message
	NotificationTypePrivateMessage = "private_message"

	// NotificationTypeCommentReply - User's comment received a reply
	NotificationTypeCommentReply = "comment_reply"

	// NotificationTypeCharacterMention - Character was mentioned in a comment
	NotificationTypeCharacterMention = "character_mention"

	// NotificationTypeActionSubmitted - Player submitted an action (for GM)
	NotificationTypeActionSubmitted = "action_submitted"

	// NotificationTypeActionResult - GM published an action result (for Player)
	NotificationTypeActionResult = "action_result"

	// NotificationTypeCommonRoomPost - New post in common room
	NotificationTypeCommonRoomPost = "common_room_post"

	// NotificationTypePhaseCreated - New phase was created and activated
	NotificationTypePhaseCreated = "phase_created"

	// NotificationTypeApplicationSubmitted - Player submitted a game application (for GM)
	NotificationTypeApplicationSubmitted = "application_submitted"

	// NotificationTypeApplicationApproved - Game application was approved
	NotificationTypeApplicationApproved = "application_approved"

	// NotificationTypeCharacterApproved - Character was approved by GM
	NotificationTypeCharacterApproved = "character_approved"

	// NotificationTypeGameStateChanged - Game state changed (paused, resumed, etc.)
	NotificationTypeGameStateChanged = "game_state_changed"

	// NotificationTypeHandoutPublished - GM published a new handout
	NotificationTypeHandoutPublished = "handout_published"
)

// ValidNotificationTypes contains all valid notification types for validation.
var ValidNotificationTypes = []string{
	NotificationTypePrivateMessage,
	NotificationTypeCommentReply,
	NotificationTypeCharacterMention,
	NotificationTypeActionSubmitted,
	NotificationTypeActionResult,
	NotificationTypeCommonRoomPost,
	NotificationTypePhaseCreated,
	NotificationTypeApplicationSubmitted,
	NotificationTypeApplicationApproved,
	NotificationTypeCharacterApproved,
	NotificationTypeGameStateChanged,
	NotificationTypeHandoutPublished,
}

// NotificationContextTypes identify the container a notification belongs to.
// They scope bulk mark-read operations: opening the container clears every
// notification pointing at it. See core.Notification for how these differ from
// the RelatedType/RelatedID pair.
const (
	// NotificationContextConversation - the notification belongs to a private
	// message conversation; the context ID is the conversation ID.
	NotificationContextConversation = "conversation"
)

// IsValidNotificationType checks if the given type is a valid notification type.
func IsValidNotificationType(notifType string) bool {
	for _, validType := range ValidNotificationTypes {
		if notifType == validType {
			return true
		}
	}
	return false
}

// DatabaseTableNames defines table names for consistent referencing.
// Useful for cleanup operations and migrations.
const (
	TableUsers            = "users"
	TableGames            = "games"
	TableGameParticipants = "game_participants"
	TableGameApplications = "game_applications"
	TableSessions         = "sessions"
	TableCharacters       = "characters"
	TablePhases           = "phases"
	TableCommunications   = "communications"
	TableNotifications    = "notifications"
)

// CommonTableCleanupOrder defines the order for cleaning up tables during tests.
// Child tables with foreign keys should be cleaned up before parent tables.
var CommonTableCleanupOrder = []string{
	TableNotifications,
	TableCommunications,
	TablePhases,
	TableCharacters,
	TableGameParticipants,
	TableGameApplications, // Must be cleaned before TableGames
	TableGames,
	TableSessions,
	TableUsers,
}

// IsValidParticipantRole checks if the given role is a valid participant role.
func IsValidParticipantRole(role string) bool {
	for _, validRole := range ValidParticipantRoles {
		if role == validRole {
			return true
		}
	}
	return false
}

// IsValidApplicationStatus checks if the given status is a valid application status.
func IsValidApplicationStatus(status string) bool {
	for _, validStatus := range ValidApplicationStatuses {
		if status == validStatus {
			return true
		}
	}
	return false
}

// GetGameStateDescriptionBrief returns a brief human-readable description of a game state.
func GetGameStateDescriptionBrief(state string) string {
	descriptions := map[string]string{
		GameStateSetup:             "SETUP",
		GameStateRecruitment:       "RECRUITMENT",
		GameStateCharacterCreation: "CHARACTER CREATION",
		GameStateInProgress:        "IN PROGRESS",
		GameStatePaused:            "PAUSED",
		GameStateCompleted:         "COMPLETED",
		GameStateCancelled:         "CANCELLED",
	}

	if desc, exists := descriptions[state]; exists {
		return desc
	}
	return "Unknown game state"
}
