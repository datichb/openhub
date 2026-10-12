> [Read in English](cli-tools.en.md)

# Reference CLI — Outils

## Skills

`oh skill` vérifie le catalogue des skills. Les skills d'une session sont celles du hub et du catalogue d'équipe (`catalog/skills/` du team-state), livrées par son paquet de session (voir [`oh bundle show`](cli-workflows.fr.md#oh-bundle-build--show), qui donne aussi le budget avec `--budget`).

> **Déconnecté en v5** ([ADR-051](../architecture/adr/051-community-skills-disconnection.fr.md)) : le registre de skills communautaires (`oh skill add`, `list`, `remove`, `search`) et `oh skill budget` sont supprimés, et `~/.oh/skills` n'est plus lu. `oh doctor` signale les paquets qui y restent.

### oh skill check

Vérifie les skills du hub (`skills/**/*.md`, hors annexes de `skills/templates/`) et les skills nommées par les agents du hub. Sort avec le code 1 en cas d'erreur.

| Type | Niveau | Problème |
|------|--------|----------|
| `duplicate_id` | erreur | Deux skills de même identifiant (dernier élément du chemin) |
| `missing_require` | erreur | `requires:` nomme une skill introuvable |
| `require_cycle` | erreur | Cycle de `requires:` |
| `invalid_frontmatter` | erreur | Frontmatter qui n'est pas du YAML valide |
| `name_mismatch` | erreur | `name:` différent du nom du fichier |
| `invalid_name` | erreur | `name:` de plus de 64 caractères ou pas en kebab-case (minuscules, chiffres, tirets simples) |
| `long_description` | erreur | `description:` de plus de 1024 caractères |
| `unknown_key` | erreur | Clé de frontmatter qui n'est ni une clé Agent Skills (`name`, `description`, `license`, `compatibility`, `metadata`, `allowed-tools`) ni une clé du hub (`requires`, `annexes`, `plugin`, `mcp`) |
| `missing_annex` | erreur | `annexes:` nomme un fichier introuvable |
| `no_description` | avertissement | `description:` absente |
| `legacy_bucket` | avertissement | Champ `bucket:` obsolète |
| `missing_agent_skill` | avertissement | Skill nommée dans les `skills:` ou `native_skills:` d'un agent mais introuvable |
| `undeclared_annex` | avertissement | Gabarit cité dans le texte mais absent d'`annexes:` |
| `orphan_annex` | avertissement | Fichier de `skills/templates/` utilisé par aucune skill |

Seules les clés Agent Skills arrivent dans le `SKILL.md` livré ; les clés du hub sont lues par oh (`plugin:` et `mcp:` ne livrent la skill que si le workflow charge ce plugin ou si la session a ce serveur MCP). Voir [Format d'un skill](../architecture/skills.fr.md#format-dun-skill).

```
oh skill check [--json]
```

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Problèmes au format JSON (`kind`, `error`, `ref`, `detail`) |

---

## Git Worktree

`oh worktree` gere les git worktrees du projet. **Alias :** `oh wt`. `oh run --location new` cree aussi un worktree par session (voir [`oh run`](cli-workflows.fr.md#oh-run)).

### oh worktree list

Liste les worktrees du projet.

**Alias :** `oh worktree ls`

```
oh worktree list [options]
```

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Sortie au format JSON |

**Exemple :**

```bash
oh worktree list
oh worktree ls --json
```

---

### oh worktree add

Cree un worktree pour une branche. Interactif si la branche est omise.

```
oh worktree add [branch]
```

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

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--force` | `-f` | bool | Forcer la suppression |

**Exemple :**

```bash
oh worktree remove ../project-feat-auth
oh worktree rm -f ../project-fix-old
```

---

### oh worktree cleanup

Supprime les worktrees dont la branche est entierement mergee dans la branche de base (detection par `git branch --merged`).

```
oh worktree cleanup [options]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--base` | `-b` | string | Branche de base (defaut : detection automatique) |
| `--force` | `-f` | bool | Supprimer sans confirmation |

**Exemple :**

```bash
oh worktree cleanup
oh worktree cleanup -b develop --force
```

---

## Metriques & Dashboard

### oh metrics

Metriques d'utilisation : sessions, tokens, couts et economies.

```
oh metrics [options]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--period` | `-d` | string | Periode d'analyse : `7d`, `30d` ou `all` (defaut : `all`) |

**Exemple :**

```bash
oh metrics
oh metrics -d 30d
oh metrics -d 7d
```

---

### oh dashboard

Tableau de bord interactif (TUI) : projets, sessions, tokens. Sans flag.

```
oh dashboard
```

---

### oh board

Tableau kanban des tickets en plein ecran.

```
oh board [options]
```

| Flag | Type | Description |
|------|------|-------------|
| `--watch` | bool | Rafraichissement automatique toutes les 5 s |

**Exemple :**

```bash
oh board
oh board --watch
```

`oh optimize` (analyse d'usage et suggestions) et `oh yield` (rapport sessions ↔ commits) existent encore mais sont masques de l'aide, sans flag.

---

## Gestion des secrets

### oh secrets

Secrets stockes dans le trousseau du systeme (ou, a defaut, dans un fichier chiffre `~/.oh/secrets.enc`, phrase de passe demandee ou `OH_PASSPHRASE`). Ils sont references par nom dans `hub.toml` (`token_key`) et resolus a l'execution, en portee projet ou globale. Ordre de resolution : secret du projet (dans un dossier de projet), puis secret global, puis variable d'environnement correspondante.

| Commande | Flags | Description |
|----------|-------|-------------|
| `oh secrets set <cle>` | `--global`, `--project <id>` | Stocke un secret ; la valeur est saisie au terminal (masquee), ou lue sur l'entree standard sans terminal (`printf %s "$TOKEN" \| oh secrets set <cle>`). Portee : projet si le dossier courant est un projet enregistre, sinon globale ; `--global` force la portee globale, `--project` cible un projet |
| `oh secrets get <cle>` | `--global`, `--project <id>`, `--reveal` | Affiche un secret, masque sauf avec `--reveal` ; `--global` : portee globale uniquement |
| `oh secrets list` | | Liste les secrets connus |
| `oh secrets delete <cle>` | `--global`, `--project <id>` | Supprime un secret du trousseau |
| `oh secrets cleanup` | `--dry-run` | Supprime les entrees orphelines du trousseau (absentes de l'index des secrets) ; `--dry-run` : les afficher sans supprimer |

```bash
oh secrets set openhub.mcp.gitlab.token
oh secrets set openhub.mcp.gitlab.token --project t-sru-b267fbf1
oh secrets get openhub.mcp.gitlab.token --reveal
oh secrets list
oh secrets delete openhub.mcp.gitlab.token --global
oh secrets cleanup --dry-run
```

---

## Utilitaires

### oh version

Affiche la version du binaire `oh`.

```
oh version
```

---

### oh completion

Genere le script d'autocompletion pour le shell indique.

```
oh completion bash|zsh|fish|powershell
```

**Exemple :**

```bash
source <(oh completion zsh)
oh completion zsh > "${fpath[1]}/_oh"
oh completion bash > /etc/bash_completion.d/oh
oh completion fish > ~/.config/fish/completions/oh.fish
oh completion powershell | Out-String | Invoke-Expression
```
