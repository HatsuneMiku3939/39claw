package main

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	"github.com/HatsuneMiku3939/39claw/internal/codex"
	"github.com/HatsuneMiku3939/39claw/internal/config"
	runtimediscord "github.com/HatsuneMiku3939/39claw/internal/runtime/discord"
)

func TestParseCLIArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		want    cliInvocation
		wantErr string
	}{
		{
			name: "defaults to serve",
			want: cliInvocation{command: cliCommandServe},
		},
		{
			name: "supports version command",
			args: []string{"version"},
			want: cliInvocation{command: cliCommandVersion},
		},
		{
			name:    "rejects unknown command",
			args:    []string{"dance"},
			wantErr: `unknown command "dance"`,
		},
		{
			name:    "rejects extra version arguments",
			args:    []string{"version", "now"},
			wantErr: "version command does not accept arguments",
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseCLIArgs(tt.args)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("parseCLIArgs() error = nil, want %q", tt.wantErr)
				}

				if err.Error() != tt.wantErr {
					t.Fatalf("parseCLIArgs() error = %q, want %q", err.Error(), tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("parseCLIArgs() error = %v", err)
			}

			if got.command != tt.want.command {
				t.Fatalf("parseCLIArgs().command = %q, want %q", got.command, tt.want.command)
			}
		})
	}
}

func TestRunCLIVersion(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runCLI([]string{"version"}, os.LookupEnv, &stdout, &stderr)
	if exitCode != exitCodeSuccess {
		t.Fatalf("runCLI() exitCode = %d, want %d", exitCode, exitCodeSuccess)
	}

	if stdout.String() != "dev\n" {
		t.Fatalf("runCLI() stdout = %q, want %q", stdout.String(), "dev\n")
	}

	if stderr.Len() != 0 {
		t.Fatalf("runCLI() stderr = %q, want empty", stderr.String())
	}
}

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

	originalNewCodexClient := newCodexClient
	t.Cleanup(func() {
		newCodexClient = originalNewCodexClient
	})

	tests := []struct {
		name              string
		env               map[string]string
		wantThreadOptions codex.ThreadOptions
		wantCodexEnv      map[string]string
		wantBootstrap     bool
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
			wantBootstrap: true,
		},
		{
			name: "injects configured codex home into codex process environment",
			env: map[string]string{
				"CLAW_MODE":                 "task",
				"CLAW_TIMEZONE":             "Asia/Tokyo",
				"CLAW_DISCORD_TOKEN":        "discord-token",
				"CLAW_DISCORD_COMMAND_NAME": "release",
				"CLAW_CODEX_WORKDIR":        "/workspace/project",
				"CLAW_CODEX_EXECUTABLE":     "codex",
				"CLAW_CODEX_HOME":           "/tmp/custom-codex-home",
			},
			wantThreadOptions: codex.ThreadOptions{
				WorkingDirectory: "/workspace/project",
				ApprovalPolicy:   codex.ApprovalModeNever,
				SandboxMode:      codex.SandboxModeWorkspaceWrite,
				WebSearchMode:    codex.WebSearchModeLive,
			},
			wantCodexEnv: map[string]string{
				"CODEX_HOME": "/tmp/custom-codex-home",
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
		{
			name: "rejects read-only sandbox in daily mode",
			env: map[string]string{
				"CLAW_MODE":                 "daily",
				"CLAW_TIMEZONE":             "Asia/Tokyo",
				"CLAW_DISCORD_TOKEN":        "discord-token",
				"CLAW_DISCORD_COMMAND_NAME": "daily",
				"CLAW_CODEX_WORKDIR":        "/workspace/project",
				"CLAW_DATADIR":              "/tmp/39claw-data",
				"CLAW_CODEX_EXECUTABLE":     "codex",
				"CLAW_CODEX_SANDBOX_MODE":   "read-only",
			},
			wantErr: "daily memory bridge requires CLAW_CODEX_SANDBOX_MODE to allow writes inside CLAW_CODEX_WORKDIR",
		},
		{
			name: "rejects non-git workdir in task mode during startup",
			env: map[string]string{
				"CLAW_MODE":                 "task",
				"CLAW_TIMEZONE":             "Asia/Tokyo",
				"CLAW_DISCORD_TOKEN":        "discord-token",
				"CLAW_DISCORD_COMMAND_NAME": "release",
				"CLAW_CODEX_WORKDIR":        "/workspace/not-a-repo",
				"CLAW_DATADIR":              "/tmp/39claw-data",
				"CLAW_CODEX_EXECUTABLE":     "codex",
			},
			wantErr: "task mode requires CLAW_CODEX_WORKDIR to exist",
		},
		{
			name: "rejects git workdir without origin remote in task mode during startup",
			env: map[string]string{
				"CLAW_MODE":                 "task",
				"CLAW_TIMEZONE":             "Asia/Tokyo",
				"CLAW_DISCORD_TOKEN":        "discord-token",
				"CLAW_DISCORD_COMMAND_NAME": "release",
				"CLAW_CODEX_WORKDIR":        "/workspace/repo-without-origin",
				"CLAW_DATADIR":              "/tmp/39claw-data",
				"CLAW_CODEX_EXECUTABLE":     "codex",
			},
			wantErr: "task mode requires CLAW_CODEX_WORKDIR to have an origin remote",
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			var capturedOptions codex.GatewayOptions
			var capturedClientOptions codex.Options

			newCodexClient = func(options codex.Options) *codex.Client {
				capturedClientOptions = options
				return codex.New(options)
			}

			newCodexGateway = func(client *codex.Client, options codex.GatewayOptions) app.CodexGateway {
				capturedOptions = options
				return stubCodexGateway{}
			}

			ctx := context.Background()
			cancel := func() {}
			if tt.wantErr == "" {
				var timeoutCtx context.Context
				timeoutCtx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
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

			if env["CLAW_MODE"] == "daily" {
				workdir := filepath.Join(t.TempDir(), "daily-workdir")
				if err := os.MkdirAll(workdir, 0o755); err != nil {
					t.Fatalf("MkdirAll(daily-workdir) error = %v", err)
				}
				env["CLAW_CODEX_WORKDIR"] = workdir
				tt.wantThreadOptions.WorkingDirectory = workdir
			}

			if env["CLAW_MODE"] == "task" && tt.wantErr == "" {
				workdir := createTaskModeRemoteBackedRepository(t)
				env["CLAW_CODEX_WORKDIR"] = workdir
				tt.wantThreadOptions.WorkingDirectory = workdir
			}

			if env["CLAW_MODE"] == "task" && strings.Contains(tt.wantErr, "origin remote") {
				env["CLAW_CODEX_WORKDIR"] = createTaskModeLocalRepository(t)
			}

			err := run(ctx, func(key string) (string, bool) {
				value, ok := env[key]
				return value, ok
			})
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("run() error = nil, want %q", tt.wantErr)
				}

				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("run() error = %q, want substring %q", err.Error(), tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("run() error = %v", err)
			}

			if tt.wantErr == "" {
				assertScheduledTaskMCPConfigOverride(t, capturedOptions.ThreadOptions.ConfigOverrides)
				capturedOptions.ThreadOptions.ConfigOverrides = nil
			}

			assertThreadOptionsEqual(t, capturedOptions.ThreadOptions, tt.wantThreadOptions)
			assertStringMapEqual(t, capturedClientOptions.Env, tt.wantCodexEnv)

			if tt.wantBootstrap {
				assertFileExists(t, filepath.Join(env["CLAW_CODEX_WORKDIR"], "AGENT_MEMORY", "MEMORY.md"))
				assertFileExists(t, filepath.Join(
					env["CLAW_CODEX_WORKDIR"],
					".agents",
					"skills",
					"39claw-daily-memory-refresh",
					"SKILL.md",
				))
			}
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

func TestCodexProcessEnv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config config.Config
		want   map[string]string
	}{
		{
			name:   "returns nil when codex home is unset",
			config: config.Config{},
			want:   nil,
		},
		{
			name: "maps configured claw codex home to codex home",
			config: config.Config{
				CodexHome: "/tmp/custom-codex-home",
			},
			want: map[string]string{
				"CODEX_HOME": "/tmp/custom-codex-home",
			},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := codexProcessEnv(tt.config)
			assertStringMapEqual(t, got, tt.want)
		})
	}
}

