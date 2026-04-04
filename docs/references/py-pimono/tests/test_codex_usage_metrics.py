from __future__ import annotations

import unittest

from pypimono.engine.infra.llm.codex.responses_client import format_usage_summary
from pypimono.engine.infra.llm.codex.responses_models import CodexResponse, CodexUsage
from pypimono.engine.infra.llm.codex.responses_parser import parse_sse_events


class ParseCodexUsageTests(unittest.TestCase):
    def test_response_done_usage_is_captured(self) -> None:
        response = parse_sse_events(
            [
                {
                    "type": "response.done",
                    "response": {
                        "status": "completed",
                        "usage": {
                            "input_tokens": 12_345,
                            "output_tokens": 678,
                            "total_tokens": 13_023,
                            "input_tokens_details": {
                                "cached_tokens": 12_000,
                            },
                        },
                    },
                }
            ]
        )

        assert response.usage is not None
        self.assertEqual(response.usage.input_tokens, 12_345)
        self.assertEqual(response.usage.output_tokens, 678)
        self.assertEqual(response.usage.total_tokens, 13_023)
        self.assertEqual(response.usage.cached_input_tokens, 12_000)
        self.assertEqual(response.usage.uncached_input_tokens, 345)


class FormatCodexUsageSummaryTests(unittest.TestCase):
    def test_summary_includes_cache_and_context_left(self) -> None:
        summary = format_usage_summary(
            model_id="gpt-5.2-codex",
            response=CodexResponse(
                usage=CodexUsage(
                    input_tokens=12_345,
                    output_tokens=678,
                    total_tokens=13_023,
                    cached_input_tokens=12_000,
                )
            ),
        )

        self.assertIn("in=12,345", summary)
        self.assertIn("out=678", summary)
        self.assertIn("total=13,023", summary)
        self.assertIn("cache_hit=12,000", summary)
        self.assertIn("cache_miss=345", summary)
        self.assertIn("cache_hit_rate=97.2%", summary)
        self.assertIn("ctx_left~=387,655/400,000", summary)

    def test_summary_handles_missing_usage(self) -> None:
        summary = format_usage_summary(
            model_id="gpt-5.2-codex",
            response=CodexResponse(),
        )

        self.assertEqual(summary, "[llm] usage unavailable")


if __name__ == "__main__":
    unittest.main()
