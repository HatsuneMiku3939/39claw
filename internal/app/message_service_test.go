package app_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
	"github.com/HatsuneMiku3939/39claw/internal/config"
	"github.com/HatsuneMiku3939/39claw/internal/thread"
)

func TestMessageServiceHandleMessageIgnoresNonMentionChatter(t *testing.T) {
	t.Parallel()

	service := newDailyMessageService(t, &memoryThreadStore{}, &fakeCodexGateway{}, nil)

	response, err := service.HandleMessage(context.Background(), app.MessageRequest{
		MessageID:  "message-1",
		Content:    "just chatting",
		Mentioned:  false,
		ReceivedAt: time.Date(2026, time.April, 5, 9, 0, 0, 0, time.UTC),
	}, nil)
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}

	if !response.Ignore {
		t.Fatal("Ignore = false, want true")
	}
}

func TestMessageServiceHandleMessageDailyReusesSameDayBinding(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{}
	gateway := &fakeCodexGateway{
		results: []app.RunTurnResult{
			{ThreadID: "thread-1", ResponseText: "First response"},
			{ThreadID: "thread-1", ResponseText: "Second response"},
		},
	}
	service := newDailyMessageService(t, store, gateway, nil)

	firstResponse, err := service.HandleMessage(context.Background(), app.MessageRequest{
		MessageID:  "message-1",
		Content:    "hello there",
		Mentioned:  true,
		ReceivedAt: time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
	}, nil)
	if err != nil {
		t.Fatalf("HandleMessage() first error = %v", err)
	}

	secondResponse, err := service.HandleMessage(context.Background(), app.MessageRequest{
		MessageID:  "message-2",
		Content:    "follow up",
		Mentioned:  true,
		ReceivedAt: time.Date(2026, time.April, 5, 7, 0, 0, 0, time.UTC),
	}, nil)
	if err != nil {
		t.Fatalf("HandleMessage() second error = %v", err)
	}

	if firstResponse.Text != "First response" {
		t.Fatalf("first response text = %q, want %q", firstResponse.Text, "First response")
	}

	if secondResponse.Text != "Second response" {
		t.Fatalf("second response text = %q, want %q", secondResponse.Text, "Second response")
	}

	if secondResponse.ReplyToID != "message-2" {
		t.Fatalf("second reply id = %q, want %q", secondResponse.ReplyToID, "message-2")
	}

	calls := gateway.Calls()
	if len(calls) != 2 {
		t.Fatalf("RunTurn() call count = %d, want %d", len(calls), 2)
	}

	if calls[0].threadID != "" {
		t.Fatalf("first thread id = %q, want empty", calls[0].threadID)
	}

	if calls[1].threadID != "thread-1" {
		t.Fatalf("second thread id = %q, want %q", calls[1].threadID, "thread-1")
	}

	binding, ok, err := store.GetThreadBinding(context.Background(), "daily", "2026-04-05")
	if err != nil {
		t.Fatalf("GetThreadBinding() error = %v", err)
	}

	if !ok {
		t.Fatal("GetThreadBinding() ok = false, want true")
	}

	if binding.CodexThreadID != "thread-1" {
		t.Fatalf("CodexThreadID = %q, want %q", binding.CodexThreadID, "thread-1")
	}

	if calls[0].input.Prompt != "hello there" {
		t.Fatalf("first prompt = %q, want %q", calls[0].input.Prompt, "hello there")
	}

	if len(calls[0].input.ImagePaths) != 0 {
		t.Fatalf("first image path count = %d, want %d", len(calls[0].input.ImagePaths), 0)
	}
}

func TestMessageServiceHandleMessageDailyRollsOverOnNextDay(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{}
	gateway := &fakeCodexGateway{
		results: []app.RunTurnResult{
			{ThreadID: "thread-1", ResponseText: "Today"},
			{ThreadID: "thread-2", ResponseText: "Tomorrow"},
		},
	}
	service := newDailyMessageService(t, store, gateway, nil)

	_, err := service.HandleMessage(context.Background(), app.MessageRequest{
		MessageID:  "message-1",
		Content:    "today",
		Mentioned:  true,
		ReceivedAt: time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
	}, nil)
	if err != nil {
		t.Fatalf("HandleMessage() first error = %v", err)
	}

	_, err = service.HandleMessage(context.Background(), app.MessageRequest{
		MessageID:  "message-2",
		Content:    "tomorrow",
		Mentioned:  true,
		ReceivedAt: time.Date(2026, time.April, 5, 15, 1, 0, 0, time.UTC),
	}, nil)
	if err != nil {
		t.Fatalf("HandleMessage() second error = %v", err)
	}

	calls := gateway.Calls()
	if len(calls) != 2 {
		t.Fatalf("RunTurn() call count = %d, want %d", len(calls), 2)
	}

	if calls[0].threadID != "" {
		t.Fatalf("first thread id = %q, want empty", calls[0].threadID)
	}

	if calls[1].threadID != "" {
		t.Fatalf("second thread id = %q, want empty", calls[1].threadID)
	}

	if _, ok, err := store.GetThreadBinding(context.Background(), "daily", "2026-04-05"); err != nil || !ok {
		t.Fatalf("same-day binding lookup = ok:%v err:%v, want ok:true err:nil", ok, err)
	}

	nextBinding, ok, err := store.GetThreadBinding(context.Background(), "daily", "2026-04-06")
	if err != nil {
		t.Fatalf("GetThreadBinding() next day error = %v", err)
	}

	if !ok {
		t.Fatal("GetThreadBinding() next day ok = false, want true")
	}

	if nextBinding.CodexThreadID != "thread-2" {
		t.Fatalf("next day thread id = %q, want %q", nextBinding.CodexThreadID, "thread-2")
	}
}

