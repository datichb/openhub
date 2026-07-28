# CLI Reference

Complete reference for the `oh` CLI — the Go binary powering OpenCode Hub.

```
oh <command> [subcommand] [flags] [arguments]
```

## Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--verbose` | `-v` | Enable verbose output |

---

## Sessions

### oh start

Launch an opencode coding session.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--agent` | `-a` | string | Agent to use |
| `--prompt` | `-m` | string | Initial prompt |
| `--provider` | `-P` | string | LLM provider (bedrock, anthropic, openai) |
| `--project` | `-p` | string | Project ID (auto-detected otherwise) |
| `--resume` | `-r` | string | Resume an existing session (session ID) |
| `--worktree` | `-w` | string | Branch to launch in a git worktree |
| `--dev` | | bool | Dev mode: epic/ticket picker + orchestrator-dev. If the selected ticket is already claimed as `planned`, it automatically transitions to `in_progress` at session start. |
| `--label` | `-l` | string | Filter tickets by label (requires --dev) |
| `--assignee` | `-A` | string | Filter tickets by assignee (requires --dev) |
| `--onboard` | | bool | Onboarding mode: creates/enriches project wiki |
| `--refresh` | | bool | Force wiki re-discovery (requires --onboard) |
| `--yes` | `-y` | bool | Skip confirmation and launch immediately |

```bash
oh start -p my-app -m "Fix the login bug"
oh start --resume abc123-session-id
oh start -w feature/auth -a architect
oh start --dev -l "priority:high" -A me
oh start --onboard --refresh
oh start -m "Refactor the auth module" -y
```

---

### oh quick

Auto-detects project from cwd and launches opencode directly. No flags, no prompts.

```bash
cd ~/projects/my-app
oh quick
```

---

### oh audit

Run an automated audit on a project.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Project ID |
| `--type` | `-t` | string | Audit type (security, performance, architecture, accessibility, ecodesign, observability, privacy). Default: security |

```bash
oh audit -p my-app
oh audit -p my-app -t performance
oh audit --type accessibility
```

---

### oh review

Launch an automated code review session with mode selection.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Project ID |
| `--mode` | `-m` | string | Review mode (see below) |

**Available modes:**

| Mode | Description |
|------|-------------|
| `standard` | Classical 6-category checklist review |
| `adversarial` | Critical review — maximum skepticism, min. 10 findings, dangerous assumptions |
| `edge-case` | Exhaustive unhandled execution path hunting |
| `standard+adversarial` | Both modes in parallel (independent sessions) + unified report |
| `all` | Standard + Adversarial + Edge-case — maximum coverage |

Without `--mode`, an interactive prompt lets you choose the review mode at session start.

```bash
oh review -p my-app
oh review -m adversarial
oh review -m standard+adversarial -p backend
oh review -m all
```

---

### oh debug

Start a debugging session with AI assistance.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Project ID |
| `--issue` | `-i` | string | Issue description |

```bash
oh debug -p my-app -i "Users get 500 on /api/auth/callback"
oh debug --issue "Memory leak in worker process"
```

---

### oh beads

Proxy to `bd` (Beads CLI). All arguments are passed through directly. Requires `bd` installed.

```bash
oh beads list
oh beads run my-bead
oh beads --help
```

---

## Projects

### oh project list

List registered projects. Aliases: `ls`

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--status` | `-s` | string | Filter by status (active, archived) |
| `--json` | | bool | Output in JSON format |

```bash
oh project list
oh project ls -s active
oh project list --json
```

---

### oh project add

Register a new project. Aliases: `register`

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--name` | `-n` | string | Project name |
| `--path` | `-d` | string | Project path (default: cwd) |
| `--language` | `-l` | string | Main language |
| `--tracker` | `-t` | string | Issue tracker (github, gitlab, jira, linear) |

```bash
oh project add -n my-app -l typescript -t github
oh project add --path ~/projects/api --name backend
oh project register
```

---

### oh project remove

Remove a registered project. Aliases: `rm`

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--force` | `-f` | bool | Skip confirmation |

