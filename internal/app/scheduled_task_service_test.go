package app

import (
	"context"
	"errors"
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

	service, err := NewScheduledTaskService(ScheduledTaskServiceDependencies{
		Mode:                   config.ModeDaily,
		Timezone:               location,
		Workdir:                "/workspace/project",
		DefaultReportChannelID: "channel-1",
		Store:                  store,
		Gateway:                fakeScheduledGateway{},
		ReportSender:           &fakeScheduledReportSender{},
		Logger:                 nil,
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
	if delivery.DiscordChannelID != "channel-1" {
		t.Fatalf("delivery channel = %q, want %q", delivery.DiscordChannelID, "channel-1")
	}
}

type fakeScheduledGateway struct{}

func (fakeScheduledGateway) RunTurn(ctx context.Context, threadID string, input CodexTurnInput) (RunTurnResult, error) {
	return RunTurnResult{
		ThreadID:     "thread-1",
		ResponseText: "Scheduled run complete.",
	}, nil
}

type fakeScheduledReportSender struct {
	mu       sync.Mutex
	messages []string
}

func (s *fakeScheduledReportSender) SendScheduledReport(ctx context.Context, channelID string, text string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, channelID+":"+text)
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
