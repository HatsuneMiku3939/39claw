package app

import "time"

type MessageRequest struct {
	UserID      string
	ChannelID   string
	MessageID   string
	Content     string
	ImagePaths  []string
	Cleanup     func()
	Mentioned   bool
	CommandName string
	CommandArgs []string
	ReceivedAt  time.Time
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

type TaskStatus string

const (
	TaskStatusOpen   TaskStatus = "open"
	TaskStatusClosed TaskStatus = "closed"
)

type Task struct {
	TaskID        string
	DiscordUserID string
	TaskName      string
	Status        TaskStatus
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ClosedAt      *time.Time
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

type CodexTurnInput struct {
	Prompt     string
	ImagePaths []string
}

type QueueAdmission struct {
	ExecuteNow bool
	Queued     bool
	Position   int
}
