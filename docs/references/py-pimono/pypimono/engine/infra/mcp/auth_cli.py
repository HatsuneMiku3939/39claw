from __future__ import annotations

import argparse
import asyncio
import threading
import time
import webbrowser
from datetime import UTC, datetime
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib import parse

from typing_extensions import override

from pypimono.engine.infra.mcp.manifest_store import (
    delete_remote_mcp_manifest,
    load_remote_mcp_manifest,
    save_remote_mcp_manifest,
)
from pypimono.engine.infra.mcp.notion import notion_server_config
from pypimono.engine.infra.mcp.oauth_client import RemoteMcpOAuthManager
from pypimono.engine.infra.mcp.remote_client import RemoteMcpClient
from pypimono.engine.infra.mcp.token_provider import ensure_valid_remote_mcp_token
from pypimono.engine.infra.mcp.token_store import (
    delete_remote_mcp_auth_state,
    load_remote_mcp_auth_state,
)
from pypimono.settings import McpNotionSettings, get_mcp_notion_settings

SUCCESS_HTML = (
    "<!doctype html><html><head><meta charset='utf-8' />"
    "<title>Auth successful</title></head><body>"
    "<p>Authentication successful. Return to your terminal.</p>"
    "</body></html>"
)


class _CallbackServer(HTTPServer):
    callback_url: str | None
    expected_path: str
    expected_state: str
    base_redirect_uri: str


class _CallbackHandler(BaseHTTPRequestHandler):
    @override
    def log_message(self, format: str, *args: object) -> None:  # noqa: A003
        return

    def do_GET(self) -> None:  # noqa: N802
        parsed = parse.urlparse(self.path)
        if parsed.path != self.server.expected_path:  # type: ignore[attr-defined]
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

        base_redirect_uri = self.server.base_redirect_uri.rstrip("/")  # type: ignore[attr-defined]
        self.server.callback_url = f"{base_redirect_uri}{self.path}"  # type: ignore[attr-defined]
        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.end_headers()
        self.wfile.write(SUCCESS_HTML.encode("utf-8"))


def _format_expiry(ts_ms: int) -> str:
    try:
        return datetime.fromtimestamp(ts_ms / 1000, tz=UTC).isoformat()
    except Exception:
        return str(ts_ms)


def _try_start_callback_server(
    redirect_uri: str,
    expected_state: str,
) -> tuple[_CallbackServer | None, threading.Thread | None]:
    parsed = parse.urlsplit(redirect_uri)
    hostname = (parsed.hostname or "").lower()
    if parsed.scheme != "http" or hostname not in {"127.0.0.1", "localhost", "::1"}:
        return None, None

    path = parsed.path or "/"
    port = parsed.port or 80
    bind_host = parsed.hostname or "127.0.0.1"

    try:
        server = _CallbackServer((bind_host, port), _CallbackHandler)
    except OSError:
        return None, None

    server.callback_url = None
    server.expected_path = path
    server.expected_state = expected_state
    server.base_redirect_uri = parse.urlunsplit((parsed.scheme, parsed.netloc, "", "", ""))

    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    return server, thread


def _wait_for_callback(server: _CallbackServer, timeout_sec: int) -> str | None:
    deadline = time.time() + timeout_sec
    while time.time() < deadline:
        if server.callback_url:
            return server.callback_url
        time.sleep(0.1)
    return None


def _sync_manifest(settings: McpNotionSettings) -> int:
    server = notion_server_config(settings)
    client = RemoteMcpClient(
        server=server,
        auth_path=str(settings.resolved_mcp_notion_auth_path),
    )
    tools = asyncio.run(client.list_tools())
    manifest = save_remote_mcp_manifest(
        settings.resolved_mcp_notion_manifest_path,
        server=server,
        tools=tools,
    )
    print(f"Saved Notion MCP manifest to {settings.resolved_mcp_notion_manifest_path}")
    print(f"tool_count: {len(manifest.tools)}")
    return 0


def _cmd_status(settings: McpNotionSettings) -> int:
    auth_path = settings.resolved_mcp_notion_auth_path
    manifest_path = settings.resolved_mcp_notion_manifest_path
    auth_state = load_remote_mcp_auth_state(auth_path)
    manifest = load_remote_mcp_manifest(manifest_path)

    print(f"enabled: {settings.pi_mcp_notion_enabled}")
    print(f"auth_file: {auth_path}")
    print(f"manifest_file: {manifest_path}")
    if auth_state is None or auth_state.credentials is None:
        print("auth_status: missing")
    else:
        print("auth_status: ready")
        print(f"access_token_expires_at: {_format_expiry(auth_state.credentials.expires_at_ms)}")
    if auth_state is not None and auth_state.pending_login is not None:
        print(f"pending_login_expires_at: {_format_expiry(auth_state.pending_login.expires_at_ms)}")
    print(f"manifest_tools: {len(manifest.tools) if manifest is not None else 0}")
    if not settings.pi_mcp_notion_enabled:
        print("warning: Notion MCP is configured locally but disabled in runtime settings.")
        print(
            "hint: set `PI_MCP_NOTION_ENABLED=true` "
            "(or `pi_mcp_notion_enabled=true`) and restart py-pimono."
        )
    return 0


