package adapters

import (
	"context"
	"errors"
	"fmt"
)

// Error classes returned by adapters.
const (
	ClassAuthentication = "authentication"
	ClassAuthorization  = "authorization"
	ClassRateLimited    = "rate_limited"
	ClassTemporary      = "temporary"
	ClassUnsupported    = "unsupported"
	ClassNormalization  = "normalization"
)

// ErrNoAdapter is returned when no adapter is registered for a Platform.
var ErrNoAdapter = errors.New("no adapter is registered for the platform")

// Error is a classified adapter failure. Detail never contains raw payloads or credentials.
type Error struct {
	Class  string
	Detail string
}

func (e *Error) Error() string {
	if e.Detail == "" {
		return e.Class
	}
	return fmt.Sprintf("%s: %s", e.Class, e.Detail)
}

// NewError constructs a classified adapter error.
func NewError(class, detail string) *Error {
	return &Error{Class: class, Detail: detail}
}

// Classify returns the adapter error class of err, or ClassTemporary for unknown errors.
func Classify(err error) (class, detail string) {
	var adapterErr *Error
	if errors.As(err, &adapterErr) {
		return adapterErr.Class, adapterErr.Detail
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ClassTemporary, "The platform did not answer within the timeout."
	}
	if errors.Is(err, ErrNoAdapter) {
		return ClassUnsupported, ErrNoAdapter.Error()
	}
	return ClassTemporary, "The platform call failed."
}

// StopsScheduling reports whether an error class must set the Connection invalid.
func StopsScheduling(class string) bool {
	return class == ClassAuthentication || class == ClassAuthorization
}

// Retryable reports whether an error class may be retried within the same run.
func Retryable(class string) bool {
	return class == ClassRateLimited || class == ClassTemporary
}
