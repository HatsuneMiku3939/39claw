package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/config"
	"github.com/oklog/ulid/v2"
)

const (
	defaultScheduledTaskTickInterval = time.Minute
	scheduledTaskCleanupTimeout      = 10 * time.Second
)

type ScheduledTaskServiceDependencies struct {
	Mode                   config.Mode
	Timezone               *time.Location
	Workdir                string
	DefaultReportChannelID string
	Store                  ScheduledTaskStore
	Gateway                CodexGateway
	ReportSender           ScheduledTaskReportSender
	WorkspaceManager       ScheduledTaskWorkspaceManager
	Logger                 *slog.Logger
	TickInterval           time.Duration
	Now                    func() time.Time
	NewRunID               func() string
	NewDeliveryID          func() string
}

type ScheduledTaskService struct {
	mode                   config.Mode
	timezone               *time.Location
	workdir                string
	defaultReportChannelID string
	store                  ScheduledTaskStore
	gateway                CodexGateway
	reportSender           ScheduledTaskReportSender
	workspaceManager       ScheduledTaskWorkspaceManager
	logger                 *slog.Logger
	tickInterval           time.Duration
	now                    func() time.Time
	newRunID               func() string
	newDeliveryID          func() string

	mu      sync.Mutex
	cancel  context.CancelFunc
	started bool
	loopWG  sync.WaitGroup
	runWG   sync.WaitGroup
}

func NewScheduledTaskService(deps ScheduledTaskServiceDependencies) (*ScheduledTaskService, error) {
	if deps.Timezone == nil {
		return nil, fmt.Errorf("timezone must not be nil")
	}
	if deps.Store == nil {
		return nil, fmt.Errorf("scheduled task store must not be nil")
	}
	if deps.Gateway == nil {
		return nil, fmt.Errorf("codex gateway must not be nil")
	}
	if deps.ReportSender == nil {
		return nil, fmt.Errorf("scheduled task report sender must not be nil")
	}

	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}

	tickInterval := deps.TickInterval
	if tickInterval <= 0 {
		tickInterval = defaultScheduledTaskTickInterval
	}

	now := deps.Now
	if now == nil {
		now = time.Now().UTC
	}

	newRunID := deps.NewRunID
	if newRunID == nil {
		newRunID = func() string { return ulid.Make().String() }
	}

	newDeliveryID := deps.NewDeliveryID
	if newDeliveryID == nil {
		newDeliveryID = func() string { return ulid.Make().String() }
	}

	if deps.Mode == config.ModeTask && deps.WorkspaceManager == nil {
		return nil, fmt.Errorf("task mode scheduled execution requires a workspace manager")
	}

	return &ScheduledTaskService{
		mode:                   deps.Mode,
		timezone:               deps.Timezone,
		workdir:                deps.Workdir,
		defaultReportChannelID: deps.DefaultReportChannelID,
		store:                  deps.Store,
		gateway:                deps.Gateway,
		reportSender:           deps.ReportSender,
		workspaceManager:       deps.WorkspaceManager,
		logger:                 logger,
		tickInterval:           tickInterval,
		now:                    now,
		newRunID:               newRunID,
		newDeliveryID:          newDeliveryID,
	}, nil
}

func (s *ScheduledTaskService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return fmt.Errorf("scheduled task service already started")
	}

	//nolint:gosec // The service keeps the cancel function and calls it during Close.
	loopCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.started = true
	s.loopWG.Add(1)
	go s.loop(loopCtx)
	return nil
}

