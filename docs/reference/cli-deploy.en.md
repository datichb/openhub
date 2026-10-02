> [Lire en francais](cli-deploy.fr.md)

# CLI Reference — Deployment

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
