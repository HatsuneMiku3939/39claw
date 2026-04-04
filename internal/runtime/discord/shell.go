package discord

import (
	"context"
	"errors"
	"log/slog"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	"github.com/HatsuneMiku3939/39claw/internal/config"
)

type Dependencies struct {
	Config  config.Config
	Logger  *slog.Logger
	Store   app.ThreadStore
	Gateway app.CodexGateway
}

type Shell struct {
	config  config.Config
	logger  *slog.Logger
	store   app.ThreadStore
	gateway app.CodexGateway
}

func NewShell(deps Dependencies) (*Shell, error) {
	if deps.Logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	if deps.Store == nil {
		return nil, errors.New("thread store must not be nil")
	}

	if deps.Gateway == nil {
		return nil, errors.New("codex gateway must not be nil")
	}

	if deps.Config.DiscordToken == "" {
		return nil, errors.New("discord token must not be empty")
	}

	return &Shell{
		config:  deps.Config,
		logger:  deps.Logger,
		store:   deps.Store,
		gateway: deps.Gateway,
	}, nil
}

func (s *Shell) Run(ctx context.Context) error {
	s.logger.Info(
		"discord runtime shell started",
		"mode",
		s.config.Mode,
		"timezone",
		s.config.TimezoneName,
	)

	<-ctx.Done()

	s.logger.Info("discord runtime shell stopped", "reason", ctx.Err())
	return nil
}
