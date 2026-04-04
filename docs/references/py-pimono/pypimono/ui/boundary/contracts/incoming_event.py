from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any

from pypimono.ui.boundary.contracts.message import ToolResultMessage, UiMessage

TYPE_AGENT_START = "agent_start"
TYPE_TURN_START = "turn_start"
TYPE_MESSAGE_START = "message_start"
TYPE_MESSAGE_END = "message_end"
TYPE_BACKGROUND_NOTICE = "background_notice"
TYPE_TOOL_EXECUTION_START = "tool_execution_start"
TYPE_TOOL_EXECUTION_END = "tool_execution_end"
TYPE_TURN_END = "turn_end"
TYPE_AGENT_END = "agent_end"


@dataclass(frozen=True)
class ToolExecutionResult:
    text: str
    details: dict[str, Any] | None = None


@dataclass(frozen=True)
class UiIncomingEvent:
    type: str
    message: UiMessage | None = None
    notice: str | None = None
    notice_source: str | None = None
    tool_call_id: str | None = None
    tool_name: str | None = None
    args: dict[str, Any] | None = None
    result: ToolExecutionResult | None = None
    is_error: bool | None = None
    tool_results: list[ToolResultMessage] = field(default_factory=list)
    messages: list[UiMessage] = field(default_factory=list)
