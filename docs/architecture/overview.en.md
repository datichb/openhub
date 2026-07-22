# Architecture Overview

## Core Concepts

### Hub

The **hub** (`openhub`) is the central repository containing the canonical sources
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

An **MCP Server** (Model Context Protocol) is a **Go native** implementation in
`cli/internal/mcp/` that provides tool integrations to agents. MCP Servers run
via `oh mcp serve <name>` (stdio JSON-RPC protocol).

Current MCP Servers:
- **figma**: Figma API integration (search files, detect UI signals, get structure)
- **gitlab**: GitLab API integration (issues, merge requests, labels, milestones)
- **gslides**: Google Slides API integration
- **github**: GitHub API integration (issues, pull requests, labels, milestones)
- **jira**: Jira API integration (issues, sprints, projects)
- **linear**: Linear API integration (issues, cycles, teams)

MCP Servers are deployed into projects as `mcpServers` entries in `opencode.json`.

See [Figma Integration Guide](../guides/figma-integration.en.md) for figma usage.
See [GitLab Integration Guide](../guides/gitlab-integration.en.md) for gitlab usage.

### Plugin & MCP Registry

The hub supports a **dynamic registry** for community-contributed plugins and MCP servers:

- **Plugins**: installed to `~/.oh/plugins/<name>/` with a `manifest.json` describing the plugin's commands and hooks. Discovered and loaded automatically at startup.
- **MCP servers**: installed to `~/.oh/mcp/<name>/` with a `manifest.json` and a binary. Registered automatically as available `mcpServers` in project configs.

This enables third-party extensions without modifying the hub itself.

### Skill Marketplace

Community skills can be installed from the [oh-skills-index](https://github.com/datichb/oh-skills-index) or from any Git URL:

```bash
oh skill add <index-name>          # install from community index
oh skill add https://github.com/...  # install from Git URL
oh skill list                      # list installed community skills
oh skill remove <name>             # remove a community skill
oh skill search <query>            # search the community index
```

Community skills are stored in `~/.oh/skills/<name>/` with a `manifest.json` describing their metadata. They are available for deployment into any project alongside hub-native skills.

### Observability & Telemetry

Every agent session is recorded in the `agent_events` SQLite table:

| Column | Content |
|--------|---------|
| `agent` | Agent name |
| `skills_loaded` | Skills loaded during the session |
| `duration_s` | Session duration in seconds |
| `tokens` | Total tokens consumed |
| `cost_usd` | Estimated cost |

```bash
oh metrics             # per-agent stats (sessions, tokens, cost, avg duration)
oh serve               # expose API + SPA dashboard on localhost
```

### Deployment

Deployment is handled by the `cli/internal/deploy/` package (Go). It performs
**transactional deployment**: agents, skills, config, and MCP servers are injected
into the target project's `opencode.json`.

Commands: `oh deploy`, `oh sync`.

### Target Project

A **target project** is an application repository onto which agents are deployed
via `oh deploy`.

---

## Diagram — Deployment Flow

```mermaid
flowchart LR
    subgraph HUB["openhub (source of truth)"]
        A[agents/*.md] --> DEP[cli/internal/deploy]
        S[skills/**/*.md] --> DEP
        MCP[cli/internal/mcp] --> DEP
        PLG[~/.oh/plugins/] --> DEP
        MCPREG[~/.oh/mcp/] --> DEP
        SKM[~/.oh/skills/] --> DEP
    end

    subgraph PROJECTS["Target Projects"]
        DEP -->|"Bucket A (inline)"| P1[".opencode/agents/*.md"]
        DEP -->|"Bucket B (native)"| P2[".opencode/skills/**/SKILL.md"]
        DEP -->|"mcpServers"| P3["opencode.json"]
    end

    subgraph TELEMETRY["Telemetry"]
        DEP --> EVT[agent_events table]
        EVT --> DASH[oh serve dashboard]
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
    participant DS as designer
    participant AU as auditor (coordinator)
    participant OD as OrchestratorDev
    participant DEV as Developer-*
    participant R as Reviewer

    U->>O: "Implement [feature]"
    O->>PL: Delegates planning
    PL-->>O: Tickets created (spec, audit, dev)
    O->>U: [CP-0] Plan + workflow mode?

    opt Spec-ux / spec-ui tickets
        O->>DS: Delegates design (mode: ux/ui/ux+ui)
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
        OD->>DEV: Delegates implementation (tests included)
        DEV-->>OD: Implementation + tests complete
        OD->>R: Automatic review
        R-->>OD: Review report
        OD->>U: [CP-2] Merge or fix? ← ALWAYS PAUSED
        OD->>U: [CP-3] Next ticket or stop?
    end
    OD-->>O: Condensed recap (per-ticket summary: status, key files, attention points) + structured block

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

Implementing and diagnosing are two distinct responsibilities entrusted
to different agents (developer, debugger). Tests are written by the developer and verified by the reviewer.

→ [ADR-004](./adr/004-qa-debugger-separation.en.md)

### 5. Read-only for non-developer agents

Agents `auditor-*`, `reviewer`, and `designer` never write to the target project.
Only `developer-*` agents modify source code files.
The `reviewer` supports multi-mode review (standard, adversarial, edge-case) with
parallel independent sessions for combined modes — context isolation guarantees unbiased analysis.

Documentary writing (wiki `docs/wiki/` and minimal `ONBOARDING.md`) is reserved for
the `onboarder` (initial generation) and `documentarian` (enrichments). All agents that
produce analysis or implementation work may enrich the wiki only by delegating to the
`documentarian` after explicit user confirmation (skill `shared/living-docs-enrichment`).
They never write directly.

This continuous enrichment loop covers all agents: `auditor` coordinator (Phase 4),
`planner` (Phase 6), `debugger` (Phase 5), `developer-*` (after each ticket), `reviewer`
(post-report), `pathfinder` (post-report), and `onboarder`
(incremental mode when `docs/wiki/index.md` already exists).

See [Living Documentation Wiki](./living-wiki.en.md) for the complete system architecture.

---

## File Structure

```
openhub/
├── agents/              ← AI role definitions (22 agents)
├── skills/              ← Protocols: Bucket A (inline) + Bucket B (on-demand)
├── cli/                 ← Go CLI binary (oh)
│   ├── cmd/             ← Cobra commands
│   └── internal/
│       ├── app/         ← Application context
│       ├── beads/       ← Beads ticket integration
│       ├── config/      ← hub.toml configuration
│       ├── deploy/      ← Transactional deployment engine
│       ├── domain/      ← Domain types (Project, Session, Secret)
│       ├── i18n/        ← Internationalization (fr/en)
│       ├── mcp/         ← Native MCP servers (figma, gitlab, gslides, github, jira, linear)
│       ├── opencode/    ← Binary management, compatibility, project config
│       ├── plugin/      ← Plugin system (RTK embedded + dynamic registry)
│       ├── prompt/      ← Stack detection, prompt builders
│       ├── storage/     ← SQLite + keychain + filecrypt + agent_events telemetry
│       ├── tui/         ← BubbleTea views (dashboard, board, picker)
│       └── worktree/    ← Git worktree management
├── docs/                ← Documentation (bilingual fr/en)
└── ~/.oh/               ← User data (runtime, not in repo)
    ├── hub/             ← Extracted hub content
    ├── plugins/         ← Community plugins (<name>/manifest.json)
    ├── mcp/             ← Community MCP servers (<name>/manifest.json + binary)
    └── skills/          ← Community skills (<name>/manifest.json + SKILL.md)
```

**Supported platforms:** macOS (darwin), Linux, Windows — amd64 and arm64.
