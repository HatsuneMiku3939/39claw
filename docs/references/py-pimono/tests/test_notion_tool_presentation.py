from __future__ import annotations

import asyncio
import json
import unittest

from pypimono.engine.infra.mcp.tool_adapter import RemoteMcpTool
from pypimono.ui.application.presentation.ui_event_presenter import present_ui_event
from pypimono.ui.application.presentation.view_models import ToolEndViewModel
from pypimono.ui.boundary.contracts.incoming_event import (
    TYPE_MESSAGE_END,
    TYPE_TOOL_EXECUTION_END,
    ToolExecutionResult,
    UiIncomingEvent,
)
from pypimono.ui.boundary.contracts.message import TextBlock, ToolResultMessage


class _FakeRemoteMcpClient:
    def __init__(self, *, result: dict, rendered_text: str):
        self._result = result
        self._rendered_text = rendered_text

    async def call_tool(self, name: str, arguments: dict) -> dict:
        del name, arguments
        return self._result

    def render_tool_result_text(self, result: dict) -> str:
        self._assert_same_result(result)
        return self._rendered_text

    def _assert_same_result(self, result: dict) -> None:
        if result != self._result:
            raise AssertionError("unexpected result payload")


class RemoteMcpToolPresentationTests(unittest.TestCase):
    def test_notion_tool_extracts_title_and_url_for_display(self) -> None:
        rendered_payload = {
            "results": [
                {
                    "title": "Project Notes",
                    "url": "https://www.notion.so/project-notes",
                }
            ]
        }
        tool = RemoteMcpTool(
            client=_FakeRemoteMcpClient(
                result={
                    "content": [
                        {
                            "type": "text",
                            "text": json.dumps(rendered_payload, ensure_ascii=False),
                        }
                    ]
                },
                rendered_text=json.dumps(rendered_payload, ensure_ascii=False),
            ),
            server_name="notion",
            name="notion-search",
            description="Search Notion",
            parameters={},
        )

        result = asyncio.run(tool.execute("call_123", {"query": "project"}))

        self.assertEqual(result.details["display_title"], "Project Notes")
        self.assertEqual(result.details["display_url"], "https://www.notion.so/project-notes")

    def test_successful_notion_tool_result_is_hidden_from_ui(self) -> None:
        event = UiIncomingEvent(
            type=TYPE_MESSAGE_END,
            message=ToolResultMessage(
                toolCallId="call_123",
                toolName="notion-fetch",
                content=[TextBlock(text='{"title":"Project Notes","url":"https://www.notion.so/project-notes"}')],
                isError=False,
            ),
        )

        self.assertEqual(present_ui_event(event), [])

    def test_successful_notion_tool_end_shows_compact_title_and_url(self) -> None:
        event = UiIncomingEvent(
            type=TYPE_TOOL_EXECUTION_END,
            tool_name="notion-fetch",
            result=ToolExecutionResult(
                text="ignored",
                details={
                    "display_title": "Project Notes",
                    "display_url": "https://www.notion.so/project-notes",
                },
            ),
            is_error=False,
        )

        presented = present_ui_event(event)

        self.assertEqual(
            presented,
            [
                ToolEndViewModel(
                    tool_name="notion-fetch",
                    is_error=False,
                    summary_text="- Project Notes\n- https://www.notion.so/project-notes",
                )
            ],
        )


if __name__ == "__main__":
    unittest.main()
