package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/config"
)

const (
	queueFullMessage           = "This conversation already has five queued messages. Please retry in a moment."
	queuedInternalErrorMessage = "Something went wrong while handling your queued message. Please retry in a moment."
)

type MessageServiceDependencies struct {
	Mode             config.Mode
	CommandName      string
	Policy           ThreadPolicy
	Store            ThreadStore
	WorkspaceManager TaskWorkspaceManager
	DailyMemory      DailyMemoryRefresher
	Gateway          CodexGateway
	Coordinator      QueueCoordinator
}

type DefaultMessageService struct {
	mode        config.Mode
	commands    commandSurface
	policy      ThreadPolicy
	store       ThreadStore
	worktrees   TaskWorkspaceManager
	dailyMemory DailyMemoryRefresher
	gateway     CodexGateway
	coordinator QueueCoordinator
	queueWG     sync.WaitGroup
}

func NewMessageService(deps MessageServiceDependencies) (*DefaultMessageService, error) {
	if deps.Mode == "" {
		return nil, errors.New("mode must not be empty")
	}

	commandName := strings.TrimSpace(deps.CommandName)
	if commandName == "" {
		return nil, errors.New("command name must not be empty")
	}

	if deps.Policy == nil {
		return nil, errors.New("thread policy must not be nil")
	}

	if deps.Store == nil {
		return nil, errors.New("thread store must not be nil")
	}

	if deps.Mode == config.ModeTask && deps.WorkspaceManager == nil {
		return nil, errors.New("task workspace manager must not be nil in task mode")
	}

	if deps.Mode == config.ModeDaily && deps.DailyMemory == nil {
		return nil, errors.New("daily memory refresher must not be nil in daily mode")
	}

	if deps.Gateway == nil {
		return nil, errors.New("codex gateway must not be nil")
	}

	if deps.Coordinator == nil {
		return nil, errors.New("queue coordinator must not be nil")
	}

	return &DefaultMessageService{
		mode:        deps.Mode,
		commands:    newCommandSurface(commandName),
		policy:      deps.Policy,
		store:       deps.Store,
		worktrees:   deps.WorkspaceManager,
		dailyMemory: deps.DailyMemory,
		gateway:     deps.Gateway,
		coordinator: deps.Coordinator,
	}, nil
}

func (s *DefaultMessageService) HandleMessage(ctx context.Context, request MessageRequest, sink DeferredReplySink) (MessageResponse, error) {
	cleanup := request.Cleanup
	request.Cleanup = nil
	defer func() {
		runCleanup(cleanup)
	}()

	if !request.Mentioned {
		return MessageResponse{Ignore: true}, nil
	}

	prepared, response, handled, err := s.prepareMessage(ctx, request, sink, cleanup)
	if err != nil {
		return MessageResponse{}, err
	}

	if handled {
		return response, nil
	}

	executionKey := buildExecutionKey(s.mode, prepared.logicalKey)
	admission, err := s.coordinator.Admit(executionKey, func() {
		s.processQueuedMessage(ctx, prepared)
	})
	if err != nil {
		if errors.Is(err, ErrExecutionQueueFull) {
			return MessageResponse{
				Text:      queueFullMessage,
				ReplyToID: request.MessageID,
			}, nil
		}

		return MessageResponse{}, fmt.Errorf("admit queued execution: %w", err)
	}

	if admission.Queued {
		cleanup = nil
		return MessageResponse{
			Text:      queuedAcknowledgementMessage(admission.Position),
			ReplyToID: request.MessageID,
		}, nil
	}

	cleanup = nil
	response, execErr := s.executePreparedMessage(ctx, prepared)
	s.completeExecution(executionKey)
	return response, execErr
}

type preparedMessage struct {
	logicalKey string
	userID     string
	taskID     string
	replyToID  string
	receivedAt time.Time
	input      CodexTurnInput
	sink       DeferredReplySink
	cleanup    func()
}

