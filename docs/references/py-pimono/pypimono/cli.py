from __future__ import annotations

import argparse
import asyncio
import sys

from pypimono import __version__


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Run the py-pimono local coding agent in the current working directory."
    )
    parser.add_argument("--version", action="version", version=f"pyai {__version__}")
    sub = parser.add_subparsers(dest="command")
    mcp = sub.add_parser("mcp", help="Manage remote MCP integrations")
    mcp_sub = mcp.add_subparsers(dest="mcp_provider", required=True)

    notion = mcp_sub.add_parser("notion", help="Manage Notion hosted MCP")
    notion_sub = notion.add_subparsers(dest="mcp_action", required=True)
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
        help="Seconds to wait for loopback callback before manual paste.",
    )

    notion_sub.add_parser("refresh", help="Refresh the stored access token immediately")
    notion_sub.add_parser("sync", help="Refresh the Notion MCP tool manifest")
    notion_sub.add_parser("logout", help="Delete local Notion MCP auth and manifest files")
    return parser


async def run() -> int:
    from pypimono.container import AppContainer

    container = AppContainer()
    ui_runtime = container.ui_infra.ui_runtime()
    chat_ui = container.ui.chat_ui()
    await ui_runtime.run(chat_ui)
    return 0


def main(argv: list[str] | None = None) -> int:
    args = _build_parser().parse_args(argv)
    if args.command == "mcp":
        from pypimono.engine.infra.mcp.auth_cli import main as mcp_main

        raw_argv = sys.argv[1:] if argv is None else argv
        return mcp_main(raw_argv[1:])
    try:
        return asyncio.run(run())
    except KeyboardInterrupt:
        return 130
