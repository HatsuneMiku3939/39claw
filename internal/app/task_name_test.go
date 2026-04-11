package app

import "testing"

func TestValidateTaskName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		taskName string
		wantErr  bool
	}{
		{name: "simple slug", taskName: "release-work", wantErr: false},
		{name: "numbers allowed", taskName: "release-v2", wantErr: false},
		{name: "too short", taskName: "ab", wantErr: true},
		{name: "uppercase rejected", taskName: "Release-work", wantErr: true},
		{name: "double hyphen rejected", taskName: "release--work", wantErr: true},
		{name: "must start with letter", taskName: "1-release", wantErr: true},
		{name: "must end with alnum", taskName: "release-", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateTaskName(tt.taskName)
			if tt.wantErr && err == nil {
				t.Fatal("ValidateTaskName() error = nil, want non-nil")
			}

			if !tt.wantErr && err != nil {
				t.Fatalf("ValidateTaskName() error = %v, want nil", err)
			}
		})
	}
}
