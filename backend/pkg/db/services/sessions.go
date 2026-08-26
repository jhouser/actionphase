package db

import (
	"actionphase/pkg/core"
	db "actionphase/pkg/db/models"
	"actionphase/pkg/observability"
	"context"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type SessionService struct {
	DB     *pgxpool.Pool
	Logger *observability.Logger
}

// Ensure SessionService implements the interface
var _ core.SessionServiceInterface = (*SessionService)(nil)

func (s *SessionService) Session(id int) (*core.Session, error) {
	return nil, nil
}

func (s *SessionService) GetSessionByID(ctx context.Context, id int32) (*core.Session, error) {
	q := db.New(s.DB)
	dbSession, err := q.GetSession(ctx, id)
	if err != nil {
		return nil, err
	}
	return &core.Session{
		ID:   int(dbSession.ID),
		User: &core.User{ID: int(dbSession.UserID)},
	}, nil
}

func (s *SessionService) SessionByToken(token string) (*core.Session, error) {
	ctx := context.Background()
	q := db.New(s.DB)
	dbSession, err := q.GetSessionByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	return &core.Session{
		ID:   int(dbSession.ID),
		User: &core.User{ID: int(dbSession.UserID)},
	}, nil
}

func (s *SessionService) Sessions() ([]*core.Session, error) {
	return nil, nil
}

// GetUserSessions returns all active sessions for a specific user
func (s *SessionService) GetUserSessions(ctx context.Context, userID int32) ([]db.Session, error) {
	q := db.New(s.DB)
	sessions, err := q.GetSessionsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return sessions, nil
}

// DeleteSession deletes a session by ID
func (s *SessionService) DeleteSession(ctx context.Context, sessionID int32) error {
	q := db.New(s.DB)

	s.Logger.Info(ctx, "Deleting session",
		"session_id", sessionID,
	)

	err := q.DeleteSession(ctx, sessionID)
	if err != nil {
		s.Logger.LogError(ctx, err, "Failed to delete session",
			"session_id", sessionID,
		)
		return err
	}

	s.Logger.Info(ctx, "Session deleted successfully",
		"session_id", sessionID,
	)

	return nil
}

func (s *SessionService) CreateSession(us *core.Session) (*core.Session, error) {
	ctx := context.Background()
	return s.createSessionInternal(ctx, us)
}

func (s *SessionService) CreateSessionWithMetadata(ctx context.Context, us *core.Session) (*core.Session, error) {
	return s.createSessionInternal(ctx, us)
}

func (s *SessionService) createSessionInternal(ctx context.Context, us *core.Session) (*core.Session, error) {
	q := db.New(s.DB)

	s.Logger.Info(ctx, "Creating new session",
		"user_id", us.User.ID,
	)

	params := db.CreateSessionParams{
		UserID:  int32(us.User.ID),
		Data:    us.Token,
		Expires: pgtype.Timestamptz{Time: time.Now().Add(core.SessionLifetime), Valid: true},
	}
	if us.IPAddress != nil {
		params.IpAddress = pgtype.Text{String: *us.IPAddress, Valid: true}
	}
	if us.UserAgent != nil {
		params.UserAgent = pgtype.Text{String: *us.UserAgent, Valid: true}
	}
	if us.Fingerprint != nil {
		params.Fingerprint = pgtype.Text{String: *us.Fingerprint, Valid: true}
	}

	dbSession, err := q.CreateSession(ctx, params)
	if err != nil {
		s.Logger.LogError(ctx, err, "Failed to create session",
			"user_id", us.User.ID,
		)
		return nil, err
	}

	s.Logger.Info(ctx, "Session created successfully",
		"session_id", dbSession.ID,
		"user_id", us.User.ID,
		"expires_at", dbSession.Expires.Time,
	)

	return &core.Session{
		ID: int(dbSession.ID),
	}, nil
}

func (s *SessionService) GetUserSessionsWithDetails(ctx context.Context, userID int32) ([]*core.SessionWithDetails, error) {
	q := db.New(s.DB)
	rows, err := q.GetUserSessionsWithDetails(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]*core.SessionWithDetails, 0, len(rows))
	for _, row := range rows {
		sd := &core.SessionWithDetails{
			ID:         int32(row.ID),
			CreatedAt:  row.CreatedAt.Time,
			LastSeenAt: row.LastSeenAt.Time,
			Expires:    row.Expires.Time,
		}
		if row.IpAddress.Valid {
			sd.IPAddress = &row.IpAddress.String
		}
		if row.UserAgent.Valid {
			sd.UserAgent = &row.UserAgent.String
		}
		if row.Fingerprint.Valid {
			sd.Fingerprint = &row.Fingerprint.String
		}
		result = append(result, sd)
	}
	return result, nil
}

