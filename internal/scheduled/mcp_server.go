package scheduled

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/oklog/ulid/v2"
)

const (
	mcpServerDisplayName    = "39claw-scheduled-tasks"
	mcpServerDisplayVersion = "0.0.0"
)

type MCPServer struct {
	Store                  app.ScheduledTaskStore
	Executor               app.ScheduledTaskExecutor
	Timezone               *time.Location
	DefaultReportChannelID string
	Now                    func() time.Time
	mu                     sync.RWMutex
}

func (s *MCPServer) BuildServer() (*mcpserver.MCPServer, error) {
	if err := s.prepare(); err != nil {
		return nil, err
	}

	server := mcpserver.NewMCPServer(
		mcpServerDisplayName,
		mcpServerDisplayVersion,
		mcpserver.WithToolCapabilities(true),
	)

	server.AddTool(scheduledTasksListTool(), s.listTasks)
	server.AddTool(scheduledTasksGetTool(), s.getTask)
	server.AddTool(scheduledTasksCreateTool(), s.createTask)
	server.AddTool(scheduledTasksUpdateTool(), s.updateTask)
	server.AddTool(scheduledTasksEnableTool(), s.enableTask)
	server.AddTool(scheduledTasksDisableTool(), s.disableTask)
	server.AddTool(scheduledTasksDeleteTool(), s.deleteTask)
	server.AddTool(scheduledTasksExecuteNowTool(), s.executeTaskNow)

	return server, nil
}

func (s *MCPServer) SetExecutor(executor app.ScheduledTaskExecutor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Executor = executor
}

func (s *MCPServer) loadExecutor() app.ScheduledTaskExecutor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Executor
}

func (s *MCPServer) prepare() error {
	if s.Store == nil {
		return fmt.Errorf("scheduled task store must not be nil")
	}
	if s.Timezone == nil {
		return fmt.Errorf("timezone must not be nil")
	}
	if s.Now == nil {
		s.Now = time.Now().UTC
	}

	return nil
}

func (s *MCPServer) listTasks(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tasks, err := s.Store.ListScheduledTasks(ctx)
	if err != nil {
		return toolErrorResult("list scheduled tasks", err), nil
	}

	return structuredToolResult(tasks), nil
}

func (s *MCPServer) getTask(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return toolErrorResult("read required name", err), nil
	}

	task, ok, err := s.Store.GetScheduledTaskByName(ctx, strings.TrimSpace(name))
	if err != nil {
		return toolErrorResult("get scheduled task", err), nil
	}
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("scheduled task %q was not found", name)), nil
	}

	return structuredToolResult(task), nil
}

func (s *MCPServer) createTask(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Name            string `json:"name"`
		ScheduleKind    string `json:"schedule_kind"`
		ScheduleExpr    string `json:"schedule_expr"`
		Prompt          string `json:"prompt"`
		Enabled         bool   `json:"enabled"`
		ReportChannelID string `json:"report_channel_id"`
	}
	if err := bindToolArguments(request, &args); err != nil {
		return toolErrorResult("parse scheduled_tasks_create arguments", err), nil
	}

	task := app.ScheduledTask{
		ScheduledTaskID: ulid.Make().String(),
		Name:            strings.TrimSpace(args.Name),
		ScheduleKind:    app.ScheduledTaskScheduleKind(strings.TrimSpace(args.ScheduleKind)),
		ScheduleExpr:    strings.TrimSpace(args.ScheduleExpr),
		Prompt:          strings.TrimSpace(args.Prompt),
		Enabled:         args.Enabled,
		ReportChannelID: strings.TrimSpace(args.ReportChannelID),
	}
	if !task.Enabled {
		now := s.Now()
		task.DisabledAt = &now
	}

	if err := app.ValidateScheduledTaskDefinition(task, s.Timezone, s.DefaultReportChannelID); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if _, ok, err := s.Store.GetScheduledTaskByName(ctx, task.Name); err != nil {
		return toolErrorResult("check scheduled task name uniqueness", err), nil
	} else if ok {
		return mcp.NewToolResultError(fmt.Sprintf("scheduled task %q already exists", task.Name)), nil
	}
	if err := s.Store.CreateScheduledTask(ctx, task); err != nil {
		return toolErrorResult("create scheduled task", err), nil
	}

	return structuredToolResult(task), nil
}

