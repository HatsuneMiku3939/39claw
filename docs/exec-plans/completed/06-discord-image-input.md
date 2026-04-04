# Implement Discord image attachment intake for Codex turns

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agents/PLANS.md`.

## Purpose / Big Picture

After this plan, a Discord user should be able to mention the bot with one or more attached images and receive a response that was generated with those images included in the Codex turn. The same support should also work when the user sends a mention that contains only images and no typed text. A contributor should be able to prove the feature without a live Discord server by running focused mapper, application, and gateway tests, then complete a manual Discord smoke test that shows the bot describing or reasoning about a real attachment.

## Progress

- [x] (2026-04-05 17:20Z) Defined the image-attachment intake plan and acceptance targets.
- [x] (2026-04-05 17:20Z) Confirmed the current repository state: Discord message intake only forwards text content, while the Codex input layer already supports local image parts.
- [x] (2026-04-05 20:25Z) Extended the normalized message request and Codex gateway contracts so a turn can carry text plus zero or more local image paths.
- [x] (2026-04-05 20:25Z) Implemented Discord attachment download, validation, and cleanup for qualifying image attachments.
- [x] (2026-04-05 20:25Z) Implemented mention-plus-image and image-only message handling end to end, including tests and documentation updates.
- [x] (2026-04-05 20:27Z) Ran `make test` and `make lint` after the implementation landed.
- [x] (2026-04-06 00:39Z) Deferred the remaining live Discord smoke test to the shared Discord runtime tech-debt entry because the implementation, automated validation, and documentation are complete.
- [x] (2026-04-06 00:39Z) Archived this plan to `docs/exec-plans/completed/06-discord-image-input.md` because no implementation work remains in `active/`.

## Surprises & Discoveries

- Observation: The Codex integration layer already accepts multipart input with `local_image` parts, so the missing work is in the Discord runtime and application contracts rather than in the Codex executor itself.
  Evidence: `internal/codex/input.go`, `internal/codex/thread.go`

- Observation: The current Discord message mapper rejects mention-only image posts because it requires non-empty post-mention text content before producing an `app.MessageRequest`.
  Evidence: `internal/runtime/discord/message_mapper.go`

- Observation: The current application service and gateway contract only pass a single prompt string, so even if the runtime downloaded attachments today, there is no path to forward them into Codex.
  Evidence: `internal/app/message_service.go`, `internal/app/message_service_impl.go`, `internal/codex/gateway.go`

## Decision Log

- Decision: Implement attachment support as an additive extension of the existing normalized message flow instead of introducing a Discord-specific side channel.
  Rationale: The architecture keeps Discord SDK details in the runtime while the application layer operates on normalized request data. Image paths should therefore become part of the normalized message contract.
  Date/Author: 2026-04-05 / Codex

- Decision: Support only image attachments in this plan and ignore non-image files.
  Rationale: The user-visible goal is image-aware Codex turns. General file ingestion would require different validation, storage, and prompt semantics and is outside this plan's scope.
  Date/Author: 2026-04-05 / Codex

- Decision: Treat mention-plus-image and image-only mention posts as qualifying normal messages.
  Rationale: The product goal is to let users ask about attached images naturally. Requiring non-empty typed text would prevent the second requested behavior.
  Date/Author: 2026-04-05 / Codex

- Decision: Archive this plan without a dedicated image-only live Discord smoke run and track that remaining proof in the shared Discord runtime tech-debt entry instead.
  Rationale: The remaining gap is operational validation, not unfinished implementation. Keeping the plan active would overstate the amount of pending engineering work while duplicating the existing live Discord smoke-test follow-up.
  Date/Author: 2026-04-06 / Codex

## Outcomes & Retrospective

This plan is now implemented for code, automated validation, and documentation updates. The intended outcome was a Discord runtime that can transform attached images into local temporary files, carry those file paths through the application layer, and invoke Codex with multipart input that combines optional text and image parts.

The implementation has now landed for the code, automated tests, and documentation updates described above. The only remaining gap is live Discord smoke validation with disposable credentials, and that gap is now tracked explicitly in `docs/exec-plans/tech-debt-tracker.md` rather than keeping this completed implementation plan in `active/`.

The plan therefore no longer belongs in `active/`: there is no remaining code, test, or documentation work scoped to this feature. What remains is a shared operational proof task for the Discord runtime as a whole, including image attachments.

Automated proof completed during implementation:

- `go test ./internal/runtime/discord ./internal/app ./internal/codex ./cmd/39claw`
- `make test`
- `make lint`

## Context and Orientation

39claw is a thin gateway between Discord and Codex. In the current implementation, the Discord runtime receives a `discordgo.MessageCreate` event, maps that event into an `internal/app.MessageRequest`, passes the request to the application layer, and then the application layer calls the Codex gateway to run or resume a thread. The key files for that flow are:

- `internal/runtime/discord/message_mapper.go`
  - strips the bot mention and currently maps only text content into `app.MessageRequest`
- `internal/runtime/discord/runtime.go`
  - receives the Discord event and hands the mapped request to the message service
- `internal/app/types.go`
  - defines `MessageRequest`, which currently has text fields but no attachment metadata
- `internal/app/message_service.go` and `internal/app/message_service_impl.go`
  - define and implement the application-level orchestration that loads thread bindings and calls the Codex gateway
- `internal/codex/gateway.go`
  - currently accepts only a text prompt and calls `thread.Run(ctx, TextInput(prompt))`
- `internal/codex/input.go`
  - already supports `TextPart`, `LocalImagePart`, and `MultiPartInput`

In this repository, a "qualifying normal message" means a mention-triggered Discord post that should be routed into the configured thread mode. Right now the mapper requires non-empty text after mention stripping, which means a message such as `@bot` with only an attached screenshot is discarded. This plan changes that rule so the bot can handle image attachments whether or not typed text is present, as long as the bot mention is still present and at least one usable input exists.

An "image attachment" in this plan means a Discord attachment whose declared content type starts with `image/`, or whose filename extension clearly identifies a common image format when Discord does not supply a content type. These attachments must be downloaded to a temporary local file because the Codex input layer expects local filesystem paths for image parts.

## Starting State

Start this plan only after confirming the repository still provides these capabilities:

- a real Discord runtime in `internal/runtime/discord`
- a working application-layer message service for `daily` and `task` mode routing
- a Codex gateway that can create or resume threads
- automated tests passing before the feature work begins

Verify that state with:

    make test
    make lint

If one of those foundations is missing, restore it before proceeding. This plan should not invent a replacement runtime or a second gateway abstraction.

## Preconditions

This document is self-contained. The facts needed for implementation are:

- normal conversation is still mention-only in v1
- Codex multipart input already exists and uses local file paths for images
- Discord-specific network fetches and attachment parsing belong in `internal/runtime/discord`
- the application layer should stay transport-agnostic and operate on normalized request data
- documentation must be updated when user-visible bot behavior changes

## Plan of Work

First, extend the normalized application contracts so a message request can carry optional text plus zero or more local image file paths. The simplest shape is to add an `ImagePaths []string` field to `internal/app.MessageRequest` and to change the `CodexGateway` interface from `RunTurn(ctx, threadID, prompt string)` to a signature that accepts structured input, such as `RunTurn(ctx, threadID string, input codex.Input)` through an app-owned transport-neutral input type, or a lighter app-owned struct that includes `Prompt string` and `ImagePaths []string`. Keep the direction clean: the app layer should not import `discordgo`, and the runtime should not call Codex directly.

Second, update the Discord runtime mapping path to recognize image attachments. Add a helper in `internal/runtime/discord` that scans `event.Attachments`, filters to supported image types, downloads them into a deterministic temporary directory owned by the runtime, and returns the resulting local paths plus a cleanup function. The runtime should invoke cleanup after the application service returns, regardless of success or failure, so temporary files do not accumulate. If an attachment download fails, return a user-facing error response instead of sending a partial or silently degraded request. If a message contains both image and non-image attachments, ignore the non-image attachments and continue as long as at least one usable input remains.

Third, relax the mapper rule that currently requires non-empty text content. After mention stripping, the runtime should treat the message as valid when either trimmed text is non-empty or at least one image attachment was successfully prepared. Keep mention-only triggering unchanged; the bot should still ignore non-mentioned image posts.

Fourth, update the Codex gateway implementation to build multipart input. When both text and images are present, construct `codex.MultiPartInput(codex.TextPart(prompt), codex.LocalImagePart(path1), ...)`. When the message has no text and only images, build multipart input that contains only the `LocalImagePart` entries. Ensure the gateway no longer rejects empty text when images are present, while still rejecting turns that contain neither text nor images.

Fifth, add comprehensive tests before relying on manual smoke checks. In `internal/runtime/discord`, add mapper or runtime tests that prove: a mention with text and an image produces a request with text plus image paths; a mention with only an image is accepted; a mention with only non-image attachments is ignored or rejected according to the chosen user-facing behavior; cleanup runs after handling; and failed downloads produce an error response. In `internal/app`, add service tests that prove image paths are forwarded to the gateway without disturbing thread binding behavior. In `internal/codex`, add gateway tests that prove multipart input is built in the correct order and that image-only input is accepted.

Finally, update the repository documentation. Add the new user-visible behavior to `README.md` and to the product or design docs that describe supported Discord interaction rules. `docs/product-specs/discord-command-behavior.md` should explicitly say that mention-triggered normal messages may include image attachments and that typed text is optional when an image is attached. If the implementation introduces constraints such as a temporary-file location or a maximum tested image count, record those in `docs/design-docs/implementation-spec.md` or another appropriate design note.

## Concrete Steps

Run all commands from `/home/filepang/playground/39claw`.

1. Confirm the repository matches the required starting state.

    make test
    make lint

2. Extend the normalized request and gateway contracts to carry image paths.

3. Implement runtime attachment filtering, download, cleanup, and image-only acceptance.

4. Update the Codex gateway to create multipart input from optional text plus image paths.

5. Add or update focused automated tests while iterating.

    go test ./internal/runtime/discord ./internal/app ./internal/codex -run 'TestRuntime|TestMessageService|TestGateway' -v

6. Run the full repository checks.

    make test
    make lint

7. Perform a manual Discord smoke test with a disposable bot environment.

    CLAW_MODE=daily \
    CLAW_TIMEZONE=Asia/Tokyo \
    CLAW_DISCORD_TOKEN=... \
    CLAW_DISCORD_GUILD_ID=... \
    CLAW_CODEX_WORKDIR=/absolute/path/to/repo \
    CLAW_SQLITE_PATH=/tmp/39claw-dev.sqlite \
    CLAW_CODEX_EXECUTABLE=/absolute/path/to/codex \
    go run ./cmd/39claw

8. In Discord, verify these scenarios against the running bot:

    mention bot with text plus screenshot -> reply reflects both request text and screenshot contents
    mention bot with only screenshot -> reply still arrives and describes or reasons about the image
    post screenshot without bot mention -> bot stays silent

## Validation and Acceptance

This plan is complete when all of the following are true:

- a mention-triggered Discord message with text plus one or more image attachments reaches Codex as multipart input
- a mention-triggered Discord message with only one or more image attachments is accepted and answered
- a mention-triggered Discord message with neither text nor usable image attachments is still ignored
- non-image attachments are not forwarded as images
- temporary downloaded image files are cleaned up after request handling
- thread binding, daily routing, task routing, and busy-guard behavior still work for image-bearing messages
- `make test` passes
- `make lint` passes
- a manual Discord smoke test proves both "text plus image" and "image-only" flows

The acceptance proof should make it obvious that the reply changed because the image was available to Codex, not only because the text prompt was forwarded.

## Idempotence and Recovery

Temporary attachment downloads must be safe to repeat. Use a runtime-owned temporary directory under the OS temp location and create per-message unique filenames or subdirectories so retries do not overwrite unrelated files. Cleanup should be best-effort and should run in a deferred path after each handled message.

If a test or smoke run leaves temporary files behind, it should be safe to delete that temporary directory manually and rerun the same command. If attachment download support introduces flaky tests because of real network fetches, replace those network calls with a narrow fakeable HTTP client in the runtime package rather than weakening the assertions.

If you discover during implementation that Codex cannot reliably process image-only multipart input in this environment, stop and record the evidence in `Surprises & Discoveries`, then adjust the product-facing documentation and acceptance criteria before landing the code.

## Artifacts and Notes

Current code facts that motivate this plan:

    internal/runtime/discord/message_mapper.go:
      trims mention text and returns false when content == ""

    internal/app/message_service.go:
      CodexGateway.RunTurn(ctx, threadID string, prompt string)

    internal/codex/input.go:
      supports TextPart, LocalImagePart, and MultiPartInput

Helpful focused test ideas:

    TestMapMessageCreateAcceptsMentionWithImageOnly
    TestRuntimeCleansUpDownloadedImagesAfterHandleMessage
    TestMessageServiceForwardsImagePathsToGateway
    TestGatewayRunTurnBuildsMultipartInputForImages

## Interfaces and Dependencies

At the end of this plan, the repository should expose interfaces shaped like these examples:

    type MessageRequest struct {
        UserID      string
        ChannelID   string
        MessageID   string
        Content     string
        ImagePaths  []string
        Mentioned   bool
        ReceivedAt  time.Time
    }

    type CodexTurnInput struct {
        Prompt     string
        ImagePaths []string
    }

    type CodexGateway interface {
        RunTurn(ctx context.Context, threadID string, input CodexTurnInput) (RunTurnResult, error)
    }

Within `internal/runtime/discord`, keep the Discord SDK details and attachment download details local to that package. Within `internal/codex`, keep multipart-input assembly local to the gateway so the rest of the repository only speaks in normalized text-plus-image data.

Revision Note: 2026-04-05 / Codex - Created this ExecPlan to add Discord image attachment support for both text-plus-image and image-only mention flows.
Revision Note: 2026-04-05 / Codex - Renumbered this active ExecPlan from `05` to `06` because `05-queued-message-handling.md` already exists.
Revision Note: 2026-04-06 / Codex - Archived this plan after implementation completion and moved the remaining live Discord smoke proof into the shared tech-debt tracker entry.
