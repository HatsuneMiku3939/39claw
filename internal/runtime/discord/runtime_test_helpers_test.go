package discord

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	"github.com/HatsuneMiku3939/39claw/internal/config"
	"github.com/HatsuneMiku3939/39claw/internal/testutil/runtimeharness"
	"github.com/HatsuneMiku3939/39claw/internal/thread"
	"github.com/bwmarrin/discordgo"
)

func newTestRuntime(t *testing.T, mode config.Mode, fakeSession *fakeSession) *Runtime {
	t.Helper()
	return newTestRuntimeWithTimeout(
		t,
		mode,
		fakeSession,
		&fakeMessageService{},
		&fakeDailyCommandService{},
		&fakeTaskCommandService{},
		http.DefaultClient,
		0,
	)
}

func newTestRuntimeWithServices(
	t *testing.T,
	mode config.Mode,
	fakeSession *fakeSession,
	messageService app.MessageService,
	taskService app.TaskCommandService,
) *Runtime {
	t.Helper()
	return newTestRuntimeWithTimeout(
		t,
		mode,
		fakeSession,
		messageService,
		&fakeDailyCommandService{},
		taskService,
		http.DefaultClient,
		0,
	)
}

func newTestRuntimeWithServicesAndClient(
	t *testing.T,
	mode config.Mode,
	fakeSession *fakeSession,
	messageService app.MessageService,
	taskService app.TaskCommandService,
	httpClient attachmentHTTPClient,
) *Runtime {
	t.Helper()
	return newTestRuntimeWithTimeout(
		t,
		mode,
		fakeSession,
		messageService,
		&fakeDailyCommandService{},
		taskService,
		httpClient,
		0,
	)
}

func newTestRuntimeWithTimeout(
	t *testing.T,
	mode config.Mode,
	fakeSession *fakeSession,
	messageService app.MessageService,
	dailyService app.DailyCommandService,
	taskService app.TaskCommandService,
	httpClient attachmentHTTPClient,
	shutdownDrainTimeout time.Duration,
) *Runtime {
	t.Helper()

	runtime, err := NewRuntime(Dependencies{
		Config: config.Config{
			Mode:               mode,
			TimezoneName:       "Asia/Tokyo",
			DiscordToken:       "discord-token",
			DiscordCommandName: "release",
		},
		Logger:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		Message:              messageService,
		DailyCommand:         dailyService,
		TaskCommand:          taskService,
		HTTPClient:           httpClient,
		ShutdownDrainTimeout: shutdownDrainTimeout,
		SessionFactory: func(token string) (session, error) {
			return fakeSession, nil
		},
	})
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}

	return runtime
}

func commandInteractionEvent(commandName string, userID string, action string, taskName string, taskID string) *discordgo.InteractionCreate {
	options := []*discordgo.ApplicationCommandInteractionDataOption{}
	if action != "" {
		options = append(options, &discordgo.ApplicationCommandInteractionDataOption{
			Name:  optionAction,
			Type:  discordgo.ApplicationCommandOptionString,
			Value: action,
		})
	}

	if taskName != "" {
		options = append(options, &discordgo.ApplicationCommandInteractionDataOption{
			Name:  optionTaskName,
			Type:  discordgo.ApplicationCommandOptionString,
			Value: taskName,
		})
	}

	if taskID != "" {
		options = append(options, &discordgo.ApplicationCommandInteractionDataOption{
			Name:  optionTaskID,
			Type:  discordgo.ApplicationCommandOptionString,
			Value: taskID,
		})
	}

	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type: discordgo.InteractionApplicationCommand,
			Data: discordgo.ApplicationCommandInteractionData{
				Name:    commandName,
				Options: options,
			},
			Member: &discordgo.Member{
				User: &discordgo.User{ID: userID},
			},
			AppID: "app-1",
			Token: "token-1",
		},
	}
}

func stringPointerValue(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

func waitForSignal(t *testing.T, ch <-chan struct{}, name string) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}

