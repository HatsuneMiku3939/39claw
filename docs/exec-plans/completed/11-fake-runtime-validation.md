# Build fake runtime validation infrastructure for adapter-level tests

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agents/PLANS.md`.

## Purpose / Big Picture

After this plan, a contributor should be able to prove the most important runtime-facing behavior without a live Discord server. They should be able to run a focused automated suite that drives fake runtime inputs, observes adapter-visible outputs, and confirms that 39claw still replies to qualifying mentions, streams in-place progress edits for immediate turns, acknowledges queued work, delivers deferred replies later, and handles command-style inputs at the application boundary.

This change matters because the repository's main confidence story should no longer depend on broad live Discord smoke checks. The result should be a reusable fake-runtime testing shape that stays useful if a future Slack or Telegram runtime is added, while keeping the current production architecture thin and Discord-specific details out of the application layer.

## Progress

- [x] (2026-04-05 19:15Z) Reviewed `.agents/PLANS.md`, the current runtime and application tests, and the validation-strategy docs updated for issue `#57`.
- [x] (2026-04-05 19:15Z) Confirmed the current repository state: Discord runtime tests already use package-local fake sessions, and application tests already use in-memory stores and fake gateways, but there is no reusable fake-runtime harness or shared contract-style suite.
- [x] (2026-04-05 19:15Z) Created this ExecPlan under `docs/exec-plans/active/` and restored the missing `active/` directory so issue `#58` has a tracked execution plan.
- [x] (2026-04-05 19:40Z) Re-read the current runtime and app contracts after the streamed immediate-reply change and updated this ExecPlan so the fake-runtime scope explicitly covers best-effort progress delivery through `MessageProgressSink` and Discord message edits.
- [x] (2026-04-09 11:38Z) Added `internal/testutil/runtimeharness` with transport-neutral message fixtures, command intents, ordered observed deliveries, and reusable assertion helpers.
- [x] (2026-04-09 11:44Z) Extracted reusable Discord fake-session and app-service helpers into `internal/runtime/discord/runtime_test_helpers_test.go` so multiple test files can dispatch harness events and observe adapter-visible outputs through one shared test seam.
- [x] (2026-04-09 11:47Z) Added `internal/runtime/discord/runtime_contract_test.go` with real-runtime contract coverage for normal mention replies, streamed immediate edits, queued acknowledgments plus deferred replies, real task-command interaction presentation, and attachment-aware message flow.
- [x] (2026-04-09 11:49Z) Updated `README.md`, `docs/design-docs/implementation-spec.md`, and `docs/exec-plans/tech-debt-tracker.md` so fake-runtime validation is now documented as the preferred automated runtime layer before optional live Discord hardening.
- [x] (2026-04-09 11:52Z) Validated the completed change by running `go test ./...` and the `make lint` equivalent `./scripts/lint -c .golangci.yml` because `make` was not installed in the execution environment.

## Surprises & Discoveries

- Observation: The repository already has most of the raw pieces needed for fake-runtime coverage, but they are split across package-local test doubles instead of a reusable harness.
  Evidence: `internal/runtime/discord/runtime_test.go`, `internal/app/message_service_test.go`

- Observation: The current `internal/app.MessageService` contract already defines the correct app/runtime boundary for reusable validation: normalized `MessageRequest`, immediate `MessageResponse`, and optional `DeferredReplySink`.
  Evidence: `internal/app/message_service.go`, `internal/app/types.go`

- Observation: The Discord runtime already exposes realistic end-to-end seams for adapter-level tests because `Runtime.Start` registers handlers on an abstract `session` interface and `runtime_test.go` can dispatch fake `discordgo` events through that interface.
  Evidence: `internal/runtime/discord/runtime.go`, `internal/runtime/discord/session.go`, `internal/runtime/discord/runtime_test.go`

- Observation: The current validation gap is not "no tests exist." The real gap is that the existing tests are difficult to reuse as a runtime-neutral pattern because the doubles and assertions are tightly coupled to one package's private test helpers.
  Evidence: `internal/runtime/discord/runtime_test.go`

- Observation: A neighboring runtime change widened the app/runtime boundary by adding `app.MessageProgressSink` for best-effort streamed updates on immediate turns, and the Discord session seam now includes message-edit and message-delete operations.
  Evidence: `internal/app/types.go`, `internal/app/message_service_impl.go`, `internal/runtime/discord/runtime.go`, `internal/runtime/discord/session.go`, `internal/runtime/discord/live_message.go`

