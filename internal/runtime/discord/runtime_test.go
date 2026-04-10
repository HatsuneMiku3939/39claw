package discord

import (
	"bytes"
	"context"
	"encoding/json"
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

	if len(fakeSession.registeredCommands) != 1 {
		t.Fatalf("registered command count = %d, want %d", len(fakeSession.registeredCommands), 1)
	}

	command := fakeSession.registeredCommands[0]
	if command.Name != "release" {
		t.Fatalf("registered command name = %q, want %q", command.Name, "release")
	}

	if len(command.Options) != 1 {
		t.Fatalf("registered option count = %d, want %d", len(command.Options), 1)
	}

	actionOption := command.Options[0]
	if actionOption.Name != optionAction {
		t.Fatalf("action option name = %q, want %q", actionOption.Name, optionAction)
	}

	if len(actionOption.Choices) != 2 {
		t.Fatalf("action choice count = %d, want %d", len(actionOption.Choices), 2)
	}

	if actionOption.Choices[0].Value != actionHelp {
		t.Fatalf("action choice value = %v, want %q", actionOption.Choices[0].Value, actionHelp)
	}

	if actionOption.Choices[1].Value != actionClear {
		t.Fatalf("second action choice value = %v, want %q", actionOption.Choices[1].Value, actionClear)
	}
}

func TestRuntimeStartRegistersTaskModeChoices(t *testing.T) {
	t.Parallel()

	fakeSession := newFakeSession("bot-user")
	runtime := newTestRuntime(t, config.ModeTask, fakeSession)

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.Close()
	})

	command := fakeSession.registeredCommands[0]
	if len(command.Options) != 3 {
		t.Fatalf("registered option count = %d, want %d", len(command.Options), 3)
	}

	actionOption := command.Options[0]
	if len(actionOption.Choices) != 6 {
		t.Fatalf("action choice count = %d, want %d", len(actionOption.Choices), 6)
	}

	if command.Options[1].Name != optionTaskName {
		t.Fatalf("task name option = %q, want %q", command.Options[1].Name, optionTaskName)
	}

	if command.Options[2].Name != optionTaskID {
		t.Fatalf("task id option = %q, want %q", command.Options[2].Name, optionTaskID)
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

	if len(fakeSession.reactions) != 1 {
		t.Fatalf("reaction count = %d, want %d", len(fakeSession.reactions), 1)
	}

	if fakeSession.reactions[0].messageID != "sent-message-1" {
		t.Fatalf("reaction message id = %q, want %q", fakeSession.reactions[0].messageID, "sent-message-1")
	}

	if fakeSession.reactions[0].emoji != completionReactionEmoji {
		t.Fatalf("reaction emoji = %q, want %q", fakeSession.reactions[0].emoji, completionReactionEmoji)
	}
}

func TestRuntimeDirectMessageHandlingRepliesWithoutMention(t *testing.T) {
	t.Parallel()

	messageService := &fakeMessageService{
		response: app.MessageResponse{
			Text:      "Hello from DM",
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
			ChannelID: "dm-channel-1",
			Content:   "hello from dm",
			Author:    &discordgo.User{ID: "user-1"},
		},
	})

	if len(messageService.requests) != 1 {
		t.Fatalf("message request count = %d, want %d", len(messageService.requests), 1)
	}

	if messageService.requests[0].Content != "hello from dm" {
		t.Fatalf("mapped content = %q, want %q", messageService.requests[0].Content, "hello from dm")
	}

	if !messageService.requests[0].Mentioned {
		t.Fatal("Mentioned = false, want true")
	}

	if len(fakeSession.sentMessages) != 1 {
		t.Fatalf("sent message count = %d, want %d", len(fakeSession.sentMessages), 1)
	}

	if fakeSession.sentMessages[0].Reference == nil || fakeSession.sentMessages[0].Reference.MessageID != "message-1" {
		t.Fatal("first sent message missing reply reference")
	}
}

