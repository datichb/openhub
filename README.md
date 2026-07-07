> [Lire en francais](README.fr.md)

# openhub (`oh`)

Central hub for managing AI assistants across multiple projects.
Shared agents, hybrid skills, integrated Beads workflow, and Go-native MCP servers.

**Single binary, zero dependencies.**

---

## Installation

### Homebrew (recommended)

```bash
brew install datichb/tap/openhub
```

### Curl script

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/openhub/main/install.sh | bash
```

### From source

```bash
cd cli && go install .
```

---

## Quick start

```bash
oh init                        # First-time setup: language, opencode, project, MCP
oh start                       # Launch opencode (auto-detects project from cwd)
oh start --dev                 # Dev mode: pick epics/tickets, orchestrator-dev
oh start --onboard             # Create project wiki (docs/wiki/)
oh deploy                      # Sync agents, skills, config, MCP to project
```

---

## Commands

| Command | Description |
|---------|-------------|
| `oh init` | First-time setup wizard |
| `oh start` | Launch an opencode session |
| `oh start --dev` | Dev mode: ticket picker + orchestrator-dev |
| `oh start --onboard` | Onboarding: create/refresh project wiki |
| `oh quick` | Quick task with auto project detection |
| `oh deploy` | Deploy agents, skills, config, MCP |
| `oh sync` | Sync all registered projects |
| `oh project list` | List registered projects |
| `oh project add` | Register a new project |
| `oh config` | Manage hub configuration |
| `oh status` | Show hub and project status |
| `oh doctor` | System health check |
| `oh metrics` | Usage and cost metrics |
| `oh dashboard` | Interactive TUI dashboard |
| `oh board` | Kanban board (Beads tickets) |
| `oh audit` | Code audit via AI agent |
| `oh review` | Code review via AI agent |
| `oh debug` | Debug session via AI agent |
| `oh upgrade opencode` | Update the opencode binary |
| `oh mcp serve` | Run a built-in MCP server |
| `oh beads` | Proxy to bd (Beads CLI) |

> Full reference: [docs/reference/cli.en.md](docs/reference/cli.en.md)

---

## Architecture

```
openhub/
├── agents/          <- AI role definitions (18 agents, 2 modes)
├── skills/          <- Protocols: Bucket A (inline) + Bucket B (on-demand)
├── cli/             <- Go CLI binary (oh)
│   └── internal/
│       ├── beads/       <- Beads ticket integration
│       ├── deploy/      <- Transactional deployment engine
│       ├── mcp/         <- Native MCP servers (figma, gitlab, gslides)
│       ├── tui/         <- BubbleTea views (dashboard, board, picker)
│       └── ...
└── docs/            <- Documentation (bilingual fr/en)
```

**Deployment flow:**

```
oh deploy
  -> .opencode/agents/*.md        (agent definitions)
  -> .opencode/skills/*/SKILL.md  (protocols)
  -> opencode.json                (provider, model, MCP, permissions)
```

---

## Agents

18 specialized agents in two modes:

- **`primary`** -- directly invocable by the user in OpenCode
- **`subagent`** -- delegated by coordinator agents

### Primary agents

| Agent | Family | Role |
|-------|--------|------|
| `orchestrator` | Coordinator | Feature end-to-end |
| `orchestrator-dev` | Coordinator | Ticket implementation (drives developers) |
| `auditor` | Coordinator | Multi-domain audit (7 domains) |
| `onboarder` | Coordinator | Project discovery, wiki creation |
| `planner` | Planning | Break down features into Beads tickets |
| `designer` | Design | Figma analysis, UX/UI specs |
| `reviewer` | Quality | PR/MR review by severity (multi-mode: standard, adversarial, edge-case) |
| `qa-engineer` | Quality | Test coverage analysis |
| `debugger` | Quality | Bug diagnosis, root cause |
| `documentarian` | Documentation | README, CHANGELOG, ADR, API docs |

### Subagents

| Agent | Delegated by | Domain |
|-------|-------------|--------|
| `developer` | `orchestrator-dev` | Implementation (frontend, backend, fullstack, api, mobile, data, devops, platform, security) |
| `developer-refactor` | `orchestrator-dev` | Structural refactoring |
| `developer-migrator` | `orchestrator-dev` | Incremental migrations |
| `auditor-subagent` | `auditor` | All audit domains (security, performance, accessibility, ecodesign, architecture, privacy, observability) |

---

## Key workflows

| Scenario | Command | Agent |
|----------|---------|-------|
| Feature end-to-end | `oh start -a orchestrator` | orchestrator |
| Ready-to-code tickets | `oh start --dev` | orchestrator-dev |
| Pre-production audit | `oh audit --type security` | auditor |
| Production bug | `oh debug --issue "..."` | debugger |
| UX/UI spec from Figma | `oh start -a designer` | designer |
| Document a feature | `oh start -a documentarian` | documentarian |
| Discover a project | `oh start --onboard` | onboarder |
| Plan without implementing | `oh start -a planner` | planner |
| Review a branch | `oh review` | reviewer |

---

## MCP Servers

Three built-in MCP servers, running natively in Go (stdio protocol):

| Server | Command | Purpose |
|--------|---------|---------|
| Figma | `oh mcp serve figma` | Design token extraction, component analysis |
| GitLab | `oh mcp serve gitlab` | Issue/MR management, pipeline status |
| Google Slides | `oh mcp serve gslides` | Presentation analysis |

Configure via `oh service setup` (stores tokens in OS keychain).

---

## Documentation

### Guides

| Document | Description |
|----------|-------------|
| [Getting started](docs/guides/getting-started.en.md) | Installation, first deployment |
| [Workflows](docs/guides/workflows.en.md) | Full feature, audit, debug scenarios |
| [Figma Integration](docs/guides/figma-integration.en.md) | MCP setup and usage |
| [GitLab Integration](docs/guides/gitlab-integration.en.md) | GitLab MCP setup |
| [LLM Providers](docs/guides/providers.en.md) | Anthropic, Bedrock, OpenRouter, Ollama |
| [Onboarding](docs/guides/onboarding.en.md) | Using the onboarder agent |

### Architecture

| Document | Description |
|----------|-------------|
| [Overview](docs/architecture/overview.en.md) | Concepts, flow diagrams |
| [Agents](docs/architecture/agents.en.md) | All 18 agents reference |
| [Skills](docs/architecture/skills.en.md) | Hybrid skill system |
| [ADRs](docs/architecture/adr/) | 21 architectural decision records |

### Reference

| Document | Description |
|----------|-------------|
| [CLI Reference](docs/reference/cli.en.md) | All commands with options and examples |
| [Configuration](docs/reference/config.en.md) | hub.toml, project settings |
| [Beads Data Model](docs/reference/beads-model.en.md) | Ticket system reference |

---

## Migration from `oc`

If you were using the bash CLI (`oc`), see the [Migration Guide](MIGRATION.md) for:
- Command equivalence table
- Configuration migration (hub.json -> hub.toml)
- Breaking changes

---

## Requirements

- **[OpenCode](https://opencode.ai)** -- AI coding agent (auto-downloaded by `oh init`)
- **[git](https://git-scm.com/)** -- version control
- **[Beads](https://beads.sh/)** *(optional)* -- ticket tracker for `oh start --dev`, `oh board`

No Node.js, jq, sqlite3, or bun required. The Go binary is self-contained.

---

## License

MIT
