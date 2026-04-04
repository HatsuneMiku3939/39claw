from __future__ import annotations

import asyncio
import json
import threading
import uuid
from typing import Any
from urllib import request
from urllib.error import HTTPError, URLError

from pypimono import __version__
from pypimono.engine.infra.mcp.http import default_user_agent
from pypimono.engine.infra.mcp.models import MCP_PROTOCOL_VERSION, RemoteMcpServerConfig, RemoteMcpToolSpec
from pypimono.engine.infra.mcp.token_provider import RemoteMcpTokenProvider


class _UnauthorizedError(RuntimeError):
    pass


def _content_type(headers: Any) -> str:
    return str(headers.get("Content-Type", "")).split(";", 1)[0].strip().lower()


def _iter_sse_payloads(response: Any) -> list[dict[str, Any]]:
    payloads: list[dict[str, Any]] = []
    data_lines: list[str] = []

    def flush() -> None:
        if not data_lines:
            return
        raw = "\n".join(data_lines).strip()
        data_lines.clear()
        if not raw:
            return
        parsed = json.loads(raw)
        if isinstance(parsed, dict):
            payloads.append(parsed)

    for raw_line in response:
        line = raw_line.decode("utf-8", errors="replace").rstrip("\r\n")
        if not line:
            flush()
            continue
        if line.startswith("data:"):
            data_lines.append(line[5:].strip())

    flush()
    return payloads


def _tool_spec_from_payload(payload: dict[str, Any]) -> RemoteMcpToolSpec | None:
    name = str(payload.get("name", "")).strip()
    description = str(payload.get("description", "")).strip()
    parameters = payload.get("inputSchema")
    if not name:
        return None
    if not isinstance(parameters, dict):
        parameters = {"type": "object", "properties": {}}
    return RemoteMcpToolSpec(name=name, description=description, parameters=parameters)