func TestRuntimeMentionHandlingStreamsProgressByEditingReply(t *testing.T) {
	t.Parallel()

	messageService := &fakeMessageService{
		handle: func(ctx context.Context, request app.MessageRequest, sink app.DeferredReplySink) (app.MessageResponse, error) {
			if request.ProgressSink == nil {
				t.Fatal("ProgressSink = nil, want non-nil")
			}

			if err := request.ProgressSink.Deliver(ctx, app.MessageProgress{Text: "Thinking..."}); err != nil {
				return app.MessageResponse{}, err
			}

			if err := request.ProgressSink.Deliver(ctx, app.MessageProgress{Text: "Partial streamed response"}); err != nil {
				return app.MessageResponse{}, err
			}

			if request.Cleanup != nil {
				request.Cleanup()
			}

			return app.MessageResponse{
				Text:      "Final streamed response",
				ReplyToID: "message-1",
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
			Content:   "<@bot-user> hello there",
			Author:    &discordgo.User{ID: "user-1"},
			Mentions: []*discordgo.User{
				{ID: "bot-user"},
			},
		},
	})

	if len(fakeSession.sentMessages) != 1 {
		t.Fatalf("sent message count = %d, want %d", len(fakeSession.sentMessages), 1)
	}

	if fakeSession.sentMessages[0].Content != "Thinking..." {
		t.Fatalf("initial content = %q, want %q", fakeSession.sentMessages[0].Content, "Thinking...")
	}

	if len(fakeSession.editedMessages) != 2 {
		t.Fatalf("edited message count = %d, want %d", len(fakeSession.editedMessages), 2)
	}

	if got := stringPointerValue(fakeSession.editedMessages[0].Content); got != "Partial streamed response" {
		t.Fatalf("first edit content = %q, want %q", got, "Partial streamed response")
	}

	if got := stringPointerValue(fakeSession.editedMessages[1].Content); got != "Final streamed response" {
		t.Fatalf("second edit content = %q, want %q", got, "Final streamed response")
	}

	if len(fakeSession.reactions) != 1 {
		t.Fatalf("reaction count = %d, want %d", len(fakeSession.reactions), 1)
	}

	if fakeSession.reactions[0].messageID != "sent-message-1" {
		t.Fatalf("reaction message id = %q, want %q", fakeSession.reactions[0].messageID, "sent-message-1")
	}
}

