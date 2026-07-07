# Référence des agents

18 agents au total, organisés en 6 familles.
Chaque agent est défini dans `agents/<famille>/<id>.md` avec un frontmatter déclarant ses métadonnées,
ses skills et son mode.

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
  skill: allow        # allow | deny — active l'outil skill natif (Bucket B)
skills: [chemin/vers/skill, ...]          # Bucket A — assemblées inline au déploiement
native_skills: [chemin/vers/skill, ...]   # Bucket B — déployées vers .opencode/skills/, chargées à la demande
---

# <Titre>

<Corps de l'agent>
```

| Champ | Rôle |
|-------|------|
| `id` | Identifiant unique, utilisé par les adapters et `oh agent` |
| `label` | Nom affiché dans l'outil |
| `description` | Phrase courte décrivant le rôle — apparaît dans les listes d'agents |
| `mode` | `primary` (défaut) ou `subagent` — contrôle la visibilité dans OpenCode |
| `permission.question` | `allow` — active l'outil `question` d'OpenCode pour cet agent. Réservé aux agents `primary` interactifs. Toujours associé à la skill `posture/tool-question`. |
| `permission.skill` | `allow` — active l'outil `skill` natif pour que l'agent puisse charger les skills Bucket B à la demande. Mettre `deny` pour les coordinateurs/orchestrateurs qui n'ont jamais besoin de skills contextuelles. |
| `skills` | **Bucket A** — chemins relatifs à `skills/`, injectés inline au déploiement, toujours actifs dès le premier token. Protocoles de workflow, formats de handoff, principes universels. |
| `native_skills` | **Bucket B** — chemins relatifs à `skills/`, déployés vers `.opencode/skills/<name>/SKILL.md`, chargés à la demande par le LLM via l'outil `skill`. Standards de domaine, stack skills, checklists. |

Voir [ADR-010](./adr/010-hybrid-skills-architecture.fr.md) pour le raisonnement derrière la séparation Bucket A / Bucket B.

### Modes primary / subagent

Le champ `mode:` contrôle comment un agent est exposé dans OpenCode :

| Mode | OpenCode |
|------|----------|
| `primary` | Visible dans le Tab picker — présent dans `.opencode/agents/` |
| `subagent` | Listé dans `opencode.json` avec `"mode": "subagent"` — invocable par d'autres agents, invisible dans le Tab picker. Présent dans `.opencode/agents/` avec description orientée délégation. |

Le mode effectif suit une priorité : **override projet** (`- Modes :` dans `projects.md`) > **frontmatter agent** > **`primary`** (défaut).

Pour modifier les modes d'un projet sans toucher aux frontmatter : `oh agent mode <PROJECT_ID>`.

---

## Famille — Coordinateurs

Agents qui pilotent d'autres agents sans jamais coder eux-mêmes.

### `onboarder`

| | |
|--|--|
| **Label** | Onboarder |
| **Fichier** | `agents/planning/onboarder.md` |
| **Skills** | `planning/onboarder-workflow`, `planning/onboarder-handoff-format`, `adapters/figma-onboarder-protocol`, `adapters/gitlab-onboarder-protocol`, `posture/expert-posture`, `posture/tool-question`, `developer/beads-plan`, `developer/dev-standards-git`, `shared/websearch-usage`, `shared/living-docs-enrichment`, `shared/wiki-navigation` — native : `planning/onboarder-standalone`, `planning/onboarder-subagent`, `planning/websearch-stack-research`, `shared/rtk-usage` |
| **MCP Servers** | `figma`, `gitlab` |
| **Invocation** | `"Onboarde-toi sur ce projet"` / `"Découvre ce projet"` / `"Avant de commencer, explore le projet"` |

Agent de découverte de projet. Explore la codebase d'un projet existant en 6 phases structurées
(vérification prérequis → exploration adaptative 7 profils → questions → rapport contexte →
détection cas particuliers → production des livrables). Produit `ONBOARDING.md`, `CONVENTIONS.md`
et optionnellement `projects.md`.

**Nouvelles capacités (enrichissement v1.1) :**
- **Phase 1.4 — Exploration contexte métier** : détection du domaine (e-commerce, fintech, santé, etc.), 
  utilisateurs cibles, concepts clés, glossaire. Analyse sémantique de la codebase pour extraire les concepts récurrents.
- **Phase 1.4bis — Exploration GitLab** (optionnelle, si projet GitLab détecté) : cartographie des labels (types, priorités, domaines), milestones actifs, volume du backlog. Enrichit `ONBOARDING.md` (section Gestion de projet) et `CONVENTIONS.md` (conventions de labelling).
- **Phase 1.5 — Exploration Figma** (optionnelle, si frontend détecté) : recherche des maquettes projet, 
  détection du design system (DSFR, Material, Custom), extraction des design tokens depuis Figma Variables 
  (couleurs, typographie, espacements, effets). Limite à 3 fichiers les plus pertinents.
- **Phase 1.6 — Exploration stratégie de test** : détection frameworks (Vitest, Jest, pytest, Playwright, Cypress), 
  calcul ratio test/source, identification philosophie (TDD, BDD, test-after), extraction seuil de couverture configuré.

Les fichiers produits incluent désormais 3 nouvelles sections : **Contexte métier**, **Design et maquettes**, 
**Stratégie de test**. Le template CONVENTIONS.md inclut également une section **Design tokens** si extraits depuis Figma.

Détecte les cas particuliers : incohérences stack/conventions, CVE connus, dette technique masquée,
architecture hybride non documentée. Produit une carte des agents recommandés en 3 niveaux
(prioritaires par risques, recommandés par stack, optionnels).

Lecture seule — ne modifie jamais de fichiers (sauf les livrables produits).
Ne déclenche jamais automatiquement un autre agent — il suggère des invocations, l'utilisateur décide.

Invocable directement, depuis `oh start` (suggestion affichée), ou depuis l'`orchestrator`
(Mode C — pré-phase sur projet inconnu).

**Phase 5 — Enrichissement incrémental :** quand `ONBOARDING.md` et `CONVENTIONS.md` existent déjà (enrichis par d'autres agents), propose un enrichissement incrémental plutôt qu'une réécriture complète. Délègue les mises à jour incrémentielles au `documentarian` via `task` (skill `living-docs-enrichment`). La réécriture complète reste disponible avec un avertissement explicite sur la perte des enrichissements accumulés.

En mode `orchestrator_feature` : utilise le mécanisme d'interruption de session — chaque fin de phase (0 à 4) produit un bloc `## Retour intermédiaire vers orchestrator` + `## Question pour l'orchestrator` et termine la session.