func TestMessageServiceHandleMessageDailyRefreshesBeforeVisibleTurn(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{
		bindings: map[string]app.ThreadBinding{
			"daily:2026-04-05": {
				Mode:             "daily",
				LogicalThreadKey: "2026-04-05",
				CodexThreadID:    "thread-previous",
			},
		},
	}

	sequence := make([]string, 0, 2)
	refresher := &fakeDailyMemoryRefresher{sequence: &sequence}
	gateway := &fakeCodexGateway{
		sequence: &sequence,
		results: []app.RunTurnResult{
			{ThreadID: "thread-new", ResponseText: "Fresh day"},
		},
	}

	service := newDailyMessageServiceWithRefresher(t, store, gateway, refresher, nil)

	response, err := service.HandleMessage(context.Background(), app.MessageRequest{
		MessageID:  "message-1",
		Content:    "hello again",
		Mentioned:  true,
		ReceivedAt: time.Date(2026, time.April, 5, 15, 1, 0, 0, time.UTC),
	}, nil)
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}

	if response.Text != "Fresh day" {
		t.Fatalf("response text = %q, want %q", response.Text, "Fresh day")
	}

	if got, want := sequence, []string{"refresh", "turn"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("sequence = %v, want %v", got, want)
	}

	calls := refresher.Calls()
	if len(calls) != 1 {
		t.Fatalf("RefreshBeforeFirstDailyTurn() call count = %d, want 1", len(calls))
	}

	if calls[0].logicalKey != "2026-04-06" {
		t.Fatalf("refresher logical key = %q, want %q", calls[0].logicalKey, "2026-04-06")
	}
}

func TestMessageServiceHandleMessageDailyContinuesWhenRefreshFails(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{}
	sequence := make([]string, 0, 2)
	refresher := &fakeDailyMemoryRefresher{
		err:      errors.New("refresh failed"),
		sequence: &sequence,
	}
	gateway := &fakeCodexGateway{
		sequence: &sequence,
		results: []app.RunTurnResult{
			{ThreadID: "thread-1", ResponseText: "Visible response"},
		},
	}

	service := newDailyMessageServiceWithRefresher(t, store, gateway, refresher, nil)

	response, err := service.HandleMessage(context.Background(), app.MessageRequest{
		MessageID:  "message-1",
		Content:    "still answer me",
		Mentioned:  true,
		ReceivedAt: time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
	}, nil)
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}

	if response.Text != "Visible response" {
		t.Fatalf("response text = %q, want %q", response.Text, "Visible response")
	}

	if got, want := sequence, []string{"refresh", "turn"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("sequence = %v, want %v", got, want)
	}
}

func TestMessageServiceHandleMessageReturnsTaskGuidance(t *testing.T) {
	t.Parallel()

	service, err := app.NewMessageService(app.MessageServiceDependencies{
		Mode:        config.ModeTask,
		CommandName: "release",
		Policy: stubThreadPolicy{
			err: app.ErrNoActiveTask,
		},
		Store:            &memoryThreadStore{},
		WorkspaceManager: &fakeTaskWorkspaceManager{},
		Gateway:          &fakeCodexGateway{},
		Coordinator:      thread.NewQueueCoordinator(),
	})
	if err != nil {
		t.Fatalf("NewMessageService() error = %v", err)
	}

	response, err := service.HandleMessage(context.Background(), app.MessageRequest{
		MessageID: "message-1",
		Content:   "do the work",
		Mentioned: true,
	}, nil)
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}

	if response.Text != "No active task is selected. Use `/release action:task-new task_name:<name>`, `/release action:task-list`, or `/release action:task-switch task_id:<id>` first." {
		t.Fatalf("guidance text = %q", response.Text)
	}
}

func TestMessageServiceHandleMessageTaskReusesTaskBindingAcrossDays(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{
		tasks: map[string]app.Task{
			"user-1:task-1": {
				TaskID:        "task-1",
				DiscordUserID: "user-1",
				TaskName:      "Release work",
				Status:        app.TaskStatusOpen,
				CreatedAt:     time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
			},
		},
		activeTasks: map[string]app.ActiveTask{
			"user-1": {
				DiscordUserID: "user-1",
				TaskID:        "task-1",
			},
		},
	}
	gateway := &fakeCodexGateway{
		results: []app.RunTurnResult{
			{ThreadID: "thread-task-1", ResponseText: "First response"},
			{ThreadID: "thread-task-1", ResponseText: "Second response"},
		},
	}
	service := newTaskMessageService(t, store, gateway, nil)

	for _, request := range []app.MessageRequest{
		{
			UserID:     "user-1",
			MessageID:  "message-1",
			Content:    "start release",
			Mentioned:  true,
			ReceivedAt: time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
		},
		{
			UserID:     "user-1",
			MessageID:  "message-2",
			Content:    "continue release",
			Mentioned:  true,
			ReceivedAt: time.Date(2026, time.April, 7, 0, 0, 0, 0, time.UTC),
		},
	} {
		if _, err := service.HandleMessage(context.Background(), request, nil); err != nil {
			t.Fatalf("HandleMessage(%s) error = %v", request.MessageID, err)
		}
	}

	calls := gateway.Calls()
	if len(calls) != 2 {
		t.Fatalf("RunTurn() call count = %d, want %d", len(calls), 2)
	}

	if calls[0].threadID != "" {
		t.Fatalf("first thread id = %q, want empty", calls[0].threadID)
	}

	if calls[1].threadID != "thread-task-1" {
		t.Fatalf("second thread id = %q, want %q", calls[1].threadID, "thread-task-1")
	}

	if calls[0].workingDirectory != "/tmp/worktrees/task-1" {
		t.Fatalf("first working directory = %q, want %q", calls[0].workingDirectory, "/tmp/worktrees/task-1")
	}

	if calls[1].workingDirectory != "/tmp/worktrees/task-1" {
		t.Fatalf("second working directory = %q, want %q", calls[1].workingDirectory, "/tmp/worktrees/task-1")
	}

	binding, ok, err := store.GetThreadBinding(context.Background(), "task", "user-1:task-1")
	if err != nil {
		t.Fatalf("GetThreadBinding() error = %v", err)
	}

	if !ok {
		t.Fatal("GetThreadBinding() ok = false, want true")
	}

	if binding.TaskID != "task-1" {
		t.Fatalf("TaskID = %q, want %q", binding.TaskID, "task-1")
	}
}

