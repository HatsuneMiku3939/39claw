package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/oklog/ulid/v2"
)

const (
	noOpenTasksMessage      = "No open tasks yet. Use `/task new <name>` to create one."
	taskIDRequiredMessage   = "A task ID is required. Use `/task list` to find an open task."
	taskNameRequiredMessage = "A task name is required. Use `/task new <name>` to create one."
)

type TaskCommandService interface {
	ShowCurrentTask(ctx context.Context, userID string) (MessageResponse, error)
	ListTasks(ctx context.Context, userID string) (MessageResponse, error)
	CreateTask(ctx context.Context, userID string, taskName string) (MessageResponse, error)
	SwitchTask(ctx context.Context, userID string, taskID string) (MessageResponse, error)
	CloseTask(ctx context.Context, userID string, taskID string) (MessageResponse, error)
}

type TaskCommandServiceDependencies struct {
	Store     ThreadStore
	NewTaskID func() string
}

type DefaultTaskCommandService struct {
	store     ThreadStore
	newTaskID func() string
}

func NewTaskCommandService(deps TaskCommandServiceDependencies) (*DefaultTaskCommandService, error) {
	if deps.Store == nil {
		return nil, errors.New("thread store must not be nil")
	}

	newTaskID := deps.NewTaskID
	if newTaskID == nil {
		newTaskID = func() string {
			return ulid.Make().String()
		}
	}

	return &DefaultTaskCommandService{
		store:     deps.Store,
		newTaskID: newTaskID,
	}, nil
}

func (s *DefaultTaskCommandService) ShowCurrentTask(ctx context.Context, userID string) (MessageResponse, error) {
	activeTask, ok, err := s.store.GetActiveTask(ctx, userID)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("load active task: %w", err)
	}

	if !ok {
		return taskCommandResponse(noActiveTaskMessage), nil
	}

	task, ok, err := s.store.GetTask(ctx, userID, activeTask.TaskID)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("load active task details: %w", err)
	}

	if !ok {
		return taskCommandResponse(
			fmt.Sprintf(
				"Active task `%s` could not be loaded. Use `/task list` or `/task switch <id>` to recover.",
				activeTask.TaskID,
			),
		), nil
	}

	return taskCommandResponse(
		fmt.Sprintf(
			"Active task: %s. Use `/task list` to see open tasks or `/task close %s` when you're done.",
			renderTask(task),
			task.TaskID,
		),
	), nil
}

func (s *DefaultTaskCommandService) ListTasks(ctx context.Context, userID string) (MessageResponse, error) {
	tasks, err := s.store.ListOpenTasks(ctx, userID)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("list open tasks: %w", err)
	}

	if len(tasks) == 0 {
		return taskCommandResponse(noOpenTasksMessage), nil
	}

	activeTask, ok, err := s.store.GetActiveTask(ctx, userID)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("load active task: %w", err)
	}

	lines := make([]string, 0, len(tasks))
	lines = append(lines, "Open tasks:")
	for _, task := range tasks {
		line := "- " + renderTask(task)
		if ok && activeTask.TaskID == task.TaskID {
			line += " [active]"
		}
		lines = append(lines, line)
	}
	lines = append(lines, "Use `/task switch <id>` to change the active task.")

	return taskCommandResponse(strings.Join(lines, "\n")), nil
}

func (s *DefaultTaskCommandService) CreateTask(ctx context.Context, userID string, taskName string) (MessageResponse, error) {
	taskName = strings.TrimSpace(taskName)
	if taskName == "" {
		return taskCommandResponse(taskNameRequiredMessage), nil
	}

	task := Task{
		TaskID:        s.newTaskID(),
		DiscordUserID: userID,
		TaskName:      taskName,
		Status:        TaskStatusOpen,
	}

	if err := s.store.CreateTask(ctx, task); err != nil {
		return MessageResponse{}, fmt.Errorf("create task: %w", err)
	}

	if err := s.store.SetActiveTask(ctx, ActiveTask{
		DiscordUserID: userID,
		TaskID:        task.TaskID,
	}); err != nil {
		return MessageResponse{}, fmt.Errorf("set active task: %w", err)
	}

	return taskCommandResponse(
		fmt.Sprintf(
			"Created task %s and made it active. Your next message will continue this task.",
			renderTask(task),
		),
	), nil
}

