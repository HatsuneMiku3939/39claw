from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any

MCP_PROTOCOL_VERSION = "2025-06-18"


@dataclass(frozen=True)
class RemoteMcpServerConfig:
    name: str
    mcp_url: str
    oauth_resource: str
    redirect_uri: str
    client_name: str = "py-pimono"
    scopes: tuple[str, ...] = ()


@dataclass(frozen=True)
class RemoteMcpOAuthCredentials:
    access_token: str
    refresh_token: str
    expires_at_ms: int
    token_type: str = "Bearer"
    scope: str | None = None


@dataclass(frozen=True)
class RemoteMcpPendingLogin:
    state: str
    verifier: str
    created_at_ms: int
    expires_at_ms: int


@dataclass(frozen=True)
class RemoteMcpAuthState:
    server: RemoteMcpServerConfig | None = None
    client_id: str | None = None
    redirect_uri: str | None = None
    issuer: str | None = None
    authorization_endpoint: str | None = None
    token_endpoint: str | None = None
    revocation_endpoint: str | None = None
    registration_client_uri: str | None = None
    credentials: RemoteMcpOAuthCredentials | None = None
    pending_login: RemoteMcpPendingLogin | None = None


@dataclass(frozen=True)
class StartedRemoteMcpLogin:
    auth_url: str
    server: RemoteMcpServerConfig
    state: str


@dataclass(frozen=True)
class RemoteMcpToolSpec:
    name: str
    description: str
    parameters: dict[str, Any] = field(default_factory=dict)


@dataclass(frozen=True)
class RemoteMcpToolManifest:
    server: RemoteMcpServerConfig
    tools: tuple[RemoteMcpToolSpec, ...]
    fetched_at: str