type fakeSession struct {
	selfUserID string

	mu     sync.Mutex
	notify chan struct{}

	opened bool
	closed bool

	registeredAppID    string
	registeredGuildID  string
	registeredCommands []*discordgo.ApplicationCommand

	messageHandlers     []func(*discordgo.Session, *discordgo.MessageCreate)
	interactionHandlers []func(*discordgo.Session, *discordgo.InteractionCreate)

	sentMessages         []*discordgo.MessageSend
	editedMessages       []*discordgo.MessageEdit
	deletedMessageIDs    []string
	interactionResponses []*discordgo.InteractionResponse
	interactionEdits     []*discordgo.WebhookEdit
	followups            []*discordgo.WebhookParams
	deliveries           []runtimeharness.Delivery
}

func newFakeSession(selfUserID string) *fakeSession {
	return &fakeSession{
		selfUserID: selfUserID,
		notify:     make(chan struct{}, 1),
	}
}

func (s *fakeSession) AddHandler(handler interface{}) func() {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch handler := handler.(type) {
	case func(*discordgo.Session, *discordgo.MessageCreate):
		s.messageHandlers = append(s.messageHandlers, handler)
	case func(*discordgo.Session, *discordgo.InteractionCreate):
		s.interactionHandlers = append(s.interactionHandlers, handler)
	}

	return func() {}
}

func (s *fakeSession) Open() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.opened = true
	return nil
}

func (s *fakeSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	return nil
}

func (s *fakeSession) ApplicationCommandBulkOverwrite(
	appID string,
	guildID string,
	commands []*discordgo.ApplicationCommand,
	options ...discordgo.RequestOption,
) ([]*discordgo.ApplicationCommand, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.registeredAppID = appID
	s.registeredGuildID = guildID
	s.registeredCommands = commands
	return commands, nil
}

func (s *fakeSession) ChannelMessageSendComplex(
	channelID string,
	data *discordgo.MessageSend,
	options ...discordgo.RequestOption,
) (*discordgo.Message, error) {
	s.mu.Lock()
	s.sentMessages = append(s.sentMessages, data)
	index := len(s.sentMessages)
	delivery := runtimeharness.Delivery{
		Kind:      runtimeharness.DeliveryKindChannelMessage,
		ChannelID: channelID,
		MessageID: "sent-message-" + strconv.Itoa(index),
		Text:      data.Content,
	}
	if data.Reference != nil {
		delivery.ReplyToID = data.Reference.MessageID
	}
	s.deliveries = append(s.deliveries, delivery)
	s.mu.Unlock()

	s.signal()

	return &discordgo.Message{
		ID:        "sent-message-" + strconv.Itoa(index),
		ChannelID: channelID,
	}, nil
}

func (s *fakeSession) ChannelMessageEditComplex(
	data *discordgo.MessageEdit,
	options ...discordgo.RequestOption,
) (*discordgo.Message, error) {
	s.mu.Lock()
	s.editedMessages = append(s.editedMessages, data)
	s.deliveries = append(s.deliveries, runtimeharness.Delivery{
		Kind:      runtimeharness.DeliveryKindChannelEdit,
		ChannelID: data.Channel,
		MessageID: data.ID,
		Text:      stringPointerValue(data.Content),
	})
	s.mu.Unlock()

	s.signal()

	return &discordgo.Message{ID: data.ID, ChannelID: data.Channel}, nil
}

func (s *fakeSession) ChannelMessageDelete(
	channelID string,
	messageID string,
	options ...discordgo.RequestOption,
) error {
	s.mu.Lock()
	s.deletedMessageIDs = append(s.deletedMessageIDs, messageID)
	s.deliveries = append(s.deliveries, runtimeharness.Delivery{
		Kind:      runtimeharness.DeliveryKindChannelDelete,
		ChannelID: channelID,
		MessageID: messageID,
	})
	s.mu.Unlock()

	s.signal()
	return nil
}

