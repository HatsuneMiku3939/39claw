from __future__ import annotations

from dataclasses import dataclass, field
from typing import Callable

from pypimono.engine.domain.agent_event import (
    AgentEvent,
    MessageEndEvent,
    ToolExecutionEndEvent,
    ToolExecutionStartEvent,
    TurnEndEvent,
)
from pypimono.engine.domain.agent_loop import AgentLoopConfig, AgentLoopContext, run_agent_loop
from pypimono.engine.domain.messages import AgentMessage
from pypimono.engine.domain.ports.event_output import AgentEventSink
from pypimono.engine.domain.ports.llm import LlmGateway
from pypimono.engine.domain.ports.tool import Tool


@dataclass
class AgentState:
    system_prompt: str = ""
    tools: list[Tool] = field(default_factory=list)
    messages: list[AgentMessage] = field(default_factory=list)
    is_streaming: bool = False
    pending_tool_calls: set[str] = field(default_factory=set)
    error: str | None = None


class Agent:
    def __init__(
        self,
        *,
        llm: LlmGateway,
        system_prompt: str = "",
        tools: list[Tool] | None = None,
    ):
        self._llm = llm
        self._state = AgentState(system_prompt=system_prompt, tools=list(tools or []))
        self._event_sinks: list[AgentEventSink] = []
        self._steering_queue: list[AgentMessage] = []
        self._followup_queue: list[AgentMessage] = []

    @property
    def state(self) -> AgentState:
        return self._state

    def subscribe(self, sink: AgentEventSink) -> Callable[[], None]:
        self._event_sinks.append(sink)

        def unsubscribe() -> None:
            self._event_sinks.remove(sink)

        return unsubscribe

    def _emit(self, event: AgentEvent) -> None:
        for sink in list(self._event_sinks):
            sink.on_event(event)

    def set_system_prompt(self, prompt: str) -> None:
        self._state.system_prompt = prompt

    def set_tools(self, tools: list[Tool]) -> None:
        self._state.tools = list(tools)

    def reset_messages(self) -> None:
        self._state.messages = []
        self._state.pending_tool_calls.clear()
        self._state.error = None
        self._steering_queue.clear()
        self._followup_queue.clear()

    def steer(self, message: AgentMessage) -> None:
        self._steering_queue.append(message)

    def follow_up(self, message: AgentMessage) -> None:
        self._followup_queue.append(message)

    async def _dequeue_steering(self) -> list[AgentMessage]:
        if not self._steering_queue:
            return []
        # one-at-a-time
        m = self._steering_queue.pop(0)
        return [m]

    async def _dequeue_followup(self) -> list[AgentMessage]:
        if not self._followup_queue:
            return []
        m = self._followup_queue.pop(0)
        return [m]

    async def prompt(self, prompts: list[AgentMessage]) -> None:
        """
        `prompts` is the delta for the current run: the new messages that should
        be appended and processed for this turn.

        In the common case this is a one-item list containing a `UserMessage`,
        but a caller may also provide multiple messages at once, such as a user
        message plus a custom steering message. By contrast, `context.messages`
        represents the accumulated history from earlier turns.
        """
        if self._state.is_streaming:
            raise RuntimeError("Agent is already streaming")

        self._state.is_streaming = True
        self._state.error = None

        context = AgentLoopContext(
            system_prompt=self._state.system_prompt,
            messages=list(self._state.messages),
            tools=list(self._state.tools),
        )
        config = AgentLoopConfig(
            llm_complete=self._llm.complete,
            get_steering_messages=self._dequeue_steering,
            get_followup_messages=self._dequeue_followup,
        )

        try:
            async for ev in run_agent_loop(prompts=prompts, context=context, config=config):
                if isinstance(ev, ToolExecutionStartEvent):
                    self._state.pending_tool_calls.add(ev.tool_call_id)
                elif isinstance(ev, ToolExecutionEndEvent):
                    self._state.pending_tool_calls.discard(ev.tool_call_id)

                if isinstance(ev, MessageEndEvent):
                    self._state.messages.append(ev.message)

                if isinstance(ev, TurnEndEvent):
                    if ev.message.stop_reason == "error":
                        self._state.error = ev.message.error_message or "error"

                self._emit(ev)
        finally:
            self._state.is_streaming = False
            self._state.pending_tool_calls.clear()
