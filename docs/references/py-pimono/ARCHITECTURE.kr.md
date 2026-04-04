# Architecture

## 왜 이렇게 쪼갰는가

`py-pimono`의 핵심 목표는 “코드 에이전트가 사실은 몇 개의 명확한 책임으로 분해될 수 있다”는 점을 드러내는 것입니다.

이 프로젝트는 코어를 다음 3개의 헥사곤으로 나눕니다.

- `engine`: 에이전트가 생각하고, LLM을 부르고, tool을 실행하는 코어
- `session`: turn과 히스토리를 관리하고, 저장/복원과 시스템 프롬프트를 담당하는 코어
- `ui`: 세션 이벤트를 화면 친화적인 표현으로 바꾸고, 실제 콘솔/TUI에 렌더링하는 코어

그리고 각 코어 사이에는 `integration` adapter가 있습니다.

이렇게 나누면 학습자나 포크하는 사람이 “어디부터 뜯어야 하는지”가 분명해집니다.

- 에이전트 루프를 바꾸고 싶다 -> `engine`
- 세션 저장 방식을 바꾸고 싶다 -> `session`
- UI를 다른 표면으로 바꾸고 싶다 -> `ui`
- 경계를 넘는 타입 변환이 궁금하다 -> `integration`

## 전체 조립

실행 진입점은 `pypimono/cli.py`이고, 실제 조립 루트는 `pypimono/container.py`입니다.

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

핵심은 각 코어가 서로를 직접 new 하지 않고, container와 adapter를 통해 연결된다는 점입니다.

## 어디를 보면 되는가

### Engine이 궁금하면

`engine`은 에이전트의 코어 로직만 담당합니다.

- 대표 파일:
  - `pypimono/engine/application/agent.py`
  - `pypimono/engine/domain/agent_loop.py`
  - `pypimono/engine/domain/messages.py`
  - `pypimono/engine/infra/llm/*`
  - `pypimono/engine/infra/tool/*`
  - `pypimono/engine/infra/mcp/*`

이 레이어를 보면 알 수 있는 것:

- 메시지가 어떻게 turn으로 흘러가는지
- LLM이 어떤 형식으로 연결되는지
- tool 호출과 tool result가 어떻게 붙는지
- remote MCP tool이 어떻게 인증되고 일반 tool처럼 노출되는지

그래서 에이전트 루프를 추가하거나, tool을 다듬거나, 특정 모델 provider를 바꾸고 싶다면 대부분 `pypimono/engine/*`만 건드리면 됩니다.

### Session이 궁금하면

`session`은 에이전트 바깥의 실행 문맥을 담당합니다.

- 대표 파일:
  - `pypimono/session/application/agent_session.py`
  - `pypimono/session/application/session_manager.py`
  - `pypimono/session/application/system_prompt_builder.py`
  - `pypimono/session/infra/store/*`
  - `pypimono/session/domain/*`

이 레이어를 보면 알 수 있는 것:

- 세션을 어디에 어떻게 저장하는지
- 이전 히스토리를 어떻게 복원하는지
- 시스템 프롬프트를 언제 어떻게 조립하는지
- UI 쪽에 어떤 이벤트를 내보내는지

그래서 세션을 클라우드에 저장하고, 여러 세션을 관리하고, 복원 규칙을 더 정교하게 만들고 싶다면 대부분 `pypimono/session/*`만 건드리면 됩니다.

### UI가 궁금하면

`ui`는 session이 만든 사건을 “사람이 보는 화면”으로 바꾸는 층입니다.

- 대표 파일:
  - `pypimono/ui/application/chat_ui.py`
  - `pypimono/ui/application/presentation/*`
  - `pypimono/ui/infra/runtime/*`
  - `pypimono/ui/infra/sinks/*`
  - `pypimono/ui/infra/tui/*`

이 레이어를 보면 알 수 있는 것:

- 세션 이벤트를 어떤 view model로 해석하는지
- plain/rich/textual 출력이 어떻게 갈리는지
- 입력 루프와 렌더링이 어떻게 동작하는지

그래서 이벤트를 어떻게 보여줄지 바꾸거나, TUI를 뜯어고치거나, 다른 인터페이스(예: web UI, desktop UI)로 옮기고 싶다면 대부분 `pypimono/ui/*`만 건드리면 됩니다.

### Integration이 궁금하면

`integration`은 코어끼리 직접 의존하지 않게 막는 adapter 계층입니다.

- `pypimono/integration/engine_session/*`
  - engine의 `Agent`를 session의 `AgentRuntimeGateway`로 감쌉니다.
- `pypimono/integration/session_ui/*`
  - session의 `SessionPort`를 ui의 `SessionGateway`로 감쌉니다.

그래서 “헥사곤 사이에 타입 누출이 없는지”, “어디서 ACL이 서는지”가 궁금하면 `integration/*`를 보면 됩니다.

## 호출 흐름

