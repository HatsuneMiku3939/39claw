from __future__ import annotations

import json
import os
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

from pypimono.engine.infra.mcp.models import RemoteMcpServerConfig, RemoteMcpToolManifest, RemoteMcpToolSpec


def _serialize_server(server: RemoteMcpServerConfig) -> dict[str, Any]:
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
    if not name or not mcp_url or not oauth_resource or not redirect_uri:
        return None
    scopes = payload.get("scopes")
    normalized_scopes = ()
    if isinstance(scopes, list):
        normalized_scopes = tuple(str(item).strip() for item in scopes if str(item).strip())
    return RemoteMcpServerConfig(
        name=name,
        mcp_url=mcp_url,
        oauth_resource=oauth_resource,
        redirect_uri=redirect_uri,
        client_name=client_name,
        scopes=normalized_scopes,
    )


def _serialize_tool(tool: RemoteMcpToolSpec) -> dict[str, Any]:
    return {
        "name": tool.name,
        "description": tool.description,
        "parameters": tool.parameters,
    }


def _deserialize_tool(payload: Any) -> RemoteMcpToolSpec | None:
    if not isinstance(payload, dict):
        return None
    name = str(payload.get("name", "")).strip()
    description = str(payload.get("description", "")).strip()
    parameters = payload.get("parameters")
    if not name:
        return None
    if not isinstance(parameters, dict):
        parameters = {"type": "object", "properties": {}}
    return RemoteMcpToolSpec(name=name, description=description, parameters=parameters)


def load_remote_mcp_manifest(path: str | Path) -> RemoteMcpToolManifest | None:
    manifest_path = Path(path).expanduser()
    if not manifest_path.exists():
        return None
    try:
        payload = json.loads(manifest_path.read_text(encoding="utf-8"))
    except Exception:
        return None
    if not isinstance(payload, dict):
        return None

    server = _deserialize_server(payload.get("server"))
    fetched_at = str(payload.get("fetched_at", "")).strip()
    raw_tools = payload.get("tools")
    if server is None or not fetched_at or not isinstance(raw_tools, list):
        return None

    tools = tuple(tool for item in raw_tools if (tool := _deserialize_tool(item)) is not None)
    return RemoteMcpToolManifest(server=server, tools=tools, fetched_at=fetched_at)


def save_remote_mcp_manifest(
    path: str | Path,
    *,
    server: RemoteMcpServerConfig,
    tools: list[RemoteMcpToolSpec],
) -> RemoteMcpToolManifest:
    manifest_path = Path(path).expanduser()
    manifest_path.parent.mkdir(parents=True, exist_ok=True)
    manifest = RemoteMcpToolManifest(
        server=server,
        tools=tuple(tools),
        fetched_at=datetime.now(UTC).isoformat(),
    )
    payload = {
        "server": _serialize_server(manifest.server),
        "fetched_at": manifest.fetched_at,
        "tools": [_serialize_tool(tool) for tool in manifest.tools],
    }
    temp_path = manifest_path.with_name(f".{manifest_path.name}.tmp")
    temp_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    os.replace(temp_path, manifest_path)
    try:
        os.chmod(manifest_path, 0o600)
    except Exception:
        pass
    return manifest


def delete_remote_mcp_manifest(path: str | Path) -> None:
    manifest_path = Path(path).expanduser()
    try:
        manifest_path.unlink()
    except FileNotFoundError:
        return
