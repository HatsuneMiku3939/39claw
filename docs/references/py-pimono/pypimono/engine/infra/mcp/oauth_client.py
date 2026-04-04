from __future__ import annotations

import base64
import hashlib
import json
import secrets
import time
from dataclasses import replace
from typing import Any
from urllib import parse, request
from urllib.error import HTTPError, URLError

from pypimono.engine.infra.mcp.http import default_user_agent
from pypimono.engine.infra.mcp.models import (
    RemoteMcpAuthState,
    RemoteMcpOAuthCredentials,
    RemoteMcpPendingLogin,
    RemoteMcpServerConfig,
    StartedRemoteMcpLogin,
)
from pypimono.engine.infra.mcp.token_store import load_remote_mcp_auth_state, save_remote_mcp_auth_state


def now_ms() -> int:
    return int(time.time() * 1000)


def _b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).decode("ascii").rstrip("=")


def generate_pkce() -> tuple[str, str]:
    verifier = _b64url(secrets.token_bytes(32))
    digest = hashlib.sha256(verifier.encode("ascii")).digest()
    challenge = _b64url(digest)
    return verifier, challenge


def create_state() -> str:
    return secrets.token_hex(16)


def _normalize_url(url: str) -> str:
    parsed = parse.urlsplit(url.strip())
    normalized = parsed._replace(query="", fragment="")
    return parse.urlunsplit(normalized).rstrip("/")


def _well_known_url(base_url: str, name: str) -> str:
    parsed = parse.urlsplit(_normalize_url(base_url))
    path = f"/.well-known/{name}"
    return parse.urlunsplit((parsed.scheme, parsed.netloc, path, "", ""))


def _validate_redirect_uri(redirect_uri: str) -> None:
    parsed = parse.urlsplit(redirect_uri)
    hostname = (parsed.hostname or "").lower()
    if parsed.scheme == "https":
        return
    if parsed.scheme == "http" and hostname in {"127.0.0.1", "localhost", "::1"}:
        return
    raise RuntimeError("Remote MCP OAuth redirect_uri must use HTTPS or loopback HTTP.")


def parse_authorization_input(value: str) -> tuple[str | None, str | None]:
    text = value.strip()
    if not text:
        return None, None

    try:
        parsed = parse.urlparse(text)
        if parsed.scheme and parsed.netloc:
            query = parse.parse_qs(parsed.query)
            code = query.get("code", [None])[0]
            state = query.get("state", [None])[0]
            return code, state
    except Exception:
        pass

    query = parse.parse_qs(text.lstrip("?"))
    code = query.get("code", [None])[0]
    state = query.get("state", [None])[0]
    return code, state


def _load_json_response(response: Any) -> dict[str, Any]:
    body = response.read().decode("utf-8", errors="replace")
    parsed = json.loads(body)
    if not isinstance(parsed, dict):
        raise RuntimeError("OAuth response is not a JSON object.")
    return parsed


def _request_json(
    url: str,
    *,
    method: str = "GET",
    headers: dict[str, str] | None = None,
    data: bytes | None = None,
) -> dict[str, Any]:
    request_headers = {
        "User-Agent": default_user_agent(),
        **(headers or {}),
    }
    req = request.Request(url, headers=request_headers, data=data, method=method)
    try:
        with request.urlopen(req, timeout=30) as response:
            return _load_json_response(response)
    except HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"Remote MCP OAuth request failed ({exc.code}): {body}") from exc
    except URLError as exc:
        raise RuntimeError(f"Remote MCP OAuth request failed: {exc}") from exc


def discover_protected_resource(server: RemoteMcpServerConfig) -> dict[str, Any]:
    metadata_url = _well_known_url(server.oauth_resource, "oauth-protected-resource")
    return _request_json(metadata_url, headers={"Accept": "application/json"})


def discover_authorization_server(issuer: str) -> dict[str, Any]:
    metadata_url = _well_known_url(issuer, "oauth-authorization-server")
    return _request_json(metadata_url, headers={"Accept": "application/json"})


def register_public_client(server: RemoteMcpServerConfig, oauth_metadata: dict[str, Any]) -> dict[str, Any]:
    registration_endpoint = str(oauth_metadata.get("registration_endpoint", "")).strip()
    if not registration_endpoint:
        raise RuntimeError("Authorization server metadata is missing registration_endpoint.")

    payload = {
        "client_name": server.client_name,
        "redirect_uris": [server.redirect_uri],
        "grant_types": ["authorization_code", "refresh_token"],
        "response_types": ["code"],
        "token_endpoint_auth_method": "none",
    }
    return _request_json(
        registration_endpoint,
        method="POST",
        headers={
            "Accept": "application/json",
            "Content-Type": "application/json",
        },
        data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
    )


def _build_credentials(
    payload: dict[str, Any],
    fallback_refresh_token: str | None = None,
) -> RemoteMcpOAuthCredentials:
    access_token = str(payload.get("access_token", "")).strip()
    refresh_token = str(payload.get("refresh_token", "")).strip() or (fallback_refresh_token or "")
    expires_in = payload.get("expires_in")
    token_type = str(payload.get("token_type", "")).strip() or "Bearer"
    scope = payload.get("scope")
    if not access_token or not refresh_token or not isinstance(expires_in, (int, float)):
        raise RuntimeError("OAuth token response missing required fields.")
    return RemoteMcpOAuthCredentials(
        access_token=access_token,
        refresh_token=refresh_token,
        expires_at_ms=now_ms() + int(expires_in * 1000),
        token_type=token_type,
        scope=str(scope) if isinstance(scope, str) and scope.strip() else None,
    )


