# Architecture

This document explains how `py-pimono` is split, how the pieces talk to each other, and where to look depending on what you want to change. The Korean version is available in [ARCHITECTURE.kr.md](ARCHITECTURE.kr.md).

## Why the codebase is split this way

The main idea behind this repository is that a code agent becomes much easier to learn from once you separate a few concerns that usually get mixed together:

- the agent loop itself
- session and history management
- the UI surface

`py-pimono` models those concerns as three hexagons:

- `engine`: the agent core that talks to the model and runs tools
- `session`: the layer that manages turns, history, and persistence
- `ui`: the layer that turns session events into something users can see and interact with

There is also an `integration` layer that adapts one hexagon to another without leaking internal types across boundaries.

That split is the whole point of the project. If you only care about agent behavior, you should not have to understand session storage first. If you only care about UI rendering, you should not have to read the LLM adapter first.

## Top-level assembly

The CLI entry point is `pypimono/cli.py`, and the top-level composition root is `pypimono/container.py`.

```text
pypimono.cli:main
  -> AppContainer
    -> UiInfraContainer
    -> UiContainer
    -> SessionUiContainer
    -> SessionContainer
    -> EngineSessionContainer
    -> EngineContainer
```

At a high level:

- `EngineContainer` creates the agent, model gateway, workspace adapter, and tools.
- `SessionContainer` creates the session manager, session store, and `AgentSession`.
- `UiContainer` creates the UI-facing application layer.
- `UiInfraContainer` chooses the actual runtime surface, such as plain console, rich console, Textual, or Discord.
- the `integration` containers bridge one hexagon to the next.

The result is an application where the core pieces are composed explicitly instead of reaching into each other directly.

## What to read depending on your question

### If you care about the agent loop

Start with `engine`.

The most useful files are:

- `pypimono/engine/application/agent.py`
- `pypimono/engine/domain/agent_loop.py`
- `pypimono/engine/domain/messages.py`
- `pypimono/engine/infra/llm/*`
- `pypimono/engine/infra/tool/*`
- `pypimono/engine/infra/mcp/*`

This is the layer that tells you:

- how a turn is executed
- when the model is called
- how tool calls and tool results are represented
- how the loop decides whether to continue or stop
- how remote MCP tools are discovered, authenticated, and exposed as regular tools

If you want to add a tool, change the loop, or swap model behavior, most of the work should stay inside `pypimono/engine/*`.

### If you care about session storage and orchestration

Start with `session`.

The most useful files are:

- `pypimono/session/application/agent_session.py`
- `pypimono/session/application/session_manager.py`
- `pypimono/session/application/system_prompt_builder.py`
- `pypimono/session/domain/*`
- `pypimono/session/infra/store/*`

This is the layer that tells you:

- how history is loaded and saved
- how the current session is restored
- when the system prompt is built
- how session-level events are emitted

If you want to move sessions to cloud storage, manage multiple session strategies, or change how history is restored, most of the work should stay inside `pypimono/session/*`.

### If you care about the UI surface

Start with `ui`.

The most useful files are:

- `pypimono/ui/application/chat_ui.py`
- `pypimono/ui/application/presentation/*`
- `pypimono/ui/infra/runtime/*`
- `pypimono/ui/infra/sinks/*`
- `pypimono/ui/infra/tui/*`

This is the layer that tells you:

- how session events are turned into display events
- how plain, rich, textual, and Discord outputs differ
- how the runtime input loop works
- where actual rendering happens

If you want to change what the user sees, make tool output prettier, or move the agent to another surface, most of the work should stay inside `pypimono/ui/*`.

### If you care about the boundaries

Start with `integration`.

The key pieces are:

- `pypimono/integration/engine_session/*`
- `pypimono/integration/session_ui/*`

This is where one hexagon is adapted to another:

- engine `Agent` -> session `AgentRuntimeGateway`
- session `SessionPort` -> UI `SessionGateway`

If you want to understand where type translation happens and how boundary leakage is avoided, this is the layer to inspect.

## Flow through the system

### From user input to the agent

When the user types something, the path looks like this:

```text
ui/infra/runtime
  -> ui.ChatUi
  -> integration.session_ui.SessionUiGatewayAdapter
  -> session.AgentSession
  -> integration.engine_session.EngineAgentRuntimeAdapter
  -> engine.Agent
```

