> [Lire en francais](cli-projects.fr.md)

# CLI Reference — Projects

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

```bash
oh project add -n my-app -l typescript
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

```bash
oh project configure my-app --provider anthropic --model claude-sonnet-4-20250514
oh project configure my-app -l go
oh project configure
```

---
