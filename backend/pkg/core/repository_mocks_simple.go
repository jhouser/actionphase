package core

import (
	db "actionphase/pkg/db/models"
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Simplified mock implementations for testing
// These provide basic functionality for common test scenarios

// SimpleMockUserRepository provides basic mock functionality for user operations
type SimpleMockUserRepository struct {
	users      map[int32]db.User
	nextID     int32
	byUsername map[string]db.User
}

func NewSimpleMockUserRepository() *SimpleMockUserRepository {
	return &SimpleMockUserRepository{
		users:      make(map[int32]db.User),
		nextID:     1,
		byUsername: make(map[string]db.User),
	}
}

func (m *SimpleMockUserRepository) CreateUser(ctx context.Context, params db.CreateUserParams) (db.User, error) {
	user := db.User{
		ID:        m.nextID,
		Username:  params.Username,
		Email:     params.Email,
		Password:  params.Password,
		CreatedAt: pgtype.Timestamp{Time: time.Now(), Valid: true},
	}
	m.users[m.nextID] = user
	m.byUsername[params.Username] = user
	m.nextID++
	return user, nil
}

func (m *SimpleMockUserRepository) GetUser(ctx context.Context, id int32) (db.User, error) {
	if user, exists := m.users[id]; exists {
		return user, nil
	}
	return db.User{}, errors.New("user not found")
}

func (m *SimpleMockUserRepository) GetUserByUsername(ctx context.Context, username string) (db.User, error) {
	if user, exists := m.byUsername[username]; exists {
		return user, nil
	}
	return db.User{}, errors.New("user not found")
}

func (m *SimpleMockUserRepository) GetUserByEmail(ctx context.Context, email string) (db.User, error) {
	for _, user := range m.users {
		if user.Email == email {
			return user, nil
		}
	}
	return db.User{}, errors.New("user not found")
}

func (m *SimpleMockUserRepository) UpdateUser(ctx context.Context, params db.UpdateUserParams) error {
	if user, exists := m.users[params.ID]; exists {
		user.Username = params.Username
		user.Email = params.Email
		user.Password = params.Password
		m.users[params.ID] = user
		return nil
	}
	return errors.New("user not found")
}

func (m *SimpleMockUserRepository) DeleteUser(ctx context.Context, id int32) error {
	if user, exists := m.users[id]; exists {
		delete(m.users, id)
		delete(m.byUsername, user.Username)
		return nil
	}
	return errors.New("user not found")
}

func (m *SimpleMockUserRepository) ListAllUsers(ctx context.Context, arg db.ListAllUsersParams) ([]db.User, error) {
	users := make([]db.User, 0, len(m.users))
	for _, user := range m.users {
		users = append(users, user)
	}
	return users, nil
}

// SimpleMockGameRepository provides basic mock functionality for game operations
type SimpleMockGameRepository struct {
	games  map[int32]db.Game
	nextID int32
}

func NewSimpleMockGameRepository() *SimpleMockGameRepository {
	return &SimpleMockGameRepository{
		games:  make(map[int32]db.Game),
		nextID: 1,
	}
}

func (m *SimpleMockGameRepository) CreateGame(ctx context.Context, params db.CreateGameParams) (db.Game, error) {
	game := db.Game{
		ID:          m.nextID,
		Title:       params.Title,
		Description: params.Description,
		GmUserID:    params.GmUserID,
		State:       pgtype.Text{String: "setup", Valid: true},
		Genre:       params.Genre,
		MaxPlayers:  params.MaxPlayers,
		IsPublic:    params.IsPublic,
		CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.games[m.nextID] = game
	m.nextID++
	return game, nil
}

func (m *SimpleMockGameRepository) GetGame(ctx context.Context, id int32) (db.Game, error) {
	if game, exists := m.games[id]; exists {
		return game, nil
	}
	return db.Game{}, errors.New("game not found")
}

func (m *SimpleMockGameRepository) UpdateGameState(ctx context.Context, params db.UpdateGameStateParams) (db.Game, error) {
	if game, exists := m.games[params.ID]; exists {
		game.State = params.State
		game.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		m.games[params.ID] = game
		return game, nil
	}
	return db.Game{}, errors.New("game not found")
}

func (m *SimpleMockGameRepository) DeleteGame(ctx context.Context, id int32) error {
	if _, exists := m.games[id]; exists {
		delete(m.games, id)
		return nil
	}
	return errors.New("game not found")
}

// Stub implementations for methods we don't need in basic tests
func (m *SimpleMockGameRepository) GetGamesByUser(ctx context.Context, userID int32) ([]db.GetGamesByUserRow, error) {
	return []db.GetGamesByUserRow{}, nil
}

func (m *SimpleMockGameRepository) GetGamesByGM(ctx context.Context, gmUserID int32) ([]db.Game, error) {
	games := make([]db.Game, 0)
	for _, game := range m.games {
		if game.GmUserID == gmUserID {
			games = append(games, game)
		}
	}
	return games, nil
}

func (m *SimpleMockGameRepository) GetRecruitingGames(ctx context.Context) ([]db.GetRecruitingGamesRow, error) {
	return []db.GetRecruitingGamesRow{}, nil
}

func (m *SimpleMockGameRepository) GetGameWithDetails(ctx context.Context, id int32) (db.GetGameWithDetailsRow, error) {
	return db.GetGameWithDetailsRow{}, nil
}

func (m *SimpleMockGameRepository) UpdateGame(ctx context.Context, params db.UpdateGameParams) (db.Game, error) {
	if game, exists := m.games[params.ID]; exists {
		game.Title = params.Title
		game.Description = params.Description
		game.Genre = params.Genre
		game.StartDate = params.StartDate
		game.EndDate = params.EndDate
		game.RecruitmentDeadline = params.RecruitmentDeadline
		game.MaxPlayers = params.MaxPlayers
		game.IsPublic = params.IsPublic
		game.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		m.games[params.ID] = game
		return game, nil
	}
	return db.Game{}, errors.New("game not found")
}

// SimpleMockSessionRepository provides basic mock functionality for session operations
type SimpleMockSessionRepository struct {
	sessions map[int32]db.Session
	nextID   int32
	byToken  map[string]db.Session
}

func NewSimpleMockSessionRepository() *SimpleMockSessionRepository {
	return &SimpleMockSessionRepository{
		sessions: make(map[int32]db.Session),
		nextID:   1,
		byToken:  make(map[string]db.Session),
	}
}

func (m *SimpleMockSessionRepository) CreateSession(ctx context.Context, params db.CreateSessionParams) (db.Session, error) {
	session := db.Session{
		ID:      m.nextID,
		UserID:  params.UserID,
		Data:    params.Data,
		Expires: params.Expires,
	}
	m.sessions[m.nextID] = session
	m.byToken[params.Data] = session
	m.nextID++
	return session, nil
}

func (m *SimpleMockSessionRepository) GetSession(ctx context.Context, id int32) (db.Session, error) {
	if session, exists := m.sessions[id]; exists {
		return session, nil
	}
	return db.Session{}, errors.New("session not found")
}

func (m *SimpleMockSessionRepository) GetSessionByToken(ctx context.Context, data string) (db.Session, error) {
	if session, exists := m.byToken[data]; exists {
		return session, nil
	}
	return db.Session{}, errors.New("session not found")
}

func (m *SimpleMockSessionRepository) DeleteSession(ctx context.Context, id int32) error {
	if session, exists := m.sessions[id]; exists {
		delete(m.sessions, id)
		delete(m.byToken, session.Data)
		return nil
	}
	return errors.New("session not found")
}

func (m *SimpleMockSessionRepository) DeleteSessionByToken(ctx context.Context, data string) error {
	if session, exists := m.byToken[data]; exists {
		delete(m.sessions, session.ID)
		delete(m.byToken, data)
		return nil
	}
	return errors.New("session not found")
}

func (m *SimpleMockSessionRepository) GetSessionsByUser(ctx context.Context, userID int32) ([]db.Session, error) {
	sessions := make([]db.Session, 0)
	for _, session := range m.sessions {
		if session.UserID == userID {
			sessions = append(sessions, session)
		}
	}
	return sessions, nil
}

// SimpleMockGameParticipantRepository provides basic mock functionality for game participant operations
type SimpleMockGameParticipantRepository struct {
	participants map[int32]db.GameParticipant
	nextID       int32
}

func NewSimpleMockGameParticipantRepository() *SimpleMockGameParticipantRepository {
	return &SimpleMockGameParticipantRepository{
		participants: make(map[int32]db.GameParticipant),
		nextID:       1,
	}
}

func (m *SimpleMockGameParticipantRepository) AddGameParticipant(ctx context.Context, params db.AddGameParticipantParams) (db.GameParticipant, error) {
	participant := db.GameParticipant{
		ID:       m.nextID,
		GameID:   params.GameID,
		UserID:   params.UserID,
		Role:     params.Role,
		Status:   pgtype.Text{String: "active", Valid: true},
		JoinedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.participants[m.nextID] = participant
	m.nextID++
	return participant, nil
}

func (m *SimpleMockGameParticipantRepository) GetGameParticipants(ctx context.Context, gameID int32) ([]db.GetGameParticipantsRow, error) {
	return []db.GetGameParticipantsRow{}, nil
}

func (m *SimpleMockGameParticipantRepository) RemoveGameParticipant(ctx context.Context, params db.RemoveGameParticipantParams) error {
	for id, participant := range m.participants {
		if participant.GameID == params.GameID && participant.UserID == params.UserID {
			delete(m.participants, id)
			return nil
		}
	}
	return errors.New("participant not found")
}

func (m *SimpleMockGameParticipantRepository) IsUserInGame(ctx context.Context, params db.IsUserInGameParams) (bool, error) {
	for _, participant := range m.participants {
		if participant.GameID == params.GameID && participant.UserID == params.UserID && participant.Status.String == "active" {
			return true, nil
		}
	}
	return false, nil
}

func (m *SimpleMockGameParticipantRepository) CanUserJoinGame(ctx context.Context, params db.CanUserJoinGameParams) (string, error) {
	return "can_join", nil
}

func (m *SimpleMockGameParticipantRepository) GetParticipantRole(ctx context.Context, params db.GetParticipantRoleParams) (string, error) {
	for _, participant := range m.participants {
		if participant.GameID == params.GameID && participant.UserID == params.UserID && participant.Status.String == "active" {
			return participant.Role, nil
		}
	}
	return "", errors.New("participant not found")
}

func (m *SimpleMockGameParticipantRepository) UpdateParticipantStatus(ctx context.Context, params db.UpdateParticipantStatusParams) (db.GameParticipant, error) {
	for id, participant := range m.participants {
		if participant.GameID == params.GameID && participant.UserID == params.UserID {
			participant.Status = params.Status
			m.participants[id] = participant
			return participant, nil
		}
	}
	return db.GameParticipant{}, errors.New("participant not found")
}

func (m *SimpleMockGameParticipantRepository) GetGameParticipantCount(ctx context.Context, gameID int32) (int64, error) {
	count := int64(0)
	for _, participant := range m.participants {
		if participant.GameID == gameID && participant.Role == "player" && participant.Status.String == "active" {
			count++
		}
	}
	return count, nil
}

// CreateMockDatabaseRepo creates a database repository with simple mocks
func CreateMockDatabaseRepo() *DatabaseRepository {
	return &DatabaseRepository{
		User:            NewSimpleMockUserRepository(),
		Session:         NewSimpleMockSessionRepository(),
		Game:            NewSimpleMockGameRepository(),
		GameParticipant: NewSimpleMockGameParticipantRepository(),
	}
}
