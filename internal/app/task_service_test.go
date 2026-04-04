package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
)

func TestTaskCommandServiceShowCurrentTask(t *testing.T) {
	t.Parallel()

	service := newTaskCommandService(t, &memoryThreadStore{
		tasks: map[string]app.Task{
			"user-1:task-1": {
				TaskID:        "task-1",
				DiscordUserID: "user-1",
				TaskName:      "Release work",
				Status:        app.TaskStatusOpen,
				CreatedAt:     time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
			},
		},
		activeTasks: map[string]app.ActiveTask{
			"user-1": {
				DiscordUserID: "user-1",
				TaskID:        "task-1",
			},
		},
	})

	response, err := service.ShowCurrentTask(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("ShowCurrentTask() error = %v", err)
	}

	if !response.Ephemeral {
		t.Fatal("Ephemeral = false, want true")
	}

	want := "Active task: `Release work` (`task-1`). Use `/release action:task-list` to see open tasks or `/release action:task-close task_id:task-1` when you're done."
	if response.Text != want {
		t.Fatalf("Text = %q, want %q", response.Text, want)
	}
}

func TestTaskCommandServiceShowCurrentTaskWithoutActiveTaskReturnsGuidance(t *testing.T) {
	t.Parallel()

	service := newTaskCommandService(t, &memoryThreadStore{})

	response, err := service.ShowCurrentTask(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("ShowCurrentTask() error = %v", err)
	}

	want := "No active task is selected. Use `/release action:task-new task_name:<name>`, `/release action:task-list`, or `/release action:task-switch task_id:<id>` first."
	if response.Text != want {
		t.Fatalf("Text = %q, want %q", response.Text, want)
	}
}

func TestTaskCommandServiceCreateTaskMakesTaskActive(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{}
	service := newTaskCommandServiceWithID(t, store, func() string {
		return "01JABCDEF0123456789TASK000"
	})

	response, err := service.CreateTask(context.Background(), "user-1", "  Release work  ")
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	want := "Created task `Release work` (`01JABCDEF0123456789TASK000`) and made it active. Your next message will continue this task."
	if response.Text != want {
		t.Fatalf("Text = %q, want %q", response.Text, want)
	}

	task, ok, err := store.GetTask(context.Background(), "user-1", "01JABCDEF0123456789TASK000")
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}

	if !ok {
		t.Fatal("GetTask() ok = false, want true")
	}

	if task.TaskName != "Release work" {
		t.Fatalf("TaskName = %q, want %q", task.TaskName, "Release work")
	}

	activeTask, ok, err := store.GetActiveTask(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetActiveTask() error = %v", err)
	}

	if !ok {
		t.Fatal("GetActiveTask() ok = false, want true")
	}

	if activeTask.TaskID != "01JABCDEF0123456789TASK000" {
		t.Fatalf("TaskID = %q, want %q", activeTask.TaskID, "01JABCDEF0123456789TASK000")
	}
}

func TestTaskCommandServiceListTasksMarksActiveTask(t *testing.T) {
	t.Parallel()

	service := newTaskCommandService(t, &memoryThreadStore{
		tasks: map[string]app.Task{
			"user-1:task-1": {
				TaskID:        "task-1",
				DiscordUserID: "user-1",
				TaskName:      "Release work",
				Status:        app.TaskStatusOpen,
				CreatedAt:     time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
			},
			"user-1:task-2": {
				TaskID:        "task-2",
				DiscordUserID: "user-1",
				TaskName:      "Docs update",
				Status:        app.TaskStatusOpen,
				CreatedAt:     time.Date(2026, time.April, 5, 1, 0, 0, 0, time.UTC),
			},
			"user-1:task-3": {
				TaskID:        "task-3",
				DiscordUserID: "user-1",
				TaskName:      "Closed work",
				Status:        app.TaskStatusClosed,
				CreatedAt:     time.Date(2026, time.April, 5, 2, 0, 0, 0, time.UTC),
			},
		},
		activeTasks: map[string]app.ActiveTask{
			"user-1": {
				DiscordUserID: "user-1",
				TaskID:        "task-2",
			},
		},
	})

	response, err := service.ListTasks(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}

	want := "Open tasks:\n- `Release work` (`task-1`)\n- `Docs update` (`task-2`) [active]\nUse `/release action:task-switch task_id:<id>` to change the active task."
	if response.Text != want {
		t.Fatalf("Text = %q, want %q", response.Text, want)
	}
}

