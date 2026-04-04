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

	gateway := codex.NewGateway(client, codex.GatewayOptions{
		ThreadOptions: codex.ThreadOptions{
			WorkingDirectory: cfg.CodexWorkdir,
			ApprovalPolicy:   codex.ApprovalModeNever,
			SandboxMode:      codex.SandboxModeWorkspaceWrite,
			WebSearchMode:    codex.WebSearchModeLive,
		},
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
