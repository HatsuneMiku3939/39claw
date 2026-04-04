from __future__ import annotations

from pypimono.session.boundary.contracts import message as session_messages
from pypimono.ui.boundary.contracts import message as ui_messages


def to_ui_message(message: session_messages.SessionMessage) -> ui_messages.UiMessage:
    if isinstance(message, session_messages.UserMessage):
        return ui_messages.UserMessage(
            content=message.content,
            timestamp=message.timestamp,
        )

    if isinstance(message, session_messages.AssistantMessage):
        return ui_messages.AssistantMessage(
            content=[to_ui_content_block(block) for block in message.content],
            stop_reason=message.stop_reason,
            error_message=message.error_message,
            timestamp=message.timestamp,
        )

    if isinstance(message, session_messages.ToolResultMessage):
        return to_ui_tool_result_message(message)

    raise TypeError(f"Unsupported session message: {type(message)!r}")


def to_ui_tool_result_message(
    message: session_messages.ToolResultMessage,
) -> ui_messages.ToolResultMessage:
    return ui_messages.ToolResultMessage(
        toolCallId=message.toolCallId,
        toolName=message.toolName,
        content=[ui_messages.TextBlock(text=block.text) for block in message.content],
        isError=message.isError,
        timestamp=message.timestamp,
    )


def to_ui_content_block(
    block: session_messages.SessionContentBlock,
) -> ui_messages.UiContentBlock:
    if isinstance(block, session_messages.TextBlock):
        return ui_messages.TextBlock(text=block.text)

    if isinstance(block, session_messages.ThinkingBlock):
        return ui_messages.ThinkingBlock(text=block.text)

    if isinstance(block, session_messages.ToolCallBlock):
        return ui_messages.ToolCallBlock(
            id=block.id,
            name=block.name,
            arguments=dict(block.arguments),
        )

    raise TypeError(f"Unsupported session content block: {type(block)!r}")
