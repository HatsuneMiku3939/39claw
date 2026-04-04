from __future__ import annotations

import time
from typing import Any, Literal, cast

from pypimono.session.domain.entry import MessageEntry, SessionEntryBase
from pypimono.session.domain.message import (
    AssistantMessage,
    SessionMessage,
    TextBlock,
    ThinkingBlock,
    ToolCallBlock,
    ToolResultMessage,
    UserMessage,
)


def now_iso() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%S", time.gmtime())


def serialize_entry(entry: SessionEntryBase) -> dict[str, Any]:
    if isinstance(entry, MessageEntry):
        return {
            "type": entry.type,
            "id": entry.id,
            "parentId": entry.parentId,
            "timestamp": entry.timestamp,
            "message": serialize_message(entry.message) if entry.message is not None else None,
        }
    raise TypeError(f"unsupported session entry type: {type(entry)!r}")


def deserialize_entry(payload: dict[str, Any]) -> SessionEntryBase:
    entry_type = payload.get("type")
    if entry_type != "message":
        raise ValueError(f"unsupported session entry type: {entry_type!r}")

    entry_id = str(payload.get("id", "")).strip()
    if not entry_id:
        raise ValueError("missing entry id")

    parent_id = payload.get("parentId")
    if parent_id is not None:
        parent_id = str(parent_id)

    timestamp = str(payload.get("timestamp", "")).strip() or now_iso()

    message_payload = payload.get("message")
    message = None
    if message_payload is not None:
        if not isinstance(message_payload, dict):
            raise ValueError("message must be an object")
        message = deserialize_message(message_payload)

    return MessageEntry(
        id=entry_id,
        parentId=parent_id,
        timestamp=timestamp,
        message=message,
    )


def serialize_message(message: SessionMessage) -> dict[str, Any]:
    if isinstance(message, UserMessage):
        return {
            "role": "user",
            "content": message.content,
            "timestamp": message.timestamp,
        }

    if isinstance(message, AssistantMessage):
        return {
            "role": "assistant",
            "content": [serialize_content_block(block) for block in message.content],
            "stop_reason": message.stop_reason,
            "error_message": message.error_message,
            "timestamp": message.timestamp,
        }

    if isinstance(message, ToolResultMessage):
        return {
            "role": "toolResult",
            "toolCallId": message.toolCallId,
            "toolName": message.toolName,
            "content": [{"type": "text", "text": block.text} for block in message.content],
            "isError": message.isError,
            "timestamp": message.timestamp,
        }

    raise TypeError(f"unsupported message type: {type(message)!r}")


def deserialize_message(payload: dict[str, Any]) -> SessionMessage:
    role = payload.get("role")
    timestamp = int(payload.get("timestamp", int(time.time() * 1000)))

    if role == "user":
        return UserMessage(
            content=str(payload.get("content", "")),
            timestamp=timestamp,
        )

    if role == "assistant":
        raw_blocks = payload.get("content", [])
        blocks: list[ThinkingBlock | TextBlock | ToolCallBlock] = []
        if isinstance(raw_blocks, list):
            for raw in raw_blocks:
                if not isinstance(raw, dict):
                    continue
                block = deserialize_content_block(raw)
                if block is not None:
                    blocks.append(block)

        raw_stop_reason = str(payload.get("stop_reason", "stop"))
        stop_reason: Literal["stop", "toolUse", "error", "aborted"]
        if raw_stop_reason in {"stop", "toolUse", "error", "aborted"}:
            stop_reason = cast(Literal["stop", "toolUse", "error", "aborted"], raw_stop_reason)
        else:
            stop_reason = "stop"

        error_message = payload.get("error_message")
        if error_message is not None:
            error_message = str(error_message)

        return AssistantMessage(
            content=blocks,
            stop_reason=stop_reason,
            error_message=error_message,
            timestamp=timestamp,
        )

    if role == "toolResult":
        raw_content = payload.get("content", [])
        content: list[TextBlock] = []
        if isinstance(raw_content, list):
            for raw in raw_content:
                if isinstance(raw, dict):
                    content.append(TextBlock(text=str(raw.get("text", ""))))

        return ToolResultMessage(
            toolCallId=str(payload.get("toolCallId", "")),
            toolName=str(payload.get("toolName", "")),
            content=content,
            isError=bool(payload.get("isError", False)),
            timestamp=timestamp,
        )

    raise ValueError(f"unsupported message role: {role!r}")


def serialize_content_block(block: ThinkingBlock | TextBlock | ToolCallBlock) -> dict[str, Any]:
    if isinstance(block, ThinkingBlock):
        return {"type": "thinking", "text": block.text}
    if isinstance(block, TextBlock):
        return {"type": "text", "text": block.text}
    if isinstance(block, ToolCallBlock):
        return {
            "type": "toolCall",
            "id": block.id,
            "name": block.name,
            "arguments": block.arguments,
        }
    raise TypeError(f"unsupported assistant content block type: {type(block)!r}")


def deserialize_content_block(
    payload: dict[str, Any],
) -> ThinkingBlock | TextBlock | ToolCallBlock | None:
    block_type: Any | None = payload.get("type")
    if block_type == "thinking":
        return ThinkingBlock(text=str(payload.get("text", "")))
    if block_type == "text":
        return TextBlock(text=str(payload.get("text", "")))
    if block_type == "toolCall":
        name = str(payload.get("name", ""))
        arguments = payload.get("arguments", {})
        if not isinstance(arguments, dict):
            arguments = {}
        block_id = payload.get("id")
        if block_id is None:
            return ToolCallBlock(name=name, arguments=arguments)
        return ToolCallBlock(id=str(block_id), name=name, arguments=arguments)
    return None
