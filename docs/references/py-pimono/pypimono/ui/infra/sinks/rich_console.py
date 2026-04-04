from __future__ import annotations

import json
from typing import Any

from rich.console import Console
from rich.markdown import Markdown
from rich.panel import Panel
from rich.syntax import Syntax
from rich.table import Table

from pypimono.ui.application.presentation.view_models import (
    AssistantTextViewModel,
    AssistantThinkingViewModel,
    AssistantToolCallViewModel,
    ToolEndViewModel,
    ToolResultViewModel,
    ToolStartViewModel,
)
from pypimono.ui.infra.sinks.base import BaseUiDisplaySink


class RichConsoleUiDisplaySink(BaseUiDisplaySink):
    def __init__(self, *, console: Console | None = None, max_preview_chars: int = 1200):
        self._console = console or Console()
        self._max_preview_chars = max_preview_chars

    def handle_tool_start(self, presented: ToolStartViewModel) -> None:
        table = Table.grid(expand=True)
        table.add_column(style="bold")
        table.add_column()
        table.add_row("name", f"[cyan]{presented.tool_name}[/cyan]")
        if presented.preview_text:
            table.add_row(
                "preview",
                Syntax(presented.preview_text, presented.preview_lexer, word_wrap=True),
            )

        self._console.print(Panel(table, title="Tool Start", border_style="cyan"))

    def handle_tool_end(self, presented: ToolEndViewModel) -> None:
        table = Table.grid(expand=True)
        table.add_column(style="bold")
        table.add_column()
        table.add_row("name", presented.tool_name)
        table.add_row(
            "status",
            "[red]error[/red]" if presented.is_error else "[green]ok[/green]",
        )
        table.add_row("summary", presented.summary_text)

        self._console.print(
            Panel(
                table,
                title="Tool End" if not presented.is_error else "Tool Error",
                border_style="green" if not presented.is_error else "red",
            )
        )

    def handle_assistant_text(self, presented: AssistantTextViewModel) -> None:
        self._console.print(
            Panel(
                Markdown(self._truncate(presented.text)),
                title="Assistant",
                border_style="blue",
            )
        )

    def handle_assistant_thinking(self, presented: AssistantThinkingViewModel) -> None:
        self._console.print(
            Panel(
                Markdown(self._truncate(presented.text)),
                title="Thinking",
                border_style="yellow",
            )
        )

    def handle_assistant_tool_call(self, presented: AssistantToolCallViewModel) -> None:
        table = Table.grid(expand=True)
        table.add_column(style="bold")
        table.add_column()
        table.add_row("id", presented.tool_call_id)
        table.add_row("name", f"[magenta]{presented.tool_name}[/magenta]")
        table.add_row("args", Syntax(self._json_dump(presented.arguments), "json", word_wrap=True))
        self._console.print(Panel(table, title="Assistant Tool Call", border_style="magenta"))

    def handle_tool_result(self, presented: ToolResultViewModel) -> None:
        border_style = "red" if presented.is_error else "green"
        title = f"Tool Result: {presented.tool_name}"

        table = Table.grid(expand=True)
        table.add_column(style="bold")
        table.add_column()
        table.add_row("id", presented.tool_call_id)
        table.add_row("status", "[red]error[/red]" if presented.is_error else "[green]ok[/green]")
        if presented.body:
            table.add_row("output", Syntax(self._truncate(presented.body), "text", word_wrap=True))

        self._console.print(Panel(table, title=title, border_style=border_style))

    def _truncate(self, text: str) -> str:
        if len(text) <= self._max_preview_chars:
            return text
        omitted = len(text) - self._max_preview_chars
        return f"{text[: self._max_preview_chars]}\n... ({omitted} more chars)"

    @staticmethod
    def _json_dump(value: Any) -> str:
        try:
            return json.dumps(value, indent=2, ensure_ascii=False, default=str)
        except Exception:
            return json.dumps(str(value), indent=2, ensure_ascii=False)


RichConsoleEventPrinter = RichConsoleUiDisplaySink
