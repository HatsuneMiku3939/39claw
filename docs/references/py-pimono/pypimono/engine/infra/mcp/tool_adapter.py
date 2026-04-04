from __future__ import annotations

import json
from copy import deepcopy
from typing import Any

from typing_extensions import override

from pypimono.engine.domain.ports.tool import OnToolUpdate, Tool, ToolResult
from pypimono.engine.infra.mcp.remote_client import RemoteMcpClient


class RemoteMcpTool(Tool):
    def __init__(
        self,
        *,
        client: RemoteMcpClient,
        server_name: str,
        name: str,
        description: str,
        parameters: dict[str, Any],
    ):
        self._client = client
        self._server_name = server_name
        self.name = name
        self.description = description
        self.parameters = deepcopy(parameters)

    @override
    async def execute(
        self,
        tool_call_id: str,
        args: dict[str, Any],
        *,
        on_update: OnToolUpdate | None = None,
    ) -> ToolResult:
        del on_update

        result = await self._client.call_tool(self.name, args)
        text = self._client.render_tool_result_text(result)
        if result.get("isError") is True:
            raise RuntimeError(text)
        return ToolResult(
            text=text,
            details=_build_result_details(
                server_name=self._server_name,
                tool_call_id=tool_call_id,
                tool_name=self.name,
                result=result,
            ),
        )


def _build_result_details(
    *,
    server_name: str,
    tool_call_id: str,
    tool_name: str,
    result: dict[str, Any],
) -> dict[str, Any]:
    details: dict[str, Any] = {
        "remote_server": server_name,
        "tool_call_id": tool_call_id,
        "tool_name": tool_name,
    }

    if server_name != "notion":
        return details

    title, url = _extract_notion_display_target(result)
    if title:
        details["display_title"] = title
    if url:
        details["display_url"] = url
    return details


def _extract_notion_display_target(result: dict[str, Any]) -> tuple[str | None, str | None]:
    structured = result.get("structuredContent")
    title, url = _extract_title_and_url(structured)
    if title or url:
        return title, url

    content = result.get("content")
    if isinstance(content, list):
        for item in content:
            title, url = _extract_title_and_url(item)
            if title or url:
                return title, url
            if not isinstance(item, dict):
                continue
            if str(item.get("type", "")).strip() != "text":
                continue
            title, url = _extract_title_and_url(_load_json_object(item.get("text")))
            if title or url:
                return title, url

    return None, None


def _extract_title_and_url(value: Any) -> tuple[str | None, str | None]:
    if isinstance(value, str):
        return _extract_title_and_url(_load_json_object(value))

    if isinstance(value, list):
        for item in value:
            title, url = _extract_title_and_url(item)
            if title or url:
                return title, url
        return None, None

    if not isinstance(value, dict):
        return None, None

    results = value.get("results")
    if isinstance(results, list):
        for item in results:
            title, url = _extract_title_and_url(item)
            if title or url:
                return title, url

    title = _normalize_text(value.get("title"))
    url = _normalize_text(value.get("url"))
    if title or url:
        return title, url

    for nested in value.values():
        title, url = _extract_title_and_url(nested)
        if title or url:
            return title, url

    return None, None


def _load_json_object(value: Any) -> Any | None:
    if not isinstance(value, str):
        return None
    text = value.strip()
    if not text or text[0] not in {"{", "["}:
        return None
    try:
        return json.loads(text)
    except json.JSONDecodeError:
        return None


def _normalize_text(value: Any) -> str | None:
    if value is None:
        return None
    text = str(value).strip()
    return text or None
