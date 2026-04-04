from __future__ import annotations

import time
import uuid

from pypimono.session.application.ports.session_store_gateway import SessionStoreGateway
from pypimono.session.domain.entry import SessionEntryBase
from pypimono.session.domain.history import SessionHistory
from pypimono.session.domain.message import SessionMessage


def now_iso() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%S", time.gmtime())


def new_id() -> str:
    return uuid.uuid4().hex[:8]


class SessionManager:
    def __init__(
        self,
        *,
        session_store: SessionStoreGateway[SessionEntryBase],
        session_id: str = "default",
    ):
        self.session_store = session_store
        self.session_id = session_id
        self.history: SessionHistory
        self._load_from_store()

    @property
    def session_location(self) -> str:
        return self.session_store.describe(session_id=self.session_id)

    def _load_from_store(self) -> None:
        loaded_entries = self.session_store.load_entries(session_id=self.session_id)
        try:
            self.history = SessionHistory(entries=loaded_entries)
        except Exception as e:
            raise ValueError(f"invalid session entry at {self.session_location}") from e

    def append_message(self, message: SessionMessage) -> str:
        entry = self.history.append_message(
            entry_id=new_id(),
            timestamp=now_iso(),
            message=message,
        )
        self._append_to_store(entry)
        return entry.id

    def context_messages(self) -> list[SessionMessage]:
        return self.history.context_messages()

    def reset(self) -> None:
        self.session_store.delete_session(session_id=self.session_id)
        self.history = SessionHistory()

    def _append_to_store(self, entry: SessionEntryBase) -> None:
        self.session_store.append_entry(
            session_id=self.session_id,
            entry=entry,
        )
