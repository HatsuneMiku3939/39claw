from __future__ import annotations

from pypimono.ui.application.presentation.assistant_message_presenter import (
    present_assistant_message,
)
from pypimono.ui.application.presentation.tool_event_formatter import (
    build_tool_end_presentation,
    build_tool_result_presentation,
    build_tool_start_presentation,
)
from pypimono.ui.application.presentation.view_models import (
    BackgroundNoticeViewModel,
    ToolEndViewModel,
    ToolResultViewModel,
    ToolStartViewModel,
    UiDisplayEvent,
)
from pypimono.ui.boundary.contracts.incoming_event import (
    TYPE_BACKGROUND_NOTICE,
    TYPE_MESSAGE_END,
    TYPE_TOOL_EXECUTION_END,
    TYPE_TOOL_EXECUTION_START,
    UiIncomingEvent,
)
from pypimono.ui.boundary.contracts.message import (
    AssistantMessage,
    ToolResultMessage,
)


def present_ui_event(event: UiIncomingEvent) -> list[UiDisplayEvent]:
    if event.type == TYPE_BACKGROUND_NOTICE:
        if not event.notice or not event.notice.strip():
            return []
        return [
            BackgroundNoticeViewModel(
                text=event.notice.strip(),
                source=event.notice_source.strip() if event.notice_source else None,
            )
        ]

    if event.type == TYPE_TOOL_EXECUTION_START:
        tool_name = str(event.tool_name or "unknown")
        presented = build_tool_start_presentation(tool_name, event.args or {})
        return [
            ToolStartViewModel(
                tool_name=tool_name,
                preview_text=presented.preview_text,
                preview_lexer=presented.preview_lexer,
            )
        ]

    if event.type == TYPE_TOOL_EXECUTION_END:
        tool_name = str(event.tool_name or "unknown")
        presented = build_tool_end_presentation(
            tool_name,
            is_error=bool(event.is_error),
            result=event.result,
        )
        return [
            ToolEndViewModel(
                tool_name=tool_name,
                is_error=bool(event.is_error),
                summary_text=presented.summary_text,
            )
        ]

    if event.type != TYPE_MESSAGE_END or event.message is None:
        return []

    if isinstance(event.message, AssistantMessage):
        return present_assistant_message(event.message)

    if isinstance(event.message, ToolResultMessage):
        presented = build_tool_result_presentation(event.message)
        if presented is None:
            return []

        return [
            ToolResultViewModel(
                tool_call_id=event.message.toolCallId,
                tool_name=event.message.toolName,
                body=presented.body,
                is_error=presented.is_error,
            )
        ]

    return []
