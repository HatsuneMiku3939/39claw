package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/HatsuneMiku3939/39claw/internal/config"
)

const (
	queueFullMessage           = "This conversation already has five queued messages. Please retry in a moment."
	queuedInternalErrorMessage = "Something went wrong while handling your queued message. Please retry in a moment."
)

type MessageServiceDependencies struct {
	Mode             config.Mode
	CommandName      string
	Logger           *slog.Logger
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
	logger      *slog.Logger
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

	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &DefaultMessageService{
		mode:        deps.Mode,
		commands:    newCommandSurface(commandName),
		logger:      logger,
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
	logger := s.messageLogger(prepared)
	queuedPrepared := prepared
	queuedPrepared.input.ProgressSink = nil
	admission, err := s.coordinator.Admit(executionKey, func() {
		s.processQueuedMessage(ctx, queuedPrepared)
	})
	if err != nil {
		if errors.Is(err, ErrExecutionQueueFull) {
			logger.Warn(
				"queue admission rejected because the waiting queue is full",
				"event",
				"queue_admission",
				"outcome",
				"queue_full",
			)
			return MessageResponse{
				Text:      queueFullMessage,
				ReplyToID: request.MessageID,
			}, nil
		}

		logger.Error(
			"queue admission failed",
			"event",
			"queue_admission",
			"outcome",
			"error",
			"error",
			err,
		)
		return MessageResponse{}, fmt.Errorf("admit queued execution: %w", err)
	}

	if admission.Queued {
		logger.Info(
			"queue admission accepted into the waiting queue",
			"event",
			"queue_admission",
			"outcome",
			"queued",
			"queue_position",
			admission.Position,
		)
		cleanup = nil
		return MessageResponse{
			Text:      queuedAcknowledgementMessage(admission.Position),
			ReplyToID: request.MessageID,
		}, nil
	}

	logger.Info(
		"queue admission will execute immediately",
		"event",
		"queue_admission",
		"outcome",
		"execute_now",
	)
	cleanup = nil
	response, execErr := s.executePreparedMessage(ctx, prepared)
	s.completeExecution(executionKey)
	return response, execErr
}

type preparedMessage struct {
	logicalKey   string
	dailySession DailySession
	userID       string
	taskID       string
	channelID    string
	replyToID    string
	receivedAt   time.Time
	input        CodexTurnInput
	sink         DeferredReplySink
	cleanup      func()
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
		channelID:  request.ChannelID,
		replyToID:  request.MessageID,
		receivedAt: request.ReceivedAt,
		input: CodexTurnInput{
			Prompt:       request.Content,
			ImagePaths:   append([]string(nil), request.ImagePaths...),
			ProgressSink: request.ProgressSink,
		},
		sink:    sink,
		cleanup: cleanup,
	}

	if s.mode == config.ModeDaily {
		session, err := ResolveActiveDailySession(ctx, s.store, logicalKey)
		if err != nil {
			return preparedMessage{}, MessageResponse{}, false, fmt.Errorf("resolve active daily session: %w", err)
		}

		prepared.logicalKey = session.LogicalThreadKey
		prepared.dailySession = session
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
		if err := s.dailyMemory.RefreshBeforeFirstDailyTurn(ctx, prepared.dailySession); err != nil {
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

	logger := s.messageLogger(prepared)
	turnStartedAt := time.Now()
	threadResumed := strings.TrimSpace(threadID) != ""
	if prepared.input.ProgressSink != nil {
		if err := prepared.input.ProgressSink.Deliver(ctx, MessageProgress{
			Text: "Thinking...",
		}); err != nil {
			ignoreProgressDeliveryError(err)
		}
	}

	logger.Info(
		"codex turn started",
		"event",
		"codex_turn_started",
		"thread_resumed",
		threadResumed,
		"prompt_char_count",
		utf8.RuneCountInString(strings.TrimSpace(prepared.input.Prompt)),
		"image_count",
		len(prepared.input.ImagePaths),
		"working_directory_set",
		strings.TrimSpace(prepared.input.WorkingDirectory) != "",
	)

	result, err := s.gateway.RunTurn(ctx, threadID, prepared.input)
	if err != nil {
		logger.Error(
			"codex turn failed",
			"event",
			"codex_turn_finished",
			"outcome",
			"error",
			"thread_resumed",
			threadResumed,
			"latency_ms",
			time.Since(turnStartedAt).Milliseconds(),
			"error",
			err,
		)
		return MessageResponse{}, fmt.Errorf("run codex turn: %w", err)
	}

	if strings.TrimSpace(result.ThreadID) == "" {
		logger.Error(
			"codex turn returned an empty thread id",
			"event",
			"codex_turn_finished",
			"outcome",
			"error",
			"thread_resumed",
			threadResumed,
			"latency_ms",
			time.Since(turnStartedAt).Milliseconds(),
		)
		return MessageResponse{}, errors.New("codex gateway returned an empty thread id")
	}

	logAttrs := []any{
		"event", "codex_turn_finished",
		"outcome", "success",
		"thread_resumed", threadResumed,
		"latency_ms", time.Since(turnStartedAt).Milliseconds(),
		"thread_id", result.ThreadID,
	}

	if result.Usage != nil {
		logAttrs = append(
			logAttrs,
			"usage_available", true,
			"input_tokens", result.Usage.InputTokens,
			"cached_input_tokens", result.Usage.CachedInputTokens,
			"output_tokens", result.Usage.OutputTokens,
		)
	} else {
		logAttrs = append(logAttrs, "usage_available", false)
	}

	logger.Info("codex turn finished", logAttrs...)

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
	logger := s.messageLogger(prepared)
	queueWaitMs := elapsedMillisecondsSince(prepared.receivedAt)
	logger.Info(
		"queued turn started",
		"event",
		"queued_turn_started",
		"queue_wait_ms",
		queueWaitMs,
	)

	response, err := s.executePreparedMessage(ctx, prepared)
	switch {
	case err == nil:
	case isLifecycleContextError(err):
		logger.Warn(
			"queued message canceled during shutdown",
			"event",
			"queued_turn_finished",
			"outcome",
			"canceled_during_shutdown",
			"queue_wait_ms",
			queueWaitMs,
			"error",
			err,
		)
		return
	default:
		logger.Error(
			"queued message failed",
			"event",
			"queued_turn_finished",
			"outcome",
			"error",
			"queue_wait_ms",
			queueWaitMs,
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
				logger.Warn(
					"queued message reply dropped during shutdown",
					"event",
					"queued_turn_finished",
					"outcome",
					"reply_dropped_during_shutdown",
					"queue_wait_ms",
					queueWaitMs,
					"error",
					err,
				)
				return
			}

			logger.Error(
				"deliver queued message reply",
				"event",
				"queued_turn_finished",
				"outcome",
				"reply_delivery_failed",
				"queue_wait_ms",
				queueWaitMs,
				"error",
				err,
			)
			return
		}
	}

	logger.Info(
		"queued message completed",
		"event",
		"queued_turn_finished",
		"outcome",
		"success",
		"queue_wait_ms",
		queueWaitMs,
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

func ignoreProgressDeliveryError(err error) {
	_ = err
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

func (s *DefaultMessageService) messageLogger(prepared preparedMessage) *slog.Logger {
	attrs := []any{
		"component", "message_service",
		"mode", string(s.mode),
		"logical_key", prepared.logicalKey,
		"channel_id", prepared.channelID,
		"reply_to_id", prepared.replyToID,
	}

	if prepared.userID != "" {
		attrs = append(attrs, "user_id", prepared.userID)
	}

	if prepared.taskID != "" {
		attrs = append(attrs, "task_id", prepared.taskID)
	}

	return s.logger.With(attrs...)
}

func elapsedMillisecondsSince(startedAt time.Time) int64 {
	if startedAt.IsZero() {
		return 0
	}

	elapsed := time.Since(startedAt)
	if elapsed < 0 {
		return 0
	}

	return elapsed.Milliseconds()
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
