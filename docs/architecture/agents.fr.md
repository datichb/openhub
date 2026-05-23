# Référence des agents

27 agents au total, organisés en 6 familles.
Chaque agent est défini dans `agents/<famille>/<id>.md` avec un frontmatter déclarant ses métadonnées,
ses cibles et ses skills.

---

## Format d'un agent

```markdown
---
id: <identifiant-unique>
label: <NomAffiché>
description: <Description courte — visible dans les outils IA>
mode: primary         # primary (défaut) | subagent
permission:
  question: allow     # optionnel — autorise l'outil question d'OpenCode (agents primary interactifs uniquement)
targets: [opencode]
skills: [chemin/vers/skill, ...]
---

# <Titre>

<Corps de l'agent>
```

| Champ | Rôle |
|-------|------|
| `id` | Identifiant unique, utilisé par les adapters et `oc agent` |
| `label` | Nom affiché dans l'outil cible |
| `description` | Phrase courte décrivant le rôle — apparaît dans les listes d'agents |
| `mode` | `primary` (défaut) ou `subagent` — contrôle la visibilité dans les outils cibles |
| `permission.question` | `allow` — active l'outil `question` d'OpenCode pour cet agent. Réservé aux agents `primary` interactifs. Toujours associé à la skill `posture/tool-question`. |
| `targets` | Cibles supportées : `opencode` |
| `skills` | Chemins relatifs à `skills/` — injectés dans l'ordre de déclaration |

### Modes primary / subagent

Le champ `mode:` contrôle comment un agent est exposé dans chaque outil cible :

| Mode | OpenCode |
|------|----------|
| `primary` | Visible dans le Tab picker — présent dans `.opencode/agents/` |
| `subagent` | Listé dans `opencode.json` avec `"mode": "subagent"` — invocable par d'autres agents, invisible dans le Tab picker. Présent dans `.opencode/agents/` avec description orientée délégation. |

Le mode effectif suit une priorité : **override projet** (`- Modes :` dans `projects.md`) > **frontmatter agent** > **`primary`** (défaut).

Pour modifier les modes d'un projet sans toucher aux frontmatter : `oc agent mode <PROJECT_ID>`.

---

## Famille — Coordinateurs

Agents qui pilotent d'autres agents sans jamais coder eux-mêmes.

### `onboarder`

| | |
|--|--|
| **Label** | Onboarder |
| **Fichier** | `agents/planning/onboarder.md` |
| **Skills** | `planning/onboarder-workflow`, `planning/onboarder-handoff-format`, `posture/expert-posture`, `posture/tool-question`, `developer/beads-plan`, `developer/dev-standards-git` |
| **Invocation** | `"Onboarde-toi sur ce projet"` / `"Découvre ce projet"` / `"Avant de commencer, explore le projet"` |

Agent de découverte de projet. Explore la codebase d'un projet existant en 6 phases structurées
(vérification prérequis → exploration adaptative 7 profils → questions → rapport contexte →
détection cas particuliers → production des livrables). Produit `ONBOARDING.md`, `CONVENTIONS.md`
et optionnellement `projects.md`.

Détecte les cas particuliers : incohérences stack/conventions, CVE connus, dette technique masquée,
architecture hybride non documentée. Produit une carte des agents recommandés en 3 niveaux
(prioritaires par risques, recommandés par stack, optionnels).

Lecture seule — ne modifie jamais de fichiers (sauf les livrables produits).
Ne déclenche jamais automatiquement un autre agent — il suggère des invocations, l'utilisateur décide.

Invocable directement, depuis `oc start` (suggestion affichée), ou depuis l'`orchestrator`
(Mode C — pré-phase sur projet inconnu).

---

### `orchestrator`

| | |
|--|--|
| **Label** | Orchestrator |
| **Fichier** | `agents/planning/orchestrator.md` |
| **Skills** | `orchestrator/orchestrator-protocol`, `orchestrator/orchestrator-workflow-modes`, `orchestrator/orchestrator-handoff-format`, `developer/beads-plan`, `posture/tool-question`, `design/design-handoff-format`, `auditor/audit-handoff-format`, `planning/planner-handoff-format`, `planning/onboarder-handoff-format`, `quality/debugger-handoff-format` |
| **Invocation** | `"Implémente [feature]"` / `"Prends en charge les tickets [IDs]"` |

