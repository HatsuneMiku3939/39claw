package app

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/config"
)

func TestScheduledTaskServiceTickExecutesAndDeliversDueRun(t *testing.T) {
	t.Parallel()

	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}

	store := &fakeScheduledTaskStore{
		tasks: []ScheduledTask{
			{
				ScheduledTaskID: "task-1",
				Name:            "daily-report",
				ScheduleKind:    ScheduledTaskScheduleKindCron,
				ScheduleExpr:    "* * * * *",
				Prompt:          "Write the report.",
				Enabled:         true,
				CreatedAt:       time.Date(2026, time.April, 12, 8, 0, 30, 0, location),
			},
		},
	}
	gateway := &fakeScheduledGateway{}

	service, err := NewScheduledTaskService(ScheduledTaskServiceDependencies{
		Mode:                config.ModeJournal,
		Timezone:            location,
		Workdir:             "/workspace/project",
		DefaultReportTarget: "channel:channel-1",
		Store:               store,
		Gateway:             gateway,
		ReportSender:        &fakeScheduledReportSender{},
		Logger:              nil,
		Now: func() time.Time {
			return time.Date(2026, time.April, 12, 8, 1, 0, 0, location)
		},
		NewRunID: func() string {
			return "run-1"
		},
		NewDeliveryID: func() string {
			return "delivery-1"
		},
	})
	if err != nil {
		t.Fatalf("NewScheduledTaskService() error = %v", err)
	}

	service.tick(context.Background())
	service.runWG.Wait()

	if len(store.admittedRuns) != 1 {
		t.Fatalf("admittedRuns count = %d, want %d", len(store.admittedRuns), 1)
	}

	run := store.runs["run-1"]
	if run.Status != ScheduledTaskRunStatusSucceeded {
		t.Fatalf("run status = %q, want %q", run.Status, ScheduledTaskRunStatusSucceeded)
	}
	if run.WorkdirPath != "/workspace/project" {
		t.Fatalf("run workdir = %q, want %q", run.WorkdirPath, "/workspace/project")
	}
	if run.ResponseText != "Scheduled run complete." {
		t.Fatalf("run response = %q, want %q", run.ResponseText, "Scheduled run complete.")
	}

	delivery := store.deliveries["delivery-1"]
	if delivery.Status != ScheduledTaskDeliveryStatusSucceeded {
		t.Fatalf("delivery status = %q, want %q", delivery.Status, ScheduledTaskDeliveryStatusSucceeded)
	}
	if delivery.ReportTarget != "channel:channel-1" {
		t.Fatalf("delivery report target = %q, want %q", delivery.ReportTarget, "channel:channel-1")
	}
	if gateway.runCount() != 1 {
		t.Fatalf("gateway run count = %d, want %d", gateway.runCount(), 1)
	}
}

func TestScheduledTaskServiceTickSkipsBackfillBeforeServiceStart(t *testing.T) {
	t.Parallel()

	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}

	store := &fakeScheduledTaskStore{
		tasks: []ScheduledTask{
			{
				ScheduledTaskID: "task-1",
				Name:            "daily-report",
				ScheduleKind:    ScheduledTaskScheduleKindCron,
				ScheduleExpr:    "* * * * *",
				Prompt:          "Write the report.",
				Enabled:         true,
				CreatedAt:       time.Date(2026, time.April, 12, 8, 0, 30, 0, location),
			},
		},
	}
	gateway := &fakeScheduledGateway{}

	service, err := NewScheduledTaskService(ScheduledTaskServiceDependencies{
		Mode:                config.ModeJournal,
		Timezone:            location,
		Workdir:             "/workspace/project",
		DefaultReportTarget: "channel:channel-1",
		Store:               store,
		Gateway:             gateway,
		ReportSender:        &fakeScheduledReportSender{},
		Now: func() time.Time {
			return time.Date(2026, time.April, 12, 8, 3, 40, 0, location)
		},
		StartedAt:     time.Date(2026, time.April, 12, 8, 3, 30, 0, location),
		NewRunID:      sequentialStringGenerator("run"),
		NewDeliveryID: sequentialStringGenerator("delivery"),
	})
	if err != nil {
		t.Fatalf("NewScheduledTaskService() error = %v", err)
	}

	service.tick(context.Background())
	service.runWG.Wait()

	if len(store.admittedRuns) != 0 {
		t.Fatalf("admittedRuns count = %d, want %d", len(store.admittedRuns), 0)
	}
	if gateway.runCount() != 0 {
		t.Fatalf("gateway run count = %d, want %d", gateway.runCount(), 0)
	}
}

