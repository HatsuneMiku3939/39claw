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

func TestTaskCommandServiceShowCurrentTaskIncludesWorktreeBranchWhenAvailable(t *testing.T) {
	t.Parallel()

	service := newTaskCommandServiceWithWorkspace(t, &memoryThreadStore{
		tasks: map[string]app.Task{
			"user-1:task-1": {
				TaskID:         "task-1",
				DiscordUserID:  "user-1",
				TaskName:       "Release work",
				Status:         app.TaskStatusOpen,
				WorktreeStatus: app.TaskWorktreeStatusReady,
				WorktreePath:   "/tmp/worktrees/task-1",
				CreatedAt:      time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
			},
		},
		activeTasks: map[string]app.ActiveTask{
			"user-1": {
				DiscordUserID: "user-1",
				TaskID:        "task-1",
			},
		},
	}, nil, &branchReportingTaskWorkspaceManager{
		branches: map[string]string{
			"task-1": "main",
		},
	})

	response, err := service.ShowCurrentTask(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("ShowCurrentTask() error = %v", err)
	}

	want := "Active task: `Release work` (`task-1`) [branch: `main`]. Use `/release action:task-list` to see open tasks or `/release action:task-close task_id:task-1` when you're done."
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

	want := "No active task is selected. Use `/release action:task-new task_name:<name>`, `/release action:task-list`, or `/release action:task-switch task_name:<name>` first."
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

	response, err := service.CreateTask(context.Background(), "user-1", "  release-work  ")
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	want := "Created task `release-work` (`01JABCDEF0123456789TASK000`) and made it active. Your next message will continue this task."
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

	if task.TaskName != "release-work" {
		t.Fatalf("TaskName = %q, want %q", task.TaskName, "release-work")
	}

	if task.BranchName != "task/release-work" {
		t.Fatalf("BranchName = %q, want %q", task.BranchName, "task/release-work")
	}

	if task.WorktreeStatus != app.TaskWorktreeStatusPending {
		t.Fatalf("WorktreeStatus = %q, want %q", task.WorktreeStatus, app.TaskWorktreeStatusPending)
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

func TestTaskCommandServiceCreateTaskRejectsInvalidSlug(t *testing.T) {
	t.Parallel()

	service := newTaskCommandService(t, &memoryThreadStore{})

	response, err := service.CreateTask(context.Background(), "user-1", "Release work")
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	if response.Text != app.TaskNameRulesDescription {
		t.Fatalf("Text = %q, want %q", response.Text, app.TaskNameRulesDescription)
	}
}

func TestTaskCommandServiceCreateTaskRejectsDuplicateOpenName(t *testing.T) {
	t.Parallel()

	service := newTaskCommandService(t, &memoryThreadStore{
		tasks: map[string]app.Task{
			"user-1:task-1": {
				TaskID:        "task-1",
				DiscordUserID: "user-1",
				TaskName:      "release-work",
				Status:        app.TaskStatusOpen,
				CreatedAt:     time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
			},
		},
	})

	response, err := service.CreateTask(context.Background(), "user-1", "release-work")
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	want := "An open task named `release-work` already exists. Use `/release action:task-switch task_name:release-work` to switch to it or `/release action:task-switch task_id:<id>` to pick by task ID when needed."
	if response.Text != want {
		t.Fatalf("Text = %q, want %q", response.Text, want)
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

	want := "Open tasks:\n- `Release work` (`task-1`)\n- `Docs update` (`task-2`) [active]\nUse `/release action:task-switch task_name:<name>` to change the active task."
	if response.Text != want {
		t.Fatalf("Text = %q, want %q", response.Text, want)
	}
}

func TestTaskCommandServiceListTasksIncludesWorktreeBranchWhenAvailable(t *testing.T) {
	t.Parallel()

	service := newTaskCommandServiceWithWorkspace(t, &memoryThreadStore{
		tasks: map[string]app.Task{
			"user-1:task-1": {
				TaskID:         "task-1",
				DiscordUserID:  "user-1",
				TaskName:       "Release work",
				Status:         app.TaskStatusOpen,
				WorktreeStatus: app.TaskWorktreeStatusReady,
				WorktreePath:   "/tmp/worktrees/task-1",
				CreatedAt:      time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
			},
			"user-1:task-2": {
				TaskID:         "task-2",
				DiscordUserID:  "user-1",
				TaskName:       "Docs update",
				Status:         app.TaskStatusOpen,
				WorktreeStatus: app.TaskWorktreeStatusPending,
				CreatedAt:      time.Date(2026, time.April, 5, 1, 0, 0, 0, time.UTC),
			},
		},
		activeTasks: map[string]app.ActiveTask{
			"user-1": {
				DiscordUserID: "user-1",
				TaskID:        "task-2",
			},
		},
	}, nil, &branchReportingTaskWorkspaceManager{
		branches: map[string]string{
			"task-1": "feature/release",
		},
	})

	response, err := service.ListTasks(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}

	want := "Open tasks:\n- `Release work` (`task-1`) [branch: `feature/release`]\n- `Docs update` (`task-2`) [active]\nUse `/release action:task-switch task_name:<name>` to change the active task."
	if response.Text != want {
		t.Fatalf("Text = %q, want %q", response.Text, want)
	}
}

func TestTaskCommandServiceSwitchTaskByIDRequiresOpenTask(t *testing.T) {
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

	response, err := service.SwitchTask(context.Background(), "user-1", "task-1", "")
	if err != nil {
		t.Fatalf("SwitchTask() error = %v", err)
	}

	want := "Task `task-1` is closed. Use `/release action:task-list` to find an open task or `/release action:task-new task_name:<name>` to create another one."
	if response.Text != want {
		t.Fatalf("Text = %q, want %q", response.Text, want)
	}
}

func TestTaskCommandServiceSwitchTaskByUniqueName(t *testing.T) {
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
	}
	service := newTaskCommandService(t, store)

	response, err := service.SwitchTask(context.Background(), "user-1", "", "release work")
	if err != nil {
		t.Fatalf("SwitchTask() error = %v", err)
	}

	want := "Active task is now `Release work` (`task-1`). Your next message will continue this task."
	if response.Text != want {
		t.Fatalf("Text = %q, want %q", response.Text, want)
	}

	activeTask, ok, err := store.GetActiveTask(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetActiveTask() error = %v", err)
	}

	if !ok {
		t.Fatal("GetActiveTask() ok = false, want true")
	}

	if activeTask.TaskID != "task-1" {
		t.Fatalf("TaskID = %q, want %q", activeTask.TaskID, "task-1")
	}
}

func TestTaskCommandServiceSwitchTaskByNameRequiresIDWhenAmbiguous(t *testing.T) {
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
				TaskName:      "Release work",
				Status:        app.TaskStatusOpen,
				CreatedAt:     time.Date(2026, time.April, 5, 1, 0, 0, 0, time.UTC),
			},
		},
	})

	response, err := service.SwitchTask(context.Background(), "user-1", "", "Release work")
	if err != nil {
		t.Fatalf("SwitchTask() error = %v", err)
	}

	want := "Multiple open tasks are named `Release work`. Retry with a task ID:\n- `Release work` (`task-1`)\n- `Release work` (`task-2`)\nUse `/release action:task-switch task_id:<id>` when the task name is ambiguous."
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

	response, err := service.CloseTask(context.Background(), "user-1", "task-1", "")
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

	response, err := service.CloseTask(context.Background(), "user-1", "task-1", "")
	if err != nil {
		t.Fatalf("CloseTask() error = %v", err)
	}

	want := "Closed task `Release work` (`task-1`). Active task remains `Docs update` (`task-2`)."
	if response.Text != want {
		t.Fatalf("Text = %q, want %q", response.Text, want)
	}
}

