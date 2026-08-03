package core

import (
	models "actionphase/pkg/db/models"
	"context"
	"io"
	"time"
)

// SessionServiceInterface defines the contract for session management operations.
// Sessions are used to manage JWT refresh tokens and provide secure token renewal.
//
// Usage Example:
//
//	sessionService := &services.SessionService{DB: pool}
//
//	// Create a new session for user login
//	session := &Session{
//	    User:  user,
//	    Token: "refresh_token_string",
//	}
//	createdSession, err := sessionService.CreateSession(session)
//
//	// Retrieve session by refresh token
//	session, err := sessionService.SessionByToken("refresh_token")
//
//	// Clean up expired sessions
//	err := sessionService.DeleteSessionByToken("old_token")
type SessionServiceInterface interface {
	// Session retrieves a session by ID
	Session(id int) (*Session, error)

	// SessionByToken retrieves a session by its token
	SessionByToken(token string) (*Session, error)

	// Sessions retrieves all sessions (primarily for admin operations)
	Sessions() ([]*Session, error)

	// CreateSession creates a new session for a user
	CreateSession(session *Session) (*Session, error)

	// DeleteSessionByToken removes a session by its token
	DeleteSessionByToken(token string) error

	// InvalidateAllUserSessions deletes all sessions for a user (used when banning)
	InvalidateAllUserSessions(ctx context.Context, userID int32) error

	// CreateSessionWithMetadata creates a session and stores IP, user agent, and fingerprint
	CreateSessionWithMetadata(ctx context.Context, session *Session) (*Session, error)

	// GetUserSessionsWithDetails returns sessions for a user including metadata (admin use)
	GetUserSessionsWithDetails(ctx context.Context, userID int32) ([]*SessionWithDetails, error)

	// UpdateSessionLastSeen updates the last_seen_at timestamp for a session
	UpdateSessionLastSeen(ctx context.Context, sessionID int32) error

	// GetSessionByID retrieves a session by its numeric ID
	GetSessionByID(ctx context.Context, id int32) (*Session, error)

	// GetUserSessions retrieves all sessions for a user
	GetUserSessions(ctx context.Context, userID int32) ([]models.Session, error)

	// DeleteSession deletes a session by numeric ID
	DeleteSession(ctx context.Context, sessionID int32) error

	// InvalidateSessionsByIP invalidates all sessions from a given IP address
	InvalidateSessionsByIP(ctx context.Context, ipAddress string) error

	// InvalidateSessionsByFingerprint invalidates all sessions with a given device fingerprint
	InvalidateSessionsByFingerprint(ctx context.Context, fingerprint string) error
}

// UserServiceInterface defines the contract for user management operations.
// Handles user account lifecycle including registration, authentication, and profile management.
//
// Usage Example:
//
//	userService := &services.UserService{DB: pool}
//
//	// Create a new user account
//	user := &User{
//	    Username: "player1",
//	    Email:    "player1@example.com",
//	    Password: "plaintext_password",
//	}
//	user.HashPassword() // Hash password before storage
//	createdUser, err := userService.CreateUser(user)
//
//	// Authenticate user login
//	user, err := userService.UserByUsername("player1")
//	if user != nil && user.CheckPasswordHash("attempted_password") {
//	    // User authenticated successfully
//	}
//
//	// Retrieve user for authorization
//	user, err := userService.GetUserByID(userID)
type UserServiceInterface interface {
	// GetUserByID retrieves a user by ID
	GetUserByID(id int) (*User, error)

	// UserByUsername retrieves a user by username
	UserByUsername(username string) (*User, error)

	// Users retrieves all users (primarily for admin operations)
	Users() ([]*User, error)

	// CreateUser creates a new user account
	CreateUser(user *User) (*User, error)

	// DeleteUser removes a user account
	DeleteUser(id int) error

	// Admin management
	SetAdminStatus(ctx context.Context, userID int32, isAdmin bool, requesterID int32) error
	ListAdmins(ctx context.Context) ([]*User, error)

	// User banning
	BanUser(ctx context.Context, userID int32, adminID int32) error
	UnbanUser(ctx context.Context, userID int32) error
	ListBannedUsers(ctx context.Context) ([]*BannedUser, error)
	CheckUserBanned(ctx context.Context, userID int32) (bool, error)

	// User listing and search (admin)
	ListAllUsers(ctx context.Context, page, pageSize int, search string) ([]*User, int64, error)
	ListAllUsersAdmin(ctx context.Context, page, pageSize int, search string) ([]*User, int64, error)

	// Registration approval
	ListPendingApprovalUsers(ctx context.Context) ([]*User, error)
	ApproveUser(ctx context.Context, userID int32) error
	RejectUser(ctx context.Context, userID int32) error
	SetPendingApproval(ctx context.Context, userID int32) error

	// UserByEmail retrieves a user by email address
	UserByEmail(email string) (*User, error)

	// SearchUsers searches for users matching a query string
	SearchUsers(ctx context.Context, query string) ([]models.SearchUsersRow, error)
}

// IPBanServiceInterface defines the contract for IP address banning.
type IPBanServiceInterface interface {
	CreateIPBan(ctx context.Context, ipAddress, reason string, createdBy int32, expiresAt *time.Time, bannedUserID *int32) (*IPBan, error)
	ListIPBans(ctx context.Context) ([]*IPBan, error)
	DeleteIPBan(ctx context.Context, id int32) error
	IsIPBanned(ctx context.Context, ipAddress string) (bool, error)
	CleanupExpiredIPBans(ctx context.Context) error
}

// FingerprintBanServiceInterface defines the contract for device fingerprint banning.
type FingerprintBanServiceInterface interface {
	CreateFingerprintBan(ctx context.Context, fingerprint, reason string, createdBy int32, bannedUserID *int32) (*FingerprintBan, error)
	ListFingerprintBans(ctx context.Context) ([]*FingerprintBan, error)
	DeleteFingerprintBan(ctx context.Context, id int32) error
	IsFingerprintBanned(ctx context.Context, fingerprint string) (bool, error)
}

// GameServiceInterface defines the contract for game management operations.
// Handles complete game lifecycle from creation through completion, including
// participant management and state transitions.
//
// Usage Example:
//
//	gameService := &services.GameService{DB: pool}
//
//	// Create a new game
//	game, err := gameService.CreateGame(ctx, CreateGameRequest{
//	    Title:       "Epic D&D Campaign",
//	    Description: "A thrilling adventure in the Forgotten Realms",
//	    GMUserID:    int32(gmUser.ID),
//	    Genre:       "Fantasy RPG",
//	    MaxPlayers:  6,
//	    IsPublic:    true,
//	})
//
//	// Transition game to accept players
//	game, err = gameService.UpdateGameState(ctx, game.ID, "recruitment")
//
//	// Players apply to join the game (no direct joining)
//	// Applications are handled by the GameApplicationService
//
//	// Check if user can perform actions
//	role, err := gameService.GetUserRole(ctx, game.ID, int32(user.ID))
//	if role == "gm" {
//	    // User is Game Master, allow game management
//	}
//
//	// Start the game when ready (auto-approves and converts applications)
//	game, err = gameService.UpdateGameState(ctx, game.ID, "character_creation")
type GameServiceInterface interface {
	// CreateGame creates a new game with the given parameters
	CreateGame(ctx context.Context, req CreateGameRequest) (*models.Game, error)

	// GetGame retrieves a game by its ID
	GetGame(ctx context.Context, gameID int32) (*models.Game, error)

	// GetGamesByUser retrieves all games associated with a user
	GetGamesByUser(ctx context.Context, userID int32) ([]models.GetGamesByUserRow, error)

	// UpdateGameState updates the state of a game (setup, recruitment, in_progress, etc.)
	UpdateGameState(ctx context.Context, gameID int32, newState string) (*models.Game, error)

	// UpdateGame updates game details
	UpdateGame(ctx context.Context, req UpdateGameRequest) (*models.Game, error)

	// DeleteGame removes a game from the system (only allowed for GMs on cancelled games)
	DeleteGame(ctx context.Context, gameID, userID int32) error

	// LeaveGame allows a user to leave a game
	LeaveGame(ctx context.Context, gameID, userID int32) error

	// GetUserRole determines a user's role in a specific game
	GetUserRole(ctx context.Context, gameID, userID int32) (string, error)

	// IsUserInGame checks if a user is participating in a game
	IsUserInGame(ctx context.Context, gameID, userID int32) (bool, error)

	// GetGameWithDetails retrieves a game with additional metadata
	GetGameWithDetails(ctx context.Context, gameID int32) (*models.GetGameWithDetailsRow, error)

	// GetRecruitingGames retrieves all games currently accepting new players
	GetRecruitingGames(ctx context.Context) ([]models.GetRecruitingGamesRow, error)

	// CanUserJoinGame checks if a user is eligible to join a specific game
	CanUserJoinGame(ctx context.Context, gameID, userID int32) (string, error)

	// AddGameParticipant adds a user as a participant to a game
	AddGameParticipant(ctx context.Context, gameID, userID int32, role string) (*models.GameParticipant, error)

	// RemoveGameParticipant removes a user from game participants
	RemoveGameParticipant(ctx context.Context, gameID, userID int32) error

	// GetGameParticipants retrieves all participants for a game
	GetGameParticipants(ctx context.Context, gameID int32) ([]models.GetGameParticipantsRow, error)

	// GetFilteredGames retrieves games with filters, sorting, and user enrichment
	GetFilteredGames(ctx context.Context, filters GameListingFilters) (*GameListingResponse, error)

	// Player Management methods

	// RemovePlayer removes a player from the game and deactivates their characters
	RemovePlayer(ctx context.Context, gameID, userID, gmUserID int32) error

	// AddParticipantWithRole adds a user directly to the game with the given role, bypassing the application process.
	// Valid roles: "player", "audience"
	AddParticipantWithRole(ctx context.Context, gameID, userID int32, role string) (*models.GameParticipant, error)

	// GetActiveParticipants retrieves all active (non-removed) participants for a game
	GetActiveParticipants(ctx context.Context, gameID int32) ([]models.GetActiveParticipantsRow, error)

	// Audience Participation methods

	// GetGameAutoAcceptAudience retrieves the auto-accept audience setting for a game
	GetGameAutoAcceptAudience(ctx context.Context, gameID int32) (bool, error)

	// UpdateGameAutoAcceptAudience updates the auto-accept audience setting for a game
	UpdateGameAutoAcceptAudience(ctx context.Context, gameID int32, autoAccept bool) error

	// CreateAudienceApplication allows a user to apply/join as an audience member
	CreateAudienceApplication(ctx context.Context, gameID, userID int32) (*models.GameParticipant, error)

	// ListAudienceMembers retrieves all audience members for a game
	ListAudienceMembers(ctx context.Context, gameID int32) ([]models.ListAudienceMembersRow, error)

	// CheckAudienceAccess verifies if a user has audience or GM access to a game
	CheckAudienceAccess(ctx context.Context, gameID, userID int32) (bool, error)

	// CanUserViewGame checks if a user can view a game's content (read-only access)
	// Returns true if:
	// - Game is completed (public archive mode - ANY user can view)
	// - User is GM, participant, or audience member (for active games)
	// Note: Cancelled games are NOT public and follow normal permission rules
	CanUserViewGame(ctx context.Context, gameID, userID int32) (bool, error)

	// UpdateGameBannerURL sets or clears the banner image URL for a game
	UpdateGameBannerURL(ctx context.Context, gameID int32, bannerURL *string) error

	// PromoteToCoGM promotes a participant to co-GM role
	PromoteToCoGM(ctx context.Context, gameID, userID, requestingUserID int32) error

	// DemoteFromCoGM removes co-GM role from a participant
	DemoteFromCoGM(ctx context.Context, gameID, userID, requestingUserID int32) error

	// TransitionPlayerToAudience moves a player to audience role
	TransitionPlayerToAudience(ctx context.Context, gameID, userID, requestingUserID int32) error

	// GetGameLogs retrieves all logs for a given game
	GetGameLogs(ctx context.Context, gameID int32) ([]models.GameLog, error)
}

