package discord

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	"github.com/HatsuneMiku3939/39claw/internal/config"
	"github.com/bwmarrin/discordgo"
)

const (
	defaultShutdownDrainTimeout = 5 * time.Second
	forcedShutdownWaitTimeout   = time.Second
)

type Dependencies struct {
	Config               config.Config
	Logger               *slog.Logger
	Message              app.MessageService
	DailyCommand         app.DailyCommandService
	TaskCommand          app.TaskCommandService
	SessionFactory       sessionFactory
	HTTPClient           attachmentHTTPClient
	ShutdownDrainTimeout time.Duration
}

type Runtime struct {
	config       config.Config
	logger       *slog.Logger
	message      app.MessageService
	dailyCommand app.DailyCommandService
	taskCommand  app.TaskCommandService

	sessionFactory sessionFactory
	httpClient     attachmentHTTPClient

	shutdownDrainTimeout time.Duration

	mu               sync.Mutex
	session          session
	cleanups         []func()
	runtimeLifecycle *lifecycleContext
	closing          bool
	workers          sync.WaitGroup
}

func NewRuntime(deps Dependencies) (*Runtime, error) {
	if deps.Logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	if deps.Message == nil {
		return nil, errors.New("message service must not be nil")
	}

	if deps.Config.Mode == config.ModeDaily && deps.DailyCommand == nil {
		return nil, errors.New("daily command service must not be nil in daily mode")
	}

	if deps.Config.Mode == config.ModeTask && deps.TaskCommand == nil {
		return nil, errors.New("task command service must not be nil in task mode")
	}

	if deps.Config.DiscordToken == "" {
		return nil, errors.New("discord token must not be empty")
	}

	factory := deps.SessionFactory
	if factory == nil {
		factory = newLiveSession
	}

	httpClient := deps.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	shutdownDrainTimeout := deps.ShutdownDrainTimeout
	if shutdownDrainTimeout <= 0 {
		shutdownDrainTimeout = defaultShutdownDrainTimeout
	}

	return &Runtime{
		config:               deps.Config,
		logger:               deps.Logger,
		message:              deps.Message,
		dailyCommand:         deps.DailyCommand,
		taskCommand:          deps.TaskCommand,
		sessionFactory:       factory,
		httpClient:           httpClient,
		shutdownDrainTimeout: shutdownDrainTimeout,
	}, nil
}

func (r *Runtime) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.session != nil {
		return errors.New("discord runtime already started")
	}

	discordSession, err := r.sessionFactory(r.config.DiscordToken)
	if err != nil {
		return fmt.Errorf("create discord session: %w", err)
	}

	r.runtimeLifecycle = newLifecycleContext()
	r.closing = false

	r.session = discordSession
	r.cleanups = []func(){
		discordSession.AddHandler(r.handleMessageCreate),
		discordSession.AddHandler(r.handleInteractionCreate),
	}

	if err := discordSession.Open(); err != nil {
		r.runtimeLifecycle.Cancel()
		r.resetLocked()
		return fmt.Errorf("open discord session: %w", err)
	}

	appID := discordSession.SelfUserID()
	if appID == "" {
		_ = discordSession.Close()
		r.runtimeLifecycle.Cancel()
		r.resetLocked()
		return errors.New("discord session did not expose the bot user id after open")
	}

	if _, err := discordSession.ApplicationCommandBulkOverwrite(
		appID,
		r.config.DiscordGuildID,
		registeredCommands(r.config),
	); err != nil {
		_ = discordSession.Close()
		r.runtimeLifecycle.Cancel()
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
	if r.session == nil {
		r.resetLocked()
		r.mu.Unlock()
		return nil
	}

	r.closing = true
	discordSession := r.session
	cleanups := append([]func(){}, r.cleanups...)
	runtimeLifecycle := r.runtimeLifecycle
	shutdownDrainTimeout := r.shutdownDrainTimeout
	r.mu.Unlock()

	for _, cleanup := range cleanups {
		if cleanup != nil {
			cleanup()
		}
	}

	r.logger.Info("discord runtime shutdown started", "graceful_timeout", shutdownDrainTimeout)

	drainCtx, drainCancel := context.WithTimeout(context.Background(), shutdownDrainTimeout)
	drainErr := r.waitForDrain(drainCtx)
	drainCancel()
	if drainErr != nil {
		r.logger.Warn(
			"discord runtime shutdown timed out; canceling queued and in-flight turns",
			"error",
			drainErr,
			"graceful_timeout",
			shutdownDrainTimeout,
		)

		if runtimeLifecycle != nil {
			runtimeLifecycle.Cancel()
		}

		forcedCtx, forcedCancel := context.WithTimeout(context.Background(), forcedShutdownWaitTimeout)
		forcedErr := r.waitForDrain(forcedCtx)
		forcedCancel()
		if forcedErr != nil {
			r.logger.Warn("discord runtime forced shutdown still had active work", "error", forcedErr)
		}
	} else {
		r.logger.Info("discord runtime shutdown drained queued and in-flight turns")
		if runtimeLifecycle != nil {
			runtimeLifecycle.Cancel()
		}
	}

	err := discordSession.Close()

	r.mu.Lock()
	r.resetLocked()
	r.mu.Unlock()

	if err != nil {
		return fmt.Errorf("close discord session: %w", err)
	}

	r.logger.Info("discord runtime stopped")
	return nil
}

