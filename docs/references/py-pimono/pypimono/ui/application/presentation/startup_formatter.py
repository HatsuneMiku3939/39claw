from __future__ import annotations

from pypimono.ui.boundary.contracts.startup import UiStartupInfo


def format_ui_startup(info: UiStartupInfo) -> str:
    if info.is_restored:
        return f"Restored {info.restored_message_count} previous messages. ({info.session_location})"
    return f"Starting a new session: {info.session_location}"
