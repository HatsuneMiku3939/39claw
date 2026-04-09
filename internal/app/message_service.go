package app

import (
	"context"
)

type ThreadPolicy interface {
	ResolveMessageKey(ctx context.Context, request MessageRequest) (string, error)
}

type DeferredReplySink interface {
	Deliver(ctx context.Context, response MessageResponse) error
}

type DeferredReplySinkFunc func(ctx context.Context, response MessageResponse) error

func (f DeferredReplySinkFunc) Deliver(ctx context.Context, response MessageResponse) error {
	return f(ctx, response)
}

type DailyMemoryRefresher interface {
	RefreshBeforeFirstDailyTurn(ctx context.Context, session DailySession) error
}

type ThreadStore interface {
	GetThreadBinding(ctx context.Context, mode string, logicalThreadKey string) (ThreadBinding, bool, error)
	UpsertThreadBinding(ctx context.Context, binding ThreadBinding) error
	GetActiveDailySession(ctx context.Context, localDate string) (DailySession, bool, error)
	GetLatestDailySessionBefore(ctx context.Context, localDate string) (DailySession, bool, error)
	CreateDailySession(ctx context.Context, session DailySession) (DailySession, error)
	RotateDailySession(ctx context.Context, localDate string, activationReason string) (DailySession, error)
	CreateTask(ctx context.Context, task Task) error
	GetTask(ctx context.Context, discordUserID string, taskID string) (Task, bool, error)
	UpdateTask(ctx context.Context, task Task) error
	ListOpenTasks(ctx context.Context, discordUserID string) ([]Task, error)
	ListClosedReadyTasks(ctx context.Context) ([]Task, error)
	SetActiveTask(ctx context.Context, activeTask ActiveTask) error
	GetActiveTask(ctx context.Context, discordUserID string) (ActiveTask, bool, error)
	ClearActiveTask(ctx context.Context, discordUserID string) error
	CloseTask(ctx context.Context, discordUserID string, taskID string) error
}

type TaskWorkspaceManager interface {
	EnsureReady(ctx context.Context, task Task) (Task, error)
	PruneClosed(ctx context.Context) error
}

type CodexGateway interface {
	RunTurn(ctx context.Context, threadID string, input CodexTurnInput) (RunTurnResult, error)
}

type QueueCoordinator interface {
	Admit(key string, work func()) (QueueAdmission, error)
	Complete(key string) (func(), bool)
	Snapshot(key string) QueueSnapshot
}

type MessageService interface {
	HandleMessage(ctx context.Context, request MessageRequest, sink DeferredReplySink) (MessageResponse, error)
}

type DrainableMessageService interface {
	WaitForDrain(ctx context.Context) error
}
