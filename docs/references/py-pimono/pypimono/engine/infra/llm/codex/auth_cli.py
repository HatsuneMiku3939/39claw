from __future__ import annotations

import argparse
import threading
import time
import webbrowser
from datetime import UTC, datetime
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
from urllib import parse

from typing_extensions import override

from pypimono.engine.infra.llm.codex.auth_models import CodexOAuthCredentials
from pypimono.engine.infra.llm.codex.oauth_client import (
    AUTHORIZE_URL,
    CLIENT_ID,
    REDIRECT_URI,
    SCOPE,
    create_state,
    exchange_authorization_code,
    generate_pkce,
    parse_authorization_input,
)
from pypimono.engine.infra.llm.codex.token_provider import ensure_valid_codex_token
from pypimono.engine.infra.llm.codex.token_store import (
    load_codex_credentials,
    resolve_codex_auth_path,
    save_codex_credentials,
)

SUCCESS_HTML = (
    "<!doctype html><html><head><meta charset='utf-8' />"
    "<title>Auth successful</title></head><body>"
    "<p>Authentication successful. Return to your terminal.</p>"
    "</body></html>"
)


class _CallbackServer(HTTPServer):
    expected_state: str
    auth_code: str | None


class _CallbackHandler(BaseHTTPRequestHandler):
    @override
    def log_message(self, format: str, *args: object) -> None:  # noqa: A003
        return

    def do_GET(self) -> None:  # noqa: N802
        parsed = parse.urlparse(self.path)
        if parsed.path != "/auth/callback":
            self.send_response(404)
            self.end_headers()
            self.wfile.write(b"Not found")
            return

        query = parse.parse_qs(parsed.query)
        state = (query.get("state") or [None])[0]
        if state != self.server.expected_state:  # type: ignore[attr-defined]
            self.send_response(400)
            self.end_headers()
            self.wfile.write(b"State mismatch")
            return

        code = (query.get("code") or [None])[0]
        if not code:
            self.send_response(400)
            self.end_headers()
            self.wfile.write(b"Missing authorization code")
            return

        self.server.auth_code = str(code)  # type: ignore[attr-defined]
        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.end_headers()
        self.wfile.write(SUCCESS_HTML.encode("utf-8"))


def _wait_for_server_code(server: _CallbackServer, timeout_sec: int) -> str | None:
    deadline = time.time() + timeout_sec
    while time.time() < deadline:
        if server.auth_code:
            return server.auth_code
        time.sleep(0.1)
    return None


def _format_expiry(ts_ms: int) -> str:
    try:
        return datetime.fromtimestamp(ts_ms / 1000, tz=UTC).isoformat()
    except Exception:
        return str(ts_ms)


def _build_authorize_url(*, originator: str, state: str, challenge: str) -> str:
    return (
        AUTHORIZE_URL
        + "?"
        + parse.urlencode(
            {
                "response_type": "code",
                "client_id": CLIENT_ID,
                "redirect_uri": REDIRECT_URI,
                "scope": SCOPE,
                "code_challenge": challenge,
                "code_challenge_method": "S256",
                "state": state,
                "id_token_add_organizations": "true",
                "codex_cli_simplified_flow": "true",
                "originator": originator,
            }
        )
    )


def login_openai_codex(
    *,
    path: str | Path | None = None,
    originator: str = "pi",
    timeout_sec: int = 600,
    open_browser: bool = True,
) -> CodexOAuthCredentials:
    verifier, challenge = generate_pkce()
    state = create_state()
    url = _build_authorize_url(originator=originator, state=state, challenge=challenge)

    auth_code: str | None = None
    server: _CallbackServer | None = None
    server_thread: threading.Thread | None = None

    try:
        server = _CallbackServer(("127.0.0.1", 1455), _CallbackHandler)
        server.expected_state = state
        server.auth_code = None
        server_thread = threading.Thread(target=server.serve_forever, daemon=True)
        server_thread.start()
    except OSError:
        server = None

    print("\nOpen this URL in your browser:")
    print(url)
    print()

    if open_browser:
        try:
            webbrowser.open(url)
        except Exception:
            pass

    try:
        if server is not None:
            auth_code = _wait_for_server_code(server, timeout_sec)

        if not auth_code:
            raw = input("Paste the authorization code (or full redirect URL): ").strip()
            code, returned_state = parse_authorization_input(raw)
            if returned_state and returned_state != state:
                raise RuntimeError("OAuth state mismatch.")
            auth_code = code

        if not auth_code:
            raise RuntimeError("Missing authorization code.")

        credentials = exchange_authorization_code(auth_code, verifier)
        save_codex_credentials(credentials, path)
        return credentials
    finally:
        if server is not None:
            try:
                server.shutdown()
            except Exception:
                pass
            try:
                server.server_close()
            except Exception:
                pass
        if server_thread is not None and server_thread.is_alive():
            server_thread.join(timeout=1)


def _cmd_status(path: str | Path | None) -> int:
    resolved = resolve_codex_auth_path(path)
    creds = load_codex_credentials(path)
    if creds is None:
        print(f"Codex auth not found: {resolved}")
        return 1
    print(f"Codex auth file: {resolved}")
    print(f"account_id: {creds.account_id}")
    print(f"access_token_expires_at: {_format_expiry(creds.expires_at_ms)}")
    return 0


def _cmd_login(path: str | Path | None, originator: str) -> int:
    creds = login_openai_codex(path=path, originator=originator)
    resolved = resolve_codex_auth_path(path)
    print(f"Saved credentials to {resolved}")
    print(f"account_id: {creds.account_id}")
    print(f"access_token_expires_at: {_format_expiry(creds.expires_at_ms)}")
    return 0


def _cmd_refresh(path: str | Path | None) -> int:
    creds = ensure_valid_codex_token(path, refresh_skew_sec=3600)
    resolved = resolve_codex_auth_path(path)
    print(f"Refreshed credentials in {resolved}")
    print(f"account_id: {creds.account_id}")
    print(f"access_token_expires_at: {_format_expiry(creds.expires_at_ms)}")
    return 0


def _build_cli_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="OpenAI Codex OAuth helper")
    parser.add_argument(
        "--auth-path",
        default=None,
        help="Path to Codex auth.json (default: $CODEX_AUTH_PATH or ~/.codex/auth.json)",
    )

    sub = parser.add_subparsers(dest="command", required=True)
    sub.add_parser("status", help="Show auth availability and token expiry")

    login_parser = sub.add_parser("login", help="Run OAuth login and save tokens")
    login_parser.add_argument("--originator", default="pi", help="OAuth originator parameter (default: pi)")

    sub.add_parser("refresh", help="Refresh token immediately")
    return parser


def main() -> int:
    parser = _build_cli_parser()
    args = parser.parse_args()

    if args.command == "status":
        return _cmd_status(args.auth_path)
    if args.command == "login":
        return _cmd_login(args.auth_path, args.originator)
    if args.command == "refresh":
        return _cmd_refresh(args.auth_path)

    parser.print_help()
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
