> [Lire en francais](getting-started.fr.md)

# Getting Started

## What is OpenHub?

OpenHub (`oh`) is a central hub that manages AI coding assistants across your projects. It provides **19 specialized AI agents** organized into 7 families (planning, development, audit, quality, design, documentation, utility) that collaborate to handle everything from feature planning to code review.

```mermaid
flowchart LR
    U[You] -->|oh run| CLI[oh CLI]
    CLI -->|configures| OC[OpenCode Runtime]
    OC -->|calls| LLM[LLM Provider<br/>Anthropic / Bedrock / OpenRouter]
    CLI -.->|builds at launch| B[Session bundle<br/>~/.oh/bundles/hash/]
    B -.->|agents, skills,<br/>permissions, MCP| OC
    HUB[(~/.oh/<br/>Hub Config)] -->|agents, skills,<br/>config| CLI
    MCP[MCP Servers<br/>GitLab, Figma, Jira...] <-->|tools| OC
```

**Key concepts** (see the full [Glossary](../reference/glossary.en.md)):
- **Hub** (`~/.oh/`) -- central configuration and agent/skill store
- **Agent** -- a specialized AI role (orchestrator, developer, reviewer, etc.)
- **Skill** -- a protocol document giving domain expertise to an agent
- **Session bundle** -- agents, skills, permissions and MCP of a session, built at launch from its workflow outside the project (`~/.oh/bundles/<hash>/`); replaces deploy (removed in v5)
- **MCP Server** -- external tool integration (GitLab, Figma, Jira, etc.)

> **New here?** Start with the [5-minute tutorial](tutorial.en.md) for a hands-on walkthrough.

This guide is the comprehensive command reference. For configuration details, see the [Configuration guide](configuration-guide.en.md).

---

## Prerequisites

| Tool | Purpose | Required |
|------|---------|----------|
| **git** | Version control | Yes |
| **opencode** | AI coding agent | Auto-downloaded by `oh init` / `oh start` |
| **bd** | Beads ticket tracker | No (for `--dev` mode and `oh board`) |

No Node.js, jq, sqlite3, bun, or Python needed. The Go binary is self-contained.

## Installation

**Supported platforms:** macOS (darwin) and Linux — amd64 and arm64.

**Homebrew (recommended — macOS/Linux):**

```bash
brew install datichb/tap/openhub
```

