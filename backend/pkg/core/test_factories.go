package core

import (
	db "actionphase/pkg/db/models"
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

// TestDataFactory provides convenient methods to create test objects
type TestDataFactory struct {
	db       *TestDatabase
	t        TestingInterface
	sequence int // For generating unique values
}

// NewTestDataFactory creates a new test data factory
func NewTestDataFactory(db *TestDatabase, t TestingInterface) *TestDataFactory {
	return &TestDataFactory{
		db:       db,
		t:        t,
		sequence: 1,
	}
}

// UserBuilder provides a fluent interface for building test users
type UserBuilder struct {
	factory  *TestDataFactory
	username string
	email    string
	password string
	isAdmin  bool
}

// NewUser starts building a new user with default values
func (f *TestDataFactory) NewUser() *UserBuilder {
	seq := f.nextSequence()
	return &UserBuilder{
		factory:  f,
		username: fmt.Sprintf("testuser%d", seq),
		email:    fmt.Sprintf("testuser%d@example.com", seq),
		password: "testpassword123",
		isAdmin:  false,
	}
}

func (b *UserBuilder) WithUsername(username string) *UserBuilder {
	b.username = username
	return b
}

func (b *UserBuilder) WithEmail(email string) *UserBuilder {
	b.email = email
	return b
}

func (b *UserBuilder) WithPassword(password string) *UserBuilder {
	b.password = password
	return b
}

func (b *UserBuilder) AsAdmin() *UserBuilder {
	b.isAdmin = true
	return b
}

// Create persists the user to the database and returns the created user
func (b *UserBuilder) Create() db.User {
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(b.password), bcrypt.DefaultCost)
	if err != nil {
		b.factory.t.Fatalf("Should be able to hash password: %v", err)
	}

	params := db.CreateUserParams{
		Username: b.username,
		Email:    b.email,
		Password: string(hashedPassword),
	}

	queries := db.New(b.factory.db.Pool)
	user, err := queries.CreateUser(context.Background(), params)
	if err != nil {
		b.factory.t.Fatalf("Should be able to create test user: %v", err)
	}

	// Verified, matching CreateTestUser: write endpoints gate on
	// RequireVerifiedEmail(Ctx), so an unverified builder user is forbidden from
	// every action a test would want it to take.
	if err := queries.MarkUserEmailVerified(context.Background(), user.ID); err != nil {
		b.factory.t.Fatalf("Should be able to mark test user email verified: %v", err)
	}
	user.EmailVerified = true

	return user
}

// Build returns the user data without persisting to database
func (b *UserBuilder) Build() User {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(b.password), bcrypt.DefaultCost)
	return User{
		Username: b.username,
		Email:    b.email,
		Password: string(hashedPassword),
	}
}

// GameBuilder provides a fluent interface for building test games
type GameBuilder struct {
	factory             *TestDataFactory
	title               string
	description         string
	gmUserID            int32
	genre               string
	state               string
	maxPlayers          int32
	isPublic            bool
	isAnonymous         bool
	startDate           *time.Time
	endDate             *time.Time
	recruitmentDeadline *time.Time
}

// NewGame starts building a new game with default values
func (f *TestDataFactory) NewGame() *GameBuilder {
	seq := f.nextSequence()
	return &GameBuilder{
		factory:     f,
		title:       fmt.Sprintf("Test Game %d", seq),
		description: fmt.Sprintf("Description for test game %d", seq),
		gmUserID:    1, // Default GM user ID
		genre:       "Fantasy",
		state:       "setup",
		maxPlayers:  4,
		isPublic:    true,
	}
}

func (b *GameBuilder) WithTitle(title string) *GameBuilder {
	b.title = title
	return b
}

func (b *GameBuilder) WithDescription(description string) *GameBuilder {
	b.description = description
	return b
}

func (b *GameBuilder) WithGM(gmUserID int32) *GameBuilder {
	b.gmUserID = gmUserID
	return b
}

func (b *GameBuilder) WithGenre(genre string) *GameBuilder {
	b.genre = genre
	return b
}

func (b *GameBuilder) WithState(state string) *GameBuilder {
	b.state = state
	return b
}

func (b *GameBuilder) WithMaxPlayers(maxPlayers int32) *GameBuilder {
	b.maxPlayers = maxPlayers
	return b
}