- Observation: The real `app.DefaultMessageService` already emits an initial `Thinking...` progress update for immediate turns, so adapter-level contract assertions must treat "first visible send plus later final edit" as the normal immediate-response shape instead of assuming a one-shot final send.
  Evidence: `internal/app/message_service_impl.go`, `internal/runtime/discord/runtime_contract_test.go`

- Observation: Preserving chronological order in the fake-session observation stream matters more than grouping by operation type, because queued flows can interleave channel sends and message edits in one runtime-visible sequence.
  Evidence: `internal/runtime/discord/runtime_test_helpers_test.go`, `internal/runtime/discord/runtime_contract_test.go`

## Decision Log

- Decision: Keep the new fake-runtime infrastructure in a test-support package such as `internal/testutil/runtimeharness` instead of introducing a new production runtime abstraction.
  Rationale: Issue `#58` is about validation shape, not a new architecture layer. Putting the harness in test support avoids leaking speculative runtime-generic interfaces into production code.
  Date/Author: 2026-04-05 / Codex

- Decision: Define the reusable contract at the app/runtime boundary, not at the Discord SDK boundary and not at the Codex gateway boundary.
  Rationale: The validation strategy agreed in issue `#57` explicitly limits runtime-agnostic contracts to message qualification, ignored-message rules, queue admission, deferred delivery handoff, command normalization, and normalized response expectations at the app/runtime boundary.
  Date/Author: 2026-04-05 / Codex

- Decision: Start with one real adapter implementation of the harness, the Discord runtime, and require one vertical-slice automated suite before expanding to more helper abstractions.
  Rationale: The repository only ships Discord today. Proving the harness on the real runtime prevents over-design and keeps future-runtime reuse grounded in an existing working example.
  Date/Author: 2026-04-05 / Codex

- Decision: Prefer reusing and relocating existing fake helpers over rewriting all runtime tests at once.
  Rationale: `runtime_test.go` and `message_service_test.go` already encode valuable behavior. The safest path is to extract stable helpers, convert the key adapter-level cases to the new harness, and leave narrowly unit-scoped tests in place when they still add value.
  Date/Author: 2026-04-05 / Codex

- Decision: Treat streamed immediate-reply edits as part of the runtime-visible contract that the fake-runtime suite must cover, but keep those edits best-effort and separate from queued deferred-delivery guarantees.
  Rationale: The production runtime now edits an in-flight Discord reply when `Codex` emits progress or partial assistant text on immediate turns. That behavior is user-visible and should be validated, but it is intentionally weaker than the queued-message contract because progress delivery failures do not fail the Codex turn.
  Date/Author: 2026-04-05 / Codex

- Decision: Keep the shared harness vocabulary intentionally small and transport-neutral, but let the Discord fake session preserve ordered deliveries directly instead of reconstructing sequences from grouped Discord payload slices.
  Rationale: The useful contract is the sequence of adapter-visible outcomes, not the exact Discord SDK payload bucket they came from. Recording ordered `runtimeharness.Delivery` values keeps assertions simple and future-runtime reuse realistic.
  Date/Author: 2026-04-09 / Codex

- Decision: Use the real `app.DefaultMessageService` for the normal mention, queueing, streaming, and attachment-aware contract scenarios, and use the real `app.DefaultTaskCommandService` for the command-style scenario.
  Rationale: This plan exists to prove the real app/runtime boundary rather than bespoke runtime-only stubs. Using the actual services gives one meaningful vertical slice while still keeping the runtime fake and deterministic.
  Date/Author: 2026-04-09 / Codex

## Outcomes & Retrospective

The repository now contains a reusable fake-runtime harness under `internal/testutil/runtimeharness`, shared Discord runtime test helpers under `internal/runtime/discord/runtime_test_helpers_test.go`, and a contract-style suite under `internal/runtime/discord/runtime_contract_test.go`. That suite exercises the real `Runtime.Start` path against a fake Discord session and proves normal mention replies, streamed immediate edits, queued acknowledgment plus deferred reply sequencing, real task-command interaction presentation, and attachment-aware message flow without a live Discord deployment.

