package app_test

import (
	"context"
	"database/sql"
	"sort"
	"testing"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	"github.com/HatsuneMiku3939/39claw/internal/config"
	"github.com/HatsuneMiku3939/39claw/internal/thread"
)

func TestMessageServiceHandleMessageIgnoresNonMentionChatter(t *testing.T) {
	t.Parallel()

	service := newDailyMessageService(t, &memoryThreadStore{}, &fakeCodexGateway{}, &stubExecutionGuard{})

	response, err := service.HandleMessage(context.Background(), app.MessageRequest{
		MessageID:  "message-1",
		Content:    "just chatting",
		Mentioned:  false,
		ReceivedAt: time.Date(2026, time.April, 5, 9, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}

	if !response.Ignore {
		t.Fatal("Ignore = false, want true")
	}
}

func TestMessageServiceHandleMessageDailyReusesSameDayBinding(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{}
	gateway := &fakeCodexGateway{
		results: []app.RunTurnResult{
			{ThreadID: "thread-1", ResponseText: "First response"},
			{ThreadID: "thread-1", ResponseText: "Second response"},
		},
	}
	service := newDailyMessageService(t, store, gateway, &stubExecutionGuard{})

	firstResponse, err := service.HandleMessage(context.Background(), app.MessageRequest{
		MessageID:  "message-1",
		Content:    "hello there",
		Mentioned:  true,
		ReceivedAt: time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("HandleMessage() first error = %v", err)
	}

	secondResponse, err := service.HandleMessage(context.Background(), app.MessageRequest{
		MessageID:  "message-2",
		Content:    "follow up",
		Mentioned:  true,
		ReceivedAt: time.Date(2026, time.April, 5, 7, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("HandleMessage() second error = %v", err)
	}

	if firstResponse.Text != "First response" {
		t.Fatalf("first response text = %q, want %q", firstResponse.Text, "First response")
	}

	if secondResponse.Text != "Second response" {
		t.Fatalf("second response text = %q, want %q", secondResponse.Text, "Second response")
	}

	if secondResponse.ReplyToID != "message-2" {
		t.Fatalf("second reply id = %q, want %q", secondResponse.ReplyToID, "message-2")
	}

	if len(gateway.calls) != 2 {
		t.Fatalf("RunTurn() call count = %d, want %d", len(gateway.calls), 2)
	}

	if gateway.calls[0].threadID != "" {
		t.Fatalf("first thread id = %q, want empty", gateway.calls[0].threadID)
	}

	if gateway.calls[1].threadID != "thread-1" {
		t.Fatalf("second thread id = %q, want %q", gateway.calls[1].threadID, "thread-1")
	}

	binding, ok, err := store.GetThreadBinding(context.Background(), "daily", "2026-04-05")
	if err != nil {
		t.Fatalf("GetThreadBinding() error = %v", err)
	}

	if !ok {
		t.Fatal("GetThreadBinding() ok = false, want true")
	}

	if binding.CodexThreadID != "thread-1" {
		t.Fatalf("CodexThreadID = %q, want %q", binding.CodexThreadID, "thread-1")
	}
}

func TestMessageServiceHandleMessageDailyRollsOverOnNextDay(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{}
	gateway := &fakeCodexGateway{
		results: []app.RunTurnResult{
			{ThreadID: "thread-1", ResponseText: "Today"},
			{ThreadID: "thread-2", ResponseText: "Tomorrow"},
		},
	}
	service := newDailyMessageService(t, store, gateway, &stubExecutionGuard{})

	_, err := service.HandleMessage(context.Background(), app.MessageRequest{
		MessageID:  "message-1",
		Content:    "today",
		Mentioned:  true,
		ReceivedAt: time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("HandleMessage() first error = %v", err)
	}

	_, err = service.HandleMessage(context.Background(), app.MessageRequest{
		MessageID:  "message-2",
		Content:    "tomorrow",
		Mentioned:  true,
		ReceivedAt: time.Date(2026, time.April, 5, 15, 1, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("HandleMessage() second error = %v", err)
	}

	if len(gateway.calls) != 2 {
		t.Fatalf("RunTurn() call count = %d, want %d", len(gateway.calls), 2)
	}

	if gateway.calls[0].threadID != "" {
		t.Fatalf("first thread id = %q, want empty", gateway.calls[0].threadID)
	}

	if gateway.calls[1].threadID != "" {
		t.Fatalf("second thread id = %q, want empty", gateway.calls[1].threadID)
	}

	if _, ok, err := store.GetThreadBinding(context.Background(), "daily", "2026-04-05"); err != nil || !ok {
		t.Fatalf("same-day binding lookup = ok:%v err:%v, want ok:true err:nil", ok, err)
	}

	nextBinding, ok, err := store.GetThreadBinding(context.Background(), "daily", "2026-04-06")
	if err != nil {
		t.Fatalf("GetThreadBinding() next day error = %v", err)
	}

	if !ok {
		t.Fatal("GetThreadBinding() next day ok = false, want true")
	}

	if nextBinding.CodexThreadID != "thread-2" {
		t.Fatalf("next day thread id = %q, want %q", nextBinding.CodexThreadID, "thread-2")
	}
}

func TestMessageServiceHandleMessageReturnsBusyResponse(t *testing.T) {
	t.Parallel()

	service := newDailyMessageService(t, &memoryThreadStore{}, &fakeCodexGateway{}, &stubExecutionGuard{
		err: app.ErrExecutionInProgress,
	})

	response, err := service.HandleMessage(context.Background(), app.MessageRequest{
		MessageID:  "message-1",
		Content:    "hello",
		Mentioned:  true,
		ReceivedAt: time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}

	if response.Text != "A response is already running for this conversation. Please retry in a moment." {
		t.Fatalf("busy response text = %q", response.Text)
	}

	if response.ReplyToID != "message-1" {
		t.Fatalf("ReplyToID = %q, want %q", response.ReplyToID, "message-1")
	}
}

func TestMessageServiceHandleMessageReturnsTaskGuidance(t *testing.T) {
	t.Parallel()

	service, err := app.NewMessageService(app.MessageServiceDependencies{
		Mode: config.ModeTask,
		Policy: stubThreadPolicy{
			err: app.ErrNoActiveTask,
		},
		Store:   &memoryThreadStore{},
		Gateway: &fakeCodexGateway{},
		Guard:   &stubExecutionGuard{},
	})
	if err != nil {
		t.Fatalf("NewMessageService() error = %v", err)
	}

	response, err := service.HandleMessage(context.Background(), app.MessageRequest{
		MessageID: "message-1",
		Content:   "do the work",
		Mentioned: true,
	})
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}

	if response.Text != "No active task is selected. Use `/task new <name>`, `/task list`, or `/task switch <id>` first." {
		t.Fatalf("guidance text = %q", response.Text)
	}
}

func TestMessageServiceHandleMessageTaskReusesTaskBindingAcrossDays(t *testing.T) {
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
	gateway := &fakeCodexGateway{
		results: []app.RunTurnResult{
			{ThreadID: "thread-task-1", ResponseText: "First response"},
			{ThreadID: "thread-task-1", ResponseText: "Second response"},
		},
	}
	service := newTaskMessageService(t, store, gateway, &stubExecutionGuard{})

	for _, request := range []app.MessageRequest{
		{
			UserID:     "user-1",
			MessageID:  "message-1",
			Content:    "start release",
			Mentioned:  true,
			ReceivedAt: time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
		},
		{
			UserID:     "user-1",
			MessageID:  "message-2",
			Content:    "continue release",
			Mentioned:  true,
			ReceivedAt: time.Date(2026, time.April, 7, 0, 0, 0, 0, time.UTC),
		},
	} {
		if _, err := service.HandleMessage(context.Background(), request); err != nil {
			t.Fatalf("HandleMessage(%s) error = %v", request.MessageID, err)
		}
	}

	if len(gateway.calls) != 2 {
		t.Fatalf("RunTurn() call count = %d, want %d", len(gateway.calls), 2)
	}

	if gateway.calls[0].threadID != "" {
		t.Fatalf("first thread id = %q, want empty", gateway.calls[0].threadID)
	}

	if gateway.calls[1].threadID != "thread-task-1" {
		t.Fatalf("second thread id = %q, want %q", gateway.calls[1].threadID, "thread-task-1")
	}

	binding, ok, err := store.GetThreadBinding(context.Background(), "task", "user-1:task-1")
	if err != nil {
		t.Fatalf("GetThreadBinding() error = %v", err)
	}

	if !ok {
		t.Fatal("GetThreadBinding() ok = false, want true")
	}

	if binding.TaskID != "task-1" {
		t.Fatalf("TaskID = %q, want %q", binding.TaskID, "task-1")
	}
}

func TestMessageServiceHandleMessageTaskSwitchesThreadsByActiveTask(t *testing.T) {
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
				TaskID:        "task-1",
			},
		},
	}
	gateway := &fakeCodexGateway{
		results: []app.RunTurnResult{
			{ThreadID: "thread-task-1", ResponseText: "Release response"},
			{ThreadID: "thread-task-2", ResponseText: "Docs response"},
		},
	}
	service := newTaskMessageService(t, store, gateway, &stubExecutionGuard{})

	if _, err := service.HandleMessage(context.Background(), app.MessageRequest{
		UserID:     "user-1",
		MessageID:  "message-1",
		Content:    "release task",
		Mentioned:  true,
		ReceivedAt: time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("HandleMessage() first error = %v", err)
	}

	if err := store.SetActiveTask(context.Background(), app.ActiveTask{
		DiscordUserID: "user-1",
		TaskID:        "task-2",
	}); err != nil {
		t.Fatalf("SetActiveTask() error = %v", err)
	}

	if _, err := service.HandleMessage(context.Background(), app.MessageRequest{
		UserID:     "user-1",
		MessageID:  "message-2",
		Content:    "docs task",
		Mentioned:  true,
		ReceivedAt: time.Date(2026, time.April, 8, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("HandleMessage() second error = %v", err)
	}

	if len(gateway.calls) != 2 {
		t.Fatalf("RunTurn() call count = %d, want %d", len(gateway.calls), 2)
	}

	if gateway.calls[1].threadID != "" {
		t.Fatalf("second thread id = %q, want empty", gateway.calls[1].threadID)
	}

	for _, key := range []string{"user-1:task-1", "user-1:task-2"} {
		if _, ok, err := store.GetThreadBinding(context.Background(), "task", key); err != nil || !ok {
			t.Fatalf("GetThreadBinding(%s) = ok:%v err:%v, want ok:true err:nil", key, ok, err)
		}
	}
}

func newDailyMessageService(
	t *testing.T,
	store app.ThreadStore,
	gateway app.CodexGateway,
	guard app.ExecutionGuard,
) *app.DefaultMessageService {
	t.Helper()

	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	policy, err := thread.NewPolicy(config.ModeDaily, tokyo, nil)
	if err != nil {
		t.Fatalf("thread.NewPolicy() error = %v", err)
	}

	service, err := app.NewMessageService(app.MessageServiceDependencies{
		Mode:    config.ModeDaily,
		Policy:  policy,
		Store:   store,
		Gateway: gateway,
		Guard:   guard,
	})
	if err != nil {
		t.Fatalf("NewMessageService() error = %v", err)
	}

	return service
}

func newTaskMessageService(
	t *testing.T,
	store app.ThreadStore,
	gateway app.CodexGateway,
	guard app.ExecutionGuard,
) *app.DefaultMessageService {
	t.Helper()

	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	policy, err := thread.NewPolicy(config.ModeTask, tokyo, store)
	if err != nil {
		t.Fatalf("thread.NewPolicy() error = %v", err)
	}

	service, err := app.NewMessageService(app.MessageServiceDependencies{
		Mode:    config.ModeTask,
		Policy:  policy,
		Store:   store,
		Gateway: gateway,
		Guard:   guard,
	})
	if err != nil {
		t.Fatalf("NewMessageService() error = %v", err)
	}

	return service
}

type stubThreadPolicy struct {
	key string
	err error
}

func (s stubThreadPolicy) ResolveMessageKey(context.Context, app.MessageRequest) (string, error) {
	return s.key, s.err
}

type stubExecutionGuard struct {
	err error
}

func (g *stubExecutionGuard) Acquire(string) (app.ReleaseFunc, error) {
	if g.err != nil {
		return nil, g.err
	}

	return func() {}, nil
}

type fakeCodexGateway struct {
	calls   []runTurnCall
	results []app.RunTurnResult
	err     error
}

type runTurnCall struct {
	threadID string
	prompt   string
}

func (g *fakeCodexGateway) RunTurn(_ context.Context, threadID string, prompt string) (app.RunTurnResult, error) {
	g.calls = append(g.calls, runTurnCall{
		threadID: threadID,
		prompt:   prompt,
	})

	if g.err != nil {
		return app.RunTurnResult{}, g.err
	}

	if len(g.results) == 0 {
		return app.RunTurnResult{}, nil
	}

	result := g.results[0]
	g.results = g.results[1:]
	return result, nil
}

type memoryThreadStore struct {
	bindings    map[string]app.ThreadBinding
	tasks       map[string]app.Task
	activeTasks map[string]app.ActiveTask
}

func (s *memoryThreadStore) GetThreadBinding(_ context.Context, mode string, logicalThreadKey string) (app.ThreadBinding, bool, error) {
	if s.bindings == nil {
		return app.ThreadBinding{}, false, nil
	}

	binding, ok := s.bindings[mode+":"+logicalThreadKey]
	return binding, ok, nil
}

func (s *memoryThreadStore) UpsertThreadBinding(_ context.Context, binding app.ThreadBinding) error {
	if s.bindings == nil {
		s.bindings = make(map[string]app.ThreadBinding)
	}

	s.bindings[binding.Mode+":"+binding.LogicalThreadKey] = binding
	return nil
}

func (s *memoryThreadStore) CreateTask(_ context.Context, task app.Task) error {
	if s.tasks == nil {
		s.tasks = make(map[string]app.Task)
	}

	if task.Status == "" {
		task.Status = app.TaskStatusOpen
	}

	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC)
	}

	task.UpdatedAt = task.CreatedAt
	s.tasks[task.DiscordUserID+":"+task.TaskID] = task
	return nil
}

func (s *memoryThreadStore) GetTask(_ context.Context, userID string, taskID string) (app.Task, bool, error) {
	if s.tasks == nil {
		return app.Task{}, false, nil
	}

	task, ok := s.tasks[userID+":"+taskID]
	return task, ok, nil
}

func (s *memoryThreadStore) ListOpenTasks(_ context.Context, userID string) ([]app.Task, error) {
	if s.tasks == nil {
		return nil, nil
	}

	tasks := make([]app.Task, 0)
	for _, task := range s.tasks {
		if task.DiscordUserID == userID && task.Status == app.TaskStatusOpen {
			tasks = append(tasks, task)
		}
	}

	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].CreatedAt.Equal(tasks[j].CreatedAt) {
			return tasks[i].TaskID < tasks[j].TaskID
		}
		return tasks[i].CreatedAt.Before(tasks[j].CreatedAt)
	})

	return tasks, nil
}

func (s *memoryThreadStore) SetActiveTask(_ context.Context, activeTask app.ActiveTask) error {
	if s.activeTasks == nil {
		s.activeTasks = make(map[string]app.ActiveTask)
	}

	s.activeTasks[activeTask.DiscordUserID] = activeTask
	return nil
}

func (s *memoryThreadStore) GetActiveTask(_ context.Context, userID string) (app.ActiveTask, bool, error) {
	if s.activeTasks == nil {
		return app.ActiveTask{}, false, nil
	}

	activeTask, ok := s.activeTasks[userID]
	return activeTask, ok, nil
}

func (s *memoryThreadStore) ClearActiveTask(_ context.Context, userID string) error {
	if s.activeTasks != nil {
		delete(s.activeTasks, userID)
	}
	return nil
}

func (s *memoryThreadStore) CloseTask(_ context.Context, userID string, taskID string) error {
	if s.tasks == nil {
		return sql.ErrNoRows
	}

	key := userID + ":" + taskID
	task, ok := s.tasks[key]
	if !ok {
		return sql.ErrNoRows
	}

	closedAt := time.Date(2026, time.April, 5, 12, 0, 0, 0, time.UTC)
	task.Status = app.TaskStatusClosed
	task.ClosedAt = &closedAt
	s.tasks[key] = task

	if s.activeTasks != nil {
		if activeTask, ok := s.activeTasks[userID]; ok && activeTask.TaskID == taskID {
			delete(s.activeTasks, userID)
		}
	}

	return nil
}
