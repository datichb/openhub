# Documentation Index

Complete index of all openhub documentation (oh v5). Guides and references are bilingual (EN/FR): each `x.en.md` page has its `x.fr.md` counterpart.

**Project governance:** [CONTRIBUTING.md](../CONTRIBUTING.md) | [SECURITY.md](../SECURITY.md) | [CHANGELOG.md](../CHANGELOG.md)

**New to v5?** Start with [Migrating to oh v5](guides/migration-v5.en.md), then [v5 Sessions](guides/sessions-v5.en.md) and the [architecture overview](architecture/overview.en.md).

---

## Guides (40 guides)

### Getting Started

| Guide | Description |
|-------|-------------|
| [Getting Started](guides/getting-started.en.md) | Installation (oh + opencode V2), first session |
| [Tutorial](guides/tutorial.en.md) | Step-by-step tutorial |
| [Migrating to v5](guides/migration-v5.en.md) | opencode V2 required, no more deployment in projects, former commands as aliases of `oh run`, free session `libre` |
| [Configuration Guide](guides/configuration-guide.en.md) | Detailed configuration options |
| [LLM Providers](guides/providers.en.md) | Anthropic, Bedrock, OpenRouter, Ollama |
| [Onboarding](guides/onboarding.en.md) | Using the onboarder agent (`oh run onboarding`) |
| [TUI Usage](guides/tui-usage.en.md) | TUI shell navigation and commands |

### Workflows and Sessions

| Guide | Description |
|-------|-------------|
| [Workflows](guides/workflows.en.md) | Usage scenarios: feature, ticket, audit, debug, review… |
| [v5 Sessions](guides/sessions-v5.en.md) | Sessions driven from oh: opening, decisions, sleep, resume, results, LLM keys |
| [Team Workflows](guides/team-workflows.en.md) | Team and project workflows in team-state: drafts, publication, history, locks, solo space |
| [Container runtime](guides/container.en.md) | Sessions in a container (Colima, Podman, Docker CLI): project dev image, settings, secrets |
| [Remote runners](guides/remote-runners.en.md) | Remote sessions on GitLab CI: runners, `oh-runner` project, `oh remote setup`, fetch and resolve |
| [Review & Feedback](guides/review-feedback.en.md) | AI code review, MR publication, feedback processing |
| [Parallel Mode](guides/parallel-mode.en.md) | **Replaced in v5** by `oh run ticket --tickets a,b` |
| [Sweep Mode](guides/sweep-mode.en.md) | **Replaced in v5** by the `sweep` workflow |

### Integrations (MCP)

| Guide | Description |
|-------|-------------|
| [Figma](guides/figma-integration.en.md) | MCP Figma setup and usage |
| [GitLab](guides/gitlab-integration.en.md) | MCP GitLab setup |
| [GitHub](guides/github-integration.en.md) | MCP GitHub setup |
| [Jira](guides/jira-integration.en.md) | MCP Jira setup |
| [Linear](guides/linear-integration.en.md) | MCP Linear setup |
| [Google Slides](guides/gslides-integration.en.md) | MCP Google Slides setup |
| [WebSearch](guides/websearch-integration.en.md) | WebSearch setup and best practices |
| [WebSearch Examples](guides/websearch-usage-examples.en.md) | WebSearch usage examples |

### Team and Collaboration

| Guide | Description |
|-------|-------------|
| [Team Setup](guides/team-setup.en.md) | Initialize and configure a team (or a solo space) |
| [Team Conventions](guides/team-conventions.en.md) | Team-wide conventions and policies |
| [Team Testing Guide](guides/team-testing-guide.en.md) | Testing team features end-to-end |
| [Notifications](guides/notifications.en.md) | Slack, Discord, Mattermost, Teams notifications |

### Agents and Skills

