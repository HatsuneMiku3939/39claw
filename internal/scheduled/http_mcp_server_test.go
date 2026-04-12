package scheduled

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sqlitestore "github.com/HatsuneMiku3939/39claw/internal/store/sqlite"
)

func TestHTTPServerServesMCPOverSSEAndSharesStore(t *testing.T) {
	t.Parallel()

	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}

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

	store := sqlitestore.New(db)
	server, err := NewHTTPServer(HTTPServerDependencies{
		Store:                  store,
		Timezone:               location,
		DefaultReportChannelID: "channel-1",
	})
	if err != nil {
		t.Fatalf("NewHTTPServer() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverURL, err := server.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if closeErr := server.Close(shutdownCtx); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	})

	response, err := http.Get(serverURL) //nolint:gosec // Test server is a loopback endpoint started by the process.
	if err != nil {
		t.Fatalf("http.Get() error = %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, want %d", serverURL, response.StatusCode, http.StatusOK)
	}

	reader := bufio.NewReader(response.Body)

	eventName, data, err := readSSEEvent(reader)
	if err != nil {
		t.Fatalf("readSSEEvent(endpoint) error = %v", err)
	}
	if eventName != "endpoint" {
		t.Fatalf("endpoint event name = %q, want %q", eventName, "endpoint")
	}
	endpointURL := data
	if !strings.Contains(endpointURL, "/mcp/scheduled-tasks/messages?sessionId=") {
		t.Fatalf("endpoint event data = %q, want message endpoint URL", endpointURL)
	}

	initializePayload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
	}
	postJSON(t, endpointURL, initializePayload)

	eventName, data, err = readSSEEvent(reader)
	if err != nil {
		t.Fatalf("readSSEEvent(initialize) error = %v", err)
	}
	if eventName != "message" {
		t.Fatalf("initialize event name = %q, want %q", eventName, "message")
	}

	var initializeResponse struct {
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(data), &initializeResponse); err != nil {
		t.Fatalf("json.Unmarshal(initialize response) error = %v", err)
	}
	if initializeResponse.Result.ProtocolVersion != "2024-11-05" {
		t.Fatalf(
			"protocolVersion = %q, want %q",
			initializeResponse.Result.ProtocolVersion,
			"2024-11-05",
		)
	}

	createPayload := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "scheduled_tasks_create",
			"arguments": map[string]any{
				"name":              "nightly-report",
				"schedule_kind":     "cron",
				"schedule_expr":     "0 9 * * *",
				"prompt":            "Write the report.",
				"enabled":           true,
				"report_channel_id": "",
			},
		},
	}
	postJSON(t, endpointURL, createPayload)

	eventName, data, err = readSSEEvent(reader)
	if err != nil {
		t.Fatalf("readSSEEvent(create) error = %v", err)
	}
	if eventName != "message" {
		t.Fatalf("create event name = %q, want %q", eventName, "message")
	}

	task, ok, err := store.GetScheduledTaskByName(context.Background(), "nightly-report")
	if err != nil {
		t.Fatalf("GetScheduledTaskByName() error = %v", err)
	}
	if !ok {
		t.Fatal("GetScheduledTaskByName() ok = false, want true")
	}
	if task.Prompt != "Write the report." {
		t.Fatalf("Prompt = %q, want %q", task.Prompt, "Write the report.")
	}
}

func postJSON(t *testing.T, url string, payload map[string]any) {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	response, err := http.Post(url, "application/json", bytes.NewReader(body)) //nolint:gosec // Test server is a loopback endpoint started by the process.
	if err != nil {
		t.Fatalf("http.Post() error = %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("POST %s status = %d, want %d", url, response.StatusCode, http.StatusAccepted)
	}
}

func readSSEEvent(reader *bufio.Reader) (string, string, error) {
	var eventName string
	var data string

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", "", err
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if eventName != "" || data != "" {
				return eventName, data, nil
			}
			continue
		}

		if strings.HasPrefix(trimmed, ":") {
			continue
		}

		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}

		switch strings.TrimSpace(key) {
		case "event":
			eventName = strings.TrimSpace(value)
		case "data":
			data = strings.TrimSpace(value)
		}
	}
}