Full validation is also complete. In this environment `make` was unavailable, so the equivalent commands were run directly: `go test ./...` and `./scripts/lint -c .golangci.yml`. Both passed, which means the plan's intended outcome is fully implemented and ready to archive.

## Context and Orientation

39claw is a thin gateway between Discord and Codex. The production message path starts in `internal/runtime/discord/runtime.go`, where the runtime receives Discord events, maps them into normalized application requests, calls the application layer, and presents the returned response back to Discord. The application boundary is defined in `internal/app/message_service.go` and `internal/app/types.go`. A normal message becomes an `app.MessageRequest`; the application returns an immediate `app.MessageResponse`; queued work can later use `app.DeferredReplySink` to publish a follow-up reply; and immediate turns may now use `app.MessageProgressSink` to push best-effort streamed progress into the runtime before the final response is ready.

In this repository, a "fake runtime" means a test harness that simulates platform-facing events and captures the runtime-visible outputs without connecting to a real Discord deployment. It is not a second production runtime and it is not a broad interface that every future runtime must implement in production code. It is a test-support shape that lets tests express scenarios such as "a qualifying mention arrives," "a command interaction arrives," and "a deferred reply is delivered later," then assert what the adapter presented externally.

The key current files are:

- `internal/runtime/discord/runtime.go`
  - owns Discord session startup, event handlers, response presentation, streamed immediate-reply edits, and shutdown draining
- `internal/runtime/discord/session.go`
  - defines the narrow `session` interface that production and test sessions both satisfy, including message send, edit, and delete operations
- `internal/runtime/discord/live_message.go`
  - keeps one in-flight Discord reply synchronized with streamed progress text by sending, editing, and trimming message chunks
- `internal/runtime/discord/runtime_test.go`
  - already contains a package-local fake session plus adapter-level tests, including streamed-reply edit assertions, but the helpers are not reusable outside this file
- `internal/runtime/discord/message_mapper.go` and `internal/runtime/discord/interaction_mapper.go`
  - normalize Discord events into `app.MessageRequest` and task-command requests
- `internal/app/message_service.go`
  - defines `MessageService`, `DeferredReplySink`, `MessageProgressSink`, and the queue-related boundary
- `internal/app/message_service_impl.go`
  - implements logical-key resolution, queue admission, best-effort progress delivery for immediate turns, deferred delivery handoff, and Codex turn orchestration
- `internal/app/task_service.go`
  - implements command-style task control and help behavior behind the runtime
- `internal/thread/queue.go`
  - owns capped in-memory queue admission for same-key work

The repository already has focused automated tests for the application layer and Discord runtime. What it does not have is a single reusable harness that can express runtime-facing contract scenarios, including streamed immediate-reply edits, and make those scenarios easy to repeat for future runtimes.

## Starting State

Start this plan only after confirming the repository still matches these assumptions:

- the production runtime is still `internal/runtime/discord`
- the app/runtime boundary still flows through `app.MessageRequest`, `app.MessageResponse`, `app.DeferredReplySink`, and `app.MessageProgressSink`
- `internal/runtime/discord/runtime_test.go` still contains working fake-session-based adapter tests
- the Discord runtime still publishes immediate-turn progress by editing one in-flight reply through the `session` interface
- `internal/app/message_service_test.go` still provides in-memory store and gateway doubles that can support end-to-end-style tests without a real Codex backend
- `make test` and `make lint` pass before the new harness work begins

Verify that state with:

    cd /home/filepang/playground/39claw
    make test
    make lint

If the repository has drifted away from that shape, update this plan before implementing so it remains truthful and self-contained.

## Preconditions

This plan assumes and fixes the following boundaries:

- runtime-agnostic validation stops at the app/runtime boundary
- production code must stay thin and should not gain a speculative cross-platform runtime interface
- adapter-level fake tests may use Discord-specific event values internally, but the reusable test-support package should describe observed behavior in transport-neutral terms
- streamed immediate-turn progress is part of the visible runtime contract, but its delivery remains best-effort rather than a hard application guarantee
- optional live Discord hardening remains a separate concern and is not part of this implementation plan

## Milestone 1: Create a reusable runtime-harness vocabulary

At the end of this milestone, the repository should have one test-support package that defines the language of runtime-facing validation: fake incoming events, observed deliveries, and reusable assertions. This package should be small enough that a future runtime can adopt it with limited glue code.

