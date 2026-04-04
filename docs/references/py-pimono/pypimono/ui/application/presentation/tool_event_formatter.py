from __future__ import annotations

import json
from abc import ABC, abstractmethod
from dataclasses import dataclass
from typing import Any, cast

from pypimono.ui.boundary.contracts.message import ToolResultMessage

_MAX_PREVIEW_LINES = 12
_MAX_LINE_CHARS = 160
_MAX_ERROR_CHARS = 240


@dataclass(frozen=True)
class ToolStartPresentation:
    preview_text: str
    preview_lexer: str


@dataclass(frozen=True)
class ToolEndPresentation:
    summary_text: str


@dataclass(frozen=True)
class ToolResultPresentation:
    body: str
    is_error: bool


class ToolPresentationStrategy(ABC):
    @abstractmethod
    def build_start(self, tool_name: str, args: dict[str, Any]) -> ToolStartPresentation:
        raise NotImplementedError

    @abstractmethod
    def build_end(
        self,
        tool_name: str,
        *,
        is_error: bool,
        result: object,
    ) -> ToolEndPresentation:
        raise NotImplementedError

    @abstractmethod
    def build_result(self, message: ToolResultMessage) -> ToolResultPresentation | None:
        raise NotImplementedError


class DefaultToolPresentationStrategy(ToolPresentationStrategy):
    def build_start(self, tool_name: str, args: dict[str, Any]) -> ToolStartPresentation:
        if args:
            try:
                preview_text = json.dumps(args, ensure_ascii=False, separators=(",", ":"), default=str)
            except Exception:
                preview_text = str(args)
            return ToolStartPresentation(preview_text=preview_text, preview_lexer="json")

        return ToolStartPresentation(preview_text="", preview_lexer="text")

    def build_end(
        self,
        tool_name: str,
        *,
        is_error: bool,
        result: object,
    ) -> ToolEndPresentation:
        if is_error:
            text = _to_str(getattr(result, "text", "")).strip()
            text = _truncate_inline(text, _MAX_ERROR_CHARS) if text else "unknown error"
            return ToolEndPresentation(summary_text=f"{tool_name} error: {text}")

        return ToolEndPresentation(summary_text=f"{tool_name} ok")

    def build_result(self, message: ToolResultMessage) -> ToolResultPresentation | None:
        return ToolResultPresentation(
            body=_join_tool_result_body(message),
            is_error=message.isError,
        )


class WriteToolPresentationStrategy(DefaultToolPresentationStrategy):
    def build_start(self, tool_name: str, args: dict[str, Any]) -> ToolStartPresentation:
        return ToolStartPresentation(
            preview_text=self._build_preview(args),
            preview_lexer="diff",
        )

    def build_end(
        self,
        tool_name: str,
        *,
        is_error: bool,
        result: object,
    ) -> ToolEndPresentation:
        if is_error:
            return super().build_end(tool_name, is_error=is_error, result=result)

        details = getattr(result, "details", None)
        path = _extract_path(details)
        if path:
            return ToolEndPresentation(summary_text=f"write {path}")
        return ToolEndPresentation(summary_text="write (path unavailable)")

    def build_result(self, message: ToolResultMessage) -> ToolResultPresentation | None:
        if not message.isError:
            return None
        return super().build_result(message)

    def _build_preview(self, args: dict[str, Any]) -> str:
        path = _to_str(args.get("path")).strip() or "(unknown path)"
        content = _to_str(args.get("content"))
        lines = content.splitlines()

        preview_lines = [
            f"--- a/{path}",
            f"+++ b/{path}",
            f"@@ -0,0 +1,{max(len(lines), 1)} @@",
        ]

        if not lines:
            preview_lines.append("+(empty file)")
            return "\n".join(preview_lines)

        for line in lines[:_MAX_PREVIEW_LINES]:
            preview_lines.append(f"+{_truncate_inline(line, _MAX_LINE_CHARS)}")

        if len(lines) > _MAX_PREVIEW_LINES:
            preview_lines.append(f"... ({len(lines) - _MAX_PREVIEW_LINES} more lines)")

        return "\n".join(preview_lines)


