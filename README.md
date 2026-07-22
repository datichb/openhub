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
oh serve                       # Local web dashboard on http://127.0.0.1:8080
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
| `oh metrics` | Usage and cost metrics (incl. agent telemetry) |
| `oh dashboard` | Interactive TUI dashboard |
| `oh board` | Kanban board (Beads tickets) |
| `oh serve [--port 8080] [--readonly]` | Local web dashboard (API + SPA, 127.0.0.1 only) |
| `oh audit` | Code audit via AI agent |
| `oh review` | Code review via AI agent |
| `oh debug` | Debug session via AI agent |
| `oh export [--output path]` | Backup DB + config + secrets to .tar.gz with SHA-256 checksum |
| `oh import <file> [--overwrite] [--merge]` | Restore from backup archive |
| `oh repair [--check-only] [--auto]` | Diagnose and repair corrupted SQLite DB |
| `oh upgrade opencode` | Update the opencode binary |
| `oh upgrade oh [--check] [version]` | Self-update the oh binary (non-Homebrew installs) |
| `oh mcp serve` | Run a built-in MCP server |
| `oh skill add <source>` | Install a community skill from index name or Git URL |
| `oh skill list` | List installed community skills |
| `oh skill remove <name>` | Uninstall a community skill |
| `oh skill search [query]` | Search the community index |
| `oh beads` | Proxy to bd (Beads CLI) |

> Full reference: [docs/reference/cli.en.md](docs/reference/cli.en.md)

---

## Architecture

```
openhub/
├── agents/          <- AI role definitions (22 agents, 2 modes)
├── skills/          <- Protocols: Bucket A (inline) + Bucket B (on-demand)
├── cli/             <- Go CLI binary (oh)
│   └── internal/
│       ├── beads/       <- Beads ticket integration
│       ├── deploy/      <- Transactional deployment engine
│       ├── mcp/         <- Native MCP servers (figma, gitlab, gslides, github, jira, linear, team)
│       ├── skillregistry/ <- Community skill discovery and install
│       ├── selfupdate/  <- oh binary self-update
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

22 specialized agents in two modes:

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
| `debugger` | Quality | Bug diagnosis, root cause |
| `benchmarker` | Quality | Lighthouse, k6, pprof, py-spy performance benchmarks |
| `test-generator` | Quality | Gap analysis, unit/integration/property-based test generation |
| `documentarian` | Documentation | README, CHANGELOG, ADR, API docs |

### Subagents

| Agent | Delegated by | Domain |
|-------|-------------|--------|
| `developer` | `orchestrator-dev` | Implementation (frontend, backend, fullstack, api, mobile, data, devops, platform, security) |
| `developer-refactor` | `orchestrator-dev` | Structural refactoring |
| `developer-migrator` | `orchestrator-dev` | Incremental migrations |
| `database` | `orchestrator-dev` | Schema, migration, query optimization, DB security audit |
| `infra` | `orchestrator-dev` | Terraform/K8s review, cost estimation, IaC security |
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

Seven built-in MCP servers, running natively in Go (stdio protocol):

| Server | Command | Purpose | Requires |
|--------|---------|---------|---------|
| Figma | `oh mcp serve figma` | Design token extraction, component analysis | `FIGMA_TOKEN` |
| GitLab | `oh mcp serve gitlab` | Issue/MR management, pipeline status | `GITLAB_TOKEN` |
| Google Slides | `oh mcp serve gslides` | Presentation analysis | Google credentials |
| GitHub | `oh mcp serve github` | Issues, PRs, Actions | `GITHUB_TOKEN` |
| Jira | `oh mcp serve jira` | Issue management, transitions | `JIRA_URL` + `JIRA_TOKEN` |
| Linear | `oh mcp serve linear` | Issues/mutations via GraphQL | `LINEAR_API_KEY` |
| Team | `oh mcp serve team` | Team coordination, claims, wiki | Team-state repo |

Configure via `oh mcp setup` (stores tokens in OS keychain).

---

## Documentation

### Guides

| Document | Description |
|----------|-------------|
| [Getting started](docs/guides/getting-started.en.md) | Installation, first deployment |
| [Workflows](docs/guides/workflows.en.md) | Full feature, audit, debug scenarios |
| [Figma Integration](docs/guides/figma-integration.en.md) | MCP setup and usage |
| [GitLab Integration](docs/guides/gitlab-integration.en.md) | GitLab MCP setup |
| [GitHub Integration](docs/guides/github-integration.en.md) | GitHub MCP setup |
| [Jira Integration](docs/guides/jira-integration.en.md) | Jira MCP setup |
| [Linear Integration](docs/guides/linear-integration.en.md) | Linear MCP setup |
| [Skill Marketplace](docs/guides/skill-marketplace.en.md) | Installing community skills |
| [Dashboard](docs/guides/dashboard.en.md) | Web dashboard setup and usage |
| [Backup & Restore](docs/guides/backup-restore.en.md) | Export/import, repair |
| [LLM Providers](docs/guides/providers.en.md) | Anthropic, Bedrock, OpenRouter, Ollama |
| [Onboarding](docs/guides/onboarding.en.md) | Using the onboarder agent |

### Architecture

| Document | Description |
|----------|-------------|
| [Overview](docs/architecture/overview.en.md) | Concepts, flow diagrams |
| [Agents](docs/architecture/agents.en.md) | All 22 agents reference |
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

**Platform support:** macOS (amd64/arm64), Linux (amd64/arm64), Windows (amd64/arm64).

---

## License

MIT
