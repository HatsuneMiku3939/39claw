# Add shared daily generation rotation and `action:clear` to `daily` mode

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agents/PLANS.md`.

## Purpose / Big Picture

After this plan, users of a `daily`-mode bot should be able to intentionally reset the current shared same-day conversation without waiting for the next local day. The bot should expose `/<instance-command> action:clear`, rotate the active shared same-day generation to a fresh Codex thread key, and run the durable-memory preflight on the first visible message of that new generation so durable preferences can carry forward through `AGENT_MEMORY/MEMORY.md` plus a generation-scoped bridge note such as `AGENT_MEMORY/2026-04-06.2.md`.

This change matters because `daily` mode currently keeps one remote Codex thread for the whole local day. Long conversations can make that thread heavy, which weakens response quality and makes resets awkward. After this plan, the shared daily experience remains low-friction, but users also gain an explicit way to cut context growth and begin a fresh shared thread mid-day.

## Progress

- [x] (2026-04-06 00:00Z) Reviewed the current `daily` routing, daily-memory, command-surface, and SQLite design and captured the agreed direction for shared same-day generation rotation.
- [x] (2026-04-06 00:00Z) Updated architecture, design, README, and product documents so the repository now describes `daily` mode as one active shared generation per day with `action:clear` and generation-scoped bridge notes.
- [ ] Add explicit `daily_sessions` persistence and migrate legacy `daily` thread bindings from `YYYY-MM-DD` to `YYYY-MM-DD#1`.
- [ ] Resolve the active `daily` generation through store-backed metadata before visible turns load or persist thread bindings.
- [ ] Add `action:clear` to the `daily` command surface and route it through a dedicated app-level daily command service.
- [ ] Extend queue coordination so `action:clear` can reject rotation while the current active generation is busy or queued.
- [ ] Run the durable-memory preflight once per fresh generation and write `AGENT_MEMORY/YYYY-MM-DD.<generation>.md`.
- [ ] Add tests for migration, same-day rotation, clear rejection while busy, same-day preflight after clear, and next-day carry-over from the last active prior-day generation.
- [ ] Run `make test` and `make lint`, then update this plan with proof artifacts and any deferred follow-up work.

## Surprises & Discoveries

- Observation: The current `daily` thread policy still does the right outer-bucket job. It returns only the configured local date, so the smallest safe change is to keep that bucket and add explicit generation metadata on top of it.
  Evidence: `internal/thread/policy.go`

- Observation: The current daily-memory preflight assumes the logical thread key itself is a date and looks up only the previous calendar day. That assumption breaks as soon as one day can contain multiple fresh starts.
  Evidence: `internal/dailymemory/service.go`

- Observation: The Discord root command already centralizes discovery for both modes. Extending it with `action:clear` is cleaner than introducing a second standalone slash command.
  Evidence: `internal/runtime/discord/commands.go`, `internal/runtime/discord/runtime.go`

## Decision Log

- Decision: Keep the `daily` thread policy focused on the local-date bucket and resolve the active shared generation later in the application layer.
  Rationale: The current policy contract is already correct for “what day is this message in?” and changing it would enlarge the implementation blast radius. A second store-backed daily-session layer is the smallest coherent change.
  Date/Author: 2026-04-06 / Codex

- Decision: Use `/<instance-command> action:clear` instead of a standalone `/clear`.
  Rationale: The repository already standardizes on one instance-specific root command. Reusing that surface keeps command discovery, help output, and runtime routing consistent.
  Date/Author: 2026-04-06 / Codex

- Decision: Model `daily` as one active shared generation per local date and persist that metadata explicitly.
  Rationale: Inferring the current generation from `updated_at` or other implicit heuristics would be fragile and hard to test. Explicit state makes migration, rotation, and preflight sourcing deterministic.
  Date/Author: 2026-04-06 / Codex