class EditToolPresentationStrategy(DefaultToolPresentationStrategy):
    def build_start(self, tool_name: str, args: dict[str, Any]) -> ToolStartPresentation:
        return ToolStartPresentation(
            preview_text=self._build_preview(args),
            preview_lexer="diff",
        )

    def _build_preview(self, args: dict[str, Any]) -> str:
        path = _to_str(args.get("path")).strip() or "(unknown path)"
        old_text = _to_str(args.get("old_text"))
        new_text = _to_str(args.get("new_text"))
        replace_all = bool(args.get("replace_all", False))

        preview_lines = [
            f"--- a/{path}",
            f"+++ b/{path}",
            "@@ replace @@",
        ]

        old_lines = old_text.splitlines() or [old_text]
        new_lines = new_text.splitlines() or [new_text]

        for line in old_lines[:_MAX_PREVIEW_LINES]:
            preview_lines.append(f"-{_truncate_inline(line, _MAX_LINE_CHARS)}")

        if len(old_lines) > _MAX_PREVIEW_LINES:
            preview_lines.append(f"... ({len(old_lines) - _MAX_PREVIEW_LINES} more removed lines)")

        for line in new_lines[:_MAX_PREVIEW_LINES]:
            preview_lines.append(f"+{_truncate_inline(line, _MAX_LINE_CHARS)}")

        if len(new_lines) > _MAX_PREVIEW_LINES:
            preview_lines.append(f"... ({len(new_lines) - _MAX_PREVIEW_LINES} more added lines)")

        if replace_all:
            preview_lines.append("(replace_all=true)")

        return "\n".join(preview_lines)


class ReadToolPresentationStrategy(DefaultToolPresentationStrategy):
    def build_start(self, tool_name: str, args: dict[str, Any]) -> ToolStartPresentation:
        return ToolStartPresentation(
            preview_text=self._build_preview(args),
            preview_lexer="text",
        )

    def _build_preview(self, args: dict[str, Any]) -> str:
        path = _to_str(args.get("path")).strip() or "(unknown path)"
        offset = args.get("offset")
        limit = args.get("limit")

        parts = [f"path={path}"]
        if offset is not None:
            parts.append(f"offset={offset}")
        if limit is not None:
            parts.append(f"limit={limit}")
        return f"read {' '.join(parts)}"


class BashToolPresentationStrategy(DefaultToolPresentationStrategy):
    def build_start(self, tool_name: str, args: dict[str, Any]) -> ToolStartPresentation:
        return ToolStartPresentation(
            preview_text=self._build_preview(args),
            preview_lexer="text",
        )

    def _build_preview(self, args: dict[str, Any]) -> str:
        command = _truncate_inline(_to_str(args.get("command")).strip(), _MAX_LINE_CHARS)
        cwd = _to_str(args.get("cwd")).strip() or "."
        timeout_sec = args.get("timeout_sec")
        timeout_text = f" timeout={timeout_sec}s" if timeout_sec is not None else ""
        return f"bash cwd={cwd}{timeout_text}\n$ {command}"


class GrepToolPresentationStrategy(DefaultToolPresentationStrategy):
    def build_start(self, tool_name: str, args: dict[str, Any]) -> ToolStartPresentation:
        return ToolStartPresentation(
            preview_text=self._build_preview(args),
            preview_lexer="text",
        )

    def _build_preview(self, args: dict[str, Any]) -> str:
        pattern = _to_str(args.get("pattern")).strip()
        path = _to_str(args.get("path")).strip() or "."
        return f"grep pattern={pattern!r} path={path}"


class FindToolPresentationStrategy(DefaultToolPresentationStrategy):
    def build_start(self, tool_name: str, args: dict[str, Any]) -> ToolStartPresentation:
        return ToolStartPresentation(
            preview_text=self._build_preview(args),
            preview_lexer="text",
        )

    def _build_preview(self, args: dict[str, Any]) -> str:
        pattern = _to_str(args.get("pattern")).strip()
        path = _to_str(args.get("path")).strip() or "."
        return f"find pattern={pattern!r} path={path}"