func (r *Runtime) resetLocked() {
	r.session = nil
	r.cleanups = nil
	r.runtimeLifecycle = nil
	r.closing = false
}

func (r *Runtime) handleMessageCreate(_ *discordgo.Session, event *discordgo.MessageCreate) {
	discordSession, runtimeCtx, ok := r.beginWork()
	if !ok {
		return
	}
	defer r.workers.Done()

	request, ok := mapMessageCreate(discordSession.SelfUserID(), event)
	if !ok {
		return
	}

	imagePaths, cleanup, err := prepareImageAttachments(runtimeCtx, r.httpClient, event.Attachments)
	if err != nil {
		if cleanup != nil {
			cleanup()
		}

		r.logger.Error("prepare image attachments", "error", err, "channel_id", request.ChannelID, "message_id", request.MessageID)
		if err := r.presentMessageResponse(discordSession, request.ChannelID, app.MessageResponse{
			Text:      imageDownloadErrorMessage,
			ReplyToID: request.MessageID,
		}); err != nil {
			r.logger.Error("present attachment error response", "error", err, "channel_id", request.ChannelID, "message_id", request.MessageID)
		}
		return
	}

	request.ImagePaths = imagePaths
	request.Cleanup = cleanup
	if strings.TrimSpace(request.Content) == "" && len(request.ImagePaths) == 0 {
		if cleanup != nil {
			cleanup()
		}

		return
	}

	livePresenter := newLiveMessagePresenter(discordSession, request.ChannelID, request.MessageID, r.config.CodexWorkdir)
	request.ProgressSink = app.MessageProgressSinkFunc(func(ctx context.Context, progress app.MessageProgress) error {
		if err := ctx.Err(); err != nil {
			return err
		}

		return livePresenter.Update(progress.Text)
	})

	deferredSink := app.DeferredReplySinkFunc(func(ctx context.Context, response app.MessageResponse) error {
		logAttrs := []any{
			"event", "deferred_reply_delivery",
			"channel_id", request.ChannelID,
			"message_id", request.MessageID,
			"reply_to_id", response.ReplyToID,
		}

		if err := ctx.Err(); err != nil {
			r.logger.Warn(
				"deferred reply delivery dropped during shutdown",
				append(logAttrs, "outcome", "dropped_on_shutdown", "error", err)...,
			)
			return err
		}

		r.mu.Lock()
		currentSession := r.session
		r.mu.Unlock()

		if currentSession == nil {
			err := errors.New("discord runtime is not running")
			r.logger.Error(
				"deferred reply delivery failed",
				append(logAttrs, "outcome", "failure", "error", err)...,
			)
			return err
		}

		if err := r.presentMessageResponse(currentSession, request.ChannelID, response); err != nil {
			r.logger.Error(
				"deferred reply delivery failed",
				append(logAttrs, "outcome", "failure", "error", err)...,
			)
			return err
		}

		r.logger.Info(
			"deferred reply delivery succeeded",
			append(logAttrs, "outcome", "success")...,
		)
		return nil
	})

	response, err := r.message.HandleMessage(runtimeCtx, request, deferredSink)
	if err != nil {
		r.logger.Error("handle message", "error", err, "channel_id", request.ChannelID, "message_id", request.MessageID)
		response = app.MessageResponse{
			Text:      internalErrorMessage,
			ReplyToID: request.MessageID,
		}
	}

	var presentErr error
	if livePresenter.Active() {
		presentErr = livePresenter.Update(response.Text)
	} else {
		presentErr = r.presentMessageResponse(discordSession, request.ChannelID, response)
	}

	if presentErr != nil {
		r.logger.Error("present message response", "error", presentErr, "channel_id", request.ChannelID, "message_id", request.MessageID)
	}
}

