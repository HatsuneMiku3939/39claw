package thread

import (
	"context"
	"testing"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	"github.com/HatsuneMiku3939/39claw/internal/config"
)

func TestPolicyResolveMessageKey(t *testing.T) {
	t.Parallel()

	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	store := stubThreadStore{
		activeTask: app.ActiveTask{
			DiscordUserID: "user-1",
			TaskID:        "task-1",
		},
		activeTaskOK: true,
	}

	tests := []struct {
		name     string
		mode     config.Mode
		store    app.ThreadStore
		request  app.MessageRequest
		want     string
		wantErr  error
		newError string
	}{
		{
			name:  "daily uses configured local date",
			mode:  config.ModeDaily,
			store: nil,
			request: app.MessageRequest{
				ReceivedAt: time.Date(2026, time.April, 5, 1, 30, 0, 0, time.UTC),
			},
			want: "2026-04-05",
		},
		{
			name:  "daily rolls over at local midnight",
			mode:  config.ModeDaily,
			store: nil,
			request: app.MessageRequest{
				ReceivedAt: time.Date(2026, time.April, 5, 15, 1, 0, 0, time.UTC),
			},
			want: "2026-04-06",
		},
		{
			name:  "task uses active task binding",
			mode:  config.ModeTask,
			store: store,
			request: app.MessageRequest{
				UserID: "user-1",
			},
			want: "user-1:task-1",
		},
		{
			name:  "task returns guidance error without active task",
			mode:  config.ModeTask,
			store: stubThreadStore{},
			request: app.MessageRequest{
				UserID: "user-1",
			},
			wantErr: app.ErrNoActiveTask,
		},
		{
			name:     "task mode requires store",
			mode:     config.ModeTask,
			store:    nil,
			newError: "thread store must not be nil in task mode",
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			policy, err := NewPolicy(tt.mode, tokyo, tt.store)
			if tt.newError != "" {
				if err == nil {
					t.Fatalf("NewPolicy() error = nil, want %q", tt.newError)
				}

				if err.Error() != tt.newError {
					t.Fatalf("NewPolicy() error = %q, want %q", err.Error(), tt.newError)
				}

				return
			}

			if err != nil {
				t.Fatalf("NewPolicy() error = %v", err)
			}

			got, err := policy.ResolveMessageKey(context.Background(), tt.request)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("ResolveMessageKey() error = nil, want %v", tt.wantErr)
				}

				if err != tt.wantErr {
					t.Fatalf("ResolveMessageKey() error = %v, want %v", err, tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("ResolveMessageKey() error = %v", err)
			}

			if got != tt.want {
				t.Fatalf("ResolveMessageKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGuardAcquire(t *testing.T) {
	t.Parallel()

	guard := NewGuard()

	release, err := guard.Acquire("daily:2026-04-05")
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	secondRelease, err := guard.Acquire("daily:2026-04-05")
	if err == nil {
		t.Fatal("Acquire() error = nil, want non-nil")
	}

	if err != app.ErrExecutionInProgress {
		t.Fatalf("Acquire() error = %v, want %v", err, app.ErrExecutionInProgress)
	}

	if secondRelease != nil {
		t.Fatal("second release = non-nil, want nil")
	}

	release()

	release, err = guard.Acquire("daily:2026-04-05")
	if err != nil {
		t.Fatalf("Acquire() after release error = %v", err)
	}

	release()
}

type stubThreadStore struct {
	activeTask   app.ActiveTask
	activeTaskOK bool
}

func (s stubThreadStore) GetThreadBinding(context.Context, string, string) (app.ThreadBinding, bool, error) {
	return app.ThreadBinding{}, false, nil
}

func (s stubThreadStore) UpsertThreadBinding(context.Context, app.ThreadBinding) error {
	return nil
}

func (s stubThreadStore) CreateTask(context.Context, app.Task) error {
	return nil
}

func (s stubThreadStore) GetTask(context.Context, string, string) (app.Task, bool, error) {
	return app.Task{}, false, nil
}

func (s stubThreadStore) ListOpenTasks(context.Context, string) ([]app.Task, error) {
	return nil, nil
}

func (s stubThreadStore) SetActiveTask(context.Context, app.ActiveTask) error {
	return nil
}

func (s stubThreadStore) GetActiveTask(context.Context, string) (app.ActiveTask, bool, error) {
	return s.activeTask, s.activeTaskOK, nil
}

func (s stubThreadStore) ClearActiveTask(context.Context, string) error {
	return nil
}

func (s stubThreadStore) CloseTask(context.Context, string, string) error {
	return nil
}