func (s *SessionService) UpdateSessionLastSeen(ctx context.Context, sessionID int32) error {
	q := db.New(s.DB)
	return q.UpdateSessionLastSeen(ctx, sessionID)
}

func (s *SessionService) DeleteSessionByToken(token string) error {
	ctx := context.Background()
	q := db.New(s.DB)

	s.Logger.Info(ctx, "Deleting session by token")

	err := q.DeleteSessionByToken(ctx, token)
	if err != nil {
		s.Logger.LogError(ctx, err, "Failed to delete session by token")
		return err
	}

	s.Logger.Info(ctx, "Session deleted by token successfully")

	return nil
}

// InvalidateAllUserSessions deletes all sessions for a user (used when banning)
func (s *SessionService) InvalidateAllUserSessions(ctx context.Context, userID int32) error {
	q := db.New(s.DB)

	s.Logger.Warn(ctx, "Invalidating all user sessions - security event",
		"user_id", userID,
		"reason", "user_ban_or_password_change",
	)

	err := q.DeleteUserSessions(ctx, userID)
	if err != nil {
		s.Logger.LogError(ctx, err, "Failed to invalidate user sessions",
			"user_id", userID,
		)
		return err
	}

	s.Logger.Info(ctx, "All user sessions invalidated successfully",
		"user_id", userID,
	)

	return nil
}

// UpdateSessionMetadata sets the IP address, user agent, and fingerprint on an existing session.
func (s *SessionService) UpdateSessionMetadata(ctx context.Context, sessionID int32, ipAddress, userAgent string, fingerprint *string) error {
	q := db.New(s.DB)
	fp := pgtype.Text{}
	if fingerprint != nil {
		fp = pgtype.Text{String: *fingerprint, Valid: true}
	}
	return q.UpdateSessionMetadata(ctx, db.UpdateSessionMetadataParams{
		ID:          sessionID,
		IpAddress:   pgtype.Text{String: ipAddress, Valid: ipAddress != ""},
		UserAgent:   pgtype.Text{String: userAgent, Valid: userAgent != ""},
		Fingerprint: fp,
	})
}

// InvalidateSessionsByIP deletes all sessions originating from the given IP address.
func (s *SessionService) InvalidateSessionsByIP(ctx context.Context, ipAddress string) error {
	q := db.New(s.DB)
	s.Logger.Warn(ctx, "Invalidating sessions by IP - security event", "ip_address", ipAddress)
	return q.DeleteSessionsByIP(ctx, pgtype.Text{String: ipAddress, Valid: true})
}

// InvalidateSessionsByFingerprint deletes all sessions with the given device fingerprint.
func (s *SessionService) InvalidateSessionsByFingerprint(ctx context.Context, fingerprint string) error {
	q := db.New(s.DB)
	s.Logger.Warn(ctx, "Invalidating sessions by fingerprint - security event", "fingerprint", fingerprint)
	return q.DeleteSessionsByFingerprint(ctx, pgtype.Text{String: fingerprint, Valid: true})
}

// CleanupExpiredSessions deletes all sessions whose expiry timestamp is in the past.
func (s *SessionService) CleanupExpiredSessions(ctx context.Context) error {
	q := db.New(s.DB)
	if err := q.DeleteExpiredSessions(ctx); err != nil {
		s.Logger.LogError(ctx, err, "Failed to delete expired sessions")
		return err
	}
	s.Logger.Info(ctx, "Expired sessions cleaned up")
	return nil
}

// UpdateSessionToken updates the token for an existing session
func (s *SessionService) UpdateSessionToken(sessionID int32, token string) error {
	ctx := context.Background()
	q := db.New(s.DB)

	s.Logger.Info(ctx, "Updating session token",
		"session_id", sessionID,
	)

	err := q.UpdateSessionToken(ctx, db.UpdateSessionTokenParams{
		ID:   sessionID,
		Data: token,
	})

	if err != nil {
		s.Logger.LogError(ctx, err, "Failed to update session token",
			"session_id", sessionID,
		)
		return err
	}

	s.Logger.Info(ctx, "Session token updated successfully",
		"session_id", sessionID,
	)

	return nil
}
