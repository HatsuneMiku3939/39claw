from __future__ import annotations

import asyncio
import json
import platform
import re
import time
from collections.abc import Callable
from typing import Any
from urllib import request
from urllib.error import HTTPError, URLError

from pypimono.engine.infra.llm.codex.responses_models import CodexResponse, CodexTextOutputItem
from pypimono.engine.infra.llm.codex.responses_parser import parse_sse_events
from pypimono.engine.infra.llm.codex.token_provider import CodexTokenProvider

DEFAULT_CODEX_BASE_URL = "https://chatgpt.com/backend-api"
MAX_RETRIES = 3
BASE_DELAY_SEC = 1.0
RETRYABLE_STATUS = {429, 500, 502, 503, 504}
KNOWN_CONTEXT_WINDOWS = {
    "gpt-5.2-codex": 400_000,
    "gpt-5.4": 400_000,
}


def _is_retryable(status: int, error_text: str) -> bool:
    if status in RETRYABLE_STATUS:
        return True
    pattern = re.compile(
        r"rate.?limit|overloaded|service.?unavailable|upstream.?connect|connection.?refused",
        re.I,
    )
    return bool(pattern.search(error_text))


def _friendly_error(status: int, raw: str) -> str:
    message = raw or f"Request failed ({status})"
    try:
        payload = json.loads(raw)
        if isinstance(payload, dict):
            error = payload.get("error")
            if isinstance(error, dict):
                code = str(error.get("code") or error.get("type") or "")
                err_msg = str(error.get("message") or "")
                if code and re.search(
                    r"usage_limit_reached|usage_not_included|rate_limit_exceeded",
                    code,
                    re.I,
                ):
                    return err_msg or "You have hit your ChatGPT usage limit."
                if status == 429:
                    return err_msg or "You have hit your ChatGPT usage limit."
                if err_msg:
                    return err_msg
    except Exception:
        pass
    return message


def _sleep_backoff(attempt: int) -> None:
    delay = BASE_DELAY_SEC * (2**attempt)
    time.sleep(delay)


def resolve_codex_url(base_url: str | None = None) -> str:
    raw = (base_url or DEFAULT_CODEX_BASE_URL).strip()
    normalized = raw.rstrip("/")
    if normalized.endswith("/codex/responses"):
        return normalized
    if normalized.endswith("/codex"):
        return f"{normalized}/responses"
    return f"{normalized}/codex/responses"


def _build_headers(
    *,
    token: str,
    account_id: str,
    originator: str,
    session_id: str | None,
) -> dict[str, str]:
    user_agent = f"pi-mono ({platform.system()} {platform.release()}; {platform.machine()})"
    headers = {
        "Authorization": f"Bearer {token}",
        "chatgpt-account-id": account_id,
        "OpenAI-Beta": "responses=experimental",
        "originator": originator,
        "User-Agent": user_agent,
        "accept": "text/event-stream",
        "content-type": "application/json",
    }
    if session_id:
        headers["session_id"] = session_id
    return headers


def _request_stream_with_retry(
    *,
    url: str,
    body_json: str,
    headers: dict[str, str],
    timeout_sec: int,
) -> Any:
    last_error: Exception | None = None

    for attempt in range(MAX_RETRIES + 1):
        req = request.Request(
            url,
            data=body_json.encode("utf-8"),
            headers=headers,
            method="POST",
        )
        try:
            return request.urlopen(req, timeout=timeout_sec)
        except HTTPError as e:
            raw = e.read().decode("utf-8", errors="replace")
            if attempt < MAX_RETRIES and _is_retryable(e.code, raw):
                _sleep_backoff(attempt)
                continue
            raise RuntimeError(_friendly_error(e.code, raw)) from e
        except URLError as e:
            last_error = e
            if attempt < MAX_RETRIES:
                _sleep_backoff(attempt)
                continue
            raise RuntimeError(f"Codex request failed: {e}") from e

    if last_error:
        raise RuntimeError(f"Codex request failed: {last_error}")
    raise RuntimeError("Codex request failed.")


def _iter_sse_events(response: Any):
    data_lines: list[str] = []

    def flush() -> dict[str, Any] | None:
        if not data_lines:
            return None
        data = "\n".join(data_lines).strip()
        data_lines.clear()
        if not data or data == "[DONE]":
            return None
        try:
            parsed = json.loads(data)
            return parsed if isinstance(parsed, dict) else None
        except json.JSONDecodeError:
            return None

    for raw_line in response:
        line = raw_line.decode("utf-8", errors="replace").rstrip("\r\n")
        if line == "":
            event = flush()
            if event is not None:
                yield event
            continue
        if line.startswith("data:"):
            data_lines.append(line[5:].strip())

    event = flush()
    if event is not None:
        yield event


