from __future__ import annotations

import base64
import hashlib
import json
import secrets
import time
from typing import Any
from urllib import parse, request
from urllib.error import HTTPError, URLError

from pypimono.engine.infra.llm.codex.auth_models import CodexOAuthCredentials

CLIENT_ID = "app_EMoamEEZ73f0CkXaXp7hrann"
AUTHORIZE_URL = "https://auth.openai.com/oauth/authorize"
TOKEN_URL = "https://auth.openai.com/oauth/token"
REDIRECT_URI = "http://localhost:1455/auth/callback"
SCOPE = "openid profile email offline_access"
JWT_CLAIM_PATH = "https://api.openai.com/auth"


def now_ms() -> int:
    return int(time.time() * 1000)


def _b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).decode("ascii").rstrip("=")


def _decode_jwt(token: str) -> dict[str, Any] | None:
    try:
        parts = token.split(".")
        if len(parts) != 3:
            return None
        payload = parts[1]
        padding = "=" * (-len(payload) % 4)
        raw = base64.urlsafe_b64decode(payload + padding)
        decoded = json.loads(raw.decode("utf-8"))
        return decoded if isinstance(decoded, dict) else None
    except Exception:
        return None


def extract_account_id(access_token: str) -> str:
    payload = _decode_jwt(access_token)
    if payload is None:
        raise RuntimeError("Failed to decode access token payload.")

    claim = payload.get(JWT_CLAIM_PATH)
    if isinstance(claim, dict):
        account_id = claim.get("chatgpt_account_id")
        if isinstance(account_id, str) and account_id:
            return account_id

    account_id = payload.get("chatgpt_account_id")
    if isinstance(account_id, str) and account_id:
        return account_id

    raise RuntimeError("Failed to extract chatgpt_account_id from access token.")


def extract_access_expiry_ms(access_token: str) -> int | None:
    payload = _decode_jwt(access_token)
    if payload is None:
        return None
    exp = payload.get("exp")
    if isinstance(exp, (int, float)):
        return int(exp * 1000)
    return None


def parse_authorization_input(value: str) -> tuple[str | None, str | None]:
    text = value.strip()
    if not text:
        return None, None

    try:
        parsed = parse.urlparse(text)
        if parsed.scheme and parsed.netloc:
            q = parse.parse_qs(parsed.query)
            code = q.get("code", [None])[0]
            state = q.get("state", [None])[0]
            return code, state
    except Exception:
        pass

    if "#" in text:
        code, state = text.split("#", 1)
        return code or None, state or None

    if "code=" in text:
        q = parse.parse_qs(text)
        code = q.get("code", [None])[0]
        state = q.get("state", [None])[0]
        return code, state

    return text, None


def generate_pkce() -> tuple[str, str]:
    verifier = _b64url(secrets.token_bytes(32))
    digest = hashlib.sha256(verifier.encode("ascii")).digest()
    challenge = _b64url(digest)
    return verifier, challenge


def create_state() -> str:
    return secrets.token_hex(16)


def _post_form(data: dict[str, str]) -> dict[str, Any]:
    encoded = parse.urlencode(data).encode("utf-8")
    req = request.Request(
        TOKEN_URL,
        data=encoded,
        headers={"Content-Type": "application/x-www-form-urlencoded"},
        method="POST",
    )

    try:
        with request.urlopen(req, timeout=30) as resp:
            body = resp.read().decode("utf-8", errors="replace")
            parsed = json.loads(body)
            if not isinstance(parsed, dict):
                raise RuntimeError("OAuth token response is not a JSON object.")
            return parsed
    except HTTPError as e:
        body = e.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"OAuth token request failed ({e.code}): {body}") from e
    except URLError as e:
        raise RuntimeError(f"OAuth token request failed: {e}") from e


def exchange_authorization_code(
    code: str,
    verifier: str,
    *,
    redirect_uri: str = REDIRECT_URI,
) -> CodexOAuthCredentials:
    response = _post_form(
        {
            "grant_type": "authorization_code",
            "client_id": CLIENT_ID,
            "code": code,
            "code_verifier": verifier,
            "redirect_uri": redirect_uri,
        }
    )

    access = str(response.get("access_token", "")).strip()
    refresh = str(response.get("refresh_token", "")).strip()
    expires_in = response.get("expires_in")
    id_token = response.get("id_token")
    if not access or not refresh or not isinstance(expires_in, (int, float)):
        raise RuntimeError("OAuth token response missing required fields.")

    account_id = extract_account_id(access)
    return CodexOAuthCredentials(
        access_token=access,
        refresh_token=refresh,
        expires_at_ms=now_ms() + int(expires_in * 1000),
        account_id=account_id,
        id_token=str(id_token) if isinstance(id_token, str) and id_token else None,
    )


def refresh_codex_token(
    refresh_token: str,
    *,
    fallback_refresh_token: str | None = None,
    fallback_account_id: str | None = None,
    fallback_id_token: str | None = None,
) -> CodexOAuthCredentials:
    response = _post_form(
        {
            "grant_type": "refresh_token",
            "refresh_token": refresh_token,
            "client_id": CLIENT_ID,
        }
    )

    access = str(response.get("access_token", "")).strip()
    refreshed = str(response.get("refresh_token", "")).strip() or (fallback_refresh_token or refresh_token)
    expires_in = response.get("expires_in")
    id_token = response.get("id_token")

    if not access or not isinstance(expires_in, (int, float)):
        raise RuntimeError("OAuth refresh response missing required fields.")

    try:
        account_id = extract_account_id(access)
    except RuntimeError:
        if fallback_account_id:
            account_id = fallback_account_id
        else:
            raise

    resolved_id_token = str(id_token) if isinstance(id_token, str) and id_token else fallback_id_token

    return CodexOAuthCredentials(
        access_token=access,
        refresh_token=refreshed,
        expires_at_ms=now_ms() + int(expires_in * 1000),
        account_id=account_id,
        id_token=resolved_id_token,
    )