Chef de projet IA. Pilote la réalisation complète d'une feature en mobilisant tous
les agents nécessaires : conception (ux-designer, ui-designer), audit (auditor-*),
implémentation (via orchestrator-dev). Impose des checkpoints explicites à chaque
phase. Ne code jamais.

**Quatre modes d'entrée :**
- **Mode D** — bug signalé → délègue immédiatement au `debugger`, sans analyse
- **Mode C** — projet inconnu → lit `ONBOARDING.md` / `CONVENTIONS.md` en premier ; propose l'`onboarder` uniquement si les deux fichiers sont absents
- **Mode A** — feature en langage naturel → délègue au `planner`
- **Mode B** — tickets Beads existants → démarrage direct

Ne route jamais directement vers les `developer-*` — délègue toujours à `orchestrator-dev`.

**Permissions techniques :** `bash`, `edit`, `write` désactivés. Agit uniquement via `task` (délégation) et `question` (checkpoints). Liste des agents invocables explicitement restreinte dans le frontmatter.

**Gestion des agents manquants :** si un agent requis n'est pas déployé dans le projet, l'orchestrateur pose une question structurée avec les options : déployer via `!oc deploy` sans quitter OpenCode / utiliser un substitut (table de substitution par domaine) / ignorer le ticket. Ne bascule jamais silencieusement vers un autre agent.

---

### `orchestrator-dev`

| | |
|--|--|
| **Label** | OrchestratorDev |
| **Fichier** | `agents/planning/orchestrator-dev.md` |
| **Skills** | `orchestrator/orchestrator-workflow-modes`, `orchestrator/orchestrator-handoff-format`, `orchestrator/orchestrator-dev-protocol`, `posture/tool-question`, `developer/developer-handoff-format`, `reviewer/reviewer-handoff-format`, `qa/qa-handoff-format`, `documentarian/documentarian-handoff-format` |
| **Invocation** | `"Implémente les tickets [IDs]"` / `"Workflow dev sur [feature]"` |

Tech lead IA spécialisé dans le pilotage de l'implémentation. Prend en charge une
liste de tickets Beads prêts à implémenter, route vers les 9 agents `developer-*`,
supervise le QA optionnel et la review. Trois modes : `manuel` (défaut), `semi-auto`,
`auto`. Invocable standalone ou depuis l'`orchestrator`.

CP-2 (commit ou corriger ?) est toujours manuel dans tous les modes.

> Voir [ADR-006](./adr/006-orchestrator-configurable-mode.fr.md) — les modes s'appliquent à `orchestrator-dev` uniquement.

---

### `auditor`

| | |
|--|--|
| **Label** | Auditeur |
| **Fichier** | `agents/auditor/auditor.md` |
| **Skills** | `auditor/auditor-workflow`, `posture/tool-question` |
| **Invocation** | `"Audite [projet/périmètre]"` / `"Audit [domaine]"` |

Coordinateur d'audit multi-domaine. Pilote la réalisation d'audits en 5 phases structurées :
vérification prérequis (périmètre, stack, accès fichiers) → chargement contexte projet (lit
`ONBOARDING.md` en priorité ou reconnaissance rapide) → sélection domaines avec vérification
compatibilité stack → délégation aux 7 sous-agents spécialisés → consolidation synthèse exécutive
(score global, top 5 actions prioritaires, recommandations transverses).

Produit une synthèse exécutive multi-domaines. Lecture seule — ne modifie jamais de fichiers.

---

## Famille — Agents d'audit

Sous-agents de l'auditeur. Tous en lecture seule. Invocables directement ou via l'auditeur.

| Agent | Fichier | Domaine | Référentiels |
|-------|---------|---------|-------------|
| `auditor-security` | `agents/auditor/auditor-security.md` | Sécurité applicative | OWASP Top 10, CVE, RGS |
| `auditor-performance` | `agents/auditor/auditor-performance.md` | Performance web | Core Web Vitals, N+1, cache |
| `auditor-accessibility` | `agents/auditor/auditor-accessibility.md` | Accessibilité | WCAG 2.1 AA, RGAA 4.1 |
| `auditor-ecodesign` | `agents/auditor/auditor-ecodesign.md` | Éco-conception | RGESN, GreenIT, Écoindex |
| `auditor-architecture` | `agents/auditor/auditor-architecture.md` | Architecture & dette | SOLID, Clean Architecture |
| `auditor-privacy` | `agents/auditor/auditor-privacy.md` | Protection des données | RGPD, EDPB, CNIL |
| `auditor-observability` | `agents/auditor/auditor-observability.md` | Observabilité | Méthode RED, SLOs, OpenTelemetry, alerting |

