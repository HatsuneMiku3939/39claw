from __future__ import annotations

import platform

from pypimono import __version__


def default_user_agent() -> str:
    return (
        f"py-pimono/{__version__} "
        f"({platform.system()} {platform.release()}; {platform.machine()})"
    )