| Guide | Description |
|-------|-------------|
| [Authoring Agents](guides/authoring.en.md) | Create custom agents |
| [Authoring Skills](guides/authoring-skills.en.md) | Create custom skills |
| [Skill Marketplace](guides/skill-marketplace.en.md) | **Historical (disconnected in v5, ADR-051)**: former community skills registry; skills now come from the hub and the team catalogue |
| [Pathfinder Agent](guides/agent-pathfinder.en.md) | Using the pathfinder agent |
| [Inter-Agent Interruption](guides/inter-agent-interruption.en.md) | Agent interruption protocol |
| [External Agents](guides/external-agents.en.md) | **Historical (removed in v5)**: per-project agents of `oh deploy`; agents now come from the workflow |

### Plugins

In v5, plugins are declared per workflow (`plugins:`) and delivered in the session bundle: see [Shipped Workflows](reference/workflows.en.md).

| Guide | Description |
|-------|-------------|
| [RTK Plugin](guides/rtk-plugin-installation.en.md) | **Replaced in v5**: `oh plugin` (global V1 plugins) removed |
| [Context Mode Plugin](guides/context-mode-plugin.en.md) | **Historical (v5)**: installed through `oh plugin`; does not load under opencode V2 |

### Operations

| Guide | Description |
|-------|-------------|
| [Troubleshooting](guides/troubleshooting.en.md) | `oh doctor`, `oh repair`, common errors |
| [Backup & Restore](guides/backup-restore.en.md) | Export/import, database repair |
| [Dashboard](guides/dashboard.en.md) | Web dashboard setup |
| [Contributing](guides/contributing.en.md) | How to contribute (code, agents, skills) |

### Migration

| Guide | Description |
|-------|-------------|
| [Migrating to v5](guides/migration-v5.en.md) | From oh v4 (opencode V1, `oh deploy`) to oh v5 |
| [Designer Fusion Migration](guides/migration-designer-fusion.en.md) | **Historical (v5)**: ux-designer/ui-designer merged into designer (before v5) |
| [Migration from `oc`](../MIGRATION.md) | From the former bash CLI |

---

## Reference (27 docs)

| Document | Description |
|----------|-------------|
| [CLI Reference (Index)](reference/cli.en.md) | Table of contents, global flags, exit codes |
| [CLI — Workflows](reference/cli-workflows.en.md) | `oh run`, `oh workflow *`, `oh bundle *` |
| [CLI — Sessions](reference/cli-sessions.en.md) | `oh session *`, `oh budget`, `oh history`, `oh beads`, deprecated aliases (`oh start`, `oh audit`, `oh review`, `oh debug`) |
| [CLI — Projects](reference/cli-projects.en.md) | `oh project list\|add\|remove\|rename\|move\|configure` |
| [CLI — Deployment](reference/cli-deploy.en.md) | `oh deploy`, `oh sync` (removed in v5), `oh migrate deploy-cleanup` |
| [CLI — Configuration](reference/cli-config.en.md) | `oh config *`, `oh provider setup`, `oh config model` |
| [CLI — Infrastructure](reference/cli-infra.en.md) | `oh init`, `oh doctor`, `oh status`, `oh migrate deploy-cleanup`, `oh upgrade oh`, `oh export`/`import`, `oh repair`, `oh purge`, `oh daemon`, `oh remote`, `oh serve` |
| [CLI — MCP](reference/cli-mcp.en.md) | `oh mcp *` (`oh plugin` removed in v5) |
| [CLI — Team](reference/cli-team.en.md) | `oh team *` (incl. `init --solo`, `promote`), `oh teams *`, `oh conventions`, `oh patterns`, `oh policies`, `oh takeover-brief` |
| [CLI — Tools](reference/cli-tools.en.md) | `oh skill *`, `oh worktree *`, `oh secrets *`, `oh metrics` |
| [Shipped Workflows](reference/workflows.en.md) | The 12 `oh/v1` workflows shipped by the hub: entry agents, inputs, checkpoints, prompt templates |
| [Workflow Schema](reference/workflow-schema.en.md) | `apiVersion: oh/v1` schema: fields, validation, layers, `extends`, `enforce` |
| [Configuration](reference/config.en.md) | hub.toml, project settings, team config |
| [Beads Model](reference/beads-model.en.md) | Ticket system data model |
| [Glossary](reference/glossary.en.md) | Terms and definitions |
| [Model Resolution](reference/model-resolution.en.md) | LLM model selection cascade |
| [Services](reference/services.en.md) | Internal services reference |
| [TUI Reference](reference/tui.en.md) | TUI keybindings and views |
| [TUI Inline Wizard](reference/tui-inline-wizard.en.md) | Wizard engine reference |
| [Audit Tools](reference/audit-tools.en.md) | Audit tool reference |

