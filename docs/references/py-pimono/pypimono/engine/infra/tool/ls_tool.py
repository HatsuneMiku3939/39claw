from __future__ import annotations

from pathlib import Path
from typing import Any

from pydantic import BaseModel, ConfigDict, Field, ValidationError, field_validator
from typing_extensions import override

from pypimono.engine.domain.ports.tool import OnToolUpdate, Tool, ToolResult
from pypimono.engine.infra.workspace_fs.port import WorkspaceFsGateway


class LsToolArgs(BaseModel):
    model_config = ConfigDict(extra="ignore")

    path: str = "."
    recursive: bool = False
    limit: int = Field(default=200, ge=1, le=2000)

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


class LsTool(Tool):
    name = "ls"
    description = "List directory contents."
    parameters: dict[str, Any] = LsToolArgs.model_json_schema()

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
            parsed = LsToolArgs.model_validate(args)
        except ValidationError as exc:
            raise _format_validation_error(self.name, exc) from exc

        root = Path(self._fs.workspace_root())
        target = Path(self._fs.resolve_path(parsed.path))
        if target.is_file():
            rel = target.relative_to(root).as_posix()
            return ToolResult(text=rel, details={"path": parsed.path, "count": 1, "truncated": False})

        if not target.exists() or not target.is_dir():
            raise FileNotFoundError(f"ls: directory not found: {parsed.path}")

        iterator = target.rglob("*") if parsed.recursive else target.iterdir()
        rows: list[str] = []
        total = 0
        for entry in iterator:
            total += 1
            if len(rows) >= parsed.limit:
                continue
            rel = entry.relative_to(root).as_posix()
            suffix = "/" if entry.is_dir() else ""
            rows.append(f"{rel}{suffix}")

        if not rows:
            return ToolResult(
                text=f"(ls) empty directory: {parsed.path}",
                details={"path": parsed.path, "count": 0},
            )

        truncated = total > len(rows)
        if truncated:
            rows.append(f"... ({total - len(rows)} more)")

        return ToolResult(
            text="\n".join(rows),
            details={
                "path": parsed.path,
                "count": len(rows),
                "total": total,
                "truncated": truncated,
            },
        )
