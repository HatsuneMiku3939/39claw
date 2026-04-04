from __future__ import annotations

import json
import os
from contextlib import contextmanager
from pathlib import Path
from typing import Any, Iterator

from pypimono.engine.infra.mcp.models import (
    RemoteMcpAuthState,
    RemoteMcpOAuthCredentials,
    RemoteMcpPendingLogin,
    RemoteMcpServerConfig,
)

try:
    import fcntl
except ImportError:  # pragma: no cover
    fcntl = None


def _load_payload(path: Path) -> dict[str, Any]:
    if not path.exists():
        return {}
    try:
        loaded = json.loads(path.read_text(encoding="utf-8"))
        return loaded if isinstance(loaded, dict) else {}
    except Exception:
        return {}


def _atomic_write_json(path: Path, payload: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temp_path = path.with_name(f".{path.name}.tmp")
    temp_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    os.replace(temp_path, path)
    try:
        os.chmod(path, 0o600)
    except Exception:
        pass


def _serialize_server(server: RemoteMcpServerConfig | None) -> dict[str, Any] | None:
    if server is None:
        return None
    return {
        "name": server.name,
        "mcp_url": server.mcp_url,
        "oauth_resource": server.oauth_resource,
        "redirect_uri": server.redirect_uri,
        "client_name": server.client_name,
        "scopes": list(server.scopes),
    }


def _deserialize_server(payload: Any) -> RemoteMcpServerConfig | None:
    if not isinstance(payload, dict):
        return None
    name = str(payload.get("name", "")).strip()
    mcp_url = str(payload.get("mcp_url", "")).strip()
    oauth_resource = str(payload.get("oauth_resource", "")).strip()
    redirect_uri = str(payload.get("redirect_uri", "")).strip()
    client_name = str(payload.get("client_name", "")).strip() or "py-pimono"
    scopes = payload.get("scopes")
    if not name or not mcp_url or not oauth_resource or not redirect_uri:
        return None
    if isinstance(scopes, list):
        normalized_scopes = tuple(str(item).strip() for item in scopes if str(item).strip())
    else:
        normalized_scopes = ()
    return RemoteMcpServerConfig(
        name=name,
        mcp_url=mcp_url,
        oauth_resource=oauth_resource,
        redirect_uri=redirect_uri,
        client_name=client_name,
        scopes=normalized_scopes,
    )


def _serialize_credentials(credentials: RemoteMcpOAuthCredentials | None) -> dict[str, Any] | None:
    if credentials is None:
        return None
    return {
        "access_token": credentials.access_token,
        "refresh_token": credentials.refresh_token,
        "expires_at_ms": credentials.expires_at_ms,
        "token_type": credentials.token_type,
        "scope": credentials.scope,
    }


def _deserialize_credentials(payload: Any) -> RemoteMcpOAuthCredentials | None:
    if not isinstance(payload, dict):
        return None
    access_token = str(payload.get("access_token", "")).strip()
    refresh_token = str(payload.get("refresh_token", "")).strip()
    expires_at_ms = payload.get("expires_at_ms")
    token_type = str(payload.get("token_type", "")).strip() or "Bearer"
    scope = payload.get("scope")
    if not access_token or not refresh_token or not isinstance(expires_at_ms, int):
        return None
    return RemoteMcpOAuthCredentials(
        access_token=access_token,
        refresh_token=refresh_token,
        expires_at_ms=expires_at_ms,
        token_type=token_type,
        scope=str(scope) if isinstance(scope, str) and scope.strip() else None,
    )


def _serialize_pending_login(pending: RemoteMcpPendingLogin | None) -> dict[str, Any] | None:
    if pending is None:
        return None
    return {
        "state": pending.state,
        "verifier": pending.verifier,
        "created_at_ms": pending.created_at_ms,
        "expires_at_ms": pending.expires_at_ms,
    }


def _deserialize_pending_login(payload: Any) -> RemoteMcpPendingLogin | None:
    if not isinstance(payload, dict):
        return None
    state = str(payload.get("state", "")).strip()
    verifier = str(payload.get("verifier", "")).strip()
    created_at_ms = payload.get("created_at_ms")
    expires_at_ms = payload.get("expires_at_ms")
    if not state or not verifier or not isinstance(created_at_ms, int) or not isinstance(expires_at_ms, int):
        return None
    return RemoteMcpPendingLogin(
        state=state,
        verifier=verifier,
        created_at_ms=created_at_ms,
        expires_at_ms=expires_at_ms,
    )


def load_remote_mcp_auth_state(path: str | Path) -> RemoteMcpAuthState | None:
    auth_path = Path(path).expanduser()
    payload = _load_payload(auth_path)
    if not payload:
        return None

    server = _deserialize_server(payload.get("server"))
    credentials = _deserialize_credentials(payload.get("credentials"))
    pending_login = _deserialize_pending_login(payload.get("pending_login"))

    return RemoteMcpAuthState(
        server=server,
        client_id=str(payload.get("client_id", "")).strip() or None,
        redirect_uri=str(payload.get("redirect_uri", "")).strip() or None,
        issuer=str(payload.get("issuer", "")).strip() or None,
        authorization_endpoint=str(payload.get("authorization_endpoint", "")).strip() or None,
        token_endpoint=str(payload.get("token_endpoint", "")).strip() or None,
        revocation_endpoint=str(payload.get("revocation_endpoint", "")).strip() or None,
        registration_client_uri=str(payload.get("registration_client_uri", "")).strip() or None,
        credentials=credentials,
        pending_login=pending_login,
    )


def save_remote_mcp_auth_state(state: RemoteMcpAuthState, path: str | Path) -> None:
    auth_path = Path(path).expanduser()
    payload = {
        "server": _serialize_server(state.server),
        "client_id": state.client_id,
        "redirect_uri": state.redirect_uri,
        "issuer": state.issuer,
        "authorization_endpoint": state.authorization_endpoint,
        "token_endpoint": state.token_endpoint,
        "revocation_endpoint": state.revocation_endpoint,
        "registration_client_uri": state.registration_client_uri,
        "credentials": _serialize_credentials(state.credentials),
        "pending_login": _serialize_pending_login(state.pending_login),
    }
    _atomic_write_json(auth_path, payload)


def delete_remote_mcp_auth_state(path: str | Path) -> None:
    auth_path = Path(path).expanduser()
    try:
        auth_path.unlink()
    except FileNotFoundError:
        return


def has_remote_mcp_auth(path: str | Path) -> bool:
    state = load_remote_mcp_auth_state(path)
    return bool(state and state.credentials and state.client_id and state.token_endpoint)


@contextmanager
def remote_mcp_auth_lock(path: str | Path) -> Iterator[None]:
    auth_path = Path(path).expanduser()
    auth_path.parent.mkdir(parents=True, exist_ok=True)
    lock_path = auth_path.with_name(f".{auth_path.name}.lock")
    with lock_path.open("a+", encoding="utf-8") as lock_handle:
        if fcntl is not None:
            fcntl.flock(lock_handle.fileno(), fcntl.LOCK_EX)
        try:
            yield
        finally:
            if fcntl is not None:
                fcntl.flock(lock_handle.fileno(), fcntl.LOCK_UN)
