package dailymemory

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
)

const defaultRefreshTimeout = time.Minute

type Refresher struct {
	Timezone *time.Location
	Store    app.ThreadStore
	Gateway  app.CodexGateway
	Workdir  string
	Timeout  time.Duration
}

func (r Refresher) RefreshBeforeFirstDailyTurn(ctx context.Context, logicalKey string, _ time.Time) error {
	if r.Timezone == nil {
		return errors.New("daily memory refresher timezone must not be nil")
	}

	if r.Store == nil {
		return errors.New("daily memory refresher store must not be nil")
	}

	if r.Gateway == nil {
		return errors.New("daily memory refresher gateway must not be nil")
	}

	workdir := strings.TrimSpace(r.Workdir)
	if workdir == "" {
		return errors.New("daily memory refresher workdir must not be empty")
	}

	currentDate, err := time.ParseInLocation(time.DateOnly, logicalKey, r.Timezone)
	if err != nil {
		return fmt.Errorf("parse current daily logical key: %w", err)
	}

	if _, ok, err := r.Store.GetThreadBinding(ctx, "daily", logicalKey); err != nil {
		return fmt.Errorf("load current daily thread binding: %w", err)
	} else if ok {
		return nil
	}

	previousDate := currentDate.AddDate(0, 0, -1).Format(time.DateOnly)
	previousBinding, ok, err := r.Store.GetThreadBinding(ctx, "daily", previousDate)
	if err != nil {
		return fmt.Errorf("load previous daily thread binding: %w", err)
	}

	if !ok {
		return nil
	}

	memoryDir := filepath.Join(workdir, memoryDirName)
	if err := os.MkdirAll(memoryDir, directoryMode); err != nil {
		return fmt.Errorf("create memory directory: %w", err)
	}

	if err := ensureMemoryFile(memoryDir); err != nil {
		return err
	}

	memoryPath := filepath.Join(memoryDir, memoryFileName)
	bridgePath := filepath.Join(memoryDir, logicalKey+".md")
	if err := ensureBridgeNote(bridgePath, previousBinding.CodexThreadID, previousDate, logicalKey); err != nil {
		return err
	}

	refreshCtx := ctx
	cancel := func() {}
	if timeout := r.effectiveTimeout(); timeout > 0 {
		refreshCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	result, err := r.Gateway.RunTurn(refreshCtx, previousBinding.CodexThreadID, app.CodexTurnInput{
		Prompt:           buildRefreshPrompt(previousDate, logicalKey),
		WorkingDirectory: workdir,
	})
	if err != nil {
		return fmt.Errorf("run daily memory refresh turn: %w", err)
	}

	if err := validateRefreshResponse(result.ResponseText, memoryPath, bridgePath); err != nil {
		return err
	}

	return nil
}

func (r Refresher) effectiveTimeout() time.Duration {
	if r.Timeout != 0 {
		return r.Timeout
	}

	return defaultRefreshTimeout
}

func ensureBridgeNote(path string, previousThreadID string, previousDate string, currentDate string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat bridge note: %w", err)
	}

	content := strings.ReplaceAll(initialBridgeNoteTemplate, "YYYY-MM-DD", currentDate)
	content = strings.ReplaceAll(content, "<previous-thread-id>", previousThreadID)
	content = strings.ReplaceAll(content, "<source-day>", previousDate)

	if err := os.WriteFile(path, []byte(content), fileMode); err != nil {
		return fmt.Errorf("write bridge note: %w", err)
	}

	return nil
}

func buildRefreshPrompt(previousDate string, currentDate string) string {
	return fmt.Sprintf(
		"Before handling the first visible user message of the new daily thread, read `.agents/skills/39claw-daily-memory-refresh/SKILL.md` and follow it now.\n\nUse the resumed previous daily thread as the source of truth.\n\nToday's bridge note path is:\n- AGENT_MEMORY/%s.md\n\nThe primary durable memory file is:\n- AGENT_MEMORY/MEMORY.md\n\nThe previous local date is %s.\nThe new local date is %s.\n\nReturn the required completion format after the refresh is complete.",
		currentDate,
		previousDate,
		currentDate,
	)
}

func validateRefreshResponse(responseText string, memoryPath string, bridgePath string) error {
	normalized := strings.ReplaceAll(strings.TrimSpace(responseText), "\r\n", "\n")
	expected := "MEMORY_REFRESH_OK\nUpdated:\n- " + memoryPath + "\n- " + bridgePath
	if normalized != expected {
		return fmt.Errorf("unexpected daily memory refresh response: %q", responseText)
	}

	return nil
}

const initialBridgeNoteTemplate = "# Daily Memory Bridge for YYYY-MM-DD\n\n" +
	"## Source\n\n" +
	"- Previous thread id: `<previous-thread-id>`\n" +
	"- Source day: `<source-day>`\n\n" +
	"## Durable Facts Promoted\n\n" +
	"- None yet.\n\n" +
	"## MEMORY.md Updates Applied\n\n" +
	"- None yet.\n\n" +
	"## Rejected Candidates\n\n" +
	"- None yet.\n\n" +
	"## Notes\n\n" +
	"- Created by the 39claw daily memory preflight before the first visible turn of the new day.\n"
