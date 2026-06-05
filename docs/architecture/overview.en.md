# Architecture Overview

## Core Concepts

### Hub

The **hub** (`opencode-hub`) is the central repository containing the canonical sources
of all agents and skills. It is the single source of truth — always edit here,
never in target projects.

### Agent

An **agent** is a Markdown file (`.md`) that defines the identity of an AI role:
who it is, what it does, what it doesn't do, and its condensed workflow.
Agents are short (~40-80 lines) and don't contain detailed protocols.

See [agents.en.md](./agents.en.md) for the complete reference.

### Skill

A **skill** is a protocol block: report format, checklist, behavior rules, examples.
The hub uses a **hybrid architecture** with two deployment paths:

| Path | Frontmatter field | When loaded |
|------|------------------|-------------|
| **Bucket A — Inline** | `skills: [...]` | Always — assembled into the system prompt at deploy time |
| **Bucket B — Native** | `native_skills: [...]` | On-demand — the LLM loads from `.opencode/skills/` via the `skill` tool |

A skill can be shared across multiple agents (e.g. `dev-standards-universal`
is Bucket A in all developer agents and the reviewer).

See [skills.en.md](./skills.en.md) for the complete reference.
See [ADR-001](./adr/001-agent-skill-separation.en.md) for the separation decision.
See [ADR-010](./adr/010-hybrid-skills-architecture.en.md) for the inline vs native split.

### MCP Server

An **MCP Server** (Model Context Protocol) is a TypeScript server that provides
tool integrations to agents. MCP Servers are deployed with agents and expose
tools via stdin/stdout protocol.

Current MCP Servers:
- **figma-mcp**: Figma API integration (search files, detect UI signals, get structure)
- **gitlab-mcp**: GitLab API integration (issues, merge requests, labels, milestones)

MCP Servers are located in `servers/` and deployed to `.opencode/servers/` in target projects.

See [servers/README.md](../../servers/README.md) for infrastructure documentation.
See [Figma Integration Guide](../guides/figma-integration.en.md) for figma-mcp usage.
See [GitLab Integration Guide](../guides/gitlab-integration.en.md) for gitlab-mcp usage.

### Adapter

An **adapter** is a shell script (`scripts/adapters/<target>.adapter.sh`) that
translates agents + skills from the hub format to the format expected by a target tool.
One adapter exists: `opencode`.

### Target Project

A **target project** is an application repository onto which agents are deployed
via `oc deploy`. The hub knows projects via `projects/projects.md`.

---

## Diagram — Deployment Flow

```mermaid
flowchart LR
    subgraph HUB["opencode-hub (source of truth)"]
        A[agents/*.md] --> PB[prompt-builder.sh]
        S[skills/**/*.md] --> PB
        PB --> ADP
        subgraph ADP["adapters/"]
            OC[opencode.adapter.sh]
        end
    end

    subgraph PROJECTS["Target Projects"]
        OC -->|"Bucket A (inline)"| P1[".opencode/agents/*.md"]
        OC -->|"Bucket B (native)"| P2[".opencode/skills/**/SKILL.md"]
    end
```

---

## Diagram — Orchestrator Workflow

The orchestrator operates at two levels: `orchestrator` (feature project manager)
delegates design, audits, then implementation to `orchestrator-dev`
(implementation tech lead) which drives the `developer-*` agents.

