package scheduled

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	sqlitestore "github.com/HatsuneMiku3939/39claw/internal/store/sqlite"
)

func TestMCPServerCreateAndListScheduledTasks(t *testing.T) {
	t.Parallel()

	db, err := sqlitestore.OpenDB(context.Background(), filepath.Join(t.TempDir(), "39claw.db"))
	if err != nil {
		t.Fatalf("OpenDB() error = %v", err)
	}
	defer db.Close()

	if err := sqlitestore.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}

	server := MCPServer{
		Store:                  sqlitestore.New(db),
		Timezone:               location,
		DefaultReportChannelID: "12345",
		Now: func() time.Time {
			return time.Date(2026, time.April, 12, 8, 0, 0, 0, time.UTC)
		},
	}

	createResult, err := server.handleToolsCall(context.Background(), mustJSON(t, map[string]any{
		"name": "scheduled_tasks_create",
		"arguments": map[string]any{
			"name":          "daily-report",
			"schedule_kind": "cron",
			"schedule_expr": "0 9 * * *",
			"prompt":        "Write the daily report.",
			"enabled":       true,
		},
	}))
	if err != nil {
		t.Fatalf("handleToolsCall(create) error = %v", err)
	}

	createdTask := decodeStructuredTask(t, createResult["structuredContent"])
	if createdTask.Name != "daily-report" {
		t.Fatalf("created task name = %q, want %q", createdTask.Name, "daily-report")
	}
	if !createdTask.Enabled {
		t.Fatal("created task Enabled = false, want true")
	}

	listResult, err := server.handleToolsCall(context.Background(), mustJSON(t, map[string]any{
		"name":      "scheduled_tasks_list",
		"arguments": map[string]any{},
	}))
	if err != nil {
		t.Fatalf("handleToolsCall(list) error = %v", err)
	}

	listedTasks := decodeStructuredTasks(t, listResult["structuredContent"])
	if len(listedTasks) != 1 {
		t.Fatalf("listed task count = %d, want %d", len(listedTasks), 1)
	}
	if listedTasks[0].Name != "daily-report" {
		t.Fatalf("listed task name = %q, want %q", listedTasks[0].Name, "daily-report")
	}
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()

	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	return payload
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
