from __future__ import annotations

from pypimono.integration.session_ui.message_mapper import (
    to_ui_message,
    to_ui_tool_result_message,
)
from pypimono.session.boundary.contracts import ui_event as session_events
from pypimono.ui.boundary.contracts import incoming_event as ui_events


def to_ui_incoming_event(event: session_events.SessionUiEvent) -> ui_events.UiIncomingEvent:
    return ui_events.UiIncomingEvent(
        type=event.type,
        message=to_ui_message(event.message) if event.message is not None else None,
        notice=event.notice,
        notice_source=event.notice_source,
        tool_call_id=event.tool_call_id,
        tool_name=event.tool_name,
        args=dict(event.args) if event.args is not None else None,
        result=_to_tool_execution_result(event.result),
        is_error=event.is_error,
        tool_results=[to_ui_tool_result_message(message) for message in event.tool_results],
        messages=[to_ui_message(message) for message in event.messages],
    )


def _to_tool_execution_result(
    result: session_events.ToolExecutionResult | None,
) -> ui_events.ToolExecutionResult | None:
    if result is None:
        return None

    details = None if result.details is None else dict(result.details)
    return ui_events.ToolExecutionResult(text=result.text, details=details)
