package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	runtimediscord "github.com/HatsuneMiku3939/39claw/internal/runtime/discord"
)

func TestRun(t *testing.T) {
	t.Parallel()

	originalNewDiscordRuntime := newDiscordRuntime
	newDiscordRuntime = func(deps runtimediscord.Dependencies) (discordRuntime, error) {
		return &stubDiscordRuntime{}, nil
	}
	t.Cleanup(func() {
		newDiscordRuntime = originalNewDiscordRuntime
	})

	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
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
				"CLAW_MODE":             "daily",
				"CLAW_TIMEZONE":         "Asia/Tokyo",
				"CLAW_DISCORD_TOKEN":    "discord-token",
				"CLAW_CODEX_WORKDIR":    "/workspace/project",
				"CLAW_SQLITE_PATH":      "",
				"CLAW_CODEX_EXECUTABLE": "codex",
				"CLAW_LOG_LEVEL":        "debug",
			},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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
				env["CLAW_SQLITE_PATH"] = filepath.Join(t.TempDir(), "39claw.db")
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
