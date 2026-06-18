> 🇬🇧 [Read in English](README.md)

# openhub

Hub central pour piloter des assistants IA sur plusieurs projets,
avec des agents partagés, des skills hybrides et un workflow Beads intégré.

Supporte **OpenCode**.

---

## Comment ça marche

openhub repose sur trois concepts : **agents**, **serveurs MCP** et **déploiement**.

- Les **agents** définissent les rôles IA (qui fait quoi, comment, dans quel ordre) avec leurs protocoles intégrés. Chaque agent possède des **skills Bucket A** (protocoles obligatoires, toujours inline) et des **skills Bucket B** (contexte de domaine, chargées à la demande).
- Les **serveurs MCP** fournissent des intégrations outils (Figma, Linear, etc.) disponibles pour tous les agents.
- Le **déploiement** assemble agents + serveurs MCP et les copie dans vos projets cibles.

```
openhub/          ← source de vérité (éditer ici, jamais dans les projets)
├── agents/            ← identité des rôles IA avec protocoles intégrés
├── skills/            ← protocoles : Bucket A (inline) + Bucket B (natif, à la demande)
├── servers/           ← serveurs MCP (intégration Figma, etc.)
└── scripts/           ← assemblage et déploiement

         oc deploy opencode MON-APP
openhub  ──────────────────────►  mon-app/.opencode/agents/*.md        (Bucket A inline)
                                   ├►  mon-app/.opencode/skills/*/SKILL.md   (Bucket B natif)
                                   ├►  mon-app/.opencode/servers/
                                   └►  mon-app/opencode.json

```

Résultat : 19 agents spécialisés + intégration Figma, toujours à jour, disponibles dans tous vos projets
depuis une source de vérité unique.

---

## Prérequis

