# Add a durable memory bridge to `daily` mode

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agents/PLANS.md`.

## Purpose / Big Picture

After this plan, the first user message on a new local day in `daily` mode should feel continuous instead of feeling like a total reset. 39claw should resume the previous day's Codex thread before the first new-day user-visible turn, run a dedicated memory refresh workflow against that previous thread, and project the durable facts into Markdown files under `CLAW_CODEX_WORKDIR/AGENT_MEMORY`. The new day's Codex thread should then be free to start fresh while still consulting that projected memory.

The user-visible proof should be practical. A user should be able to tell the bot a durable preference on one day, send the first message on the next day, and receive a response that reflects the remembered preference without having to restate it. On disk, the operator should be able to inspect `${CLAW_CODEX_WORKDIR}/AGENT_MEMORY/MEMORY.md` and `${CLAW_CODEX_WORKDIR}/AGENT_MEMORY/YYYY-MM-DD.md` and see the durable memory projection that made the carry-over possible.

## Progress

- [x] (2026-04-05 01:19Z) Captured the agreed product direction for a `daily` memory bridge, including the `MEMORY.md + YYYY-MM-DD.md` storage model and app-driven preflight refresh.
- [x] (2026-04-05 01:26Z) Expanded the plan with a precise managed skill contract, exact bootstrap targets, and exact memory-file templates so the runtime-injected skill can be implemented without reopening format questions.
- [x] (2026-04-05 01:41Z) Reworked the plan to use the fixed in-workdir `AGENT_MEMORY` directory instead of an external facts directory and removed the `CLAW_FACTS_DIR` contract.
- [ ] Update architecture, design, and product documents so `daily` mode explicitly supports a durable memory bridge instead of a pure date-boundary reset.
- [ ] Add startup bootstrap code that materializes the managed memory skill and managed `AGENTS.md` block in the configured daily-mode workdir.
- [ ] Add startup validation that requires at least `workspace-write` sandboxing whenever the daily memory bridge is enabled.
- [ ] Implement the `daily` preflight coordinator that refreshes memory from the previous daily thread before the first new-day user-visible turn.
- [ ] Implement graceful fallback behavior when the preflight refresh fails or times out.
- [ ] Add unit and integration coverage for bootstrap, preflight, fallback, and cross-day continuity behavior.
- [ ] Run `make test` and `make lint` after the implementation lands.

## Surprises & Discoveries

- Observation: The existing startup path already centralizes Codex client creation and thread-option assembly in `cmd/39claw/main.go`, so bootstrap and sandbox validation changes can stay near current startup wiring instead of spreading across the Discord runtime.
  Evidence: `cmd/39claw/main.go`, `internal/codex/client.go`, and `internal/codex/exec.go`

- Observation: `daily` mode queueing already serializes work on today's logical thread key, so the first message of a new day can perform a preflight refresh inside the normal execution slot without adding a second queueing system.
  Evidence: `internal/app/message_service_impl.go` and `internal/thread/policy.go`

- Observation: The current `daily` product spec still promises a clean fresh start on a new day unless the user re-supplies context, so the product and architecture documents must be updated before implementation or the repository will contradict the agreed direction.
  Evidence: `ARCHITECTURE.md`, `docs/design-docs/thread-modes.md`, and `docs/product-specs/daily-mode-user-flow.md`

- Observation: Keeping memory files inside `CLAW_CODEX_WORKDIR` removes the need for a separate discovery environment variable or extra writable roots, but it also means the feature depends on a write-capable sandbox mode.
  Evidence: `cmd/39claw/main.go`, `internal/codex/exec.go`, and the current Codex CLI sandbox model

## Decision Log

- Decision: The durable memory projection for `daily` mode will use `${CLAW_CODEX_WORKDIR}/AGENT_MEMORY/MEMORY.md` plus `${CLAW_CODEX_WORKDIR}/AGENT_MEMORY/YYYY-MM-DD.md` files.
  Rationale: This keeps durable memory inside the same workspace Codex already understands, removes the need for a separate discovery environment variable, and still preserves a compact primary memory surface plus searchable per-day bridge notes.
  Date/Author: 2026-04-05 / Codex

- Decision: The runtime, not the user-facing `AGENTS.md` instructions alone, will force the previous-thread memory refresh before the first visible turn of a new day.
  Rationale: The user's primary goal is reliable continuity, so memory refresh must be a product behavior rather than an optional agent habit.
  Date/Author: 2026-04-05 / Codex

- Decision: The memory directory path will be fixed to `AGENT_MEMORY` under `CLAW_CODEX_WORKDIR`, and no separate `CLAW_FACTS_DIR` environment variable will be used.
  Rationale: A fixed workspace-relative location is simpler for both runtime bootstrap and Codex prompt instructions, and the user wants these files to remain directly visible and editable inside the workdir.
  Date/Author: 2026-04-05 / Codex

- Decision: The workdir `AGENTS.md` file will contain a runtime-managed block that points directly at `AGENT_MEMORY/MEMORY.md` and `AGENT_MEMORY/YYYY-MM-DD.md`, but normal conversation turns will not be responsible for performing the refresh workflow.
  Rationale: The agent should always know where memory lives, yet the refresh itself must remain a runtime-controlled preflight step so the user experiences continuity consistently.
  Date/Author: 2026-04-05 / Codex

- Decision: The once-per-day preflight gate will be derived from the absence of today's thread binding rather than from a separate persisted preflight table.
  Rationale: If today's binding already exists, the first-turn refresh has already happened or was intentionally bypassed. If preflight succeeds and the process crashes before today's thread is created, rerunning the idempotent refresh on the next attempt is acceptable and keeps persistence simpler.
  Date/Author: 2026-04-05 / Codex

- Decision: Startup bootstrap failures for the managed skill, managed `AGENTS.md` block, or `AGENT_MEMORY` directory should fail daily-mode startup, but preflight execution failures should degrade continuity rather than availability.
  Rationale: The runtime should not advertise a configured durable-memory system that it cannot initialize at all, but transient refresh failures should not block the bot from replying to users.
  Date/Author: 2026-04-05 / Codex

- Decision: The daily memory bridge requires Codex sandbox mode `workspace-write` or `danger-full-access`; `read-only` is not supported when the feature is enabled.
  Rationale: The feature must create and update files inside `CLAW_CODEX_WORKDIR/AGENT_MEMORY`, so a write-capable sandbox is an explicit product constraint rather than an implementation accident.
  Date/Author: 2026-04-05 / Codex

- Decision: The runtime-managed memory refresh skill will be a self-contained single-file skill with no bundled asset templates.
  Rationale: A one-file skill is easier to materialize into the configured workdir, easier to diff when the managed content changes, and avoids creating extra runtime-managed files whose only purpose would be to hold static templates that can instead be spelled out in the skill contract below.
  Date/Author: 2026-04-05 / Codex

- Decision: The preflight prompt will explicitly tell Codex to read the managed skill file first instead of embedding the full refresh workflow every time.
  Rationale: This keeps the per-turn prompt short while still letting the runtime fully control which exact skill instructions are used.
  Date/Author: 2026-04-05 / Codex

## Outcomes & Retrospective

This plan is not yet implemented. At this stage, the repository has a clear direction for how durable memory should fit a Codex-native `daily` mode: memory generation is a runtime-controlled preflight on the previous thread, memory consumption happens through ordinary Markdown files plus `AGENTS.md` guidance, and failure handling prioritizes replying to the user even when continuity refresh fails.

The main remaining risks are making runtime-managed workdir edits safe, keeping the memory refresh prompt idempotent, and updating the current documentation so it no longer promises a hard fresh reset at day boundaries.

## Context and Orientation

39claw is a thin Discord-to-Codex gateway. In `daily` mode today, the logical thread key is just the configured local date. When the date changes, the message service computes a new key, finds no stored binding for that key, and starts a brand-new Codex thread. This keeps the implementation simple, but it also means durable context disappears unless the user manually repeats it.

This plan introduces a durable memory bridge without turning `daily` mode into `task` mode. The new day still gets a new Codex thread. The difference is that the runtime first resumes the previous daily thread, asks Codex to refresh a durable Markdown memory projection, and only then starts the new day's visible conversation.

Terms used in this plan:

- durable memory: information likely to matter on a future day, such as user preferences, standing workflow expectations, or long-lived project context
- bridge note: the dated Markdown file for the current day that records what was promoted from the previous thread into durable memory
- preflight: an internal Codex turn that runs before the first user-visible turn of a new day
- managed `AGENTS.md` block: a runtime-owned section inside the configured workdir's `AGENTS.md` file that 39claw can create or update without overwriting unrelated user-authored instructions
- memory directory: `${CLAW_CODEX_WORKDIR}/AGENT_MEMORY`, created and maintained by the runtime inside the Codex workspace

The most relevant files today are:

- `cmd/39claw/main.go`
  - startup wiring, Codex client creation, and thread option assembly
- `internal/config/config.go`
  - environment loading and `CLAW_DATADIR` / `CLAW_CODEX_WORKDIR` configuration
- `internal/app/message_service_impl.go`
  - normal message orchestration and the best current insertion point for a preflight step
- `internal/app/types.go`
  - message, thread binding, and task data shapes
- `internal/thread/policy.go`
  - `daily` key derivation based on local date
- `internal/codex/client.go` and `internal/codex/exec.go`
  - Codex child-process environment assembly and CLI argument construction
- `internal/store/sqlite/store.go`
  - persistent thread binding storage
- `ARCHITECTURE.md`
  - authoritative description of `daily` and `task` mode behavior
- `docs/design-docs/thread-modes.md`
  - concise explanation of the current thread mode tradeoffs
- `docs/product-specs/daily-mode-user-flow.md`
  - current user promise for same-day continuity and date-boundary reset

## Plan of Work

Start by updating the repository documents that define `daily` mode. `ARCHITECTURE.md`, `docs/design-docs/thread-modes.md`, `docs/design-docs/implementation-spec.md`, `docs/design-docs/state-and-storage.md`, and `docs/product-specs/daily-mode-user-flow.md` must all explain that a new day now means a fresh Codex thread plus a runtime-managed durable memory bridge. Keep the distinction explicit: the remote thread resets, but durable memory can carry forward through Markdown files under `${CLAW_CODEX_WORKDIR}/AGENT_MEMORY`.

Next, add a small startup bootstrap layer for daily mode. This layer should derive `memoryDir := filepath.Join(cfg.CodexWorkdir, "AGENT_MEMORY")`, ensure the directory exists, ensure `MEMORY.md` exists, and materialize runtime-managed artifacts inside `CLAW_CODEX_WORKDIR`: a dedicated daily-memory skill under `.agents/skills/<stable-skill-name>/` and a managed block inside `AGENTS.md`. If `AGENTS.md` does not yet exist, create it. If it already exists, replace only the content between stable markers such as `<!-- 39claw:daily-memory start -->` and `<!-- 39claw:daily-memory end -->`. The managed block should explain that durable memory lives under `AGENT_MEMORY/`, that `MEMORY.md` is the primary source, that the latest dated note is secondary bridge context, and that the latest explicit user instruction overrides memory. The same bootstrap or validation path should reject `read-only` sandbox mode for deployments that enable this feature.

Then add a focused daily-memory service in the application layer. This service should accept the configured timezone, the thread store, and the Codex gateway. Given an incoming `daily` request, it should compute today's date key and the previous day's date key. If today's binding already exists, it should do nothing. If today's binding is missing and the previous day's binding is also missing, it should do nothing. If today's binding is missing and the previous day's binding exists, it should build the current day's bridge filename, construct the internal refresh prompt, and run one internal Codex turn against the previous day's thread ID before the visible turn continues.

The refresh prompt should be deterministic and idempotent. It should instruct Codex to read and update `AGENT_MEMORY/MEMORY.md` and `AGENT_MEMORY/YYYY-MM-DD.md`, preserve only durable facts, prefer explicit user statements over inference, avoid transient chatter and temporary TODO items, and revise existing memory rather than appending near-duplicates. The prompt should mention the exact bridge filename for the current local day so the runtime does not rely on the agent guessing the date.

After that, integrate the preflight step into `internal/app/message_service_impl.go`. The cleanest place is after the logical thread key is resolved but before the visible `gateway.RunTurn` call. Reuse the existing queue coordinator so the first message of the new day occupies the same serialized execution slot as any queued follow-ups. The preflight should not generate any user-facing message. If it succeeds, proceed to the normal visible turn. If it fails, log the failure, skip the memory refresh for that attempt, and still proceed to the visible turn so users are not blocked.

Finally, add tests and proofs. Cover startup bootstrap idempotence, managed `AGENTS.md` block replacement, startup validation for write-capable sandboxing, previous-thread preflight invocation, fallback when the previous day's thread is missing, fallback when refresh fails, and a behavioral case where a preference established on day one is projected into day two memory before the first visible turn.

## Managed Skill Contract

The runtime-managed skill must be written into the configured daily-mode workdir at exactly:

- `.agents/skills/39claw-daily-memory-refresh/SKILL.md`

No additional managed asset files are required for this skill. The runtime should overwrite this file with the exact managed content whenever startup bootstrap runs.

The managed skill must use this frontmatter and section structure:

    ---
    name: 39claw-daily-memory-refresh
    description: Refresh durable Markdown memory for 39claw daily mode before the first visible turn of a new local day. Use when the runtime resumes the previous daily Codex thread and needs to update the durable memory files under AGENT_MEMORY.
    ---

    # 39claw Daily Memory Refresh

    ## Purpose

    Refresh durable memory before the first visible turn of a new local day in 39claw daily mode.

    The source of truth is the resumed previous daily thread.
    The writable memory directory is `AGENT_MEMORY/` under the current workspace.

    ## Files

    Read and update these files:

    - `AGENT_MEMORY/MEMORY.md`
    - the dated bridge note path provided in the runtime prompt

    Treat `MEMORY.md` as the primary durable memory file.
    Treat the dated bridge note as a record of what was promoted or rejected during today's refresh.

    ## Rules

    - Preserve only durable facts that are likely to matter on a future day.
    - Prefer explicit user statements over inferred conclusions.
    - Do not store transient chatter, jokes, or temporary TODO items.
    - Update existing memory instead of appending duplicate facts.
    - Keep `MEMORY.md` concise and current.
    - If a new fact replaces an older one, revise the older wording in `MEMORY.md`.
    - If memory conflicts with the latest explicit user instruction, the latest explicit user instruction wins.

    ## Required `MEMORY.md` structure

    Ensure `MEMORY.md` uses exactly these top-level headings:

    - `# Memory`
    - `## User Preferences`
    - `## Workflow Preferences`
    - `## Active Long-Lived Context`
    - `## Superseded or Historical Notes`

    Keep each section short and scannable.
    Use flat bullet lists under the sections.

    ## Required dated bridge note structure

    Ensure today's dated note uses exactly these top-level headings:

    - `# Daily Memory Bridge for YYYY-MM-DD`
    - `## Source`
    - `## Durable Facts Promoted`
    - `## MEMORY.md Updates Applied`
    - `## Rejected Candidates`
    - `## Notes`

    The `## Source` section must name the previous thread ID and the previous local date.

    ## Completion format

    After updating the files, reply with plain text in exactly this shape:

        MEMORY_REFRESH_OK
        Updated:
        - <absolute path to MEMORY.md>
        - <absolute path to today's dated note>

    If no durable facts changed, still return the same format and list both files.

