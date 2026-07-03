> 🇬🇧 [Read in English](cli.en.md)

# Reference CLI

Le binaire `oh` est le point d'entree unique du hub opencode. Il orchestre les sessions IA, la gestion de projets, le deploiement de configuration et l'outillage developeur.

```
oh [--verbose] <commande> [sous-commande] [options] [arguments]
```

| Flag global | Description |
|-------------|-------------|
| `--verbose` | Active les logs detailles |

---

## Session

### oh init

Assistant de configuration initiale.

```
oh init
```

Lance un wizard interactif qui configure : langue, opencode, projet, serveurs MCP, base de donnees et deploiement.

**Exemple :**

```bash
oh init
```

---

### oh start

Lance une session opencode.

```
oh start [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--agent` | `-a` | Agent a utiliser |
| `--prompt` | `-p` | Prompt initial |
| `--provider` | `-P` | Provider LLM |
| `--project` | `-j` | Projet cible |
| `--resume` | `-r` | Reprendre la derniere session |
| `--worktree` | `-w` | Utiliser un worktree dedie |
| `--dev` | | Mode developpement |
| `--label` | `-l` | Label de la session |
| `--assignee` | `-A` | Assignee du ticket |
| `--onboard` | | Active l'onboarding |
| `--refresh` | | Force le rafraichissement du contexte |

**Exemple :**

```bash
oh start -j mon-projet -a coder -p "Ajoute un endpoint /health"
oh start --resume
oh start --worktree -l "feat/auth"
```

---

### oh quick

Tache rapide avec selection interactive du projet.

```
oh quick
```

Ouvre un selecteur de projet puis lance une session courte.

**Exemple :**

```bash
oh quick
```

---

### oh audit

Audit de code selon un type d'analyse.

```
oh audit [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-j` | Projet cible |
| `--type` | `-t` | Type d'audit |

Types disponibles : `security`, `performance`, `architecture`, `accessibility`, `ecodesign`, `observability`, `privacy`.

**Exemple :**

```bash
oh audit -j api-gateway -t security
oh audit --type performance
```

---

### oh review

Revue de code du projet.

```
oh review [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-j` | Projet cible |

**Exemple :**

```bash
oh review -j frontend
```

---

### oh debug

Session de debug ciblee.

```
oh debug [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-j` | Projet cible |
| `--issue` | `-i` | Numero du ticket a debugger |

**Exemple :**

```bash
oh debug -j backend -i 42
```

---

### oh conventions

Affiche les conventions du projet.

```
oh conventions [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-j` | Projet cible |

**Exemple :**

```bash
oh conventions -j mon-projet
```

---

### oh beads

Proxy vers la commande `bd` (beads). Tous les arguments sont transmis directement.

```
oh beads [arguments...]
```

**Exemple :**

```bash
oh beads list
oh beads run mon-bead
```

---

## Projet

### oh project list

Liste les projets enregistres.

```
oh project list [options]
oh project ls [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--status` | `-s` | Filtrer par statut |
| `--json` | | Sortie JSON |

**Exemple :**

```bash
oh project list
oh project ls --json
oh project list -s active
```

---

### oh project add

Enregistre un nouveau projet.

```
oh project add [options]
oh project register [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--name` | `-n` | Nom du projet |
| `--path` | `-p` | Chemin du projet |
| `--language` | `-l` | Langage principal |
| `--tracker` | `-t` | URL du tracker |

**Exemple :**

```bash
oh project add -n api -p ./services/api -l go -t https://github.com/org/api/issues
```

---

### oh project remove

Supprime un projet du registre.

```
oh project remove [options]
oh project rm [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--force` | `-f` | Suppression sans confirmation |

**Exemple :**

```bash
oh project remove mon-projet
oh project rm -f legacy-app
```

---

### oh project rename

Renomme un projet.

```
oh project rename <ancien-nom> <nouveau-nom>
```

**Exemple :**

```bash
oh project rename api api-v2
```

---

### oh project move

Change le chemin d'un projet.

```
oh project move <nom> <nouveau-chemin>
```

**Exemple :**

```bash
oh project move api ../new-location/api
```

---

### oh project configure

Configure les options d'un projet.