// GameApplicationServiceInterface defines the contract for game application operations.
// Handles the application flow where players apply to join games and require GM approval.
//
// Usage Example:
//
//	applicationService := &services.GameApplicationService{DB: pool}
//
//	// Player applies to join a game
//	application, err := applicationService.CreateGameApplication(ctx, CreateGameApplicationRequest{
//	    GameID:  123,
//	    UserID:  456,
//	    Role:    "player",
//	    Message: "I'd love to play a wizard in this campaign!",
//	})
//
//	// GM views all applications for their game
//	applications, err := applicationService.GetGameApplications(ctx, 123)
//
//	// GM approves an application
//	err = applicationService.ApproveGameApplication(ctx, application.ID, gmUserID)
//
//	// Check if user can apply to a game
//	canApply, err := applicationService.CanUserApplyToGame(ctx, gameID, userID)
type GameApplicationServiceInterface interface {
	// CreateGameApplication creates a new application to join a game
	CreateGameApplication(ctx context.Context, req CreateGameApplicationRequest) (*models.GameApplication, error)

	// GetGameApplication retrieves a specific application by ID
	GetGameApplication(ctx context.Context, applicationID int32) (*models.GameApplication, error)

	// GetGameApplications retrieves all applications for a game with user details
	GetGameApplications(ctx context.Context, gameID int32) ([]models.GetGameApplicationsRow, error)

	// GetGameApplicationsByStatus retrieves applications for a game filtered by status
	GetGameApplicationsByStatus(ctx context.Context, gameID int32, status string) ([]models.GetGameApplicationsByStatusRow, error)

	// GetUserGameApplications retrieves all applications submitted by a user
	GetUserGameApplications(ctx context.Context, userID int32) ([]models.GetUserGameApplicationsRow, error)

	// ApproveGameApplication approves an application and optionally creates participant
	ApproveGameApplication(ctx context.Context, applicationID, reviewerID int32) error

	// RejectGameApplication rejects an application
	RejectGameApplication(ctx context.Context, applicationID, reviewerID int32) error

	// DeleteGameApplication removes an application (for cleanup or withdrawal)
	DeleteGameApplication(ctx context.Context, applicationID, userID int32) error

	// DeleteStaleApprovedApplicationForUser removes a leftover 'approved' application for a
	// user who is no longer an active participant in the game (no-op otherwise)
	DeleteStaleApprovedApplicationForUser(ctx context.Context, gameID, userID int32) error

	// CanUserApplyToGame checks if a user is eligible to apply to a game
	CanUserApplyToGame(ctx context.Context, gameID, userID int32) (string, error)

	// HasUserAppliedToGame checks if user has an existing application
	HasUserAppliedToGame(ctx context.Context, gameID, userID int32) (bool, error)

	// CountPendingApplicationsForGame returns count of pending applications
	CountPendingApplicationsForGame(ctx context.Context, gameID int32) (int64, error)

	// BulkApproveApplications approves all pending applications for a game
	BulkApproveApplications(ctx context.Context, gameID, reviewerID int32) error

	// GetApprovedApplicationsForGame retrieves approved applications for participant creation
	GetApprovedApplicationsForGame(ctx context.Context, gameID int32) ([]models.GetApprovedApplicationsForGameRow, error)

	// GetGameApplicationByUserAndGame retrieves a user's application for a specific game
	GetGameApplicationByUserAndGame(ctx context.Context, gameID, userID int32) (*models.GameApplication, error)

	// BulkRejectApplications rejects all pending applications for a game
	BulkRejectApplications(ctx context.Context, gameID, reviewerID int32) error

	// ConvertApprovedApplicationsToParticipants converts approved applications to participants
	ConvertApprovedApplicationsToParticipants(ctx context.Context, gameID int32) error

	// PublishApplicationStatuses makes application decisions visible to applicants
	PublishApplicationStatuses(ctx context.Context, gameID int32) error

	// DeleteRejectedApplications removes rejected application records
	DeleteRejectedApplications(ctx context.Context, gameID int32) error

	// GetPublicGameApplicants retrieves public-facing applicant information for a game
	GetPublicGameApplicants(ctx context.Context, gameID int32) ([]models.GetPublicGameApplicantsRow, error)
}

// CreateGameRequest represents the parameters needed to create a new game
type CreateGameRequest struct {
	Title                   string
	Description             string
	GMUserID                int32
	Genre                   string
	StartDate               *time.Time
	EndDate                 *time.Time
	RecruitmentDeadline     *time.Time
	MaxPlayers              int32
	IsPublic                bool
	IsAnonymous             bool
	AutoAcceptAudience      bool
	AllowGroupConversations bool
	PortraitAvatars         bool
	BannerURL               *string
	CommonRoomOpenDay       *int16
	CommonRoomOpenTime      *string // "HH:MM"
	CommonRoomCloseDay      *int16
	CommonRoomCloseTime     *string // "HH:MM"
	ScheduleTimezone        *string // IANA timezone name, e.g. "America/New_York"
}

// UpdateGameRequest represents the parameters needed to update an existing game
type UpdateGameRequest struct {
	ID                      int32
	Title                   string
	Description             string
	Genre                   string
	StartDate               *time.Time
	EndDate                 *time.Time
	RecruitmentDeadline     *time.Time
	MaxPlayers              int32
	IsPublic                bool
	IsAnonymous             bool
	AutoAcceptAudience      bool
	AllowGroupConversations bool
	PortraitAvatars         bool
	BannerURL               *string
	CommonRoomOpenDay       *int16
	CommonRoomOpenTime      *string // "HH:MM"
	CommonRoomCloseDay      *int16
	CommonRoomCloseTime     *string // "HH:MM"
	ScheduleTimezone        *string // IANA timezone name, e.g. "America/New_York"
}

