package discord

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	"github.com/HatsuneMiku3939/39claw/internal/config"
	"github.com/HatsuneMiku3939/39claw/internal/testutil/runtimeharness"
	"github.com/HatsuneMiku3939/39claw/internal/thread"
)

func TestRuntimeContractDailyMentionReply(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{}
	gateway := &fakeCodexGateway{
		results: []app.RunTurnResult{
			{ThreadID: "thread-1", ResponseText: "Hello from runtime"},
		},
	}

	fakeSession := newFakeSession("bot-user")
	runtime := newTestRuntimeWithServices(
		t,
		config.ModeDaily,
		fakeSession,
		newContractDailyMessageService(t, store, gateway, nil),
		&fakeTaskCommandService{},
	)

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.Close()
	})

	fakeSession.dispatchHarnessMessage(runtimeharness.MessageEvent{
		UserID:    "user-1",
		ChannelID: "channel-1",
		MessageID: "message-1",
		Content:   "hello there",
		Mentioned: true,
	})

	runtimeharness.RequireDeliveries(t, fakeSession.Deliveries(),
		runtimeharness.Expectation{
			Kind:      runtimeharness.DeliveryKindChannelMessage,
			ChannelID: "channel-1",
			ReplyToID: "message-1",
			Text:      "Thinking...",
		},
		runtimeharness.Expectation{
			Kind:      runtimeharness.DeliveryKindChannelEdit,
			ChannelID: "channel-1",
			MessageID: "sent-message-1",
			Text:      "Hello from runtime",
		},
	)
}

func TestRuntimeContractStreamsImmediateReplyEdits(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{}
	gateway := &progressCodexGateway{
		progress: []string{"Thinking...", "Partial streamed response"},
		result: app.RunTurnResult{
			ThreadID:     "thread-1",
			ResponseText: "Final streamed response",
		},
	}

	fakeSession := newFakeSession("bot-user")
	runtime := newTestRuntimeWithServices(
		t,
		config.ModeDaily,
		fakeSession,
		newContractDailyMessageService(t, store, gateway, nil),
		&fakeTaskCommandService{},
	)

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.Close()
	})

	fakeSession.dispatchHarnessMessage(runtimeharness.MessageEvent{
		UserID:    "user-1",
		ChannelID: "channel-1",
		MessageID: "message-1",
		Content:   "stream me",
		Mentioned: true,
	})

	runtimeharness.RequireDeliveries(t, fakeSession.Deliveries(),
		runtimeharness.Expectation{
			Kind:      runtimeharness.DeliveryKindChannelMessage,
			ChannelID: "channel-1",
			MessageID: "sent-message-1",
			ReplyToID: "message-1",
			Text:      "Thinking...",
		},
		runtimeharness.Expectation{
			Kind:      runtimeharness.DeliveryKindChannelEdit,
			ChannelID: "channel-1",
			MessageID: "sent-message-1",
			Text:      "Partial streamed response",
		},
		runtimeharness.Expectation{
			Kind:      runtimeharness.DeliveryKindChannelEdit,
			ChannelID: "channel-1",
			MessageID: "sent-message-1",
			Text:      "Final streamed response",
		},
	)
}

func TestRuntimeContractQueuedAcknowledgementAndDeferredReply(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{}
	gateway := newBlockingCodexGateway(
		app.RunTurnResult{ThreadID: "thread-1", ResponseText: "First response"},
		app.RunTurnResult{ThreadID: "thread-1", ResponseText: "Second response"},
	)

	fakeSession := newFakeSession("bot-user")
	runtime := newTestRuntimeWithServices(
		t,
		config.ModeDaily,
		fakeSession,
		newContractDailyMessageService(t, store, gateway, thread.NewQueueCoordinator()),
		&fakeTaskCommandService{},
	)

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.Close()
	})

	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		fakeSession.dispatchHarnessMessage(runtimeharness.MessageEvent{
			UserID:    "user-1",
			ChannelID: "channel-1",
			MessageID: "message-1",
			Content:   "start long turn",
			Mentioned: true,
		})
	}()

	waitForSignal(t, gateway.started, "first queued contract turn start")

	fakeSession.dispatchHarnessMessage(runtimeharness.MessageEvent{
		UserID:    "user-1",
		ChannelID: "channel-1",
		MessageID: "message-2",
		Content:   "follow up later",
		Mentioned: true,
	})

	runtimeharness.RequireDeliveries(t, fakeSession.Deliveries(),
		runtimeharness.Expectation{
			Kind:      runtimeharness.DeliveryKindChannelMessage,
			ChannelID: "channel-1",
			MessageID: "sent-message-1",
			ReplyToID: "message-1",
			Text:      "Thinking...",
		},
		runtimeharness.Expectation{
			Kind:      runtimeharness.DeliveryKindChannelMessage,
			ChannelID: "channel-1",
			MessageID: "sent-message-2",
			ReplyToID: "message-2",
			Text:      "A response is already running for this conversation. Your message has been queued at position 1.",
		},
	)

	close(gateway.release)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	deliveries, err := fakeSession.waitForDeliveryCount(ctx, 4)
	if err != nil {
		t.Fatalf("waitForDeliveryCount() error = %v", err)
	}

	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first queued contract turn to finish")
	}

	runtimeharness.RequireDeliveries(t, deliveries,
		runtimeharness.Expectation{
			Kind:      runtimeharness.DeliveryKindChannelMessage,
			ChannelID: "channel-1",
			MessageID: "sent-message-1",
			ReplyToID: "message-1",
			Text:      "Thinking...",
		},
		runtimeharness.Expectation{
			Kind:      runtimeharness.DeliveryKindChannelMessage,
			ChannelID: "channel-1",
			MessageID: "sent-message-2",
			ReplyToID: "message-2",
			Text:      "A response is already running for this conversation. Your message has been queued at position 1.",
		},
		runtimeharness.Expectation{
			Kind:      runtimeharness.DeliveryKindChannelEdit,
			ChannelID: "channel-1",
			MessageID: "sent-message-1",
			Text:      "First response",
		},
		runtimeharness.Expectation{
			Kind:      runtimeharness.DeliveryKindChannelMessage,
			ChannelID: "channel-1",
			MessageID: "sent-message-3",
			ReplyToID: "message-2",
			Text:      "Second response",
		},
	)
}

