from __future__ import annotations

from abc import ABC, abstractmethod

from pypimono.ui.application.ports.ui_port import UiPort


class UiRunner(ABC):
    @abstractmethod
    async def run(self, ui: UiPort) -> None:
        raise NotImplementedError

    async def notify_background(self, text: str) -> None:
        del text