func TestMessageServiceHandleMessageTaskSwitchesThreadsByActiveTask(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{
		tasks: map[string]app.Task{
			"user-1:task-1": {
				TaskID:        "task-1",
				DiscordUserID: "user-1",
				TaskName:      "Release work",
				Status:        app.TaskStatusOpen,
				CreatedAt:     time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
			},
			"user-1:task-2": {
				TaskID:        "task-2",
				DiscordUserID: "user-1",
				TaskName:      "Docs update",
				Status:        app.TaskStatusOpen,
				CreatedAt:     time.Date(2026, time.April, 5, 1, 0, 0, 0, time.UTC),
			},
		},
		activeTasks: map[string]app.ActiveTask{
			"user-1": {
				DiscordUserID: "user-1",
				TaskID:        "task-1",
			},
		},
	}
	gateway := &fakeCodexGateway{
		results: []app.RunTurnResult{
			{ThreadID: "thread-task-1", ResponseText: "Release response"},
			{ThreadID: "thread-task-2", ResponseText: "Docs response"},
		},
	}
	service := newTaskMessageService(t, store, gateway, nil)

	if _, err := service.HandleMessage(context.Background(), app.MessageRequest{
		UserID:     "user-1",
		MessageID:  "message-1",
		Content:    "release task",
		Mentioned:  true,
		ReceivedAt: time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
	}, nil); err != nil {
		t.Fatalf("HandleMessage() first error = %v", err)
	}

	if err := store.SetActiveTask(context.Background(), app.ActiveTask{
		DiscordUserID: "user-1",
		TaskID:        "task-2",
	}); err != nil {
		t.Fatalf("SetActiveTask() error = %v", err)
	}

	if _, err := service.HandleMessage(context.Background(), app.MessageRequest{
		UserID:     "user-1",
		MessageID:  "message-2",
		Content:    "docs task",
		Mentioned:  true,
		ReceivedAt: time.Date(2026, time.April, 8, 0, 0, 0, 0, time.UTC),
	}, nil); err != nil {
		t.Fatalf("HandleMessage() second error = %v", err)
	}

	calls := gateway.Calls()
	if len(calls) != 2 {
		t.Fatalf("RunTurn() call count = %d, want %d", len(calls), 2)
	}

	if calls[1].threadID != "" {
		t.Fatalf("second thread id = %q, want empty", calls[1].threadID)
	}

	if calls[0].workingDirectory != "/tmp/worktrees/task-1" {
		t.Fatalf("first working directory = %q, want %q", calls[0].workingDirectory, "/tmp/worktrees/task-1")
	}

	if calls[1].workingDirectory != "/tmp/worktrees/task-2" {
		t.Fatalf("second working directory = %q, want %q", calls[1].workingDirectory, "/tmp/worktrees/task-2")
	}

	for _, key := range []string{"user-1:task-1", "user-1:task-2"} {
		if _, ok, err := store.GetThreadBinding(context.Background(), "task", key); err != nil || !ok {
			t.Fatalf("GetThreadBinding(%s) = ok:%v err:%v, want ok:true err:nil", key, ok, err)
		}
	}
}

func TestMessageServiceHandleMessageTaskRetriesFailedWorkspaceSetup(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{
		tasks: map[string]app.Task{
			"user-1:task-1": {
				TaskID:        "task-1",
				DiscordUserID: "user-1",
				TaskName:      "Release work",
				Status:        app.TaskStatusOpen,
				CreatedAt:     time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
			},
		},
		activeTasks: map[string]app.ActiveTask{
			"user-1": {
				DiscordUserID: "user-1",
				TaskID:        "task-1",
			},
		},
	}
	gateway := &fakeCodexGateway{
		results: []app.RunTurnResult{
			{ThreadID: "thread-task-1", ResponseText: "Recovered response"},
		},
	}
	worktrees := &fakeTaskWorkspaceManager{
		store:     store,
		failCount: 1,
	}
	service := newTaskMessageServiceWithWorktrees(t, store, gateway, nil, worktrees)

	firstResponse, err := service.HandleMessage(context.Background(), app.MessageRequest{
		UserID:     "user-1",
		MessageID:  "message-1",
		Content:    "start release",
		Mentioned:  true,
		ReceivedAt: time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
	}, nil)
	if err != nil {
		t.Fatalf("HandleMessage() first error = %v", err)
	}

	if firstResponse.Text != "Task workspace setup failed. Please retry after checking the configured repository." {
		t.Fatalf("first response text = %q", firstResponse.Text)
	}

	if calls := gateway.Calls(); len(calls) != 0 {
		t.Fatalf("RunTurn() call count after failed setup = %d, want %d", len(calls), 0)
	}

	task, ok, err := store.GetTask(context.Background(), "user-1", "task-1")
	if err != nil {
		t.Fatalf("GetTask() after failed setup error = %v", err)
	}

	if !ok {
		t.Fatal("GetTask() after failed setup ok = false, want true")
	}

	if task.WorktreeStatus != app.TaskWorktreeStatusFailed {
		t.Fatalf("WorktreeStatus after failed setup = %q, want %q", task.WorktreeStatus, app.TaskWorktreeStatusFailed)
	}

	secondResponse, err := service.HandleMessage(context.Background(), app.MessageRequest{
		UserID:     "user-1",
		MessageID:  "message-2",
		Content:    "retry release",
		Mentioned:  true,
		ReceivedAt: time.Date(2026, time.April, 5, 0, 1, 0, 0, time.UTC),
	}, nil)
	if err != nil {
		t.Fatalf("HandleMessage() second error = %v", err)
	}

	if secondResponse.Text != "Recovered response" {
		t.Fatalf("second response text = %q, want %q", secondResponse.Text, "Recovered response")
	}

	calls := gateway.Calls()
	if len(calls) != 1 {
		t.Fatalf("RunTurn() call count after retry = %d, want %d", len(calls), 1)
	}

	if calls[0].workingDirectory != "/tmp/worktrees/task-1" {
		t.Fatalf("working directory after retry = %q, want %q", calls[0].workingDirectory, "/tmp/worktrees/task-1")
	}

	if worktrees.ensureCount != 2 {
		t.Fatalf("EnsureReady() count = %d, want %d", worktrees.ensureCount, 2)
	}
}

