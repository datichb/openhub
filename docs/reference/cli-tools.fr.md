> [Read in English](cli-tools.en.md)

# Reference CLI — Outils

## Marketplace de Skills

## Marketplace de Skills

### oh skill add

Installe un skill depuis une source (chemin local, URL git ou nom dans le registre).

```
oh skill add <source>
```

**Exemple :**

```bash
oh skill add rtk
oh skill add https://github.com/org/mon-skill
oh skill add ./skill-local
```

---

### oh skill list

Liste les skills installes.

**Alias :** `oh skill ls`

```
oh skill list
```

---

### oh skill remove

Supprime un skill installe.

**Alias :** `oh skill rm`

```
oh skill remove <nom>
oh skill rm <nom>
```

---

### oh skill search

Recherche dans le registre de skills.

```
oh skill search [requete]
```

**Exemple :**

```bash
oh skill search
oh skill search react
oh skill search "code review"
```

---


### oh skill budget

Affiche le budget context window par agent (cout du system prompt toujours charge en lignes et tokens).

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--all` | `-a` | bool | Afficher le budget de tous les agents |
| `--threshold` | `-t` | int | Seuil en lignes pour flaguer un skill (defaut : 150) |

```bash
oh skill budget orchestrator-dev
oh skill budget --all
oh skill budget --all --threshold 200
```

---

## Git Worktree

## Git Worktree

### oh worktree list

Liste les worktrees du projet.

**Alias :** `oh worktree ls`

```
oh worktree list [options]
```

| Flag | Description |
|------|-------------|
| `--json` | Sortie au format JSON |

**Exemple :**

```bash
oh worktree list
oh worktree ls --json
```

---

### oh worktree add

Cree un worktree.

```
oh worktree add [branch]
```

Interactif si branche omise.

**Exemple :**

```bash
oh worktree add feat/new-feature
oh worktree add fix/bug-123
```

---

### oh worktree remove

Supprime un worktree.

**Alias :** `oh worktree rm`

```
oh worktree remove [path]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--force` | `-f` | Forcer la suppression |

**Exemple :**

```bash
oh worktree remove ../project-feat-auth
oh worktree rm -f ../project-fix-old
```

---

### oh worktree cleanup

Nettoie les worktrees dont la branche a ete mergee.

```
oh worktree cleanup [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--base` | `-b` | Branche de base (defaut : auto-detect) |
| `--force` | `-f` | Supprimer sans confirmation |

**Exemple :**

```bash
oh worktree cleanup
oh worktree cleanup -b develop --force
```

---


---

## Metriques & Dashboard

## Analytique

### oh metrics

Affiche les metriques d'utilisation, les statistiques de sessions et la telemetrie des agents.

```
oh metrics [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--period` | `-d` | Periode d'analyse (7d, 30d, all). Defaut : all |

**Exemple :**

```bash
oh metrics
oh metrics -d 30d
oh metrics -d 7d
```

---

### oh dashboard

Tableau de bord interactif (TUI).

```
oh dashboard
```

Pas de flags. Lance une interface terminale interactive avec vue d'ensemble des projets, sessions et metriques.

**Exemple :**

```bash
oh dashboard
```

---

### oh board

Tableau kanban des tickets.

```
oh board [options]
```

| Flag | Description |
|------|-------------|
| `--watch` | Rafraichissement auto toutes les 5s |

**Exemple :**

```bash
oh board
oh board --watch
```

---


---

## Gestion des secrets

## Gestion des secrets

### oh secrets

Gerer les secrets stockes dans le trousseau OS ou le stockage chiffre.

```bash
oh secrets set <cle> <valeur>    # stocker un secret
oh secrets get <cle>             # recuperer un secret
oh secrets list                  # lister toutes les cles
oh secrets delete <cle>          # supprimer un secret
```

---


### oh secrets cleanup

Scanner le trousseau systeme pour les entrees orphelines et optionnellement les supprimer.

| Flag | Type | Description |
|------|------|-------------|
| `--dry-run` | bool | Afficher les entrees orphelines sans les supprimer |

```bash
oh secrets cleanup
oh secrets cleanup --dry-run
```

---

## Utilitaires

## Utilitaires

### oh version

Affiche la version du binaire `oh`.

```
oh version
```

**Exemple :**

```bash
oh version
# oh v1.2.0 (go1.22, darwin/arm64)
```

---

### oh completion

Genere le script d'autocompletion pour le shell indique.

```
oh completion [bash|zsh|fish|powershell]
```

**Exemple :**

```bash
oh completion zsh > "${fpath[1]}/_oh"
oh completion bash > /etc/bash_completion.d/oh
oh completion fish > ~/.config/fish/completions/oh.fish
oh completion powershell | Out-String | Invoke-Expression
```

---

