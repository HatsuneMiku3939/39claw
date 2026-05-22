# Rename user-facing task controls to thread controls

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agents/PLANS.md`.

## Purpose / Big Picture

After this plan, users of a `thread` mode 39claw bot will control interactive thread work with thread-facing Discord vocabulary instead of task-facing vocabulary. A user will create an interactive work item with `/<instance-command> action:thread-new thread_name:<name>`, switch with `/<instance-command> action:thread-switch thread_name:<name>`, and route one normal message with a leading `thread:<name>` prefix.

This is a user-facing vocabulary rename only. The internal durable work item is still represented by the existing task domain model, SQLite tables, store methods, task worktree lifecycle, task branch prefix, and task-scoped Codex thread binding. Scheduled tasks are also intentionally unchanged because they are genuinely scheduled tasks, not interactive thread controls. A contributor can see the rename working by running app and Discord runtime tests and by inspecting the generated slash-command choices and options: `thread-*`, `thread_name`, and `thread_id` should be user-visible, while the old interactive `task-*`, `task_name`, `task_id`, and `task:<name>` surfaces should no longer be accepted as current controls.

## Progress

- [x] (2026-05-22 22:33Z) Reviewed `.agents/PLANS.md`, the active ExecPlan index, the current Discord command registration code, the app-layer command guidance helpers, and the current product/design documentation references for `action:task-*`.
- [x] (2026-05-22 22:33Z) Confirmed the current runtime does not expose a literal fixed `/task` slash command; it exposes one configured root command whose action choices currently include `task-*` values.
- [x] (2026-05-22 22:33Z) Investigated the broader rename request and found that task terminology is deeply embedded in SQLite schema, Go types, logical keys, worktree state, branch names, logs, and scheduled-task automation.
- [x] (2026-05-22 22:33Z) Narrowed the accepted scope to user-facing interactive thread controls while keeping scheduled tasks and internal task persistence unchanged.
- [ ] Update Discord runtime action constants, registered command choices, option names, command routing, help text, unsupported-action guidance, and runtime tests to use `thread-*`, `thread_name`, and `thread_id`.
- [ ] Update normal-message one-shot parsing from `task:<name>` to `thread:<name>`, including safe rejection guidance for stale leading `task:<name>` prefixes.
- [ ] Update app-layer user-facing command guidance so missing-context, ambiguous-target, reset-context, and one-shot recovery messages point to `action:thread-*`, `thread_name`, `thread_id`, and `thread:<name>`.
- [ ] Update repository documentation and examples so interactive thread-mode controls use thread-facing vocabulary while scheduled-task docs and scheduled-task tools remain unchanged.
- [ ] Run focused tests for the app, thread policy, and Discord runtime packages, then run `make test` and `make lint`.
- [ ] Record implementation evidence and any changed decisions back into this ExecPlan.

## Surprises & Discoveries

- Observation: There is no current hard-coded `/task` slash command to rename.
  Evidence: `internal/runtime/discord/commands.go` registers exactly one command named by `cfg.DiscordCommandName`, and `README.md` describes that name as coming from `CLAW_DISCORD_COMMAND_NAME`.

- Observation: The current user-facing interactive command vocabulary still says `task-*` even after the mode rename to `thread`.
  Evidence: `internal/runtime/discord/interaction_mapper.go` defines `actionTaskCurrent = "task-current"` and related values, while `README.md`, `docs/product-specs/thread-mode-user-flow.md`, `docs/product-specs/discord-command-behavior.md`, and `docs/design-docs/implementation-spec.md` document `/<instance-command> action:task-*`.

- Observation: The word `task` remains a real internal domain concept and is not safe to remove globally as part of this user-facing rename.
  Evidence: the SQLite `tasks` and `active_tasks` tables, `thread_bindings.task_id`, `internal/app.Task`, `internal/app.ActiveTask`, `internal/app/task_service.go`, `internal/app/task_workspace.go`, task branch names such as `task/<slug>`, and logical keys shaped like `userID:taskID` all model persisted state.

- Observation: Scheduled tasks are a separate product surface that should remain task-named.
  Evidence: `internal/app/scheduled_tasks.go`, `internal/scheduled/mcp_server.go`, `migrations/sqlite/0004_scheduled_tasks.sql`, `migrations/sqlite/0005_scheduled_task_history.sql`, `docs/design-docs/scheduled-tasks.md`, and `docs/product-specs/scheduled-tasks-user-flow.md` all describe time-based automation tasks rather than interactive thread controls.

- Observation: If `task:<name>` simply stops being parsed, a stale prefix could become ordinary prompt text for the active thread and run Codex unintentionally.
  Evidence: `internal/app/message_service_impl.go` currently calls `ParseTaskOverride` before thread policy resolution; if no override is detected and an active task exists, the message proceeds as normal content.

## Decision Log

- Decision: Rename user-facing interactive thread controls from task vocabulary to thread vocabulary.
  Rationale: The user clarified that the desired rename includes the command actions, command option names, and one-shot normal-message prefix, but should remain user-facing only.
  Date/Author: 2026-05-22 / Codex

- Decision: Keep the internal task entity and SQLite schema unchanged.
  Rationale: Renaming `Task` types and `tasks` tables to `Thread` would collide with existing Codex thread and `thread` mode terminology, require a risky schema migration, and exceed the accepted user-facing scope.
  Date/Author: 2026-05-22 / Codex

- Decision: Keep scheduled-task vocabulary unchanged.
  Rationale: Scheduled tasks are actually task-like automation definitions, not interactive thread controls. Renaming them would make the product less accurate and would touch MCP tools, SQLite tables, scripts, and active scheduled-task planning for no benefit to this request.
  Date/Author: 2026-05-22 / Codex

- Decision: Do not keep old interactive `task-*`, `task_name`, `task_id`, or `task:<name>` aliases as supported controls.
  Rationale: The command surface should have one current vocabulary. Aliases would make help text, tests, and user support more confusing.
  Date/Author: 2026-05-22 / Codex

- Decision: Safely reject stale leading `task:<name>` prefixes instead of treating them as normal prompt text.
  Rationale: Without a rejection guard, a user who remembers the old prefix could accidentally send a prompt to the active thread instead of routing to the intended thread. Rejection is not a compatibility alias; it is a safety check that tells the user to retry with `thread:<name>`.
  Date/Author: 2026-05-22 / Codex

## Outcomes & Retrospective

Not yet implemented. When the implementation lands, update this section with the final behavior, test results, and any remaining manual Discord smoke-test gap.

## Context and Orientation

39claw is a Go-based Discord bot. The bot has two configured modes. `journal` mode is a shared day-based assistant flow. `thread` mode is a repository work flow where each user selects a durable work item, and each work item can have its own Codex thread and Git worktree.

A Discord slash command is the command users invoke with a leading slash. In this repository, each running bot instance registers exactly one slash command whose root name comes from `CLAW_DISCORD_COMMAND_NAME`; examples in docs use placeholders such as `/<instance-command>` or concrete names such as `/dev`. The command has a required string option named `action`. In `thread` mode, that action option currently accepts values such as `task-new` and `task-switch`; this plan changes those to `thread-new` and `thread-switch`.

A command option is a named field in the Discord slash-command UI. The current interactive thread controls expose `task_name` and `task_id`. This plan changes those user-visible option names to `thread_name` and `thread_id` while continuing to pass their values into the existing task service internally.

A one-shot override is a prefix at the first meaningful token of a normal message that routes only that one message to a named work item without changing the saved active selection. The current prefix is `task:<name>`. This plan changes the current prefix to `thread:<name>` and rejects stale leading `task:<name>` prefixes with guidance.

The internal task entity is the persisted implementation model for the durable work item in `thread` mode. It has a task ID, task name, task branch, optional worktree path, and task-scoped Codex conversation continuity. This plan does not rename the task entity, the `tasks` SQLite table, `active_tasks`, `thread_bindings.task_id`, task service APIs, task branch prefix, or task worktree directories.

The most relevant files are:

- `internal/runtime/discord/interaction_mapper.go`
  - defines action constants and reads the `action`, `task_name`, and `task_id` command options from Discord interactions
- `internal/runtime/discord/commands.go`
  - registers slash-command action choices and option names, and renders command help and unsupported-action text
- `internal/runtime/discord/runtime.go`
  - routes a mapped command request to the daily command service or the task command service
- `internal/app/task_override.go`
  - parses the current `task:<name>` one-shot prefix and should become the parser for `thread:<name>` plus stale-prefix rejection
- `internal/thread/policy.go`
  - resolves the effective internal task from either the saved active task or the parsed one-shot override name
- `internal/app/command_surface.go`
  - renders user-facing command snippets used by app-layer task guidance messages
- `internal/app/task_service.go`
  - implements create, switch, list, close, current, and reset-context task operations
- `internal/app/message_service_impl.go`
  - invokes the one-shot parser and emits recovery guidance for missing, closed, or ambiguous targets
- `internal/app/types.go`
  - contains internal request fields such as `TaskOverrideName`; these may remain internally task-named
- `internal/runtime/discord/*_test.go`, `internal/app/*_test.go`, and `internal/thread/*_test.go`
  - contain focused tests with expected command strings, option names, and one-shot prefix behavior
- `README.md`, `docs/product-specs/thread-mode-user-flow.md`, `docs/product-specs/discord-command-behavior.md`, `docs/design-docs/thread-modes.md`, `docs/design-docs/thread-mode-worktrees.md`, `docs/design-docs/state-and-storage.md`, `docs/design-docs/architecture-overview.md`, and `docs/design-docs/implementation-spec.md`
  - contain current user-facing and design-facing interactive thread-mode examples
- `example/thread-repository.md`
  - contains an operator walkthrough with concrete `/dev action:task-*` examples

Do not rename scheduled-task files, scheduled-task MCP tools, scheduled-task SQLite tables, scheduled-task docs, or `scripts/debug-scheduled-mcp.sh`.

## Plan of Work

First, change the Discord runtime user-facing command surface. In `internal/runtime/discord/interaction_mapper.go`, replace the action values with:

    actionThreadCurrent = "thread-current"
    actionThreadList = "thread-list"
    actionThreadNew = "thread-new"
    actionThreadSwitch = "thread-switch"
    actionThreadClose = "thread-close"
    actionThreadResetContext = "thread-reset-context"

Also replace the option names with:

    optionThreadName = "thread_name"
    optionThreadID = "thread_id"

The mapped `commandRequest` fields may be renamed to `ThreadName` and `ThreadID`, or they may remain internally named `TaskName` and `TaskID`. The user-visible option names must be `thread_name` and `thread_id`. If the request fields remain task-named, add a short code comment only if needed to explain that the Discord vocabulary differs from the internal persistence model.

In `internal/runtime/discord/commands.go`, register only the new `thread-*` choices for `thread` mode. Register the optional command fields as `thread_name` and `thread_id`. Update option descriptions, `helpResponse`, `taskUnavailableJournalMode` if needed, and `unsupportedActionText` so all interactive command examples use `thread-*`, `thread_name`, and `thread_id`. Keep scheduled-task text untouched.

In `internal/runtime/discord/runtime.go`, route the new `actionThread*` constants to the same app-layer `TaskCommandService` methods. No task service method signatures need to change for this user-facing rename.

Second, change the one-shot prefix. In `internal/app/task_override.go`, change the accepted current prefix from `task:` to `thread:`. Rename `ParseTaskOverride` and `ParsedTaskOverride` only if it improves readability; internal names may remain task-named because the parsed value still selects an internal task. The parser must reject a leading `task:<name>` prefix with a clear message such as:

    The `task:<name>` prefix has been renamed. Use `thread:<name>` to route one message to another thread.

The parser should continue to treat a later literal `task:<name>` or `thread:<name>` inside the body as ordinary text. Only the first meaningful token matters.

Third, update app-layer command guidance. In `internal/app/command_surface.go`, change rendered command snippets from `action:task-* task_name:<name>` to `action:thread-* thread_name:<name>` and from `task_id:<id>` to `thread_id:<id>`. The helper method names may remain internally task-named or be renamed to thread-facing helper names; prefer names that make call sites readable without implying a storage migration.

Update call sites in `internal/app/task_service.go` and `internal/app/message_service_impl.go` to use the new snippets. Keep internal task entity names, task validation, task storage calls, and user-facing words such as "task" only when they describe stored work items rather than command syntax. Where possible in user-facing thread-mode prose, prefer "thread" for the selected user workflow and "scheduled task" for scheduler automation.

Fourth, update tests. Adjust runtime command tests so registered action choices include `thread-current`, `thread-list`, `thread-new`, `thread-switch`, `thread-close`, and `thread-reset-context`; option names include `thread_name` and `thread_id`; and old interactive `task-*` action choices are absent. Update command-routing tests to dispatch the new constants and option names.

Update app tests to expect guidance strings such as `Use /release action:thread-new thread_name:<name>`, `retry /release action:thread-reset-context`, and stale-prefix guidance for `task:<name>`. Update one-shot parser tests so `thread:docs-update fix it` routes, while `task:docs-update fix it` is rejected.

Add or adjust at least one focused assertion that stale `task-*` actions are not treated as supported command actions. Add or adjust one focused assertion that stale leading `task:<name>` does not run as ordinary prompt text.

Fifth, update current documentation and examples. Replace interactive thread-mode command examples and one-shot examples with thread-facing vocabulary in:

- `README.md`
- `docs/product-specs/thread-mode-user-flow.md`
- `docs/product-specs/discord-command-behavior.md`
- `docs/design-docs/thread-modes.md`
- `docs/design-docs/thread-mode-worktrees.md`
- `docs/design-docs/state-and-storage.md`
- `docs/design-docs/architecture-overview.md`
- `docs/design-docs/implementation-spec.md`
- `example/thread-repository.md`

Do not replace scheduled-task terminology, scheduled-task MCP tool names, scheduled-task table names, or active scheduled-task ExecPlan text. Scheduled task remains scheduled task.

Finally, run focused tests first, then repository checks. Update this ExecPlan with the validation transcript and any surprises before considering the plan complete.

## Concrete Steps

Start from the repository root:

    cd /home/filepang/.local/share/39claw/39claw/worktrees/01KS8X0GS6AM3VAMC9GB36A23T

Confirm the current working tree and references:

    git status --short
    rg -n 'action:task-|task_name|task_id|task:<name>|task:' internal docs README.md example --glob '!docs/exec-plans/completed/**'

Implement the user-facing rename in the Discord runtime, one-shot parser, app command guidance, tests, and current documentation named in the Plan of Work.

Run focused tests after the code changes:

    go test ./internal/runtime/discord ./internal/app ./internal/thread

Run the full repository checks required by this repository:

    make test
    make lint

After implementation, run final searches:

    rg -n 'action:task-|task_name|task_id|task:<name>|task:' internal docs README.md example --glob '!docs/exec-plans/completed/**'
    rg -n 'scheduled[_ -]?task|ScheduledTask|scheduled_tasks' internal docs README.md migrations scripts example --glob '!docs/exec-plans/completed/**'

Expected final search behavior: current interactive thread-mode user-facing surfaces should no longer use `action:task-*`, `task_name`, `task_id`, or `task:<name>`. Remaining `task` hits are acceptable when they refer to internal task persistence, Go type names, log attributes, SQLite schema, tests for internal task IDs, scheduled tasks, or this active ExecPlan's explanation of the rename.

## Validation and Acceptance

The change is accepted when the following behavior is true:

- In `thread` mode, registered Discord action choices include `thread-current`, `thread-list`, `thread-new`, `thread-switch`, `thread-close`, and `thread-reset-context`.
- In `thread` mode, registered Discord action choices do not include `task-current`, `task-list`, `task-new`, `task-switch`, `task-close`, or `task-reset-context`.
- In `thread` mode, registered command options are `thread_name` and `thread_id`, not `task_name` and `task_id`.
- `/<instance-command> action:thread-current` routes to the current-thread or current-work-item response backed by the existing task service.
- `/<instance-command> action:thread-list` routes to the open-thread list response backed by the existing task service.
- `/<instance-command> action:thread-new thread_name:<name>` creates the internal task record and makes it active.
- `/<instance-command> action:thread-switch thread_name:<name>` switches the active internal task.
- `/<instance-command> action:thread-close thread_name:<name>` closes the selected internal task.
- `/<instance-command> action:thread-reset-context` resets only the saved Codex conversation continuity for the active internal task.
- A normal message that starts with `thread:<name>` routes only that message to the named open internal task without changing the saved active internal task.
- A normal message that starts with stale `task:<name>` is rejected with guidance to use `thread:<name>` and is not sent to Codex as ordinary prompt text.
- Help text and unsupported-action text mention `thread-*`, `thread_name`, `thread_id`, and `thread:<name>`, not the old interactive `task-*` command syntax.
- Scheduled-task docs, MCP tools, HTTP paths, SQLite tables, and runtime behavior still use scheduled-task vocabulary.
- No SQLite migration is added solely for this user-facing rename.
- `make test` and `make lint` pass.

A manual Discord smoke test should be recorded if credentials are available:

    1. Start a `thread` mode bot with a disposable guild command name such as `dev`.
    2. Open Discord and confirm the slash-command action picker offers `thread-current`, `thread-list`, `thread-new`, `thread-switch`, `thread-close`, and `thread-reset-context`.
    3. Confirm the slash-command options shown for create, switch, and close are `thread_name` and `thread_id`.
    4. Run `/dev action:thread-new thread_name:smoke-test`.
    5. Observe an ephemeral success response saying the thread or work item was created and made active.
    6. Send a normal message starting with `thread:smoke-test`.
    7. Observe that the message routes to that selected work item.
    8. Send a normal message starting with `task:smoke-test`.
    9. Observe a rejection that tells the user to use `thread:smoke-test`.

If Discord credentials are not available, record that the manual smoke test was not run and rely on the runtime registration, routing, app, and parser tests.

## Idempotence and Recovery

The implementation should be safe to retry. Renaming user-facing constants, parser strings, tests, and documentation is a normal text/code change and can be repeated until tests pass.

This plan must not add a SQLite migration because no persisted task data changes. Existing task records, active-task mappings, thread bindings, branch names, and worktree paths continue to work after the user-facing rename.

This plan intentionally does not support old interactive command aliases. If a user submits a stale Discord interaction payload with `action:task-new`, the bot should treat it as unsupported and return guidance that lists the new `thread-*` actions. In normal Discord usage, users select from the newly registered choices after command registration updates.

This plan intentionally rejects stale leading `task:<name>` one-shot prefixes. If a user sends one, the bot should return guidance and should not route the message to Codex. This prevents accidental execution against the active thread.

If deployment leaves an old command registration cached in Discord, restart the bot with the same `CLAW_DISCORD_GUILD_ID` or global command registration settings it normally uses and allow Discord's command registration propagation to update the choices. Do not add compatibility aliases solely for stale command caches without first updating this plan and recording the tradeoff.

## Artifacts and Notes

Initial search evidence from 2026-05-22:

    README.md:150:- `/<instance-command> action:task-*` controls which task is active
    internal/runtime/discord/interaction_mapper.go:11: optionTaskName = "task_name"
    internal/runtime/discord/interaction_mapper.go:12: optionTaskID = "task_id"
    internal/runtime/discord/interaction_mapper.go:16: actionTaskCurrent = "task-current"
    internal/app/task_override.go:14: if !strings.HasPrefix(trimmed, "task:")
    internal/app/command_surface.go:14: return fmt.Sprintf("`/%s action:task-list`", s.commandName)
    docs/product-specs/thread-mode-user-flow.md:196:- one-shot `task:<name>` override syntax on normal messages
    docs/product-specs/discord-command-behavior.md:123:- `/<instance-command> action:task-current`

The final implementation notes should include short test transcripts for:

    go test ./internal/runtime/discord ./internal/app ./internal/thread
    make test
    make lint

Revision Note: 2026-05-22 22:33Z / Codex - Created the initial active ExecPlan after confirming the first requested rename meant changing `action:task-*` slash-command action values to `action:thread-*`.

Revision Note: 2026-05-22 22:33Z / Codex - Revised the plan after broader investigation and user clarification: user-facing interactive controls should rename fully to thread vocabulary, including `thread_name`, `thread_id`, and `thread:<name>`, while internal task persistence and scheduled-task vocabulary remain unchanged.
