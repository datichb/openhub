# ADR-003 — Orchestrator with Explicit Checkpoints

## Status

Accepted

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
