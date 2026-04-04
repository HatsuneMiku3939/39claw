from __future__ import annotations

from abc import ABC, abstractmethod
from collections.abc import Callable
from dataclasses import dataclass

from pypimono.session.application.ports.event_sinks import (
    SessionRuntimeEventSink,
)
from pypimono.session.boundary.contracts.message import SessionMessage


@dataclass(frozen=True)
class RuntimeToolInfo:
    name: str
    description: str


class AgentRuntimeGateway(ABC):
    @abstractmethod
    def set_system_prompt(self, prompt: str) -> None:
        raise NotImplementedError

    @abstractmethod
    def list_tools(self) -> list[RuntimeToolInfo]:
        raise NotImplementedError

    @abstractmethod
    def has_messages(self) -> bool:
        raise NotImplementedError

    @abstractmethod
    def restore_messages(self, messages: list[SessionMessage]) -> None:
        raise NotImplementedError

    @abstractmethod
    def append_messages(self, messages: list[SessionMessage]) -> None:
        raise NotImplementedError

    @abstractmethod
    def clear_messages(self) -> None:
        raise NotImplementedError

    @abstractmethod
    def subscribe(self, sink: SessionRuntimeEventSink) -> Callable[[], None]:
        raise NotImplementedError

    @abstractmethod
    async def prompt(self, prompts: list[SessionMessage]) -> None:
        raise NotImplementedError