**Curl script (macOS/Linux):**

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/openhub/main/install.sh | bash
```

**From source:**

```bash
cd cli && go install .
```

## First-time Setup

```bash
oh init
```

This interactive wizard in 3 steps will:

**[1/3] Hub Configuration:**
- Display a preamble with requirements (provider, MCP tokens)
- Ask your preferred language (fr/en)
- Ask opencode version to use (default: latest)
- Choose default LLM provider (Bedrock, Anthropic, OpenRouter, GitHub Copilot)
- Auto-detect existing credentials and offer to use them or configure new ones

**[2/3] MCP Servers (optional):**
- Offer to configure MCP services (Figma, GitLab, Google Slides)
- For each selected service: prompt for token and store in keychain
- Services without token are skipped (configurable later via `oh mcp setup`)

**[3/3] First Project (optional):**
- Offer to register a first project
- If yes: launches the project wizard (name, path, language, agents, MCP)
- If no: initialization complete (`oh project add` available later)

Hub content (agents and skills) is automatically extracted to `~/.oh/hub/` from the binary.

## Register a Project

If you have additional projects after init:

```bash
oh project add
```

Or non-interactively:

```bash
oh project add --name my-app --path ~/workspace/my-app --language typescript --tracker github
```

## Session Bundle

Nothing is deployed into the project anymore (`oh deploy` / `oh sync` removed in v5). Each session starts from a session bundle built at launch outside the project (`~/.oh/bundles/<hash>/`) from its workflow: agents, skills, permissions, MCP servers, provider and model. To inspect it:

```bash
oh bundle show <workflow>                  # auto-detect project from cwd
oh bundle show <workflow> -p my-project    # explicit project
oh bundle show <workflow> --budget         # include the context budget
oh bundle build <workflow>                 # build the bundle without launching
```

Projects deployed with an older version: clean up the leftovers (`.opencode/agents`, `.opencode/skills`, oh keys in `opencode.json`…) with `oh migrate deploy-cleanup --dry-run` then `oh migrate deploy-cleanup`.

## Start a Session

```bash
oh start                     # auto-detect project, show recap, confirm then launch
oh start -p my-project       # explicit project
oh start -a orchestrator     # use specific agent
oh start -m "explain..."     # with initial prompt
oh start --dev               # dev mode: pick epics/tickets
oh start --onboard           # create project wiki
oh start --recap              # show configuration recap + confirmation
oh start -r <session-id>     # resume a previous session
```

The start flow:

1. Resolves project (from cwd or `--project` flag)
2. Resolves provider and bearer token
3. Detects project stack (language/framework)
4. Displays a rich configuration recap
5. Launches directly (use `--recap` to show summary + confirmation)
6. Launches opencode

## Quick Start

```bash
oh start                     # auto-detect project, launch immediately
```

## Day-to-Day Commands

### Essential

```bash
oh start                     # launch AI session
oh bundle show <workflow>    # inspect the session bundle of a workflow
oh status                    # show hub and current project status
oh doctor                    # system health check
```

### Development

```bash
oh start --dev               # pick epic/ticket, launches orchestrator-dev
oh start --dev --label bug   # filter tickets by label
oh audit --type security     # code audit
oh review                    # code review
oh debug --issue "crash..."  # debug session
```

### Infrastructure

```bash
oh migrate deploy-cleanup    # remove leftovers of former deployments (oh < v5)
oh provider setup            # configure provider credentials
oh mcp setup                 # configure MCP server tokens
oh metrics                   # per-agent usage and cost stats
oh serve                     # start local web dashboard on localhost:8080
oh                           # interactive TUI dashboard (no arguments)
oh board                     # kanban board (requires bd)
oh export                    # export all hub data to a backup file
oh repair                    # repair corrupted database or config state
```

### Team (optional)

```bash
oh team init                 # set up team collaboration
oh team claim <ticket-id>    # claim a ticket for yourself
oh team release <ticket-id>  # release a claimed ticket
oh team status               # show team status
oh team sync-tracker         # sync claims back to external tracker
```

## Development Workflow

```bash
oh start --dev               # pick epic/ticket, launches orchestrator-dev
oh start --dev --label bug   # filter tickets by label
oh audit --type security     # code audit
oh review                    # code review
oh debug --issue "crash on login"  # debug session
```

## Worktree Management

```bash
oh start -w feature/login    # creates worktree and launches there
oh worktree list             # list active worktrees
oh worktree cleanup          # remove merged worktrees
```

## Configuration

```bash
oh config list               # show all config
oh config set opencode.default_provider anthropic
oh config language fr        # switch to French
oh config websearch enable   # enable web search for agents
```

## Community Skills

Install community skills from the index or a Git URL:

```bash
oh skill add <index-name>            # install from community index
oh skill add https://github.com/...  # install from Git URL
oh skill list                        # list installed community skills
oh skill search <query>              # search the community index
```

See [skills.en.md](../architecture/skills.en.md#community-skills-marketplace) for details.

## Upgrading

```bash
brew upgrade openhub          # upgrade oh itself (Homebrew)
oh upgrade oh                 # upgrade oh itself (non-Homebrew)
oh upgrade opencode          # upgrade the opencode binary
oh upgrade opencode 1.18.0   # pin a specific version
```

## Uninstalling

```bash
brew uninstall openhub
rm -rf ~/.oh                 # remove configuration and database
```

## Troubleshooting

Run diagnostics:

```bash
oh doctor
```

`oh doctor` checks:
- `oh` version (latest available vs installed)
- `opencode` binary presence and version
- Provider credentials
- MCP server connectivity
- Project registry integrity

Common issues:

- **opencode not found** — run `oh init` or `oh upgrade opencode`
- **Missing provider credentials** — run `oh provider setup`
- **MCP server errors** — verify tokens with `oh mcp setup`
- **Project not detected** — ensure you're in a registered project directory (`oh project list`)
- **Corrupted state** — run `oh repair` to attempt automatic recovery
