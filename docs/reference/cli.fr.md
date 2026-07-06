> 🇬🇧 [Read in English](cli.en.md)

# Reference CLI

Le binaire `oh` est le point d'entree unique du hub opencode. Il orchestre les sessions IA, la gestion de projets, le deploiement de configuration et l'outillage developeur.

```
oh [--verbose] <commande> [sous-commande] [options] [arguments]
```

## Flags globaux

| Flag | Court | Description |
|------|-------|-------------|
| `--verbose` | `-v` | Active la sortie verbose |

---

## Sessions

### oh start

Lance une session opencode.

```
oh start [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--agent` | `-a` | Agent a utiliser |
| `--prompt` | `-p` | Prompt initial |
| `--provider` | `-P` | Provider LLM (bedrock, anthropic, openai) |
| `--project` | `-j` | ID du projet (detection auto sinon) |
| `--resume` | `-r` | Reprendre une session existante (ID de session) |
| `--worktree` | `-w` | Branche pour lancer dans un git worktree |
| `--dev` | | Mode dev : picker epics/tickets + orchestrator-dev |
| `--label` | `-l` | Filtrer tickets par label (requiert --dev) |
| `--assignee` | `-A` | Filtrer tickets par assignee (requiert --dev) |
| `--onboard` | | Mode onboarding : cree/enrichit le wiki projet |
| `--refresh` | | Force la re-decouverte du wiki (requiert --onboard) |
| `--yes` | `-y` | Lancer directement sans confirmation |

**Exemple :**

```bash
oh start -j mon-projet -a coder -p "Ajoute un endpoint /health"
oh start --resume abc123-def456
oh start --worktree feat/auth --dev -l "priority:high"
oh start --onboard --refresh
```

---

### oh quick

Lancement rapide. Detection auto du projet depuis le repertoire courant.

```
oh quick
```

Pas de flags. Detecte automatiquement le projet et lance une session courte.

**Exemple :**

```bash
cd ~/projects/api-gateway
oh quick
```

---

### oh audit

Lance un audit de code via opencode.

```
oh audit [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-j` | ID du projet |
| `--type` | `-t` | Type d'audit (defaut : security) |

Types disponibles : `security`, `performance`, `architecture`, `accessibility`, `ecodesign`, `observability`, `privacy`.

**Exemple :**

```bash
oh audit -j api-gateway -t security
oh audit --type performance
oh audit -t ecodesign
```

---

### oh review

Lance une review de code via opencode.

```
oh review [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-j` | ID du projet |

**Exemple :**

```bash
oh review -j frontend
oh review
```

---

### oh debug

Lance une session de debug via opencode.

```
oh debug [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-j` | ID du projet |
| `--issue` | `-i` | Description du probleme |

**Exemple :**

```bash
oh debug -j backend -i "Timeout sur les requetes POST /api/users"
oh debug --issue "Memory leak dans le worker pool"
```

---

### oh beads

Proxy vers `bd` (Beads CLI). Tous les arguments sont passes directement a `bd`.

```
oh beads [arguments...]
```

Necessite `bd` installe et accessible dans le PATH.

**Exemple :**

```bash
oh beads list
oh beads run mon-bead
oh beads status
```

---

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
| `--path` | `-p` | Chemin du projet (defaut : repertoire courant) |
| `--language` | `-l` | Langage principal |
| `--tracker` | `-t` | Issue tracker (github, gitlab, jira, linear) |

**Exemple :**

