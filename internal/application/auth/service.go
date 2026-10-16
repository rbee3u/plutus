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

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
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

// Login authenticates a username and password.
func (s *Service) Login(ctx context.Context, username, password, ip, userAgent string) (User, Session, error) {
	_, normalized, err := NormalizeUsername(username)
	if err != nil {
		return User{}, Session{}, ErrInvalidCredentials
	}
	if !s.limiter.allow("login:user:"+normalized, s.now(), 5.0/60.0, 5) || !s.limiter.allow("login:ip:"+ip, s.now(), 20.0/60.0, 20) {
		return User{}, Session{}, ErrRateLimited
	}
	row, err := s.q.GetUserByUsernameKey(ctx, normalized)
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
	if err != nil {
		return User{}, Session{}, err
	}
	if !ok {
		return User{}, Session{}, ErrInvalidCredentials
	}
	var replacement string
	if rehash {
		replacement, _ = HashPassword(password)
	}
	user, err := userFromRow(&row)
	if err != nil {
		return User{}, Session{}, err
	}
	session, err := mysql.WithTx(ctx, s.db, func(q *sqlcgen.Queries) (Session, error) {
		current, err := q.LockUser(ctx, row.ID)
		if err != nil {
			return Session{}, err
		}
		if current.Status != "active" {
			return Session{}, ErrUserDisabled
		}
		// Password replacement and session creation serialize on the same user.
		if current.PasswordHash != row.PasswordHash {
			return Session{}, ErrInvalidCredentials
		}
		if replacement != "" {
			if err := q.UpdateUserPasswordHash(ctx, sqlcgen.UpdateUserPasswordHashParams{
				PasswordHash: replacement, UpdatedAt: s.now().UTC(), ID: row.ID,
			}); err != nil {
				return Session{}, err
			}
		}
		return s.createSession(ctx, q, user.ID, userAgent)
	})
	if err != nil {
		return User{}, Session{}, err
	}
	return user, session, nil
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

// CSRFValid reports whether the header matches the cookie.
func CSRFValid(header, cookie string) bool {
	if header == "" || cookie == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(header), []byte(cookie)) == 1
}

var dummyPHC = encodePHC(make([]byte, argonSaltLen), make([]byte, argonKeyLen), argonTime, argonMemoryKiB, argonThreads)
