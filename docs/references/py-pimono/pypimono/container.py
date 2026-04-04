from __future__ import annotations

from dependency_injector import containers, providers

from pypimono.engine.container import EngineContainer
from pypimono.integration.engine_session.container import EngineSessionContainer
from pypimono.integration.session_ui.container import SessionUiContainer
from pypimono.session.container import SessionContainer
from pypimono.settings import get_app_settings, get_mcp_notion_settings
from pypimono.ui.container import UiContainer
from pypimono.ui.infra.container import UiInfraContainer


class AppContainer(containers.DeclarativeContainer):
    settings = providers.Singleton(get_app_settings)
    notion_settings = providers.Singleton(get_mcp_notion_settings)
    announce = providers.Object(print)

    ui_infra = providers.Container(
        UiInfraContainer,
        settings=settings,
        announce=announce,
    )

    llm_announce = providers.Callable(
        lambda ui_runtime: ui_runtime.llm_announce,
        ui_infra.ui_runtime,
    )

    engine = providers.Container(
        EngineContainer,
        settings=settings,
        notion_settings=notion_settings,
        llm_announce=llm_announce,
    )

    engine_session = providers.Container(
        EngineSessionContainer,
        agent=engine.agent,
    )

    session = providers.Container(
        SessionContainer,
        settings=settings,
        runtime_gateway=engine_session.agent_runtime_gateway,
    )

    session_ui = providers.Container(
        SessionUiContainer,
        session_port=session.agent_session,
    )

    ui = providers.Container(
        UiContainer,
        session_gateway=session_ui.session_gateway,
    )