func TestRuntimeMentionHandlingSanitizesWorkspacePathsForDiscord(t *testing.T) {
	t.Parallel()

	messageService := &fakeMessageService{
		response: app.MessageResponse{
			Text: "[디펜더봇 평가-머지-릴리즈 자동화](" +
				"/home/filepang/Documents/filepang/1%20Project/direnv-action/%EB%94%94%ED%8E%9C%EB%8D%94%EB%B4%87%20%ED%8F%89%EA%B0%80-%EB%A8%B8%EC%A7%80-%EB%A6%B4%EB%A6%AC%EC%A6%88%20%EC%9E%90%EB%8F%99%ED%99%94.md" +
				")",
			ReplyToID: "message-1",
		},
	}
	fakeSession := newFakeSession("bot-user")
	runtime := newTestRuntimeWithServices(t, config.ModeDaily, fakeSession, messageService, &fakeTaskCommandService{})
	runtime.config.CodexWorkdir = "/home/filepang/Documents/filepang"

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
			Content:   "<@bot-user> sanitize this",
			Author:    &discordgo.User{ID: "user-1"},
			Mentions: []*discordgo.User{
				{ID: "bot-user"},
			},
		},
	})

	if len(fakeSession.sentMessages) != 1 {
		t.Fatalf("sent message count = %d, want %d", len(fakeSession.sentMessages), 1)
	}

	want := "디펜더봇 평가-머지-릴리즈 자동화 (`workspace/1 Project/direnv-action/디펜더봇 평가-머지-릴리즈 자동화.md`)"
	if fakeSession.sentMessages[0].Content != want {
		t.Fatalf("sent content = %q, want %q", fakeSession.sentMessages[0].Content, want)
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
				Deferred:  true,
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

	if len(fakeSession.reactions) != 0 {
		t.Fatalf("reaction count after ack = %d, want %d", len(fakeSession.reactions), 0)
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

	if len(fakeSession.reactions) != 1 {
		t.Fatalf("reaction count after deferred delivery = %d, want %d", len(fakeSession.reactions), 1)
	}

	if fakeSession.reactions[0].messageID != "sent-message-2" {
		t.Fatalf("reaction message id = %q, want %q", fakeSession.reactions[0].messageID, "sent-message-2")
	}
}

func TestRuntimeDeferredReplyDeliveryLogsStructuredSuccess(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))

	delivered := make(chan struct{})
	messageService := &fakeMessageService{
		handle: func(ctx context.Context, request app.MessageRequest, sink app.DeferredReplySink) (app.MessageResponse, error) {
			go func() {
				_ = sink.Deliver(context.Background(), app.MessageResponse{
					Text:      "Final queued response",
					ReplyToID: request.MessageID,
				})
				close(delivered)
			}()

			return app.MessageResponse{
				Text:      "A response is already running for this conversation. Your message has been queued at position 1.",
				ReplyToID: request.MessageID,
				Deferred:  true,
			}, nil
		},
	}

	fakeSession := newFakeSession("bot-user")
	runtime, err := NewRuntime(Dependencies{
		Config: config.Config{
			Mode:               config.ModeDaily,
			TimezoneName:       "Asia/Tokyo",
			DiscordToken:       "discord-token",
			DiscordCommandName: "release",
		},
		Logger:       logger,
		Message:      messageService,
		DailyCommand: &fakeDailyCommandService{},
		TaskCommand:  &fakeTaskCommandService{},
		HTTPClient:   http.DefaultClient,
		SessionFactory: func(token string) (session, error) {
			return fakeSession, nil
		},
	})
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}

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

	select {
	case <-delivered:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for deferred runtime delivery")
	}

	entries := parseRuntimeJSONLogEntries(t, logBuffer.Bytes())
	deferredReply := findRuntimeLogEntry(t, entries, "deferred_reply_delivery")
	if deferredReply["outcome"] != "success" {
		t.Fatalf("outcome = %v, want %q", deferredReply["outcome"], "success")
	}

	if deferredReply["channel_id"] != "channel-1" {
		t.Fatalf("channel_id = %v, want %q", deferredReply["channel_id"], "channel-1")
	}

	if deferredReply["message_id"] != "message-1" {
		t.Fatalf("message_id = %v, want %q", deferredReply["message_id"], "message-1")
	}
}