- Decision: Reject `action:clear` while the current shared generation still has in-flight or queued work.
  Rationale: Allowing rotation while old replies are still pending would create confusing post-clear replies from the old generation. A safe rejection is easier to explain and preserves reply ordering.
  Date/Author: 2026-04-06 / Codex

- Decision: Store bridge notes as `AGENT_MEMORY/YYYY-MM-DD.<generation>.md`.
  Rationale: One date can now have multiple fresh starts, so the bridge note must identify which generation it belongs to while keeping `MEMORY.md` as the single durable summary file.
  Date/Author: 2026-04-06 / Codex

## Outcomes & Retrospective

Implementation has not started yet. The intended outcome is a `daily` mode that still behaves like a shared day-based assistant but can intentionally rotate to a fresh same-day Codex thread when conversation length becomes a problem. This plan will be complete when the runtime can persist active daily generations, expose `action:clear`, reject clear while busy, refresh durable memory from the previous recorded generation, and prove the full behavior through automated tests.

## Context and Orientation

39claw is a thin Discord-to-Codex gateway. The current `daily` implementation uses a single logical thread key per configured local date. That flow is spread across these files:

- `internal/thread/policy.go`
  - resolves the `daily` bucket as `YYYY-MM-DD`
- `internal/app/message_service_impl.go`
  - prepares normal messages, runs the daily-memory preflight, loads or creates thread bindings, and sends the visible Codex turn
- `internal/dailymemory/service.go`
  - runs the hidden preflight refresh and currently assumes one thread per local day
- `internal/store/sqlite/store.go`
  - creates the SQLite schema and persists `thread_bindings`, `tasks`, and `active_tasks`
- `internal/runtime/discord/commands.go`
  - defines the slash-command choices and help output
- `internal/runtime/discord/interaction_mapper.go`
  - normalizes Discord slash-command inputs into command requests
- `internal/runtime/discord/runtime.go`
  - routes commands and normal messages into the application layer
- `docs/product-specs/daily-mode-user-flow.md`
  - explains the intended user-facing `daily` behavior

For this plan, these terms are important:

A “daily bucket” is the configured local date string such as `2026-04-06`. It is the outer grouping for `daily` mode.

A “daily generation” is one concrete shared visible conversation within that bucket, identified by a logical thread key such as `2026-04-06#2`.

The “active generation” is the only same-day generation that new normal messages should target.

A “bridge note” is the generation-scoped Markdown file created or refreshed during preflight, for example `AGENT_MEMORY/2026-04-06.2.md`.

A “legacy daily binding” is a pre-migration `thread_bindings` row whose logical thread key is only `YYYY-MM-DD`.

The architecture must stay thin. `thread_bindings` remain the source of truth for actual Codex thread IDs. New daily-only metadata should describe which same-day generation is active and which previous generation should be used as the preflight source.

## Starting State

Start this plan only after confirming the repository still matches these assumptions:

- `internal/thread/policy.go` still resolves `daily` as only the configured local date
- `internal/app/message_service_impl.go` still runs the daily-memory preflight before visible thread-binding lookup
- `internal/dailymemory/service.go` still expects a date-like logical key and writes `AGENT_MEMORY/YYYY-MM-DD.md`
- `internal/runtime/discord/commands.go` still owns the root command choices and currently exposes `action:help` only for `daily` mode
- `internal/store/sqlite/store.go` still creates `thread_bindings`, `tasks`, and `active_tasks`
- `make test` and `make lint` pass before implementation work begins

Verify that state with:

    cd /home/filepang/playground/39claw
    make test
    make lint

If the repository has drifted away from that shape, update this ExecPlan first so it remains self-contained and truthful.

## Preconditions

This plan fixes the following product and implementation choices:

- `daily` mode remains shared for the whole bot instance, not per-user
- `action:clear` rotates the shared same-day generation for the whole instance
- the first generation for any new local date is always `#1`
- each local date has at most one active generation at a time
- `action:clear` must fail safely when the current generation still has in-flight or queued work
- the first visible message of a fresh generation runs the hidden durable-memory preflight when a previous recorded generation exists
- `MEMORY.md` remains the primary durable-memory file
- bridge notes are generation-scoped and use `AGENT_MEMORY/YYYY-MM-DD.<generation>.md`