---

### `orchestrator`

| | |
|--|--|
| **Label** | Orchestrator |
| **Fichier** | `agents/planning/orchestrator.md` |
| **Skills** | `posture/coordination-only`, `posture/concision-posture`, `posture/retranscription-coordinateur`, `orchestrator/orchestrator-protocol`, `orchestrator/orchestrator-workflow-modes`, `orchestrator/orchestrator-handoff-format`, `developer/beads-plan`, `posture/tool-question`, `posture/tool-todowrite`, `planning/planner-handoff-format`, `shared/hub-workflow-reference` — native : `planning/pathfinder-handoff-format`, `design/design-handoff-format`, `auditor/audit-handoff-format`, `planning/onboarder-handoff-format`, `quality/debugger-handoff-format`, `shared/rtk-usage` |
| **MCP Servers** | _(aucun)_ |
| **Invocation** | `"Implémente [feature]"` / `"Prends en charge les tickets [IDs]"` / `"Implémente le ticket #42"` |

Chef de projet IA. Pilote la réalisation complète d'une feature en mobilisant tous
les agents nécessaires : conception (`designer`), audit (auditor-*),
implémentation (via orchestrator-dev). Impose des checkpoints explicites à chaque
phase. Ne code jamais.

**Quatre modes d'entrée :**
- **Mode D** — bug signalé → délègue immédiatement au `debugger`, sans analyse
- **Mode C** — aucun contexte projet dans la session → propose l'`onboarder` si nécessaire
- **Mode A** — feature en langage naturel → délègue au `planner`
- **Mode B** — tickets Beads existants → transmet les IDs directement au `planner` (aucun `bd show`)

Ne route jamais directement vers les `developer-*` — délègue toujours à `orchestrator-dev`.

**Permissions techniques :** `bash`, `read`, `edit`, `write` tous désactivés. Agit uniquement via `task` (délégation) et `question` (checkpoints). Liste des agents invocables explicitement restreinte dans le frontmatter.