func (s *ScheduledTaskService) Close(ctx context.Context) error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	cancel := s.cancel
	s.cancel = nil
	s.started = false
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.loopWG.Wait()
		s.runWG.Wait()
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *ScheduledTaskService) loop(ctx context.Context) {
	defer s.loopWG.Done()

	ticker := time.NewTicker(s.tickInterval)
	defer ticker.Stop()

	s.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *ScheduledTaskService) tick(ctx context.Context) {
	tasks, err := s.store.ListEnabledScheduledTasks(ctx)
	if err != nil {
		s.logger.Error("list enabled scheduled tasks", "error", err)
		return
	}

	now := s.now().In(s.timezone)
	for _, task := range tasks {
		schedule, err := ParseScheduledTaskSchedule(task, s.timezone)
		if err != nil {
			s.logger.Error("parse scheduled task definition", "task", task.Name, "error", err)
			continue
		}

		anchor := task.CreatedAt.In(s.timezone).Add(-time.Second)
		latestRun, ok, err := s.store.GetLatestScheduledTaskRunForTask(ctx, task.ScheduledTaskID)
		if err != nil {
			s.logger.Error("load latest scheduled task run", "task", task.Name, "error", err)
			continue
		}
		if ok {
			anchor = latestRun.ScheduledFor.In(s.timezone)
		}

		for due := schedule.Next(anchor); !due.IsZero() && !due.After(now); due = schedule.Next(due) {
			run, admitted, err := s.store.AdmitScheduledTaskRun(ctx, ScheduledTaskRun{
				ScheduledRunID:  s.newRunID(),
				ScheduledTaskID: task.ScheduledTaskID,
				Mode:            string(s.mode),
				ScheduledFor:    due.UTC(),
				Attempt:         1,
				Status:          ScheduledTaskRunStatusPending,
			})
			if err != nil {
				s.logger.Error("admit scheduled task run", "task", task.Name, "scheduled_for", due, "error", err)
				break
			}
			if !admitted {
				continue
			}

			s.runWG.Add(1)
			go func(task ScheduledTask, run ScheduledTaskRun) {
				defer s.runWG.Done()
				s.executeRun(ctx, task, run)
			}(task, run)
		}
	}
}

func (s *ScheduledTaskService) executeRun(ctx context.Context, task ScheduledTask, run ScheduledTaskRun) {
	startedAt := s.now()
	run.Status = ScheduledTaskRunStatusRunning
	run.StartedAt = &startedAt
	run.UpdatedAt = startedAt
	if err := s.store.UpdateScheduledTaskRun(ctx, run); err != nil {
		s.logger.Error("mark scheduled task run started", "run_id", run.ScheduledRunID, "error", err)
		return
	}

	workdir := s.workdir
	cleanup := func(context.Context) error { return nil }
	if s.mode == config.ModeTask {
		tempWorktreePath, cleanupFn, err := s.workspaceManager.PrepareTemporaryWorktree(ctx, run.ScheduledRunID)
		if err != nil {
			s.failRun(ctx, task, run, "workspace_prepare_failed", err.Error(), true)
			return
		}
		workdir = tempWorktreePath
		run.TempWorktreePath = tempWorktreePath
		run.WorkdirPath = tempWorktreePath
		cleanup = cleanupFn
	} else {
		run.WorkdirPath = s.workdir
	}

	prompt := buildScheduledRunPrompt(task, run)
	result, err := s.gateway.RunTurn(ctx, "", CodexTurnInput{
		Prompt:           prompt,
		WorkingDirectory: workdir,
	})

	finishedAt := s.now()
	run.CodexThreadID = result.ThreadID
	run.ResponseText = result.ResponseText
	run.FinishedAt = &finishedAt
	run.UpdatedAt = finishedAt

	if err != nil {
		s.failRun(ctx, task, run, "codex_run_failed", err.Error(), true)
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), scheduledTaskCleanupTimeout)
		if cleanupErr := cleanup(cleanupCtx); cleanupErr != nil {
			s.logger.Error("cleanup scheduled task worktree after failure", "run_id", run.ScheduledRunID, "error", cleanupErr)
		}
		cancel()
		return
	}

	run.Status = ScheduledTaskRunStatusSucceeded
	if updateErr := s.store.UpdateScheduledTaskRun(ctx, run); updateErr != nil {
		s.logger.Error("persist scheduled task run success", "run_id", run.ScheduledRunID, "error", updateErr)
	}

	s.deliverRunResult(ctx, task, run)

	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), scheduledTaskCleanupTimeout)
	if cleanupErr := cleanup(cleanupCtx); cleanupErr != nil {
		run.ErrorCode = "workspace_cleanup_failed"
		run.ErrorMessage = cleanupErr.Error()
		run.UpdatedAt = s.now()
		if updateErr := s.store.UpdateScheduledTaskRun(ctx, run); updateErr != nil {
			s.logger.Error("persist scheduled run cleanup failure", "run_id", run.ScheduledRunID, "error", updateErr)
		}
		s.logger.Error("cleanup scheduled task worktree", "run_id", run.ScheduledRunID, "error", cleanupErr)
	}
	cancel()
}

