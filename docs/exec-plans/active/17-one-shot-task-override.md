# Add one-shot task override routing for normal task-mode messages

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agents/PLANS.md`.

## Purpose / Big Picture

After this plan, a `task`-mode user should be able to send a normal message such as `task:release-bot-v1 fix the failing tests` or `task:docs-cleanup` followed by a newline and body text, and have only that one message routed to the named task. The saved active task must remain unchanged. Queue admission, saved thread binding lookup, task worktree selection, and the final Codex turn must all use the overridden task for that one message.

This change matters because the current task workflow makes users switch the active task even when they only want to send one out-of-band follow-up to another task. The visible proof is simple: create two tasks, keep `docs-cleanup` active, send one override message for `release-bot-v1`, and observe that the reply explicitly confirms `release-bot-v1` handled the turn while `/<instance-command> action:task-current` still reports `docs-cleanup`.

## Progress

- [x] (2026-04-11 06:58Z) Reviewed issue `#85`, the merged documentation-first PR `#86`, the current message mapper, task routing policy, queue admission path, task command service, and SQLite store helpers; wrote this initial active ExecPlan.
- [x] (2026-04-11 07:35Z) Added reusable task-name validation and one-shot override parsing helpers in `internal/app`, plus a closed-task-name lookup in the SQLite store and test doubles.
- [x] (2026-04-11 07:47Z) Routed task-mode normal messages through an effective task selected from either the active task or a one-shot `task:<name>` prefix, stripping the prefix before the Codex turn and preserving the override target through queue admission, worktree selection, and thread binding reuse.
- [x] (2026-04-11 07:55Z) Tightened task creation to require slug-style names for new tasks, rejected duplicate open task names, and updated task help text plus override-specific acknowledgment and rejection responses.
- [x] (2026-04-11 08:03Z) Added focused helper, policy, message-service, and runtime contract tests for override success and rejection paths; verified targeted packages with `go test ./internal/app ./internal/thread ./internal/runtime/discord ./internal/store/sqlite ./internal/dailymemory`.

## Surprises & Discoveries

- Observation: The repository docs now describe slug-style task names and one-shot overrides, but the code still treats `task_name` as a trimmed free-form label and still derives a branch slug with fallback to `task_id`.
  Evidence: `internal/app/task_service.go`, `internal/app/task_branch.go`

- Observation: The Discord message mapper already strips bot mentions and trims the remaining message body before the app layer sees it, which means a parser that only checks the first token of the normalized content can still satisfy the “mention before `task:<name>`” product rule in guild channels.
  Evidence: `internal/runtime/discord/message_mapper.go`

- Observation: Queue admission already freezes the logical thread key before queued work executes. If the override changes the logical key before `QueueCoordinator.Admit`, the existing queue implementation will automatically preserve the intended task for queued turns.
  Evidence: `internal/app/message_service_impl.go`, `internal/app/message_service_test.go` (`TestMessageServiceHandleMessageFreezesTaskContextForQueuedWork`)

- Observation: The SQLite migration runner only applies SQL files from `migrations/sqlite/`. It does not have a built-in hook for running Go-based data-rewrite logic during migration, which makes silent backfill of arbitrary legacy free-form task names an awkward fit for this feature.
  Evidence: `internal/store/sqlite/migrate.go`

- Observation: Queue freezing did not require new coordinator behavior. Once `prepareMessage` resolves the overridden logical key before admission, the existing queued execution path already keeps using the chosen task even if the saved active task still points elsewhere.
  Evidence: `internal/app/message_service_impl.go`, `internal/app/message_service_test.go` (`TestMessageServiceHandleMessageQueuesTaskOverrideAgainstSelectedTask`)

## Decision Log

- Decision: Parse one-shot override syntax in the app-layer message preparation path and carry the selected task name through `app.MessageRequest`, instead of teaching the Discord runtime to resolve tasks directly.
  Rationale: The app layer already owns message qualification results, queue admission, routing-key selection, and user-facing rejection responses. Keeping the parser there avoids pushing task-routing policy into the Discord adapter.
  Date/Author: 2026-04-11 / Codex

