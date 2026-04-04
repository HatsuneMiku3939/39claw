from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class CodexOAuthCredentials:
    access_token: str
    refresh_token: str
    expires_at_ms: int
    account_id: str
    id_token: str | None = None
