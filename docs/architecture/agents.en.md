# Agent Reference

13 agents in total, organized into 6 families.
Each agent is defined in `agents/<family>/<id>.md` with a frontmatter declaring its metadata,
skills, and mode.

---

## Agent Format

```markdown
---
id: <unique-identifier>
label: <DisplayedName>
description: <Short description — visible in AI tools>
mode: primary         # primary (default) | subagent
permission:
  question: allow     # optional — enables OpenCode's question tool (interactive primary agents only)
  skill: allow        # allow | deny — enables the native skill tool (Bucket B)
skills: [path/to/skill, ...]          # Bucket A — assembled inline at deploy time
native_skills: [path/to/skill, ...]   # Bucket B — deployed to .opencode/skills/, loaded on-demand
---

# <Title>

<Agent body>
```

| Field | Role |
|-------|------|
| `id` | Unique identifier, used by adapters and `oc agent` |
| `label` | Name displayed in the tool |
| `description` | Short phrase describing the role — appears in agent lists |
| `mode` | `primary` (default) or `subagent` — controls visibility in OpenCode |
| `permission.question` | `allow` — enables OpenCode's `question` tool for this agent. Reserved for interactive `primary` agents. Always paired with the `posture/tool-question` skill. |
| `permission.skill` | `allow` — enables the native `skill` tool so the agent can load Bucket B skills on-demand. Set to `deny` for coordinators/orchestrators that never need contextual skills. |
| `skills` | **Bucket A** — paths relative to `skills/`, injected inline at deploy time, always active from the first token. Workflow protocols, handoff formats, universal principles. |
| `native_skills` | **Bucket B** — paths relative to `skills/`, deployed to `.opencode/skills/<name>/SKILL.md`, loaded on-demand by the LLM via the `skill` tool. Domain standards, stack skills, checklists. |

See [ADR-010](./adr/010-hybrid-skills-architecture.en.md) for the rationale behind the Bucket A / Bucket B split.

### Primary / Subagent Modes

The `mode:` field controls how an agent is exposed in OpenCode:

| Mode | OpenCode |
|------|----------|
| `primary` | Visible in the Tab picker — present in `.opencode/agents/` |
| `subagent` | Listed in `opencode.json` with `"mode": "subagent"` — invocable by other agents, hidden in Tab picker. Present in `.opencode/agents/` with delegation-oriented description. |

The effective mode follows a priority: **project override** (`- Modes:` in `projects.md`) > **agent frontmatter** > **`primary`** (default).

To modify modes for a project without touching frontmatter: `oc agent mode <PROJECT_ID>`.

---

## Family — Coordinators

Agents that drive other agents without ever coding themselves.

### `onboarder`

| | |
|--|--|
| **Label** | Onboarder |
| **File** | `agents/planning/onboarder.md` |
| **Skills** | `planning/onboarder-workflow`, `planning/onboarder-handoff-format`, `posture/expert-posture`, `posture/tool-question`, `developer/beads-plan`, `developer/dev-standards-git`, `shared/living-docs-enrichment` |
| **Invocation** | `"Onboard yourself on this project"` / `"Discover this project"` / `"Before starting, explore the project"` |

Project discovery agent. Explores an existing project's codebase in 6 structured phases
(prerequisites check → adaptive exploration 7 profiles → questions → context report →
edge case detection → deliverables production). Produces `ONBOARDING.md`, `CONVENTIONS.md`
and optionally `projects.md`.

Detects edge cases: stack/conventions inconsistencies, known CVEs, hidden technical debt,
undocumented hybrid architecture. Produces a prioritized agent map in 3 levels
(priority by risk, recommended by stack, optional).

Read-only — never modifies files (except the deliverables it produces).
Never automatically triggers another agent — it suggests invocations, the user decides.

Invocable directly, from `oc start` (suggestion displayed), or from the `orchestrator`
(Mode C — pre-phase on unknown project).

**Phase 5 — Incremental enrichment:** when `ONBOARDING.md` and `CONVENTIONS.md` already exist (enriched by other agents), proposes incremental enrichment rather than a full overwrite. Delegates incremental updates to the `documentarian` via `task` (skill `living-docs-enrichment`). Full overwrite remains available with an explicit warning about losing accumulated enrichments.

---

### `orchestrator`

