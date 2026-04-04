from __future__ import annotations

from pypimono.session.boundary.contracts import message as contract_messages
from pypimono.session.domain import message as domain_messages


def to_contract_messages(
    messages: list[domain_messages.SessionMessage],
) -> list[contract_messages.SessionMessage]:
    return [to_contract_message(message) for message in messages]


def to_contract_message(
    message: domain_messages.SessionMessage,
) -> contract_messages.SessionMessage:
    if isinstance(message, domain_messages.UserMessage):
        return contract_messages.UserMessage(content=message.content, timestamp=message.timestamp)

    if isinstance(message, domain_messages.AssistantMessage):
        content: list[contract_messages.SessionContentBlock] = [
            _to_contract_content_block(block) for block in message.content
        ]
        return contract_messages.AssistantMessage(
            content=content,
            stop_reason=message.stop_reason,
            error_message=message.error_message,
            timestamp=message.timestamp,
        )

    if isinstance(message, domain_messages.ToolResultMessage):
        return contract_messages.ToolResultMessage(
            toolCallId=message.toolCallId,
            toolName=message.toolName,
            content=[contract_messages.TextBlock(text=block.text) for block in message.content],
            isError=message.isError,
            timestamp=message.timestamp,
        )

    raise TypeError(f"unsupported domain message type: {type(message)!r}")


def _to_contract_content_block(
    block: domain_messages.SessionContentBlock,
) -> contract_messages.SessionContentBlock:
    if isinstance(block, domain_messages.TextBlock):
        return contract_messages.TextBlock(text=block.text)
    if isinstance(block, domain_messages.ThinkingBlock):
        return contract_messages.ThinkingBlock(text=block.text)
    if isinstance(block, domain_messages.ToolCallBlock):
        return contract_messages.ToolCallBlock(id=block.id, name=block.name, arguments=dict(block.arguments))
    raise TypeError(f"unsupported domain content block type: {type(block)!r}")


def to_domain_message(
    message: contract_messages.SessionMessage,
) -> domain_messages.SessionMessage:
    if isinstance(message, contract_messages.UserMessage):
        return domain_messages.UserMessage(content=message.content, timestamp=message.timestamp)

    if isinstance(message, contract_messages.AssistantMessage):
        content: list[domain_messages.SessionContentBlock] = [
            _to_domain_content_block(block) for block in message.content
        ]
        return domain_messages.AssistantMessage(
            content=content,
            stop_reason=message.stop_reason,
            error_message=message.error_message,
            timestamp=message.timestamp,
        )

    if isinstance(message, contract_messages.ToolResultMessage):
        return domain_messages.ToolResultMessage(
            toolCallId=message.toolCallId,
            toolName=message.toolName,
            content=[domain_messages.TextBlock(text=block.text) for block in message.content],
            isError=message.isError,
            timestamp=message.timestamp,
        )

    raise TypeError(f"unsupported contract message type: {type(message)!r}")


def _to_domain_content_block(
    block: contract_messages.SessionContentBlock,
) -> domain_messages.SessionContentBlock:
    if isinstance(block, contract_messages.TextBlock):
        return domain_messages.TextBlock(text=block.text)
    if isinstance(block, contract_messages.ThinkingBlock):
        return domain_messages.ThinkingBlock(text=block.text)
    if isinstance(block, contract_messages.ToolCallBlock):
        return domain_messages.ToolCallBlock(id=block.id, name=block.name, arguments=dict(block.arguments))
    raise TypeError(f"unsupported contract content block type: {type(block)!r}")
