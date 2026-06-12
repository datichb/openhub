> 🇫🇷 [Lire en français](README.fr.md)

# opencode-hub

Central hub for managing AI assistants across multiple projects,
with shared agents, hybrid skills, and an integrated Beads workflow.

Supports **OpenCode**.

---

## How it works

opencode-hub is built around three concepts: **agents**, **MCP servers**, and **deployment**.

- **Agents** define AI roles (who does what, how, and in what order) with integrated protocols (code standards, checklists, report formats). Each agent has **Bucket A skills** (mandatory protocols, always inline) and **Bucket B skills** (domain context, loaded on-demand).
- **MCP Servers** provide tool integrations (Figma, Linear, etc.) available to all agents.
- **Deployment** assembles agents + MCP servers and copies them into your target projects.

```
opencode-hub/          ← single source of truth (edit here, never in projects)
├── agents/            ← AI role definitions with integrated protocols
├── skills/            ← protocols: Bucket A (inline) + Bucket B (native, on-demand)
├── servers/           ← MCP servers (Figma integration, etc.)
└── scripts/           ← assembly and deployment

         oc deploy opencode MY-APP
opencode-hub  ──────────────────────►  my-app/.opencode/agents/*.md       (Bucket A inline)
                                   ├►  my-app/.opencode/skills/*/SKILL.md  (Bucket B native)
                                   ├►  my-app/.opencode/servers/
                                   └►  my-app/opencode.json

```

Result: 19 specialized agents + Figma integration, always up to date, available across all your projects
from a single source of truth.

---

## Requirements