```bash
oh project remove my-app
oh project rm my-app -f
```

---

### oh project rename

Rename a project. Interactive if args omitted.

```bash
oh project rename my-app new-name
oh project rename
```

---

### oh project move

Move a project to a new path. Interactive if args omitted.

```bash
oh project move my-app ~/new-location
oh project move
```

---

### oh project configure

Configure project settings. Interactive if args omitted.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--provider` | `-P` | string | LLM provider |
| `--model` | `-m` | string | LLM model |
| `--language` | `-l` | string | Main language |
| `--tracker` | `-t` | string | Issue tracker |

```bash
oh project configure my-app --provider anthropic --model claude-sonnet-4-20250514
oh project configure my-app -l go -t gitlab
oh project configure
```

---

## Deployment

### oh deploy

Deploy agents, skills, and configuration to a project.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Project ID |
| `--provider` | `-P` | string | Provider to configure |
| `--model` | `-m` | string | Model to configure |
| `--check` | | bool | Check if agents/skills changed since last deploy |
| `--diff` | | bool | Show changes without applying |

```bash
oh deploy -p my-app
oh deploy --check
oh deploy --diff
oh deploy -p my-app -P anthropic -m claude-sonnet-4-20250514
```

---

### oh sync

Synchronize project configuration with remote state.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Project ID |
| `--all` | | bool | Sync all active projects |
| `--dry-run` | | bool | Show changes without applying |

```bash
oh sync -p my-app
oh sync --all
oh sync --dry-run
```

---

## Configuration

### oh config list

List all configuration values. Aliases: `ls`

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Output in JSON format |

```bash
oh config list
oh config ls --json
```

---

### oh config get

Get a configuration value.

```bash
oh config get default_provider
oh config get language
```

---

### oh config set

Set a configuration value.

```bash
oh config set default_provider anthropic
oh config set language en
```

---

### oh config unset

Remove a configuration value.

```bash
oh config unset default_provider
```

---

### oh config path

Print the configuration file path.

```bash
oh config path
```

---

### oh config language

Set or display the interface language.

```bash
oh config language fr
oh config language en
oh config language
```

---

### oh config websearch

Enable, disable, or check web search status.

```bash
oh config websearch enable
oh config websearch disable
oh config websearch status
```

---

## Infrastructure

### oh init

First-time setup wizard. Configures language, opencode, project, MCP servers, and deploy targets interactively.

```bash
oh init
```

---

### oh doctor

Run diagnostic checks on the environment. Checks: OS, git, opencode, bd, fzf, compatibility, config, database, API keys. Also checks for available `oh` binary updates.

```bash
oh doctor
```

---

### oh status

Display current environment status.

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Output in JSON format |

```bash
oh status
oh status --json
```

---

### oh upgrade opencode

Upgrade opencode to the latest (or specified) version.

```bash
oh upgrade opencode
oh upgrade opencode 0.2.15
```

> See also: `oh upgrade oh` to update the `oh` binary itself.

---

### oh upgrade oh

Update the `oh` binary in-place (atomic replacement). For non-Homebrew installs only.

```
oh upgrade oh [version] [--check]
```

| Flag | Type | Description |
|------|------|-------------|
| `--check` | bool | Verify available version without downloading |
| `version` | string | Target version (default: latest) |

```bash
oh upgrade oh
oh upgrade oh 1.3.0
oh upgrade oh --check
```

> **Homebrew users:** use `brew upgrade openhub` instead.

---

### oh mcp enable

Enable an MCP service at hub level or for a specific project.

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Project name or ID |

```bash
oh mcp enable figma
oh mcp enable gitlab --project my-project
```

---

### oh mcp disable

Disable an MCP service at hub level or for a specific project.

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Project name or ID |

```bash
oh mcp disable figma
oh mcp disable gitlab --project my-project
```

---

### oh mcp reset

Remove the project-level override for an MCP service (revert to hub config).

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | **(required)** Project name or ID |

```bash
oh mcp reset figma --project my-project
```

---

### oh mcp setup

Interactive wizard to configure an MCP service (token, options).

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Project name or ID |

```bash
oh mcp setup
oh mcp setup --project my-project
```

---

### oh mcp status

Display the status of all MCP services.

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Project name or ID (shows effective config) |

```bash
oh mcp status
oh mcp status --project my-project
```

---

### oh mcp serve

Serve a built-in MCP server via stdio.

```bash
oh mcp serve figma
oh mcp serve gitlab
oh mcp serve gslides
oh mcp serve team
oh mcp serve github
oh mcp serve jira
oh mcp serve linear
```

---

### oh mcp list

List available MCP servers. Aliases: `ls`

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Output in JSON format |

```bash
oh mcp list
oh mcp ls --json
```

---

### oh service setup (deprecated)

> **Deprecated:** Use `oh mcp setup` instead.

Interactive wizard to configure MCP service tokens in keychain.

```bash
oh service setup
```

---

### oh service remove (deprecated)

> **Deprecated:** Use `oh mcp disable` instead.

Remove a configured service.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--force` | `-f` | bool | Skip confirmation |