**Injection de contexte :** le contexte projet (stack, conventions) est injecté automatiquement dans la session via le champ `instructions` de `opencode.json` (cache valide `.opencode/context.json` ou `ONBOARDING.md`/`CONVENTIONS.md`). L'orchestrateur ne lit jamais de fichiers directement — si le contexte est absent de la session, il propose l'`onboarder`.

**Gestion des agents manquants :** si un agent requis n'est pas déployé dans le projet, l'agent orchestrator pose une question structurée avec les options : déployer via `!oh deploy` sans quitter OpenCode / utiliser un substitut (table de substitution par domaine) / ignorer le ticket. Ne bascule jamais silencieusement vers un autre agent.

**Gate de complétion (CP-feature) :** avant de construire le CP-feature, vérifie que le rapport final d'orchestrator-dev documente les 3 checks de complétion (tests passés, comportement observable conforme, régressions documentées). Si absent → bloquant : question à l'utilisateur (redemander à orchestrator-dev / accepter / stop).

---

### `orchestrator-dev`

| | |
|--|--|
| **Label** | OrchestratorDev |
| **Fichier** | `agents/planning/orchestrator-dev.md` |
| **Skills** | `posture/coordination-only`, `posture/concision-posture`, `posture/retranscription-coordinateur`, `orchestrator/orchestrator-workflow-modes`, `orchestrator/orchestrator-dev-protocol`, `orchestrator/orchestrator-handoff-format`, `posture/tool-question`, `posture/tool-todowrite`, `developer/developer-handoff-format`, `reviewer/reviewer-handoff-format`, `qa/qa-handoff-format`, `documentarian/documentarian-handoff-format` — native : `orchestrator/orchestrator-dev-standalone`, `orchestrator/orchestrator-dev-subagent`, `developer/dev-drift-detection`, `orchestrator/session-state-protocol`, `shared/rtk-usage` |
| **Invocation** | `"Implémente les tickets [IDs]"` / `"Workflow dev sur [feature]"` |

Tech lead IA spécialisé dans le pilotage de l'implémentation. Prend en charge une
liste de tickets Beads prêts à implémenter, route vers l'agent `developer` avec le domaine approprié précisé dans le prompt d'invocation,
supervise le QA optionnel et la review. Trois modes : `manuel` (défaut), `semi-auto`,
`auto`. Invocable standalone ou depuis l'`orchestrator`.

CP-2 (commit ou corriger ?) est toujours manuel dans tous les modes.

`bd close`, `bd comments add` et `bd update` sont toujours exécutés par l'agent `developer` dans les prompts de délégation — jamais directement par `orchestrator-dev`. L'orchestrateur-dev se limite à la lecture des tickets Beads (`bd show`, `bd list`).

En mode `orchestrator_feature` : tous les CPs (CP-1, CP-QA, CP-3, branche dédiée, CP-2, blocage, ticket bloqué) produisent un bloc `## Question pour l'orchestrator` + `## Retour vers orchestrator` (partiel) et terminent la session pour que l'agent orchestrator relaie la question à l'utilisateur.