| | |
|--|--|
| **Label** | Orchestrator |
| **File** | `agents/planning/orchestrator.md` |
| **Skills** | `orchestrator/orchestrator-protocol`, `orchestrator/orchestrator-workflow-modes`, `orchestrator/orchestrator-handoff-format`, `developer/beads-plan`, `posture/tool-question`, `design/design-handoff-format`, `auditor/audit-handoff-format`, `planning/planner-handoff-format`, `planning/onboarder-handoff-format`, `quality/debugger-handoff-format` |
| **Invocation** | `"Implement [feature]"` / `"Handle tickets [IDs]"` |

AI project manager. Drives the complete delivery of a feature by mobilizing all
necessary agents: design (ux-designer, ui-designer), audit (auditor-*),
implementation (via orchestrator-dev). Enforces explicit checkpoints at each
phase. Never codes.

**Four entry modes:**
- **Mode D** — bug reported → delegates immediately to `debugger`, no analysis
- **Mode C** — no project context in session → proposes `onboarder` if needed
- **Mode A** — feature in natural language → delegates to `planner`
- **Mode B** — existing Beads tickets → transmits IDs directly to `planner` (no `bd show`)

Never routes directly to `developer-*` — always delegates to `orchestrator-dev`.

**Technical permissions:** `bash`, `read`, `edit`, `write` all disabled. Acts only via `task` (delegation) and `question` (checkpoints). List of invocable agents explicitly restricted in the frontmatter.

**Context injection:** project context (stack, conventions) is automatically injected into the session via the `instructions` field of `opencode.json` (valid cache `.opencode/context.json` or `ONBOARDING.md`/`CONVENTIONS.md`). The orchestrator never reads files directly — if context is absent from the session, it proposes the `onboarder`.

**Missing agent handling:** if a required agent is not deployed in the project, the orchestrator asks a structured question with options: deploy via `!oc deploy` without leaving OpenCode / use a substitute (substitution table by domain) / skip the ticket. Never silently falls back to another agent.

---

### `orchestrator-dev`

| | |
|--|--|
| **Label** | OrchestratorDev |
| **File** | `agents/planning/orchestrator-dev.md` |
| **Skills** | `orchestrator/orchestrator-dev-protocol`, `orchestrator/orchestrator-handoff-format`, `orchestrator/orchestrator-workflow-modes`, `posture/tool-question`, `developer/developer-handoff-format`, `reviewer/reviewer-handoff-format`, `qa/qa-handoff-format` |
| **Invocation** | `"Implement tickets [IDs]"` / `"Dev workflow for [feature]"` |

AI tech lead specialized in driving implementation. Takes a list of ready-to-implement
Beads tickets, routes to the `developer` agent with the appropriate domain specified in the
invocation prompt, supervises optional QA and review.
Three modes: `manual` (default), `semi-auto`, `auto`. Invocable standalone or from the `orchestrator`.

CP-2 (commit or fix?) is always manual in all modes.

`bd close`, `bd comments add`, and `bd update` are always executed by the `developer-*` agents in delegation prompts — never directly by `orchestrator-dev`. The orchestrator-dev only reads Beads tickets (`bd show`, `bd list`).

> See [ADR-006](./adr/006-orchestrator-configurable-mode.en.md) — modes apply to `orchestrator-dev` only.

---

### `auditor`

| | |
|--|--|
| **Label** | Auditor |
| **File** | `agents/auditor/auditor.md` |
| **Skills** | `auditor/auditor-workflow`, `auditor/audit-protocol-light`, `auditor/audit-handoff-format`, `shared/living-docs-enrichment`, `posture/tool-question` |
| **Invocation** | `"Audit [project/scope]"` / `"Audit [domain]"` |

Multi-domain audit coordinator. Drives audits in 5 structured phases: prerequisites check
(scope, stack, file access) → project context loading (reads `ONBOARDING.md` first, or
quick reconnaissance) → domain selection with stack compatibility check → delegation to
the `auditor-subagent` agent (invoked as many times as needed, one domain per invocation) → consolidation executive summary (global score, top 5 priority
actions, cross-cutting recommendations).

Produces a multi-domain executive summary. Read-only — never modifies files.

---

## Family — Audit Agents

Single subagent of the auditor (ADR-017). Read-only. Invocable via the auditor or directly.

| Agent | File | Domain | References |
|-------|------|--------|-----------|
| `auditor-subagent` | `agents/auditor/auditor-subagent.md` | Security, Performance, Accessibility, Ecodesign, Architecture, Privacy, Observability — domain specified at invocation | OWASP Top 10, Core Web Vitals, WCAG 2.1 AA / RGAA 4.1, RGESN / GreenIT, SOLID / Clean Architecture, GDPR / EDPB / CNIL, RED method / SLOs / OpenTelemetry |

