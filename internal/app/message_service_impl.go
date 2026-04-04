package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/HatsuneMiku3939/39claw/internal/config"
)

const (
	busyRetryMessage    = "A response is already running for this conversation. Please retry in a moment."
	noActiveTaskMessage = "No active task is selected. Use `/task new <name>`, `/task list`, or `/task switch <id>` first."
)

type ExecutionGuard interface {
	Acquire(key string) (ReleaseFunc, error)
}

type ReleaseFunc func()

type MessageServiceDependencies struct {
	Mode    config.Mode
	Policy  ThreadPolicy
	Store   ThreadStore
	Gateway CodexGateway
	Guard   ExecutionGuard
}

type DefaultMessageService struct {
	mode    config.Mode
	policy  ThreadPolicy
	store   ThreadStore
	gateway CodexGateway
	guard   ExecutionGuard
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

	if deps.Guard == nil {
		return nil, errors.New("execution guard must not be nil")
	}

	return &DefaultMessageService{
		mode:    deps.Mode,
		policy:  deps.Policy,
		store:   deps.Store,
		gateway: deps.Gateway,
		guard:   deps.Guard,
	}, nil
}

func (s *DefaultMessageService) HandleMessage(ctx context.Context, request MessageRequest) (MessageResponse, error) {
	if !request.Mentioned {
		return MessageResponse{Ignore: true}, nil
	}

	logicalKey, err := s.policy.ResolveMessageKey(ctx, request)
	if err != nil {
		if errors.Is(err, ErrNoActiveTask) {
			return MessageResponse{
				Text:      noActiveTaskMessage,
				ReplyToID: request.MessageID,
			}, nil
		}

		return MessageResponse{}, fmt.Errorf("resolve logical thread key: %w", err)
	}

	release, err := s.guard.Acquire(buildExecutionKey(s.mode, logicalKey))
	if err != nil {
		if errors.Is(err, ErrExecutionInProgress) {
			return MessageResponse{
				Text:      busyRetryMessage,
				ReplyToID: request.MessageID,
			}, nil
		}

		return MessageResponse{}, fmt.Errorf("acquire execution guard: %w", err)
	}
	defer release()

	binding, ok, err := s.store.GetThreadBinding(ctx, string(s.mode), logicalKey)
	if err != nil {
		return MessageResponse{}, fmt.Errorf("load thread binding: %w", err)
	}

	threadID := ""
	if ok {
		threadID = binding.CodexThreadID
	} else {
		binding = ThreadBinding{
			Mode:             string(s.mode),
			LogicalThreadKey: logicalKey,
		}
	}

	if s.mode == config.ModeTask {
		activeTask, activeTaskOK, err := s.store.GetActiveTask(ctx, request.UserID)
		if err != nil {
			return MessageResponse{}, fmt.Errorf("load active task for binding: %w", err)
		}

		if !activeTaskOK {
			return MessageResponse{
				Text:      noActiveTaskMessage,
				ReplyToID: request.MessageID,
			}, nil
		}

		binding.TaskID = activeTask.TaskID
	}

	result, err := s.gateway.RunTurn(ctx, threadID, strings.TrimSpace(request.Content))
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
		ReplyToID: request.MessageID,
	}, nil
}

func buildExecutionKey(mode config.Mode, logicalKey string) string {
	return string(mode) + ":" + logicalKey
}
