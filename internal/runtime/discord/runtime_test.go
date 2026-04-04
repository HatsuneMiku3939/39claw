package discord

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	"github.com/HatsuneMiku3939/39claw/internal/config"
	"github.com/bwmarrin/discordgo"
)

func TestRuntimeStartRegistersCommands(t *testing.T) {
	t.Parallel()

	fakeSession := newFakeSession("bot-user")
	runtime := newTestRuntime(t, config.ModeDaily, fakeSession)
	runtime.config.DiscordGuildID = "guild-1"

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.Close()
	})

	if !fakeSession.opened {
		t.Fatal("session opened = false, want true")
	}

	if fakeSession.registeredAppID != "bot-user" {
		t.Fatalf("registered app id = %q, want %q", fakeSession.registeredAppID, "bot-user")
	}

	if fakeSession.registeredGuildID != "guild-1" {
		t.Fatalf("registered guild id = %q, want %q", fakeSession.registeredGuildID, "guild-1")
	}

	if len(fakeSession.registeredCommands) != 2 {
		t.Fatalf("registered command count = %d, want %d", len(fakeSession.registeredCommands), 2)
	}
}

func TestRuntimeMentionHandlingRepliesToTriggerMessage(t *testing.T) {
	t.Parallel()

	messageService := &fakeMessageService{
		response: app.MessageResponse{
			Text:      "Hello from runtime",
			ReplyToID: "message-1",
		},
	}
	fakeSession := newFakeSession("bot-user")
	runtime := newTestRuntimeWithServices(t, config.ModeDaily, fakeSession, messageService, &fakeTaskCommandService{})

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.Close()
	})

	fakeSession.dispatchMessage(&discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "message-1",
			ChannelID: "channel-1",
			Content:   "<@bot-user> hello there",
			Author:    &discordgo.User{ID: "user-1"},
			Mentions: []*discordgo.User{
				{ID: "bot-user"},
			},
		},
	})

	if len(messageService.requests) != 1 {
		t.Fatalf("message request count = %d, want %d", len(messageService.requests), 1)
	}

	if messageService.requests[0].Content != "hello there" {
		t.Fatalf("mapped content = %q, want %q", messageService.requests[0].Content, "hello there")
	}

	if len(messageService.requests[0].ImagePaths) != 0 {
		t.Fatalf("image path count = %d, want %d", len(messageService.requests[0].ImagePaths), 0)
	}

	if len(fakeSession.sentMessages) != 1 {
		t.Fatalf("sent message count = %d, want %d", len(fakeSession.sentMessages), 1)
	}

	if fakeSession.sentMessages[0].Reference == nil || fakeSession.sentMessages[0].Reference.MessageID != "message-1" {
		t.Fatal("first sent message missing reply reference")
	}
}

func TestRuntimeMentionHandlingPresentsQueuedAcknowledgementAndDeferredReply(t *testing.T) {
	t.Parallel()

	deliverQueuedResponse := make(chan struct{})
	delivered := make(chan struct{})
	messageService := &fakeMessageService{
		handle: func(ctx context.Context, request app.MessageRequest, sink app.DeferredReplySink) (app.MessageResponse, error) {
			go func() {
				<-deliverQueuedResponse
				_ = sink.Deliver(context.Background(), app.MessageResponse{
					Text:      "Final queued response",
					ReplyToID: request.MessageID,
				})
				if request.Cleanup != nil {
					request.Cleanup()
				}
				close(delivered)
			}()

			return app.MessageResponse{
				Text:      "A response is already running for this conversation. Your message has been queued at position 1.",
				ReplyToID: request.MessageID,
			}, nil
		},
	}
	fakeSession := newFakeSession("bot-user")
	runtime := newTestRuntimeWithServices(t, config.ModeDaily, fakeSession, messageService, &fakeTaskCommandService{})

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.Close()
	})

	fakeSession.dispatchMessage(&discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "message-1",
			ChannelID: "channel-1",
			Content:   "<@bot-user> queue me",
			Author:    &discordgo.User{ID: "user-1"},
			Mentions: []*discordgo.User{
				{ID: "bot-user"},
			},
		},
	})

	if len(fakeSession.sentMessages) != 1 {
		t.Fatalf("sent message count after ack = %d, want %d", len(fakeSession.sentMessages), 1)
	}

	if fakeSession.sentMessages[0].Content != "A response is already running for this conversation. Your message has been queued at position 1." {
		t.Fatalf("queued ack content = %q", fakeSession.sentMessages[0].Content)
	}

	close(deliverQueuedResponse)

	select {
	case <-delivered:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for deferred runtime delivery")
	}

	if len(fakeSession.sentMessages) != 2 {
		t.Fatalf("sent message count after deferred delivery = %d, want %d", len(fakeSession.sentMessages), 2)
	}

	if fakeSession.sentMessages[1].Content != "Final queued response" {
		t.Fatalf("deferred content = %q, want %q", fakeSession.sentMessages[1].Content, "Final queued response")
	}

	if fakeSession.sentMessages[1].Reference == nil || fakeSession.sentMessages[1].Reference.MessageID != "message-1" {
		t.Fatal("deferred sent message missing reply reference")
	}
}

