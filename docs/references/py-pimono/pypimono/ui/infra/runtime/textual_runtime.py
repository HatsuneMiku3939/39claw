from __future__ import annotations

from collections.abc import Callable
from typing import Any

from typing_extensions import override

from pypimono.ui.application.ports.ui_port import UiPort
from pypimono.ui.infra.runtime.base import UiRunner

TextualAppFactory = Callable[..., Any]


class TextualRuntime(UiRunner):
    def __init__(self, *, app_factory: TextualAppFactory, announce: Callable[[str], None]):
        self._app_factory = app_factory
        self._announce = announce

    @override
    async def run(self, ui: UiPort) -> None:
        app = self._app_factory(ui=ui)
        run_async = getattr(app, "run_async", None)
        if run_async is None or not callable(run_async):
            raise TypeError("textual app factory must return an object with async run_async()")
        try:
            await run_async(mouse=True)
        finally:
            detach_ui = getattr(app, "detach_ui", None)
            if callable(detach_ui):
                detach_ui()

    @override
    async def notify_background(self, text: str) -> None:
        self._announce(text)