func (s *fakeSession) InteractionRespond(
	interaction *discordgo.Interaction,
	resp *discordgo.InteractionResponse,
	options ...discordgo.RequestOption,
) error {
	s.mu.Lock()
	s.interactionResponses = append(s.interactionResponses, resp)
	text := ""
	ephemeral := false
	if resp.Data != nil {
		text = resp.Data.Content
		ephemeral = resp.Data.Flags == discordgo.MessageFlagsEphemeral
	}
	s.deliveries = append(s.deliveries, runtimeharness.Delivery{
		Kind:      runtimeharness.DeliveryKindInteractionResponse,
		Text:      text,
		Ephemeral: ephemeral,
	})
	s.mu.Unlock()

	s.signal()
	return nil
}

func (s *fakeSession) InteractionResponseEdit(
	interaction *discordgo.Interaction,
	newresp *discordgo.WebhookEdit,
	options ...discordgo.RequestOption,
) (*discordgo.Message, error) {
	s.mu.Lock()
	s.interactionEdits = append(s.interactionEdits, newresp)
	s.deliveries = append(s.deliveries, runtimeharness.Delivery{
		Kind: runtimeharness.DeliveryKindInteractionEdit,
		Text: stringPointerValue(newresp.Content),
	})
	s.mu.Unlock()

	s.signal()
	return &discordgo.Message{ID: "edited-message"}, nil
}

func (s *fakeSession) FollowupMessageCreate(
	interaction *discordgo.Interaction,
	wait bool,
	data *discordgo.WebhookParams,
	options ...discordgo.RequestOption,
) (*discordgo.Message, error) {
	s.mu.Lock()
	s.followups = append(s.followups, data)
	s.deliveries = append(s.deliveries, runtimeharness.Delivery{
		Kind:      runtimeharness.DeliveryKindInteractionFollowup,
		Text:      data.Content,
		Ephemeral: data.Flags == discordgo.MessageFlagsEphemeral,
	})
	s.mu.Unlock()

	s.signal()
	return &discordgo.Message{ID: "followup-message"}, nil
}

func (s *fakeSession) SelfUserID() string {
	return s.selfUserID
}

func (s *fakeSession) dispatchMessage(event *discordgo.MessageCreate) {
	s.mu.Lock()
	handlers := append([]func(*discordgo.Session, *discordgo.MessageCreate){}, s.messageHandlers...)
	s.mu.Unlock()

	for _, handler := range handlers {
		handler(nil, event)
	}
}

func (s *fakeSession) dispatchInteraction(event *discordgo.InteractionCreate) {
	s.mu.Lock()
	handlers := append([]func(*discordgo.Session, *discordgo.InteractionCreate){}, s.interactionHandlers...)
	s.mu.Unlock()

	for _, handler := range handlers {
		handler(nil, event)
	}
}

func (s *fakeSession) dispatchHarnessMessage(event runtimeharness.MessageEvent) {
	content := event.Content
	mentions := []*discordgo.User{}
	if event.Mentioned {
		if content == "" {
			content = "<@" + s.selfUserID + ">"
		} else {
			content = "<@" + s.selfUserID + "> " + content
		}
		mentions = append(mentions, &discordgo.User{ID: s.selfUserID})
	}

	attachments := make([]*discordgo.MessageAttachment, 0, len(event.Attachments))
	for _, attachment := range event.Attachments {
		attachments = append(attachments, &discordgo.MessageAttachment{
			Filename:    attachment.Filename,
			ContentType: attachment.ContentType,
			URL:         attachment.URL,
		})
	}

	s.dispatchMessage(&discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:          event.MessageID,
			ChannelID:   event.ChannelID,
			Content:     content,
			Author:      &discordgo.User{ID: event.UserID},
			Mentions:    mentions,
			Attachments: attachments,
			Timestamp:   time.Date(2026, time.April, 9, 12, 0, 0, 0, time.UTC),
		},
	})
}

func (s *fakeSession) dispatchHarnessCommand(intent runtimeharness.CommandIntent) {
	s.dispatchInteraction(commandInteractionEvent(
		intent.CommandName,
		intent.UserID,
		intent.Action,
		intent.TaskName,
		intent.TaskID,
	))
}

