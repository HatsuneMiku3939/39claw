package codex

import "encoding/json"

type ApprovalMode string

const (
	ApprovalModeNever     ApprovalMode = "never"
	ApprovalModeOnRequest ApprovalMode = "on-request"
	ApprovalModeOnFailure ApprovalMode = "on-failure"
	ApprovalModeUntrusted ApprovalMode = "untrusted"
)

type SandboxMode string

const (
	SandboxModeReadOnly         SandboxMode = "read-only"
	SandboxModeWorkspaceWrite   SandboxMode = "workspace-write"
	SandboxModeDangerFullAccess SandboxMode = "danger-full-access"
)

type ModelReasoningEffort string

const (
	ModelReasoningEffortMinimal ModelReasoningEffort = "minimal"
	ModelReasoningEffortLow     ModelReasoningEffort = "low"
	ModelReasoningEffortMedium  ModelReasoningEffort = "medium"
	ModelReasoningEffortHigh    ModelReasoningEffort = "high"
	ModelReasoningEffortXHigh   ModelReasoningEffort = "xhigh"
)

type WebSearchMode string

const (
	WebSearchModeDisabled WebSearchMode = "disabled"
	WebSearchModeCached   WebSearchMode = "cached"
	WebSearchModeLive     WebSearchMode = "live"
)

type ThreadOptions struct {
	Model                 string
	SandboxMode           SandboxMode
	WorkingDirectory      string
	AdditionalDirectories []string
	ConfigOverrides       []string
	SkipGitRepoCheck      bool
	ModelReasoningEffort  ModelReasoningEffort
	NetworkAccessEnabled  *bool
	WebSearchMode         WebSearchMode
	ApprovalPolicy        ApprovalMode
}

type ThreadError struct {
	Message string `json:"message"`
}

type Usage struct {
	InputTokens       int `json:"input_tokens"`
	CachedInputTokens int `json:"cached_input_tokens"`
	OutputTokens      int `json:"output_tokens"`
}

type FileUpdateChange struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

type TodoItem struct {
	Text      string `json:"text"`
	Completed bool   `json:"completed"`
}

type Item struct {
	ID               string             `json:"id,omitempty"`
	Type             string             `json:"type"`
	Text             string             `json:"text,omitempty"`
	Command          string             `json:"command,omitempty"`
	AggregatedOutput string             `json:"aggregated_output,omitempty"`
	ExitCode         *int               `json:"exit_code,omitempty"`
	Status           string             `json:"status,omitempty"`
	Changes          []FileUpdateChange `json:"changes,omitempty"`
	Server           string             `json:"server,omitempty"`
	Tool             string             `json:"tool,omitempty"`
	Arguments        json.RawMessage    `json:"arguments,omitempty"`
	Result           json.RawMessage    `json:"result,omitempty"`
	Error            *ThreadError       `json:"error,omitempty"`
	Query            string             `json:"query,omitempty"`
	Items            []TodoItem         `json:"items,omitempty"`
	Message          string             `json:"message,omitempty"`
}

type Event struct {
	Type     string       `json:"type"`
	ThreadID string       `json:"thread_id,omitempty"`
	Usage    *Usage       `json:"usage,omitempty"`
	Error    *ThreadError `json:"error,omitempty"`
	Item     *Item        `json:"item,omitempty"`
	Message  string       `json:"message,omitempty"`
}

type Turn struct {
	Items         []Item
	FinalResponse string
	Usage         *Usage
}