## Milestone 1: Add explicit daily-session persistence and migration

At the end of this milestone, SQLite should persist which same-day generation is active for each local date, and legacy `daily` thread bindings should be normalized from `YYYY-MM-DD` to `YYYY-MM-DD#1`.

Add a new table in `internal/store/sqlite/store.go` named `daily_sessions` with these columns:

- `local_date TEXT NOT NULL`
- `generation INTEGER NOT NULL`
- `logical_thread_key TEXT NOT NULL`
- `previous_logical_thread_key TEXT NULL`
- `activation_reason TEXT NOT NULL`
- `is_active INTEGER NOT NULL DEFAULT 1`
- `created_at TEXT NOT NULL`
- `updated_at TEXT NOT NULL`

Use `PRIMARY KEY (local_date, generation)` and `UNIQUE (logical_thread_key)`. Add a partial unique index so only one row per `local_date` may have `is_active = 1`.

Extend the store API so the application layer can:

- load the active daily generation for a local date
- load the latest recorded generation before a local date
- create generation `#1` when a date has not been seen before
- rotate to the next same-day generation transactionally

During schema initialization, add a migration that rewrites legacy `daily` thread-binding keys from `YYYY-MM-DD` to `YYYY-MM-DD#1` and backfills matching active `daily_sessions` rows. The migration must be idempotent.

## Milestone 2: Resolve the active generation before visible turns

At the end of this milestone, normal `daily` messages should target the active generation key such as `2026-04-06#2`, not the bare date bucket.

Keep `internal/thread/policy.go` unchanged for `daily`. It should still return only the local-date bucket string.

Introduce a small daily-session resolver in the application layer. It should accept the bucket date and do the following:

- if an active generation exists for that date, return it
- if no active generation exists, create generation `#1`
- when creating generation `#1`, record the most recent active generation from an earlier day as `previous_logical_thread_key` if one exists

Update `internal/app/message_service_impl.go` so the visible `daily` path resolves a generation before it calls the preflight or loads a thread binding. The actual `thread_bindings` key used for `daily` turns must always be the generation key such as `2026-04-06#2`.

## Milestone 3: Add `action:clear` and reject it while busy

At the end of this milestone, `daily` mode should expose `/<instance-command> action:clear`, and invoking it on an idle active generation should rotate the shared same-day session to the next generation.

Add `action:clear` to:

- `internal/runtime/discord/interaction_mapper.go`
- `internal/runtime/discord/commands.go`
- `internal/runtime/discord/runtime.go`

Do not route this through the task command service. Add a dedicated app-level daily command service, for example `DailyCommandService`, with one method equivalent to:

    Clear(ctx context.Context, userID string, receivedAt time.Time) (MessageResponse, error)

That service must:

- resolve today's active generation
- inspect the queue state for that logical key
- reject the clear with an ephemeral retry-later response if the current generation has in-flight or queued work
- otherwise rotate to the next same-day generation and confirm success

To support this safety check, extend the queue coordinator interface with a lightweight status or snapshot method that reports whether a key is in flight and how many items are queued.

## Milestone 4: Run durable-memory preflight per generation

At the end of this milestone, the first visible message of any fresh generation should run the hidden durable-memory refresh from the immediately previous recorded generation rather than only from the previous calendar day.

Change the daily-memory refresher contract so it receives resolved generation metadata rather than just a date-like logical key. Then update `internal/dailymemory/service.go` to:

- skip preflight if the current generation already has a thread binding
- skip preflight if `previous_logical_thread_key` is empty
- otherwise load the previous generation's thread binding directly by that key
- create or preserve `AGENT_MEMORY/MEMORY.md`
- create or preserve `AGENT_MEMORY/YYYY-MM-DD.<generation>.md`
- run the hidden Codex turn against the previous generation's thread ID

