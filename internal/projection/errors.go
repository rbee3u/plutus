package projection

import "errors"

var (
	// ErrTemporary means the rebuild can be retried.
	ErrTemporary = errors.New("projection rebuild is temporarily unavailable")
	// ErrFailed means the generation was stored as failed without activation.
	ErrFailed = errors.New("projection rebuild failed")
)
