from __future__ import annotations

import shutil
import subprocess
from pathlib import Path
from typing import Any

from pydantic import BaseModel, ConfigDict, Field, ValidationError, field_validator
from typing_extensions import override

from pypimono.engine.domain.ports.tool import OnToolUpdate, Tool, ToolResult
from pypimono.engine.infra.workspace_fs.port import WorkspaceFsGateway


class FindToolArgs(BaseModel):
    model_config = ConfigDict(extra="ignore")

    pattern: str = Field(min_length=1)
    path: str = "."
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


class FindTool(Tool):
    name = "find"
    description = "Find files by glob pattern."
    parameters: dict[str, Any] = FindToolArgs.model_json_schema()

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
            parsed = FindToolArgs.model_validate(args)
        except ValidationError as exc:
            raise _format_validation_error(self.name, exc) from exc

        root = Path(self._fs.workspace_root())
        base = Path(self._fs.resolve_path(parsed.path))

        results = self._find_with_rg(root, base, parsed.pattern)
        if results is None:
            results = self._find_with_python(root, base, parsed.pattern)

        total = len(results)
        if total == 0:
            return ToolResult(text=f"(find) no files match: {parsed.pattern}")

        limited = results[: parsed.limit]
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

    def _find_with_rg(self, root: Path, base: Path, pattern: str) -> list[str] | None:
        if shutil.which("rg") is None:
            return None

        rel_base = base.relative_to(root).as_posix()
        cmd = ["rg", "--files", "-g", pattern]
        if rel_base != ".":
            cmd.append(rel_base)

        completed = subprocess.run(cmd, cwd=str(root), capture_output=True, text=True, check=False)
        if completed.returncode not in (0, 1):
            return None

        lines = [line.strip() for line in completed.stdout.splitlines() if line.strip()]
        lines.sort()
        return lines

    def _find_with_python(self, root: Path, base: Path, pattern: str) -> list[str]:
        results: list[str] = []

        if base.is_file():
            if base.match(pattern):
                results.append(base.relative_to(root).as_posix())
            return results

        for candidate in base.rglob(pattern):
            if candidate.is_file():
                results.append(candidate.relative_to(root).as_posix())

        results.sort()
        return results
