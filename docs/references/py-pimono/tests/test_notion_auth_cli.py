from __future__ import annotations

import io
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path

from pypimono.engine.infra.mcp.auth_cli import _cmd_status
from pypimono.settings import McpNotionSettings


class NotionStatusCommandTests(unittest.TestCase):
    def test_status_reports_disabled_runtime_warning(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            temp_path = Path(temp_dir)
            settings = McpNotionSettings(
                pi_mcp_notion_enabled=False,
                pi_mcp_notion_auth_path=temp_path / "notion-auth.json",
                pi_mcp_notion_manifest_path=temp_path / "notion-tools.json",
            )
            stdout = io.StringIO()

            with redirect_stdout(stdout):
                result = _cmd_status(settings)

        output = stdout.getvalue()
        self.assertEqual(result, 0)
        self.assertIn("enabled: False", output)
        self.assertIn("warning: Notion MCP is configured locally but disabled in runtime settings.", output)

    def test_status_omits_disabled_warning_when_runtime_enabled(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            temp_path = Path(temp_dir)
            settings = McpNotionSettings(
                pi_mcp_notion_enabled=True,
                pi_mcp_notion_auth_path=temp_path / "notion-auth.json",
                pi_mcp_notion_manifest_path=temp_path / "notion-tools.json",
            )
            stdout = io.StringIO()

            with redirect_stdout(stdout):
                result = _cmd_status(settings)

        output = stdout.getvalue()
        self.assertEqual(result, 0)
        self.assertIn("enabled: True", output)
        self.assertNotIn(
            "warning: Notion MCP is configured locally but disabled in runtime settings.",
            output,
        )


if __name__ == "__main__":
    unittest.main()
