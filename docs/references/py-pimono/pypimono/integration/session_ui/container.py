from __future__ import annotations

from dependency_injector import containers, providers

from pypimono.integration.session_ui.session_gateway_adapter import SessionUiGatewayAdapter
from pypimono.session.application.ports.session_port import SessionPort


class SessionUiContainer(containers.DeclarativeContainer):
    session_port = providers.Dependency(instance_of=SessionPort)

    session_gateway = providers.Singleton(SessionUiGatewayAdapter, session=session_port)