func TestRuntimeMentionHandlingDownloadsImageAttachments(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("png-bytes"))
	}))
	t.Cleanup(server.Close)

	messageService := &fakeMessageService{
		response: app.MessageResponse{
			Text:      "Image-aware response",
			ReplyToID: "message-1",
		},
	}
	fakeSession := newFakeSession("bot-user")
	runtime := newTestRuntimeWithServicesAndClient(t, config.ModeDaily, fakeSession, messageService, &fakeTaskCommandService{}, server.Client())

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.Close()
	})

	fakeSession.dispatchMessage(&discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "message-1",
			ChannelID: "channel-1",
			Content:   "<@bot-user> describe this",
			Author:    &discordgo.User{ID: "user-1"},
			Mentions: []*discordgo.User{
				{ID: "bot-user"},
			},
			Attachments: []*discordgo.MessageAttachment{
				{
					Filename:    "sample.png",
					ContentType: "image/png",
					URL:         server.URL + "/sample.png",
				},
			},
		},
	})

	if len(messageService.requests) != 1 {
		t.Fatalf("message request count = %d, want %d", len(messageService.requests), 1)
	}

	request := messageService.requests[0]
	if request.Content != "describe this" {
		t.Fatalf("content = %q, want %q", request.Content, "describe this")
	}

	if len(request.ImagePaths) != 1 {
		t.Fatalf("image path count = %d, want %d", len(request.ImagePaths), 1)
	}

	if _, err := os.Stat(request.ImagePaths[0]); !os.IsNotExist(err) {
		t.Fatalf("downloaded image path should be cleaned up, stat err = %v", err)
	}
}

func TestRuntimeMentionHandlingAcceptsImageOnlyMessage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("png-bytes"))
	}))
	t.Cleanup(server.Close)

	messageService := &fakeMessageService{
		response: app.MessageResponse{
			Text:      "Image-only response",
			ReplyToID: "message-1",
		},
	}
	fakeSession := newFakeSession("bot-user")
	runtime := newTestRuntimeWithServicesAndClient(t, config.ModeDaily, fakeSession, messageService, &fakeTaskCommandService{}, server.Client())

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.Close()
	})

	fakeSession.dispatchMessage(&discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "message-1",
			ChannelID: "channel-1",
			Content:   "<@bot-user>",
			Author:    &discordgo.User{ID: "user-1"},
			Mentions: []*discordgo.User{
				{ID: "bot-user"},
			},
			Attachments: []*discordgo.MessageAttachment{
				{
					Filename:    "sample.png",
					ContentType: "image/png",
					URL:         server.URL + "/sample.png",
				},
			},
		},
	})

	if len(messageService.requests) != 1 {
		t.Fatalf("message request count = %d, want %d", len(messageService.requests), 1)
	}

	if messageService.requests[0].Content != "" {
		t.Fatalf("content = %q, want empty", messageService.requests[0].Content)
	}

	if len(messageService.requests[0].ImagePaths) != 1 {
		t.Fatalf("image path count = %d, want %d", len(messageService.requests[0].ImagePaths), 1)
	}
}