func TestMessageServiceHandleMessageForwardsImagePathsToGateway(t *testing.T) {
	t.Parallel()

	gateway := &fakeCodexGateway{
		results: []app.RunTurnResult{
			{ThreadID: "thread-1", ResponseText: "Image response"},
		},
	}
	service := newDailyMessageService(t, &memoryThreadStore{}, gateway, nil)

	_, err := service.HandleMessage(context.Background(), app.MessageRequest{
		MessageID:  "message-1",
		Content:    "describe this screenshot",
		ImagePaths: []string{"/tmp/one.png", "/tmp/two.png"},
		Mentioned:  true,
		ReceivedAt: time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
	}, nil)
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}

	calls := gateway.Calls()
	if len(calls) != 1 {
		t.Fatalf("RunTurn() call count = %d, want %d", len(calls), 1)
	}

	if calls[0].input.Prompt != "describe this screenshot" {
		t.Fatalf("prompt = %q, want %q", calls[0].input.Prompt, "describe this screenshot")
	}

	if got := calls[0].input.ImagePaths; len(got) != 2 || got[0] != "/tmp/one.png" || got[1] != "/tmp/two.png" {
		t.Fatalf("image paths = %v, want [/tmp/one.png /tmp/two.png]", got)
	}
}

func TestMessageServiceHandleMessageQueuesBusyTurnAndDeliversDeferredReply(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{}
	gateway := newBlockingCodexGateway(
		app.RunTurnResult{ThreadID: "thread-1", ResponseText: "First response"},
		app.RunTurnResult{ThreadID: "thread-1", ResponseText: "Second response"},
	)
	service := newDailyMessageService(t, store, gateway, thread.NewQueueCoordinator())

	firstDone := make(chan app.MessageResponse, 1)
	firstErr := make(chan error, 1)
	go func() {
		response, err := service.HandleMessage(context.Background(), app.MessageRequest{
			MessageID:  "message-1",
			Content:    "start long turn",
			Mentioned:  true,
			ReceivedAt: time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
		}, nil)
		firstDone <- response
		firstErr <- err
	}()

	waitForSignal(t, gateway.started, "first codex turn start")

	delivered := make(chan app.MessageResponse, 1)
	secondResponse, err := service.HandleMessage(context.Background(), app.MessageRequest{
		MessageID:  "message-2",
		Content:    "follow up later",
		Mentioned:  true,
		ReceivedAt: time.Date(2026, time.April, 5, 0, 1, 0, 0, time.UTC),
	}, app.DeferredReplySinkFunc(func(ctx context.Context, response app.MessageResponse) error {
		delivered <- response
		return nil
	}))
	if err != nil {
		t.Fatalf("HandleMessage() second error = %v", err)
	}

	if secondResponse.Text != "A response is already running for this conversation. Your message has been queued at position 1." {
		t.Fatalf("queued response text = %q", secondResponse.Text)
	}

	if secondResponse.ReplyToID != "message-2" {
		t.Fatalf("queued ReplyToID = %q, want %q", secondResponse.ReplyToID, "message-2")
	}

	close(gateway.release)

	select {
	case err := <-firstErr:
		if err != nil {
			t.Fatalf("first HandleMessage() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first response")
	}

	select {
	case response := <-firstDone:
		if response.Text != "First response" {
			t.Fatalf("first response text = %q, want %q", response.Text, "First response")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first response payload")
	}

	select {
	case response := <-delivered:
		if response.Text != "Second response" {
			t.Fatalf("deferred response text = %q, want %q", response.Text, "Second response")
		}
		if response.ReplyToID != "message-2" {
			t.Fatalf("deferred ReplyToID = %q, want %q", response.ReplyToID, "message-2")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for deferred response")
	}

	calls := gateway.Calls()
	if len(calls) != 2 {
		t.Fatalf("RunTurn() call count = %d, want %d", len(calls), 2)
	}

	if calls[1].threadID != "thread-1" {
		t.Fatalf("queued thread id = %q, want %q", calls[1].threadID, "thread-1")
	}
}

func TestMessageServiceWaitForDrainWaitsForDeferredReplyDelivery(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{}
	gateway := newBlockingCodexGateway(
		app.RunTurnResult{ThreadID: "thread-1", ResponseText: "First response"},
		app.RunTurnResult{ThreadID: "thread-1", ResponseText: "Second response"},
	)
	service := newDailyMessageService(t, store, gateway, thread.NewQueueCoordinator())

	firstDone := make(chan error, 1)
	go func() {
		_, err := service.HandleMessage(context.Background(), app.MessageRequest{
			MessageID:  "message-1",
			Content:    "start long turn",
			Mentioned:  true,
			ReceivedAt: time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
		}, nil)
		firstDone <- err
	}()

	waitForSignal(t, gateway.started, "first codex turn start")

	deliveryStarted := make(chan struct{})
	allowDelivery := make(chan struct{})
	_, err := service.HandleMessage(context.Background(), app.MessageRequest{
		MessageID:  "message-2",
		Content:    "follow up later",
		Mentioned:  true,
		ReceivedAt: time.Date(2026, time.April, 5, 0, 1, 0, 0, time.UTC),
	}, app.DeferredReplySinkFunc(func(ctx context.Context, response app.MessageResponse) error {
		close(deliveryStarted)
		<-allowDelivery
		return nil
	}))
	if err != nil {
		t.Fatalf("HandleMessage() queued error = %v", err)
	}

	close(gateway.release)

	select {
	case err := <-firstDone:
		if err != nil {
			t.Fatalf("first HandleMessage() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first response")
	}

	waitForSignal(t, deliveryStarted, "deferred reply delivery start")

	drainCtx, drainCancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer drainCancel()
	if err := service.WaitForDrain(drainCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitForDrain() error = %v, want %v", err, context.DeadlineExceeded)
	}

	close(allowDelivery)

	drainCtx, drainCancel = context.WithTimeout(context.Background(), time.Second)
	defer drainCancel()
	if err := service.WaitForDrain(drainCtx); err != nil {
		t.Fatalf("WaitForDrain() error = %v", err)
	}
}

func TestMessageServiceHandleMessageReturnsQueueFullResponse(t *testing.T) {
	t.Parallel()

	service := newDailyMessageService(t, &memoryThreadStore{}, &fakeCodexGateway{}, &stubQueueCoordinator{
		err: app.ErrExecutionQueueFull,
	})

	response, err := service.HandleMessage(context.Background(), app.MessageRequest{
		MessageID:  "message-6",
		Content:    "hello",
		Mentioned:  true,
		ReceivedAt: time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
	}, nil)
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}

	if response.Text != "This conversation already has five queued messages. Please retry in a moment." {
		t.Fatalf("queue-full response text = %q", response.Text)
	}

	if response.ReplyToID != "message-6" {
		t.Fatalf("ReplyToID = %q, want %q", response.ReplyToID, "message-6")
	}
}

func TestMessageServiceHandleMessageLogsExecuteNowAndTurnUsage(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))

	service := newDailyMessageServiceWithLogger(
		t,
		&memoryThreadStore{},
		&fakeCodexGateway{
			results: []app.RunTurnResult{
				{
					ThreadID:     "thread-1",
					ResponseText: "Done",
					Usage: &app.TokenUsage{
						InputTokens:       10,
						CachedInputTokens: 2,
						OutputTokens:      4,
					},
				},
			},
		},
		noopDailyMemoryRefresher{},
		nil,
		logger,
	)

	_, err := service.HandleMessage(context.Background(), app.MessageRequest{
		UserID:      "user-1",
		ChannelID:   "channel-1",
		MessageID:   "message-1",
		Content:     "hello there",
		Mentioned:   true,
		ReceivedAt:  time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
		ImagePaths:  []string{"/tmp/image.png"},
		CommandName: "release",
	}, nil)
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}

	entries := parseJSONLogEntries(t, logBuffer.Bytes())

	queueAdmission := findLogEntry(t, entries, "queue_admission")
	if queueAdmission["outcome"] != "execute_now" {
		t.Fatalf("queue admission outcome = %v, want %q", queueAdmission["outcome"], "execute_now")
	}

	turnStarted := findLogEntry(t, entries, "codex_turn_started")
	if turnStarted["thread_resumed"] != false {
		t.Fatalf("thread_resumed = %v, want %v", turnStarted["thread_resumed"], false)
	}

	if turnStarted["image_count"] != float64(1) {
		t.Fatalf("image_count = %v, want %v", turnStarted["image_count"], 1)
	}

	turnFinished := findLogEntry(t, entries, "codex_turn_finished")
	if turnFinished["outcome"] != "success" {
		t.Fatalf("turn outcome = %v, want %q", turnFinished["outcome"], "success")
	}

	if turnFinished["thread_id"] != "thread-1" {
		t.Fatalf("thread_id = %v, want %q", turnFinished["thread_id"], "thread-1")
	}

	if turnFinished["input_tokens"] != float64(10) {
		t.Fatalf("input_tokens = %v, want %v", turnFinished["input_tokens"], 10)
	}

	if turnFinished["cached_input_tokens"] != float64(2) {
		t.Fatalf("cached_input_tokens = %v, want %v", turnFinished["cached_input_tokens"], 2)
	}

	if turnFinished["output_tokens"] != float64(4) {
		t.Fatalf("output_tokens = %v, want %v", turnFinished["output_tokens"], 4)
	}
}