func TestTaskCommandServiceCloseTaskTriggersPruning(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{
		tasks: map[string]app.Task{
			"user-1:task-1": {
				TaskID:         "task-1",
				DiscordUserID:  "user-1",
				TaskName:       "Release work",
				Status:         app.TaskStatusOpen,
				CreatedAt:      time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
				WorktreeStatus: app.TaskWorktreeStatusReady,
			},
		},
	}

	worktrees := &countingTaskWorkspaceManager{}
	service := newTaskCommandServiceWithWorkspace(t, store, nil, worktrees)

	if _, err := service.CloseTask(context.Background(), "user-1", "task-1", ""); err != nil {
		t.Fatalf("CloseTask() error = %v", err)
	}

	if worktrees.pruneCalls != 1 {
		t.Fatalf("PruneClosed() call count = %d, want %d", worktrees.pruneCalls, 1)
	}
}

func TestTaskCommandServiceCloseTaskByUniqueName(t *testing.T) {
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

	response, err := service.CloseTask(context.Background(), "user-1", "", "Release work")
	if err != nil {
		t.Fatalf("CloseTask() error = %v", err)
	}

	want := "Closed task `Release work` (`task-1`). No active task is selected now."
	if response.Text != want {
		t.Fatalf("Text = %q, want %q", response.Text, want)
	}
}