### 사용자 입력이 engine까지 가는 흐름

```text
ui/infra/runtime
  -> ui.ChatUi
  -> integration.session_ui.SessionUiGatewayAdapter
  -> session.AgentSession
  -> integration.engine_session.EngineAgentRuntimeAdapter
  -> engine.Agent
```

### 엔진 이벤트가 화면까지 올라오는 흐름

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

이 흐름 덕분에 `engine`은 UI를 모르고, `ui`는 engine 내부 타입을 몰라도 됩니다.

## 디렉토리 지도

### engine

- `pypimono/engine/domain/*`
  - 엔진의 핵심 모델과 규칙
  - `agent_event.py`, `messages.py`, `agent_loop.py`, `ports/*`
- `pypimono/engine/application/*`
  - 엔진 유스케이스
  - 중심은 `pypimono/engine/application/agent.py`
- `pypimono/engine/infra/llm/*`
  - LLM 포트 구현
  - Codex/Mock 포함
- `pypimono/engine/infra/mcp/*`
  - remote MCP OAuth, 토큰 저장, manifest sync, remote tool adapter 구현
- `pypimono/engine/infra/tool/*`
  - `read`, `write`, `edit`, `bash`, `grep`, `find`, `ls` 구현
- `pypimono/engine/infra/workspace_fs/*`
  - 로컬 파일시스템 어댑터

### session

- `pypimono/session/domain/*`
  - 세션 히스토리, 엔트리, 메시지 같은 내부 모델
- `pypimono/session/boundary/contracts/*`
  - session이 바깥에 공개하는 계약 타입
- `pypimono/session/boundary/mappers/*`
  - session domain <-> boundary 계약 변환
- `pypimono/session/application/*`
  - 세션 유스케이스와 orchestration
- `pypimono/session/application/ports/*`
  - `SessionPort`, `AgentRuntimeGateway`, store/event sink 계약
- `pypimono/session/infra/store/*`
  - JSONL 세션 저장 구현

### ui

- `pypimono/ui/boundary/contracts/*`
  - UI가 session 바깥에서 받는 계약 타입
- `pypimono/ui/application/*`
  - UI 유스케이스
  - 중심은 `pypimono/ui/application/chat_ui.py`
- `pypimono/ui/application/ports/*`
  - `UiPort`, `SessionGateway`, sink 포트
- `pypimono/ui/application/presentation/*`
  - session 이벤트를 UI 표시용 view model로 바꾸는 계층
- `pypimono/ui/infra/runtime/*`
  - console/textual runtime
- `pypimono/ui/infra/sinks/*`
  - plain/rich/textual 출력 구현
- `pypimono/ui/infra/tui/*`
  - Textual 프레임워크 의존 코드

### integration

- `pypimono/integration/engine_session/*`
- `pypimono/integration/session_ui/*`

## 처음 읽는 순서

처음 파악할 때는 아래 순서가 가장 빠릅니다.

1. `pypimono/cli.py`
2. `pypimono/container.py`
3. 대표 유스케이스
   - engine: `pypimono/engine/application/agent.py`
   - session: `pypimono/session/application/agent_session.py`
   - ui: `pypimono/ui/application/chat_ui.py`
4. 경계를 넘는 부분
   - `pypimono/integration/engine_session/runtime_adapter.py`
   - `pypimono/integration/session_ui/session_gateway_adapter.py`
5. 화면 표시가 궁금하면
   - `pypimono/ui/application/presentation/*`
   - `pypimono/ui/infra/sinks/*`
6. 구현 세부가 궁금하면
   - `pypimono/engine/infra/*`
   - `pypimono/session/infra/*`
   - `pypimono/ui/infra/*`

## 실행과 설정

### 설치

기본 설치:

```bash
uv sync
```

Notion hosted MCP 연동이 필요하면 auth/manifest를 한 번 준비하면 됩니다.

```bash
uv run -m pypimono mcp notion login
```

`rich` 출력이 필요하면:

```bash
uv sync --extra rich
```

`textual` TUI가 필요하면:

```bash
uv sync --extra textual
```

둘 다 필요하면:

```bash
uv sync --extra ui
```

로컬 개발 도구와 UI 의존성까지 한 번에 맞추려면:

```bash
uv sync --group dev
```

현재 활성화된 가상환경에 직접 sync하려면:

```bash
uv sync --group dev --active
```

### 실행

```bash
uv run pyai
```

또는

```bash
uv run py
```

소스 체크아웃 상태에서 직접 실행하려면:

```bash
uv run python -m pypimono
```

CLI 진입점은 `pypimono/cli.py`이고, 직접 객체를 조립하지 않고 container에서 resolve해서 실행합니다.

### LLM 선택

`PI_LLM_PROVIDER` 값으로 동작 모드를 고를 수 있습니다.