func TestTaskCommandServiceSwitchTaskRequiresOpenTask(t *testing.T) {
	t.Parallel()

	service := newTaskCommandService(t, &memoryThreadStore{
		tasks: map[string]app.Task{
			"user-1:task-1": {
				TaskID:        "task-1",
				DiscordUserID: "user-1",
				TaskName:      "Closed work",
				Status:        app.TaskStatusClosed,
				CreatedAt:     time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
			},
		},
	})

	response, err := service.SwitchTask(context.Background(), "user-1", "task-1")
	if err != nil {
		t.Fatalf("SwitchTask() error = %v", err)
	}

	want := "Task `task-1` is closed. Use `/release action:task-list` to find an open task or `/release action:task-new task_name:<name>` to create another one."
	if response.Text != want {
		t.Fatalf("Text = %q, want %q", response.Text, want)
	}
}

func TestTaskCommandServiceCloseTaskClearsActiveTaskWhenClosingCurrentTask(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{
		tasks: map[string]app.Task{
			"user-1:task-1": {
				TaskID:        "task-1",
				DiscordUserID: "user-1",
				TaskName:      "Release work",
				Status:        app.TaskStatusOpen,
				CreatedAt:     time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
			},
		},
		activeTasks: map[string]app.ActiveTask{
			"user-1": {
				DiscordUserID: "user-1",
				TaskID:        "task-1",
			},
		},
	}
	service := newTaskCommandService(t, store)

	response, err := service.CloseTask(context.Background(), "user-1", "task-1")
	if err != nil {
		t.Fatalf("CloseTask() error = %v", err)
	}

	want := "Closed task `Release work` (`task-1`). No active task is selected now."
	if response.Text != want {
		t.Fatalf("Text = %q, want %q", response.Text, want)
	}

	if _, ok, err := store.GetActiveTask(context.Background(), "user-1"); err != nil || ok {
		t.Fatalf("GetActiveTask() = ok:%v err:%v, want ok:false err:nil", ok, err)
	}
}

func TestTaskCommandServiceCloseTaskKeepsDifferentActiveTask(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{
		tasks: map[string]app.Task{
			"user-1:task-1": {
				TaskID:        "task-1",
				DiscordUserID: "user-1",
				TaskName:      "Release work",
				Status:        app.TaskStatusOpen,
				CreatedAt:     time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
			},
			"user-1:task-2": {
				TaskID:        "task-2",
				DiscordUserID: "user-1",
				TaskName:      "Docs update",
				Status:        app.TaskStatusOpen,
				CreatedAt:     time.Date(2026, time.April, 5, 1, 0, 0, 0, time.UTC),
			},
		},
		activeTasks: map[string]app.ActiveTask{
			"user-1": {
				DiscordUserID: "user-1",
				TaskID:        "task-2",
			},
		},
	}
	service := newTaskCommandService(t, store)

	response, err := service.CloseTask(context.Background(), "user-1", "task-1")
	if err != nil {
		t.Fatalf("CloseTask() error = %v", err)
	}

	want := "Closed task `Release work` (`task-1`). Active task remains `Docs update` (`task-2`)."
	if response.Text != want {
		t.Fatalf("Text = %q, want %q", response.Text, want)
	}
}

func newTaskCommandService(t *testing.T, store app.ThreadStore) *app.DefaultTaskCommandService {
	t.Helper()
	return newTaskCommandServiceWithID(t, store, nil)
}

func newTaskCommandServiceWithID(
	t *testing.T,
	store app.ThreadStore,
	newTaskID func() string,
) *app.DefaultTaskCommandService {
	t.Helper()

	service, err := app.NewTaskCommandService(app.TaskCommandServiceDependencies{
		CommandName: "release",
		Store:       store,
		NewTaskID:   newTaskID,
	})
	if err != nil {
		t.Fatalf("NewTaskCommandService() error = %v", err)
	}

	return service
}
