# Implement capped per-thread message queueing for busy Codex turns

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agents/PLANS.md`.

## Purpose / Big Picture

After this plan, a user who sends a normal message while the same logical conversation is already running should no longer be told to retry manually. Instead, 39claw should immediately acknowledge that the message was accepted into a queue, then post the real answer after the earlier turn finishes. The queue must be small and explicit: each logical thread key gets at most five waiting messages in memory, and once that limit is reached the bot should fall back to a clear retry-later response.

This change is visible in Discord. In `daily` mode, if two people mention the bot on the same configured local date, the second message should receive a queued acknowledgment and later receive the actual answer as a follow-up reply to the original message. In `task` mode, the same behavior should apply per `user + task_id` context without allowing later task switches to reroute already-queued work.

## Progress

- [x] (2026-04-04 19:22Z) Synced the worktree to `origin/master`, created `feature/task-queue-busy-handling`, and wrote the first complete ExecPlan draft.
- [ ] Replace busy rejection with capped queue admission at the logical-thread layer.
- [ ] Refactor the app message orchestration so queued work can produce a deferred reply after the initial acknowledgment has already been sent.
- [ ] Update Discord runtime handling so it can present both immediate acknowledgments and later queued-turn completions.
- [ ] Add focused tests for queue admission, queue overflow, deferred follow-up delivery, and task-context freezing.
- [ ] Update `ARCHITECTURE.md`, design docs, product specs, and `README.md` to describe the new queueing behavior and its in-memory limitations.
- [ ] Run `make test` and `make lint`, then record proof artifacts in this plan.

## Surprises & Discoveries

- Observation: The current `internal/app.MessageService` contract is purely synchronous. It returns one `MessageResponse` and assumes that the same call lifecycle both performs Codex work and produces the final reply.
  Evidence: `internal/app/message_service.go`, `internal/runtime/discord/runtime.go`

- Observation: Replacing busy rejection with queueing is not only a thread-guard change. The runtime needs a transport-neutral way for the app layer to deliver a later response after the initial acknowledgment is already posted.
  Evidence: `internal/app/message_service_impl.go`, `internal/runtime/discord/presenter.go`

- Observation: `task` mode currently consults the active task twice: once while resolving the logical key and again just before persisting the binding. That is safe for immediate execution, but it would be wrong for queued work because the active task may change before the queued item runs.
  Evidence: `internal/thread/policy.go`, `internal/app/message_service_impl.go`

## Decision Log

- Decision: Implement queueing as an in-memory per-logical-thread queue with a maximum of five waiting items per key, not a durable SQLite-backed queue.
  Rationale: The user explicitly chose the lightweight in-memory path. This keeps the change small enough for the current architecture while still improving the Discord experience for overlapping turns.
  Date/Author: 2026-04-04 / Codex

- Decision: Treat the queue limit of five as “five waiting items in addition to the currently running turn.”
  Rationale: The user requested a maximum queue length of five. Counting only waiting items makes the limit match the user-facing concept of queue length rather than total in-flight work.
  Date/Author: 2026-04-04 / Codex

- Decision: Freeze routing context at enqueue time.
  Rationale: A queued message must execute against the logical thread key that existed when the user sent it. In `task` mode that means capturing the active task ID immediately so later `/task switch` commands do not silently reroute queued work.
  Date/Author: 2026-04-04 / Codex

- Decision: Keep queue ownership in the application layer and let the Discord runtime provide only a deferred-delivery callback.
  Rationale: Queue admission rules depend on logical thread resolution, which is application behavior rather than Discord transport behavior. The runtime should remain a thin adapter that presents immediate and deferred responses.
  Date/Author: 2026-04-04 / Codex

## Outcomes & Retrospective

This plan is not implemented yet. The intended outcome is a bot that feels patient instead of brittle during overlapping requests while still keeping concurrency boundaries explicit and bounded.

The main architectural cost is that normal-message handling can no longer be modeled as a single synchronous request/response call. The implementation must preserve the current thin-runtime boundary while adding a safe deferred-reply path for queued turns.

## Context and Orientation

The current normal-message flow lives in `internal/app/message_service_impl.go`. A qualifying mention resolves a logical thread key, attempts to acquire `internal/thread/guard.go`, runs Codex immediately through `internal/codex/gateway.go`, persists the returned thread binding, and returns one `MessageResponse`. If another message arrives for the same logical key while the first turn is still running, the app returns the busy text instead of queueing.

The Discord adapter in `internal/runtime/discord/runtime.go` is intentionally thin. It maps Discord events into `internal/app.MessageRequest`, calls `MessageService.HandleMessage`, and immediately presents the returned `MessageResponse` with `presentMessage` in `internal/runtime/discord/presenter.go`. That means there is currently no path for “ack now, answer later” behavior.

In this repository, a “logical thread key” is the stable internal identity for a conversation bucket. In `daily` mode it is the configured local date such as `2026-04-05`. In `task` mode it is `userID + ":" + taskID`. A “queued turn” in this plan means a normal message that has been accepted for later execution because another turn for the same logical key is already running. The queue is intentionally in memory only, which means queued items are lost if the bot process exits before they run.

## Starting State

Start this plan only after confirming the repository still provides all of the following:

- `cmd/39claw` starts the real Discord runtime
- `internal/app` contains the real normal-message and task-command services
- `internal/thread` currently serializes same-key work through the busy guard
- `internal/runtime/discord` can present normal replies and chunk long responses
- `make test` and `make lint` pass on the current branch

Verify that state with:

    cd /home/filepang/.codex/worktrees/dcb6/39claw
    make test
    make lint

If the repository has drifted so far that those assumptions are no longer true, repair the missing foundation first and then update this plan so it remains truthful.

## Preconditions

This plan assumes the following facts, repeated here so the reader does not need another ExecPlan:

- normal message handling is mention-only
- normal message routing is app-owned, not Discord-owned
- `daily` mode uses a date-based logical thread key
- `task` mode uses a user-and-task-based logical thread key
- overlapping turns for different logical keys may run concurrently
- the new queue is intentionally not durable across restart

## Plan of Work

Begin by replacing the current busy guard with a queue coordinator in `internal/thread`. The queue coordinator should own a map keyed by `mode + ":" + logicalKey`, just like the existing guard key shape. Each entry should track whether a turn is currently running and a FIFO slice of waiting work items. It should expose small, explicit operations for:

- admitting the first turn for immediate execution
- admitting a later turn into the waiting queue when the key is already running
- rejecting a later turn when five waiting items already exist
- marking a completed turn and selecting the next queued item, if any

Do not make this coordinator know about Discord or Codex. It should only know about queue state and sequencing.

Next, refactor `internal/app/message_service_impl.go` so the message service separates “prepare the work” from “execute the work.” Introduce a preparation step that resolves mention handling, missing-task guidance, the logical thread key, and any frozen task metadata before queue admission. The frozen task metadata must include the active `task_id` when the bot is running in `task` mode. This preparation result should be the single source of truth used later by queued execution, so the worker path never re-reads the current active task and accidentally routes to the wrong task after a switch.

Add a transport-neutral deferred delivery interface to `internal/app/message_service.go`. The message service must still return an immediate `MessageResponse`, but when a message is queued it must also remember how to publish the final response later. A minimal interface such as:

    type DeferredReplySink interface {
        Deliver(ctx context.Context, response MessageResponse) error
    }

is enough because the runtime can bind channel ID and original message ID into the sink closure before passing it to the app layer. Keep the app layer unaware of Discord SDK types.

Refactor the message service execution path into two phases. For the first admitted turn on an idle key, keep the current user-visible behavior: execute Codex immediately and return the final response in the same call. For a queued turn, return a new acknowledgment response immediately, for example “A response is already running for this conversation. Your message has been queued.” Then, after the currently running turn completes, the message service should process the next queued item in FIFO order and use that item’s `DeferredReplySink` to publish the actual answer.

Implement the queue drain carefully. When a turn completes, the service should ask the queue coordinator for the next waiting item and start draining queued items sequentially until the queue is empty. The drain loop may run in a goroutine owned by the application service, but it must never process two items for the same key at once. Different logical keys should still be free to run in parallel.

Update user-facing text in `internal/app/message_service_impl.go` or a nearby constants file. There are now three expected states for overlapping normal messages:

- immediate success because the key was idle
- queued acknowledgment because the key was busy but the waiting queue had room
- queue-full rejection because five waiting items were already queued

Choose text that stays user-facing and concise. Mention queueing explicitly. If a queue position is included, keep it human-readable and stable.

Update `internal/runtime/discord/runtime.go` so message-create handling passes a deferred sink to the app service. The sink implementation should call `presentMessage` against the same channel and original message ID that triggered the queued request, producing a later reply rooted to the correct Discord message. If deferred delivery fails because the runtime is shutting down or the Discord API call fails, log the error with `slog` and drop that queued reply. This plan does not add durable retries.

Update `internal/runtime/discord/presenter.go` only if needed to make deferred presentation share the existing chunking and reply logic. The goal is to reuse the same `presentMessage` path for both immediate and deferred responses so Discord formatting stays consistent.

Finally, update the documentation set. `ARCHITECTURE.md` should describe capped queueing in the request flow and concurrency model. `docs/design-docs/implementation-spec.md` must replace the current “busy or retry response rather than queueing implicitly” language with the new capped queue behavior. `docs/product-specs/daily-mode-user-flow.md`, `docs/product-specs/task-mode-user-flow.md`, and `docs/product-specs/discord-command-behavior.md` should explain the queued-acknowledgment behavior in user terms. `README.md` should replace “busy-thread rejection” with queueing language and mention that queued items are lost on restart because the queue is in memory only.

## Concrete Steps

Run all commands from `/home/filepang/.codex/worktrees/dcb6/39claw`.

1. Confirm the repository still matches the stated starting state.

    make test
    make lint

2. Add the queue coordinator and its tests.

    go test ./internal/thread -run 'TestPolicy|TestQueue'

3. Refactor the app message service to support frozen routing context, queue admission, deferred delivery, and queue-full rejection.

    go test ./internal/app -run 'TestMessageService'

4. Update the Discord runtime tests so they prove immediate queued acknowledgment followed by later deferred delivery.

    go test ./internal/runtime/discord -run 'TestRuntime|TestPresent'

5. Run the full repository checks after the feature lands.

    make test
    make lint

6. Record proof artifacts in this plan once the implementation is complete. Include at least one focused app test transcript and one runtime test transcript that show queued acknowledgment plus later follow-up delivery.

## Validation and Acceptance

This plan is complete when all of the following are true:

- if a logical thread key is idle, a qualifying normal mention still behaves as before and returns the final answer immediately
- if a second qualifying mention arrives while the same logical thread key is running, the bot immediately replies with a queued acknowledgment instead of a retry-later error
- after the running turn completes, the queued message receives its actual answer as a later reply to the original Discord message
- queued messages for the same key are processed in FIFO order
- once five waiting messages already exist for the key, the next message is rejected with a queue-full retry response
- different logical thread keys can still make progress independently
- in `task` mode, a queued message continues against the task that was active when the message was accepted, even if the user switches tasks before the queued message runs
- in `daily` mode, a queued message continues against the date bucket that was resolved from the message timestamp at acceptance time
- `make test` passes
- `make lint` passes

The most important human-visible proof is this Discord scenario:

1. Send a long-running mention that keeps Codex busy.
2. Send a second mention in the same logical conversation.
3. Observe an immediate “queued” acknowledgment.
4. Wait for the first turn to finish.
5. Observe the real second answer arrive later as a reply to the second message.

## Idempotence and Recovery

This plan is safe to iterate on because the queue is in memory only. Restarting the process clears the queue state. That is acceptable for this plan, but it must be documented clearly so operators are not surprised.

If a contributor partially implements the queue and then discovers deferred delivery is broken, the safe recovery path is to keep the old busy rejection until both the queue coordinator and the deferred reply sink path are working together. Do not merge a half-state where queued messages are acknowledged but never answered.

If shutdown behavior becomes flaky, prefer dropping queued in-memory work during shutdown rather than blocking process exit indefinitely. This repository does not yet need graceful drain persistence.

## Artifacts and Notes

Target user-facing examples:

    idle key:
    user mention -> final answer immediately

    busy key with room in queue:
    user mention -> "A response is already running for this conversation. Your message has been queued."
    later -> final answer posted as a reply to that same message

    busy key with five waiting items already queued:
    user mention -> "This conversation already has five queued messages. Please retry in a moment."

Useful implementation notes:

    `daily` queue key:
    daily:2026-04-05

    `task` queue key:
    task:user-1:task-7

    frozen task metadata example:
    request received while active task is task-7
    user switches to task-8 before queued execution starts
    queued request must still run against task:user-1:task-7

## Interfaces and Dependencies

Preserve the existing architectural boundary that keeps `discordgo` imports inside `internal/runtime/discord` and `cmd/39claw`.

At the end of this plan, the repository should have app-facing interfaces shaped like the following or equivalent stable names:

    type DeferredReplySink interface {
        Deliver(ctx context.Context, response MessageResponse) error
    }

    type MessageService interface {
        HandleMessage(ctx context.Context, request MessageRequest, sink DeferredReplySink) (MessageResponse, error)
    }

    type PreparedMessageTarget struct {
        LogicalKey string
        TaskID     string
    }

    type QueueAdmission struct {
        ExecuteNow bool
        Queued     bool
        Position   int
    }

The exact type names may differ, but the final design must preserve these semantics:

- queue state is keyed by logical thread identity
- the app layer can freeze routing context before queue admission
- the runtime provides a deferred-reply sink without leaking Discord SDK types into the app package
- the queue coordinator can admit, reject, and advance work deterministically

Tests to add or revise must cover these repository areas:

- `internal/thread/policy_test.go` or a new queue test file for queue admission, release, and overflow behavior
- `internal/app/message_service_test.go` for queued acknowledgment, FIFO drain, queue-full rejection, and frozen task-context routing
- `internal/runtime/discord/runtime_test.go` for immediate acknowledgment followed by deferred delivery through the fake session
- `README.md` smoke-test notes for manual queue verification

Revision Note: 2026-04-04 / Codex - Created this ExecPlan after syncing the worktree to `origin/master` and branching `feature/task-queue-busy-handling` to replace busy-thread rejection with capped in-memory queueing.