func (s *fakeSession) Deliveries() []runtimeharness.Delivery {
	s.mu.Lock()
	defer s.mu.Unlock()

	deliveries := make([]runtimeharness.Delivery, len(s.deliveries))
	copy(deliveries, s.deliveries)
	return deliveries
}

func (s *fakeSession) waitForDeliveryCount(ctx context.Context, want int) ([]runtimeharness.Delivery, error) {
	for {
		deliveries := s.Deliveries()
		if len(deliveries) >= want {
			return deliveries, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-s.notify:
		}
	}
}

func (s *fakeSession) signal() {
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

type fakeMessageService struct {
	requests     []app.MessageRequest
	response     app.MessageResponse
	err          error
	handle       func(ctx context.Context, request app.MessageRequest, sink app.DeferredReplySink) (app.MessageResponse, error)
	waitForDrain func(ctx context.Context) error
}

func (s *fakeMessageService) HandleMessage(ctx context.Context, request app.MessageRequest, sink app.DeferredReplySink) (app.MessageResponse, error) {
	s.requests = append(s.requests, request)

	if s.handle != nil {
		return s.handle(ctx, request, sink)
	}

	if request.Cleanup != nil {
		request.Cleanup()
	}

	if s.err != nil {
		return app.MessageResponse{}, s.err
	}

	return s.response, nil
}

func (s *fakeMessageService) WaitForDrain(ctx context.Context) error {
	if s.waitForDrain == nil {
		return nil
	}

	return s.waitForDrain(ctx)
}

type fakeTaskCommandService struct {
	currentCalls []string
	createCalls  []struct {
		userID   string
		taskName string
	}

	currentResponse app.MessageResponse
	listResponse    app.MessageResponse
	createResponse  app.MessageResponse
	switchResponse  app.MessageResponse
	closeResponse   app.MessageResponse
}

func (s *fakeTaskCommandService) ShowCurrentTask(ctx context.Context, userID string) (app.MessageResponse, error) {
	s.currentCalls = append(s.currentCalls, userID)
	return s.currentResponse, nil
}

func (s *fakeTaskCommandService) ListTasks(ctx context.Context, userID string) (app.MessageResponse, error) {
	return s.listResponse, nil
}

func (s *fakeTaskCommandService) CreateTask(ctx context.Context, userID string, taskName string) (app.MessageResponse, error) {
	s.createCalls = append(s.createCalls, struct {
		userID   string
		taskName string
	}{userID: userID, taskName: taskName})
	return s.createResponse, nil
}

func (s *fakeTaskCommandService) SwitchTask(ctx context.Context, userID string, taskID string) (app.MessageResponse, error) {
	return s.switchResponse, nil
}

func (s *fakeTaskCommandService) CloseTask(ctx context.Context, userID string, taskID string) (app.MessageResponse, error) {
	return s.closeResponse, nil
}

type fakeDailyCommandService struct {
	clearCalls []struct {
		userID     string
		receivedAt time.Time
	}
	clearResponse app.MessageResponse
	clearErr      error
}

func (s *fakeDailyCommandService) Clear(ctx context.Context, userID string, receivedAt time.Time) (app.MessageResponse, error) {
	s.clearCalls = append(s.clearCalls, struct {
		userID     string
		receivedAt time.Time
	}{
		userID:     userID,
		receivedAt: receivedAt,
	})

	if s.clearErr != nil {
		return app.MessageResponse{}, s.clearErr
	}

	return s.clearResponse, nil
}

type stubQueueCoordinator struct {
	snapshot app.QueueSnapshot
}

func (c *stubQueueCoordinator) Admit(string, func()) (app.QueueAdmission, error) {
	return app.QueueAdmission{ExecuteNow: true}, nil
}

func (c *stubQueueCoordinator) Complete(string) (func(), bool) {
	return nil, false
}

func (c *stubQueueCoordinator) Snapshot(string) app.QueueSnapshot {
	return c.snapshot
}

type noopDailyMemoryRefresher struct{}

func (noopDailyMemoryRefresher) RefreshBeforeFirstDailyTurn(context.Context, app.DailySession) error {
	return nil
}

type memoryThreadStore struct {
	mu            sync.Mutex
	bindings      map[string]app.ThreadBinding
	dailySessions map[string]app.DailySession
	tasks         map[string]app.Task
	activeTasks   map[string]app.ActiveTask
}

func (s *memoryThreadStore) GetThreadBinding(_ context.Context, mode string, logicalThreadKey string) (app.ThreadBinding, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.bindings == nil {
		return app.ThreadBinding{}, false, nil
	}

	binding, ok := s.bindings[mode+":"+logicalThreadKey]
	return binding, ok, nil
}

func (s *memoryThreadStore) UpsertThreadBinding(_ context.Context, binding app.ThreadBinding) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.bindings == nil {
		s.bindings = make(map[string]app.ThreadBinding)
	}

	s.bindings[binding.Mode+":"+binding.LogicalThreadKey] = binding
	return nil
}

