# CLI Reference

Complete reference for the `oh` CLI — the Go binary powering OpenCode Hub.

```
oh <command> [subcommand] [flags] [arguments]
```

Global flag: `--verbose` — Enable debug output.

---

## Session Commands

### oh init

First-time setup wizard. Configures language, opencode, project, MCP servers, bd, and deploy targets interactively.

```bash
oh init
```

### oh start

Launch an opencode coding session.

| Flag | Short | Description |
|------|-------|-------------|
| `--agent` | `-a` | Agent to use |
| `--prompt` | `-p` | Initial prompt |
| `--provider` | `-P` | LLM provider override |
| `--project` | `-j` | Project name |
| `--resume` | `-r` | Resume last session |
| `--worktree` | `-w` | Use a git worktree |
| `--dev` | | Development mode |
| `--label` | `-l` | Session label |
| `--assignee` | `-A` | Assign to user |
| `--onboard` | | Run onboarding sequence |
| `--refresh` | | Refresh project context |

```bash
oh start -j my-app -p "Fix the login bug"
oh start --resume
oh start -w feature/auth -a architect
```

### oh quick

Quick task with interactive project selection. Prompts for project if not in a registered directory.

```bash
oh quick
```

### oh audit

Run a code audit on the current or specified project.

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-j` | Project name |
| `--type` | `-t` | Audit type |

Available types: `security`, `performance`, `architecture`, `accessibility`, `ecodesign`, `observability`, `privacy`.

```bash
oh audit -j my-app -t security
oh audit --type performance
```

### oh review

Start a code review session.

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-j` | Project name |

```bash
oh review -j my-app
```

### oh debug

Start a debug session, optionally linked to a tracker issue.

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-j` | Project name |
| `--issue` | `-i` | Issue ID from tracker |

```bash
oh debug -j my-app -i 42
```

### oh conventions

Display project conventions (coding standards, naming, structure).

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-j` | Project name |

```bash
oh conventions -j my-app
```

### oh beads

Proxy to the `bd` CLI. All arguments are passed through directly.

```bash
oh beads list
oh beads run my-bead
```

---

## Project Commands

### oh project list

List registered projects. Alias: `ls`.

| Flag | Short | Description |
|------|-------|-------------|
| `--status` | `-s` | Filter by status |
| `--json` | | Output as JSON |

```bash
oh project list
oh project ls --json
oh project list -s active
```

### oh project add

Register a new project. Alias: `register`.

| Flag | Short | Description |
|------|-------|-------------|
| `--name` | `-n` | Project name |
| `--path` | `-p` | Path to project root |
| `--language` | `-l` | Primary language |
| `--tracker` | `-t` | Issue tracker URL |

```bash
oh project add -n my-app -p ./my-app -l typescript
```

### oh project remove

Remove a project from the hub. Alias: `rm`.

| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Skip confirmation |

```bash
oh project remove my-app
oh project rm my-app -f
```

### oh project rename

Rename a registered project.

```bash
oh project rename old-name new-name
```

### oh project move

Change the path of a registered project.

```bash
oh project move my-app /new/path/to/my-app
```

### oh project configure

Update project configuration.

| Flag | Short | Description |
|------|-------|-------------|
| `--provider` | `-P` | LLM provider |
| `--model` | `-m` | Model name |
| `--language` | `-l` | Primary language |
| `--tracker` | `-t` | Issue tracker URL |

```bash
oh project configure my-app -P anthropic -m claude-sonnet
```

---

## Config Commands

### oh config get

Get a configuration value by key.

```bash
oh config get default_provider
```

### oh config set

Set a configuration value.

```bash
oh config set default_provider anthropic
oh config set default_model claude-sonnet-4
```

### oh config unset

Remove a configuration key.

```bash
oh config unset default_model
```

### oh config list

List all configuration values. Alias: `ls`.

| Flag | Short | Description |
|------|-------|-------------|
| `--json` | | Output as JSON |

```bash
oh config list
oh config ls --json
```

### oh config path

Show the path to the configuration file.

```bash
oh config path
```

### oh config language

Show or change the interface language.

```bash
oh config language        # show current
oh config language fr     # switch to French
oh config language en     # switch to English
```

### oh config websearch

Manage WebSearch permissions for agents.

```bash
oh config websearch status
oh config websearch enable
oh config websearch disable
```

---

## Deploy Commands

### oh deploy

Deploy agents, skills, config, and MCP servers to a project.

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-j` | Project name |
| `--provider` | `-P` | Provider override |
| `--model` | `-m` | Model override |
| `--check` | | Dry-run validation only |
| `--diff` | | Show diff before applying |

```bash
oh deploy -j my-app
oh deploy -j my-app --check
oh deploy --diff
```

### oh sync

Synchronize project configurations from the hub.

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-j` | Target project |
| `--all` | | Sync all projects |
| `--dry-run` | | Preview changes only |