// PhaseServiceInterface defines the contract for game phase management operations.
// Handles the alternating Common Room and Action phases that define ActionPhase gameplay.
//
// Usage Example:
//
//	phaseService := &services.PhaseService{DB: pool}
//
//	// Create the first phase for a new game
//	phase, err := phaseService.CreatePhase(ctx, CreatePhaseRequest{
//	    GameID:      123,
//	    PhaseType:   "common_room",
//	    PhaseNumber: 1,
//	    Title:       "Opening Scene",
//	    StartTime:   time.Now(),
//	    EndTime:     time.Now().Add(48 * time.Hour),
//	})
//
//	// Transition to action phase
//	actionPhase, err := phaseService.TransitionToNextPhase(ctx, 123, gmUserID,
//	    TransitionPhaseRequest{
//	        PhaseType: "action",
//	        Title:     "Submit Your Actions",
//	        Deadline:  time.Now().Add(72 * time.Hour),
//	    })
//
//	// Get current active phase
//	currentPhase, err := phaseService.GetActivePhase(ctx, gameID)
type PhaseServiceInterface interface {
	// CreatePhase creates a new phase for a game
	CreatePhase(ctx context.Context, req CreatePhaseRequest) (*models.GamePhase, error)

	// GetPhase retrieves a specific phase by ID
	GetPhase(ctx context.Context, phaseID int32) (*models.GamePhase, error)

	// GetActivePhase retrieves the currently active phase for a game
	GetActivePhase(ctx context.Context, gameID int32) (*models.GamePhase, error)

	// GetGamePhases retrieves all phases for a game in chronological order
	GetGamePhases(ctx context.Context, gameID int32) ([]models.GamePhase, error)

	// UpdatePhase updates phase details (title, description, times)
	UpdatePhase(ctx context.Context, req UpdatePhaseRequest) (*models.GamePhase, error)

	// TransitionToNextPhase ends current phase and starts a new one
	TransitionToNextPhase(ctx context.Context, gameID, userID int32, req TransitionPhaseRequest) (*models.GamePhase, error)

	// ExtendPhaseDeadline extends the end time or deadline of a phase
	ExtendPhaseDeadline(ctx context.Context, phaseID int32, newDeadline time.Time) (*models.GamePhase, error)

	// ActivatePhase makes a specific phase the active phase for a game
	ActivatePhase(ctx context.Context, phaseID, userID int32) error

	// DeactivatePhase ends the current active phase
	DeactivatePhase(ctx context.Context, gameID, userID int32) error

	// GetPhaseHistory retrieves phase transition history for a game
	GetPhaseHistory(ctx context.Context, gameID int32) ([]PhaseTransitionInfo, error)

	// RunScheduledActivations activates any phases whose start_time has arrived.
	// Returns the number of phases examined and activated.
	RunScheduledActivations(ctx context.Context) (examined int, activated int, err error)

	// CanUserManagePhases checks if a user has GM permissions for a game's phases
	CanUserManagePhases(ctx context.Context, gameID, userID int32) (bool, error)

	// CanUserSubmitActions checks if a user can submit actions in a game
	CanUserSubmitActions(ctx context.Context, gameID, userID int32) (bool, error)

	// DeletePhase removes a phase (only allowed when safe)
	DeletePhase(ctx context.Context, phaseID int32) error

	// CanDeletePhase checks if a phase can be safely deleted
	CanDeletePhase(ctx context.Context, phaseID int32) error
}

// ActionSubmissionServiceInterface defines the contract for action submission operations.
// Handles player action submissions during Action phases of games.
//
// Usage Example:
//
//	actionService := &services.ActionSubmissionService{DB: pool}
//
//	// Player submits action during action phase
//	submission, err := actionService.SubmitAction(ctx, SubmitActionRequest{
//	    GameID:    123,
//	    PhaseID:   456,
//	    UserID:    789,
//	    Content:   richTextContent,
//	    IsDraft:   false,
//	})
//
//	// GM retrieves all submissions for processing
//	submissions, err := actionService.GetPhaseSubmissions(ctx, phaseID)
//
//	// GM sends result to player
//	result, err := actionService.CreateActionResult(ctx, CreateActionResultRequest{
//	    GameID:             123,
//	    PhaseID:            456,
//	    UserID:             789,
//	    ActionSubmissionID: submission.ID,
//	    Content:            gmResponseContent,
//	})
type ActionSubmissionServiceInterface interface {
	// SubmitAction creates or updates an action submission for a phase
	SubmitAction(ctx context.Context, req SubmitActionRequest) (*models.ActionSubmission, error)

	// GetActionSubmission retrieves a specific action submission
	GetActionSubmission(ctx context.Context, submissionID int32) (*models.ActionSubmission, error)

	// GetUserPhaseSubmission retrieves a user's submission for a specific phase
	GetUserPhaseSubmission(ctx context.Context, phaseID, userID int32) (*models.ActionSubmission, error)

	// GetPhaseSubmissions retrieves all submissions for a phase (GM only)
	GetPhaseSubmissions(ctx context.Context, phaseID int32) ([]models.ActionSubmission, error)

	// DeleteActionSubmission removes an action submission (before deadline)
	DeleteActionSubmission(ctx context.Context, submissionID, userID int32) error

	// CreateActionResult creates a GM result for an action submission
	CreateActionResult(ctx context.Context, req CreateActionResultRequest) (*models.ActionResult, error)

	// GetActionResult retrieves a specific action result
	GetActionResult(ctx context.Context, resultID int32) (*models.ActionResult, error)

	// GetUserPhaseResults retrieves all results for a user in a phase
	GetUserPhaseResults(ctx context.Context, phaseID, userID int32) ([]models.ActionResult, error)

	// PublishActionResult makes an action result visible to the player
	PublishActionResult(ctx context.Context, resultID, userID int32) error

	// PublishAllPhaseResults publishes all unpublished results for a phase
	PublishAllPhaseResults(ctx context.Context, phaseID int32) error

	// GetUnpublishedResultsCount returns the count of unpublished results for a phase
	GetUnpublishedResultsCount(ctx context.Context, phaseID int32) (int64, error)

	// UpdateActionResult updates the content of an unpublished action result
	UpdateActionResult(ctx context.Context, resultID int32, content string) (*models.ActionResult, error)

	// DeleteActionResult deletes an unpublished (draft) action result
	DeleteActionResult(ctx context.Context, resultID int32) error

	// GetSubmissionStats returns statistics about submissions for a phase
	GetSubmissionStats(ctx context.Context, phaseID int32) (*ActionSubmissionStats, error)

	// CanUserSubmitAction checks if user can submit/edit actions for a phase
	CanUserSubmitAction(ctx context.Context, phaseID, userID int32) (bool, error)

	// Audience Participation methods

	// ListAllActionSubmissions retrieves all action submissions for a game (for audience/GM)
	ListAllActionSubmissions(ctx context.Context, gameID, phaseID int32, limit, offset int32) ([]models.ListAllActionSubmissionsRow, error)

	// CountAllActionSubmissions counts total action submissions for pagination
	CountAllActionSubmissions(ctx context.Context, gameID, phaseID int32) (int64, error)

	// Draft Character Updates methods

	// CreateDraftCharacterUpdate creates or updates a draft character sheet update for an action result
	// Uses upsert behavior - if draft already exists for this field, it updates the value
	CreateDraftCharacterUpdate(ctx context.Context, req CreateDraftCharacterUpdateRequest) (*models.ActionResultCharacterUpdate, error)

	// GetDraftCharacterUpdates retrieves all draft updates for an action result
	GetDraftCharacterUpdates(ctx context.Context, actionResultID int32) ([]models.ActionResultCharacterUpdate, error)

	// UpdateDraftCharacterUpdate updates the field value of an existing draft
	UpdateDraftCharacterUpdate(ctx context.Context, draftID int32, fieldValue string) (*models.ActionResultCharacterUpdate, error)

	// DeleteDraftCharacterUpdate removes a draft character update
	DeleteDraftCharacterUpdate(ctx context.Context, draftID int32) error

	// GetDraftUpdateCount returns the count of draft updates for an action result
	GetDraftUpdateCount(ctx context.Context, actionResultID int32) (int64, error)

	// GetUserActions retrieves all action submissions by a user in a game
	GetUserActions(ctx context.Context, gameID, userID int32) ([]models.GetUserActionsRow, error)

	// GetGameActions retrieves all action submissions for a game (GM view)
	GetGameActions(ctx context.Context, gameID int32) ([]models.GetGameActionsRow, error)

	// GetUserResults retrieves all action results for a user in a game
	GetUserResults(ctx context.Context, gameID, userID int32) ([]models.GetUserResultsRow, error)

	// GetGameResults retrieves all action results for a game (GM view)
	GetGameResults(ctx context.Context, gameID int32) ([]models.GetGameResultsRow, error)
}

