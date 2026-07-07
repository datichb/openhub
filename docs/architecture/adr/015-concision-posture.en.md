> 🇫🇷 [Lire en français](015-concision-posture.fr.md)

# ADR-015 — Concision posture skills for internal agents

## Status

Accepted

## Context

The hub's internal agents (orchestrator, orchestrator-dev, planner, pathfinder, developer, reviewer) produce verbose outputs that are not formal deliverables for the end user, but coordination exchanges. These outputs systematically contain:

- **Valueless intro phrases**: "Sure!", "I'm going to now...", "Here's what I found:"
- **Restatements of known context**: repetition of what the user just said or what is already established in the session
- **Redundant transitions between titled sections**: "Let's now move to the next section:" before a `##` heading
- **Closing formulas**: "Feel free to ask any other questions."

These patterns carry no information and unnecessarily lengthen responses. Over long sessions with several chained agents, this represents 30-40% of response token volume.

The caveman project (JuliusBrussee/caveman, 71k stars) validates this approach at scale: average 65% reduction in output tokens across 10 benchmarks (22-87% depending on task type) with 100% technical accuracy maintained. The research paper "Brevity Constraints Reverse Performance Hierarchies in Language Models" (arxiv, March 2026) confirms that constraining to brevity improves accuracy by 26 points on certain benchmarks.

However, caveman in `full` or `ultra` mode is too aggressive for a hub where some agents produce formal deliverables (audit reports, UX specs, diagnostic reports). A `lite` level — filler suppression only — is the right trade-off for primary agents.

For subagents, the situation differs fundamentally: their output is consumed by a coordinator agent, not a human. This justifies a dedicated, more aggressive skill: `posture/subagent-concision-posture`.

The decision is to create two separate custom skills rather than a single multi-level skill, to avoid ambiguity in agent context and keep each skill's injected size minimal.

1. **Per-agent control**: skills are selectively injected into relevant agents. The caveman plugin is global.
2. **Preserved formalism**: the `lite` level is precisely defined to not touch formal deliverables. caveman `full` mode does not make this distinction.
3. **Clear separation of concerns**: primary agents get `concision-posture` (lite), subagents get `subagent-concision-posture` (compact). No level ambiguity.
4. **No external dependency**: Markdown skills have no npm/binary prerequisites.

## Decision

Create two skills:

### `skills/posture/concision-posture.md` — `lite` level for primary agents

**`lite` level — suppresses only:**
- Valueless intro phrases ("Sure!", "I'm going to...", "Here is...")
- Known-context restatements already in the session
- Redundant transitions between titled sections
- Closing formulas ("Feel free to...", "I hope this...")

**Does not affect:**
- Handoff blocks (functional contracts)
- Mandatory narrative recaps (planner, onboarder, designers)
- Review reports, QA reports, diagnostic reports
- Technical justifications, warnings, hypotheses

**Agents in scope:** orchestrator, orchestrator-dev, planner, pathfinder, reviewer

---

### `skills/posture/subagent-concision-posture.md` — `compact` level for subagents

**Principle:** a subagent's output is consumed by a coordinator agent, not a human. The expected content is: (1) the structured handoff block, (2) raw technical data not encodable in that block.

**Suppresses (in addition to everything `lite` suppresses):**
- Method explanations ("I explored files X, Y, Z starting with...")
- Decision justifications in free prose — these go in the handoff block's dedicated fields (`risks`, `recommendations`)
- Non-critical warnings outside the handoff block
- Pre-handoff summary of work done (the handoff block already contains this)

**Never suppresses:**
- The complete handoff block (non-negotiable functional contract)
- Raw technical data the coordinator must receive: stacktraces, diffs, code excerpts with line numbers

**Decision rule:** "Is this content in the handoff block? → If yes: do not repeat in prose. If no: is it raw technical data the coordinator must receive? → If no: do not write it."

**Agents in scope:** developer, developer-refactor, developer-migrator, auditor-architecture, auditor-security, auditor-observability, auditor-ecodesign, auditor-accessibility, auditor-performance, auditor-privacy

**Note on auditor-* agents:** previously excluded from any concision skill (their reports were considered formal deliverables). This decision is revised: auditor reports are consumed by the `auditor` coordinator which retranscribes them to the user. The subagent-concision-posture skill does not suppress handoff block content — it eliminates only the prose wrapper around it.

**Configuration**: `token_optimization.output_verbosity: "lite"` (primary agents) and `token_optimization.subagent_verbosity: "subagent"` (subagents) in `config/hub.json`.

## Consequences

### Positive

- **-30-40% output tokens on primary agents** (`lite` level, coordination exchanges).
- **-40-60% output tokens on subagents** (`compact` level, inter-agent exchanges).
- **No information loss**: `lite` removes only syntactic noise; `compact` removes prose wrappers while preserving handoff block integrity and raw technical data.
- **No ambiguity**: two distinct skills with explicit, non-overlapping scopes. Each agent loads only what applies to its communication mode.
- **Lighter context per agent**: each skill is smaller than a combined multi-level skill would be.
- **Configurable**: verbosity keys in `hub.json` document the active levels.
- **No dependency**: Markdown files in `skills/posture/`, zero setup.

### Negative / trade-offs

- **Two skills to maintain**: any change to shared principles (e.g. what filler looks like as models evolve) must be applied to both files.
- **Over-concision risk on subagents**: the `compact` level is more aggressive. The decision rule ("is it in the handoff block or raw technical data?") is the guard against information loss.

## Rejected Alternatives

**caveman plugin as-is**: caveman in `full` mode does not distinguish coordination exchanges from formal deliverables. No per-agent control. Additional npm dependency.

**Single skill with multiple levels (`lite`, `subagent`)**: reduces file count but increases context size for every agent (they load rules that don't apply to them) and introduces level-selection ambiguity. Two focused skills are cleaner.

**Concision rules in each agent separately**: content duplication, distributed maintenance, inconsistency risk between agents.

**Do nothing**: output tokens represent 40-60% of total cost on long multi-agent sessions. Filler is an observable and measurable pattern.

## Impact

| File | Action |
|------|--------|
| `skills/posture/concision-posture.md` | Modified — scope updated to primary agents only, references to subagents removed |
| `skills/posture/subagent-concision-posture.md` | Created — compact level for all mode:subagent agents |
| `config/hub.json` | Modified — added `token_optimization.subagent_verbosity: "subagent"` |
| `agents/developer/developer.md` | Modified — `posture/concision-posture` replaced by `posture/subagent-concision-posture` |
| `agents/developer/developer-refactor.md` | Modified — `posture/subagent-concision-posture` added |
| `agents/developer/developer-migrator.md` | Modified — `posture/subagent-concision-posture` added |
| `agents/quality/debugger.md` | Modified — `posture/subagent-concision-posture` removed (fix C-3: switched to `mode: primary`, dual-role pattern via `debugger-subagent`) |
| `agents/auditor/auditor-architecture.md` | Modified — `posture/subagent-concision-posture` added |
| `agents/auditor/auditor-security.md` | Modified — `posture/subagent-concision-posture` added |
| `agents/auditor/auditor-observability.md` | Modified — `posture/subagent-concision-posture` added |
| `agents/auditor/auditor-ecodesign.md` | Modified — `posture/subagent-concision-posture` added |
| `agents/auditor/auditor-accessibility.md` | Modified — `posture/subagent-concision-posture` added |
| `agents/auditor/auditor-performance.md` | Modified — `posture/subagent-concision-posture` added |
| `agents/auditor/auditor-privacy.md` | Modified — `posture/subagent-concision-posture` added |
