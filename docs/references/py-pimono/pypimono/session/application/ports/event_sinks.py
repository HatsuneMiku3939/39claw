from __future__ import annotations

from abc import ABC, abstractmethod

from pypimono.session.boundary.contracts.ui_event import SessionUiEvent


class SessionEventSink(ABC):
    @abstractmethod
    def on_event(self, event: SessionUiEvent) -> None:
        raise NotImplementedError


class SessionRuntimeEventSink(ABC):
    @abstractmethod
    def on_event(self, event: SessionUiEvent) -> None:
        raise NotImplementedError
