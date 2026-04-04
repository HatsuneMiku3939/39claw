from __future__ import annotations

from abc import ABC, abstractmethod


class WorkspaceFsGateway(ABC):
    @abstractmethod
    def workspace_root(self) -> str:
        raise NotImplementedError

    @abstractmethod
    def resolve_path(self, path: str) -> str:
        raise NotImplementedError

    @abstractmethod
    def read_text(self, path: str, *, encoding: str = "utf-8") -> str:
        raise NotImplementedError

    @abstractmethod
    def write_text(self, path: str, content: str, *, encoding: str = "utf-8") -> int:
        raise NotImplementedError