```mermaid
sequenceDiagram
    participant U as User
    participant O as Orchestrator
    participant PL as Planner
    participant DS as ux/ui-designer
    participant AU as auditor (coordinator)
    participant OD as OrchestratorDev
    participant DEV as Developer-*
    participant QA as QA Engineer
    participant R as Reviewer

    U->>O: "Implement [feature]"
    O->>PL: Delegates planning
    PL-->>O: Tickets created (spec, audit, dev)
    O->>U: [CP-0] Plan + workflow mode?

    opt Spec-ux / spec-ui tickets
        O->>DS: Delegates design
        DS-->>O: Spec produced
        O->>U: [CP-spec] Validate spec?
    end

    opt Tickets label:audit-*
        O->>AU: Delegates audit
        AU-->>O: Audit report
        Note over AU: delegates to specialized auditor-* subagents
        O->>U: [CP-audit] Fix / accept / ignore?
    end

    O->>OD: Dev tickets (+ mode passed)
    loop For each dev ticket
        OD->>DEV: Delegates implementation
        DEV-->>OD: Implementation complete
        opt QA enabled (ticket without tdd label)
            OD->>QA: Delegates verification
            QA-->>OD: Tests written + coverage report
        end
        OD->>R: Automatic review
        R-->>OD: Review report
        OD->>U: [CP-2] Merge or fix? ← ALWAYS PAUSED
        OD->>U: [CP-3] Next ticket or stop?
    end
    OD-->>O: Narrative recap (verbatim reports + attention points) + structured block

    O->>U: [CP-feature] Global feature summary
```

---

## Diagram — Debug Workflow

```mermaid
sequenceDiagram
    participant U as User
    participant D as Debugger
    participant B as Beads

    U->>D: Stacktrace / logs / description
    D->>D: Reproduction → Isolation → Identification → Hypothesis
    D-->>U: Diagnostic report + suggested ticket
    U->>D: [CP] Create ticket?
    D->>B: bd create + bd update
    B-->>D: ID created
    D-->>U: Ticket #XX created
```

---

## Design Principles

### 1. Identity / Protocol Separation

The agent defines **who** it is, the skill defines **how** it works.
This separation enables protocol reuse across agents and keeps
agent files readable.

Skills are further split into two buckets: Bucket A (always-on, inline)
for mandatory protocols and workflow contracts, and Bucket B (native, on-demand)
for domain-specific context loaded only when the task requires it.

→ [ADR-001](./adr/001-agent-skill-separation.en.md)
→ [ADR-010](./adr/010-hybrid-skills-architecture.en.md)

### 2. Specialization over Generalism

Developer agents are segmented into 9 specializations so each agent
receives only context relevant to its domain.

→ [ADR-002](./adr/002-developer-segmentation.en.md)

### 3. Explicit Checkpoints

The orchestrator never advances the workflow automatically. Each critical
step requires explicit user confirmation.

→ [ADR-003](./adr/003-orchestrator-checkpoints.en.md)

### 4. Separation of Quality Responsibilities

Implementing, testing, and diagnosing are three distinct responsibilities entrusted
to three different agents (developer, qa-engineer, debugger).

→ [ADR-004](./adr/004-qa-debugger-separation.en.md)

### 5. Read-only for non-developer agents

Agents `auditor-*`, `reviewer`, `ux-designer`, and `ui-designer` never write to the target project.
Only `developer-*` and `qa-engineer` agents modify source code files.

Documentary writing (`ONBOARDING.md`, `CONVENTIONS.md`) is reserved for the `documentarian`.
Analysis agents (`auditor` coordinator, `planner`, `debugger`) may enrich these files
only by delegating to the `documentarian` after explicit user confirmation
(skill `living-docs-enrichment`). They never write directly.

---

## File Structure

```
opencode-hub/
├── agents/          ← Canonical agent sources (edit here)
├── skills/          ← Injectable protocols and standards
├── scripts/
│   ├── adapters/    ← Translation hub → target tool format
│   ├── lib/         ← Shared helpers (prompt-builder, adapter-manager)
│   └── cmd-*.sh     ← Implementation of oc commands
├── config/
│   ├── hub.json             ← Global hub configuration
│   ├── stack-skills.json    ← Stack → dynamically injected skills mapping
│   └── providers/           ← LLM provider configuration
├── projects/
│   ├── projects.md       ← Project registry (local, git-ignored)
│   └── projects.example.md ← Versioned template
└── docs/            ← Documentation (this folder)
    ├── architecture/
    ├── guides/
    ├── dev/         ← Bash gotchas and developer guides
    ├── presentations/ ← Presentations and slides
    └── reference/
```
