from __future__ import annotations

from abc import ABC, abstractmethod

from pypimono.ui.application.presentation.view_models import UiDisplayEvent
from pypimono.ui.boundary.contracts.incoming_event import UiIncomingEvent


class UiDisplaySink(ABC):
    @abstractmethod
    def on_event(self, event: UiDisplayEvent) -> None:
        raise NotImplementedError


class UiIncomingEventSink(ABC):
    @abstractmethod
    def on_event(self, event: UiIncomingEvent) -> None:
        raise NotImplementedError
