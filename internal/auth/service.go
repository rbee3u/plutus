package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/portfolio"
)

const (
	SessionCookieName = "plutus_session"
	CSRFCookieName    = "plutus_csrf"
	tokenBytes        = 32
)

// Service owns authentication use cases.
type Service struct {
	q        *sqlcgen.Queries
	db       *sql.DB
	idle     time.Duration
	absolute time.Duration
	now      func() time.Time
	limiter  *limiter
}

// NewService constructs an authentication service.
func NewService(db *sql.DB, idle, absolute time.Duration) *Service {
	if idle <= 0 {
		idle = 7 * 24 * time.Hour
	}
	if absolute <= 0 {
		absolute = 30 * 24 * time.Hour
	}
	return &Service{
		q:        sqlcgen.New(db),
		db:       db,
		idle:     idle,
		absolute: absolute,
		now:      time.Now,
		limiter:  newLimiter(),
	}
}

// User is a persisted login identity.
type User struct {
	ID       uuid.UUID
	Username string
	Status   string
}

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

// CreateUser inserts a user and default undeletable portfolio.
func (s *Service) CreateUser(ctx context.Context, username, password string) (User, error) {
	display, normalized, err := NormalizeUsername(username)
	if err != nil {
		return User{}, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}
	now := s.now().UTC()
	userID, err := uuid.NewV7()
	if err != nil {
		return User{}, err
	}
	portfolioID, err := uuid.NewV7()
	if err != nil {
		return User{}, err
	}
	userBytes, err := userID.MarshalBinary()
	if err != nil {
		return User{}, err
	}
	portfolioBytes, err := portfolioID.MarshalBinary()
	if err != nil {
		return User{}, err
	}
	assetBytes, err := catalog.AssetUSD.MarshalBinary()
	if err != nil {
		return User{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	if err := q.InsertUser(ctx, sqlcgen.InsertUserParams{
		ID:                 userBytes,
		Username:           display,
		UsernameNormalized: normalized,
		Status:             "active",
		PasswordHash:       hash,
		CreatedAt:          now,
		UpdatedAt:          now,
	}); err != nil {
		if isDuplicate(err) {
			return User{}, ErrUserExists
		}
		return User{}, err
	}
	if err := q.InsertPortfolio(ctx, sqlcgen.InsertPortfolioParams{
		ID:             portfolioBytes,
		UserID:         userBytes,
		Name:           portfolio.DefaultName,
		Timezone:       "Asia/Shanghai",
		DisplayAssetID: assetBytes,
		Status:         "active",
		Undeletable:    true,
		Version:        1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}); err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return User{ID: userID, Username: display, Status: "active"}, nil
}

// Login authenticates a username and password.
func (s *Service) Login(ctx context.Context, username, password, ip, clientLabel string) (User, Session, error) {
	_, normalized, err := NormalizeUsername(username)
	if err != nil {
		return User{}, Session{}, ErrInvalidCredentials
	}
	if !s.limiter.allow("login:user:"+normalized, s.now(), 5.0/60.0, 5) || !s.limiter.allow("login:ip:"+ip, s.now(), 20.0/60.0, 20) {
		return User{}, Session{}, ErrRateLimited
	}
	row, err := s.q.GetUserByNormalizedUsername(ctx, normalized)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_, _, _ = VerifyPassword(password, dummyPHC)
			return User{}, Session{}, ErrInvalidCredentials
		}
		return User{}, Session{}, err
	}
	if row.Status != "active" {
		return User{}, Session{}, ErrUserDisabled
	}
	ok, rehash, err := VerifyPassword(password, row.PasswordHash)
	if err != nil || !ok {
		return User{}, Session{}, ErrInvalidCredentials
	}
	if rehash {
		newHash, err := HashPassword(password)
		if err == nil {
			_ = s.q.UpdateUserPasswordHash(ctx, sqlcgen.UpdateUserPasswordHashParams{
				PasswordHash: newHash,
				UpdatedAt:    s.now().UTC(),
				ID:           row.ID,
			})
		}
	}
	user, err := userFromRow(&row)
	if err != nil {
		return User{}, Session{}, err
	}
	session, err := s.createSession(ctx, user.ID, clientLabel)
	if err != nil {
		return User{}, Session{}, err
	}
	return user, session, nil
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
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return Session{}, err
	}
	userBytes, err := userID.MarshalBinary()
	if err != nil {
		return Session{}, err
	}
	label := sql.NullString{}
	if clientLabel != "" {
		label = sql.NullString{String: clientLabel, Valid: true}
	}
	if err := s.q.InsertSession(ctx, sqlcgen.InsertSessionParams{
		ID:                idBytes,
		UserID:            userBytes,
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
	id, err := sessionID.MarshalBinary()
	if err != nil {
		return err
	}
	return s.q.RevokeSession(ctx, sqlcgen.RevokeSessionParams{RevokedAt: sql.NullTime{Time: s.now().UTC(), Valid: true}, ID: id})
}

// LogoutAll revokes every session for the user.
func (s *Service) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	id, err := userID.MarshalBinary()
	if err != nil {
		return err
	}
	return s.q.RevokeUserSessions(ctx, sqlcgen.RevokeUserSessionsParams{
		RevokedAt: sql.NullTime{Time: s.now().UTC(), Valid: true},
		UserID:    id,
	})
}

