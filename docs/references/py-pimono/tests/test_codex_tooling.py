from __future__ import annotations

import unittest
from dataclasses import dataclass
from typing import Any

from pypimono.engine.domain.ports.tool import Tool, ToolResult
from pypimono.engine.infra.llm.codex.tooling import build_tool_specs_from_tools


@dataclass
class _DummyTool(Tool):
    name: str
    description: str
    parameters: dict[str, Any]

    async def execute(
        self,
        tool_call_id: str,
        args: dict[str, Any],
        *,
        on_update=None,
    ) -> ToolResult:
        del tool_call_id, args, on_update
        return ToolResult(text="ok")


class BuildToolSpecsTests(unittest.TestCase):
    def test_dynamic_tool_schema_is_preserved(self) -> None:
        tool = _DummyTool(
            name="notion_search",
            description="Search pages from Notion.",
            parameters={
                "type": "object",
                "properties": {
                    "query": {"type": "string"},
                },
                "required": ["query"],
            },
        )

        specs = build_tool_specs_from_tools([tool])

        self.assertEqual(
            specs,
            [
                {
                    "type": "function",
                    "name": "notion_search",
                    "description": "Search pages from Notion.",
                    "parameters": {
                        "type": "object",
                        "properties": {
                            "query": {"type": "string"},
                        },
                        "required": ["query"],
                    },
                    "strict": None,
                }
            ],
        )


if __name__ == "__main__":
    unittest.main()
