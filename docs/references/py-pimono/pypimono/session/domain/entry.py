from __future__ import annotations

from dataclasses import dataclass, field
from typing import Literal

from pypimono.session.domain.message import SessionMessage


@dataclass
class SessionEntryBase:
    type: str
    id: str
    parentId: str | None
    timestamp: str


@dataclass
class MessageEntry(SessionEntryBase):
    type: Literal["message"] = field(default="message", init=False)
    message: SessionMessage | None = None
