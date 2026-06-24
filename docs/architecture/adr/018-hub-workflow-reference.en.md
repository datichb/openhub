> 🇫🇷 [Lire en français](018-hub-workflow-reference.fr.md)

# ADR-018 — Extracting hub workflow into a canonical `hub-workflow-reference` skill

## Status

Proposed

## Context

The BMAD × Superpowers comparative analysis (2026-06-24) revealed that the hub workflow description — agent catalogue, routing heuristic, handoff table — is **duplicated across at least 4 locations**:

| Source | Duplicated content | Estimated size |
|---|---|---|
| `agents/planning/orchestrator.md` lines ~40–91 | Agent catalogue + routing heuristic | ~60 lines |
| `skills/orchestrator/orchestrator-protocol.md` routing section | Full routing + modes A/B/D/E | ~300 lines |
| `skills/planning/planner-workflow.md` lines 42–110 | Orchestrator routing table, all agents | ~70 lines |
| `agents/planning/orchestrator-dev.md` | Partial domain→agent mapping | ~50 lines |

This duplication creates several problems:

**Coherence drift:** When an agent is added or modified, all 4 sources must be updated manually. In practice, some have already diverged (e.g., `planner-workflow.md` declares itself "routing source of truth" while `orchestrator-protocol.md` contains a more detailed version).

**Difficult onboarding:** A new user cannot understand the hub workflow from a single entry point — it must be reconstructed from multiple disparate files.

**Maintenance cost:** Adding each new agent requires updating 4 locations instead of one.

**Existing precedent:** The hub already solved this for a similar case — `orchestrator-workflow-modes.md` is declared "single source of truth" for execution modes and is consumed by both `orchestrator` and `orchestrator-dev` via bucket-A. This pattern is proven and functional.

## Decision

Create a skill `skills/shared/hub-workflow-reference.md` declared as the **canonical source of truth** for:

1. The agent catalogue (family, role, mode, when to invoke, expected output)
2. The pathfinder vs planner routing heuristic (formalized decision criteria)
3. The handoff table (sender → format → receiver)
4. Standard chaining order and variants
5. Integration with complexity scoring (item K of BMAD plan)

Replace duplicated sections in the 4 existing sources with references to the canonical skill, following the `orchestrator-workflow-modes.md` pattern.

## Consequences

### Positive

- **Single source of truth** — adding an agent is done in one place only
- **In-session guide** — any agent can load `@hub-workflow-reference` to know its position in the workflow and available agents
- **Reduced maintenance** — the 4 duplicated sources become pointers
- **Guaranteed consistency** — the routing seen by the orchestrator = the routing seen by the planner
- **Foundation for `oc-help`** — the skill becomes the substrate for an in-session guidance command

### Negative / Risks

- **Post-refactor drift risk** — if an agent is modified without updating the central skill. Mitigation: rule formalized in `docs/guides/authoring-skills.md` (item F) + mentioned in the skill itself
- **Breaking the "planner = routing source of truth" contract** — `planner-workflow.md` currently declares itself the source of truth. The status must be explicitly transferred to the new skill, and `planner-workflow.md`'s header must be updated
- **Behavioral regression risk** — if operational content (invocation templates, behavioral rules) is moved along with descriptive content. Mitigation: move **only** catalogue/routing/handoffs; leave operational rules in each agent/skill

### What stays in each file

| File | What stays (does not move) |
|---|---|
| `orchestrator-protocol.md` | Operational rules, invocation templates, CP management, retransmission protocols |
| `planner-workflow.md` | The 7 planning phases, Beads templates, behavioral constraints |
| `orchestrator.md` | Permissions, behavioral rules, handoff-format loading |
| `orchestrator-dev.md` | Domain→native_skills mapping (implementation-specific), delegation rules |

## Alternatives Considered

### Option 1 — Standalone skill without modifying existing sources

Create `hub-workflow-reference.md` as pure documentation without touching the 4 sources. Advantage: zero regression risk. Disadvantage: duplication persists and the skill drifts immediately on the first agent addition.

**Rejected** — solves discovery but not maintenance and consistency.

### Option 2 (selected) — Extract + replace with pointers

Same pattern as `orchestrator-workflow-modes.md`. Extract duplicated content, replace with `@hub-workflow-reference` in the 4 sources.

### Option 3 — Centralize everything in `orchestrator-protocol.md`

`orchestrator-protocol.md` is already the most complete document (1172 lines). It could be declared the source of truth with others pointing to it. Disadvantage: this file is orchestrator-specific — planner and orchestrator-dev would consume an "orchestrator" skill for their own workflow, which is semantically incorrect.

**Rejected** — inappropriate coupling.

## Governance Rule

Adding a new agent to the hub **must** include an update to `hub-workflow-reference.md`. This rule is documented in `docs/guides/authoring-skills.md` (item F).

The header of `hub-workflow-reference.md` must explicitly declare: `source-of-truth: true`.