def _cmd_login(settings: McpNotionSettings, *, open_browser: bool, timeout_sec: int) -> int:
    server = notion_server_config(settings)
    manager = RemoteMcpOAuthManager(auth_path=str(settings.resolved_mcp_notion_auth_path))
    started = manager.start_login(server)

    callback_server, callback_thread = _try_start_callback_server(server.redirect_uri, started.state)

    print("\nOpen this URL in your browser:")
    print(started.auth_url)
    print()

    if open_browser:
        try:
            webbrowser.open(started.auth_url)
        except Exception:
            pass

    try:
        callback_url: str | None = None
        if callback_server is not None:
            callback_url = _wait_for_callback(callback_server, timeout_sec)
        if not callback_url:
            callback_url = input("Paste the full redirect URL: ").strip()

        try:
            credentials = manager.complete_login(callback_url)
        except RuntimeError as exc:
            print(f"Notion MCP login failed: {exc}")
            print(
                "Use the redirect URL returned from this exact "
                "`uv run -m pypimono mcp notion login` run. "
                "If you started login more than once, rerun the command and approve only the newest URL."
            )
            return 1
        print(f"Saved Notion MCP auth to {settings.resolved_mcp_notion_auth_path}")
        print(f"access_token_expires_at: {_format_expiry(credentials.expires_at_ms)}")
    finally:
        if callback_server is not None:
            try:
                callback_server.shutdown()
            except Exception:
                pass
            try:
                callback_server.server_close()
            except Exception:
                pass
        if callback_thread is not None and callback_thread.is_alive():
            callback_thread.join(timeout=1)

    try:
        return _sync_manifest(settings)
    except Exception as exc:
        print(f"Manifest sync failed after login: {exc}")
        print("Run `uv run -m pypimono mcp notion sync` after confirming auth.")
        return 0


def _cmd_refresh(settings: McpNotionSettings) -> int:
    creds = ensure_valid_remote_mcp_token(
        settings.resolved_mcp_notion_auth_path,
        refresh_skew_sec=3600,
        force_refresh=True,
    )
    print(f"Refreshed Notion MCP auth in {settings.resolved_mcp_notion_auth_path}")
    print(f"access_token_expires_at: {_format_expiry(creds.expires_at_ms)}")
    return 0


def _cmd_sync(settings: McpNotionSettings) -> int:
    return _sync_manifest(settings)


def _cmd_logout(settings: McpNotionSettings) -> int:
    delete_remote_mcp_auth_state(settings.resolved_mcp_notion_auth_path)
    delete_remote_mcp_manifest(settings.resolved_mcp_notion_manifest_path)
    print(f"Removed Notion MCP auth: {settings.resolved_mcp_notion_auth_path}")
    print(f"Removed Notion MCP manifest: {settings.resolved_mcp_notion_manifest_path}")
    return 0


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Remote MCP OAuth helper")
    sub = parser.add_subparsers(dest="server", required=True)

    notion = sub.add_parser("notion", help="Manage Notion hosted MCP")
    notion_sub = notion.add_subparsers(dest="command", required=True)

    notion_sub.add_parser("status", help="Show Notion MCP auth and manifest status")

    login = notion_sub.add_parser("login", help="Run OAuth login and sync the manifest")
    login.add_argument(
        "--open-browser",
        action=argparse.BooleanOptionalAction,
        default=None,
        help="Open the authorization URL in the local browser.",
    )
    login.add_argument(
        "--timeout-sec",
        type=int,
        default=600,
        help="Seconds to wait for loopback callback before falling back to manual paste.",
    )

    notion_sub.add_parser("refresh", help="Refresh the stored access token immediately")
    notion_sub.add_parser("sync", help="Refresh the Notion MCP tool manifest")
    notion_sub.add_parser("logout", help="Delete local Notion MCP auth and manifest files")
    return parser


def main(argv: list[str] | None = None) -> int:
    parser = _build_parser()
    args = parser.parse_args(argv)
    settings = get_mcp_notion_settings()

    if args.server != "notion":
        parser.print_help()
        return 1

    if args.command == "status":
        return _cmd_status(settings)
    if args.command == "login":
        open_browser = settings.pi_mcp_notion_open_browser if args.open_browser is None else args.open_browser
        return _cmd_login(settings, open_browser=open_browser, timeout_sec=args.timeout_sec)
    if args.command == "refresh":
        return _cmd_refresh(settings)
    if args.command == "sync":
        return _cmd_sync(settings)
    if args.command == "logout":
        return _cmd_logout(settings)

    parser.print_help()
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
