from __future__ import annotations

from collections.abc import Callable

from rich.markdown import Markdown as RichMarkdown
from rich.style import Style
from textual import events
from textual.app import App, ComposeResult
from textual.containers import Horizontal
from textual.message import Message
from textual.selection import Selection
from textual.strip import Strip
from textual.widgets import Header, RichLog, Static, TextArea
from typing_extensions import override

from pypimono.ui.application.ports.ui_port import UiPort
from pypimono.ui.application.presentation.startup_formatter import format_ui_startup
from pypimono.ui.infra.sinks.textual import TextualUiDisplaySink

_MIN_PROMPT_ROWS = 1
_MAX_PROMPT_ROWS = 8
_PROMPT_NEWLINE_KEYS = {
    "shift+enter",
    "shift+return",
    "shift+ctrl+m",
    "ctrl+j",
    "alt+enter",
    "meta+enter",
}


class SelectableRichLog(RichLog):
    """RichLog with visible mouse drag selection and copy support."""

    def get_selection(self, selection: Selection) -> tuple[str, str] | None:
        text = "\n".join(line.text for line in self.lines)
        return selection.extract(text), "\n"

    def selection_updated(self, selection: Selection | None) -> None:
        self._line_cache.clear()
        self.refresh()

    def render_line(self, y: int) -> Strip:
        scroll_x, scroll_y = self.scroll_offset
        line_index = scroll_y + y
        strip = super().render_line(y)
        selection = self.text_selection
        if selection is None:
            return strip.apply_offsets(scroll_x, line_index)

        span = selection.get_span(line_index)
        if span is None:
            return strip.apply_offsets(scroll_x, line_index)

        start, end = span
        if end == -1:
            end = self._widest_line_width

        visible_start = max(0, start - scroll_x)
        visible_end = min(strip.cell_length, end - scroll_x)
        if visible_end <= visible_start:
            return strip

        selection_style: Style = self.screen.get_component_rich_style("screen--selection")

        if visible_start == 0 and visible_end >= strip.cell_length:
            return strip.apply_style(selection_style).apply_offsets(scroll_x, line_index)

        left, middle, right = strip.divide([visible_start, visible_end, strip.cell_length])
        return Strip.join([left, middle.apply_style(selection_style), right]).apply_offsets(
            scroll_x,
            line_index,
        )


class PromptInputArea(TextArea):
    class Submitted(Message):
        @override
        def __init__(self, prompt_input: PromptInputArea):
            super().__init__()
            self.prompt_input = prompt_input

        @property
        def control(self) -> PromptInputArea:
            return self.prompt_input

    async def _on_key(self, event: events.Key) -> None:
        key = event.key
        if key in _PROMPT_NEWLINE_KEYS:
            event.stop()
            event.prevent_default()
            if not self.read_only:
                self.insert("\n")
            return

        if key == "enter":
            event.stop()
            event.prevent_default()
            self.post_message(self.Submitted(self))
            return

        await super()._on_key(event)


