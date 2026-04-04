from __future__ import annotations

import asyncio
import re
from collections.abc import Callable
from dataclasses import dataclass, field
from typing import Any

from pypimono.ui.application.ports.ui_port import UiPort
from pypimono.ui.application.presentation.startup_formatter import format_ui_startup
from pypimono.ui.infra.runtime.base import UiRunner
from pypimono.ui.infra.sinks.discord import DiscordTurnUpdate, DiscordUiDisplaySink


@dataclass
class _AssistantReplyState:
    text: str = ""
    messages: list[Any] = field(default_factory=list)


class DiscordRuntime(UiRunner):
    def __init__(
        self,
        *,
        bot_token: str | None,
        announce: Callable[[str], None],
        channel_id: int | None = None,
    ):
        self._bot_token = bot_token
        self._announce = announce
        self._channel_id = channel_id
        self._prompt_lock = asyncio.Lock()
        self._client = None
        self._discord = None

    async def run(self, ui: UiPort) -> None:
        if not self._bot_token:
            raise ValueError("PI_DISCORD_BOT_TOKEN is required when PI_OUTPUT_STYLE=discord")

        try:
            import discord
        except ModuleNotFoundError as exc:
            raise ModuleNotFoundError(
                "discord.py is not installed. Install with `pip install discord.py`."
            ) from exc

        sink = DiscordUiDisplaySink()
        ui.subscribe(sink)

        intents = discord.Intents.default()
        intents.message_content = True

        client = discord.Client(intents=intents)
        self._client = client
        self._discord = discord
        startup_text = format_ui_startup(ui.startup_info)

        @client.event
        async def on_ready() -> None:
            self._announce(f"Discord bot ready: {client.user}")
            if self._channel_id is None:
                return
            channel = client.get_channel(self._channel_id)
            if channel is None:
                self._announce(f"Discord channel not found: {self._channel_id}")
                return
            if isinstance(channel, discord.abc.Messageable):
                await channel.send(
                    f"✅ py-pimono connected.\n{startup_text}\n"
                    "Mention the bot in this channel to chat, or send a DM directly."
                )

        @client.event
        async def on_message(message: discord.Message) -> None:
            if message.author.bot:
                return

            is_direct_message = isinstance(message.channel, discord.DMChannel)

            if is_direct_message:
                prompt = message.content.strip()
            else:
                if self._channel_id is not None and message.channel.id != self._channel_id:
                    return
                if client.user is None or not client.user.mentioned_in(message):
                    return
                prompt = _strip_bot_mention(message.content, bot_user_id=client.user.id)

            if not prompt:
                if is_direct_message:
                    await message.reply("Please include a message.")
                else:
                    await message.reply("Please include a message after the mention.")
                return

            async with self._prompt_lock:
                sink.set_background_notice_emitter(lambda text: _reply_text(message, text))
                updates = sink.begin_turn()
                relay_task = asyncio.create_task(_relay_turn_updates(message, updates))
                prompt_error: str | None = None

                try:
                    async with message.channel.typing():
                        await ui.prompt(prompt)
                except Exception as exc:
                    prompt_error = str(exc)
                finally:
                    sink.end_turn()

                sent_count = await relay_task

            if prompt_error is not None:
                await _reply_text(message, f"Error: {prompt_error}")
            elif sent_count == 0:
                await _reply_text(message, "(Response was empty.)")

        try:
            await client.start(self._bot_token)
        finally:
            self._client = None
            self._discord = None

    async def notify_background(self, text: str) -> None:
        client = self._client
        discord = self._discord
        if client is None or discord is None or self._channel_id is None:
            self._announce(text)
            return

        channel = client.get_channel(self._channel_id)
        if channel is None:
            try:
                channel = await client.fetch_channel(self._channel_id)
            except Exception as exc:
                self._announce(f"background notice failed: {exc}")
                return

        if not isinstance(channel, discord.abc.Messageable):
            self._announce(f"background notice target is not messageable: {self._channel_id}")
            return

        await channel.send(text)


def _strip_bot_mention(text: str, *, bot_user_id: int) -> str:
    return re.sub(rf"<@!?{bot_user_id}>", "", text).strip()


async def _relay_turn_updates(
    message: Any,
    updates: asyncio.Queue[DiscordTurnUpdate],
) -> int:
    assistant = _AssistantReplyState()
    sent_count = 0

    while True:
        update = await updates.get()
        if update.kind == "done":
            return sent_count

        if update.kind == "tool_end":
            prefix = "[tool_error]" if update.is_error else "[tool_end]"
            sent_count += await _reply_text(message, f"{prefix} {update.text}".strip())
            continue

        if update.kind == "assistant_text":
            assistant.text = f"{assistant.text}\n\n{update.text}".strip() if assistant.text else update.text
            sent_count += await _sync_assistant_reply(message, assistant)

    return sent_count


async def _sync_assistant_reply(message: Any, assistant: _AssistantReplyState) -> int:
    chunks = _chunk_message(assistant.text)
    sent_count = 0

    for index, chunk in enumerate(chunks):
        if index < len(assistant.messages):
            await assistant.messages[index].edit(content=chunk)
            continue
        assistant.messages.append(await message.reply(chunk, mention_author=False))
        sent_count += 1

    if len(assistant.messages) > len(chunks):
        for stale in assistant.messages[len(chunks) :]:
            await stale.delete()
        del assistant.messages[len(chunks) :]

    return sent_count


async def _reply_text(message: Any, text: str) -> int:
    sent_count = 0
    for chunk in _chunk_message(text):
        await message.reply(chunk, mention_author=False)
        sent_count += 1
    return sent_count


def _chunk_message(text: str, *, limit: int = 1900) -> list[str]:
    if len(text) <= limit:
        return [text]

    lines = text.splitlines(keepends=True)
    chunks: list[str] = []
    current = ""

    for line in lines:
        if len(current) + len(line) <= limit:
            current += line
            continue

        if current:
            chunks.append(current)
            current = ""

        while len(line) > limit:
            chunks.append(line[:limit])
            line = line[limit:]

        current = line

    if current:
        chunks.append(current)

    return chunks