func (s *MCPServer) updateTask(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Name            string  `json:"name"`
		ScheduleKind    *string `json:"schedule_kind"`
		ScheduleExpr    *string `json:"schedule_expr"`
		Prompt          *string `json:"prompt"`
		Enabled         *bool   `json:"enabled"`
		ReportChannelID *string `json:"report_channel_id"`
	}
	if err := bindToolArguments(request, &args); err != nil {
		return toolErrorResult("parse scheduled_tasks_update arguments", err), nil
	}

	task, ok, err := s.Store.GetScheduledTaskByName(ctx, strings.TrimSpace(args.Name))
	if err != nil {
		return toolErrorResult("load scheduled task", err), nil
	}
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("scheduled task %q was not found", args.Name)), nil
	}

	if args.ScheduleKind != nil {
		task.ScheduleKind = app.ScheduledTaskScheduleKind(strings.TrimSpace(*args.ScheduleKind))
	}
	if args.ScheduleExpr != nil {
		task.ScheduleExpr = strings.TrimSpace(*args.ScheduleExpr)
	}
	if args.Prompt != nil {
		task.Prompt = strings.TrimSpace(*args.Prompt)
	}
	if args.Enabled != nil {
		task.Enabled = *args.Enabled
		if task.Enabled {
			task.DisabledAt = nil
		} else {
			now := s.Now()
			task.DisabledAt = &now
		}
	}
	if args.ReportChannelID != nil {
		task.ReportChannelID = strings.TrimSpace(*args.ReportChannelID)
	}

	if err := app.ValidateScheduledTaskDefinition(task, s.Timezone, s.DefaultReportChannelID); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := s.Store.UpdateScheduledTask(ctx, task); err != nil {
		return toolErrorResult("update scheduled task", err), nil
	}

	return structuredToolResult(task), nil
}

func (s *MCPServer) enableTask(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return s.toggleTask(ctx, request, true)
}

func (s *MCPServer) disableTask(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return s.toggleTask(ctx, request, false)
}

func (s *MCPServer) deleteTask(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return toolErrorResult("read required name", err), nil
	}

	task, ok, err := s.Store.GetScheduledTaskByName(ctx, strings.TrimSpace(name))
	if err != nil {
		return toolErrorResult("load scheduled task", err), nil
	}
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("scheduled task %q was not found", name)), nil
	}
	if err := s.Store.DeleteScheduledTask(ctx, task.ScheduledTaskID); err != nil {
		return toolErrorResult("delete scheduled task", err), nil
	}

	return structuredToolResult(map[string]any{"deleted": task.Name}), nil
}

func (s *MCPServer) executeTaskNow(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	executor := s.loadExecutor()
	if executor == nil {
		return mcp.NewToolResultError("scheduled task execute-now endpoint is not available"), nil
	}

	name, err := request.RequireString("name")
	if err != nil {
		return toolErrorResult("read required name", err), nil
	}

	run, err := executor.ExecuteTaskNow(ctx, strings.TrimSpace(name))
	if err != nil {
		return toolErrorResult("execute scheduled task immediately", err), nil
	}

	return structuredToolResult(run), nil
}

func (s *MCPServer) toggleTask(
	ctx context.Context,
	request mcp.CallToolRequest,
	enabled bool,
) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return toolErrorResult("read required name", err), nil
	}

	task, ok, err := s.Store.GetScheduledTaskByName(ctx, strings.TrimSpace(name))
	if err != nil {
		return toolErrorResult("load scheduled task", err), nil
	}
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("scheduled task %q was not found", name)), nil
	}

	task.Enabled = enabled
	if enabled {
		task.DisabledAt = nil
	} else {
		now := s.Now()
		task.DisabledAt = &now
	}

	if err := app.ValidateScheduledTaskDefinition(task, s.Timezone, s.DefaultReportChannelID); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := s.Store.UpdateScheduledTask(ctx, task); err != nil {
		return toolErrorResult("update scheduled task enabled state", err), nil
	}

	return structuredToolResult(task), nil
}

