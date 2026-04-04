from __future__ import annotations

import re
import shutil
import subprocess
from pathlib import Path
from typing import Any

from pydantic import BaseModel, ConfigDict, Field, ValidationError, field_validator
from typing_extensions import override

from pypimono.engine.domain.ports.tool import OnToolUpdate, Tool, ToolResult
from pypimono.engine.infra.workspace_fs.port import WorkspaceFsGateway


class GrepToolArgs(BaseModel):
    model_config = ConfigDict(extra="ignore")

    pattern: str = Field(min_length=1)
    path: str = "."
    case_sensitive: bool = False
    fixed_strings: bool = False
    limit: int = Field(default=200, ge=1, le=2000)

    @field_validator("pattern", mode="before")
    @classmethod
    def _normalize_pattern(cls, value: Any) -> str:
        return str(value).strip() if value is not None else ""

    @field_validator("path", mode="before")
    @classmethod
    def _normalize_path(cls, value: Any) -> str:
        text = str(value).strip() if value is not None else ""
        return text or "."


def _format_validation_error(tool_name: str, exc: ValidationError) -> ValueError:
    details = []
    for error in exc.errors(include_url=False):
        loc = ".".join(str(part) for part in error.get("loc", ()))
        msg = str(error.get("msg", "invalid value"))
        details.append(f"{loc}: {msg}" if loc else msg)
    message = "; ".join(details) if details else "invalid args"
    return ValueError(f"{tool_name}: {message}")


class GrepTool(Tool):
    name = "grep"
    description = "Search file contents for patterns."
    parameters: dict[str, Any] = GrepToolArgs.model_json_schema()

    def __init__(self, fs: WorkspaceFsGateway):
        self._fs = fs

    @override
    async def execute(
        self,
        tool_call_id: str,
        args: dict[str, Any],
        *,
        on_update: OnToolUpdate | None = None,
    ) -> ToolResult:
        del tool_call_id, on_update

        try:
            parsed = GrepToolArgs.model_validate(args)
        except ValidationError as exc:
            raise _format_validation_error(self.name, exc) from exc

        root = Path(self._fs.workspace_root())
        base = Path(self._fs.resolve_path(parsed.path))

        lines = self._grep_with_rg(root, base, parsed)
        if lines is None:
            lines = self._grep_with_python(root, base, parsed)

        total = len(lines)
        if total == 0:
            return ToolResult(text=f"(grep) no matches: {parsed.pattern}")

        limited = lines[: parsed.limit]
        if total > parsed.limit:
            limited.append(f"... ({total - parsed.limit} more)")

        return ToolResult(
            text="\n".join(limited),
            details={
                "pattern": parsed.pattern,
                "path": parsed.path,
                "total": total,
                "truncated": total > parsed.limit,
            },
        )

    def _grep_with_rg(self, root: Path, base: Path, parsed: GrepToolArgs) -> list[str] | None:
        if shutil.which("rg") is None:
            return None

        rel_base = base.relative_to(root).as_posix()
        cmd = ["rg", "--line-number", "--no-heading"]
        if not parsed.case_sensitive:
            cmd.append("--ignore-case")
        if parsed.fixed_strings:
            cmd.append("--fixed-strings")
        cmd.append(parsed.pattern)
        cmd.append(rel_base if rel_base != "." else ".")

        completed = subprocess.run(cmd, cwd=str(root), capture_output=True, text=True, check=False)
        if completed.returncode not in (0, 1):
            return None

        return [line.rstrip() for line in completed.stdout.splitlines() if line.strip()]

    def _grep_with_python(self, root: Path, base: Path, parsed: GrepToolArgs) -> list[str]:
        paths = [base] if base.is_file() else [p for p in base.rglob("*") if p.is_file()]
        results: list[str] = []

        matcher = None
        if not parsed.fixed_strings:
            flags = 0 if parsed.case_sensitive else re.IGNORECASE
            matcher = re.compile(parsed.pattern, flags)

        for path in paths:
            # Skip large files for predictable latency.
            if path.stat().st_size > 1_000_000:
                continue
            try:
                text = path.read_text(encoding="utf-8", errors="ignore")
            except OSError:
                continue

            for lineno, line in enumerate(text.splitlines(), start=1):
                if parsed.fixed_strings:
                    haystack = line if parsed.case_sensitive else line.lower()
                    needle = parsed.pattern if parsed.case_sensitive else parsed.pattern.lower()
                    matched = needle in haystack
                else:
                    assert matcher is not None
                    matched = matcher.search(line) is not None

                if matched:
                    rel = path.relative_to(root).as_posix()
                    results.append(f"{rel}:{lineno}:{line}")

        return results
