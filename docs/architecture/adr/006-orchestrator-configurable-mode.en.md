# ADR-006 — Configurable Workflow Mode for the Dev Orchestrator

## Status

Accepted

## Context

ADR-003 established a fully manual workflow for the orchestrator: each checkpoint (`[CP-0]` through `[CP-3]`) waits for an explicit user response before continuing. This decision was justified to guarantee control and prevent error propagation.

However, real-world usage revealed a frequent case where this rigor becomes friction without value: features with many homogeneous, well-planned tickets (e.g. CRUD on N entities, sequential migrations, repetitive refactoring tasks). The user types `yes`, `next`, `yes`, `next`... in a loop, without ever exercising real judgment on those steps.

ADR-003 had explicitly noted configurable mode as *"possible as a future evolution"*, without immediate proven value. That proof is now established.

Since then, the orchestrator has been split into two levels (see the architecture refactor):
- `orchestrator` — feature project manager (design → audit → implementation)
- `orchestrator-dev` — implementation tech lead (developer-* + QA + review)

**The modes in this ADR apply exclusively to `orchestrator-dev`.**
The feature `orchestrator` keeps always-manual checkpoints (CP-0, CP-spec, CP-audit, CP-feature) — none of these checkpoints can be automated.

## Decision

`orchestrator-dev` offers **three workflow modes**, chosen once for the entire session at the `[CP-0]` moment:

| Mode | Description |
|------|-------------|
| `manual` | Original ADR-003 behavior — all checkpoints are pauses |
| `semi-auto` | CP-1 and CP-3 automatic, CP-0 / CP-2 remain manual |
| `auto` | CP-0, CP-1, CP-3 automatic, CP-2 **always manual** |

**The default mode is `manual`** — existing behavior is preserved without change for users who don't specify a mode.

**CP-2 (merge or fix?) is non-automatable in all modes.** This rule is absolute: "no technical error" ≠ "conforms to functional expectations". The merge decision commits the user's responsibility.

The mode is declared when invoking `orchestrator-dev` or selected at the `[CP-0]` moment. When `orchestrator-dev` is invoked from the `orchestrator`, the mode is passed as a parameter.

## Consequences

### Positive

- Eliminates repetitive friction on features with homogeneous tickets
- Preserves existing behavior by default (`manual`) — backward-compatible
- CP-2 remains always manual: the error propagation risk identified in ADR-003 is kept under control
- The user is always free to type "stop" at any time — `semi-auto` and `auto` modes reduce pauses but don't prevent interruption
- The split into two orchestrators clarifies scope: modes only concern the implementation phase, never the design or audit phase

### Negative / trade-offs

- Slight additional complexity in the `orchestrator-dev-protocol` skill
- The user needs to know the 3 modes to benefit from them (mitigated by the question asked at CP-0)

## Rejected Alternatives

**Configurable mode in `projects.md`**: useful persistence for projects with a preferred mode, but introduces coupling between project configuration and a specific agent's behavior. Can be grafted onto this decision as a future evolution if the need is confirmed.

**Suppression of CP-0**: rejected — CP-0 is the initial consent to start the workflow. Removing it would mean an accidental invocation of the orchestrator would start a complete workflow without confirmation.

**Automatic CP-2 with confidence score**: rejected — a confidence score on an AI review report introduces false precision. The merge decision is a non-delegable human responsibility.
