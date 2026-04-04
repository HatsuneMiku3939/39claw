from __future__ import annotations

from datetime import datetime
from importlib import resources
from pathlib import Path

from pypimono.session.application.ports.agent_runtime_gateway import RuntimeToolInfo

_FALLBACK_TEMPLATE = (
    "You are a minimal coding agent.\n\n"
    "Available tools:\n"
    "{{AVAILABLE_TOOLS}}\n\n"
    "Current date and time: {{CURRENT_DATETIME}}\n"
    "Current working directory: {{CURRENT_WORKING_DIRECTORY}}"
)


def _load_default_template() -> str:
    try:
        return (
            resources.files("pypimono.prompts")
            .joinpath("default_system_prompt.md")
            .read_text(encoding="utf-8")
            .strip()
        )
    except (FileNotFoundError, ModuleNotFoundError, OSError):
        return _FALLBACK_TEMPLATE


def build_session_system_prompt(
    *,
    prompt_cwd: Path,
    tools: list[RuntimeToolInfo],
    now: datetime | None = None,
) -> str:
    prompt_path = prompt_cwd / "PROMPT.md"
    if prompt_path.exists():
        template = prompt_path.read_text(encoding="utf-8").strip()
    else:
        template = _load_default_template()

    date_time = (now or datetime.now().astimezone()).strftime("%A, %B %d, %Y, %I:%M:%S %p %Z")
    available_tools = "(none)"
    if tools:
        available_tools = "\n".join(f"- {tool.name}: {tool.description}" for tool in tools)

    return (
        template.replace("{{AVAILABLE_TOOLS}}", available_tools)
        .replace("{{CURRENT_DATETIME}}", date_time)
        .replace("{{CURRENT_WORKING_DIRECTORY}}", str(prompt_cwd))
        .strip()
    )
