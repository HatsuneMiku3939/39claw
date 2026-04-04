package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFromLookup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     map[string]string
		want    Config
		wantErr string
	}{
		{
			name: "loads required and optional values",
			env: map[string]string{
				"CLAW_MODE":                         "task",
				"CLAW_TIMEZONE":                     "Asia/Tokyo",
				"CLAW_DISCORD_TOKEN":                "discord-token",
				"CLAW_DISCORD_GUILD_ID":             "guild-1",
				"CLAW_CODEX_WORKDIR":                "/workspace/project",
				"CLAW_DATADIR":                      "/tmp/39claw-data",
				"CLAW_CODEX_EXECUTABLE":             "/usr/local/bin/codex",
				"CLAW_CODEX_BASE_URL":               "https://example.test",
				"CLAW_CODEX_API_KEY":                "api-key",
				"CLAW_CODEX_MODEL":                  "gpt-test",
				"CLAW_CODEX_SANDBOX_MODE":           "danger-full-access",
				"CLAW_CODEX_ADDITIONAL_DIRECTORIES": "/workspace/shared:/workspace/cache",
				"CLAW_CODEX_SKIP_GIT_REPO_CHECK":    "true",
				"CLAW_CODEX_APPROVAL_POLICY":        "on-request",
				"CLAW_CODEX_MODEL_REASONING_EFFORT": "high",
				"CLAW_CODEX_WEB_SEARCH_MODE":        "cached",
				"CLAW_CODEX_NETWORK_ACCESS":         "false",
				"CLAW_LOG_LEVEL":                    "debug",
			},
			want: Config{
				Mode:                       ModeTask,
				TimezoneName:               "Asia/Tokyo",
				DiscordToken:               "discord-token",
				DiscordGuildID:             "guild-1",
				DataDir:                    "/tmp/39claw-data",
				SQLitePath:                 filepath.Join("/tmp/39claw-data", "39claw.sqlite"),
				CodexExecutable:            "/usr/local/bin/codex",
				CodexBaseURL:               "https://example.test",
				CodexAPIKey:                "api-key",
				CodexWorkdir:               "/workspace/project",
				CodexModel:                 "gpt-test",
				CodexSandboxMode:           "danger-full-access",
				CodexAdditionalDirectories: []string{"/workspace/shared", "/workspace/cache"},
				CodexSkipGitRepoCheck:      true,
				CodexApprovalPolicy:        "on-request",
				CodexModelReasoningEffort:  "high",
				CodexWebSearchMode:         "cached",
				CodexNetworkAccess:         boolPtr(false),
				LogLevel:                   "debug",
			},
		},
		{
			name: "defaults optional values",
			env: map[string]string{
				"CLAW_MODE":             "daily",
				"CLAW_TIMEZONE":         "UTC",
				"CLAW_DISCORD_TOKEN":    "discord-token",
				"CLAW_CODEX_WORKDIR":    "/workspace/project",
				"CLAW_DATADIR":          "/tmp/39claw-data",
				"CLAW_CODEX_EXECUTABLE": "codex",
			},
			want: Config{
				Mode:            ModeDaily,
				TimezoneName:    "UTC",
				DiscordToken:    "discord-token",
				DataDir:         "/tmp/39claw-data",
				SQLitePath:      filepath.Join("/tmp/39claw-data", "39claw.sqlite"),
				CodexExecutable: "codex",
				CodexWorkdir:    "/workspace/project",
				LogLevel:        "info",
			},
		},
		{
			name: "rejects missing required value",
			env: map[string]string{
				"CLAW_MODE": "daily",
			},
			wantErr: "missing required environment variable CLAW_TIMEZONE",
		},
		{
			name: "rejects unsupported mode",
			env: map[string]string{
				"CLAW_MODE":             "nightly",
				"CLAW_TIMEZONE":         "UTC",
				"CLAW_DISCORD_TOKEN":    "discord-token",
				"CLAW_CODEX_WORKDIR":    "/workspace/project",
				"CLAW_DATADIR":          "/tmp/39claw-data",
				"CLAW_CODEX_EXECUTABLE": "codex",
			},
			wantErr: `unsupported CLAW_MODE "nightly"`,
		},
		{
			name: "rejects invalid skip git repo check override",
			env: map[string]string{
				"CLAW_MODE":                      "task",
				"CLAW_TIMEZONE":                  "UTC",
				"CLAW_DISCORD_TOKEN":             "discord-token",
				"CLAW_CODEX_WORKDIR":             "/workspace/project",
				"CLAW_DATADIR":                   "/tmp/39claw-data",
				"CLAW_CODEX_EXECUTABLE":          "codex",
				"CLAW_CODEX_SKIP_GIT_REPO_CHECK": "not-bool",
			},
			wantErr: `parse CLAW_CODEX_SKIP_GIT_REPO_CHECK: strconv.ParseBool: parsing "not-bool": invalid syntax`,
		},
		{
			name: "rejects invalid network access override",
			env: map[string]string{
				"CLAW_MODE":                 "task",
				"CLAW_TIMEZONE":             "UTC",
				"CLAW_DISCORD_TOKEN":        "discord-token",
				"CLAW_CODEX_WORKDIR":        "/workspace/project",
				"CLAW_DATADIR":              "/tmp/39claw-data",
				"CLAW_CODEX_EXECUTABLE":     "codex",
				"CLAW_CODEX_NETWORK_ACCESS": "maybe",
			},
			wantErr: `parse CLAW_CODEX_NETWORK_ACCESS: strconv.ParseBool: parsing "maybe": invalid syntax`,
		},
		{
			name: "rejects invalid timezone",
			env: map[string]string{
				"CLAW_MODE":             "daily",
				"CLAW_TIMEZONE":         "Mars/Olympus",
				"CLAW_DISCORD_TOKEN":    "discord-token",
				"CLAW_CODEX_WORKDIR":    "/workspace/project",
				"CLAW_DATADIR":          "/tmp/39claw-data",
				"CLAW_CODEX_EXECUTABLE": "codex",
			},
			wantErr: `load timezone "Mars/Olympus": unknown time zone Mars/Olympus`,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := LoadFromLookup(func(key string) (string, bool) {
				value, ok := tt.env[key]
				return value, ok
			})
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("LoadFromLookup() error = nil, want %q", tt.wantErr)
				}

				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("LoadFromLookup() error = %q, want substring %q", err.Error(), tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("LoadFromLookup() error = %v", err)
			}

			if got.Mode != tt.want.Mode {
				t.Fatalf("Mode = %q, want %q", got.Mode, tt.want.Mode)
			}

			if got.Timezone == nil {
				t.Fatal("Timezone = nil, want non-nil")
			}

			if got.TimezoneName != tt.want.TimezoneName {
				t.Fatalf("TimezoneName = %q, want %q", got.TimezoneName, tt.want.TimezoneName)
			}

			if got.DiscordToken != tt.want.DiscordToken {
				t.Fatalf("DiscordToken = %q, want %q", got.DiscordToken, tt.want.DiscordToken)
			}

			if got.DiscordGuildID != tt.want.DiscordGuildID {
				t.Fatalf("DiscordGuildID = %q, want %q", got.DiscordGuildID, tt.want.DiscordGuildID)
			}

			if got.DataDir != tt.want.DataDir {
				t.Fatalf("DataDir = %q, want %q", got.DataDir, tt.want.DataDir)
			}

			if got.SQLitePath != tt.want.SQLitePath {
				t.Fatalf("SQLitePath = %q, want %q", got.SQLitePath, tt.want.SQLitePath)
			}

			if got.CodexExecutable != tt.want.CodexExecutable {
				t.Fatalf("CodexExecutable = %q, want %q", got.CodexExecutable, tt.want.CodexExecutable)
			}

			if got.CodexBaseURL != tt.want.CodexBaseURL {
				t.Fatalf("CodexBaseURL = %q, want %q", got.CodexBaseURL, tt.want.CodexBaseURL)
			}

			if got.CodexAPIKey != tt.want.CodexAPIKey {
				t.Fatalf("CodexAPIKey = %q, want %q", got.CodexAPIKey, tt.want.CodexAPIKey)
			}

			if got.CodexWorkdir != tt.want.CodexWorkdir {
				t.Fatalf("CodexWorkdir = %q, want %q", got.CodexWorkdir, tt.want.CodexWorkdir)
			}

			if got.CodexModel != tt.want.CodexModel {
				t.Fatalf("CodexModel = %q, want %q", got.CodexModel, tt.want.CodexModel)
			}

			if got.CodexSandboxMode != tt.want.CodexSandboxMode {
				t.Fatalf("CodexSandboxMode = %q, want %q", got.CodexSandboxMode, tt.want.CodexSandboxMode)
			}

			if strings.Join(got.CodexAdditionalDirectories, ",") != strings.Join(tt.want.CodexAdditionalDirectories, ",") {
				t.Fatalf(
					"CodexAdditionalDirectories = %v, want %v",
					got.CodexAdditionalDirectories,
					tt.want.CodexAdditionalDirectories,
				)
			}

			if got.CodexSkipGitRepoCheck != tt.want.CodexSkipGitRepoCheck {
				t.Fatalf("CodexSkipGitRepoCheck = %t, want %t", got.CodexSkipGitRepoCheck, tt.want.CodexSkipGitRepoCheck)
			}

			if got.CodexApprovalPolicy != tt.want.CodexApprovalPolicy {
				t.Fatalf("CodexApprovalPolicy = %q, want %q", got.CodexApprovalPolicy, tt.want.CodexApprovalPolicy)
			}

			if got.CodexModelReasoningEffort != tt.want.CodexModelReasoningEffort {
				t.Fatalf(
					"CodexModelReasoningEffort = %q, want %q",
					got.CodexModelReasoningEffort,
					tt.want.CodexModelReasoningEffort,
				)
			}

			if got.CodexWebSearchMode != tt.want.CodexWebSearchMode {
				t.Fatalf("CodexWebSearchMode = %q, want %q", got.CodexWebSearchMode, tt.want.CodexWebSearchMode)
			}

			switch {
			case got.CodexNetworkAccess == nil && tt.want.CodexNetworkAccess == nil:
			case got.CodexNetworkAccess == nil || tt.want.CodexNetworkAccess == nil:
				t.Fatalf("CodexNetworkAccess = %v, want %v", got.CodexNetworkAccess, tt.want.CodexNetworkAccess)
			case *got.CodexNetworkAccess != *tt.want.CodexNetworkAccess:
				t.Fatalf("CodexNetworkAccess = %t, want %t", *got.CodexNetworkAccess, *tt.want.CodexNetworkAccess)
			}

			if got.LogLevel != tt.want.LogLevel {
				t.Fatalf("LogLevel = %q, want %q", got.LogLevel, tt.want.LogLevel)
			}
		})
	}
}

func boolPtr(value bool) *bool {
	return &value
}