```bash
oh sync -j my-app
oh sync --all --dry-run
```

---

## Analytics Commands

### oh status

Show hub and project status overview.

| Flag | Short | Description |
|------|-------|-------------|
| `--json` | | Output as JSON |

```bash
oh status
oh status --json
```

### oh metrics

Display usage metrics and statistics.

| Flag | Short | Description |
|------|-------|-------------|
| `--period` | `-p` | Time period: `7d`, `30d`, `all` |

```bash
oh metrics
oh metrics -p 30d
```

### oh dashboard

Launch the interactive TUI dashboard.

```bash
oh dashboard
```

### oh board

Display a Kanban board of current tasks.

| Flag | Short | Description |
|------|-------|-------------|
| `--watch` | | Auto-refresh mode |

```bash
oh board
oh board --watch
```

### oh optimize

Show optimization suggestions for the current project or hub configuration.

```bash
oh optimize
```

### oh yield

Display session-commit yield (productivity metrics per session).

```bash
oh yield
```

---

## Infrastructure Commands

### oh doctor

Run a system health check. Validates dependencies, configuration, and connectivity.

```bash
oh doctor
```

### oh version

Print version information (binary version, build date, Go version).

```bash
oh version
```

### oh completion

Generate shell completion scripts.

```bash
oh completion bash
oh completion zsh
oh completion fish
oh completion powershell
```

To install completions:

```bash
# Bash
oh completion bash > /etc/bash_completion.d/oh

# Zsh
oh completion zsh > "${fpath[1]}/_oh"

# Fish
oh completion fish > ~/.config/fish/completions/oh.fish
```

### oh upgrade opencode

Update the opencode binary to a specific or latest version.

```bash
oh upgrade opencode
oh upgrade opencode 0.3.0
```

---

## Plugin Commands

### oh plugin list

List installed plugins. Alias: `ls`.

```bash
oh plugin list
oh plugin ls
```

### oh plugin install

Install a plugin by name.

```bash
oh plugin install rtk
```

### oh plugin remove

Remove an installed plugin. Aliases: `rm`, `uninstall`.

| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Skip confirmation |

```bash
oh plugin remove rtk
oh plugin rm rtk -f
```

### oh plugin status

Show status of installed plugins.

```bash
oh plugin status
```

---

## Worktree Commands

### oh worktree list

List active git worktrees. Alias: `ls`.

| Flag | Short | Description |
|------|-------|-------------|
| `--json` | | Output as JSON |

```bash
oh worktree list
oh worktree ls --json
```

### oh worktree add

Create a new git worktree for isolated work.

```bash
oh worktree add feature/auth
oh worktree add bugfix/login
```

### oh worktree remove

Remove a git worktree. Alias: `rm`.

| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Force removal even if dirty |

```bash
oh worktree remove ./worktrees/feature-auth
oh worktree rm ./worktrees/old-branch -f
```

### oh worktree cleanup

Remove worktrees whose branches have been merged.

| Flag | Short | Description |
|------|-------|-------------|
| `--base` | `-b` | Base branch to compare against |
| `--force` | `-f` | Skip confirmation |

```bash
oh worktree cleanup
oh worktree cleanup -b main -f
```

---

## MCP Commands

### oh mcp serve

Run an MCP server over stdio. Used for tool integration with coding agents.

```bash
oh mcp serve my-server
```

### oh mcp list

List available MCP server configurations. Alias: `ls`.

| Flag | Short | Description |
|------|-------|-------------|
| `--json` | | Output as JSON |

```bash
oh mcp list
oh mcp ls --json
```

---

## Agent & Skills Commands

### oh agent list

List available agents. Alias: `ls`.

| Flag | Short | Description |
|------|-------|-------------|
| `--json` | | Output as JSON |

```bash
oh agent list
oh agent ls --json
```

### oh skills list

List available skills. Alias: `ls`.

| Flag | Short | Description |
|------|-------|-------------|
| `--json` | | Output as JSON |

```bash
oh skills list
oh skills ls --json
```

---

## Service Commands

### oh service

Show status of configured external services (default when no subcommand given).

```bash
oh service
```

### oh service setup

Interactive credential wizard for configuring service access (API keys, tokens).

```bash
oh service setup
```

### oh service remove

Remove a configured service.

| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Skip confirmation |

```bash
oh service remove gitlab
oh service remove figma -f
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Error (command failed) |
| `2` | Warning (partial success or non-critical issue) |

---

## Shell Completion

Install shell completions for tab-completion of commands, subcommands, and flags.

```bash
# Bash (add to ~/.bashrc)
source <(oh completion bash)

# Zsh (add to ~/.zshrc)
source <(oh completion zsh)

# Fish
oh completion fish | source

# PowerShell (add to $PROFILE)
oh completion powershell | Out-String | Invoke-Expression
```
