from __future__ import annotations

from pypimono.session.boundary.contracts.startup import SessionStartupInfo
from pypimono.ui.boundary.contracts.startup import UiStartupInfo


def to_ui_startup_info(info: SessionStartupInfo) -> UiStartupInfo:
    return UiStartupInfo(
        session_location=info.session_location,
        restored_message_count=info.restored_message_count,
    )