func (s *memoryThreadStore) GetActiveDailySession(_ context.Context, localDate string) (app.DailySession, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, session := range s.dailySessions {
		if session.LocalDate == localDate && session.IsActive {
			return session, true, nil
		}
	}

	return app.DailySession{}, false, nil
}

func (s *memoryThreadStore) GetLatestDailySessionBefore(_ context.Context, localDate string) (app.DailySession, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var latest app.DailySession
	ok := false
	for _, session := range s.dailySessions {
		if !session.IsActive || session.LocalDate >= localDate {
			continue
		}

		if !ok || session.LocalDate > latest.LocalDate || (session.LocalDate == latest.LocalDate && session.Generation > latest.Generation) {
			latest = session
			ok = true
		}
	}

	return latest, ok, nil
}

func (s *memoryThreadStore) CreateDailySession(_ context.Context, session app.DailySession) (app.DailySession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, existing := range s.dailySessions {
		if existing.LocalDate == session.LocalDate && existing.IsActive {
			return existing, nil
		}
	}

	if s.dailySessions == nil {
		s.dailySessions = make(map[string]app.DailySession)
	}

	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC)
	}
	if session.UpdatedAt.IsZero() {
		session.UpdatedAt = session.CreatedAt
	}
	session.IsActive = true
	s.dailySessions[session.LocalDate+":"+session.LogicalThreadKey] = session
	return session, nil
}

func (s *memoryThreadStore) RotateDailySession(_ context.Context, localDate string, activationReason string) (app.DailySession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.dailySessions == nil {
		return app.DailySession{}, sql.ErrNoRows
	}

	var activeKey string
	var active app.DailySession
	ok := false
	for key, session := range s.dailySessions {
		if session.LocalDate == localDate && session.IsActive {
			activeKey = key
			active = session
			ok = true
			break
		}
	}

	if !ok {
		return app.DailySession{}, sql.ErrNoRows
	}

	active.IsActive = false
	s.dailySessions[activeKey] = active

	next := app.DailySession{
		LocalDate:                localDate,
		Generation:               active.Generation + 1,
		LogicalThreadKey:         app.BuildDailyLogicalKey(localDate, active.Generation+1),
		PreviousLogicalThreadKey: active.LogicalThreadKey,
		ActivationReason:         activationReason,
		IsActive:                 true,
		CreatedAt:                time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
		UpdatedAt:                time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
	}
	s.dailySessions[next.LocalDate+":"+next.LogicalThreadKey] = next
	return next, nil
}

func (s *memoryThreadStore) CreateTask(_ context.Context, task app.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.tasks == nil {
		s.tasks = make(map[string]app.Task)
	}

	if task.Status == "" {
		task.Status = app.TaskStatusOpen
	}

	if task.BranchName == "" {
		task.BranchName = app.DefaultTaskBranchName(task.TaskID)
	}

	if task.WorktreeStatus == "" {
		task.WorktreeStatus = app.TaskWorktreeStatusPending
	}

	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC)
	}

	task.UpdatedAt = task.CreatedAt
	s.tasks[task.DiscordUserID+":"+task.TaskID] = task
	return nil
}

