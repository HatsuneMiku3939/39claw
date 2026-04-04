from __future__ import annotations

from collections.abc import Callable

from typing_extensions import override

from pypimono.ui.application.ports.event_sinks import (
    UiDisplaySink,
    UiIncomingEventSink,
)
from pypimono.ui.application.ports.session_gateway import SessionGateway
from pypimono.ui.application.ports.ui_port import UiPort
from pypimono.ui.application.presentation.ui_event_presenter import present_ui_event
from pypimono.ui.application.presentation.view_models import UiDisplayEvent
from pypimono.ui.boundary.contracts.incoming_event import UiIncomingEvent
from pypimono.ui.boundary.contracts.startup import UiStartupInfo


class ChatUi(UiPort, UiIncomingEventSink):
    def __init__(self, *, session: SessionGateway):
        self._session = session
        self._display_sinks: list[UiDisplaySink] = []
        self._session.subscribe(self)

    @property
    def startup_info(self) -> UiStartupInfo:
        return self._session.startup_info

    def subscribe(self, sink: UiDisplaySink) -> Callable[[], None]:
        self._display_sinks.append(sink)

        def unsubscribe() -> None:
            if sink in self._display_sinks:
                self._display_sinks.remove(sink)

        return unsubscribe

    @override
    def on_event(self, event: UiIncomingEvent) -> None:
        for displayed in present_ui_event(event):
            self._emit(displayed)

    async def prompt(self, text: str) -> None:
        await self._session.prompt(text)

    def _emit(self, event: UiDisplayEvent) -> None:
        for sink in list(self._display_sinks):
            sink.on_event(event)