class TextualChatApp(App[None]):
    BINDINGS = [("ctrl+q", "quit", "Quit")]
    CSS = """
    Screen {
        layout: vertical;
    }

    #chat-log {
        height: 1fr;
        border: none;
        padding: 0 1;
    }

    #prompt-bar {
        height: auto;
        min-height: 3;
        border-top: none;
        background: $boost;
        padding: 0 1;
    }

    #prompt-label {
        width: 6;
        color: $primary;
        text-style: bold;
        content-align: center middle;
    }

    #prompt-input {
        width: 1fr;
        min-height: 1;
        max-height: 8;
        border: none;
        margin: 0;
    }
    """

    class AssistantChunk(Message):
        @override
        def __init__(self, text: str):
            super().__init__()
            self.text = text

    class ThinkingChunk(Message):
        @override
        def __init__(self, text: str):
            super().__init__()
            self.text = text

    class BackgroundNotice(Message):
        @override
        def __init__(self, text: str, source: str | None):
            super().__init__()
            self.text = text
            self.source = source

    class ToolStarted(Message):
        @override
        def __init__(self, name: str, preview_text: str, preview_lexer: str):
            super().__init__()
            self.name = name
            self.preview_text = preview_text
            self.preview_lexer = preview_lexer

    class ToolFinished(Message):
        @override
        def __init__(self, name: str, is_error: bool, summary_text: str):
            super().__init__()
            self.name = name
            self.is_error = is_error
            self.summary_text = summary_text

    class PromptDone(Message):
        pass

    class PromptFailed(Message):
        @override
        def __init__(self, error_text: str):
            super().__init__()
            self.error_text = error_text

    @override
    def __init__(self, *, ui: UiPort):
        super().__init__()
        self._ui = ui
        self._unsubscribe_ui: Callable[[], None] | None = None
        self._busy = False

    @override
    def compose(self) -> ComposeResult:
        yield Header()
        yield SelectableRichLog(id="chat-log", wrap=True, auto_scroll=True, markup=True)
        yield Horizontal(
            Static("YOU>", id="prompt-label"),
            PromptInputArea(
                placeholder="Type a message. (Enter to send, Shift+Enter for newline, /exit to quit)",
                compact=True,
                id="prompt-input",
            ),
            id="prompt-bar",
        )

    def on_mount(self) -> None:
        self._unsubscribe_ui = self._ui.subscribe(
            TextualUiDisplaySink(
                emit_assistant=lambda text: self.post_message(self.AssistantChunk(text)),
                emit_thinking=lambda text: self.post_message(self.ThinkingChunk(text)),
                emit_notice=lambda text, source: self.post_message(self.BackgroundNotice(text, source)),
                emit_tool_start=lambda name, preview_text, preview_lexer: self.post_message(
                    self.ToolStarted(name, preview_text, preview_lexer)
                ),
                emit_tool_end=lambda name, is_error, summary_text: self.post_message(
                    self.ToolFinished(name, is_error, summary_text)
                ),
            )
        )
        self._log_line(f"[yellow]{format_ui_startup(self._ui.startup_info)}[/yellow]")
        self._log_line(
            "[white]Interactive agent started. Type a message. Exit with /exit or Ctrl+Q[/white]"
        )
        self._log_line(
            "[dim]Drag the log area to select text, Ctrl+C to copy. Enter sends, Shift+Enter"
            " inserts a newline (some terminals use Ctrl+J instead).[/dim]"
        )
        self._log_line("")
        prompt_input = self.query_one("#prompt-input", PromptInputArea)
        prompt_input.focus()
        self._sync_prompt_input_height(prompt_input)

    def on_text_area_changed(self, event: TextArea.Changed) -> None:
        if event.text_area.id != "prompt-input":
            return
        self._sync_prompt_input_height(event.text_area)

    def on_prompt_input_area_submitted(self, message: PromptInputArea.Submitted) -> None:
        if message.prompt_input.id != "prompt-input":
            return
        self.action_submit_prompt()

    def action_submit_prompt(self) -> None:
        if self._busy:
            return

        prompt_input = self.query_one("#prompt-input", PromptInputArea)
        user_text = prompt_input.text.strip()
        prompt_input.text = ""
        self._sync_prompt_input_height(prompt_input)

        if not user_text:
            return

        if user_text.lower() in {"quit", "exit", "/exit"}:
            self.exit()
            return

        self._log_line("[bold green]You[/bold green]")
        self._log_markdown(user_text)
        self._log_line("")
        self._set_busy(True)
        self.run_worker(self._run_prompt(user_text), exclusive=True)

    def on_textual_chat_app_assistant_chunk(self, message: AssistantChunk) -> None:
        self._log_line("[bold cyan]Assistant[/bold cyan]")
        self._log_markdown(message.text)
        self._log_line("")

    def on_textual_chat_app_thinking_chunk(self, message: ThinkingChunk) -> None:
        self._log_line("[bold yellow]Thinking[/bold yellow]")
        self._log_markdown(message.text)
        self._log_line("")

    def on_textual_chat_app_background_notice(self, message: BackgroundNotice) -> None:
        source = message.source or "Notice"
        self._log_line(f"[bold blue]{source}[/bold blue]")
        self._log_markdown(message.text)
        self._log_line("")

    def on_textual_chat_app_tool_started(self, message: ToolStarted) -> None:
        self._log_line(f"[bold magenta]Tool Start[/bold magenta] {message.name}")
        preview = message.preview_text.strip()
        if preview:
            self._log_markdown(
                f"```{message.preview_lexer}\n{_truncate_block(preview, 1200)}\n```"
            )

    def on_textual_chat_app_tool_finished(self, message: ToolFinished) -> None:
        status = "[red]error[/red]" if message.is_error else "[green]ok[/green]"
        summary = _truncate_block(message.summary_text, 240).strip()
        if summary:
            self._log_line(f"[bold magenta]Tool End[/bold magenta] {status} {summary}")
            return
        self._log_line(f"[bold magenta]Tool End[/bold magenta] {status} {message.name}")

    def on_textual_chat_app_prompt_failed(self, message: PromptFailed) -> None:
        self._log_line(f"[bold red]Error[/bold red] {message.error_text}")

    def on_textual_chat_app_prompt_done(self, _: PromptDone) -> None:
        self._set_busy(False)

    async def _run_prompt(self, user_text: str) -> None:
        try:
            await self._ui.prompt(user_text)
        except Exception as exc:  # pragma: no cover - runtime UI path
            self.post_message(self.PromptFailed(str(exc)))
        finally:
            self.post_message(self.PromptDone())

    def detach_ui(self) -> None:
        if self._unsubscribe_ui is None:
            return
        unsubscribe = self._unsubscribe_ui
        self._unsubscribe_ui = None
        unsubscribe()

    def _set_busy(self, busy: bool) -> None:
        self._busy = busy
        prompt_input = self.query_one("#prompt-input", PromptInputArea)
        prompt_input.disabled = busy
        if not busy:
            prompt_input.focus()

    def _log_line(self, text: str) -> None:
        chat_log = self.query_one("#chat-log", SelectableRichLog)
        chat_log.write(text, scroll_end=True)

    def _log_markdown(self, body: str) -> None:
        chat_log = self.query_one("#chat-log", SelectableRichLog)
        chat_log.write(RichMarkdown(body), scroll_end=True)

    def _sync_prompt_input_height(self, prompt_input: TextArea) -> None:
        rows = max(_MIN_PROMPT_ROWS, min(_MAX_PROMPT_ROWS, prompt_input.wrapped_document.height))
        prompt_input.styles.height = rows


def _truncate_block(text: str, limit: int) -> str:
    compact = text.strip()
    if len(compact) <= limit:
        return compact
    return f"{compact[:limit]}..."
