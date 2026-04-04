from __future__ import annotations

from abc import ABC, abstractmethod
from typing import Generic, TypeVar

SessionEntryT = TypeVar("SessionEntryT")


class SessionStoreGateway(ABC, Generic[SessionEntryT]):
    """Session persistence output port.

    The application layer uses this port without knowing where session data is stored
    (for example, in files, databases, or remote storage). Concrete adapters are
    provided by the infrastructure layer.
    """

    @abstractmethod
    def load_entries(self, *, session_id: str) -> list[SessionEntryT]:
        """Load session entries for a session.

        Args:
            session_id: Logical session identifier.

        Returns:
            Stored entries in insertion order. If no session exists, return an empty list.
        """
        raise NotImplementedError

    @abstractmethod
    def append_entry(self, *, session_id: str, entry: SessionEntryT) -> None:
        """Append one entry to a session.

        Args:
            session_id: Logical session identifier.
            entry: A session entry object.
        """
        raise NotImplementedError

    @abstractmethod
    def delete_session(self, *, session_id: str) -> None:
        """Delete all persisted data for a session.

        Args:
            session_id: Logical session identifier.
        """
        raise NotImplementedError

    @abstractmethod
    def describe(self, *, session_id: str) -> str:
        """Return a human-readable storage location for the session.

        Primarily used in CLI output and log messages.
        """
        raise NotImplementedError