func (b *GameBuilder) AsPrivate() *GameBuilder {
	b.isPublic = false
	return b
}

func (b *GameBuilder) WithAnonymous() *GameBuilder {
	b.isAnonymous = true
	return b
}

func (b *GameBuilder) WithStartDate(startDate time.Time) *GameBuilder {
	b.startDate = &startDate
	return b
}

func (b *GameBuilder) WithEndDate(endDate time.Time) *GameBuilder {
	b.endDate = &endDate
	return b
}

func (b *GameBuilder) WithRecruitmentDeadline(deadline time.Time) *GameBuilder {
	b.recruitmentDeadline = &deadline
	return b
}

// Create persists the game to the database and returns the created game
func (b *GameBuilder) Create() db.Game {
	params := db.CreateGameParams{
		Title:       b.title,
		Description: pgtype.Text{String: b.description, Valid: true},
		GmUserID:    b.gmUserID,
		Genre:       pgtype.Text{String: b.genre, Valid: true},
		MaxPlayers:  pgtype.Int4{Int32: b.maxPlayers, Valid: true},
		IsPublic:    pgtype.Bool{Bool: b.isPublic, Valid: true},
		IsAnonymous: b.isAnonymous,
	}

	if b.startDate != nil {
		params.StartDate = pgtype.Timestamptz{Time: *b.startDate, Valid: true}
	}

	if b.endDate != nil {
		params.EndDate = pgtype.Timestamptz{Time: *b.endDate, Valid: true}
	}

	if b.recruitmentDeadline != nil {
		params.RecruitmentDeadline = pgtype.Timestamptz{Time: *b.recruitmentDeadline, Valid: true}
	}

	queries := db.New(b.factory.db.Pool)
	game, err := queries.CreateGame(context.Background(), params)
	if err != nil {
		b.factory.t.Fatalf("Should be able to create test game: %v", err)
	}

	// Update game state if different from default
	if b.state != "setup" {
		updateParams := db.UpdateGameStateParams{
			ID:    game.ID,
			State: pgtype.Text{String: b.state, Valid: true},
		}
		game, err = queries.UpdateGameState(context.Background(), updateParams)
		if err != nil {
			b.factory.t.Fatalf("Should be able to update game state: %v", err)
		}
	}

	return game
}

// SessionBuilder provides a fluent interface for building test sessions
type SessionBuilder struct {
	factory *TestDataFactory
	userID  int32
	data    string
	expires *time.Time
}

// NewSession starts building a new session with default values
func (f *TestDataFactory) NewSession() *SessionBuilder {
	seq := f.nextSequence()
	return &SessionBuilder{
		factory: f,
		userID:  1, // Default user ID
		data:    fmt.Sprintf("test_session_token_%d", seq),
	}
}

func (b *SessionBuilder) WithUserID(userID int32) *SessionBuilder {
	b.userID = userID
	return b
}

func (b *SessionBuilder) WithData(data string) *SessionBuilder {
	b.data = data
	return b
}

func (b *SessionBuilder) WithExpiry(expires time.Time) *SessionBuilder {
	b.expires = &expires
	return b
}

func (b *SessionBuilder) ExpiringIn(duration time.Duration) *SessionBuilder {
	expiry := time.Now().Add(duration)
	b.expires = &expiry
	return b
}

// Create persists the session to the database and returns the created session
func (b *SessionBuilder) Create() db.Session {
	params := db.CreateSessionParams{
		UserID: b.userID,
		Data:   b.data,
	}

	if b.expires != nil {
		params.Expires = pgtype.Timestamptz{Time: *b.expires, Valid: true}
	} else {
		// Default to 24 hours from now
		params.Expires = pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true}
	}

	queries := db.New(b.factory.db.Pool)
	session, err := queries.CreateSession(context.Background(), params)
	if err != nil {
		b.factory.t.Fatalf("Should be able to create test session: %v", err)
	}

	return session
}

// GameParticipantBuilder provides a fluent interface for building test game participants
type GameParticipantBuilder struct {
	factory *TestDataFactory
	gameID  int32
	userID  int32
	role    string
	status  string
}

