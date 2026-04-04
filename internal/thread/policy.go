package thread

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	"github.com/HatsuneMiku3939/39claw/internal/config"
)

type Policy struct {
	mode     config.Mode
	timezone *time.Location
	store    app.ThreadStore
}

func NewPolicy(mode config.Mode, timezone *time.Location, store app.ThreadStore) (*Policy, error) {
	if timezone == nil {
		return nil, errors.New("timezone must not be nil")
	}

	if mode == config.ModeTask && store == nil {
		return nil, errors.New("thread store must not be nil in task mode")
	}

	return &Policy{
		mode:     mode,
		timezone: timezone,
		store:    store,
	}, nil
}

func (p *Policy) ResolveMessageKey(ctx context.Context, request app.MessageRequest) (string, error) {
	switch p.mode {
	case config.ModeDaily:
		return request.ReceivedAt.In(p.timezone).Format(time.DateOnly), nil
	case config.ModeTask:
		activeTask, ok, err := p.store.GetActiveTask(ctx, request.UserID)
		if err != nil {
			return "", err
		}

		if !ok {
			return "", app.ErrNoActiveTask
		}

		return BuildTaskKey(request.UserID, activeTask.TaskID), nil
	default:
		return "", fmt.Errorf("unsupported mode %q", p.mode)
	}
}

func BuildTaskKey(userID string, taskID string) string {
	return userID + ":" + taskID
}
