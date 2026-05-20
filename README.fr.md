> 🇬🇧 [Read in English](README.md)

# opencode-hub

Hub central pour piloter des assistants IA sur plusieurs projets,
avec des agents partagés, des skills injectables et un workflow Beads intégré.

Supporte **OpenCode**.

---

## Comment ça marche

opencode-hub repose sur trois concepts : **agents**, **skills** et **déploiement**.

- Les **agents** définissent les rôles IA (qui fait quoi, comment, dans quel ordre).
- Les **skills** sont des protocoles injectables (standards de code, checklists, formats de rapport) — déclarés une fois, réutilisés entre plusieurs agents.
- Le **déploiement** assemble agents + skills et les copie dans vos projets cibles.

```
opencode-hub/          ← source de vérité (éditer ici, jamais dans les projets)
├── agents/            ← identité des rôles IA (~40-80 lignes par agent)
├── skills/            ← protocoles détaillés injectables
└── scripts/           ← assemblage et déploiement

         oc deploy opencode MON-APP
opencode-hub  ──────────────────────►  mon-app/.opencode/agents/*.md
                                   └►  mon-app/opencode.json

```

Résultat : 27 agents spécialisés, toujours à jour, disponibles dans tous vos projets
depuis une source de vérité unique.

---

## Installation

### One-liner (recommandé)

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/opencode-hub/main/install.sh | bash
```

Le script automatise tout : clone dans `~/.opencode-hub`, vérification des dépendances
avec confirmation, création de l'alias `oc`, et configuration interactive des cibles AI.

Après l'installation, recharger le shell :

```bash
source ~/.zshrc   # ou source ~/.bashrc
```

### Installation manuelle

```bash
git clone https://github.com/datichb/opencode-hub.git ~/.opencode-hub
echo 'alias oc="~/.opencode-hub/oc.sh"' >> ~/.zshrc && source ~/.zshrc
oc install
```

### Installer une version spécifique

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/opencode-hub/main/install.sh | VERSION=v1.0.0 bash
```

---

## Mise à jour

### Mettre à jour les sources du hub

```bash
oc upgrade           # pull le dernier main
oc upgrade v1.1.0    # basculer sur un tag de release spécifique
```

### Mettre à jour les outils installés (opencode, Beads, skills)

```bash
oc update
```

---

## Désinstallation

```bash
oc uninstall
# ou directement :
bash ~/.opencode-hub/uninstall.sh
```

Guide interactif en 4 étapes — tout est optionnel et demande confirmation :

| Étape | Action | Défaut |
|-------|--------|--------|
| 1 | Nettoyer les agents déployés dans les projets (`.opencode/agents/`, `opencode.json`) | `[y/N]` |
| 2 | Supprimer le hub (`~/.opencode-hub`) | `[y/N]` |
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

27 agents en deux modes :

- **`primary`** — visibles directement dans l'outil IA (tab picker OpenCode). Invocables par l'utilisateur.
- **`subagent`** — invisibles dans le picker. Invocables uniquement par délégation depuis un agent coordinateur.

### Agents primaires (invocables directement)

| Agent | Famille | Rôle |
|-------|---------|------|
| `orchestrator` | Coordinateur | Feature de A à Z — délègue spec, audit, implémentation |
| `orchestrator-dev` | Coordinateur | Implémentation de tickets — pilote les `developer-*` |
| `auditor` | Coordinateur | Audit multi-domaine — délègue aux 7 `auditor-*` |
| `onboarder` | Coordinateur | Découverte de projet existant, rapport de contexte |
| `planner` | Planification | Décompose une feature en tickets Beads |
| `ux-designer` | Design | Spec UX — user flows, critères d'acceptance |
| `ui-designer` | Design | Spec UI — tokens, composants, guidelines visuelles |
| `reviewer` | Qualité | Review de PR/MR par sévérité |
| `qa-engineer` | Qualité | Tests manquants (unit / integration / E2E) |
| `debugger` | Qualité | Diagnostic de bugs, rapport de cause racine |
| `documentarian` | Documentation | README, CHANGELOG, ADR, doc API |

### Sous-agents (délégués par les coordinateurs)

| Agent | Délégué par | Domaine |
|-------|-------------|---------|
| `developer-frontend` | `orchestrator-dev` | UI, composants, Vue.js, CSS, a11y |
| `developer-backend` | `orchestrator-dev` | Services, repositories, migrations |
| `developer-fullstack` | `orchestrator-dev` | Features front + back |
| `developer-data` | `orchestrator-dev` | Pipelines, ETL, ML, dbt |
| `developer-devops` | `orchestrator-dev` | Docker, CI/CD, scripts shell |
| `developer-mobile` | `orchestrator-dev` | React Native, Flutter, iOS, Android |
| `developer-api` | `orchestrator-dev` | REST, GraphQL, webhooks |
| `developer-platform` | `orchestrator-dev` | Terraform, K8s, Helm, GitOps |
| `developer-security` | `orchestrator-dev` | Hardening post-audit sécurité |
| `auditor-security` | `auditor` | OWASP Top 10, CVE, RGS |
| `auditor-performance` | `auditor` | Core Web Vitals, N+1, cache |
| `auditor-accessibility` | `auditor` | WCAG 2.1 AA, RGAA 4.1 |
| `auditor-ecodesign` | `auditor` | RGESN, GreenIT, Écoindex |
| `auditor-architecture` | `auditor` | SOLID, Clean Architecture, dette technique |
| `auditor-privacy` | `auditor` | RGPD, EDPB, CNIL |
| `auditor-observability` | `auditor` | Méthode RED, SLOs, OpenTelemetry |

> Les sous-agents sont aussi invocables directement si besoin (ex : `auditor-security` seul sans passer par `auditor`).

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

> Scénarios détaillés avec diagrammes et prompts réels : [docs/guides/workflows.fr.md](docs/guides/workflows.fr.md)

---

## Documentation

### Guides

| Document | Description |
|----------|-------------|
| [Démarrage rapide](docs/guides/getting-started.fr.md) | Installation complète, premier déploiement |
| [Providers LLM](docs/guides/providers.fr.md) | Anthropic, MammouthAI, GitHub Models, Bedrock, Ollama |
| [Workflows](docs/guides/workflows.fr.md) | Feature complète, audit, debug — scénarios illustrés |
| [Contribuer](docs/guides/contributing.fr.md) | Ajouter un agent, un skill, un adapter |

### Architecture

| Document | Description |
|----------|-------------|
| [Vue d'ensemble](docs/architecture/overview.fr.md) | Concepts, diagrammes de flux, principes de design |
| [Agents](docs/architecture/agents.fr.md) | Référence exhaustive des 27 agents |
| [Skills](docs/architecture/skills.fr.md) | Référence exhaustive des skills et leurs dépendances |
| [ADR](docs/architecture/adr/) | Décisions architecturales (6 ADR) |

### Référence

| Document | Description |
|----------|-------------|
| [CLI](docs/reference/cli.fr.md) | Toutes les commandes `oc` avec options et exemples |
| [Configuration](docs/reference/config.fr.md) | hub.json, projects.md, paths.local.md |

---

## Licence

MIT