```bash
oh service remove gitlab
oh service remove figma -f
```

---

### oh plugin install

Install a plugin by name.

```bash
oh plugin install my-plugin
```

---

### oh plugin remove

Remove an installed plugin. Aliases: `rm`, `uninstall`

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--force` | `-f` | bool | Skip confirmation |

```bash
oh plugin remove my-plugin
oh plugin rm my-plugin -f
```

---

### oh plugin list

List installed plugins. Aliases: `ls`

```bash
oh plugin list
oh plugin ls
```

---

### oh plugin status

Show status of all installed plugins.

```bash
oh plugin status
```

---

### oh export

Create a backup archive of the hub data.

```
oh export [--output <path>]
```

Creates a `.tar.gz` archive containing: `oh.db`, `hub.toml`, `secrets.enc` (encrypted). Includes a SHA-256 checksum file.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--output` | `-o` | string | Output path (default: `./oh-backup-YYYY-MM-DD.tar.gz`) |

```bash
oh export
oh export --output ~/backups/oh-backup.tar.gz
```

---

### oh import

Restore from a backup archive.

```
oh import <file> [--overwrite] [--merge]
```

Verifies SHA-256 checksum before writing any data.

| Flag | Type | Description |
|------|------|-------------|
| `--overwrite` | bool | Overwrite existing data |
| `--merge` | bool | Merge projects only (non-destructive) |

```bash
oh import oh-backup-2025-07-22.tar.gz
oh import ~/backups/oh-backup.tar.gz --overwrite
oh import ~/backups/oh-backup.tar.gz --merge
```

---

### oh repair

Diagnose and repair the SQLite database.

```
oh repair [--check-only] [--auto]
```

Runs `PRAGMA integrity_check` on `~/.oh/oh.db`. If corruption is detected, presents recovery options: restore from backup, reinitialize database, or manually re-register projects. Also displays the current schema version.

| Flag | Type | Description |
|------|------|-------------|
| `--check-only` | bool | Diagnose without making changes |
| `--auto` | bool | Non-interactive mode |

```bash
oh repair
oh repair --check-only
oh repair --auto
```

---

### oh serve

Start a local web dashboard.

```
oh serve [--port 8080] [--readonly]
```

Starts an HTTP server bound to `127.0.0.1` only (never exposed to the network). Dashboard shows projects, sessions, and agent telemetry.