func TestRuntimeCloseWaitsForDeferredQueuedReplyDrain(t *testing.T) {
	t.Parallel()

	releaseDeferred := make(chan struct{})
	drainDone := make(chan struct{})
	messageService := &fakeMessageService{
		handle: func(ctx context.Context, request app.MessageRequest, sink app.DeferredReplySink) (app.MessageResponse, error) {
			go func() {
				defer close(drainDone)
				<-releaseDeferred
				_ = sink.Deliver(ctx, app.MessageResponse{
					Text:      "Final queued response",
					ReplyToID: request.MessageID,
				})
			}()

			return app.MessageResponse{
				Text:      "A response is already running for this conversation. Your message has been queued at position 1.",
				ReplyToID: request.MessageID,
				Deferred:  true,
			}, nil
		},
		waitForDrain: func(ctx context.Context) error {
			select {
			case <-drainDone:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}

	fakeSession := newFakeSession("bot-user")
	runtime := newTestRuntimeWithTimeout(
		t,
		config.ModeDaily,
		fakeSession,
		messageService,
		&fakeDailyCommandService{},
		&fakeTaskCommandService{},
		http.DefaultClient,
		time.Second,
	)

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

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

	closeDone := make(chan error, 1)
	go func() {
		closeDone <- runtime.Close()
	}()

	select {
	case err := <-closeDone:
		t.Fatalf("Close() returned early with err = %v", err)
	case <-time.After(25 * time.Millisecond):
	}

	close(releaseDeferred)

	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for runtime close")
	}

	if len(fakeSession.sentMessages) != 2 {
		t.Fatalf("sent message count = %d, want %d", len(fakeSession.sentMessages), 2)
	}

	if fakeSession.sentMessages[1].Content != "Final queued response" {
		t.Fatalf("deferred content = %q, want %q", fakeSession.sentMessages[1].Content, "Final queued response")
	}

	if !fakeSession.closed {
		t.Fatal("session closed = false, want true")
	}
}

func TestRuntimeCloseCancelsDeferredDrainWhenTimeoutExpires(t *testing.T) {
	t.Parallel()

	drainDone := make(chan struct{})
	canceled := make(chan struct{})
	messageService := &fakeMessageService{
		handle: func(ctx context.Context, request app.MessageRequest, sink app.DeferredReplySink) (app.MessageResponse, error) {
			go func() {
				defer close(drainDone)
				<-ctx.Done()
				close(canceled)
			}()

			return app.MessageResponse{
				Text:      "A response is already running for this conversation. Your message has been queued at position 1.",
				ReplyToID: request.MessageID,
				Deferred:  true,
			}, nil
		},
		waitForDrain: func(ctx context.Context) error {
			select {
			case <-drainDone:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}

	fakeSession := newFakeSession("bot-user")
	runtime := newTestRuntimeWithTimeout(
		t,
		config.ModeDaily,
		fakeSession,
		messageService,
		&fakeDailyCommandService{},
		&fakeTaskCommandService{},
		http.DefaultClient,
		20*time.Millisecond,
	)

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

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

	if err := runtime.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for runtime cancellation")
	}

	if !fakeSession.closed {
		t.Fatal("session closed = false, want true")
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
			GuildID:   "guild-1",
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

func TestRuntimeHelpActionReturnsConfiguredCommandInfo(t *testing.T) {
	t.Parallel()

	fakeSession := newFakeSession("bot-user")
	runtime := newTestRuntime(t, config.ModeDaily, fakeSession)

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.Close()
	})

	fakeSession.dispatchInteraction(commandInteractionEvent("release", "user-1", actionHelp, "", ""))

	if len(fakeSession.interactionResponses) != 1 {
		t.Fatalf("interaction response count = %d, want %d", len(fakeSession.interactionResponses), 1)
	}

	response := fakeSession.interactionResponses[0]
	if response.Data == nil {
		t.Fatal("response data = nil, want non-nil")
	}

	if !strings.Contains(response.Data.Content, "Command: /release") {
		t.Fatalf("response content = %q, want configured command name", response.Data.Content)
	}

	if !strings.Contains(response.Data.Content, "Mode: daily") {
		t.Fatalf("response content = %q, want mode guidance", response.Data.Content)
	}

	if !strings.Contains(response.Data.Content, "action:clear") {
		t.Fatalf("response content = %q, want clear action guidance", response.Data.Content)
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

	fakeSession.dispatchInteraction(commandInteractionEvent("release", "user-1", actionTaskCurrent, "", ""))

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

func TestRuntimeDailyClearActionRoutesIdleSuccess(t *testing.T) {
	t.Parallel()

	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	store := &memoryThreadStore{}
	dailyService, err := app.NewDailyCommandService(app.DailyCommandServiceDependencies{
		CommandName: "release",
		Timezone:    tokyo,
		Store:       store,
		Coordinator: &stubQueueCoordinator{},
	})
	if err != nil {
		t.Fatalf("NewDailyCommandService() error = %v", err)
	}

	fakeSession := newFakeSession("bot-user")
	runtime := newTestRuntimeWithTimeout(
		t,
		config.ModeDaily,
		fakeSession,
		&fakeMessageService{},
		dailyService,
		&fakeTaskCommandService{},
		http.DefaultClient,
		0,
	)

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.Close()
	})

	fakeSession.dispatchInteraction(commandInteractionEvent("release", "user-1", actionClear, "", ""))

	if len(fakeSession.interactionResponses) != 1 {
		t.Fatalf("interaction response count = %d, want %d", len(fakeSession.interactionResponses), 1)
	}

	response := fakeSession.interactionResponses[0]
	if response.Data == nil || response.Data.Flags != discordgo.MessageFlagsEphemeral {
		t.Fatalf("response flags = %v, want ephemeral", response.Data.Flags)
	}

	if !strings.Contains(response.Data.Content, "#2") {
		t.Fatalf("response content = %q, want rotated generation confirmation", response.Data.Content)
	}
}

func TestRuntimeDailyClearActionRejectsBusyGenerationEphemerally(t *testing.T) {
	t.Parallel()

	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	store := &memoryThreadStore{}
	localDate := time.Now().In(tokyo).Format(time.DateOnly)
	if _, err := store.CreateDailySession(context.Background(), app.DailySession{
		LocalDate:        localDate,
		Generation:       1,
		LogicalThreadKey: localDate + "#1",
		ActivationReason: app.DailySessionActivationAutomatic,
		IsActive:         true,
	}); err != nil {
		t.Fatalf("CreateDailySession() error = %v", err)
	}

	dailyService, err := app.NewDailyCommandService(app.DailyCommandServiceDependencies{
		CommandName: "release",
		Timezone:    tokyo,
		Store:       store,
		Coordinator: &stubQueueCoordinator{snapshot: app.QueueSnapshot{InFlight: true, Queued: 1}},
	})
	if err != nil {
		t.Fatalf("NewDailyCommandService() error = %v", err)
	}

	fakeSession := newFakeSession("bot-user")
	runtime := newTestRuntimeWithTimeout(
		t,
		config.ModeDaily,
		fakeSession,
		&fakeMessageService{},
		dailyService,
		&fakeTaskCommandService{},
		http.DefaultClient,
		0,
	)

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.Close()
	})

	fakeSession.dispatchInteraction(commandInteractionEvent("release", "user-1", actionClear, "", ""))

	if len(fakeSession.interactionResponses) != 1 {
		t.Fatalf("interaction response count = %d, want %d", len(fakeSession.interactionResponses), 1)
	}

	response := fakeSession.interactionResponses[0]
	if response.Data == nil || response.Data.Flags != discordgo.MessageFlagsEphemeral {
		t.Fatalf("response flags = %v, want ephemeral", response.Data.Flags)
	}

	if !strings.Contains(response.Data.Content, "still busy") {
		t.Fatalf("response content = %q, want busy rejection", response.Data.Content)
	}
}