The `auditor-subagent` receives the domain + `native_skill` to load in the invocation prompt from the `auditor` coordinator.
It injects `auditor/audit-protocol-light` (common lightweight report format)
+ its domain-specific skill (`auditor/audit-<domain>`) loaded on-demand
+ `auditor/audit-handoff-format` (structured return contract when invoked from the orchestrator).

All reports produced include a **`### Findings to document`** section
at the end — findings to capitalize in `ONBOARDING.md` / `CONVENTIONS.md`.
This section is consolidated by the `auditor` coordinator in Phase 4 (skill `living-docs-enrichment`).
The agent never makes `task` calls — its read-only constraint is strict.

---

## Family — Developer Agents

1 generic agent specialized by domain at invocation time.
Follows the same Beads workflow (`bd claim → implement → test → bd close`).

The **domain** and the **native_skills to load** are passed by `orchestrator-dev` in the invocation prompt.
Each `task` instance runs in its own isolated session — parallel invocations with different domains are fully independent.

Common skills for all domains: `dev-standards-universal`, `dev-standards-simplicity`, `dev-standards-security`, `dev-standards-git`, `dev-standards-testing`, `beads-plan`, `beads-dev`, `developer/developer-handoff-format`, `shared/living-docs-enrichment`.

| Agent | File | Domain | Specific Native Skills |
|-------|------|--------|----------------------|
| `developer` | `agents/developer/developer.md` | frontend, backend, fullstack, api, mobile, data, devops, platform, security — domain passed at invocation | Domain skills injected via invocation prompt (see `orchestrator-dev-protocol`) |

**Separate agents (distinct workflow):**

| Agent | File | Domain |
|-------|------|--------|
| `developer-refactor` | `agents/developer/developer-refactor.md` | Structural refactoring only — never changes observable behavior |
| `developer-migrator` | `agents/developer/developer-migrator.md` | Incremental migrations — framework upgrades, major versions, EOL dependencies |

> See [ADR-013](./adr/013-developer-agent-consolidation.en.md) for the consolidation decision.
> See [ADR-002](./adr/002-developer-segmentation.en.md) (superseded) for the previous segmentation rationale.

**Domain → native_skills mapping (summary):**

| Domain | Native skills |
|--------|--------------|
| `frontend` | `dev-standards-frontend`, `dev-standards-frontend-a11y`, `dev-standards-testing` + detected stacks |
| `backend` | `dev-standards-backend`, `dev-standards-api`, `dev-standards-testing` + detected stacks |
| `fullstack` | `dev-standards-frontend`, `dev-standards-frontend-a11y`, `dev-standards-backend`, `dev-standards-api`, `dev-standards-testing` + detected stacks |
| `api` | `dev-standards-backend`, `dev-standards-api`, `dev-standards-testing` |
| `mobile` | `dev-standards-testing` + detected mobile stacks |
| `data` | `dev-standards-testing` + detected data stacks |
| `devops` | `dev-standards-devops` + detected infra stacks |
| `platform` | `dev-standards-devops` + detected platform stacks |
| `security` | `dev-standards-security-hardening`, `dev-standards-backend`, `dev-standards-testing` |

**Post-ticket — Living docs enrichment:** after each `bd close`, identifies patterns, conventions, or technical constraints discovered during implementation that are absent from `CONVENTIONS.md` or `ONBOARDING.md`, and proposes to the user to capitalize them (skill `living-docs-enrichment`).

---

## Family — Design Agents

UX/UI design agents. Work upstream of implementation.
Never code. Invocable directly or via the `orchestrator`.

### `ux-designer`

| | |
|--|--|
| **Label** | UXDesigner |
| **File** | `agents/design/ux-designer.md` |
| **Skills** | `designer/ux-protocol`, `developer/beads-plan`, `developer/beads-dev`, `posture/expert-posture`, `posture/tool-question`, `design/design-handoff-format` |
| **Invocation** | `"Analyze the flow for [feature]"` / `"UX spec for [ticket]"` / `"UX audit of [screen]"` |

User experience expert. Analyzes needs, identifies friction, produces textual user flows
and actionable UX specifications with acceptance criteria. Asks at least 2 context
questions before specifying. Reads and closes Beads tickets. Does not produce graphic mockups.

Invocable directly, via the `orchestrator`, or via the `planner` (PHASE 1.5 —
optional design delegation). When invoked from the `planner`, produces the spec
in the standardized format `## SPEC UX — [feature]` to allow automatic reintegration
into the plan (no `bd close` — the planner resumes control).

