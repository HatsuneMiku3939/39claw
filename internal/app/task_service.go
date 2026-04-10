package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/oklog/ulid/v2"
)

type TaskCommandService interface {
	ShowCurrentTask(ctx context.Context, userID string) (MessageResponse, error)
	ListTasks(ctx context.Context, userID string) (MessageResponse, error)
	CreateTask(ctx context.Context, userID string, taskName string) (MessageResponse, error)
	SwitchTask(ctx context.Context, userID string, taskID string, taskName string) (MessageResponse, error)
	CloseTask(ctx context.Context, userID string, taskID string, taskName string) (MessageResponse, error)
}

type TaskCommandServiceDependencies struct {
	CommandName      string
	Store            ThreadStore
	WorkspaceManager TaskWorkspaceManager
	NewTaskID        func() string
}

type DefaultTaskCommandService struct {
	commands  commandSurface
	store     ThreadStore
	worktrees TaskWorkspaceManager
	newTaskID func() string
}

func NewTaskCommandService(deps TaskCommandServiceDependencies) (*DefaultTaskCommandService, error) {
	if deps.Store == nil {
		return nil, errors.New("thread store must not be nil")
	}

	commandName := strings.TrimSpace(deps.CommandName)
	if commandName == "" {
		return nil, errors.New("command name must not be empty")
	}

	newTaskID := deps.NewTaskID
	if newTaskID == nil {
		newTaskID = func() string {
			return ulid.Make().String()
		}
	}

	return &DefaultTaskCommandService{
		commands:  newCommandSurface(commandName),
		store:     deps.Store,
		worktrees: deps.WorkspaceManager,
		newTaskID: newTaskID,
	}, nil
}

func (s *DefaultTaskCommandService) ShowCurrentTask(ctx context.Context, userID string) (MessageResponse, error) {
	activeTask, ok, err := s.store.GetActiveTask(ctx, userID)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("load active task: %w", err)
	}

	if !ok {
		return taskCommandResponse(s.noActiveTaskMessage()), nil
	}

	task, ok, err := s.store.GetTask(ctx, userID, activeTask.TaskID)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("load active task details: %w", err)
	}

	if !ok {
		return taskCommandResponse(
			fmt.Sprintf(
				"Active task `%s` could not be loaded. Use %s or %s to recover.",
				activeTask.TaskID,
				s.commands.taskList(),
				s.commands.taskSwitchPlaceholder(),
			),
		), nil
	}

	renderedTask, err := s.renderTaskWithBranch(ctx, task)
	if err != nil {
		return MessageResponse{}, err
	}

	return taskCommandResponse(
		fmt.Sprintf(
			"Active task: %s. Use %s to see open tasks or %s when you're done.",
			renderedTask,
			s.commands.taskList(),
			s.commands.taskClose(task.TaskID),
		),
	), nil
}

func (s *DefaultTaskCommandService) ListTasks(ctx context.Context, userID string) (MessageResponse, error) {
	tasks, err := s.store.ListOpenTasks(ctx, userID)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("list open tasks: %w", err)
	}

	if len(tasks) == 0 {
		return taskCommandResponse(s.noOpenTasksMessage()), nil
	}

	activeTask, ok, err := s.store.GetActiveTask(ctx, userID)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("load active task: %w", err)
	}

	lines := make([]string, 0, len(tasks))
	lines = append(lines, "Open tasks:")
	for _, task := range tasks {
		renderedTask, err := s.renderTaskWithBranch(ctx, task)
		if err != nil {
			return MessageResponse{}, err
		}

		line := "- " + renderedTask
		if ok && activeTask.TaskID == task.TaskID {
			line += " [active]"
		}
		lines = append(lines, line)
	}
	lines = append(lines, fmt.Sprintf("Use %s to change the active task.", s.commands.taskSwitchPlaceholder()))

	return taskCommandResponse(strings.Join(lines, "\n")), nil
}

