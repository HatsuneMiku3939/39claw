from __future__ import annotations

from typing import Any

from pydantic import BaseModel, ConfigDict, Field, ValidationError, field_validator
from typing_extensions import override

from pypimono.engine.domain.ports.tool import OnToolUpdate, Tool, ToolResult
from pypimono.engine.infra.workspace_fs.port import WorkspaceFsGateway


class ReadToolArgs(BaseModel):
    model_config = ConfigDict(extra="ignore")

    path: str = Field(min_length=1)
    offset: int | None = Field(default=None, ge=1)
    limit: int | None = Field(default=None, ge=1)

    @field_validator("path", mode="before")
    @classmethod
    def _normalize_path(cls, value: Any) -> str:
        return str(value).strip() if value is not None else ""


def _format_validation_error(tool_name: str, exc: ValidationError) -> ValueError:
    details = []
    for error in exc.errors(include_url=False):
        loc = ".".join(str(part) for part in error.get("loc", ()))
        msg = str(error.get("msg", "invalid value"))
        details.append(f"{loc}: {msg}" if loc else msg)
    message = "; ".join(details) if details else "invalid args"
    return ValueError(f"{tool_name}: {message}")


class ReadTool(Tool):
    name = "read"
    description = "Read file contents from the workspace."
    parameters: dict[str, Any] = ReadToolArgs.model_json_schema()

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
            parsed = ReadToolArgs.model_validate(args)
        except ValidationError as exc:
            raise _format_validation_error(self.name, exc) from exc

        text = self._fs.read_text(parsed.path, encoding="utf-8")
        lines = text.splitlines()

        start = parsed.offset - 1 if parsed.offset is not None else 0
        if start >= len(lines):
            return ToolResult(text=f"(read) offset {start + 1} beyond EOF ({len(lines)} lines).")

        end = start + parsed.limit if parsed.limit is not None else len(lines)
        chunk = "\n".join(lines[start:end])

        return ToolResult(
            text=chunk,
            details={"path": parsed.path, "offset": start + 1, "lines": len(lines)},
        )