func TestScheduledTaskServiceTickUsesLatestRunAsAnchor(t *testing.T) {
	t.Parallel()

	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}

	existingRunTime := time.Date(2026, time.April, 12, 8, 2, 0, 0, location).UTC()
	store := &fakeScheduledTaskStore{
		tasks: []ScheduledTask{
			{
				ScheduledTaskID: "task-1",
				Name:            "daily-report",
				ScheduleKind:    ScheduledTaskScheduleKindCron,
				ScheduleExpr:    "* * * * *",
				Prompt:          "Write the report.",
				Enabled:         true,
				CreatedAt:       time.Date(2026, time.April, 12, 8, 0, 30, 0, location),
			},
		},
		runs: map[string]ScheduledTaskRun{
			"run-existing": {
				ScheduledRunID:  "run-existing",
				ScheduledTaskID: "task-1",
				ScheduledFor:    existingRunTime,
				Attempt:         1,
				Status:          ScheduledTaskRunStatusSucceeded,
			},
		},
	}
	gateway := &fakeScheduledGateway{}

	service, err := NewScheduledTaskService(ScheduledTaskServiceDependencies{
		Mode:                config.ModeJournal,
		Timezone:            location,
		Workdir:             "/workspace/project",
		DefaultReportTarget: "channel:channel-1",
		Store:               store,
		Gateway:             gateway,
		ReportSender:        &fakeScheduledReportSender{},
		Now: func() time.Time {
			return time.Date(2026, time.April, 12, 8, 4, 0, 0, location)
		},
		NewRunID:      sequentialStringGenerator("run"),
		NewDeliveryID: sequentialStringGenerator("delivery"),
	})
	if err != nil {
		t.Fatalf("NewScheduledTaskService() error = %v", err)
	}

	service.tick(context.Background())
	service.runWG.Wait()

	if len(store.admittedRuns) != 1 {
		t.Fatalf("admittedRuns count = %d, want %d", len(store.admittedRuns), 1)
	}

	wantDueTime := time.Date(2026, time.April, 12, 8, 4, 0, 0, location).UTC()
	if !store.admittedRuns[0].ScheduledFor.Equal(wantDueTime) {
		t.Fatalf("admittedRuns[0].ScheduledFor = %v, want %v", store.admittedRuns[0].ScheduledFor, wantDueTime)
	}
}

func TestScheduledTaskServiceExecuteTaskNowRunsImmediately(t *testing.T) {
	t.Parallel()

	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	store := &fakeScheduledTaskStore{
		tasks: []ScheduledTask{
			{
				ScheduledTaskID: "task-1",
				Name:            "daily-report",
				ScheduleKind:    ScheduledTaskScheduleKindCron,
				ScheduleExpr:    "* * * * *",
				Prompt:          "Write the report.",
				Enabled:         true,
				CreatedAt:       time.Date(2026, time.April, 12, 8, 0, 30, 0, location),
			},
		},
	}
	gateway := &fakeScheduledGateway{}
	reportSender := &fakeScheduledReportSender{}

	service, err := NewScheduledTaskService(ScheduledTaskServiceDependencies{
		Mode:                config.ModeJournal,
		Timezone:            location,
		Workdir:             "/workspace/project",
		DefaultReportTarget: "channel:channel-1",
		Store:               store,
		Gateway:             gateway,
		ReportSender:        reportSender,
		Logger:              logger,
		Now: func() time.Time {
			return time.Date(2026, time.April, 12, 8, 5, 10, 0, location)
		},
		NewRunID:      func() string { return "run-now-1" },
		NewDeliveryID: func() string { return "delivery-now-1" },
	})
	if err != nil {
		t.Fatalf("NewScheduledTaskService() error = %v", err)
	}

	run, err := service.ExecuteTaskNow(context.Background(), "daily-report")
	if err != nil {
		t.Fatalf("ExecuteTaskNow() error = %v", err)
	}

	if run.ScheduledRunID != "run-now-1" {
		t.Fatalf("run ID = %q, want %q", run.ScheduledRunID, "run-now-1")
	}
	if run.Status != ScheduledTaskRunStatusSucceeded {
		t.Fatalf("run status = %q, want %q", run.Status, ScheduledTaskRunStatusSucceeded)
	}
	if gateway.runCount() != 1 {
		t.Fatalf("gateway run count = %d, want %d", gateway.runCount(), 1)
	}
	if len(reportSender.messages) != 1 {
		t.Fatalf("report sender message count = %d, want %d", len(reportSender.messages), 1)
	}
	if !strings.Contains(logs.String(), "scheduled task execute-now requested") {
		t.Fatalf("log output = %q, want execute-now message", logs.String())
	}
}

