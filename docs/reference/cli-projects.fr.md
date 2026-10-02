> [Read in English](cli-projects.en.md)

# Reference CLI — Projets

## Projets

### oh project list

Liste les projets enregistres.

**Alias :** `oh project ls`

```
oh project list [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--status` | `-s` | Filtrer par statut (active, archived) |
| `--json` | | Sortie au format JSON |

**Exemple :**

```bash
oh project list
oh project ls --json
oh project list -s active
```

---

### oh project add

Enregistre un nouveau projet.

**Alias :** `oh project register`

```
oh project add [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--name` | `-n` | Nom du projet |
| `--path` | `-d` | Chemin du projet (defaut : repertoire courant) |
| `--language` | `-l` | Langage principal |

**Exemple :**

```bash
oh project add -n api -d ./services/api -l go -t github
oh project register -n frontend --language typescript
```

---

### oh project remove

Supprime un projet.

**Alias :** `oh project rm`

```
oh project remove [project-id]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--force` | `-f` | Supprimer sans confirmation |

**Exemple :**

```bash
oh project remove mon-projet
oh project rm -f legacy-app
```

---

### oh project rename

Renomme un projet.

```
oh project rename [project-id] [new-name]
```

Interactif si arguments omis.

**Exemple :**

```bash
oh project rename api api-v2
```

---

### oh project move

Deplace un projet (change le chemin enregistre).

```
oh project move [project-id] [new-path]
```

Interactif si arguments omis.

**Exemple :**

```bash
oh project move api ../new-location/api
```

---

### oh project configure

Configure un projet.

```
oh project configure [project-id] [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--provider` | `-P` | Provider LLM |
| `--model` | `-m` | Modele LLM |
| `--language` | `-l` | Langage principal |

**Exemple :**

```bash
oh project configure api -P anthropic -m claude-sonnet-4-20250514
```

---
