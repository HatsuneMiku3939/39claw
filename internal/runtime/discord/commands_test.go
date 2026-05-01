package discord

import (
	"strings"
	"testing"

	"github.com/HatsuneMiku3939/39claw/internal/config"
)

func TestUnsupportedActionText(t *testing.T) {
	t.Parallel()

	journal := unsupportedActionText("miku", config.ModeJournal)
	for _, want := range []string{"action:help", "action:clear"} {
		if !strings.Contains(journal, want) {
			t.Fatalf("journal unsupported action text = %q, want substring %q", journal, want)
		}
	}

	thread := unsupportedActionText("miku", config.ModeThread)
	for _, want := range []string{
		"action:help",
		"action:task-current",
		"action:task-list",
		"action:task-new task_name:<name>",
		"action:task-switch task_name:<name>",
		"action:task-close task_name:<name>",
		"action:task-reset-context",
	} {
		if !strings.Contains(thread, want) {
			t.Fatalf("thread unsupported action text = %q, want substring %q", thread, want)
		}
	}
}
