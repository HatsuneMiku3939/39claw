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
	waiting []queuedWork
}

type queuedWork struct {
	run    func()
	onDrop func()
}

func NewQueueCoordinator() *QueueCoordinator {
	return &QueueCoordinator{
		entries: make(map[string]*queueEntry),
	}
}

func (c *QueueCoordinator) Admit(key string, work func(), onDrop func()) (app.QueueAdmission, error) {
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

	entry.waiting = append(entry.waiting, queuedWork{
		run:    work,
		onDrop: onDrop,
	})
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

	next := entry.waiting[0].run
	entry.waiting = entry.waiting[1:]
	return next, true
}

func (c *QueueCoordinator) Snapshot(key string) app.QueueSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return app.QueueSnapshot{}
	}

	return app.QueueSnapshot{
		InFlight: entry.running,
		Queued:   len(entry.waiting),
	}
}

func (c *QueueCoordinator) DropQueued(key string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return 0
	}

	dropped := len(entry.waiting)
	for _, waiting := range entry.waiting {
		if waiting.onDrop != nil {
			waiting.onDrop()
		}
	}
	entry.waiting = nil
	return dropped
}