func TestRuntimeMentionHandlingIgnoresImageOnlyMessageWithoutUsableImages(t *testing.T) {
	t.Parallel()

	messageService := &fakeMessageService{}
	fakeSession := newFakeSession("bot-user")
	runtime := newTestRuntimeWithServices(t, config.ModeDaily, fakeSession, messageService, &fakeTaskCommandService{})

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.Close()
	})

	fakeSession.dispatchMessage(&discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "message-1",
			ChannelID: "channel-1",
			Content:   "<@bot-user>",
			Author:    &discordgo.User{ID: "user-1"},
			Mentions: []*discordgo.User{
				{ID: "bot-user"},
			},
			Attachments: []*discordgo.MessageAttachment{
				{
					Filename:    "notes.txt",
					ContentType: "text/plain",
					URL:         "https://example.test/notes.txt",
				},
			},
		},
	})

	if len(messageService.requests) != 0 {
		t.Fatalf("message request count = %d, want %d", len(messageService.requests), 0)
	}

	if len(fakeSession.sentMessages) != 0 {
		t.Fatalf("sent message count = %d, want %d", len(fakeSession.sentMessages), 0)
	}
}

func TestRuntimeMentionHandlingReturnsErrorWhenAttachmentDownloadFails(t *testing.T) {
	t.Parallel()

	messageService := &fakeMessageService{}
	fakeSession := newFakeSession("bot-user")
	runtime := newTestRuntimeWithServicesAndClient(t, config.ModeDaily, fakeSession, messageService, &fakeTaskCommandService{}, http.DefaultClient)

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.Close()
	})

	fakeSession.dispatchMessage(&discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "message-1",
			ChannelID: "channel-1",
			Content:   "<@bot-user> describe this",
			Author:    &discordgo.User{ID: "user-1"},
			Mentions: []*discordgo.User{
				{ID: "bot-user"},
			},
			Attachments: []*discordgo.MessageAttachment{
				{
					Filename:    "sample.png",
					ContentType: "image/png",
					URL:         "http://127.0.0.1:1/unreachable.png",
				},
			},
		},
	})

	if len(messageService.requests) != 0 {
		t.Fatalf("message request count = %d, want %d", len(messageService.requests), 0)
	}

	if len(fakeSession.sentMessages) != 1 {
		t.Fatalf("sent message count = %d, want %d", len(fakeSession.sentMessages), 1)
	}

	if fakeSession.sentMessages[0].Content != imageDownloadErrorMessage {
		t.Fatalf("error message = %q, want %q", fakeSession.sentMessages[0].Content, imageDownloadErrorMessage)
	}
}

func TestRuntimeIgnoresUnsupportedChatter(t *testing.T) {
	t.Parallel()

	messageService := &fakeMessageService{}
	fakeSession := newFakeSession("bot-user")
	runtime := newTestRuntimeWithServices(t, config.ModeDaily, fakeSession, messageService, &fakeTaskCommandService{})

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.Close()
	})

	fakeSession.dispatchMessage(&discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "message-1",
			ChannelID: "channel-1",
			Content:   "just chatting",
			Author:    &discordgo.User{ID: "user-1"},
		},
	})

	if len(messageService.requests) != 0 {
		t.Fatalf("message request count = %d, want %d", len(messageService.requests), 0)
	}

	if len(fakeSession.sentMessages) != 0 {
		t.Fatalf("sent message count = %d, want %d", len(fakeSession.sentMessages), 0)
	}
}

func TestRuntimeTaskCommandInDailyModeIsEphemeral(t *testing.T) {
	t.Parallel()

	fakeSession := newFakeSession("bot-user")
	runtime := newTestRuntime(t, config.ModeDaily, fakeSession)

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.Close()
	})

	fakeSession.dispatchInteraction(taskInteractionEvent("user-1", taskActionCurrent, "", ""))

	if len(fakeSession.interactionResponses) != 1 {
		t.Fatalf("interaction response count = %d, want %d", len(fakeSession.interactionResponses), 1)
	}

	response := fakeSession.interactionResponses[0]
	if response.Data == nil {
		t.Fatal("response data = nil, want non-nil")
	}

	if response.Data.Flags != discordgo.MessageFlagsEphemeral {
		t.Fatalf("response flags = %v, want ephemeral", response.Data.Flags)
	}

	if !strings.Contains(response.Data.Content, "not available") {
		t.Fatalf("response content = %q, want task unavailable guidance", response.Data.Content)
	}
}

