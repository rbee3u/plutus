package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/rbee3u/plutus/internal/application/auth"
	"github.com/rbee3u/plutus/internal/infra/idempotency"
)

// createIdempotent authenticates before replay and looks up resources only for a
// fresh request. Typed request JSON defines the hash, including omitted values.
func createIdempotent[T any](ctx context.Context, a *API, csrf *string, key string, request any, status int, create func(context.Context, auth.User) (T, error)) (T, error) {
	var zero T
	user, session, err := a.currentSession(ctx)
	if err != nil {
		return zero, err
	}
	if err := a.requireCSRF(ctx, csrf, &session); err != nil {
		return zero, err
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return zero, err
	}
	run := func(ctx context.Context) (idempotency.Record, error) {
		body, err := create(ctx, user)
		if err != nil {
			return idempotency.Record{}, err
		}
		encoded, err := json.Marshal(body)
		if err != nil {
			return idempotency.Record{}, err
		}
		return idempotency.Record{Status: status, Body: encoded}, nil
	}
	var record idempotency.Record
	if a.Idempotency == nil {
		record, err = run(ctx)
	} else {
		record, err = a.Idempotency.Run(ctx, user.ID, http.MethodPost, requestPath(ctx), key, raw, run)
	}
	if errors.Is(err, idempotency.ErrConflict) {
		return zero, newProblemError(http.StatusConflict, "idempotency_conflict", "Idempotency key conflict", "The idempotency key was reused with a different request.")
	}
	if err != nil {
		return zero, err
	}
	var body T
	if err := json.Unmarshal(record.Body, &body); err != nil {
		return zero, err
	}
	return body, nil
}

func requestPath(ctx context.Context) string {
	c := ginFrom(ctx)
	if c == nil {
		return ""
	}
	return c.Request.URL.Path
}
