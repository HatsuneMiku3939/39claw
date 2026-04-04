from __future__ import annotations

import json
import uuid
from collections.abc import Iterable
from typing import Any

from pypimono.engine.infra.llm.codex.responses_models import (
    CodexReasoningOutputItem,
    CodexResponse,
    CodexTextOutputItem,
    CodexToolCallOutputItem,
    CodexUsage,
)


def _parse_json_dict(raw: str) -> dict[str, Any]:
    try:
        parsed = json.loads(raw)
        if isinstance(parsed, dict):
            return parsed
        return {"value": parsed}
    except Exception:
        return {}


def _extract_message_text(item: dict[str, Any]) -> str:
    content = item.get("content")
    if not isinstance(content, list):
        return ""

    parts: list[str] = []
    for block in content:
        if not isinstance(block, dict):
            continue
        block_type = block.get("type")
        if block_type == "output_text":
            text = block.get("text")
            if isinstance(text, str):
                parts.append(text)
        elif block_type == "refusal":
            refusal = block.get("refusal")
            if isinstance(refusal, str):
                parts.append(refusal)
    return "".join(parts)


def _find_last_builder(builders: list[dict[str, Any]], kind: str) -> dict[str, Any] | None:
    for builder in reversed(builders):
        if builder.get("kind") == kind:
            return builder
    return None


def _find_builder_by_item_id(
    builders: list[dict[str, Any]],
    kind: str,
    item_id: str | None,
) -> dict[str, Any] | None:
    if not item_id:
        return None
    for builder in reversed(builders):
        if builder.get("kind") != kind:
            continue
        if builder.get("item_id") == item_id:
            return builder
    return None


def _extract_reasoning_summary(item: dict[str, Any]) -> str:
    summary = item.get("summary")
    if not isinstance(summary, list):
        return ""

    parts: list[str] = []
    for part in summary:
        if not isinstance(part, dict):
            continue
        text = part.get("text")
        if isinstance(text, str) and text:
            parts.append(text)
    return "\n\n".join(parts).strip()


def _summary_index(value: object) -> int | None:
    if isinstance(value, int):
        return value
    if isinstance(value, str):
        stripped = value.strip()
        if stripped.isdigit():
            return int(stripped)
    return None


def _set_reasoning_summary_text(
    builder: dict[str, Any],
    *,
    text: str,
    summary_index: int | None,
    append: bool,
) -> None:
    if summary_index is None:
        if append:
            builder["summary_text"] = str(builder.get("summary_text", "")) + text
        else:
            builder["summary_text"] = text
        return

    by_index = builder.get("summary_by_index")
    if not isinstance(by_index, dict):
        by_index = {}
        builder["summary_by_index"] = by_index

    current = by_index.get(summary_index, "")
    if append:
        by_index[summary_index] = str(current) + text
    else:
        by_index[summary_index] = text


def _coerce_int(value: object) -> int | None:
    if isinstance(value, bool):
        return None
    if isinstance(value, int):
        return value
    if isinstance(value, float) and value.is_integer():
        return int(value)
    if isinstance(value, str):
        stripped = value.strip()
        if stripped.isdigit():
            return int(stripped)
    return None


def _extract_usage(response: dict[str, Any]) -> CodexUsage | None:
    usage = response.get("usage")
    if not isinstance(usage, dict):
        return None

    cached_input_tokens = None
    for key in ("input_tokens_details", "prompt_tokens_details"):
        details = usage.get(key)
        if not isinstance(details, dict):
            continue
        cached_input_tokens = _coerce_int(details.get("cached_tokens"))
        if cached_input_tokens is not None:
            break

    parsed = CodexUsage(
        input_tokens=_coerce_int(usage.get("input_tokens")),
        output_tokens=_coerce_int(usage.get("output_tokens")),
        total_tokens=_coerce_int(usage.get("total_tokens")),
        cached_input_tokens=cached_input_tokens,
    )

    if (
        parsed.input_tokens is None
        and parsed.output_tokens is None
        and parsed.total_tokens is None
        and parsed.cached_input_tokens is None
    ):
        return None
    return parsed


