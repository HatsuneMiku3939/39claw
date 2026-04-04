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
)

const exitCodeFailure = 1

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

	shell, err := runtimediscord.NewShell(runtimediscord.Dependencies{
		Config:  cfg,
		Logger:  logger,
		Store:   store,
		Gateway: gateway,
	})
	if err != nil {
		return fmt.Errorf("build discord runtime: %w", err)
	}

	if err := shell.Run(ctx); err != nil {
		return fmt.Errorf("run discord runtime: %w", err)
	}

	return nil
}

var (
	_ app.ThreadStore  = (*sqlitestore.Store)(nil)
	_ app.CodexGateway = (*codex.Gateway)(nil)
)