The runtime-generated preflight prompt must tell Codex to read that exact skill file first and must name today's bridge note explicitly. Use this exact prompt shape, with the workspace-relative memory paths and local dates substituted by the runtime:

    Before handling the first visible user message of the new daily thread, read `.agents/skills/39claw-daily-memory-refresh/SKILL.md` and follow it now.

    Use the resumed previous daily thread as the source of truth.

    Today's bridge note path is:
    - AGENT_MEMORY/YYYY-MM-DD.md

    The primary durable memory file is:
    - AGENT_MEMORY/MEMORY.md

    The previous local date is YYYY-MM-DD.
    The new local date is YYYY-MM-DD.

    Return the required completion format after the refresh is complete.

When bootstrap creates `MEMORY.md` for the first time, it must use this exact initial content:

    # Memory

    ## User Preferences

    - None recorded yet.

    ## Workflow Preferences

    - None recorded yet.

    ## Active Long-Lived Context

    - None recorded yet.

    ## Superseded or Historical Notes

    - None recorded yet.

When preflight creates today's dated bridge note for the first time, it must use this exact initial content before Codex edits it:

    # Daily Memory Bridge for YYYY-MM-DD

    ## Source

    - Previous thread id: `<previous-thread-id>`
    - Source day: `YYYY-MM-DD`

    ## Durable Facts Promoted

    - None yet.

    ## MEMORY.md Updates Applied

    - None yet.

    ## Rejected Candidates

    - None yet.

    ## Notes

    - Created by the 39claw daily memory preflight before the first visible turn of the new day.

