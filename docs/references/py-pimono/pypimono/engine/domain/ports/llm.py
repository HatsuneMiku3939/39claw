from __future__ import annotations

from abc import ABC, abstractmethod
from collections.abc import Sequence

from pypimono.engine.domain.messages import AgentMessage, AssistantMessage
from pypimono.engine.domain.ports.tool import Tool


class LlmGateway(ABC):
    @abstractmethod
    async def complete(
        self,
        *,
        system_prompt: str,
        messages: Sequence[AgentMessage],
        tools: Sequence[Tool],
    ) -> AssistantMessage:
        raise NotImplementedError
