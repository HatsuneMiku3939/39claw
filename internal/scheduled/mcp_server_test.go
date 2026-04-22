package scheduled

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	sqlitestore "github.com/HatsuneMiku3939/39claw/internal/store/sqlite"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestMCPServerCreateAndListScheduledTasks(t *testing.T) {
	t.Parallel()

	db, err := sqlitestore.OpenDB(context.Background(), filepath.Join(t.TempDir(), "39claw.db"))
	if err != nil {
		t.Fatalf("OpenDB() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Fatalf("db.Close() error = %v", closeErr)
		}
	})

	if err := sqlitestore.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}

	scheduledServer := &MCPServer{
		Store:               sqlitestore.New(db),
		Timezone:            location,
		DefaultReportTarget: "channel:12345",
		Now: func() time.Time {
			return time.Date(2026, time.April, 12, 8, 0, 0, 0, time.UTC)
		},
	}

	mcpServer, err := scheduledServer.BuildServer()
	if err != nil {
		t.Fatalf("BuildServer() error = %v", err)
	}

	mcpClient, err := client.NewInProcessClient(mcpServer)
	if err != nil {
		t.Fatalf("NewInProcessClient() error = %v", err)
	}
	defer mcpClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := mcpClient.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	initializeMCPClient(t, ctx, mcpClient)

	createResult, err := mcpClient.CallTool(ctx, callToolRequest("scheduled_tasks_create", map[string]any{
		"name":          "daily-report",
		"schedule_kind": "cron",
		"schedule_expr": "0 9 * * *",
		"prompt":        "Write the daily report.",
		"enabled":       true,
	}))
	if err != nil {
		t.Fatalf("CallTool(create) error = %v", err)
	}

	createdTask := decodeStructuredTask(t, createResult.StructuredContent)
	if createdTask.Name != "daily-report" {
		t.Fatalf("created task name = %q, want %q", createdTask.Name, "daily-report")
	}
	if !createdTask.Enabled {
		t.Fatal("created task Enabled = false, want true")
	}

	listResult, err := mcpClient.CallTool(ctx, callToolRequest("scheduled_tasks_list", map[string]any{}))
	if err != nil {
		t.Fatalf("CallTool(list) error = %v", err)
	}

	listedTasks := decodeStructuredTasks(t, listResult.StructuredContent)
	if len(listedTasks) != 1 {
		t.Fatalf("listed task count = %d, want %d", len(listedTasks), 1)
	}
	if listedTasks[0].Name != "daily-report" {
		t.Fatalf("listed task name = %q, want %q", listedTasks[0].Name, "daily-report")
	}
}

func TestMCPServerExecuteNowUsesExecutor(t *testing.T) {
	t.Parallel()

	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}

	scheduledServer := &MCPServer{
		Store:               &fakeScheduledTaskStore{},
		Executor:            fakeScheduledTaskExecutor{},
		Timezone:            location,
		DefaultReportTarget: "channel:12345",
	}

	mcpServer, err := scheduledServer.BuildServer()
	if err != nil {
		t.Fatalf("BuildServer() error = %v", err)
	}

	mcpClient, err := client.NewInProcessClient(mcpServer)
	if err != nil {
		t.Fatalf("NewInProcessClient() error = %v", err)
	}
	defer mcpClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := mcpClient.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	initializeMCPClient(t, ctx, mcpClient)

	result, err := mcpClient.CallTool(ctx, callToolRequest("scheduled_tasks_execute_now", map[string]any{
		"name": "daily-report",
	}))
	if err != nil {
		t.Fatalf("CallTool(execute_now) error = %v", err)
	}
	if result.IsError {
		t.Fatal("CallTool(execute_now) IsError = true, want false")
	}

	run := decodeStructuredRun(t, result.StructuredContent)
	if run.ScheduledRunID != "run-debug-1" {
		t.Fatalf("run ID = %q, want %q", run.ScheduledRunID, "run-debug-1")
	}
	if run.Status != app.ScheduledTaskRunStatusSucceeded {
		t.Fatalf("run status = %q, want %q", run.Status, app.ScheduledTaskRunStatusSucceeded)
	}
}

func initializeMCPClient(t *testing.T, ctx context.Context, mcpClient *client.Client) {
	t.Helper()

	request := mcp.InitializeRequest{}
	request.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	request.Params.ClientInfo = mcp.Implementation{
		Name:    "39claw-test-client",
		Version: "1.0.0",
	}

	if _, err := mcpClient.Initialize(ctx, request); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
}

