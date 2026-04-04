from __future__ import annotations

import time
import uuid
from collections.abc import Sequence
from dataclasses import dataclass, field
from typing import Any, Literal, Union


def now_ms() -> int:
    return int(time.time() * 1000)


@dataclass(frozen=True)
class TextBlock:
    type: Literal["text"] = "text"
    text: str = ""


@dataclass(frozen=True)
class ThinkingBlock:
    type: Literal["thinking"] = "thinking"
    text: str = ""


@dataclass(frozen=True)
class ToolCallBlock:
    type: Literal["toolCall"] = "toolCall"
    id: str = field(default_factory=lambda: f"call_{uuid.uuid4().hex[:12]}")
    name: str = ""
    arguments: dict[str, Any] = field(default_factory=dict)


ContentBlock = Union[TextBlock, ThinkingBlock, ToolCallBlock]


@dataclass
class UserMessage:
    role: Literal["user"] = "user"
    content: str = ""
    timestamp: int = field(default_factory=now_ms)


@dataclass
class AssistantMessage:
    role: Literal["assistant"] = "assistant"
    content: list[ContentBlock] = field(default_factory=list)
    stop_reason: Literal["stop", "toolUse", "error", "aborted"] = "stop"
    error_message: str | None = None
    timestamp: int = field(default_factory=now_ms)


@dataclass
class ToolResultMessage:
    role: Literal["toolResult"] = "toolResult"
    toolCallId: str = ""
    toolName: str = ""
    content: list[TextBlock] = field(default_factory=list)
    isError: bool = False
    timestamp: int = field(default_factory=now_ms)


AgentMessage = Union[UserMessage, AssistantMessage, ToolResultMessage]


def strip_thinking_from_messages(messages: Sequence[AgentMessage]) -> list[AgentMessage]:
    out: list[AgentMessage] = []

    for message in messages:
        if not isinstance(message, AssistantMessage):
            out.append(message)
            continue

        content = [block for block in message.content if not isinstance(block, ThinkingBlock)]
        if len(content) == len(message.content):
            out.append(message)
            continue

        out.append(
            AssistantMessage(
                content=content,
                stop_reason=message.stop_reason,
                error_message=message.error_message,
                timestamp=message.timestamp,
            )
        )

    return out
