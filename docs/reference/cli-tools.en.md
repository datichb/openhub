> [Lire en francais](cli-tools.fr.md)

# CLI Reference — Tools

## Skills Marketplace

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


### oh skill budget

Display the context window budget per agent (always-loaded system prompt cost in lines and tokens).

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--all` | `-a` | bool | Show budget for all agents |
| `--threshold` | `-t` | int | Line threshold to flag a skill (default: 150) |

```bash
oh skill budget orchestrator-dev
oh skill budget --all
oh skill budget --all --threshold 200
```

---

## Git Worktree

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


---

## Metrics & Dashboard

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


---

## Secrets Management

## Secrets Management

### oh secrets

Manage secrets stored in the OS keychain or encrypted fallback store.

```bash
oh secrets set <key> <value>    # store a secret
oh secrets get <key>            # retrieve a secret
oh secrets list                 # list all secret keys
oh secrets delete <key>         # delete a secret
```

---


### oh secrets cleanup

Scan the OS keychain for orphaned entries and optionally delete them.

| Flag | Type | Description |
|------|------|-------------|
| `--dry-run` | bool | Show orphaned entries without deleting them |

```bash
oh secrets cleanup
oh secrets cleanup --dry-run
```

---

## Utilities

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
