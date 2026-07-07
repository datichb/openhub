> 🇫🇷 [Lire en français](016-execution-path-skills.fr.md)

# ADR-016 — Execution Path Skills: Extracting the Standalone/Subagent Dual Path

## Status

Accepted

## Context

Directly-invocable primary agents (`planner`, `pathfinder`, `onboarder`, `auditor`, `orchestrator-dev`, `reviewer`, `qa-engineer`) can be invoked in two ways:

1. **Standalone** — directly by the user: communication via the `question` tool, text recaps visible in the discussion, no orchestrator handoff blocks
2. **Subagent** — via `task` from the orchestrator: session interruption mechanism, structured blocks `## Retour intermédiaire vers orchestrator` + `## Question pour l'orchestrator`, mandatory `task_id`, `question` tool forbidden

Before this decision, both paths coexisted in agent workflow skills (e.g. `planner-workflow.md`), separated by a conditional branch detected at startup:

```
If prompt contains `[CONTEXTE] Invoqué depuis l'orchestrateur feature`:
  → Subagent path
Else:
  → Standalone path
```

This architecture had several problems:

1. **Unnecessary context load**: both paths (up to ~300 lines each) always traveled together in context, even though only one was active per session.
2. **Path confusion**: cross-rule branching ("if CONTEXTE = X, else…") increased the risk of LLM errors mid-session — the anti-error checklists in the skills were a symptom.
3. **Strong coupling**: adding a rule to the subagent path required editing a complex skill shared with the standalone path, increasing regression risk.
4. **Degraded testability**: it was not possible to test one path independently of the other.

## Decision

Extract each execution path into a dedicated **Bucket B skill**, loaded at session startup based on the invocation context.

### Loading Mechanism — Option B1 (injection + fallback)

The orchestrator injects the subagent skill into the `task` prompt via the `[SKILL:<name>]` marker:

```
[CONTEXTE] Invoqué depuis l'orchestrateur feature.
[SKILL:planning/planner-subagent]
```

The agent applies the following rule at startup:

> If the prompt contains `[SKILL:<name>]` → load that skill via the `skill` tool.
> Otherwise (direct invocation) → load the `<agent>-standalone` skill (implicit default).

### Structure of New Skills

For each affected agent, two Bucket B skills are created:

| Agent | Standalone skill | Subagent skill |
|-------|-----------------|----------------|
| `planner` | `planning/planner-standalone` | `planning/planner-subagent` |
| `pathfinder` | `planning/pathfinder-standalone` | `planning/pathfinder-subagent` |
| `onboarder` | `planning/onboarder-standalone` | `planning/onboarder-subagent` |
| `auditor` | `auditor/auditor-standalone` | `auditor/auditor-subagent` |
| `orchestrator-dev` | `orchestrator/orchestrator-dev-standalone` | `orchestrator/orchestrator-dev-subagent` |
| `reviewer` | `reviewer/reviewer-standalone` | `reviewer/reviewer-subagent` |
| `qa-engineer` | `qa/qa-standalone` | `qa/qa-subagent` |

### Skill Contents

**`-standalone` skill**:
- Absolute rule: text recap before `question` tool call
- Self-check before each checkpoint
- Format of validation questions via `question` tool (one per phase)
- Final output format (no orchestrator handoff block)

> **Note (reviewer):** The reviewer's standalone path additionally handles
> **multi-mode orchestration** — interactive mode selection (standard / adversarial /
> edge-case / combinations), parallel session launch for combined modes via self-delegation
> (`task → reviewer`), and report fusion via the `review-merge` skill. This extends
> the base pattern without breaking the standalone/subagent contract.

**`-subagent` skill**:
- Context confirmation at startup
- Interruption mechanism: produce recap + structured blocks + terminate session
- Self-check before each session end
- Format of `## Retour intermédiaire vers orchestrator` and `## Question pour l'orchestrator` blocks with `task_id`
- Final output format (with orchestrator handoff block)
- List of common errors to avoid

### What Does Not Change

Source skills (`planner-workflow`, `onboarder-workflow`, etc.) retain the detailed workflow phases, Beads creation templates, and business rules. Only the standalone/subagent bifurcation logic is removed and replaced with a reference to the new dedicated skills.

## Consequences

### Positive

- **Context reduction**: in standalone sessions, the subagent path is never loaded (~30-60% reduction on path skills). In subagent sessions, vice versa.
- **Clarity**: each skill has a single responsibility, without conditional branching.
- **Testability**: both paths can be tested and validated independently.
- **Extensibility**: adding behavior to one path no longer risks disrupting the other.
- **Observability**: the orchestrator explicitly controls which path is activated via `[SKILL:...]` injection.

### Watch Out For

- The orchestrator must inject `[SKILL:...]` in **all** `task` prompts to affected agents, including `task_id` re-invocations.
- Affected agents must have `skill: allow` in their permissions to load the skill at startup.
- If `[SKILL:...]` is omitted in a subagent prompt, the agent loads the standalone skill by default — degraded but not broken behavior.

## Alternatives Considered

### Option A — Full auto-detection by the agent

The agent detects the `[CONTEXTE]` marker itself and loads the right skill. This option was rejected because it keeps detection logic in the agent and does not allow the orchestrator to explicitly control the activated path.

### Option B2 — Standalone as auto-loaded native_skill

The standalone skill is always loaded automatically; only the subagent skill is injected in the prompt. This option is less explicit: it is not clear from reading the prompt whether the standalone path is active by default or not.

**Option B1** (implemented) offers the best balance: the standalone fallback is implicit and predictable, subagent injection is explicit and observable.
