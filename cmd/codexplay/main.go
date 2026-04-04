package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/HatsuneMiku3939/39bot/internal/codex"
)

const (
	exitCodeSuccess = 0
	exitCodeFailure = 1
	exitCodeUsage   = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

type cliConfig struct {
	prompt                string
	stream                bool
	resumeThreadID        string
	images                []string
	codexPath             string
	baseURL               string
	apiKey                string
	model                 string
	sandboxMode           string
	workingDirectory      string
	additionalDirectories []string
	skipGitRepoCheck      bool
	approvalPolicy        string
	modelReasoningEffort  string
	webSearchMode         string
	networkAccess         string
}

func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	config, helpShown, err := parseCLIConfig(args, stdin, stderr)
	if helpShown {
		return exitCodeSuccess
	}

	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
		return exitCodeUsage
	}

	threadOptions, err := config.threadOptions()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
		return exitCodeUsage
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client := codex.New(codex.Options{
		ExecutablePath: config.codexPath,
		BaseURL:        config.baseURL,
		APIKey:         config.apiKey,
	})

	input := config.input()
	thread := client.StartThread(threadOptions)
	if config.resumeThreadID != "" {
		thread = client.ResumeThread(config.resumeThreadID, threadOptions)
	}

	if config.stream {
		return runStreamed(ctx, thread, input, stdout, stderr)
	}

	return runBuffered(ctx, thread, input, stdout, stderr)
}

func runBuffered(
	ctx context.Context,
	thread *codex.Thread,
	input codex.Input,
	stdout io.Writer,
	stderr io.Writer,
) int {
	turn, err := thread.Run(ctx, input)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "run failed: %v\n", err)
		return exitCodeFailure
	}

	if turn.FinalResponse != "" {
		if _, err := io.WriteString(stdout, turn.FinalResponse); err != nil {
			_, _ = fmt.Fprintf(stderr, "write response: %v\n", err)
			return exitCodeFailure
		}

		if !strings.HasSuffix(turn.FinalResponse, "\n") {
			if _, err := io.WriteString(stdout, "\n"); err != nil {
				_, _ = fmt.Fprintf(stderr, "write newline: %v\n", err)
				return exitCodeFailure
			}
		}
	}

	writeMetadata(stderr, thread.ID(), turn.Usage)
	return exitCodeSuccess
}

func runStreamed(
	ctx context.Context,
	thread *codex.Thread,
	input codex.Input,
	stdout io.Writer,
	stderr io.Writer,
) int {
	stream, err := thread.RunStreamed(ctx, input)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "stream setup failed: %v\n", err)
		return exitCodeFailure
	}

	for event := range stream.Events() {
		payload, err := json.Marshal(event)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "marshal event: %v\n", err)
			return exitCodeFailure
		}

		if _, err := fmt.Fprintln(stdout, string(payload)); err != nil {
			_, _ = fmt.Fprintf(stderr, "write event: %v\n", err)
			return exitCodeFailure
		}
	}

	if err := stream.Wait(); err != nil {
		_, _ = fmt.Fprintf(stderr, "stream failed: %v\n", err)
		return exitCodeFailure
	}

	writeMetadata(stderr, thread.ID(), nil)
	return exitCodeSuccess
}

func writeMetadata(stderr io.Writer, threadID string, usage *codex.Usage) {
	if threadID != "" {
		_, _ = fmt.Fprintf(stderr, "thread_id=%s\n", threadID)
	}

	if usage != nil {
		_, _ = fmt.Fprintf(
			stderr,
			"usage input=%d cached=%d output=%d\n",
			usage.InputTokens,
			usage.CachedInputTokens,
			usage.OutputTokens,
		)
	}
}

func parseCLIConfig(args []string, stdin io.Reader, stderr io.Writer) (cliConfig, bool, error) {
	var config cliConfig
	var images stringSliceFlag
	var additionalDirectories stringSliceFlag

	fs := flag.NewFlagSet("codexplay", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "Usage: codexplay [options] [prompt]")
		_, _ = fmt.Fprintln(stderr)
		_, _ = fmt.Fprintln(stderr, "A small CLI for exercising internal/codex against the real Codex CLI.")
		_, _ = fmt.Fprintln(stderr)
		_, _ = fmt.Fprintln(stderr, "Options:")
		fs.PrintDefaults()
		_, _ = fmt.Fprintln(stderr)
		_, _ = fmt.Fprintln(stderr, `Examples:`)
		_, _ = fmt.Fprintln(stderr, `  codexplay --prompt "Summarize this repo"`)
		_, _ = fmt.Fprintln(stderr, `  codexplay --stream --image ./ui.png "Describe this screenshot"`)
		_, _ = fmt.Fprintln(stderr, `  codexplay --resume thread_123 --stream "Continue the plan"`)
	}

	fs.StringVar(&config.prompt, "prompt", "", "Prompt text to send to Codex")
	fs.BoolVar(&config.stream, "stream", false, "Print streamed Codex events as JSONL")
	fs.StringVar(&config.resumeThreadID, "resume", "", "Resume an existing Codex thread ID")
	fs.Var(&images, "image", "Attach a local image path (repeatable)")
	fs.StringVar(&config.codexPath, "codex-path", "", "Override the codex executable path")
	fs.StringVar(&config.baseURL, "base-url", "", "Override the Codex/OpenAI base URL")
	fs.StringVar(&config.apiKey, "api-key", "", "Override the API key for the codex process")
	fs.StringVar(&config.model, "model", "", "Model name to pass to Codex")
	fs.StringVar(&config.sandboxMode, "sandbox", "", "Sandbox mode: read-only, workspace-write, danger-full-access")
	fs.StringVar(&config.workingDirectory, "cwd", "", "Working directory for the Codex run")
	fs.Var(&additionalDirectories, "add-dir", "Additional writable directory for Codex (repeatable)")
	fs.BoolVar(&config.skipGitRepoCheck, "skip-git-repo-check", false, "Skip the Git repository check")
	fs.StringVar(&config.approvalPolicy, "approval-policy", "", "Approval policy: never, on-request, on-failure, untrusted")
	fs.StringVar(&config.modelReasoningEffort, "model-reasoning-effort", "", "Reasoning effort: minimal, low, medium, high, xhigh")
	fs.StringVar(&config.webSearchMode, "web-search", "", "Web search mode: disabled, cached, live")
	fs.StringVar(&config.networkAccess, "network-access", "", `Set network access to "true" or "false"`)

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return cliConfig{}, true, nil
		}

		return cliConfig{}, false, err
	}

	config.images = append([]string(nil), images...)
	config.additionalDirectories = append([]string(nil), additionalDirectories...)

	prompt, err := resolvePrompt(config.prompt, fs.Args(), stdin)
	if err != nil {
		return cliConfig{}, false, err
	}

	config.prompt = prompt
	return config, false, nil
}

