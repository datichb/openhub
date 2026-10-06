# Documentation Index

Complete index of all openhub documentation. Guides and references are bilingual (EN/FR).

**Project governance:** [CONTRIBUTING.md](../CONTRIBUTING.md) | [SECURITY.md](../SECURITY.md) | [CHANGELOG.md](../CHANGELOG.md)

---

## Guides (35 guides)

### Getting Started

| Guide | Description |
|-------|-------------|
| [Getting Started](guides/getting-started.en.md) | Installation, first deployment |
| [Tutorial](guides/tutorial.en.md) | Step-by-step tutorial |
| [Configuration Guide](guides/configuration-guide.en.md) | Detailed configuration options |
| [LLM Providers](guides/providers.en.md) | Anthropic, Bedrock, OpenRouter, Ollama |
| [Onboarding](guides/onboarding.en.md) | Using the onboarder agent |
| [TUI Usage](guides/tui-usage.en.md) | TUI shell navigation and commands |

### Workflows and Execution Modes

| Guide | Description |
|-------|-------------|
| [Workflows](guides/workflows.en.md) | Full feature, audit, debug scenarios |
| [Migrating to v5](guides/migration-v5.en.md) ([fr](guides/migration-v5.fr.md)) | opencode V2 required, former commands as aliases of `oh run`, free session `libre` |
| [v5 Sessions](guides/sessions-v5.en.md) | opencode V2 sessions: opening, sleep, resume, LLM keys |
| [Container runtime](guides/container.en.md) | Sessions in a container (Colima, Podman): project dev image, settings, secrets |
| [Remote runners](guides/remote-runners.en.md) ([fr](guides/remote-runners.fr.md)) | Remote sessions on GitLab CI: runners, oh-runner project, `oh remote setup` |
| [Parallel Mode](guides/parallel-mode.en.md) | Replaced in v5 by `oh run ticket --tickets a,b` |
| [Sweep Mode](guides/sweep-mode.en.md) | Replaced in v5 by the `sweep` workflow |
| [Review & Feedback](guides/review-feedback.en.md) | AI code review, MR publication, feedback processing |

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
| [Team Setup](guides/team-setup.en.md) | Initialize and configure a team |
| [Team Workflows](guides/team-workflows.en.md) | v5 workflows in team-state: layout, workflows.lock, locks |
| [Team Conventions](guides/team-conventions.en.md) | Team-wide conventions and policies |
| [Team Testing Guide](guides/team-testing-guide.en.md) | Testing team features end-to-end |
| [Notifications](guides/notifications.en.md) | Slack, Discord, Mattermost, Teams notifications |

### Agents and Skills

| Guide | Description |
|-------|-------------|
| [Authoring Agents](guides/authoring.en.md) | Create custom agents |
| [Authoring Skills](guides/authoring-skills.en.md) | Create custom skills |
| [External Agents](guides/external-agents.en.md) | Per-project agent customization |
| [Skill Marketplace](guides/skill-marketplace.en.md) | Install community skills |
| [Pathfinder Agent](guides/agent-pathfinder.en.md) | Using the pathfinder agent |
| [Inter-Agent Interruption](guides/inter-agent-interruption.en.md) | Agent interruption protocol |

### Plugins

| Guide | Description |
|-------|-------------|
| [RTK Plugin](guides/rtk-plugin-installation.en.md) | Removed in v5 (plugins are declared per workflow) |
| [Context Mode Plugin](guides/context-mode-plugin.en.md) | Context mode plugin setup |

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
| [Designer Fusion Migration](guides/migration-designer-fusion.en.md) | Migrating from ux-designer/ui-designer to designer |

---

## Reference (15 docs)

| Document | Description |
|----------|-------------|
| [CLI Reference (Index)](reference/cli.en.md) | Table of contents, global flags, exit codes |
| [CLI — Sessions](reference/cli-sessions.en.md) | `oh start`, `oh review`, `oh audit`, `oh debug` |
| [CLI — Projects](reference/cli-projects.en.md) | `oh project list\|add\|remove\|rename\|move\|configure` |
| [CLI — Deployment](reference/cli-deploy.en.md) | `oh deploy`, `oh sync` |
| [CLI — Configuration](reference/cli-config.en.md) | `oh config *`, `oh provider setup`, `oh config model` |
| [CLI — Infrastructure](reference/cli-infra.en.md) | `oh init`, `oh doctor`, `oh repair`, `oh upgrade`, `oh purge`, `oh serve` |
| [CLI — MCP & Plugins](reference/cli-mcp.en.md) | `oh mcp *`, `oh plugin *` |
| [CLI — Team](reference/cli-team.en.md) | `oh team *`, `oh teams *`, `oh conventions`, `oh policies` |
| [CLI — Tools](reference/cli-tools.en.md) | `oh skill *`, `oh worktree *`, `oh secrets *`, `oh metrics` |
| [Configuration](reference/config.en.md) | hub.toml, project settings, team config |
| [Beads Model](reference/beads-model.en.md) | Ticket system data model |
| [Glossary](reference/glossary.en.md) | Terms and definitions |
| [Model Resolution](reference/model-resolution.en.md) | LLM model selection cascade |
| [Shipped Workflows](reference/workflows.en.md) | `oh/v1` workflows shipped by the hub, inputs, checkpoints, prompt templates |
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

---

## Architecture (6 docs + 36 ADRs)

| Document | Description |
|----------|-------------|
| [Overview](architecture/overview.en.md) | Concepts, flow diagrams |
| [Agents](architecture/agents.en.md) | All 19 agents reference |
| [Skills](architecture/skills.en.md) | Hybrid skill system (Bucket A/B) |
| [Task Delegation](architecture/task-delegation.en.md) | Inter-agent delegation model |
| [Living Wiki](architecture/living-wiki.en.md) | Living documentation wiki architecture |
| [TodoWrite Isolation](architecture/todowrite-session-isolation.en.md) | Session isolation design |
| [ADRs](architecture/adr/) | 36 Architecture Decision Records |

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
| [Config Cascade](wiki/technical/config-cascade.en.md) | Configuration resolution across levels |
| [Conventions](wiki/conventions.en.md) | Code style, commit format, review process |
| [Architecture](wiki/architecture.en.md) | System architecture overview |
| [Stack](wiki/stack.en.md) | Technology stack and dependencies |

---

## Developer Internal (15 docs)

Internal development notes (monolingual, not translated).

| Document | Description |
|----------|-------------|
| [Architecture](dev/architecture.md) | Internal architecture notes |
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
