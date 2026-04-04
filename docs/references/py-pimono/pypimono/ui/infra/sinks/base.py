from __future__ import annotations

from abc import ABC

from typing_extensions import override

from pypimono.ui.application.ports.event_sinks import UiDisplaySink
from pypimono.ui.application.presentation.view_models import (
    AssistantTextViewModel,
    AssistantThinkingViewModel,
    AssistantToolCallViewModel,
    BackgroundNoticeViewModel,
    ToolEndViewModel,
    ToolResultViewModel,
    ToolStartViewModel,
    UiDisplayEvent,
)


class BaseUiDisplaySink(UiDisplaySink, ABC):
    @override
    def on_event(self, event: UiDisplayEvent) -> None:
        self._dispatch(event)

    def _dispatch(self, presented: UiDisplayEvent) -> None:
        if isinstance(presented, ToolStartViewModel):
            self.handle_tool_start(presented)
            return

        if isinstance(presented, ToolEndViewModel):
            self.handle_tool_end(presented)
            return

        if isinstance(presented, AssistantThinkingViewModel):
            self.handle_assistant_thinking(presented)
            return

        if isinstance(presented, AssistantTextViewModel):
            self.handle_assistant_text(presented)
            return

        if isinstance(presented, AssistantToolCallViewModel):
            self.handle_assistant_tool_call(presented)
            return

        if isinstance(presented, ToolResultViewModel):
            self.handle_tool_result(presented)
            return

        if isinstance(presented, BackgroundNoticeViewModel):
            self.handle_background_notice(presented)

    def handle_tool_start(self, presented: ToolStartViewModel) -> None:
        return

    def handle_tool_end(self, presented: ToolEndViewModel) -> None:
        return

    def handle_assistant_thinking(self, presented: AssistantThinkingViewModel) -> None:
        return

    def handle_assistant_text(self, presented: AssistantTextViewModel) -> None:
        return

    def handle_assistant_tool_call(self, presented: AssistantToolCallViewModel) -> None:
        return

    def handle_tool_result(self, presented: ToolResultViewModel) -> None:
        return

    def handle_background_notice(self, presented: BackgroundNoticeViewModel) -> None:
        return
