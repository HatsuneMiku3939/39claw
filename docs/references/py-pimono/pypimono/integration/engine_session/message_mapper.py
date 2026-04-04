from __future__ import annotations

from pypimono.engine.domain import messages as engine_messages
from pypimono.session.boundary.contracts import message as session_messages

EngineContentBlock = (
    engine_messages.TextBlock
    | engine_messages.ThinkingBlock
    | engine_messages.ToolCallBlock
)


def to_engine_message(message: session_messages.SessionMessage) -> engine_messages.AgentMessage:
    if isinstance(message, session_messages.UserMessage):
        return engine_messages.UserMessage(content=message.content, timestamp=message.timestamp)

    if isinstance(message, session_messages.AssistantMessage):
        content: list[EngineContentBlock] = [
            _to_engine_content_block(block) for block in message.content
        ]
        return engine_messages.AssistantMessage(
            content=content,
            stop_reason=message.stop_reason,
            error_message=message.error_message,
            timestamp=message.timestamp,
        )

    if isinstance(message, session_messages.ToolResultMessage):
        return engine_messages.ToolResultMessage(
            toolCallId=message.toolCallId,
            toolName=message.toolName,
            content=[engine_messages.TextBlock(text=block.text) for block in message.content],
            isError=message.isError,
            timestamp=message.timestamp,
        )

    raise TypeError(f"unsupported session message type: {type(message)!r}")


def to_session_message(message: engine_messages.AgentMessage) -> session_messages.SessionMessage:
    if isinstance(message, engine_messages.UserMessage):
        return session_messages.UserMessage(content=message.content, timestamp=message.timestamp)

    if isinstance(message, engine_messages.AssistantMessage):
        content: list[session_messages.SessionContentBlock] = [
            _to_session_content_block(block) for block in message.content
        ]
        return session_messages.AssistantMessage(
            content=content,
            stop_reason=message.stop_reason,
            error_message=message.error_message,
            timestamp=message.timestamp,
        )

    if isinstance(message, engine_messages.ToolResultMessage):
        return to_session_tool_result_message(message)

    raise TypeError(f"unsupported engine message type: {type(message)!r}")


def to_session_tool_result_message(
    message: engine_messages.ToolResultMessage,
) -> session_messages.ToolResultMessage:
    return session_messages.ToolResultMessage(
        toolCallId=message.toolCallId,
        toolName=message.toolName,
        content=[session_messages.TextBlock(text=block.text) for block in message.content],
        isError=message.isError,
        timestamp=message.timestamp,
    )


def _to_engine_content_block(
    block: session_messages.SessionContentBlock,
) -> EngineContentBlock:
    if isinstance(block, session_messages.TextBlock):
        return engine_messages.TextBlock(text=block.text)
    if isinstance(block, session_messages.ThinkingBlock):
        return engine_messages.ThinkingBlock(text=block.text)
    if isinstance(block, session_messages.ToolCallBlock):
        return engine_messages.ToolCallBlock(id=block.id, name=block.name, arguments=dict(block.arguments))
    raise TypeError(f"unsupported session content block type: {type(block)!r}")


def _to_session_content_block(
    block: EngineContentBlock,
) -> session_messages.SessionContentBlock:
    if isinstance(block, engine_messages.TextBlock):
        return session_messages.TextBlock(text=block.text)
    if isinstance(block, engine_messages.ThinkingBlock):
        return session_messages.ThinkingBlock(text=block.text)
    if isinstance(block, engine_messages.ToolCallBlock):
        return session_messages.ToolCallBlock(id=block.id, name=block.name, arguments=dict(block.arguments))
    raise TypeError(f"unsupported engine content block type: {type(block)!r}")
