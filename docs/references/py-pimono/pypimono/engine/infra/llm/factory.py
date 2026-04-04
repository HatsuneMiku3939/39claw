from __future__ import annotations

from collections.abc import Callable

from pypimono.engine.domain.ports.llm import LlmGateway
from pypimono.engine.infra.llm.codex.adapter import OpenAICodexLlm
from pypimono.engine.infra.llm.codex.token_store import has_codex_cli_auth
from pypimono.engine.infra.llm.mock_llm import MockLlm
from pypimono.settings import AppSettings, LlmProvider, get_codex_settings


def _resolve_reasoning_effort(app_settings: AppSettings, codex_reasoning_effort: str | None) -> str | None:
    level = (app_settings.pi_thinking_level or "").strip().lower()
    if level:
        mapping = {
            "none": "none",
            "minimal": "minimal",
            "low": "low",
            "medium": "medium",
            "high": "high",
            "xhigh": "xhigh",
        }
        return mapping.get(level)

    effort = app_settings.pi_reasoning_effort or codex_reasoning_effort
    return effort.strip() if effort and effort.strip() else None


def create_llm(
    *,
    app_settings: AppSettings,
    session_id: str,
    announce: Callable[[str], None] | None = None,
) -> LlmGateway:
    provider_pref = app_settings.pi_llm_provider
    codex_provider_options = {
        LlmProvider.AUTO,
        LlmProvider.CODEX,
        LlmProvider.OPENAI_CODEX,
    }

    if provider_pref == LlmProvider.MOCKLLM:
        if announce:
            announce("Running with MockLlm.")
        return MockLlm()

    if provider_pref in codex_provider_options:
        codex_settings = get_codex_settings()
        codex_auth_path = codex_settings.resolved_codex_auth_path
        codex_auth_path_str = str(codex_auth_path)
        codex_available = has_codex_cli_auth(codex_auth_path_str)

        if codex_available:
            model_id = codex_settings.codex_model
            llm = OpenAICodexLlm(
                model_id,
                auth_path=codex_auth_path_str,
                base_url=codex_settings.codex_base_url,
                session_id=session_id,
                originator=codex_settings.codex_originator,
                text_verbosity=codex_settings.codex_text_verbosity,
                reasoning_effort=_resolve_reasoning_effort(
                    app_settings, codex_settings.codex_reasoning_effort
                ),
                announce=announce,
            )
            if announce:
                announce(f"Using Codex OAuth: openai-codex/{model_id}")
            return llm

        if announce:
            announce(f"Codex auth not found; running with MockLlm. ({codex_auth_path})")
            announce("If needed, run: codex login")
        return MockLlm()

    raise NotImplementedError(
        f"provider '{provider_pref}' is not implemented. "
        f"Supported: {LlmProvider.AUTO.value}, {LlmProvider.CODEX.value}, "
        f"{LlmProvider.OPENAI_CODEX.value}, {LlmProvider.MOCKLLM.value}"
    )
