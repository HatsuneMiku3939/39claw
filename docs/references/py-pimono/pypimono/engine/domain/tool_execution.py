from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Sequence

from pypimono.engine.domain.ports.tool import Tool, ToolResult


@dataclass(frozen=True)
class ToolExecution:
    result: ToolResult
    is_error: bool


async def execute_tool_call(
    *,
    tools: Sequence[Tool],
    tool_call_id: str,
    tool_name: str,
    args: dict[str, Any],
) -> ToolExecution:
    tool = next((candidate for candidate in tools if candidate.name == tool_name), None)
    if tool is None:
        return ToolExecution(
            result=ToolResult(
                text=f"tool not found: {tool_name}",
                details={"exception": "RuntimeError"},
            ),
            is_error=True,
        )

    try:
        result = await tool.execute(tool_call_id, args)
        return ToolExecution(result=result, is_error=False)
    except Exception as exc:
        return ToolExecution(
            result=ToolResult(text=str(exc), details={"exception": type(exc).__name__}),
            is_error=True,
        )