func TestRuntimeTaskCommandRoutesTaskModeActions(t *testing.T) {
	t.Parallel()

	taskService := &fakeTaskCommandService{
		currentResponse: app.MessageResponse{
			Text:      "Current task",
			Ephemeral: true,
		},
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

	fakeSession.dispatchInteraction(commandInteractionEvent("release", "user-1", actionTaskCurrent, "", ""))
	fakeSession.dispatchInteraction(commandInteractionEvent("release", "user-1", actionTaskNew, "Release work", ""))

	if len(taskService.currentCalls) != 1 {
		t.Fatalf("current call count = %d, want %d", len(taskService.currentCalls), 1)
	}

	if len(taskService.createCalls) != 1 {
		t.Fatalf("create call count = %d, want %d", len(taskService.createCalls), 1)
	}

	if taskService.createCalls[0].taskName != "Release work" {
		t.Fatalf("create task name = %q, want %q", taskService.createCalls[0].taskName, "Release work")
	}

	if len(fakeSession.interactionResponses) != 2 {
		t.Fatalf("interaction response count = %d, want %d", len(fakeSession.interactionResponses), 2)
	}

	response := fakeSession.interactionResponses[1]
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

func parseRuntimeJSONLogEntries(t *testing.T, output []byte) []map[string]any {
	t.Helper()

	lines := bytes.Split(bytes.TrimSpace(output), []byte("\n"))
	entries := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		entry := map[string]any{}
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}

		entries = append(entries, entry)
	}

	return entries
}

func findRuntimeLogEntry(t *testing.T, entries []map[string]any, event string) map[string]any {
	t.Helper()

	for _, entry := range entries {
		if entry["event"] == event {
			return entry
		}
	}

	t.Fatalf("log event %q not found in %v", event, entries)
	return nil
}