**API endpoints:**
- `GET /api/v1/projects`
- `GET /api/v1/sessions`
- `GET /api/v1/metrics/agents`

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--port` | `-p` | int | Port (default: 8080) |
| `--readonly` | | bool | Disable write endpoints (default: true) |

```bash
oh serve
oh serve --port 9090
oh serve --readonly=false
```

> **Security:** The server binds to `127.0.0.1` only and is never accessible from the network.

---

---

## Skills Marketplace

### oh skill add

Install a skill from a source (local path, git URL, or registry name).

```
oh skill add <source>
```

```bash
oh skill add rtk
oh skill add https://github.com/org/my-skill
oh skill add ./local-skill-dir
```

---

### oh skill list

List installed skills. Aliases: `ls`

```
oh skill list
oh skill ls
```

---

### oh skill remove

Remove an installed skill. Aliases: `rm`

```
oh skill remove <name>
oh skill rm <name>
```

---

### oh skill search

Search the skills registry.

```
oh skill search [query]
```

```bash
oh skill search
oh skill search react
oh skill search "code review"
```

---

## Git Worktree

### oh worktree list

List active worktrees. Aliases: `ls`

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Output in JSON format |

```bash
oh worktree list
oh worktree ls --json
```

---

### oh worktree add

Create a new worktree. Interactive if branch omitted.

```bash
oh worktree add feature/new-auth
oh worktree add
```

---

### oh worktree remove

Remove a worktree. Aliases: `rm`

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--force` | `-f` | bool | Force removal |

```bash
oh worktree remove ./worktrees/feature-auth
oh worktree rm ./worktrees/old-branch -f
```

---

### oh worktree cleanup

Remove worktrees for merged branches.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--base` | `-b` | string | Base branch for detection (default: auto-detect) |
| `--force` | `-f` | bool | Skip confirmation |

```bash
oh worktree cleanup
oh worktree cleanup -b main -f
```

---

## Analytics

### oh claim

Claim a ticket and create a claim entry in the team database.

```
oh claim <ticket-id> [options]
```

| Flag | Type | Description |
|------|------|-------------|
| `--planned` | bool | Create the claim in `planned` status (TODO column) instead of starting immediately in `in_progress` |

Without `--planned`, the claim is created directly in `in_progress` status.

```bash
oh claim TICKET-123
oh claim TICKET-123 --planned
```

---

### oh team status

Display team state: active claims, members, in-progress tickets.

```bash
oh team status
```

---

### oh team activity

Display team activity history.

```bash
oh team activity
```

---

### oh team board

Display the team kanban board (claims by status).

| Flag | Type | Description |
|------|------|-------------|
| `--watch` | bool | Auto-refresh every 5s |

```bash
oh team board
oh team board --watch
```

---

### oh team sync-tracker

Synchronize claims with the external tracker (GitLab/Jira).

```bash
oh team sync-tracker
```

Pulls issue states from the tracker, updates claim statuses, mirrors labels, and auto-creates planned claims for assigned issues not yet claimed. Uses configuration from `team-state/config.toml` and hub MCP config.

No flags. Configuration is read from `team-state/config.toml` and the hub MCP config.

---

## Analytics

### oh metrics

Display project metrics, session statistics, and agent telemetry.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--period` | `-d` | string | Analysis period (7d, 30d, all). Default: all |

```bash
oh metrics
oh metrics -d 7d
oh metrics --period 30d
```

---

### oh dashboard

Interactive TUI dashboard showing project and session overview.

```bash
oh dashboard
```

---

### oh board

Display a compact board view of active sessions.

| Flag | Type | Description |
|------|------|-------------|
| `--watch` | bool | Auto-refresh every 5s |

```bash
oh board
oh board --watch
```

---

## Utilities

### oh version

Print the oh CLI version.

```bash
oh version
```

---

### oh completion

Generate shell completion scripts.

```bash
oh completion bash
oh completion zsh
oh completion fish
oh completion powershell

# Install for current shell (zsh example):
oh completion zsh > ~/.oh-completion.zsh
echo "source ~/.oh-completion.zsh" >> ~/.zshrc
```