func TestScheduledTaskServiceExecuteRunDoesNotRetryCanceledRun(t *testing.T) {
	t.Parallel()

	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}

	store := &fakeScheduledTaskStore{
		tasks: []ScheduledTask{
			{
				ScheduledTaskID: "task-1",
				Name:            "daily-report",
				ScheduleKind:    ScheduledTaskScheduleKindCron,
				ScheduleExpr:    "* * * * *",
				Prompt:          "Write the report.",
				Enabled:         true,
				CreatedAt:       time.Date(2026, time.April, 12, 8, 0, 30, 0, location),
			},
		},
	}
	gateway := &fakeScheduledGateway{err: context.Canceled}

	service, err := NewScheduledTaskService(ScheduledTaskServiceDependencies{
		Mode:                config.ModeJournal,
		Timezone:            location,
		Workdir:             "/workspace/project",
		DefaultReportTarget: "channel:channel-1",
		Store:               store,
		Gateway:             gateway,
		ReportSender:        &fakeScheduledReportSender{},
		Now: func() time.Time {
			return time.Date(2026, time.April, 12, 8, 5, 10, 0, location)
		},
		NewRunID:      sequentialStringGenerator("run"),
		NewDeliveryID: sequentialStringGenerator("delivery"),
	})
	if err != nil {
		t.Fatalf("NewScheduledTaskService() error = %v", err)
	}

	run, admitted, err := store.AdmitScheduledTaskRun(context.Background(), ScheduledTaskRun{
		ScheduledRunID:  "run-1",
		ScheduledTaskID: "task-1",
		Mode:            "journal",
		ScheduledFor:    time.Date(2026, time.April, 12, 8, 5, 10, 0, location).UTC(),
		Attempt:         1,
		Status:          ScheduledTaskRunStatusPending,
	})
	if err != nil {
		t.Fatalf("AdmitScheduledTaskRun() error = %v", err)
	}
	if !admitted {
		t.Fatal("AdmitScheduledTaskRun() admitted = false, want true")
	}

	finalRun := service.executeRun(context.Background(), store.tasks[0], run)
	if finalRun.Status != ScheduledTaskRunStatusCanceled {
		t.Fatalf("final run status = %q, want %q", finalRun.Status, ScheduledTaskRunStatusCanceled)
	}
	if finalRun.ErrorCode != "codex_run_canceled" {
		t.Fatalf("final run error code = %q, want %q", finalRun.ErrorCode, "codex_run_canceled")
	}
	if len(store.admittedRuns) != 1 {
		t.Fatalf("admittedRuns count = %d, want %d", len(store.admittedRuns), 1)
	}
}

func TestNewScheduledTaskServiceDefaultNowUsesLiveClock(t *testing.T) {
	t.Parallel()

	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}

	service, err := NewScheduledTaskService(ScheduledTaskServiceDependencies{
		Mode:                config.ModeJournal,
		Timezone:            location,
		Workdir:             "/workspace/project",
		DefaultReportTarget: "channel:channel-1",
		Store:               &fakeScheduledTaskStore{},
		Gateway:             &fakeScheduledGateway{},
		ReportSender:        &fakeScheduledReportSender{},
	})
	if err != nil {
		t.Fatalf("NewScheduledTaskService() error = %v", err)
	}

	first := service.now()
	time.Sleep(10 * time.Millisecond)
	second := service.now()
	if !second.After(first) {
		t.Fatalf("default now clock did not advance: first=%v second=%v", first, second)
	}
}

type fakeScheduledGateway struct {
	mu     sync.Mutex
	inputs []CodexTurnInput
	err    error
}

func (g *fakeScheduledGateway) RunTurn(ctx context.Context, threadID string, input CodexTurnInput) (RunTurnResult, error) {
	g.mu.Lock()
	g.inputs = append(g.inputs, input)
	g.mu.Unlock()

	if g.err != nil {
		return RunTurnResult{}, g.err
	}

	return RunTurnResult{
		ThreadID:     "thread-1",
		ResponseText: "Scheduled run complete.",
	}, nil
}

func (g *fakeScheduledGateway) runCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.inputs)
}

type fakeScheduledReportSender struct {
	mu       sync.Mutex
	messages []string
}

func (s *fakeScheduledReportSender) SendScheduledReport(ctx context.Context, reportTarget string, text string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, reportTarget+":"+text)
	return "discord-message-1", nil
}

