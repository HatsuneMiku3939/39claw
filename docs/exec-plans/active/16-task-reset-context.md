# Add task-scoped context reset without recreating the worktree

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agents/PLANS.md`.

## Purpose / Big Picture

After this plan, a `task`-mode user should be able to run `/<instance-command> action:task-reset-context` to keep the current active task and its existing worktree exactly as they are while intentionally discarding only the saved Codex conversation continuity for that task. The next normal message for that same active task should then start a fresh Codex thread inside the same task worktree.

This change matters because `task` mode deliberately couples long-lived work to both a durable task identity and an isolated Git worktree, but users do not always want to preserve the entire remote conversation history forever. Sometimes the repository state is correct and should be kept, while the Codex thread has become noisy, overloaded, or no longer reflects the desired discussion. The user-visible proof is simple: run the reset action on an idle active task, send the next normal task message, and observe that the task and worktree remain unchanged while the Codex thread ID is recreated from scratch.

## Progress

- [x] (2026-04-11 02:32Z) Reviewed issue `#80`, the current task-mode routing and command surface, the existing queue behavior, and the prior `daily` clear-generation ExecPlan; wrote this initial active ExecPlan.
- [ ] Update architecture, design, and product documents so the repository explicitly distinguishes task identity continuity, task workspace continuity, and Codex thread continuity.
- [ ] Add a persistence primitive that can remove a saved thread binding for one logical task key without touching task metadata or worktree state.
- [ ] Implement `action:task-reset-context` in the app layer and root-command routing, including explicit idle-success and busy-rejection responses.
- [ ] Add store, app, and runtime tests that prove the reset path, no-op fresh-state behavior, busy rejection, and next-message fresh-thread behavior.
- [ ] Run repository validation with `make test` and `make lint`, or the repository-equivalent direct commands if `make` is unavailable in the execution environment, and record the result in this plan.

## Surprises & Discoveries

- Observation: The architecture and mode-design documents already imply that `task` mode needs a way to “clear or close the active task”, but the concrete product docs and command surface still expose only `task-current`, `task-list`, `task-new`, `task-switch`, and `task-close`.
  Evidence: `ARCHITECTURE.md`, `docs/design-docs/thread-modes.md`, `docs/design-docs/implementation-spec.md`, `docs/product-specs/task-mode-user-flow.md`, `docs/product-specs/discord-command-behavior.md`

- Observation: The queue coordinator already exposes a lightweight `Snapshot` method, so the busy-reset guard does not need a second queue or a new coordinator type.
  Evidence: `internal/thread/queue.go`, `internal/app/message_service.go`

- Observation: The current thread-binding persistence contract has `GetThreadBinding` and `UpsertThreadBinding`, but no delete or clear operation. Resetting task context therefore needs an explicit storage primitive instead of a purely command-layer change.
  Evidence: `internal/app/message_service.go`, `internal/store/sqlite/store.go`

- Observation: Queued task work captures the logical key at admission time but loads the thread binding only when execution actually starts. If reset were allowed while queued work exists, the queued turn could become the first turn on the fresh thread before the user's post-reset message arrives.
  Evidence: `internal/app/message_service_impl.go`

- Observation: The current task execution path already creates a fresh remote thread automatically whenever no binding exists for the task logical key. That means deleting the saved binding is enough to trigger a clean restart without changing the task record or worktree path.
  Evidence: `internal/app/message_service_impl.go`

## Decision Log

- Decision: Expose the feature as `/<instance-command> action:task-reset-context` and scope it to the current active task only.
  Rationale: The issue is specifically about preserving the active task identity while resetting its conversation continuity. Accepting `task_name` or `task_id` selectors would broaden the surface unnecessarily and would make it easier to reset the wrong task.
  Date/Author: 2026-04-11 / Codex

- Decision: Implement reset by deleting the `task`-mode thread-binding row for the active task logical key instead of mutating the task record, the active-task pointer, or the worktree metadata.
  Rationale: The existing normal-message path already interprets “no binding for this logical key” as “start a fresh remote thread and persist the returned thread ID.” Reusing that path is the smallest coherent change and keeps `thread_bindings` as the single source of truth for saved Codex thread IDs.
  Date/Author: 2026-04-11 / Codex

