> [Lire en français](getting-started.fr.md)

# Getting Started

This guide covers installation, first-time setup, and daily usage of the `oh` CLI.

## Prerequisites

| Tool | Purpose | Required |
|------|---------|----------|
| **git** | Version control | Yes |
| **opencode** | AI coding agent | Auto-downloaded by `oh init` / `oh start` |
| **bd** | Beads ticket tracker | No (for `--dev` mode and `oh board`) |

No Node.js, jq, sqlite3, bun, or Python needed. The Go binary is self-contained.

## Installation

**Supported platforms:** macOS (darwin), Linux, and Windows — amd64 and arm64.

**Homebrew (recommended — macOS/Linux):**

```bash
brew install datichb/tap/openhub
```

**Curl script (macOS/Linux):**

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/openhub/main/install.sh | bash
```

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/datichb/openhub/main/install.ps1 | iex
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

## Deploy Agents & Skills

Deploy the shared agents, skills, and configuration into a project:

```bash
oh deploy                    # auto-detect project from cwd
oh deploy -p my-project      # explicit project
oh deploy --check            # verify if deploy is needed (exit code 1 if stale)
oh deploy --diff             # show what would change
```

This generates:

- `.opencode/agents/*.md` — agent definitions
- `.opencode/skills/*/SKILL.md` — skill protocols
- `opencode.json` — provider, model, MCP, permissions

## Start a Session

```bash
oh start                     # auto-detect project, show recap, confirm then launch
oh start -p my-project       # explicit project
oh start -a orchestrator     # use specific agent
oh start -m "explain..."     # with initial prompt
oh start --dev               # dev mode: pick epics/tickets
oh start --onboard           # create project wiki
oh start -y                  # skip confirmation prompt
oh start -r <session-id>     # resume a previous session
```

The start flow:

1. Resolves project (from cwd or `--project` flag)
2. Resolves provider and bearer token
3. Detects project stack (language/framework)
4. Displays a rich configuration recap
5. Waits for confirmation (Press Enter or `--yes` to skip)
6. Launches opencode

## Quick Start (no recap)

```bash
oh quick                     # auto-detect project, launch immediately
```

## Day-to-Day Commands

```bash
oh sync --all                # sync agents/skills to all projects
oh status                    # show hub and current project status
oh doctor                    # system health check (also verifies provider credentials and oh version)
oh provider setup            # configure provider credentials
oh metrics                   # per-agent usage and cost stats (sessions, tokens, avg duration)
oh serve                     # start local dashboard API + SPA on localhost
oh dashboard                 # interactive TUI dashboard
oh board                     # kanban board (requires bd)
oh team sync-tracker         # sync claim ExternalIID links back to the external tracker (Jira, Linear, GitLab Issues)
oh export                    # export all hub data (projects, sessions, secrets) to a backup file
oh repair                    # repair corrupted database or config state
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
- **MCP server errors** — verify tokens with `oh service setup --project <id>`
- **Project not detected** — ensure you're in a registered project directory (`oh project list`)
- **Corrupted state** — run `oh repair` to attempt automatic recovery