Create a new package under `internal/testutil/runtimeharness`. Keep it test-support-only in purpose even if the Go files are normal source files. Define transport-neutral types that describe what the tests care about, not what Discord SDK types happen to look like. The package should include a small set of event and observation types such as:

- a normal-message fixture that carries user ID, channel ID, message ID, mention state, text payload, and optional attachment metadata
- a command-intent fixture that carries user ID, channel ID, command name, action name, and any task-related arguments
- an observed-delivery record that captures the visible outcome: channel ID, reply target, text payload, whether the delivery was immediate, streamed-edit, or deferred, and whether the response was ephemeral

Keep these types plain and boring. They exist so tests can describe scenarios consistently. Do not add a production dependency from `internal/app` or `internal/runtime/discord` to this package.

Add reusable assertion helpers here as well. For example, provide helpers that verify "one immediate reply rooted to message X," "one streamed immediate reply that evolves through edits," "one queued acknowledgment followed by one deferred reply," or "one ephemeral interaction response." The helpers should compare only runtime-visible behavior and avoid asserting implementation trivia such as specific helper function names or internal log wording.

## Milestone 2: Adapt the Discord tests to the reusable harness

At the end of this milestone, the Discord runtime should have a contract-style suite that is driven through fake runtime inputs and verified through normalized observed deliveries. The suite must use the real `discord.Runtime` startup path, not a hand-wired imitation of the runtime logic.

First, extract or rewrite the current package-local fake session helpers in `internal/runtime/discord/runtime_test.go` so they can be reused by multiple test files. It is acceptable to keep the fake session implementation inside the `internal/runtime/discord` test package as long as the observed outputs can be converted into the shared `runtimeharness` vocabulary. Preserve the current ability to:

- dispatch fake `discordgo.MessageCreate` events
- dispatch fake `discordgo.InteractionCreate` events
- record sent channel messages
- record interaction responses and follow-up edits or messages
- inspect registration state when needed

Next, add a new contract-style test file in `internal/runtime/discord`, for example `runtime_contract_test.go`. This file should build one or more real `Runtime` values with realistic dependencies and then exercise scenarios through the harness. Use the real application service where doing so gives meaningful cross-layer coverage, and use focused fakes only where the runtime boundary itself is the subject under test.

The minimum scenarios to cover are:

1. A qualifying normal mention produces one reply to the triggering message.
2. A qualifying immediate turn that emits progress produces one reply that is updated in place as progress or partial assistant text arrives.
3. A queued normal mention produces one immediate queued acknowledgment and later one deferred reply to the original message.
4. A command-style interaction produces the correct visible presentation, including the ephemeral flag where applicable.
5. One representative attachment-aware message flow proves that attachment metadata and reply semantics can be driven through the fake runtime path without a live Discord server.

For the queueing scenario, prefer wiring the real `app.DefaultMessageService`, the real `thread.QueueCoordinator`, an in-memory thread store double, and a fake Codex gateway that can block and release on command. That gives one real vertical slice that covers runtime event handling, app-level queue admission, and deferred delivery without a live Discord deployment.

For the command scenario, use the real task-command service when practical so the suite validates the actual runtime mapping and presentation path, not a bespoke fake response path.

## Milestone 3: Tighten helper ownership and keep focused unit tests

At the end of this milestone, the repository should have a clear split between narrow unit tests and the new runtime contract tests. Existing unit tests that still provide low-level value should remain, but duplicated scenario setup should move into shared helpers or the new contract suite.

As part of this cleanup, remove or simplify any now-redundant helper code in `internal/runtime/discord/runtime_test.go` that was only needed because there was no shared harness vocabulary. Keep focused mapper, formatter, and presenter unit tests in place when they assert small transformation rules that the broader contract suite would not explain clearly.

Do not try to convert every existing runtime test into the new harness in one pass. The goal is to establish a reusable path and prove it on the most important adapter-level behaviors first.

## Milestone 4: Document the fake-runtime validation path

At the end of this milestone, contributors should know where the fake-runtime suite lives, when to run it, and how it fits into the repository's broader validation strategy.

Update the most relevant documents:

- `README.md`
  - mention the focused fake-runtime suite as the first choice for runtime behavior validation before optional live Discord hardening
- `docs/design-docs/implementation-spec.md`
  - add the concrete package path or test command if the final harness layout should be discoverable there
