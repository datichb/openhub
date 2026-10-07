> [Lire en francais](cli-tools.fr.md)

# CLI Reference — Tools

## Skills Marketplace

`oh skill` installs, lists and removes community skills (community index or git repository). The skills available to a session are those of its session bundle (see [`oh bundle show`](cli-workflows.en.md#oh-bundle-build--show)).

### oh skill add

Installs a skill from the community index (by name) or from a git URL.

```
oh skill add <source>
```

**Example:**

```bash
oh skill add golang-idioms
oh skill add https://github.com/u/oh-skill-example
```

---

### oh skill list

Lists the installed community skills.

**Alias:** `oh skill ls`

```
oh skill list
```

---

### oh skill remove

Uninstalls a community skill.

**Alias:** `oh skill rm`

```
oh skill remove <name>
oh skill rm <name>
```

---

### oh skill search

Searches the community index (without query: the whole index).

```
oh skill search [query]
```

**Example:**

```bash
oh skill search
oh skill search go
oh skill search "code review"
```

---

### oh skill check

Checks the hub skills and the installed community skills: duplicate ids, `requires:` dependencies missing or cyclic, invalid frontmatter (`name:` different from the file name, missing description), obsolete `bucket:` field, skills referenced by agents but missing. Exits with code 1 on errors.

```
oh skill check [--json]
```

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Problems in JSON format |

---

### oh skill budget

Deprecated alias in v5: the budget of a session is read with `oh bundle show <workflow> --budget`. `oh skill budget <workflow>` redirects there (warning); with an agent name or `--all`, the former per-agent computation (cost in lines and tokens of the always-loaded system prompt, most expensive skills) stays available with a warning.

```
oh skill budget [workflow|agent] [options]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--all` | `-a` | bool | Show the budget of every agent |
| `--threshold` | `-t` | int | Line threshold to flag a skill (default: 150) |

```bash
oh skill budget ticket                  # = oh bundle show ticket --budget
oh skill budget orchestrator-dev
oh skill budget --all --threshold 200
```

---

## Git Worktree

`oh worktree` manages the git worktrees of the project. **Alias:** `oh wt`. `oh run --location new` also creates one worktree per session (see [`oh run`](cli-workflows.en.md#oh-run)).

### oh worktree list

Lists the worktrees of the project.

**Alias:** `oh worktree ls`

```
oh worktree list [options]
```

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Output in JSON format |

**Example:**

```bash
oh worktree list
oh worktree ls --json
```

---

### oh worktree add

Creates a worktree for a branch. Interactive when the branch is omitted.

```
oh worktree add [branch]
```

**Example:**

```bash
oh worktree add feat/new-feature
oh worktree add fix/bug-123
```

---

### oh worktree remove

Removes a worktree.

**Alias:** `oh worktree rm`

```
oh worktree remove [path]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--force` | `-f` | bool | Force the removal |

**Example:**

```bash
oh worktree remove ../project-feat-auth
oh worktree rm -f ../project-fix-old
```

---

### oh worktree cleanup

Removes the worktrees whose branch is fully merged into the base branch (detected with `git branch --merged`).

```
oh worktree cleanup [options]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--base` | `-b` | string | Base branch (default: auto-detected) |
| `--force` | `-f` | bool | Remove without confirmation |

**Example:**

```bash
oh worktree cleanup
oh worktree cleanup -b develop --force
```

---

## Metrics & Dashboard

### oh metrics

Usage metrics: sessions, tokens, costs and savings.

```
oh metrics [options]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--period` | `-d` | string | Analysis period: `7d`, `30d` or `all` (default: `all`) |

**Example:**

```bash
oh metrics
oh metrics -d 30d
oh metrics -d 7d
```

---

### oh dashboard

Interactive dashboard (TUI): projects, sessions, tokens. No flags.

```
oh dashboard
```

---

### oh board

Full-screen ticket kanban board.

```
oh board [options]
```

| Flag | Type | Description |
|------|------|-------------|
| `--watch` | bool | Automatic refresh every 5 s |

**Example:**

```bash
oh board
oh board --watch
```

`oh optimize` (usage analysis and suggestions) and `oh yield` (sessions ↔ commits report) still exist but are hidden from the help, without flags.

---

## Secrets Management

### oh secrets

Secrets stored in the system keychain (or, as a fallback, in an encrypted file `~/.oh/secrets.enc`, passphrase prompted or `OH_PASSPHRASE`). They are referenced by name in `hub.toml` (`token_key`) and resolved at runtime, in project or global scope. Resolution order: project secret (inside a project folder), then global secret, then the matching environment variable.

| Command | Flags | Description |
|---------|-------|-------------|
| `oh secrets set <key>` | `--global`, `--project <id>` | Stores a secret; the value is typed in the terminal (hidden). Scope: project when the current folder is a registered project, otherwise global; `--global` forces the global scope, `--project` targets a project |
| `oh secrets get <key>` | `--global`, `--project <id>`, `--reveal` | Shows a secret, masked unless `--reveal`; `--global`: global scope only |
| `oh secrets list` | | Lists the known secrets |
| `oh secrets delete <key>` | `--global`, `--project <id>` | Deletes a secret from the keychain |
| `oh secrets cleanup` | `--dry-run` | Removes orphaned keychain entries (absent from the secrets index); `--dry-run`: show them without deleting |

```bash
oh secrets set openhub.mcp.gitlab.token
oh secrets set openhub.mcp.gitlab.token --project t-sru-b267fbf1
oh secrets get openhub.mcp.gitlab.token --reveal
oh secrets list
oh secrets delete openhub.mcp.gitlab.token --global
oh secrets cleanup --dry-run
```

---

## Utilities

### oh version

Prints the version of the `oh` binary.

```
oh version
```

---

### oh completion

Generates the completion script for the given shell.

```
oh completion bash|zsh|fish|powershell
```

**Example:**

```bash
source <(oh completion zsh)
oh completion zsh > "${fpath[1]}/_oh"
oh completion bash > /etc/bash_completion.d/oh
oh completion fish > ~/.config/fish/completions/oh.fish
oh completion powershell | Out-String | Invoke-Expression
```
