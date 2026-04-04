from __future__ import annotations

from pathlib import Path
from typing import Callable

from typing_extensions import override

from pypimono.session.application.ports.agent_runtime_gateway import AgentRuntimeGateway
from pypimono.session.application.ports.event_sinks import (
    SessionEventSink,
    SessionRuntimeEventSink,
)
from pypimono.session.application.ports.session_port import SessionPort
from pypimono.session.application.session_manager import SessionManager
from pypimono.session.application.system_prompt_builder import build_session_system_prompt
from pypimono.session.boundary.contracts.message import UserMessage
from pypimono.session.boundary.contracts.startup import SessionStartupInfo
from pypimono.session.boundary.contracts.ui_event import (
    TYPE_MESSAGE_END,
    SessionUiEvent,
)
from pypimono.session.boundary.mappers.message_mapper import (
    to_contract_messages,
    to_domain_message,
)


class AgentSession(SessionPort, SessionRuntimeEventSink):
    def __init__(
        self,
        *,
        runtime: AgentRuntimeGateway,
        session_manager: SessionManager,
        prompt_cwd: Path | None = None,
    ):
        self.runtime = runtime
        self.session_manager = session_manager
        self._event_sinks: list[SessionEventSink] = []
        self._restored_message_count = 0
        resolved_prompt_cwd = (prompt_cwd or Path.cwd()).resolve()

        self.runtime.set_system_prompt(
            build_session_system_prompt(
                prompt_cwd=resolved_prompt_cwd,
                tools=self.runtime.list_tools(),
            )
        )

        restored = self.session_manager.context_messages()
        if restored and not self.runtime.has_messages():
            self.runtime.restore_messages(to_contract_messages(restored))
            self._restored_message_count = len(restored)

        self.runtime.subscribe(self)

    @property
    def startup_info(self) -> SessionStartupInfo:
        return SessionStartupInfo(
            session_location=self.session_manager.session_location,
            restored_message_count=self._restored_message_count,
        )

    def subscribe(self, sink: SessionEventSink) -> Callable[[], None]:
        self._event_sinks.append(sink)

        def unsubscribe() -> None:
            self._event_sinks.remove(sink)

        return unsubscribe

    def _emit(self, event: SessionUiEvent) -> None:
        for sink in list(self._event_sinks):
            sink.on_event(event)

    @override
    def on_event(self, event: SessionUiEvent) -> None:
        if event.type == TYPE_MESSAGE_END and event.message is not None:
            self.session_manager.append_message(to_domain_message(event.message))
        self._emit(event)

    async def reset(self) -> None:
        self.runtime.clear_messages()
        self.session_manager.reset()
        self._restored_message_count = 0

    async def prompt(self, text: str) -> None:
        user = UserMessage(content=text)
        await self.runtime.prompt([user])
