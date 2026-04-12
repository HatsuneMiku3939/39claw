package app

import (
	"context"
	"time"
)

type ScheduledTaskScheduleKind string

const (
	ScheduledTaskScheduleKindCron ScheduledTaskScheduleKind = "cron"
	ScheduledTaskScheduleKindAt   ScheduledTaskScheduleKind = "at"
)

type ScheduledTask struct {
	ScheduledTaskID string
	Name            string
	ScheduleKind    ScheduledTaskScheduleKind
	ScheduleExpr    string
	Prompt          string
	Enabled         bool
	ReportChannelID string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DisabledAt      *time.Time
}

type ScheduledTaskRunStatus string

const (
	ScheduledTaskRunStatusPending   ScheduledTaskRunStatus = "pending"
	ScheduledTaskRunStatusRunning   ScheduledTaskRunStatus = "running"
	ScheduledTaskRunStatusSucceeded ScheduledTaskRunStatus = "succeeded"
	ScheduledTaskRunStatusFailed    ScheduledTaskRunStatus = "failed"
	ScheduledTaskRunStatusCanceled  ScheduledTaskRunStatus = "canceled"
)

type ScheduledTaskRun struct {
	ScheduledRunID   string
	ScheduledTaskID  string
	Mode             string
	ScheduledFor     time.Time
	Attempt          int
	Status           ScheduledTaskRunStatus
	CodexThreadID    string
	WorkdirPath      string
	TempWorktreePath string
	StartedAt        *time.Time
	FinishedAt       *time.Time
	ErrorCode        string
	ErrorMessage     string
	ResponseText     string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ScheduledTaskDeliveryStatus string

const (
	ScheduledTaskDeliveryStatusPending   ScheduledTaskDeliveryStatus = "pending"
	ScheduledTaskDeliveryStatusSucceeded ScheduledTaskDeliveryStatus = "succeeded"
	ScheduledTaskDeliveryStatusFailed    ScheduledTaskDeliveryStatus = "failed"
	ScheduledTaskDeliveryStatusSkipped   ScheduledTaskDeliveryStatus = "skipped"
)

type ScheduledTaskDelivery struct {
	ScheduledDeliveryID string
	ScheduledRunID      string
	DiscordChannelID    string
	DiscordMessageID    string
	Status              ScheduledTaskDeliveryStatus
	DeliveredAt         *time.Time
	ErrorCode           string
	ErrorMessage        string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type ScheduledTaskStore interface {
	ListScheduledTasks(ctx context.Context) ([]ScheduledTask, error)
	ListEnabledScheduledTasks(ctx context.Context) ([]ScheduledTask, error)
	GetScheduledTaskByID(ctx context.Context, scheduledTaskID string) (ScheduledTask, bool, error)
	GetScheduledTaskByName(ctx context.Context, name string) (ScheduledTask, bool, error)
	CreateScheduledTask(ctx context.Context, task ScheduledTask) error
	UpdateScheduledTask(ctx context.Context, task ScheduledTask) error
	DeleteScheduledTask(ctx context.Context, scheduledTaskID string) error
	GetLatestScheduledTaskRunForTask(ctx context.Context, scheduledTaskID string) (ScheduledTaskRun, bool, error)
	AdmitScheduledTaskRun(ctx context.Context, run ScheduledTaskRun) (ScheduledTaskRun, bool, error)
	UpdateScheduledTaskRun(ctx context.Context, run ScheduledTaskRun) error
	ListScheduledTaskRunsForDueTime(ctx context.Context, scheduledTaskID string, scheduledFor time.Time) ([]ScheduledTaskRun, error)
	CreateScheduledTaskDelivery(ctx context.Context, delivery ScheduledTaskDelivery) error
	UpdateScheduledTaskDelivery(ctx context.Context, delivery ScheduledTaskDelivery) error
}

type ScheduledTaskReportSender interface {
	SendScheduledReport(ctx context.Context, channelID string, text string) (string, error)
}

type ScheduledTaskWorkspaceManager interface {
	PrepareTemporaryWorktree(ctx context.Context, runID string) (string, func(context.Context) error, error)
}