func callToolRequest(name string, arguments map[string]any) mcp.CallToolRequest {
	request := mcp.CallToolRequest{}
	request.Params.Name = name
	request.Params.Arguments = arguments
	return request
}

func decodeStructuredTask(t *testing.T, value any) app.ScheduledTask {
	t.Helper()

	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(structured task) error = %v", err)
	}

	var task app.ScheduledTask
	if err := json.Unmarshal(payload, &task); err != nil {
		t.Fatalf("json.Unmarshal(structured task) error = %v", err)
	}

	return task
}

func decodeStructuredTasks(t *testing.T, value any) []app.ScheduledTask {
	t.Helper()

	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(structured tasks) error = %v", err)
	}

	var tasks []app.ScheduledTask
	if err := json.Unmarshal(payload, &tasks); err != nil {
		t.Fatalf("json.Unmarshal(structured tasks) error = %v", err)
	}

	return tasks
}

func decodeStructuredRun(t *testing.T, value any) app.ScheduledTaskRun {
	t.Helper()

	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(structured run) error = %v", err)
	}

	var run app.ScheduledTaskRun
	if err := json.Unmarshal(payload, &run); err != nil {
		t.Fatalf("json.Unmarshal(structured run) error = %v", err)
	}

	return run
}

type fakeScheduledTaskStore struct{}

func (fakeScheduledTaskStore) ListScheduledTasks(ctx context.Context) ([]app.ScheduledTask, error) {
	return nil, nil
}

func (fakeScheduledTaskStore) ListEnabledScheduledTasks(ctx context.Context) ([]app.ScheduledTask, error) {
	return nil, nil
}

func (fakeScheduledTaskStore) GetScheduledTaskByID(ctx context.Context, scheduledTaskID string) (app.ScheduledTask, bool, error) {
	return app.ScheduledTask{}, false, nil
}

func (fakeScheduledTaskStore) GetScheduledTaskByName(ctx context.Context, name string) (app.ScheduledTask, bool, error) {
	return app.ScheduledTask{}, false, nil
}

func (fakeScheduledTaskStore) CreateScheduledTask(ctx context.Context, task app.ScheduledTask) error {
	return nil
}

func (fakeScheduledTaskStore) UpdateScheduledTask(ctx context.Context, task app.ScheduledTask) error {
	return nil
}

func (fakeScheduledTaskStore) DeleteScheduledTask(ctx context.Context, scheduledTaskID string) error {
	return nil
}

func (fakeScheduledTaskStore) GetLatestScheduledTaskRunForTask(ctx context.Context, scheduledTaskID string) (app.ScheduledTaskRun, bool, error) {
	return app.ScheduledTaskRun{}, false, nil
}

func (fakeScheduledTaskStore) AdmitScheduledTaskRun(ctx context.Context, run app.ScheduledTaskRun) (app.ScheduledTaskRun, bool, error) {
	return app.ScheduledTaskRun{}, false, nil
}

func (fakeScheduledTaskStore) UpdateScheduledTaskRun(ctx context.Context, run app.ScheduledTaskRun) error {
	return nil
}

func (fakeScheduledTaskStore) ListScheduledTaskRunsForDueTime(
	ctx context.Context,
	scheduledTaskID string,
	scheduledFor time.Time,
) ([]app.ScheduledTaskRun, error) {
	return nil, nil
}

func (fakeScheduledTaskStore) CreateScheduledTaskDelivery(ctx context.Context, delivery app.ScheduledTaskDelivery) error {
	return nil
}

func (fakeScheduledTaskStore) UpdateScheduledTaskDelivery(ctx context.Context, delivery app.ScheduledTaskDelivery) error {
	return nil
}

type fakeScheduledTaskExecutor struct{}

func (fakeScheduledTaskExecutor) ExecuteTaskNow(ctx context.Context, taskName string) (app.ScheduledTaskRun, error) {
	return app.ScheduledTaskRun{
		ScheduledRunID:  "run-debug-1",
		ScheduledTaskID: "task-1",
		Mode:            "daily",
		ScheduledFor:    time.Date(2026, time.April, 12, 8, 1, 0, 0, time.UTC),
		Attempt:         1,
		Status:          app.ScheduledTaskRunStatusSucceeded,
		ResponseText:    "done",
	}, nil
}
