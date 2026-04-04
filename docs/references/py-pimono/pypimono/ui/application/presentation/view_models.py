from __future__ import annotations

from dataclasses import dataclass
from typing import Any, TypeAlias


@dataclass(frozen=True)
class ToolStartViewModel:
    tool_name: str
    preview_text: str
    preview_lexer: str


@dataclass(frozen=True)
class ToolEndViewModel:
    tool_name: str
    is_error: bool
    summary_text: str


@dataclass(frozen=True)
class AssistantTextViewModel:
    text: str


@dataclass(frozen=True)
class AssistantThinkingViewModel:
    text: str


@dataclass(frozen=True)
class AssistantToolCallViewModel:
    tool_call_id: str
    tool_name: str
    arguments: dict[str, Any]


@dataclass(frozen=True)
class ToolResultViewModel:
    tool_call_id: str
    tool_name: str
    body: str
    is_error: bool


@dataclass(frozen=True)
class BackgroundNoticeViewModel:
    text: str
    source: str | None = None


UiDisplayEvent: TypeAlias = (
    ToolStartViewModel
    | ToolEndViewModel
    | AssistantTextViewModel
    | AssistantThinkingViewModel
    | AssistantToolCallViewModel
    | ToolResultViewModel
    | BackgroundNoticeViewModel
)
