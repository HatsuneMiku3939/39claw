package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

var scheduledCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

var supportedScheduledAtLayouts = []string{
	"2006-01-02 15:04",
	time.RFC3339,
}

const maxScheduledTaskNameLength = 80

type ParsedSchedule interface {
	Next(time.Time) time.Time
}

func ParseScheduledTaskSchedule(task ScheduledTask, location *time.Location) (ParsedSchedule, error) {
	if location == nil {
		return nil, fmt.Errorf("schedule timezone must not be nil")
	}

	switch task.ScheduleKind {
	case ScheduledTaskScheduleKindCron:
		spec := strings.TrimSpace(task.ScheduleExpr)
		if spec == "" {
			return nil, fmt.Errorf("cron schedule must not be empty")
		}

		parsed, err := scheduledCronParser.Parse(spec)
		if err != nil {
			return nil, fmt.Errorf("parse cron schedule: %w", err)
		}

		return scheduledCron{schedule: parsed, location: location}, nil
	case ScheduledTaskScheduleKindAt:
		atTime, err := ParseScheduledTaskAt(task.ScheduleExpr, location)
		if err != nil {
			return nil, err
		}

		return scheduledAt{at: atTime}, nil
	default:
		return nil, fmt.Errorf("unsupported schedule kind %q", task.ScheduleKind)
	}
}

func ValidateScheduledTaskDefinition(task ScheduledTask, location *time.Location, defaultReportChannelID string) error {
	name := strings.TrimSpace(task.Name)
	if name == "" {
		return fmt.Errorf("scheduled task name must not be empty")
	}

	if len(name) > maxScheduledTaskNameLength {
		return fmt.Errorf("scheduled task name must be 80 characters or fewer")
	}

	if strings.TrimSpace(task.Prompt) == "" {
		return fmt.Errorf("scheduled task prompt must not be empty")
	}

	if _, err := ParseScheduledTaskSchedule(task, location); err != nil {
		return err
	}

	if task.Enabled && ResolveScheduledTaskReportChannel(task, defaultReportChannelID) == "" {
		return fmt.Errorf("enabled scheduled tasks require report_channel_id or CLAW_SCHEDULED_REPORT_CHANNEL_ID")
	}

	return nil
}

func ResolveScheduledTaskReportChannel(task ScheduledTask, defaultReportChannelID string) string {
	if reportChannelID := strings.TrimSpace(task.ReportChannelID); reportChannelID != "" {
		return reportChannelID
	}

	return strings.TrimSpace(defaultReportChannelID)
}

func ParseScheduledTaskAt(expr string, location *time.Location) (time.Time, error) {
	if location == nil {
		return time.Time{}, fmt.Errorf("schedule timezone must not be nil")
	}

	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("at schedule must not be empty")
	}

	var lastErr error
	for _, layout := range supportedScheduledAtLayouts {
		parsed, err := time.ParseInLocation(layout, trimmed, location)
		if err == nil {
			return parsed, nil
		}
		lastErr = err
	}

	return time.Time{}, fmt.Errorf("parse at schedule %q: %w", expr, lastErr)
}

type scheduledCron struct {
	schedule cron.Schedule
	location *time.Location
}

func (s scheduledCron) Next(after time.Time) time.Time {
	return s.schedule.Next(after.In(s.location))
}

type scheduledAt struct {
	at time.Time
}

func (s scheduledAt) Next(after time.Time) time.Time {
	if !s.at.After(after) {
		return time.Time{}
	}

	return s.at
}