class RemoteMcpClient:
    def __init__(
        self,
        *,
        server: RemoteMcpServerConfig,
        auth_path: str,
        refresh_skew_sec: int = 60,
    ):
        self.server = server
        self.protocol_version = MCP_PROTOCOL_VERSION
        self.token_provider = RemoteMcpTokenProvider(
            auth_path=auth_path,
            refresh_skew_sec=refresh_skew_sec,
        )
        self._session_lock = threading.Lock()
        self._session_id: str | None = None
        self._initialized = False

    async def list_tools(self) -> list[RemoteMcpToolSpec]:
        return await asyncio.to_thread(self._list_tools_sync)

    async def call_tool(self, name: str, arguments: dict[str, Any]) -> dict[str, Any]:
        return await asyncio.to_thread(self._call_tool_sync, name, arguments)

    def render_tool_result_text(self, result: dict[str, Any]) -> str:
        content = result.get("content")
        chunks: list[str] = []
        if isinstance(content, list):
            for item in content:
                if not isinstance(item, dict):
                    continue
                item_type = str(item.get("type", "")).strip()
                if item_type == "text":
                    text = str(item.get("text", "")).strip()
                    if text:
                        chunks.append(text)
                        continue
                chunks.append(json.dumps(item, ensure_ascii=False, indent=2))

        structured = result.get("structuredContent")
        if not chunks and structured is not None:
            chunks.append(json.dumps(structured, ensure_ascii=False, indent=2))

        if not chunks:
            chunks.append(json.dumps(result, ensure_ascii=False, indent=2))

        return "\n\n".join(chunks)

    def _build_headers(self, *, access_token: str) -> dict[str, str]:
        headers = {
            "Authorization": f"Bearer {access_token}",
            "Accept": "application/json, text/event-stream",
            "Content-Type": "application/json",
            "Mcp-Protocol-Version": self.protocol_version,
            "User-Agent": default_user_agent(),
        }
        if self._session_id:
            headers["Mcp-Session-Id"] = self._session_id
        return headers

    def _send_jsonrpc(
        self,
        *,
        method: str,
        params: dict[str, Any] | None = None,
        notification: bool = False,
        retry_on_unauthorized: bool = True,
    ) -> dict[str, Any] | None:
        body: dict[str, Any] = {"jsonrpc": "2.0", "method": method}
        request_id: str | None = None
        if params is not None:
            body["params"] = params
        if not notification:
            request_id = f"rpc_{uuid.uuid4().hex[:12]}"
            body["id"] = request_id

        access_token = self.token_provider.get().access_token
        try:
            response_payload = self._send_once(body=body, access_token=access_token)
        except _UnauthorizedError:
            if not retry_on_unauthorized:
                raise
            access_token = self.token_provider.refresh().access_token
            response_payload = self._send_once(body=body, access_token=access_token)

        if notification:
            return response_payload
        if response_payload is None:
            raise RuntimeError(f"MCP method {method} returned an empty response.")
        if "error" in response_payload:
            raise RuntimeError(self._format_rpc_error(response_payload["error"]))
        if request_id is not None and response_payload.get("id") not in {request_id, None}:
            raise RuntimeError(f"MCP method {method} returned a mismatched response id.")
        result = response_payload.get("result")
        if not isinstance(result, dict):
            raise RuntimeError(f"MCP method {method} returned an invalid result payload.")
        return result

    def _send_once(self, *, body: dict[str, Any], access_token: str) -> dict[str, Any] | None:
        encoded = json.dumps(body, ensure_ascii=False).encode("utf-8")
        req = request.Request(
            self.server.mcp_url,
            data=encoded,
            headers=self._build_headers(access_token=access_token),
            method="POST",
        )
        try:
            with request.urlopen(req, timeout=60) as response:
                session_id = response.headers.get("Mcp-Session-Id") or response.headers.get("mcp-session-id")
                if isinstance(session_id, str) and session_id.strip():
                    self._session_id = session_id.strip()
                if response.status in {202, 204}:
                    return None
                content_type = _content_type(response.headers)
                if content_type == "text/event-stream":
                    payloads = _iter_sse_payloads(response)
                    return payloads[-1] if payloads else None
                body_text = response.read().decode("utf-8", errors="replace").strip()
                if not body_text:
                    return None
                payload = json.loads(body_text)
                if not isinstance(payload, dict):
                    raise RuntimeError("MCP response is not a JSON object.")
                return payload
        except HTTPError as exc:
            body_text = exc.read().decode("utf-8", errors="replace")
            if exc.code == 401:
                raise _UnauthorizedError(body_text) from exc
            raise RuntimeError(f"MCP request failed ({exc.code}): {body_text}") from exc
        except URLError as exc:
            raise RuntimeError(f"MCP request failed: {exc}") from exc

    def _ensure_initialized(self) -> None:
        with self._session_lock:
            if self._initialized:
                return

            result = self._send_jsonrpc(
                method="initialize",
                params={
                    "protocolVersion": self.protocol_version,
                    "capabilities": {},
                    "clientInfo": {
                        "name": self.server.client_name,
                        "version": __version__,
                    },
                },
            )
            server_protocol_version = result.get("protocolVersion") if isinstance(result, dict) else None
            if isinstance(server_protocol_version, str) and server_protocol_version.strip():
                self.protocol_version = server_protocol_version.strip()

            self._send_jsonrpc(
                method="notifications/initialized",
                notification=True,
                retry_on_unauthorized=False,
            )
            self._initialized = True

    def _list_tools_sync(self) -> list[RemoteMcpToolSpec]:
        self._ensure_initialized()

        tools: list[RemoteMcpToolSpec] = []
        cursor: str | None = None
        while True:
            params = {"cursor": cursor} if cursor else {}
            result = self._send_jsonrpc(method="tools/list", params=params)
            raw_tools = result.get("tools", []) if isinstance(result, dict) else []
            if not isinstance(raw_tools, list):
                raise RuntimeError("MCP tools/list returned an invalid tools payload.")
            for item in raw_tools:
                if not isinstance(item, dict):
                    continue
                tool = _tool_spec_from_payload(item)
                if tool is not None:
                    tools.append(tool)
            next_cursor = result.get("nextCursor") if isinstance(result, dict) else None
            if not isinstance(next_cursor, str) or not next_cursor.strip():
                break
            cursor = next_cursor.strip()
        return tools

    def _call_tool_sync(self, name: str, arguments: dict[str, Any]) -> dict[str, Any]:
        self._ensure_initialized()
        return self._send_jsonrpc(
            method="tools/call",
            params={"name": name, "arguments": arguments},
        ) or {}

    def _format_rpc_error(self, payload: Any) -> str:
        if not isinstance(payload, dict):
            return f"MCP request failed: {payload}"
        message = str(payload.get("message", "")).strip()
        code = payload.get("code")
        if message and code is not None:
            return f"{message} ({code})"
        return message or "MCP request failed."