Keep the preflight idempotence rule simple: once the fresh generation has a visible thread binding, that generation's first-turn preflight has already happened or was intentionally skipped.

## Milestone 5: Update tests and validate the full behavior

At the end of this milestone, the repository should prove the new `daily` behavior through automated tests and aligned documentation.

Add or update tests in these areas:

- `internal/store/sqlite/store_test.go`
  - legacy daily-key migration to `#1`
  - one active generation per date
  - same-day rotation to `#2`
  - prior-generation lookup
- `internal/app/message_service_test.go`
  - first same-day message creates generation `#1`
  - follow-up same-day message reuses the active generation
  - first visible message after clear targets a fresh generation
  - same-day preflight after clear uses the immediately previous generation
  - first message on a new day uses generation `#1` and preflights from the last active prior-day generation
- `internal/dailymemory/service_test.go`
  - generation-scoped bridge note paths
  - previous-generation sourcing
  - existing-thread-binding short-circuit
- `internal/runtime/discord/runtime_test.go`
  - `daily` help includes `action:clear`
  - `action:clear` succeeds when idle
  - `action:clear` is rejected ephemerally when the current generation is busy or queued
- queue tests under `internal/thread`
  - queue snapshot behavior used by clear safety checks

Finish by running `make test` and `make lint`, then update this plan's `Progress`, `Surprises & Discoveries`, and `Outcomes & Retrospective` sections with the actual results.

## Plan of Work

Begin in `internal/store/sqlite/store.go`. Add the `daily_sessions` table, active-generation uniqueness, and the legacy `daily` key migration. Extend the app-facing store contract with focused daily-session methods rather than overloading task methods.

Next, introduce a small app-level daily-session resolver and use it from `internal/app/message_service_impl.go` before the visible turn loads a thread binding or calls the daily-memory refresher. The visible `daily` path should always operate on the generation key, not on the raw bucket string.

Then extend the root command surface with `action:clear` and add a dedicated daily command service. Wire that service through `cmd/39claw/main.go` and `internal/runtime/discord/runtime.go`. Extend the queue coordinator with a snapshot method so the clear path can fail safely while the current generation is busy.

After that, change the daily-memory preflight to operate per generation and to write generation-scoped bridge notes. Make sure same-day clear uses the immediately previous generation as the durable-memory source and that the next local day uses the last active generation from the previous day.

Finally, update tests and rerun the full checks. This plan is not complete until the repository docs, behavior, and automated proofs all agree.

## Concrete Steps

Run all commands from `/home/filepang/playground/39claw`.

1. Confirm the starting state and baseline health.

    make test
    make lint

2. Implement the store and migration changes, then run focused store tests.

    go test ./internal/store/sqlite -run 'TestStore' -v

3. Implement daily-generation resolution and generation-scoped preflight, then run the focused app and memory tests.

    go test ./internal/app ./internal/dailymemory -run 'TestMessageService|TestRefresher' -v

4. Implement `action:clear` and the busy-clear rejection path, then run the runtime and queue tests.

    go test ./internal/runtime/discord ./internal/thread -run 'TestRuntime|TestQueue' -v

5. Run the full repository checks.

    make test
    make lint

6. Record concise proof artifacts in this plan before marking it complete.

## Validation and Acceptance

This plan is complete when all of the following are true:

- SQLite persists explicit same-day `daily` generation state
- legacy `daily` thread-binding keys are migrated from `YYYY-MM-DD` to `YYYY-MM-DD#1`
- each local date has at most one active generation
- the first same-day message for a fresh date uses generation `#1`
- same-day follow-up messages reuse the active generation
- `/<instance-command> action:clear` exists in `daily` mode
- `action:clear` rotates the shared same-day generation only when the current generation is idle
- `action:clear` is rejected ephemerally when the current generation has in-flight or queued work
- the first visible message after clear starts or resumes a fresh same-day generation key and runs preflight from the immediately previous generation when that generation has a binding
- the first visible message on a new local day starts generation `#1` and uses the last active prior-day generation as the preflight source
- bridge notes are written to `AGENT_MEMORY/YYYY-MM-DD.<generation>.md`
- `MEMORY.md` remains the primary durable-memory file
- `make test` passes
- `make lint` passes