- **[OpenCode](https://opencode.ai)** — agent de développement IA
- **[jq](https://jqlang.github.io/jq/)** — requis pour `oc deploy` (génère `opencode.json`)
- **[git](https://git-scm.com/)**, **[node](https://nodejs.org/)**, **[bun](https://bun.sh/)** — outillage standard
- **[sqlite3](https://sqlite.org/)** — requis pour `oc metrics` et `oc dashboard` (lit la base de sessions OpenCode). Natif sur macOS ; sur Linux : `sudo apt-get install sqlite3`
- **[Beads](https://beads.sh/)** *(optionnel)* — intégration gestionnaire de tâches pour les vues tickets dans les métriques et le dashboard

Le script d'installation détecte et installe les dépendances manquantes automatiquement (Homebrew sur macOS, apt-get sur Linux).

---

## Installation

### One-liner (recommandé)

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/openhub/main/install.sh | bash
```

Le script automatise tout : clone dans `~/.openhub`, vérification des dépendances
avec confirmation, création de l'alias `oc`, et configuration du fournisseur LLM.

Après l'installation, recharger le shell :

```bash
source ~/.zshrc   # ou source ~/.bashrc
```

### Installation manuelle

```bash
git clone https://github.com/datichb/openhub.git ~/.openhub
echo 'alias oc="~/.openhub/oc.sh"' >> ~/.zshrc && source ~/.zshrc
oc install
```

### Installer une version spécifique

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/openhub/main/install.sh | VERSION=v1.0.0 bash
```

---

## Mise à jour

### Mettre à jour les sources du hub

```bash
oc upgrade           # pull le dernier main
oc upgrade v1.1.0    # basculer sur un tag de release spécifique
```

### Mettre à jour les outils installés (opencode, Beads)

```bash
oc update
```

---

## Désinstallation

```bash
oc uninstall
# ou directement :
bash ~/.openhub/uninstall.sh
```

Guide interactif en 4 étapes — tout est optionnel et demande confirmation :

| Étape | Action | Défaut |
|-------|--------|--------|
| 1 | Nettoyer les agents déployés dans les projets (`.opencode/agents/`, `opencode.json`) | `[y/N]` |
| 2 | Supprimer le hub (`~/.openhub`) | `[y/N]` |
| 3 | Retirer l'alias `oc` et les exports bun du fichier rc | `[Y/n]` |
| 4 | Désinstaller opencode, Beads, bun (séparément) | `[y/N]` |

> `jq` et `node` ne sont jamais désinstallés. Un backup `.bak` est créé avant toute
> modification du fichier rc.

---

## Démarrage rapide

```bash
# 1. Enregistrer un projet
oc init MON-APP ~/workspace/mon-app

# 2. Déployer les agents dans le projet
oc deploy opencode MON-APP

# 3. Lancer l'outil dans le projet
oc start MON-APP
```

> Guide complet : [docs/guides/getting-started.fr.md](docs/guides/getting-started.fr.md)

---

## Agents disponibles

19 agents en deux modes :

- **`primary`** — visibles directement dans l'outil IA (tab picker OpenCode). Invocables par l'utilisateur.
- **`subagent`** — invisibles dans le picker. Invocables uniquement par délégation depuis un agent coordinateur.

### Agents primaires (invocables directement)

| Agent | Famille | Rôle |
|-------|---------|------|
| `orchestrator` | Coordinateur | Feature de A à Z — délègue spec, audit, implémentation |
| `orchestrator-dev` | Coordinateur | Implémentation de tickets — pilote l'agent `developer` (domaine précisé à l'invocation) |
| `auditor` | Coordinateur | Audit multi-domaine — délègue aux 7 `auditor-*` |
| `onboarder` | Coordinateur | Découverte de projet existant — détecte la stack, le domaine métier, les designs Figma, la stratégie de test, et produit un rapport de contexte |
| `planner` | Planification | Décompose une feature en tickets Beads |
| `ux-designer` | Design | Spec UX — user flows, critères d'acceptance |
| `ui-designer` | Design | Spec UI — tokens, composants, guidelines visuelles |
| `reviewer` | Qualité | Review de PR/MR par sévérité |
| `qa-engineer` | Qualité | Tests manquants (unit / integration / E2E). **Automatiquement invoqué pour le code critique** (API, services, >200 lignes). Produit un rapport de couverture et des points d'attention pour le reviewer. |
| `debugger` | Qualité | Diagnostic de bugs, rapport de cause racine |
| `documentarian` | Documentation | README, CHANGELOG, ADR, doc API |

### Sous-agents (délégués par les coordinateurs)

| Agent | Délégué par | Domaine |
|-------|-------------|---------|
| `developer` | `orchestrator-dev` | Implémentation — domaine précisé à l'invocation : frontend, backend, fullstack, api, mobile, data, devops, platform, security |
| `developer-refactor` | `orchestrator-dev` | Refactoring structurel — ne modifie jamais le comportement observable |
| `developer-migrator` | `orchestrator-dev` | Migrations incrémentales — upgrades de framework, versions majeures, dépendances EOL |
| `auditor-subagent` | `auditor` | Tous les domaines d'audit : security (OWASP Top 10, CVE, RGS), performance (Core Web Vitals, N+1, cache), accessibility (WCAG 2.1 AA, RGAA 4.1), ecodesign (RGESN, GreenIT, Écoindex), architecture (SOLID, Clean Architecture), privacy (RGPD, EDPB, CNIL), observability (Méthode RED, SLOs, OpenTelemetry) |

> Référence complète : [docs/architecture/agents.fr.md](docs/architecture/agents.fr.md)

---

## Workflows disponibles

| Scénario | Point d'entrée | Prompt type |
|----------|---------------|-------------|
| Feature de A à Z | `orchestrator` | `"Implémente [feature]"` |
| Tickets prêts à coder | `orchestrator-dev` | `"Implémente les tickets bd-X à bd-Y"` |
| Audit avant mise en prod | `auditor` | `"Audite le projet"` |
| Bug en production | `debugger` | `"Ce bug : [stacktrace]"` |
| Spec UX/UI standalone | `ux-designer` / `ui-designer` | `"Spec UX pour [feature]"` |
| Documenter une feature | `documentarian` | `"Documente [sujet]"` |
| Découvrir un projet existant | `onboarder` | `"Onboarde-toi sur ce projet"` |
| Planifier sans implémenter | `planner` | `"Décompose [feature] en tickets"` |
| Déboguer un bug | `debugger` | `"oc debug"` |
| Lancer un audit | `auditor` | `"oc audit"` |
| Vérifier les conventions | `reviewer` | `"oc conventions"` |
| Reviewer une branche | `reviewer` | `"oc review"` |

> Scénarios détaillés avec diagrammes et prompts réels : [docs/guides/workflows.fr.md](docs/guides/workflows.fr.md)

---

## Intégration Figma

openhub s'intègre avec Figma pour enrichir les workflows de planification avec le contexte design.

### Fonctionnalités

- **Détection automatique de maquettes** : Pathfinder, Planner et Onboarder recherchent les fichiers Figma par nom de feature ou projet
- **Détection de signaux UX/UI** : Détection automatique des flows multi-étapes et composants visuels
- **Extraction des design tokens** : Extraction des couleurs, typographie, espacements et effets depuis Figma Variables
- **Détection du design system** : Identification automatique de DSFR, Material Design ou design system custom
- **Estimation enrichie** : Ajustement de la complexité selon les composants et états détectés
- **Pré-remplissage du contexte design** : Remplissage auto des champs `--design` dans les tickets avec les données Figma

### Configuration

1. **Obtenir vos tokens Figma** (Personal Access Token + Team ID)
2. **Configurer** `~/.config/opencode/config.json` :
   ```json
   {
     "env": {
       "FIGMA_PERSONAL_ACCESS_TOKEN": "figd_xxx",
       "FIGMA_TEAM_ID": "123456"
     }
   }
   ```
3. **Organiser vos fichiers Figma** selon les conventions (voir `config/figma.conventions.md`)
4. **Déployer** vers vos projets avec `oc deploy opencode MON-APP`

### Utilisation

Les agents Pathfinder, Planner et Onboarder interrogent automatiquement Figma lors de l'analyse de features UI ou de l'exploration de projets :

```bash
# Pathfinder avec enrichissement Figma
> Pathfinder cette feature: dashboard utilisateur

# Planner avec contexte Figma (Phase 1.3)
> Planifie cette feature: processus inscription

# Onboarder avec exploration Figma (Phase 1.5)
> Onboarde-toi sur ce projet
# → Extrait les design tokens, détecte le design system, liste les composants
```

📖 **Documentation complète** : [Guide Intégration Figma](docs/guides/figma-integration.fr.md)

---

## Documentation

### Guides

| Document | Description |
|----------|-------------|
| [Démarrage rapide](docs/guides/getting-started.fr.md) | Installation complète, premier déploiement |
| [Intégration Figma](docs/guides/figma-integration.fr.md) | Configuration MCP Figma, setup et tests |
| [Providers LLM](docs/guides/providers.fr.md) | Anthropic, MammouthAI, GitHub Models, Bedrock, Ollama |
| [Workflows](docs/guides/workflows.fr.md) | Feature complète, audit, debug — scénarios illustrés |
| [Contribuer](docs/guides/contributing.fr.md) | Ajouter un agent, un skill, un adapter |
| [Onboarding](docs/guides/onboarding.fr.md) | Guide d'utilisation de l'agent onboarder |
| [Authoring](docs/guides/authoring.fr.md) | Guide de conception d'agents et skills |

### Architecture

| Document | Description |
|----------|-------------|
| [Vue d'ensemble](docs/architecture/overview.fr.md) | Concepts, diagrammes de flux, principes de design |
| [Agents](docs/architecture/agents.fr.md) | Référence exhaustive des 19 agents |
| [Serveurs MCP](servers/README.md) | Architecture et développement des serveurs MCP |
| [ADR](docs/architecture/adr/) | Décisions architecturales (9 ADR) |
| [Adapters](docs/architecture/adapters.fr.md) | Architecture des adapters |

### Référence

| Document | Description |
|----------|-------------|
| [CLI](docs/reference/cli.fr.md) | Toutes les commandes `oc` avec options et exemples |
| [Configuration](docs/reference/config.fr.md) | hub.json, projects.md, paths.local.md |
| [Conventions Figma](config/figma.conventions.md) | Conventions d'organisation des fichiers Figma |
| [Modèle de données Beads](docs/reference/beads-model.fr.md) | Référence du modèle de données Beads |
| [Outils d'audit](docs/reference/audit-tools.fr.md) | Référence des outils d'audit par domaine |
| [Résolution de modèle](docs/reference/model-resolution.fr.md) | Résolution de modèle par agent |

### Développement

| Document | Description |
|----------|-------------|
| [Optimisations de performance](docs/dev/performance-optimizations.fr.md) | Améliorations de performance dans `oc deploy` |
| [Système de barre de progression](docs/dev/progress-bar.fr.md) | Système de feedback visuel pour opérations longues |
| [Pièges shell](docs/dev/shell-gotchas.md) | Pièges courants dans les scripts bash |

---

## Licence

MIT
