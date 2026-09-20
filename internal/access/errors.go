package access

import "errors"

var (
	// ErrNotFound is returned when the identity or connection is missing.
	ErrNotFound = errors.New("resource not found")
	// ErrConflict is returned for uniqueness violations.
	ErrConflict = errors.New("resource already exists")
	// ErrPrecondition is returned when If-Match does not match.
	ErrPrecondition = errors.New("version mismatch")
	// ErrInvalidAccountMode is returned for modes outside ADR 0005.
	ErrInvalidAccountMode = errors.New("account mode is not supported")
	// ErrInvalidKind is returned for an invalid identity kind or parent pairing.
	ErrInvalidKind = errors.New("identity kind is invalid")
	// ErrInvalidPlatform is returned when the platform is missing or disabled.
	ErrInvalidPlatform = errors.New("platform is not available")
	// ErrInvalidLabel is returned when a connection label is empty or too long.
	ErrInvalidLabel = errors.New("label is invalid")
	// ErrCredentialsRequired is returned when enabling a connection without ciphertext.
	ErrCredentialsRequired = errors.New("credentials are not configured")
	// ErrInvalidStatus is returned for an illegal status transition.
	ErrInvalidStatus = errors.New("status is invalid")
	// ErrInvalidParent is returned when the parent identity is missing or archived.
	ErrInvalidParent = errors.New("parent identity is invalid")
	// ErrIdentityFrozen is returned when identity fields cannot change after Observations exist.
	ErrIdentityFrozen = errors.New("identity fields are immutable after observations exist")
	// ErrMasterKeyRequired is returned when credentials cannot be stored without a master key.
	ErrMasterKeyRequired = errors.New("master key is required")
)