func (s *DefaultTaskCommandService) SwitchTask(ctx context.Context, userID string, taskID string) (MessageResponse, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return taskCommandResponse(taskIDRequiredMessage), nil
	}

	task, ok, err := s.store.GetTask(ctx, userID, taskID)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("load task: %w", err)
	}

	if !ok {
		return taskCommandResponse(
			fmt.Sprintf("Task `%s` was not found. Use `/task list` to find an open task.", taskID),
		), nil
	}

	if task.Status != TaskStatusOpen {
		return taskCommandResponse(
			fmt.Sprintf(
				"Task `%s` is closed. Use `/task list` to find an open task or `/task new <name>` to create another one.",
				taskID,
			),
		), nil
	}

	if err := s.store.SetActiveTask(ctx, ActiveTask{
		DiscordUserID: userID,
		TaskID:        task.TaskID,
	}); err != nil {
		return MessageResponse{}, fmt.Errorf("set active task: %w", err)
	}

	return taskCommandResponse(
		fmt.Sprintf("Active task is now %s. Your next message will continue this task.", renderTask(task)),
	), nil
}

func (s *DefaultTaskCommandService) CloseTask(ctx context.Context, userID string, taskID string) (MessageResponse, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return taskCommandResponse(taskIDRequiredMessage), nil
	}

	task, ok, err := s.store.GetTask(ctx, userID, taskID)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("load task: %w", err)
	}

	if !ok {
		return taskCommandResponse(
			fmt.Sprintf("Task `%s` was not found. Use `/task list` to find an open task.", taskID),
		), nil
	}

	if task.Status == TaskStatusClosed {
		return taskCommandResponse(
			fmt.Sprintf(
				"Task `%s` is already closed. Use `/task list` to find an open task or `/task new <name>` to create another one.",
				taskID,
			),
		), nil
	}

	activeTask, hasActiveTask, err := s.store.GetActiveTask(ctx, userID)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("load active task before close: %w", err)
	}

	if err := s.store.CloseTask(ctx, userID, task.TaskID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return taskCommandResponse(
				fmt.Sprintf("Task `%s` was not found. Use `/task list` to find an open task.", taskID),
			), nil
		}

		return MessageResponse{}, fmt.Errorf("close task: %w", err)
	}

	if hasActiveTask && activeTask.TaskID == task.TaskID {
		return taskCommandResponse(
			fmt.Sprintf("Closed task %s. No active task is selected now.", renderTask(task)),
		), nil
	}

	nextActiveTaskLine, err := s.renderActiveTaskSuffix(ctx, userID)
	if err != nil {
		return MessageResponse{}, err
	}

	return taskCommandResponse(
		fmt.Sprintf("Closed task %s.%s", renderTask(task), nextActiveTaskLine),
	), nil
}

func (s *DefaultTaskCommandService) renderActiveTaskSuffix(ctx context.Context, userID string) (string, error) {
	activeTask, ok, err := s.store.GetActiveTask(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("load active task after close: %w", err)
	}

	if !ok {
		return " No active task is selected now.", nil
	}

	task, ok, err := s.store.GetTask(ctx, userID, activeTask.TaskID)
	if err != nil {
		return "", fmt.Errorf("load remaining active task details: %w", err)
	}

	if !ok {
		return " No active task is selected now.", nil
	}

	return " Active task remains " + renderTask(task) + ".", nil
}

func renderTask(task Task) string {
	return fmt.Sprintf("`%s` (`%s`)", task.TaskName, task.TaskID)
}

func taskCommandResponse(text string) MessageResponse {
	return MessageResponse{
		Text:      text,
		Ephemeral: true,
	}
}
