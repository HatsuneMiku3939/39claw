package app

import "errors"

var (
	ErrNoActiveTask       = errors.New("no active task selected")
	ErrExecutionQueueFull = errors.New("execution queue full for thread key")
)