- `docs/exec-plans/tech-debt-tracker.md`
  - if implementation of this plan materially narrows the remaining live Discord gap, update the wording so the debt entry reflects the new fake-runtime coverage

Do not create a brand-new design note unless implementation discovers something that cannot be explained clearly in the existing docs.

## Plan of Work

Begin by creating `internal/testutil/runtimeharness` with a small transport-neutral vocabulary for runtime events and deliveries. Keep the package purpose narrow: it should only help tests describe and assert behavior at the app/runtime boundary, including immediate reply edits and deferred follow-up replies.

Next, inspect the current fake helpers in `internal/runtime/discord/runtime_test.go` and decide which ones should become stable shared test utilities. A likely shape is to keep the low-level fake Discord session in the Discord test package, but add conversion helpers that transform recorded Discord-specific outputs into `runtimeharness` observations.

Then add one new contract-style test file in `internal/runtime/discord` that uses the shared harness vocabulary. For at least one queueing scenario, wire the real application service and queue coordinator so the test proves observable queue behavior through the runtime boundary. For at least one immediate-turn scenario, wire a fake Codex gateway or scripted message service that emits progress updates so the harness proves the reply-edit behavior through the same runtime boundary. For the other required scenarios, choose the thinnest dependencies that still make the visible behavior meaningful.

After the new suite is stable, trim duplicated setup from existing runtime tests where it improves readability. Leave small unit tests in place when they still explain isolated rules better than a vertical slice would.

Finally, update the docs named above so the repository's written guidance matches the new test infrastructure and the validation strategy already agreed in issue `#57`.

## Concrete Steps

Run all commands from `/home/filepang/workspaces/39claw/39claw`.

1. Confirm the starting state and capture a clean baseline.

    make test
    make lint

2. Add the new test-support package and its self-tests, if needed.

    go test ./internal/testutil/runtimeharness -v

3. Build or refactor the Discord fake-session helpers until they support the new contract suite cleanly, including reply edits and any chunk-trimming deletes.

    go test ./internal/runtime/discord -run 'TestRuntimeStart|TestRuntimeMention|TestRuntimeDeferred|TestRuntime.*Stream' -v

4. Add the new contract-style suite and run only the new scenarios while iterating.

    go test ./internal/runtime/discord -run 'TestRuntimeContract' -v

5. Run the application-layer tests if the contract suite uses real message-service or task-service wiring and the supporting doubles changed.

    go test ./internal/app -run 'TestMessageService|TestTaskService' -v

6. Run the full repository checks after the new harness and docs land.

    make test
    make lint

7. Record concise proof artifacts and any discovered follow-up work in this plan before closing or archiving it.

## Validation and Acceptance

This plan is complete when all of the following are true:

- the repository contains a reusable fake-runtime test-support package under a stable path such as `internal/testutil/runtimeharness`
- the new package defines transport-neutral inputs or observations for runtime-facing behavior instead of exposing Discord SDK types directly
- at least one Discord adapter suite drives fake runtime events through the real `Runtime` startup path and verifies observable outputs through the shared harness vocabulary
- the suite proves a normal mention reply flow, a streamed immediate-reply edit flow, a queued-acknowledgment-plus-deferred-reply flow, and one command-style interaction flow
- at least one queueing test uses the real app-layer queue admission path rather than a runtime-only stub so the suite behaves like an end-to-end slice
- existing focused runtime unit tests still pass, with duplicated setup reduced where practical
- repository docs mention the fake-runtime validation path as the preferred automated layer before optional live Discord hardening
- `make test` passes
- `make lint` passes

The most important human-readable proof is this automated scenario:

1. Start the real Discord runtime in a test with a fake session.
2. Dispatch one qualifying message that stays busy long enough to admit a second message into the queue.
3. Observe one immediate queued acknowledgment reply to the second message.
4. Release the first turn.
5. Observe one later deferred reply to that same second message.

That proof must happen entirely inside automated tests without a live Discord deployment.

An additional human-readable proof should cover the new immediate-turn streaming contract:

1. Start the real Discord runtime in a test with a fake session.
2. Dispatch one qualifying message whose dependency emits progress and then a final response.
3. Observe one initial reply message.
4. Observe that same reply being edited in place as progress or partial assistant text arrives.
5. Observe the final edit settle on the completed assistant response.

## Idempotence and Recovery

