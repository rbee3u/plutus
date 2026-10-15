package identity

import "github.com/rbee3u/plutus/internal/shared/apperr"

var (
	// ErrNotFound is returned when the identity or account is missing from the portfolio.
	ErrNotFound = apperr.NotFound("Identity not found.")
	// ErrInvalid is returned when the platform, account structure, account types, or names are malformed.
	ErrInvalid = apperr.Invalid("Identity request is invalid.")
	// ErrConflict is returned when the display name is already used on the platform.
	ErrConflict = apperr.Conflict("Identity display name already exists.")
	// ErrLoginConflict prevents duplicate active bindings within one portfolio and platform.
	ErrLoginConflict = apperr.Conflict("This login is already bound to an active identity in this portfolio.")
	// ErrPrecondition is returned when If-Match does not match the current version.
	ErrPrecondition = apperr.Precondition("Identity version mismatch.")
	// ErrArchived is returned when mutating an archived identity.
	ErrArchived = apperr.StateConflict("Identity is archived.")
	// ErrConnectionEnabled is returned when archiving while the connection is enabled.
	ErrConnectionEnabled = apperr.StateConflict("Disable the connection before archiving.")
	// ErrCredentialsMissing is returned when validating or enabling without credentials.
	ErrCredentialsMissing = apperr.StateConflict("Credentials are not configured.")
	// ErrMasterKeyRequired is returned when credentials are used without a master key.
	ErrMasterKeyRequired = apperr.Unavailable("Master key is not configured.")
	// ErrValidationFailed is returned when the platform rejected the validation call.
	ErrValidationFailed = apperr.Unprocessable("Connection validation failed.")
	// ErrAdapterUnavailable is returned when no adapter exists for the platform.
	ErrAdapterUnavailable = apperr.Unavailable("No adapter is available for the platform.")
)
