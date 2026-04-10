package app

import (
	"context"
	"time"
)

type MessageRequest struct {
	UserID       string
	ChannelID    string
	MessageID    string
	Content      string
	ImagePaths   []string
	Cleanup      func()
	ProgressSink MessageProgressSink
	Mentioned    bool
	CommandName  string
	CommandArgs  []string
	ReceivedAt   time.Time
}

type MessageResponse struct {
	Text      string
	ReplyToID string
	Ephemeral bool
	Ignore    bool
}

type ThreadBinding struct {
	Mode             string
	LogicalThreadKey string
	CodexThreadID    string
	TaskID           string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type DailySession struct {
	LocalDate                string
	Generation               int
	LogicalThreadKey         string
	PreviousLogicalThreadKey string
	ActivationReason         string
	IsActive                 bool
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

type TaskStatus string

const (
	TaskStatusOpen   TaskStatus = "open"
	TaskStatusClosed TaskStatus = "closed"
)

type TaskWorktreeStatus string

const (
	TaskWorktreeStatusPending TaskWorktreeStatus = "pending"
	TaskWorktreeStatusReady   TaskWorktreeStatus = "ready"
	TaskWorktreeStatusFailed  TaskWorktreeStatus = "failed"
	TaskWorktreeStatusPruned  TaskWorktreeStatus = "pruned"
)

type Task struct {
	TaskID            string
	DiscordUserID     string
	TaskName          string
	Status            TaskStatus
	BranchName        string
	BaseRef           string
	WorktreePath      string
	WorktreeStatus    TaskWorktreeStatus
	CreatedAt         time.Time
	UpdatedAt         time.Time
	ClosedAt          *time.Time
	WorktreeCreatedAt *time.Time
	WorktreePrunedAt  *time.Time
	LastUsedAt        *time.Time
}

type ActiveTask struct {
	DiscordUserID string
	TaskID        string
	UpdatedAt     time.Time
}

type TokenUsage struct {
	InputTokens       int
	CachedInputTokens int
	OutputTokens      int
}

type RunTurnResult struct {
	ThreadID     string
	ResponseText string
	Usage        *TokenUsage
}

type MessageProgress struct {
	Text string
}

type MessageProgressSink interface {
	Deliver(ctx context.Context, progress MessageProgress) error
}

type MessageProgressSinkFunc func(ctx context.Context, progress MessageProgress) error

func (f MessageProgressSinkFunc) Deliver(ctx context.Context, progress MessageProgress) error {
	return f(ctx, progress)
}

type CodexTurnInput struct {
	Prompt           string
	ImagePaths       []string
	WorkingDirectory string
	ProgressSink     MessageProgressSink
}

type QueueAdmission struct {
	ExecuteNow bool
	Queued     bool
	Position   int
}

type QueueSnapshot struct {
	InFlight bool
	Queued   int
}
