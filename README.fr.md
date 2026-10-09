> [Read in English](README.md)

# openhub (`oh`)

Hub central pour lancer des sessions de développement assistées par IA sur vos projets : agents et skills partagés, workflows déclaratifs, suivi des tickets Beads intégré et serveurs MCP natifs en Go.

**Binaire unique, zéro dépendance.** `oh` est la tour de contrôle, [opencode](https://opencode.ai) V2 la cabine.

---

## Ce que fait oh v5

- **Workflows déclaratifs.** Chaque cas d'usage (feature, ticket, review, audit…) est un workflow YAML (`apiVersion: oh/v1`) : agent d'entrée, agents membres et leur ordre, checkpoints par mode (`manuel`, `semi-auto`, `auto`), entrées, sorties, ressources, niveau de risque et restrictions. Le hub en livre 12 ; une équipe ou un projet peut les étendre dans son dépôt team-state.
- **Paquet de session.** Plus rien n'est écrit dans vos projets. Au lancement, oh construit hors du projet un paquet de session (`~/.oh/bundles/<hash>/`, immuable) à partir du workflow : agents, skills, permissions, serveurs MCP, plugin oh.
- **Monde fermé.** Une session ne voit que les agents et les skills de son paquet (les agents natifs d'opencode sont désactivés). C'est vérifié à chaque démarrage de serveur (`Attest`) : si autre chose est visible, la session ne démarre pas.
- **Sessions pilotées depuis oh.** Un `opencode serve` par groupe, plusieurs sessions par serveur (une par ticket, un worktree par session qui écrit). Les décisions (⏸ checkpoint, ? question, ! permission, $ budget, ✗ erreur) se prennent depuis la TUI, la CLI, opencode ou le navigateur ; la première réponse gagne. Fermer opencode n'arrête pas la session ; un serveur inactif se met en veille après 5 minutes et reprend sur le même paquet.
- **Les secrets restent sur la machine.** Le démon `ohd` héberge un proxy d'identifiants : opencode ne reçoit qu'un jeton de groupe (`ohs_…`), la vraie clé LLM reste dans votre trousseau.
- **Local, conteneur ou distant.** Une session tourne sur votre machine, dans un conteneur construit depuis le Dockerfile de développement du projet (Colima, Podman, Docker CLI), ou dans un job GitLab CI. Beads reste toujours sur la machine.

---

## Installation

### 1. oh

**Homebrew (recommandé) :**

```bash
brew install datichb/tap/openhub
```

**Script curl :**

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/openhub/main/install.sh | bash
```

**Depuis les sources :**

```bash
cd cli && go install .
```

### 2. opencode V2 (2.0.0 ou plus)

oh n'installe plus opencode. Installez-le avec son propre outil :

```bash
brew install anomalyco/tap/opencode    # ou voir https://opencode.ai
```

opencode V1 n'est plus pris en charge : oh refuse de lancer une session, avec un message clair. `oh doctor` vérifie la version. Vous venez d'oh v4 : voir [Migrer vers oh v5](docs/guides/migration-v5.fr.md).

---

## Démarrage rapide

```bash
oh init                              # Premier réglage : langue, vérification d'opencode V2, projet, MCP, espace des workflows
oh run quick -i request="Corriger la coquille de l'en-tête"   # Petite modification, un seul agent
oh run feature                       # Feature de bout en bout (plan, checkpoints, implémentation, review)
oh run ticket --tickets bd-42,bd-43  # Une session par ticket Beads (un serveur, un worktree chacune)
oh session inbox                     # Décisions en attente de toutes les sessions
oh                                   # TUI : Démarrer, Sessions, catalogue des workflows, réglages
```

Options utiles de `oh run` : `--recap` (récapitulatif et confirmation), `--mode manuel|semi-auto|auto`, `--runtime local|container|remote`, `--location base|new|<chemin>`, `--headless`, `-a/--agent` (avec `libre`). Sans argument, `oh run` lance le workflow par défaut du projet.

Dans la TUI, **Démarrer** liste les workflows (★ épinglés, récents), ouvre la fiche de lancement (Entrées → Options → Récap, `Ctrl+S` lance), et la vue **Sessions** rassemble les sessions à traiter, en cours, en veille, terminées et à récupérer.

---

## Commandes principales

| Commande | Description |
|----------|-------------|
| `oh` | TUI interactive (terminal interactif) |
| `oh init` | Assistant de configuration initiale |
| `oh run [workflow]` | Lancer un workflow (une session par ticket avec `--tickets`) |
| `oh workflow list` · `show` · `validate` | Catalogue des workflows, workflow résolu (avec les origines), validation |
| `oh workflow new` · `edit` · `diff` · `publish` · `history` · `restore` · `archive` | Workflows d'équipe ou de projet (brouillons, publication, historique) |
| `oh bundle show <workflow>` · `build` | Paquet de session d'un workflow (agents, skills, permissions, `--budget`) |
| `oh session list` · `inbox` | Sessions (en cours, en attente, en veille ; `--all`) et décisions en attente |
| `oh session attach <id>` · `follow` · `open` | Ouvrir une session (onglet, tmux, navigateur, terminal courant), la suivre en direct, l'ouvrir dans le navigateur |
| `oh session approve` · `answer` · `dismiss` | Répondre à un checkpoint ou une permission, à une question, classer une alerte |
| `oh session send` · `interrupt` · `model` · `compact` · `fork` | Piloter une session en cours |
| `oh session results <id>` | Fichiers modifiés, branche, coût ; `--mr` (description de MR), `--patch` |
| `oh session resume` · `stop` | Reprendre une session en veille, arrêter une session |
| `oh session fetch` · `resolve` | Récupérer une session distante terminée, rejouer son journal Beads |
| `oh daemon status` · `stop` | Démon oh (proxy d'identifiants, supervision des sessions) |
| `oh budget show` · `set` · `unset` · `raise` | Restrictions des sessions (désactivées par défaut) |
| `oh remote setup` · `status` | Exécution distante sur GitLab CI (projet `oh-runner`) |
| `oh migrate deploy-cleanup` | Retirer des projets ce qu'avait laissé l'ancien `oh deploy` |
| `oh project list` · `add` · `configure` · `remove` | Projets enregistrés |
| `oh config list` · `oh config model default <m>` | Configuration du hub, cascade des modèles |
| `oh provider setup` · `oh mcp setup` · `oh secrets set` | Identifiants LLM, jetons MCP, secrets (trousseau du système) |
| `oh team init [--solo]` · `oh team promote` | Fonctions d'équipe, espace solo et son partage |
| `oh team claim` · `release` · `status` · `board` | Réservation des tickets et vue d'équipe |
| `oh doctor` · `oh status` · `oh repair` | Diagnostic, état, réparation de la base |
| `oh metrics` · `oh dashboard` · `oh board` · `oh serve` | Usage et coûts, tableau de bord TUI, kanban Beads, tableau de bord web local |
| `oh export` · `oh import` | Sauvegarde et restauration |
| `oh upgrade oh` | Mise à jour d'oh (hors Homebrew) |
| `oh skill check` | Contrôle du catalogue des skills (doublons, `requires:`, frontmatter, annexes) |
| `oh beads …` | Relais vers `bd` (CLI Beads) |

**Alias dépréciés** (avertissement, puis `oh run`) : `oh start` → `oh run feature`, `oh start --agent X` → `oh run libre --agent X`, `oh start --dev` → `oh run ticket`, `oh audit` → `oh run audit`, `oh review` → `oh run review` (`--publish` reste une commande d'oh), `oh debug` → `oh run debug`. `oh deploy` et `oh sync` affichent seulement un message de migration.

> Référence complète : [docs/reference/cli.fr.md](docs/reference/cli.fr.md) · [CLI — Workflows](docs/reference/cli-workflows.fr.md) · [CLI — Sessions](docs/reference/cli-sessions.fr.md) · [Sessions v5](docs/guides/sessions-v5.fr.md)

---

## Workflows

Les 12 workflows livrés par le hub :

| Workflow | Agent d'entrée | Usage |
|----------|----------------|-------|
| `feature` | `orchestrator` | Feature de bout en bout : plan, conception, implémentation, review |
| `ticket` | `orchestrator-dev` | Tickets Beads prêts à coder (`--tickets a,b` : une session par ticket) |
| `quick` | `developer` | Petite modification bien délimitée |
| `cadrage` | `conductor` | Cadrer sans implémenter |
| `onboarding` | `onboarder` | Découvrir un projet, écrire son wiki (`docs/wiki/`) |
| `review` | `reviewer` | Relire une branche (lecture seule) |
| `review-feedback` | `orchestrator-dev` | Traiter les retours d'une review |
| `audit` | `auditor` | Audit multi-domaines (`-i type=security`…) |
| `debug` | `debugger` | Diagnostiquer un bug (`-i issue="…"`) |
| `sweep` | `conductor` | Série de modifications guidée par un but (`-i goal=…`) |
| `brief-enrich` | `brief-enricher` | Enrichir un brief de reprise (sans interface) |
| `libre` | au choix (`--agent`) | Session libre avec l'agent de votre choix |

Les workflows se résolvent **par couches** : hub < équipe < projet < options de session. Une couche peut en étendre une autre (`extends`) et verrouiller des champs (`enforce`) ; la sécurité ne peut que se durcir. Voir [Workflows livrés](docs/reference/workflows.fr.md), [Schéma des workflows](docs/reference/workflow-schema.fr.md) et [Workflows d'équipe](docs/guides/team-workflows.fr.md).

---

## Agents

20 agents en 7 familles. Leur mode par défaut vient de leur frontmatter ; un workflow choisit les membres d'une session et peut changer leur mode.

- **`primary`** : point d'entrée d'une session
- **`subagent`** : délégué par un autre agent du workflow

### Agents primaires (15)

| Agent | Famille | Rôle |
|-------|---------|------|
| `conductor` | Planning | Agent d'entrée générique : suit la carte du workflow, lance les agents dans l'ordre, passe les checkpoints |
| `orchestrator` | Planning | Coordinateur de feature de bout en bout |
| `orchestrator-dev` | Planning | Implémentation des tickets (pilote les développeurs) |
| `planner` | Planning | Découpage des features en tickets Beads |
| `pathfinder` | Planning | Reconnaissance rapide, estimation de complexité |
| `onboarder` | Planning | Découverte du projet, création du wiki |
| `auditor` | Auditor | Coordinateur d'audit multi-domaines (7 domaines) |
| `designer` | Design | Analyse Figma, specs UX/UI (recon, ux, ui, ux+ui) |
| `reviewer` | Quality | Review de PR/MR par sévérité (standard, adversarial, edge-case) |
| `debugger` | Quality | Diagnostic de bug, cause racine |
| `benchmarker` | Quality | Benchmarks de performance Lighthouse, k6, pprof, py-spy |
| `test-generator` | Quality | Analyse des manques, tests unitaires/intégration/property-based |
| `database` | Developer | Schéma, migrations, optimisation des requêtes, audit de sécurité BD |
| `infra` | Developer | Review Terraform/K8s, estimation des coûts, sécurité IaC |
| `documentarian` | Documentation | README, CHANGELOG, ADR, docs d'API |

### Sous-agents (5)

| Agent | Délégué par | Domaine |
|-------|-------------|---------|
| `developer` | `orchestrator-dev` | Implémentation (frontend, backend, fullstack, api, mobile, data, devops, platform, security) |
| `developer-refactor` | `orchestrator-dev` | Refactoring structurel |
| `developer-migrator` | `orchestrator-dev` | Migrations incrémentales |
| `auditor-subagent` | `auditor` | Tous les domaines d'audit (sécurité, performance, accessibilité, écoconception, architecture, vie privée, observabilité) |
| `brief-enricher` | workflow `brief-enrich` | Enrichissement d'un brief de reprise, en lecture seule |

Voir [Agents](docs/architecture/agents.fr.md).

---

## Environnements d'exécution

| Environnement | Où tourne opencode | Ce qui reste sur la machine |
|---------------|--------------------|-----------------------------|
| local | `opencode serve` sur votre machine | tout |
| conteneur | un conteneur par groupe (Colima, Podman, Docker CLI), image = Dockerfile de dev du projet + couche oh, paquet monté en lecture seule | clés (proxy), Beads (passerelle), serveurs MCP d'oh (passerelle) |
| distant | job GitLab CI du projet `oh-runner` | vos clés ; Beads : instantané en entrée, journal en sortie, rejoué localement (`oh session resolve`) |

Choix par `oh run --runtime …`, sinon le réglage du projet, les Réglages, puis le défaut du workflow, dans la limite de `runtime.allowed` du workflow. Conteneur et distant fonctionnent sous macOS et Linux. Voir [Conteneur](docs/guides/container.fr.md) et [Runners distants](docs/guides/remote-runners.fr.md).

---

## Arborescence du dépôt

```
openhub/
├── agents/              <- Définitions des agents (20 agents, 7 familles)
├── skills/              <- Protocoles : Bucket A (inline) + Bucket B (à la demande)
├── workflows/           <- Workflows livrés (oh/v1) + gabarits de prompt
├── permissions/         <- Bases de permissions des agents
├── cli/                 <- Binaire Go (oh)
│   ├── cmd/             <- Commandes Cobra et câblage de la TUI (oh-bd : faux bd des conteneurs)
│   └── internal/
│       ├── workflow/      <- Schéma oh/v1, résolution par couches, validation, prompts
│       ├── services/      <- Services partagés CLI/TUI (workflow, session, checkpoint, remote)
│       ├── bundle/        <- Paquets de session (~/.oh/bundles/<hash>/)
│       ├── bricks/        <- Agents, skills, permissions, cascade des modèles
│       ├── sessionspec/   <- Modèle neutre (SessionSpec, BundleSpec)
│       ├── adapters/      <- Adaptateurs d'outil (opencode V2 : rendu, serveur, Attest, plugin)
│       ├── runsvc/        <- Lancement : groupes de serveurs, sessions, worktrees, veille, reprise
│       ├── daemon/        <- Démon ohd : supervision, décisions, flux, notifications
│       ├── credproxy/     <- Proxy d'identifiants LLM
│       ├── gateway/       <- Passerelles Beads et MCP
│       ├── runtime/       <- Environnements d'exécution (local, conteneur)
│       ├── remote/        <- Contrat machine ↔ job GitLab CI, pipeline généré
│       ├── limits/        <- Restrictions des sessions (I6)
│       ├── teamstate/     <- Dépôt team-state (claims, workflows, catalogue, solo)
│       ├── mcp/           <- Serveurs MCP natifs (figma, gitlab, gslides, github, jira, linear, team, workflow)
│       ├── deploycleanup/ <- Nettoyage des anciens déploiements
│       ├── storage/       <- SQLite (oh.db), trousseau, chiffrement de fichiers
│       ├── tui/           <- TUI tview/tcell
│       └── ...            <- beads, config, i18n, prompt, tracker, worktree, termlaunch, selfupdate…
└── docs/                <- Documentation (bilingue fr/en)
```

Les données de l'utilisateur sont dans `~/.oh/` (hors dépôt) : `hub.toml`, `oh.db`, contenu du hub extrait, paquets de session, sessions, groupes de serveurs, espaces solo. Voir [Vue d'ensemble de l'architecture](docs/architecture/overview.fr.md).

---

## Serveurs MCP

Serveurs MCP intégrés, natifs en Go (stdio, `oh mcp serve <nom>`). Ceux activés pour le projet sont placés dans le paquet de session ; un workflow peut les filtrer (`mcp:`).

| Serveur | Rôle | Prérequis |
|---------|------|-----------|
| `figma` | Extraction des design tokens, analyse des composants | `FIGMA_TOKEN` |
| `gitlab` | Issues, MR, pipelines | `GITLAB_TOKEN` |
| `gslides` | Analyse de présentations | Identifiants Google |
| `github` | Issues, PR, Actions | `GITHUB_TOKEN` |
| `jira` | Issues, transitions | `JIRA_URL` + `JIRA_TOKEN` |
| `linear` | Issues/mutations via GraphQL | `LINEAR_API_KEY` |
| `team` | Coordination d'équipe, claims, wiki | Dépôt team-state |
| `workflow` | État du workflow, checkpoints, sorties typées (ajouté à toute session) | — |

Configuration par `oh mcp setup` (jetons rangés dans le trousseau du système ; le serveur lit lui-même son jeton). Hors machine (conteneur), les serveurs MCP d'oh tournent sur la machine, derrière la passerelle MCP du démon.

---

## Documentation

| Sujet | Documents |
|-------|-----------|
| Démarrer | [Prise en main](docs/guides/getting-started.fr.md) · [Tutoriel](docs/guides/tutorial.fr.md) · [Migrer vers oh v5](docs/guides/migration-v5.fr.md) |
| Sessions | [Sessions v5](docs/guides/sessions-v5.fr.md) · [Workflows (scénarios)](docs/guides/workflows.fr.md) · [Review et retours](docs/guides/review-feedback.fr.md) |
| Workflows | [Workflows livrés](docs/reference/workflows.fr.md) · [Schéma des workflows](docs/reference/workflow-schema.fr.md) · [Workflows d'équipe](docs/guides/team-workflows.fr.md) |
| Exécution | [Conteneur](docs/guides/container.fr.md) · [Runners distants](docs/guides/remote-runners.fr.md) |
| Architecture | [Vue d'ensemble](docs/architecture/overview.fr.md) · [Agents](docs/architecture/agents.fr.md) · [Skills](docs/architecture/skills.fr.md) · [ADR](docs/architecture/adr/) (51) |
| Référence | [CLI](docs/reference/cli.fr.md) · [Configuration](docs/reference/config.fr.md) · [Glossaire](docs/reference/glossary.fr.md) · [Modèle Beads](docs/reference/beads-model.fr.md) |
| Exploitation | [Dépannage](docs/guides/troubleshooting.fr.md) · [Fournisseurs](docs/guides/providers.fr.md) · [Sauvegarde et restauration](docs/guides/backup-restore.fr.md) · [Tableau de bord](docs/guides/dashboard.fr.md) |

Index complet : [docs/README.md](docs/README.md).

---

## Migration

- **Depuis oh v4** (opencode V1, `oh deploy`) : [Migrer vers oh v5](docs/guides/migration-v5.fr.md). Restes des anciens déploiements : `oh migrate deploy-cleanup`.
- **Depuis la CLI bash `oc`** : [Guide de migration](MIGRATION.md) (équivalence des commandes, hub.json → hub.toml).

---

## Prérequis

- **[opencode](https://opencode.ai) V2** (2.0.0 ou plus), installé avec son propre outil
- **[git](https://git-scm.com/)**
- **[Beads](https://beads.sh/)** *(optionnel)* : suivi des tickets pour `oh run ticket`, `oh board`
- **Colima, Podman ou Docker CLI** *(optionnel)* : sessions en conteneur
- **GitLab avec des runners CI** *(optionnel)* : sessions distantes

Pas besoin de Node.js, jq, sqlite3 ni bun. Le binaire Go est autonome.

**Plateformes :** macOS et Linux (amd64, arm64) ; Windows (amd64, arm64) en local uniquement (démon dans le processus oh, ni conteneur ni distant).

---

## Contribuer

Voir [CONTRIBUTING.md](CONTRIBUTING.md) pour l'environnement de développement, les conventions et le processus de PR.

## Sécurité

Voir [SECURITY.fr.md](SECURITY.fr.md) pour signaler une vulnérabilité et pour le modèle de sécurité (proxy d'identifiants, démon, monde fermé, passerelles, conteneur et distant).

---

## Licence

MIT
