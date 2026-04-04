from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Literal, TypeAlias

from pypimono.engine.domain.messages import AgentMessage, AssistantMessage, ToolResultMessage
from pypimono.engine.domain.ports.tool import ToolResult

# event type values
TYPE_AGENT_START = "agent_start"
TYPE_TURN_START = "turn_start"
TYPE_MESSAGE_START = "message_start"
TYPE_MESSAGE_END = "message_end"
TYPE_TOOL_EXECUTION_START = "tool_execution_start"
TYPE_TOOL_EXECUTION_END = "tool_execution_end"
TYPE_TURN_END = "turn_end"
TYPE_AGENT_END = "agent_end"


@dataclass(frozen=True)
class AgentStartEvent:
    type: Literal["agent_start"] = TYPE_AGENT_START


@dataclass(frozen=True)
class TurnStartEvent:
    type: Literal["turn_start"] = TYPE_TURN_START


@dataclass(frozen=True)
class MessageStartEvent:
    message: AgentMessage
    type: Literal["message_start"] = TYPE_MESSAGE_START


@dataclass(frozen=True)
class MessageEndEvent:
    message: AgentMessage
    type: Literal["message_end"] = TYPE_MESSAGE_END


@dataclass(frozen=True)
class ToolExecutionStartEvent:
    tool_call_id: str
    tool_name: str
    args: dict[str, Any] = field(default_factory=dict)
    type: Literal["tool_execution_start"] = TYPE_TOOL_EXECUTION_START


@dataclass(frozen=True)
class ToolExecutionEndEvent:
    tool_call_id: str
    tool_name: str
    result: ToolResult
    is_error: bool
    type: Literal["tool_execution_end"] = TYPE_TOOL_EXECUTION_END


@dataclass(frozen=True)
class TurnEndEvent:
    message: AssistantMessage
    tool_results: list[ToolResultMessage] = field(default_factory=list)
    type: Literal["turn_end"] = TYPE_TURN_END


@dataclass(frozen=True)
class AgentEndEvent:
    messages: list[AgentMessage] = field(default_factory=list)
    type: Literal["agent_end"] = TYPE_AGENT_END


AgentEvent: TypeAlias = (
    AgentStartEvent
    | TurnStartEvent
    | MessageStartEvent
    | MessageEndEvent
    | ToolExecutionStartEvent
    | ToolExecutionEndEvent
    | TurnEndEvent
    | AgentEndEvent
)