// MessageServiceInterface defines the contract for message and comment operations.
// Handles both Common Room posts/comments and future private messaging between characters.
//
// Key Design Principles:
// - All messages MUST be sent as a character (character_id is required)
// - Visibility types: 'game' (Common Room) or 'private' (future DMs)
// - Reddit-style threading: parent_id creates threaded conversations
// - Thread building: Frontend recursively calls GetPostComments to build comment trees
//
// Usage Example:
//
//	messageService := &services.MessageService{DB: pool}
//
//	// Create a Common Room post
//	post, err := messageService.CreatePost(ctx, CreatePostRequest{
//	    GameID:      123,
//	    PhaseID:     456,
//	    AuthorID:    789,
//	    CharacterID: 111,
//	    Content:     "What should we do next?",
//	    Visibility:  "game",
//	})
//
//	// Add a comment to the post
//	comment, err := messageService.CreateComment(ctx, CreateCommentRequest{
//	    GameID:      123,
//	    PhaseID:     456,
//	    AuthorID:    222,
//	    CharacterID: 333,
//	    Content:     "I think we should investigate the old ruins",
//	    ParentID:    post.ID,
//	    Visibility:  "game",
//	})
//
//	// Get all posts for a game
//	posts, err := messageService.GetGamePosts(ctx, gameID, phaseID, limit, offset)
//
//	// Get comments for a post (direct children only - frontend builds tree)
//	comments, err := messageService.GetPostComments(ctx, postID)
type MessageServiceInterface interface {
	// CreatePost creates a new top-level message post
	CreatePost(ctx context.Context, req CreatePostRequest) (*models.Message, error)

	// GetPost retrieves a specific post by ID with metadata
	GetPost(ctx context.Context, postID int32) (*MessageWithDetails, error)

	// GetGamePosts retrieves posts for a game, optionally filtered by phase
	GetGamePosts(ctx context.Context, gameID int32, phaseID *int32, limit, offset int32) ([]MessageWithDetails, error)

	// GetPhasePosts retrieves all posts for a specific phase
	GetPhasePosts(ctx context.Context, phaseID int32) ([]MessageWithDetails, error)

	// UpdatePost updates the content of an existing post
	UpdatePost(ctx context.Context, postID int32, content string) (*models.Message, error)

	// DeletePost soft-deletes a post (preserves thread structure)
	DeletePost(ctx context.Context, postID int32) error

	// CreateComment creates a comment reply to a post or another comment
	CreateComment(ctx context.Context, req CreateCommentRequest) (*models.Message, error)

	// GetComment retrieves a specific comment by ID with metadata
	GetComment(ctx context.Context, commentID int32) (*MessageWithDetails, error)

	// GetPostComments retrieves direct child comments for a post or comment
	GetPostComments(ctx context.Context, parentID int32) ([]MessageWithDetails, error)

	// UpdateComment updates the content and optionally the character of an existing comment
	UpdateComment(ctx context.Context, commentID int32, content string, newCharacterID *int32) (*models.Message, error)

	// DeleteComment soft-deletes a comment (preserves thread structure)
	// deleterID: the user performing the deletion (could be author, GM, or admin)
	DeleteComment(ctx context.Context, commentID int32, deleterID int32) error

	// CanUserEditComment checks if a user can edit a comment (must be author)
	CanUserEditComment(ctx context.Context, commentID int32, userID int32) (bool, error)

	// CanUserDeleteComment checks if a user can delete a comment (author, GM, or admin in admin mode)
	CanUserDeleteComment(ctx context.Context, commentID int32, userID int32, isAdmin bool) (bool, error)

	// GetGamePostCount returns total post count for a game
	GetGamePostCount(ctx context.Context, gameID int32, phaseID *int32) (int64, error)

	// GetPostCommentCount returns total comment count for a post
	GetPostCommentCount(ctx context.Context, postID int32) (int64, error)

	// GetUserPostsInGame retrieves all posts by a user in a game
	GetUserPostsInGame(ctx context.Context, gameID, userID int32) ([]MessageWithDetails, error)

	// AddReaction adds a reaction to a message
	AddReaction(ctx context.Context, messageID, userID int32, reactionType string) (*models.MessageReaction, error)

	// RemoveReaction removes a reaction from a message
	RemoveReaction(ctx context.Context, messageID, userID int32, reactionType string) error

	// GetMessageReactions retrieves all reactions for a message
	GetMessageReactions(ctx context.Context, messageID int32) ([]models.GetMessageReactionsRow, error)

	// GetReactionCounts retrieves reaction counts grouped by type
	GetReactionCounts(ctx context.Context, messageID int32) ([]models.GetReactionCountsRow, error)

	// ValidateCharacterOwnership verifies character belongs to author and game
	ValidateCharacterOwnership(ctx context.Context, characterID, authorID, gameID int32) error

	// Audience Participation methods (Private Conversation Viewing)

	// ListAllPrivateConversations lists all private conversations in a game (for audience/GM)
	// Supports pagination (limit/offset) and filtering by participant names
	ListAllPrivateConversations(ctx context.Context, params ListAllPrivateConversationsParams) ([]models.ListAllPrivateConversationsRow, error)

	// CountAllPrivateConversations returns the total count of private conversations in a game,
	// applying the same participant filter as ListAllPrivateConversations
	CountAllPrivateConversations(ctx context.Context, gameID int32, participantNames []string) (int64, error)

	// GetConversationParticipantNames returns all participant names in the game's conversations,
	// narrowed to co-participants of all selected names when selectedNames is non-empty.
	GetConversationParticipantNames(ctx context.Context, gameID int32, selectedNames []string) ([]string, error)

	// GetAudienceConversationMessages retrieves all messages in a conversation (for audience/GM)
	GetAudienceConversationMessages(ctx context.Context, conversationID int32) ([]models.GetAudienceConversationMessagesRow, error)

	// ListRecentCommentsWithParents retrieves recent comments with their parent messages/posts
	// for the "New Comments" view. Supports pagination via limit/offset.
	ListRecentCommentsWithParents(ctx context.Context, gameID int32, limit, offset int32) ([]CommentWithParent, error)

	// ListRecentUnreadCommentsWithParents behaves like ListRecentCommentsWithParents but
	// omits comments the user has manually marked as read. Backs the "New Comments"
	// view's unread-only filter in manual read mode.
	ListRecentUnreadCommentsWithParents(ctx context.Context, gameID, userID int32, limit, offset int32) ([]CommentWithParent, error)

	// GetTotalCommentCount returns the total count of non-deleted comments in a game
	GetTotalCommentCount(ctx context.Context, gameID int32) (int64, error)

	// GetTotalUnreadCommentCount returns the count of non-deleted comments in a game
	// that the user has not manually marked as read
	GetTotalUnreadCommentCount(ctx context.Context, gameID, userID int32) (int64, error)

	// GetPostCommentsWithThreads retrieves paginated top-level comments with all nested replies
	// Uses a recursive CTE to load entire comment trees in a single query (eliminates N+1 pattern)
	// Returns flat array with depth field for frontend tree building
	GetPostCommentsWithThreads(ctx context.Context, postID int32, limit int32, offset int32, maxDepth int32) ([]CommentWithDepth, error)

	// CountTopLevelComments returns the total count of top-level comments for a post
	// Used for pagination metadata (calculating has_more, total pages, etc.)
	CountTopLevelComments(ctx context.Context, postID int32) (int64, error)

	// ListCharacterPostsAndComments retrieves paginated public messages by a specific character
	// Returns posts and comments with parent context for the Character Page
	ListCharacterPostsAndComments(ctx context.Context, characterID int32, limit, offset int32) ([]CharacterMessage, error)

	// CountCharacterPostsAndComments returns the total count of public messages by a character
	CountCharacterPostsAndComments(ctx context.Context, characterID int32) (int64, error)

	// ToggleCommentRead marks or unmarks a single comment as manually read by the current user
	ToggleCommentRead(ctx context.Context, userID, gameID, postID, commentID int32, markAsRead bool) error

	// GetManualReadCommentIDsForGame retrieves all comment IDs manually marked as read by a user in a game
	GetManualReadCommentIDsForGame(ctx context.Context, userID, gameID int32) ([]*ManualCommentReads, error)

	// DeleteManualCommentReadsForGame removes all manual comment read records for a game (e.g. on game reset)
	DeleteManualCommentReadsForGame(ctx context.Context, gameID int32) error

	// MarkAllCommentsReadForPhase marks every comment in a phase as manually read by the current user
	MarkAllCommentsReadForPhase(ctx context.Context, userID, gameID, phaseID int32) error

	// Draft Post methods — posts stored before phase activation, visible to GM only

	// GetDraftPostForPhase retrieves the draft post for a pending phase (returns nil if none exists)
	GetDraftPostForPhase(ctx context.Context, phaseID int32) (*MessageWithDetails, error)

	// CreateDraftPost creates a draft post for a pending phase (max one per phase)
	CreateDraftPost(ctx context.Context, req CreatePostRequest) (*MessageWithDetails, error)

	// UpdateDraftPost replaces the content of an existing draft post
	UpdateDraftPost(ctx context.Context, postID int32, content string) (*MessageWithDetails, error)

	// DeleteDraftPost hard-deletes a draft post
	DeleteDraftPost(ctx context.Context, postID int32) error

	// PublishDraftPostsForPhase clears is_draft on all draft posts for the phase (called at activation)
	PublishDraftPostsForPhase(ctx context.Context, phaseID int32) error

	// DeleteDraftPostsForPhase hard-deletes all draft posts for a phase (called when phase is deleted)
	DeleteDraftPostsForPhase(ctx context.Context, phaseID int32) error

	// GetMessage retrieves a specific message (post or comment) by ID with metadata
	GetMessage(ctx context.Context, messageID int32) (*MessageWithDetails, error)

	// GetMessageWithParentContext retrieves a message plus up to maxParents nearest
	// ancestors (ordered parent-to-child) and the true root post ID, in a single query.
	// Used for deep-linking to nested comments without a per-level request waterfall.
	GetMessageWithParentContext(ctx context.Context, messageID int32, maxParents int32) (*MessageThreadContext, error)

	// CanUserEditPost checks if a user can edit a post (must be author)
	CanUserEditPost(ctx context.Context, postID int32, userID int32) (bool, error)

	// MarkPostAsRead marks a post as read by a user, recording the last read comment
	MarkPostAsRead(ctx context.Context, userID, gameID, postID int32, lastReadCommentID *int32) (*ReadMarker, error)

	// GetUserReadMarkersForGame retrieves all read markers for a user in a game
	GetUserReadMarkersForGame(ctx context.Context, userID, gameID int32) ([]*ReadMarker, error)

	// GetPostsWithUnreadInfo retrieves posts with unread status for the authenticated user
	GetPostsWithUnreadInfo(ctx context.Context, gameID int32) ([]*PostUnreadInfo, error)

	// GetUnreadCommentIDsForPosts retrieves unread comment IDs for posts a user has read markers for
	GetUnreadCommentIDsForPosts(ctx context.Context, userID, gameID int32) ([]*PostUnreadComments, error)
}

// CreatePhaseRequest represents the parameters needed to create a new game phase
type CreatePhaseRequest struct {
	GameID      int32
	PhaseType   string // "common_room", "action", "results"
	PhaseNumber int32
	Title       string
	Description string
	StartTime   *time.Time
	EndTime     *time.Time
	Deadline    *time.Time // For action phases
}

// UpdatePhaseRequest represents the parameters needed to update a phase
type UpdatePhaseRequest struct {
	ID          int32
	Title       string
	Description string
	StartTime   *time.Time
	EndTime     *time.Time
	Deadline    *time.Time
}

// TransitionPhaseRequest represents the parameters for phase transitions
type TransitionPhaseRequest struct {
	PhaseType   string // "common_room", "action", "results"
	Title       string
	Description string
	Duration    *time.Duration // If specified, calculates EndTime from now
	EndTime     *time.Time     // Explicit end time
	Deadline    *time.Time     // For action phases
	Reason      string         // Optional reason for transition
}

