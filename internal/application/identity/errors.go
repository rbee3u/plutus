package identity

import "github.com/rbee3u/plutus/internal/shared/apperr"

var (
	// ErrNotFound is returned when the identity or account is missing from the Portfolio.
	ErrNotFound = apperr.NotFound("identity not found")
	// ErrInvalid is returned when the Platform, Account Structure, Account Types, or names are malformed.
	ErrInvalid = apperr.Invalid("identity request is invalid")
	// ErrConflict is returned when the display name is already used on the Platform.
	ErrConflict = apperr.Conflict("identity display name already exists")
	// ErrLoginConflict prevents duplicate active bindings within one Portfolio and Platform.
	ErrLoginConflict = apperr.Conflict("this login is already bound to an active Identity in this Portfolio")
	// ErrPrecondition is returned when If-Match does not match the current version.
	ErrPrecondition = apperr.Precondition("identity version mismatch")
	// ErrArchived is returned when mutating an archived identity.
	ErrArchived = apperr.StateConflict("identity is archived")
	// ErrConnectionEnabled is returned when archiving while the Connection is enabled.
	ErrConnectionEnabled = apperr.StateConflict("disable the connection before archiving")
	// ErrCredentialsMissing is returned when validating or enabling without credentials.
	ErrCredentialsMissing = apperr.StateConflict("credentials are not configured")
	// ErrMasterKeyRequired is returned when credentials are used without a master key.
	ErrMasterKeyRequired = apperr.Unavailable("master key is not configured")
	// ErrValidationFailed is returned when the Platform rejected the validation call.
	ErrValidationFailed = apperr.Unprocessable("connection validation failed")
	// ErrAdapterUnavailable is returned when no adapter exists for the Platform.
	ErrAdapterUnavailable = apperr.Unavailable("no adapter is available for the platform")
)
