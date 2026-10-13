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
| `--yes` | `-y` | bool | Beads zéro impact : retirer ce que `bd init` a écrit sans confirmation et annuler son commit s'il n'est pas poussé (sinon : liste, annulation du commit comprise, puis confirmation ; sans terminal, rien n'est changé) |

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

---

## Wiki du projet

### oh wiki check

Vérifie le wiki du projet (`docs/wiki/` du plus proche dossier parent qui en a un) : frontmatter (`updated` en date AAAA-MM-JJ, `confidence` dans `confirmed | mixed | uncertain` et cohérent avec les tags de la page, `agents`), liens relatifs et ancres, lignes de tag (`` — `TAG` · agent · AAAA-MM-JJ[ · source] ``, une source pour `CONFIRMÉ`), pages sans tag, fraîcheur (page plus vieille que le dernier commit d'un fichier qu'elle cite ; fichier cité absent) et god nodes calculés par oh face à la table de `index.md`. Affiche aussi les god nodes calculés. Code de sortie non nul s'il y a des erreurs (utilisable en CI).

```
oh wiki check [dir] [--fix] [--json]
```

| Flag | Type | Description |
|------|------|-------------|
| `--fix` | bool | Répare ce qu'oh sait calculer : `confidence` des pages, table des god nodes de `index.md` (et son `updated`) |
| `--json` | bool | Rapport en JSON (`pages`, `god_nodes`, `issues` avec `page`, `line`, `kind`, `severity`, `message`, `fixable`) |

oh ne suit jamais un lien symbolique dans `docs/wiki/` ; une page de plus de 256 Ko, ou au-delà de 400 pages, n'est pas lue (avertissement).

### oh wiki pending

Liste les propositions des agents pour le wiki du projet ([`wiki_propose`](cli-mcp.fr.md#serveur-mcp-wiki)) encore à appliquer : acceptées à la carte du commit (oh les applique au passage du checkpoint) ou en attente (workflows sans commit, pour `doc-sync`).

```
oh wiki pending [dir] [--all] [--session <id>] [--json]
```

| Flag | Type | Description |
|------|------|-------------|
| `--all` | bool | Toutes les propositions, appliquées et refusées comprises |
| `--session` | string | Seulement celles d'une session |
| `--json` | bool | Sortie JSON |

Le projet est celui qu'oh connaît pour le dossier (ou un dossier parent).
