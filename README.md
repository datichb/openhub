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
| `oh start --recap` | Launch with configuration recap |
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
├── agents/          <- AI role definitions (19 agents, 7 families)
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

19 specialized agents across 7 families, in two modes:

- **`primary`** -- directly invocable by the user in OpenCode
- **`subagent`** -- delegated by coordinator agents

### Primary agents (14)

| Agent | Family | Role |
|-------|--------|------|
| `orchestrator` | Planning | Feature end-to-end coordinator |
| `orchestrator-dev` | Planning | Ticket implementation (drives developers) |
| `planner` | Planning | Break down features into Beads tickets |
| `pathfinder` | Planning | Fast reconnaissance, complexity estimation |
| `onboarder` | Planning | Project discovery, wiki creation |
| `auditor` | Auditor | Multi-domain audit coordinator (7 domains) |
| `designer` | Design | Figma analysis, UX/UI specs (4 modes: recon, ux, ui, ux+ui) |
| `reviewer` | Quality | PR/MR review by severity (multi-mode: standard, adversarial, edge-case) |
| `debugger` | Quality | Bug diagnosis, root cause |
| `benchmarker` | Quality | Lighthouse, k6, pprof, py-spy performance benchmarks |
| `test-generator` | Quality | Gap analysis, unit/integration/property-based test generation |
| `database` | Developer | Schema, migration, query optimization, DB security audit |
| `infra` | Developer | Terraform/K8s review, cost estimation, IaC security |
| `documentarian` | Documentation | README, CHANGELOG, ADR, API docs |

### Subagents (5)

| Agent | Delegated by | Domain |
|-------|-------------|--------|
| `developer` | `orchestrator-dev` | Implementation (frontend, backend, fullstack, api, mobile, data, devops, platform, security) |
| `developer-refactor` | `orchestrator-dev` | Structural refactoring |
| `developer-migrator` | `orchestrator-dev` | Incremental migrations |
| `auditor-subagent` | `auditor` | All audit domains (security, performance, accessibility, ecodesign, architecture, privacy, observability) |
| `brief-enricher` | Various | Read-only takeover brief enrichment |

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
| Parallel multi-ticket | `oh start --parallel` | orchestrator-dev |

## Commands

### Sessions

| Command | Description |
|---------|-------------|
| `oh start --recap` | Launch with configuration recap + confirmation |
| `oh start` | Launch immediately (quick mode by default) |
| `oh start --dev` | Dev mode: pick tickets to implement |
| `oh start --onboard` | Discover and document a codebase |
| `oh start --parallel` | Parallel sessions on multiple tickets |
| `oh audit --type <t>` | Code audit (security, performance, architecture, accessibility, ecodesign, observability) |
| `oh review` | Code review (standard, adversarial, edge-case, complete) |
| `oh debug --issue "..."` | Debug session |

### Projects & Deployment

| Command | Description |
|---------|-------------|
| `oh project add` | Register a new project |
| `oh project list` | List all projects |
| `oh project configure` | Configure project settings |
| `oh project remove` | Unregister a project |
| `oh deploy` | Deploy agents/skills to project |
| `oh sync --all` | Sync to all projects |

### Configuration

| Command | Description |
|---------|-------------|
| `oh init` | First-time setup wizard |
| `oh config list` | Show all settings |
| `oh config model default <m>` | Set default model |
| `oh provider setup` | Configure LLM provider credentials |
| `oh mcp setup` | Configure MCP server tokens |
| `oh secrets set <key> <val>` | Store a secret |

### Team Collaboration

| Command | Description |
|---------|-------------|
| `oh team init` | Set up team features |
| `oh team claim <id>` | Claim a ticket |
| `oh team release <id>` | Release a ticket |
| `oh team status` | Team status overview |
| `oh team activity` | Recent team events |
| `oh team board` | Team kanban board |
| `oh team sync-tracker` | Sync claims to external tracker |
| `oh teams list` | List all teams |
| `oh takeover-brief show <id>` | View takeover context |

### Quality & Governance

| Command | Description |
|---------|-------------|
| `oh conventions check` | Validate against conventions |
| `oh patterns list` | List team patterns |
| `oh policies check` | Validate against policies |
| `oh worktree list` | List active worktrees |

### System

| Command | Description |
|---------|-------------|
| `oh doctor` | Health check (version, credentials, MCP) |
| `oh status` | Hub and project status |
| `oh metrics` | Agent usage and cost stats |
| `oh serve` | Start local web dashboard |
| `oh export` | Export hub data |
| `oh import` | Import/restore hub data |
| `oh repair` | Repair corrupted state |
| `oh upgrade oh` | Upgrade oh binary |
| `oh upgrade opencode` | Upgrade opencode binary |
| `oh plugin list` | List installed plugins |

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
| [Parallel Mode](docs/guides/parallel-mode.en.md) | Run N concurrent AI sessions in isolated worktrees |
| [Sweep Mode](docs/guides/sweep-mode.en.md) | Goal-driven task decomposition and parallel execution |
| [Review & Feedback](docs/guides/review-feedback.en.md) | AI code review, MR publication, feedback processing |
| [Troubleshooting](docs/guides/troubleshooting.en.md) | `oh doctor`, `oh repair`, common errors |
| [Notifications](docs/guides/notifications.en.md) | Slack, Discord, Mattermost, Teams notifications |
| [Figma Integration](docs/guides/figma-integration.en.md) | MCP setup and usage |
| [GitLab Integration](docs/guides/gitlab-integration.en.md) | GitLab MCP setup |
| [GitHub Integration](docs/guides/github-integration.en.md) | GitHub MCP setup |
| [Jira Integration](docs/guides/jira-integration.en.md) | Jira MCP setup |
| [Linear Integration](docs/guides/linear-integration.en.md) | Linear MCP setup |
| [Google Slides Integration](docs/guides/gslides-integration.en.md) | Google Slides MCP setup |
| [Skill Marketplace](docs/guides/skill-marketplace.en.md) | Installing community skills |
| [Dashboard](docs/guides/dashboard.en.md) | Web dashboard setup and usage |
| [Backup & Restore](docs/guides/backup-restore.en.md) | Export/import, repair |
| [LLM Providers](docs/guides/providers.en.md) | Anthropic, Bedrock, OpenRouter, Ollama |
| [Onboarding](docs/guides/onboarding.en.md) | Using the onboarder agent |

### Architecture

| Document | Description |
|----------|-------------|
| [Overview](docs/architecture/overview.en.md) | Concepts, flow diagrams |
| [Agents](docs/architecture/agents.en.md) | All 19 agents reference |
| [Skills](docs/architecture/skills.en.md) | Hybrid skill system |
| [ADRs](docs/architecture/adr/) | 36 architectural decision records |

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

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, conventions, and PR process.

## Security

See [SECURITY.md](SECURITY.md) for vulnerability reporting and security scope.

---

## License

MIT