func (s *memoryThreadStore) GetTask(_ context.Context, userID string, taskID string) (app.Task, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.tasks == nil {
		return app.Task{}, false, nil
	}

	task, ok := s.tasks[userID+":"+taskID]
	return task, ok, nil
}

func (s *memoryThreadStore) UpdateTask(_ context.Context, task app.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.tasks == nil {
		return sql.ErrNoRows
	}

	key := task.DiscordUserID + ":" + task.TaskID
	if _, ok := s.tasks[key]; !ok {
		return sql.ErrNoRows
	}

	s.tasks[key] = task
	return nil
}

func (s *memoryThreadStore) ListOpenTasks(_ context.Context, userID string) ([]app.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tasks := make([]app.Task, 0)
	for _, task := range s.tasks {
		if task.DiscordUserID == userID && task.Status == app.TaskStatusOpen {
			tasks = append(tasks, task)
		}
	}

	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].CreatedAt.Equal(tasks[j].CreatedAt) {
			return tasks[i].TaskID < tasks[j].TaskID
		}
		return tasks[i].CreatedAt.Before(tasks[j].CreatedAt)
	})

	return tasks, nil
}

func (s *memoryThreadStore) ListClosedReadyTasks(_ context.Context) ([]app.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tasks := make([]app.Task, 0)
	for _, task := range s.tasks {
		if task.Status == app.TaskStatusClosed && task.WorktreeStatus == app.TaskWorktreeStatusReady && task.ClosedAt != nil {
			tasks = append(tasks, task)
		}
	}

	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].ClosedAt.Equal(*tasks[j].ClosedAt) {
			return tasks[i].TaskID < tasks[j].TaskID
		}
		return tasks[i].ClosedAt.After(*tasks[j].ClosedAt)
	})

	return tasks, nil
}

func (s *memoryThreadStore) SetActiveTask(_ context.Context, activeTask app.ActiveTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.activeTasks == nil {
		s.activeTasks = make(map[string]app.ActiveTask)
	}

	s.activeTasks[activeTask.DiscordUserID] = activeTask
	return nil
}

func (s *memoryThreadStore) GetActiveTask(_ context.Context, discordUserID string) (app.ActiveTask, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.activeTasks == nil {
		return app.ActiveTask{}, false, nil
	}

	activeTask, ok := s.activeTasks[discordUserID]
	return activeTask, ok, nil
}

func (s *memoryThreadStore) ClearActiveTask(_ context.Context, discordUserID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.activeTasks != nil {
		delete(s.activeTasks, discordUserID)
	}

	return nil
}

func (s *memoryThreadStore) CloseTask(_ context.Context, discordUserID string, taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.tasks == nil {
		return sql.ErrNoRows
	}

	key := discordUserID + ":" + taskID
	task, ok := s.tasks[key]
	if !ok {
		return sql.ErrNoRows
	}

	closedAt := time.Date(2026, time.April, 5, 1, 0, 0, 0, time.UTC)
	task.Status = app.TaskStatusClosed
	task.ClosedAt = &closedAt
	task.UpdatedAt = closedAt
	s.tasks[key] = task

	if s.activeTasks != nil {
		delete(s.activeTasks, discordUserID)
	}

	return nil
}

type runTurnCall struct {
	threadID         string
	workingDirectory string
	input            app.CodexTurnInput
}

type fakeCodexGateway struct {
	mu      sync.Mutex
	calls   []runTurnCall
	results []app.RunTurnResult
}

func (g *fakeCodexGateway) RunTurn(_ context.Context, threadID string, input app.CodexTurnInput) (app.RunTurnResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.calls = append(g.calls, runTurnCall{
		threadID:         threadID,
		workingDirectory: input.WorkingDirectory,
		input: app.CodexTurnInput{
			Prompt:           input.Prompt,
			ImagePaths:       append([]string(nil), input.ImagePaths...),
			WorkingDirectory: input.WorkingDirectory,
		},
	})

	if len(g.results) == 0 {
		return app.RunTurnResult{}, nil
	}

	result := g.results[0]
	g.results = g.results[1:]
	return result, nil
}

