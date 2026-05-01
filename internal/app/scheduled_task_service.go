package app

import (
	"context"
	"errors"
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
	scheduledTaskPersistenceTimeout  = 5 * time.Second
)

type ScheduledTaskServiceDependencies struct {
	Mode                config.Mode
	Timezone            *time.Location
	Workdir             string
	DefaultReportTarget string
	Store               ScheduledTaskStore
	Gateway             CodexGateway
	ReportSender        ScheduledTaskReportSender
	WorkspaceManager    ScheduledTaskWorkspaceManager
	Logger              *slog.Logger
	TickInterval        time.Duration
	Now                 func() time.Time
	StartedAt           time.Time
	NewRunID            func() string
	NewDeliveryID       func() string
}

type ScheduledTaskService struct {
	mode                config.Mode
	timezone            *time.Location
	workdir             string
	defaultReportTarget string
	store               ScheduledTaskStore
	gateway             CodexGateway
	reportSender        ScheduledTaskReportSender
	workspaceManager    ScheduledTaskWorkspaceManager
	logger              *slog.Logger
	tickInterval        time.Duration
	now                 func() time.Time
	newRunID            func() string
	newDeliveryID       func() string
	startedAt           time.Time

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
		now = func() time.Time {
			return time.Now().UTC()
		}
	}

	newRunID := deps.NewRunID
	if newRunID == nil {
		newRunID = func() string { return ulid.Make().String() }
	}

	newDeliveryID := deps.NewDeliveryID
	if newDeliveryID == nil {
		newDeliveryID = func() string { return ulid.Make().String() }
	}

	if deps.Mode == config.ModeThread && deps.WorkspaceManager == nil {
		return nil, fmt.Errorf("thread mode scheduled execution requires a workspace manager")
	}

	startedAt := deps.StartedAt
	if !startedAt.IsZero() {
		startedAt = startedAt.In(deps.Timezone)
	}

	return &ScheduledTaskService{
		mode:                deps.Mode,
		timezone:            deps.Timezone,
		workdir:             deps.Workdir,
		defaultReportTarget: deps.DefaultReportTarget,
		store:               deps.Store,
		gateway:             deps.Gateway,
		reportSender:        deps.ReportSender,
		workspaceManager:    deps.WorkspaceManager,
		logger:              logger,
		tickInterval:        tickInterval,
		now:                 now,
		startedAt:           startedAt,
		newRunID:            newRunID,
		newDeliveryID:       newDeliveryID,
	}, nil
}