// NewGameParticipant starts building a new game participant with default values
func (f *TestDataFactory) NewGameParticipant() *GameParticipantBuilder {
	return &GameParticipantBuilder{
		factory: f,
		gameID:  1, // Default game ID
		userID:  1, // Default user ID
		role:    "player",
		status:  "active",
	}
}

func (b *GameParticipantBuilder) ForGame(gameID int32) *GameParticipantBuilder {
	b.gameID = gameID
	return b
}

func (b *GameParticipantBuilder) WithUser(userID int32) *GameParticipantBuilder {
	b.userID = userID
	return b
}

func (b *GameParticipantBuilder) WithRole(role string) *GameParticipantBuilder {
	b.role = role
	return b
}

func (b *GameParticipantBuilder) WithStatus(status string) *GameParticipantBuilder {
	b.status = status
	return b
}

func (b *GameParticipantBuilder) AsGM() *GameParticipantBuilder {
	b.role = "gm"
	return b
}

func (b *GameParticipantBuilder) AsPlayer() *GameParticipantBuilder {
	b.role = "player"
	return b
}

func (b *GameParticipantBuilder) AsObserver() *GameParticipantBuilder {
	b.role = "observer"
	return b
}

// Create persists the game participant to the database and returns the created participant
func (b *GameParticipantBuilder) Create() db.GameParticipant {
	params := db.AddGameParticipantParams{
		GameID: b.gameID,
		UserID: b.userID,
		Role:   b.role,
	}

	queries := db.New(b.factory.db.Pool)
	participant, err := queries.AddGameParticipant(context.Background(), params)
	if err != nil {
		b.factory.t.Fatalf("Should be able to create test game participant: %v", err)
	}

	// Update status if different from default
	if b.status != "active" {
		updateParams := db.UpdateParticipantStatusParams{
			GameID: b.gameID,
			UserID: b.userID,
			Status: pgtype.Text{String: b.status, Valid: true},
		}
		participant, err = queries.UpdateParticipantStatus(context.Background(), updateParams)
		if err != nil {
			b.factory.t.Fatalf("Should be able to update participant status: %v", err)
		}
	}

	return participant
}

// Convenience methods for creating common test scenarios

// CreateUserWithGame creates a user and a game they own
func (f *TestDataFactory) CreateUserWithGame() (db.User, db.Game) {
	user := f.NewUser().Create()
	game := f.NewGame().WithGM(user.ID).Create()
	return user, game
}

// CreateGameWithParticipants creates a game with specified number of participants
func (f *TestDataFactory) CreateGameWithParticipants(numParticipants int) (db.Game, []db.User, []db.GameParticipant) {
	// Create GM
	gm := f.NewUser().WithUsername("gm").WithEmail("gm@test.com").Create()
	game := f.NewGame().WithGM(gm.ID).Create()

	// Create participants
	users := make([]db.User, numParticipants)
	participants := make([]db.GameParticipant, numParticipants)

	for i := 0; i < numParticipants; i++ {
		user := f.NewUser().
			WithUsername(fmt.Sprintf("player%d", i+1)).
			WithEmail(fmt.Sprintf("player%d@test.com", i+1)).
			Create()

		participant := f.NewGameParticipant().
			ForGame(game.ID).
			WithUser(user.ID).
			AsPlayer().
			Create()

		users[i] = user
		participants[i] = participant
	}

	return game, users, participants
}

// CreateAuthenticatedUser creates a user and an associated session for authentication testing
func (f *TestDataFactory) CreateAuthenticatedUser() (db.User, db.Session) {
	user := f.NewUser().Create()
	session := f.NewSession().
		WithUserID(user.ID).
		ExpiringIn(24 * time.Hour).
		Create()

	return user, session
}

// Helper method to get next sequence number
func (f *TestDataFactory) nextSequence() int {
	seq := f.sequence
	f.sequence++
	return seq
}

// ResetSequence resets the sequence counter for predictable test data
func (f *TestDataFactory) ResetSequence() {
	f.sequence = 1
}

// Batch creation methods for performance testing

// CreateUsersInBatch creates multiple users efficiently
func (f *TestDataFactory) CreateUsersInBatch(count int) []db.User {
	users := make([]db.User, count)

	for i := 0; i < count; i++ {
		users[i] = f.NewUser().
			WithUsername("batch_user_" + strconv.Itoa(i)).
			WithEmail("batch_user_" + strconv.Itoa(i) + "@test.com").
			Create()
	}

	return users
}

