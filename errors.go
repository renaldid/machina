package machina

import "errors"

var (
	// ErrInvalidTransition is returned by Send when no transition is registered
	// for the current state and the given event.
	ErrInvalidTransition = errors.New("invalid transition")

	// ErrGuardRejected is returned by Send when a guard function returns false.
	ErrGuardRejected = errors.New("transition rejected by guard")
)
