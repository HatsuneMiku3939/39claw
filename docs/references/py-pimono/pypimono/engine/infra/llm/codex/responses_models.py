from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any


@dataclass(frozen=True)
class CodexUsage:
    input_tokens: int | None = None
    output_tokens: int | None = None
    total_tokens: int | None = None
    cached_input_tokens: int | None = None

    @property
    def uncached_input_tokens(self) -> int | None:
        if self.input_tokens is None:
            return None
        cached = max(self.cached_input_tokens or 0, 0)
        return max(self.input_tokens - cached, 0)


@dataclass(frozen=True)
class CodexTextOutputItem:
    type: str = "text"
    text: str = ""


@dataclass(frozen=True)
class CodexToolCallOutputItem:
    type: str = "tool_call"
    name: str = ""
    call_id: str = ""
    item_id: str | None = None
    arguments: dict[str, Any] = field(default_factory=dict)


@dataclass(frozen=True)
class CodexReasoningOutputItem:
    type: str = "reasoning"
    summary: str = ""


CodexOutputItem = CodexTextOutputItem | CodexReasoningOutputItem | CodexToolCallOutputItem


@dataclass(frozen=True)
class CodexResponse:
    output_items: list[CodexOutputItem] = field(default_factory=list)
    status: str = "completed"
    error_message: str | None = None
    usage: CodexUsage | None = None
