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
| `--prompt` | `-p` | string | Initial prompt |
| `--provider` | `-P` | string | LLM provider (bedrock, anthropic, openai) |
| `--project` | `-j` | string | Project ID (auto-detected otherwise) |
| `--resume` | `-r` | string | Resume an existing session (session ID) |
| `--worktree` | `-w` | string | Branch to launch in a git worktree |
| `--dev` | | bool | Dev mode: epic/ticket picker + orchestrator-dev |
| `--label` | `-l` | string | Filter tickets by label (requires --dev) |
| `--assignee` | `-A` | string | Filter tickets by assignee (requires --dev) |
| `--onboard` | | bool | Onboarding mode: creates/enriches project wiki |
| `--refresh` | | bool | Force wiki re-discovery (requires --onboard) |
| `--yes` | `-y` | bool | Skip confirmation and launch immediately |

```bash
oh start -j my-app -p "Fix the login bug"
oh start --resume abc123-session-id
oh start -w feature/auth -a architect
oh start --dev -l "priority:high" -A me
oh start --onboard --refresh
oh start -p "Refactor the auth module" -y
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
| `--project` | `-j` | string | Project ID |
| `--type` | `-t` | string | Audit type (security, performance, architecture, accessibility, ecodesign, observability, privacy). Default: security |

```bash
oh audit -j my-app
oh audit -j my-app -t performance
oh audit --type accessibility
```

---

### oh review

Launch an automated code review session.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-j` | string | Project ID |

```bash
oh review -j my-app
oh review
```

---

### oh debug

Start a debugging session with AI assistance.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-j` | string | Project ID |
| `--issue` | `-i` | string | Issue description |

```bash
oh debug -j my-app -i "Users get 500 on /api/auth/callback"
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
| `--path` | `-p` | string | Project path (default: cwd) |
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
| `--project` | `-j` | string | Project ID |
| `--provider` | `-P` | string | Provider to configure |
| `--model` | `-m` | string | Model to configure |
| `--check` | | bool | Check if agents/skills changed since last deploy |
| `--diff` | | bool | Show changes without applying |

```bash
oh deploy -j my-app
oh deploy --check
oh deploy --diff
oh deploy -j my-app -P anthropic -m claude-sonnet-4-20250514
```

---

### oh sync

Synchronize project configuration with remote state.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-j` | string | Project ID |
| `--all` | | bool | Sync all active projects |
| `--dry-run` | | bool | Show changes without applying |

```bash
oh sync -j my-app
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

Run diagnostic checks on the environment. Checks: OS, git, opencode, bd, fzf, compatibility, config, database, API keys.

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

---

### oh mcp serve

Serve a built-in MCP server via stdio.

```bash
oh mcp serve figma
oh mcp serve gitlab
oh mcp serve gslides
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

### oh service setup

Interactive wizard to configure MCP service tokens in keychain.

```bash
oh service setup
```

---

### oh service remove

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

### oh metrics

Display project metrics and session statistics.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--period` | `-p` | string | Analysis period (7d, 30d, all). Default: all |

```bash
oh metrics
oh metrics -p 7d
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
