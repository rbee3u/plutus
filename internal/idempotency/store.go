package idempotency

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
)

// ErrConflict is returned when the same key is reused with a different body.
var ErrConflict = errors.New("idempotency key conflict")

// Record is a stored idempotent response.
type Record struct {
	Status  int
	Headers map[string]string
	Body    json.RawMessage
}

// Store persists idempotency records.
type Store struct {
	db  *sql.DB
	now func() time.Time
	ttl time.Duration
}

// NewStore constructs an idempotency store.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db, now: time.Now, ttl: 24 * time.Hour}
}

// Hash returns the SHA-256 digest of the request body.
func Hash(body []byte) []byte {
	sum := sha256.Sum256(body)
	return sum[:]
}

// Get returns a non-expired record for the key, if any.
func (s *Store) Get(ctx context.Context, userID uuid.UUID, method, path, key string) (*Record, []byte, error) {
	userBytes, err := userID.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}
	var hash []byte
	var status uint16
	var headers, body json.RawMessage
	var expires time.Time
	err = s.db.QueryRowContext(ctx, `
SELECT request_hash, response_status, response_headers_json, response_body_json, expires_at
FROM idempotency_records
WHERE user_id = ? AND method = ? AND path = ? AND `+"`key`"+` = ?`,
		userBytes, method, path, key,
	).Scan(&hash, &status, &headers, &body, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if !expires.After(s.now().UTC()) {
		return nil, nil, nil
	}
	headerMap := map[string]string{}
	if len(headers) > 0 {
		if err := json.Unmarshal(headers, &headerMap); err != nil {
			return nil, nil, err
		}
	}
	return &Record{Status: int(status), Headers: headerMap, Body: body}, hash, nil
}

// Put stores a response. A duplicate key with the same hash is ignored.
func (s *Store) Put(ctx context.Context, userID uuid.UUID, portfolioID *uuid.UUID, method, path, key string, requestHash []byte, rec Record) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return err
	}
	userBytes, err := userID.MarshalBinary()
	if err != nil {
		return err
	}
	var portfolioBytes []byte
	if portfolioID != nil {
		portfolioBytes, err = portfolioID.MarshalBinary()
		if err != nil {
			return err
		}
	}
	headers, err := json.Marshal(rec.Headers)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	_, err = s.db.ExecContext(ctx, `
INSERT INTO idempotency_records (
    id, user_id, portfolio_id, `+"`key`"+`, method, path, request_hash, response_status,
    response_headers_json, response_body_json, created_at, expires_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		idBytes, userBytes, nullableBytes(portfolioBytes), key, method, path, requestHash,
		uint16(rec.Status), headers, rec.Body, now, now.Add(s.ttl),
	)
	if mysql.IsDuplicate(err) {
		return nil
	}
	return err
}

func nullableBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}