- Decision: Enforce the new slug contract for newly created tasks in `TaskCommandService.CreateTask`, and treat `task_id` as a compatibility-only selector for legacy pre-slug tasks that may already exist in SQLite.
  Rationale: The feature requires deterministic new routing targets, but the current migration system is SQL-only and the existing repository already has tests and persistence code that assume older free-form names. Compatibility through `task_id` keeps old tasks operable without making the new contract ambiguous.
  Date/Author: 2026-04-11 / Codex

- Decision: Keep the schema migration scope minimal for this feature and add exact-name lookup helpers in the store layer instead of introducing a new backfilled routing-name column.
  Rationale: The user-visible behavior can be delivered by validating new writes and resolving overrides against exact open-task names. Adding a second persisted routing identifier would broaden the change substantially and would require more documentation churn than the issue calls for.
  Date/Author: 2026-04-11 / Codex

- Decision: Confirm override-driven turns in visible reply text and in queued acknowledgments, but keep the confirmation as a lightweight prefix rather than a larger structured wrapper.
  Rationale: The product requirement is immediate, readable proof of which task handled the one-shot turn. A short prefix satisfies that proof requirement without reformatting the rest of the Codex response or changing downstream reply handling.
  Date/Author: 2026-04-11 / Codex

## Outcomes & Retrospective

Implementation is complete on the working branch. The app layer now parses `task:<name>` at message preparation time, carries the override through `MessageRequest.TaskOverrideName`, resolves exact open-task matches in the routing policy, and reuses the resulting logical key for queue admission, worktree preparation, and thread binding persistence. Queued override turns now acknowledge the selected task by name, and final override responses prepend a short confirmation line before the Codex answer.

The compatibility boundary stayed intentionally narrow. New tasks must use the documented immutable slug contract and duplicate open names are rejected, while older non-slug tasks remain reachable through `task_id` for command workflows. This avoided a data migration while still making one-shot routing deterministic for new tasks.

Remaining work outside this plan is only repository-level validation and PR review. Before merge, run the standard `make test` and `make lint` commands from the repository root and include the results in the implementation PR.

## Context and Orientation

39claw is a thin Discord-to-Codex gateway. In `task` mode, a normal Discord message currently takes this path:

1. `internal/runtime/discord/message_mapper.go` strips bot mentions, trims the remaining message text, and builds `app.MessageRequest`.
2. `internal/app/message_service_impl.go` asks `internal/thread/policy.go` for a logical task key, admits that key into the in-memory queue, and later loads the saved thread binding plus task worktree before calling Codex.
3. `internal/store/sqlite/store.go` persists task rows, active-task state, and thread bindings.
4. `internal/app/task_service.go` owns the slash-command workflow for `task-new`, `task-switch`, `task-close`, and `task-reset-context`.

The terms used in this plan are specific:

A “saved active task” is the row in `active_tasks` for one Discord user. It is the default task for future normal messages when no override is present.

An “effective task” is the task that one specific normal message should use. Today it is always the saved active task. After this feature it may be either the saved active task or a one-shot override target.

A “one-shot override” is a `task:<name>` prefix that appears at the first meaningful token of a task-mode normal message. It changes only the effective task for that one message. It does not mutate the saved active task.

A “task slug” is the new task-name contract documented in PR `#86`: lowercase ASCII, letters or digits plus single interior hyphens, no spaces, starts with a letter, ends with a letter or digit, length 3 through 32, and immutable after creation.

The current code is not ready for this feature yet:

- `internal/thread/policy.go` only knows how to route task-mode messages through the saved active task. It has no concept of override syntax or exact-name task lookup.
- `internal/app/task_service.go` trims task names but does not validate the new slug contract and does not prevent duplicate open-task names.
- `internal/app/task_branch.go` still slugifies arbitrary task names and falls back to `task_id`.
- `internal/store/sqlite/store.go` can fetch tasks by `task_id` and list all open tasks, but it cannot resolve an exact open task by name or report whether a matching closed task exists.
- `internal/runtime/discord/commands.go` and task-command messages still describe ambiguous-name fallback instead of the new slug-first contract.

The repository already contains useful tests that should remain green while this feature is added:

- `internal/app/message_service_test.go`
  - covers queue admission, frozen logical keys, task-bound routing, reset-context behavior, and attachment forwarding
- `internal/app/task_service_test.go`
  - covers create, switch, close, and reset-context command behavior
- `internal/thread/policy_test.go`
  - covers active-task routing behavior and queue snapshots
