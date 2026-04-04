# Implement `daily` mode routing and persistence

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agents/PLANS.md`.

## Purpose / Big Picture

After this plan, a user should be able to mention the bot in `daily` mode and get same-day continuity automatically. The first qualifying mention of the local day should create a new Codex thread binding, later mentions on the same configured local date should reuse it, and the first qualifying mention on the next local date should create a fresh binding. This should be provable through store-backed tests and application-level orchestration tests without requiring a live Discord server.

## Progress

- [x] (2026-04-04 15:27Z) Defined the `daily` mode plan and its acceptance targets.
- [x] (2026-04-05 16:00Z) Confirmed the repository provides the foundation capabilities listed in `Starting State` by rerunning `make test` and `make lint`.
- [x] (2026-04-05 16:17Z) Implemented the `daily` logical thread key policy based on the configured timezone, including a local-midnight rollover test.
- [x] (2026-04-05 16:17Z) Implemented thread-binding load and upsert behavior for `daily` mode, including a SQLite reopen persistence test.
- [x] (2026-04-05 16:17Z) Implemented the message orchestration path that creates or resumes Codex threads in `daily` mode.
- [x] (2026-04-05 16:17Z) Added the per-logical-thread busy guard so overlapping turns are rejected instead of queued.
- [x] (2026-04-05 16:17Z) Added tests for ignored chatter, same-day reuse, next-day rollover, busy-thread rejection, and missing-task guidance.

## Surprises & Discoveries

- Observation: `daily` mode is the smallest complete user-facing slice because it does not require task commands or explicit task selection.
  Evidence: `docs/design-docs/implementation-spec.md`

- Observation: The first persisted Codex thread ID is created by the first successful turn, not by a separate empty-thread API call.
  Evidence: `internal/codex/gateway.go`, `internal/app/message_service_impl.go`

- Observation: Keeping busy-thread rejection and missing-task guidance inside the application layer makes the daily workflow fully testable without a Discord runtime.
  Evidence: `go test ./internal/thread ./internal/app ./internal/store/sqlite -run 'TestMessageService|TestPolicy|TestGuard|TestStoreThreadBinding' -v`

## Decision Log

- Decision: Deliver `daily` mode before `task` mode.
  Rationale: It is the first end-to-end behavior in the implementation-spec delivery order and exercises the storage and gateway seams with less state complexity.
  Date/Author: 2026-04-04 / Codex

- Decision: Treat the configured timezone as the only source of truth for daily bucket calculation.
  Rationale: The product specs describe date boundaries in terms of the bot instance's configured local timezone.
  Date/Author: 2026-04-04 / Codex

- Decision: Use application-layer sentinel errors for `no active task` and `execution already in progress`.
  Rationale: The message service needs to translate these states into user-facing responses without importing the thread package and creating a package cycle.
  Date/Author: 2026-04-05 / Codex

- Decision: Persist the returned thread ID after every successful turn instead of only on first creation.
  Rationale: The current gateway contract returns the authoritative thread ID for both new and resumed turns, so persisting the latest value keeps the binding path simple and idempotent.
  Date/Author: 2026-04-05 / Codex

## Outcomes & Retrospective

This plan now produces the first real user-facing feature in the repository at the application layer. The app can route mention-triggered `daily` conversation correctly, persist same-day continuity over restart, reject overlapping turns for the same logical key, and ignore unsupported chatter. The remaining work for later plans is to connect this behavior to the real Discord runtime and presenter.

## Context and Orientation

The relevant user-facing behavior comes from `docs/product-specs/daily-mode-user-flow.md` and `docs/product-specs/discord-command-behavior.md`.

In `daily` mode, the logical thread key is the configured local date formatted as `YYYY-MM-DD`. That key is not exposed to end users, but it is the stable internal identity for the current day's shared conversation. A "qualifying normal message" means a mention-triggered message that the bot is expected to handle. Unsupported non-mention chatter must still be ignored.

The repository must persist the mapping between the `daily` logical key and the Codex thread ID in SQLite. If the process restarts, the app must be able to load the same binding and continue the same remote thread on the same day.

## Starting State

Start this plan only after confirming the repository provides all of the following capabilities:

- `cmd/39claw` has a real startup path instead of a greeting stub
- `internal/config` can load mode and timezone configuration
- `internal/app` exposes message request and response contracts
- `internal/thread` exposes a thread-policy seam and an execution guard seam
- `internal/store/sqlite` can create the schema and upsert thread bindings
- `internal/codex` exposes an application-friendly gateway wrapper

Verify that state with:

    make test
    make lint

If one or more of those capabilities is missing, stop and implement the missing foundation work first inside the same branch. Do not work around a missing seam by introducing a second temporary abstraction in this plan.

## Preconditions

This plan assumes only the repository state listed above. It does not require the reader to open another ExecPlan. The important repository facts are repeated here:

- normal conversation is mention-only in v1
- the `daily` logical key is the configured local date formatted as `YYYY-MM-DD`
- SQLite stores the mapping from logical thread key to Codex thread ID
- overlapping turns for the same logical key must be rejected instead of queued

## Plan of Work

Implement the `daily` key resolver in `internal/thread/policy.go` or a mode-specific helper file such as `internal/thread/daily_policy.go`. The resolver should accept a timestamp and a loaded timezone and return the local date in `YYYY-MM-DD` format.

Implement the application-layer normal-message service for `daily` mode in `internal/app/message_service.go`. It should reject unsupported non-mention chatter by returning a response that signals "ignore". For a qualifying mention, it should resolve the logical key, consult the thread store, create a new Codex thread when no binding exists, or resume an existing thread when a binding is present. It should then call the Codex gateway, persist the returned thread ID, and return a normalized response for later presentation.

Wire the per-logical-thread guard into the message service. If a second request arrives while the same logical key is already in flight, the service should return a busy or retry response instead of waiting in an internal queue. Keep this behavior in the application layer so the later Discord runtime only needs to present the returned message.

Add tests in `internal/thread`, `internal/app`, and `internal/store/sqlite`. Use a fake Codex gateway in app tests so they can assert thread creation versus thread reuse without talking to the real Codex CLI. Include restart-oriented tests that reopen the SQLite store and confirm that the previously written thread binding is still available.

If the repository does not yet have a concrete message-service implementation, create it here. Do not postpone that work on the assumption that another document covers it. This plan is responsible for the first complete end-to-end behavior for normal messages.

## Concrete Steps

Run all commands from `/home/filepang/playground/39claw`.

1. Confirm the repository matches the required starting state.

    make test
    make lint

2. Implement the `daily` key policy and message orchestration.

3. Run focused tests while iterating.

    go test ./internal/thread ./internal/app ./internal/store/sqlite -run 'TestDaily|TestThreadBinding|TestBusy'

4. Run the full repository checks after the plan lands.

    make test
    make lint

5. Record a short proof artifact for the next contributor:

    go test ./internal/thread ./internal/app ./internal/store/sqlite -run 'TestDaily|TestThreadBinding|TestBusy' -v

Completed proof artifact:

    go test ./internal/thread ./internal/app ./internal/store/sqlite -run 'TestMessageService|TestPolicy|TestGuard|TestStoreThreadBinding' -v

## Validation and Acceptance

This plan is complete when:

- in `daily` mode, the first qualifying mention creates a new thread binding
- a second same-day qualifying mention reuses the existing thread binding
- the first qualifying mention on the next configured local date creates a new logical binding
- a restarted process can still load the same-day binding from SQLite
- unsupported non-mention chatter is ignored
- overlapping turns for the same logical thread key are rejected with a busy or retry response
- `make test` passes
- `make lint` passes

The next plan should be able to assume these repository facts:

- a concrete message-service path exists for normal mentions
- `daily` routing behavior is tested without a live Discord server
- thread-binding persistence survives store reopen

## Idempotence and Recovery

The store may be reopened many times during testing. Keep test fixtures isolated so reopening the same SQLite file is safe. If a daily-policy bug is discovered after persistence tests are written, fix the policy and rerun the app-level tests rather than deleting the store code and rewriting it.

If you open this plan and discover that the repository does not satisfy the starting-state checklist, repair the missing foundation item first and update `Progress` to reflect that detour. The plan remains valid as long as the document tells the truth about the current state.

## Artifacts and Notes

Useful test scenarios to keep visible in this plan:

    first mention at 2026-04-05T09:00:00+09:00 -> key 2026-04-05
    second mention at 2026-04-05T16:00:00+09:00 -> key 2026-04-05
    first mention at 2026-04-06T00:01:00+09:00 -> key 2026-04-06

## Interfaces and Dependencies

This plan should build on seams shaped like these examples:

    type ThreadStore interface {
        FindThreadBinding(ctx context.Context, mode string, logicalKey string) (ThreadBinding, bool, error)
        UpsertThreadBinding(ctx context.Context, binding ThreadBinding) error
    }

    type CodexGateway interface {
        StartOrResumeTurn(ctx context.Context, threadID string, prompt string) (RunTurnResult, error)
    }

Keep the Discord runtime out of scope here. The app tests should speak in request and response structs, not in Discord SDK payloads.

Revision Note: 2026-04-04 / Codex - Created this smaller child ExecPlan during the split of the original all-in-one runtime plan.
Revision Note: 2026-04-04 / Codex - Removed the parent-plan dependency and added explicit starting-state and recovery guidance so the document can stand alone.
Revision Note: 2026-04-05 / Codex - Recorded the completed application-layer daily routing implementation, updated proof commands, and captured the thread-ID persistence nuance.
Revision Note: 2026-04-05 / Codex - Moved this fully completed ExecPlan from `active/` to `completed/` during ExecPlan index cleanup.
