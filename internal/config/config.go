package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Mode string

const (
	ModeDaily Mode = "daily"
	ModeTask  Mode = "task"
)

type Config struct {
	Mode                       Mode
	Timezone                   *time.Location
	TimezoneName               string
	DiscordToken               string
	DiscordGuildID             string
	DiscordCommandName         string
	DataDir                    string
	SQLitePath                 string
	CodexExecutable            string
	CodexBaseURL               string
	CodexAPIKey                string
	CodexWorkdir               string
	CodexModel                 string
	CodexSandboxMode           string
	CodexAdditionalDirectories []string
	CodexSkipGitRepoCheck      bool
	CodexApprovalPolicy        string
	CodexModelReasoningEffort  string
	CodexWebSearchMode         string
	CodexNetworkAccess         *bool
	LogLevel                   string
	LogFormat                  string
}

var discordCommandNamePattern = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)

func LoadFromEnv() (Config, error) {
	return LoadFromLookup(os.LookupEnv)
}

func LoadFromLookup(lookup func(string) (string, bool)) (Config, error) {
	required := []string{
		"CLAW_MODE",
		"CLAW_TIMEZONE",
		"CLAW_DISCORD_TOKEN",
		"CLAW_DISCORD_COMMAND_NAME",
		"CLAW_CODEX_WORKDIR",
		"CLAW_DATADIR",
		"CLAW_CODEX_EXECUTABLE",
	}

	optionalKeys := []string{
		"CLAW_CODEX_BASE_URL",
		"CLAW_CODEX_API_KEY",
		"CLAW_DISCORD_GUILD_ID",
		"CLAW_LOG_LEVEL",
		"CLAW_LOG_FORMAT",
		"CLAW_CODEX_MODEL",
		"CLAW_CODEX_SANDBOX_MODE",
		"CLAW_CODEX_ADDITIONAL_DIRECTORIES",
		"CLAW_CODEX_SKIP_GIT_REPO_CHECK",
		"CLAW_CODEX_APPROVAL_POLICY",
		"CLAW_CODEX_MODEL_REASONING_EFFORT",
		"CLAW_CODEX_WEB_SEARCH_MODE",
		"CLAW_CODEX_NETWORK_ACCESS",
	}

	values := make(map[string]string, len(required)+len(optionalKeys))
	for _, key := range required {
		value, ok := lookup(key)
		if !ok || strings.TrimSpace(value) == "" {
			return Config{}, fmt.Errorf("missing required environment variable %s", key)
		}
		values[key] = strings.TrimSpace(value)
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

	discordCommandName, err := parseDiscordCommandName(values["CLAW_DISCORD_COMMAND_NAME"])
	if err != nil {
		return Config{}, err
	}

	logLevel := values["CLAW_LOG_LEVEL"]
	if logLevel == "" {
		logLevel = "info"
	}

	logFormat := values["CLAW_LOG_FORMAT"]
	if logFormat == "" {
		logFormat = "json"
	}

	skipGitRepoCheck, err := loadOptionalBool(values, "CLAW_CODEX_SKIP_GIT_REPO_CHECK")
	if err != nil {
		return Config{}, err
	}

	networkAccess, err := loadOptionalBoolPointer(values, "CLAW_CODEX_NETWORK_ACCESS")
	if err != nil {
		return Config{}, err
	}

	return Config{
		Mode:                       mode,
		Timezone:                   location,
		TimezoneName:               values["CLAW_TIMEZONE"],
		DiscordToken:               values["CLAW_DISCORD_TOKEN"],
		DiscordGuildID:             values["CLAW_DISCORD_GUILD_ID"],
		DiscordCommandName:         discordCommandName,
		DataDir:                    values["CLAW_DATADIR"],
		SQLitePath:                 sqlitePath(values["CLAW_DATADIR"]),
		CodexExecutable:            values["CLAW_CODEX_EXECUTABLE"],
		CodexBaseURL:               values["CLAW_CODEX_BASE_URL"],
		CodexAPIKey:                values["CLAW_CODEX_API_KEY"],
		CodexWorkdir:               values["CLAW_CODEX_WORKDIR"],
		CodexModel:                 values["CLAW_CODEX_MODEL"],
		CodexSandboxMode:           values["CLAW_CODEX_SANDBOX_MODE"],
		CodexAdditionalDirectories: splitAdditionalDirectories(values["CLAW_CODEX_ADDITIONAL_DIRECTORIES"]),
		CodexSkipGitRepoCheck:      skipGitRepoCheck,
		CodexApprovalPolicy:        values["CLAW_CODEX_APPROVAL_POLICY"],
		CodexModelReasoningEffort:  values["CLAW_CODEX_MODEL_REASONING_EFFORT"],
		CodexWebSearchMode:         values["CLAW_CODEX_WEB_SEARCH_MODE"],
		CodexNetworkAccess:         networkAccess,
		LogLevel:                   logLevel,
		LogFormat:                  logFormat,
	}, nil
}

func ValidateRuntimePaths(cfg Config) error {
	if cfg.Mode != ModeTask {
		return nil
	}

	info, err := os.Stat(cfg.CodexWorkdir)
	if err != nil {
		return fmt.Errorf("task mode requires CLAW_CODEX_WORKDIR to exist: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("task mode requires CLAW_CODEX_WORKDIR to be a directory: %s", cfg.CodexWorkdir)
	}

	gitEntryPath := filepath.Join(cfg.CodexWorkdir, ".git")
	_, err = os.Stat(gitEntryPath)
	if err == nil {
		return nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("task mode requires CLAW_CODEX_WORKDIR to be a Git repository root: %s", cfg.CodexWorkdir)
	}

	return fmt.Errorf("inspect CLAW_CODEX_WORKDIR git metadata: %w", err)
}

func sqlitePath(dataDir string) string {
	return filepath.Join(dataDir, "39claw.sqlite")
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

func parseDiscordCommandName(raw string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if !discordCommandNamePattern.MatchString(normalized) {
		return "", fmt.Errorf(
			"invalid CLAW_DISCORD_COMMAND_NAME %q: use 1-32 lowercase letters, digits, or hyphens",
			raw,
		)
	}

	return normalized, nil
}

func splitAdditionalDirectories(raw string) []string {
	directories := filepath.SplitList(raw)
	filtered := make([]string, 0, len(directories))
	for _, directory := range directories {
		trimmed := strings.TrimSpace(directory)
		if trimmed == "" {
			continue
		}

		filtered = append(filtered, trimmed)
	}

	if len(filtered) == 0 {
		return nil
	}

	return filtered
}

func loadOptionalBool(values map[string]string, key string) (bool, error) {
	raw := values[key]
	if raw == "" {
		return false, nil
	}

	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", key, err)
	}

	return parsed, nil
}

func loadOptionalBoolPointer(values map[string]string, key string) (*bool, error) {
	raw := values[key]
	if raw == "" {
		return nil, nil
	}

	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", key, err)
	}

	return &parsed, nil
}
