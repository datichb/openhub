# ADR-013 — Consolidation of developer-* agents into a generic developer agent

## Status

Accepted — supersedes [ADR-002](./002-developer-segmentation.en.md)

## Context

ADR-002 split the original `developer.md` into 9 specialized agents to reduce
the injected context and enable precise routing. At the time, this was the right
trade-off: each agent loaded only its domain-relevant skills.

Since then, the hub has evolved significantly:

- **Hybrid skills architecture (ADR-010)** introduced Bucket B (native_skills),
  which are loaded on-demand by the LLM via the `skill` tool — not injected at startup.
  The original "context size" concern from ADR-002 is now resolved at the architecture level.
- **Dynamic skill injection** (ADR-008) allows stacks to be injected at invocation time,
  not baked into the agent file.
- **9 agent files sharing identical structure** — same permissions, same Beads workflow,
  same Bucket A skills — created a maintenance burden where any fix (e.g., to the handoff
  format, the Beads workflow steps, a permission change) had to be applied 9 times.

The context-reduction benefit from ADR-002 now comes from Bucket B (native_skills loaded
on-demand), not from having separate agent files. The segmentation served its purpose but
is no longer the right architectural lever.

## Decision

The 9 `developer-*` agents are merged into a single generic `developer` agent.
Specialization is no longer encoded in the agent file — it is passed at **invocation time**
by `orchestrator-dev` via the prompt, which specifies:

1. The **domain** (`frontend`, `backend`, `fullstack`, `api`, `mobile`, `data`, `devops`, `platform`, `security`)
2. The **native_skills to load** for that domain (explicit list per domain in the routing protocol)

`developer-refactor` and `developer-migrator` remain as separate agents — their workflows
are fundamentally different (no new features, specific safety constraints, pre-condition checks).

## Consequences

### Positive

- **Single file to maintain** — any change to the Beads workflow, permissions, or handoff
  format is applied in one place
- **Guaranteed consistency** — behavioral drift between agents (e.g., one agent missing a
  permission fix) becomes impossible
- **Extensibility** — adding a new domain requires only a new entry in the routing protocol
  and `stack-skills.json`, not a new agent file
- **Parallel isolation confirmed** — when `orchestrator-dev` invokes multiple `developer`
  agents in parallel via `task`, each instance receives its own isolated session context.
  Skills loaded in one session do not leak into another.

### Negative / trade-offs

- **The invocation prompt is now the carrier of domain context** — if `orchestrator-dev`
  sends a malformed or incomplete prompt (missing domain or skills list), the agent will
  lack specialization. Mitigated by the explicit format defined in `orchestrator-dev-protocol.md`.
- **Loss of agent-level description granularity** — the OpenCode tab picker and agent
  lists now show a single `developer` entry instead of 9 named specializations. Acceptable
  since `developer` is a subagent (hidden from the picker) and only invoked by `orchestrator-dev`.

## Rejected Alternatives

**Keep all 9 agents, add a shared base skill**: reduces duplication but doesn't eliminate it —
the 9 files still exist, still diverge over time, still require 9 updates for each structural change.

**Keep specialized agents, load all their Bucket B skills generically**: defeats the purpose
of specialization — the agent would receive all domain standards at once.

## Impact

| File | Action |
|------|--------|
| `agents/developer/developer.md` | Created |
| `agents/developer/developer-frontend.md` | Deleted |
| `agents/developer/developer-backend.md` | Deleted |
| `agents/developer/developer-fullstack.md` | Deleted |
| `agents/developer/developer-api.md` | Deleted |
| `agents/developer/developer-mobile.md` | Deleted |
| `agents/developer/developer-data.md` | Deleted |
| `agents/developer/developer-devops.md` | Deleted |
| `agents/developer/developer-platform.md` | Deleted |
| `agents/developer/developer-security.md` | Deleted |
| `agents/developer/developer-refactor.md` | Kept |
| `agents/developer/developer-migrator.md` | Kept |
| `agents/planning/orchestrator-dev.md` | Updated (agent table + task permissions) |
| `skills/orchestrator/orchestrator-dev-protocol.md` | Updated (routing matrix + invocation format) |
| `config/stack-skills.json` | Updated (`_agent_scope` key) |
