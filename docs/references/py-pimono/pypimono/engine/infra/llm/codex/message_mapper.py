from __future__ import annotations

import json
import uuid
from collections.abc import Sequence
from typing import Any

from pypimono.engine.domain.messages import (
    AgentMessage,
    AssistantMessage,
    TextBlock,
    ThinkingBlock,
    ToolCallBlock,
    ToolResultMessage,
    UserMessage,
)
from pypimono.engine.infra.llm.codex.responses_models import (
    CodexReasoningOutputItem,
    CodexResponse,
    CodexTextOutputItem,
    CodexToolCallOutputItem,
)
from pypimono.engine.infra.llm.codex.tooling import normalize_tool_call_id


def convert_input_messages(messages: Sequence[AgentMessage]) -> list[dict[str, Any]]:
    out: list[dict[str, Any]] = []
    msg_id = 0

    for msg in messages:
        if isinstance(msg, UserMessage):
            out.append(
                {
                    "role": "user",
                    "content": [{"type": "input_text", "text": msg.content}],
                }
            )
            continue

        if isinstance(msg, AssistantMessage):
            for block in msg.content:
                if isinstance(block, TextBlock):
                    out.append(
                        {
                            "type": "message",
                            "role": "assistant",
                            "content": [
                                {
                                    "type": "output_text",
                                    "text": block.text,
                                    "annotations": [],
                                }
                            ],
                            "status": "completed",
                            "id": f"msg_{msg_id}",
                        }
                    )
                    msg_id += 1
                    continue

                if isinstance(block, ThinkingBlock):
                    # Engine policy strips thinking before LLM calls. Ignore defensively if any remain.
                    continue

                if isinstance(block, ToolCallBlock):
                    call_id, item_id = normalize_tool_call_id(block.id)
                    out.append(
                        {
                            "type": "function_call",
                            "id": item_id,
                            "call_id": call_id,
                            "name": block.name,
                            "arguments": json.dumps(block.arguments, ensure_ascii=False),
                        }
                    )

            continue

        if isinstance(msg, ToolResultMessage):
            call_id = msg.toolCallId.split("|", 1)[0] if msg.toolCallId else f"call_{uuid.uuid4().hex[:12]}"
            content = "\n".join(part.text for part in msg.content)
            out.append(
                {
                    "type": "function_call_output",
                    "call_id": call_id,
                    "output": content if content else "(empty tool result)",
                }
            )

    return out


def assistant_from_codex_response(response: CodexResponse) -> AssistantMessage:
    content: list[ThinkingBlock | TextBlock | ToolCallBlock] = []
    has_tool_calls = False

    for item in response.output_items:
        if isinstance(item, CodexReasoningOutputItem):
            if item.summary:
                content.append(ThinkingBlock(text=item.summary))
            continue

        if isinstance(item, CodexTextOutputItem):
            if item.text:
                content.append(TextBlock(text=item.text))
            continue

        if isinstance(item, CodexToolCallOutputItem):
            if not item.name.strip():
                continue
            call_id = item.call_id.strip() if item.call_id else ""
            if not call_id:
                call_id = f"call_{uuid.uuid4().hex[:12]}"
            full_call_id = call_id
            if item.item_id and item.item_id.strip():
                full_call_id = f"{call_id}|{item.item_id}"
            content.append(
                ToolCallBlock(
                    id=full_call_id,
                    name=item.name.strip(),
                    arguments=item.arguments if isinstance(item.arguments, dict) else {},
                )
            )
            has_tool_calls = True

    if response.status in {"failed", "cancelled", "error"}:
        return AssistantMessage(
            content=content,
            stop_reason="error",
            error_message=response.error_message,
        )
    if has_tool_calls:
        return AssistantMessage(content=content, stop_reason="toolUse")
    return AssistantMessage(content=content, stop_reason="stop")