## Concrete Steps

Run all commands from `/home/filepang/.codex/worktrees/3065/39claw`.

1. Update the repository documents that define `daily` mode and storage behavior.

   Edit:

   - `ARCHITECTURE.md`
   - `docs/design-docs/thread-modes.md`
   - `docs/design-docs/implementation-spec.md`
   - `docs/design-docs/state-and-storage.md`
   - `docs/product-specs/daily-mode-user-flow.md`
   - `README.md` if operator-facing setup needs a durable memory note

2. Add startup bootstrap logic and supporting helpers in:

   - `cmd/39claw/main.go`
   - `internal/config/config.go` and tests only if new derived helpers or validation are added there
   - a new focused package or files such as `internal/dailymemory/bootstrap.go`

3. Add daily-memory preflight orchestration in:

   - `internal/app/message_service_impl.go`
   - a new focused package or files such as `internal/dailymemory/service.go`
   - `internal/app/types.go` only if new normalized helper types make the preflight easier to test

4. Add or update tests in:

   - `cmd/39claw/main_test.go`
   - `internal/app/message_service_test.go`
   - `internal/config/config_test.go` if configuration helpers change
   - package-specific tests for managed-file bootstrap and prompt generation

5. Run focused tests while iterating.

   go test ./cmd/39claw ./internal/app ./internal/config ./internal/codex

   Expected result:

       ok   github.com/HatsuneMiku3939/39claw/cmd/39claw
       ok   github.com/HatsuneMiku3939/39claw/internal/app
       ok   github.com/HatsuneMiku3939/39claw/internal/config
       ok   github.com/HatsuneMiku3939/39claw/internal/codex

