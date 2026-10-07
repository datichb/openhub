> [Lire en francais](cli-projects.fr.md)

# CLI Reference — Projects

## Projects

`oh project` manages the projects registered in the hub. **Alias:** `oh p`.

### oh project list

Lists the registered projects.

**Alias:** `oh project ls`

```
oh project list [options]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--status` | `-s` | string | Filter by status (`active`, `archived`) |
| `--json` | | bool | Output in JSON format |

**Example:**

```bash
oh project list
oh project ls -s active
oh project list --json
```

---

### oh project add

Registers a project in the hub. Without flags, runs an interactive wizard.

**Alias:** `oh project register`

```
oh project add [options]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--name` | `-n` | string | Project name |
| `--path` | `-d` | string | Project path (default: current directory) |
| `--language` | `-l` | string | Main language |

**Example:**

```bash
oh project add -n api -d ./services/api -l go
oh project register -n frontend --language typescript
oh project add
```

---

### oh project remove

Removes a project from the registry (files on disk are not deleted).

**Alias:** `oh project rm`

```
oh project remove [project-id] [options]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--force` | `-f` | bool | Remove without confirmation |

**Example:**

```bash
oh project remove my-project
oh project rm -f legacy-app
```

---

### oh project rename

Changes the display name of a project. The ID does not change. Interactive when arguments are omitted.

```
oh project rename [project-id] [new-name]
```

**Example:**

```bash
oh project rename api api-v2
oh project rename
```

---

### oh project move

Updates the registered path of a project (after moving it on disk). Does not move the folder. Interactive when arguments are omitted.

```
oh project move [project-id] [new-path]
```

**Example:**

```bash
oh project move api ../new-location/api
oh project move
```

---

### oh project configure

Configures the settings of a project: LLM provider, model, language. They are used when sessions are launched (`oh run`). Without flags, runs an interactive wizard.

```
oh project configure [project-id] [options]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--provider` | `-P` | string | LLM provider: `bedrock`, `anthropic`, `openrouter`, `github-copilot` (value not checked) |
| `--model` | `-m` | string | LLM model |
| `--language` | `-l` | string | Main language |

The default workflow and runtime of the project are set in the TUI (Project config › Execution), see [`oh run`](cli-workflows.en.md#oh-run).

**Example:**

```bash
oh project configure api -P anthropic -m claude-sonnet-4-20250514
oh project configure api -l go
oh project configure
```
