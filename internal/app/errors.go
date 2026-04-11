package app

import "errors"

var (
	ErrNoActiveTask          = errors.New("no active task selected")
	ErrExecutionQueueFull    = errors.New("execution queue full for thread key")
	ErrTaskOverrideNotFound  = errors.New("task override target not found")
	ErrTaskOverrideClosed    = errors.New("task override target is closed")
	ErrTaskOverrideAmbiguous = errors.New("task override target is ambiguous")
)