Tous les agents d'audit injectent `auditor/audit-protocol-light` (format de rapport commun allégé)
+ leur skill de domaine spécifique (`auditor/audit-<domaine>`)
+ `auditor/audit-handoff-format` (contrat de retour structuré quand invoqué depuis l'orchestrator).

---

## Famille — Agents développeurs

9 agents spécialisés par domaine technique. Tous suivent le même workflow Beads
(`bd claim → implémenter → tester → bd close`).

Skills communs à tous : `dev-standards-universal`, `dev-standards-security`, `dev-standards-git`, `beads-plan`, `beads-dev`, `developer/developer-handoff-format`.

| Agent | Fichier | Domaine | Skills spécifiques |
|-------|---------|---------|-------------------|
| `developer-frontend` | `agents/developer/developer-frontend.md` | UI, composants, Vue.js, CSS, a11y | `dev-standards-frontend`, `dev-standards-frontend-a11y`, `dev-standards-vuejs`, `dev-standards-testing` |
| `developer-backend` | `agents/developer/developer-backend.md` | Services, repositories, migrations | `dev-standards-backend`, `dev-standards-testing` |
| `developer-fullstack` | `agents/developer/developer-fullstack.md` | Features front + back | `dev-standards-frontend`, `dev-standards-backend`, `dev-standards-testing` |
| `developer-data` | `agents/developer/developer-data.md` | Pipelines, ETL, ML, dbt | `dev-standards-data` |
| `developer-devops` | `agents/developer/developer-devops.md` | Docker, CI/CD, scripts shell | `dev-standards-devops` |
| `developer-mobile` | `agents/developer/developer-mobile.md` | React Native, Flutter, iOS, Android | `dev-standards-mobile` |
| `developer-api` | `agents/developer/developer-api.md` | REST, GraphQL, webhooks | `dev-standards-backend`, `dev-standards-api`, `dev-standards-testing` |
| `developer-platform` | `agents/developer/developer-platform.md` | Terraform, K8s, Helm, GitOps, infra as code | `dev-standards-platform` |
| `developer-security` | `agents/developer/developer-security.md` | Hardening applicatif post-audit | `dev-standards-security-hardening`, `dev-standards-backend`, `dev-standards-testing` |

> Voir [ADR-002](./adr/002-developer-segmentation.fr.md) pour la décision de segmentation.

`developer-platform` se distingue de `developer-devops` : DevOps couvre Dockerfile,
docker-compose, GitHub Actions et scripts shell applicatifs ; Platform couvre
Terraform/Pulumi, manifests Kubernetes, Helm charts, ArgoCD/Flux.

`developer-security` se distingue de `developer-backend` : il intervient
exclusivement après un audit `auditor-security` pour corriger les failles identifiées
(headers HTTP, CORS, hashing, JWT, sessions, rate limiting, chiffrement). Il ne
réalise pas d'audit.

---

## Famille — Agents de design

Agents de conception UX/UI. Travaillent en amont de l'implémentation.
Ne codent jamais. Invocables directement ou via l'`orchestrator`.

### `ux-designer`

| | |
|--|--|
| **Label** | UXDesigner |
| **Fichier** | `agents/design/ux-designer.md` |
| **Skills** | `designer/ux-protocol`, `developer/beads-plan`, `developer/beads-dev`, `posture/expert-posture`, `posture/tool-question`, `design/design-handoff-format` |
| **Invocation** | `"Analyse le flow de [feature]"` / `"Spec UX pour [ticket]"` / `"Audit UX de [écran]"` |

Expert en expérience utilisateur. Analyse les besoins, identifie les frictions,
produit des user flows textuels et des spécifications UX actionnables avec critères
d'acceptance. Pose au moins 2 questions de contexte avant de spécifier.
Lit et clôt les tickets Beads. Ne produit pas de maquettes graphiques.

Invocable directement, via l'`orchestrator`, ou via le `planner` (PHASE 1.5 —
délégation design optionnelle). Quand invoqué depuis le `planner`, produit la spec
au format standardisé `## SPEC UX — [feature]` pour permettre la réintégration
automatique dans le plan (pas de `bd close` — le planner reprend la main).