**Dérive architecturale (BLOCKED_ARCHITECTURE) :** quand un developer retourne ce statut, charge le skill `developer/dev-drift-detection` via l'outil `skill` et présente 3 options à l'utilisateur : réviser le scope du ticket Beads / revert + nouvelle approche / bifurquer vers un ticket de refactoring prérequis (mis en `blocked` jusqu'à résolution).

> Voir [ADR-006](./adr/006-orchestrator-configurable-mode.fr.md) — les modes s'appliquent à `orchestrator-dev` uniquement.

---

### `auditor`

| | |
|--|--|
| **Label** | Auditeur |
| **Fichier** | `agents/auditor/auditor.md` |
| **Skills** | `auditor/auditor-workflow`, `posture/coordination-only`, `posture/retranscription-coordinateur`, `auditor/audit-protocol-light`, `auditor/audit-handoff-format`, `shared/living-docs-enrichment`, `posture/tool-question` — native : `auditor/auditor-standalone`, `auditor/auditor-subagent`, `shared/rtk-usage` |
| **Invocation** | `"Audite [projet/périmètre]"` / `"Audit [domaine]"` |

Coordinateur d'audit multi-domaine. Pilote la réalisation d'audits en 5 phases structurées :
vérification prérequis (périmètre, stack, accès fichiers) → chargement contexte projet (lit
`ONBOARDING.md` en priorité ou reconnaissance rapide) → sélection domaines avec vérification
compatibilité stack → délégation à l'agent `auditor-subagent` (invoqué autant de fois que nécessaire, un domaine par invocation) → consolidation synthèse exécutive
(score global, top 5 actions prioritaires, recommandations transverses).

Produit une synthèse exécutive multi-domaines. Lecture seule — ne modifie jamais de fichiers directement.

**Phase 4 — Enrichissement des documents vivants :** après la synthèse, consolide les sections
`### Découvertes à documenter` des rapports reçus et propose à l'utilisateur d'enrichir
`ONBOARDING.md` et/ou `CONVENTIONS.md`. Si accepté, délègue l'écriture au `documentarian` via `task`
(skill `living-docs-enrichment`). Ne peut invoquer le `documentarian` sans confirmation explicite.

En mode `orchestrator_feature` : utilise le mécanisme d'interruption de session — chaque fin de phase (0 à 3) produit un bloc `## Retour intermédiaire vers orchestrator` + `## Question pour l'orchestrator` et termine la session.

---

## Famille — Agents d'audit

Sous-agent unique de l'auditeur (ADR-017). Lecture seule. Invocable via l'auditeur ou directement.

| Agent | Fichier | Domaine | Référentiels |
|-------|---------|---------|-------------|
| `auditor-subagent` | `agents/auditor/auditor-subagent.md` | Sécurité, Performance, Accessibilité, Éco-conception, Architecture, Privacy, Observabilité — domaine précisé à l'invocation | OWASP Top 10, Core Web Vitals, WCAG 2.1 AA / RGAA 4.1, RGESN / GreenIT, SOLID / Clean Architecture, RGPD / EDPB / CNIL, Méthode RED / SLOs / OpenTelemetry |

L'agent `auditor-subagent` reçoit le domaine + la `native_skill` à charger dans le prompt d'invocation du coordinateur `auditor`.
Il injecte `auditor/audit-protocol-light` (format de rapport commun allégé)
+ la skill de domaine spécifique (`auditor/audit-<domaine>`) chargée à la demande
+ `auditor/audit-handoff-format` (contrat de retour structuré quand invoqué depuis l'orchestrator).

Tous les rapports produits incluent une section **`### Découvertes à documenter`**
en fin de rapport — les découvertes à capitaliser dans `ONBOARDING.md` / `CONVENTIONS.md`.
Cette section est consolidée par le coordinateur `auditor` en Phase 4 (skill `living-docs-enrichment`).
L'agent ne fait jamais d'appel `task` — sa lecture seule est stricte.

---

## Famille — Agents développeurs

1 agent générique spécialisé par domaine au moment de l'invocation.
Suit le même workflow Beads (`bd claim → implémenter → tester → bd close`).

Le **domaine** et les **native_skills à charger** sont transmis par `orchestrator-dev` dans le prompt d'invocation.
Chaque instance `task` s'exécute dans sa propre session isolée — les invocations parallèles avec des domaines différents sont totalement indépendantes.

Skills communs à tous les domaines : `dev-standards-universal`, `dev-standards-simplicity`, `dev-standards-security`, `dev-standards-git`, `dev-standards-testing`, `beads-plan`, `beads-dev`, `developer/developer-handoff-format`, `shared/living-docs-enrichment`.

| Agent | Fichier | Domaine | Native Skills spécifiques |
|-------|---------|---------|--------------------------|
| `developer` | `agents/developer/developer.md` | frontend, backend, fullstack, api, mobile, data, devops, platform, security — domaine précisé à l'invocation | Skills de domaine injectés via le prompt d'invocation (voir `orchestrator-dev-protocol`) |

**Agents séparés (workflow distinct) :**

| Agent | Fichier | Domaine |
|-------|---------|---------|
| `developer-refactor` | `agents/developer/developer-refactor.md` | Refactoring structurel uniquement — ne modifie jamais le comportement observable |
| `developer-migrator` | `agents/developer/developer-migrator.md` | Migrations incrémentales — upgrades de framework, versions majeures, dépendances EOL |

> Voir [ADR-013](./adr/013-developer-agent-consolidation.fr.md) pour la décision de consolidation.
> Voir [ADR-002](./adr/002-developer-segmentation.fr.md) (remplacé) pour la justification de la segmentation précédente.

**Mapping domaine → native_skills (résumé) :**

| Domaine | Native skills |
|---------|--------------|
| `frontend` | `dev-standards-frontend`, `dev-standards-frontend-a11y`, `dev-standards-testing` + stacks détectées |
| `backend` | `dev-standards-backend`, `dev-standards-api`, `dev-standards-testing` + stacks détectées |
| `fullstack` | `dev-standards-frontend`, `dev-standards-frontend-a11y`, `dev-standards-backend`, `dev-standards-api`, `dev-standards-testing` + stacks détectées |
| `api` | `dev-standards-backend`, `dev-standards-api`, `dev-standards-testing` |
| `mobile` | `dev-standards-testing` + stacks mobile détectées |
| `data` | `dev-standards-testing` + stacks data détectées |
| `devops` | `dev-standards-devops` + stacks infra détectées |
| `platform` | `dev-standards-devops` + stacks platform détectées |
| `security` | `dev-standards-security-hardening`, `dev-standards-backend`, `dev-standards-testing` |

**Post-ticket — Enrichissement des documents vivants :** après chaque `bd close`, identifie les patterns, conventions ou contraintes techniques découverts lors de l'implémentation qui sont absents de `CONVENTIONS.md` ou `ONBOARDING.md`, et propose à l'utilisateur de les capitaliser (skill `living-docs-enrichment`).

---

## Famille — Agents de design

Agent de conception UX/UI. Travaille en amont de l'implémentation.
Ne code jamais. Invocable directement ou via l'`orchestrator`.

### `designer`

| | |
|--|--|
| **Label** | Designer |
| **Fichier** | `agents/design/designer.md` |
| **Mode** | `primary` |
| **Permissions** | Pas de `write`, pas d'`edit`. `bash` deny-by-default (allowlist : `bd show *`, `bd list *`). Accès MCP Figma (seul agent du hub avec cette permission). |
| **Skills (inline)** | `designer/designer-protocol`, `design/design-planner-format`, `design/design-handoff-format` |
| **Skills (native)** | `designer/ux-protocol`, `designer/ui-protocol`, `designer/figma-recon-protocol`, `designer/figma-deep-protocol`, `designer/designer-execution-modes` |
| **Invocation** | `"Explore Figma pour [feature]"` / `"Spec UX pour [ticket]"` / `"Spec UI pour [composant]"` / `"Spec design complète pour [feature]"` |

Agent de design unifié. Opère en quatre modes précisés à l'invocation :

| Mode | Déclencheur | Sortie |
|------|-------------|--------|
| `recon` | Exploration Figma demandée (par planner/pathfinder/onboarder via `task`) | Découvertes Figma : composants, tokens, design system détecté |
| `ux` | Spec UX demandée | Flows utilisateurs, heuristiques Nielsen, critères d'acceptance — pas de maquettes graphiques |
| `ui` | Spec UI demandée | Tokens de design, variants/états des composants, guidelines UI pour `developer-frontend` |
| `ux+ui` | Spec design complète demandée | Phase UX complète puis phase UI dans une seule session |

**Seul agent Figma :** `designer` est le seul agent du hub avec accès MCP Figma.
`planner`, `pathfinder` et `onboarder` délèguent tous leurs besoins Figma au `designer`
via `task` (mode `recon`) au lieu d'appeler le MCP directement.

**Quand invoqué depuis le `planner` (Phase 1.5 — délégation design optionnelle) :** produit
la spec au format standardisé `## SPEC UX — [feature]` et/ou `## SPEC UI — [NomComposant]`
pour permettre la réintégration automatique dans le plan (pas de `bd close` — le planner reprend la main).

En mode `orchestrator_feature` : n'utilise jamais l'outil `question` — les clarifications
critiques passent par les blocs `## Retour intermédiaire vers orchestrator` + `## Question pour l'orchestrator`
avec terminaison de session.

**Délégation :** invoqué par `orchestrator`, `planner`, `pathfinder`, `onboarder`.

---

## Famille — Agents qualité

Agents dédiés à la qualité du code, invocables standalone ou via l'agent orchestrator.

### `reviewer`

| | |
|--|--|
| **Label** | CodeReviewer |
| **Fichier** | `agents/quality/reviewer.md` |
| **Skills** | `developer/dev-standards-universal`, `reviewer/review-protocol`, `posture/concision-posture`, `posture/tool-question`, `shared/living-docs-enrichment`, `shared/wiki-navigation`, `reviewer/reviewer-handoff-format` — native : `reviewer/reviewer-standalone`, `reviewer/reviewer-subagent`, `reviewer/reviewer-adversarial`, `reviewer/reviewer-edge-case`, `reviewer/review-merge`, `developer/dev-standards-security`, `developer/dev-standards-backend`, `developer/dev-standards-frontend`, `developer/dev-standards-frontend-data`, `developer/dev-standards-frontend-a11y`, `developer/dev-standards-testing`, `developer/dev-standards-git`, `shared/rtk-usage` |
| **Invocation** | Nom de branche / URL de PR + optionnellement `bd show <ID>` (le reviewer récupère lui-même le diff via `git diff`) |

Analyse les diffs de PR/MR. Produit un rapport structuré par sévérité (Critique /
Majeur / Mineur / Suggestion / Points positifs). Lecture seule — ne modifie jamais
de fichiers.

**Review multi-mode :** supporte trois modes de review combinables :
- **Standard** — checklist 6 catégories, sévérité calibrée (défaut pour les reviews par ticket)
- **Adversarial** — posture de scepticisme maximal, min. 10 findings, hypothèses dangereuses, challenges d'architecture, score de confiance (obligatoire au CP-feature, optionnel via `oh review`)
- **Edge-case** — chasse exhaustive aux chemins d'exécution non gérés (disponible partout en option)

Les modes combinés (`standard+adversarial`, `all`) lancent des **sessions parallèles indépendantes** avec isolation contextuelle totale, puis fusionnent les résultats via le skill `review-merge` (déduplication, hiérarchie de sévérité, tag de provenance).

**Post-rapport — Enrichissement des documents vivants :** après la production du rapport de review, identifie les conventions et patterns observés dans le diff qui sont absents de `CONVENTIONS.md` ou `ONBOARDING.md`, et propose de les capitaliser. Si accepté, délègue l'écriture au `documentarian` via `task` (skill `living-docs-enrichment`).

---

### `qa-engineer`

| | |
|--|--|
| **Label** | QAEngineer |
| **Fichier** | `agents/quality/qa-engineer.md` |
| **Skills** | `developer/dev-standards-universal`, `posture/expert-posture`, `posture/concision-posture`, `posture/tool-question`, `qa/qa-protocol`, `qa/qa-handoff-format`, `shared/living-docs-enrichment`, `shared/wiki-navigation` — native : `qa/qa-standalone`, `qa/qa-subagent`, `developer/dev-standards-git`, `shared/rtk-usage` |
| **Invocation** | `"Écris les tests pour la branche [X]"` / `"QA sur le ticket [ID]"` |

Écrit les tests manquants (unit / integration / E2E) à partir d'un diff ou d'un
ticket Beads. Produit un rapport de couverture avant/après. Ne modifie jamais
le code fonctionnel.

**Non pertinent pour les tickets TDD** : quand un ticket porte le label `tdd`,
les tests sont écrits par le developer lui-même avant l'implémentation (boucle
red/green/refactor). L'`orchestrator-dev` saute automatiquement le CP-QA pour
ces tickets — le `qa-engineer` n'est pas invoqué.

