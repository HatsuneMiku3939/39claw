package dailymemory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

	if _, err := os.Stat(filepath.Join(workdir, "AGENTS.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(AGENTS.md) error = %v, want not exist", err)
	}
}

func TestBootstrapEnsureDoesNotModifyExistingAgentsFile(t *testing.T) {
	t.Parallel()

	workdir := t.TempDir()
	agentsPath := filepath.Join(workdir, "AGENTS.md")
	existing := "# Team Notes\n\nKeep this instruction.\n"
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

	if string(updated) != existing {
		t.Fatalf("AGENTS.md contents = %q, want %q", string(updated), existing)
	}

	memoryContents, err := os.ReadFile(memoryPath)
	if err != nil {
		t.Fatalf("ReadFile(MEMORY.md) error = %v", err)
	}

	if string(memoryContents) != "custom memory" {
		t.Fatalf("MEMORY.md contents = %q, want existing content to be preserved", string(memoryContents))
	}
}
