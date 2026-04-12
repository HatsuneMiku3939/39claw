package scheduled

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	sqlitestore "github.com/HatsuneMiku3939/39claw/internal/store/sqlite"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestHTTPServerSupportsMCPGoSSEClientAndSharesStore(t *testing.T) {
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
	httpServer, err := NewHTTPServer(HTTPServerDependencies{
		Store:                  store,
		Timezone:               location,
		DefaultReportChannelID: "channel-1",
	})
	if err != nil {
		t.Fatalf("NewHTTPServer() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverURL, err := httpServer.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if closeErr := httpServer.Close(shutdownCtx); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	})

	mcpClient, err := client.NewSSEMCPClient(serverURL)
	if err != nil {
		t.Fatalf("NewSSEMCPClient() error = %v", err)
	}
	defer mcpClient.Close()

	clientCtx, clientCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer clientCancel()

	if err := mcpClient.Start(clientCtx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	initializeRequest := mcp.InitializeRequest{}
	initializeRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initializeRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "39claw-test-client",
		Version: "1.0.0",
	}

	initializeResult, err := mcpClient.Initialize(clientCtx, initializeRequest)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if initializeResult.ServerInfo.Name != mcpServerDisplayName {
		t.Fatalf("ServerInfo.Name = %q, want %q", initializeResult.ServerInfo.Name, mcpServerDisplayName)
	}

	listToolsResult, err := mcpClient.ListTools(clientCtx, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if len(listToolsResult.Tools) != 7 {
		t.Fatalf("tool count = %d, want %d", len(listToolsResult.Tools), 7)
	}

	createResult, err := mcpClient.CallTool(clientCtx, callToolRequest("scheduled_tasks_create", map[string]any{
		"name":          "nightly-report",
		"schedule_kind": "cron",
		"schedule_expr": "0 9 * * *",
		"prompt":        "Write the report.",
		"enabled":       true,
	}))
	if err != nil {
		t.Fatalf("CallTool(create) error = %v", err)
	}
	if createResult.IsError {
		t.Fatalf("CallTool(create) IsError = true, want false")
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
