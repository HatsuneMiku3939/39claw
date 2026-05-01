package thread

import (
	"context"
	"errors"
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
			mode:  config.ModeJournal,
			store: nil,
			request: app.MessageRequest{
				ReceivedAt: time.Date(2026, time.April, 5, 1, 30, 0, 0, time.UTC),
			},
			want: "2026-04-05",
		},
		{
			name:  "daily rolls over at local midnight",
			mode:  config.ModeJournal,
			store: nil,
			request: app.MessageRequest{
				ReceivedAt: time.Date(2026, time.April, 5, 15, 1, 0, 0, time.UTC),
			},
			want: "2026-04-06",
		},
		{
			name:  "task uses active task binding",
			mode:  config.ModeThread,
			store: store,
			request: app.MessageRequest{
				UserID: "user-1",
			},
			want: "user-1:task-1",
		},
		{
			name:  "task returns guidance error without active task",
			mode:  config.ModeThread,
			store: stubThreadStore{},
			request: app.MessageRequest{
				UserID: "user-1",
			},
			wantErr: app.ErrNoActiveTask,
		},
		{
			name: "task override uses exact open task name",
			mode: config.ModeThread,
			store: stubThreadStore{
				openTasks: []app.Task{
					{TaskID: "task-2", DiscordUserID: "user-1", TaskName: "docs-update", Status: app.TaskStatusOpen},
				},
			},
			request: app.MessageRequest{
				UserID:           "user-1",
				TaskOverrideName: "docs-update",
			},
			want: "user-1:task-2",
		},
		{
			name: "task override reports closed task",
			mode: config.ModeThread,
			store: stubThreadStore{
				closedTaskByName: map[string]bool{"docs-update": true},
			},
			request: app.MessageRequest{
				UserID:           "user-1",
				TaskOverrideName: "docs-update",
			},
			wantErr: app.ErrTaskOverrideClosed,
		},
		{
			name: "task override reports ambiguous open task names",
			mode: config.ModeThread,
			store: stubThreadStore{
				openTasks: []app.Task{
					{TaskID: "task-2", DiscordUserID: "user-1", TaskName: "docs-update", Status: app.TaskStatusOpen},
					{TaskID: "task-3", DiscordUserID: "user-1", TaskName: "docs-update", Status: app.TaskStatusOpen},
				},
			},
			request: app.MessageRequest{
				UserID:           "user-1",
				TaskOverrideName: "docs-update",
			},
			wantErr: app.ErrTaskOverrideAmbiguous,
		},
		{
			name:  "task override reports missing task",
			mode:  config.ModeThread,
			store: stubThreadStore{},
			request: app.MessageRequest{
				UserID:           "user-1",
				TaskOverrideName: "docs-update",
			},
			wantErr: app.ErrTaskOverrideNotFound,
		},
		{
			name:     "thread mode requires store",
			mode:     config.ModeThread,
			store:    nil,
			newError: "thread store must not be nil in thread mode",
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

				if !errors.Is(err, tt.wantErr) {
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

func TestQueueCoordinatorAdmitAndComplete(t *testing.T) {
	t.Parallel()

	coordinator := NewQueueCoordinator()
	key := "daily:2026-04-05"
	order := make([]int, 0, 5)

	admission, err := coordinator.Admit(key, func() {
		order = append(order, 0)
	})
	if err != nil {
		t.Fatalf("Admit() error = %v", err)
	}

	if !admission.ExecuteNow || admission.Queued {
		t.Fatalf("first admission = %+v, want immediate execution", admission)
	}

	for i := 1; i <= 5; i++ {
		index := i
		admission, err := coordinator.Admit(key, func() {
			order = append(order, index)
		})
		if err != nil {
			t.Fatalf("Admit() queued error = %v", err)
		}

		if !admission.Queued || admission.Position != i {
			t.Fatalf("queued admission = %+v, want queued position %d", admission, i)
		}
	}

	if _, err := coordinator.Admit(key, func() {}); err != app.ErrExecutionQueueFull {
		t.Fatalf("Admit() overflow error = %v, want %v", err, app.ErrExecutionQueueFull)
	}

	for expected := 1; expected <= 5; expected++ {
		work, ok := coordinator.Complete(key)
		if !ok {
			t.Fatalf("Complete() ok = false at position %d, want true", expected)
		}

		work()
	}

	if len(order) != 5 {
		t.Fatalf("work count = %d, want %d", len(order), 5)
	}

	for index, got := range order {
		want := index + 1
		if got != want {
			t.Fatalf("order[%d] = %d, want %d", index, got, want)
		}
	}

	if work, ok := coordinator.Complete(key); ok || work != nil {
		t.Fatalf("Complete() final state = ok:%v nil-work:%v, want ok:false nil-work:true", ok, work == nil)
	}
}

func TestQueueCoordinatorSnapshot(t *testing.T) {
	t.Parallel()

	coordinator := NewQueueCoordinator()
	key := "daily:2026-04-05#1"

	if snapshot := coordinator.Snapshot(key); snapshot != (app.QueueSnapshot{}) {
		t.Fatalf("initial snapshot = %+v, want zero value", snapshot)
	}

	if _, err := coordinator.Admit(key, func() {}); err != nil {
		t.Fatalf("Admit() first error = %v", err)
	}

	if _, err := coordinator.Admit(key, func() {}); err != nil {
		t.Fatalf("Admit() second error = %v", err)
	}

	if snapshot := coordinator.Snapshot(key); !snapshot.InFlight || snapshot.Queued != 1 {
		t.Fatalf("queued snapshot = %+v, want in_flight:true queued:1", snapshot)
	}

	if work, ok := coordinator.Complete(key); !ok || work == nil {
		t.Fatalf("Complete() = ok:%v nil-work:%v, want ok:true nil-work:false", ok, work == nil)
	}

	if snapshot := coordinator.Snapshot(key); !snapshot.InFlight || snapshot.Queued != 0 {
		t.Fatalf("running snapshot = %+v, want in_flight:true queued:0", snapshot)
	}

	if work, ok := coordinator.Complete(key); ok || work != nil {
		t.Fatalf("final Complete() = ok:%v nil-work:%v, want ok:false nil-work:true", ok, work == nil)
	}

	if snapshot := coordinator.Snapshot(key); snapshot != (app.QueueSnapshot{}) {
		t.Fatalf("final snapshot = %+v, want zero value", snapshot)
	}
}

type stubThreadStore struct {
	activeTask       app.ActiveTask
	activeTaskOK     bool
	openTasks        []app.Task
	closedTaskByName map[string]bool
}

func (s stubThreadStore) GetThreadBinding(context.Context, string, string) (app.ThreadBinding, bool, error) {
	return app.ThreadBinding{}, false, nil
}

func (s stubThreadStore) UpsertThreadBinding(context.Context, app.ThreadBinding) error {
	return nil
}

func (s stubThreadStore) DeleteThreadBinding(context.Context, string, string) error {
	return nil
}

func (s stubThreadStore) GetActiveDailySession(context.Context, string) (app.DailySession, bool, error) {
	return app.DailySession{}, false, nil
}

func (s stubThreadStore) GetLatestDailySessionBefore(context.Context, string) (app.DailySession, bool, error) {
	return app.DailySession{}, false, nil
}

func (s stubThreadStore) CreateDailySession(context.Context, app.DailySession) (app.DailySession, error) {
	return app.DailySession{}, nil
}

func (s stubThreadStore) RotateDailySession(context.Context, string, string) (app.DailySession, error) {
	return app.DailySession{}, nil
}

func (s stubThreadStore) CreateTask(context.Context, app.Task) error {
	return nil
}

func (s stubThreadStore) GetTask(context.Context, string, string) (app.Task, bool, error) {
	return app.Task{}, false, nil
}

func (s stubThreadStore) UpdateTask(context.Context, app.Task) error {
	return nil
}

func (s stubThreadStore) ListOpenTasks(context.Context, string) ([]app.Task, error) {
	return append([]app.Task(nil), s.openTasks...), nil
}

func (s stubThreadStore) HasClosedTaskWithName(_ context.Context, _ string, taskName string) (bool, error) {
	return s.closedTaskByName[taskName], nil
}

func (s stubThreadStore) ListClosedReadyTasks(context.Context) ([]app.Task, error) {
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
