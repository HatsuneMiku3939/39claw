from __future__ import annotations

from dependency_injector import containers, providers

from pypimono.session.application.agent_session import AgentSession
from pypimono.session.application.ports.agent_runtime_gateway import AgentRuntimeGateway
from pypimono.session.application.session_manager import SessionManager
from pypimono.session.infra.store.entry_codec import deserialize_entry, serialize_entry
from pypimono.session.infra.store.local_jsonl_session_store import LocalJsonlSessionStore
from pypimono.settings import AppSettings


class SessionContainer(containers.DeclarativeContainer):
    settings = providers.Dependency(instance_of=AppSettings)
    runtime_gateway = providers.Dependency(instance_of=AgentRuntimeGateway)

    sessions_dir = providers.Callable(lambda settings: settings.resolved_sessions_dir, settings)
    session_id = providers.Callable(lambda settings: settings.pi_session_id, settings)
    runtime_cwd = providers.Callable(lambda settings: settings.runtime_cwd, settings)

    session_store = providers.Singleton(
        LocalJsonlSessionStore,
        base_dir=sessions_dir,
        serialize_entry=serialize_entry,
        deserialize_entry=deserialize_entry,
    )

    session_manager = providers.Singleton(
        SessionManager,
        session_store=session_store,
        session_id=session_id,
    )

    agent_session = providers.Singleton(
        AgentSession,
        runtime=runtime_gateway,
        session_manager=session_manager,
        prompt_cwd=runtime_cwd,
    )
