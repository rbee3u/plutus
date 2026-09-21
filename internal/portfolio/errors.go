package portfolio

import "errors"

var (
	// ErrNotFound is returned when the portfolio is not owned or missing.
	ErrNotFound = errors.New("portfolio not found")
	// ErrConflict is returned when the portfolio name is already used.
	ErrConflict = errors.New("portfolio name already exists")
	// ErrPrecondition is returned when If-Match does not match the current version.
	ErrPrecondition = errors.New("portfolio version mismatch")
	// ErrUndeletable is returned when deleting the portfolio created with the user.
	ErrUndeletable = errors.New("portfolio is undeletable")
	// ErrInvalidTimezone is returned when the timezone is not a valid IANA name.
	ErrInvalidTimezone = errors.New("timezone is invalid")
	// ErrInvalidDisplayAsset is returned when the display asset is not USD or CNY.
	ErrInvalidDisplayAsset = errors.New("display asset is not supported")
	// ErrInvalidName is returned when the portfolio name is empty or too long.
	ErrInvalidName = errors.New("portfolio name is invalid")
)
