from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from pypimono.engine.container import build_tools
from pypimono.engine.infra.workspace_fs.local_workspace_fs import LocalWorkspaceFs
from pypimono.settings import McpNotionSettings


class BuildToolsTests(unittest.TestCase):
    def test_build_tools_without_notion_keeps_local_tools_only(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            fs = LocalWorkspaceFs(root=Path(temp_dir))
            tools = build_tools(
                workspace_fs=fs,
                notion_settings=McpNotionSettings(pi_mcp_notion_enabled=False),
            )

        self.assertEqual(
            [tool.name for tool in tools],
            ["read", "write", "edit", "bash", "grep", "find", "ls"],
        )

    def test_build_tools_with_missing_notion_manifest_still_returns_local_tools(self) -> None:
        announcements: list[str] = []
        with tempfile.TemporaryDirectory() as temp_dir:
            fs = LocalWorkspaceFs(root=Path(temp_dir))
            tools = build_tools(
                workspace_fs=fs,
                notion_settings=McpNotionSettings(
                    pi_mcp_notion_enabled=True,
                    pi_mcp_notion_manifest_path=Path(temp_dir) / "missing-manifest.json",
                ),
                announce=announcements.append,
            )

        self.assertEqual(
            [tool.name for tool in tools],
            ["read", "write", "edit", "bash", "grep", "find", "ls"],
        )
        self.assertEqual(
            announcements,
            [
                "Notion MCP enabled but no manifest was found. "
                "Run: `uv run -m pypimono mcp notion login`."
            ],
        )


if __name__ == "__main__":
    unittest.main()
