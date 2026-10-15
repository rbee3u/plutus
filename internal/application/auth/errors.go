package auth

import "github.com/rbee3u/plutus/internal/shared/apperr"

var (
	// ErrUsernameRequired is returned when the username is empty after trim.
	ErrUsernameRequired = apperr.Invalid("Username is required.")
	// ErrUsernameTooLong is returned when the display username exceeds 191 characters.
	ErrUsernameTooLong = apperr.Invalid("Username is too long.")
	// ErrUserExists is returned when the normalized username is taken.
	ErrUserExists = apperr.Conflict("Username is already taken.")
	// ErrUserNotFound is returned when a CLI user action targets an unknown username.
	ErrUserNotFound = apperr.NotFound("User not found.")
	// ErrInvalidCredentials is returned for unknown users or bad passwords.
	ErrInvalidCredentials = apperr.Unauthenticated("Invalid credentials.")
	// ErrUserDisabled is returned when the user cannot authenticate.
	ErrUserDisabled = apperr.Unauthenticated("User is disabled.")
	// ErrUnauthenticated is returned when no valid session is present.
	ErrUnauthenticated = apperr.Unauthenticated("Unauthenticated.")
	// ErrCSRF is returned when the CSRF token is missing or mismatched.
	ErrCSRF = apperr.Forbidden("CSRF token is invalid.")
	// ErrRateLimited is returned when login is throttled.
	ErrRateLimited = apperr.RateLimited("Rate limited.")
	// ErrSessionNotFound is returned when a session does not belong to the user.
	ErrSessionNotFound = apperr.NotFound("Session not found.")
)