// SubmitActionRequest represents the parameters needed to submit an action
type SubmitActionRequest struct {
	GameID      int32
	PhaseID     int32
	UserID      int32
	CharacterID *int32      // Optional reference to character
	Content     interface{} // Rich text content (JSON)
	IsDraft     bool
}

// CreateActionResultRequest represents the parameters needed to create action results
type CreateActionResultRequest struct {
	GameID             int32
	PhaseID            int32
	UserID             int32
	CharacterID        *int32      // Optional reference to character (for multi-character scenarios)
	ActionSubmissionID *int32      // Optional reference to the action submission this result is for
	GMUserID           int32       // The GM creating the result
	Content            interface{} // Rich text content (JSON)
	IsPublished        bool
}

// ActionSubmissionStats provides statistics about action submissions for a phase
type ActionSubmissionStats struct {
	PhaseID          int32
	TotalPlayers     int32
	SubmittedCount   int32
	DraftCount       int32
	SubmissionRate   float64 // Percentage of players who submitted
	AverageWordCount int32
	LatestSubmission *time.Time
}

// CreateDraftCharacterUpdateRequest represents the parameters needed to create a draft character update
type CreateDraftCharacterUpdateRequest struct {
	ActionResultID int32
	CharacterID    int32
	ModuleType     string // "abilities", "skills", "inventory", "currency"
	FieldName      string
	FieldValue     string
	FieldType      string // "text", "number", "boolean", "json"
	Operation      string // "upsert", "delete"
}

// PhaseTransitionInfo represents a phase transition record
type PhaseTransitionInfo struct {
	ID              int32
	GameID          int32
	FromPhaseID     *int32
	ToPhaseID       int32
	InitiatedBy     int32
	Reason          string
	CreatedAt       time.Time
	FromPhaseType   string // Type of phase transitioned from
	ToPhaseType     string // Type of phase transitioned to
	FromPhaseNum    int32  // Phase number transitioned from
	ToPhaseNum      int32  // Phase number transitioned to
	InitiatedByUser string // Username who initiated transition
}

// CreateGameApplicationRequest represents the parameters needed to create a game application
type CreateGameApplicationRequest struct {
	GameID  int32
	UserID  int32
	Role    string
	Message string
}

// CreatePostRequest represents the parameters needed to create a new post
type CreatePostRequest struct {
	GameID      int32
	PhaseID     *int32 // Optional - can be nil for game-wide posts
	AuthorID    int32
	CharacterID int32
	Content     string
	Visibility  string // "game" or "private"
}

// CreateCommentRequest represents the parameters needed to create a comment
type CreateCommentRequest struct {
	GameID      int32
	PhaseID     *int32 // Optional - inherits from parent
	AuthorID    int32
	CharacterID int32
	Content     string
	ParentID    int32  // Required - the post or comment being replied to
	RootPostID  int32  // Required - the top-level post this comment belongs to (for read tracking)
	Visibility  string // "game" or "private"
}

// MessageWithDetails represents a message with additional metadata
type MessageWithDetails struct {
	models.Message
	AuthorUsername     string
	CharacterName      string
	CharacterAvatarUrl *string // Optional - character's avatar URL
	CommentCount       int64   // For posts
	ReplyCount         int64   // For comments
}

// MessageThreadContext is a deep-linked comment plus a bounded slice of its
// ancestor chain and the true root post ID (for read tracking / replies).
type MessageThreadContext struct {
	// Chain is the target comment plus up to MaxParents nearest ancestors,
	// ordered parent-to-child (nearest included ancestor → target).
	Chain []MessageWithDetails
	// RootPostID is the top-level post at the head of the full thread, even when
	// Chain is trimmed and does not itself reach the root.
	RootPostID int32
	// HasFullThread is true when Chain reaches the root post (nothing was trimmed above).
	HasFullThread bool
}

// CommentWithDepth represents a comment with its nesting depth for tree building
// Used for paginated comment loading with recursive CTE queries
type CommentWithDepth struct {
	Comment MessageWithDetails
	Depth   int32 // Nesting level: 0 = top-level, 1+ = nested replies
}

// CharacterMessage represents a post or comment by a specific character
// Used for the Character Page to show their activity feed with parent context
type CharacterMessage struct {
	// Message data
	ID                 int32
	GameID             int32
	ParentID           *int32
	AuthorID           int32
	CharacterID        int32
	Content            string
	MessageType        string // "post" or "comment"
	CreatedAt          time.Time
	EditedAt           *time.Time
	EditCount          int32
	DeletedAt          *time.Time
	IsDeleted          bool
	AuthorUsername     string
	CharacterName      *string
	CharacterAvatarUrl *string

	// Parent data (only set when MessageType == "comment")
	ParentContent            *string
	ParentCreatedAt          *time.Time
	ParentDeletedAt          *time.Time
	ParentIsDeleted          *bool
	ParentMessageType        *string // "post" or "comment"
	ParentAuthorUsername     *string
	ParentCharacterName      *string
	ParentCharacterAvatarUrl *string
}

// CommentWithParent represents a comment along with its parent message/post
// Used for the "New Comments" view to show recent activity with context
type CommentWithParent struct {
	// Comment data
	ID                 int32
	GameID             int32
	ParentID           *int32
	PostID             *int32
	AuthorID           int32
	CharacterID        int32
	Content            string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	EditedAt           *time.Time
	EditCount          int32
	DeletedAt          *time.Time
	IsDeleted          bool
	AuthorUsername     string
	CharacterName      *string
	CharacterAvatarUrl *string

	// Parent data (the post or comment this comment is replying to)
	ParentContent            *string
	ParentCreatedAt          *time.Time
	ParentDeletedAt          *time.Time
	ParentIsDeleted          *bool
	ParentMessageType        *string // "post" or "comment"
	ParentAuthorUsername     *string
	ParentCharacterName      *string
	ParentCharacterAvatarUrl *string
}

// ListAllPrivateConversationsParams represents parameters for listing private conversations
type ListAllPrivateConversationsParams struct {
	GameID           int32
	ParticipantNames []string // Filter by participant names (character names or usernames)
	Limit            int32    // Number of results to return
	Offset           int32    // Number of results to skip
}

