package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// Session is an authenticated browser session.
type Session struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	Token             string
	CSRF              string
	CreatedAt         time.Time
	LastSeenAt        time.Time
	IdleExpiresAt     time.Time
	AbsoluteExpiresAt time.Time
	ClientLabel       *string
	Current           bool
}

func (s *Service) createSession(ctx context.Context, userID uuid.UUID, clientLabel string) (Session, error) {
	now := s.now().UTC()
	id, err := uuid.NewV7()
	if err != nil {
		return Session{}, err
	}
	_, token, tokenHash, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	csrfRaw, csrf, _, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	label := sql.NullString{}
	if clientLabel != "" {
		label = sql.NullString{String: clientLabel, Valid: true}
	}
	if err := s.q.InsertSession(ctx, sqlcgen.InsertSessionParams{
		ID:                id[:],
		UserID:            userID[:],
		TokenHash:         tokenHash[:],
		CsrfSecret:        csrfRaw,
		CreatedAt:         now,
		LastSeenAt:        now,
		IdleExpiresAt:     now.Add(s.idle),
		AbsoluteExpiresAt: now.Add(s.absolute),
		ClientLabel:       label,
	}); err != nil {
		return Session{}, err
	}
	var labelPtr *string
	if label.Valid {
		v := label.String
		labelPtr = &v
	}
	return Session{
		ID:                id,
		UserID:            userID,
		Token:             token,
		CSRF:              csrf,
		CreatedAt:         now,
		LastSeenAt:        now,
		IdleExpiresAt:     now.Add(s.idle),
		AbsoluteExpiresAt: now.Add(s.absolute),
		ClientLabel:       labelPtr,
		Current:           true,
	}, nil
}

// Authenticate loads a valid session from a cookie token.
func (s *Service) Authenticate(ctx context.Context, token string) (User, Session, error) {
	if token == "" {
		return User{}, Session{}, ErrUnauthenticated
	}
	sum := sha256.Sum256([]byte(token))
	row, err := s.q.GetSessionByTokenHash(ctx, sum[:])
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, Session{}, ErrUnauthenticated
		}
		return User{}, Session{}, err
	}
	now := s.now().UTC()
	if row.RevokedAt.Valid || now.After(row.IdleExpiresAt) || now.After(row.AbsoluteExpiresAt) {
		return User{}, Session{}, ErrUnauthenticated
	}
	userRow, err := s.q.GetUserByID(ctx, row.UserID)
	if err != nil {
		return User{}, Session{}, ErrUnauthenticated
	}
	if userRow.Status != "active" {
		return User{}, Session{}, ErrUserDisabled
	}
	if err := s.q.TouchSession(ctx, sqlcgen.TouchSessionParams{
		LastSeenAt:    now,
		IdleExpiresAt: now.Add(s.idle),
		ID:            row.ID,
	}); err != nil {
		return User{}, Session{}, err
	}
	user, err := userFromRow(&userRow)
	if err != nil {
		return User{}, Session{}, err
	}
	session, err := sessionFromRow(&row, token, true)
	if err != nil {
		return User{}, Session{}, err
	}
	session.LastSeenAt = now
	session.IdleExpiresAt = now.Add(s.idle)
	return user, session, nil
}

// Logout revokes one session.
func (s *Service) Logout(ctx context.Context, sessionID uuid.UUID) error {
	return s.q.RevokeSession(ctx, sqlcgen.RevokeSessionParams{RevokedAt: sql.NullTime{Time: s.now().UTC(), Valid: true}, ID: sessionID[:]})
}

// LogoutAll revokes every session for the user.
func (s *Service) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	return s.q.RevokeUserSessions(ctx, sqlcgen.RevokeUserSessionsParams{
		RevokedAt: sql.NullTime{Time: s.now().UTC(), Valid: true},
		UserID:    userID[:],
	})
}

// ListSessions returns the user's sessions.
func (s *Service) ListSessions(ctx context.Context, userID, currentID uuid.UUID) ([]Session, error) {
	rows, err := s.q.ListSessionsByUser(ctx, userID[:])
	if err != nil {
		return nil, err
	}
	out := make([]Session, 0, len(rows))
	now := s.now().UTC()
	for i := range rows {
		row := &rows[i]
		if row.RevokedAt.Valid || now.After(row.IdleExpiresAt) || now.After(row.AbsoluteExpiresAt) {
			continue
		}
		sess, err := sessionFromRow(row, "", bytesEqualUUID(row.ID, currentID))
		if err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, nil
}

// RevokeOwnedSession revokes one session that belongs to the user.
func (s *Service) RevokeOwnedSession(ctx context.Context, userID, sessionID uuid.UUID) error {
	row, err := s.q.GetSessionByID(ctx, sessionID[:])
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrSessionNotFound
		}
		return err
	}
	owner, err := uuid.FromBytes(row.UserID)
	if err != nil || owner != userID || row.RevokedAt.Valid {
		return ErrSessionNotFound
	}
	return s.q.RevokeSession(ctx, sqlcgen.RevokeSessionParams{
		RevokedAt: sql.NullTime{Time: s.now().UTC(), Valid: true},
		ID:        sessionID[:],
	})
}

func sessionFromRow(row *sqlcgen.Session, token string, current bool) (Session, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return Session{}, err
	}
	userID, err := uuid.FromBytes(row.UserID)
	if err != nil {
		return Session{}, err
	}
	csrf := base64.RawURLEncoding.EncodeToString(row.CsrfSecret)
	var label *string
	if row.ClientLabel.Valid {
		v := row.ClientLabel.String
		label = &v
	}
	return Session{
		ID:                id,
		UserID:            userID,
		Token:             token,
		CSRF:              csrf,
		CreatedAt:         row.CreatedAt,
		LastSeenAt:        row.LastSeenAt,
		IdleExpiresAt:     row.IdleExpiresAt,
		AbsoluteExpiresAt: row.AbsoluteExpiresAt,
		ClientLabel:       label,
		Current:           current,
	}, nil
}

func bytesEqualUUID(raw []byte, id uuid.UUID) bool {
	parsed, err := uuid.FromBytes(raw)
	return err == nil && parsed == id
}