def _post_form(url: str, data: dict[str, str]) -> dict[str, Any]:
    encoded = parse.urlencode(data).encode("utf-8")
    return _request_json(
        url,
        method="POST",
        headers={
            "Accept": "application/json",
            "Content-Type": "application/x-www-form-urlencoded",
        },
        data=encoded,
    )


def exchange_authorization_code(
    *,
    auth_state: RemoteMcpAuthState,
    code: str,
) -> RemoteMcpOAuthCredentials:
    if not auth_state.client_id or not auth_state.redirect_uri or not auth_state.token_endpoint:
        raise RuntimeError("Remote MCP OAuth state is incomplete.")
    if auth_state.pending_login is None:
        raise RuntimeError("Remote MCP OAuth login is not pending.")

    payload = _post_form(
        auth_state.token_endpoint,
        {
            "grant_type": "authorization_code",
            "client_id": auth_state.client_id,
            "code": code,
            "code_verifier": auth_state.pending_login.verifier,
            "redirect_uri": auth_state.redirect_uri,
            "resource": auth_state.server.oauth_resource if auth_state.server else "",
        },
    )
    return _build_credentials(payload)


def refresh_remote_mcp_token(auth_state: RemoteMcpAuthState) -> RemoteMcpOAuthCredentials:
    if not auth_state.client_id or not auth_state.token_endpoint or auth_state.credentials is None:
        raise RuntimeError("Remote MCP OAuth state is incomplete.")

    payload = _post_form(
        auth_state.token_endpoint,
        {
            "grant_type": "refresh_token",
            "client_id": auth_state.client_id,
            "refresh_token": auth_state.credentials.refresh_token,
            "resource": auth_state.server.oauth_resource if auth_state.server else "",
        },
    )
    return _build_credentials(payload, fallback_refresh_token=auth_state.credentials.refresh_token)


class RemoteMcpOAuthManager:
    def __init__(self, *, auth_path: str):
        self.auth_path = auth_path

    def start_login(self, server: RemoteMcpServerConfig) -> StartedRemoteMcpLogin:
        _validate_redirect_uri(server.redirect_uri)
        resource_metadata = discover_protected_resource(server)
        authorization_servers = resource_metadata.get("authorization_servers")
        if not isinstance(authorization_servers, list) or not authorization_servers:
            raise RuntimeError("Protected resource metadata is missing authorization_servers.")
        issuer = str(authorization_servers[0]).strip()
        if not issuer:
            raise RuntimeError("Authorization server issuer is missing.")

        oauth_metadata = discover_authorization_server(issuer)
        registration = register_public_client(server, oauth_metadata)
        client_id = str(registration.get("client_id", "")).strip()
        if not client_id:
            raise RuntimeError("Dynamic client registration did not return client_id.")

        verifier, challenge = generate_pkce()
        state = create_state()

        query = {
            "response_type": "code",
            "client_id": client_id,
            "redirect_uri": server.redirect_uri,
            "code_challenge": challenge,
            "code_challenge_method": "S256",
            "state": state,
            "prompt": "consent",
            "resource": server.oauth_resource,
        }
        if server.scopes:
            query["scope"] = " ".join(server.scopes)

        authorization_endpoint = str(oauth_metadata.get("authorization_endpoint", "")).strip()
        token_endpoint = str(oauth_metadata.get("token_endpoint", "")).strip()
        if not authorization_endpoint or not token_endpoint:
            raise RuntimeError("Authorization server metadata is missing required endpoints.")

        existing = load_remote_mcp_auth_state(self.auth_path)
        registration_client_uri = str(registration.get("registration_client_uri", "")).strip() or None
        if registration_client_uri is not None and not registration_client_uri.startswith("http"):
            registration_client_uri = parse.urljoin(
                issuer.rstrip("/") + "/",
                registration_client_uri.lstrip("/"),
            )

        next_state = RemoteMcpAuthState(
            server=server,
            client_id=client_id,
            redirect_uri=server.redirect_uri,
            issuer=issuer,
            authorization_endpoint=authorization_endpoint,
            token_endpoint=token_endpoint,
            revocation_endpoint=str(oauth_metadata.get("revocation_endpoint", "")).strip() or None,
            registration_client_uri=registration_client_uri,
            credentials=existing.credentials if existing is not None else None,
            pending_login=RemoteMcpPendingLogin(
                state=state,
                verifier=verifier,
                created_at_ms=now_ms(),
                expires_at_ms=now_ms() + 10 * 60 * 1000,
            ),
        )
        save_remote_mcp_auth_state(next_state, self.auth_path)

        auth_url = authorization_endpoint + "?" + parse.urlencode(query)
        return StartedRemoteMcpLogin(auth_url=auth_url, server=server, state=state)

    def complete_login(self, callback_url: str) -> RemoteMcpOAuthCredentials:
        auth_state = load_remote_mcp_auth_state(self.auth_path)
        if auth_state is None or auth_state.pending_login is None:
            raise RuntimeError("Remote MCP OAuth login was not started.")
        if auth_state.pending_login.expires_at_ms <= now_ms():
            raise RuntimeError("Remote MCP OAuth login expired. Start login again.")

        code, returned_state = parse_authorization_input(callback_url)
        if not code or not returned_state:
            raise RuntimeError(
                "Paste the full redirect URL from the same login run so code and state can be verified."
            )
        if returned_state != auth_state.pending_login.state:
            raise RuntimeError(
                "Remote MCP OAuth state mismatch. "
                "This redirect URL belongs to a different login attempt."
            )

        credentials = exchange_authorization_code(auth_state=auth_state, code=code)
        save_remote_mcp_auth_state(
            replace(auth_state, credentials=credentials, pending_login=None),
            self.auth_path,
        )
        return credentials