```
oh project configure <nom> [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--provider` | `-P` | Provider LLM |
| `--model` | `-m` | Modele a utiliser |
| `--language` | `-l` | Langage principal |
| `--tracker` | `-t` | URL du tracker |

**Exemple :**

```bash
oh project configure api -P anthropic -m claude-sonnet-4-20250514
```

---

## Configuration

### oh config get

Affiche la valeur d'une cle de configuration.

```
oh config get <cle>
```

**Exemple :**

```bash
oh config get default_provider
```

---

### oh config set

Modifie une valeur de configuration.

```
oh config set <cle> <valeur>
```

**Exemple :**

```bash
oh config set default_provider anthropic
oh config set language fr
```

---

### oh config unset

Supprime une cle de configuration.

```
oh config unset <cle>
```

**Exemple :**

```bash
oh config unset custom_model
```

---

### oh config list

Affiche toute la configuration.

```
oh config list [options]
oh config ls [options]
```

| Flag | Description |
|------|-------------|
| `--json` | Sortie JSON |

**Exemple :**

```bash
oh config list
oh config ls --json
```

---

### oh config path

Affiche le chemin du fichier de configuration.

```
oh config path
```

**Exemple :**

```bash
oh config path
# /home/user/.config/opencode-hub/config.yaml
```

---

### oh config language

Affiche ou change la langue de l'interface.

```
oh config language [lang]
```

Langues supportees : `fr`, `en`.

**Exemple :**

```bash
oh config language        # Affiche la langue courante
oh config language fr     # Passe en francais
```

---

### oh config websearch

Gere les permissions de recherche web.

```
oh config websearch [enable|disable|status]
```

**Exemple :**

```bash
oh config websearch status
oh config websearch enable
```

---

## Deploiement

### oh deploy

Deploie les agents, skills, configuration et serveurs MCP vers un projet.

```
oh deploy [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-j` | Projet cible |
| `--provider` | `-P` | Provider LLM |
| `--model` | `-m` | Modele a utiliser |
| `--check` | | Verification sans deploiement |
| `--diff` | | Affiche les differences |

**Exemple :**

```bash
oh deploy -j api
oh deploy --check --diff
oh deploy -P anthropic -m claude-sonnet-4-20250514
```

---

### oh sync

Synchronise la configuration des projets.

```
oh sync [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-j` | Projet cible (defaut: tous) |
| `--all` | | Synchroniser tous les projets |
| `--dry-run` | | Simulation sans modification |

**Exemple :**

```bash
oh sync --all
oh sync -j frontend --dry-run
```

---

## Analytique

### oh status

Affiche l'etat du hub et du projet courant.

```
oh status [options]
```

| Flag | Description |
|------|-------------|
| `--json` | Sortie JSON |

**Exemple :**

```bash
oh status
oh status --json
```

---

### oh metrics

Affiche les metriques d'utilisation.

```
oh metrics [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--period` | `-p` | Periode : `7d`, `30d`, `all` |

**Exemple :**

```bash
oh metrics
oh metrics -p 30d
```

---

### oh dashboard

Tableau de bord interactif (TUI).

```
oh dashboard
```

Lance une interface terminale interactive avec vue d'ensemble des projets, sessions et metriques.

**Exemple :**

```bash
oh dashboard
```

---

### oh board

Kanban interactif des tickets.

```
oh board [options]
```

| Flag | Description |
|------|-------------|
| `--watch` | Rafraichissement automatique |

**Exemple :**

```bash
oh board
oh board --watch
```

---

### oh optimize

Affiche des suggestions d'optimisation pour le projet courant.

```
oh optimize
```

**Exemple :**

```bash
oh optimize
```

---

### oh yield

Rapport detaille des sessions et commits.

```
oh yield
```

**Exemple :**

```bash
oh yield
```

---

## Infrastructure

### oh doctor

Diagnostic systeme complet.

```
oh doctor
```

Verifie : binaires requis, configuration, connectivite, serveurs MCP, permissions.

**Exemple :**

```bash
oh doctor
```

---

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

Genere le script de completion pour le shell indique.

```
oh completion [bash|zsh|fish|powershell]
```

**Exemple :**

```bash
oh completion zsh > ~/.zfunc/_oh
oh completion bash > /etc/bash_completion.d/oh
oh completion fish > ~/.config/fish/completions/oh.fish
```