func TestStartScheduledMCPServerLogsURL(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	server, serverURL, err := startScheduledMCPServer(
		context.Background(),
		noopScheduledTaskStore{},
		config.Config{
			Mode:                     config.ModeTask,
			Timezone:                 location,
			ScheduledReportChannelID: "1234567890",
		},
		logger,
	)
	if err != nil {
		t.Fatalf("startScheduledMCPServer() error = %v", err)
	}
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := server.Close(shutdownCtx); err != nil {
			t.Fatalf("server.Close() error = %v", err)
		}
	})

	if !strings.HasPrefix(serverURL, "http://") {
		t.Fatalf("serverURL = %q, want http:// prefix", serverURL)
	}

	logOutput := logs.String()
	if !strings.Contains(logOutput, "scheduled MCP HTTP server started") {
		t.Fatalf("log output = %q, want startup message", logOutput)
	}
	if !strings.Contains(logOutput, serverURL) {
		t.Fatalf("log output = %q, want server URL %q", logOutput, serverURL)
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

func (r *stubDiscordRuntime) SendScheduledReport(ctx context.Context, channelID string, text string) (string, error) {
	return "scheduled-message", nil
}

type stubCodexGateway struct{}

func (stubCodexGateway) RunTurn(ctx context.Context, threadID string, input app.CodexTurnInput) (app.RunTurnResult, error) {
	return app.RunTurnResult{}, nil
}

type noopScheduledTaskStore struct{}

func (noopScheduledTaskStore) ListScheduledTasks(ctx context.Context) ([]app.ScheduledTask, error) {
	return nil, nil
}

func (noopScheduledTaskStore) ListEnabledScheduledTasks(ctx context.Context) ([]app.ScheduledTask, error) {
	return nil, nil
}

func (noopScheduledTaskStore) GetScheduledTaskByID(ctx context.Context, scheduledTaskID string) (app.ScheduledTask, bool, error) {
	return app.ScheduledTask{}, false, nil
}

func (noopScheduledTaskStore) GetScheduledTaskByName(ctx context.Context, name string) (app.ScheduledTask, bool, error) {
	return app.ScheduledTask{}, false, nil
}

func (noopScheduledTaskStore) CreateScheduledTask(ctx context.Context, task app.ScheduledTask) error {
	return nil
}

func (noopScheduledTaskStore) UpdateScheduledTask(ctx context.Context, task app.ScheduledTask) error {
	return nil
}

func (noopScheduledTaskStore) DeleteScheduledTask(ctx context.Context, scheduledTaskID string) error {
	return nil
}

func (noopScheduledTaskStore) GetLatestScheduledTaskRunForTask(ctx context.Context, scheduledTaskID string) (app.ScheduledTaskRun, bool, error) {
	return app.ScheduledTaskRun{}, false, nil
}

func (noopScheduledTaskStore) AdmitScheduledTaskRun(ctx context.Context, run app.ScheduledTaskRun) (app.ScheduledTaskRun, bool, error) {
	return app.ScheduledTaskRun{}, false, nil
}

func (noopScheduledTaskStore) UpdateScheduledTaskRun(ctx context.Context, run app.ScheduledTaskRun) error {
	return nil
}

func (noopScheduledTaskStore) ListScheduledTaskRunsForDueTime(
	ctx context.Context,
	scheduledTaskID string,
	scheduledFor time.Time,
) ([]app.ScheduledTaskRun, error) {
	return nil, nil
}

func (noopScheduledTaskStore) CreateScheduledTaskDelivery(ctx context.Context, delivery app.ScheduledTaskDelivery) error {
	return nil
}

func (noopScheduledTaskStore) UpdateScheduledTaskDelivery(ctx context.Context, delivery app.ScheduledTaskDelivery) error {
	return nil
}

func createTaskModeRemoteBackedRepository(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("git is required for task-mode startup tests: %v", err)
	}

	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	source := filepath.Join(root, "source")

	runGitTest(t, root, "init", "--bare", "-b", "main", remote)
	runGitTest(t, root, "clone", remote, source)
	runGitTest(t, source, "config", "user.email", "codex@example.com")
	runGitTest(t, source, "config", "user.name", "Codex")
	assertNoError(t, os.WriteFile(filepath.Join(source, "README.md"), []byte("hello\n"), 0o644))
	runGitTest(t, source, "add", "README.md")
	runGitTest(t, source, "commit", "-m", "initial commit")
	runGitTest(t, source, "push", "-u", "origin", "main")

	return source
}