---

### `ui-designer`

| | |
|--|--|
| **Label** | UIDesigner |
| **File** | `agents/design/ui-designer.md` |
| **Skills** | `designer/ui-protocol`, `developer/beads-plan`, `developer/beads-dev`, `posture/expert-posture`, `posture/tool-question`, `design/design-handoff-format` |
| **Invocation** | `"UI spec for [component]"` / `"Design system [project]"` / `"Harmonize [screen]"` |

Interface design expert. Defines design system foundations (tokens),
specifies visual components with variants and states, produces actionable UI guidelines
for `developer-frontend`. Uses only tokens — never hard-coded values. Always proposes
options for art direction decisions.

Invocable directly, via the `orchestrator`, or via the `planner` (PHASE 1.5 —
optional design delegation). When invoked from the `planner`, produces the spec
in the standardized format `## SPEC UI — [ComponentName]` to allow automatic reintegration
into the plan (no `bd close` — the planner resumes control).

---

## Family — Quality Agents

Agents dedicated to code quality, invocable standalone or via the orchestrator.

### `reviewer`

| | |
|--|--|
| **Label** | CodeReviewer |
| **File** | `agents/quality/reviewer.md` |
| **Skills** | `dev-standards-universal`, `dev-standards-security`, `dev-standards-backend`, `dev-standards-frontend`, `dev-standards-frontend-a11y`, `dev-standards-testing`, `dev-standards-git`, `reviewer/review-protocol`, `posture/tool-question`, `reviewer/reviewer-handoff-format`, `shared/living-docs-enrichment` |
| **Invocation** | Branch name / PR URL + optionally `bd show <ID>` (the reviewer fetches the diff itself via `git diff`) |

Analyzes PR/MR diffs. Produces a structured report by severity (Critical /
Major / Minor / Suggestion / Positive points). Read-only — never modifies files.

**Post-report — Living docs enrichment:** after producing the review report, identifies conventions and patterns observed in the diff that are absent from `CONVENTIONS.md` or `ONBOARDING.md`, and proposes to capitalize them. If accepted, delegates writing to the `documentarian` via `task` (skill `living-docs-enrichment`).

---

### `qa-engineer`

| | |
|--|--|
| **Label** | QAEngineer |
| **File** | `agents/quality/qa-engineer.md` |
| **Skills** | `dev-standards-universal`, `dev-standards-git`, `posture/expert-posture`, `posture/tool-question`, `qa/qa-protocol`, `qa/qa-handoff-format`, `shared/living-docs-enrichment` |
| **Invocation** | `"Write tests for branch [X]"` / `"QA on ticket [ID]"` |

Writes missing tests (unit / integration / E2E) from a diff or a
Beads ticket. Produces a before/after coverage report. Never modifies functional code.

**Not relevant for TDD tickets**: when a ticket carries the `tdd` label,
tests are written by the developer themselves before implementation (red/green/refactor loop).
`orchestrator-dev` automatically skips CP-QA for these tickets — `qa-engineer` is not invoked.

**Post-report — Living docs enrichment:** after producing the coverage report, identifies test conventions adopted and systematic edge cases revealed by the tests that are absent from `CONVENTIONS.md`, and proposes to capitalize them. If accepted, delegates writing to the `documentarian` via `task` (skill `living-docs-enrichment`).

> See [ADR-004](./adr/004-qa-debugger-separation.en.md).

---

### `debugger`

| | |
|--|--|
| **Label** | Debugger |
| **File** | `agents/quality/debugger.md` |
| **Skills** | `quality/debugger-workflow`, `quality/debugger-handoff-format`, `shared/living-docs-enrichment`, `posture/expert-posture` |
| **Invocation** | `"This bug: [stacktrace]"` / `"Analyze these logs: [logs]"` |

Diagnoses the root cause of a bug in 6 structured phases: artefact verification
(Phase 0 — pauses if insufficient) → contextual exploration → complementary questions
(optional) → 4-step diagnosis (reproduction/isolation/identification/graded hypothesis
high/medium/low) → edge case detection (race conditions, environment-specific, data,
configuration, dependencies, regression). Produces a diagnostic report with graded
hypotheses. Creates a Beads correction ticket after explicit confirmation.
Never fixes the bug.