func TestRuntimeTaskCommandRoutesTaskModeSubcommands(t *testing.T) {
	t.Parallel()

	taskService := &fakeTaskCommandService{
		createResponse: app.MessageResponse{
			Text:      "Created task `Release work` (`task-1`) and made it active. Your next message will continue this task.",
			Ephemeral: true,
		},
	}
	fakeSession := newFakeSession("bot-user")
	runtime := newTestRuntimeWithServices(t, config.ModeTask, fakeSession, &fakeMessageService{}, taskService)

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.Close()
	})

	fakeSession.dispatchInteraction(taskInteractionEvent("user-1", taskActionNew, "Release work", ""))

	if len(taskService.createCalls) != 1 {
		t.Fatalf("create call count = %d, want %d", len(taskService.createCalls), 1)
	}

	if taskService.createCalls[0].taskName != "Release work" {
		t.Fatalf("create task name = %q, want %q", taskService.createCalls[0].taskName, "Release work")
	}

	if len(fakeSession.interactionResponses) != 1 {
		t.Fatalf("interaction response count = %d, want %d", len(fakeSession.interactionResponses), 1)
	}

	response := fakeSession.interactionResponses[0]
	if response.Data == nil || response.Data.Flags != discordgo.MessageFlagsEphemeral {
		t.Fatal("task response should be ephemeral")
	}
}

func TestPresentInteractionChunksLongResponses(t *testing.T) {
	t.Parallel()

	fakeSession := newFakeSession("bot-user")
	longCode := "```go\n" + strings.Repeat("fmt.Println(\"hello\")\n", 220) + "```"

	if err := presentInteraction(fakeSession, &discordgo.Interaction{
		AppID: "app-1",
		Token: "token-1",
	}, app.MessageResponse{
		Text:      longCode,
		Ephemeral: true,
	}); err != nil {
		t.Fatalf("presentInteraction() error = %v", err)
	}

	if len(fakeSession.interactionResponses) != 1 {
		t.Fatalf("interaction response count = %d, want %d", len(fakeSession.interactionResponses), 1)
	}

	if fakeSession.interactionResponses[0].Type != discordgo.InteractionResponseDeferredChannelMessageWithSource {
		t.Fatalf("response type = %v, want deferred", fakeSession.interactionResponses[0].Type)
	}

	if len(fakeSession.interactionEdits) != 1 {
		t.Fatalf("interaction edit count = %d, want %d", len(fakeSession.interactionEdits), 1)
	}

	if len(fakeSession.followups) == 0 {
		t.Fatal("followup count = 0, want at least one")
	}
}

func TestChunkTextPreservesCodeFences(t *testing.T) {
	t.Parallel()

	text := "```go\n" + strings.Repeat("fmt.Println(\"hi\")\n", 220) + "```"
	chunks := chunkText(text)
	if len(chunks) < 2 {
		t.Fatalf("chunk count = %d, want at least %d", len(chunks), 2)
	}

	for _, chunk := range chunks {
		if len(chunk) > discordMessageLimit {
			t.Fatalf("chunk length = %d, want <= %d", len(chunk), discordMessageLimit)
		}

		if strings.Count(chunk, "```")%2 != 0 {
			t.Fatalf("chunk has unbalanced code fence markers: %q", chunk)
		}
	}
}

func newTestRuntime(t *testing.T, mode config.Mode, fakeSession *fakeSession) *Runtime {
	t.Helper()
	return newTestRuntimeWithServicesAndClient(t, mode, fakeSession, &fakeMessageService{}, &fakeTaskCommandService{}, http.DefaultClient)
}

func newTestRuntimeWithServices(
	t *testing.T,
	mode config.Mode,
	fakeSession *fakeSession,
	messageService app.MessageService,
	taskService app.TaskCommandService,
) *Runtime {
	t.Helper()
	return newTestRuntimeWithServicesAndClient(t, mode, fakeSession, messageService, taskService, http.DefaultClient)
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

	runtime, err := NewRuntime(Dependencies{
		Config: config.Config{
			Mode:         mode,
			TimezoneName: "Asia/Tokyo",
			DiscordToken: "discord-token",
		},
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Message:     messageService,
		TaskCommand: taskService,
		HTTPClient:  httpClient,
		SessionFactory: func(token string) (session, error) {
			return fakeSession, nil
		},
	})
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}

	return runtime
}