- **[OpenCode](https://opencode.ai)** — AI coding agent
- **[jq](https://jqlang.github.io/jq/)** — required for `oc deploy` (generates `opencode.json`)
- **[git](https://git-scm.com/)**, **[node](https://nodejs.org/)**, **[bun](https://bun.sh/)** — standard toolchain
- **[sqlite3](https://sqlite.org/)** — required for `oc metrics` and `oc dashboard` (reads OpenCode session database). Native on macOS; on Linux: `sudo apt-get install sqlite3`
- **[Beads](https://beads.sh/)** *(optional)* — task tracker integration for ticket views in metrics and dashboard

The install script detects and installs missing dependencies automatically (Homebrew on macOS, apt-get on Linux).

---

## Installation

### One-liner (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/opencode-hub/main/install.sh | bash
```

The script automates everything: clones to `~/.opencode-hub`, checks dependencies
with confirmation prompts, creates the `oc` alias, and configures the LLM provider.

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

### Update installed tools (opencode, Beads)

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

19 agents in two modes:

- **`primary`** — directly visible in the AI tool (OpenCode tab picker). Invocable by the user.
- **`subagent`** — hidden from the picker. Only invocable by delegation from a coordinator agent.

### Primary agents (directly invocable)

| Agent | Family | Role |
|-------|--------|------|
| `orchestrator` | Coordinator | Feature end-to-end — delegates spec, audit, implementation |
| `orchestrator-dev` | Coordinator | Ticket implementation — drives `developer` agent (domain passed at invocation) |
| `auditor` | Coordinator | Multi-domain audit — delegates to 7 `auditor-*` agents |
| `onboarder` | Coordinator | Discovery of existing projects — detects stack, business domain, Figma designs, test strategy, and produces context report |
| `planner` | Planning | Breaks down a feature into Beads tickets |
| `ux-designer` | Design | UX spec — user flows, acceptance criteria |
| `ui-designer` | Design | UI spec — tokens, components, visual guidelines |
| `reviewer` | Quality | PR/MR review by severity |
| `qa-engineer` | Quality | Missing tests (unit / integration / E2E). **Automatically invoked for critical code** (API, services, >200 lines). Produces coverage report and review attention points. |
| `debugger` | Quality | Bug diagnosis, root cause report |
| `documentarian` | Documentation | README, CHANGELOG, ADR, API docs |

### Subagents (delegated by coordinators)

| Agent | Delegated by | Domain |
|-------|-------------|--------|
| `developer` | `orchestrator-dev` | Implementation — domain passed at invocation: frontend, backend, fullstack, api, mobile, data, devops, platform, security |
| `developer-refactor` | `orchestrator-dev` | Structural refactoring — never changes observable behavior |
| `developer-migrator` | `orchestrator-dev` | Incremental migrations — framework upgrades, major versions, EOL dependencies |
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
| Debug a bug | `debugger` | `"oc debug"` |
| Run an audit | `auditor` | `"oc audit"` |
| Check conventions | `reviewer` | `"oc conventions"` |
| Review a branch | `reviewer` | `"oc review"` |

> Detailed scenarios with diagrams and real prompts: [docs/guides/workflows.en.md](docs/guides/workflows.en.md)

---

## Figma Integration

opencode-hub integrates with Figma to enrich planning workflows with design context.

### Features

- **Automatic maquette detection**: Pathfinder, Planner, and Onboarder search Figma files by feature or project name
- **UX/UI signal detection**: Automatic detection of multi-step flows and visual components
- **Design tokens extraction**: Extract colors, typography, spacing, and effects from Figma Variables
- **Design system detection**: Automatically identify DSFR, Material Design, or custom design systems
- **Enriched estimation**: Adjust complexity based on detected components and states
- **Design context pre-filling**: Auto-populate `--design` fields in tickets with Figma data

### Setup

1. **Get your Figma tokens** (Personal Access Token + Team ID)
2. **Configure** `~/.config/opencode/config.json`:
   ```json
   {
     "env": {
       "FIGMA_PERSONAL_ACCESS_TOKEN": "figd_xxx",
       "FIGMA_TEAM_ID": "123456"
     }
   }
   ```
3. **Organize your Figma files** according to conventions (see `config/figma.conventions.md`)
4. **Deploy** to your projects with `oc deploy opencode MY-APP`

### Usage

The Pathfinder, Planner, and Onboarder agents automatically query Figma when analyzing UI features or exploring projects:

```bash
# Pathfinder with Figma enrichment
> Pathfinder cette feature: dashboard utilisateur

# Planner with Figma context (Phase 1.3)
> Planifie cette feature: processus inscription

# Onboarder with Figma exploration (Phase 1.5)
> Onboarde-toi sur ce projet
# → Extracts design tokens, detects design system, lists components
```

📖 **Full documentation**: [Figma Integration Guide](docs/guides/figma-integration.en.md)

---

## Documentation

### Guides

| Document | Description |
|----------|-------------|
| [Getting started](docs/guides/getting-started.en.md) | Full installation, first deployment |
| [Figma Integration](docs/guides/figma-integration.en.md) | Figma MCP setup, configuration, and testing |
| [LLM Providers](docs/guides/providers.en.md) | Anthropic, MammouthAI, GitHub Models, Bedrock, Ollama |
| [Workflows](docs/guides/workflows.en.md) | Full feature, audit, debug — illustrated scenarios |
| [Contributing](docs/guides/contributing.en.md) | Adding an agent, an adapter |
| [Onboarding](docs/guides/onboarding.en.md) | Onboarding agent guide (using the onboarder to discover a project) |
| [Authoring](docs/guides/authoring.en.md) | Authoring guide (designing agents) |

### Architecture

| Document | Description |
|----------|-------------|
| [Overview](docs/architecture/overview.en.md) | Concepts, flow diagrams, design principles |
| [Agents](docs/architecture/agents.en.md) | Exhaustive reference for all 19 agents |
| [MCP Servers](servers/README.md) | MCP servers architecture and development |
| [ADR](docs/architecture/adr/) | Architectural decision records (9 ADRs) |
| [Adapters](docs/architecture/adapters.en.md) | Adapters architecture |

### Reference

| Document | Description |
|----------|-------------|
| [CLI](docs/reference/cli.en.md) | All `oc` commands with options and examples |
| [Configuration](docs/reference/config.en.md) | hub.json, projects.md, paths.local.md |
| [Figma Conventions](config/figma.conventions.md) | Figma file organization conventions |
| [Beads data model](docs/reference/beads-model.en.md) | Beads data model reference |
| [Audit tools](docs/reference/audit-tools.en.md) | Audit tools reference by domain |
| [Model resolution](docs/reference/model-resolution.en.md) | Model resolution by agent |

### Development

| Document | Description |
|----------|-------------|
| [Performance optimizations](docs/dev/performance-optimizations.md) | Performance improvements in `oc deploy` |
| [Progress bar system](docs/dev/progress-bar.md) | Visual feedback system for long operations |
| [Shell gotchas](docs/dev/shell-gotchas.md) | Common pitfalls in bash scripting |

---

## License

MIT