This path matters because it shows that the UI does not call the engine directly. The input crosses one boundary at a time.

### From engine events back to the screen

When the engine emits events, the flow moves in the opposite direction:

```text
engine.AgentEvent
  -> integration.engine_session
  -> session.SessionUiEvent
  -> integration.session_ui
  -> ui.UiIncomingEvent
  -> ui/application/presentation
  -> ui.UiDisplayEvent
  -> ui/infra/sinks
```

That means:

- the engine does not know what UI framework is being used
- the UI does not need engine-specific event types
- the session layer remains the orchestration boundary in the middle

## Directory map

### `engine`

- `pypimono/engine/domain/*`
  - core models and rules
  - includes `agent_event.py`, `messages.py`, `agent_loop.py`, and `ports/*`
- `pypimono/engine/application/*`
  - engine use cases
  - the main entry point is `pypimono/engine/application/agent.py`
- `pypimono/engine/infra/llm/*`
  - model gateway implementations
  - includes Codex and MockLlm
- `pypimono/engine/infra/mcp/*`
  - remote MCP OAuth, token storage, manifest sync, and remote tool adapters
- `pypimono/engine/infra/tool/*`
  - built-in tools such as `read`, `write`, `edit`, `bash`, `grep`, `find`, and `ls`
- `pypimono/engine/infra/workspace_fs/*`
  - local filesystem adapter for the workspace

### `session`

- `pypimono/session/domain/*`
  - session models such as history, entries, and session messages
- `pypimono/session/boundary/contracts/*`
  - the contract types exposed by the session layer
- `pypimono/session/boundary/mappers/*`
  - translation between session domain models and session contracts
- `pypimono/session/application/*`
  - session use cases and orchestration
- `pypimono/session/application/ports/*`
  - contracts such as `SessionPort`, `AgentRuntimeGateway`, and event/store ports
- `pypimono/session/infra/store/*`
  - JSONL-based session persistence

### `ui`

- `pypimono/ui/boundary/contracts/*`
  - contract types that the UI layer receives from session
- `pypimono/ui/application/*`
  - UI use cases
  - the main entry point is `pypimono/ui/application/chat_ui.py`
- `pypimono/ui/application/ports/*`
  - ports such as `UiPort`, `SessionGateway`, and event sinks
- `pypimono/ui/application/presentation/*`
  - translation from incoming session events to UI-facing display models
- `pypimono/ui/infra/runtime/*`
  - runtime entry points such as plain console, Textual, or Discord
- `pypimono/ui/infra/sinks/*`
  - rendering sinks for plain, rich, textual, and Discord output
- `pypimono/ui/infra/tui/*`
  - Textual-specific code

### `integration`

- `pypimono/integration/engine_session/*`
- `pypimono/integration/session_ui/*`

## Suggested reading order

If you are new to the repository, this is the fastest way to get oriented:

1. `pypimono/cli.py`
2. `pypimono/container.py`
3. the three main use cases
   - engine: `pypimono/engine/application/agent.py`
   - session: `pypimono/session/application/agent_session.py`
   - ui: `pypimono/ui/application/chat_ui.py`
4. the boundary adapters
   - `pypimono/integration/engine_session/runtime_adapter.py`
   - `pypimono/integration/session_ui/session_gateway_adapter.py`
5. the presentation and rendering layer
   - `pypimono/ui/application/presentation/*`
   - `pypimono/ui/infra/sinks/*`
6. implementation details as needed
   - `pypimono/engine/infra/*`
   - `pypimono/session/infra/*`
   - `pypimono/ui/infra/*`

## Runtime and configuration reference

### Installation

Base install:

```bash
uv sync
```

If you want to enable the Notion hosted MCP integration, authenticate once and sync the manifest:

```bash
uv run -m pypimono mcp notion login
```

If you want `rich` output:

```bash
uv sync --extra rich
```

If you want the Textual TUI:

```bash
uv sync --extra textual
```

If you want both:

```bash
uv sync --extra ui
```

For local development with development tools and UI dependencies:

```bash
uv sync --group dev
```

To sync into the currently active virtual environment instead of `.venv`:

```bash
uv sync --group dev --active
```

### Running the CLI

```bash
uv run pyai
```

You can also use:

```bash
uv run py
```

If you want to run directly from a source checkout:

```bash
uv run python -m pypimono
```