- Decision: Reject reset whenever the queue snapshot for the active task key reports either `InFlight` or `Queued > 0`.
  Rationale: In-flight work could still deliver an old-thread reply after the user believes the reset succeeded, and queued work could become the first turn on the fresh thread before the user's next visible message. Rejecting both states keeps ordering and user expectations clear.
  Date/Author: 2026-04-11 / Codex

- Decision: Treat “no saved binding exists yet” as a successful no-op with explicit user guidance instead of returning an error.
  Rationale: If the active task has never produced a saved Codex thread ID, the user intent is already satisfied: the next normal message will start fresh in the same workspace. Returning a user-facing failure for that case would create needless friction.
  Date/Author: 2026-04-11 / Codex

## Outcomes & Retrospective

This plan is not implemented yet. At plan-creation time, the repository already has the needed task identity, worktree, queue, and thread-binding foundations, but it does not yet have a task-specific context-reset action or a binding-deletion primitive. The main goal of this plan is to land that feature without widening it into destructive task closure, workspace recreation, or branch reset behavior.

## Context and Orientation

39claw is a thin Discord-to-Codex gateway. In `task` mode, one user-scoped task maps to three related but distinct pieces of state:

1. A persisted task record in SQLite, including fields such as `task_id`, `task_name`, `branch_name`, `worktree_path`, and `worktree_status`.
2. An active-task pointer for the requesting Discord user, which decides which task normal messages target.
3. A thread-binding row that maps the logical task key `discord_user_id + ":" + task_id` to a saved Codex thread ID.

The current task execution path is spread across these files:

- `internal/thread/policy.go`
  - resolves the active task into the logical key `user_id:task_id`
- `internal/app/message_service_impl.go`
  - admits per-key execution into the queue, prepares the task worktree, loads the binding for the logical key, calls Codex, and persists the returned thread ID
- `internal/app/task_service.go`
  - owns the current task-control actions and user-facing task command responses
- `internal/app/message_service.go`
  - defines the store and queue interfaces consumed by the app layer
- `internal/store/sqlite/store.go`
  - implements thread-binding and task persistence
- `internal/runtime/discord/interaction_mapper.go`
  - defines the root-command action constants and extracts command requests from Discord interactions
- `internal/runtime/discord/commands.go`
  - defines the root-command choices, help text, and unsupported-action guidance
- `internal/runtime/discord/runtime.go`
  - routes the parsed root-command action to the correct app-layer service
- `cmd/39claw/main.go`
  - wires the queue coordinator, store, message service, and task command service together

For this plan, these terms are important:

A “task logical key” is the string `discord_user_id + ":" + task_id`. It is the durable routing key for queued work and thread bindings in `task` mode.

A “task workspace continuity” means the task keeps the same reserved branch metadata and the same worktree path under `${CLAW_DATADIR}/worktrees/<task_id>` when one already exists.

A “Codex thread continuity” means later turns resume the same saved remote thread ID instead of starting a fresh one.

A “task context reset” in this plan means keeping task identity and workspace continuity unchanged while removing only the saved Codex thread continuity for the active task.

The issue is not asking for task closure, branch reset, worktree deletion, or checkout recreation. Those behaviors must stay unchanged.

## Starting State

Start this plan only after confirming the repository still matches all of the following facts:

- `task` normal-message routing still resolves the active task key through `internal/thread/policy.go`.
- `internal/app/message_service_impl.go` still loads a thread binding just before calling Codex and automatically starts a fresh thread whenever the binding is missing.
- `internal/app/task_service.go` still owns the task command responses and does not yet define a reset-context action.
- `internal/runtime/discord/commands.go` still exposes only `help`, `task-current`, `task-list`, `task-new`, `task-switch`, and `task-close` in `task` mode.
- `internal/thread/queue.go` still provides `Snapshot` so command paths can detect busy or queued work.
- `internal/store/sqlite/store.go` still has no delete primitive for thread bindings.

Verify that state from the repository root with:

    make test
    make lint

If `make` is unavailable in the execution environment, run:

    go test ./...
    ./scripts/lint -c .golangci.yml

If the repository has drifted away from this starting state, update this ExecPlan first so it remains self-contained and truthful.

## Preconditions

This plan fixes the following behavior choices:

- the reset action is `action:task-reset-context`, not an overloaded reuse of the `daily`-mode `action:clear`
- the action operates on the currently active task only
- the action must not recreate, delete, or modify the task worktree or branch
- the action must not change which task is active
- the next normal message after a successful reset must use the same task worktree but no saved Codex thread ID
- the action must fail safely while that task has in-flight or queued work
- if the active task has no saved binding yet, the command returns a no-op success that still explains the next normal message will start fresh

