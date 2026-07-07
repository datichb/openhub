# ADR-004 — QA Engineer and Debugger as Separate Agents

## Status

Partially superseded (July 2026)

> **Note:** The `qa-engineer` agent has been removed (July 2026). Its responsibilities (test writing) are transferred to the `developer` agent. The `reviewer` verifies coverage. The `debugger` agent remains unchanged. See ADR-023.

## Context

Two responsibilities were initially absent or diluted across existing agents:

1. **Writing tests**: developer agents write their own tests, which introduces confirmation bias — you test what you coded, not what should work.

2. **Bug diagnosis**: developer agents can also debug, but rigorous diagnosis (reading stacktraces, graded hypotheses, root cause isolation) is a distinct skill from implementation.

The question was: integrate these responsibilities into existing agents (developer, reviewer) or create dedicated agents?

## Decision

Two dedicated agents are created:

- **`qa-engineer`**: receives an implementation, writes missing tests (unit / integration / E2E), produces a coverage report. Never modifies functional code. Invocable standalone or as an optional `[CP-QA]` step in the orchestrator.

- **`debugger`**: receives a stacktrace or logs, applies a 4-step diagnostic methodology, produces a root cause report with graded hypotheses, and creates a Beads correction ticket after confirmation. Never fixes the bug.

## Consequences

### Positive

- Separation of responsibilities: implement ≠ test ≠ diagnose
- QA has an independent view of the implementation (no author bias)
- The Debugger formalizes diagnosis before correction, reducing fixes in the wrong direction
- Both agents are invocable independently of the orchestrator workflow

### Negative / trade-offs

- 2 additional agents to maintain
- QA must understand the implementation without being its author — depends on the quality of the provided diff/context
- The boundary between "debugger identifies" and "developer fixes" can create back-and-forth if the diagnosis is incomplete

## Rejected Alternatives

**QA integrated into developer**: rejected — same agent, same bias, no external perspective.

**Extended review**: giving the reviewer the responsibility of writing missing tests. Rejected because the reviewer is read-only by principle (implicit ADR-001) and because conflating review and test writing blurs responsibilities.

**Debugger integrated into developer**: rejected because rigorous diagnosis with graded hypotheses requires a distinct mode of thinking from implementation — mixing the two pushes toward fixing before the root cause is identified.