The CLI entry point is `pypimono/cli.py`, and the objects are assembled through the container graph rather than being manually wired in the CLI.

### LLM selection

Use `PI_LLM_PROVIDER` to choose the model path:

- `auto` (default): use Codex if Codex OAuth is available, otherwise fall back to MockLlm
- `codex` / `openai-codex`: force the Codex path
- `mockllm`: force the mock provider

Example:

```bash
PI_LLM_PROVIDER=codex uv run pyai
```

Settings are loaded centrally in `pypimono/settings.py`, and the provider selection logic lives in `pypimono/engine/infra/llm/factory.py`.

### Thinking level

Use `PI_THINKING_LEVEL` to control the reasoning effort:

- `xhigh` (default)
- `high`
- `medium`
- `low`
- `minimal`
- `none`

Example:

```bash
export PI_THINKING_LEVEL=xhigh
uv run pyai
```

If `PI_THINKING_LEVEL` is set, it takes precedence over `CODEX_REASONING_EFFORT`.

### Output styles

Use `PI_OUTPUT_STYLE` to select the runtime surface:

- `textual` (default): TUI with an input widget and scrollable chat log
- `rich`: rich-formatted console output
- `plain`: plain text console output
- `discord`: Discord bot-based chat interface

Depending on the style, you may need extras:

- `plain`: no extra dependency
- `rich`: `uv sync --extra rich`
- `textual`: `uv sync --extra textual`
- `discord`: `uv sync --extra discord`

If the required extra is missing, the runtime falls back to plain console output with a notice.

Examples:

```bash
PI_OUTPUT_STYLE=plain uv run pyai
```

```bash
PI_OUTPUT_STYLE=textual uv run pyai
```

Textual usage tips:

- scroll the log with the mouse wheel
- drag the log area to select text
- copy with `Ctrl+C`
- send with `Enter`
- insert a newline with `Shift+Enter`
- quit with `/exit` or `Ctrl+Q`

### Session storage

Session persistence is abstracted behind `SessionStoreGateway`. The default implementation stores JSONL files locally.

- `PI_SESSIONS_DIR` (default: `.sessions`)
- `PI_SESSION_ID` (default: `default`)

Example:

```bash
export PI_SESSIONS_DIR=.data/sessions
export PI_SESSION_ID=dev-run
uv run pyai
```

### System prompt template

The default system prompt template lives in `pypimono/prompts/default_system_prompt.md`.
It is rendered once when `AgentSession` starts rather than being rebuilt every turn.

If the current working directory contains `PROMPT.md`, that file takes precedence over the packaged default.

Template variables:

- `{{AVAILABLE_TOOLS}}`: the currently registered tool list
- `{{CURRENT_DATETIME}}`: the startup timestamp
- `{{CURRENT_WORKING_DIRECTORY}}`: the absolute path of the current working directory

### Built-in tools

The default tool set includes:

- `read`
- `write`
- `edit`
- `bash`
- `grep`
- `find`
- `ls`

## Codex OAuth integration

This project includes a Codex OAuth integration that talks to `chatgpt.com/backend-api`.

- `pypimono/engine/infra/llm/codex/*`
  - token refresh
  - auth file storage
  - Responses API SSE handling
- `pypimono/engine/infra/llm/codex/auth_cli.py`
  - login, status, and refresh commands

Default auth locations:

- `~/.codex/auth.json`
- or a custom path via `CODEX_AUTH_PATH`

### Reuse Codex CLI auth

```bash
codex login
export PI_LLM_PROVIDER=codex
uv run pyai
```

### Log in through the Python module

```bash
uv run python -m pypimono.engine.infra.llm.codex.auth_cli login
export PI_LLM_PROVIDER=codex
uv run pyai
```

Check auth status:

```bash
uv run python -m pypimono.engine.infra.llm.codex.auth_cli status
```

Force a refresh:

```bash
uv run python -m pypimono.engine.infra.llm.codex.auth_cli refresh
```

### Relevant Codex environment variables

- `CODEX_MODEL` (default: `gpt-5.4`)
- `CODEX_AUTH_PATH` (default: `~/.codex/auth.json`)
- `CODEX_BASE_URL` (default: `https://chatgpt.com/backend-api`)
- `CODEX_ORIGINATOR` (default: `pi`)
- `CODEX_TEXT_VERBOSITY` (default: `medium`)
- `CODEX_REASONING_EFFORT` (optional)
