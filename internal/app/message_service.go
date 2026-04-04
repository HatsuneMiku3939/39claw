package app

import "context"

type ThreadPolicy interface {
	ResolveMessageKey(ctx context.Context, request MessageRequest) (string, error)
}

type ThreadStore interface {
	GetThreadBinding(ctx context.Context, mode string, logicalThreadKey string) (ThreadBinding, bool, error)
	UpsertThreadBinding(ctx context.Context, binding ThreadBinding) error
	CreateTask(ctx context.Context, task Task) error
	GetTask(ctx context.Context, discordUserID string, taskID string) (Task, bool, error)
	ListOpenTasks(ctx context.Context, discordUserID string) ([]Task, error)
	SetActiveTask(ctx context.Context, activeTask ActiveTask) error
	GetActiveTask(ctx context.Context, discordUserID string) (ActiveTask, bool, error)
	ClearActiveTask(ctx context.Context, discordUserID string) error
	CloseTask(ctx context.Context, discordUserID string, taskID string) error
}

type CodexGateway interface {
	RunTurn(ctx context.Context, threadID string, input CodexTurnInput) (RunTurnResult, error)
}

type MessageService interface {
	HandleMessage(ctx context.Context, request MessageRequest) (MessageResponse, error)
}
