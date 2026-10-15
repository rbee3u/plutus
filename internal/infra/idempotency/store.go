// Package idempotency commits successful responses with their business writes.
package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/infra/sqljson"
	"github.com/rbee3u/plutus/internal/shared/apperr"
)

var (
	ErrConflict   = apperr.Conflict("Idempotency key conflict.")
	ErrInvalidKey = apperr.Invalid("Idempotency-Key must contain at most 255 visible ASCII characters.")
)

type Record struct {
	Status  int
	Headers map[string]string
	Body    json.RawMessage
}
type Store struct {
	db  *sql.DB
	now func() time.Time
	ttl time.Duration
}

func NewStore(db *sql.DB) *Store { return &Store{db: db, now: time.Now, ttl: 24 * time.Hour} }

func Hash(body []byte) []byte { hash := sha256.Sum256(body); return hash[:] }

// Run serializes creation per user. The lock covers missing and expired keys,
// so a failed attempt leaves neither business rows nor a durable pending record.
// Authorization and CSRF precede this call; resource lookup belongs in create.
func (s *Store) Run(ctx context.Context, userID uuid.UUID, method, path, key string, body []byte, create func(context.Context) (Record, error)) (Record, error) {
	if len(key) > 255 {
		return Record{}, ErrInvalidKey
	}
	for _, ch := range key {
		if ch < '!' || ch > '~' {
			return Record{}, ErrInvalidKey
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Record{}, err
	}
	defer func() { _ = tx.Rollback() }()
	ctx = mysql.TransactionContext(ctx, s.db, tx)
	q := sqlcgen.New(tx)
	if _, err := q.LockUser(ctx, userID[:]); err != nil {
		return Record{}, err
	}
	now := s.now().UTC()
	hash := Hash(body)
	if key != "" {
		stored, err := q.LockIdempotencyRecord(ctx, sqlcgen.LockIdempotencyRecordParams{UserID: userID[:], Method: method, Path: path, IdempotencyKey: key})
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return Record{}, err
		}
		if err == nil && stored.ExpiresAt.After(now) {
			if !bytes.Equal(hash, stored.RequestHash) {
				return Record{}, ErrConflict
			}
			record := Record{Status: int(stored.ResponseStatus), Body: json.RawMessage(stored.ResponseBodyJson), Headers: map[string]string{}}
			if len(stored.ResponseHeadersJson) > 0 {
				if err := json.Unmarshal(stored.ResponseHeadersJson, &record.Headers); err != nil {
					return Record{}, err
				}
			}
			return record, tx.Commit()
		}
		if err == nil {
			if err := q.DeleteIdempotencyRecord(ctx, sqlcgen.DeleteIdempotencyRecordParams{UserID: userID[:], Method: method, Path: path, IdempotencyKey: key}); err != nil {
				return Record{}, err
			}
		}
	}
	record, err := create(ctx)
	if err != nil {
		return Record{}, err
	}
	if record.Status < 200 || record.Status >= 300 || !json.Valid(record.Body) {
		return Record{}, errors.New("idempotency requires a successful JSON response")
	}
	if key != "" {
		headers, err := json.Marshal(record.Headers)
		if err != nil {
			return Record{}, err
		}
		if err := q.InsertIdempotencyRecord(ctx, sqlcgen.InsertIdempotencyRecordParams{
			UserID: userID[:], Method: method, Path: path, IdempotencyKey: key, RequestHash: hash,
			ResponseStatus: uint16(record.Status), ResponseHeadersJson: sqljson.Column(headers), ResponseBodyJson: sqljson.Column(record.Body),
			ExpiresAt: now.Add(s.ttl),
		}); err != nil {
			return Record{}, err
		}
	}
	return record, tx.Commit()
}

// PruneExpired retains successful responses for their full retry window,
// independently of the lifetime of any portfolio referenced by the response.
func (s *Store) PruneExpired(ctx context.Context) error {
	_, err := sqlcgen.New(s.db).DeleteExpiredIdempotencyRecords(ctx, s.now().UTC())
	return err
}
