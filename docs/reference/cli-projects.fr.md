> [Read in English](cli-projects.en.md)

# Reference CLI — Projets

## Projets

`oh project` gere les projets enregistres dans le hub. **Alias :** `oh p`.

### oh project list

Liste les projets enregistres.

**Alias :** `oh project ls`

```
oh project list [options]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--status` | `-s` | string | Filtrer par statut (`active`, `archived`) |
| `--json` | | bool | Sortie au format JSON |

**Exemple :**

```bash
oh project list
oh project ls -s active
oh project list --json
```

---

### oh project add

Enregistre un projet dans le hub. Sans flag, lance un assistant interactif.

**Alias :** `oh project register`

```
oh project add [options]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--name` | `-n` | string | Nom du projet |
| `--path` | `-d` | string | Chemin du projet (defaut : repertoire courant) |
| `--language` | `-l` | string | Langage principal |

**Exemple :**

```bash
oh project add -n api -d ./services/api -l go
oh project register -n frontend --language typescript
oh project add
```

---

### oh project remove

Supprime un projet du registre (les fichiers sur disque ne sont pas supprimes).

**Alias :** `oh project rm`

```
oh project remove [project-id] [options]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--force` | `-f` | bool | Supprimer sans confirmation |

**Exemple :**

```bash
oh project remove mon-projet
oh project rm -f legacy-app
```

---

### oh project rename

Change le nom d'affichage d'un projet. L'ID ne change pas. Interactif si les arguments sont omis.

```
oh project rename [project-id] [new-name]
```

**Exemple :**

```bash
oh project rename api api-v2
oh project rename
```

---

### oh project move

Met a jour le chemin enregistre d'un projet (apres un deplacement sur le disque). Ne deplace pas le dossier. Interactif si les arguments sont omis.

```
oh project move [project-id] [new-path]
```

**Exemple :**

```bash
oh project move api ../new-location/api
oh project move
```

---

### oh project configure

Configure les parametres d'un projet : provider LLM, modele, langage. Ils sont utilises au lancement des sessions (`oh run`). Sans flag, lance un assistant interactif.

```
oh project configure [project-id] [options]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--provider` | `-P` | string | Provider LLM : `bedrock`, `anthropic`, `openrouter`, `github-copilot` (valeur non verifiee) |
| `--model` | `-m` | string | Modele LLM |
| `--language` | `-l` | string | Langage principal |

Le workflow et l'environnement d'execution par defaut du projet se reglent dans la TUI (Config projet › Execution), voir [`oh run`](cli-workflows.fr.md#oh-run).

**Exemple :**

```bash
oh project configure api -P anthropic -m claude-sonnet-4-20250514
oh project configure api -l go
oh project configure
```
