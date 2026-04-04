# Implement the Discord runtime, commands, and response presentation

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agents/PLANS.md`.

## Purpose / Big Picture

After this plan, `39claw` should expose the implemented application behavior through a real Discord runtime. A user in a Discord server should be able to mention the bot for normal conversation, invoke `/help` and `/task ...` slash commands, receive replies in the same channel, and get long responses chunked into readable Discord-safe messages. This is the plan that turns the internal application pipeline into an actual bot experience.

## Progress

- [x] (2026-04-04 15:27Z) Defined the Discord runtime plan and its acceptance targets.
- [x] (2026-04-05 02:12Z) Confirmed the repository provides the capabilities listed in `Starting State` by running `make test` and `make lint`.
- [x] (2026-04-05 03:42Z) Added the real Discord session runtime using a thin adapter and replaced the fake runtime shell in `cmd/39claw`.
- [x] (2026-04-05 03:42Z) Added mention filtering for normal-message handling and same-channel reply targeting.
- [x] (2026-04-05 03:42Z) Added `/help` and `/task ...` slash-command registration and routing, using `/task current` for the current-task action.
- [x] (2026-04-05 03:42Z) Added ephemeral task-command responses and Discord-safe response chunking that preserves fenced code blocks when practical.
- [x] (2026-04-05 03:42Z) Added runtime-level tests and a README smoke-test checklist.
- [ ] Run the manual smoke test against a disposable Discord server and record the result here.

## Surprises & Discoveries

- Observation: Discord-specific behavior should stay in a thin adapter because the core design explicitly keeps application logic independent from the Discord SDK.
  Evidence: `ARCHITECTURE.md`
- Observation: Discord slash commands are easier to keep explicit and stable when the “show current task” action is exposed as `/task current` alongside the other task subcommands.
  Evidence: `internal/runtime/discord/commands.go`
- Observation: Guild-scoped command overwrites are worth keeping as an optional configuration path because they make smoke-test retries immediate while preserving global registration as the default.
  Evidence: `internal/config/config.go`, `README.md`

## Decision Log

- Decision: Use `github.com/bwmarrin/discordgo` for the first runtime implementation unless implementation evidence forces a change.
  Rationale: It directly supports message handling, slash commands, replies, and interaction responses in Go while keeping the runtime surface thin.
  Date/Author: 2026-04-04 / Codex

- Decision: Keep chunking and presentation in the runtime adapter rather than the core app service.
  Rationale: Discord message-length limits are transport concerns, not core orchestration concerns.
  Date/Author: 2026-04-04 / Codex

- Decision: Expose the current-task action as `/task current`.
  Rationale: Discord slash-command structure is cleaner and more predictable when all task actions share the same explicit subcommand model.
  Date/Author: 2026-04-05 / Codex

- Decision: Add optional `CLAW_DISCORD_GUILD_ID` support for guild-scoped command registration during smoke tests.
  Rationale: Global command propagation is slower, while guild-scoped overwrites are immediate and safe to repeat.
  Date/Author: 2026-04-05 / Codex

## Outcomes & Retrospective

The outcome of this plan should be a bot that is observable from Discord, not just from unit tests. Success means the end-user workflow now matches the product docs closely enough for a real smoke test.

The runtime implementation is now in place and covered by focused tests. `cmd/39claw` boots a real Discord adapter, message mentions route through the app layer, slash commands are registered and presented correctly, task-control responses are ephemeral, and long output is chunked for Discord readability. The remaining gap is only the final real-server smoke test, which still requires disposable Discord credentials.

## Context and Orientation

The app layer should already know how to handle `daily` and `task` semantics; the runtime's responsibility is to translate Discord inputs into app requests and to translate app responses back into Discord messages.

In v1, normal conversation is mention-only. Unsupported non-mention chatter should be ignored. `/help` should explain the commands that are actually available for the configured mode. `/task ...` should return a not-available response when the bot instance is running in `daily` mode. Task-control responses should be ephemeral by default.

Long responses must be chunked into Discord-safe messages. When the content contains fenced code blocks, the chunker should preserve readable formatting when practical instead of splitting raw text arbitrarily.

## Starting State

Start this plan only after confirming the repository provides all of the following capabilities:

- a real startup path in `cmd/39claw`
- configuration loading and logger setup
- app-layer message handling for `daily` mode
- app-layer task command handling for `task` mode
- SQLite-backed thread and task persistence

Verify that state with:

    make test
    make lint

If one of those capabilities is missing, implement the missing application behavior before building the Discord adapter. This plan should not become the place where core routing logic is invented for the first time.

## Preconditions

This document is self-contained. The facts you need are repeated here:

- normal conversation is mention-only in v1
- `/help` is always an explicit slash command
- `/task ...` is the explicit task-control surface
- task-control responses are ephemeral by default
- transport-specific behavior such as chunking and Discord reply metadata belongs in the runtime adapter, not in the core app service

## Plan of Work

Create the real runtime under `internal/runtime/discord`. Add a small `Runtime` type that owns a `discordgo.Session`, registers handlers, and exposes `Start` and `Close` methods. Keep Discord-specific payload parsing in mapper files such as `message_mapper.go` and `interaction_mapper.go`.

Implement mention filtering for message-create events. The mapper should produce `internal/app.MessageRequest` values only for qualifying mention-triggered messages. Unsupported chatter should be ignored without noise.

Implement slash-command registration and routing for `/help` and `/task`. Map the command payloads into calls to the application services. In the implemented runtime, the task command family is exposed as `/task current`, `/task list`, `/task new`, `/task switch`, and `/task close` so the slash-command UI stays explicit and stable. When the configured mode is `daily`, `/task ...` should return a clear not-available response. When the configured mode is `task`, `/help` should describe the task workflow and task command success responses should be ephemeral.

Implement the presenter in files such as `presenter.go` and `chunker.go`. Normal conversation responses should reply to the triggering message in the same channel. Command responses should use interaction responses and set ephemeral flags where appropriate. Long content should be chunked under Discord limits while trying to preserve code fences and a readable message sequence.

Add runtime-oriented tests using fake session collaborators or a narrow wrapper around the Discord session. The tests should prove mention filtering, command mapping, ephemeral task-control behavior, and chunking logic. Finish with a manual smoke test described in `README.md`.

## Concrete Steps

Run all commands from `/home/filepang/playground/39claw`.

1. Confirm the repository matches the required starting state.

    make test
    make lint

2. Add the Discord runtime adapter and presenter.

3. Run focused tests while iterating.

    go test ./internal/runtime/discord -run 'TestMessage|TestCommand|TestChunk'

4. Run the full repository checks after the plan lands.

    make test
    make lint

5. Perform a manual smoke test with a disposable Discord server.

    CLAW_MODE=daily \
    CLAW_TIMEZONE=Asia/Tokyo \
    CLAW_DISCORD_TOKEN=... \
    CLAW_DISCORD_GUILD_ID=... \
    CLAW_CODEX_WORKDIR=/absolute/path/to/repo \
    CLAW_SQLITE_PATH=/tmp/39claw-dev.sqlite \
    CLAW_CODEX_EXECUTABLE=/absolute/path/to/codex \
    go run ./cmd/39claw

6. Record a short proof artifact for later contributors:

    go test ./internal/runtime/discord -run 'TestMessage|TestCommand|TestChunk' -v

## Validation and Acceptance

This plan is complete when:

- normal message handling is mention-only
- unsupported non-mention chatter is ignored
- normal conversation replies in the same channel and targets the triggering message as the reply root
- `/help` responds with commands appropriate to the configured mode
- `/task current`, `/task list`, `/task new`, `/task switch`, and `/task close` are available in `task` mode and clearly not available in `daily` mode
- task-control command responses are ephemeral by default
- long replies are chunked into Discord-safe messages while preserving code fences when practical
- `make test` passes
- `make lint` passes
- a manual smoke test in a real Discord server succeeds

At the end of this plan, the repository should no longer need a fake runtime shell. `cmd/39claw` should boot the real Discord adapter.

## Idempotence and Recovery

Command registration should be safe to repeat during development. If the runtime uses guild-scoped commands for smoke testing, document how to clean them up in `README.md` so retries do not leave stale command definitions behind.

If you open this plan and discover that the app layer is still missing one of the required behaviors, pause the runtime work and add the missing behavior first. The recovery rule is to keep Discord-specific code thin. Do not duplicate core business logic inside the runtime package.

## Artifacts and Notes

Useful smoke-test checklist:

    mention bot in daily mode -> receives same-channel reply
    send unrelated chatter without mention -> bot stays silent
    run /help in daily mode -> no task workflow advertised
    run /task current in daily mode -> clear not-available message
    switch to task mode and run /task new demo -> ephemeral success response

Proof artifact recorded during implementation:

    go test ./internal/runtime/discord -v
    --- PASS: TestRuntimeStartRegistersCommands
    --- PASS: TestRuntimeMentionHandlingRepliesToTriggerMessage
    --- PASS: TestRuntimeTaskCommandInDailyModeIsEphemeral
    --- PASS: TestRuntimeTaskCommandRoutesTaskModeSubcommands
    --- PASS: TestPresentInteractionChunksLongResponses
    --- PASS: TestChunkTextPreservesCodeFences

## Interfaces and Dependencies

The runtime should depend on app-layer surfaces shaped like these examples:

    type MessageService interface {
        HandleMessage(ctx context.Context, request MessageRequest) (MessageResponse, error)
    }

    type TaskCommandService interface {
        ShowCurrent(ctx context.Context, userID string) (MessageResponse, error)
        List(ctx context.Context, userID string) (MessageResponse, error)
        New(ctx context.Context, userID string, name string) (MessageResponse, error)
        Switch(ctx context.Context, userID string, taskID string) (MessageResponse, error)
        Close(ctx context.Context, userID string, taskID string) (MessageResponse, error)
    }

Keep `discordgo` imports inside `internal/runtime/discord` and `cmd/39claw` only.

Revision Note: 2026-04-04 / Codex - Created this smaller child ExecPlan during the split of the original all-in-one runtime plan.
Revision Note: 2026-04-04 / Codex - Removed the parent-plan dependency and added explicit starting-state and recovery guidance so the document can stand alone.
Revision Note: 2026-04-05 / Codex - Recorded the implemented runtime progress, the `/task current` command-shape decision, and the optional guild-scoped command-registration path for faster smoke tests.