6. Run the full repository checks after the implementation lands.

   make test
   make lint

7. Record proof artifacts showing:

   - the managed `AGENTS.md` block points Codex at `AGENT_MEMORY/MEMORY.md`
   - startup rejects `read-only` sandbox mode when the daily memory bridge is enabled
   - the first new-day message triggers a refresh turn against the previous day's thread before the visible turn
   - a failed refresh still yields a normal user-visible response

## Validation and Acceptance

This plan is complete when all of the following are true:

- `daily` mode startup materializes a managed durable-memory skill and a managed `AGENTS.md` block inside the configured workdir without overwriting unrelated user-authored instructions
- startup rejects `read-only` sandbox mode when the daily memory bridge is enabled
- `${CLAW_CODEX_WORKDIR}/AGENT_MEMORY/MEMORY.md` is treated as the primary durable-memory file
- `${CLAW_CODEX_WORKDIR}/AGENT_MEMORY/YYYY-MM-DD.md` is refreshed for the current local day during the first-message preflight when a previous daily thread exists
- the first visible user turn of a new day starts a fresh Codex thread after the preflight completes
- if no previous daily thread exists, the bot skips preflight and still starts a fresh thread normally
- if the preflight refresh fails or times out, the bot still answers the user and does not get stuck in a retry loop inside one request
- memory guidance in the managed `AGENTS.md` block tells Codex to prefer the latest explicit user instruction over memory when they conflict
- `make test` passes
- `make lint` passes

