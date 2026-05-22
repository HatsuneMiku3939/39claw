package dailymemory

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/HatsuneMiku3939/39claw/internal/app"
)

type Refresher struct {
	Store   app.ThreadStore
	Gateway app.CodexGateway
	Workdir string
}

func (r Refresher) RefreshBeforeFirstDailyTurn(ctx context.Context, session app.DailySession) error {
	if r.Store == nil {
		return errors.New("journal memory refresher store must not be nil")
	}

	if r.Gateway == nil {
		return errors.New("journal memory refresher gateway must not be nil")
	}

	workdir := strings.TrimSpace(r.Workdir)
	if workdir == "" {
		return errors.New("journal memory refresher workdir must not be empty")
	}

	if strings.TrimSpace(session.LogicalThreadKey) == "" {
		return errors.New("journal memory refresher logical thread key must not be empty")
	}

	if strings.TrimSpace(session.LocalDate) == "" || session.Generation < 1 {
		return errors.New("journal memory refresher session metadata is incomplete")
	}

	if _, ok, err := r.Store.GetThreadBinding(ctx, "journal", session.LogicalThreadKey); err != nil {
		return fmt.Errorf("load current journal thread binding: %w", err)
	} else if ok {
		return nil
	}

	if strings.TrimSpace(session.PreviousLogicalThreadKey) == "" {
		return nil
	}

	previousBinding, ok, err := r.Store.GetThreadBinding(ctx, "journal", session.PreviousLogicalThreadKey)
	if err != nil {
		return fmt.Errorf("load previous journal thread binding: %w", err)
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
	bridgeFilename := fmt.Sprintf("%s.%d.md", session.LocalDate, session.Generation)
	bridgePath := filepath.Join(memoryDir, bridgeFilename)
	if err := ensureBridgeNote(
		bridgePath,
		previousBinding.CodexThreadID,
		session.PreviousLogicalThreadKey,
		session.LogicalThreadKey,
	); err != nil {
		return err
	}

	result, err := r.Gateway.RunTurn(ctx, previousBinding.CodexThreadID, app.CodexTurnInput{
		Prompt:           buildRefreshPrompt(session.PreviousLogicalThreadKey, session.LogicalThreadKey, bridgeFilename),
		WorkingDirectory: workdir,
	})
	if err != nil {
		return fmt.Errorf("run journal memory refresh turn: %w", err)
	}

	if err := validateRefreshResponse(result.ResponseText, memoryPath, bridgePath); err != nil {
		return err
	}

	return nil
}

func ensureBridgeNote(path string, previousThreadID string, previousLogicalKey string, currentLogicalKey string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat bridge note: %w", err)
	}

	content := strings.ReplaceAll(initialBridgeNoteTemplate, "CURRENT-LOGICAL-KEY", currentLogicalKey)
	content = strings.ReplaceAll(content, "<previous-thread-id>", previousThreadID)
	content = strings.ReplaceAll(content, "<source-logical-key>", previousLogicalKey)

	if err := os.WriteFile(path, []byte(content), fileMode); err != nil {
		return fmt.Errorf("write bridge note: %w", err)
	}

	return nil
}

func buildRefreshPrompt(previousLogicalKey string, currentLogicalKey string, bridgeFilename string) string {
	return fmt.Sprintf(
		"Before handling the first visible user message of the new journal generation, read `.agents/skills/39claw-journal-memory-refresh/SKILL.md` and follow it now.\n\nUse the resumed previous journal generation as the source of truth.\n\nToday's bridge note path is:\n- AGENT_MEMORY/%s\n\nThe primary durable memory file is:\n- AGENT_MEMORY/MEMORY.md\n\nThe previous logical key is %s.\nThe new logical key is %s.\n\nReturn the required completion format after the refresh is complete.",
		bridgeFilename,
		previousLogicalKey,
		currentLogicalKey,
	)
}

func validateRefreshResponse(responseText string, memoryPath string, bridgePath string) error {
	normalized := strings.ReplaceAll(strings.TrimSpace(responseText), "\r\n", "\n")
	expected := "MEMORY_REFRESH_OK\nUpdated:\n- " + memoryPath + "\n- " + bridgePath
	if normalized != expected {
		return fmt.Errorf("unexpected journal memory refresh response: %q", responseText)
	}

	return nil
}

const initialBridgeNoteTemplate = "# Journal Memory Bridge for CURRENT-LOGICAL-KEY\n\n" +
	"## Source\n\n" +
	"- Previous thread id: `<previous-thread-id>`\n" +
	"- Source logical key: `<source-logical-key>`\n\n" +
	"## Durable Facts Promoted\n\n" +
	"- None yet.\n\n" +
	"## MEMORY.md Updates Applied\n\n" +
	"- None yet.\n\n" +
	"## Rejected Candidates\n\n" +
	"- None yet.\n\n" +
	"## Notes\n\n" +
	"- Created by the 39claw journal memory preflight before the first visible turn of the new generation.\n"
