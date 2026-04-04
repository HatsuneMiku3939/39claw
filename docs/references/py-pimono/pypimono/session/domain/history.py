from __future__ import annotations

from dataclasses import dataclass, field

from pypimono.session.domain.entry import MessageEntry, SessionEntryBase
from pypimono.session.domain.message import SessionMessage


@dataclass
class SessionHistory:
    entries: list[SessionEntryBase] = field(default_factory=list)
    leaf_id: str | None = field(init=False, default=None)

    def __post_init__(self) -> None:
        self.entries = list(self.entries)
        for index, entry in enumerate(self.entries, start=1):
            if not isinstance(entry, SessionEntryBase):
                raise TypeError(f"invalid session entry at index {index}: {type(entry)!r}")
            self.leaf_id = entry.id

    def append_message(
        self,
        *,
        entry_id: str,
        timestamp: str,
        message: SessionMessage,
    ) -> MessageEntry:
        entry = MessageEntry(
            id=entry_id,
            parentId=self.leaf_id,
            timestamp=timestamp,
            message=message,
        )
        self.entries.append(entry)
        self.leaf_id = entry.id
        return entry

    def context_messages(self) -> list[SessionMessage]:
        out: list[SessionMessage] = []
        for entry in self.entries:
            if isinstance(entry, MessageEntry) and entry.message is not None:
                out.append(entry.message)
        return out