func resolvePrompt(flagPrompt string, positional []string, stdin io.Reader) (string, error) {
	if flagPrompt != "" && len(positional) > 0 {
		return "", errors.New("use either --prompt or positional prompt arguments, not both")
	}

	if flagPrompt != "" {
		return flagPrompt, nil
	}

	if len(positional) > 0 {
		return strings.Join(positional, " "), nil
	}

	if file, ok := stdin.(*os.File); ok {
		info, err := file.Stat()
		if err != nil {
			return "", fmt.Errorf("stat stdin: %w", err)
		}

		if info.Mode()&os.ModeCharDevice == 0 {
			content, err := io.ReadAll(stdin)
			if err != nil {
				return "", fmt.Errorf("read stdin: %w", err)
			}

			return strings.TrimRight(string(content), "\r\n"), nil
		}
	}

	return "", errors.New("prompt is required; use --prompt, positional text, or piped stdin")
}

func (c cliConfig) input() codex.Input {
	parts := make([]codex.InputPart, 0, len(c.images)+1)
	if c.prompt != "" {
		parts = append(parts, codex.TextPart(c.prompt))
	}

	for _, image := range c.images {
		parts = append(parts, codex.LocalImagePart(image))
	}

	return codex.MultiPartInput(parts...)
}

func (c cliConfig) threadOptions() (codex.ThreadOptions, error) {
	options := codex.ThreadOptions{
		Model:                 c.model,
		WorkingDirectory:      c.workingDirectory,
		AdditionalDirectories: append([]string(nil), c.additionalDirectories...),
		SkipGitRepoCheck:      c.skipGitRepoCheck,
	}

	if c.sandboxMode != "" {
		mode, err := parseSandboxMode(c.sandboxMode)
		if err != nil {
			return codex.ThreadOptions{}, err
		}

		options.SandboxMode = mode
	}

	if c.approvalPolicy != "" {
		policy, err := parseApprovalPolicy(c.approvalPolicy)
		if err != nil {
			return codex.ThreadOptions{}, err
		}

		options.ApprovalPolicy = policy
	}

	if c.modelReasoningEffort != "" {
		effort, err := parseReasoningEffort(c.modelReasoningEffort)
		if err != nil {
			return codex.ThreadOptions{}, err
		}

		options.ModelReasoningEffort = effort
	}

	if c.webSearchMode != "" {
		mode, err := parseWebSearchMode(c.webSearchMode)
		if err != nil {
			return codex.ThreadOptions{}, err
		}

		options.WebSearchMode = mode
	}

	if c.networkAccess != "" {
		enabled, err := strconv.ParseBool(c.networkAccess)
		if err != nil {
			return codex.ThreadOptions{}, fmt.Errorf("parse network access: %w", err)
		}

		options.NetworkAccessEnabled = &enabled
	}

	return options, nil
}

func parseSandboxMode(value string) (codex.SandboxMode, error) {
	switch codex.SandboxMode(value) {
	case codex.SandboxModeReadOnly, codex.SandboxModeWorkspaceWrite, codex.SandboxModeDangerFullAccess:
		return codex.SandboxMode(value), nil
	default:
		return "", fmt.Errorf("invalid sandbox mode %q", value)
	}
}

func parseApprovalPolicy(value string) (codex.ApprovalMode, error) {
	switch codex.ApprovalMode(value) {
	case codex.ApprovalModeNever, codex.ApprovalModeOnRequest, codex.ApprovalModeOnFailure, codex.ApprovalModeUntrusted:
		return codex.ApprovalMode(value), nil
	default:
		return "", fmt.Errorf("invalid approval policy %q", value)
	}
}

func parseReasoningEffort(value string) (codex.ModelReasoningEffort, error) {
	switch codex.ModelReasoningEffort(value) {
	case codex.ModelReasoningEffortMinimal,
		codex.ModelReasoningEffortLow,
		codex.ModelReasoningEffortMedium,
		codex.ModelReasoningEffortHigh,
		codex.ModelReasoningEffortXHigh:
		return codex.ModelReasoningEffort(value), nil
	default:
		return "", fmt.Errorf("invalid model reasoning effort %q", value)
	}
}

func parseWebSearchMode(value string) (codex.WebSearchMode, error) {
	switch codex.WebSearchMode(value) {
	case codex.WebSearchModeDisabled, codex.WebSearchModeCached, codex.WebSearchModeLive:
		return codex.WebSearchMode(value), nil
	default:
		return "", fmt.Errorf("invalid web search mode %q", value)
	}
}
