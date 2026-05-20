> 🇫🇷 [Lire en français](README.fr.md)

# opencode-hub

Central hub for managing AI assistants across multiple projects,
with shared agents, injectable skills, and an integrated Beads workflow.

Supports **OpenCode**.

---

## How it works

opencode-hub is built around three concepts: **agents**, **skills**, and **deployment**.

- **Agents** define AI roles (who does what, how, and in what order).
- **Skills** are injectable protocols (code standards, checklists, report formats) — declared once, reused across multiple agents.
- **Deployment** assembles agents + skills and copies them into your target projects.

```
opencode-hub/          ← single source of truth (edit here, never in projects)
├── agents/            ← AI role definitions (~40-80 lines per agent)
├── skills/            ← detailed injectable protocols
└── scripts/           ← assembly and deployment

         oc deploy opencode MY-APP
opencode-hub  ──────────────────────►  my-app/.opencode/agents/*.md
                                   └►  my-app/opencode.json

```

Result: 27 specialized agents, always up to date, available across all your projects
from a single source of truth.

---

## Installation

### One-liner (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/opencode-hub/main/install.sh | bash
```

The script automates everything: clones to `~/.opencode-hub`, checks dependencies
with confirmation prompts, creates the `oc` alias, and interactively configures AI targets.

After installation, reload your shell:

```bash
source ~/.zshrc   # or source ~/.bashrc
```

### Manual installation

```bash
git clone https://github.com/datichb/opencode-hub.git ~/.opencode-hub
echo 'alias oc="~/.opencode-hub/oc.sh"' >> ~/.zshrc && source ~/.zshrc
oc install
```

### Install a specific version

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/opencode-hub/main/install.sh | VERSION=v1.0.0 bash
```

---

## Upgrading

### Update hub sources

```bash
oc upgrade           # pull latest main
oc upgrade v1.1.0    # switch to a specific release tag
```

### Update installed tools (opencode, Beads, skills)

```bash
oc update
```

---

## Uninstallation

```bash
oc uninstall
# or directly:
bash ~/.opencode-hub/uninstall.sh
```

Interactive 4-step guide — everything is optional and requires confirmation:

| Step | Action | Default |
|------|--------|---------|
| 1 | Clean deployed agents from projects (`.opencode/agents/`, `opencode.json`) | `[y/N]` |
| 2 | Remove the hub (`~/.opencode-hub`) | `[y/N]` |
| 3 | Remove the `oc` alias and bun exports from the rc file | `[Y/n]` |
| 4 | Uninstall opencode, Beads, bun (separately) | `[y/N]` |

> `jq` and `node` are never uninstalled. A `.bak` backup is created before any
> modification to the rc file.

---

## Quick start

```bash
# 1. Register a project
oc init MY-APP ~/workspace/my-app

# 2. Deploy agents into the project
oc deploy opencode MY-APP

# 3. Launch the tool in the project
oc start MY-APP
```

> Full guide: [docs/guides/getting-started.en.md](docs/guides/getting-started.en.md)

---

## Available agents

27 agents in two modes:

- **`primary`** — directly visible in the AI tool (OpenCode tab picker). Invocable by the user.
- **`subagent`** — hidden from the picker. Only invocable by delegation from a coordinator agent.

### Primary agents (directly invocable)

| Agent | Family | Role |
|-------|--------|------|
| `orchestrator` | Coordinator | Feature end-to-end — delegates spec, audit, implementation |
| `orchestrator-dev` | Coordinator | Ticket implementation — drives `developer-*` agents |
| `auditor` | Coordinator | Multi-domain audit — delegates to 7 `auditor-*` agents |
| `onboarder` | Coordinator | Discovery of existing projects, context report |
| `planner` | Planning | Breaks down a feature into Beads tickets |
| `ux-designer` | Design | UX spec — user flows, acceptance criteria |
| `ui-designer` | Design | UI spec — tokens, components, visual guidelines |
| `reviewer` | Quality | PR/MR review by severity |
| `qa-engineer` | Quality | Missing tests (unit / integration / E2E) |
| `debugger` | Quality | Bug diagnosis, root cause report |
| `documentarian` | Documentation | README, CHANGELOG, ADR, API docs |

