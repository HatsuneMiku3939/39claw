package discord

import (
	"strings"
	"testing"

	"github.com/HatsuneMiku3939/39claw/internal/config"
)

func TestUnsupportedActionText(t *testing.T) {
	t.Parallel()

	daily := unsupportedActionText("miku", config.ModeDaily)
	for _, want := range []string{"action:help", "action:clear"} {
		if !strings.Contains(daily, want) {
			t.Fatalf("daily unsupported action text = %q, want substring %q", daily, want)
		}
	}

	task := unsupportedActionText("miku", config.ModeTask)
	for _, want := range []string{
		"action:help",
		"action:task-current",
		"action:task-list",
		"action:task-new task_name:<name>",
		"action:task-switch task_name:<name>",
		"action:task-close task_name:<name>",
		"action:task-reset-context",
	} {
		if !strings.Contains(task, want) {
			t.Fatalf("task unsupported action text = %q, want substring %q", task, want)
		}
	}
}
