from __future__ import annotations

from pypimono.ui.application.presentation.view_models import (
    AssistantTextViewModel,
    AssistantThinkingViewModel,
    AssistantToolCallViewModel,
    BackgroundNoticeViewModel,
    ToolEndViewModel,
    ToolResultViewModel,
    ToolStartViewModel,
)
from pypimono.ui.infra.sinks.base import BaseUiDisplaySink


class ConsoleUiDisplaySink(BaseUiDisplaySink):
    TOOL_START_PREFIX = "[tool_start]"
    TOOL_END_PREFIX = "[tool_end]"
    ASSISTANT_PREFIX = "ASSISTANT:"
    ASSISTANT_THINKING_PREFIX = "ASSISTANT(thinking):"
    ASSISTANT_TOOL_CALL_PREFIX = "ASSISTANT(toolCall):"

    def handle_tool_start(self, presented: ToolStartViewModel) -> None:
        print(f"{self.TOOL_START_PREFIX} {presented.tool_name}")
        if presented.preview_text:
            print(presented.preview_text)

    def handle_tool_end(self, presented: ToolEndViewModel) -> None:
        print(f"{self.TOOL_END_PREFIX} {presented.summary_text}")

    def handle_assistant_thinking(self, presented: AssistantThinkingViewModel) -> None:
        print(f"{self.ASSISTANT_THINKING_PREFIX} {presented.text}")

    def handle_assistant_text(self, presented: AssistantTextViewModel) -> None:
        print(f"{self.ASSISTANT_PREFIX} {presented.text}")

    def handle_assistant_tool_call(self, presented: AssistantToolCallViewModel) -> None:
        print(f"{self.ASSISTANT_TOOL_CALL_PREFIX} {presented.tool_name} {presented.arguments}")

    def handle_tool_result(self, presented: ToolResultViewModel) -> None:
        print(f"TOOLRESULT({presented.tool_name}): {presented.body}")

    def handle_background_notice(self, presented: BackgroundNoticeViewModel) -> None:
        prefix = f"[{presented.source}]" if presented.source else "[notice]"
        print(f"{prefix} {presented.text}")


ConsoleEventPrinter = ConsoleUiDisplaySink