**Post-rapport — Enrichissement des documents vivants :** après la production du rapport de couverture, identifie les conventions de test adoptées et les cas limites systématiques révélés par les tests qui sont absents de `CONVENTIONS.md`, et propose de les capitaliser. Si accepté, délègue l'écriture au `documentarian` via `task` (skill `living-docs-enrichment`).

> Voir [ADR-004](./adr/004-qa-debugger-separation.fr.md).

---

### `debugger`

| | |
|--|--|
| **Label** | Debugger |
| **Fichier** | `agents/quality/debugger.md` |
| **Skills** | `quality/debugger-workflow`, `quality/debugger-handoff-format`, `shared/living-docs-enrichment`, `posture/expert-posture`, `posture/tool-question`, `shared/wiki-navigation` — native : `quality/debugger-standalone`, `quality/debugger-subagent`, `shared/rtk-usage` |
| **Invocation** | `"Ce bug : [stacktrace]"` / `"Analyse ces logs : [logs]"` |

Diagnostique la cause racine d'un bug en 6 phases structurées : vérification des artefacts
disponibles (Phase 0 — pause si insuffisants) → exploration contextuelle → questions
complémentaires (optionnel) → diagnostic 4 étapes (reproduction/isolation/identification/
hypothèse graduée haute/moyenne/faible) → détection cas particuliers (race conditions,
environnement-spécifique, données, configuration, dépendances, régression). Produit un
rapport de diagnostic avec hypothèses graduées. Crée un ticket Beads de correction après
confirmation explicite. Ne corrige jamais le bug.

