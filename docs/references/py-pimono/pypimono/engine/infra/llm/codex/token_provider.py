from __future__ import annotations

from pathlib import Path

from pypimono.engine.infra.llm.codex.auth_models import CodexOAuthCredentials
from pypimono.engine.infra.llm.codex.oauth_client import now_ms, refresh_codex_token
from pypimono.engine.infra.llm.codex.token_store import (
    has_codex_cli_auth,
    load_codex_credentials,
    resolve_codex_auth_path,
    save_codex_credentials,
)


def ensure_valid_codex_token(
    path: str | Path | None = None,
    *,
    refresh_skew_sec: int = 60,
) -> CodexOAuthCredentials:
    current = load_codex_credentials(path)
    if current is None:
        auth_path = resolve_codex_auth_path(path)
        raise RuntimeError(
            f"Codex auth not found: {auth_path}. "
            "Run codex login first (or python -m pypimono.engine.infra.llm.codex.auth_cli login)."
        )

    if current.expires_at_ms > now_ms() + refresh_skew_sec * 1000:
        return current

    refreshed = refresh_codex_token(
        current.refresh_token,
        fallback_refresh_token=current.refresh_token,
        fallback_account_id=current.account_id,
        fallback_id_token=current.id_token,
    )
    save_codex_credentials(refreshed, path)
    return refreshed


class CodexTokenProvider:
    def __init__(
        self,
        *,
        auth_path: str | Path | None = None,
        refresh_skew_sec: int = 60,
    ):
        self.auth_path = auth_path
        self.refresh_skew_sec = refresh_skew_sec

    @property
    def resolved_auth_path(self) -> Path:
        return resolve_codex_auth_path(self.auth_path)

    def has_auth(self) -> bool:
        return has_codex_cli_auth(self.auth_path)

    def get(self) -> CodexOAuthCredentials:
        return ensure_valid_codex_token(self.auth_path, refresh_skew_sec=self.refresh_skew_sec)
