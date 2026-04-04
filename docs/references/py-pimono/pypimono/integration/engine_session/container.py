from __future__ import annotations

from dependency_injector import containers, providers

from pypimono.engine.application.agent import Agent
from pypimono.integration.engine_session.runtime_adapter import EngineAgentRuntimeAdapter


class EngineSessionContainer(containers.DeclarativeContainer):
    agent = providers.Dependency(instance_of=Agent)

    # Integration providers connect one bounded context's concrete service
    # to another context's output-port contract.
    agent_runtime_gateway = providers.Singleton(EngineAgentRuntimeAdapter, agent=agent)
