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
oh run feature                 # Launch a workflow (auto-detects project from cwd)
oh run ticket --tickets bd-42  # Implement a Beads ticket (one session per ticket)
oh run onboarding              # Create project wiki (docs/wiki/)
oh bundle show feature         # Session bundle of a workflow (agents, skills, budget)
oh serve                       # Local web dashboard on http://127.0.0.1:8080
```

---

## Commands

| Command | Description |
|---------|-------------|
| `oh init` | First-time setup wizard |
| `oh run <workflow>` | Launch a workflow session (opencode V2) |
| `oh workflow list` | Workflow catalogue |
| `oh bundle show <workflow>` | Session bundle (agents, skills, budget) |
| `oh start`, `oh audit`, `oh review`, `oh debug` | Deprecated aliases of `oh run` |
| `oh session list` | List v5 sessions (working, waiting, sleeping) |
| `oh session attach <id>` | Open a session (new tab/window; resumes a sleeping session) |
| `oh session stop <id>` | Stop a session |
| `oh daemon status` | oh daemon status (credential proxy, live servers) |
| `oh migrate deploy-cleanup` | Remove what the former `oh deploy` left in projects |
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
│       ├── deploy/      <- Legacy per-project deployment (opencode V1)
│       ├── bundle/      <- Session bundles (agents, skills, permissions)
│       ├── adapters/    <- Tool adapters (opencode V2)
│       ├── daemon/      <- oh daemon: credential proxy, session tracking
│       ├── runsvc/      <- Session launcher (server groups, attach, resume)
│       ├── mcp/         <- Native MCP servers (figma, gitlab, gslides, github, jira, linear, team)
│       ├── skillregistry/ <- Community skill discovery and install
│       ├── selfupdate/  <- oh binary self-update
│       ├── tui/         <- tview/tcell TUI (dashboard, board, sessions)
│       └── ...
└── docs/            <- Documentation (bilingual fr/en)
```

**Session flow (v5):**

```
oh run <workflow>
  -> ~/.oh/bundles/<hash>/   (session bundle: workflow agents, skills, permissions, MCP, plugin)
  -> opencode serve          (one server per group, closed world: only the bundle is visible)
  -> session opened in a terminal tab (iTerm2, Terminal.app, tmux) or followed from the TUI
```
Nothing is written into the project (`oh deploy` was removed in v5).

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
| Feature end-to-end | `oh run feature` | orchestrator |
| Ready-to-code tickets | `oh run ticket --tickets bd-42` | orchestrator-dev |
| Pre-production audit | `oh run audit -i type=security` | auditor |
| Production bug | `oh run debug -i issue="..."` | debugger |
| UX/UI spec from Figma | `oh run libre --agent designer` | designer |
| Document a feature | `oh run libre --agent documentarian` | documentarian |
| Discover a project | `oh run onboarding` | onboarder |
| Plan without implementing | `oh run cadrage` | conductor |
| Review a branch | `oh run review` | reviewer |
| Parallel multi-ticket | `oh run ticket --tickets bd-1,bd-2` | orchestrator-dev |

## Commands

### Sessions

| Command | Description |
|---------|-------------|
| `oh run <workflow> --recap` | Launch with recap + confirmation |
| `oh run <workflow>` | Launch immediately |
| `oh run ticket --tickets a,b` | One session per ticket (one server, a worktree per writing session) |
| `oh run onboarding` | Discover and document a codebase |
| `oh run libre --agent <id>` | Free session with the agent of your choice |
| `oh run audit -i type=<t>` | Code audit (security, performance, architecture, accessibility, ecodesign, observability) |
| `oh run review` | Code review (standard, adversarial, edge-case, complete) |
| `oh run debug -i issue="..."` | Debug session |
| `oh start`, `oh audit`, `oh review`, `oh debug` | Deprecated aliases of `oh run` (v5) |

### Projects & Deployment

| Command | Description |
|---------|-------------|
| `oh project add` | Register a new project |
| `oh project list` | List all projects |
| `oh project configure` | Configure project settings |
| `oh project remove` | Unregister a project |
| `oh migrate deploy-cleanup` | Remove the files of the former `oh deploy` (v5) |

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
- **[Beads](https://beads.sh/)** *(optional)* -- ticket tracker for `oh run ticket`, `oh board`

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
