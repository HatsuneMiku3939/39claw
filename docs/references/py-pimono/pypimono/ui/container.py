from __future__ import annotations

from dependency_injector import containers, providers

from pypimono.ui.application.chat_ui import ChatUi
from pypimono.ui.application.ports.session_gateway import SessionGateway


class UiContainer(containers.DeclarativeContainer):
    session_gateway = providers.Dependency(instance_of=SessionGateway)

    chat_ui = providers.Singleton(ChatUi, session=session_gateway)
