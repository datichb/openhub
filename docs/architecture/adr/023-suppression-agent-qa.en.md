# 023 — Removal of qa-engineer agent and CP-QA checkpoint

## Status

accepted

## Context

The `qa-engineer` agent was a dedicated agent that intervened between the implementation (developer) and the review (reviewer) to write missing tests and produce a coverage report. Its invocation was managed by the CP-QA checkpoint in `orchestrator-dev`, with conditional activation based on the diff risk level (high/medium/low).

In practice, this separation introduced several issues:

- **Workflow latency**: an additional agent in the loop added a full delegation/return cycle between implementation and review
- **Redundancy**: the `developer` agent already wrote tests at step 5 of its workflow, making the QA pass often redundant
- **orchestrator-dev protocol complexity**: CP-QA management (risk evaluation, modes, auto/manual configuration) represented ~220 lines of protocol
- **Pre-review (step 3.5) already runs `npm test`**: tests are automatically executed before any review, ensuring the code is functional

## Decision

We decided to:

1. **Remove the `qa-engineer` agent** and all its associated skills (`qa-protocol`, `qa-standalone`, `qa-subagent`, `qa-handoff-format`)
2. **Remove the CP-QA checkpoint** from the `orchestrator-dev` workflow
3. **Transfer test coverage responsibility to the `developer`** — enriching `dev-standards-testing` skill with the systematic checklist and completion gate from `qa-protocol`
4. **Add a coverage criterion to the `reviewer`** — the reviewer verifies that acceptance criteria are covered by tests and can request additional tests via a 🟠 Major finding

## Consequences

### Positive

- **Simplified workflow**: Developer → Pre-review (auto tests) → Reviewer → CP-2
- **Less latency**: removal of a full agent cycle from the loop
- **Lighter orchestrator protocol**: −220 lines, removal of a complete step and its conditional logic
- **Clear single responsibility**: the developer owns test coverage, the reviewer validates
- **Direct feedback loop**: if tests are insufficient, the reviewer flags it and the developer fixes at next cycle

### Negative / Trade-offs

- **Loss of QA specialization**: the concentrated expertise of the QA agent (exclusive focus on tests, structured report) is diluted into the developer
- **Increased cognitive load for the developer**: must apply the systematic coverage checklist on top of implementation
- **Less separation of concerns**: the same agent writing code writes tests — potential bias toward tests that follow implementation rather than behavior

## Rejected alternatives

| Alternative | Reason for rejection |
|-------------|---------------------|
| Keep QA but make it mandatory only for high risk | Doesn't resolve latency or redundancy — developer already writes tests |
| Transform QA into a simple automated coverage check | Pre-review (step 3.5) already runs tests — duplication |
| Merge QA into reviewer instead of developer | Reviewer is read-only — cannot write tests, only request them |