- `internal/runtime/discord/message_mapper_test.go`
  - covers mention stripping and DM mapping
- `internal/runtime/discord/runtime_test.go` and `internal/runtime/discord/runtime_contract_test.go`
  - cover root-command dispatch, user-visible task-command replies, and adapter-level Discord behavior

## Starting State

Start this plan only after confirming the repository still matches these facts:

- PR `#86` is merged and the documentation now describes one-shot `task:<name>` overrides plus slug-style task names.
- `internal/runtime/discord/message_mapper.go` still strips bot mentions and returns trimmed content in `app.MessageRequest`.
- `internal/thread/policy.go` still returns `ErrNoActiveTask` when no saved active task exists and still resolves task-mode routing only from `active_tasks`.
- `internal/app/task_service.go` still accepts free-form task names, derives `BranchName` through `DefaultTaskBranchName`, and still supports ambiguity fallback through `task_id`.
- `internal/store/sqlite/store.go` still has no exact open-task-by-name helper.
- `internal/app/message_service_test.go` still contains `TestMessageServiceHandleMessageFreezesTaskContextForQueuedWork`, because that test proves the queue already freezes logical keys before deferred execution.

Verify that state from the repository root:

    make test
    make lint

If any of those facts have drifted, update this plan first so it remains self-contained and truthful.

## Plan of Work

Begin in the app layer by adding reusable task-name and task-override helpers under `internal/app`. The task-name helper should expose strict validation for new task slugs. The override helper should inspect the normalized message body, recognize `task:<name>` only at the first meaningful token, return the selected task name when valid, return the cleaned prompt body that should be sent to Codex, and produce an explicit rejection string when the prefix is malformed or leaves the message with neither body text nor attachments.

Next, extend the message-routing contract. `app.MessageRequest` should gain a `TaskOverrideName` field. `internal/app/message_service_impl.go` should parse the one-shot prefix before it calls the thread policy, strip the prefix from the prompt that will reach Codex, and preserve the selected override name in the request it passes to `ThreadPolicy.ResolveMessageKey`. That same preparation step is where the feature should reject malformed override syntax or `task:<name>` messages that have no remaining body text and no attachments.

Then update the routing and store layers. `internal/thread/policy.go` should continue using the active task when `TaskOverrideName` is empty, but when it is set, it should resolve the effective task from an exact open-task-by-name lookup. Add store helpers that can answer two questions precisely: “is there an open task with this exact name?” and “does a closed task with this name exist when no open task does?” The message path needs the second answer to tell the user “that task is closed” instead of “that task does not exist.”

Tighten the task-command surface next. `internal/app/task_service.go` should reject new task names that do not match the documented slug contract and should reject creation when an open task with the same name already exists for that user. The same file should keep `task_id` selectors working as a compatibility path for older tasks, but the help and error text should stop presenting ambiguous-name fallback as the normal path for newly created tasks. `internal/runtime/discord/commands.go`, `internal/app/command_surface.go`, and any task-command output strings should be updated to reflect the slug-first contract and the one-shot override examples.

Finally, add automated proof. Update or add tests across the app, thread, store, and Discord runtime layers so the new route selection is exercised under immediate execution, queued execution, attachments-only messages, invalid-format rejection, missing-task rejection, closed-task rejection, and the “active task remains unchanged” rule. The plan is not complete until `make test` and `make lint` both pass from the repository root.

## Milestone 1: Add strict task-name and override parsing primitives

At the end of this milestone, the repository should have reusable helpers that define the task-name contract and the override syntax in one place instead of scattering string checks across multiple packages.

Add a new helper file such as `internal/app/task_name.go` that defines the slug validator used by task creation. The validator should reject uppercase letters, spaces, consecutive hyphens, names shorter than 3 characters, names longer than 32 characters, names that do not start with a letter, and names that do not end with a letter or digit.

Add a second helper file such as `internal/app/task_override.go` that parses normalized message content. It should recognize these forms:

    task:release-bot-v1 fix the failing tests
    task:docs-cleanup
    summarize current status

and it should accept `task:<name>` by itself when the message still has one or more image attachments. It must not treat a later `task:<name>` substring inside the body as an override.

The output of the parser should tell the caller whether an override was present, what the task name is, what prompt text should still be sent to Codex, and whether the message must be rejected before routing continues.

