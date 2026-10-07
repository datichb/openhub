> [Lire en français](README.fr.md)

# openhub (`oh`)

Central hub to run AI coding sessions across your projects, with shared agents and skills, declarative workflows, an integrated Beads ticket flow and Go-native MCP servers.

**Single binary, zero dependencies.** `oh` is the control tower, [opencode](https://opencode.ai) V2 is the cockpit.

---

## What oh v5 does

- **Declarative workflows.** Each use case (feature, ticket, review, audit…) is a YAML workflow (`apiVersion: oh/v1`): entry agent, member agents and their order, checkpoints per mode (`manuel`, `semi-auto`, `auto`), inputs, outputs, resources, risk level and limits. The hub ships 12 of them; a team or a project can extend them in its team-state repository.
- **Session bundle.** Nothing is written into your projects any more. At launch, oh builds a session bundle outside the project (`~/.oh/bundles/<hash>/`, immutable) from the workflow: agents, skills, permissions, MCP servers, oh plugin.
- **Closed world.** A session sees only the agents and skills of its bundle (opencode's native agents are disabled). It is checked at each server start (`Attest`); if anything else is visible, the session does not start.
- **Sessions driven from oh.** One `opencode serve` per group, several sessions per server (one per ticket, one worktree per writing session). Decisions (⏸ checkpoint, ? question, ! permission, $ budget, ✗ error) are answered from the TUI, the CLI, opencode or the browser; the first answer wins. Closing opencode does not stop the session; an idle server goes to sleep after 5 minutes and resumes on the same bundle.
- **Secrets stay on the machine.** The `ohd` daemon hosts a credential proxy: opencode only gets a group token (`ohs_…`), the real LLM key stays in your keychain.
- **Local, container or remote.** Run a session on your machine, in a container built from the project's dev Dockerfile (Colima, Podman, Docker CLI), or in a GitLab CI job. Beads always stays on the machine.

---

## Installation

### 1. oh

**Homebrew (recommended):**

```bash
brew install datichb/tap/openhub
```

**Curl script:**

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/openhub/main/install.sh | bash
```

**From source:**

```bash
cd cli && go install .
```

### 2. opencode V2 (2.0.0 or later)

oh no longer installs opencode. Install it with its own tool:

```bash
brew install anomalyco/tap/opencode    # or see https://opencode.ai
```

opencode V1 is no longer supported: oh refuses to start a session with a clear message. `oh doctor` checks the version. Coming from oh v4: see [Migrating to oh v5](docs/guides/migration-v5.en.md).

---

## Quick start

```bash
oh init                              # First-time setup: language, opencode V2 check, project, MCP, workflow space
oh run quick -i request="Fix the typo in the header"   # Small change, one agent
oh run feature                       # Feature end-to-end (plan, checkpoints, implementation, review)
oh run ticket --tickets bd-42,bd-43  # One session per Beads ticket (one server, one worktree each)
oh session inbox                     # Pending decisions of all sessions
oh                                   # TUI: Start, Sessions, workflow catalogue, settings
```

Useful options of `oh run`: `--recap` (summary and confirmation), `--mode manuel|semi-auto|auto`, `--runtime local|container|remote`, `--location base|new|<path>`, `--headless`, `-a/--agent` (with `libre`). Without argument, `oh run` launches the project's default workflow.

In the TUI, **Start** lists the workflows (★ pinned, recent ones), opens the launch form (Inputs → Options → Recap, `Ctrl+S` launches), and the **Sessions** view gathers the sessions to handle, in progress, sleeping, finished and to fetch.

---

## Main commands

| Command | Description |
|---------|-------------|
| `oh` | Interactive TUI (interactive terminal) |
| `oh init` | First-time setup wizard |
| `oh run [workflow]` | Launch a workflow (one session per ticket with `--tickets`) |
| `oh workflow list` · `show` · `validate` | Workflow catalogue, resolved workflow (with origins), validation |
| `oh workflow new` · `edit` · `diff` · `publish` · `history` · `restore` · `archive` | Team or project workflows (drafts, publication, history) |
| `oh bundle show <workflow>` · `build` | Session bundle of a workflow (agents, skills, permissions, `--budget`) |
| `oh session list` · `inbox` | Sessions (in progress, waiting, sleeping; `--all`) and pending decisions |
| `oh session attach <id>` · `follow` · `open` | Open a session (tab, tmux, browser, current terminal), follow it live, open it in the browser |
| `oh session approve` · `answer` · `dismiss` | Answer a checkpoint or a permission, a question, dismiss an alert |
| `oh session send` · `interrupt` · `model` · `compact` · `fork` | Steer a running session |
| `oh session results <id>` | Changed files, branch, cost; `--mr` (MR description), `--patch` |
| `oh session resume` · `stop` | Resume a sleeping session, stop a session |
| `oh session fetch` · `resolve` | Fetch a finished remote session, replay its Beads journal |
| `oh daemon status` · `stop` | oh daemon (credential proxy, session supervision) |
| `oh budget show` · `set` · `unset` · `raise` | Session limits (disabled by default) |
| `oh remote setup` · `status` | Remote execution on GitLab CI (`oh-runner` project) |
| `oh migrate deploy-cleanup` | Remove what the former `oh deploy` left in projects |
| `oh project list` · `add` · `configure` · `remove` | Registered projects |
| `oh config list` · `oh config model default <m>` | Hub configuration, model cascade |
| `oh provider setup` · `oh mcp setup` · `oh secrets set` | LLM credentials, MCP tokens, secrets (OS keychain) |
| `oh team init [--solo]` · `oh team promote` | Team features, solo space and its sharing |
| `oh team claim` · `release` · `status` · `board` | Ticket claims and team view |
| `oh doctor` · `oh status` · `oh repair` | Health check, status, database repair |
| `oh metrics` · `oh dashboard` · `oh board` · `oh serve` | Usage and cost, TUI dashboard, Beads kanban, local web dashboard |
| `oh export` · `oh import` | Backup and restore |
| `oh upgrade oh` | Self-update of oh (non-Homebrew installs) |
| `oh skill add` · `list` · `remove` · `search` | Community skills |
| `oh beads …` | Proxy to `bd` (Beads CLI) |

**Deprecated aliases** (warning, then `oh run`): `oh start` → `oh run feature`, `oh start --agent X` → `oh run libre --agent X`, `oh start --dev` → `oh run ticket`, `oh audit` → `oh run audit`, `oh review` → `oh run review` (`--publish` stays an oh command), `oh debug` → `oh run debug`. `oh deploy` and `oh sync` only display a migration message.

> Full reference: [docs/reference/cli.en.md](docs/reference/cli.en.md) · [CLI — Workflows](docs/reference/cli-workflows.en.md) · [CLI — Sessions](docs/reference/cli-sessions.en.md) · [v5 Sessions](docs/guides/sessions-v5.en.md)

---

## Workflows

The 12 workflows shipped by the hub:

| Workflow | Entry agent | Use |
|----------|-------------|-----|
| `feature` | `orchestrator` | Feature end-to-end: plan, design, implementation, review |
| `ticket` | `orchestrator-dev` | Ready-to-code Beads tickets (`--tickets a,b`: one session per ticket) |
| `quick` | `developer` | Small, well-defined change |
| `cadrage` | `conductor` | Plan without implementing |
| `onboarding` | `onboarder` | Discover a project, write its wiki (`docs/wiki/`) |
| `review` | `reviewer` | Review a branch (read only) |
| `review-feedback` | `orchestrator-dev` | Process the feedback of a review |
| `audit` | `auditor` | Multi-domain audit (`-i type=security`…) |
| `debug` | `debugger` | Diagnose a bug (`-i issue="…"`) |
| `sweep` | `conductor` | Goal-driven series of changes (`-i goal=…`) |
| `brief-enrich` | `brief-enricher` | Enrich a takeover brief (headless) |
| `libre` | your choice (`--agent`) | Free session with the agent of your choice |

Workflows resolve **by layers**: hub < team < project < session options. A layer can extend another (`extends`) and lock fields (`enforce`); security can only harden. See [Shipped workflows](docs/reference/workflows.en.md), [Workflow schema](docs/reference/workflow-schema.en.md) and [Team workflows](docs/guides/team-workflows.en.md).

---

## Agents

20 agents in 7 families. Their default mode comes from their frontmatter; a workflow decides which ones are members of a session and may change their mode.

- **`primary`**: entry point of a session
- **`subagent`**: delegated by another agent of the workflow

### Primary agents (15)

| Agent | Family | Role |
|-------|--------|------|
| `conductor` | Planning | Generic entry agent: follows the workflow map, runs the agents in order, passes checkpoints |
| `orchestrator` | Planning | Feature end-to-end coordinator |
| `orchestrator-dev` | Planning | Ticket implementation (drives the developers) |
| `planner` | Planning | Break down features into Beads tickets |
| `pathfinder` | Planning | Fast reconnaissance, complexity estimation |
| `onboarder` | Planning | Project discovery, wiki creation |
| `auditor` | Auditor | Multi-domain audit coordinator (7 domains) |
| `designer` | Design | Figma analysis, UX/UI specs (recon, ux, ui, ux+ui) |
| `reviewer` | Quality | PR/MR review by severity (standard, adversarial, edge-case) |
| `debugger` | Quality | Bug diagnosis, root cause |
| `benchmarker` | Quality | Lighthouse, k6, pprof, py-spy performance benchmarks |
| `test-generator` | Quality | Gap analysis, unit/integration/property-based tests |
| `database` | Developer | Schema, migrations, query optimization, DB security audit |
| `infra` | Developer | Terraform/K8s review, cost estimation, IaC security |
| `documentarian` | Documentation | README, CHANGELOG, ADR, API docs |

### Subagents (5)

| Agent | Delegated by | Domain |
|-------|-------------|--------|
| `developer` | `orchestrator-dev` | Implementation (frontend, backend, fullstack, api, mobile, data, devops, platform, security) |
| `developer-refactor` | `orchestrator-dev` | Structural refactoring |
| `developer-migrator` | `orchestrator-dev` | Incremental migrations |
| `auditor-subagent` | `auditor` | All audit domains (security, performance, accessibility, ecodesign, architecture, privacy, observability) |
| `brief-enricher` | `brief-enrich` workflow | Read-only takeover brief enrichment |

See [Agents](docs/architecture/agents.en.md).

---

## Execution environments

| Environment | Where opencode runs | What stays on the machine |
|-------------|---------------------|---------------------------|
| local | `opencode serve` on your machine | everything |
| container | one container per group (Colima, Podman, Docker CLI), image = project dev Dockerfile + oh layer, bundle mounted read-only | keys (proxy), Beads (gateway), oh MCP servers (gateway) |
| remote | GitLab CI job of the `oh-runner` project | your keys; Beads: snapshot in, journal out, replayed locally (`oh session resolve`) |

Choose with `oh run --runtime …`, else the project setting, the Settings, then the workflow default, within the workflow's `runtime.allowed`. Container and remote run on macOS and Linux. See [Container runtime](docs/guides/container.en.md) and [Remote runners](docs/guides/remote-runners.en.md).

---

## Repository layout

```
openhub/
├── agents/              <- Agent definitions (20 agents, 7 families)
├── skills/              <- Protocols: Bucket A (inline) + Bucket B (on demand)
├── workflows/           <- Shipped workflows (oh/v1) + prompt templates
├── permissions/         <- Permission bases of the agents
├── cli/                 <- Go binary (oh)
│   ├── cmd/             <- Cobra commands and TUI wiring (oh-bd: fake bd of containers)
│   └── internal/
│       ├── workflow/      <- oh/v1 schema, layered resolution, validation, prompts
│       ├── services/      <- Services shared by CLI and TUI (workflow, session, checkpoint, remote)
│       ├── bundle/        <- Session bundles (~/.oh/bundles/<hash>/)
│       ├── bricks/        <- Agents, skills, permissions, model cascade
│       ├── sessionspec/   <- Neutral model (SessionSpec, BundleSpec)
│       ├── adapters/      <- Tool adapters (opencode V2: render, server, Attest, plugin)
│       ├── runsvc/        <- Launcher: server groups, sessions, worktrees, sleep, resume
│       ├── daemon/        <- ohd daemon: supervision, decisions, streams, notifications
│       ├── credproxy/     <- LLM credential proxy
│       ├── gateway/       <- Beads and MCP gateways
│       ├── runtime/       <- Execution environments (local, container)
│       ├── remote/        <- Machine ↔ GitLab CI job contract, generated pipeline
│       ├── limits/        <- Session limits (I6)
│       ├── teamstate/     <- team-state repository (claims, workflows, catalogue, solo)
│       ├── mcp/           <- Native MCP servers (figma, gitlab, gslides, github, jira, linear, team, workflow)
│       ├── deploycleanup/ <- Cleanup of former deployments
│       ├── storage/       <- SQLite (oh.db), keychain, file encryption
│       ├── tui/           <- tview/tcell TUI
│       └── ...            <- beads, config, i18n, prompt, tracker, worktree, termlaunch, selfupdate…
└── docs/                <- Documentation (bilingual fr/en)
```

User data lives in `~/.oh/` (outside the repository): `hub.toml`, `oh.db`, extracted hub content, session bundles, sessions, server groups, solo spaces, community skills. See [Architecture overview](docs/architecture/overview.en.md).

---

## MCP servers

Built-in MCP servers, running natively in Go (stdio, `oh mcp serve <name>`). Those enabled for the project are put in the session bundle; a workflow can filter them (`mcp:`).

| Server | Purpose | Requires |
|--------|---------|----------|
| `figma` | Design token extraction, component analysis | `FIGMA_TOKEN` |
| `gitlab` | Issues, MRs, pipelines | `GITLAB_TOKEN` |
| `gslides` | Presentation analysis | Google credentials |
| `github` | Issues, PRs, Actions | `GITHUB_TOKEN` |
| `jira` | Issues, transitions | `JIRA_URL` + `JIRA_TOKEN` |
| `linear` | Issues/mutations via GraphQL | `LINEAR_API_KEY` |
| `team` | Team coordination, claims, wiki | team-state repository |
| `workflow` | Workflow status, checkpoints, typed outputs (added to every session) | — |

Configure them with `oh mcp setup` (tokens stored in the OS keychain; the server reads its token itself). Outside the machine (container), oh MCP servers run on the machine behind the daemon's MCP gateway.

---

## Documentation

| Topic | Documents |
|-------|-----------|
| Start | [Getting started](docs/guides/getting-started.en.md) · [Tutorial](docs/guides/tutorial.en.md) · [Migrating to oh v5](docs/guides/migration-v5.en.md) |
| Sessions | [v5 Sessions](docs/guides/sessions-v5.en.md) · [Workflows (scenarios)](docs/guides/workflows.en.md) · [Review & Feedback](docs/guides/review-feedback.en.md) |
| Workflows | [Shipped workflows](docs/reference/workflows.en.md) · [Workflow schema](docs/reference/workflow-schema.en.md) · [Team workflows](docs/guides/team-workflows.en.md) |
| Execution | [Container runtime](docs/guides/container.en.md) · [Remote runners](docs/guides/remote-runners.en.md) |
| Architecture | [Overview](docs/architecture/overview.en.md) · [Agents](docs/architecture/agents.en.md) · [Skills](docs/architecture/skills.en.md) · [ADRs](docs/architecture/adr/) (48) |
| Reference | [CLI](docs/reference/cli.en.md) · [Configuration](docs/reference/config.en.md) · [Glossary](docs/reference/glossary.en.md) · [Beads model](docs/reference/beads-model.en.md) |
| Operations | [Troubleshooting](docs/guides/troubleshooting.en.md) · [Providers](docs/guides/providers.en.md) · [Backup & Restore](docs/guides/backup-restore.en.md) · [Dashboard](docs/guides/dashboard.en.md) |

Full index: [docs/README.md](docs/README.md).

---

## Migration

- **From oh v4** (opencode V1, `oh deploy`): [Migrating to oh v5](docs/guides/migration-v5.en.md). Leftovers of former deployments: `oh migrate deploy-cleanup`.
- **From the bash CLI `oc`**: [Migration Guide](MIGRATION.md) (command equivalence, hub.json → hub.toml).

---

## Requirements

- **[opencode](https://opencode.ai) V2** (2.0.0 or later), installed with its own tool
- **[git](https://git-scm.com/)**
- **[Beads](https://beads.sh/)** *(optional)*: ticket tracker for `oh run ticket`, `oh board`
- **Colima, Podman or Docker CLI** *(optional)*: container sessions
- **GitLab with CI runners** *(optional)*: remote sessions

No Node.js, jq, sqlite3 or bun required. The Go binary is self-contained.

**Platforms:** macOS and Linux (amd64, arm64); Windows (amd64, arm64) in local mode only (daemon inside the oh process, no container or remote).

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, conventions and PR process.

## Security

See [SECURITY.md](SECURITY.md) for vulnerability reporting and the security model (credential proxy, daemon, closed world, gateways, container and remote).

---

## License

MIT
