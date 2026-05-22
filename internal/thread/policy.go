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

	if mode == config.ModeThread && store == nil {
		return nil, errors.New("thread store must not be nil in thread mode")
	}

	return &Policy{
		mode:     mode,
		timezone: timezone,
		store:    store,
	}, nil
}

func (p *Policy) ResolveMessageKey(ctx context.Context, request app.MessageRequest) (string, error) {
	switch p.mode {
	case config.ModeJournal:
		return request.ReceivedAt.In(p.timezone).Format(time.DateOnly), nil
	case config.ModeThread:
		if request.TaskOverrideName != "" {
			openTasks, err := p.store.ListOpenTasks(ctx, request.UserID)
			if err != nil {
				return "", err
			}

			matches := make([]app.Task, 0, 1)
			for _, task := range openTasks {
				if task.TaskName == request.TaskOverrideName {
					matches = append(matches, task)
				}
			}

			switch len(matches) {
			case 1:
				return BuildTaskKey(request.UserID, matches[0].TaskID), nil
			case 0:
				closed, err := p.store.HasClosedTaskWithName(ctx, request.UserID, request.TaskOverrideName)
				if err != nil {
					return "", err
				}
				if closed {
					return "", fmt.Errorf("%w: %s", app.ErrTaskOverrideClosed, request.TaskOverrideName)
				}
				return "", fmt.Errorf("%w: %s", app.ErrTaskOverrideNotFound, request.TaskOverrideName)
			default:
				return "", fmt.Errorf("%w: %s", app.ErrTaskOverrideAmbiguous, request.TaskOverrideName)
			}
		}

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
