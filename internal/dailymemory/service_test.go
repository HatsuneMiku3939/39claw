package dailymemory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
)

func TestRefresherRefreshBeforeFirstDailyTurnUsesPreviousBinding(t *testing.T) {
	t.Parallel()

	workdir := t.TempDir()
	if err := (Bootstrap{Workdir: workdir}).Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}

	tokyo := mustLoadLocation(t, "Asia/Tokyo")
	store := &stubThreadStore{
		bindings: map[string]app.ThreadBinding{
			"daily:2026-04-05": {
				Mode:             "daily",
				LogicalThreadKey: "2026-04-05",
				CodexThreadID:    "thread-previous",
			},
		},
	}

	memoryPath := filepath.Join(workdir, memoryDirName, memoryFileName)
	bridgePath := filepath.Join(workdir, memoryDirName, "2026-04-06.md")
	gateway := &stubCodexGateway{
		result: app.RunTurnResult{
			ThreadID:     "thread-previous",
			ResponseText: "MEMORY_REFRESH_OK\nUpdated:\n- " + memoryPath + "\n- " + bridgePath,
		},
	}

	refresher := Refresher{
		Timezone: tokyo,
		Store:    store,
		Gateway:  gateway,
		Workdir:  workdir,
		Timeout:  time.Second,
	}

	err := refresher.RefreshBeforeFirstDailyTurn(
		context.Background(),
		"2026-04-06",
		time.Date(2026, time.April, 6, 9, 0, 0, 0, tokyo),
	)
	if err != nil {
		t.Fatalf("RefreshBeforeFirstDailyTurn() error = %v", err)
	}

	if len(gateway.calls) != 1 {
		t.Fatalf("RunTurn() call count = %d, want 1", len(gateway.calls))
	}

	call := gateway.calls[0]
	if call.threadID != "thread-previous" {
		t.Fatalf("threadID = %q, want %q", call.threadID, "thread-previous")
	}

	if call.input.WorkingDirectory != workdir {
		t.Fatalf("WorkingDirectory = %q, want %q", call.input.WorkingDirectory, workdir)
	}

	if call.input.Prompt != buildRefreshPrompt("2026-04-05", "2026-04-06") {
		t.Fatalf("prompt = %q, want %q", call.input.Prompt, buildRefreshPrompt("2026-04-05", "2026-04-06"))
	}

	bridgeContents, err := os.ReadFile(bridgePath)
	if err != nil {
		t.Fatalf("ReadFile(bridge note) error = %v", err)
	}

	expectedBridge := "# Daily Memory Bridge for 2026-04-06\n\n" +
		"## Source\n\n" +
		"- Previous thread id: `thread-previous`\n" +
		"- Source day: `2026-04-05`\n\n" +
		"## Durable Facts Promoted\n\n" +
		"- None yet.\n\n" +
		"## MEMORY.md Updates Applied\n\n" +
		"- None yet.\n\n" +
		"## Rejected Candidates\n\n" +
		"- None yet.\n\n" +
		"## Notes\n\n" +
		"- Created by the 39claw daily memory preflight before the first visible turn of the new day.\n"
	if string(bridgeContents) != expectedBridge {
		t.Fatalf("bridge note contents = %q, want %q", string(bridgeContents), expectedBridge)
	}
}

func TestRefresherSkipsWhenCurrentBindingExists(t *testing.T) {
	t.Parallel()

	refresher := Refresher{
		Timezone: mustLoadLocation(t, "Asia/Tokyo"),
		Store: &stubThreadStore{
			bindings: map[string]app.ThreadBinding{
				"daily:2026-04-06": {
					Mode:             "daily",
					LogicalThreadKey: "2026-04-06",
					CodexThreadID:    "thread-current",
				},
			},
		},
		Gateway: &stubCodexGateway{},
		Workdir: t.TempDir(),
	}

	if err := refresher.RefreshBeforeFirstDailyTurn(context.Background(), "2026-04-06", time.Time{}); err != nil {
		t.Fatalf("RefreshBeforeFirstDailyTurn() error = %v", err)
	}
}

func TestRefresherSkipsWhenPreviousBindingIsMissing(t *testing.T) {
	t.Parallel()

	gateway := &stubCodexGateway{}
	refresher := Refresher{
		Timezone: mustLoadLocation(t, "Asia/Tokyo"),
		Store:    &stubThreadStore{},
		Gateway:  gateway,
		Workdir:  t.TempDir(),
	}

	if err := refresher.RefreshBeforeFirstDailyTurn(context.Background(), "2026-04-06", time.Time{}); err != nil {
		t.Fatalf("RefreshBeforeFirstDailyTurn() error = %v", err)
	}

	if len(gateway.calls) != 0 {
		t.Fatalf("RunTurn() call count = %d, want 0", len(gateway.calls))
	}
}