## Milestone 1: Define the task-reset contract in repository docs

At the end of this milestone, the repository documentation should clearly explain that `task` mode now has a dedicated way to discard only Codex conversation continuity while leaving the task record and worktree intact.

Update these documents:

- `ARCHITECTURE.md`
- `docs/design-docs/thread-modes.md`
- `docs/design-docs/implementation-spec.md`
- `docs/design-docs/task-mode-worktrees.md`
- `docs/product-specs/task-mode-user-flow.md`
- `docs/product-specs/discord-command-behavior.md`

Describe the difference between:

- task identity continuity
- task workspace continuity
- Codex thread continuity

Add the new root-command action to the task-mode command lists. Document these user-visible rules:

- `/<instance-command> action:task-reset-context` keeps the current task active
- it does not recreate or delete the worktree
- it only drops saved Codex conversation continuity
- it is rejected while the current task has in-flight or queued work
- the next normal message starts a fresh Codex thread in the same workspace

Also update `docs/exec-plans/index.md` so this plan appears under `Current Active Plans`.

## Milestone 2: Add a binding-deletion primitive and task command orchestration

At the end of this milestone, the app layer should be able to reset the active task context without touching task metadata or workspace state.

Extend `internal/app/message_service.go` so the `ThreadStore` interface can delete one thread binding by `(mode, logical_thread_key)`. Use a generic method such as:

    DeleteThreadBinding(ctx context.Context, mode string, logicalThreadKey string) error

Implement that method in:

- `internal/store/sqlite/store.go`
- every in-memory or fake store used by `internal/app` and `internal/runtime/discord` tests
- the relevant store test files under `internal/store/sqlite`

The delete method must be idempotent. Deleting a missing row should not be treated as an application error.

Then extend the task command layer:

- add `ResetContext(ctx context.Context, userID string) (MessageResponse, error)` to `TaskCommandService`
- update `TaskCommandServiceDependencies` to accept the `QueueCoordinator`
- update `cmd/39claw/main.go` so the live task command service receives that coordinator
- add any small helper needed inside `internal/app` to build the task logical key from `userID` and `taskID` without creating an import cycle with `internal/thread`

Implement the reset algorithm in `internal/app/task_service.go`:

1. Load the active task for the requesting user.
2. If no active task exists, return the existing actionable task guidance.
3. Load the task details for the active task ID so the response can still name the task.
4. Build the task logical key from `userID` and `taskID`.
5. Read `coordinator.Snapshot(buildExecutionKey(config.ModeTask, logicalKey))`.
6. If the snapshot reports in-flight or queued work, return an ephemeral retry-later response that explains why reset is unsafe.
7. Delete the `task`-mode thread binding for that logical key.
8. Return an explicit success response that says the task remains active, the workspace is unchanged, and the next normal message will start a fresh Codex thread.

When step 7 finds no existing binding, return a no-op success response rather than a failure. The message should still explain that the next normal message will start fresh for the same task.

## Milestone 3: Wire `action:task-reset-context` through the root command surface

At the end of this milestone, task-mode Discord instances should expose the new action in the same root command used for the existing task workflow.

Update:

- `internal/runtime/discord/interaction_mapper.go`
- `internal/runtime/discord/commands.go`
- `internal/runtime/discord/runtime.go`
- `internal/runtime/discord/runtime_test_helpers_test.go`
- any runtime contract tests or helpers that fake `TaskCommandService`

Add the new string constant:

    actionTaskResetContext = "task-reset-context"

Then:

- include it in the task-mode action choices returned by `registeredCommands`
- add it to `helpResponse`
- update `unsupportedActionText` so the new action is part of the supported surface
- route the action from `Runtime.routeCommand` to `TaskCommandService.ResetContext`
- keep `daily` mode behavior unchanged so this action remains unavailable there

The root-command surface must stay explicit and stable. Do not introduce a second slash command or overload `action:clear` in `task` mode.

## Milestone 4: Prove reset, no-op, and busy-guard behavior with tests

At the end of this milestone, the repository should have focused automated proof that reset preserves task identity and workspace continuity while dropping only saved Codex thread continuity.

Add or update tests in:

