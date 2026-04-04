# Replace shared `/help` and `/task` slash commands with one instance-specific root command

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agents/PLANS.md`.

## Purpose / Big Picture

After this plan, each 39claw deployment should expose exactly one slash-command entry in Discord search instead of a shared `/help` command plus a shared `/task ...` family. A user should be able to type the command name that belongs to the current bot instance, select one result, and then choose an explicit action such as help, list tasks, create a task, or switch tasks. This removes the multiplicative command-search clutter that appears when several bot instances are installed in the same Discord server.

The user-visible proof is simple. If a bot instance starts with `CLAW_DISCORD_COMMAND_NAME=release`, Discord search should show `/release` as one command result for that bot instance. In `task` mode, selecting `/release` should then offer action choices such as `help`, `task-current`, `task-list`, `task-new`, `task-switch`, and `task-close`. In `daily` mode, selecting the instance command should offer only `help`.

## Progress

- [x] (2026-04-04 22:29Z) Captured the UX problem, chose the one-root-command-per-instance direction, and wrote this ExecPlan.
- [x] (2026-04-04 23:07Z) Added required `CLAW_DISCORD_COMMAND_NAME` loading, normalization, and validation in `internal/config`.
- [x] (2026-04-04 23:07Z) Replaced shared `/help` and `/task ...` registration with one configurable root command plus `action`, `task_name`, and `task_id` options.
- [x] (2026-04-04 23:07Z) Rewrote interaction parsing and runtime routing to dispatch by explicit action choices rather than command-name/subcommand pairs.
- [x] (2026-04-04 23:07Z) Updated help text and task guidance so responses reference the configured root command and current mode.
- [x] (2026-04-04 23:07Z) Added and updated Go tests for config loading, command registration, interaction parsing, runtime routing, and root-command response wording.
- [x] (2026-04-04 23:07Z) Updated `README.md`, `docs/product-specs/discord-command-behavior.md`, `docs/product-specs/task-mode-user-flow.md`, and `docs/design-docs/implementation-spec.md` to describe the new command surface.
- [x] (2026-04-04 23:07Z) Ran `make test` and `make lint` successfully after the implementation landed.
- [ ] Manually confirm in Discord that one bot instance contributes only one command-search entry.

## Surprises & Discoveries

- Observation: The current `/task` command family already creates multiple searchable results in Discord because the subcommands are surfaced as separate leaf entries in the command picker.
  Evidence: Manual Discord UI observation recorded on 2026-04-04, plus the current multi-command registration in `internal/runtime/discord/commands.go`.

- Observation: The current runtime hard-codes command names in three places: command registration, interaction mapping, and runtime routing.
  Evidence: `internal/runtime/discord/commands.go`, `internal/runtime/discord/interaction_mapper.go`, and `internal/runtime/discord/runtime.go`.

- Observation: The current configuration has no concept of bot-instance identity on the Discord command surface, so two instances installed in one server necessarily collide on `/help` and `/task`.
  Evidence: `internal/config/config.go` and `README.md`.

- Observation: Task guidance text is not isolated to the Discord runtime. The app-layer message and task services also embed command examples, so the configured root command has to flow into those services to keep user-facing copy consistent.
  Evidence: `internal/app/message_service_impl.go` and `internal/app/task_service.go`.

## Decision Log

- Decision: Replace the shared `/help` and `/task ...` command set with one per-instance root command whose name is supplied by configuration.
  Rationale: This is the only option in scope that reduces Discord search clutter from `instance count x command count` down to approximately `instance count`.
  Date/Author: 2026-04-04 / Codex

- Decision: Represent control operations as a required `action` option with fixed string choices such as `help`, `task-current`, and `task-new`, rather than as Discord subcommands.
  Rationale: Discord search is cluttered by leaf command entries. A single root command with a choice option preserves explicit UX without multiplying search results.
  Date/Author: 2026-04-04 / Codex

- Decision: Introduce `CLAW_DISCORD_COMMAND_NAME` as the only new command-surface environment variable.
  Rationale: The command name itself is the user-visible identity in Discord slash-command search, so a second label value would add configuration complexity without solving the core UX problem.
  Date/Author: 2026-04-04 / Codex

- Decision: Keep mention-based parsing of literal text like `/help` or `/task ...` out of scope for this change.
  Rationale: The selected direction is “one instance-specific root command” because it is more native to Discord UI. Mixing in mention-side command parsing would broaden the scope and blur the product contract again.
  Date/Author: 2026-04-04 / Codex

- Decision: Pass the configured root command name into the app-layer message and task services so all recovery guidance uses the same command surface.
  Rationale: Missing-task and task-state responses are produced below the Discord runtime boundary. Keeping those strings aligned at the source avoids a split-brain UX where slash-command help and task guidance disagree.
  Date/Author: 2026-04-04 / Codex

## Outcomes & Retrospective

Implementation is complete in code, tests, and repository docs. The resulting command surface now scales with the number of installed bot instances instead of scaling with instances multiplied by slash-command variants, and help/task guidance consistently identifies the configured root command for the current deployment. The remaining gap is manual Discord verification in a live guild to confirm the slash-command picker now shows exactly one searchable root command per instance.

## Context and Orientation

39claw is a thin Discord runtime that routes qualifying user input into Codex threads. The current normal-message path is mention-only and should remain unchanged by this plan. The only behavior being redesigned here is the explicit slash-command surface.

The current command registration lives in `internal/runtime/discord/commands.go`. That file currently registers two top-level command names, `help` and `task`, and the `task` command contains five subcommands. The parser in `internal/runtime/discord/interaction_mapper.go` assumes those fixed names and converts Discord interactions into a `commandRequest` with an embedded `taskCommandRequest`. The router in `internal/runtime/discord/runtime.go` switches on the fixed command name and then switches again on the task action. The configuration loader in `internal/config/config.go` does not yet know anything about slash-command naming, so every deployment exposes the same user-facing command names.

The relevant user-facing documentation is split across `README.md`, `docs/product-specs/discord-command-behavior.md`, and `docs/product-specs/task-mode-user-flow.md`. The implementation defaults live in `docs/design-docs/implementation-spec.md`. All four documents currently describe `/help` and `/task ...` as the stable command surface, so they must change together with the code.

A “root command” in this plan means the first word of a Discord slash command, such as `/release`. An “action choice” means a fixed value selected inside that command, such as `help` or `task-new`. The goal is to keep one root command per bot instance and move workflow branching into the action choice.

## Starting State

Start from the current repository behavior:

- `internal/runtime/discord/commands.go` registers two top-level slash commands and six searchable leaf entries in `task` mode.
- `internal/runtime/discord/runtime.go` expects `commandHelp` and `commandTask`.
- `internal/config/config.go` requires mode, timezone, Discord token, workdir, SQLite path, and Codex executable, but no command identity values.
- `README.md` and the product docs still instruct users to run `/help` and `/task ...`.

Confirm that starting state before implementation:

    make test
    make lint

If these checks fail before any code changes, fix the unrelated regression first or record it in `Surprises & Discoveries` before continuing.

## Milestones

### Milestone 1: Add explicit command identity to configuration and registration

At the end of this milestone, a bot instance can describe its own slash-command identity. The configuration layer should require a unique `CLAW_DISCORD_COMMAND_NAME`. The Discord command registration code should then use that value to register exactly one root command for the current instance.

The important proof for this milestone is local and testable without Discord itself. Configuration tests should prove the new environment variable is required and that invalid command names are rejected with actionable errors. Runtime registration tests should prove the command count drops from two to one and that the registered command name matches the configured value.

### Milestone 2: Route one root command through explicit action choices

At the end of this milestone, the runtime no longer cares about command names like `help` or `task`. Instead, it should receive a single root command and dispatch based on a required `action` choice. `daily` mode should offer only the `help` action. `task` mode should offer `help`, `task-current`, `task-list`, `task-new`, `task-switch`, and `task-close`.

The key proof for this milestone is end-to-end runtime behavior in tests. A fake interaction for the configured root command should reach the same app-layer task services as before. Help should still be handled locally, task actions should still call `TaskCommandService`, and invalid or missing arguments should still return user-facing guidance rather than infrastructure errors.

### Milestone 3: Make the new surface understandable in Discord and in docs

At the end of this milestone, user-facing copy and documentation should explain the instance-specific command model clearly. Help output should mention the configured root command by name and show examples using the action option format. Repository docs should stop telling users to run `/help` or `/task ...` and should instead describe the root command plus actions.

The important proof for this milestone is behavioral and manual. With a live bot started in one guild, Discord search should show one root command entry for the instance. Selecting that command should reveal action choices rather than separate searchable leaf commands. The help response should tell the user what bot they are talking to and which actions are available in the current mode.

## Plan of Work

Extend `internal/config/config.go` and `internal/config/config_test.go` first. Add `DiscordCommandName` to `config.Config`. Load `CLAW_DISCORD_COMMAND_NAME` as a required environment variable. Normalize it to lowercase trimmed text and validate it conservatively so deployments cannot register an invalid Discord command name. Update all config-loading tests and startup examples to include the new required variable.

Rework the Discord command schema in `internal/runtime/discord/commands.go`. Replace the fixed registration of `/help` and `/task` with a function that accepts the full config or the specific values it needs. Register exactly one `discordgo.ApplicationCommand` whose `Name` is `config.DiscordCommandName`. In `daily` mode, attach one required string option named `action` with one choice: `help`. In `task` mode, attach the same `action` option with the additional task choices and add optional string options for `task_name` and `task_id`. Keep the descriptions user-facing and concise. Update help response generation so it prints examples like `/<command> action:help` and `/<command> action:task-new task_name:<name>`. The help output should explicitly show the command name and current mode, for example `Command: /release` and `Mode: task`.

Simplify interaction parsing in `internal/runtime/discord/interaction_mapper.go`. Remove the assumption that the command name itself determines behavior. Keep the root command name on the request only for logging and sanity checks, but parse the action value from the `action` option into a generic action field on `commandRequest`. Parse `task_name` and `task_id` from sibling options instead of from subcommand-specific option lists. The new request shape should be simple enough that `runtime.go` needs only one switch on the action string.

Refactor `internal/runtime/discord/runtime.go` to route actions instead of command names. The router should handle `help` locally and dispatch the task actions to the existing `TaskCommandService`. Preserve the current behavior where task commands are unavailable in `daily` mode, but move the message wording to mention the configured root command rather than `/task ...`. Preserve ephemeral responses for command interactions. Remove or rewrite any “unsupported command” text so it references the new action vocabulary.

Update tests next. In `internal/runtime/discord/runtime_test.go`, change command-registration assertions from “two commands” to “one command whose name matches the config”. Add interaction tests for `action=help`, `action=task-current`, and one mutating task action such as `task-new`. Add or update parser coverage in a new or existing test file so malformed interactions, missing action values, and task-name or task-id extraction are exercised directly. In `internal/config/config_test.go`, add validation coverage for missing command names, normalized valid names, and rejected invalid names.

Update documentation after the code is stable. Rewrite `README.md` so quick start and command examples use the configured root command. Update `docs/product-specs/discord-command-behavior.md` to define one instance-specific root command with action choices as the new v1 command surface. Update `docs/product-specs/task-mode-user-flow.md` so task flows refer to `/<instance-command> action:task-*` examples instead of `/task ...`. Update `docs/design-docs/implementation-spec.md` so its Discord behavior and configuration defaults mention `CLAW_DISCORD_COMMAND_NAME` and the one-root-command structure.

## Concrete Steps

Run all commands from `/home/filepang/playground/39claw`.

1. Confirm the starting state is clean enough to change.

    make test
    make lint

    Expected result:

        all Go tests pass
        lint passes with 0 issues

2. Implement the configuration changes in:

    - `internal/config/config.go`
    - `internal/config/config_test.go`

3. Replace the slash-command registration and parsing flow in:

    - `internal/runtime/discord/commands.go`
    - `internal/runtime/discord/interaction_mapper.go`
    - `internal/runtime/discord/runtime.go`
    - `internal/runtime/discord/runtime_test.go`

4. Update user-facing docs in:

    - `README.md`
    - `docs/product-specs/discord-command-behavior.md`
    - `docs/product-specs/task-mode-user-flow.md`
    - `docs/design-docs/implementation-spec.md`

5. Run focused tests while iterating.

    go test ./internal/config ./internal/runtime/discord -run 'Test(LoadFromLookup|Runtime)'

    Expected result:

        ok   github.com/HatsuneMiku3939/39claw/internal/config
        ok   github.com/HatsuneMiku3939/39claw/internal/runtime/discord

6. Run the full repository checks after the implementation lands.

    make test
    make lint

7. Perform a manual Discord proof in a test guild with one bot instance configured like this:

    CLAW_MODE=task
    CLAW_TIMEZONE=Asia/Tokyo
    CLAW_DISCORD_TOKEN=...
    CLAW_DISCORD_GUILD_ID=...
    CLAW_DISCORD_COMMAND_NAME=release
    CLAW_CODEX_WORKDIR=/absolute/path/to/workdir
    CLAW_DATADIR=/tmp/39claw
    CLAW_CODEX_EXECUTABLE=/absolute/path/to/codex
    go run ./cmd/39claw

    Expected manual result:

        Discord search shows one `/release` command entry for this bot instance
        selecting `/release` exposes action choices instead of separate leaf command search entries
        `action=help` identifies the configured command as `/release` and the mode as `task`
        `action=task-new` creates a task and keeps ordinary mention-based conversation unchanged

## Validation and Acceptance

This plan is complete when all of the following are true:

- every 39claw instance registers exactly one top-level slash command in Discord
- the top-level command name comes from `CLAW_DISCORD_COMMAND_NAME`
- `CLAW_DISCORD_COMMAND_NAME` is required at startup and rejects invalid values with actionable errors
- `daily` mode exposes only the `help` action through the root command
- `task` mode exposes `help`, `task-current`, `task-list`, `task-new`, `task-switch`, and `task-close`
- the help response shows the configured root command and current mode instead of `/help` and `/task ...`
- the task workflow still uses the existing app-layer `TaskCommandService`
- normal mention-based conversation behavior is unchanged
- legacy `/help` and `/task ...` are no longer registered for this bot instance
- `make test` passes
- `make lint` passes
- manual Discord verification shows one searchable root-command result per bot instance rather than one result per task leaf action

The acceptance bar is user-facing, not only internal. A human should be able to install several 39claw instances in one server and see command-search clutter scale with the number of instances, not with the number of task actions.

## Idempotence and Recovery

The config and runtime edits are safe to rerun because Discord command registration already uses bulk overwrite semantics. Reapplying the final code should replace the prior command schema cleanly for the configured guild or globally, depending on `CLAW_DISCORD_GUILD_ID`.

The riskiest part of this migration is the new required environment variable. If startup fails after deployment, the recovery path is simple: set `CLAW_DISCORD_COMMAND_NAME` to a unique lowercase command name, then restart the process.

If a partial implementation leaves docs and code out of sync, prefer restoring consistency by finishing the command-surface change rather than reintroducing the old `/help` and `/task ...` registrations. This plan is intentionally opinionated so the repository does not drift into a dual-surface UX.

## Artifacts and Notes

Desired Discord search behavior after the plan:

    Installed bot instances:
      - Release bot -> /release
      - Docs bot -> /docs
      - Daily bot -> /daily

    User types:
      /re

    Discord search should show:
      /release

    Not:
      /help
      /task current
      /task list
      /task new
      /task switch
      /task close

Desired task-mode help text shape:

    Command: /release
    Mode: task
    Available actions:
    - `/release action:help` shows this help message.
    - `/release action:task-current` shows the active task.
    - `/release action:task-list` lists open tasks.
    - `/release action:task-new task_name:<name>` creates and activates a task.
    - `/release action:task-switch task_id:<id>` changes the active task.
    - `/release action:task-close task_id:<id>` closes a task.
    - Mention the bot in a message to continue the current conversation.

Out of scope reminders:

    mention text that literally starts with "/help" is still ordinary conversation input
    this plan does not add per-channel or per-user command surfaces
    this plan does not change thread policy, SQLite schema, or Codex execution flow

## Interfaces and Dependencies

At the end of this plan, the configuration and Discord runtime should expose interfaces shaped like this:

    type Config struct {
        Mode                 Mode
        Timezone             *time.Location
        TimezoneName         string
        DiscordToken         string
        DiscordGuildID       string
        DiscordCommandName   string
        ...
    }

    type commandRequest struct {
        Name     string
        UserID   string
        Action   string
        TaskName string
        TaskID   string
    }

    func registeredCommands(cfg config.Config) []*discordgo.ApplicationCommand

The root command should use one required option and up to two optional options:

    option "action"    -> required string choice
    option "task_name" -> optional string, interpreted only by action "task-new"
    option "task_id"   -> optional string, interpreted only by actions "task-switch" and "task-close"

The runtime must continue to depend on the existing `app.TaskCommandService` methods:

    ShowCurrentTask(ctx context.Context, userID string) (MessageResponse, error)
    ListTasks(ctx context.Context, userID string) (MessageResponse, error)
    CreateTask(ctx context.Context, userID string, taskName string) (MessageResponse, error)
    SwitchTask(ctx context.Context, userID string, taskID string) (MessageResponse, error)
    CloseTask(ctx context.Context, userID string, taskID string) (MessageResponse, error)

No change in this plan should bypass that app-layer contract or route task control into Codex.

Revision Note: 2026-04-04 / Codex - Created this ExecPlan after deciding to replace the shared `/help` and `/task ...` command family with one instance-specific root command in order to avoid Discord command-search explosion in multi-instance deployments.
Revision Note: 2026-04-05 / Codex - Simplified the plan to use only `CLAW_DISCORD_COMMAND_NAME` after deciding a second display-label setting added complexity without meaningful UX value.
