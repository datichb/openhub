> [Read in English](README.md)

# openhub (`oh`)

Hub central pour la gestion d'assistants IA sur plusieurs projets.
Agents partages, skills hybrides, workflow Beads integre et serveurs MCP natifs en Go.

**Binaire unique, zero dependance.**

---

## Installation

### Homebrew (recommande)

```bash
brew install datichb/tap/openhub
```

### Script curl

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/openhub/main/install.sh | bash
```

### Depuis les sources

```bash
cd cli && go install .
```

---

## Demarrage rapide

```bash
oh init                        # Premier setup : langue, opencode, projet, MCP
oh start                       # Lance opencode (detection auto du projet)
oh start --dev                 # Mode dev : choix epics/tickets, orchestrator-dev
oh start --onboard             # Cree le wiki projet (docs/wiki/)
oh deploy                      # Synchronise agents, skills, config, MCP
oh serve                       # Dashboard web local sur http://127.0.0.1:8080
```

---

## Commandes

| Commande | Description |
|----------|-------------|
| `oh init` | Assistant de configuration initiale |
| `oh start` | Lancer une session opencode |
| `oh start --dev` | Mode dev : picker tickets + orchestrator-dev |
| `oh start --onboard` | Onboarding : creer/enrichir le wiki projet |
| `oh start --recap` | Lancer avec récap de configuration |
| `oh deploy` | Deployer agents, skills, config, MCP |
| `oh sync` | Synchroniser tous les projets enregistres |
| `oh project list` | Lister les projets enregistres |
| `oh project add` | Enregistrer un nouveau projet |
| `oh config` | Gerer la configuration du hub |
| `oh status` | Afficher l'etat du hub et du projet |
| `oh doctor` | Diagnostic systeme |
| `oh metrics` | Metriques d'utilisation et cout (incl. telemetrie agents) |
| `oh dashboard` | Tableau de bord interactif (TUI) |
| `oh board` | Kanban des tickets (Beads) |
| `oh serve [--port 8080] [--readonly]` | Dashboard web local (API + SPA, 127.0.0.1 uniquement) |
| `oh audit` | Audit de code via agent IA |
| `oh review` | Revue de code via agent IA |
| `oh debug` | Session de debug via agent IA |
| `oh export [--output path]` | Sauvegarde DB + config + secrets en .tar.gz avec checksum SHA-256 |
| `oh import <file> [--overwrite] [--merge]` | Restauration depuis une archive de sauvegarde |
| `oh repair [--check-only] [--auto]` | Diagnostic et reparation de la base SQLite corrompue |
| `oh upgrade opencode` | Mettre a jour le binaire opencode |
| `oh upgrade oh [--check] [version]` | Auto-mise a jour du binaire oh (hors Homebrew) |
| `oh mcp serve` | Lancer un serveur MCP integre |
| `oh skill add <source>` | Installer un skill communautaire (nom ou URL Git) |
| `oh skill list` | Lister les skills communautaires installes |
| `oh skill remove <name>` | Desinstaller un skill communautaire |
| `oh skill search [query]` | Rechercher dans l'index communautaire |
| `oh beads` | Proxy vers bd (CLI Beads) |

> Reference complete : [docs/reference/cli.fr.md](docs/reference/cli.fr.md)

---

## Architecture

```
openhub/
├── agents/          <- Definitions des roles IA (19 agents, 7 familles)
├── skills/          <- Protocoles : Bucket A (inline) + Bucket B (on-demand)
├── cli/             <- Binaire Go (oh)
│   └── internal/
│       ├── beads/       <- Integration tickets Beads
│       ├── deploy/      <- Moteur de deploiement transactionnel
│       ├── mcp/         <- Serveurs MCP natifs (figma, gitlab, gslides, github, jira, linear, team)
│       ├── skillregistry/ <- Decouverte et installation de skills communautaires
│       ├── selfupdate/  <- Auto-mise a jour du binaire oh
│       ├── tui/         <- Vues BubbleTea (dashboard, board, picker)
│       └── ...
└── docs/            <- Documentation (bilingue fr/en)
```

**Flux de deploiement :**

```
oh deploy
  -> .opencode/agents/*.md        (definitions d'agents)
  -> .opencode/skills/*/SKILL.md  (protocoles)
  -> opencode.json                (provider, model, MCP, permissions)