---

### `ui-designer`

| | |
|--|--|
| **Label** | UIDesigner |
| **Fichier** | `agents/design/ui-designer.md` |
| **Skills** | `designer/ui-protocol`, `developer/beads-plan`, `developer/beads-dev`, `posture/expert-posture`, `posture/tool-question`, `design/design-handoff-format` |
| **Invocation** | `"Spec UI pour [composant]"` / `"Design system [projet]"` / `"Harmonise [écran]"` |

Expert en design d'interface. Définit les fondations d'un design system (tokens),
spécifie les composants visuels avec variants et états, produit des guidelines UI
actionnables pour `developer-frontend`. Utilise uniquement des tokens — jamais de
valeurs en dur. Propose toujours des options pour les décisions de direction artistique.

Invocable directement, via l'`orchestrator`, ou via le `planner` (PHASE 1.5 —
délégation design optionnelle). Quand invoqué depuis le `planner`, produit la spec
au format standardisé `## SPEC UI — [NomComposant]` pour permettre la réintégration
automatique dans le plan (pas de `bd close` — le planner reprend la main).

---

## Famille — Agents qualité

Agents dédiés à la qualité du code, invocables standalone ou via l'orchestrateur.

### `reviewer`

| | |
|--|--|
| **Label** | CodeReviewer |
| **Fichier** | `agents/quality/reviewer.md` |
| **Skills** | `dev-standards-universal`, `dev-standards-security`, `dev-standards-backend`, `dev-standards-frontend`, `dev-standards-frontend-a11y`, `dev-standards-testing`, `dev-standards-git`, `reviewer/review-protocol`, `posture/tool-question`, `reviewer/reviewer-handoff-format` |
| **Invocation** | Diff collé / nom de branche / URL de PR + optionnellement `bd show <ID>` |

Analyse les diffs de PR/MR. Produit un rapport structuré par sévérité (Critique /
Majeur / Mineur / Suggestion / Points positifs). Lecture seule — ne modifie jamais
de fichiers.

---

### `qa-engineer`

| | |
|--|--|
| **Label** | QAEngineer |
| **Fichier** | `agents/quality/qa-engineer.md` |
| **Skills** | `dev-standards-universal`, `dev-standards-testing`, `dev-standards-git`, `posture/expert-posture`, `posture/tool-question`, `qa/qa-protocol`, `qa/qa-handoff-format` |
| **Invocation** | `"Écris les tests pour la branche [X]"` / `"QA sur le ticket [ID]"` |

Écrit les tests manquants (unit / integration / E2E) à partir d'un diff ou d'un
ticket Beads. Produit un rapport de couverture avant/après. Ne modifie jamais
le code fonctionnel.

**Non pertinent pour les tickets TDD** : quand un ticket porte le label `tdd`,
les tests sont écrits par le developer lui-même avant l'implémentation (boucle
red/green/refactor). L'`orchestrator-dev` saute automatiquement le CP-QA pour
ces tickets — le `qa-engineer` n'est pas invoqué.

> Voir [ADR-004](./adr/004-qa-debugger-separation.fr.md).

---

### `debugger`

| | |
|--|--|
| **Label** | Debugger |
| **Fichier** | `agents/quality/debugger.md` |
| **Skills** | `quality/debugger-workflow`, `posture/tool-question`, `quality/debugger-handoff-format` |
| **Invocation** | `"Ce bug : [stacktrace]"` / `"Analyse ces logs : [logs]"` |

Diagnostique la cause racine d'un bug en 6 phases structurées : vérification des artefacts
disponibles (Phase 0 — pause si insuffisants) → exploration contextuelle → questions
complémentaires (optionnel) → diagnostic 4 étapes (reproduction/isolation/identification/
hypothèse graduée haute/moyenne/faible) → détection cas particuliers (race conditions,
environnement-spécifique, données, configuration, dépendances, régression). Produit un
rapport de diagnostic avec hypothèses graduées. Crée un ticket Beads de correction après
confirmation explicite. Ne corrige jamais le bug.

> Voir [ADR-004](./adr/004-qa-debugger-separation.fr.md).