// ReadMarker tracks which comments a user has read in a common room post
type ReadMarker struct {
	ID                int32
	UserID            int32
	GameID            int32
	PostID            int32
	LastReadCommentID *int32    // The most recent comment read (nil if only post read)
	LastReadAt        time.Time // When the user last viewed this thread
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// PostUnreadInfo contains aggregated data about a post's comments
// Used to determine if a post has unread content
type PostUnreadInfo struct {
	PostID          int32
	PostCreatedAt   time.Time
	TotalComments   int64
	LatestCommentAt *time.Time // Nil if no comments exist
}

// PostUnreadComments contains the specific unread comment IDs for a post
type PostUnreadComments struct {
	PostID           int32
	UnreadCommentIDs []int32 // IDs of comments that are unread (greater than last_read_comment_id)
}

// ManualCommentReads contains the comment IDs manually marked as read by a user for a post
type ManualCommentReads struct {
	PostID         int32
	ReadCommentIDs []int32
}

// NotificationServiceInterface defines the contract for notification operations.
// Handles creating, retrieving, and managing user notifications.
//
// Usage Example:
//
//	notificationService := &services.NotificationService{DB: pool}
//
//	// Create a notification
//	notification, err := notificationService.CreateNotification(ctx, &CreateNotificationRequest{
//	    UserID: 123,
//	    GameID: &gameID,
//	    Type:   NotificationTypeActionResult,
//	    Title:  "You received an action result",
//	    LinkURL: &linkURL,
//	})
//
//	// Get unread count
//	count, err := notificationService.GetUnreadCount(ctx, userID)
//
//	// Mark as read
//	err := notificationService.MarkAsRead(ctx, notificationID, userID)
type NotificationServiceInterface interface {
	// CreateNotification creates a new notification for a user
	CreateNotification(ctx context.Context, req *CreateNotificationRequest) (*Notification, error)

	// CreateBulkNotifications creates notifications for multiple users at once
	// Used for game-wide notifications (e.g., new phase, new post)
	CreateBulkNotifications(ctx context.Context, userIDs []int32, req *CreateNotificationRequest) error

	// GetUserNotifications retrieves a user's notifications with pagination
	GetUserNotifications(ctx context.Context, userID int32, limit, offset int) ([]*Notification, error)

	// GetUnreadCount returns the count of unread notifications for a user
	GetUnreadCount(ctx context.Context, userID int32) (int64, error)

	// GetUnreadNotifications retrieves only unread notifications for a user
	GetUnreadNotifications(ctx context.Context, userID int32, limit int) ([]*Notification, error)

	// MarkAsRead marks a notification as read
	MarkAsRead(ctx context.Context, notificationID, userID int32) error

	// MarkAsUnread marks a notification as unread (allows users to revisit it later)
	MarkAsUnread(ctx context.Context, notificationID, userID int32) error

	// MarkAllAsRead marks all of a user's unread notifications as read
	MarkAllAsRead(ctx context.Context, userID int32) error

	// DeleteNotification deletes a notification (user must own it)
	DeleteNotification(ctx context.Context, notificationID, userID int32) error

	// DeleteOldReadNotifications cleans up read notifications older than 30 days
	// Called by background job
	DeleteOldReadNotifications(ctx context.Context) error

	// Helper methods for common notification scenarios
	// These methods handle the creation logic for specific notification types

	// NotifyPrivateMessage creates a notification for a new private message
	NotifyPrivateMessage(ctx context.Context, recipientUserID int32, messageID int32, gameID int32, conversationID int32, senderCharacterName string) error

	// NotifyCommentReply creates a notification when someone replies to a comment
	NotifyCommentReply(ctx context.Context, originalCommentAuthorID int32, replyID int32, gameID int32, replierCharacterName string) error

	// NotifyCharacterMention creates a notification when a character is mentioned
	NotifyCharacterMention(ctx context.Context, characterOwnerID int32, commentID int32, gameID int32, mentioningCharacterName string, mentionedCharacterName string) error

	// NotifyActionSubmitted creates notifications for the GM and all co-GMs when a player submits an action
	NotifyActionSubmitted(ctx context.Context, actionID int32, gameID int32, submitterUserID int32, characterName string) error

	// NotifyActionResult creates a notification for player when GM publishes result
	NotifyActionResult(ctx context.Context, playerUserID int32, resultID int32, gameID int32, actionTitle string) error

	// NotifyCommonRoomPost creates notifications for all game participants about new post
	NotifyCommonRoomPost(ctx context.Context, gameID int32, postID int32, postTitle string, excludeUserID int32) error

	// NotifyPhaseCreated creates notifications for all participants when phase created
	NotifyPhaseCreated(ctx context.Context, gameID int32, phaseID int32, phaseTitle string, excludeUserID int32) error

	// NotifyApplicationApproved creates a notification when a game application is approved
	NotifyApplicationApproved(ctx context.Context, playerUserID int32, gameID int32, gameTitle string) error

	// NotifyCharacterApproved creates a notification when a character is approved by the GM
	NotifyCharacterApproved(ctx context.Context, playerUserID int32, gameID int32, characterID int32, characterName string) error

	// NotifyGameStateChanged creates notifications for all participants when game state changes
	NotifyGameStateChanged(ctx context.Context, gameID int32, newState string, gameTitle string, excludeUserID int32) error

	// NotifyHandoutPublished creates notifications for all players when a handout is published
	NotifyHandoutPublished(ctx context.Context, gameID int32, handoutID int32, handoutTitle string, excludeUserID int32) error
}

// StorageBackendInterface defines the contract for file storage operations.
// Supports both local filesystem and cloud storage (S3-compatible).
//
// Usage Example:
//
//	// Local storage
//	localStorage := storage.NewLocalStorage("/var/uploads", "http://localhost:3000/uploads")
//	avatarURL, err := localStorage.Upload(ctx, "avatars/characters/1/avatar.jpg", file, "image/jpeg")
//
//	// S3 storage
//	s3Storage := storage.NewS3Storage("my-bucket", "us-east-1", "https://cdn.example.com")
//	avatarURL, err := s3Storage.Upload(ctx, "avatars/characters/1/avatar.jpg", file, "image/jpeg")
type StorageBackendInterface interface {
	// Upload saves a file to storage and returns its public URL
	// path: relative path within storage (e.g., "avatars/characters/1/avatar.jpg")
	// file: the file data to upload
	// contentType: MIME type of the file (e.g., "image/jpeg")
	Upload(ctx context.Context, path string, file io.Reader, contentType string) (string, error)

	// Delete removes a file from storage
	// path: relative path within storage
	Delete(ctx context.Context, path string) error

	// GetURL returns the public URL for a file path
	// path: relative path within storage
	GetURL(path string) string
}

// AvatarServiceInterface defines the contract for avatar management operations.
// Handles character avatar uploads, deletion, and storage cleanup.
//
// Usage Example:
//
//	avatarService := &services.AvatarService{
//	    DB: pool,
//	    Storage: localStorage,
//	    CharacterService: characterService,
//	}
//
//	// Upload character avatar
//	avatarURL, err := avatarService.UploadCharacterAvatar(ctx, characterID, file, "avatar.jpg", "image/jpeg")
//
//	// Delete character avatar
//	err := avatarService.DeleteCharacterAvatar(ctx, characterID)
type AvatarServiceInterface interface {
	// UploadCharacterAvatar uploads an avatar image for a character
	// Returns the public URL of the uploaded avatar
	// Validates file type (must be image/jpeg, image/png, or image/webp)
	// Validates file size (must be <= 5MB)
	// Deletes previous avatar if exists
	UploadCharacterAvatar(ctx context.Context, characterID int32, file io.Reader, filename string, contentType string) (string, error)

	// DeleteCharacterAvatar removes a character's avatar
	// Deletes the file from storage and updates database
	DeleteCharacterAvatar(ctx context.Context, characterID int32) error
}

// DashboardServiceInterface defines the contract for dashboard data operations.
// Provides aggregated view of user's games, deadlines, and activity for the dashboard page.
//
// Usage Example:
//
//	dashboardService := &services.DashboardService{DB: pool}
//
//	// Get user's complete dashboard data
//	dashboard, err := dashboardService.GetUserDashboard(ctx, userID)
//	if !dashboard.HasGames {
//	    // Redirect user to games listing page
//	}
//
//	// Access urgent games requiring action
//	for _, game := range dashboard.PlayerGames {
//	    if game.IsUrgent {
//	        // Display with urgent styling
//	    }
//	}
type DashboardServiceInterface interface {
	// GetUserDashboard retrieves complete dashboard data for a user
	// Returns aggregated game information, recent activity, and upcoming deadlines
	// If user has no games, returns DashboardData with HasGames = false
	GetUserDashboard(ctx context.Context, userID int32) (*DashboardData, error)
}

// HandoutServiceInterface defines the contract for handout management operations.
// Handouts are GM-created informational documents (rules, world info) that persist across all game phases.
// Only GMs can create/update/delete handouts and comments. Players can view published handouts.
//
// Usage Example:
//
//	handoutService := &services.HandoutService{DB: pool, NotificationService: notifService}
//
//	// GM creates a draft handout
//	handout, err := handoutService.CreateHandout(ctx, gameID, "Character Rules", content, "draft")
//
//	// GM publishes handout (triggers player notifications)
//	handout, err := handoutService.PublishHandout(ctx, handoutID, gmUserID)
//
//	// Player views published handouts
//	handouts, err := handoutService.ListHandouts(ctx, gameID, playerUserID, false)
type HandoutServiceInterface interface {
	// CreateHandout creates a new handout
	// Only GMs can create handouts
	// Status must be "draft" or "published"
	CreateHandout(ctx context.Context, gameID int32, title string, content string, status string, userID int32) (*Handout, error)

	// GetHandout retrieves a handout by ID
	// Returns error if handout is draft and user is not GM
	GetHandout(ctx context.Context, handoutID int32, userID int32) (*Handout, error)

	// ListHandouts retrieves all handouts for a game
	// If isGM is true, returns all handouts (including drafts)
	// If isGM is false, returns only published handouts
	ListHandouts(ctx context.Context, gameID int32, userID int32, isGM bool) ([]*Handout, error)

	// UpdateHandout updates a handout's title, content, and status
	// Only GMs can update handouts
	// Changing status to "published" triggers notifications
	UpdateHandout(ctx context.Context, handoutID int32, title string, content string, status string, userID int32) (*Handout, error)

	// DeleteHandout removes a handout
	// Only GMs can delete handouts
	// Cascade deletes all comments on the handout
	DeleteHandout(ctx context.Context, handoutID int32, userID int32) error

	// PublishHandout changes a draft handout to published
	// Only GMs can publish handouts
	// Triggers notifications to all game participants
	PublishHandout(ctx context.Context, handoutID int32, userID int32) (*Handout, error)

	// UnpublishHandout changes a published handout to draft
	// Only GMs can unpublish handouts
	// Hides handout from players
	UnpublishHandout(ctx context.Context, handoutID int32, userID int32) (*Handout, error)

	// CreateHandoutComment adds a comment to a handout
	// Only GMs can comment on handouts
	// Supports threaded replies via parentCommentID
	CreateHandoutComment(ctx context.Context, handoutID int32, userID int32, parentCommentID *int32, content string) (*HandoutComment, error)

	// ListHandoutComments retrieves all comments for a handout
	// Returns comments in chronological order
	// Excludes deleted comments
	ListHandoutComments(ctx context.Context, handoutID int32) ([]*HandoutComment, error)

	// UpdateHandoutComment updates a comment's content
	// Only the comment author (GM) can update
	// Increments edit count and sets edited_at timestamp
	UpdateHandoutComment(ctx context.Context, commentID int32, userID int32, content string) (*HandoutComment, error)

	// DeleteHandoutComment soft-deletes a comment
	// Only GMs can delete comments
	// Sets deleted_at timestamp and deleted_by_user_id
	DeleteHandoutComment(ctx context.Context, commentID int32, userID int32, isGM bool) error
}

// EmailServiceInterface defines the contract for email sending operations.
// Supports multiple providers (Resend for production, MailHog for development).
//
// Usage Example:
//
//	emailService := &email.EmailService{
//	    Provider: "resend",
//	    ResendAPIKey: os.Getenv("RESEND_API_KEY"),
//	}
//
//	// Send password reset email
//	err := emailService.SendPasswordResetEmail(ctx, "user@example.com", "abc123", "https://app.com/reset?token=abc123")
type EmailServiceInterface interface {
	// SendEmail sends a generic email
	SendEmail(ctx context.Context, req *SendEmailRequest) error

	// SendPasswordResetEmail sends a password reset email with token
	SendPasswordResetEmail(ctx context.Context, email, token, resetURL string) error

	// SendEmailVerificationEmail sends an email verification link
	SendEmailVerificationEmail(ctx context.Context, email, token, verifyURL string) error

	// SendPasswordChangedEmail notifies user of password change
	SendPasswordChangedEmail(ctx context.Context, email string) error

	// SendEmailChangedEmail notifies user of email change
	SendEmailChangedEmail(ctx context.Context, oldEmail, newEmail string) error

	// SendAccountDeletionScheduledEmail notifies user account will be deleted
	SendAccountDeletionScheduledEmail(ctx context.Context, email string, scheduledFor time.Time) error
}

// ==========================================
// Account Security Request/Response Types
// ==========================================

// SendEmailRequest represents a request to send an email
type SendEmailRequest struct {
	To       string
	Subject  string
	HTMLBody string
	TextBody string
}

// ==========================================
// Game Deadlines Service Interface
// ==========================================

// DeadlineServiceInterface defines the contract for game deadline operations.
// Allows GMs to create arbitrary deadlines separate from phase transitions.
// These deadlines can be displayed in game views to help players track important dates.
type DeadlineServiceInterface interface {
	// CreateDeadline creates a new deadline for a game.
	// Only GMs can create deadlines for their games.
	CreateDeadline(ctx context.Context, req CreateDeadlineRequest) (*models.GameDeadline, error)

	// GetDeadline retrieves a specific deadline by ID.
	// Returns error if deadline is not found or has been soft-deleted.
	GetDeadline(ctx context.Context, deadlineID int32) (*models.GameDeadline, error)

	// GetGameDeadlines retrieves all active deadlines for a game.
	// If includeExpired is true, returns expired deadlines as well.
	// Results are ordered by deadline timestamp ascending (soonest first).
	GetGameDeadlines(ctx context.Context, gameID int32, includeExpired bool) ([]models.GameDeadline, error)

	// GetAllGameDeadlines retrieves all deadline types (arbitrary, phase, and poll) for a game.
	// Returns a unified view of all deadline sources sorted chronologically.
	// This aggregates deadlines from game_deadlines, game_phases, and common_room_polls tables.
	GetAllGameDeadlines(ctx context.Context, gameID int32, includeExpired bool) ([]UnifiedDeadline, error)

	// GetUpcomingDeadlines retrieves upcoming deadlines across all user's games.
	// Used for dashboard view to show deadlines from all games the user participates in.
	// Returns deadline with associated game information for context.
	GetUpcomingDeadlines(ctx context.Context, userID int32, limit int32) ([]DeadlineWithGame, error)

	// UpdateDeadline updates deadline details (title, description, timestamp).
	// Only GMs can update deadlines for their games.
	UpdateDeadline(ctx context.Context, deadlineID int32, req UpdateDeadlineRequest) (*models.GameDeadline, error)

	// DeleteDeadline soft-deletes a deadline by setting deleted_at timestamp.
	// Only GMs can delete deadlines for their games.
	// userID is the ID of the user requesting deletion (must be GM).
	DeleteDeadline(ctx context.Context, deadlineID int32, userID int32) error
}

// ==========================================
// Game Deadlines Request/Response Types
// ==========================================

// CreateDeadlineRequest represents parameters for creating a deadline.
type CreateDeadlineRequest struct {
	GameID      int32
	Title       string
	Description string
	Deadline    time.Time
	CreatedBy   int32
}

// UpdateDeadlineRequest represents parameters for updating a deadline.
type UpdateDeadlineRequest struct {
	Title       string
	Description string
	Deadline    time.Time
}

// DeadlineWithGame includes game context for cross-game deadline views.
// Used when displaying deadlines from multiple games (e.g., dashboard view).
type DeadlineWithGame struct {
	models.GameDeadline
	GameTitle string
	GameID    int32
}

// ==========================================
// Common Room Polling System
// ==========================================

// PollServiceInterface defines the contract for poll management operations.
// Enables GMs to create polls for player consensus-building in the common room.
type PollServiceInterface interface {
	// CreatePollWithOptions creates a new poll with its options in a transaction.
	// Only GMs can create polls for their games.
	// Returns the created poll along with its options.
	CreatePollWithOptions(ctx context.Context, req CreatePollRequest) (*PollWithOptions, error)

	// GetPoll retrieves a specific poll by ID.
	// Returns error if poll is not found or has been soft-deleted.
	GetPoll(ctx context.Context, pollID int32) (*models.CommonRoomPoll, error)

	// GetPollWithOptions retrieves a poll with all its options.
	GetPollWithOptions(ctx context.Context, pollID int32) (*PollWithOptions, error)

	// ListPollsByPhase retrieves all active polls for a specific game phase.
	// Results are ordered by creation time descending (newest first).
	ListPollsByPhase(ctx context.Context, gameID int32, phaseID int32) ([]models.CommonRoomPoll, error)

	// ListPollsByGame retrieves all active polls for a game.
	// If includeExpired is true, returns expired polls as well.
	// Results are ordered by deadline ascending (soonest first).
	ListPollsByGame(ctx context.Context, gameID int32, includeExpired bool) ([]models.CommonRoomPoll, error)

	// SubmitVote submits or updates a user's vote for a poll.
	// Enforces one vote per user per poll (with or without character context).
	SubmitVote(ctx context.Context, req SubmitVoteRequest) (*models.PollVote, error)

	// GetVote retrieves a user's vote for a poll.
	// Returns nil if user hasn't voted yet.
	GetVote(ctx context.Context, pollID int32, userID int32) (*models.PollVote, error)

	// GetPollResults retrieves aggregated results for a poll.
	// Returns vote counts per option and respects show_individual_votes setting.
	// If canSeeIndividualVotes is true (GM, co-GM, or audience), individual votes are always included.
	GetPollResults(ctx context.Context, pollID int32, canSeeIndividualVotes bool) (*PollResults, error)

	// UpdatePoll updates poll details (question, description, deadline, settings).
	// Only GMs can update polls for their games.
	// Cannot update after votes have been cast (enforced at handler level).
	UpdatePoll(ctx context.Context, pollID int32, req UpdatePollRequest) (*models.CommonRoomPoll, error)

	// DeletePoll soft-deletes a poll by setting is_deleted flag.
	// Only GMs can delete polls for their games.
	DeletePoll(ctx context.Context, pollID int32) error

	// HasUserVoted checks if a user has already voted in a poll.
	// Useful for validation before rendering vote UI.
	HasUserVoted(ctx context.Context, pollID int32, userID int32) (bool, error)
}

// ==========================================
// Polling System Request/Response Types
// ==========================================

// CreatePollRequest represents parameters for creating a poll with options.
type CreatePollRequest struct {
	GameID               int32
	PhaseID              *int32 // Optional - poll can be phase-specific or game-wide
	CreatedByUserID      int32  // GM creating the poll
	CreatedByCharacterID *int32 // Optional - GM can create as a character
	Question             string
	Description          *string // Optional
	Deadline             time.Time
	ShowIndividualVotes  bool
	AllowOtherOption     bool
	// HideResultsFromPlayers hides results from players permanently — even after
	// the deadline. Mutually exclusive with ShowIndividualVotes.
	HideResultsFromPlayers bool
	// AllowAudienceVoting permits audience members to vote; their votes are
	// attributed anonymously since they may not have characters.
	AllowAudienceVoting bool
	// ShowRunningTotalsToPlayers lets players see results while voting is still
	// open. Vote attribution still follows ShowIndividualVotes: tallies only by
	// default, per-voter detail when that flag is also set. Mutually exclusive
	// with HideResultsFromPlayers.
	ShowRunningTotalsToPlayers bool
	Options                    []PollOptionInput // List of poll options
}

// PollOptionInput represents a single poll option during creation.
type PollOptionInput struct {
	Text         string
	DisplayOrder int32
}

// UpdatePollRequest represents parameters for updating a poll.
type UpdatePollRequest struct {
	Question                   string
	Description                *string
	Deadline                   time.Time
	ShowIndividualVotes        bool
	AllowOtherOption           bool
	HideResultsFromPlayers     bool
	AllowAudienceVoting        bool
	ShowRunningTotalsToPlayers bool
}

// SubmitVoteRequest represents parameters for submitting a vote.
type SubmitVoteRequest struct {
	PollID           int32
	UserID           int32
	SelectedOptionID *int32  // Mutually exclusive with OtherResponse
	OtherResponse    *string // Mutually exclusive with SelectedOptionID
}

// PollWithOptions includes poll and its options together.
type PollWithOptions struct {
	Poll    models.CommonRoomPoll
	Options []models.PollOption
}

// PollResults contains aggregated voting results.
type PollResults struct {
	Poll                models.CommonRoomPoll
	OptionResults       []OptionResult
	OtherResponses      []OtherResponse
	TotalVotes          int32
	ShowIndividualVotes bool
}

// OptionResult represents vote count for a specific option.
type OptionResult struct {
	Option    models.PollOption
	VoteCount int32
	Voters    []VoterInfo // Only populated if show_individual_votes is true
}

// OtherResponse represents a custom "other" response.
type OtherResponse struct {
	VoteID        int32
	OtherText     string
	Username      string
	CharacterName *string
	// IsAnonymous marks a response cast by an audience member. Audience identity
	// is never disclosed, so callers must not surface Username or CharacterName.
	IsAnonymous bool
}

// VoterInfo represents a user who voted (for individual vote display).
type VoterInfo struct {
	UserID        int32
	Username      string
	CharacterName *string
	// IsAnonymous marks a vote cast by an audience member. Audience identity is
	// never disclosed, so callers must not surface Username or CharacterName.
	IsAnonymous bool
}

// ==============================================================================
// User Profile System
// ==============================================================================

// UserProfile represents a user's public profile information.
type UserProfile struct {
	ID          int32     `json:"id"`
	Username    string    `json:"username"`
	DisplayName *string   `json:"display_name"`
	Bio         *string   `json:"bio"`
	AvatarURL   *string   `json:"avatar_url"`
	CreatedAt   time.Time `json:"created_at"`
	Timezone    string    `json:"timezone"`
	IsAdmin     bool      `json:"is_admin"`
}

// UserGame represents a game the user has participated in.
type UserGame struct {
	GameID      int32               `json:"game_id"`
	Title       string              `json:"title"`
	State       string              `json:"state"`
	IsAnonymous bool                `json:"is_anonymous"`
	UserRole    string              `json:"user_role"` // "player", "co_gm", "audience"
	GMUsername  string              `json:"gm_username"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
	StartDate   *time.Time          `json:"start_date"`
	EndDate     *time.Time          `json:"end_date"`
	Characters  []UserGameCharacter `json:"characters"` // Empty for anonymous games
}

// UserGameCharacter represents a character the user played in a game.
type UserGameCharacter struct {
	ID            int32   `json:"id"`
	Name          string  `json:"name"`
	AvatarURL     *string `json:"avatar_url"`
	CharacterType string  `json:"character_type"`
}

// UserGameHistoryMetadata provides pagination context for user game history.
type UserGameHistoryMetadata struct {
	Page            int  `json:"page"`              // Current page number (1-indexed)
	PageSize        int  `json:"page_size"`         // Number of items per page
	TotalPages      int  `json:"total_pages"`       // Total number of pages
	TotalCount      int  `json:"total_count"`       // Total count of all games
	HasNextPage     bool `json:"has_next_page"`     // Whether there's a next page
	HasPreviousPage bool `json:"has_previous_page"` // Whether there's a previous page
}

// UserProfileResponse is the complete response for a user profile.
type UserProfileResponse struct {
	User     UserProfile             `json:"user"`
	Games    []UserGame              `json:"games"`
	Metadata UserGameHistoryMetadata `json:"metadata"`
}

// UserProfileServiceInterface defines the contract for user profile operations.
type UserProfileServiceInterface interface {
	// GetUserProfile retrieves a user's profile and game history with pagination.
	GetUserProfile(ctx context.Context, userID int32, page, pageSize int) (*UserProfileResponse, error)

	// GetUserGames retrieves all games a user has participated in with pagination.
	// Applies privacy filtering for anonymous games.
	GetUserGames(ctx context.Context, userID int32, limit, offset int) ([]UserGame, error)

	// UpdateUserProfile updates a user's display name and/or bio.
	// Nil values are ignored.
	UpdateUserProfile(ctx context.Context, userID int32, displayName *string, bio *string) error
}

// DiscordEmbed is the payload for a Discord embed DM.
type DiscordEmbed struct {
	Title       string // Displayed as the embed title (plain text, not a link)
	URL         string // Makes the title a clickable hyperlink
	Description string // Body text beneath the title (optional)
	Color       int    // Left-border color as a decimal integer (e.g. 0x5865F2 = 5793266)
	Footer      string // Small footer text
	Timestamp   string // ISO 8601 timestamp shown in footer
}

// DiscordClientInterface defines the contract for sending Discord DMs.
type DiscordClientInterface interface {
	// SendDM sends a rich embed DM to a Discord user by their Discord user ID.
	SendDM(ctx context.Context, discordUserID string, embed DiscordEmbed) error
}

// DiscordAccount represents a linked Discord account for a user.
type DiscordAccount struct {
	ID              int32      `json:"id"`
	UserID          int32      `json:"user_id"`
	DiscordUserID   string     `json:"discord_user_id"`
	DiscordUsername string     `json:"discord_username"`
	AccessToken     string     `json:"-"` // Never expose tokens in JSON
	RefreshToken    *string    `json:"-"`
	TokenExpiresAt  *time.Time `json:"-"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// UpsertDiscordAccountRequest contains the data needed to link/update a Discord account.
type UpsertDiscordAccountRequest struct {
	UserID          int32
	DiscordUserID   string
	DiscordUsername string
	AccessToken     string
	RefreshToken    *string
	TokenExpiresAt  *time.Time
}

// DiscordAccountServiceInterface defines the contract for Discord account management.
type DiscordAccountServiceInterface interface {
	// GetDiscordAccount retrieves a user's linked Discord account
	// Returns nil, nil if no Discord account is linked
	GetDiscordAccount(ctx context.Context, userID int32) (*DiscordAccount, error)

	// UpsertDiscordAccount creates or updates a user's Discord account link
	UpsertDiscordAccount(ctx context.Context, req *UpsertDiscordAccountRequest) (*DiscordAccount, error)

	// DeleteDiscordAccount removes a user's Discord account link
	DeleteDiscordAccount(ctx context.Context, userID int32) error
}

// UserAvatarServiceInterface defines the contract for user avatar operations.
type UserAvatarServiceInterface interface {
	// UploadUserAvatar uploads an avatar image for a user.
	// Returns the public URL of the uploaded avatar.
	UploadUserAvatar(ctx context.Context, userID int32, file io.Reader, filename string, contentType string) (string, error)

	// DeleteUserAvatar removes a user's avatar.
	DeleteUserAvatar(ctx context.Context, userID int32) error
}

// CharacterServiceInterface defines the contract for character management operations.
type CharacterServiceInterface interface {
	CreateCharacter(ctx context.Context, req CreateCharacterRequest) (*models.Character, error)
	CreateGamemasterNPC(ctx context.Context, gameID int32) error
	RenameCharacter(ctx context.Context, characterID int32, newName string) (*models.Character, error)
	GetCharacter(ctx context.Context, characterID int32) (*models.Character, error)
	GetCharactersByGame(ctx context.Context, gameID int32) ([]models.GetCharactersByGameRow, error)
	GetPlayerCharacters(ctx context.Context, gameID int32) ([]models.GetPlayerCharactersByGameRow, error)
	GetNPCs(ctx context.Context, gameID int32) ([]models.GetNPCsByGameRow, error)
	GetUserControllableCharacters(ctx context.Context, gameID, userID int32) ([]models.GetUserControllableCharactersRow, error)
	// GetUserControllableCharactersAcrossGames returns the user's controllable
	// characters in all in_progress games, each carrying the game context its
	// sheet permissions depend on. Backs surfaces with no game in scope, such as
	// the global Utility Drawer.
	GetUserControllableCharactersAcrossGames(ctx context.Context, userID int32) ([]models.GetUserControllableCharactersAcrossGamesRow, error)
	ApproveCharacter(ctx context.Context, characterID int32) (*models.Character, error)
	AssignNPCToUser(ctx context.Context, characterID, assignedUserID, assignedByUserID int32) error
	SetCharacterData(ctx context.Context, req CharacterDataRequest) error
	GetCharacterData(ctx context.Context, characterID int32) ([]models.CharacterDatum, error)
	GetCharacterDataByModule(ctx context.Context, characterID int32, moduleType string) ([]models.CharacterDatum, error)
	GetPublicCharacterData(ctx context.Context, characterID int32) ([]models.CharacterDatum, error)
	CanUserEditCharacter(ctx context.Context, characterID, userID int32) (bool, error)
	ReassignCharacter(ctx context.Context, characterID, newOwnerUserID int32) (*models.Character, error)
	ListInactiveCharacters(ctx context.Context, gameID int32) ([]models.ListInactiveCharactersRow, error)
	DeactivatePlayerCharacters(ctx context.Context, gameID, userID int32) error
	DeleteCharacter(ctx context.Context, characterID int32) error
	ListAudienceNPCs(ctx context.Context, gameID int32) ([]models.ListAudienceNPCsRow, error)
	GetCharacterActivityStats(ctx context.Context, characterID int32) (*CharacterActivityStats, error)
	GetCharacterActivityStatsByGame(ctx context.Context, gameID int32) (map[int32]*CharacterActivityStats, error)
	AssignNPCToAudience(ctx context.Context, characterID, assignedUserID, assignedByUserID int32) (*models.NpcAssignment, error)
}

// UserPreferencesServiceInterface defines the contract for user preferences operations.
type UserPreferencesServiceInterface interface {
	GetUserPreferences(ctx context.Context, userID int32) (*PreferencesData, error)
	UpdateUserPreferences(ctx context.Context, userID int32, prefs PreferencesData) (*PreferencesData, error)
}

// ConversationServiceInterface defines the contract for private conversation operations.
type ConversationServiceInterface interface {
	CreateConversation(ctx context.Context, req CreateConversationRequest) (*models.Conversation, error)
	GetUserConversations(ctx context.Context, gameID int32, userID int32) ([]models.GetUserConversationsRow, error)
	GetUserUnreadConversations(ctx context.Context, gameID int32, userID int32, limit int32) ([]models.GetUserUnreadConversationsRow, error)
	GetConversationParticipants(ctx context.Context, conversationID int32) ([]models.GetConversationParticipantsRow, error)
	SendMessage(ctx context.Context, req SendConversationMessageRequest) (*models.PrivateMessage, error)
	GetConversationMessages(ctx context.Context, conversationID int32, userID int32) ([]models.GetConversationMessagesRow, error)
	MarkConversationAsRead(ctx context.Context, conversationID int32, userID int32) error
	AddParticipant(ctx context.Context, conversationID int32, characterID int32) error
	UpdatePrivateMessage(ctx context.Context, messageID int32, userID int32, content string) (*models.PrivateMessage, error)
	DeletePrivateMessage(ctx context.Context, messageID int32, userID int32) error
	CanUserAccessConversation(ctx context.Context, conversationID int32, userID int32, isAdmin bool) (bool, error)
	GetConversation(ctx context.Context, conversationID int32) (*models.Conversation, error)
	GetPrivateMessage(ctx context.Context, messageID int32) (*models.PrivateMessage, error)

	// DeleteConversation permanently removes a conversation that has never had a
	// message sent in it. Only the creator or a GM/co-GM of the game may do so.
	// Returns ErrConversationNotEmpty if any message exists (including
	// soft-deleted ones) and ErrConversationDeleteForbidden if the user is not
	// authorized.
	DeleteConversation(ctx context.Context, conversationID int32, userID int32) error
}
