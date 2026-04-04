from __future__ import annotations

from dependency_injector import containers, providers

from pypimono.engine.application.agent import Agent
from pypimono.engine.domain.ports.tool import Tool
from pypimono.engine.infra.llm.factory import create_llm
from pypimono.engine.infra.mcp.notion import build_notion_tools
from pypimono.engine.infra.tool import (
    BashTool,
    EditTool,
    FindTool,
    GrepTool,
    LsTool,
    ReadTool,
    WriteTool,
)
from pypimono.engine.infra.workspace_fs.local_workspace_fs import LocalWorkspaceFs
from pypimono.settings import AppSettings, McpNotionSettings


def build_tools(
    *,
    workspace_fs: LocalWorkspaceFs,
    notion_settings: McpNotionSettings,
    announce=None,
) -> list[Tool]:
    local_tools: list[Tool] = [
        ReadTool(workspace_fs),
        WriteTool(workspace_fs),
        EditTool(workspace_fs),
        BashTool(workspace_fs),
        GrepTool(workspace_fs),
        FindTool(workspace_fs),
        LsTool(workspace_fs),
    ]
    remote_tools = build_notion_tools(notion_settings, announce=announce)
    return [*local_tools, *remote_tools]


class EngineContainer(containers.DeclarativeContainer):
    settings = providers.Dependency(instance_of=AppSettings)
    notion_settings = providers.Dependency(instance_of=McpNotionSettings)
    llm_announce = providers.Dependency()

    runtime_cwd = providers.Callable(lambda settings: settings.runtime_cwd, settings)
    session_id = providers.Callable(lambda settings: settings.pi_session_id, settings)

    llm = providers.Singleton(
        create_llm,
        app_settings=settings,
        session_id=session_id,
        announce=llm_announce,
    )

    workspace_fs = providers.Singleton(LocalWorkspaceFs, root=runtime_cwd)

    tools = providers.Callable(
        build_tools,
        workspace_fs=workspace_fs,
        notion_settings=notion_settings,
        announce=llm_announce,
    )

    agent = providers.Singleton(Agent, llm=llm, tools=tools)
