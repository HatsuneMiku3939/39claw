package thread

import (
	"sync"

	"github.com/HatsuneMiku3939/39claw/internal/app"
)

type Guard struct {
	mu      sync.Mutex
	running map[string]int
}

func NewGuard() *Guard {
	return &Guard{
		running: make(map[string]int),
	}
}

func (g *Guard) Acquire(key string) (app.ReleaseFunc, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.running[key] > 0 {
		return nil, app.ErrExecutionInProgress
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
