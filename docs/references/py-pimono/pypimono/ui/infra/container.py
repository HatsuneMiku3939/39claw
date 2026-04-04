from __future__ import annotations

import sys
from dataclasses import dataclass
from typing import Callable

from dependency_injector import containers, providers

from pypimono.settings import AppSettings, OutputStyle
from pypimono.ui.application.ports.ui_port import UiPort
from pypimono.ui.infra.runtime.base import UiRunner
from pypimono.ui.infra.runtime.console_runtime import ConsoleRuntime
from pypimono.ui.infra.runtime.discord_runtime import DiscordRuntime
from pypimono.ui.infra.runtime.textual_runtime import TextualRuntime


@dataclass(frozen=True)
class UiRuntime:
    runner: UiRunner
    llm_announce: Callable[[str], None] | None

    async def run(self, ui: UiPort) -> None:
        await self.runner.run(ui)

    async def notify_background(self, text: str) -> None:
        await self.runner.notify_background(text)


def create_ui_runtime(
    *,
    output_style: OutputStyle,
    announce: Callable[[str], None],
    discord_bot_token: str | None,
    discord_channel_id: int | None,
) -> UiRuntime:
    announce(f"output style: {output_style}")

    if output_style == OutputStyle.DISCORD:
        return UiRuntime(
            runner=DiscordRuntime(
                bot_token=discord_bot_token,
                announce=announce,
                channel_id=discord_channel_id,
            ),
            llm_announce=announce,
        )

    if output_style == OutputStyle.TEXTUAL:
        if not (sys.stdin.isatty() and sys.stdout.isatty()):
            announce("textual mode requires an interactive TTY. Falling back to console output.")
            return UiRuntime(
                runner=ConsoleRuntime(output_style=OutputStyle.PLAIN, announce=announce),
                llm_announce=announce,
            )

        try:
            from pypimono.ui.infra.tui.textual_chat_app import TextualChatApp
        except ModuleNotFoundError:
            announce("textual is not installed. Falling back to console output.")
            return UiRuntime(
                runner=ConsoleRuntime(output_style=OutputStyle.PLAIN, announce=announce),
                llm_announce=announce,
            )

        return UiRuntime(
            runner=TextualRuntime(app_factory=TextualChatApp, announce=announce),
            llm_announce=None,
        )

    return UiRuntime(
        runner=ConsoleRuntime(output_style=output_style, announce=announce),
        llm_announce=announce,
    )


class UiInfraContainer(containers.DeclarativeContainer):
    settings = providers.Dependency(instance_of=AppSettings)
    announce = providers.Dependency()

    output_style = providers.Callable(lambda settings: settings.pi_output_style, settings)
    discord_bot_token = providers.Callable(lambda settings: settings.pi_discord_bot_token, settings)
    discord_channel_id = providers.Callable(lambda settings: settings.pi_discord_channel_id, settings)

    ui_runtime = providers.Singleton(
        create_ui_runtime,
        output_style=output_style,
        announce=announce,
        discord_bot_token=discord_bot_token,
        discord_channel_id=discord_channel_id,
    )