// ChangePassword replaces the hash and revokes other sessions.
func (s *Service) ChangePassword(ctx context.Context, userID, sessionID uuid.UUID, current, next string) error {
	userBytes, err := userID.MarshalBinary()
	if err != nil {
		return err
	}
	row, err := s.q.GetUserByID(ctx, userBytes)
	if err != nil {
		return err
	}
	ok, _, err := VerifyPassword(current, row.PasswordHash)
	if err != nil || !ok {
		return ErrInvalidCredentials
	}
	hash, err := HashPassword(next)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	if err := s.q.UpdateUserPasswordHash(ctx, sqlcgen.UpdateUserPasswordHashParams{
		PasswordHash: hash,
		UpdatedAt:    now,
		ID:           userBytes,
	}); err != nil {
		return err
	}
	keep, err := sessionID.MarshalBinary()
	if err != nil {
		return err
	}
	return s.q.RevokeOtherUserSessions(ctx, sqlcgen.RevokeOtherUserSessionsParams{
		RevokedAt: sql.NullTime{Time: now, Valid: true},
		UserID:    userBytes,
		KeepID:    keep,
	})
}

// ListSessions returns the user's sessions.
func (s *Service) ListSessions(ctx context.Context, userID, currentID uuid.UUID) ([]Session, error) {
	userBytes, err := userID.MarshalBinary()
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListSessionsByUser(ctx, userBytes)
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

// DisableUser marks the user disabled and revokes sessions.
func (s *Service) DisableUser(ctx context.Context, username string) error {
	_, normalized, err := NormalizeUsername(username)
	if err != nil {
		return err
	}
	row, err := s.q.GetUserByNormalizedUsername(ctx, normalized)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUserNotFound
		}
		return err
	}
	now := s.now().UTC()
	if err := s.q.SetUserStatus(ctx, sqlcgen.SetUserStatusParams{Status: "disabled", UpdatedAt: now, ID: row.ID}); err != nil {
		return err
	}
	return s.q.RevokeUserSessions(ctx, sqlcgen.RevokeUserSessionsParams{
		RevokedAt: sql.NullTime{Time: now, Valid: true},
		UserID:    row.ID,
	})
}

// ResetPassword replaces the password hash and revokes sessions.
func (s *Service) ResetPassword(ctx context.Context, username, password string) error {
	_, normalized, err := NormalizeUsername(username)
	if err != nil {
		return err
	}
	row, err := s.q.GetUserByNormalizedUsername(ctx, normalized)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUserNotFound
		}
		return err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	if err := s.q.UpdateUserPasswordHash(ctx, sqlcgen.UpdateUserPasswordHashParams{
		PasswordHash: hash,
		UpdatedAt:    now,
		ID:           row.ID,
	}); err != nil {
		return err
	}
	return s.q.RevokeUserSessions(ctx, sqlcgen.RevokeUserSessionsParams{
		RevokedAt: sql.NullTime{Time: now, Valid: true},
		UserID:    row.ID,
	})
}

// RevokeOwnedSession revokes one session that belongs to the user.
func (s *Service) RevokeOwnedSession(ctx context.Context, userID, sessionID uuid.UUID) error {
	idBytes, err := sessionID.MarshalBinary()
	if err != nil {
		return err
	}
	row, err := s.q.GetSessionByID(ctx, idBytes)
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
		ID:        idBytes,
	})
}

// NewPreauthCSRF returns a random CSRF token for unauthenticated requests.
func NewPreauthCSRF() (string, error) {
	_, token, _, err := randomToken()
	return token, err
}

func randomToken() (raw []byte, token string, hash [32]byte, err error) {
	raw = make([]byte, tokenBytes)
	if _, err = rand.Read(raw); err != nil {
		return nil, "", hash, err
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	hash = sha256.Sum256([]byte(token))
	return raw, token, hash, nil
}

func userFromRow(row *sqlcgen.User) (User, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return User{}, err
	}
	return User{ID: id, Username: row.Username, Status: row.Status}, nil
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

func isDuplicate(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

func csrfMatch(header, cookie string) bool {
	if header == "" || cookie == "" {
		return false
	}
	if len(header) != len(cookie) {
		return subtle.ConstantTimeCompare([]byte(header), []byte(cookie)) == 1
	}
	return subtle.ConstantTimeCompare([]byte(header), []byte(cookie)) == 1
}

// CSRFValid reports whether the header matches the cookie.
func CSRFValid(header, cookie string) bool {
	return csrfMatch(header, cookie)
}

var dummyPHC = encodePHC(make([]byte, argonSaltLen), make([]byte, argonKeyLen), argonTime, argonMemoryKiB, argonThreads)
