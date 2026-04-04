package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Mode string

const (
	ModeDaily Mode = "daily"
	ModeTask  Mode = "task"

	optionalEnvironmentVariableCount = 3
)

type Config struct {
	Mode            Mode
	Timezone        *time.Location
	TimezoneName    string
	DiscordToken    string
	CodexWorkdir    string
	SQLitePath      string
	CodexExecutable string
	CodexBaseURL    string
	CodexAPIKey     string
	LogLevel        string
}

func LoadFromEnv() (Config, error) {
	return LoadFromLookup(os.LookupEnv)
}

func LoadFromLookup(lookup func(string) (string, bool)) (Config, error) {
	required := []string{
		"CLAW_MODE",
		"CLAW_TIMEZONE",
		"CLAW_DISCORD_TOKEN",
		"CLAW_CODEX_WORKDIR",
		"CLAW_SQLITE_PATH",
		"CLAW_CODEX_EXECUTABLE",
	}

	values := make(map[string]string, len(required)+optionalEnvironmentVariableCount)
	for _, key := range required {
		value, ok := lookup(key)
		if !ok || strings.TrimSpace(value) == "" {
			return Config{}, fmt.Errorf("missing required environment variable %s", key)
		}
		values[key] = strings.TrimSpace(value)
	}

	optionalKeys := []string{
		"CLAW_CODEX_BASE_URL",
		"CLAW_CODEX_API_KEY",
		"CLAW_LOG_LEVEL",
	}
	for _, key := range optionalKeys {
		value, ok := lookup(key)
		if !ok {
			continue
		}
		values[key] = strings.TrimSpace(value)
	}

	mode, err := parseMode(values["CLAW_MODE"])
	if err != nil {
		return Config{}, err
	}

	location, err := time.LoadLocation(values["CLAW_TIMEZONE"])
	if err != nil {
		return Config{}, fmt.Errorf("load timezone %q: %w", values["CLAW_TIMEZONE"], err)
	}

	logLevel := values["CLAW_LOG_LEVEL"]
	if logLevel == "" {
		logLevel = "info"
	}

	return Config{
		Mode:            mode,
		Timezone:        location,
		TimezoneName:    values["CLAW_TIMEZONE"],
		DiscordToken:    values["CLAW_DISCORD_TOKEN"],
		CodexWorkdir:    values["CLAW_CODEX_WORKDIR"],
		SQLitePath:      values["CLAW_SQLITE_PATH"],
		CodexExecutable: values["CLAW_CODEX_EXECUTABLE"],
		CodexBaseURL:    values["CLAW_CODEX_BASE_URL"],
		CodexAPIKey:     values["CLAW_CODEX_API_KEY"],
		LogLevel:        logLevel,
	}, nil
}

func parseMode(raw string) (Mode, error) {
	switch Mode(strings.ToLower(strings.TrimSpace(raw))) {
	case ModeDaily:
		return ModeDaily, nil
	case ModeTask:
		return ModeTask, nil
	default:
		return "", fmt.Errorf("unsupported CLAW_MODE %q", raw)
	}
}