### MCP Server References

| Document | Description |
|----------|-------------|
| [MCP Figma](reference/mcp-figma.en.md) | Figma MCP tools |
| [MCP GitLab](reference/mcp-gitlab.en.md) | GitLab MCP tools |
| [MCP GitHub](reference/mcp-github.en.md) | GitHub MCP tools |
| [MCP Jira](reference/mcp-jira.en.md) | Jira MCP tools |
| [MCP Linear](reference/mcp-linear.en.md) | Linear MCP tools |
| [MCP Google Slides](reference/mcp-gslides.en.md) | Google Slides MCP tools |
| [MCP Team](reference/mcp-team.en.md) | Team state MCP tools |

The `workflow` MCP server (`workflow_status`, `workflow_checkpoint`, `workflow_outputs`) is added to every session bundle: see [Shipped Workflows](reference/workflows.en.md) and [ADR-042](architecture/adr/042-checkpoints-headless-decisions.en.md).

---

## Architecture (6 docs + 51 ADRs)

| Document | Description |
|----------|-------------|
| [Overview](architecture/overview.en.md) | v5 concepts: workflow → session bundle → group server → session, daemon, credential proxy, gateways, closed world |
| [Agents](architecture/agents.en.md) | The 20 agents (7 families, including `conductor`) |
| [Skills](architecture/skills.en.md) | Hybrid skill system (Bucket A/B), delivery through the session bundle |
| [Task Delegation](architecture/task-delegation.en.md) | Inter-agent delegation model (`calls:`, `after:`) |
| [Living Wiki](architecture/living-wiki.en.md) | Living documentation wiki architecture |
| [TodoWrite Isolation](architecture/todowrite-session-isolation.en.md) | Session isolation design |
| [ADRs](architecture/adr/) | 51 Architecture Decision Records (v5: 038 to 051) |

### v5 decisions

| ADR | Decision |
|-----|----------|
| [038](architecture/adr/038-sessionspec-tool-adapters.en.md) | `SessionSpec` neutral model and tool adapters |
| [039](architecture/adr/039-declarative-workflows-oh-v1.en.md) | Declarative `oh/v1` workflows |
| [040](architecture/adr/040-workflows-team-state-governance.en.md) | Workflows in team-state, governance, solo space |
| [041](architecture/adr/041-closed-world-isolation.en.md) | Closed world and isolation check (`Attest`) |
| [042](architecture/adr/042-checkpoints-headless-decisions.en.md) | Three-level checkpoints and headless decisions |
| [043](architecture/adr/043-session-bundle-deploy-removal.en.md) | Session bundle and removal of per-project deployment |
| [044](architecture/adr/044-credential-proxy-session-limits.en.md) | LLM credential proxy and session limits (I6) |
| [045](architecture/adr/045-execution-environments.en.md) | Execution environments: local, container, remote |
| [046](architecture/adr/046-beads-gateways.en.md) | Beads on the machine and gateways |
| [047](architecture/adr/047-session-interaction-daemon.en.md) | Session interaction, multi-session, `ohd` daemon |
| [048](architecture/adr/048-opencode-v1-abandonment.en.md) | opencode V1 abandonment |
| [049](architecture/adr/049-tool-independence-architecture-guard.en.md) | Tool independence and architecture guard |
| [050](architecture/adr/050-session-context-capability.en.md) | Evolving session state through an adapter capability |
| [051](architecture/adr/051-community-skills-disconnection.en.md) | Community skills registry disconnected |