**Phase 5 — Living docs enrichment:** after the report, identifies blind spots uncovered by the diagnosis and error patterns worth remembering, then proposes to the user to enrich `ONBOARDING.md` and/or `CONVENTIONS.md`. If accepted, delegates writing to the `documentarian` via `task` (skill `living-docs-enrichment`). Cannot invoke the `documentarian` without explicit user confirmation.

> See [ADR-004](./adr/004-qa-debugger-separation.en.md).

---

## Family — Planning Agents

### `planner`

| | |
|--|--|
| **Label** | ProjectPlanner |
| **File** | `agents/planning/planner.md` |
| **Skills** | `developer/beads-plan`, `planning/planner-workflow`, `posture/expert-posture`, `posture/tool-question`, `planning/planner-handoff-format`, `shared/living-docs-enrichment` |
| **Invocation** | Natural language feature description |

Functional and technical consultant who analyzes the project context before planning.
Workflow in 7 phases: prerequisites check → contextual exploration (codebase, tickets,
UX/UI signals) → optional design delegation (Phase 1.5) → complementary questions →
hierarchical plan (epics → tickets, deduced and justified priorities) → edge case
detection (duplicates, oversized tickets, circular dependencies) → Beads creation with
full enrichment → optional ai-delegated delegation (Phase 5.5) → final verification.

Creates epics in Beads if > 5 tickets (asks otherwise), uses `--parent` and `--deps`
for hierarchy and dependencies. Handles contingencies: scope change, ticket splitting,
late dependency, duplicate. Never codes. Iterative phases with backwards possible
(max 3 iterations per phase).

**Phase 1.5 — Design delegation (optional):** when UX or UI signals are detected
in Phase 1, the planner offers 3 options to the user:
- **Option A** (`"invoke UX/UI"`) — directly invokes `ux-designer` / `ui-designer`
  as a sub-agent, awaits the structured block `## SPEC UX/UI — …` and integrates the spec into the plan.
- **Option B** — the user invokes the agents themselves and pastes the spec back.
- **Option C** (`"continue without UX/UI"`) — proceeds with available context,
  partial `--design` fields + `bd comments add` to trace the missing spec.

**Phase 6 — Living docs enrichment:** after plan validation, identifies architectural patterns and conventions observed in the codebase but absent from `ONBOARDING.md`/`CONVENTIONS.md`, and proposes to the user to capitalize them. If accepted, delegates writing to the `documentarian` via `task` (skill `living-docs-enrichment`).

---

## Family — Documentation Agents

### `documentarian`

| | |
|--|--|
| **Label** | Documentarian |
| **File** | `agents/documentation/documentarian.md` |
| **Skills** | `developer/dev-standards-git`, `developer/beads-plan`, `developer/beads-dev`, `documentarian/doc-protocol`, `documentarian/doc-standards`, `documentarian/doc-adr`, `documentarian/doc-api`, `documentarian/doc-changelog`, `documentarian/doc-slides`, `posture/expert-posture`, `posture/tool-question` |
| **Invocation** | `"Document [topic]"` / `"Create an ADR for [decision]"` / `"Update the CHANGELOG"` / `"What's missing in the docs?"` / `"Create a presentation for [topic]"` |

Writes and updates technical, functional, architectural documentation, API docs,
changelogs, and Marp presentations. Systematically explores existing structure before writing.
Adapts to the format in place — recommends improvements without imposing them.
Never changes a format without explicit confirmation.

Guiding principle: **explore → adapt or propose → wait if needed → write**.

---

## Rules Common to All Agents

- **Read-only agents**: auditor-subagent, reviewer, debugger, ux-designer, ui-designer — never modify files
- **Agents that write code**: developer-*, qa-engineer — only modify files in their domain
- **Agents that write documentation**: documentarian — only modifies documentation files (all other agents may propose enrichments to `ONBOARDING.md`/`CONVENTIONS.md` via the `living-docs-enrichment` skill, always delegated to `documentarian` after explicit user confirmation)
- **Agents that create tickets**: planner (feature tickets), debugger (bug tickets after confirmation)
- **Agents that read tickets**: all can do `bd show <ID>` to contextualize their work
- **Coordinator agents**: orchestrator, orchestrator-dev, auditor — never code, drive other agents
- **Discovery agents**: onboarder — read-only, explores and reports, doesn't drive other agents
- **`primary` agents**: orchestrator, orchestrator-dev, planner, auditor, ui-designer, ux-designer, documentarian, onboarder, debugger, qa-engineer, reviewer — directly visible to the user
- **`subagent` agents**: `developer`, `developer-refactor`, `developer-migrator` and `auditor-subagent` — invocable by coordinator agents
