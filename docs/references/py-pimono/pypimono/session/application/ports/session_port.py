from __future__ import annotations

from abc import ABC, abstractmethod
from collections.abc import Callable

from pypimono.session.application.ports.event_sinks import SessionEventSink
from pypimono.session.boundary.contracts.startup import SessionStartupInfo


class SessionPort(ABC):
    @property
    @abstractmethod
    def startup_info(self) -> SessionStartupInfo:
        raise NotImplementedError

    @abstractmethod
    def subscribe(self, sink: SessionEventSink) -> Callable[[], None]:
        raise NotImplementedError

    @abstractmethod
    async def reset(self) -> None:
        raise NotImplementedError

    @abstractmethod
    async def prompt(self, text: str) -> None:
        raise NotImplementedError