// CreateGamesInBatch creates multiple games efficiently
func (f *TestDataFactory) CreateGamesInBatch(count int, gmUserID int32) []db.Game {
	games := make([]db.Game, count)

	for i := 0; i < count; i++ {
		games[i] = f.NewGame().
			WithTitle("Batch Game " + strconv.Itoa(i)).
			WithGM(gmUserID).
			Create()
	}

	return games
}

// CharacterBuilder provides a fluent interface for building test characters
type CharacterBuilder struct {
	factory       *TestDataFactory
	gameID        int32
	userID        *int32
	name          string
	characterType string
	status        string
}

// NewCharacter starts building a new character with default values
func (f *TestDataFactory) NewCharacter() *CharacterBuilder {
	seq := f.nextSequence()
	return &CharacterBuilder{
		factory:       f,
		gameID:        1, // Default game ID
		userID:        nil,
		name:          fmt.Sprintf("Test Character %d", seq),
		characterType: "player_character",
		status:        "pending",
	}
}

func (b *CharacterBuilder) InGame(game db.Game) *CharacterBuilder {
	b.gameID = game.ID
	return b
}

func (b *CharacterBuilder) ForGame(gameID int32) *CharacterBuilder {
	b.gameID = gameID
	return b
}

func (b *CharacterBuilder) OwnedBy(user db.User) *CharacterBuilder {
	userID := user.ID
	b.userID = &userID
	return b
}

func (b *CharacterBuilder) WithUserID(userID int32) *CharacterBuilder {
	b.userID = &userID
	return b
}

func (b *CharacterBuilder) GMControlled() *CharacterBuilder {
	b.userID = nil
	return b
}

func (b *CharacterBuilder) WithName(name string) *CharacterBuilder {
	b.name = name
	return b
}

func (b *CharacterBuilder) WithCharacterType(characterType string) *CharacterBuilder {
	b.characterType = characterType
	return b
}

func (b *CharacterBuilder) PlayerCharacter() *CharacterBuilder {
	b.characterType = "player_character"
	return b
}

func (b *CharacterBuilder) NPC() *CharacterBuilder {
	b.characterType = "npc"
	b.userID = nil
	return b
}

// Deprecated: Use NPC() instead. NPC type consolidation removed distinction between GM and audience NPCs.
func (b *CharacterBuilder) NPCGMControlled() *CharacterBuilder {
	return b.NPC()
}

// Deprecated: Use NPC() instead. NPC type consolidation removed distinction between GM and audience NPCs.
func (b *CharacterBuilder) NPCAudience() *CharacterBuilder {
	return b.NPC()
}

func (b *CharacterBuilder) WithStatus(status string) *CharacterBuilder {
	b.status = status
	return b
}

func (b *CharacterBuilder) Pending() *CharacterBuilder {
	b.status = "pending"
	return b
}

func (b *CharacterBuilder) Approved() *CharacterBuilder {
	b.status = "approved"
	return b
}

func (b *CharacterBuilder) Rejected() *CharacterBuilder {
	b.status = "rejected"
	return b
}

// Create persists the character to the database and returns the created character
func (b *CharacterBuilder) Create() db.Character {
	params := db.CreateCharacterParams{
		GameID:        b.gameID,
		Name:          b.name,
		CharacterType: b.characterType,
		Status:        pgtype.Text{String: b.status, Valid: true},
	}

	if b.userID != nil {
		params.UserID = pgtype.Int4{Int32: *b.userID, Valid: true}
	}

	queries := db.New(b.factory.db.Pool)
	character, err := queries.CreateCharacter(context.Background(), params)
	if err != nil {
		b.factory.t.Fatalf("Should be able to create test character: %v", err)
	}

	return character
}

// PhaseBuilder provides a fluent interface for building test game phases
type PhaseBuilder struct {
	factory     *TestDataFactory
	gameID      int32
	phaseType   string
	phaseNumber *int32
	title       string
	description string
	startTime   *time.Time
	endTime     *time.Time
	deadline    *time.Time
	isActive    bool
}

