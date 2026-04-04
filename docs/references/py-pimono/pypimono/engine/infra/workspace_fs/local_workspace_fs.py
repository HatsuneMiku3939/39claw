from __future__ import annotations

from pathlib import Path

from typing_extensions import override

from pypimono.engine.infra.workspace_fs.port import WorkspaceFsGateway


class LocalWorkspaceFs(WorkspaceFsGateway):
    def __init__(self, root: str | Path):
        self._root = Path(root).resolve()

    @override
    def workspace_root(self) -> str:
        return str(self._root)

    @override
    def resolve_path(self, path: str) -> str:
        return str(self._resolve_path(path))

    def _resolve_path(self, path: str) -> Path:
        raw = str(path).strip()
        if not raw:
            raise ValueError("path must not be empty")

        candidate = Path(raw)
        if not candidate.is_absolute():
            candidate = self._root / candidate

        resolved = candidate.resolve()
        try:
            resolved.relative_to(self._root)
        except ValueError as exc:
            raise PermissionError(f"path escapes workspace: {path}") from exc

        return resolved

    @override
    def read_text(self, path: str, *, encoding: str = "utf-8") -> str:
        target = self._resolve_path(path)
        if not target.exists() or not target.is_file():
            raise FileNotFoundError(f"read: file not found: {path}")
        return target.read_text(encoding=encoding)

    @override
    def write_text(self, path: str, content: str, *, encoding: str = "utf-8") -> int:
        target = self._resolve_path(path)
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(content, encoding=encoding)
        return len(content.encode(encoding))
