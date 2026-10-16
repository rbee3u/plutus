// Package apperr defines the shared application error type and the kinds the
// transport maps onto HTTP problems. Domain packages declare their sentinel
// errors with the constructors here; the transport classifies any returned
// error by kind instead of importing every domain.
package apperr

import "errors"

// Kind classifies an application error.
type Kind int

// Error kinds. Kinds without a transport mapping fall through to 500.
const (
	KindInternal Kind = iota
	KindNotFound
	KindInvalid
	KindConflict
	KindPrecondition
	KindStateConflict
	KindUndeletable
	KindUnavailable
	KindUnprocessable
	KindUnauthenticated
	KindForbidden
	KindRateLimited
)

// Error is an application error with a kind and a user-safe message.
type Error struct {
	kind    Kind
	message string
}

// New returns an Error of the given kind.
func New(kind Kind, message string) *Error { return &Error{kind: kind, message: message} }

// Error implements error.
func (e *Error) Error() string { return e.message }

// Kind reports the error's classification.
func (e *Error) Kind() Kind { return e.kind }

// NotFound reports missing data.
func NotFound(message string) *Error { return New(KindNotFound, message) }

// Invalid reports malformed input.
func Invalid(message string) *Error { return New(KindInvalid, message) }

// Conflict reports a uniqueness or general conflict.
func Conflict(message string) *Error { return New(KindConflict, message) }

// Precondition reports a failed If-Edit-Version version check.
func Precondition(message string) *Error { return New(KindPrecondition, message) }

// StateConflict reports a state that disallows the action.
func StateConflict(message string) *Error { return New(KindStateConflict, message) }

// Undeletable reports a protected resource.
func Undeletable(message string) *Error { return New(KindUndeletable, message) }

// Unavailable reports a dependency that is not currently usable.
func Unavailable(message string) *Error { return New(KindUnavailable, message) }

// Unprocessable reports a well-formed request that cannot be processed.
func Unprocessable(message string) *Error { return New(KindUnprocessable, message) }

// Unauthenticated reports a missing or invalid session.
func Unauthenticated(message string) *Error { return New(KindUnauthenticated, message) }

// Forbidden reports a rejected CSRF token or similar.
func Forbidden(message string) *Error { return New(KindForbidden, message) }

// RateLimited reports throttling.
func RateLimited(message string) *Error { return New(KindRateLimited, message) }

// Internal reports an unexpected failure that should stay opaque.
func Internal(message string) *Error { return New(KindInternal, message) }

// KindOf reports the Kind carried by err, or KindInternal when none is.
func KindOf(err error) Kind {
	var target *Error
	if errors.As(err, &target) {
		return target.kind
	}
	return KindInternal
}
