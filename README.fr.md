> [Read in English](README.md)

# openhub (`oh`)

Hub central pour la gestion d'assistants IA sur plusieurs projets.
Agents partages, skills hybrides, workflow Beads integre et serveurs MCP natifs en Go.

**Binaire unique, zero dependance.**

---

## Installation

### Homebrew (recommande)

```bash
brew install datichb/openhub/oh
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
```

---

## Commandes

| Commande | Description |
|----------|-------------|
| `oh init` | Assistant de configuration initiale |
| `oh start` | Lancer une session opencode |
| `oh start --dev` | Mode dev : picker tickets + orchestrator-dev |
| `oh start --onboard` | Onboarding : creer/enrichir le wiki projet |
| `oh quick` | Tache rapide avec detection auto du projet |
| `oh deploy` | Deployer agents, skills, config, MCP |
| `oh sync` | Synchroniser tous les projets enregistres |
| `oh project list` | Lister les projets enregistres |
| `oh project add` | Enregistrer un nouveau projet |
| `oh config` | Gerer la configuration du hub |
| `oh status` | Afficher l'etat du hub et du projet |
| `oh doctor` | Diagnostic systeme |
| `oh metrics` | Metriques d'utilisation et cout |
| `oh dashboard` | Tableau de bord interactif (TUI) |
| `oh board` | Kanban des tickets (Beads) |
| `oh audit` | Audit de code via agent IA |
| `oh review` | Revue de code via agent IA |
| `oh debug` | Session de debug via agent IA |
| `oh upgrade opencode` | Mettre a jour le binaire opencode |
| `oh mcp serve` | Lancer un serveur MCP integre |
| `oh beads` | Proxy vers bd (CLI Beads) |

> Reference complete : [docs/reference/cli.fr.md](docs/reference/cli.fr.md)

---

## Architecture

```
openhub/
├── agents/          <- Definitions des roles IA (18 agents, 2 modes)
├── skills/          <- Protocoles : Bucket A (inline) + Bucket B (on-demand)
├── cli/             <- Binaire Go (oh)
│   └── internal/
│       ├── beads/       <- Integration tickets Beads
│       ├── deploy/      <- Moteur de deploiement transactionnel
│       ├── mcp/         <- Serveurs MCP natifs (figma, gitlab, gslides)
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

18 agents specialises en deux modes :

- **`primary`** -- invocable directement par l'utilisateur dans OpenCode
- **`subagent`** -- delegue par les agents coordinateurs

### Agents primaires

| Agent | Famille | Role |
|-------|---------|------|
| `orchestrator` | Coordinateur | Feature end-to-end |
| `orchestrator-dev` | Coordinateur | Implementation tickets (dirige les developers) |
| `auditor` | Coordinateur | Audit multi-domaine (7 domaines) |
| `onboarder` | Coordinateur | Decouverte projet, creation wiki |
| `planner` | Planification | Decouper features en tickets Beads |
| `designer` | Design | Analyse Figma, specs UX/UI |
| `reviewer` | Qualite | Revue PR/MR par severite |
| `qa-engineer` | Qualite | Analyse couverture de tests |
| `debugger` | Qualite | Diagnostic bugs, root cause |
| `documentarian` | Documentation | README, CHANGELOG, ADR, API docs |

### Sous-agents

| Agent | Delegue par | Domaine |
|-------|------------|---------|
| `developer` | `orchestrator-dev` | Implementation (frontend, backend, fullstack, api, mobile, data, devops, platform, security) |
| `developer-refactor` | `orchestrator-dev` | Refactoring structurel |
| `developer-migrator` | `orchestrator-dev` | Migrations incrementales |
| `auditor-subagent` | `auditor` | Tous domaines d'audit (securite, performance, accessibilite, ecoconception, architecture, vie privee, observabilite) |

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

---

## Serveurs MCP

Trois serveurs MCP integres, natifs en Go (protocole stdio) :

| Serveur | Commande | Fonction |
|---------|----------|----------|
| Figma | `oh mcp serve figma` | Extraction design tokens, analyse composants |
| GitLab | `oh mcp serve gitlab` | Gestion issues/MR, statut pipelines |
| Google Slides | `oh mcp serve gslides` | Analyse de presentations |

Configuration via `oh service setup` (stockage tokens dans le keychain OS).

---

## Documentation

### Guides

| Document | Description |
|----------|-------------|
| [Demarrage rapide](docs/guides/getting-started.fr.md) | Installation, premier deploiement |
| [Workflows](docs/guides/workflows.fr.md) | Scenarios feature, audit, debug |
| [Integration Figma](docs/guides/figma-integration.fr.md) | Configuration MCP Figma |
| [Integration GitLab](docs/guides/gitlab-integration.fr.md) | Configuration MCP GitLab |
| [Providers LLM](docs/guides/providers.fr.md) | Anthropic, Bedrock, OpenRouter, Ollama |
| [Onboarding](docs/guides/onboarding.fr.md) | Utiliser l'agent onboarder |

### Architecture

| Document | Description |
|----------|-------------|
| [Vue d'ensemble](docs/architecture/overview.fr.md) | Concepts, diagrammes |
| [Agents](docs/architecture/agents.fr.md) | Reference des 18 agents |
| [Skills](docs/architecture/skills.fr.md) | Systeme de skills hybrides |
| [ADR](docs/architecture/adr/) | 21 decisions architecturales |

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

---

## Licence

MIT
