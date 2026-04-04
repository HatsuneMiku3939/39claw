from __future__ import annotations

from typing import Any

from pypimono.engine.domain import agent_event as engine_events
from pypimono.integration.engine_session.message_mapper import (
    to_session_message,
    to_session_tool_result_message,
)
from pypimono.session.boundary.contracts import ui_event as session_ui_events


def to_session_runtime_event(event: engine_events.AgentEvent) -> session_ui_events.SessionUiEvent:
    if isinstance(event, engine_events.AgentStartEvent):
        return session_ui_events.SessionUiEvent(type=event.type)

    if isinstance(event, engine_events.TurnStartEvent):
        return session_ui_events.SessionUiEvent(type=event.type)

    if isinstance(event, engine_events.MessageStartEvent):
        return session_ui_events.SessionUiEvent(type=event.type, message=to_session_message(event.message))

    if isinstance(event, engine_events.MessageEndEvent):
        return session_ui_events.SessionUiEvent(type=event.type, message=to_session_message(event.message))

    if isinstance(event, engine_events.ToolExecutionStartEvent):
        return session_ui_events.SessionUiEvent(
            type=event.type,
            tool_call_id=event.tool_call_id,
            tool_name=event.tool_name,
            args=dict(event.args),
        )

    if isinstance(event, engine_events.ToolExecutionEndEvent):
        return session_ui_events.SessionUiEvent(
            type=event.type,
            tool_call_id=event.tool_call_id,
            tool_name=event.tool_name,
            result=_to_tool_execution_result(event),
            is_error=event.is_error,
        )

    if isinstance(event, engine_events.TurnEndEvent):
        return session_ui_events.SessionUiEvent(
            type=event.type,
            message=to_session_message(event.message),
            tool_results=[
                to_session_tool_result_message(message)
                for message in event.tool_results
            ],
        )

    if isinstance(event, engine_events.AgentEndEvent):
        return session_ui_events.SessionUiEvent(
            type=event.type,
            messages=[to_session_message(message) for message in event.messages],
        )

    raise TypeError(f"unsupported engine event type: {type(event)!r}")


def _to_tool_execution_result(
    event: engine_events.ToolExecutionEndEvent,
) -> session_ui_events.ToolExecutionResult:
    return session_ui_events.ToolExecutionResult(
        text=event.result.text,
        details=_to_dict_or_none(event.result.details),
    )


def _to_dict_or_none(value: object) -> dict[str, Any] | None:
    if value is None:
        return None
    if not isinstance(value, dict):
        raise TypeError(f"expected dict details but got {type(value)!r}")
    return {str(key): item for key, item in value.items()}
