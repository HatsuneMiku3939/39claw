package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HatsuneMiku3939/39bot/internal/codex"
)

func TestParseCLIConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantPrompt string
		wantImages []string
		wantStream bool
		wantResume string
		wantErr    string
	}{
		{
			name:       "uses prompt flag",
			args:       []string{"--prompt", "hello world", "--stream"},
			wantPrompt: "hello world",
			wantStream: true,
		},
		{
			name:       "uses positional prompt",
			args:       []string{"hello", "world"},
			wantPrompt: "hello world",
		},
		{
			name:       "reads prompt from piped stdin",
			stdin:      "hello from stdin\n",
			wantPrompt: "hello from stdin",
		},
		{
			name:       "collects repeated image flags",
			args:       []string{"--prompt", "describe", "--image", "one.png", "--image", "two.png"},
			wantPrompt: "describe",
			wantImages: []string{"one.png", "two.png"},
		},
		{
			name:    "rejects mixed prompt sources",
			args:    []string{"--prompt", "hello", "world"},
			wantErr: "use either --prompt or positional prompt arguments, not both",
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdin bytes.Buffer
			stdin.WriteString(tt.stdin)

			config, helpShown, err := parseCLIConfig(tt.args, stdinReader(t, tt.stdin), ioDiscard{})
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("parseCLIConfig() error = nil, want %q", tt.wantErr)
				}

				if err.Error() != tt.wantErr {
					t.Fatalf("parseCLIConfig() error = %q, want %q", err.Error(), tt.wantErr)
				}

				if helpShown {
					t.Fatal("helpShown = true, want false")
				}

				return
			}

			if err != nil {
				t.Fatalf("parseCLIConfig() error = %v", err)
			}

			if helpShown {
				t.Fatal("helpShown = true, want false")
			}

			if config.prompt != tt.wantPrompt {
				t.Fatalf("prompt = %q, want %q", config.prompt, tt.wantPrompt)
			}

			if strings.Join(config.images, ",") != strings.Join(tt.wantImages, ",") {
				t.Fatalf("images = %v, want %v", config.images, tt.wantImages)
			}

			if config.stream != tt.wantStream {
				t.Fatalf("stream = %t, want %t", config.stream, tt.wantStream)
			}

			if config.resumeThreadID != tt.wantResume {
				t.Fatalf("resume = %q, want %q", config.resumeThreadID, tt.wantResume)
			}
		})
	}
}

func TestCLIConfigThreadOptions(t *testing.T) {
	t.Parallel()

	networkAccess := "true"
	config := cliConfig{
		model:                 "gpt-test",
		sandboxMode:           "workspace-write",
		workingDirectory:      "/workspace/project",
		additionalDirectories: []string{"/workspace/shared"},
		skipGitRepoCheck:      true,
		approvalPolicy:        "on-request",
		modelReasoningEffort:  "high",
		webSearchMode:         "live",
		networkAccess:         networkAccess,
	}

	options, err := config.threadOptions()
	if err != nil {
		t.Fatalf("threadOptions() error = %v", err)
	}

	if options.Model != "gpt-test" {
		t.Fatalf("model = %q, want %q", options.Model, "gpt-test")
	}

	if options.SandboxMode != codex.SandboxModeWorkspaceWrite {
		t.Fatalf("sandbox = %q, want %q", options.SandboxMode, codex.SandboxModeWorkspaceWrite)
	}

	if options.WorkingDirectory != "/workspace/project" {
		t.Fatalf("working directory = %q", options.WorkingDirectory)
	}

	if options.ApprovalPolicy != codex.ApprovalModeOnRequest {
		t.Fatalf("approval policy = %q", options.ApprovalPolicy)
	}

	if options.ModelReasoningEffort != codex.ModelReasoningEffortHigh {
		t.Fatalf("reasoning effort = %q", options.ModelReasoningEffort)
	}

	if options.WebSearchMode != codex.WebSearchModeLive {
		t.Fatalf("web search mode = %q", options.WebSearchMode)
	}

	if options.NetworkAccessEnabled == nil || !*options.NetworkAccessEnabled {
		t.Fatalf("network access = %v, want true", options.NetworkAccessEnabled)
	}
}

func TestCLIConfigThreadOptionsRejectsInvalidEnum(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  cliConfig
		wantErr string
	}{
		{
			name:    "invalid sandbox",
			config:  cliConfig{sandboxMode: "sandbox-party"},
			wantErr: `invalid sandbox mode "sandbox-party"`,
		},
		{
			name:    "invalid approval policy",
			config:  cliConfig{approvalPolicy: "sometimes"},
			wantErr: `invalid approval policy "sometimes"`,
		},
		{
			name:    "invalid reasoning effort",
			config:  cliConfig{modelReasoningEffort: "turbo"},
			wantErr: `invalid model reasoning effort "turbo"`,
		},
		{
			name:    "invalid web search",
			config:  cliConfig{webSearchMode: "offline"},
			wantErr: `invalid web search mode "offline"`,
		},
		{
			name:    "invalid network access",
			config:  cliConfig{networkAccess: "maybe"},
			wantErr: `parse network access: strconv.ParseBool: parsing "maybe": invalid syntax`,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := tt.config.threadOptions()
			if err == nil {
				t.Fatalf("threadOptions() error = nil, want %q", tt.wantErr)
			}

			if err.Error() != tt.wantErr {
				t.Fatalf("threadOptions() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestCLIConfigInput(t *testing.T) {
	t.Parallel()

	config := cliConfig{
		prompt: "describe image",
		images: []string{"one.png", "two.png"},
	}

	input := config.input()
	if len(input.Parts) != 3 {
		t.Fatalf("part count = %d, want %d", len(input.Parts), 3)
	}

	if input.Parts[0].Type != codex.InputPartTypeText || input.Parts[0].Text != "describe image" {
		t.Fatalf("first part = %#v", input.Parts[0])
	}

	if input.Parts[1].Type != codex.InputPartTypeLocalImage || input.Parts[1].Path != "one.png" {
		t.Fatalf("second part = %#v", input.Parts[1])
	}

	if input.Parts[2].Type != codex.InputPartTypeLocalImage || input.Parts[2].Path != "two.png" {
		t.Fatalf("third part = %#v", input.Parts[2])
	}
}

func stdinReader(t *testing.T, content string) *os.File {
	t.Helper()

	path := filepath.Join(t.TempDir(), "stdin.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("os.Open() error = %v", err)
	}

	t.Cleanup(func() {
		_ = file.Close()
	})

	return file
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}
