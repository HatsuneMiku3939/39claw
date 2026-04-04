from __future__ import annotations

from dataclasses import dataclass
from typing import Any, TypeAlias

from pypimono.ui.application.presentation.view_models import (
    AssistantTextViewModel,
    AssistantThinkingViewModel,
    AssistantToolCallViewModel,
    UiDisplayEvent,
)
from pypimono.ui.boundary.contracts.message import (
    AssistantMessage,
    TextBlock,
    ThinkingBlock,
    ToolCallBlock,
)


@dataclass(frozen=True)
class AssistantTextSegment:
    text: str


@dataclass(frozen=True)
class AssistantThinkingSegment:
    text: str


@dataclass(frozen=True)
class AssistantToolCallSegment:
    tool_call_id: str
    tool_name: str
    arguments: dict[str, Any]


AssistantMessageSegment: TypeAlias = (
    AssistantTextSegment | AssistantThinkingSegment | AssistantToolCallSegment
)


def present_assistant_message(message: AssistantMessage) -> list[UiDisplayEvent]:
    return [_present_segment(segment) for segment in split_assistant_message(message)]


def split_assistant_message(message: AssistantMessage) -> list[AssistantMessageSegment]:
    segments: list[AssistantMessageSegment] = []
    text_chunks: list[str] = []

    def flush_text() -> None:
        if not text_chunks:
            return
        segments.append(AssistantTextSegment(text="\n\n".join(text_chunks)))
        text_chunks.clear()

    for block in message.content:
        if isinstance(block, TextBlock):
            if block.text.strip():
                text_chunks.append(block.text)
            continue

        flush_text()

        if isinstance(block, ThinkingBlock):
            if block.text.strip():
                segments.append(AssistantThinkingSegment(text=block.text))
            continue

        if isinstance(block, ToolCallBlock):
            segments.append(
                AssistantToolCallSegment(
                    tool_call_id=block.id,
                    tool_name=block.name,
                    arguments=dict(block.arguments),
                )
            )

    flush_text()
    return segments


def _present_segment(segment: AssistantMessageSegment) -> UiDisplayEvent:
    if isinstance(segment, AssistantTextSegment):
        return AssistantTextViewModel(text=segment.text)

    if isinstance(segment, AssistantThinkingSegment):
        return AssistantThinkingViewModel(text=segment.text)

    return AssistantToolCallViewModel(
        tool_call_id=segment.tool_call_id,
        tool_name=segment.tool_name,
        arguments=dict(segment.arguments),
    )