The most important human-readable proof scenario is:

1. Mention the bot on `2026-04-06` and create visible generation `2026-04-06#1`.
2. Invoke `/<instance-command> action:clear`.
3. Mention the bot again on `2026-04-06`.
4. Observe that the visible turn uses a fresh thread binding for `2026-04-06#2`.
5. Observe that the hidden preflight ran against the `2026-04-06#1` thread and wrote `AGENT_MEMORY/2026-04-06.2.md`.

The most important safety proof is:

1. Start one visible turn for the current active generation.
2. Queue a second visible turn for that same generation.
3. Invoke `action:clear`.
4. Observe an ephemeral rejection that tells the user the current shared generation is still busy.
5. Observe that the active generation does not rotate until the prior work has drained.

## Idempotence and Recovery

The schema migration must be safe to run more than once. Only legacy daily keys without `#` should be rewritten, and daily-session backfill should use conflict-safe inserts.

If implementation is interrupted after the migration but before the application and runtime code are updated, do not ship that partial state. Finish the app-layer and runtime work in the same branch so the migrated data shape and the running code agree.

If the busy-clear rejection path becomes unexpectedly invasive, update this plan before weakening the contract. Do not silently ship a version that allows `action:clear` to interleave with old queued replies.

## Artifacts and Notes

Useful key transitions to keep visible in tests:

    2026-04-06 first visible message -> active generation 2026-04-06#1
    2026-04-06 action:clear -> active generation 2026-04-06#2
    2026-04-06 next visible message -> preflight from 2026-04-06#1, visible binding persisted for 2026-04-06#2
    2026-04-07 first visible message -> active generation 2026-04-07#1, preflight from the last active 2026-04-06 generation

Useful bridge note paths:

    AGENT_MEMORY/2026-04-06.1.md
    AGENT_MEMORY/2026-04-06.2.md
    AGENT_MEMORY/2026-04-07.1.md

Suggested success response for `action:clear`:

    Started a fresh shared daily session for today. Your next mention will use a new thread.

Suggested rejection response for `action:clear` while busy:

    The current shared daily session is still busy. Please wait for queued work to finish, then retry `action:clear`.

## Interfaces and Dependencies

Add a new daily-session type in `internal/app`:

    type DailySession struct {
        LocalDate                string
        Generation               int
        LogicalThreadKey         string
        PreviousLogicalThreadKey string
        ActivationReason         string
        IsActive                 bool
        CreatedAt                time.Time
        UpdatedAt                time.Time
    }

Extend the app-facing store contract with daily-session methods equivalent to:

    GetActiveDailySession(ctx context.Context, localDate string) (DailySession, bool, error)
    GetLatestDailySessionBefore(ctx context.Context, localDate string) (DailySession, bool, error)
    CreateDailySession(ctx context.Context, session DailySession) error
    RotateDailySession(ctx context.Context, localDate string, next DailySession) error

Extend the queue coordinator with a lightweight status shape:

    type QueueSnapshot struct {
        InFlight bool
        Queued   int
    }

    Snapshot(key string) QueueSnapshot

Change the daily-memory refresher boundary to accept the resolved generation metadata instead of a bare date-like logical key.

Add an app-level daily command service interface shaped like:

    type DailyCommandService interface {
        Clear(ctx context.Context, userID string, receivedAt time.Time) (MessageResponse, error)
    }

Keep the Discord runtime thin. It should normalize command input, call the correct application service, and present the response. All same-day generation and busy-clear business rules belong in the application and store layers.

Revision Note: 2026-04-06 / Codex - Created this active ExecPlan after agreeing that `daily` clear rotates the whole shared bot-instance conversation, not per-user state, and that busy same-day generations must reject clear requests instead of rotating immediately.