- `auto` (기본): Codex OAuth 가능하면 Codex, 아니면 MockLlm
- `codex` / `openai-codex`: Codex OAuth 사용
- `mockllm`: MockLlm 강제 사용

예시:

```bash
PI_LLM_PROVIDER=codex uv run pyai
```

환경 변수 로딩은 `pypimono/settings.py`에서 중앙 관리합니다.
LLM 선택/생성 로직은 `pypimono/engine/infra/llm/factory.py`에서 처리합니다.

### Thinking level

`PI_THINKING_LEVEL`로 모델 추론 강도를 제어할 수 있습니다.

- `xhigh` (기본)
- `high`
- `medium`
- `low`
- `minimal`
- `none`

예시:

```bash
export PI_THINKING_LEVEL=xhigh
uv run pyai
```

`PI_THINKING_LEVEL`이 설정되어 있으면 `CODEX_REASONING_EFFORT`보다 우선합니다.

### CLI 출력 스타일

`PI_OUTPUT_STYLE` 값으로 콘솔 출력 스타일을 고를 수 있습니다.

- `textual` (기본): 입력 위젯 + 채팅 로그 기반 TUI
- `rich`: 패널/색상 기반 출력
- `plain`: 단순 텍스트 출력
- `discord`: 디스코드 봇 기반 채팅 인터페이스

선택한 출력 스타일에 따라 extras가 필요합니다.

- `plain`: 추가 설치 없음
- `rich`: `uv sync --extra rich`
- `textual`: `uv sync --extra textual`
- `discord`: `uv sync --extra discord`

필수 extras가 없으면 런타임에서 안내 후 plain 쪽으로 fallback합니다.

예시:

```bash
PI_OUTPUT_STYLE=plain uv run pyai
```

```bash
PI_OUTPUT_STYLE=textual uv run pyai
```

Textual 사용 팁:

- 마우스 휠로 채팅 로그 스크롤
- 로그 영역 드래그로 텍스트 선택, `Ctrl+C`로 복사
- 입력: `Enter` 전송, `Shift+Enter` 줄바꿈
- 종료: `/exit` 또는 `Ctrl+Q`

### 세션 저장

세션 저장은 `SessionStoreGateway`를 통해 추상화되어 있으며, 기본 구현은 JSONL 파일 어댑터입니다.

- `PI_SESSIONS_DIR` (기본: `.sessions`)
- `PI_SESSION_ID` (기본: `default`)

예시:

```bash
export PI_SESSIONS_DIR=.data/sessions
export PI_SESSION_ID=dev-run
uv run pyai
```

### 시스템 프롬프트 템플릿

기본 시스템 프롬프트 템플릿은 `pypimono/prompts/default_system_prompt.md`를 사용합니다.
`AgentSession` 시작 시 한 번 렌더링하며, 이후 turn마다 재빌드하지 않습니다.

현재 작업 디렉터리에 `PROMPT.md`가 있으면 그 파일이 기본 템플릿보다 우선합니다.

- `{{AVAILABLE_TOOLS}}`: 현재 등록된 툴 목록
- `{{CURRENT_DATETIME}}`: 시작 시각
- `{{CURRENT_WORKING_DIRECTORY}}`: 현재 작업 디렉터리 절대경로

### 기본 도구

현재 기본으로 등록되는 도구:

- `read`
- `write`
- `edit`
- `bash`
- `grep`
- `find`
- `ls`

## Codex OAuth

이 프로젝트는 `chatgpt.com/backend-api` 경로를 사용하는 Codex OAuth 연동을 구현합니다.

- `pypimono/engine/infra/llm/codex/*`: OAuth 토큰 갱신, auth 저장소, responses SSE 호출
- `pypimono/engine/infra/llm/codex/auth_cli.py`: OAuth 로그인/상태/리프레시 CLI

기본 인증 파일 경로:

- `~/.codex/auth.json`
- 또는 `CODEX_AUTH_PATH`로 커스텀 경로 지정

### Codex CLI auth 재사용

```bash
codex login
export PI_LLM_PROVIDER=codex
uv run pyai
```

### Python 모듈로 직접 로그인

```bash
uv run python -m pypimono.engine.infra.llm.codex.auth_cli login
export PI_LLM_PROVIDER=codex
uv run pyai
```

상태 확인:

```bash
uv run python -m pypimono.engine.infra.llm.codex.auth_cli status
```

강제 리프레시:

```bash
uv run python -m pypimono.engine.infra.llm.codex.auth_cli refresh
```

### Codex 관련 환경 변수

- `CODEX_MODEL` (기본: `gpt-5.4`)
- `CODEX_AUTH_PATH` (기본: `~/.codex/auth.json`)
- `CODEX_BASE_URL` (기본: `https://chatgpt.com/backend-api`)
- `CODEX_ORIGINATOR` (기본: `pi`)
- `CODEX_TEXT_VERBOSITY` (기본: `medium`)
- `CODEX_REASONING_EFFORT` (선택)
