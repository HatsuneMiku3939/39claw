package app

import "errors"

var (
	ErrNoActiveTask        = errors.New("no active task selected")
	ErrExecutionInProgress = errors.New("execution already in progress for thread key")
)