This plan is safe to iterate on because the new harness is additive. If the first version of the shared vocabulary proves awkward, adjust the helper package before migrating more tests. Do not add production runtime abstraction layers just to make the tests look more generic.

If a partial refactor breaks the current Discord runtime tests, the safe recovery path is to keep the old package-local helpers working while the new harness is introduced beside them. Only remove duplicated helpers after the new contract suite is green and understandable.

If the first attempt at a transport-neutral vocabulary becomes too Discord-shaped, stop and simplify it. The right boundary is "what the app/runtime contract makes visible," not "everything Discord can do."

## Artifacts and Notes

Current code facts that motivate this plan:

    internal/app/message_service.go:
      type DeferredReplySink interface {
          Deliver(ctx context.Context, response MessageResponse) error
      }

    internal/app/types.go:
      type MessageProgressSink interface {
          Deliver(ctx context.Context, progress MessageProgress) error
      }

    internal/runtime/discord/session.go:
      Runtime startup and event handling already depend on a narrow session interface with send, edit, and delete operations.

    internal/runtime/discord/runtime_test.go:
      already contains fake-session-driven scenarios for mentions, streamed reply edits, queued replies, shutdown drain, and attachment downloads.

Helpful target test names:

    TestRuntimeContractDailyMentionReply
    TestRuntimeContractStreamsImmediateReplyEdits
    TestRuntimeContractQueuedAcknowledgementAndDeferredReply
    TestRuntimeContractTaskCommandUsesRealServiceAndEphemeralResponse
    TestRuntimeContractAttachmentAwareMessageUsesDownloadedImagePaths

Helpful dependency shape for the vertical queueing slice:

    real runtime
    real app.DefaultMessageService
    real thread.QueueCoordinator
    in-memory thread store test double
    fake Codex gateway with controllable blocking
    fake Discord session that records deliveries

Validation proof captured during implementation:

    $ go test ./...
    ok   github.com/HatsuneMiku3939/39claw/internal/runtime/discord   (cached)
    ok   github.com/HatsuneMiku3939/39claw/internal/testutil/runtimeharness (cached)
    ... full repository test suite passed

    $ ./scripts/lint -c .golangci.yml
    0 issues.
    Linting passed

## Interfaces and Dependencies

At the end of this plan, the new test-support package should expose stable names equivalent to these examples:

    package runtimeharness

    type MessageEvent struct {
        UserID      string
        ChannelID   string
        MessageID   string
        Mentioned   bool
        Content     string
        Attachments []Attachment
    }

    type CommandEvent struct {
        UserID      string
        ChannelID   string
        CommandName string
        Action      string
        TaskName    string
        TaskID      string
    }

    type Delivery struct {
        ChannelID string
        ReplyToID string
        Text      string
        Ephemeral bool
        Deferred  bool
        Edited    bool
    }

    func RequireReplyTo(t *testing.T, deliveries []Delivery, replyToID string)
    func RequireStreamedEditFlow(t *testing.T, deliveries []Delivery, replyToID string)
    func RequireQueuedFlow(t *testing.T, deliveries []Delivery, replyToID string, ackText string, finalText string)

These names are examples, not mandatory exact spelling, but the final package must provide the same capabilities. The production runtime must continue to depend only on `internal/app`, `internal/config`, `discordgo`, and its existing collaborators. The new harness must not become a production dependency.

Revision Note: 2026-04-05 / Codex - Created this ExecPlan for issue `#58` after documenting the validation strategy in issue `#57`, creating the missing `docs/exec-plans/active/` directory, and confirming that the repository already has package-local fake runtime helpers that can be promoted into a reusable harness.
Revision Note: 2026-04-05 19:40Z / Codex - Updated this ExecPlan after the streamed immediate Discord reply change landed on the working branch. The plan now treats `MessageProgressSink` and in-place reply edits as part of the runtime-visible behavior that the future fake-runtime harness must validate.
Revision Note: 2026-04-09 11:49Z / Codex - Updated the plan after implementing the reusable `internal/testutil/runtimeharness` package, extracting shared Discord fake-session helpers, adding real-runtime contract tests, and documenting the new fake-runtime validation path. The only remaining plan step is full-repository validation.
Revision Note: 2026-04-09 11:52Z / Codex - Recorded successful full-repository validation. `make` was unavailable in the execution environment, so the equivalent commands `go test ./...` and `./scripts/lint -c .golangci.yml` were used instead. Both passed.
