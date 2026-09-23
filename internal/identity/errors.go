package identity

import "errors"

var (
	// ErrNotFound is returned when the identity or account is missing from the Portfolio.
	ErrNotFound = errors.New("identity not found")
	// ErrInvalid is returned when platform, mode, account types, or names are malformed.
	ErrInvalid = errors.New("identity request is invalid")
	// ErrConflict is returned when the display name is already used on the Platform.
	ErrConflict = errors.New("identity display name already exists")
	// ErrPrecondition is returned when If-Match does not match the current version.
	ErrPrecondition = errors.New("identity version mismatch")
	// ErrArchived is returned when mutating an archived identity.
	ErrArchived = errors.New("identity is archived")
	// ErrConnectionEnabled is returned when archiving while the Connection is enabled.
	ErrConnectionEnabled = errors.New("disable the connection before archiving")
	// ErrCredentialsMissing is returned when validating or enabling without credentials.
	ErrCredentialsMissing = errors.New("credentials are not configured")
	// ErrMasterKeyRequired is returned when credentials are used without a master key.
	ErrMasterKeyRequired = errors.New("master key is not configured")
	// ErrValidationFailed is returned when the Platform rejected the validation call.
	ErrValidationFailed = errors.New("connection validation failed")
	// ErrAdapterUnavailable is returned when no adapter exists for the Platform.
	ErrAdapterUnavailable = errors.New("no adapter is available for the platform")
)