**Mode `--forensic`** : analyse criminalistique renforcée avec graduation de preuves
(Confirmed / Deduced / Hypothesized). Stronghold-first — ancrage sur une preuve Confirmed
avant tout raisonnement. Produit un case file `.investigation-{slug}.md` (table d'hypothèses,
preuves, timeline, preuves manquantes). Evidence manquante = finding en soi. Délégation
obligatoire si >5 fichiers ou >10K tokens.

**Phase 5 — Enrichissement des documents vivants :** après le rapport, identifie les zones d'ombre
levées par le diagnostic et les patterns d'erreur à mémoriser, puis propose à l'utilisateur d'enrichir
`ONBOARDING.md` et/ou `CONVENTIONS.md`. Si accepté, délègue l'écriture au `documentarian` via `task`
(skill `living-docs-enrichment`). Ne peut invoquer le `documentarian` sans confirmation explicite.

En mode `orchestrator_feature` : utilise le mécanisme d'interruption de session — chaque checkpoint (fin de phase, pause, confirmation d'action irréversible) produit un bloc `## Retour intermédiaire vers orchestrator` + `## Question pour l'orchestrator` et termine la session.

> Voir [ADR-004](./adr/004-qa-debugger-separation.fr.md).

---

## Famille — Agents de planification

### `planner`

