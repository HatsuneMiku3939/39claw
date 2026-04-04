package thread

import (
	"sync"

	"github.com/HatsuneMiku3939/39claw/internal/app"
)

const maxWaitingMessagesPerKey = 5

type QueueCoordinator struct {
	mu      sync.Mutex
	entries map[string]*queueEntry
}

type queueEntry struct {
	running bool
	waiting []func()
}

func NewQueueCoordinator() *QueueCoordinator {
	return &QueueCoordinator{
		entries: make(map[string]*queueEntry),
	}
}

func (c *QueueCoordinator) Admit(key string, work func()) (app.QueueAdmission, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok || !entry.running {
		c.entries[key] = &queueEntry{running: true}
		return app.QueueAdmission{
			ExecuteNow: true,
		}, nil
	}

	if len(entry.waiting) >= maxWaitingMessagesPerKey {
		return app.QueueAdmission{}, app.ErrExecutionQueueFull
	}

	entry.waiting = append(entry.waiting, work)
	return app.QueueAdmission{
		Queued:   true,
		Position: len(entry.waiting),
	}, nil
}

func (c *QueueCoordinator) Complete(key string) (func(), bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}

	if len(entry.waiting) == 0 {
		delete(c.entries, key)
		return nil, false
	}

	next := entry.waiting[0]
	entry.waiting = entry.waiting[1:]
	return next, true
}
