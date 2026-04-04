from __future__ import annotations

from dataclasses import replace
from pathlib import Path

from pypimono.engine.infra.mcp.models import RemoteMcpOAuthCredentials
from pypimono.engine.infra.mcp.oauth_client import now_ms, refresh_remote_mcp_token
from pypimono.engine.infra.mcp.token_store import (
    has_remote_mcp_auth,
    load_remote_mcp_auth_state,
    remote_mcp_auth_lock,
    save_remote_mcp_auth_state,
)


def ensure_valid_remote_mcp_token(
    path: str | Path,
    *,
    refresh_skew_sec: int = 60,
    force_refresh: bool = False,
) -> RemoteMcpOAuthCredentials:
    auth_path = Path(path).expanduser()
    with remote_mcp_auth_lock(auth_path):
        current = load_remote_mcp_auth_state(auth_path)
        if current is None or current.credentials is None:
            raise RuntimeError(
                f"Remote MCP auth not found: {auth_path}. "
                "Run `uv run -m pypimono mcp notion login` first."
            )

        if (
            not force_refresh
            and current.credentials.expires_at_ms > now_ms() + refresh_skew_sec * 1000
        ):
            return current.credentials

        refreshed = refresh_remote_mcp_token(current)
        save_remote_mcp_auth_state(replace(current, credentials=refreshed), auth_path)
        return refreshed


class RemoteMcpTokenProvider:
    def __init__(
        self,
        *,
        auth_path: str | Path,
        refresh_skew_sec: int = 60,
    ):
        self.auth_path = Path(auth_path).expanduser()
        self.refresh_skew_sec = refresh_skew_sec

    def has_auth(self) -> bool:
        return has_remote_mcp_auth(self.auth_path)

    def get(self) -> RemoteMcpOAuthCredentials:
        return ensure_valid_remote_mcp_token(
            self.auth_path,
            refresh_skew_sec=self.refresh_skew_sec,
        )

    def refresh(self) -> RemoteMcpOAuthCredentials:
        return ensure_valid_remote_mcp_token(
            self.auth_path,
            refresh_skew_sec=self.refresh_skew_sec,
            force_refresh=True,
        )