func scheduledTasksListTool() mcp.Tool {
	return mcp.NewTool(
		"scheduled_tasks_list",
		mcp.WithDescription("List the scheduled tasks managed by 39claw."),
	)
}

func scheduledTasksGetTool() mcp.Tool {
	return mcp.NewTool(
		"scheduled_tasks_get",
		mcp.WithDescription("Get one scheduled task by name."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Scheduled task name.")),
	)
}

func scheduledTasksCreateTool() mcp.Tool {
	return mcp.NewTool(
		"scheduled_tasks_create",
		mcp.WithDescription("Create a scheduled task definition owned by 39claw."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Scheduled task name.")),
		mcp.WithString(
			"schedule_kind",
			mcp.Required(),
			mcp.Enum("cron", "at"),
			mcp.Description("Schedule kind: cron or at."),
		),
		mcp.WithString("schedule_expr", mcp.Required(), mcp.Description("Cron expression or local-time timestamp.")),
		mcp.WithString("prompt", mcp.Required(), mcp.Description("Prompt to execute on each scheduled run.")),
		mcp.WithBoolean("enabled", mcp.Required(), mcp.Description("Whether the task starts enabled.")),
		mcp.WithString("report_channel_id", mcp.Description("Optional Discord channel override for reports.")),
	)
}

func scheduledTasksUpdateTool() mcp.Tool {
	return mcp.NewTool(
		"scheduled_tasks_update",
		mcp.WithDescription("Update one scheduled task by name."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Scheduled task name.")),
		mcp.WithString("schedule_kind", mcp.Enum("cron", "at"), mcp.Description("Optional schedule kind override.")),
		mcp.WithString("schedule_expr", mcp.Description("Optional schedule expression override.")),
		mcp.WithString("prompt", mcp.Description("Optional prompt override.")),
		mcp.WithBoolean("enabled", mcp.Description("Optional enabled-state override.")),
		mcp.WithString("report_channel_id", mcp.Description("Optional Discord channel override.")),
	)
}

func scheduledTasksEnableTool() mcp.Tool {
	return mcp.NewTool(
		"scheduled_tasks_enable",
		mcp.WithDescription("Enable a scheduled task by name."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Scheduled task name.")),
	)
}

func scheduledTasksDisableTool() mcp.Tool {
	return mcp.NewTool(
		"scheduled_tasks_disable",
		mcp.WithDescription("Disable a scheduled task by name."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Scheduled task name.")),
	)
}

func scheduledTasksDeleteTool() mcp.Tool {
	return mcp.NewTool(
		"scheduled_tasks_delete",
		mcp.WithDescription("Delete a scheduled task by name."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Scheduled task name.")),
	)
}

func scheduledTasksExecuteNowTool() mcp.Tool {
	return mcp.NewTool(
		"scheduled_tasks_execute_now",
		mcp.WithDescription("Execute one scheduled task immediately for debugging by name."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Scheduled task name.")),
	)
}

func bindToolArguments(request mcp.CallToolRequest, target any) error {
	rawArgs := request.GetRawArguments()
	if rawArgs == nil {
		rawArgs = map[string]any{}
	}

	payload, err := json.Marshal(rawArgs)
	if err != nil {
		return fmt.Errorf("marshal raw tool arguments: %w", err)
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("unmarshal tool arguments: %w", err)
	}

	return nil
}

func structuredToolResult(payload any) *mcp.CallToolResult {
	jsonBytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return mcp.NewToolResultStructured(payload, fmt.Sprintf("%v", payload))
	}

	return mcp.NewToolResultStructured(payload, string(jsonBytes))
}

func toolErrorResult(message string, err error) *mcp.CallToolResult {
	return mcp.NewToolResultError(fmt.Sprintf("%s: %v", message, err))
}
