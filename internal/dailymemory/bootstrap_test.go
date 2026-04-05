package dailymemory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrapEnsureCreatesManagedArtifactsIdempotently(t *testing.T) {
	t.Parallel()

	workdir := t.TempDir()
	bootstrap := Bootstrap{Workdir: workdir}

	if err := bootstrap.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure() first error = %v", err)
	}

	if err := bootstrap.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure() second error = %v", err)
	}

	memoryPath := filepath.Join(workdir, memoryDirName, memoryFileName)
	memoryContents, err := os.ReadFile(memoryPath)
	if err != nil {
		t.Fatalf("ReadFile(MEMORY.md) error = %v", err)
	}

	if string(memoryContents) != initialMemoryContents {
		t.Fatalf("MEMORY.md contents = %q, want %q", string(memoryContents), initialMemoryContents)
	}

	skillPath := filepath.Join(workdir, managedSkillRelativePath)
	skillContents, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("ReadFile(SKILL.md) error = %v", err)
	}

	if string(skillContents) != managedSkillContents {
		t.Fatalf("SKILL.md contents = %q, want %q", string(skillContents), managedSkillContents)
	}

	agentsPath := filepath.Join(workdir, agentsFileName)
	agentsContents, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("ReadFile(AGENTS.md) error = %v", err)
	}

	if string(agentsContents) != managedAgentsBlock {
		t.Fatalf("AGENTS.md contents = %q, want %q", string(agentsContents), managedAgentsBlock)
	}
}

func TestBootstrapEnsureReplacesManagedBlockWithoutOverwritingUserContent(t *testing.T) {
	t.Parallel()

	workdir := t.TempDir()
	agentsPath := filepath.Join(workdir, agentsFileName)
	existing := strings.Join([]string{
		"# Team Notes",
		"",
		"Keep this instruction.",
		"",
		managedBlockStart,
		"outdated block",
		managedBlockEnd,
		"",
		"Tail note.",
		"",
	}, "\n")
	if err := os.WriteFile(agentsPath, []byte(existing), fileMode); err != nil {
		t.Fatalf("WriteFile(AGENTS.md) error = %v", err)
	}

	memoryPath := filepath.Join(workdir, memoryDirName, memoryFileName)
	if err := os.MkdirAll(filepath.Dir(memoryPath), directoryMode); err != nil {
		t.Fatalf("MkdirAll(memory dir) error = %v", err)
	}
	if err := os.WriteFile(memoryPath, []byte("custom memory"), fileMode); err != nil {
		t.Fatalf("WriteFile(MEMORY.md) error = %v", err)
	}

	if err := (Bootstrap{Workdir: workdir}).Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}

	updated, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("ReadFile(AGENTS.md) error = %v", err)
	}

	updatedText := string(updated)
	if !strings.Contains(updatedText, "# Team Notes") {
		t.Fatalf("updated AGENTS.md lost prefix content: %q", updatedText)
	}

	if !strings.Contains(updatedText, "Tail note.") {
		t.Fatalf("updated AGENTS.md lost suffix content: %q", updatedText)
	}

	if strings.Count(updatedText, managedBlockStart) != 1 {
		t.Fatalf("managed block start count = %d, want 1", strings.Count(updatedText, managedBlockStart))
	}

	if !strings.Contains(updatedText, managedAgentsBlock[:len(managedAgentsBlock)-1]) {
		t.Fatalf("updated AGENTS.md missing managed block: %q", updatedText)
	}

	memoryContents, err := os.ReadFile(memoryPath)
	if err != nil {
		t.Fatalf("ReadFile(MEMORY.md) error = %v", err)
	}

	if string(memoryContents) != "custom memory" {
		t.Fatalf("MEMORY.md contents = %q, want existing content to be preserved", string(memoryContents))
	}
}
