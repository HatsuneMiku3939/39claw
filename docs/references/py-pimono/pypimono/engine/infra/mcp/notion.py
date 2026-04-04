from __future__ import annotations

from copy import deepcopy
from typing import Callable

from pypimono.engine.domain.ports.tool import Tool
from pypimono.engine.infra.mcp.manifest_store import load_remote_mcp_manifest
from pypimono.engine.infra.mcp.models import RemoteMcpServerConfig
from pypimono.engine.infra.mcp.remote_client import RemoteMcpClient
from pypimono.engine.infra.mcp.tool_adapter import RemoteMcpTool
from pypimono.settings import McpNotionSettings


def notion_server_config(settings: McpNotionSettings) -> RemoteMcpServerConfig:
    return RemoteMcpServerConfig(
        name="notion",
        mcp_url=settings.pi_mcp_notion_url,
        oauth_resource=settings.pi_mcp_notion_oauth_resource,
        redirect_uri=settings.pi_mcp_notion_redirect_uri,
        client_name=settings.pi_mcp_notion_client_name,
        scopes=settings.resolved_mcp_notion_scopes,
    )


def build_notion_tools(
    settings: McpNotionSettings,
    *,
    announce: Callable[[str], None] | None = None,
) -> list[Tool]:
    if not settings.pi_mcp_notion_enabled:
        return []

    manifest = load_remote_mcp_manifest(settings.resolved_mcp_notion_manifest_path)
    if manifest is None:
        if announce is not None:
            announce(
                "Notion MCP enabled but no manifest was found. "
                "Run: `uv run -m pypimono mcp notion login`."
            )
        return []

    client = RemoteMcpClient(
        server=notion_server_config(settings),
        auth_path=str(settings.resolved_mcp_notion_auth_path),
    )
    tools: list[Tool] = []
    for spec in manifest.tools:
        tools.append(
            RemoteMcpTool(
                client=client,
                server_name="notion",
                name=spec.name,
                description=spec.description,
                parameters=deepcopy(spec.parameters),
            )
        )
    return tools
