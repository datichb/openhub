> 🇬🇧 [Read in English](cli.en.md)

# Reference CLI

Le binaire `oh` est le point d'entree unique du hub opencode. Il orchestre les sessions IA, la gestion de projets, le deploiement de configuration et l'outillage developeur.

```
oh [--verbose] [--no-tui] <commande> [sous-commande] [options] [arguments]
```

> **TUI interactif** : `oh` sans arguments dans un terminal interactif lance le shell TUI unifié avec menu navigable. Voir la [référence TUI](tui.fr.md) pour les raccourcis et fonctionnalités.

## Flags globaux

| Flag | Court | Description |
|------|-------|-------------|
| `--verbose` | `-v` | Active la sortie verbose |
| `--no-tui` | | Force le mode CLI classique (désactive le TUI) |

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
| `--prompt` | `-m` | Prompt initial |
| `--provider` | `-P` | Provider LLM (bedrock, anthropic, openai) |
| `--project` | `-p` | ID du projet (detection auto sinon) |
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
oh start -p mon-projet -a coder -m "Ajoute un endpoint /health"
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
| `--project` | `-p` | ID du projet |
| `--type` | `-t` | Type d'audit (defaut : security) |

Types disponibles : `security`, `performance`, `architecture`, `accessibility`, `ecodesign`, `observability`, `privacy`.

**Exemple :**

```bash
oh audit -p api-gateway -t security
oh audit --type performance
oh audit -t ecodesign
```

---

### oh review

Lance une review de code via opencode avec sélection du mode.

```
oh review [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-p` | ID du projet |
| `--mode` | `-m` | Mode de review (voir ci-dessous) |

**Modes disponibles :**

| Mode | Description |
|------|-------------|
| `standard` | Review classique — checklist 6 catégories |
| `adversarial` | Critique approfondie — scepticisme maximal, min. 10 findings, hypothèses dangereuses |
| `edge-case` | Chasse aux chemins d'exécution non gérés |
| `standard+adversarial` | Les deux en parallèle (sessions indépendantes) + rapport unifié |
| `all` | Standard + Adversarial + Edge-case — couverture maximale |

Sans `--mode`, un prompt interactif propose le choix du mode au démarrage de la session.

**Exemple :**

```bash
oh review -p frontend
oh review -m adversarial
oh review -m standard+adversarial -p backend
oh review -m all
```

---

### oh debug

Lance une session de debug via opencode.

```
oh debug [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-p` | ID du projet |
| `--issue` | `-i` | Description du probleme |

**Exemple :**