---

## Famille — Agents de planification

### `planner`

| | |
|--|--|
| **Label** | ProjectPlanner |
| **Fichier** | `agents/planning/planner.md` |
| **Skills** | `developer/beads-plan`, `planning/planner-workflow`, `posture/expert-posture`, `posture/tool-question`, `planning/planner-handoff-format` |
| **Invocation** | Description d'une feature en langage naturel |

Consultant fonctionnel et technique qui analyse le contexte projet avant de planifier.
Workflow en 7 phases : vérification prérequis → exploration contextuelle (codebase, tickets,
signaux UX/UI) → délégation design optionnelle (Phase 1.5) → questions complémentaires →
plan hiérarchique (epics → tickets, priorités déduites et justifiées) → détection cas
particuliers (doublons, tickets trop gros, dépendances circulaires) → création Beads avec
enrichissement complet → délégation ai-delegated optionnelle (Phase 5.5) → vérification finale.

Crée les epics dans Beads si > 5 tickets (demande sinon), utilise `--parent` et `--deps`
pour la hiérarchie et les dépendances. Gère les aléas : scope change, ticket à scinder,
dépendance tardive, doublon. Ne code jamais. Phases itératives avec retours en arrière
possibles (max 3 itérations par phase).

**Phase 1.5 — Délégation design (optionnelle) :** quand des signaux UX ou UI sont détectés
en Phase 1, le planner propose 3 options à l'utilisateur :
- **Option A** (`"invoquer UX/UI"`) — invoque directement `ux-designer` / `ui-designer`
  en sous-agent, attend le bloc structuré `## SPEC UX/UI — …` et intègre la spec dans le plan.
- **Option B** — l'utilisateur invoque lui-même les agents et colle la spec.
- **Option C** (`"continuer sans UX/UI"`) — poursuit avec le contexte disponible,
  tickets `--design` partiels + `bd comments add` pour tracer la spec manquante.

---

## Famille — Agents de documentation

### `documentarian`

| | |
|--|--|
| **Label** | Documentarian |
| **Fichier** | `agents/documentation/documentarian.md` |
| **Skills** | `developer/dev-standards-git`, `developer/beads-plan`, `developer/beads-dev`, `documentarian/doc-protocol`, `documentarian/doc-standards`, `documentarian/doc-adr`, `documentarian/doc-api`, `documentarian/doc-changelog`, `documentarian/doc-slides`, `posture/expert-posture`, `posture/tool-question` |
| **Invocation** | `"Documente [sujet]"` / `"Crée un ADR pour [décision]"` / `"Mets à jour le CHANGELOG"` / `"Qu'est-ce qui manque dans la doc ?"` / `"Crée une présentation pour [sujet]"` |

Rédige et met à jour la documentation technique, fonctionnelle, architecturale, API,
les changelogs et les présentations Marp. Explore systématiquement la structure existante
avant d'écrire. S'adapte au format en place — recommande des améliorations sans les imposer.
Ne change jamais un format sans confirmation explicite.

Principe directeur : **explorer → adapter ou proposer → attendre si nécessaire → écrire**.

---

## Règles communes à tous les agents

- **Agents en lecture seule** : auditor-*, reviewer, debugger, ux-designer, ui-designer — ne modifient jamais de fichiers
- **Agents qui écrivent du code** : developer-*, qa-engineer — modifient uniquement les fichiers de leur domaine
- **Agents qui écrivent de la documentation** : documentarian — modifie uniquement les fichiers de documentation
- **Agents qui créent des tickets** : planner (tickets feature), debugger (tickets bug après confirmation)
- **Agents qui lisent les tickets** : tous peuvent faire `bd show <ID>` pour contextualiser leur travail
- **Agents coordinateurs** : orchestrator, orchestrator-dev, auditor — ne codent jamais, pilotent d'autres agents
- **Agents de découverte** : onboarder — lecture seule, explore et rapporte, ne pilote pas d'autres agents
- **Agents `primary`** : orchestrator, orchestrator-dev, planner, auditor, ui-designer, ux-designer, documentarian, onboarder, debugger, qa-engineer, reviewer — visibles directement par l'utilisateur
- **Agents `subagent`** : tous les `developer-*` et `auditor-*` (sauf `auditor` lui-même) — invocables par des agents coordinateurs
