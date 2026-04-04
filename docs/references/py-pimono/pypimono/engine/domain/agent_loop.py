from __future__ import annotations

from dataclasses import dataclass, field
from typing import Awaitable, Callable

from pypimono.engine.domain.agent_event import (
    AgentEndEvent,
    AgentStartEvent,
    MessageEndEvent,
    MessageStartEvent,
    ToolExecutionEndEvent,
    ToolExecutionStartEvent,
    TurnEndEvent,
    TurnStartEvent,
)
from pypimono.engine.domain.messages import (
    AgentMessage,
    AssistantMessage,
    TextBlock,
    ToolCallBlock,
    ToolResultMessage,
    strip_thinking_from_messages,
)
from pypimono.engine.domain.ports.tool import Tool
from pypimono.engine.domain.tool_execution import execute_tool_call


@dataclass
class AgentLoopContext:
    system_prompt: str
    messages: list[AgentMessage]
    tools: list[Tool]


GetQueuedMessages = Callable[[], Awaitable[list[AgentMessage]]]


@dataclass
class AgentLoopConfig:
    llm_complete: Callable[..., Awaitable[AssistantMessage]]
    get_steering_messages: GetQueuedMessages | None = None
    get_followup_messages: GetQueuedMessages | None = None


@dataclass
class _AgentLoopState:
    system_prompt: str
    messages: list[AgentMessage]
    tools: list[Tool]
    new_messages: list[AgentMessage]
    pending: list[AgentMessage] = field(default_factory=list)
    has_more_tool_calls: bool = True
    first_turn: bool = True

    @classmethod
    def from_inputs(
        cls,
        *,
        context: AgentLoopContext,
        prompts: list[AgentMessage],
        pending: list[AgentMessage],
    ) -> _AgentLoopState:
        return cls(
            system_prompt=context.system_prompt,
            messages=[*context.messages, *prompts],
            tools=list(context.tools),
            new_messages=list(prompts),
            pending=list(pending),
        )

    def append_message(self, message: AgentMessage) -> tuple[MessageStartEvent, MessageEndEvent]:
        self.messages.append(message)
        self.new_messages.append(message)
        return (MessageStartEvent(message=message), MessageEndEvent(message=message))


async def _dequeue(queue_fn: GetQueuedMessages | None) -> list[AgentMessage]:
    if queue_fn is None:
        return []
    return await queue_fn()


async def run_agent_loop(
    *,
    prompts: list[AgentMessage],
    context: AgentLoopContext,
    config: AgentLoopConfig,
):
    """
    Async generator of typed domain events.
    Emits `new_messages` in the final `agent_end` event.
    """
    pending = await _dequeue(config.get_steering_messages)
    state = _AgentLoopState.from_inputs(context=context, prompts=prompts, pending=pending)

    yield AgentStartEvent()
    yield TurnStartEvent()
    for prompt in prompts:
        yield MessageStartEvent(message=prompt)
        yield MessageEndEvent(message=prompt)

    while state.has_more_tool_calls or state.pending:
        if not state.first_turn:
            yield TurnStartEvent()
        state.first_turn = False

        if state.pending:
            for message in state.pending:
                started, ended = state.append_message(message)
                yield started
                yield ended
            state.pending = []

        # Preserve thinking summaries in session/UI state, but do not recycle them into model context.
        assistant = await config.llm_complete(
            system_prompt=state.system_prompt,
            messages=strip_thinking_from_messages(state.messages),
            tools=state.tools,
        )
        started, ended = state.append_message(assistant)
        yield started
        yield ended

        if assistant.stop_reason in ("error", "aborted"):
            yield TurnEndEvent(message=assistant, tool_results=[])
            yield AgentEndEvent(messages=list(state.new_messages))
            return

        tool_calls = [block for block in assistant.content if isinstance(block, ToolCallBlock)]
        state.has_more_tool_calls = len(tool_calls) > 0

        tool_results: list[ToolResultMessage] = []
        if state.has_more_tool_calls:
            for tool_call in tool_calls:
                yield ToolExecutionStartEvent(
                    tool_call_id=tool_call.id,
                    tool_name=tool_call.name,
                    args=dict(tool_call.arguments),
                )

                execution = await execute_tool_call(
                    tools=state.tools,
                    tool_call_id=tool_call.id,
                    tool_name=tool_call.name,
                    args=tool_call.arguments,
                )

                yield ToolExecutionEndEvent(
                    tool_call_id=tool_call.id,
                    tool_name=tool_call.name,
                    result=execution.result,
                    is_error=execution.is_error,
                )

                tool_result_message = ToolResultMessage(
                    toolCallId=tool_call.id,
                    toolName=tool_call.name,
                    content=[TextBlock(text=execution.result.text)],
                    isError=execution.is_error,
                )
                tool_results.append(tool_result_message)

                result_started, result_ended = state.append_message(tool_result_message)
                yield result_started
                yield result_ended

        yield TurnEndEvent(message=assistant, tool_results=tool_results)

        # Keep steering/follow-up minimal: check steering only after each turn.
        state.pending = await _dequeue(config.get_steering_messages)

    yield AgentEndEvent(messages=list(state.new_messages))