## Milestone 2: Route messages through an effective task

At the end of this milestone, a normal task-mode message should route through the overridden task whenever a valid `task:<name>` prefix is present, and every later step of message execution should use that same effective task.

Update `internal/app/types.go` so `MessageRequest` carries `TaskOverrideName string`.

Update `internal/app/message_service_impl.go` in `prepareMessage` so that:

- task-mode requests parse the one-shot override before calling `ThreadPolicy.ResolveMessageKey`
- malformed override syntax returns an explicit user-facing rejection response
- `task:<name>` with no remaining body text and no image attachments returns an explicit rejection response
- valid overrides strip the prefix from `CodexTurnInput.Prompt`
- the selected `TaskOverrideName` is preserved in the request that goes to the policy

Update `internal/thread/policy.go` so task-mode routing now works like this:

- when `TaskOverrideName` is empty, load the saved active task exactly as today
- when `TaskOverrideName` is set, resolve the exact open task by name for that user
- when the named task exists but is closed, return a dedicated error that the message path can translate into explicit user-facing guidance
- when the named task does not exist, return a dedicated not-found error instead of falling back to the active task

Do not change queue semantics. The existing queue coordinator should continue freezing the logical key chosen before admission. The feature must only change which logical key is selected for that one message.

## Milestone 3: Tighten task creation and compatibility behavior

At the end of this milestone, newly created tasks should always have strict slug names, duplicate open names should be rejected, and compatibility with older free-form task rows should remain available through `task_id`.

Update `internal/app/task_service.go` so `CreateTask` validates the new slug contract before it creates a task row. It must reject invalid names with explicit guidance and must reject an open-name collision before creating the row. The success response should continue making the new task active.

Update store helpers in `internal/store/sqlite/store.go` and the `app.ThreadStore` interface so the app layer can:

- fetch one open task by exact name
- detect whether at least one closed task with that exact name exists

Preserve `task_id` support in `SwitchTask` and `CloseTask` so legacy tasks remain reachable. When `task_name` is used for a new slug-style task, the exact-name path should be the normal route. Update the user-facing help text and error messages in:

- `internal/runtime/discord/commands.go`
- `internal/app/command_surface.go`
- `internal/app/task_service.go`

so they describe slug-style names and no longer present ambiguous-name fallback as the expected modern workflow.

## Milestone 4: Add automated proof and acceptance coverage

At the end of this milestone, automated tests should prove the feature end to end without relying on a live Discord server.

Update or add tests in these files:

- `internal/app/task_service_test.go`
- `internal/app/message_service_test.go`
- `internal/thread/policy_test.go`
- `internal/store/sqlite/store_test.go`
- `internal/runtime/discord/message_mapper_test.go`
- `internal/runtime/discord/runtime_test.go`
- `internal/runtime/discord/runtime_contract_test.go`

Add explicit coverage for:

- valid override on an immediate task-mode turn
- valid override on a queued task-mode turn where the saved active task changes before the queued turn executes
- override with attachments and no body text
- invalid override format
- override to a missing task
- override to a closed task
- no active task plus no override
- valid override while another task remains saved as active
- task creation rejection for invalid slug names
- task creation rejection for duplicate open-task names

The app-level reply path should also prove the “immediate user-facing confirmation” requirement. For queued turns, the queued acknowledgment should name the overridden task. For immediate turns, the returned assistant-facing response should include a short confirmation prefix naming the overridden task before the Codex answer text so the Discord user can see which task handled the message.

## Concrete Steps

Run all commands from the repository root.

Start with focused tests while editing the routing helpers:

    go test ./internal/thread -run 'TestPolicy' -v
    go test ./internal/app -run 'TestTaskCommandService|TestMessageServiceHandleMessageTask' -v

Once the parsing helper exists, add or rename tests so the new cases are obvious. Good target names include:

    TestTaskCommandServiceCreateTaskRejectsInvalidSlug
    TestTaskCommandServiceCreateTaskRejectsDuplicateOpenName
    TestPolicyResolveMessageKeyUsesTaskOverrideName
    TestMessageServiceHandleMessageTaskOverrideRoutesOnlyCurrentMessage
    TestMessageServiceHandleMessageTaskOverrideRejectsClosedTask
    TestMessageServiceHandleMessageTaskOverrideAllowsAttachmentsOnly