```bash
oh project add -n api -p ./services/api -l go -t github
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
| `--tracker` | `-t` | Issue tracker |

**Exemple :**

```bash
oh project configure api -P anthropic -m claude-sonnet-4-20250514
oh project configure frontend --tracker linear
```

---

## Deploiement

### oh deploy

Deploie agents, skills et config dans un projet.

```
oh deploy [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-j` | ID du projet |
| `--provider` | `-P` | Provider a configurer |
| `--model` | `-m` | Modele a configurer |
| `--check` | | Verifie si les agents/skills ont change |
| `--diff` | | Affiche les changements sans les appliquer |

**Exemple :**

```bash
oh deploy -j api
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
| `--project` | `-j` | ID du projet |
| `--all` | | Synchroniser tous les projets actifs |
| `--dry-run` | | Afficher les changements sans les appliquer |

**Exemple :**

```bash
oh sync --all
oh sync -j frontend --dry-run
```

---

## Configuration

### oh config list

Affiche la configuration.

**Alias :** `oh config ls`

```
oh config list [options]
```

| Flag | Description |
|------|-------------|
| `--json` | Sortie au format JSON |

**Exemple :**

```bash
oh config list
oh config ls --json
```

---

### oh config get

Lire une valeur de configuration.

```
oh config get <key>
```

**Exemple :**

```bash
oh config get default_provider
oh config get language
```

---

### oh config set

Definir une valeur de configuration.

```
oh config set <key> <value>
```

**Exemple :**

```bash
oh config set default_provider anthropic
oh config set language fr
```

---

### oh config unset

Supprimer une cle de configuration.

```
oh config unset <key>
```

**Exemple :**

```bash
oh config unset custom_model
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

Changer la langue de l'interface.

```
oh config language [fr|en]
```

**Exemple :**

```bash
oh config language        # Affiche la langue courante
oh config language fr     # Passe en francais
oh config language en     # Passe en anglais
```

---

### oh config websearch

Gerer les permissions de recherche web (WebSearch).

```
oh config websearch [enable|disable|status]
```

**Exemple :**

```bash
oh config websearch status
oh config websearch enable
oh config websearch disable
```

---

## Infrastructure

### oh init

Initialise oh pour la premiere fois. Wizard interactif.

```
oh init
```

Pas de flags. Configure : langue, opencode, projet, serveurs MCP, base de donnees et deploiement.

**Exemple :**

```bash
oh init
```

---

### oh doctor

Verifie l'etat du systeme.

```
oh doctor
```

Pas de flags. Checks : OS, git, opencode, bd, fzf, compatibilite, config, BDD, cles API.

**Exemple :**

```bash
oh doctor
```

---

### oh status

Affiche l'etat du hub.

```
oh status [options]
```

| Flag | Description |
|------|-------------|
| `--json` | Sortie au format JSON |

**Exemple :**

```bash
oh status
oh status --json
```

---

### oh upgrade opencode

Met a jour opencode.

```
oh upgrade opencode [version]
```

Pas de flags. Argument version optionnel (derniere version si omis).

**Exemple :**

```bash
oh upgrade opencode          # Derniere version
oh upgrade opencode 0.3.1   # Version specifique
```

---

### oh mcp serve

Lance un serveur MCP integre via stdio.

```
oh mcp serve <name>
```

Sert un serveur MCP natif (figma, gitlab, gslides).

**Exemple :**

```bash
oh mcp serve gitlab
oh mcp serve figma
oh mcp serve gslides
```

---

### oh mcp list

Liste les serveurs MCP disponibles.

**Alias :** `oh mcp ls`

```
oh mcp list [options]
```

| Flag | Description |
|------|-------------|
| `--json` | Sortie au format JSON |

**Exemple :**

```bash
oh mcp list
oh mcp ls --json
```

---

### oh service setup

Configure un service MCP. Wizard interactif. Stocke les tokens dans le keychain.

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
oh service remove [service-name]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--force` | `-f` | Supprimer sans confirmation |

**Exemple :**

```bash
oh service remove gitlab
oh service remove -f figma
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

**Alias :** `oh plugin rm`, `oh plugin uninstall`

```
oh plugin remove <name>
```

| Flag | Court | Description |
|------|-------|-------------|
| `--force` | `-f` | Supprimer sans confirmation |

**Exemple :**

```bash
oh plugin remove rtk
oh plugin rm -f rtk
```

---

### oh plugin list

Liste les plugins installes.

**Alias :** `oh plugin ls`

```
oh plugin list
```

**Exemple :**

```bash
oh plugin list
oh plugin ls
```

---

### oh plugin status

Affiche le statut des plugins installes.

```
oh plugin status
```

**Exemple :**

```bash
oh plugin status
```

---

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

## Analytique

### oh metrics

Affiche les metriques d'utilisation.

```
oh metrics [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--period` | `-p` | Periode d'analyse (7d, 30d, all). Defaut : all |

**Exemple :**

```bash
oh metrics
oh metrics -p 30d
oh metrics -p 7d
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

## Codes de sortie

| Code | Signification |
|------|---------------|
| `0` | Succes |
| `1` | Erreur |
| `2` | Avertissement |