```bash
oh debug -p backend -i "Timeout sur les requetes POST /api/users"
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
| `--path` | `-d` | Chemin du projet (defaut : repertoire courant) |
| `--language` | `-l` | Langage principal |
| `--tracker` | `-t` | Issue tracker (github, gitlab, jira, linear) |

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

Pas de flags. Checks : OS, git, opencode, bd, fzf, compatibilite, config, BDD, cles API. Verifie egalement la disponibilite de mises a jour pour le binaire `oh`.

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

> Voir aussi : `oh upgrade oh` pour mettre a jour le binaire `oh` lui-meme.

---

### oh upgrade oh

Met a jour le binaire `oh` en place (remplacement atomique). Uniquement pour les installations hors Homebrew.

```
oh upgrade oh [version] [--check]
```

| Flag | Description |
|------|-------------|
| `--check` | Verifie la version disponible sans telecharger |
| `version` | Version cible (defaut : derniere) |

**Exemple :**

```bash
oh upgrade oh
oh upgrade oh 1.3.0
oh upgrade oh --check
```

> **Utilisateurs Homebrew :** utilisez `brew upgrade openhub` a la place.

---

### oh mcp enable

Active un service MCP au niveau hub ou pour un projet.

```
oh mcp enable <service> [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-p` | Nom ou ID du projet |

**Exemple :**

```bash
oh mcp enable figma
oh mcp enable gitlab --project mon-projet
```

---

### oh mcp disable

Desactive un service MCP au niveau hub ou pour un projet.

```
oh mcp disable <service> [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-p` | Nom ou ID du projet |

**Exemple :**

```bash
oh mcp disable figma
oh mcp disable gitlab --project mon-projet
```

---

### oh mcp reset

Supprime l'override projet pour un service (retour a la config hub).

```
oh mcp reset <service> --project <name>
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-p` | **(requis)** Nom ou ID du projet |

**Exemple :**

```bash
oh mcp reset figma --project mon-projet
```

---

### oh mcp setup

Configure un service MCP (wizard interactif : token, options).

```
oh mcp setup [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-p` | Nom ou ID du projet |

**Exemple :**

```bash
oh mcp setup
oh mcp setup --project mon-projet
```

---

### oh mcp status

Affiche le statut de tous les services MCP.

```
oh mcp status [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-p` | Nom ou ID du projet (affiche la config effective) |

**Exemple :**

```bash
oh mcp status
oh mcp status --project mon-projet
```

---

### oh mcp serve

Lance un serveur MCP integre via stdio.

```
oh mcp serve <name>
```

Sert un serveur MCP natif (figma, gitlab, gslides, team, github, jira, linear).

**Exemple :**

```bash
oh mcp serve gitlab
oh mcp serve figma
oh mcp serve gslides
oh mcp serve github
oh mcp serve jira
oh mcp serve linear
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

### oh service setup (deprecated)

> **Deprecated :** Utilisez `oh mcp setup` a la place.

Configure un service MCP. Wizard interactif. Stocke les tokens dans le keychain.

```
oh service setup
```

---

### oh service remove (deprecated)

> **Deprecated :** Utilisez `oh mcp disable` a la place.

Supprime un service.

```
oh service remove [service-name]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--force` | `-f` | Supprimer sans confirmation |

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

### oh export

Cree une archive de sauvegarde des donnees du hub.

```
oh export [--output <chemin>]
```

Cree une archive `.tar.gz` contenant : `oh.db`, `hub.toml`, `secrets.enc` (chiffre). Inclut un fichier de checksum SHA-256.

| Flag | Court | Description |
|------|-------|-------------|
| `--output` | `-o` | Chemin de sortie (defaut : `./oh-backup-YYYY-MM-DD.tar.gz`) |

**Exemple :**

```bash
oh export
oh export --output ~/sauvegardes/oh-backup.tar.gz
```

---

### oh import

Restaure depuis une archive de sauvegarde.

```
oh import <fichier> [--overwrite] [--merge]
```

Verifie le checksum SHA-256 avant d'ecrire les donnees.

| Flag | Description |
|------|-------------|
| `--overwrite` | Ecraser les donnees existantes |
| `--merge` | Fusionner les projets uniquement (non-destructif) |

**Exemple :**

```bash
oh import oh-backup-2025-07-22.tar.gz
oh import ~/sauvegardes/oh-backup.tar.gz --overwrite
oh import ~/sauvegardes/oh-backup.tar.gz --merge
```

---

### oh repair

Diagnostique et repare la base de donnees SQLite.

```
oh repair [--check-only] [--auto]
```

Execute `PRAGMA integrity_check` sur `~/.oh/oh.db`. En cas de corruption, propose des options de recuperation : restaurer depuis une sauvegarde, reinitialiser la base de donnees, ou re-enregistrer les projets manuellement. Affiche egalement la version courante du schema.

| Flag | Description |
|------|-------------|
| `--check-only` | Diagnostiquer sans effectuer de modifications |
| `--auto` | Mode non-interactif |

**Exemple :**

```bash
oh repair
oh repair --check-only
oh repair --auto
```

---

### oh serve

Lance un tableau de bord web local.

```
oh serve [--port 8080] [--readonly]
```

Demarre un serveur HTTP lie a `127.0.0.1` uniquement (jamais expose sur le reseau). Le tableau de bord affiche les projets, sessions et la telemetrie des agents.

**Endpoints API :**
- `GET /api/v1/projects`
- `GET /api/v1/sessions`
- `GET /api/v1/metrics/agents`

| Flag | Court | Description |
|------|-------|-------------|
| `--port` | `-p` | Port (defaut : 8080) |
| `--readonly` | | Desactiver les endpoints d'ecriture (defaut : true) |

**Exemple :**

```bash
oh serve
oh serve --port 9090
oh serve --readonly=false
```

> **Securite :** Le serveur est lie a `127.0.0.1` uniquement et n'est jamais accessible depuis le reseau.

---

---

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