def parse_sse_events(events: Iterable[dict[str, Any]]) -> CodexResponse:
    builders: list[dict[str, Any]] = []
    current_builder: dict[str, Any] | None = None
    status = "completed"
    usage: CodexUsage | None = None

    for event in events:
        event_type = event.get("type")
        if not isinstance(event_type, str):
            continue

        if event_type == "error":
            code = event.get("code")
            message = event.get("message")
            raise RuntimeError(f"Codex error: {message or code or event}")

        if event_type == "response.failed":
            response = event.get("response")
            if isinstance(response, dict):
                error = response.get("error")
                if isinstance(error, dict):
                    err_msg = error.get("message")
                    if isinstance(err_msg, str) and err_msg:
                        raise RuntimeError(err_msg)
            raise RuntimeError("Codex response failed.")

        if event_type in {"response.done", "response.completed"}:
            response = event.get("response")
            if isinstance(response, dict):
                response_status = response.get("status")
                if isinstance(response_status, str) and response_status:
                    status = response_status
                usage = _extract_usage(response) or usage
            continue

        if event_type == "response.output_item.added":
            item = event.get("item")
            if not isinstance(item, dict):
                continue
            item_type = item.get("type")
            if item_type == "message":
                current_builder = {"kind": "text", "text": ""}
                builders.append(current_builder)
            elif item_type == "function_call":
                raw_args = item.get("arguments")
                args_json = raw_args if isinstance(raw_args, str) else ""
                current_builder = {
                    "kind": "tool",
                    "name": item.get("name", ""),
                    "call_id": item.get("call_id", ""),
                    "item_id": item.get("id", ""),
                    "args_json": args_json,
                    "arguments": _parse_json_dict(args_json) if args_json else {},
                }
                builders.append(current_builder)
            elif item_type == "reasoning":
                item_id = item.get("id")
                current_builder = {
                    "kind": "reasoning",
                    "item_id": item_id if isinstance(item_id, str) else "",
                    "summary_text": _extract_reasoning_summary(item),
                    "summary_by_index": {},
                }
                builders.append(current_builder)
            else:
                current_builder = None
            continue

        if event_type in {"response.output_text.delta", "response.refusal.delta"}:
            if current_builder and current_builder.get("kind") == "text":
                delta = event.get("delta")
                if isinstance(delta, str):
                    current_builder["text"] = str(current_builder.get("text", "")) + delta
            continue

        if event_type == "response.function_call_arguments.delta":
            if current_builder and current_builder.get("kind") == "tool":
                delta = event.get("delta")
                if isinstance(delta, str):
                    raw = str(current_builder.get("args_json", "")) + delta
                    current_builder["args_json"] = raw
                    current_builder["arguments"] = _parse_json_dict(raw)
            continue

        if event_type == "response.function_call_arguments.done":
            if current_builder and current_builder.get("kind") == "tool":
                raw = event.get("arguments")
                if isinstance(raw, str):
                    current_builder["args_json"] = raw
                    current_builder["arguments"] = _parse_json_dict(raw)
            continue

        if event_type == "response.reasoning_summary_text.delta":
            item_id = event.get("item_id")
            target = _find_builder_by_item_id(
                builders,
                "reasoning",
                item_id if isinstance(item_id, str) else None,
            ) or _find_last_builder(builders, "reasoning")
            if target is not None:
                delta = event.get("delta")
                if isinstance(delta, str):
                    _set_reasoning_summary_text(
                        target,
                        text=delta,
                        summary_index=_summary_index(event.get("summary_index")),
                        append=True,
                    )
            continue

        if event_type == "response.reasoning_summary_text.done":
            item_id = event.get("item_id")
            target = _find_builder_by_item_id(
                builders,
                "reasoning",
                item_id if isinstance(item_id, str) else None,
            ) or _find_last_builder(builders, "reasoning")
            if target is not None:
                final_text = event.get("text")
                if isinstance(final_text, str):
                    _set_reasoning_summary_text(
                        target,
                        text=final_text,
                        summary_index=_summary_index(event.get("summary_index")),
                        append=False,
                    )
            continue

        if event_type == "response.reasoning_summary_part.done":
            item_id = event.get("item_id")
            target = _find_builder_by_item_id(
                builders,
                "reasoning",
                item_id if isinstance(item_id, str) else None,
            ) or _find_last_builder(builders, "reasoning")
            if target is not None:
                part = event.get("part")
                if isinstance(part, dict):
                    text = part.get("text")
                    if isinstance(text, str):
                        _set_reasoning_summary_text(
                            target,
                            text=text,
                            summary_index=_summary_index(event.get("summary_index")),
                            append=False,
                        )
            continue

        if event_type == "response.output_item.done":
            item = event.get("item")
            if not isinstance(item, dict):
                continue
            item_type = item.get("type")

            if item_type == "message":
                target = (
                    current_builder
                    if current_builder and current_builder.get("kind") == "text"
                    else _find_last_builder(builders, "text")
                )
                if target is not None:
                    final_text = _extract_message_text(item)
                    if final_text:
                        target["text"] = final_text
                current_builder = None
                continue

            if item_type == "function_call":
                target = (
                    current_builder
                    if current_builder and current_builder.get("kind") == "tool"
                    else _find_last_builder(builders, "tool")
                )
                if target is not None:
                    name = item.get("name")
                    call_id = item.get("call_id")
                    item_id = item.get("id")
                    args = item.get("arguments")
                    if isinstance(name, str):
                        target["name"] = name
                    if isinstance(call_id, str):
                        target["call_id"] = call_id
                    if isinstance(item_id, str):
                        target["item_id"] = item_id
                    if isinstance(args, str):
                        target["args_json"] = args
                        target["arguments"] = _parse_json_dict(args)
                current_builder = None
                continue

            if item_type == "reasoning":
                item_id = item.get("id")
                target = (
                    current_builder
                    if current_builder and current_builder.get("kind") == "reasoning"
                    else _find_builder_by_item_id(
                        builders,
                        "reasoning",
                        item_id if isinstance(item_id, str) else None,
                    )
                    or _find_last_builder(builders, "reasoning")
                )
                if target is not None:
                    if isinstance(item_id, str):
                        target["item_id"] = item_id
                    final_summary = _extract_reasoning_summary(item)
                    if final_summary:
                        target["summary_text"] = final_summary
                        target["summary_by_index"] = {}
                current_builder = None
                continue

    output_items = []

    for builder in builders:
        kind = builder.get("kind")
        if kind == "text":
            text = builder.get("text")
            if isinstance(text, str) and text:
                output_items.append(CodexTextOutputItem(text=text))
            continue

        if kind == "reasoning":
            summary = ""
            by_index = builder.get("summary_by_index")
            if isinstance(by_index, dict) and by_index:
                parts: list[str] = []
                ordered_keys = sorted(key for key in by_index if isinstance(key, int))
                for key in ordered_keys:
                    text = by_index.get(key)
                    if isinstance(text, str) and text.strip():
                        parts.append(text)
                if parts:
                    summary = "\n\n".join(parts).strip()
            if not summary:
                raw_summary = builder.get("summary_text")
                if isinstance(raw_summary, str):
                    summary = raw_summary.strip()
            if summary:
                output_items.append(CodexReasoningOutputItem(summary=summary))
            continue

        if kind == "tool":
            name = builder.get("name")
            if not isinstance(name, str) or not name.strip():
                continue
            call_id = builder.get("call_id")
            if not isinstance(call_id, str) or not call_id.strip():
                call_id = f"call_{uuid.uuid4().hex[:12]}"

            item_id = builder.get("item_id")
            arguments = builder.get("arguments")
            if not isinstance(arguments, dict):
                arguments = {}

            output_items.append(
                CodexToolCallOutputItem(
                    name=name.strip(),
                    call_id=call_id,
                    item_id=item_id if isinstance(item_id, str) and item_id.strip() else None,
                    arguments=arguments,
                )
            )

    error_message = None
    if status in {"failed", "cancelled"}:
        error_message = f"Codex response {status}."
    return CodexResponse(
        output_items=output_items,
        status=status,
        error_message=error_message,
        usage=usage,
    )