### Subagents (delegated by coordinators)

| Agent | Delegated by | Domain |
|-------|-------------|--------|
| `developer-frontend` | `orchestrator-dev` | UI, components, Vue.js, CSS, a11y |
| `developer-backend` | `orchestrator-dev` | Services, repositories, migrations |
| `developer-fullstack` | `orchestrator-dev` | Full-stack features |
| `developer-data` | `orchestrator-dev` | Pipelines, ETL, ML, dbt |
| `developer-devops` | `orchestrator-dev` | Docker, CI/CD, shell scripts |
| `developer-mobile` | `orchestrator-dev` | React Native, Flutter, iOS, Android |
| `developer-api` | `orchestrator-dev` | REST, GraphQL, webhooks |
| `developer-platform` | `orchestrator-dev` | Terraform, K8s, Helm, GitOps |
| `developer-security` | `orchestrator-dev` | Hardening after security audit |
| `auditor-security` | `auditor` | OWASP Top 10, CVE, RGS |
| `auditor-performance` | `auditor` | Core Web Vitals, N+1, cache |
| `auditor-accessibility` | `auditor` | WCAG 2.1 AA, RGAA 4.1 |
| `auditor-ecodesign` | `auditor` | RGESN, GreenIT, Écoindex |
| `auditor-architecture` | `auditor` | SOLID, Clean Architecture, technical debt |
| `auditor-privacy` | `auditor` | GDPR, EDPB, CNIL |
| `auditor-observability` | `auditor` | RED method, SLOs, OpenTelemetry |

> Subagents can also be invoked directly when needed (e.g. `auditor-security` alone without going through `auditor`).

> Full reference: [docs/architecture/agents.en.md](docs/architecture/agents.en.md)

---

## Available workflows

| Scenario | Entry point | Typical prompt |
|----------|------------|----------------|
| Feature end-to-end | `orchestrator` | `"Implement [feature]"` |
| Ready-to-code tickets | `orchestrator-dev` | `"Implement tickets bd-X to bd-Y"` |
| Pre-production audit | `auditor` | `"Audit the project"` |
| Production bug | `debugger` | `"This bug: [stacktrace]"` |
| Standalone UX/UI spec | `ux-designer` / `ui-designer` | `"UX spec for [feature]"` |
| Document a feature | `documentarian` | `"Document [topic]"` |
| Discover an existing project | `onboarder` | `"Onboard yourself on this project"` |
| Plan without implementing | `planner` | `"Break down [feature] into tickets"` |

> Detailed scenarios with diagrams and real prompts: [docs/guides/workflows.en.md](docs/guides/workflows.en.md)

---

## Documentation

### Guides

| Document | Description |
|----------|-------------|
| [Getting started](docs/guides/getting-started.en.md) | Full installation, first deployment |
| [LLM Providers](docs/guides/providers.en.md) | Anthropic, MammouthAI, GitHub Models, Bedrock, Ollama |
| [Workflows](docs/guides/workflows.en.md) | Full feature, audit, debug — illustrated scenarios |
| [Contributing](docs/guides/contributing.en.md) | Adding an agent, a skill, an adapter |

### Architecture

| Document | Description |
|----------|-------------|
| [Overview](docs/architecture/overview.en.md) | Concepts, flow diagrams, design principles |
| [Agents](docs/architecture/agents.en.md) | Exhaustive reference for all 27 agents |
| [Skills](docs/architecture/skills.en.md) | Exhaustive reference for skills and their dependencies |
| [ADR](docs/architecture/adr/) | Architectural decision records (6 ADRs) |

### Reference

| Document | Description |
|----------|-------------|
| [CLI](docs/reference/cli.en.md) | All `oc` commands with options and examples |
| [Configuration](docs/reference/config.en.md) | hub.json, projects.md, paths.local.md |

---

## License

MIT
