package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	"github.com/HatsuneMiku3939/39claw/internal/codex"
	"github.com/HatsuneMiku3939/39claw/internal/config"
	"github.com/HatsuneMiku3939/39claw/internal/dailymemory"
	"github.com/HatsuneMiku3939/39claw/internal/observe"
	runtimediscord "github.com/HatsuneMiku3939/39claw/internal/runtime/discord"
	sqlitestore "github.com/HatsuneMiku3939/39claw/internal/store/sqlite"
	"github.com/HatsuneMiku3939/39claw/internal/thread"
	"github.com/HatsuneMiku3939/39claw/version"
)

const (
	exitCodeSuccess = 0
	exitCodeFailure = 1
	exitCodeUsage   = 2
)

type cliCommand string

const (
	cliCommandServe   cliCommand = "serve"
	cliCommandVersion cliCommand = "version"
)

type discordRuntime interface {
	Start(ctx context.Context) error
	Close() error
}

var newDiscordRuntime = func(deps runtimediscord.Dependencies) (discordRuntime, error) {
	return runtimediscord.NewRuntime(deps)
}

var newCodexGateway = func(client *codex.Client, options codex.GatewayOptions) app.CodexGateway {
	return codex.NewGateway(client, options)
}

var newCodexClient = codex.New

func main() {
	os.Exit(runCLI(os.Args[1:], os.LookupEnv, os.Stdout, os.Stderr))
}

func runCLI(
	args []string,
	lookupEnv func(string) (string, bool),
	stdout io.Writer,
	stderr io.Writer,
) int {
	command, err := parseCLIArgs(args)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
		return exitCodeUsage
	}

	if command == cliCommandVersion {
		_, _ = fmt.Fprintln(stdout, version.Version)
		return exitCodeSuccess
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err = run(ctx, lookupEnv)
	stop()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
		return exitCodeFailure
	}

	return exitCodeSuccess
}

func parseCLIArgs(args []string) (cliCommand, error) {
	if len(args) == 0 {
		return cliCommandServe, nil
	}

	switch args[0] {
	case string(cliCommandVersion):
		if len(args) > 1 {
			return "", errors.New("version command does not accept arguments")
		}

		return cliCommandVersion, nil
	default:
		return "", fmt.Errorf("unknown command %q", args[0])
	}
}

func run(ctx context.Context, lookupEnv func(string) (string, bool)) error {
	cfg, err := config.LoadFromLookup(lookupEnv)
	if err != nil {
		return err
	}

	if err := config.ValidateRuntimePaths(cfg); err != nil {
		return err
	}

	logger, err := observe.NewLogger(cfg.LogLevel, cfg.LogFormat)
	if err != nil {
		return err
	}
	slog.SetDefault(logger)

	db, err := sqlitestore.OpenDB(ctx, cfg.SQLitePath)
	if err != nil {
		return fmt.Errorf("open sqlite database: %w", err)
	}
	defer func() {
		closeErr := db.Close()
		if closeErr != nil && !errors.Is(closeErr, context.Canceled) {
			logger.Error("close sqlite database", "error", closeErr)
		}
	}()

	if err := sqlitestore.Migrate(ctx, db); err != nil {
		return fmt.Errorf("migrate sqlite database: %w", err)
	}

	store := sqlitestore.New(db)

	client := newCodexClient(codex.Options{
		ExecutablePath: cfg.CodexExecutable,
		Env:            codexProcessEnv(cfg),
		BaseURL:        cfg.CodexBaseURL,
		APIKey:         cfg.CodexAPIKey,
	})

	threadOptions, err := loadThreadOptions(cfg)
	if err != nil {
		return err
	}

	if cfg.Mode == config.ModeDaily {
		if threadOptions.SandboxMode == codex.SandboxModeReadOnly {
			return errors.New("daily memory bridge requires CLAW_CODEX_SANDBOX_MODE to allow writes inside CLAW_CODEX_WORKDIR")
		}

		if err := (dailymemory.Bootstrap{Workdir: cfg.CodexWorkdir}).Ensure(ctx); err != nil {
			return fmt.Errorf("bootstrap daily memory bridge: %w", err)
		}
	}

	gateway := newCodexGateway(client, codex.GatewayOptions{
		ThreadOptions: threadOptions,
	})

	coordinator := thread.NewQueueCoordinator()
	var dailyMemory app.DailyMemoryRefresher
	var dailyCommand app.DailyCommandService
	if cfg.Mode == config.ModeDaily {
		dailyMemory = dailymemory.Refresher{
			Store:   store,
			Gateway: gateway,
			Workdir: cfg.CodexWorkdir,
		}

		dailyCommand, err = app.NewDailyCommandService(app.DailyCommandServiceDependencies{
			CommandName: cfg.DiscordCommandName,
			Timezone:    cfg.Timezone,
			Store:       store,
			Coordinator: coordinator,
		})
		if err != nil {
			return fmt.Errorf("build daily command service: %w", err)
		}
	}

	var workspaceManager app.TaskWorkspaceManager
	if cfg.Mode == config.ModeTask {
		workspaceManager, err = app.NewTaskWorkspaceManager(ctx, app.TaskWorkspaceManagerDependencies{
			Store:            store,
			SourceRepository: cfg.CodexWorkdir,
			DataDir:          cfg.DataDir,
			Logger:           logger,
		})
		if err != nil {
			return fmt.Errorf("build task workspace manager: %w", err)
		}
	}

	policy, err := thread.NewPolicy(cfg.Mode, cfg.Timezone, store)
	if err != nil {
		return fmt.Errorf("build thread policy: %w", err)
	}

	messageService, err := app.NewMessageService(app.MessageServiceDependencies{
		Mode:             cfg.Mode,
		CommandName:      cfg.DiscordCommandName,
		Policy:           policy,
		Store:            store,
		WorkspaceManager: workspaceManager,
		DailyMemory:      dailyMemory,
		Gateway:          gateway,
		Coordinator:      coordinator,
	})
	if err != nil {
		return fmt.Errorf("build message service: %w", err)
	}

	taskService, err := app.NewTaskCommandService(app.TaskCommandServiceDependencies{
		CommandName:      cfg.DiscordCommandName,
		Store:            store,
		WorkspaceManager: workspaceManager,
	})
	if err != nil {
		return fmt.Errorf("build task service: %w", err)
	}

	runtime, err := newDiscordRuntime(runtimediscord.Dependencies{
		Config:       cfg,
		Logger:       logger,
		Message:      messageService,
		DailyCommand: dailyCommand,
		TaskCommand:  taskService,
	})
	if err != nil {
		return fmt.Errorf("build discord runtime: %w", err)
	}

	if err := runtime.Start(ctx); err != nil {
		return fmt.Errorf("start discord runtime: %w", err)
	}

	<-ctx.Done()

	if err := runtime.Close(); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("close discord runtime: %w", err)
	}

	return nil
}

