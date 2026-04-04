from __future__ import annotations

import asyncio
from collections.abc import Sequence

from typing_extensions import override

from pypimono.engine.domain.messages import (
    AgentMessage,
    AssistantMessage,
    TextBlock,
    ToolCallBlock,
    ToolResultMessage,
    UserMessage,
)
from pypimono.engine.domain.ports.llm import LlmGateway
from pypimono.engine.domain.ports.tool import Tool


class MockLlm(LlmGateway):
    """
    Rule-based LLM stub.
    - If the last message is a user message and starts with "read: <path>",
      produce a read tool call.
    - If the last message is a tool result, summarize it into a final reply.
    """

    @override
    async def complete(
        self,
        *,
        system_prompt: str,
        messages: Sequence[AgentMessage],
        tools: Sequence[Tool],
    ) -> AssistantMessage:
        del system_prompt
        await asyncio.sleep(1)
        tool_names = {tool.name for tool in tools}

        if not messages:
            return AssistantMessage(content=[TextBlock(text="(mock) empty context")], stop_reason="stop")

        last = messages[-1]

        if isinstance(last, UserMessage):
            text = last.content.strip()
            if text.lower().startswith("read:"):
                path = text.split(":", 1)[1].strip()
                if "read" not in tool_names:
                    return AssistantMessage(
                        content=[TextBlock(text="(mock) read tool not available")],
                        stop_reason="stop",
                    )
                return AssistantMessage(
                    content=[ToolCallBlock(name="read", arguments={"path": path})],
                    stop_reason="toolUse",
                )
            return AssistantMessage(
                content=[TextBlock(text=f"(mock) you said: {text}")],
                stop_reason="stop",
            )

        if isinstance(last, ToolResultMessage):
            body = "\n".join(b.text for b in last.content)
            snippet = body if len(body) <= 200 else body[:200] + "…"
            return AssistantMessage(
                content=[TextBlock(text=f"(mock) read result excerpt:\n{snippet}")],
                stop_reason="stop",
            )

        return AssistantMessage(content=[TextBlock(text="(mock) ok")], stop_reason="stop")