func TestMessageServiceHandleMessageLogsQueueFullAdmission(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))

	service := newDailyMessageServiceWithLogger(
		t,
		&memoryThreadStore{},
		&fakeCodexGateway{},
		noopDailyMemoryRefresher{},
		&stubQueueCoordinator{err: app.ErrExecutionQueueFull},
		logger,
	)

	_, err := service.HandleMessage(context.Background(), app.MessageRequest{
		ChannelID:  "channel-1",
		MessageID:  "message-6",
		Content:    "hello",
		Mentioned:  true,
		ReceivedAt: time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
	}, nil)
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}

	entries := parseJSONLogEntries(t, logBuffer.Bytes())
	queueAdmission := findLogEntry(t, entries, "queue_admission")
	if queueAdmission["outcome"] != "queue_full" {
		t.Fatalf("queue admission outcome = %v, want %q", queueAdmission["outcome"], "queue_full")
	}
}

func TestMessageServiceHandleMessageFreezesTaskContextForQueuedWork(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{
		tasks: map[string]app.Task{
			"user-1:task-1": {
				TaskID:        "task-1",
				DiscordUserID: "user-1",
				TaskName:      "Release work",
				Status:        app.TaskStatusOpen,
				CreatedAt:     time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
			},
			"user-1:task-2": {
				TaskID:        "task-2",
				DiscordUserID: "user-1",
				TaskName:      "Docs update",
				Status:        app.TaskStatusOpen,
				CreatedAt:     time.Date(2026, time.April, 5, 1, 0, 0, 0, time.UTC),
			},
		},
		activeTasks: map[string]app.ActiveTask{
			"user-1": {
				DiscordUserID: "user-1",
				TaskID:        "task-1",
			},
		},
	}
	gateway := newBlockingCodexGateway(
		app.RunTurnResult{ThreadID: "thread-task-1", ResponseText: "First response"},
		app.RunTurnResult{ThreadID: "thread-task-1", ResponseText: "Queued response"},
	)
	service := newTaskMessageService(t, store, gateway, thread.NewQueueCoordinator())

	firstDone := make(chan error, 1)
	go func() {
		_, err := service.HandleMessage(context.Background(), app.MessageRequest{
			UserID:     "user-1",
			MessageID:  "message-1",
			Content:    "start release",
			Mentioned:  true,
			ReceivedAt: time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
		}, nil)
		firstDone <- err
	}()

	waitForSignal(t, gateway.started, "first task turn start")

	delivered := make(chan app.MessageResponse, 1)
	response, err := service.HandleMessage(context.Background(), app.MessageRequest{
		UserID:     "user-1",
		MessageID:  "message-2",
		Content:    "continue release",
		Mentioned:  true,
		ReceivedAt: time.Date(2026, time.April, 5, 0, 1, 0, 0, time.UTC),
	}, app.DeferredReplySinkFunc(func(ctx context.Context, response app.MessageResponse) error {
		delivered <- response
		return nil
	}))
	if err != nil {
		t.Fatalf("HandleMessage() queued error = %v", err)
	}

	if response.ReplyToID != "message-2" {
		t.Fatalf("queued ReplyToID = %q, want %q", response.ReplyToID, "message-2")
	}

	if err := store.SetActiveTask(context.Background(), app.ActiveTask{
		DiscordUserID: "user-1",
		TaskID:        "task-2",
	}); err != nil {
		t.Fatalf("SetActiveTask() error = %v", err)
	}

	close(gateway.release)

	select {
	case err := <-firstDone:
		if err != nil {
			t.Fatalf("first HandleMessage() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first task turn")
	}

	select {
	case deliveredResponse := <-delivered:
		if deliveredResponse.Text != "Queued response" {
			t.Fatalf("queued response text = %q, want %q", deliveredResponse.Text, "Queued response")
		}
		if deliveredResponse.ReplyToID != "message-2" {
			t.Fatalf("queued response ReplyToID = %q, want %q", deliveredResponse.ReplyToID, "message-2")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for queued task response")
	}

	calls := gateway.Calls()
	if len(calls) != 2 {
		t.Fatalf("RunTurn() call count = %d, want %d", len(calls), 2)
	}

	if calls[1].threadID != "thread-task-1" {
		t.Fatalf("queued task thread id = %q, want %q", calls[1].threadID, "thread-task-1")
	}

	if _, ok, err := store.GetThreadBinding(context.Background(), "task", "user-1:task-1"); err != nil || !ok {
		t.Fatalf("GetThreadBinding(task-1) = ok:%v err:%v, want ok:true err:nil", ok, err)
	}

	if _, ok, err := store.GetThreadBinding(context.Background(), "task", "user-1:task-2"); err != nil {
		t.Fatalf("GetThreadBinding(task-2) error = %v", err)
	} else if ok {
		t.Fatal("GetThreadBinding(task-2) ok = true, want false")
	}
}

func TestMessageServiceHandleMessageQueuedWorkUsesCallerContext(t *testing.T) {
	t.Parallel()

	store := &memoryThreadStore{}
	firstRelease := make(chan struct{})
	gateway := newScriptedCodexGateway(
		[]app.RunTurnResult{
			{ThreadID: "thread-1", ResponseText: "First response"},
			{ThreadID: "thread-1", ResponseText: "Second response"},
		},
		[]chan struct{}{firstRelease, nil},
	)
	service := newDailyMessageService(t, store, gateway, thread.NewQueueCoordinator())

	firstDone := make(chan error, 1)
	go func() {
		_, err := service.HandleMessage(context.Background(), app.MessageRequest{
			MessageID:  "message-1",
			Content:    "start long turn",
			Mentioned:  true,
			ReceivedAt: time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
		}, nil)
		firstDone <- err
	}()

	waitForSignal(t, gateway.started, "first codex turn start")

	queuedCtx, cancelQueued := context.WithCancel(context.Background())
	delivered := make(chan app.MessageResponse, 1)
	_, err := service.HandleMessage(queuedCtx, app.MessageRequest{
		MessageID:  "message-2",
		Content:    "follow up later",
		Mentioned:  true,
		ReceivedAt: time.Date(2026, time.April, 5, 0, 1, 0, 0, time.UTC),
	}, app.DeferredReplySinkFunc(func(ctx context.Context, response app.MessageResponse) error {
		delivered <- response
		return nil
	}))
	if err != nil {
		t.Fatalf("HandleMessage() queued error = %v", err)
	}

	cancelQueued()
	close(firstRelease)

	select {
	case err := <-firstDone:
		if err != nil {
			t.Fatalf("first HandleMessage() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first response")
	}

	drainCtx, drainCancel := context.WithTimeout(context.Background(), time.Second)
	defer drainCancel()
	if err := service.WaitForDrain(drainCtx); err != nil {
		t.Fatalf("WaitForDrain() error = %v", err)
	}

	select {
	case response := <-delivered:
		t.Fatalf("unexpected deferred response = %+v", response)
	default:
	}

	calls := gateway.Calls()
	if len(calls) != 2 {
		t.Fatalf("RunTurn() call count = %d, want %d", len(calls), 2)
	}
}

func newDailyMessageService(
	t *testing.T,
	store app.ThreadStore,
	gateway app.CodexGateway,
	coordinator app.QueueCoordinator,
) *app.DefaultMessageService {
	t.Helper()

	return newDailyMessageServiceWithLogger(t, store, gateway, noopDailyMemoryRefresher{}, coordinator, nil)
}

func newDailyMessageServiceWithRefresher(
	t *testing.T,
	store app.ThreadStore,
	gateway app.CodexGateway,
	refresher app.DailyMemoryRefresher,
	coordinator app.QueueCoordinator,
) *app.DefaultMessageService {
	t.Helper()

	return newDailyMessageServiceWithLogger(t, store, gateway, refresher, coordinator, nil)
}

func newDailyMessageServiceWithLogger(
	t *testing.T,
	store app.ThreadStore,
	gateway app.CodexGateway,
	refresher app.DailyMemoryRefresher,
	coordinator app.QueueCoordinator,
	logger *slog.Logger,
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
		Logger:      logger,
		Policy:      policy,
		Store:       store,
		DailyMemory: refresher,
		Gateway:     gateway,
		Coordinator: coordinator,
	})
	if err != nil {
		t.Fatalf("NewMessageService() error = %v", err)
	}

	return service
}

func newTaskMessageService(
	t *testing.T,
	store app.ThreadStore,
	gateway app.CodexGateway,
	coordinator app.QueueCoordinator,
) *app.DefaultMessageService {
	t.Helper()

	worktrees := &fakeTaskWorkspaceManager{}
	if memoryStore, ok := store.(*memoryThreadStore); ok {
		worktrees.store = memoryStore
	}

	return newTaskMessageServiceWithWorktrees(t, store, gateway, coordinator, worktrees)
}

func newTaskMessageServiceWithWorktrees(
	t *testing.T,
	store app.ThreadStore,
	gateway app.CodexGateway,
	coordinator app.QueueCoordinator,
	worktrees app.TaskWorkspaceManager,
) *app.DefaultMessageService {
	t.Helper()

	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("time.LoadLocation() error = %v", err)
	}

	policy, err := thread.NewPolicy(config.ModeTask, tokyo, store)
	if err != nil {
		t.Fatalf("thread.NewPolicy() error = %v", err)
	}

	if coordinator == nil {
		coordinator = thread.NewQueueCoordinator()
	}

	service, err := app.NewMessageService(app.MessageServiceDependencies{
		Mode:             config.ModeTask,
		CommandName:      "release",
		Policy:           policy,
		Store:            store,
		WorkspaceManager: worktrees,
		Gateway:          gateway,
		Coordinator:      coordinator,
	})
	if err != nil {
		t.Fatalf("NewMessageService() error = %v", err)
	}

	return service
}

