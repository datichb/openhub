# Agent Reference

27 agents in total, organized into 6 families.
Each agent is defined in `agents/<family>/<id>.md` with a frontmatter declaring its metadata,
targets, and skills.

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
targets: [opencode]
skills: [path/to/skill, ...]
---

# <Title>

<Agent body>
```

| Field | Role |
|-------|------|
| `id` | Unique identifier, used by adapters and `oc agent` |
| `label` | Name displayed in the target tool |
| `description` | Short phrase describing the role — appears in agent lists |
| `mode` | `primary` (default) or `subagent` — controls visibility in target tools |
| `permission.question` | `allow` — enables OpenCode's `question` tool for this agent. Reserved for interactive `primary` agents. Always paired with the `posture/tool-question` skill. |
| `targets` | Supported targets: `opencode` |
| `skills` | Paths relative to `skills/` — injected in declaration order |

### Primary / Subagent Modes

The `mode:` field controls how an agent is exposed in each target tool:

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
| **Skills** | `planning/onboarder-workflow`, `planning/onboarder-handoff-format`, `posture/expert-posture`, `posture/tool-question`, `developer/beads-plan`, `developer/dev-standards-git` |
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
- **Mode C** — unknown project → reads `ONBOARDING.md` / `CONVENTIONS.md` first; proposes `onboarder` only if both files are absent
- **Mode A** — feature in natural language → delegates to `planner`
- **Mode B** — existing Beads tickets → direct start

Never routes directly to `developer-*` — always delegates to `orchestrator-dev`.

**Technical permissions:** `bash`, `edit`, `write` disabled. Acts only via `task` (delegation) and `question` (checkpoints). List of invocable agents explicitly restricted in the frontmatter.

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
Beads tickets, routes to the 9 `developer-*` agents, supervises optional QA and review.
Three modes: `manual` (default), `semi-auto`, `auto`. Invocable standalone or from the `orchestrator`.

CP-2 (commit or fix?) is always manual in all modes.

> See [ADR-006](./adr/006-orchestrator-configurable-mode.en.md) — modes apply to `orchestrator-dev` only.

---

### `auditor`

| | |
|--|--|
| **Label** | Auditor |
| **File** | `agents/auditor/auditor.md` |
| **Skills** | `auditor/auditor-workflow`, `auditor/audit-protocol-light`, `auditor/audit-handoff-format`, `auditor/living-docs-enrichment`, `posture/tool-question` |
| **Invocation** | `"Audit [project/scope]"` / `"Audit [domain]"` |

Multi-domain audit coordinator. Drives audits in 5 structured phases: prerequisites check
(scope, stack, file access) → project context loading (reads `ONBOARDING.md` first, or
quick reconnaissance) → domain selection with stack compatibility check → delegation to
7 specialized subagents → consolidation executive summary (global score, top 5 priority
actions, cross-cutting recommendations).

Produces a multi-domain executive summary. Read-only — never modifies files.

---

## Family — Audit Agents

Auditor's subagents. All read-only. Invocable directly or via the auditor.

| Agent | File | Domain | References |
|-------|------|--------|-----------|
| `auditor-security` | `agents/auditor/auditor-security.md` | Application security | OWASP Top 10, CVE, RGS |
| `auditor-performance` | `agents/auditor/auditor-performance.md` | Web performance | Core Web Vitals, N+1, cache |
| `auditor-accessibility` | `agents/auditor/auditor-accessibility.md` | Accessibility | WCAG 2.1 AA, RGAA 4.1 |
| `auditor-ecodesign` | `agents/auditor/auditor-ecodesign.md` | Eco-design | RGESN, GreenIT, Écoindex |
| `auditor-architecture` | `agents/auditor/auditor-architecture.md` | Architecture & debt | SOLID, Clean Architecture |
| `auditor-privacy` | `agents/auditor/auditor-privacy.md` | Data protection | GDPR, EDPB, CNIL |
| `auditor-observability` | `agents/auditor/auditor-observability.md` | Observability | RED method, SLOs, OpenTelemetry, alerting |

All audit agents inject `auditor/audit-protocol-light` (common lightweight report format)
+ their domain-specific skill (`auditor/audit-<domain>`)
+ `auditor/audit-handoff-format` (structured return contract when invoked from the orchestrator).

---

## Family — Developer Agents

9 agents specialized by technical domain. All follow the same Beads workflow
(`bd claim → implement → test → bd close`).

Common skills for all: `dev-standards-universal`, `dev-standards-security`, `dev-standards-git`, `beads-plan`, `beads-dev`, `developer/developer-handoff-format`.

| Agent | File | Domain | Specific Skills |
|-------|------|--------|----------------|
| `developer-frontend` | `agents/developer/developer-frontend.md` | UI, components, Vue.js, CSS, a11y | `dev-standards-frontend`, `dev-standards-frontend-a11y`, `dev-standards-vuejs`, `dev-standards-testing` |
| `developer-backend` | `agents/developer/developer-backend.md` | Services, repositories, migrations | `dev-standards-backend`, `dev-standards-testing` |
| `developer-fullstack` | `agents/developer/developer-fullstack.md` | Full-stack features | `dev-standards-frontend`, `dev-standards-backend`, `dev-standards-testing` |
| `developer-data` | `agents/developer/developer-data.md` | Pipelines, ETL, ML, dbt | `stacks/* (dynamic injection)` |
| `developer-devops` | `agents/developer/developer-devops.md` | Docker, CI/CD, shell scripts | `dev-standards-devops` |
| `developer-mobile` | `agents/developer/developer-mobile.md` | React Native, Flutter, iOS, Android | `stacks/* (dynamic injection)` |
| `developer-api` | `agents/developer/developer-api.md` | REST, GraphQL, webhooks | `dev-standards-backend`, `dev-standards-api`, `dev-standards-testing` |
| `developer-platform` | `agents/developer/developer-platform.md` | Terraform, K8s, Helm, GitOps, infra as code | `stacks/* (dynamic injection)` |
| `developer-security` | `agents/developer/developer-security.md` | Application hardening post-audit | `dev-standards-security-hardening`, `dev-standards-backend`, `dev-standards-testing` |

> See [ADR-002](./adr/002-developer-segmentation.en.md) for the segmentation decision.

`developer-platform` differs from `developer-devops`: DevOps covers Dockerfile,
docker-compose, GitHub Actions and application shell scripts; Platform covers
Terraform/Pulumi, Kubernetes manifests, Helm charts, ArgoCD/Flux.

`developer-security` differs from `developer-backend`: it intervenes
exclusively after an `auditor-security` audit to fix identified vulnerabilities
(HTTP headers, CORS, hashing, JWT, sessions, rate limiting, encryption). It does not
perform audits.

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
| **Skills** | `dev-standards-universal`, `dev-standards-security`, `dev-standards-backend`, `dev-standards-frontend`, `dev-standards-frontend-a11y`, `dev-standards-testing`, `dev-standards-git`, `reviewer/review-protocol`, `posture/tool-question`, `reviewer/reviewer-handoff-format` |
| **Invocation** | Pasted diff / branch name / PR URL + optionally `bd show <ID>` |

Analyzes PR/MR diffs. Produces a structured report by severity (Critical /
Major / Minor / Suggestion / Positive points). Read-only — never modifies files.

---

### `qa-engineer`

| | |
|--|--|
| **Label** | QAEngineer |
| **File** | `agents/quality/qa-engineer.md` |
| **Skills** | `dev-standards-universal`, `dev-standards-testing`, `dev-standards-git`, `posture/expert-posture`, `posture/tool-question`, `qa/qa-protocol`, `qa/qa-handoff-format` |
| **Invocation** | `"Write tests for branch [X]"` / `"QA on ticket [ID]"` |

Writes missing tests (unit / integration / E2E) from a diff or a
Beads ticket. Produces a before/after coverage report. Never modifies functional code.

**Not relevant for TDD tickets**: when a ticket carries the `tdd` label,
tests are written by the developer themselves before implementation (red/green/refactor loop).
`orchestrator-dev` automatically skips CP-QA for these tickets — `qa-engineer` is not invoked.

> See [ADR-004](./adr/004-qa-debugger-separation.en.md).

---

### `debugger`

| | |
|--|--|
| **Label** | Debugger |
| **File** | `agents/quality/debugger.md` |
| **Skills** | `quality/debugger-workflow`, `quality/debugger-handoff-format`, `auditor/living-docs-enrichment`, `posture/expert-posture` |
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
| **Skills** | `developer/beads-plan`, `planning/planner-workflow`, `posture/expert-posture`, `posture/tool-question`, `planning/planner-handoff-format`, `auditor/living-docs-enrichment` |
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

- **Read-only agents**: auditor-*, reviewer, debugger, ux-designer, ui-designer — never modify files
- **Agents that write code**: developer-*, qa-engineer — only modify files in their domain
- **Agents that write documentation**: documentarian — only modifies documentation files
- **Agents that create tickets**: planner (feature tickets), debugger (bug tickets after confirmation)
- **Agents that read tickets**: all can do `bd show <ID>` to contextualize their work
- **Coordinator agents**: orchestrator, orchestrator-dev, auditor — never code, drive other agents
- **Discovery agents**: onboarder — read-only, explores and reports, doesn't drive other agents
- **`primary` agents**: orchestrator, orchestrator-dev, planner, auditor, ui-designer, ux-designer, documentarian, onboarder, debugger, qa-engineer, reviewer — directly visible to the user
- **`subagent` agents**: all `developer-*` and `auditor-*` (except `auditor` itself) — invocable by coordinator agents
