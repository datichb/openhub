> [Read in English](cli-deploy.en.md)

# Reference CLI — Deploiement

## Deploiement

### oh deploy

Deploie agents, skills et config dans un projet.

```
oh deploy [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-p` | ID du projet |
| `--provider` | `-P` | Provider a configurer |
| `--model` | `-m` | Modele a configurer |
| `--check` | | Verifie si les agents/skills ont change |
| `--diff` | | Affiche les changements sans les appliquer |

**Exemple :**

```bash
oh deploy -p api
oh deploy --check --diff
oh deploy -P anthropic -m claude-sonnet-4-20250514
```

---

### oh sync

Synchronise agents, skills et config vers les projets.

```
oh sync [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-p` | ID du projet |
| `--all` | | Synchroniser tous les projets actifs |
| `--dry-run` | | Afficher les changements sans les appliquer |

**Exemple :**

```bash
oh sync --all
oh sync -p frontend --dry-run
```

---
