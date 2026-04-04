from __future__ import annotations

from collections.abc import Callable, Sequence

from typing_extensions import override

from pypimono.engine.domain.messages import AgentMessage, AssistantMessage
from pypimono.engine.domain.ports.llm import LlmGateway
from pypimono.engine.domain.ports.tool import Tool
from pypimono.engine.infra.llm.codex.message_mapper import (
    assistant_from_codex_response,
    convert_input_messages,
)
from pypimono.engine.infra.llm.codex.responses_client import OpenAICodexResponsesClient
from pypimono.engine.infra.llm.codex.tooling import build_tool_specs_from_tools


class OpenAICodexLlm(LlmGateway):
    def __init__(
        self,
        model_id: str,
        *,
        auth_path: str | None = None,
        base_url: str | None = None,
        session_id: str | None = None,
        originator: str = "pi",
        text_verbosity: str = "medium",
        reasoning_effort: str | None = None,
        timeout_sec: int = 180,
        announce: Callable[[str], None] | None = None,
    ):
        self.client = OpenAICodexResponsesClient(
            model_id=model_id,
            auth_path=auth_path,
            base_url=base_url,
            session_id=session_id,
            originator=originator,
            text_verbosity=text_verbosity,
            reasoning_effort=reasoning_effort,
            timeout_sec=timeout_sec,
            announce=announce,
        )

    @override
    async def complete(
        self,
        *,
        system_prompt: str,
        messages: Sequence[AgentMessage],
        tools: Sequence[Tool],
    ) -> AssistantMessage:
        response = await self.client.complete(
            system_prompt=system_prompt,
            input_items=convert_input_messages(messages),
            tool_specs=build_tool_specs_from_tools(tools),
        )
        return assistant_from_codex_response(response)
