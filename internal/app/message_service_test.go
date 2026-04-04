package app_test

import (
	"context"
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
	bindings map[string]app.ThreadBinding
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

func (s *memoryThreadStore) CreateTask(context.Context, app.Task) error {
	return nil
}

func (s *memoryThreadStore) GetTask(context.Context, string, string) (app.Task, bool, error) {
	return app.Task{}, false, nil
}

func (s *memoryThreadStore) ListOpenTasks(context.Context, string) ([]app.Task, error) {
	return nil, nil
}

func (s *memoryThreadStore) SetActiveTask(context.Context, app.ActiveTask) error {
	return nil
}

func (s *memoryThreadStore) GetActiveTask(context.Context, string) (app.ActiveTask, bool, error) {
	return app.ActiveTask{}, false, nil
}

func (s *memoryThreadStore) ClearActiveTask(context.Context, string) error {
	return nil
}

func (s *memoryThreadStore) CloseTask(context.Context, string, string) error {
	return nil
}
