package app

import "context"

type TaskCommandService interface {
	ShowCurrentTask(ctx context.Context, userID string) (MessageResponse, error)
	ListTasks(ctx context.Context, userID string) (MessageResponse, error)
	CreateTask(ctx context.Context, userID string, taskName string) (MessageResponse, error)
	SwitchTask(ctx context.Context, userID string, taskID string) (MessageResponse, error)
	CloseTask(ctx context.Context, userID string, taskID string) (MessageResponse, error)
}
