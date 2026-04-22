package app

import (
	"testing"
	"time"
)

func TestParseScheduledTaskScheduleSupportsCronAndAt(t *testing.T) {
	t.Parallel()

	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}

	cronSchedule, err := ParseScheduledTaskSchedule(ScheduledTask{
		ScheduleKind: ScheduledTaskScheduleKindCron,
		ScheduleExpr: "5 14 * * *",
	}, location)
	if err != nil {
		t.Fatalf("ParseScheduledTaskSchedule(cron) error = %v", err)
	}

	after := time.Date(2026, time.April, 12, 14, 4, 0, 0, location)
	if got := cronSchedule.Next(after); !got.Equal(time.Date(2026, time.April, 12, 14, 5, 0, 0, location)) {
		t.Fatalf("cron Next() = %v, want %v", got, time.Date(2026, time.April, 12, 14, 5, 0, 0, location))
	}

	atSchedule, err := ParseScheduledTaskSchedule(ScheduledTask{
		ScheduleKind: ScheduledTaskScheduleKindAt,
		ScheduleExpr: "2026-04-12 15:30",
	}, location)
	if err != nil {
		t.Fatalf("ParseScheduledTaskSchedule(at) error = %v", err)
	}

	wantAt := time.Date(2026, time.April, 12, 15, 30, 0, 0, location)
	if got := atSchedule.Next(after); !got.Equal(wantAt) {
		t.Fatalf("at Next() = %v, want %v", got, wantAt)
	}
	if got := atSchedule.Next(wantAt); !got.IsZero() {
		t.Fatalf("at Next() after due time = %v, want zero", got)
	}
}

func TestValidateScheduledTaskDefinitionRequiresReportTargetWhenEnabled(t *testing.T) {
	t.Parallel()

	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}

	err = ValidateScheduledTaskDefinition(ScheduledTask{
		Name:         "Daily report",
		ScheduleKind: ScheduledTaskScheduleKindCron,
		ScheduleExpr: "0 9 * * *",
		Prompt:       "Write the report.",
		Enabled:      true,
	}, location, "")
	if err == nil {
		t.Fatal("ValidateScheduledTaskDefinition() error = nil, want report target error")
	}

	if err := ValidateScheduledTaskDefinition(ScheduledTask{
		Name:         "Daily report",
		ScheduleKind: ScheduledTaskScheduleKindCron,
		ScheduleExpr: "0 9 * * *",
		Prompt:       "Write the report.",
		Enabled:      true,
	}, location, "channel:12345"); err != nil {
		t.Fatalf("ValidateScheduledTaskDefinition() with default report target error = %v", err)
	}
}

func TestValidateScheduledTaskDefinitionRejectsInvalidReportTarget(t *testing.T) {
	t.Parallel()

	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}

	err = ValidateScheduledTaskDefinition(ScheduledTask{
		Name:         "Daily report",
		ScheduleKind: ScheduledTaskScheduleKindCron,
		ScheduleExpr: "0 9 * * *",
		Prompt:       "Write the report.",
		ReportTarget: "mail:user-1",
	}, location, "")
	if err == nil {
		t.Fatal("ValidateScheduledTaskDefinition() error = nil, want invalid report target error")
	}
}
