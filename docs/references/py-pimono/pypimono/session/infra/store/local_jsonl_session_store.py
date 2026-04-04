from __future__ import annotations

import json
from pathlib import Path
from typing import Any, Callable, Generic, TypeVar

from typing_extensions import override

from pypimono.session.application.ports.session_store_gateway import SessionStoreGateway

SessionEntryT = TypeVar("SessionEntryT")


class LocalJsonlSessionStore(SessionStoreGateway[SessionEntryT], Generic[SessionEntryT]):
    """SessionStoreGateway adapter that persists sessions to local JSONL files."""

    def __init__(
        self,
        *,
        base_dir: str | Path,
        serialize_entry: Callable[[SessionEntryT], dict[str, Any]],
        deserialize_entry: Callable[[dict[str, Any]], SessionEntryT],
    ):
        self._base_dir = Path(base_dir)
        self._base_dir.mkdir(parents=True, exist_ok=True)
        self._serialize_entry = serialize_entry
        self._deserialize_entry = deserialize_entry

    def _session_file(self, *, session_id: str) -> Path:
        safe_id = session_id.strip()
        if not safe_id:
            raise ValueError("session_id must not be empty")
        return self._base_dir / f"{safe_id}.jsonl"

    @override
    def load_entries(self, *, session_id: str) -> list[SessionEntryT]:
        file_path = self._session_file(session_id=session_id)
        if not file_path.exists():
            return []

        entries: list[SessionEntryT] = []
        for line_no, line in enumerate(file_path.read_text(encoding="utf-8").splitlines(), start=1):
            raw = line.strip()
            if not raw:
                continue
            try:
                payload = json.loads(raw)
            except Exception as e:
                raise ValueError(f"invalid JSONL record at {file_path}:{line_no}") from e
            if not isinstance(payload, dict):
                raise ValueError(f"invalid JSONL record at {file_path}:{line_no}: object expected")
            try:
                entries.append(self._deserialize_entry(payload))
            except Exception as e:
                raise ValueError(f"invalid JSONL record at {file_path}:{line_no}") from e
        return entries

    @override
    def append_entry(self, *, session_id: str, entry: SessionEntryT) -> None:
        file_path = self._session_file(session_id=session_id)
        payload = self._serialize_entry(entry)
        with file_path.open("a", encoding="utf-8") as f:
            f.write(json.dumps(payload, ensure_ascii=False))
            f.write("\n")

    @override
    def delete_session(self, *, session_id: str) -> None:
        file_path = self._session_file(session_id=session_id)
        if file_path.exists():
            file_path.unlink()

    @override
    def describe(self, *, session_id: str) -> str:
        return str(self._session_file(session_id=session_id))
