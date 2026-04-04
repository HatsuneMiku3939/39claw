from __future__ import annotations

from collections.abc import Callable

from typing_extensions import override

from pypimono.engine.application.agent import Agent
from pypimono.engine.domain import agent_event as engine_events
from pypimono.engine.domain.ports.event_output import AgentEventSink
from pypimono.integration.engine_session.event_mapper import (
    to_session_runtime_event,
)
from pypimono.integration.engine_session.message_mapper import to_engine_message
from pypimono.session.application.ports.agent_runtime_gateway import AgentRuntimeGateway, RuntimeToolInfo
from pypimono.session.application.ports.event_sinks import (
    SessionRuntimeEventSink,
)
from pypimono.session.boundary.contracts import message as session_messages


class _EngineToSessionRuntimeEventBridge(AgentEventSink):
    def __init__(self, *, sink: SessionRuntimeEventSink):
        self._sink: SessionRuntimeEventSink = sink

    @override
    def on_event(self, event: engine_events.AgentEvent) -> None:
        self._sink.on_event(to_session_runtime_event(event))


class EngineAgentRuntimeAdapter(AgentRuntimeGateway):
    def __init__(self, *, agent: Agent):
        self._agent = agent

    @override
    def set_system_prompt(self, prompt: str) -> None:
        self._agent.set_system_prompt(prompt)

    @override
    def list_tools(self) -> list[RuntimeToolInfo]:
        return [
            RuntimeToolInfo(name=tool.name, description=tool.description)
            for tool in self._agent.state.tools
        ]

    @override
    def has_messages(self) -> bool:
        return bool(self._agent.state.messages)

    @override
    def restore_messages(self, messages: list[session_messages.SessionMessage]) -> None:
        self._agent.state.messages = [to_engine_message(message) for message in messages]

    @override
    def append_messages(self, messages: list[session_messages.SessionMessage]) -> None:
        self._agent.state.messages.extend(to_engine_message(message) for message in messages)

    @override
    def clear_messages(self) -> None:
        self._agent.reset_messages()

    @override
    def subscribe(
        self,
        sink: SessionRuntimeEventSink,
    ) -> Callable[[], None]:
        bridge = _EngineToSessionRuntimeEventBridge(sink=sink)
        return self._agent.subscribe(bridge)

    @override
    async def prompt(self, prompts: list[session_messages.SessionMessage]) -> None:
        await self._agent.prompt([to_engine_message(prompt) for prompt in prompts])