Acceptance is behavioral. A contributor should be able to inspect the `AGENT_MEMORY` directory on disk, observe the previous-thread refresh in tests or logs, and then verify that the next day's first user-visible answer reflects durable memory without reusing the previous day's remote thread.

## Idempotence and Recovery

Startup bootstrap must be safe to rerun. Recreating the `AGENT_MEMORY` directory should succeed when it already exists. Rewriting the managed skill files should replace only runtime-owned content. Rewriting the managed `AGENTS.md` block should preserve all content outside the stable markers.

The preflight refresh itself must be safe to retry. If the process crashes after a successful refresh but before today's first visible thread binding is created, the next first-message attempt may run the refresh again. That is acceptable as long as the memory prompt updates files deterministically instead of blindly appending duplicates.

If bootstrap fails because the workdir cannot be written, daily-mode startup should fail fast with a clear error rather than silently running without the promised durable-memory contract. If the preflight refresh fails later because Codex or storage is temporarily unavailable, recover by logging the failure and continuing with the visible turn.

## Artifacts and Notes

Important expected sequence after this plan:

    user sends the first mention of 2026-04-06
    -> message service resolves today's key "2026-04-06"
    -> today's binding is missing
    -> previous binding for "2026-04-05" exists
    -> runtime resumes the previous thread and runs the daily memory refresh prompt
    -> Codex updates:
         AGENT_MEMORY/MEMORY.md
         AGENT_MEMORY/2026-04-06.md
    -> runtime starts the visible 2026-04-06 daily thread
    -> user receives a normal reply that can rely on the refreshed memory

    Important managed `AGENTS.md` block shape:

    <!-- 39claw:daily-memory start -->
    Durable memory files are stored under `AGENT_MEMORY/` in the current workspace.
    Read `AGENT_MEMORY/MEMORY.md` as the primary durable memory file.
    Read the most relevant `AGENT_MEMORY/YYYY-MM-DD.md` note when bridge context is needed.
    If memory conflicts with the latest explicit user instruction, follow the latest explicit user instruction.
    <!-- 39claw:daily-memory end -->