func TestRuntimeContractTaskCommandUsesRealServiceAndEphemeralResponse(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{}
	taskService := newContractTaskCommandService(t, store, func() string { return "task-1" })

	fakeSession := newFakeSession("bot-user")
	runtime := newTestRuntimeWithServices(
		t,
		config.ModeTask,
		fakeSession,
		&fakeMessageService{},
		taskService,
	)

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.Close()
	})

	fakeSession.dispatchHarnessCommand(runtimeharness.CommandIntent{
		CommandName: "release",
		UserID:      "user-1",
		Action:      actionTaskNew,
		TaskName:    "Release work",
	})

	runtimeharness.RequireDeliveries(t, fakeSession.Deliveries(),
		runtimeharness.Expectation{
			Kind:      runtimeharness.DeliveryKindInteractionResponse,
			Text:      "Created task `Release work` (`task-1`) and made it active. Your next message will continue this task.",
			Ephemeral: runtimeharness.Bool(true),
		},
	)

	activeTask, ok, err := store.GetActiveTask(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetActiveTask() error = %v", err)
	}

	if !ok {
		t.Fatal("GetActiveTask() ok = false, want true")
	}

	if activeTask.TaskID != "task-1" {
		t.Fatalf("active task id = %q, want %q", activeTask.TaskID, "task-1")
	}
}

func TestRuntimeContractAttachmentAwareMessageUsesDownloadedImagePaths(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("png-bytes"))
	}))
	t.Cleanup(server.Close)

	store := &memoryThreadStore{}
	gateway := &fakeCodexGateway{
		results: []app.RunTurnResult{
			{ThreadID: "thread-1", ResponseText: "Image-aware response"},
		},
	}

	fakeSession := newFakeSession("bot-user")
	runtime := newTestRuntimeWithServicesAndClient(
		t,
		config.ModeDaily,
		fakeSession,
		newContractDailyMessageService(t, store, gateway, nil),
		&fakeTaskCommandService{},
		server.Client(),
	)

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.Close()
	})

	fakeSession.dispatchHarnessMessage(runtimeharness.MessageEvent{
		UserID:    "user-1",
		ChannelID: "channel-1",
		MessageID: "message-1",
		Content:   "describe this",
		Mentioned: true,
		Attachments: []runtimeharness.Attachment{
			{
				Filename:    "sample.png",
				ContentType: "image/png",
				URL:         server.URL + "/sample.png",
			},
		},
	})

	runtimeharness.RequireDeliveries(t, fakeSession.Deliveries(),
		runtimeharness.Expectation{
			Kind:      runtimeharness.DeliveryKindChannelMessage,
			ChannelID: "channel-1",
			ReplyToID: "message-1",
			Text:      "Thinking...",
		},
		runtimeharness.Expectation{
			Kind:      runtimeharness.DeliveryKindChannelEdit,
			ChannelID: "channel-1",
			MessageID: "sent-message-1",
			Text:      "Image-aware response",
		},
	)

	calls := gateway.Calls()
	if len(calls) != 1 {
		t.Fatalf("RunTurn() call count = %d, want %d", len(calls), 1)
	}

	if len(calls[0].input.ImagePaths) != 1 {
		t.Fatalf("image path count = %d, want %d", len(calls[0].input.ImagePaths), 1)
	}

	if _, err := os.Stat(calls[0].input.ImagePaths[0]); !os.IsNotExist(err) {
		t.Fatalf("downloaded image path should be cleaned up, stat err = %v", err)
	}
}