```

---

## Agents

19 agents specialises en 7 familles, deux modes :

- **`primary`** -- invocable directement par l'utilisateur dans OpenCode
- **`subagent`** -- delegue par les agents coordinateurs

### Agents primaires (14)

| Agent | Famille | Role |
|-------|---------|------|
| `orchestrator` | Planning | Coordinateur feature end-to-end |
| `orchestrator-dev` | Planning | Implementation tickets (dirige les developers) |
| `planner` | Planning | Decouper features en tickets Beads |
| `pathfinder` | Planning | Reconnaissance rapide, estimation complexite |
| `onboarder` | Planning | Decouverte projet, creation wiki |
| `auditor` | Auditor | Coordinateur audit multi-domaine (7 domaines) |
| `designer` | Design | Analyse Figma, specs UX/UI (4 modes : recon, ux, ui, ux+ui) |
| `reviewer` | Qualite | Revue PR/MR par severite (multi-mode : standard, adversarial, edge-case) |
| `debugger` | Qualite | Diagnostic bugs, root cause |
| `benchmarker` | Qualite | Benchmarks Lighthouse, k6, pprof, py-spy |
| `test-generator` | Qualite | Analyse des lacunes, generation tests unitaires/integration/property-based |
| `database` | Developer | Schema, migration, optimisation requetes, audit securite DB |
| `infra` | Developer | Revue Terraform/K8s, estimation couts, securite IaC |
| `documentarian` | Documentation | README, CHANGELOG, ADR, API docs |

### Sous-agents (5)

| Agent | Delegue par | Domaine |
|-------|------------|---------|
| `developer` | `orchestrator-dev` | Implementation (frontend, backend, fullstack, api, mobile, data, devops, platform, security) |
| `developer-refactor` | `orchestrator-dev` | Refactoring structurel |
| `developer-migrator` | `orchestrator-dev` | Migrations incrementales |
| `auditor-subagent` | `auditor` | Tous domaines d'audit (securite, performance, accessibilite, ecoconception, architecture, vie privee, observabilite) |
| `brief-enricher` | Divers | Enrichissement takeover brief (lecture seule) |

---

## Workflows cles

| Scenario | Commande | Agent |
|----------|----------|-------|
| Feature complete | `oh start -a orchestrator` | orchestrator |
| Tickets prets | `oh start --dev` | orchestrator-dev |
| Audit pre-production | `oh audit --type security` | auditor |
| Bug production | `oh debug --issue "..."` | debugger |
| Spec UX/UI depuis Figma | `oh start -a designer` | designer |
| Documenter une feature | `oh start -a documentarian` | documentarian |
| Decouvrir un projet | `oh start --onboard` | onboarder |
| Planifier sans implementer | `oh start -a planner` | planner |
| Revue d'une branche | `oh review` | reviewer |
| Parallele multi-tickets | `oh start --parallel` | orchestrator-dev |

## Commandes

### Sessions

| Commande | Description |
|----------|-------------|
| `oh start --recap` | Lancer avec récap de configuration + confirmation |
| `oh start` | Lancer immédiatement (mode rapide par défaut) |
| `oh start --dev` | Mode dev : choisir des tickets a implementer |
| `oh start --onboard` | Decouvrir et documenter un codebase |
| `oh start --parallel` | Sessions paralleles sur plusieurs tickets |
| `oh audit --type <t>` | Audit de code (security, performance, architecture, accessibility, ecodesign, observability) |
| `oh review` | Revue de code (standard, adversarial, edge-case, complete) |
| `oh debug --issue "..."` | Session de debogage |

### Projets et deploiement

| Commande | Description |
|----------|-------------|
| `oh project add` | Enregistrer un nouveau projet |
| `oh project list` | Lister tous les projets |
| `oh project configure` | Configurer les parametres d'un projet |
| `oh project remove` | Desenregistrer un projet |
| `oh deploy` | Deployer agents/skills dans le projet |
| `oh sync --all` | Synchroniser vers tous les projets |

### Configuration

| Commande | Description |
|----------|-------------|
| `oh init` | Assistant de configuration initiale |
| `oh config list` | Afficher tous les parametres |
| `oh config model default <m>` | Definir le modele par defaut |
| `oh provider setup` | Configurer les credentials provider |
| `oh mcp setup` | Configurer les tokens MCP |
| `oh secrets set <cle> <val>` | Stocker un secret |

### Collaboration equipe

| Commande | Description |
|----------|-------------|
| `oh team init` | Configurer les fonctionnalites equipe |
| `oh team claim <id>` | Revendiquer un ticket |
| `oh team release <id>` | Liberer un ticket |
| `oh team status` | Vue d'ensemble equipe |
| `oh team activity` | Evenements recents de l'equipe |
| `oh team board` | Kanban equipe |
| `oh team sync-tracker` | Synchroniser claims vers le tracker externe |
| `oh teams list` | Lister toutes les equipes |
| `oh takeover-brief show <id>` | Voir le contexte de reprise |

### Qualite et gouvernance

| Commande | Description |
|----------|-------------|
| `oh conventions check` | Valider les conventions |
| `oh patterns list` | Lister les patterns d'equipe |
| `oh policies check` | Valider les policies |
| `oh worktree list` | Lister les worktrees actifs |

### Systeme

| Commande | Description |
|----------|-------------|
| `oh doctor` | Verification de sante (version, credentials, MCP) |
| `oh status` | Statut hub et projet |
| `oh metrics` | Stats d'usage et couts par agent |
| `oh serve` | Demarrer le dashboard web local |
| `oh export` | Exporter les donnees du hub |
| `oh import` | Importer/restaurer les donnees |
| `oh repair` | Reparer l'etat corrompu |
| `oh upgrade oh` | Mettre a jour le binaire oh |
| `oh upgrade opencode` | Mettre a jour le binaire opencode |
| `oh plugin list` | Lister les plugins installes |

---

## Serveurs MCP

Sept serveurs MCP integres, natifs en Go (protocole stdio) :

| Serveur | Commande | Fonction | Requis |
|---------|----------|----------|--------|
| Figma | `oh mcp serve figma` | Extraction design tokens, analyse composants | `FIGMA_TOKEN` |
| GitLab | `oh mcp serve gitlab` | Gestion issues/MR, statut pipelines | `GITLAB_TOKEN` |
| Google Slides | `oh mcp serve gslides` | Analyse de presentations | Credentials Google |
| GitHub | `oh mcp serve github` | Issues, PRs, Actions | `GITHUB_TOKEN` |
| Jira | `oh mcp serve jira` | Gestion issues, transitions | `JIRA_URL` + `JIRA_TOKEN` |
| Linear | `oh mcp serve linear` | Issues/mutations via GraphQL | `LINEAR_API_KEY` |
| Team | `oh mcp serve team` | Coordination equipe, claims, wiki | Repo team-state |

Configuration via `oh mcp setup` (stockage tokens dans le keychain OS).

---

## Documentation

### Guides

| Document | Description |
|----------|-------------|
| [Demarrage rapide](docs/guides/getting-started.fr.md) | Installation, premier deploiement |
| [Workflows](docs/guides/workflows.fr.md) | Scenarios feature, audit, debug |
| [Integration Figma](docs/guides/figma-integration.fr.md) | Configuration MCP Figma |
| [Integration GitLab](docs/guides/gitlab-integration.fr.md) | Configuration MCP GitLab |
| [Integration GitHub](docs/guides/github-integration.fr.md) | Configuration MCP GitHub |
| [Integration Jira](docs/guides/jira-integration.fr.md) | Configuration MCP Jira |
| [Integration Linear](docs/guides/linear-integration.fr.md) | Configuration MCP Linear |
| [Marketplace de skills](docs/guides/skill-marketplace.fr.md) | Installer des skills communautaires |
| [Dashboard](docs/guides/dashboard.fr.md) | Configuration et utilisation du dashboard web |
| [Sauvegarde & Restauration](docs/guides/backup-restore.fr.md) | Export/import, reparation |
| [Providers LLM](docs/guides/providers.fr.md) | Anthropic, Bedrock, OpenRouter, Ollama |
| [Onboarding](docs/guides/onboarding.fr.md) | Utiliser l'agent onboarder |

### Architecture

| Document | Description |
|----------|-------------|
| [Vue d'ensemble](docs/architecture/overview.fr.md) | Concepts, diagrammes |
| [Agents](docs/architecture/agents.fr.md) | Reference des 19 agents |
| [Skills](docs/architecture/skills.fr.md) | Systeme de skills hybrides |
| [ADR](docs/architecture/adr/) | 32 decisions architecturales |

### Reference

| Document | Description |
|----------|-------------|
| [Reference CLI](docs/reference/cli.fr.md) | Toutes les commandes avec options et exemples |
| [Configuration](docs/reference/config.fr.md) | hub.toml, parametres projet |
| [Modele Beads](docs/reference/beads-model.fr.md) | Reference systeme de tickets |

---

## Migration depuis `oc`

Si vous utilisiez la CLI bash (`oc`), consultez le [Guide de migration](MIGRATION.md) pour :
- Table d'equivalence des commandes
- Migration de configuration (hub.json -> hub.toml)
- Breaking changes

---

## Prerequis

- **[OpenCode](https://opencode.ai)** -- agent de code IA (telecharge automatiquement par `oh init`)
- **[git](https://git-scm.com/)** -- controle de version
- **[Beads](https://beads.sh/)** *(optionnel)* -- tracker de tickets pour `oh start --dev`, `oh board`

Aucun Node.js, jq, sqlite3 ou bun requis. Le binaire Go est autonome.

**Plateformes supportees :** macOS (amd64/arm64), Linux (amd64/arm64), Windows (amd64/arm64).

---

## Licence

MIT
