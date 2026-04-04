# Tech Debt Tracker

This file records follow-up work that is intentionally deferred while delivering an active ExecPlan.

## How to Use This File

- Add one entry when implementation deliberately leaves a non-blocking gap behind.
- Link the relevant ExecPlan and explain why the work was deferred.
- Remove or mark an entry resolved when the follow-up lands.

## Entry Template

### Title

- Status: `open` or `resolved`
- Date:
- Related plan:
- Owner:

### Context

Explain the current behavior and why the gap exists.

### Risk

Explain the user-facing or contributor-facing downside if the debt remains.

### Next step

Describe the smallest safe follow-up that should address the debt.

## Current Entries

### Discord runtime smoke test follow-up

- Status: open
- Date: 2026-04-05
- Related plan: `docs/exec-plans/completed/04-discord-runtime-and-presentation.md`
- Owner: Unassigned

### Context

The Discord runtime implementation has landed and the delivery plan has been archived, but the final manual smoke test against a disposable Discord server was not run before archival because disposable bot credentials were not available in the implementation environment.

### Risk

Automated tests prove message mapping, command routing, chunking, and presentation behavior, but they cannot prove that live Discord registration and reply flow behave exactly as expected in a real server.

### Next step

Run the documented smoke test from `README.md` with a disposable Discord bot token and guild ID, then either mark this entry resolved or record any runtime-specific fixes in a follow-up ExecPlan.