var (
	_ app.ThreadStore  = (*sqlitestore.Store)(nil)
	_ app.CodexGateway = (*codex.Gateway)(nil)
)

func loadThreadOptions(cfg config.Config) (codex.ThreadOptions, error) {
	options := codex.ThreadOptions{
		Model:                 cfg.CodexModel,
		WorkingDirectory:      cfg.CodexWorkdir,
		AdditionalDirectories: append([]string(nil), cfg.CodexAdditionalDirectories...),
		SkipGitRepoCheck:      cfg.CodexSkipGitRepoCheck,
		NetworkAccessEnabled:  cloneBoolPointer(cfg.CodexNetworkAccess),
		ApprovalPolicy:        codex.ApprovalModeNever,
		SandboxMode:           codex.SandboxModeWorkspaceWrite,
		WebSearchMode:         codex.WebSearchModeLive,
	}

	if cfg.CodexSandboxMode != "" {
		sandboxMode, err := codex.ParseSandboxMode(cfg.CodexSandboxMode)
		if err != nil {
			return codex.ThreadOptions{}, fmt.Errorf("parse CLAW_CODEX_SANDBOX_MODE: %w", err)
		}

		options.SandboxMode = sandboxMode
	}

	if cfg.CodexApprovalPolicy != "" {
		approvalPolicy, err := codex.ParseApprovalPolicy(cfg.CodexApprovalPolicy)
		if err != nil {
			return codex.ThreadOptions{}, fmt.Errorf("parse CLAW_CODEX_APPROVAL_POLICY: %w", err)
		}

		options.ApprovalPolicy = approvalPolicy
	}

	if cfg.CodexModelReasoningEffort != "" {
		effort, err := codex.ParseModelReasoningEffort(cfg.CodexModelReasoningEffort)
		if err != nil {
			return codex.ThreadOptions{}, fmt.Errorf("parse CLAW_CODEX_MODEL_REASONING_EFFORT: %w", err)
		}

		options.ModelReasoningEffort = effort
	}

	if cfg.CodexWebSearchMode != "" {
		webSearchMode, err := codex.ParseWebSearchMode(cfg.CodexWebSearchMode)
		if err != nil {
			return codex.ThreadOptions{}, fmt.Errorf("parse CLAW_CODEX_WEB_SEARCH_MODE: %w", err)
		}

		options.WebSearchMode = webSearchMode
	}

	return options, nil
}

func codexProcessEnv(cfg config.Config) map[string]string {
	if cfg.CodexHome == "" {
		return nil
	}

	return map[string]string{
		"CODEX_HOME": cfg.CodexHome,
	}
}

func cloneBoolPointer(value *bool) *bool {
	if value == nil {
		return nil
	}

	cloned := *value
	return &cloned
}