func (s *ScheduledTaskService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return fmt.Errorf("scheduled task service already started")
	}
	if s.startedAt.IsZero() {
		s.startedAt = s.now().In(s.timezone)
	}

	//nolint:gosec // The service keeps the cancel function and calls it during Close.
	loopCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.started = true
	s.loopWG.Add(1)
	s.logger.Info(
		"scheduled task service started",
		"mode", s.mode,
		"tick_interval", s.tickInterval.String(),
		"timezone", s.timezone.String(),
	)
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
		s.logger.Info("scheduled task service stopped", "mode", s.mode)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *ScheduledTaskService) ExecuteTaskNow(ctx context.Context, taskName string) (ScheduledTaskRun, error) {
	trimmedName := strings.TrimSpace(taskName)
	if trimmedName == "" {
		return ScheduledTaskRun{}, fmt.Errorf("scheduled task name must not be empty")
	}

	task, ok, err := s.store.GetScheduledTaskByName(ctx, trimmedName)
	if err != nil {
		return ScheduledTaskRun{}, fmt.Errorf("load scheduled task %q: %w", trimmedName, err)
	}
	if !ok {
		return ScheduledTaskRun{}, fmt.Errorf("scheduled task %q was not found", trimmedName)
	}

	run := ScheduledTaskRun{
		ScheduledRunID:  s.newRunID(),
		ScheduledTaskID: task.ScheduledTaskID,
		Mode:            string(s.mode),
		ScheduledFor:    s.now().UTC(),
		Attempt:         1,
		Status:          ScheduledTaskRunStatusPending,
	}
	s.logger.Info(
		"scheduled task execute-now requested",
		"task", task.Name,
		"task_id", task.ScheduledTaskID,
		"scheduled_for", run.ScheduledFor,
	)

	admittedRun, admitted, err := s.store.AdmitScheduledTaskRun(ctx, run)
	if err != nil {
		return ScheduledTaskRun{}, fmt.Errorf("admit execute-now scheduled task run: %w", err)
	}
	if !admitted {
		return ScheduledTaskRun{}, fmt.Errorf("scheduled task %q execute-now run was not admitted", trimmedName)
	}

	s.runWG.Add(1)
	defer s.runWG.Done()

	return s.executeRun(ctx, task, admittedRun), nil
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
	now := s.now().In(s.timezone)
	s.logger.Debug("scheduled task tick started", "mode", s.mode, "now", now)

	tasks, err := s.store.ListEnabledScheduledTasks(ctx)
	if err != nil {
		s.logger.Error("list enabled scheduled tasks", "error", err)
		return
	}

	totalDue := 0
	totalAdmitted := 0
	for _, task := range tasks {
		taskLogger := s.logger.With("task", task.Name, "task_id", task.ScheduledTaskID)
		schedule, err := ParseScheduledTaskSchedule(task, s.timezone)
		if err != nil {
			taskLogger.Error("parse scheduled task definition", "error", err)
			continue
		}

		anchor := task.CreatedAt.In(s.timezone).Add(-time.Second)
		latestRun, ok, err := s.store.GetLatestScheduledTaskRunForTask(ctx, task.ScheduledTaskID)
		if err != nil {
			taskLogger.Error("load latest scheduled task run", "error", err)
			continue
		}
		if ok {
			anchor = latestRun.ScheduledFor.In(s.timezone)
		}
		if task.ScheduleKind == ScheduledTaskScheduleKindCron && !s.startedAt.IsZero() {
			startFloor := s.startedAt.Add(-time.Nanosecond)
			if anchor.Before(startFloor) {
				taskLogger.Debug(
					"scheduled task advanced anchor to scheduler start",
					"previous_anchor", anchor,
					"scheduler_started_at", s.startedAt,
				)
				anchor = startFloor
			}
		}
		taskLogger.Debug(
			"scheduled task evaluation started",
			"schedule_kind", task.ScheduleKind,
			"schedule_expr", task.ScheduleExpr,
			"anchor", anchor,
			"now", now,
		)

		var latestDue time.Time
		skippedBackfillCount := 0
		for due := schedule.Next(anchor); !due.IsZero() && !due.After(now); due = schedule.Next(due) {
			totalDue++
			if !latestDue.IsZero() {
				skippedBackfillCount++
			}
			latestDue = due
		}

		if latestDue.IsZero() {
			taskLogger.Debug("scheduled task evaluation found no due run")
			continue
		}

		taskLogger.Info("scheduled task due run identified", "scheduled_for", latestDue)
		if skippedBackfillCount > 0 {
			taskLogger.Debug(
				"scheduled task skipped overdue backfill runs",
				"latest_scheduled_for", latestDue,
				"skipped_count", skippedBackfillCount,
			)
		}

		run, admitted, err := s.store.AdmitScheduledTaskRun(ctx, ScheduledTaskRun{
			ScheduledRunID:  s.newRunID(),
			ScheduledTaskID: task.ScheduledTaskID,
			Mode:            string(s.mode),
			ScheduledFor:    latestDue.UTC(),
			Attempt:         1,
			Status:          ScheduledTaskRunStatusPending,
		})
		if err != nil {
			taskLogger.Error("admit scheduled task run", "scheduled_for", latestDue, "error", err)
			continue
		}
		if !admitted {
			taskLogger.Info("scheduled task run already admitted", "scheduled_for", latestDue)
			continue
		}
		totalAdmitted++
		taskLogger.Info("scheduled task run admitted", "scheduled_for", latestDue, "run_id", run.ScheduledRunID)

		s.runWG.Add(1)
		go func(task ScheduledTask, run ScheduledTaskRun) {
			defer s.runWG.Done()
			s.executeRun(ctx, task, run)
		}(task, run)
	}

	s.logger.Debug(
		"scheduled task tick finished",
		"mode", s.mode,
		"task_count", len(tasks),
		"due_count", totalDue,
		"admitted_count", totalAdmitted,
	)
}