// NewPhase starts building a new phase with default values
func (f *TestDataFactory) NewPhase() *PhaseBuilder {
	seq := f.nextSequence()
	return &PhaseBuilder{
		factory:     f,
		gameID:      1, // Default game ID
		phaseType:   PhaseTypeCommonRoom,
		phaseNumber: nil, // Will auto-increment if not set
		title:       fmt.Sprintf("Test Phase %d", seq),
		description: "",
		isActive:    false,
	}
}

func (b *PhaseBuilder) InGame(game db.Game) *PhaseBuilder {
	b.gameID = game.ID
	return b
}

func (b *PhaseBuilder) ForGame(gameID int32) *PhaseBuilder {
	b.gameID = gameID
	return b
}

func (b *PhaseBuilder) WithPhaseType(phaseType string) *PhaseBuilder {
	b.phaseType = phaseType
	return b
}

func (b *PhaseBuilder) CommonRoom() *PhaseBuilder {
	b.phaseType = PhaseTypeCommonRoom
	return b
}

func (b *PhaseBuilder) ActionPhase() *PhaseBuilder {
	b.phaseType = PhaseTypeAction
	return b
}

func (b *PhaseBuilder) InterludePhase() *PhaseBuilder {
	b.phaseType = PhaseTypeInterlude
	return b
}

func (b *PhaseBuilder) WithPhaseNumber(number int32) *PhaseBuilder {
	b.phaseNumber = &number
	return b
}

func (b *PhaseBuilder) WithTitle(title string) *PhaseBuilder {
	b.title = title
	return b
}

func (b *PhaseBuilder) WithDescription(description string) *PhaseBuilder {
	b.description = description
	return b
}

func (b *PhaseBuilder) WithStartTime(startTime time.Time) *PhaseBuilder {
	b.startTime = &startTime
	return b
}

func (b *PhaseBuilder) WithEndTime(endTime time.Time) *PhaseBuilder {
	b.endTime = &endTime
	return b
}

func (b *PhaseBuilder) WithDeadline(deadline time.Time) *PhaseBuilder {
	b.deadline = &deadline
	return b
}

// WithDeadlineIn sets the deadline to a duration from now
func (b *PhaseBuilder) WithDeadlineIn(duration time.Duration) *PhaseBuilder {
	deadline := time.Now().Add(duration)
	b.deadline = &deadline
	return b
}

// WithTimeRange sets start time to now and end time to duration from now
func (b *PhaseBuilder) WithTimeRange(duration time.Duration) *PhaseBuilder {
	now := time.Now()
	end := now.Add(duration)
	b.startTime = &now
	b.endTime = &end
	return b
}

func (b *PhaseBuilder) Active() *PhaseBuilder {
	b.isActive = true
	return b
}

func (b *PhaseBuilder) Inactive() *PhaseBuilder {
	b.isActive = false
	return b
}

// Create persists the phase to the database and returns the created phase
func (b *PhaseBuilder) Create() db.GamePhase {
	// Auto-generate phase number if not set
	phaseNumber := int32(1)
	if b.phaseNumber != nil {
		phaseNumber = *b.phaseNumber
	} else {
		// Get next phase number for this game
		queries := db.New(b.factory.db.Pool)
		phases, err := queries.GetGamePhases(context.Background(), b.gameID)
		if err == nil && len(phases) > 0 {
			// Find max phase number
			maxPhaseNumber := int32(0)
			for _, p := range phases {
				if p.PhaseNumber > maxPhaseNumber {
					maxPhaseNumber = p.PhaseNumber
				}
			}
			phaseNumber = maxPhaseNumber + 1
		}
	}

	params := db.CreateGamePhaseParams{
		GameID:      b.gameID,
		PhaseType:   b.phaseType,
		PhaseNumber: phaseNumber,
		Title:       b.title,
	}

	if b.description != "" {
		params.Description = pgtype.Text{String: b.description, Valid: true}
	}

	if b.startTime != nil {
		params.StartTime = pgtype.Timestamptz{Time: *b.startTime, Valid: true}
	}

	if b.endTime != nil {
		params.EndTime = pgtype.Timestamptz{Time: *b.endTime, Valid: true}
	}

	if b.deadline != nil {
		params.Deadline = pgtype.Timestamptz{Time: *b.deadline, Valid: true}
	}

	queries := db.New(b.factory.db.Pool)
	phase, err := queries.CreateGamePhase(context.Background(), params)
	if err != nil {
		b.factory.t.Fatalf("Should be able to create test phase: %v", err)
	}

	// Activate if requested
	if b.isActive {
		phase, err = queries.ActivatePhase(context.Background(), phase.ID)
		if err != nil {
			b.factory.t.Fatalf("Should be able to activate test phase: %v", err)
		}
	}

	return phase
}

