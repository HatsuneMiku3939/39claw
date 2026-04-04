from __future__ import annotations

from abc import ABC, abstractmethod
from collections.abc import Callable

from pypimono.ui.application.ports.event_sinks import UiIncomingEventSink
from pypimono.ui.boundary.contracts.startup import UiStartupInfo


class SessionGateway(ABC):
    @property
    @abstractmethod
    def startup_info(self) -> UiStartupInfo:
        raise NotImplementedError

    @abstractmethod
    def subscribe(self, sink: UiIncomingEventSink) -> Callable[[], None]:
        raise NotImplementedError

    @abstractmethod
    async def prompt(self, text: str) -> None:
        raise NotImplementedError
