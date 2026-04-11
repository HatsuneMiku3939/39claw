package app

import "testing"

func TestParseTaskOverride(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		content    string
		hasImages  bool
		want       ParsedTaskOverride
		wantReject string
	}{
		{
			name:    "no override",
			content: "continue the active task",
			want: ParsedTaskOverride{
				Prompt: "continue the active task",
			},
		},
		{
			name:    "valid override with body",
			content: "task:release-bot-v1 fix the flaky test",
			want: ParsedTaskOverride{
				Matched:  true,
				TaskName: "release-bot-v1",
				Prompt:   "fix the flaky test",
			},
		},
		{
			name:      "valid override with attachments only",
			content:   "task:release-bot-v1",
			hasImages: true,
			want: ParsedTaskOverride{
				Matched:  true,
				TaskName: "release-bot-v1",
				Prompt:   "",
			},
		},
		{
			name:       "reject missing body without attachments",
			content:    "task:release-bot-v1",
			wantReject: "Task override messages need body text or at least one image attachment.",
		},
		{
			name:       "reject invalid task name",
			content:    "task:Release Work do it",
			wantReject: "Invalid task override. Use `task:<name>` with a slug-style task name such as `task:release-bot-v1`.",
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ParseTaskOverride(tt.content, tt.hasImages)
			if tt.wantReject != "" {
				if got.RejectMessage != tt.wantReject {
					t.Fatalf("RejectMessage = %q, want %q", got.RejectMessage, tt.wantReject)
				}
				return
			}

			if got != tt.want {
				t.Fatalf("ParseTaskOverride() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