func (g *fakeCodexGateway) Calls() []runTurnCall {
	g.mu.Lock()
	defer g.mu.Unlock()

	calls := make([]runTurnCall, len(g.calls))
	copy(calls, g.calls)
	return calls
}

type progressCodexGateway struct {
	fakeCodexGateway
	progress []string
	result   app.RunTurnResult
}

func (g *progressCodexGateway) RunTurn(ctx context.Context, threadID string, input app.CodexTurnInput) (app.RunTurnResult, error) {
	g.mu.Lock()
	g.calls = append(g.calls, runTurnCall{
		threadID:         threadID,
		workingDirectory: input.WorkingDirectory,
		input: app.CodexTurnInput{
			Prompt:           input.Prompt,
			ImagePaths:       append([]string(nil), input.ImagePaths...),
			WorkingDirectory: input.WorkingDirectory,
		},
	})
	g.mu.Unlock()

	for _, update := range g.progress {
		if input.ProgressSink == nil {
			return app.RunTurnResult{}, context.Canceled
		}

		if err := input.ProgressSink.Deliver(ctx, app.MessageProgress{Text: update}); err != nil {
			return app.RunTurnResult{}, err
		}
	}

	return g.result, nil
}

type blockingCodexGateway struct {
	mu        sync.Mutex
	calls     []runTurnCall
	results   []app.RunTurnResult
	started   chan struct{}
	release   chan struct{}
	startOnce sync.Once
}

func newBlockingCodexGateway(results ...app.RunTurnResult) *blockingCodexGateway {
	return &blockingCodexGateway{
		results: append([]app.RunTurnResult(nil), results...),
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (g *blockingCodexGateway) RunTurn(_ context.Context, threadID string, input app.CodexTurnInput) (app.RunTurnResult, error) {
	g.mu.Lock()
	g.calls = append(g.calls, runTurnCall{
		threadID:         threadID,
		workingDirectory: input.WorkingDirectory,
		input: app.CodexTurnInput{
			Prompt:           input.Prompt,
			ImagePaths:       append([]string(nil), input.ImagePaths...),
			WorkingDirectory: input.WorkingDirectory,
		},
	})

	var result app.RunTurnResult
	if len(g.results) > 0 {
		result = g.results[0]
		g.results = g.results[1:]
	}
	release := g.release
	g.mu.Unlock()

	g.startOnce.Do(func() {
		close(g.started)
	})

	<-release
	return result, nil
}

func (g *blockingCodexGateway) Calls() []runTurnCall {
	g.mu.Lock()
	defer g.mu.Unlock()

	calls := make([]runTurnCall, len(g.calls))
	copy(calls, g.calls)
	return calls
}

func newContractDailyMessageService(
	t *testing.T,
	store app.ThreadStore,
	gateway app.CodexGateway,
	coordinator app.QueueCoordinator,
) *app.DefaultMessageService {
	t.Helper()

	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	policy, err := thread.NewPolicy(config.ModeDaily, tokyo, nil)
	if err != nil {
		t.Fatalf("thread.NewPolicy() error = %v", err)
	}

	if coordinator == nil {
		coordinator = thread.NewQueueCoordinator()
	}

	service, err := app.NewMessageService(app.MessageServiceDependencies{
		Mode:        config.ModeDaily,
		CommandName: "release",
		Policy:      policy,
		Store:       store,
		DailyMemory: noopDailyMemoryRefresher{},
		Gateway:     gateway,
		Coordinator: coordinator,
	})
	if err != nil {
		t.Fatalf("NewMessageService() error = %v", err)
	}

	return service
}

func newContractTaskCommandService(
	t *testing.T,
	store app.ThreadStore,
	newTaskID func() string,
) *app.DefaultTaskCommandService {
	t.Helper()

	service, err := app.NewTaskCommandService(app.TaskCommandServiceDependencies{
		CommandName: "release",
		Store:       store,
		NewTaskID:   newTaskID,
	})
	if err != nil {
		t.Fatalf("NewTaskCommandService() error = %v", err)
	}

	return service
}
