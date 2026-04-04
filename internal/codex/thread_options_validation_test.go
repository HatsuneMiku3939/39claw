package codex

import "testing"

func TestParseSandboxMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    SandboxMode
		wantErr string
	}{
		{
			name:  "read only",
			value: "read-only",
			want:  SandboxModeReadOnly,
		},
		{
			name:  "workspace write",
			value: "workspace-write",
			want:  SandboxModeWorkspaceWrite,
		},
		{
			name:  "danger full access",
			value: "danger-full-access",
			want:  SandboxModeDangerFullAccess,
		},
		{
			name:    "rejects invalid value",
			value:   "sandbox-party",
			wantErr: `invalid sandbox mode "sandbox-party"`,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseSandboxMode(tt.value)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ParseSandboxMode() error = nil, want %q", tt.wantErr)
				}

				if err.Error() != tt.wantErr {
					t.Fatalf("ParseSandboxMode() error = %q, want %q", err.Error(), tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("ParseSandboxMode() error = %v", err)
			}

			if got != tt.want {
				t.Fatalf("ParseSandboxMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseApprovalPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    ApprovalMode
		wantErr string
	}{
		{
			name:  "never",
			value: "never",
			want:  ApprovalModeNever,
		},
		{
			name:  "on request",
			value: "on-request",
			want:  ApprovalModeOnRequest,
		},
		{
			name:  "on failure",
			value: "on-failure",
			want:  ApprovalModeOnFailure,
		},
		{
			name:  "untrusted",
			value: "untrusted",
			want:  ApprovalModeUntrusted,
		},
		{
			name:    "rejects invalid value",
			value:   "sometimes",
			wantErr: `invalid approval policy "sometimes"`,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseApprovalPolicy(tt.value)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ParseApprovalPolicy() error = nil, want %q", tt.wantErr)
				}

				if err.Error() != tt.wantErr {
					t.Fatalf("ParseApprovalPolicy() error = %q, want %q", err.Error(), tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("ParseApprovalPolicy() error = %v", err)
			}

			if got != tt.want {
				t.Fatalf("ParseApprovalPolicy() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseModelReasoningEffort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    ModelReasoningEffort
		wantErr string
	}{
		{
			name:  "minimal",
			value: "minimal",
			want:  ModelReasoningEffortMinimal,
		},
		{
			name:  "low",
			value: "low",
			want:  ModelReasoningEffortLow,
		},
		{
			name:  "medium",
			value: "medium",
			want:  ModelReasoningEffortMedium,
		},
		{
			name:  "high",
			value: "high",
			want:  ModelReasoningEffortHigh,
		},
		{
			name:  "xhigh",
			value: "xhigh",
			want:  ModelReasoningEffortXHigh,
		},
		{
			name:    "rejects invalid value",
			value:   "turbo",
			wantErr: `invalid model reasoning effort "turbo"`,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseModelReasoningEffort(tt.value)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ParseModelReasoningEffort() error = nil, want %q", tt.wantErr)
				}

				if err.Error() != tt.wantErr {
					t.Fatalf("ParseModelReasoningEffort() error = %q, want %q", err.Error(), tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("ParseModelReasoningEffort() error = %v", err)
			}

			if got != tt.want {
				t.Fatalf("ParseModelReasoningEffort() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseWebSearchMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    WebSearchMode
		wantErr string
	}{
		{
			name:  "disabled",
			value: "disabled",
			want:  WebSearchModeDisabled,
		},
		{
			name:  "cached",
			value: "cached",
			want:  WebSearchModeCached,
		},
		{
			name:  "live",
			value: "live",
			want:  WebSearchModeLive,
		},
		{
			name:    "rejects invalid value",
			value:   "offline",
			wantErr: `invalid web search mode "offline"`,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseWebSearchMode(tt.value)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ParseWebSearchMode() error = nil, want %q", tt.wantErr)
				}

				if err.Error() != tt.wantErr {
					t.Fatalf("ParseWebSearchMode() error = %q, want %q", err.Error(), tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("ParseWebSearchMode() error = %v", err)
			}

			if got != tt.want {
				t.Fatalf("ParseWebSearchMode() = %q, want %q", got, tt.want)
			}
		})
	}
}
