from __future__ import annotations

import asyncio
from typing import Callable

from typing_extensions import override

from pypimono.settings import OutputStyle
from pypimono.ui.application.ports.ui_port import UiPort
from pypimono.ui.application.presentation.startup_formatter import format_ui_startup
from pypimono.ui.infra.runtime.base import UiRunner
from pypimono.ui.infra.sinks.console import ConsoleUiDisplaySink

InputReader = Callable[[str], str]


class ConsoleRuntime(UiRunner):
    def __init__(
        self,
        *,
        output_style: OutputStyle,
        announce: Callable[[str], None],
    ):
        self._output_style = output_style
        self._announce = announce

    @override
    async def run(self, ui: UiPort) -> None:
        display_sink, input_reader = self._create_console_io()
        ui.subscribe(display_sink)

        self._announce(format_ui_startup(ui.startup_info))
        self._announce("Interactive agent started. Type quit/exit to leave.")
        self._announce("Example: read: hello.txt")

        while True:
            try:
                user_text = (await self._ainput("\nYOU> ", reader=input_reader)).strip()
            except (EOFError, KeyboardInterrupt):
                self._announce("\nExiting.")
                break

            if not user_text:
                continue
            if user_text.lower() in {"quit", "exit"}:
                self._announce("Exiting.")
                break

            await ui.prompt(user_text)

    @override
    async def notify_background(self, text: str) -> None:
        self._announce(text)

    async def _ainput(self, prompt: str, *, reader: InputReader = input) -> str:
        return await asyncio.to_thread(reader, prompt)

    def _create_console_io(self):
        if self._output_style == OutputStyle.RICH:
            try:
                from rich.console import Console

                from pypimono.ui.infra.sinks.rich_console import (
                    RichConsoleUiDisplaySink,
                )

                console = Console()

                def rich_input_reader(_: str) -> str:
                    return console.input("\n[bold cyan]YOU> [/bold cyan]")

                return RichConsoleUiDisplaySink(console=console), rich_input_reader
            except ModuleNotFoundError:
                self._announce("rich is not installed. Falling back to plain console output.")
                return ConsoleUiDisplaySink(), input
        return ConsoleUiDisplaySink(), input
