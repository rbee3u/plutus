package httpserver

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/idempotency"
)

func (a *API) lookupIdempotency(ctx context.Context, userID uuid.UUID, key, method, path string, body []byte) (*idempotency.Record, []byte, error) {
	if a.Idempotency == nil || key == "" {
		return nil, nil, nil
	}
	hash := idempotency.Hash(body)
	rec, storedHash, err := a.Idempotency.Get(ctx, userID, method, path, key)
	if err != nil {
		return nil, nil, err
	}
	if rec == nil {
		return nil, hash, nil
	}
	if !bytes.Equal(storedHash, hash) {
		return nil, nil, idempotency.ErrConflict
	}
	return rec, hash, nil
}

func (a *API) storeIdempotency(ctx context.Context, userID uuid.UUID, portfolioID *uuid.UUID, key, method, path string, hash []byte, status int, headers map[string]string, body any) {
	if a.Idempotency == nil || key == "" || len(hash) == 0 {
		return
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return
	}
	_ = a.Idempotency.Put(ctx, userID, portfolioID, method, path, key, hash, idempotency.Record{
		Status:  status,
		Headers: headers,
		Body:    raw,
	})
}

func etagPtr(value string) *string {
	return &value
}

func requestPath(ctx context.Context) string {
	c := ginFrom(ctx)
	if c == nil {
		return ""
	}
	return c.Request.URL.Path
}