func TestTaskCommandServiceCloseTaskByNameRequiresIDWhenAmbiguous(t *testing.T) {
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
				TaskName:      "Release work",
				Status:        app.TaskStatusOpen,
				CreatedAt:     time.Date(2026, time.April, 5, 1, 0, 0, 0, time.UTC),
			},
		},
	})

	response, err := service.CloseTask(context.Background(), "user-1", "", "Release work")
	if err != nil {
		t.Fatalf("CloseTask() error = %v", err)
	}

	want := "Multiple open tasks are named `Release work`. Retry with a task ID:\n- `Release work` (`task-1`)\n- `Release work` (`task-2`)\nUse `/release action:task-close task_id:<id>` when the task name is ambiguous."
	if response.Text != want {
		t.Fatalf("Text = %q, want %q", response.Text, want)
	}
}

func TestTaskCommandServiceResetContextClearsBindingWhenIdle(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{
		tasks: map[string]app.Task{
			"user-1:task-1": {
				TaskID:        "task-1",
				DiscordUserID: "user-1",
				TaskName:      "Release work",
				Status:        app.TaskStatusOpen,
			},
		},
		activeTasks: map[string]app.ActiveTask{
			"user-1": {
				DiscordUserID: "user-1",
				TaskID:        "task-1",
			},
		},
		bindings: map[string]app.ThreadBinding{
			"thread:user-1:task-1": {
				Mode:             "thread",
				LogicalThreadKey: "user-1:task-1",
				CodexThreadID:    "thread-old",
				TaskID:           "task-1",
			},
		},
	}
	service := newTaskCommandServiceWithCoordinator(t, store, &stubQueueCoordinator{})

	response, err := service.ResetContext(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("ResetContext() error = %v", err)
	}

	want := "Reset Codex conversation continuity for active task `Release work` (`task-1`). The task is still active, the workspace is unchanged, and your next normal message will start a fresh Codex thread for this task."
	if response.Text != want {
		t.Fatalf("Text = %q, want %q", response.Text, want)
	}

	if _, ok, err := store.GetThreadBinding(context.Background(), "thread", "user-1:task-1"); err != nil {
		t.Fatalf("GetThreadBinding() error = %v", err)
	} else if ok {
		t.Fatal("GetThreadBinding() ok = true, want false")
	}
}

func TestTaskCommandServiceResetContextWithoutBindingReturnsNoOp(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{
		tasks: map[string]app.Task{
			"user-1:task-1": {
				TaskID:        "task-1",
				DiscordUserID: "user-1",
				TaskName:      "Release work",
				Status:        app.TaskStatusOpen,
			},
		},
		activeTasks: map[string]app.ActiveTask{
			"user-1": {
				DiscordUserID: "user-1",
				TaskID:        "task-1",
			},
		},
	}
	service := newTaskCommandServiceWithCoordinator(t, store, &stubQueueCoordinator{})

	response, err := service.ResetContext(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("ResetContext() error = %v", err)
	}

	want := "Active task `Release work` (`task-1`) does not have saved Codex conversation continuity yet. The task is still active, the workspace is unchanged, and your next normal message will already start fresh."
	if response.Text != want {
		t.Fatalf("Text = %q, want %q", response.Text, want)
	}
}

