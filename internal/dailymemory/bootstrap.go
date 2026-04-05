package dailymemory

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	managedSkillRelativePath = ".agents/skills/39claw-daily-memory-refresh/SKILL.md"
	memoryDirName            = "AGENT_MEMORY"
	memoryFileName           = "MEMORY.md"

	directoryMode = 0o755
	fileMode      = 0o644
)

type Bootstrap struct {
	Workdir   string
	MemoryDir string
}

func (b Bootstrap) Ensure(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	workdir := strings.TrimSpace(b.Workdir)
	if workdir == "" {
		return errors.New("daily memory bootstrap workdir must not be empty")
	}

	info, err := os.Stat(workdir)
	if err != nil {
		return fmt.Errorf("stat daily memory workdir: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("daily memory workdir must be a directory: %s", workdir)
	}

	memoryDir := b.resolvedMemoryDir()
	if err := os.MkdirAll(memoryDir, directoryMode); err != nil {
		return fmt.Errorf("create memory directory: %w", err)
	}

	if err := ensureMemoryFile(memoryDir); err != nil {
		return err
	}

	skillPath := filepath.Join(workdir, managedSkillRelativePath)
	if err := os.MkdirAll(filepath.Dir(skillPath), directoryMode); err != nil {
		return fmt.Errorf("create managed skill directory: %w", err)
	}

	if err := os.WriteFile(skillPath, []byte(managedSkillContents), fileMode); err != nil {
		return fmt.Errorf("write managed skill: %w", err)
	}

	return nil
}

func (b Bootstrap) resolvedMemoryDir() string {
	if strings.TrimSpace(b.MemoryDir) != "" {
		return b.MemoryDir
	}

	return filepath.Join(strings.TrimSpace(b.Workdir), memoryDirName)
}

func ensureMemoryFile(memoryDir string) error {
	memoryPath := filepath.Join(memoryDir, memoryFileName)
	if _, err := os.Stat(memoryPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat memory file: %w", err)
	}

	if err := os.WriteFile(memoryPath, []byte(initialMemoryContents), fileMode); err != nil {
		return fmt.Errorf("write memory file: %w", err)
	}

	return nil
}

const managedSkillContents = "---\n" +
	"name: 39claw-daily-memory-refresh\n" +
	"description: Refresh durable Markdown memory for 39claw daily mode before the first visible turn of a new local day. Use when the runtime resumes the previous daily Codex thread and needs to update the durable memory files under AGENT_MEMORY.\n" +
	"---\n\n" +
	"# 39claw Daily Memory Refresh\n\n" +
	"## Purpose\n\n" +
	"Refresh durable memory before the first visible turn of a new local day in 39claw daily mode.\n\n" +
	"The source of truth is the resumed previous daily thread.\n" +
	"The writable memory directory is `AGENT_MEMORY/` under the current workspace.\n\n" +
	"## Files\n\n" +
	"Read and update these files:\n\n" +
	"- `AGENT_MEMORY/MEMORY.md`\n" +
	"- the dated bridge note path provided in the runtime prompt\n\n" +
	"Treat `MEMORY.md` as the primary durable memory file.\n" +
	"Treat the dated bridge note as a record of what was promoted or rejected during today's refresh.\n\n" +
	"## Rules\n\n" +
	"- Preserve only durable facts that are likely to matter on a future day.\n" +
	"- Prefer explicit user statements over inferred conclusions.\n" +
	"- Do not store transient chatter, jokes, or temporary TODO items.\n" +
	"- Update existing memory instead of appending duplicate facts.\n" +
	"- Keep `MEMORY.md` concise and current.\n" +
	"- If a new fact replaces an older one, revise the older wording in `MEMORY.md`.\n" +
	"- If memory conflicts with the latest explicit user instruction, the latest explicit user instruction wins.\n\n" +
	"## Required `MEMORY.md` structure\n\n" +
	"Ensure `MEMORY.md` uses exactly these top-level headings:\n\n" +
	"- `# Memory`\n" +
	"- `## User Preferences`\n" +
	"- `## Workflow Preferences`\n" +
	"- `## Active Long-Lived Context`\n" +
	"- `## Superseded or Historical Notes`\n\n" +
	"Keep each section short and scannable.\n" +
	"Use flat bullet lists under the sections.\n\n" +
	"## Required dated bridge note structure\n\n" +
	"Ensure today's dated note uses exactly these top-level headings:\n\n" +
	"- `# Daily Memory Bridge for YYYY-MM-DD`\n" +
	"- `## Source`\n" +
	"- `## Durable Facts Promoted`\n" +
	"- `## MEMORY.md Updates Applied`\n" +
	"- `## Rejected Candidates`\n" +
	"- `## Notes`\n\n" +
	"The `## Source` section must name the previous thread ID and the previous local date.\n\n" +
	"## Completion format\n\n" +
	"After updating the files, reply with plain text in exactly this shape:\n\n" +
	"    MEMORY_REFRESH_OK\n" +
	"    Updated:\n" +
	"    - <absolute path to MEMORY.md>\n" +
	"    - <absolute path to today's dated note>\n\n" +
	"If no durable facts changed, still return the same format and list both files.\n"

const initialMemoryContents = `# Memory

## User Preferences

- None recorded yet.

## Workflow Preferences

- None recorded yet.

## Active Long-Lived Context

- None recorded yet.

## Superseded or Historical Notes

- None recorded yet.
`