// ActionSubmissionBuilder provides a fluent interface for building test action submissions
type ActionSubmissionBuilder struct {
	factory     *TestDataFactory
	gameID      int32
	userID      int32
	phaseID     int32
	characterID *int32
	content     string
}

// NewActionSubmission starts building a new action submission with default values
func (f *TestDataFactory) NewActionSubmission() *ActionSubmissionBuilder {
	seq := f.nextSequence()
	return &ActionSubmissionBuilder{
		factory:     f,
		gameID:      1, // Default game ID
		userID:      1, // Default user ID
		phaseID:     1, // Default phase ID
		characterID: nil,
		content:     fmt.Sprintf("Test action submission %d", seq),
	}
}

func (b *ActionSubmissionBuilder) InGame(game db.Game) *ActionSubmissionBuilder {
	b.gameID = game.ID
	return b
}

func (b *ActionSubmissionBuilder) ForGame(gameID int32) *ActionSubmissionBuilder {
	b.gameID = gameID
	return b
}

func (b *ActionSubmissionBuilder) ByUser(user db.User) *ActionSubmissionBuilder {
	b.userID = user.ID
	return b
}

func (b *ActionSubmissionBuilder) WithUserID(userID int32) *ActionSubmissionBuilder {
	b.userID = userID
	return b
}

func (b *ActionSubmissionBuilder) InPhase(phase db.GamePhase) *ActionSubmissionBuilder {
	b.phaseID = phase.ID
	b.gameID = phase.GameID // Also set game ID from phase
	return b
}

func (b *ActionSubmissionBuilder) ForPhase(phaseID int32) *ActionSubmissionBuilder {
	b.phaseID = phaseID
	return b
}

func (b *ActionSubmissionBuilder) AsCharacter(character db.Character) *ActionSubmissionBuilder {
	charID := character.ID
	b.characterID = &charID
	return b
}

func (b *ActionSubmissionBuilder) WithCharacterID(characterID int32) *ActionSubmissionBuilder {
	b.characterID = &characterID
	return b
}

func (b *ActionSubmissionBuilder) WithContent(content string) *ActionSubmissionBuilder {
	b.content = content
	return b
}

// Create persists the action submission to the database and returns the created submission
func (b *ActionSubmissionBuilder) Create() db.ActionSubmission {
	params := db.SubmitActionParams{
		GameID:  b.gameID,
		UserID:  b.userID,
		PhaseID: b.phaseID,
		Content: b.content,
	}

	if b.characterID != nil {
		params.CharacterID = pgtype.Int4{Int32: *b.characterID, Valid: true}
	}

	queries := db.New(b.factory.db.Pool)
	submission, err := queries.SubmitAction(context.Background(), params)
	if err != nil {
		b.factory.t.Fatalf("Should be able to create test action submission: %v", err)
	}

	return submission
}

// MessageBuilder provides a fluent interface for building test messages (posts and comments)
type MessageBuilder struct {
	factory               *TestDataFactory
	gameID                int32
	phaseID               *int32
	authorID              int32
	characterID           int32
	content               string
	messageType           string
	parentID              *int32
	visibility            string
	mentionedCharacterIDs []int32
}

// NewPost starts building a new post with default values
func (f *TestDataFactory) NewPost() *MessageBuilder {
	seq := f.nextSequence()
	return &MessageBuilder{
		factory:               f,
		gameID:                1, // Default game ID
		phaseID:               nil,
		authorID:              1, // Default author ID
		characterID:           1, // Default character ID
		content:               fmt.Sprintf("Test post %d", seq),
		messageType:           "post",
		parentID:              nil,
		visibility:            "game",
		mentionedCharacterIDs: []int32{},
	}
}