class LsToolPresentationStrategy(DefaultToolPresentationStrategy):
    def build_start(self, tool_name: str, args: dict[str, Any]) -> ToolStartPresentation:
        return ToolStartPresentation(
            preview_text=self._build_preview(args),
            preview_lexer="text",
        )

    def _build_preview(self, args: dict[str, Any]) -> str:
        path = _to_str(args.get("path")).strip() or "."
        recursive = bool(args.get("recursive", False))
        return f"ls path={path} recursive={recursive}"


class NotionToolPresentationStrategy(DefaultToolPresentationStrategy):
    def build_end(
        self,
        tool_name: str,
        *,
        is_error: bool,
        result: object,
    ) -> ToolEndPresentation:
        if is_error:
            return super().build_end(tool_name, is_error=is_error, result=result)

        details = getattr(result, "details", None)
        title = _extract_display_value(details, "display_title")
        url = _extract_display_value(details, "display_url")
        if title and url:
            return ToolEndPresentation(summary_text=f"- {title}\n- {url}")
        if title:
            return ToolEndPresentation(summary_text=f"- {title}")
        if url:
            return ToolEndPresentation(summary_text=f"- {url}")
        return ToolEndPresentation(summary_text=f"{tool_name} ok")

    def build_result(self, message: ToolResultMessage) -> ToolResultPresentation | None:
        if not message.isError:
            return None
        return super().build_result(message)


class ToolPresentationRegistry:
    def __init__(
        self,
        *,
        strategies: dict[str, ToolPresentationStrategy],
        default: ToolPresentationStrategy,
    ):
        self._strategies = dict(strategies)
        self._default = default

    def resolve(self, tool_name: str) -> ToolPresentationStrategy:
        if tool_name.startswith("notion-"):
            return _NOTION_TOOL_STRATEGY
        return self._strategies.get(tool_name, self._default)


_DEFAULT_STRATEGY = DefaultToolPresentationStrategy()
_NOTION_TOOL_STRATEGY = NotionToolPresentationStrategy()
_TOOL_PRESENTATION_REGISTRY = ToolPresentationRegistry(
    strategies={
        "write": WriteToolPresentationStrategy(),
        "edit": EditToolPresentationStrategy(),
        "read": ReadToolPresentationStrategy(),
        "bash": BashToolPresentationStrategy(),
        "grep": GrepToolPresentationStrategy(),
        "find": FindToolPresentationStrategy(),
        "ls": LsToolPresentationStrategy(),
    },
    default=_DEFAULT_STRATEGY,
)


def build_tool_start_presentation(tool_name: str, args: object) -> ToolStartPresentation:
    payload = _as_dict(args)
    strategy = _TOOL_PRESENTATION_REGISTRY.resolve(tool_name)
    return strategy.build_start(tool_name, payload)


def build_tool_end_presentation(
    tool_name: str,
    *,
    is_error: bool,
    result: object,
) -> ToolEndPresentation:
    strategy = _TOOL_PRESENTATION_REGISTRY.resolve(tool_name)
    return strategy.build_end(tool_name, is_error=is_error, result=result)


def build_tool_result_presentation(message: ToolResultMessage) -> ToolResultPresentation | None:
    strategy = _TOOL_PRESENTATION_REGISTRY.resolve(message.toolName)
    return strategy.build_result(message)


def _extract_path(details: object) -> str | None:
    path = _as_dict(details).get("path")
    if path is None:
        return None
    text = _to_str(path).strip()
    return text or None


def _extract_display_value(details: object, key: str) -> str | None:
    value = _as_dict(details).get(key)
    if value is None:
        return None
    text = _to_str(value).strip()
    return text or None


def _truncate_inline(text: str, limit: int) -> str:
    normalized = " ".join(text.split())
    if len(normalized) <= limit:
        return normalized
    return f"{normalized[:limit]}..."


def _to_str(value: object) -> str:
    if value is None:
        return ""
    return str(value)


def _as_dict(value: object) -> dict[str, Any]:
    if not isinstance(value, dict):
        return {}
    return cast(dict[str, Any], value)


def _join_tool_result_body(message: ToolResultMessage) -> str:
    return "\n".join(block.text for block in message.content if block.text)
