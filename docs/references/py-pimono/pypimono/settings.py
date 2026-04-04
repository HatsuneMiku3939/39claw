from __future__ import annotations

from enum import StrEnum
from functools import lru_cache
from pathlib import Path

from pydantic import field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict

_BASE_SETTINGS_CONFIG = SettingsConfigDict(
    extra="ignore",
    case_sensitive=False,
    env_file=".env",
    env_file_encoding="utf-8",
)


class LlmProvider(StrEnum):
    AUTO = "auto"
    CODEX = "codex"
    OPENAI_CODEX = "openai-codex"
    MOCKLLM = "mockllm"


class OutputStyle(StrEnum):
    PLAIN = "plain"
    RICH = "rich"
    TEXTUAL = "textual"
    DISCORD = "discord"


class AppSettings(BaseSettings):
    model_config = _BASE_SETTINGS_CONFIG

    pi_llm_provider: LlmProvider = LlmProvider.AUTO
    pi_output_style: OutputStyle = OutputStyle.TEXTUAL
    pi_thinking_level: str | None = "xhigh"
    pi_reasoning_effort: str | None = None
    pi_sessions_dir: str = ".sessions"
    pi_session_id: str = "default"
    pi_discord_bot_token: str | None = None
    pi_discord_channel_id: int | None = None

    @property
    def runtime_cwd(self) -> Path:
        return Path.cwd().resolve()

    @property
    def resolved_sessions_dir(self) -> Path:
        sessions_dir = Path(self.pi_sessions_dir).expanduser()
        if sessions_dir.is_absolute():
            return sessions_dir
        return (self.runtime_cwd / sessions_dir).resolve()

    @field_validator("pi_llm_provider", mode="before")
    @classmethod
    def _normalize_provider(cls, value):
        if isinstance(value, str):
            return value.strip().lower()
        return value

    @field_validator("pi_output_style", mode="before")
    @classmethod
    def _normalize_output_style(cls, value):
        if isinstance(value, str):
            return value.strip().lower()
        return value

    @field_validator(
        "pi_sessions_dir",
        "pi_session_id",
        mode="before",
    )
    @classmethod
    def _normalize_text_fields(cls, value):
        if isinstance(value, str):
            normalized = value.strip()
            if normalized:
                return normalized
        return value

    @field_validator(
        "pi_sessions_dir",
        "pi_session_id",
    )
    @classmethod
    def _require_non_empty_text_fields(cls, value: str):
        if not value:
            raise ValueError("must not be empty")
        return value


class McpNotionSettings(BaseSettings):
    model_config = _BASE_SETTINGS_CONFIG

    pi_mcp_notion_enabled: bool = False
    pi_mcp_notion_url: str = "https://mcp.notion.com/mcp"
    pi_mcp_notion_oauth_resource: str = "https://mcp.notion.com"
    pi_mcp_notion_redirect_uri: str = "http://127.0.0.1:14565/oauth/callback"
    pi_mcp_notion_auth_path: Path | None = None
    pi_mcp_notion_manifest_path: Path | None = None
    pi_mcp_notion_open_browser: bool = False
    pi_mcp_notion_client_name: str = "py-pimono"
    pi_mcp_notion_scopes: str | None = None

    @property
    def resolved_mcp_notion_auth_path(self) -> Path:
        if self.pi_mcp_notion_auth_path is not None:
            return self.pi_mcp_notion_auth_path.expanduser()
        return Path.home() / ".pypimono" / "mcp" / "notion-auth.json"

    @property
    def resolved_mcp_notion_manifest_path(self) -> Path:
        if self.pi_mcp_notion_manifest_path is not None:
            return self.pi_mcp_notion_manifest_path.expanduser()
        return Path.home() / ".pypimono" / "mcp" / "notion-tools.json"

    @property
    def resolved_mcp_notion_scopes(self) -> tuple[str, ...]:
        raw = self.pi_mcp_notion_scopes
        if raw is None:
            return ()
        normalized = raw.replace(",", " ")
        return tuple(part for part in normalized.split() if part)

    @field_validator(
        "pi_mcp_notion_url",
        "pi_mcp_notion_oauth_resource",
        "pi_mcp_notion_redirect_uri",
        "pi_mcp_notion_client_name",
        "pi_mcp_notion_scopes",
        mode="before",
    )
    @classmethod
    def _normalize_text_fields(cls, value):
        if isinstance(value, str):
            normalized = value.strip()
            if normalized:
                return normalized
        return value

    @field_validator(
        "pi_mcp_notion_url",
        "pi_mcp_notion_oauth_resource",
        "pi_mcp_notion_redirect_uri",
        "pi_mcp_notion_client_name",
    )
    @classmethod
    def _require_non_empty_text_fields(cls, value: str):
        if not value:
            raise ValueError("must not be empty")
        return value


class CodexSettings(BaseSettings):
    model_config = _BASE_SETTINGS_CONFIG

    codex_model: str = "gpt-5.4"
    codex_auth_path: Path | None = None
    codex_base_url: str | None = None
    codex_originator: str = "pi"
    codex_text_verbosity: str = "medium"
    codex_reasoning_effort: str | None = None

    @property
    def resolved_codex_auth_path(self) -> Path:
        if self.codex_auth_path is not None:
            return self.codex_auth_path.expanduser()
        return Path.home() / ".codex" / "auth.json"


@lru_cache(maxsize=1)
def get_app_settings() -> AppSettings:
    return AppSettings()


@lru_cache(maxsize=1)
def get_codex_settings() -> CodexSettings:
    return CodexSettings()


@lru_cache(maxsize=1)
def get_mcp_notion_settings() -> McpNotionSettings:
    return McpNotionSettings()


def reset_settings_cache() -> None:
    get_app_settings.cache_clear()
    get_codex_settings.cache_clear()
    get_mcp_notion_settings.cache_clear()