func (s *DefaultTaskCommandService) CreateTask(ctx context.Context, userID string, taskName string) (MessageResponse, error) {
	taskName = strings.TrimSpace(taskName)
	if taskName == "" {
		return taskCommandResponse(s.taskNameRequiredMessage()), nil
	}

	task := Task{
		TaskID:         s.newTaskID(),
		DiscordUserID:  userID,
		TaskName:       taskName,
		Status:         TaskStatusOpen,
		WorktreeStatus: TaskWorktreeStatusPending,
	}

	task.BranchName = DefaultTaskBranchName(task.TaskName, task.TaskID)

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

func (s *DefaultTaskCommandService) SwitchTask(ctx context.Context, userID string, taskID string, taskName string) (MessageResponse, error) {
	taskID = strings.TrimSpace(taskID)
	taskName = strings.TrimSpace(taskName)
	if taskID == "" && taskName == "" {
		return taskCommandResponse(s.taskSelectorRequiredMessage()), nil
	}

	task, ok, err := s.loadOpenTaskBySelector(ctx, userID, taskID, taskName, "task-switch")
	if err != nil {
		var commandErr taskCommandError
		if errors.As(err, &commandErr) {
			return taskCommandResponse(commandErr.Error()), nil
		}

		return MessageResponse{}, err
	}

	if !ok {
		if taskID != "" {
			return taskCommandResponse(
				fmt.Sprintf("Task `%s` was not found. Use %s to find an open task.", taskID, s.commands.taskList()),
			), nil
		}

		return taskCommandResponse(
			fmt.Sprintf("No open task named `%s` was found. Use %s to find an open task.", taskName, s.commands.taskList()),
		), nil
	}

	if task.Status != TaskStatusOpen {
		return taskCommandResponse(
			fmt.Sprintf(
				"Task `%s` is closed. Use %s to find an open task or %s to create another one.",
				taskID,
				s.commands.taskList(),
				s.commands.taskNewPlaceholder(),
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

func (s *DefaultTaskCommandService) CloseTask(ctx context.Context, userID string, taskID string, taskName string) (MessageResponse, error) {
	taskID = strings.TrimSpace(taskID)
	taskName = strings.TrimSpace(taskName)
	if taskID == "" && taskName == "" {
		return taskCommandResponse(s.taskSelectorRequiredMessage()), nil
	}

	task, ok, err := s.loadOpenTaskBySelector(ctx, userID, taskID, taskName, "task-close")
	if err != nil {
		var commandErr taskCommandError
		if errors.As(err, &commandErr) {
			return taskCommandResponse(commandErr.Error()), nil
		}

		return MessageResponse{}, err
	}

	if !ok {
		if taskID != "" {
			return taskCommandResponse(
				fmt.Sprintf("Task `%s` was not found. Use %s to find an open task.", taskID, s.commands.taskList()),
			), nil
		}

		return taskCommandResponse(
			fmt.Sprintf("No open task named `%s` was found. Use %s to find an open task.", taskName, s.commands.taskList()),
		), nil
	}

	if task.Status == TaskStatusClosed {
		return taskCommandResponse(
			fmt.Sprintf(
				"Task `%s` is already closed. Use %s to find an open task or %s to create another one.",
				taskID,
				s.commands.taskList(),
				s.commands.taskNewPlaceholder(),
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
				fmt.Sprintf("Task `%s` was not found. Use %s to find an open task.", taskID, s.commands.taskList()),
			), nil
		}

		return MessageResponse{}, fmt.Errorf("close task: %w", err)
	}

	if hasActiveTask && activeTask.TaskID == task.TaskID {
		s.pruneClosedTaskWorktrees(ctx)
		return taskCommandResponse(
			fmt.Sprintf("Closed task %s. No active task is selected now.", renderTask(task)),
		), nil
	}

	s.pruneClosedTaskWorktrees(ctx)

	nextActiveTaskLine, err := s.renderActiveTaskSuffix(ctx, userID)
	if err != nil {
		return MessageResponse{}, err
	}

	return taskCommandResponse(
		fmt.Sprintf("Closed task %s.%s", renderTask(task), nextActiveTaskLine),
	), nil
}

func (s *DefaultTaskCommandService) noActiveTaskMessage() string {
	return fmt.Sprintf(
		"No active task is selected. Use %s, %s, or %s first.",
		s.commands.taskNewPlaceholder(),
		s.commands.taskList(),
		s.commands.taskSwitchPlaceholder(),
	)
}

func (s *DefaultTaskCommandService) noOpenTasksMessage() string {
	return fmt.Sprintf("No open tasks yet. Use %s to create one.", s.commands.taskNewPlaceholder())
}

func (s *DefaultTaskCommandService) taskSelectorRequiredMessage() string {
	return fmt.Sprintf("A task name or task ID is required. Use %s to find an open task.", s.commands.taskList())
}

func (s *DefaultTaskCommandService) taskNameRequiredMessage() string {
	return fmt.Sprintf("A task name is required. Use %s to create one.", s.commands.taskNewPlaceholder())
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

func (s *DefaultTaskCommandService) renderTaskWithBranch(ctx context.Context, task Task) (string, error) {
	renderedTask := renderTask(task)
	if s.worktrees == nil {
		return renderedTask, nil
	}

	branchReader, ok := s.worktrees.(TaskWorkspaceBranchReader)
	if !ok {
		return renderedTask, nil
	}

	branch, branchOK, err := branchReader.CurrentBranch(ctx, task)
	if err != nil {
		return "", fmt.Errorf("load task worktree branch: %w", err)
	}
	if !branchOK {
		return renderedTask, nil
	}

	return fmt.Sprintf("%s [branch: `%s`]", renderedTask, branch), nil
}

func taskCommandResponse(text string) MessageResponse {
	return MessageResponse{
		Text:      text,
		Ephemeral: true,
	}
}

func (s *DefaultTaskCommandService) loadOpenTaskBySelector(
	ctx context.Context,
	userID string,
	taskID string,
	taskName string,
	action string,
) (Task, bool, error) {
	if taskID != "" {
		task, ok, err := s.store.GetTask(ctx, userID, taskID)
		if err != nil {
			return Task{}, false, fmt.Errorf("load task: %w", err)
		}

		return task, ok, nil
	}

	tasks, err := s.store.ListOpenTasks(ctx, userID)
	if err != nil {
		return Task{}, false, fmt.Errorf("list open tasks for switch: %w", err)
	}

	matches := make([]Task, 0, len(tasks))
	for _, task := range tasks {
		if strings.EqualFold(task.TaskName, taskName) {
			matches = append(matches, task)
		}
	}

	switch len(matches) {
	case 0:
		return Task{}, false, nil
	case 1:
		return matches[0], true, nil
	default:
		lines := []string{
			fmt.Sprintf("Multiple open tasks are named `%s`. Retry with a task ID:", taskName),
		}
		for _, task := range matches {
			lines = append(lines, "- "+renderTask(task))
		}
		lines = append(lines, fmt.Sprintf("Use %s when the task name is ambiguous.", s.commands.taskIDPlaceholder(action)))
		return Task{}, false, taskCommandError(strings.Join(lines, "\n"))
	}
}

type taskCommandError string

func (e taskCommandError) Error() string {
	return string(e)
}

func (s *DefaultTaskCommandService) pruneClosedTaskWorktrees(ctx context.Context) {
	if s.worktrees == nil {
		return
	}

	if err := s.worktrees.PruneClosed(ctx); err != nil {
		slog.Error("prune closed task worktrees", "error", err)
	}
}
