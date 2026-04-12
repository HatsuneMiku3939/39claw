package scheduled

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	"github.com/oklog/ulid/v2"
)

type MCPServer struct {
	Store                  app.ScheduledTaskStore
	Timezone               *time.Location
	DefaultReportChannelID string
	Now                    func() time.Time
}

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func (s MCPServer) handleRequest(ctx context.Context, request jsonRPCRequest) jsonRPCResponse {
	response := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
	}

	switch request.Method {
	case "initialize":
		response.Result = map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]any{
				"name":    "39claw-scheduled-tasks",
				"version": "0.0.0",
			},
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
		}
	case "notifications/initialized":
		return jsonRPCResponse{}
	case "ping":
		response.Result = map[string]any{}
	case "tools/list":
		response.Result = map[string]any{
			"tools": scheduledTaskTools(),
		}
	case "tools/call":
		result, err := s.handleToolsCall(ctx, request.Params)
		if err != nil {
			response.Result = map[string]any{
				"content": []map[string]any{
					{
						"type": "text",
						"text": err.Error(),
					},
				},
				"isError": true,
			}
			return response
		}

		response.Result = result
	default:
		response.Error = &jsonRPCError{
			Code:    -32601,
			Message: fmt.Sprintf("method %q not found", request.Method),
		}
	}

	return response
}

//nolint:gocyclo // The MCP tool dispatch is intentionally explicit and table-driven enough at this scale.
func (s MCPServer) handleToolsCall(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, fmt.Errorf("parse tool call params: %w", err)
	}

	switch params.Name {
	case "scheduled_tasks_list":
		tasks, err := s.Store.ListScheduledTasks(ctx)
		if err != nil {
			return nil, fmt.Errorf("list scheduled tasks: %w", err)
		}
		return toolSuccessResult(tasks), nil
	case "scheduled_tasks_get":
		var args struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return nil, fmt.Errorf("parse scheduled_tasks_get arguments: %w", err)
		}
		task, ok, err := s.Store.GetScheduledTaskByName(ctx, strings.TrimSpace(args.Name))
		if err != nil {
			return nil, fmt.Errorf("get scheduled task: %w", err)
		}
		if !ok {
			return nil, fmt.Errorf("scheduled task %q was not found", args.Name)
		}
		return toolSuccessResult(task), nil
	case "scheduled_tasks_create":
		var args struct {
			Name            string `json:"name"`
			ScheduleKind    string `json:"schedule_kind"`
			ScheduleExpr    string `json:"schedule_expr"`
			Prompt          string `json:"prompt"`
			Enabled         bool   `json:"enabled"`
			ReportChannelID string `json:"report_channel_id"`
		}
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return nil, fmt.Errorf("parse scheduled_tasks_create arguments: %w", err)
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
			return nil, err
		}
		if _, ok, err := s.Store.GetScheduledTaskByName(ctx, task.Name); err != nil {
			return nil, fmt.Errorf("check scheduled task name uniqueness: %w", err)
		} else if ok {
			return nil, fmt.Errorf("scheduled task %q already exists", task.Name)
		}

		if err := s.Store.CreateScheduledTask(ctx, task); err != nil {
			return nil, fmt.Errorf("create scheduled task: %w", err)
		}
		return toolSuccessResult(task), nil
	case "scheduled_tasks_update":
		var args struct {
			Name            string  `json:"name"`
			ScheduleKind    *string `json:"schedule_kind"`
			ScheduleExpr    *string `json:"schedule_expr"`
			Prompt          *string `json:"prompt"`
			Enabled         *bool   `json:"enabled"`
			ReportChannelID *string `json:"report_channel_id"`
		}
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return nil, fmt.Errorf("parse scheduled_tasks_update arguments: %w", err)
		}
		task, ok, err := s.Store.GetScheduledTaskByName(ctx, strings.TrimSpace(args.Name))
		if err != nil {
			return nil, fmt.Errorf("load scheduled task: %w", err)
		}
		if !ok {
			return nil, fmt.Errorf("scheduled task %q was not found", args.Name)
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
			return nil, err
		}
		if err := s.Store.UpdateScheduledTask(ctx, task); err != nil {
			return nil, fmt.Errorf("update scheduled task: %w", err)
		}
		return toolSuccessResult(task), nil
	case "scheduled_tasks_enable":
		return s.toggleTask(ctx, params.Arguments, true)
	case "scheduled_tasks_disable":
		return s.toggleTask(ctx, params.Arguments, false)
	case "scheduled_tasks_delete":
		var args struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return nil, fmt.Errorf("parse scheduled_tasks_delete arguments: %w", err)
		}
		task, ok, err := s.Store.GetScheduledTaskByName(ctx, strings.TrimSpace(args.Name))
		if err != nil {
			return nil, fmt.Errorf("load scheduled task: %w", err)
		}
		if !ok {
			return nil, fmt.Errorf("scheduled task %q was not found", args.Name)
		}
		if err := s.Store.DeleteScheduledTask(ctx, task.ScheduledTaskID); err != nil {
			return nil, fmt.Errorf("delete scheduled task: %w", err)
		}
		return toolSuccessResult(map[string]any{"deleted": task.Name}), nil
	default:
		return nil, fmt.Errorf("unsupported tool %q", params.Name)
	}
}