func TestTaskCommandServiceResetContextRejectsBusyTask(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{
		tasks: map[string]app.Task{
			"user-1:task-1": {
				TaskID:        "task-1",
				DiscordUserID: "user-1",
				TaskName:      "Release work",
				Status:        app.TaskStatusOpen,
			},
		},
		activeTasks: map[string]app.ActiveTask{
			"user-1": {
				DiscordUserID: "user-1",
				TaskID:        "task-1",
			},
		},
		bindings: map[string]app.ThreadBinding{
			"thread:user-1:task-1": {
				Mode:             "thread",
				LogicalThreadKey: "user-1:task-1",
				CodexThreadID:    "thread-old",
				TaskID:           "task-1",
			},
		},
	}
	service := newTaskCommandServiceWithCoordinator(t, store, &stubQueueCoordinator{
		snapshot: app.QueueSnapshot{InFlight: true, Queued: 1},
	})

	response, err := service.ResetContext(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("ResetContext() error = %v", err)
	}

	want := "This task still has running or queued work. Wait for pending replies to finish, then retry `/release action:task-reset-context`."
	if response.Text != want {
		t.Fatalf("Text = %q, want %q", response.Text, want)
	}

	if _, ok, err := store.GetThreadBinding(context.Background(), "thread", "user-1:task-1"); err != nil {
		t.Fatalf("GetThreadBinding() error = %v", err)
	} else if !ok {
		t.Fatal("GetThreadBinding() ok = false, want true")
	}
}

func TestTaskCommandServiceResetContextWithoutActiveTaskReturnsGuidance(t *testing.T) {
	t.Parallel()

	service := newTaskCommandServiceWithCoordinator(t, &memoryThreadStore{}, &stubQueueCoordinator{})

	response, err := service.ResetContext(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("ResetContext() error = %v", err)
	}

	want := "No active task is selected. Use `/release action:task-new task_name:<name>`, `/release action:task-list`, or `/release action:task-switch task_name:<name>` first."
	if response.Text != want {
		t.Fatalf("Text = %q, want %q", response.Text, want)
	}
}

func newTaskCommandService(t *testing.T, store app.ThreadStore) *app.DefaultTaskCommandService {
	t.Helper()
	return newTaskCommandServiceWithWorkspaceAndCoordinator(t, store, nil, nil, &stubQueueCoordinator{})
}

func newTaskCommandServiceWithID(
	t *testing.T,
	store app.ThreadStore,
	newTaskID func() string,
) *app.DefaultTaskCommandService {
	t.Helper()
	return newTaskCommandServiceWithWorkspaceAndCoordinator(t, store, newTaskID, nil, &stubQueueCoordinator{})
}

func newTaskCommandServiceWithCoordinator(
	t *testing.T,
	store app.ThreadStore,
	coordinator app.QueueCoordinator,
) *app.DefaultTaskCommandService {
	t.Helper()
	return newTaskCommandServiceWithWorkspaceAndCoordinator(t, store, nil, nil, coordinator)
}

func newTaskCommandServiceWithWorkspace(
	t *testing.T,
	store app.ThreadStore,
	newTaskID func() string,
	worktrees app.TaskWorkspaceManager,
) *app.DefaultTaskCommandService {
	t.Helper()
	return newTaskCommandServiceWithWorkspaceAndCoordinator(t, store, newTaskID, worktrees, &stubQueueCoordinator{})
}

func newTaskCommandServiceWithWorkspaceAndCoordinator(
	t *testing.T,
	store app.ThreadStore,
	newTaskID func() string,
	worktrees app.TaskWorkspaceManager,
	coordinator app.QueueCoordinator,
) *app.DefaultTaskCommandService {
	t.Helper()

	service, err := app.NewTaskCommandService(app.TaskCommandServiceDependencies{
		CommandName:      "release",
		Store:            store,
		Coordinator:      coordinator,
		WorkspaceManager: worktrees,
		NewTaskID:        newTaskID,
	})
	if err != nil {
		t.Fatalf("NewTaskCommandService() error = %v", err)
	}

	return service
}

type countingTaskWorkspaceManager struct {
	pruneCalls int
}

func (*countingTaskWorkspaceManager) EnsureReady(context.Context, app.Task) (app.Task, error) {
	return app.Task{}, nil
}

func (m *countingTaskWorkspaceManager) PruneClosed(context.Context) error {
	m.pruneCalls++
	return nil
}

type branchReportingTaskWorkspaceManager struct {
	branches map[string]string
}

func (*branchReportingTaskWorkspaceManager) EnsureReady(context.Context, app.Task) (app.Task, error) {
	return app.Task{}, nil
}

func (*branchReportingTaskWorkspaceManager) PruneClosed(context.Context) error {
	return nil
}

func (m *branchReportingTaskWorkspaceManager) CurrentBranch(_ context.Context, task app.Task) (string, bool, error) {
	branch, ok := m.branches[task.TaskID]
	return branch, ok, nil
}