| | |
|--|--|
| **Label** | ProjectPlanner |
| **Fichier** | `agents/planning/planner.md` |
| **Skills** | `developer/beads-plan`, `planning/planner-workflow`, `planning/planner-handoff-format`, `design/design-planner-format`, `posture/expert-posture`, `posture/concision-posture`, `posture/tool-question`, `shared/living-docs-enrichment`, `shared/websearch-usage`, `shared/hub-workflow-reference`, `adapters/figma-planner-protocol`, `adapters/gitlab-planner-protocol` — native : `planning/planner-standalone`, `planning/planner-subagent`, `planning/websearch-stack-research`, `shared/rtk-usage` |
| **MCP Servers** | `figma`, `gitlab` |
| **Invocation** | Description d'une feature en langage naturel / `"Planifie le ticket #42"` |

Consultant fonctionnel et technique qui analyse le contexte projet avant de planifier.
Workflow en 7 phases : vérification prérequis → **complexity scoring** (Phase 0.5 — 4 critères :
domaines techniques, intégrations tiers, sensibilité sécurité, taille codebase ; score 4–16 pts ;
tiers Small/Medium/Large/Enterprise ; conditionne pathfinder obligatoire et audit pré-implémentation)
→ exploration contextuelle (codebase, tickets,
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
- **Option A** (`"invoquer design"`) — invoque directement `designer`
  en sous-agent (mode `ux`, `ui` ou `ux+ui`), attend le bloc structuré `## SPEC UX/UI — …` et intègre la spec dans le plan.
- **Option B** — l'utilisateur invoque lui-même l'agent et colle la spec.
- **Option C** (`"continuer sans design"`) — poursuit avec le contexte disponible,
  tickets `--design` partiels + `bd comments add` pour tracer la spec manquante.

**Phase 6 — Enrichissement des documents vivants :** après validation du plan, identifie les
patterns architecturaux et conventions observées dans la codebase mais absents de
`ONBOARDING.md`/`CONVENTIONS.md`, et propose à l'utilisateur de les capitaliser. Si accepté,
délègue l'écriture au `documentarian` via `task` (skill `living-docs-enrichment`).

---

### `pathfinder`

| | |
|--|--|
| **Label** | Pathfinder |
| **Fichier** | `agents/planning/pathfinder.md` |
| **Skills** | `developer/beads-plan`, `planning/pathfinder-protocol`, `planning/pathfinder-handoff-format`, `posture/concision-posture`, `posture/tool-question`, `shared/websearch-usage`, `shared/living-docs-enrichment`, `shared/wiki-navigation`, `adapters/figma-pathfinder-protocol`, `adapters/gitlab-pathfinder-protocol` — native : `planning/pathfinder-standalone`, `planning/pathfinder-subagent`, `planning/websearch-stack-research`, `shared/rtk-usage` |
| **MCP Servers** | `figma`, `gitlab` |
| **Invocation** | `"Pathfinder la feature [X]"` / `"Estime la complexité de [feature]"` / `"Pathfinder le ticket #42"` |

Agent de reconnaissance rapide. Explore le contexte d'une feature et produit une estimation
de complexité (XS/S/M/L/XL) avec un rapport structuré exploitable par le planner ou l'orchestrator.
Workflow libre — pas de phases rigides. Suggère l'escalade vers le planner si la feature dépasse M.

**Enrichissement GitLab (optionnel) :** si un ticket ou une MR GitLab est fourni, utilise
`gitlab-pathfinder-protocol` pour ajuster l'estimation selon les critères d'acceptation, labels et milestone.

**Enrichissement Figma (optionnel) :** si la feature touche une interface utilisateur, utilise
`figma-pathfinder-protocol` pour détecter les composants Figma et ajuster la complexité.

**Post-rapport — Enrichissement des documents vivants :** après la production du rapport, identifie les patterns architecturaux et conventions observés lors de la reconnaissance qui sont absents de `ONBOARDING.md`/`CONVENTIONS.md`, et propose à l'utilisateur de les capitaliser. Si accepté, délègue l'écriture au `documentarian` via `task` (skill `living-docs-enrichment`).

---

## Famille — Agents de documentation

### `documentarian`

| | |
|--|--|
| **Label** | Documentarian |
| **Fichier** | `agents/documentation/documentarian.md` |
| **Skills** | `developer/dev-standards-git`, `developer/beads-plan`, `developer/beads-dev`, `documentarian/doc-protocol`, `posture/expert-posture`, `posture/tool-question`, `documentarian/documentarian-handoff-format`, `shared/websearch-usage` — native : `documentarian/doc-standards`, `documentarian/doc-adr`, `documentarian/doc-api`, `documentarian/doc-changelog`, `documentarian/doc-slides`, `documentarian/doc-wiki-protocol`, `shared/skill-authoring-protocol`, `shared/rtk-usage` |
| **Invocation** | `"Documente [sujet]"` / `"Crée un ADR pour [décision]"` / `"Mets à jour le CHANGELOG"` / `"Qu'est-ce qui manque dans la doc ?"` / `"Crée une présentation pour [sujet]"` |

Rédige et met à jour la documentation technique, fonctionnelle, architecturale, API,
les changelogs et les présentations Marp. Explore systématiquement la structure existante
avant d'écrire. S'adapte au format en place — recommande des améliorations sans les imposer.
Ne change jamais un format sans confirmation explicite.

Principe directeur : **explorer → adapter ou proposer → attendre si nécessaire → écrire**.

---

## Règles communes à tous les agents

- **Agents en lecture seule** : auditor-subagent, reviewer, designer — ne modifient jamais de fichiers directement
- **Agents qui délèguent l'écriture documentaire** : auditor (coordinateur), planner, debugger — peuvent invoquer le `documentarian` via `task` pour enrichir `ONBOARDING.md` / `CONVENTIONS.md`, uniquement après confirmation explicite de l'utilisateur (skill `living-docs-enrichment`)
- **Agents qui écrivent du code** : `developer`, `developer-refactor`, `developer-migrator`, `qa-engineer` — modifient uniquement les fichiers de leur domaine
- **Agents qui écrivent de la documentation** : documentarian — modifie uniquement les fichiers de documentation ; seul agent autorisé à écrire dans `ONBOARDING.md` et `CONVENTIONS.md` (tous les autres agents peuvent proposer des enrichissements à `ONBOARDING.md`/`CONVENTIONS.md` via la skill `living-docs-enrichment`, toujours délégués au `documentarian` après confirmation explicite de l'utilisateur)
- **Agents qui créent des tickets** : planner (tickets feature), debugger (tickets bug après confirmation)
- **Agents qui lisent les tickets** : tous peuvent faire `bd show <ID>` pour contextualiser leur travail
- **Agents coordinateurs** : orchestrator, orchestrator-dev, auditor — ne codent jamais, pilotent d'autres agents
- **Agents de découverte** : onboarder — lecture seule, explore et rapporte, ne pilote pas d'autres agents
- **Agents `primary`** : orchestrator, orchestrator-dev, planner, auditor, designer, documentarian, onboarder, debugger, qa-engineer, reviewer — visibles directement par l'utilisateur
- **Agents `subagent`** : `developer`, `developer-refactor`, `developer-migrator` et `auditor-subagent` — invocables par des agents coordinateurs