// NewComment starts building a new comment with default values
func (f *TestDataFactory) NewComment() *MessageBuilder {
	seq := f.nextSequence()
	return &MessageBuilder{
		factory:               f,
		gameID:                1, // Default game ID
		phaseID:               nil,
		authorID:              1, // Default author ID
		characterID:           1, // Default character ID
		content:               fmt.Sprintf("Test comment %d", seq),
		messageType:           "comment",
		parentID:              nil, // Must be set before Create()
		visibility:            "game",
		mentionedCharacterIDs: []int32{},
	}
}

func (b *MessageBuilder) InGame(game db.Game) *MessageBuilder {
	b.gameID = game.ID
	return b
}

func (b *MessageBuilder) ForGame(gameID int32) *MessageBuilder {
	b.gameID = gameID
	return b
}

func (b *MessageBuilder) InPhase(phase db.GamePhase) *MessageBuilder {
	phaseID := phase.ID
	b.phaseID = &phaseID
	b.gameID = phase.GameID // Also set game ID from phase
	return b
}

func (b *MessageBuilder) WithPhaseID(phaseID int32) *MessageBuilder {
	b.phaseID = &phaseID
	return b
}

func (b *MessageBuilder) ByAuthor(user db.User) *MessageBuilder {
	b.authorID = user.ID
	return b
}

func (b *MessageBuilder) WithAuthorID(authorID int32) *MessageBuilder {
	b.authorID = authorID
	return b
}

func (b *MessageBuilder) ByCharacter(character db.Character) *MessageBuilder {
	b.characterID = character.ID
	return b
}

func (b *MessageBuilder) WithCharacterID(characterID int32) *MessageBuilder {
	b.characterID = characterID
	return b
}

func (b *MessageBuilder) WithContent(content string) *MessageBuilder {
	b.content = content
	return b
}

func (b *MessageBuilder) OnPost(post db.Message) *MessageBuilder {
	parentID := post.ID
	b.parentID = &parentID
	b.gameID = post.GameID
	b.messageType = "comment"
	return b
}

func (b *MessageBuilder) WithParentID(parentID int32) *MessageBuilder {
	b.parentID = &parentID
	b.messageType = "comment"
	return b
}

func (b *MessageBuilder) GameVisible() *MessageBuilder {
	b.visibility = "game"
	return b
}

func (b *MessageBuilder) Private() *MessageBuilder {
	b.visibility = "private"
	return b
}

func (b *MessageBuilder) WithVisibility(visibility string) *MessageBuilder {
	b.visibility = visibility
	return b
}

func (b *MessageBuilder) MentioningCharacters(characterIDs ...int32) *MessageBuilder {
	b.mentionedCharacterIDs = characterIDs
	return b
}

// Create persists the message to the database and returns the created message
func (b *MessageBuilder) Create() db.Message {
	queries := db.New(b.factory.db.Pool)

	var message db.Message
	var err error

	if b.messageType == "post" {
		params := db.CreatePostParams{
			GameID:                b.gameID,
			AuthorID:              b.authorID,
			CharacterID:           b.characterID,
			Content:               b.content,
			Visibility:            db.MessageVisibility(b.visibility),
			MentionedCharacterIds: b.mentionedCharacterIDs,
		}

		if b.phaseID != nil {
			params.PhaseID = pgtype.Int4{Int32: *b.phaseID, Valid: true}
		}

		message, err = queries.CreatePost(context.Background(), params)
	} else {
		// comment
		if b.parentID == nil {
			b.factory.t.Fatalf("Comment must have a parent ID")
		}

		params := db.CreateCommentParams{
			GameID:                b.gameID,
			AuthorID:              b.authorID,
			CharacterID:           b.characterID,
			Content:               b.content,
			ParentID:              pgtype.Int4{Int32: *b.parentID, Valid: true},
			Visibility:            db.MessageVisibility(b.visibility),
			MentionedCharacterIds: b.mentionedCharacterIDs,
		}

		if b.phaseID != nil {
			params.PhaseID = pgtype.Int4{Int32: *b.phaseID, Valid: true}
		}

		message, err = queries.CreateComment(context.Background(), params)
	}

	if err != nil {
		b.factory.t.Fatalf("Should be able to create test message: %v", err)
	}

	return message
}
