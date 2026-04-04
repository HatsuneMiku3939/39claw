from __future__ import annotations

from collections.abc import Callable

from typing_extensions import override

from pypimono.integration.session_ui.event_mapper import to_ui_incoming_event
from pypimono.integration.session_ui.startup_mapper import to_ui_startup_info
from pypimono.session.application.ports.event_sinks import (
    SessionEventSink,
)
from pypimono.session.application.ports.session_port import SessionPort
from pypimono.session.boundary.contracts.ui_event import SessionUiEvent
from pypimono.ui.application.ports.event_sinks import UiIncomingEventSink
from pypimono.ui.application.ports.session_gateway import SessionGateway
from pypimono.ui.boundary.contracts.startup import UiStartupInfo


class _SessionToUiEventBridge(SessionEventSink):
    def __init__(self, *, sink: UiIncomingEventSink):
        self._sink = sink

    @override
    def on_event(self, event: SessionUiEvent) -> None:
        self._sink.on_event(to_ui_incoming_event(event))


class SessionUiGatewayAdapter(SessionGateway):
    def __init__(self, *, session: SessionPort):
        self._session = session

    @property
    def startup_info(self) -> UiStartupInfo:
        return to_ui_startup_info(self._session.startup_info)

    def subscribe(self, sink: UiIncomingEventSink) -> Callable[[], None]:
        bridge = _SessionToUiEventBridge(sink=sink)
        return self._session.subscribe(bridge)

    async def prompt(self, text: str) -> None:
        await self._session.prompt(text)
