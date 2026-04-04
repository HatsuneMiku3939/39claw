# py-pimono

![py-pimono](./image.png)

Language: [English](README.md) | [한국어](README.kr.md)

`py-pimono`는 [`pi-mono`](https://github.com/mariozechner/pi-mono)의 아이디어를 Python으로 다시 구현한 로컬 코딩 에이전트입니다.

이 프로젝트의 목적은 두 가지입니다.

1. 복잡해 보이는 코드 에이전트를 `engine`, `session`, `ui`라는 3개의 분리된 헥사곤으로 쪼개서, 학습자가 LLM과 함께 에이전트 루프부터 UI까지의 흐름을 빠르게 파악할 수 있게 하는 것
2. 자신만의 에이전트를 만들고 싶을 때, 잘 분리된 Python 코드베이스에서 시작할 수 있게 하는 것

상세 구조와 디렉토리 지도, 호출 흐름, 설정 레퍼런스는 [ARCHITECTURE.kr.md](ARCHITECTURE.kr.md)에 분리해 두었습니다.

## Quickstart

### 프로젝트 sync 후 실행

기본 실행은 Codex 경로와 `textual` UI를 사용합니다. 따라서 Codex 인증이 준비된 환경이라면 바로 실행할 수 있습니다.

```bash
uv sync --extra ui
uv run pyai
```

Codex가 준비되지 않은 환경에서 동작만 먼저 확인하려면, 실행할 때 provider를 꺼서 `mockllm`으로 돌리면 됩니다.

```bash
PI_LLM_PROVIDER=mockllm uv run pyai
```

기본 `textual` 대신 plain 출력으로 실행하려면:

```bash
PI_OUTPUT_STYLE=plain uv run pyai
```

환경에 따라 `pyai` 명령이 바로 잡히지 않으면, 다음처럼 모듈을 직접 실행하면 됩니다:

```bash
python3 -m pypimono
```

위의 다른 예시들도 같은 방식으로 `pyai` 대신 `python3 -m pypimono`로 바꿔서 실행하면 됩니다.

### 선택 사항: Notion MCP 로그인

Notion hosted MCP 연동은 선택 사항입니다. 사용하려면 OAuth 로그인을 1회 완료하고 tool manifest를 동기화하면 됩니다.

```bash
uv run -m pypimono mcp notion login
```


디스코드 봇 인터페이스로 실행하려면:

```bash
uv sync --extra discord
PI_OUTPUT_STYLE=discord \
PI_DISCORD_BOT_TOKEN=<봇 토큰> \
PI_DISCORD_CHANNEL_ID=<선택: 채널 ID> \
uv run pyai
```

길드 채널에서는 `@봇이름 README 요약해줘`처럼 봇을 멘션하면 응답합니다. 1:1 DM에서는 그냥 질문만 보내면 됩니다.

> 참고: 이걸 실행한다고 “디스코드 서버(길드)”가 새로 켜지는 것은 아닙니다.  
> 로컬(또는 서버)에서 **봇 프로세스**가 실행되어, 기존 디스코드 서버/DM의 메시지를 받아 처리하는 방식입니다.

#### Discord 설정 가이드 (권한/DM/서버)

1. **Discord Developer Portal에서 봇 생성**
   - Application 생성 → Bot 탭에서 Bot 추가
   - Token 발급 후 `PI_DISCORD_BOT_TOKEN`에 설정

2. **Privileged Gateway Intents 설정**
   - Bot 탭에서 **Message Content Intent**를 켜세요.
   - 현재 구현은 길드 채널에서는 봇 멘션 뒤의 본문을 읽고, DM에서는 메시지 본문을 바로 읽습니다.

3. **봇 초대 링크(OAuth2 URL) 생성**
   - Scopes: `bot` (슬래시 커맨드를 쓸 계획이면 `applications.commands`도 추가)
   - Bot Permissions 권장:
     - `View Channels`
     - `Send Messages`
     - `Read Message History`
     - (선택) `Embed Links`, `Attach Files`

4. **어디서 반응할지 선택**
   - `PI_DISCORD_CHANNEL_ID`를 설정하면 길드에서는 **해당 채널에서만** 반응합니다(운영 권장).
   - DM은 채널 ID와 무관하게 동작합니다.

5. **DM(1:1) 가능 여부**
   - 가능합니다. 단, 사용자가 봇과 DM을 열 수 있는 상태여야 합니다(보통 같은 서버 공유 필요).
   - 봇은 일반적으로 먼저 임의 DM을 시작하지 못하므로, 사용자가 먼저 DM을 시작하는 흐름이 안전합니다.

6. **운영 팁**
   - 길드에서 오용 방지를 위해 `PI_DISCORD_CHANNEL_ID` 고정 + 별도 전용 채널 운영을 권장합니다.
   - 길드 채널에서는 봇을 멘션한 메시지에만 반응합니다.

### 로컬 개발

로컬에서 코드를 수정하면서 개발하려면 저장소를 clone한 뒤 `uv`로 개발 환경을 sync하는 편이 가장 단순합니다.

```bash
git clone https://github.com/solvit-team/py-pimono
cd py-pimono
uv sync --group dev
uv run pyai
```

## vs pi-ai

이 프로젝트는 “완성된 하나의 앱”보다는, 분리된 코어를 보여주는 학습용/출발점용 코드베이스에 가깝습니다.

### 왜 재구현했는가

- `pi-mono`의 철학과 에이전트 루프를 Python에서 이해하고, 포크하고, 다시 조립하기 쉽게 만드는 데 초점을 둡니다.
- 나는 Python 기반의 백엔드 소스코드를 자주 작성하는데, 도메인 로직 안에 AI agent를 넣어 보는 실험을 하려 해도 거대한 agent 저장소에서는 어떤 부분을 어떻게 녹여야 하는지 감이 잘 잡히지 않았습니다.
- 그래서 정교한 예외 처리나 완성형 기능 집합보다, 핵심 에이전트 루프의 논리가 어떻게 돌아가고 다른 구성 요소와 어떻게 상호작용하는지가 명확히 분리되어 보이는 재구현이 더 중요했습니다.

### pi-mono와 무엇이 다른가

- 나는 전반적으로 `pi-mono`의 철학과 방향성에 동의함에도 불구하고, [제작자 블로그](https://mariozechner.at/posts/2025-11-30-pi-coding-agent/)에 드러난 `pi-mono`의 의도와는 다른 선택을 한 부분이 있습니다.
- 예를 들어 멀티 프로바이더 컨텍스트 핸드오프 같은 범용성보다, 한 번에 하나의 모델 경로를 명확하게 운용하는 단순한 구조를 우선합니다. 세션 도중 모델을 자주 바꾸는 흐름은 KV 캐시 활용 측면에서도 손해라고 봤기 때문입니다. 실제로 저는 모델을 자주 교체하지 않고, 가장 좋은 모델로 처음부터 끝까지 운용합니다.
- 이 저장소는 스트리밍 출력, slash command 등을 지원하지 않습니다. 필요하면 단순성 위에 구현하세요.
- 완성형 앱보다는, 에이전트 코어, 세션 관리 방식, UI 표시 방식을 읽고, 이해하고, 수정하고, 복제하기 쉬운 학습용/출발점용 '진짜 미니멀' 코드베이스에 더 가깝습니다.
- 물론, 이 모든 것들이 없어도 이 코드베이스는 잘 돌아갑니다.

## 구조 개요

이 프로젝트의 코어는 3개의 헥사곤과 그 사이를 연결하는 adapter 계층으로 나뉩니다.

### Engine

에이전트 루프, LLM 호출, tool 실행

훅을 추가하거나, 도구를 추가하거나, 모델을 변경하거나, 에이전트의 코어 로직을 커스터마이즈하고 싶다면 주로 `pypimono/engine/*`만 건드리면 됩니다.

### Session

세션 저장/복원, 시스템 프롬프트 구성, turn orchestration

세션을 클라우드에 저장하거나, 여러 세션을 관리하거나, 복원 규칙을 더 정교하게 만들고 싶다면 `pypimono/session/*`만 보면 됩니다.

### UI

세션 이벤트를 화면에 보여줄 표현으로 바꾸고, 실제 콘솔/TUI로 렌더링

그래서 도구마다의 출력을 어떻게 예쁘게 표시할지, 어떤 부분을 보여주고 어떤 부분을 숨길지 등을 바꿀지, 다른 UI 표면으로 옮길지를 고민한다면 `pypimono/ui/*`만 건드리면 됩니다.

### 조립

헥사곤 간 조립은 `pypimono/integration`이, 전체 조립은 `AppContainer`가 맡습니다.

```text
pypimono.cli:main
  -> AppContainer
    -> EngineContainer
    -> SessionContainer
    -> UiContainer
    -> integration containers
```

더 자세한 설명은 [ARCHITECTURE.md](ARCHITECTURE.md)를 보면 됩니다.
