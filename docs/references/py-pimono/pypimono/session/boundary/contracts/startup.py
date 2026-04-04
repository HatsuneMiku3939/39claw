from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class SessionStartupInfo:
    session_location: str
    restored_message_count: int = 0

    @property
    def is_restored(self) -> bool:
        return self.restored_message_count > 0