func waitForSignal(t *testing.T, ch <-chan struct{}, name string) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}

func parseJSONLogEntries(t *testing.T, output []byte) []map[string]any {
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

func findLogEntry(t *testing.T, entries []map[string]any, event string) map[string]any {
	t.Helper()

	for _, entry := range entries {
		if entry["event"] == event {
			return entry
		}
	}

	t.Fatalf("log event %q not found in %v", event, entries)
	return nil
}

type stubThreadPolicy struct {
	key string
	err error
}

func (s stubThreadPolicy) ResolveMessageKey(context.Context, app.MessageRequest) (string, error) {
	return s.key, s.err
}

type stubQueueCoordinator struct {
	admission app.QueueAdmission
	err       error
}

func (c *stubQueueCoordinator) Admit(string, func()) (app.QueueAdmission, error) {
	if c.err != nil {
		return app.QueueAdmission{}, c.err
	}

	if c.admission == (app.QueueAdmission{}) {
		return app.QueueAdmission{ExecuteNow: true}, nil
	}

	return c.admission, nil
}

func (c *stubQueueCoordinator) Complete(string) (func(), bool) {
	return nil, false
}

type fakeCodexGateway struct {
	mu       sync.Mutex
	calls    []runTurnCall
	results  []app.RunTurnResult
	err      error
	sequence *[]string
}

type scriptedCodexGateway struct {
	mu        sync.Mutex
	calls     []runTurnCall
	results   []app.RunTurnResult
	blockers  []chan struct{}
	started   chan struct{}
	startOnce sync.Once
}

type runTurnCall struct {
	threadID         string
	workingDirectory string
	input            app.CodexTurnInput
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

	if g.sequence != nil {
		*g.sequence = append(*g.sequence, "turn")
	}

	if g.err != nil {
		return app.RunTurnResult{}, g.err
	}

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

type blockingCodexGateway struct {
	mu        sync.Mutex
	calls     []runTurnCall
	results   []app.RunTurnResult
	err       error
	started   chan struct{}
	release   chan struct{}
	startOnce sync.Once
}

type noopDailyMemoryRefresher struct{}

func (noopDailyMemoryRefresher) RefreshBeforeFirstDailyTurn(context.Context, string, time.Time) error {
	return nil
}

type dailyMemoryCall struct {
	logicalKey string
	receivedAt time.Time
}

type fakeDailyMemoryRefresher struct {
	mu       sync.Mutex
	calls    []dailyMemoryCall
	err      error
	sequence *[]string
}

func (r *fakeDailyMemoryRefresher) RefreshBeforeFirstDailyTurn(_ context.Context, logicalKey string, receivedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.calls = append(r.calls, dailyMemoryCall{
		logicalKey: logicalKey,
		receivedAt: receivedAt,
	})

	if r.sequence != nil {
		*r.sequence = append(*r.sequence, "refresh")
	}

	return r.err
}

func (r *fakeDailyMemoryRefresher) Calls() []dailyMemoryCall {
	r.mu.Lock()
	defer r.mu.Unlock()

	calls := make([]dailyMemoryCall, len(r.calls))
	copy(calls, r.calls)
	return calls
}

func newBlockingCodexGateway(results ...app.RunTurnResult) *blockingCodexGateway {
	return &blockingCodexGateway{
		results: append([]app.RunTurnResult(nil), results...),
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func newScriptedCodexGateway(results []app.RunTurnResult, blockers []chan struct{}) *scriptedCodexGateway {
	clonedResults := append([]app.RunTurnResult(nil), results...)
	clonedBlockers := append([]chan struct{}(nil), blockers...)
	return &scriptedCodexGateway{
		results:  clonedResults,
		blockers: clonedBlockers,
		started:  make(chan struct{}),
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
	err := g.err
	release := g.release
	g.mu.Unlock()

	g.startOnce.Do(func() {
		close(g.started)
	})

	<-release

	if err != nil {
		return app.RunTurnResult{}, err
	}

	return result, nil
}

func (g *blockingCodexGateway) Calls() []runTurnCall {
	g.mu.Lock()
	defer g.mu.Unlock()

	calls := make([]runTurnCall, len(g.calls))
	copy(calls, g.calls)
	return calls
}

func (g *scriptedCodexGateway) RunTurn(ctx context.Context, threadID string, input app.CodexTurnInput) (app.RunTurnResult, error) {
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

	var blocker chan struct{}
	if len(g.blockers) > 0 {
		blocker = g.blockers[0]
		g.blockers = g.blockers[1:]
	}
	g.mu.Unlock()

	g.startOnce.Do(func() {
		close(g.started)
	})

	if err := ctx.Err(); err != nil {
		return app.RunTurnResult{}, err
	}

	if blocker != nil {
		<-blocker
	}

	if err := ctx.Err(); err != nil {
		return app.RunTurnResult{}, err
	}

	return result, nil
}

func (g *scriptedCodexGateway) Calls() []runTurnCall {
	g.mu.Lock()
	defer g.mu.Unlock()

	calls := make([]runTurnCall, len(g.calls))
	copy(calls, g.calls)
	return calls
}

type memoryThreadStore struct {
	mu          sync.Mutex
	bindings    map[string]app.ThreadBinding
	tasks       map[string]app.Task
	activeTasks map[string]app.ActiveTask
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

	if s.tasks == nil {
		return nil, nil
	}

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

type fakeTaskWorkspaceManager struct {
	store       *memoryThreadStore
	ensureCount int
	failCount   int
}

func (m *fakeTaskWorkspaceManager) EnsureReady(_ context.Context, task app.Task) (app.Task, error) {
	m.ensureCount++
	if m.failCount > 0 {
		m.failCount--
		task.WorktreeStatus = app.TaskWorktreeStatusFailed
		if m.store != nil {
			_ = m.store.UpdateTask(context.Background(), task)
		}
		return app.Task{}, errors.New("workspace setup failed")
	}

	if task.BranchName == "" {
		task.BranchName = app.DefaultTaskBranchName(task.TaskID)
	}
	task.BaseRef = "main"
	task.WorktreePath = "/tmp/worktrees/" + task.TaskID
	task.WorktreeStatus = app.TaskWorktreeStatusReady
	now := time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC)
	if task.WorktreeCreatedAt == nil {
		task.WorktreeCreatedAt = &now
	}

	if m.store != nil {
		if err := m.store.UpdateTask(context.Background(), task); err != nil {
			return app.Task{}, err
		}
	}

	return task, nil
}

func (*fakeTaskWorkspaceManager) PruneClosed(context.Context) error {
	return nil
}

func (s *memoryThreadStore) GetActiveTask(_ context.Context, userID string) (app.ActiveTask, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.activeTasks == nil {
		return app.ActiveTask{}, false, nil
	}

	activeTask, ok := s.activeTasks[userID]
	return activeTask, ok, nil
}

func (s *memoryThreadStore) ClearActiveTask(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.activeTasks != nil {
		delete(s.activeTasks, userID)
	}
	return nil
}

func (s *memoryThreadStore) CloseTask(_ context.Context, userID string, taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.tasks == nil {
		return sql.ErrNoRows
	}

	key := userID + ":" + taskID
	task, ok := s.tasks[key]
	if !ok {
		return sql.ErrNoRows
	}

	closedAt := time.Date(2026, time.April, 5, 12, 0, 0, 0, time.UTC)
	task.Status = app.TaskStatusClosed
	task.ClosedAt = &closedAt
	s.tasks[key] = task

	if s.activeTasks != nil {
		if activeTask, ok := s.activeTasks[userID]; ok && activeTask.TaskID == taskID {
			delete(s.activeTasks, userID)
		}
	}

	return nil
}
