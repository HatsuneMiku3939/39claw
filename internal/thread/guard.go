package thread

import (
	"errors"
	"sync"
)

var ErrExecutionInProgress = errors.New("execution already in progress for thread key")

type ReleaseFunc func()

type Guard struct {
	mu      sync.Mutex
	running map[string]int
}

func NewGuard() *Guard {
	return &Guard{
		running: make(map[string]int),
	}
}

func (g *Guard) Acquire(key string) (ReleaseFunc, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.running[key] > 0 {
		return nil, ErrExecutionInProgress
	}

	g.running[key]++

	released := false
	return func() {
		g.mu.Lock()
		defer g.mu.Unlock()

		if released {
			return
		}
		released = true

		g.running[key]--
		if g.running[key] <= 0 {
			delete(g.running, key)
		}
	}, nil
}
