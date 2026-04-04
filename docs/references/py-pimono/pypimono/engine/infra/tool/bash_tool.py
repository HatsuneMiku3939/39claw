from __future__ import annotations

import subprocess
from pathlib import Path
from typing import Any

from pydantic import BaseModel, ConfigDict, Field, ValidationError, field_validator
from typing_extensions import override

from pypimono.engine.domain.ports.tool import OnToolUpdate, Tool, ToolResult
from pypimono.engine.infra.workspace_fs.port import WorkspaceFsGateway

MAX_OUTPUT_CHARS = 12000


class BashToolArgs(BaseModel):
    model_config = ConfigDict(extra="ignore")

    command: str = Field(min_length=1)
    cwd: str = "."
    timeout_sec: int = Field(default=30, ge=1, le=300)

    @field_validator("command", mode="before")
    @classmethod
    def _normalize_command(cls, value: Any) -> str:
        return str(value).strip() if value is not None else ""

    @field_validator("cwd", mode="before")
    @classmethod
    def _normalize_cwd(cls, value: Any) -> str:
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


def _truncate_output(text: str) -> tuple[str, bool]:
    if len(text) <= MAX_OUTPUT_CHARS:
        return text, False
    return text[:MAX_OUTPUT_CHARS], True


class BashTool(Tool):
    name = "bash"
    description = "Execute shell commands in the workspace."
    parameters: dict[str, Any] = BashToolArgs.model_json_schema()

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
            parsed = BashToolArgs.model_validate(args)
        except ValidationError as exc:
            raise _format_validation_error(self.name, exc) from exc

        cwd = Path(self._fs.resolve_path(parsed.cwd))
        try:
            completed = subprocess.run(
                parsed.command,
                cwd=str(cwd),
                shell=True,
                capture_output=True,
                text=True,
                timeout=parsed.timeout_sec,
                check=False,
            )
        except subprocess.TimeoutExpired as exc:
            raise RuntimeError(f"bash: timed out after {parsed.timeout_sec}s") from exc

        stdout = (completed.stdout or "").rstrip()
        stderr = (completed.stderr or "").rstrip()
        merged = stdout if not stderr else f"{stdout}\n{stderr}".strip()
        merged = merged or "(no output)"
        body, truncated = _truncate_output(merged)
        output = f"(bash) exit={completed.returncode}\n{body}"
        if truncated:
            output += "\n... (output truncated)"

        if completed.returncode != 0:
            raise RuntimeError(output)

        return ToolResult(
            text=output,
            details={
                "cwd": str(cwd),
                "exit_code": completed.returncode,
                "truncated": truncated,
            },
        )
