from __future__ import annotations

from typing import Any

from pydantic import BaseModel, ConfigDict, Field, ValidationError, field_validator
from typing_extensions import override

from pypimono.engine.domain.ports.tool import OnToolUpdate, Tool, ToolResult
from pypimono.engine.infra.workspace_fs.port import WorkspaceFsGateway


class EditToolArgs(BaseModel):
    model_config = ConfigDict(extra="ignore")

    path: str = Field(min_length=1)
    old_text: str = Field(min_length=1)
    new_text: str
    replace_all: bool = False

    @field_validator("path", mode="before")
    @classmethod
    def _normalize_path(cls, value: Any) -> str:
        return str(value).strip() if value is not None else ""

    @field_validator("old_text", "new_text", mode="before")
    @classmethod
    def _normalize_text(cls, value: Any) -> str:
        if value is None:
            return ""
        return str(value)


def _format_validation_error(tool_name: str, exc: ValidationError) -> ValueError:
    details = []
    for error in exc.errors(include_url=False):
        loc = ".".join(str(part) for part in error.get("loc", ()))
        msg = str(error.get("msg", "invalid value"))
        details.append(f"{loc}: {msg}" if loc else msg)
    message = "; ".join(details) if details else "invalid args"
    return ValueError(f"{tool_name}: {message}")


class EditTool(Tool):
    name = "edit"
    description = "Apply targeted text replacement in a file."
    parameters: dict[str, Any] = EditToolArgs.model_json_schema()

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
            parsed = EditToolArgs.model_validate(args)
        except ValidationError as exc:
            raise _format_validation_error(self.name, exc) from exc

        original = self._fs.read_text(parsed.path, encoding="utf-8")
        occurrences = original.count(parsed.old_text)
        if occurrences == 0:
            raise ValueError(f"edit: old_text not found in {parsed.path}")

        if parsed.replace_all:
            updated = original.replace(parsed.old_text, parsed.new_text)
            replacements = occurrences
        else:
            updated = original.replace(parsed.old_text, parsed.new_text, 1)
            replacements = 1

        self._fs.write_text(parsed.path, updated, encoding="utf-8")
        return ToolResult(
            text=f"(edit) replaced {replacements} occurrence(s) in {parsed.path}",
            details={"path": parsed.path, "replacements": replacements},
        )
