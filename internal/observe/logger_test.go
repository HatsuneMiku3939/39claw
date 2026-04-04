package observe

import (
	"log/slog"
	"testing"
)

func TestParseLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    slog.Level
		wantErr string
	}{
		{name: "default info", input: "", want: slog.LevelInfo},
		{name: "debug", input: "debug", want: slog.LevelDebug},
		{name: "warn", input: "warn", want: slog.LevelWarn},
		{name: "warning alias", input: "warning", want: slog.LevelWarn},
		{name: "error", input: "error", want: slog.LevelError},
		{name: "invalid", input: "trace", wantErr: `unsupported log level "trace"`},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseLevel(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("parseLevel() error = nil, want %q", tt.wantErr)
				}

				if err.Error() != tt.wantErr {
					t.Fatalf("parseLevel() error = %q, want %q", err.Error(), tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("parseLevel() error = %v", err)
			}

			if got != tt.want {
				t.Fatalf("parseLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}
