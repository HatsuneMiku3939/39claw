package observe

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

const (
	LogFormatJSON = "json"
	LogFormatText = "text"
)

func NewLogger(level string, format string) (*slog.Logger, error) {
	return newLogger(level, format, os.Stderr)
}

func newLogger(level string, format string, writer io.Writer) (*slog.Logger, error) {
	parsedLevel, err := parseLevel(level)
	if err != nil {
		return nil, err
	}

	parsedFormat, err := parseFormat(format)
	if err != nil {
		return nil, err
	}

	options := &slog.HandlerOptions{
		Level: parsedLevel,
	}

	var handler slog.Handler
	switch parsedFormat {
	case LogFormatJSON:
		handler = slog.NewJSONHandler(writer, options)
	case LogFormatText:
		handler = slog.NewTextHandler(writer, options)
	default:
		return nil, fmt.Errorf("unsupported log format %q", parsedFormat)
	}

	return slog.New(handler), nil
}

func parseLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unsupported log level %q", raw)
	}
}

func parseFormat(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", LogFormatJSON:
		return LogFormatJSON, nil
	case LogFormatText:
		return LogFormatText, nil
	default:
		return "", fmt.Errorf("unsupported log format %q", raw)
	}
}
