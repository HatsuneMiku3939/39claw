package main

import (
	"context"
	"testing"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	"github.com/HatsuneMiku3939/39claw/internal/codex"
	"github.com/HatsuneMiku3939/39claw/internal/config"
	runtimediscord "github.com/HatsuneMiku3939/39claw/internal/runtime/discord"
)

func TestRun(t *testing.T) {
	originalNewDiscordRuntime := newDiscordRuntime
	newDiscordRuntime = func(deps runtimediscord.Dependencies) (discordRuntime, error) {
		return &stubDiscordRuntime{}, nil
	}
	t.Cleanup(func() {
		newDiscordRuntime = originalNewDiscordRuntime
	})

	originalNewCodexGateway := newCodexGateway
	t.Cleanup(func() {
		newCodexGateway = originalNewCodexGateway
	})

	tests := []struct {
		name              string
		env               map[string]string
		wantThreadOptions codex.ThreadOptions
		wantErr           string
	}{
		{
			name: "returns config validation error when required env is missing",
			env: map[string]string{
				"CLAW_MODE": "daily",
			},
			wantErr: "missing required environment variable CLAW_TIMEZONE",
		},
		{
			name: "boots foundation path and exits cleanly on canceled context",
			env: map[string]string{
				"CLAW_MODE":                 "daily",
				"CLAW_TIMEZONE":             "Asia/Tokyo",
				"CLAW_DISCORD_TOKEN":        "discord-token",
				"CLAW_DISCORD_COMMAND_NAME": "daily",
				"CLAW_CODEX_WORKDIR":        "/workspace/project",
				"CLAW_DATADIR":              "",
				"CLAW_CODEX_EXECUTABLE":     "codex",
				"CLAW_LOG_LEVEL":            "debug",
			},
			wantThreadOptions: codex.ThreadOptions{
				WorkingDirectory: "/workspace/project",
				ApprovalPolicy:   codex.ApprovalModeNever,
				SandboxMode:      codex.SandboxModeWorkspaceWrite,
				WebSearchMode:    codex.WebSearchModeLive,
			},
		},
		{
			name: "passes configured codex thread options to gateway",
			env: map[string]string{
				"CLAW_MODE":                         "task",
				"CLAW_TIMEZONE":                     "Asia/Tokyo",
				"CLAW_DISCORD_TOKEN":                "discord-token",
				"CLAW_DISCORD_COMMAND_NAME":         "release",
				"CLAW_CODEX_WORKDIR":                "/workspace/project",
				"CLAW_CODEX_EXECUTABLE":             "codex",
				"CLAW_CODEX_MODEL":                  "gpt-test",
				"CLAW_CODEX_SANDBOX_MODE":           "danger-full-access",
				"CLAW_CODEX_ADDITIONAL_DIRECTORIES": "/workspace/shared:/workspace/cache",
				"CLAW_CODEX_SKIP_GIT_REPO_CHECK":    "true",
				"CLAW_CODEX_APPROVAL_POLICY":        "on-request",
				"CLAW_CODEX_MODEL_REASONING_EFFORT": "high",
				"CLAW_CODEX_WEB_SEARCH_MODE":        "cached",
				"CLAW_CODEX_NETWORK_ACCESS":         "true",
			},
			wantThreadOptions: codex.ThreadOptions{
				Model:                 "gpt-test",
				SandboxMode:           codex.SandboxModeDangerFullAccess,
				WorkingDirectory:      "/workspace/project",
				AdditionalDirectories: []string{"/workspace/shared", "/workspace/cache"},
				SkipGitRepoCheck:      true,
				ModelReasoningEffort:  codex.ModelReasoningEffortHigh,
				NetworkAccessEnabled:  boolPtr(true),
				WebSearchMode:         codex.WebSearchModeCached,
				ApprovalPolicy:        codex.ApprovalModeOnRequest,
			},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			var capturedOptions codex.GatewayOptions
			newCodexGateway = func(client *codex.Client, options codex.GatewayOptions) app.CodexGateway {
				capturedOptions = options
				return stubCodexGateway{}
			}

			ctx := context.Background()
			cancel := func() {}
			if tt.wantErr == "" {
				var timeoutCtx context.Context
				timeoutCtx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
				ctx = timeoutCtx
			}
			defer cancel()

			env := make(map[string]string, len(tt.env))
			for key, value := range tt.env {
				env[key] = value
			}

			if tt.wantErr == "" {
				env["CLAW_DATADIR"] = t.TempDir()
			}

			err := run(ctx, func(key string) (string, bool) {
				value, ok := env[key]
				return value, ok
			})
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("run() error = nil, want %q", tt.wantErr)
				}

				if err.Error() != tt.wantErr {
					t.Fatalf("run() error = %q, want %q", err.Error(), tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("run() error = %v", err)
			}

			assertThreadOptionsEqual(t, capturedOptions.ThreadOptions, tt.wantThreadOptions)
		})
	}
}

func TestLoadThreadOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  config.Config
		want    codex.ThreadOptions
		wantErr string
	}{
		{
			name: "applies defaults",
			config: config.Config{
				CodexWorkdir: "/workspace/project",
			},
			want: codex.ThreadOptions{
				WorkingDirectory: "/workspace/project",
				ApprovalPolicy:   codex.ApprovalModeNever,
				SandboxMode:      codex.SandboxModeWorkspaceWrite,
				WebSearchMode:    codex.WebSearchModeLive,
			},
		},
		{
			name: "applies overrides",
			config: config.Config{
				CodexWorkdir:               "/workspace/project",
				CodexModel:                 "gpt-test",
				CodexSandboxMode:           "danger-full-access",
				CodexAdditionalDirectories: []string{"/workspace/shared"},
				CodexSkipGitRepoCheck:      true,
				CodexApprovalPolicy:        "on-request",
				CodexModelReasoningEffort:  "high",
				CodexWebSearchMode:         "cached",
				CodexNetworkAccess:         boolPtr(false),
			},
			want: codex.ThreadOptions{
				Model:                 "gpt-test",
				SandboxMode:           codex.SandboxModeDangerFullAccess,
				WorkingDirectory:      "/workspace/project",
				AdditionalDirectories: []string{"/workspace/shared"},
				SkipGitRepoCheck:      true,
				ModelReasoningEffort:  codex.ModelReasoningEffortHigh,
				NetworkAccessEnabled:  boolPtr(false),
				WebSearchMode:         codex.WebSearchModeCached,
				ApprovalPolicy:        codex.ApprovalModeOnRequest,
			},
		},
		{
			name: "rejects invalid sandbox mode",
			config: config.Config{
				CodexSandboxMode: "sandbox-party",
			},
			wantErr: `parse CLAW_CODEX_SANDBOX_MODE: invalid sandbox mode "sandbox-party"`,
		},
		{
			name: "rejects invalid approval policy",
			config: config.Config{
				CodexApprovalPolicy: "sometimes",
			},
			wantErr: `parse CLAW_CODEX_APPROVAL_POLICY: invalid approval policy "sometimes"`,
		},
		{
			name: "rejects invalid reasoning effort",
			config: config.Config{
				CodexModelReasoningEffort: "turbo",
			},
			wantErr: `parse CLAW_CODEX_MODEL_REASONING_EFFORT: invalid model reasoning effort "turbo"`,
		},
		{
			name: "rejects invalid web search mode",
			config: config.Config{
				CodexWebSearchMode: "offline",
			},
			wantErr: `parse CLAW_CODEX_WEB_SEARCH_MODE: invalid web search mode "offline"`,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := loadThreadOptions(tt.config)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("loadThreadOptions() error = nil, want %q", tt.wantErr)
				}

				if err.Error() != tt.wantErr {
					t.Fatalf("loadThreadOptions() error = %q, want %q", err.Error(), tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("loadThreadOptions() error = %v", err)
			}

			assertThreadOptionsEqual(t, got, tt.want)
		})
	}
}

type stubDiscordRuntime struct{}

func (r *stubDiscordRuntime) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (r *stubDiscordRuntime) Close() error {
	return nil
}

type stubCodexGateway struct{}

func (stubCodexGateway) RunTurn(ctx context.Context, threadID string, input app.CodexTurnInput) (app.RunTurnResult, error) {
	return app.RunTurnResult{}, nil
}

func assertThreadOptionsEqual(t *testing.T, got codex.ThreadOptions, want codex.ThreadOptions) {
	t.Helper()

	if got.Model != want.Model {
		t.Fatalf("Model = %q, want %q", got.Model, want.Model)
	}

	if got.SandboxMode != want.SandboxMode {
		t.Fatalf("SandboxMode = %q, want %q", got.SandboxMode, want.SandboxMode)
	}

	if got.WorkingDirectory != want.WorkingDirectory {
		t.Fatalf("WorkingDirectory = %q, want %q", got.WorkingDirectory, want.WorkingDirectory)
	}

	if len(got.AdditionalDirectories) != len(want.AdditionalDirectories) {
		t.Fatalf("AdditionalDirectories length = %d, want %d", len(got.AdditionalDirectories), len(want.AdditionalDirectories))
	}

	for index := range got.AdditionalDirectories {
		if got.AdditionalDirectories[index] != want.AdditionalDirectories[index] {
			t.Fatalf(
				"AdditionalDirectories[%d] = %q, want %q",
				index,
				got.AdditionalDirectories[index],
				want.AdditionalDirectories[index],
			)
		}
	}

	if got.SkipGitRepoCheck != want.SkipGitRepoCheck {
		t.Fatalf("SkipGitRepoCheck = %t, want %t", got.SkipGitRepoCheck, want.SkipGitRepoCheck)
	}

	if got.ModelReasoningEffort != want.ModelReasoningEffort {
		t.Fatalf("ModelReasoningEffort = %q, want %q", got.ModelReasoningEffort, want.ModelReasoningEffort)
	}

	switch {
	case got.NetworkAccessEnabled == nil && want.NetworkAccessEnabled == nil:
	case got.NetworkAccessEnabled == nil || want.NetworkAccessEnabled == nil:
		t.Fatalf("NetworkAccessEnabled = %v, want %v", got.NetworkAccessEnabled, want.NetworkAccessEnabled)
	case *got.NetworkAccessEnabled != *want.NetworkAccessEnabled:
		t.Fatalf("NetworkAccessEnabled = %t, want %t", *got.NetworkAccessEnabled, *want.NetworkAccessEnabled)
	}

	if got.WebSearchMode != want.WebSearchMode {
		t.Fatalf("WebSearchMode = %q, want %q", got.WebSearchMode, want.WebSearchMode)
	}

	if got.ApprovalPolicy != want.ApprovalPolicy {
		t.Fatalf("ApprovalPolicy = %q, want %q", got.ApprovalPolicy, want.ApprovalPolicy)
	}
}

func boolPtr(value bool) *bool {
	return &value
}