## Interfaces and Dependencies

At the end of this plan, the repository should expose a focused bootstrap helper with responsibilities equivalent to:

    type DailyMemoryBootstrap struct {
        Workdir   string
        MemoryDir string
    }

    func (b DailyMemoryBootstrap) Ensure(ctx context.Context) error

This helper should create or refresh the managed daily-memory skill files and the managed `AGENTS.md` block inside the configured workdir.

The application layer should also expose a daily-memory preflight helper with responsibilities equivalent to:

    type DailyMemoryRefresher interface {
        RefreshBeforeFirstDailyTurn(ctx context.Context, request MessageRequest) error
    }

The implementation should depend on the existing `ThreadStore` and `CodexGateway` abstractions instead of reaching into SQL or Codex CLI details directly. The startup validation and thread option assembly paths must guarantee that Codex can write inside `CLAW_CODEX_WORKDIR`, which means the effective sandbox mode for this feature must be `workspace-write` or `danger-full-access`.

Revision Note: 2026-04-05 / Codex - Created this active ExecPlan after agreeing the `daily` durable-memory bridge behavior, storage layout, and runtime-controlled preflight refresh model.
Revision Note: 2026-04-05 / Codex - Expanded the plan with the exact runtime-managed skill contract, exact prompt shape, and exact initial memory-file templates so the injected daily-memory skill can be implemented deterministically.
Revision Note: 2026-04-05 / Codex - Reworked the plan to use `CLAW_CODEX_WORKDIR/AGENT_MEMORY` instead of an external facts directory and to treat write-capable sandboxing as an explicit feature requirement.