---

## Diagrams

Sources (`.mermaid`) and rendered SVG files live in [`diagrams/`](diagrams/).

| Diagram | Description |
|---------|-------------|
| [System overview](diagrams/system-overview.svg) | oh (control tower), daemon, group servers, opencode (cockpit) |
| [Session lifecycle](diagrams/session-lifecycle.svg) | Launch, bundle, `Attest`, work, decisions, sleep, resume, stop |
| [Execution topology](diagrams/execution-topology.svg) | Local, container and remote: what runs where, what stays on the machine |
| [Workflow scenarios map](diagrams/workflow-scenarios-map.svg) | Which workflow for which situation |
| [Workflow layers](diagrams/config-resolution.svg) | Hub < team < project < session options |
| [Model resolution cascade](diagrams/model-resolution-cascade.svg) | From workflow·agent down to frontmatter |
| [Skill delivery](diagrams/skill-injection-flow.svg) | Bucket A / Bucket B through the session bundle |
| [CLI command tree](diagrams/cli-command-tree.svg) | Main `oh` commands |
| [TUI navigation](diagrams/tui-navigation-modes.svg) | TUI views and omnibar |
| [Agent hierarchy](diagrams/agent-hierarchy.svg) | Agents and delegations |

---

## Design (2 docs)

| Document | Description |
|----------|-------------|
| [Aurum Design System](design/aurum.md) | Color palette, components, tokens |
| [TUI Shell Design](design/tui-shell.md) | Shell layout, navigation modes |

---

## Wiki (Living Documentation)

| Page | Description |
|------|-------------|
| [Wiki index](wiki/README.md) | Entry point of the project wiki |
| [Architecture](wiki/architecture.en.md) | System architecture overview |
| [Stack](wiki/stack.en.md) | Technology stack and dependencies |
| [Conventions](wiki/conventions.en.md) | Code style, commit format, review process |
| [Config Cascade](wiki/technical/config-cascade.en.md) | Configuration resolution across levels |

---

## Developer Internal (15 docs)

Internal development notes (mostly monolingual, not translated). Some describe the state before v5.

| Document | Description |
|----------|-------------|
| [Architecture](dev/architecture.md) | Internal architecture notes |
| [v5 Audit](dev/audit-v5.md) | v5 audit (security, design, ergonomics, functional, architecture) and correction plan (P0/P1/P2) |
| [CLI Migration Analysis](dev/cli-migration-analysis.md) | Bash-to-Go migration analysis |
| [CLI Migration Plan v2](dev/cli-migration-plan-v2.md) | Migration implementation plan |
| [CLI Remaining Work](dev/cli-remaining-work.md) | Post-migration remaining tasks |
| [Agent Reliability](dev/fiabilisation-agents-en-cours.md) | Agent reliability improvements |
| [File Locking](dev/file-locking.md) | File locking strategy |
| [Git Hooks](dev/git-hooks.md) | Optional pre-commit and commit-msg hooks |
| [Performance Optimizations](dev/performance-optimizations.md) | Performance notes |
| [Progress Bar](dev/progress-bar.md) | Progress bar implementation |
| [Roadmap Audit](dev/roadmap-audit.md) | Roadmap audit notes |
| [Shell Gotchas](dev/shell-gotchas.md) | Shell compatibility issues |
| [TUI Architecture](dev/tui-architecture.md) | TUI internal architecture |
| [TUI Testing](dev/tui-testing.md) | TUI test strategy |
