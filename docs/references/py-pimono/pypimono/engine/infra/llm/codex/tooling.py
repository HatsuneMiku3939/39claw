from __future__ import annotations

import re
import uuid
from collections.abc import Iterable
from copy import deepcopy
from typing import Any

from pypimono.engine.domain.ports.tool import Tool


def _sanitize_for_item_id(value: str) -> str:
    sanitized = re.sub(r"[^a-zA-Z0-9_-]", "_", value).strip("_")
    if not sanitized:
        sanitized = f"fc_{uuid.uuid4().hex[:10]}"
    if not sanitized.startswith("fc"):
        sanitized = f"fc_{sanitized}"
    sanitized = sanitized[:64]
    sanitized = sanitized.rstrip("_")
    return sanitized or "fc"


def normalize_tool_call_id(raw_id: str) -> tuple[str, str]:
    if "|" in raw_id:
        call_id, item_id = raw_id.split("|", 1)
        call_id = call_id[:64].rstrip("_")
        item_id = item_id[:64].rstrip("_")
        if not call_id:
            call_id = f"call_{uuid.uuid4().hex[:12]}"
        if not item_id:
            item_id = _sanitize_for_item_id(call_id)
        return call_id, item_id

    call_id = re.sub(r"[^a-zA-Z0-9_-]", "_", raw_id).strip("_")
    if not call_id:
        call_id = f"call_{uuid.uuid4().hex[:12]}"
    call_id = call_id[:64].rstrip("_") or call_id
    item_id = _sanitize_for_item_id(call_id)
    return call_id, item_id


def build_tool_specs_from_tools(tools: Iterable[Tool]) -> list[dict[str, Any]]:
    specs: list[dict[str, Any]] = []
    for tool in tools:
        specs.append(
            {
                "type": "function",
                "name": tool.name,
                "description": tool.description,
                "parameters": deepcopy(tool.parameters),
                "strict": None,
            }
        )

    return specs
