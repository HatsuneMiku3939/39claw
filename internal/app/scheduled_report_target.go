package app

import (
	"fmt"
	"strings"
)

type ScheduledReportTargetKind string

const (
	ScheduledReportTargetKindChannel ScheduledReportTargetKind = "channel"
	ScheduledReportTargetKindDM      ScheduledReportTargetKind = "dm"
)

type ScheduledReportTarget struct {
	Kind ScheduledReportTargetKind
	ID   string
}

func ParseScheduledReportTarget(raw string) (ScheduledReportTarget, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ScheduledReportTarget{}, fmt.Errorf("scheduled report target must not be empty")
	}

	kind, id, ok := strings.Cut(trimmed, ":")
	if !ok {
		return ScheduledReportTarget{}, fmt.Errorf("scheduled report target must use channel:<id> or dm:<user_id>")
	}

	normalizedKind := ScheduledReportTargetKind(strings.ToLower(strings.TrimSpace(kind)))
	normalizedID := strings.TrimSpace(id)
	if normalizedID == "" {
		return ScheduledReportTarget{}, fmt.Errorf("scheduled report target id must not be empty")
	}

	switch normalizedKind {
	case ScheduledReportTargetKindChannel, ScheduledReportTargetKindDM:
		return ScheduledReportTarget{
			Kind: normalizedKind,
			ID:   normalizedID,
		}, nil
	default:
		return ScheduledReportTarget{}, fmt.Errorf("scheduled report target kind must be channel or dm")
	}
}