func TestRefresherReturnsErrorWhenCompletionFormatIsUnexpected(t *testing.T) {
	t.Parallel()

	workdir := t.TempDir()
	refresher := Refresher{
		Timezone: mustLoadLocation(t, "Asia/Tokyo"),
		Store: &stubThreadStore{
			bindings: map[string]app.ThreadBinding{
				"daily:2026-04-05": {
					Mode:             "daily",
					LogicalThreadKey: "2026-04-05",
					CodexThreadID:    "thread-previous",
				},
			},
		},
		Gateway: &stubCodexGateway{
			result: app.RunTurnResult{
				ThreadID:     "thread-previous",
				ResponseText: "not the required format",
			},
		},
		Workdir: workdir,
	}

	err := refresher.RefreshBeforeFirstDailyTurn(context.Background(), "2026-04-06", time.Time{})
	if err == nil {
		t.Fatal("RefreshBeforeFirstDailyTurn() error = nil, want non-nil")
	}
}

func TestRefresherReturnsTimeout(t *testing.T) {
	t.Parallel()

	workdir := t.TempDir()
	refresher := Refresher{
		Timezone: mustLoadLocation(t, "Asia/Tokyo"),
		Store: &stubThreadStore{
			bindings: map[string]app.ThreadBinding{
				"daily:2026-04-05": {
					Mode:             "daily",
					LogicalThreadKey: "2026-04-05",
					CodexThreadID:    "thread-previous",
				},
			},
		},
		Gateway: timeoutCodexGateway{},
		Workdir: workdir,
		Timeout: 10 * time.Millisecond,
	}

	err := refresher.RefreshBeforeFirstDailyTurn(context.Background(), "2026-04-06", time.Time{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("RefreshBeforeFirstDailyTurn() error = %v, want deadline exceeded", err)
	}
}

type stubThreadStore struct {
	bindings map[string]app.ThreadBinding
}

func (s *stubThreadStore) GetThreadBinding(_ context.Context, mode string, logicalThreadKey string) (app.ThreadBinding, bool, error) {
	if s.bindings == nil {
		return app.ThreadBinding{}, false, nil
	}

	binding, ok := s.bindings[mode+":"+logicalThreadKey]
	return binding, ok, nil
}

func (s *stubThreadStore) UpsertThreadBinding(context.Context, app.ThreadBinding) error {
	return nil
}

func (s *stubThreadStore) CreateTask(context.Context, app.Task) error {
	return nil
}

func (s *stubThreadStore) GetTask(context.Context, string, string) (app.Task, bool, error) {
	return app.Task{}, false, nil
}

func (s *stubThreadStore) UpdateTask(context.Context, app.Task) error {
	return nil
}

func (s *stubThreadStore) ListOpenTasks(context.Context, string) ([]app.Task, error) {
	return nil, nil
}

func (s *stubThreadStore) ListClosedReadyTasks(context.Context) ([]app.Task, error) {
	return nil, nil
}

func (s *stubThreadStore) SetActiveTask(context.Context, app.ActiveTask) error {
	return nil
}

func (s *stubThreadStore) GetActiveTask(context.Context, string) (app.ActiveTask, bool, error) {
	return app.ActiveTask{}, false, nil
}

func (s *stubThreadStore) ClearActiveTask(context.Context, string) error {
	return nil
}

func (s *stubThreadStore) CloseTask(context.Context, string, string) error {
	return nil
}

type codexCall struct {
	threadID string
	input    app.CodexTurnInput
}

type stubCodexGateway struct {
	calls  []codexCall
	result app.RunTurnResult
	err    error
}

func (g *stubCodexGateway) RunTurn(ctx context.Context, threadID string, input app.CodexTurnInput) (app.RunTurnResult, error) {
	g.calls = append(g.calls, codexCall{
		threadID: threadID,
		input:    input,
	})

	if g.err != nil {
		return app.RunTurnResult{}, g.err
	}

	return g.result, nil
}

type timeoutCodexGateway struct{}

func (timeoutCodexGateway) RunTurn(ctx context.Context, threadID string, input app.CodexTurnInput) (app.RunTurnResult, error) {
	<-ctx.Done()
	return app.RunTurnResult{}, ctx.Err()
}

func mustLoadLocation(t *testing.T, name string) *time.Location {
	t.Helper()

	location, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("time.LoadLocation(%q) error = %v", name, err)
	}

	return location
}