- `internal/app/task_service_test.go`
- `internal/app/message_service_test.go`
- `internal/store/sqlite/store_test.go`
- `internal/runtime/discord/runtime_test.go`
- `internal/runtime/discord/interaction_mapper_test.go`
- any relevant runtime contract or helper tests

Cover these cases explicitly:

- idle active task with an existing binding: reset succeeds and deletes the binding
- idle active task with no binding yet: reset returns a no-op success
- active task with an in-flight turn: reset is rejected
- active task with queued work: reset is rejected
- no active task: reset returns the same actionable guidance used elsewhere in task mode
- task-mode command registration and help output include `task-reset-context`
- `daily`-mode command registration does not include `task-reset-context`
- after a successful reset, the next normal task message uses the same task worktree path but calls the Codex gateway with an empty incoming thread ID and then persists the new returned thread ID

For the message-service proof, reuse the existing fake gateway and task workspace helpers. The key assertion is that worktree continuity stays intact while thread continuity restarts.

## Plan of Work

First, update the docs so the repository explicitly states the intended user-facing contract before the code changes land. Keep the explanation narrow: this feature is only a task-scoped conversation reset, not workspace destruction or task closure.

Second, add the storage primitive to delete a single thread-binding row and update all in-memory test doubles to support it. This should not require a schema migration because the `thread_bindings` table already exists and only the CRUD surface is changing.

Third, implement the new task command in `internal/app/task_service.go`. Keep the success and rejection text explicit. The command must explain that the task remains active and that only Codex conversation continuity changed.

Fourth, wire the new action through the Discord root command. The runtime should stay thin by passing the request straight to the app-layer task command service.

Finally, add focused store, app, and runtime tests before running the full repository checks. The most important behavioral proof is that the next normal task message after reset still uses the same worktree path but no longer resumes the old remote thread ID.

## Concrete Steps

Run all commands from `/home/filepang/.local/share/39claw/39claw/worktrees/01KNX5YCGY27MKM3N16W5ES58V`.

1. Confirm the repository matches the documented starting state.

    make test
    make lint

    If `make` is unavailable:

        go test ./...
        ./scripts/lint -c .golangci.yml

    Expected result:

        all Go tests pass
        lint passes with 0 issues

2. Update the task-reset contract documents.

    Edit:

    - `ARCHITECTURE.md`
    - `docs/design-docs/thread-modes.md`
    - `docs/design-docs/implementation-spec.md`
    - `docs/design-docs/task-mode-worktrees.md`
    - `docs/product-specs/task-mode-user-flow.md`
    - `docs/product-specs/discord-command-behavior.md`
    - `docs/exec-plans/index.md`

3. Add the persistence and app-layer reset support.

    Edit:

    - `internal/app/message_service.go`
    - `internal/app/task_service.go`
    - `internal/store/sqlite/store.go`
    - `cmd/39claw/main.go`

    Also update the test doubles that satisfy `ThreadStore` or `TaskCommandService`.

4. Add or update focused tests while iterating.

    Suggested focused commands:

        go test ./internal/store/sqlite -run 'TestStore(DeleteThreadBinding|ThreadBinding)'
        go test ./internal/app -run 'Test(TaskCommandServiceResetContext|MessageServiceHandleMessageTaskStartsFreshThreadAfterReset)'
        go test ./internal/runtime/discord -run 'Test(RuntimeTaskResetContext|RegisteredCommandsTaskModeIncludesResetContext|RegisteredCommandsDailyModeOmitsTaskResetContext)'

    Expected result:

        each focused package passes with the new reset-context coverage

5. Run the full repository checks after implementation.

    make test
    make lint

    If `make` is unavailable:

        go test ./...
        ./scripts/lint -c .golangci.yml

6. Capture a short proof transcript in this plan once implementation lands.

    Example scenario to record:

        /<instance-command> action:task-reset-context
        -> task remains active, workspace unchanged, next message starts fresh

        normal task message after reset
        -> gateway receives empty incoming thread id
        -> returned thread id is persisted for the same logical task key

## Validation and Acceptance

This plan is complete when all of the following are true:

- `task`-mode root commands expose `action:task-reset-context`
- invoking `action:task-reset-context` does not change the active task selection
- invoking `action:task-reset-context` does not recreate, delete, or modify the task worktree or branch
- invoking `action:task-reset-context` deletes or otherwise clears only the saved Codex thread continuity for that active task
- invoking the action while that task has in-flight or queued work returns a user-facing rejection
- invoking the action on an idle task with no saved binding returns a clear no-op success
- the next normal message after a successful reset starts a fresh Codex thread in the same task worktree
- `daily` mode does not expose the new action
- `make test` passes
- `make lint` passes

Human verification should follow this exact story:

1. Create or select a task that already has a saved thread binding and a ready worktree.
2. Invoke `/<instance-command> action:task-reset-context`.
3. Observe an ephemeral success response that says the task is still active and the workspace is unchanged.
4. Send a normal message for that task.
5. Observe that the gateway starts with no incoming thread ID while still using the same task worktree path.
6. Observe that the returned new thread ID is persisted under the same logical task key.

Also verify the safety story:

1. Start a long-running task turn.
2. While it is still in flight, invoke `action:task-reset-context`.
3. Observe an ephemeral rejection that tells the user to retry after pending replies finish.

Repeat that safety check with one queued waiting turn and confirm the same rejection.

## Idempotence and Recovery

Deleting a thread binding for a logical task key must be safe to repeat. If the process crashes after deleting the binding but before replying to Discord, rerunning the same command should either delete nothing and return the no-op success or delete a newly recreated binding intentionally. That is acceptable because the desired end state is still “no saved Codex thread continuity for this task.”

Do not add any destructive Git behavior to this plan. If a draft implementation begins modifying task worktrees, branch refs, or task records beyond the saved thread binding, stop and update this plan first because the work has drifted outside the approved scope.

If queue handling becomes more invasive than expected, keep the feature contract and update the plan rather than weakening the busy guard. The safety promise is part of the user-facing behavior, not an optional polish item.

## Artifacts and Notes

Keep these user-facing response targets visible during implementation.

Suggested success response:

    Reset Codex conversation continuity for active task `<task_name>` (`<task_id>`). The task is still active and the workspace is unchanged. Your next normal message will start a fresh Codex thread for this task.

Suggested no-op success response:

    Active task `<task_name>` (`<task_id>`) does not have saved Codex conversation continuity yet. The task is still active, the workspace is unchanged, and your next normal message will already start fresh.

Suggested busy rejection response:

    This task still has running or queued work. Wait for pending replies to finish, then retry `/<instance-command> action:task-reset-context`.

Keep this mental model explicit:

    task record stays
    active-task pointer stays
    worktree path stays
    thread binding goes away
    next normal task message creates a new remote thread

## Interfaces and Dependencies

Define or update the following application-facing contracts.

In `internal/app/message_service.go`, the store interface should include:

    type ThreadStore interface {
        GetThreadBinding(ctx context.Context, mode string, logicalThreadKey string) (ThreadBinding, bool, error)
        UpsertThreadBinding(ctx context.Context, binding ThreadBinding) error
        DeleteThreadBinding(ctx context.Context, mode string, logicalThreadKey string) error
        ...
    }

In `internal/app/task_service.go`, the task command interface should include:

    type TaskCommandService interface {
        ShowCurrentTask(ctx context.Context, userID string) (MessageResponse, error)
        ListTasks(ctx context.Context, userID string) (MessageResponse, error)
        CreateTask(ctx context.Context, userID string, taskName string) (MessageResponse, error)
        SwitchTask(ctx context.Context, userID string, taskID string, taskName string) (MessageResponse, error)
        CloseTask(ctx context.Context, userID string, taskID string, taskName string) (MessageResponse, error)
        ResetContext(ctx context.Context, userID string) (MessageResponse, error)
    }

In `internal/runtime/discord/interaction_mapper.go`, define:

    const actionTaskResetContext = "task-reset-context"

In `internal/runtime/discord/runtime.go`, extend the command router so:

    case actionTaskResetContext:
        return r.taskCommand.ResetContext(ctx, request.UserID)

Keep the existing dependencies:

- `QueueCoordinator` remains the source of busy or queued status
- `ThreadStore` remains the source of truth for saved Codex thread IDs
- `TaskWorkspaceManager` remains unchanged by this feature because reset must not mutate worktree state

Revision Note: 2026-04-11 02:32Z / Codex - Created this active ExecPlan for issue `#80` after reviewing the current task-mode routing, queue, command, and persistence architecture and choosing the smallest coherent design: delete the saved task thread binding while preserving active-task and worktree state.
