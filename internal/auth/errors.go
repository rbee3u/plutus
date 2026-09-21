package auth

import "errors"

var (
	// ErrUsernameRequired is returned when the username is empty after trim.
	ErrUsernameRequired = errors.New("username is required")
	// ErrUsernameTooLong is returned when the display username exceeds 191 characters.
	ErrUsernameTooLong = errors.New("username is too long")
	// ErrUserExists is returned when the normalized username is taken.
	ErrUserExists = errors.New("username is already taken")
	// ErrUserNotFound is returned when a CLI user action targets an unknown username.
	ErrUserNotFound = errors.New("user not found")
	// ErrInvalidCredentials is returned for unknown users or bad passwords.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrUserDisabled is returned when the user cannot authenticate.
	ErrUserDisabled = errors.New("user is disabled")
	// ErrUnauthenticated is returned when no valid session is present.
	ErrUnauthenticated = errors.New("unauthenticated")
	// ErrCSRF is returned when the CSRF token is missing or mismatched.
	ErrCSRF = errors.New("csrf token is invalid")
	// ErrRateLimited is returned when login is throttled.
	ErrRateLimited = errors.New("rate limited")
	// ErrSessionNotFound is returned when a session does not belong to the user.
	ErrSessionNotFound = errors.New("session not found")
)