---

### oh upgrade opencode

Met a jour opencode vers une version donnee.

```
oh upgrade opencode [version]
```

**Exemple :**

```bash
oh upgrade opencode          # Derniere version
oh upgrade opencode 0.3.1   # Version specifique
```

---

## Plugin

### oh plugin list

Liste les plugins installes.

```
oh plugin list
oh plugin ls
```

**Exemple :**

```bash
oh plugin list
```

---

### oh plugin install

Installe un plugin.

```
oh plugin install <name>
```

**Exemple :**

```bash
oh plugin install rtk
```

---

### oh plugin remove

Supprime un plugin.

```
oh plugin remove <name>
oh plugin rm <name>
oh plugin uninstall <name>
```

| Flag | Court | Description |
|------|-------|-------------|
| `--force` | `-f` | Suppression sans confirmation |

**Exemple :**

```bash
oh plugin remove rtk
oh plugin rm -f rtk
```

---

### oh plugin status

Affiche l'etat des plugins installes.

```
oh plugin status
```

**Exemple :**

```bash
oh plugin status
```

---

## Worktree

### oh worktree list

Liste les worktrees du projet.

```
oh worktree list [options]
oh worktree ls [options]
```

| Flag | Description |
|------|-------------|
| `--json` | Sortie JSON |

**Exemple :**

```bash
oh worktree list
oh worktree ls --json
```

---

### oh worktree add

Cree un nouveau worktree.

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

```
oh worktree remove [path]
oh worktree rm [path]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--force` | `-f` | Suppression forcee |

**Exemple :**

```bash
oh worktree remove ../project-feat-auth
oh worktree rm -f ../project-fix-old
```

---

### oh worktree cleanup

Supprime les worktrees dont la branche a ete mergee.

```
oh worktree cleanup [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--base` | `-b` | Branche de reference (defaut: main) |
| `--force` | `-f` | Suppression sans confirmation |

**Exemple :**

```bash
oh worktree cleanup
oh worktree cleanup -b develop --force
```

---

## MCP

### oh mcp serve

Lance un serveur MCP en mode stdio.

```
oh mcp serve <name>
```

**Exemple :**

```bash
oh mcp serve gitlab
oh mcp serve figma
```

---

### oh mcp list

Liste les serveurs MCP disponibles.

```
oh mcp list [options]
oh mcp ls [options]
```

| Flag | Description |
|------|-------------|
| `--json` | Sortie JSON |

**Exemple :**

```bash
oh mcp list
oh mcp ls --json
```

---

## Agents / Skills

### oh agent list

Liste les agents disponibles.

```
oh agent list [options]
oh agent ls [options]
```

| Flag | Description |
|------|-------------|
| `--json` | Sortie JSON |

**Exemple :**

```bash
oh agent list
oh agent ls --json
```

---

### oh skills list

Liste les skills disponibles.

```
oh skills list [options]
oh skills ls [options]
```

| Flag | Description |
|------|-------------|
| `--json` | Sortie JSON |

**Exemple :**

```bash
oh skills list
oh skills ls --json
```

---

## Services

### oh service

Affiche l'etat des services configures (comportement par defaut).

```
oh service
```

**Exemple :**

```bash
oh service
```

---

### oh service setup

Wizard interactif de configuration d'un service.

```
oh service setup
```

**Exemple :**

```bash
oh service setup
```

---

### oh service remove

Supprime un service.

```
oh service remove [name]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--force` | `-f` | Suppression sans confirmation |

**Exemple :**

```bash
oh service remove gitlab
oh service remove -f figma
```

---

## Codes de sortie

| Code | Signification |
|------|---------------|
| `0` | Succes |
| `1` | Erreur |
| `2` | Avertissement |

---

## Completion shell

Pour activer la completion automatique :

**Zsh :**

```bash
oh completion zsh > "${fpath[1]}/_oh"
source ~/.zshrc
```

**Bash :**

```bash
oh completion bash > /etc/bash_completion.d/oh
source /etc/bash_completion.d/oh
```

**Fish :**

```bash
oh completion fish > ~/.config/fish/completions/oh.fish
```

**PowerShell :**

```powershell
oh completion powershell | Out-String | Invoke-Expression
```