After the app and thread layers are green, run the adapter tests:

    go test ./internal/runtime/discord -run 'TestMapMessageCreate|TestRuntime' -v
    go test ./internal/runtime/discord -run 'TestRuntimeContract' -v

Finish with the repository-standard validation commands:

    make test
    make lint

Expected final transcript:

    $ make test
    ok   github.com/HatsuneMiku3939/39claw/internal/app               ...
    ok   github.com/HatsuneMiku3939/39claw/internal/runtime/discord   ...
    ok   github.com/HatsuneMiku3939/39claw/internal/thread            ...
    ...

    $ make lint
    0 issues.
    Linting passed

## Validation and Acceptance

This plan is complete only when a contributor can demonstrate all of the following:

1. Create two tasks with slug names such as `docs-cleanup` and `release-bot-v1`.
2. Keep `docs-cleanup` active.
3. Send a normal task-mode message that begins with `task:release-bot-v1`.
4. Observe that the reply explicitly confirms `release-bot-v1` handled that message.
5. Run `/<instance-command> action:task-current` and observe that `docs-cleanup` is still the active task.
6. Send two overlapping override messages for the same target task and observe that the second one queues behind the first without being rerouted when the active task changes later.
7. Attempt `task:Release Work` or `task:bad--slug` and observe an explicit rejection response.
8. Attempt `task:closed-task` when only a closed task with that name exists and observe an explicit “closed task” rejection.

Automated acceptance must include the new app, thread, store, and runtime tests plus passing repository-wide `make test` and `make lint`.

## Idempotence and Recovery

The code changes in this plan should be additive and safe to rerun. The focused `go test` commands can be rerun after each edit without mutating local state. The repository-wide `make test` and `make lint` commands are also safe to repeat.

Because this plan deliberately avoids a data-rewrite SQLite migration, recovery is straightforward: if the implementation is abandoned partway through, revert the code changes and keep the existing schema. If later implementation work reveals that legacy task-name data needs a stronger migration story, update this ExecPlan first before adding new schema files or backfill logic.

If the working tree drifts while the feature is being implemented, refresh this plan's `Starting State`, `Progress`, and `Decision Log` sections before continuing. The next contributor should never need outside context to understand which compatibility choices were made and why.

## Artifacts and Notes

The issue's target UX should remain the anchor:

    task:release-bot-v1 fix the failing tests
    task:docs-cleanup
    summarize current status and next steps

The compatibility story should also remain explicit in user-facing review notes:

    New tasks must use slug-style names.
    Legacy tasks with older free-form names remain reachable through `task_id` during the transition.

The implementation PR that follows this plan should link back to:

- issue `#85`
- documentation PR `#86`

## Interfaces and Dependencies

Do not add new third-party dependencies for this feature.

At the end of implementation, these interfaces and helpers should exist:

In `internal/app/types.go`, extend `MessageRequest` with:

    TaskOverrideName string

In `internal/app/task_name.go`, define a validator with a narrow API such as:

    func ValidateTaskName(taskName string) error

In `internal/app/task_override.go`, define a parser result and parser such as:

    type ParsedTaskOverride struct {
        Matched       bool
        TaskName      string
        Prompt        string
        RejectMessage string
    }

    func ParseTaskOverride(content string, hasImages bool) ParsedTaskOverride

In `internal/app/message_service.go`, extend `ThreadStore` with exact-name helpers such as:

    GetOpenTaskByName(ctx context.Context, discordUserID string, taskName string) (Task, bool, error)
    HasClosedTaskWithName(ctx context.Context, discordUserID string, taskName string) (bool, error)

In `internal/app/errors.go`, add dedicated routing errors so the message path can distinguish:

    invalid override format
    missing override target
    closed override target

In `internal/thread/policy.go`, keep the existing public method:

    ResolveMessageKey(ctx context.Context, request app.MessageRequest) (string, error)

but teach it to prefer `request.TaskOverrideName` when that field is non-empty.

In `internal/runtime/discord/commands.go`, keep the existing root-command structure and do not add another slash command. Only update text and examples so they match the slug-first contract.

Plan revision note (2026-04-11 / Codex): Created the initial active ExecPlan after documentation PR `#86` merged so issue `#85` can move into the execution-planning phase with a self-contained implementation guide.
