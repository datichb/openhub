# ADR-003 — Orchestrator with Explicit Checkpoints

## Status

~~Accepted~~ **Superseded by [ADR-042](./042-checkpoints-headless-decisions.en.md)**

## Context

When designing the `orchestrator` agent, two philosophies were in opposition:

1. **Full automation**: the orchestrator chains planner → developer → qa → reviewer without interruption, presenting a final result to the user.
2. **Explicit checkpoints**: the orchestrator pauses at each key step and waits for explicit confirmation before continuing.

Full automation seemed more fluid, but presented significant risks in a context where AI agents can produce incorrect, incomplete, or non-conforming results.

## Decision

The orchestrator enforces **explicit checkpoints** (noted `[CP-X]`) at each critical step:

- `[CP-0]` — Before starting the workflow (validation of planned tickets)
- `[CP-1]` — Before each ticket (confirmation to start)
- `[CP-QA]` — ~~Before the QA step (optional, user's choice)~~ **REMOVED (July 2026)** — The CP-QA checkpoint and `qa-engineer` agent have been removed. The `developer` now owns test writing; the `reviewer` verifies coverage; pre-review runs tests automatically. See ADR-023.
- `[CP-2]` — After review (merge or corrections?)
- `[CP-3]` — After each ticket (next ticket or stop?)

The orchestrator never advances to the next step without an explicit response.

## Evolutions

### May 2026 — Conditional activation of CP-QA

> **⚠️ REMOVED (July 2026)** — The `[CP-QA]` checkpoint and the `qa-engineer` agent were removed. Writing tests is now the responsibility of the `developer` agent (TDD or after implementation). The `reviewer` checks the test coverage. The pre-review workflow runs the tests automatically. See ADR-023.

~~The `[CP-QA]` checkpoint was improved with a **conditional activation based on the risk level** detected automatically in the diff:~~

~~**Behavior by risk:**~~

- ~~**🔴 High risk** (API, services, critical code, >200 lines) → QA mandatory, no checkpoint~~
- ~~**🟡 Medium risk** (utils, business logic in components) → QA recommended by default~~
- ~~**⚪ Low risk** (pure UI, docs, config) → QA optional~~

~~**TDD tickets:** instead of skipping QA automatically, a quick coverage audit checks that TDD was applied correctly (coverage >= 80%, every criterion covered). If TDD is incomplete, the qa-engineer writes the missing tests.~~

~~**Added value of the qa-engineer:** the qa-engineer now writes a `### Points to watch for the review` section in its handoff, passed to the reviewer to focus the review on the critical areas (untestable code, uncovered edge cases, assumptions made).~~

~~This approach maximizes quality on critical code without slowing down simple tickets.~~

## Consequences

### Positive

- The user maintains control at every step
- Errors from one agent are caught before propagating to subsequent steps
- Allows interrupting, skipping a ticket, or changing direction at any time
- Suitable for a context where AI agents are not infallible

### Negative / trade-offs

- Slower than a fully automated workflow
- Requires active user presence throughout the workflow
- Can become tedious on features with many simple tickets

## Rejected Alternatives

**Full automation**: rejected because an undetected implementation bug at ticket 2 can contaminate tickets 3 through N before the user intervenes.

**Automation with error-only alerts**: rejected because "no error" does not mean "conforms to expectations" — the review can flag functional problems that don't generate technical errors.

**Configurable mode** (auto / manual): possible as a future evolution, but introduces configuration complexity without proven immediate value.
