from __future__ import annotations

from abc import ABC, abstractmethod

from pypimono.engine.domain.agent_event import AgentEvent


class AgentEventSink(ABC):
    """Abstract sink for consuming agent runtime events."""

    @abstractmethod
    def on_event(self, event: AgentEvent) -> None:
        raise NotImplementedError
