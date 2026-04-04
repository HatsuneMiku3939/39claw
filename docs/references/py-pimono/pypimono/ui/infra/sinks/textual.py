from __future__ import annotations

from collections.abc import Callable

from pypimono.ui.application.presentation.view_models import (
    AssistantTextViewModel,
    AssistantThinkingViewModel,
    BackgroundNoticeViewModel,
    ToolEndViewModel,
    ToolStartViewModel,
)
from pypimono.ui.infra.sinks.base import BaseUiDisplaySink


class TextualUiDisplaySink(BaseUiDisplaySink):
    def __init__(
        self,
        *,
        emit_assistant: Callable[[str], None],
        emit_thinking: Callable[[str], None],
        emit_notice: Callable[[str, str | None], None],
        emit_tool_start: Callable[[str, str, str], None],
        emit_tool_end: Callable[[str, bool, str], None],
    ):
        self._emit_assistant = emit_assistant
        self._emit_thinking = emit_thinking
        self._emit_notice = emit_notice
        self._emit_tool_start = emit_tool_start
        self._emit_tool_end = emit_tool_end

    def handle_tool_start(self, presented: ToolStartViewModel) -> None:
        self._emit_tool_start(
            presented.tool_name,
            presented.preview_text,
            presented.preview_lexer,
        )

    def handle_tool_end(self, presented: ToolEndViewModel) -> None:
        self._emit_tool_end(
            presented.tool_name,
            presented.is_error,
            presented.summary_text,
        )

    def handle_assistant_text(self, presented: AssistantTextViewModel) -> None:
        self._emit_assistant(presented.text)

    def handle_assistant_thinking(self, presented: AssistantThinkingViewModel) -> None:
        self._emit_thinking(presented.text)

    def handle_background_notice(self, presented: BackgroundNoticeViewModel) -> None:
        self._emit_notice(presented.text, presented.source)