func (s MCPServer) toggleTask(ctx context.Context, raw json.RawMessage, enabled bool) (map[string]any, error) {
	var args struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("parse toggle arguments: %w", err)
	}

	task, ok, err := s.Store.GetScheduledTaskByName(ctx, strings.TrimSpace(args.Name))
	if err != nil {
		return nil, fmt.Errorf("load scheduled task: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("scheduled task %q was not found", args.Name)
	}

	task.Enabled = enabled
	if enabled {
		task.DisabledAt = nil
	} else {
		now := s.Now()
		task.DisabledAt = &now
	}
	if err := app.ValidateScheduledTaskDefinition(task, s.Timezone, s.DefaultReportChannelID); err != nil {
		return nil, err
	}
	if err := s.Store.UpdateScheduledTask(ctx, task); err != nil {
		return nil, fmt.Errorf("update scheduled task enabled state: %w", err)
	}

	return toolSuccessResult(task), nil
}

func scheduledTaskTools() []mcpTool {
	return []mcpTool{
		{
			Name:        "scheduled_tasks_list",
			Description: "List the scheduled tasks managed by 39claw.",
			InputSchema: objectSchema(nil),
		},
		{
			Name:        "scheduled_tasks_get",
			Description: "Get one scheduled task by name.",
			InputSchema: objectSchema(map[string]any{"name": stringSchema()}, "name"),
		},
		{
			Name:        "scheduled_tasks_create",
			Description: "Create a scheduled task definition owned by 39claw.",
			InputSchema: objectSchema(map[string]any{
				"name":              stringSchema(),
				"schedule_kind":     enumStringSchema("cron", "at"),
				"schedule_expr":     stringSchema(),
				"prompt":            stringSchema(),
				"enabled":           map[string]any{"type": "boolean"},
				"report_channel_id": stringSchema(),
			}, "name", "schedule_kind", "schedule_expr", "prompt", "enabled"),
		},
		{
			Name:        "scheduled_tasks_update",
			Description: "Update one scheduled task by name.",
			InputSchema: objectSchema(map[string]any{
				"name":              stringSchema(),
				"schedule_kind":     enumStringSchema("cron", "at"),
				"schedule_expr":     stringSchema(),
				"prompt":            stringSchema(),
				"enabled":           map[string]any{"type": "boolean"},
				"report_channel_id": stringSchema(),
			}, "name"),
		},
		{
			Name:        "scheduled_tasks_enable",
			Description: "Enable a scheduled task by name.",
			InputSchema: objectSchema(map[string]any{"name": stringSchema()}, "name"),
		},
		{
			Name:        "scheduled_tasks_disable",
			Description: "Disable a scheduled task by name.",
			InputSchema: objectSchema(map[string]any{"name": stringSchema()}, "name"),
		},
		{
			Name:        "scheduled_tasks_delete",
			Description: "Delete a scheduled task by name.",
			InputSchema: objectSchema(map[string]any{"name": stringSchema()}, "name"),
		},
	}
}

func stringSchema() map[string]any {
	return map[string]any{"type": "string"}
}

func enumStringSchema(values ...string) map[string]any {
	return map[string]any{
		"type": "string",
		"enum": values,
	}
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	if properties == nil {
		properties = map[string]any{}
	}

	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

func toolSuccessResult(payload any) map[string]any {
	jsonBytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		jsonBytes = []byte(fmt.Sprintf("%v", payload))
	}
	return map[string]any{
		"content": []map[string]any{
			{
				"type": "text",
				"text": string(jsonBytes),
			},
		},
		"structuredContent": payload,
	}
}
