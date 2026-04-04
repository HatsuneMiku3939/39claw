package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/HatsuneMiku3939/39claw/internal/config"
)

const (
	noActiveTaskMessage        = "No active task is selected. Use `/task new <name>`, `/task list`, or `/task switch <id>` first."
	queueFullMessage           = "This conversation already has five queued messages. Please retry in a moment."
	queuedInternalErrorMessage = "Something went wrong while handling your queued message. Please retry in a moment."
)

type MessageServiceDependencies struct {
	Mode        config.Mode
	Policy      ThreadPolicy
	Store       ThreadStore
	Gateway     CodexGateway
	Coordinator QueueCoordinator
}

type DefaultMessageService struct {
	mode        config.Mode
	policy      ThreadPolicy
	store       ThreadStore
	gateway     CodexGateway
	coordinator QueueCoordinator
}

func NewMessageService(deps MessageServiceDependencies) (*DefaultMessageService, error) {
	if deps.Mode == "" {
		return nil, errors.New("mode must not be empty")
	}

	if deps.Policy == nil {
		return nil, errors.New("thread policy must not be nil")
	}

	if deps.Store == nil {
		return nil, errors.New("thread store must not be nil")
	}

	if deps.Gateway == nil {
		return nil, errors.New("codex gateway must not be nil")
	}

	if deps.Coordinator == nil {
		return nil, errors.New("queue coordinator must not be nil")
	}

	return &DefaultMessageService{
		mode:        deps.Mode,
		policy:      deps.Policy,
		store:       deps.Store,
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
	queuedCtx := context.WithoutCancel(ctx)
	admission, err := s.coordinator.Admit(executionKey, func() {
		s.processQueuedMessage(queuedCtx, prepared)
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
	taskID     string
	replyToID  string
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
				Text:      noActiveTaskMessage,
				ReplyToID: request.MessageID,
			}, true, nil
		}

		return preparedMessage{}, MessageResponse{}, false, fmt.Errorf("resolve logical thread key: %w", err)
	}

	prepared := preparedMessage{
		logicalKey: logicalKey,
		replyToID:  request.MessageID,
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

func (s *DefaultMessageService) executePreparedMessage(ctx context.Context, prepared preparedMessage) (MessageResponse, error) {
	defer runCleanup(prepared.cleanup)

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

	return MessageResponse{
		Text:      result.ResponseText,
		ReplyToID: prepared.replyToID,
	}, nil
}

func (s *DefaultMessageService) processQueuedMessage(ctx context.Context, prepared preparedMessage) {
	response, err := s.executePreparedMessage(ctx, prepared)
	if err != nil {
		response = MessageResponse{
			Text:      queuedInternalErrorMessage,
			ReplyToID: prepared.replyToID,
		}
	}

	if prepared.sink != nil {
		if err := prepared.sink.Deliver(ctx, response); err != nil {
			return
		}
	}
}

func (s *DefaultMessageService) completeExecution(executionKey string) {
	work, ok := s.coordinator.Complete(executionKey)
	if !ok {
		return
	}

	go s.drainQueue(executionKey, work)
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
