from __future__ import annotations

import asyncio
from dataclasses import dataclass
from typing import Literal

from pypimono.ui.application.presentation.view_models import (
    AssistantTextViewModel,
    AssistantThinkingViewModel,
    AssistantToolCallViewModel,
    BackgroundNoticeViewModel,
    ToolEndViewModel,
    ToolResultViewModel,
    ToolStartViewModel,
)
from pypimono.ui.infra.sinks.base import BaseUiDisplaySink


@dataclass(frozen=True)
class DiscordTurnUpdate:
    kind: Literal["tool_end", "assistant_text", "done"]
    text: str = ""
    is_error: bool = False


class DiscordUiDisplaySink(BaseUiDisplaySink):
    def __init__(self) -> None:
        self._queue: asyncio.Queue[DiscordTurnUpdate] | None = None
        self._background_notice_emitter = None

    def begin_turn(self) -> asyncio.Queue[DiscordTurnUpdate]:
        self._queue = asyncio.Queue()
        return self._queue

    def end_turn(self) -> None:
        self._publish(DiscordTurnUpdate(kind="done"))

    def set_background_notice_emitter(self, emitter) -> None:
        self._background_notice_emitter = emitter

    def handle_tool_start(self, presented: ToolStartViewModel) -> None:
        return

    def handle_tool_end(self, presented: ToolEndViewModel) -> None:
        summary = presented.summary_text.strip() or presented.tool_name
        self._publish(
            DiscordTurnUpdate(
                kind="tool_end",
                text=summary,
                is_error=presented.is_error,
            )
        )

    def handle_assistant_thinking(self, presented: AssistantThinkingViewModel) -> None:
        return

    def handle_assistant_text(self, presented: AssistantTextViewModel) -> None:
        text = presented.text.strip()
        if not text:
            return
        self._publish(DiscordTurnUpdate(kind="assistant_text", text=text))

    def handle_assistant_tool_call(self, presented: AssistantToolCallViewModel) -> None:
        return

    def handle_tool_result(self, presented: ToolResultViewModel) -> None:
        return

    def handle_background_notice(self, presented: BackgroundNoticeViewModel) -> None:
        if self._background_notice_emitter is None:
            return
        prefix = f"[{presented.source}] " if presented.source else ""
        loop = asyncio.get_running_loop()
        loop.create_task(self._background_notice_emitter(f"{prefix}{presented.text}".strip()))

    def _publish(self, update: DiscordTurnUpdate) -> None:
        if self._queue is None:
            return

        queue = self._queue
        if update.kind == "done":
            self._queue = None
        queue.put_nowait(update)
