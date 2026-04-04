from __future__ import annotations

import json
import os
import time
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

from pypimono.engine.infra.llm.codex.auth_models import CodexOAuthCredentials
from pypimono.engine.infra.llm.codex.oauth_client import (
    extract_access_expiry_ms,
    extract_account_id,
)


def default_codex_auth_path() -> Path:
    env_path = os.getenv("CODEX_AUTH_PATH")
    if env_path:
        return Path(env_path).expanduser()
    return Path.home() / ".codex" / "auth.json"


def resolve_codex_auth_path(path: str | Path | None = None) -> Path:
    if path is not None:
        return Path(path).expanduser()
    return default_codex_auth_path()


def _load_auth_payload(path: Path) -> dict[str, Any]:
    if not path.exists():
        return {}
    try:
        loaded = json.loads(path.read_text(encoding="utf-8"))
        return loaded if isinstance(loaded, dict) else {}
    except Exception:
        return {}


def _save_auth_payload(path: Path, payload: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    try:
        os.chmod(path, 0o600)
    except Exception:
        pass


def load_codex_credentials(
    path: str | Path | None = None,
) -> CodexOAuthCredentials | None:
    auth_path = resolve_codex_auth_path(path)
    payload = _load_auth_payload(auth_path)
    tokens = payload.get("tokens")
    if not isinstance(tokens, dict):
        return None

    access = str(tokens.get("access_token", "")).strip()
    refresh = str(tokens.get("refresh_token", "")).strip()
    if not access or not refresh:
        return None

    account_id = str(tokens.get("account_id", "")).strip()
    if not account_id:
        try:
            account_id = extract_account_id(access)
        except RuntimeError:
            return None

    id_token = tokens.get("id_token")
    expires_at_ms = extract_access_expiry_ms(access) or (int(time.time() * 1000) + 5 * 60 * 1000)

    return CodexOAuthCredentials(
        access_token=access,
        refresh_token=refresh,
        expires_at_ms=expires_at_ms,
        account_id=account_id,
        id_token=str(id_token) if isinstance(id_token, str) and id_token else None,
    )


def save_codex_credentials(credentials: CodexOAuthCredentials, path: str | Path | None = None) -> None:
    auth_path = resolve_codex_auth_path(path)
    payload = _load_auth_payload(auth_path)
    payload.setdefault("auth_mode", "chatgpt")

    tokens: dict[str, Any] = {}
    existing_tokens = payload.get("tokens")
    if isinstance(existing_tokens, dict):
        tokens.update(existing_tokens)

    tokens["access_token"] = credentials.access_token
    tokens["refresh_token"] = credentials.refresh_token
    tokens["account_id"] = credentials.account_id
    if credentials.id_token:
        tokens["id_token"] = credentials.id_token
    payload["tokens"] = tokens
    payload["last_refresh"] = datetime.now(UTC).isoformat()

    _save_auth_payload(auth_path, payload)


def has_codex_cli_auth(path: str | Path | None = None) -> bool:
    return load_codex_credentials(path) is not None