type fakeScheduledTaskStore struct {
	mu         sync.Mutex
	tasks      []ScheduledTask
	runs       map[string]ScheduledTaskRun
	deliveries map[string]ScheduledTaskDelivery

	admittedRuns []ScheduledTaskRun
}

func (s *fakeScheduledTaskStore) ListScheduledTasks(ctx context.Context) ([]ScheduledTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]ScheduledTask(nil), s.tasks...), nil
}

func (s *fakeScheduledTaskStore) ListEnabledScheduledTasks(ctx context.Context) ([]ScheduledTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var tasks []ScheduledTask
	for _, task := range s.tasks {
		if task.Enabled {
			tasks = append(tasks, task)
		}
	}

	return tasks, nil
}

func (s *fakeScheduledTaskStore) GetScheduledTaskByID(ctx context.Context, scheduledTaskID string) (ScheduledTask, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, task := range s.tasks {
		if task.ScheduledTaskID == scheduledTaskID {
			return task, true, nil
		}
	}

	return ScheduledTask{}, false, nil
}

func (s *fakeScheduledTaskStore) GetScheduledTaskByName(ctx context.Context, name string) (ScheduledTask, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, task := range s.tasks {
		if task.Name == name {
			return task, true, nil
		}
	}

	return ScheduledTask{}, false, nil
}

func (s *fakeScheduledTaskStore) CreateScheduledTask(ctx context.Context, task ScheduledTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks = append(s.tasks, task)
	return nil
}

func (s *fakeScheduledTaskStore) UpdateScheduledTask(ctx context.Context, task ScheduledTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for index := range s.tasks {
		if s.tasks[index].ScheduledTaskID == task.ScheduledTaskID {
			s.tasks[index] = task
			return nil
		}
	}

	return errors.New("scheduled task not found")
}

func (s *fakeScheduledTaskStore) DeleteScheduledTask(ctx context.Context, scheduledTaskID string) error {
	return nil
}

func (s *fakeScheduledTaskStore) GetLatestScheduledTaskRunForTask(
	ctx context.Context,
	scheduledTaskID string,
) (ScheduledTaskRun, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var latest ScheduledTaskRun
	found := false
	for _, run := range s.runs {
		if run.ScheduledTaskID != scheduledTaskID {
			continue
		}
		if !found || run.ScheduledFor.After(latest.ScheduledFor) || (run.ScheduledFor.Equal(latest.ScheduledFor) && run.Attempt > latest.Attempt) {
			latest = run
			found = true
		}
	}

	return latest, found, nil
}

func (s *fakeScheduledTaskStore) AdmitScheduledTaskRun(ctx context.Context, run ScheduledTaskRun) (ScheduledTaskRun, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.runs == nil {
		s.runs = make(map[string]ScheduledTaskRun)
	}

	for _, existing := range s.runs {
		if existing.ScheduledTaskID == run.ScheduledTaskID &&
			existing.ScheduledFor.Equal(run.ScheduledFor) &&
			existing.Attempt == run.Attempt {
			return ScheduledTaskRun{}, false, nil
		}
	}

	s.runs[run.ScheduledRunID] = run
	s.admittedRuns = append(s.admittedRuns, run)
	return run, true, nil
}

func (s *fakeScheduledTaskStore) UpdateScheduledTaskRun(ctx context.Context, run ScheduledTaskRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.runs == nil {
		s.runs = make(map[string]ScheduledTaskRun)
	}
	s.runs[run.ScheduledRunID] = run
	return nil
}

func (s *fakeScheduledTaskStore) ListScheduledTaskRunsForDueTime(
	ctx context.Context,
	scheduledTaskID string,
	scheduledFor time.Time,
) ([]ScheduledTaskRun, error) {
	return nil, nil
}

func (s *fakeScheduledTaskStore) CreateScheduledTaskDelivery(ctx context.Context, delivery ScheduledTaskDelivery) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.deliveries == nil {
		s.deliveries = make(map[string]ScheduledTaskDelivery)
	}
	s.deliveries[delivery.ScheduledDeliveryID] = delivery
	return nil
}

func (s *fakeScheduledTaskStore) UpdateScheduledTaskDelivery(ctx context.Context, delivery ScheduledTaskDelivery) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.deliveries == nil {
		s.deliveries = make(map[string]ScheduledTaskDelivery)
	}
	s.deliveries[delivery.ScheduledDeliveryID] = delivery
	return nil
}

func sequentialStringGenerator(prefix string) func() string {
	var mu sync.Mutex
	counter := 0

	return func() string {
		mu.Lock()
		defer mu.Unlock()
		counter++
		return prefix + "-" + strconv.Itoa(counter)
	}
}