func (s *ScheduledTaskService) failRun(
	ctx context.Context,
	task ScheduledTask,
	run ScheduledTaskRun,
	errorCode string,
	errorMessage string,
	mayRetry bool,
) {
	finishedAt := s.now()
	run.Status = ScheduledTaskRunStatusFailed
	run.ErrorCode = errorCode
	run.ErrorMessage = errorMessage
	run.FinishedAt = &finishedAt
	run.UpdatedAt = finishedAt
	if err := s.store.UpdateScheduledTaskRun(ctx, run); err != nil {
		s.logger.Error("persist scheduled task run failure", "run_id", run.ScheduledRunID, "error", err)
	}

	if mayRetry && run.Attempt < 2 {
		retryRun, admitted, err := s.store.AdmitScheduledTaskRun(ctx, ScheduledTaskRun{
			ScheduledRunID:  s.newRunID(),
			ScheduledTaskID: run.ScheduledTaskID,
			Mode:            run.Mode,
			ScheduledFor:    run.ScheduledFor,
			Attempt:         run.Attempt + 1,
			Status:          ScheduledTaskRunStatusPending,
		})
		if err != nil {
			s.logger.Error("admit scheduled task retry run", "run_id", run.ScheduledRunID, "error", err)
			return
		}
		if admitted {
			s.runWG.Add(1)
			go func() {
				defer s.runWG.Done()
				s.executeRun(ctx, task, retryRun)
			}()
		}
	}
}

func (s *ScheduledTaskService) deliverRunResult(ctx context.Context, task ScheduledTask, run ScheduledTaskRun) {
	channelID := ResolveScheduledTaskReportChannel(task, s.defaultReportChannelID)
	delivery := ScheduledTaskDelivery{
		ScheduledDeliveryID: s.newDeliveryID(),
		ScheduledRunID:      run.ScheduledRunID,
		DiscordChannelID:    channelID,
		Status:              ScheduledTaskDeliveryStatusPending,
	}

	if channelID == "" {
		now := s.now()
		delivery.Status = ScheduledTaskDeliveryStatusSkipped
		delivery.DeliveredAt = &now
		delivery.ErrorCode = "missing_report_channel"
		delivery.ErrorMessage = "scheduled task does not resolve to a report channel"
		if err := s.store.CreateScheduledTaskDelivery(ctx, delivery); err != nil {
			s.logger.Error("record skipped scheduled task delivery", "run_id", run.ScheduledRunID, "error", err)
		}
		return
	}

	if err := s.store.CreateScheduledTaskDelivery(ctx, delivery); err != nil {
		s.logger.Error("create scheduled task delivery", "run_id", run.ScheduledRunID, "error", err)
		return
	}

	messageID, err := s.reportSender.SendScheduledReport(ctx, channelID, formatScheduledReport(task, run))
	now := s.now()
	delivery.DeliveredAt = &now
	delivery.UpdatedAt = now
	if err != nil {
		delivery.Status = ScheduledTaskDeliveryStatusFailed
		delivery.ErrorCode = "discord_delivery_failed"
		delivery.ErrorMessage = err.Error()
		if updateErr := s.store.UpdateScheduledTaskDelivery(ctx, delivery); updateErr != nil {
			s.logger.Error("update failed scheduled task delivery", "run_id", run.ScheduledRunID, "error", updateErr)
		}
		return
	}

	delivery.Status = ScheduledTaskDeliveryStatusSucceeded
	delivery.DiscordMessageID = messageID
	if err := s.store.UpdateScheduledTaskDelivery(ctx, delivery); err != nil {
		s.logger.Error("update successful scheduled task delivery", "run_id", run.ScheduledRunID, "error", err)
	}
}

func buildScheduledRunPrompt(task ScheduledTask, run ScheduledTaskRun) string {
	var builder strings.Builder
	builder.WriteString("Scheduled task: ")
	builder.WriteString(task.Name)
	builder.WriteString("\nDue time: ")
	builder.WriteString(run.ScheduledFor.Format(time.RFC3339))
	builder.WriteString("\n\n")
	builder.WriteString(task.Prompt)
	return builder.String()
}

func formatScheduledReport(task ScheduledTask, run ScheduledTaskRun) string {
	return fmt.Sprintf(
		"Scheduled task `%s` ran at `%s`.\n\n%s",
		task.Name,
		run.ScheduledFor.Format(time.RFC3339),
		strings.TrimSpace(run.ResponseText),
	)
}
