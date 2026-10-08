package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/portfolio"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// User is a persisted login identity.
type User struct {
	ID       uuid.UUID
	Username string
	Status   string
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
	return mysql.WithTx(ctx, s.db, func(q *sqlcgen.Queries) (User, error) {
		if err := q.InsertUser(ctx, sqlcgen.InsertUserParams{
			ID:                 userID[:],
			Username:           display,
			UsernameNormalized: normalized,
			Status:             "active",
			PasswordHash:       hash,
			CreatedAt:          now,
			UpdatedAt:          now,
		}); err != nil {
			if mysql.IsDuplicate(err) {
				return User{}, ErrUserExists
			}
			return User{}, err
		}
		if err := q.InsertPortfolio(ctx, sqlcgen.InsertPortfolioParams{
			ID:                        portfolioID[:],
			UserID:                    userID[:],
			Name:                      portfolio.DefaultName,
			Timezone:                  portfolio.DefaultTimezone,
			CollectionIntervalSeconds: uint32(portfolio.DefaultCollectionInterval / time.Second),
			Undeletable:               true,
			Version:                   1,
			CreatedAt:                 now,
			UpdatedAt:                 now,
		}); err != nil {
			return User{}, err
		}
		return User{ID: userID, Username: display, Status: "active"}, nil
	})
}

// ChangePassword replaces the hash and revokes all user sessions.
func (s *Service) ChangePassword(ctx context.Context, userID uuid.UUID, current, next string) error {
	row, err := s.q.GetUserByID(ctx, userID[:])
	if err != nil {
		return err
	}
	ok, _, err := VerifyPassword(current, row.PasswordHash)
	if err != nil || !ok {
		return ErrInvalidCredentials
	}
	if current == next {
		return ErrSamePassword
	}
	same, _, err := VerifyPassword(next, row.PasswordHash)
	if err == nil && same {
		return ErrSamePassword
	}
	hash, err := HashPassword(next)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	if err := s.q.UpdateUserPasswordHash(ctx, sqlcgen.UpdateUserPasswordHashParams{
		PasswordHash: hash,
		UpdatedAt:    now,
		ID:           userID[:],
	}); err != nil {
		return err
	}
	return s.q.RevokeUserSessions(ctx, sqlcgen.RevokeUserSessionsParams{
		RevokedAt: sql.NullTime{Time: now, Valid: true},
		UserID:    userID[:],
	})
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

func userFromRow(row *sqlcgen.User) (User, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return User{}, err
	}
	return User{ID: id, Username: row.Username, Status: row.Status}, nil
}
