package config

import (
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
				"CLAW_MODE":             "task",
				"CLAW_TIMEZONE":         "Asia/Tokyo",
				"CLAW_DISCORD_TOKEN":    "discord-token",
				"CLAW_CODEX_WORKDIR":    "/workspace/project",
				"CLAW_SQLITE_PATH":      "/tmp/39claw.db",
				"CLAW_CODEX_EXECUTABLE": "/usr/local/bin/codex",
				"CLAW_CODEX_BASE_URL":   "https://example.test",
				"CLAW_CODEX_API_KEY":    "api-key",
				"CLAW_LOG_LEVEL":        "debug",
			},
			want: Config{
				Mode:            ModeTask,
				TimezoneName:    "Asia/Tokyo",
				DiscordToken:    "discord-token",
				CodexWorkdir:    "/workspace/project",
				SQLitePath:      "/tmp/39claw.db",
				CodexExecutable: "/usr/local/bin/codex",
				CodexBaseURL:    "https://example.test",
				CodexAPIKey:     "api-key",
				LogLevel:        "debug",
			},
		},
		{
			name: "defaults log level to info",
			env: map[string]string{
				"CLAW_MODE":             "daily",
				"CLAW_TIMEZONE":         "UTC",
				"CLAW_DISCORD_TOKEN":    "discord-token",
				"CLAW_CODEX_WORKDIR":    "/workspace/project",
				"CLAW_SQLITE_PATH":      "/tmp/39claw.db",
				"CLAW_CODEX_EXECUTABLE": "codex",
			},
			want: Config{
				Mode:            ModeDaily,
				TimezoneName:    "UTC",
				DiscordToken:    "discord-token",
				CodexWorkdir:    "/workspace/project",
				SQLitePath:      "/tmp/39claw.db",
				CodexExecutable: "codex",
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
				"CLAW_SQLITE_PATH":      "/tmp/39claw.db",
				"CLAW_CODEX_EXECUTABLE": "codex",
			},
			wantErr: `unsupported CLAW_MODE "nightly"`,
		},
		{
			name: "rejects invalid timezone",
			env: map[string]string{
				"CLAW_MODE":             "daily",
				"CLAW_TIMEZONE":         "Mars/Olympus",
				"CLAW_DISCORD_TOKEN":    "discord-token",
				"CLAW_CODEX_WORKDIR":    "/workspace/project",
				"CLAW_SQLITE_PATH":      "/tmp/39claw.db",
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

			if got.CodexWorkdir != tt.want.CodexWorkdir {
				t.Fatalf("CodexWorkdir = %q, want %q", got.CodexWorkdir, tt.want.CodexWorkdir)
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

			if got.LogLevel != tt.want.LogLevel {
				t.Fatalf("LogLevel = %q, want %q", got.LogLevel, tt.want.LogLevel)
			}
		})
	}
}