func createTaskModeLocalRepository(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("git is required for task-mode startup tests: %v", err)
	}

	repo := filepath.Join(t.TempDir(), "source")
	assertNoError(t, os.MkdirAll(repo, 0o755))
	runGitTest(t, repo, "init", "-b", "main")
	runGitTest(t, repo, "config", "user.email", "codex@example.com")
	runGitTest(t, repo, "config", "user.name", "Codex")
	assertNoError(t, os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0o644))
	runGitTest(t, repo, "add", "README.md")
	runGitTest(t, repo, "commit", "-m", "initial commit")

	return repo
}

func runGitTest(t *testing.T, workdir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = workdir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v error = %v\n%s", args, err, output)
	}
}

func assertNoError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}
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

	if len(got.ConfigOverrides) != len(want.ConfigOverrides) {
		t.Fatalf("ConfigOverrides length = %d, want %d", len(got.ConfigOverrides), len(want.ConfigOverrides))
	}

	for index := range got.ConfigOverrides {
		if got.ConfigOverrides[index] != want.ConfigOverrides[index] {
			t.Fatalf(
				"ConfigOverrides[%d] = %q, want %q",
				index,
				got.ConfigOverrides[index],
				want.ConfigOverrides[index],
			)
		}
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

func assertScheduledTaskMCPConfigOverride(t *testing.T, got []string) {
	t.Helper()

	if len(got) != 1 {
		t.Fatalf("ConfigOverrides length = %d, want %d", len(got), 1)
	}

	override := got[0]
	for _, want := range []string{
		`mcp_servers.scheduled-tasks={`,
		`url = "http://127.0.0.1:`,
		`/mcp/scheduled-tasks"`,
	} {
		if !strings.Contains(override, want) {
			t.Fatalf("ConfigOverrides[0] = %q, want substring %q", override, want)
		}
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func assertStringMapEqual(t *testing.T, got map[string]string, want map[string]string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("map length = %d, want %d; got=%v want=%v", len(got), len(want), got, want)
	}

	for key, wantValue := range want {
		gotValue, ok := got[key]
		if !ok {
			t.Fatalf("map missing key %q; got=%v want=%v", key, got, want)
		}

		if gotValue != wantValue {
			t.Fatalf("map[%q] = %q, want %q", key, gotValue, wantValue)
		}
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%s) error = %v", path, err)
	}

	if info.IsDir() {
		t.Fatalf("Stat(%s) returned a directory, want file", path)
	}
}
