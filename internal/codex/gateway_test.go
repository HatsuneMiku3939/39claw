package codex

import (
	"context"
	"testing"

	"github.com/HatsuneMiku3939/39claw/internal/app"
)

func TestGatewayRunTurn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		mode         string
		threadID     string
		prompt       string
		wantThreadID string
		wantText     string
		wantUsage    *app.TokenUsage
		wantErr      string
	}{
		{
			name:         "starts a new thread when thread id is empty",
			mode:         "single-success",
			prompt:       "hello",
			wantThreadID: "thread_test",
			wantText:     "Hi from helper",
			wantUsage: &app.TokenUsage{
				InputTokens:       42,
				CachedInputTokens: 12,
				OutputTokens:      5,
			},
		},
		{
			name:         "resumes an existing thread",
			mode:         "resume-success",
			threadID:     "thread_saved",
			prompt:       "continue",
			wantThreadID: "thread_saved",
			wantText:     "Resumed helper response",
			wantUsage: &app.TokenUsage{
				InputTokens:       10,
				CachedInputTokens: 0,
				OutputTokens:      3,
			},
		},
		{
			name:    "rejects empty prompt",
			mode:    "single-success",
			prompt:  "  ",
			wantErr: "prompt must not be empty",
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, _ := newTestClient(t, tt.mode)
			gateway := NewGateway(client, GatewayOptions{})

			result, err := gateway.RunTurn(context.Background(), tt.threadID, tt.prompt)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("RunTurn() error = nil, want %q", tt.wantErr)
				}

				if err.Error() != tt.wantErr {
					t.Fatalf("RunTurn() error = %q, want %q", err.Error(), tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("RunTurn() error = %v", err)
			}

			if result.ThreadID != tt.wantThreadID {
				t.Fatalf("ThreadID = %q, want %q", result.ThreadID, tt.wantThreadID)
			}

			if result.ResponseText != tt.wantText {
				t.Fatalf("ResponseText = %q, want %q", result.ResponseText, tt.wantText)
			}

			if tt.wantUsage == nil {
				if result.Usage != nil {
					t.Fatalf("Usage = %#v, want nil", result.Usage)
				}
				return
			}

			if result.Usage == nil {
				t.Fatal("Usage = nil, want non-nil")
			}

			if *result.Usage != *tt.wantUsage {
				t.Fatalf("Usage = %#v, want %#v", result.Usage, tt.wantUsage)
			}
		})
	}
}