def _resolve_context_window_limit(model_id: str) -> int | None:
    normalized = model_id.strip().lower()
    for prefix, limit in KNOWN_CONTEXT_WINDOWS.items():
        if normalized.startswith(prefix):
            return limit
    return None


def format_usage_summary(*, model_id: str, response: CodexResponse) -> str:
    usage = response.usage
    if usage is None:
        return "[llm] usage unavailable"

    parts = ["[llm]"]
    if usage.input_tokens is not None:
        parts.append(f"in={usage.input_tokens:,}")
    if usage.output_tokens is not None:
        parts.append(f"out={usage.output_tokens:,}")
    if usage.total_tokens is not None:
        parts.append(f"total={usage.total_tokens:,}")
    if usage.cached_input_tokens is not None:
        parts.append(f"cache_hit={usage.cached_input_tokens:,}")
        uncached = usage.uncached_input_tokens
        if uncached is not None:
            parts.append(f"cache_miss={uncached:,}")
        if usage.input_tokens and usage.input_tokens > 0:
            cache_ratio = usage.cached_input_tokens / usage.input_tokens
            parts.append(f"cache_hit_rate={cache_ratio:.1%}")

    context_limit = _resolve_context_window_limit(model_id)
    if context_limit is not None and usage.input_tokens is not None:
        context_left = max(context_limit - usage.input_tokens, 0)
        parts.append(f"ctx_left~={context_left:,}/{context_limit:,}")

    return " ".join(parts)


class OpenAICodexResponsesClient:
    def __init__(
        self,
        *,
        model_id: str,
        auth_path: str | None = None,
        base_url: str | None = None,
        session_id: str | None = None,
        originator: str = "pi",
        text_verbosity: str = "medium",
        reasoning_effort: str | None = None,
        timeout_sec: int = 180,
        announce: Callable[[str], None] | None = None,
    ):
        self.model_id = model_id
        self.base_url = base_url
        self.session_id = session_id
        self.originator = originator
        self.text_verbosity = text_verbosity
        self.reasoning_effort = reasoning_effort
        self.timeout_sec = timeout_sec
        self.token_provider = CodexTokenProvider(auth_path=auth_path)
        self.announce = announce

    async def complete(
        self,
        *,
        system_prompt: str,
        input_items: list[dict[str, Any]],
        tool_specs: list[dict[str, Any]],
    ) -> CodexResponse:
        return await asyncio.to_thread(
            self._complete_sync,
            system_prompt,
            input_items,
            tool_specs,
        )

    def _complete_sync(
        self,
        system_prompt: str,
        input_items: list[dict[str, Any]],
        tool_specs: list[dict[str, Any]],
    ) -> CodexResponse:
        try:
            credentials = self.token_provider.get()
            request_body: dict[str, Any] = {
                "model": self.model_id,
                "store": False,
                "stream": True,
                "instructions": system_prompt,
                "input": input_items,
                "text": {"verbosity": self.text_verbosity},
                "include": ["reasoning.encrypted_content"],
                "tool_choice": "auto",
                "parallel_tool_calls": True,
            }
            if self.session_id:
                request_body["prompt_cache_key"] = self.session_id

            if tool_specs:
                request_body["tools"] = tool_specs

            reasoning_payload: dict[str, Any] = {}
            if self.reasoning_effort:
                reasoning_payload["effort"] = self.reasoning_effort
            if reasoning_payload:
                request_body["reasoning"] = reasoning_payload

            body_json = json.dumps(request_body, ensure_ascii=False)
            headers = _build_headers(
                token=credentials.access_token,
                account_id=credentials.account_id,
                originator=self.originator,
                session_id=self.session_id,
            )
            url = resolve_codex_url(self.base_url)

            with _request_stream_with_retry(
                url=url,
                body_json=body_json,
                headers=headers,
                timeout_sec=self.timeout_sec,
            ) as response:
                events = list(_iter_sse_events(response))

            parsed = parse_sse_events(events)
            if self.announce is not None:
                self.announce(format_usage_summary(model_id=self.model_id, response=parsed))
            return parsed
        except Exception as e:
            return CodexResponse(
                output_items=[CodexTextOutputItem(text=f"Codex request failed: {e}")],
                status="error",
                error_message=str(e),
            )