func (s *DefaultMessageService) prepareMessage(
	ctx context.Context,
	request MessageRequest,
	sink DeferredReplySink,
	cleanup func(),
) (preparedMessage, MessageResponse, bool, error) {
	logicalKey, err := s.policy.ResolveMessageKey(ctx, request)
	if err != nil {
		if errors.Is(err, ErrNoActiveTask) {
			return preparedMessage{}, MessageResponse{
				Text:      s.noActiveTaskMessage(),
				ReplyToID: request.MessageID,
			}, true, nil
		}

		return preparedMessage{}, MessageResponse{}, false, fmt.Errorf("resolve logical thread key: %w", err)
	}

	prepared := preparedMessage{
		logicalKey: logicalKey,
		userID:     request.UserID,
		replyToID:  request.MessageID,
		receivedAt: request.ReceivedAt,
		input: CodexTurnInput{
			Prompt:     request.Content,
			ImagePaths: append([]string(nil), request.ImagePaths...),
		},
		sink:    sink,
		cleanup: cleanup,
	}

	if s.mode == config.ModeTask {
		taskID, err := taskIDFromLogicalKey(request.UserID, logicalKey)
		if err != nil {
			return preparedMessage{}, MessageResponse{}, false, err
		}

		prepared.taskID = taskID
	}

	return prepared, MessageResponse{}, false, nil
}

func (s *DefaultMessageService) noActiveTaskMessage() string {
	return fmt.Sprintf(
		"No active task is selected. Use %s, %s, or %s first.",
		s.commands.taskNewPlaceholder(),
		s.commands.taskList(),
		s.commands.taskSwitchPlaceholder(),
	)
}

func (s *DefaultMessageService) executePreparedMessage(ctx context.Context, prepared preparedMessage) (MessageResponse, error) {
	defer runCleanup(prepared.cleanup)

	if s.mode == config.ModeTask {
		task, response, handled, err := s.ensureTaskReady(ctx, prepared)
		if err != nil {
			return MessageResponse{}, err
		}

		if handled {
			return response, nil
		}

		prepared.input.WorkingDirectory = task.WorktreePath
	}

	if s.mode == config.ModeDaily {
		if err := s.dailyMemory.RefreshBeforeFirstDailyTurn(ctx, prepared.logicalKey, prepared.receivedAt); err != nil {
			slog.Error("refresh daily memory bridge", "logical_key", prepared.logicalKey, "error", err)
		}
	}

	binding, ok, err := s.store.GetThreadBinding(ctx, string(s.mode), prepared.logicalKey)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("load thread binding: %w", err)
	}

	threadID := ""
	if ok {
		threadID = binding.CodexThreadID
	} else {
		binding = ThreadBinding{
			Mode:             string(s.mode),
			LogicalThreadKey: prepared.logicalKey,
		}
	}

	if s.mode == config.ModeTask {
		binding.TaskID = prepared.taskID
	}

	result, err := s.gateway.RunTurn(ctx, threadID, prepared.input)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("run codex turn: %w", err)
	}

	if strings.TrimSpace(result.ThreadID) == "" {
		return MessageResponse{}, errors.New("codex gateway returned an empty thread id")
	}

	binding.CodexThreadID = result.ThreadID
	if err := s.store.UpsertThreadBinding(ctx, binding); err != nil {
		return MessageResponse{}, fmt.Errorf("persist thread binding: %w", err)
	}

	if s.mode == config.ModeTask {
		if err := s.touchTaskLastUsed(ctx, prepared.userID, prepared.taskID); err != nil {
			slog.Error("update task last used timestamp", "task_id", prepared.taskID, "error", err)
		}
	}

	return MessageResponse{
		Text:      result.ResponseText,
		ReplyToID: prepared.replyToID,
	}, nil
}