func taskInteractionEvent(userID string, action string, taskName string, taskID string) *discordgo.InteractionCreate {
	options := []*discordgo.ApplicationCommandInteractionDataOption{}
	if action != "" {
		subcommand := &discordgo.ApplicationCommandInteractionDataOption{
			Name: action,
			Type: discordgo.ApplicationCommandOptionSubCommand,
		}
		switch action {
		case taskActionNew:
			subcommand.Options = []*discordgo.ApplicationCommandInteractionDataOption{
				{
					Name:  "name",
					Type:  discordgo.ApplicationCommandOptionString,
					Value: taskName,
				},
			}
		case taskActionSwitch, taskActionClose:
			subcommand.Options = []*discordgo.ApplicationCommandInteractionDataOption{
				{
					Name:  "id",
					Type:  discordgo.ApplicationCommandOptionString,
					Value: taskID,
				},
			}
		}
		options = append(options, subcommand)
	}

	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type: discordgo.InteractionApplicationCommand,
			Data: discordgo.ApplicationCommandInteractionData{
				Name:    commandTask,
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

type fakeSession struct {
	selfUserID string

	opened bool
	closed bool

	registeredAppID    string
	registeredGuildID  string
	registeredCommands []*discordgo.ApplicationCommand

	messageHandlers     []func(*discordgo.Session, *discordgo.MessageCreate)
	interactionHandlers []func(*discordgo.Session, *discordgo.InteractionCreate)

	sentMessages         []*discordgo.MessageSend
	interactionResponses []*discordgo.InteractionResponse
	interactionEdits     []*discordgo.WebhookEdit
	followups            []*discordgo.WebhookParams
}

func newFakeSession(selfUserID string) *fakeSession {
	return &fakeSession{selfUserID: selfUserID}
}

func (s *fakeSession) AddHandler(handler interface{}) func() {
	switch handler := handler.(type) {
	case func(*discordgo.Session, *discordgo.MessageCreate):
		s.messageHandlers = append(s.messageHandlers, handler)
	case func(*discordgo.Session, *discordgo.InteractionCreate):
		s.interactionHandlers = append(s.interactionHandlers, handler)
	}

	return func() {}
}

func (s *fakeSession) Open() error {
	s.opened = true
	return nil
}

func (s *fakeSession) Close() error {
	s.closed = true
	return nil
}

func (s *fakeSession) ApplicationCommandBulkOverwrite(
	appID string,
	guildID string,
	commands []*discordgo.ApplicationCommand,
	options ...discordgo.RequestOption,
) ([]*discordgo.ApplicationCommand, error) {
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
	s.sentMessages = append(s.sentMessages, data)
	return &discordgo.Message{ID: "sent-message", ChannelID: channelID}, nil
}

func (s *fakeSession) InteractionRespond(
	interaction *discordgo.Interaction,
	resp *discordgo.InteractionResponse,
	options ...discordgo.RequestOption,
) error {
	s.interactionResponses = append(s.interactionResponses, resp)
	return nil
}

func (s *fakeSession) InteractionResponseEdit(
	interaction *discordgo.Interaction,
	newresp *discordgo.WebhookEdit,
	options ...discordgo.RequestOption,
) (*discordgo.Message, error) {
	s.interactionEdits = append(s.interactionEdits, newresp)
	return &discordgo.Message{ID: "edited-message"}, nil
}

func (s *fakeSession) FollowupMessageCreate(
	interaction *discordgo.Interaction,
	wait bool,
	data *discordgo.WebhookParams,
	options ...discordgo.RequestOption,
) (*discordgo.Message, error) {
	s.followups = append(s.followups, data)
	return &discordgo.Message{ID: "followup-message"}, nil
}

func (s *fakeSession) SelfUserID() string {
	return s.selfUserID
}

func (s *fakeSession) dispatchMessage(event *discordgo.MessageCreate) {
	for _, handler := range s.messageHandlers {
		handler(nil, event)
	}
}

func (s *fakeSession) dispatchInteraction(event *discordgo.InteractionCreate) {
	for _, handler := range s.interactionHandlers {
		handler(nil, event)
	}
}

type fakeMessageService struct {
	requests []app.MessageRequest
	response app.MessageResponse
	err      error
	handle   func(ctx context.Context, request app.MessageRequest, sink app.DeferredReplySink) (app.MessageResponse, error)
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

type fakeTaskCommandService struct {
	createCalls []struct {
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