func (s *ScheduledTaskService) executeRun(ctx context.Context, task ScheduledTask, run ScheduledTaskRun) ScheduledTaskRun {
	runLogger := s.logger.With(
		"task", task.Name,
		"task_id", task.ScheduledTaskID,
		"run_id", run.ScheduledRunID,
		"scheduled_for", run.ScheduledFor,
	)
	runLogger.Info("scheduled task run started", "mode", run.Mode, "attempt", run.Attempt)

	startedAt := s.now()
	run.Status = ScheduledTaskRunStatusRunning
	run.StartedAt = &startedAt
	run.UpdatedAt = startedAt
	if err := s.store.UpdateScheduledTaskRun(ctx, run); err != nil {
		runLogger.Error("mark scheduled task run started", "error", err)
		return run
	}

	workdir := s.workdir
	cleanup := func(context.Context) error { return nil }
	if s.mode == config.ModeThread {
		runLogger.Info("scheduled task worktree preparation started")
		tempWorktreePath, cleanupFn, err := s.workspaceManager.PrepareTemporaryWorktree(ctx, run.ScheduledRunID)
		if err != nil {
			status := ScheduledTaskRunStatusFailed
			shouldRetry := true
			errorCode := "workspace_prepare_failed"
			if isCancellationError(err) || ctx.Err() != nil {
				status = ScheduledTaskRunStatusCanceled
				shouldRetry = false
				errorCode = "workspace_prepare_canceled"
			}
			return s.failRun(ctx, task, run, status, errorCode, err.Error(), shouldRetry)
		}
		workdir = tempWorktreePath
		run.TempWorktreePath = tempWorktreePath
		run.WorkdirPath = tempWorktreePath
		cleanup = cleanupFn
		runLogger.Info("scheduled task worktree prepared", "workdir", workdir)
	} else {
		run.WorkdirPath = s.workdir
	}

	prompt := buildScheduledRunPrompt(task, run)
	runLogger.Info("scheduled task Codex run started", "workdir", workdir)
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
		status := ScheduledTaskRunStatusFailed
		shouldRetry := true
		errorCode := "codex_run_failed"
		if isCancellationError(err) || ctx.Err() != nil {
			status = ScheduledTaskRunStatusCanceled
			shouldRetry = false
			errorCode = "codex_run_canceled"
		}
		run = s.failRun(ctx, task, run, status, errorCode, err.Error(), shouldRetry)
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), scheduledTaskCleanupTimeout)
		if cleanupErr := cleanup(cleanupCtx); cleanupErr != nil {
			runLogger.Error("cleanup scheduled task worktree after failure", "error", cleanupErr)
		}
		cancel()
		return run
	}
	runLogger.Info(
		"scheduled task Codex run finished",
		"thread_id", run.CodexThreadID,
		"response_char_count", len(run.ResponseText),
	)

	run.Status = ScheduledTaskRunStatusSucceeded
	if updateErr := s.store.UpdateScheduledTaskRun(ctx, run); updateErr != nil {
		runLogger.Error("persist scheduled task run success", "error", updateErr)
	}

	s.deliverRunResult(ctx, task, run)

	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), scheduledTaskCleanupTimeout)
	if cleanupErr := cleanup(cleanupCtx); cleanupErr != nil {
		run.ErrorCode = "workspace_cleanup_failed"
		run.ErrorMessage = cleanupErr.Error()
		run.UpdatedAt = s.now()
		if updateErr := s.store.UpdateScheduledTaskRun(ctx, run); updateErr != nil {
			runLogger.Error("persist scheduled run cleanup failure", "error", updateErr)
		}
		runLogger.Error("cleanup scheduled task worktree", "error", cleanupErr)
	}
	cancel()
	runLogger.Info("scheduled task run finished", "status", run.Status)
	return run
}