func (s *DefaultMessageService) processQueuedMessage(ctx context.Context, prepared preparedMessage) {
	response, err := s.executePreparedMessage(ctx, prepared)
	switch {
	case err == nil:
	case isLifecycleContextError(err):
		slog.Warn(
			"queued message canceled during shutdown",
			"logical_key",
			prepared.logicalKey,
			"reply_to_id",
			prepared.replyToID,
			"error",
			err,
		)
		return
	default:
		slog.Error(
			"queued message failed",
			"logical_key",
			prepared.logicalKey,
			"reply_to_id",
			prepared.replyToID,
			"error",
			err,
		)
		response = MessageResponse{
			Text:      queuedInternalErrorMessage,
			ReplyToID: prepared.replyToID,
		}
	}

	if prepared.sink != nil {
		if err := prepared.sink.Deliver(ctx, response); err != nil {
			if isLifecycleContextError(err) {
				slog.Warn(
					"queued message reply dropped during shutdown",
					"logical_key",
					prepared.logicalKey,
					"reply_to_id",
					prepared.replyToID,
					"error",
					err,
				)
				return
			}

			slog.Error(
				"deliver queued message reply",
				"logical_key",
				prepared.logicalKey,
				"reply_to_id",
				prepared.replyToID,
				"error",
				err,
			)
			return
		}
	}

	slog.Info(
		"queued message completed",
		"logical_key",
		prepared.logicalKey,
		"reply_to_id",
		prepared.replyToID,
	)
}

func (s *DefaultMessageService) completeExecution(executionKey string) {
	work, ok := s.coordinator.Complete(executionKey)
	if !ok {
		return
	}

	s.queueWG.Add(1)
	go func() {
		defer s.queueWG.Done()
		s.drainQueue(executionKey, work)
	}()
}

func (s *DefaultMessageService) drainQueue(executionKey string, work func()) {
	for {
		work()

		next, ok := s.coordinator.Complete(executionKey)
		if !ok {
			return
		}

		work = next
	}
}

func (s *DefaultMessageService) WaitForDrain(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		s.queueWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func buildExecutionKey(mode config.Mode, logicalKey string) string {
	return string(mode) + ":" + logicalKey
}

func queuedAcknowledgementMessage(position int) string {
	return fmt.Sprintf(
		"A response is already running for this conversation. Your message has been queued at position %d.",
		position,
	)
}

func runCleanup(cleanup func()) {
	if cleanup != nil {
		cleanup()
	}
}

func taskIDFromLogicalKey(userID string, logicalKey string) (string, error) {
	prefix := userID + ":"
	if userID == "" || !strings.HasPrefix(logicalKey, prefix) || len(logicalKey) <= len(prefix) {
		return "", fmt.Errorf("invalid task logical thread key %q for user %q", logicalKey, userID)
	}

	return strings.TrimPrefix(logicalKey, prefix), nil
}

func isLifecycleContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func (s *DefaultMessageService) ensureTaskReady(
	ctx context.Context,
	prepared preparedMessage,
) (Task, MessageResponse, bool, error) {
	task, ok, err := s.store.GetTask(ctx, prepared.userID, prepared.taskID)
	if err != nil {
		return Task{}, MessageResponse{}, false, fmt.Errorf("load task for execution: %w", err)
	}

	if !ok || task.Status != TaskStatusOpen {
		return Task{}, MessageResponse{
			Text:      s.noActiveTaskMessage(),
			ReplyToID: prepared.replyToID,
		}, true, nil
	}

	task, err = s.worktrees.EnsureReady(ctx, task)
	if err != nil {
		slog.Error("prepare task worktree", "task_id", prepared.taskID, "error", err)
		return Task{}, MessageResponse{
			Text:      "Task workspace setup failed. Please retry after checking the configured repository.",
			ReplyToID: prepared.replyToID,
		}, true, nil
	}

	return task, MessageResponse{}, false, nil
}

func (s *DefaultMessageService) touchTaskLastUsed(ctx context.Context, userID string, taskID string) error {
	task, ok, err := s.store.GetTask(ctx, userID, taskID)
	if err != nil {
		return fmt.Errorf("load task before last-used update: %w", err)
	}

	if !ok {
		return nil
	}

	now := time.Now().UTC()
	task.LastUsedAt = &now
	task.UpdatedAt = now
	if err := s.store.UpdateTask(ctx, task); err != nil {
		return fmt.Errorf("persist task last-used update: %w", err)
	}

	return nil
}
