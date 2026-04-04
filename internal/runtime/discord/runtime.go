package discord

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	"github.com/HatsuneMiku3939/39claw/internal/config"
	"github.com/bwmarrin/discordgo"
)

type Dependencies struct {
	Config         config.Config
	Logger         *slog.Logger
	Message        app.MessageService
	TaskCommand    app.TaskCommandService
	SessionFactory sessionFactory
}

type Runtime struct {
	config      config.Config
	logger      *slog.Logger
	message     app.MessageService
	taskCommand app.TaskCommandService

	sessionFactory sessionFactory

	mu       sync.Mutex
	session  session
	cleanups []func()
}

func NewRuntime(deps Dependencies) (*Runtime, error) {
	if deps.Logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	if deps.Message == nil {
		return nil, errors.New("message service must not be nil")
	}

	if deps.TaskCommand == nil {
		return nil, errors.New("task command service must not be nil")
	}

	if deps.Config.DiscordToken == "" {
		return nil, errors.New("discord token must not be empty")
	}

	factory := deps.SessionFactory
	if factory == nil {
		factory = newLiveSession
	}

	return &Runtime{
		config:         deps.Config,
		logger:         deps.Logger,
		message:        deps.Message,
		taskCommand:    deps.TaskCommand,
		sessionFactory: factory,
	}, nil
}

func (r *Runtime) Start(_ context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.session != nil {
		return errors.New("discord runtime already started")
	}

	discordSession, err := r.sessionFactory(r.config.DiscordToken)
	if err != nil {
		return fmt.Errorf("create discord session: %w", err)
	}

	r.session = discordSession
	r.cleanups = []func(){
		discordSession.AddHandler(r.handleMessageCreate),
		discordSession.AddHandler(r.handleInteractionCreate),
	}

	if err := discordSession.Open(); err != nil {
		r.resetLocked()
		return fmt.Errorf("open discord session: %w", err)
	}

	appID := discordSession.SelfUserID()
	if appID == "" {
		_ = discordSession.Close()
		r.resetLocked()
		return errors.New("discord session did not expose the bot user id after open")
	}

	if _, err := discordSession.ApplicationCommandBulkOverwrite(
		appID,
		r.config.DiscordGuildID,
		registeredCommands(),
	); err != nil {
		_ = discordSession.Close()
		r.resetLocked()
		return fmt.Errorf("register application commands: %w", err)
	}

	r.logger.Info(
		"discord runtime started",
		"mode",
		r.config.Mode,
		"timezone",
		r.config.TimezoneName,
		"guild_id",
		r.config.DiscordGuildID,
	)

	return nil
}

func (r *Runtime) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.session == nil {
		return nil
	}

	for _, cleanup := range r.cleanups {
		if cleanup != nil {
			cleanup()
		}
	}

	err := r.session.Close()
	r.resetLocked()
	if err != nil {
		return fmt.Errorf("close discord session: %w", err)
	}

	r.logger.Info("discord runtime stopped")
	return nil
}

func (r *Runtime) resetLocked() {
	r.session = nil
	r.cleanups = nil
}

func (r *Runtime) handleMessageCreate(_ *discordgo.Session, event *discordgo.MessageCreate) {
	r.mu.Lock()
	discordSession := r.session
	r.mu.Unlock()

	if discordSession == nil {
		return
	}

	request, ok := mapMessageCreate(discordSession.SelfUserID(), event)
	if !ok {
		return
	}

	response, err := r.message.HandleMessage(context.Background(), request)
	if err != nil {
		r.logger.Error("handle message", "error", err, "channel_id", request.ChannelID, "message_id", request.MessageID)
		response = app.MessageResponse{
			Text:      internalErrorMessage,
			ReplyToID: request.MessageID,
		}
	}

	if err := presentMessage(discordSession, request.ChannelID, response); err != nil {
		r.logger.Error("present message response", "error", err, "channel_id", request.ChannelID, "message_id", request.MessageID)
	}
}

func (r *Runtime) handleInteractionCreate(_ *discordgo.Session, event *discordgo.InteractionCreate) {
	r.mu.Lock()
	discordSession := r.session
	r.mu.Unlock()

	if discordSession == nil {
		return
	}

	request, ok := mapInteractionCommand(event)
	if !ok {
		return
	}

	response, err := r.routeCommand(context.Background(), request)
	if err != nil {
		r.logger.Error("handle interaction", "error", err, "command", request.Name, "user_id", request.UserID)
		response = app.MessageResponse{
			Text:      internalErrorMessage,
			Ephemeral: true,
		}
	}

	if err := presentInteraction(discordSession, event.Interaction, response); err != nil {
		r.logger.Error("present interaction response", "error", err, "command", request.Name, "user_id", request.UserID)
	}
}

func (r *Runtime) routeCommand(ctx context.Context, request commandRequest) (app.MessageResponse, error) {
	switch request.Name {
	case commandHelp:
		return helpResponse(r.config.Mode), nil
	case commandTask:
		if r.config.Mode != config.ModeTask {
			return app.MessageResponse{
				Text:      taskUnavailableDailyMode,
				Ephemeral: true,
			}, nil
		}

		switch request.Task.Action {
		case "", taskActionCurrent:
			return r.taskCommand.ShowCurrentTask(ctx, request.UserID)
		case taskActionList:
			return r.taskCommand.ListTasks(ctx, request.UserID)
		case taskActionNew:
			return r.taskCommand.CreateTask(ctx, request.UserID, request.Task.Name)
		case taskActionSwitch:
			return r.taskCommand.SwitchTask(ctx, request.UserID, request.Task.ID)
		case taskActionClose:
			return r.taskCommand.CloseTask(ctx, request.UserID, request.Task.ID)
		default:
			return app.MessageResponse{
				Text:      unsupportedTaskActionText,
				Ephemeral: true,
			}, nil
		}
	default:
		return app.MessageResponse{
			Text:      "Unsupported command.",
			Ephemeral: true,
		}, nil
	}
}