func (s *ScheduledTaskService) failRun(
	ctx context.Context,
	task ScheduledTask,
	run ScheduledTaskRun,
	status ScheduledTaskRunStatus,
	errorCode string,
	errorMessage string,
	mayRetry bool,
) ScheduledTaskRun {
	runLogger := s.logger.With(
		"task", task.Name,
		"task_id", task.ScheduledTaskID,
		"run_id", run.ScheduledRunID,
		"scheduled_for", run.ScheduledFor,
	)
	finishedAt := s.now()
	run.Status = status
	run.ErrorCode = errorCode
	run.ErrorMessage = errorMessage
	run.FinishedAt = &finishedAt
	run.UpdatedAt = finishedAt
	persistCtx, cancel := detachedStoreContext(ctx)
	defer cancel()
	if err := s.store.UpdateScheduledTaskRun(persistCtx, run); err != nil {
		runLogger.Error("persist scheduled task run failure", "error", err)
	}
	runLogger.Error("scheduled task run failed", "error_code", errorCode, "error_message", errorMessage)

	if mayRetry && status == ScheduledTaskRunStatusFailed && run.Attempt < 2 {
		retryRun, admitted, err := s.store.AdmitScheduledTaskRun(persistCtx, ScheduledTaskRun{
			ScheduledRunID:  s.newRunID(),
			ScheduledTaskID: run.ScheduledTaskID,
			Mode:            run.Mode,
			ScheduledFor:    run.ScheduledFor,
			Attempt:         run.Attempt + 1,
			Status:          ScheduledTaskRunStatusPending,
		})
		if err != nil {
			runLogger.Error("admit scheduled task retry run", "error", err)
			return run
		}
		if admitted {
			runLogger.Info("scheduled task retry run admitted", "retry_run_id", retryRun.ScheduledRunID, "attempt", retryRun.Attempt)
			s.runWG.Add(1)
			go func() {
				defer s.runWG.Done()
				s.executeRun(ctx, task, retryRun)
			}()
		}
	}

	return run
}

func detachedStoreContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx.Err() == nil {
		return ctx, func() {}
	}

	return context.WithTimeout(context.WithoutCancel(ctx), scheduledTaskPersistenceTimeout)
}

func isCancellationError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func (s *ScheduledTaskService) deliverRunResult(ctx context.Context, task ScheduledTask, run ScheduledTaskRun) {
	reportTarget := ResolveScheduledTaskReportTarget(task, s.defaultReportTarget)
	deliveryLogger := s.logger.With(
		"task", task.Name,
		"task_id", task.ScheduledTaskID,
		"run_id", run.ScheduledRunID,
		"report_target", reportTarget,
	)
	delivery := ScheduledTaskDelivery{
		ScheduledDeliveryID: s.newDeliveryID(),
		ScheduledRunID:      run.ScheduledRunID,
		ReportTarget:        reportTarget,
		Status:              ScheduledTaskDeliveryStatusPending,
	}

	if reportTarget == "" {
		now := s.now()
		delivery.Status = ScheduledTaskDeliveryStatusSkipped
		delivery.DeliveredAt = &now
		delivery.ErrorCode = "missing_report_target"
		delivery.ErrorMessage = "scheduled task does not resolve to a report target"
		if err := s.store.CreateScheduledTaskDelivery(ctx, delivery); err != nil {
			deliveryLogger.Error("record skipped scheduled task delivery", "error", err)
		}
		deliveryLogger.Info("scheduled task delivery skipped", "reason", delivery.ErrorCode)
		return
	}

	if err := s.store.CreateScheduledTaskDelivery(ctx, delivery); err != nil {
		deliveryLogger.Error("create scheduled task delivery", "error", err)
		return
	}
	deliveryLogger.Info("scheduled task delivery started")

	messageID, err := s.reportSender.SendScheduledReport(ctx, reportTarget, formatScheduledReport(task, run))
	now := s.now()
	delivery.DeliveredAt = &now
	delivery.UpdatedAt = now
	if err != nil {
		delivery.Status = ScheduledTaskDeliveryStatusFailed
		delivery.ErrorCode = "discord_delivery_failed"
		delivery.ErrorMessage = err.Error()
		if updateErr := s.store.UpdateScheduledTaskDelivery(ctx, delivery); updateErr != nil {
			deliveryLogger.Error("update failed scheduled task delivery", "error", updateErr)
		}
		deliveryLogger.Error("scheduled task delivery failed", "error_message", delivery.ErrorMessage)
		return
	}

	delivery.Status = ScheduledTaskDeliveryStatusSucceeded
	delivery.DiscordMessageID = messageID
	if err := s.store.UpdateScheduledTaskDelivery(ctx, delivery); err != nil {
		deliveryLogger.Error("update successful scheduled task delivery", "error", err)
	}
	deliveryLogger.Info("scheduled task delivery finished", "message_id", messageID, "status", delivery.Status)
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
