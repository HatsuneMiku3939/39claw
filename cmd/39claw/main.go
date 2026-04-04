package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	"github.com/HatsuneMiku3939/39claw/internal/codex"
	"github.com/HatsuneMiku3939/39claw/internal/config"
	"github.com/HatsuneMiku3939/39claw/internal/observe"
	runtimediscord "github.com/HatsuneMiku3939/39claw/internal/runtime/discord"
	sqlitestore "github.com/HatsuneMiku3939/39claw/internal/store/sqlite"
	"github.com/HatsuneMiku3939/39claw/internal/thread"
)

const exitCodeFailure = 1

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

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.LookupEnv)
	stop()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(exitCodeFailure)
	}
}

func run(ctx context.Context, lookupEnv func(string) (string, bool)) error {
	cfg, err := config.LoadFromLookup(lookupEnv)
	if err != nil {
		return err
	}

	logger, err := observe.NewLogger(cfg.LogLevel)
	if err != nil {
		return err
	}

	store, err := sqlitestore.Open(cfg.SQLitePath)
	if err != nil {
		return fmt.Errorf("open sqlite store: %w", err)
	}
	defer func() {
		closeErr := store.Close()
		if closeErr != nil && !errors.Is(closeErr, context.Canceled) {
			logger.Error("close sqlite store", "error", closeErr)
		}
	}()

	if err := store.InitSchema(ctx); err != nil {
		return fmt.Errorf("initialize sqlite schema: %w", err)
	}

	client := codex.New(codex.Options{
		ExecutablePath: cfg.CodexExecutable,
		BaseURL:        cfg.CodexBaseURL,
		APIKey:         cfg.CodexAPIKey,
	})

	threadOptions, err := loadThreadOptions(cfg)
	if err != nil {
		return err
	}

	gateway := newCodexGateway(client, codex.GatewayOptions{
		ThreadOptions: threadOptions,
	})

	policy, err := thread.NewPolicy(cfg.Mode, cfg.Timezone, store)
	if err != nil {
		return fmt.Errorf("build thread policy: %w", err)
	}

	messageService, err := app.NewMessageService(app.MessageServiceDependencies{
		Mode:    cfg.Mode,
		Policy:  policy,
		Store:   store,
		Gateway: gateway,
		Guard:   thread.NewGuard(),
	})
	if err != nil {
		return fmt.Errorf("build message service: %w", err)
	}

	taskService, err := app.NewTaskCommandService(app.TaskCommandServiceDependencies{
		Store: store,
	})
	if err != nil {
		return fmt.Errorf("build task service: %w", err)
	}

	runtime, err := newDiscordRuntime(runtimediscord.Dependencies{
		Config:      cfg,
		Logger:      logger,
		Message:     messageService,
		TaskCommand: taskService,
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

func cloneBoolPointer(value *bool) *bool {
	if value == nil {
		return nil
	}

	cloned := *value
	return &cloned
}
