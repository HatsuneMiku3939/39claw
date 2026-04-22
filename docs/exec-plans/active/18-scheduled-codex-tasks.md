# Implement scheduled Codex tasks with MCP-managed definitions and runtime execution

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agents/PLANS.md`.

## Purpose / Big Picture

After this plan, a 39claw operator should be able to start the bot, ask it in normal conversation to create a scheduled task, and then observe 39claw execute that task automatically on schedule and deliver the result to Discord. In `daily` mode, the scheduled run should execute directly in `CLAW_CODEX_WORKDIR`. In `task` mode, the scheduled run should execute in its own fresh temporary worktree that is created for that run and removed after the run finishes.

The user-visible proof is concrete. A contributor should be able to run the bot with a test schedule, create a task through the Codex-visible management tools, wait for the due time, and observe a Discord delivery that names the scheduled task. The same contributor should also be able to inspect SQLite and see one canonical task definition, one admitted run record for the due occurrence, and a separate delivery record. If the bot runs in `task` mode, the contributor should be able to observe that the scheduled run used a temporary worktree that was cleaned up after completion.

## Progress

- [x] (2026-04-12 08:10Z) Reviewed `.agents/PLANS.md`, `docs/design-docs/scheduled-tasks.md`, `docs/product-specs/scheduled-tasks-user-flow.md`, and the current app/store/codex runtime shape to capture a self-contained implementation plan.
- [x] (2026-04-12 08:40Z) Added the scheduled-task MCP tool surface and recorded the per-run Codex `--config` override used to register it.
- [x] (2026-04-12 08:42Z) Added the instance-level scheduled report target setting, SQLite migrations `0004_scheduled_tasks.sql` and `0005_scheduled_task_history.sql`, and store APIs for scheduled-task definitions, runs, and deliveries.
- [x] (2026-04-12 08:44Z) Implemented per-run MCP config injection plus MCP-backed create, list, get, update, enable, disable, and delete operations for scheduled tasks.
- [x] (2026-04-12 08:47Z) Implemented the scheduler loop, due-run admission, fresh-thread execution path, and task-mode temporary scheduled-run worktree creation and cleanup.
- [x] (2026-04-12 08:48Z) Implemented bot-initiated Discord report delivery and stored delivery outcomes separately from run outcomes.
- [x] (2026-04-12 08:50Z) Added automated coverage for schedule parsing, MCP config injection, MCP management operations, task-mode temporary worktrees, scheduler execution, and delivery recording.
- [x] (2026-04-12 08:52Z) Ran `go test ./...` and `make lint` after the implementation landed.

## Surprises & Discoveries

- Observation: The repository already has strong infrastructure for thread bindings, active tasks, task worktree creation, and Discord reply delivery, but it has no scheduler loop, no scheduled-task persistence, and no local MCP server.
  Evidence: `internal/app/message_service_impl.go`, `internal/app/task_workspace.go`, `internal/store/sqlite/store.go`, and the absence of any scheduled-task package under `internal/`.

- Observation: The Codex gateway already reports `mcp_tool_call` progress events, which means the runtime can surface tool activity once a local MCP server exists, and the CLI can accept MCP registration through `--config` without rewriting the user's Codex home.
  Evidence: `internal/codex/gateway.go` handles `item.Type == "mcp_tool_call"`, and manual CLI inspection confirmed `codex exec --config 'mcp_servers.<name>=...'` support.

- Observation: `task` mode already owns the managed bare-parent and worktree lifecycle code for interactive tasks, but scheduled tasks intentionally must not reuse an interactive task worktree.
  Evidence: `internal/app/task_workspace.go` and `docs/design-docs/scheduled-tasks.md`.

- Observation: The current Codex CLI can register a local MCP server entirely through one `--config` override and does not require a managed `CODEX_HOME`, which made it possible to expose the scheduled-task tools from the running 39claw process itself.
  Evidence: the implementation now injects a config string shaped like `mcp_servers.scheduled-tasks={url = "http://127.0.0.1:<port>/mcp/scheduled-tasks"}` into `internal/codex/exec.go`, and `cmd/39claw/main.go` starts the loopback streamable HTTP endpoint before the Discord runtime begins serving.

- Observation: The first implementation admitted every overdue recurring cron occurrence on one scheduler tick, but for personal-instance use that created an undesirable backlog flood after downtime. The current design now skips recurring cron boundaries that happened before the current scheduler process started instead of replaying that backlog after startup.
  Evidence: the first scheduler test initially admitted two `* * * * *` runs until the fixture creation time was moved off the previous minute boundary in `internal/app/scheduled_task_service_test.go`.

- Observation: The hand-rolled transport was not accepted by the real Codex CLI during initialization, and a first `mcp-go` SSE variant still mismatched Codex's expectations for `url`-based MCP registration, so the scheduled-task endpoint had to move onto `github.com/mark3labs/mcp-go` streamable HTTP.
  Evidence: live Codex runs failed first with `Deserialize error: expected value at line 1 column 1, when process initialize response` and then with `UnexpectedContentType(Some("text/plain; charset=utf-8; body: Method not allowed\n"))`, and the final implementation now validates the endpoint through `mcp-go`'s own in-process and streamable HTTP clients in `internal/scheduled/*_test.go`.

## Decision Log

- Decision: Implement scheduled-task management through a local MCP server exposed by the 39claw binary itself instead of through new slash commands.
  Rationale: The product and design documents already define conversational management through Codex-mediated tools. A local MCP server keeps the implementation aligned with that product contract instead of creating a second management surface that would later need to be replaced.
  Date/Author: 2026-04-12 / Codex

- Decision: Add an optional instance-level default report target as `CLAW_SCHEDULED_REPORT_TARGET`.
  Rationale: The product spec says `report_target` is optional per task and that omitted values should fall back to instance-level reporting behavior. A dedicated environment variable is the smallest explicit way to define that default without tying scheduled delivery to one user's last message context.
  Date/Author: 2026-04-12 / Codex

- Decision: In `task` mode, scheduled runs must create a fresh temporary worktree for the run and remove it after the run reaches a terminal result.
  Rationale: This matches the current design note and prevents scheduled automation from borrowing or mutating an interactive task worktree that may belong to a different ongoing task context.
  Date/Author: 2026-04-12 / Codex

- Decision: A scheduled task may be created without an explicit `report_target`, but enabling it requires a resolved report target from either the task definition or `CLAW_SCHEDULED_REPORT_TARGET`.
  Rationale: This preserves the product-visible small schema while preventing silently “successful” scheduled runs that have nowhere valid to report.
  Date/Author: 2026-04-12 / Codex

## Outcomes & Retrospective

Implementation landed across the runtime, store, Discord adapter, and a new `internal/scheduled` package. 39claw now owns canonical scheduled-task definitions, due-run admission, delivery recording, and MCP exposure through a loopback streamable HTTP server hosted inside the main process, while Codex still owns interpretation and execution of the scheduled prompt.

The MCP registration risk was resolved more simply than the original milestone expected. Instead of materializing a managed `CODEX_HOME`, the bot now appends one per-run Codex config override that registers a loopback `scheduled-tasks` MCP endpoint served by the main 39claw process. That keeps user-provided `CODEX_HOME` behavior intact and lets the MCP handlers share the same SQLite-backed store instance as the scheduler runtime.

The final transport layer intentionally does not hand-roll MCP framing. The repository now uses `github.com/mark3labs/mcp-go` for the scheduled-task tool server and verifies the same endpoint shape with `mcp-go` clients in tests, which reduced protocol risk after the first bespoke transport implementations failed against a real Codex session.

The main remaining acceptance gap is operator-level live validation with a real Codex session and a Discord channel. The repository now has automated coverage for the moving pieces, but a future cleanup pass should still capture one manual end-to-end transcript before archiving this plan.

## Context and Orientation

39claw is a Go-based Discord bot. The current runtime receives Discord inputs, resolves a logical thread key, resumes or creates a Codex thread, and posts the result back to Discord. That user-driven path is implemented mostly in `internal/app/message_service_impl.go`, `internal/thread`, `internal/codex`, and `internal/runtime/discord`.

Persistent state lives in SQLite through `internal/store/sqlite/store.go`. The current application schema supports thread bindings, daily sessions, tasks, and active-task selection. There is no existing scheduled-task table, no scheduler loop, and no bot-initiated delivery path that originates outside a Discord user message.

The current task-mode worktree behavior matters because scheduled tasks add one more execution path. Interactive `task` mode already uses a managed bare parent repository and task worktrees under `CLAW_DATADIR`. Scheduled tasks must not reuse a user's interactive task worktree. Instead, when the bot runs in `task` mode, a scheduled run should create its own temporary worktree from the managed bare parent, execute the run there, and remove the worktree afterward.

The most relevant current files are:

- `docs/design-docs/scheduled-tasks.md`
  - the implementation-facing design rules for canonical definitions, scheduler behavior, task-mode temporary worktrees, and Discord delivery separation
- `docs/product-specs/scheduled-tasks-user-flow.md`
  - the user-facing behavior that the implementation must satisfy
- `cmd/39claw/main.go`
  - the bot startup path, configuration loading, and dependency wiring
- `internal/config/config.go`
  - environment-variable loading and validation
- `internal/app/types.go`
  - the shared application data types such as `Task`, `ThreadBinding`, and `CodexTurnInput`
- `internal/app/message_service_impl.go`
  - the current user-message execution path and queue coordination
- `internal/app/task_workspace.go`
  - the managed bare-parent and worktree behavior that scheduled `task`-mode runs should reuse at a lower level
- `internal/app/task_service.go`
  - the existing task command workflow and task persistence touchpoints
- `internal/codex/gateway.go` and `internal/codex/exec.go`
  - the Codex CLI integration and per-turn working-directory override
- `internal/store/sqlite/store.go`
  - the current persistence implementation and migration-aware schema handling
- `internal/runtime/discord`
  - the Discord adapter that posts replies and command responses

Terms used in this plan:

A “scheduled task definition” is the canonical SQLite-backed record that stores the task name, schedule, prompt, enabled state, and optional per-task report channel override.

A “scheduled run record” is the durable row that represents one admitted due occurrence. It is separate from the definition row so future runs and history can coexist safely.

A “delivery record” is the durable row that records whether Discord delivery succeeded, failed, or was skipped. Delivery status must not overwrite the run status.

A “scheduled-task MCP config override” is the per-run Codex `--config` fragment that registers the loopback streamable HTTP MCP endpoint without mutating the user's `CODEX_HOME`.

A “temporary scheduled-run worktree” is the short-lived Git worktree used only for one scheduled run when the bot instance runs in `task` mode. It is distinct from any interactive task worktree and must be removed after the run reaches a terminal result.

## Starting State

Begin implementation only after confirming the repository still matches these assumptions:

- there is no active scheduled-task implementation under `internal/`
- `docs/design-docs/scheduled-tasks.md` and `docs/product-specs/scheduled-tasks-user-flow.md` describe the target behavior
- `internal/app/task_workspace.go` still owns the managed bare-parent and task worktree logic for interactive task mode
- `cmd/39claw/main.go` still injects `CODEX_HOME` into the spawned Codex environment when configured
- `make test` and `make lint` pass before implementation begins

Verify that state with:

    cd <repository-root>
    make test
    make lint

If any of these assumptions have drifted, update this ExecPlan first so it remains self-contained and truthful.

## Preconditions

This plan fixes the following implementation choices before coding begins:

- scheduled tasks are stored in new SQLite tables instead of in repository files
- management happens through a local MCP server invoked by Codex, not through new slash commands
- the repository adds `CLAW_SCHEDULED_REPORT_TARGET` as the optional instance default report target
- the scheduler loop runs inside the 39claw process and participates in startup and shutdown
- scheduled runs always use fresh Codex threads
- `daily` mode scheduled runs execute directly in `CLAW_CODEX_WORKDIR`
- `task` mode scheduled runs execute in fresh temporary worktrees created from the managed bare parent and removed afterward
- delivery is recorded separately from execution
- infrastructure-level failure may retry a due run once, but content-level “failure” is whatever Codex writes into the report

## Milestone 1: Prove and document the Codex-visible management surface

At the end of this milestone, a contributor should know exactly how 39claw exposes schedule-management tools to Codex and how the current Codex CLI discovers those tools through per-run `--config` overrides.

Add a local MCP server surface to 39claw and expose it from the running process through a loopback HTTP endpoint. That endpoint should expose a minimal first tool such as `scheduled_tasks_list` backed by SQLite. The goal of this milestone is not the full feature. The goal is to prove the integration path end to end.

Alongside that entrypoint, add the smallest possible Codex CLI registration path for the local MCP server. The contributor executing this plan must record the exact override shape back into this ExecPlan as soon as the prototype works, because the repository did not previously document this path anywhere else.

Validation for this milestone is practical. Run a small integration test or manual `codexplay` transcript that starts a Codex thread with the MCP config override, triggers the prototype tool, and proves that the event stream contains an `mcp_tool_call` item. Do not move to Milestone 2 until this prototype path is working and documented in this plan.

## Milestone 2: Add canonical persistence and management operations

At the end of this milestone, the repository should have SQLite-backed scheduled-task definitions, run records, and delivery records, plus store methods that the MCP server can call directly.

Add new migrations after the current latest version. Use one migration for the canonical definitions table and one migration for run and delivery history so the bootstrap logic stays easy to reason about. The new tables should cover:

- `scheduled_tasks`
  - stable ID
  - unique instance-scoped name
  - schedule kind
  - schedule expression
  - prompt
  - enabled state
  - nullable `report_target`
  - created and updated timestamps
- `scheduled_task_runs`
  - run ID
  - scheduled task ID
  - due time
  - attempt number
  - status such as `pending`, `running`, `succeeded`, `failed`, `canceled`
  - fresh Codex thread ID when known
  - terminal timestamps
  - nullable working-directory evidence for debugging
- `scheduled_task_deliveries`
  - delivery ID
  - run ID
  - resolved target channel ID
  - delivery status such as `pending`, `succeeded`, `failed`, `skipped`
  - terminal timestamps
  - error text when delivery fails

Extend the store interface in `internal/app/message_service.go` or a nearby shared interface file with a focused scheduled-task store boundary rather than bloating `ThreadStore` further. Keep the names explicit: list tasks, get one task by ID or name, create, update, delete, admit due run, mark run started, finish run, record delivery, and query due tasks.

Implement the full management operations in the MCP server on top of that store. These operations must validate task names, schedule syntax, prompt non-emptiness, and report-target rules. Enabling a task must fail when neither the task definition nor `CLAW_SCHEDULED_REPORT_TARGET` resolves to a report target.

## Milestone 3: Add scheduler loop and due-run admission

At the end of this milestone, a running bot process should notice due scheduled tasks, admit each due occurrence once, and dispatch execution without depending on an incoming Discord message.

Add a scheduler service under `internal/app` or a clearly named sibling package. That service should:

- accept a clock abstraction so tests can drive time deterministically
- parse `cron` schedules with a dedicated library such as `github.com/robfig/cron/v3`
- parse one-shot `at` timestamps in the configured instance timezone using the Go standard library
- poll or compute due tasks on a short cadence without a busy loop
- admit each due occurrence exactly once using durable run-state checks in SQLite
- dispatch the actual Codex execution through a separate run executor

Keep the scheduler lifecycle explicit in `cmd/39claw/main.go`. Start it after store and Discord initialization succeed, and stop it during shutdown before canceling the shared runtime context. Reuse the repository’s current shutdown pattern so scheduled runs can finish or be canceled in a way that leaves clear run status behind.

This milestone must also define the retry rule concretely. When a run fails for infrastructure reasons before a terminal Codex result exists, the scheduler may create exactly one retry attempt. The retry attempt must share the same logical due occurrence but have its own run row or attempt number so history stays auditable.

## Milestone 4: Execute scheduled runs and integrate task-mode temporary worktrees

At the end of this milestone, admitted scheduled runs should actually execute through the Codex gateway, using the correct working-directory rule for the current bot mode.

Implement a scheduled-run executor that takes a due run, resolves its working directory, runs Codex in a fresh thread, captures the response, and records the terminal run status. Reuse the existing `CodexGateway` and `CodexTurnInput` path rather than creating a second Codex integration stack.

For `daily` mode, scheduled runs should pass `CLAW_CODEX_WORKDIR` directly as the working directory.

For `task` mode, scheduled runs must not call the interactive `TaskWorkspaceManager.EnsureReady` path with a fake user task. Instead, extract the lower-level managed bare-parent preparation logic from `internal/app/task_workspace.go` into a reusable helper that can do both of these things:

- prepare or sync the managed bare parent for the configured source checkout
- create a temporary worktree rooted at a caller-provided path and base ref

Use that helper to create a scheduled-run worktree under a new path such as `${CLAW_DATADIR}/scheduled-worktrees/<run-id>`, execute the run there, and then remove the worktree after the run reaches a terminal result. The cleanup path must run on success and on handled failure, and it must leave a logged error plus durable run evidence when cleanup itself fails.

This milestone must also record the exact prompt sent to Codex. The prompt should be the stored scheduled-task prompt, optionally prefixed with a small runtime-owned header that names the task and due time when that helps debugging, but it must stay small and deterministic.

## Milestone 5: Deliver scheduled results to Discord

At the end of this milestone, successful scheduled runs should appear in Discord with a clear task identity, and failed deliveries should be visible in durable delivery records.

Add a small bot-initiated delivery boundary to the Discord runtime so the scheduler can send a message to a channel without a triggering user message. Keep this separate from the reply path used by `MessageService`. The delivery API should accept channel ID, rendered text, and enough metadata to log the task and run IDs.

Resolve the target channel by this rule:

1. use the task definition’s `report_target` when present
2. otherwise use `CLAW_SCHEDULED_REPORT_TARGET`
3. if neither exists, refuse to enable the task, or mark delivery as `skipped` only for legacy rows that predate the validation rule

The message format does not need to be ornate, but it must be identifiable. Include the scheduled task name and enough context for a human to tell that the message came from a scheduled run rather than an interactive reply.

## Milestone 6: Complete automated coverage and finalize the living document

At the end of this milestone, the feature should be proven through tests, the plan should record what changed, and the repository should be ready to move this plan from `active/` to `completed/` after implementation is actually done.

Add unit and integration coverage for:

- schedule parsing and next-due computation
- store CRUD and due-run admission idempotence
- MCP management-tool handlers
- per-run MCP config injection for the local MCP server
- scheduler retry behavior
- daily-mode working-directory selection
- task-mode temporary scheduled-run worktree creation and cleanup
- Discord delivery success and failure recording

After implementation lands, update `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` with the real results and command outputs. Do not archive this plan until the full acceptance criteria below are satisfied.

## Plan of Work

Start with the integration risk, not the database. Build the smallest possible local MCP server and prove that Codex can see it through a per-run config override. Once that path exists, the repository can safely commit to management-through-tools without guessing.

Next, add the canonical scheduled-task persistence and keep the history tables separate from the definition table. This lets the scheduler, delivery layer, and management surface evolve independently without overloading one row with incompatible responsibilities.

After persistence is in place, add the scheduler loop and a run executor that reuses the existing Codex gateway. Keep scheduled execution separate from interactive message handling: both paths may call the gateway, but only user messages go through `MessageService`.

Then implement Discord delivery for bot-initiated reports and make delivery records explicit. A run that succeeded but could not be delivered must still count as an executed run.

Finally, wire the task-mode temporary worktree path for scheduled runs by extracting the low-level managed bare-parent logic out of the existing interactive task workspace manager. Do not try to fake an interactive task row just to reuse the current API. Scheduled runs are instance-scoped automation, not hidden user tasks.

## Concrete Steps

Run all commands from the repository root.

1. Confirm the baseline before implementation.

    make test
    make lint

2. Prototype the MCP path.

    go test ./internal/codex ./cmd/39claw -run 'Test.*Scheduled.*MCP|Test.*CodexHome' -v

    Extend or add tests until there is concrete proof that a Codex thread launched by 39claw can see at least one scheduled-task management tool.

3. Add migrations and store coverage.

    go test ./internal/store/sqlite -run 'Test.*Scheduled' -v

4. Add scheduler and runtime coverage.

    go test ./internal/app ./internal/runtime/discord -run 'Test.*Scheduled' -v

5. Run the full repository checks after the feature lands.

    make test
    make lint

Expected command outcomes after implementation:

    $ go test ./internal/store/sqlite -run 'Test.*Scheduled' -v
    === RUN   TestScheduledTaskCRUD
    --- PASS: TestScheduledTaskCRUD (0.00s)
    === RUN   TestAdmitDueScheduledRunOnce
    --- PASS: TestAdmitDueScheduledRunOnce (0.00s)
    PASS

    $ go test ./internal/app -run 'Test.*Scheduled' -v
    === RUN   TestSchedulerDispatchesDueRun
    --- PASS: TestSchedulerDispatchesDueRun (0.00s)
    === RUN   TestTaskModeScheduledRunUsesTemporaryWorktree
    --- PASS: TestTaskModeScheduledRunUsesTemporaryWorktree (0.01s)
    PASS

    $ make lint
    0 issues.
    Linting passed

## Validation and Acceptance

This plan is complete when all of the following are true:

- a Codex conversation can manage scheduled tasks through 39claw-owned tools rather than by editing files directly
- scheduled-task definitions persist in SQLite and survive restart
- due occurrences are admitted once even across restart
- scheduled runs always start fresh Codex threads
- `daily` mode scheduled runs execute directly in `CLAW_CODEX_WORKDIR`
- `task` mode scheduled runs execute in fresh temporary worktrees that are removed after completion
- Discord delivery uses either the per-task `report_target` or `CLAW_SCHEDULED_REPORT_TARGET`
- delivery status is recorded separately from execution status
- infrastructure failure retries a due run once and records the retry clearly
- `make test` passes
- `make lint` passes

The acceptance bar is behavioral. A contributor should be able to create a scheduled task, wait for it to fire, observe the Discord delivery, and inspect SQLite to confirm that definition state, run state, and delivery state were all recorded separately.

## Idempotence and Recovery

All schema changes must be additive and safe to rerun through the existing migration runner. A failed scheduled run must never delete the task definition. A failed Discord delivery must never retroactively mark the Codex run as failed. Temporary worktree cleanup in `task` mode is best-effort after the run reaches a terminal result; if cleanup fails, record the failure, leave the run result intact, and make the cleanup path safe to retry manually.

The scheduler itself must be restart-safe. If the process exits after admitting a due run but before delivery completes, the next startup should inspect durable run state and avoid admitting the same due occurrence again while still allowing incomplete terminal transitions to be repaired or reported.

## Artifacts and Notes

The most important proof artifacts to capture during implementation are:

- the exact `--config` override shape needed for the local MCP server
- a short event transcript showing an `mcp_tool_call` item for a scheduled-task management tool
- SQLite test evidence that one due occurrence is admitted once
- task-mode test evidence that a scheduled run creates and removes a temporary worktree under `${CLAW_DATADIR}/scheduled-worktrees/<run-id>`
- Discord runtime evidence that delivery failure is recorded separately from run failure

Example end-to-end behavior to preserve:

    user asks the bot to create a scheduled task named "daily-ops" with a cron schedule
    Codex uses a scheduled-task MCP tool to create it
    SQLite stores one scheduled_tasks row
    the scheduler admits the next due occurrence
    Codex executes the prompt in a fresh thread
    Discord receives a message that names "daily-ops"
    SQLite stores one scheduled_task_runs row and one scheduled_task_deliveries row

## Interfaces and Dependencies

Implement the feature with these concrete interfaces and package directions:

- in `internal/config/config.go`, add:

    Config.ScheduledReportTarget string

  and load it from `CLAW_SCHEDULED_REPORT_TARGET`.

- in `internal/app/types.go`, add stable scheduled-task data types such as:

    type ScheduledTask struct { ... }
    type ScheduledTaskRun struct { ... }
    type ScheduledTaskDelivery struct { ... }

- define a focused scheduled-task store boundary, for example in `internal/app/message_service.go` or a new `internal/app/scheduled_interfaces.go`, with methods equivalent to:

    CreateScheduledTask(ctx context.Context, task ScheduledTask) error
    UpdateScheduledTask(ctx context.Context, task ScheduledTask) error
    DeleteScheduledTask(ctx context.Context, taskID string) error
    GetScheduledTask(ctx context.Context, taskID string) (ScheduledTask, bool, error)
    ListScheduledTasks(ctx context.Context) ([]ScheduledTask, error)
    ListDueScheduledTasks(ctx context.Context, now time.Time) ([]ScheduledTask, error)
    AdmitScheduledRun(ctx context.Context, task ScheduledTask, dueAt time.Time) (ScheduledTaskRun, bool, error)
    MarkScheduledRunStarted(ctx context.Context, run ScheduledTaskRun) error
    FinishScheduledRun(ctx context.Context, run ScheduledTaskRun) error
    RecordScheduledDelivery(ctx context.Context, delivery ScheduledTaskDelivery) error

- in `cmd/39claw/main.go`, wire:

  - scheduled-task store implementation
  - local MCP server bootstrap
  - scheduler service startup and shutdown
  - bot-initiated Discord delivery helper

- in `internal/app/task_workspace.go`, extract reusable managed-repository helpers so scheduled runs in `task` mode can create temporary worktrees without creating fake interactive tasks.

- add `github.com/robfig/cron/v3` for cron parsing unless a simpler in-repository parser is introduced and documented here first.

Revision note: Created on 2026-04-12 to lock the implementation approach for `docs/design-docs/scheduled-tasks.md` before feature work begins. The plan intentionally front-loads the local MCP integration proof because that is the main unresolved infrastructure dependency.