func (r *Runtime) handleInteractionCreate(_ *discordgo.Session, event *discordgo.InteractionCreate) {
	discordSession, runtimeCtx, ok := r.beginWork()
	if !ok {
		return
	}
	defer r.workers.Done()

	request, ok := mapInteractionCommand(event)
	if !ok {
		return
	}

	response, err := r.routeCommand(runtimeCtx, request)
	if err != nil {
		r.logger.Error("handle interaction", "error", err, "command", request.Name, "user_id", request.UserID)
		response = app.MessageResponse{
			Text:      internalErrorMessage,
			Ephemeral: true,
		}
	}

	if err := r.presentInteractionResponse(discordSession, event.Interaction, response); err != nil {
		r.logger.Error("present interaction response", "error", err, "command", request.Name, "user_id", request.UserID)
	}
}

func (r *Runtime) beginWork() (session, context.Context, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.session == nil || r.closing || r.runtimeLifecycle == nil {
		return nil, nil, false
	}

	r.workers.Add(1)
	return r.session, r.runtimeLifecycle, true
}

func (r *Runtime) waitForDrain(ctx context.Context) error {
	if err := waitForWaitGroup(ctx, &r.workers); err != nil {
		return err
	}

	drainer, ok := r.message.(app.DrainableMessageService)
	if !ok {
		return nil
	}

	return drainer.WaitForDrain(ctx)
}

func waitForWaitGroup(ctx context.Context, wg *sync.WaitGroup) error {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type lifecycleContext struct {
	mu   sync.Mutex
	done chan struct{}
	err  error
	once sync.Once
}

func newLifecycleContext() *lifecycleContext {
	return &lifecycleContext{
		done: make(chan struct{}),
	}
}

func (c *lifecycleContext) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (c *lifecycleContext) Done() <-chan struct{} {
	return c.done
}

func (c *lifecycleContext) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.err
}

func (c *lifecycleContext) Value(any) any {
	return nil
}

func (c *lifecycleContext) Cancel() {
	c.once.Do(func() {
		c.mu.Lock()
		c.err = context.Canceled
		c.mu.Unlock()
		close(c.done)
	})
}

func (r *Runtime) presentMessageResponse(discordSession session, channelID string, response app.MessageResponse) error {
	response.Text = formatDiscordResponseText(response.Text, r.config.CodexWorkdir)
	return presentMessage(discordSession, channelID, response)
}

func (r *Runtime) presentInteractionResponse(
	discordSession session,
	interaction *discordgo.Interaction,
	response app.MessageResponse,
) error {
	response.Text = formatDiscordResponseText(response.Text, r.config.CodexWorkdir)
	return presentInteraction(discordSession, interaction, response)
}

func (r *Runtime) routeCommand(ctx context.Context, request commandRequest) (app.MessageResponse, error) {
	if request.Name != r.config.DiscordCommandName {
		return app.MessageResponse{
			Text:      fmt.Sprintf("Unsupported command. Use `/%s action:%s`.", r.config.DiscordCommandName, actionHelp),
			Ephemeral: true,
		}, nil
	}

	switch request.Action {
	case actionHelp:
		return helpResponse(r.config.DiscordCommandName, r.config.Mode), nil
	case actionClear:
		if r.config.Mode != config.ModeDaily {
			return app.MessageResponse{
				Text:      unsupportedActionText(r.config.DiscordCommandName, r.config.Mode),
				Ephemeral: true,
			}, nil
		}

		return r.dailyCommand.Clear(ctx, request.UserID, time.Now())
	case actionTaskCurrent, actionTaskList, actionTaskNew, actionTaskSwitch, actionTaskClose:
		if r.config.Mode != config.ModeTask {
			return app.MessageResponse{
				Text:      taskUnavailableDailyMode(r.config.DiscordCommandName),
				Ephemeral: true,
			}, nil
		}

		switch request.Action {
		case actionTaskCurrent:
			return r.taskCommand.ShowCurrentTask(ctx, request.UserID)
		case actionTaskList:
			return r.taskCommand.ListTasks(ctx, request.UserID)
		case actionTaskNew:
			return r.taskCommand.CreateTask(ctx, request.UserID, request.TaskName)
		case actionTaskSwitch:
			return r.taskCommand.SwitchTask(ctx, request.UserID, request.TaskID)
		case actionTaskClose:
			return r.taskCommand.CloseTask(ctx, request.UserID, request.TaskID)
		}

		return app.MessageResponse{
			Text:      unsupportedActionText(r.config.DiscordCommandName, r.config.Mode),
			Ephemeral: true,
		}, nil
	default:
		return app.MessageResponse{
			Text:      unsupportedActionText(r.config.DiscordCommandName, r.config.Mode),
			Ephemeral: true,
		}, nil
	}
}
