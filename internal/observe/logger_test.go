package observe

import (
	"bytes"
	"encoding/json"
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

func TestParseFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "default json", input: "", want: LogFormatJSON},
		{name: "json", input: "json", want: LogFormatJSON},
		{name: "text", input: "text", want: LogFormatText},
		{name: "invalid", input: "pretty", wantErr: `unsupported log format "pretty"`},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseFormat(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("parseFormat() error = nil, want %q", tt.wantErr)
				}

				if err.Error() != tt.wantErr {
					t.Fatalf("parseFormat() error = %q, want %q", err.Error(), tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("parseFormat() error = %v", err)
			}

			if got != tt.want {
				t.Fatalf("parseFormat() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewLoggerJSONFormat(t *testing.T) {
	var buffer bytes.Buffer

	logger, err := newLogger("info", "json", &buffer)
	if err != nil {
		t.Fatalf("newLogger() error = %v", err)
	}

	logger.Info("hello", "event", "test_event", "answer", 39)

	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buffer.Bytes()), &entry); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if entry["msg"] != "hello" {
		t.Fatalf("msg = %v, want %q", entry["msg"], "hello")
	}

	if entry["event"] != "test_event" {
		t.Fatalf("event = %v, want %q", entry["event"], "test_event")
	}

	if entry["answer"] != float64(39) {
		t.Fatalf("answer = %v, want %v", entry["answer"], 39)
	}
}
