# Changelog

Toutes les modifications notables de ce projet sont documentées dans ce fichier.

Format : [Keep a Changelog](https://keepachangelog.com/fr/1.0.0/)
Versioning : [Semantic Versioning](https://semver.org/lang/fr/)

---

## [Unreleased]

### Fixed

- **`task_id` — nature clarifiée et garde-fou ajouté** (`task_id-delegation.fr.md`, `orchestrator-protocol.md`) :
  - Le `task_id` est un ID de session OpenCode standard (session persistée côté serveur, non un état LLM) — la reprise de session est fiable tant que la session existe ; le risque de "perte de contexte LLM" n'existe pas
  - Risque résiduel documenté : session introuvable si OpenCode redémarre pendant la fenêtre d'attente entre question montante et reprise
  - Garde-fou ajouté dans `orchestrator-protocol` : "Cas C — session introuvable" — détecter l'absence de résultat après ré-invocation avec `task_id` et proposer à l'utilisateur de relancer depuis les tickets restants ou de stopper

### Documentation

- `docs/architecture/task-delegation.fr.md` : section `### Zone d'ombre` renommée `### Le task_id est un ID de session OpenCode` — nature réelle documentée (session persistante, navigation TUI, SDK), tableau des points encore inconnus, référence au garde-fou
- `docs/architecture/task-delegation.fr.md` : section `### task_id — mécanisme opaque` renommée `### task_id — risque de session introuvable` — tableau causes/probabilité/impact, description du garde-fou

### Added

- **`docs/architecture/task-delegation.fr.md`** — nouveau document de référence sur le mécanisme de délégation inter-agents via l'outil `task` : mécanique de base (paramètres, sessions isolées, permissions par whitelist), hiérarchie des 4 niveaux d'agents, protocoles de communication (handoff contracts), reprise de session via `task_id`, marqueur de contexte d'invocation, checkpoints et compteurs anti-boucle, points d'attention et limites connues

- **Skill `auditor/living-docs-enrichment`** — nouveau skill partagé entre `auditor` (coordinateur), `planner` et `debugger` permettant d'enrichir de manière incrémentale les fichiers `ONBOARDING.md` et `CONVENTIONS.md` du projet cible :
  - **Flux en 5 étapes** : identification des découvertes → résumé affiché en texte clair → confirmation via `question` → délégation au `documentarian` via `task` → confirmation de la délégation
  - **Aucune écriture directe** : le `documentarian` est le seul agent autorisé à écrire dans ces fichiers
  - **Sources de découvertes** : auditor — sections `### Découvertes à documenter` des rapports des 7 sous-agents ; planner — patterns détectés en Phase 1 (conventions de nommage, bibliothèques non documentées) ; debugger — zones d'ombre levées par le diagnostic et patterns d'erreur récurrents
  - **Tableau de correspondance** origine × sections prioritaires pour ONBOARDING.md et CONVENTIONS.md (11 origines : audit sécurité/performance/accessibilité/éco-conception/architecture/privacy/observabilité, diagnostic bug, planification feature)
  - **Règles de qualité** : enrichissements factuels, concis, contextualisés, non redondants, actionnables
- **Section `### Découvertes à documenter`** ajoutée dans le format de rapport des 7 agents `auditor-*` — remontée des découvertes à capitaliser vers le coordinateur, lecture seule stricte conservée (aucun appel `task`)
- **Permissions `task.documentarian = allow`** ajoutées dans `opencode.json` pour `auditor`, `planner`, `debugger` et `orchestrator`

### Changed

- **Agent `auditor`** (`agents/auditor/auditor.md`) : skill `auditor/living-docs-enrichment` ajouté ; Phase 4 enrichie — consolidation des sections `### Découvertes à documenter` et proposition d'enrichissement après la synthèse exécutive ; permission `task.documentarian = allow` ajoutée
- **Agent `planner`** (`agents/planning/planner.md`) : skill `auditor/living-docs-enrichment` ajouté ; Phase 6 enrichie — identification des patterns et conventions observés, proposition d'enrichissement après validation du plan ; permission `task.documentarian = allow` ajoutée
- **Agent `debugger`** (`agents/quality/debugger.md`) : skill `auditor/living-docs-enrichment` ajouté ; Phase 5 enrichie — identification des zones d'ombre levées et patterns d'erreur, proposition d'enrichissement après le rapport ; permission `task.documentarian = allow` ajoutée
- **Agents `auditor-*`** (×7) : ajout de la section `### Découvertes à documenter` en fin de rapport — lecture seule stricte conservée (`write: deny`, aucun appel `task`)

### Fixed

- **Récap partiel vs final** (`orchestrator-handoff-format`, `orchestrator-protocol`, `orchestrator-dev-protocol`) — la distinction entre récap partiel (émis avec une question montante) et récap final (émis seul en fin de session) était implicite et reposait sur un signal contextuel ; rendu explicite par l'ajout d'un champ obligatoire `**Type de récap :** partiel | final` dans le bloc `## Retour vers orchestrator` ; règles de détection et d'interdiction ajoutées dans les trois skills concernés

- **Transmission du mode de workflow** (`orchestrator-workflow-modes`, `orchestrator-protocol`, `orchestrator-dev-protocol`) — le mode (`manuel`/`semi-auto`/`auto`) était transmis via texte libre sans contrat de format ; cinq correctifs appliqués :
  - Valeurs canoniques définies (`manuel`, `semi-auto`, `auto`) et interdiction des labels bruts d'interface
  - Autocontrôle avant délégation côté orchestrator
  - Re-transmission obligatoire du mode dans chaque prompt de reprise `task_id` (correctif critique — corrige une perte silencieuse du mode à chaque CP-2)
  - Règle de parsing documentée côté orchestrator-dev avec fallback `manuel` explicite et signal d'alerte
  - Confirmation visible du mode reçu dans le message de démarrage d'orchestrator-dev

### Documentation

- `docs/architecture/task-delegation.fr.md` : section `### Récap partiel vs final` enrichie — tableau comparatif, arbre de détection, diagramme d'état Mermaid, tableau des erreurs possibles
- `docs/architecture/task-delegation.fr.md` : section `### Transmission du mode via prompt` enrichie — valeurs canoniques, 4 cas de défaillance avec probabilité et impact, résumé des correctifs appliqués
- `docs/architecture/skills.fr.md` : ajout de `auditor/living-docs-enrichment` dans le domaine `auditor/`, mise à jour de la matrice de dépendances (`auditor`, `planner`, `debugger`)
- `docs/architecture/agents.fr.md` : skills et descriptions mis à jour pour `auditor`, `planner`, `debugger` ; règles communes nuancées — distinction lecture seule stricte (`auditor-*`, `reviewer`) vs délégation documentaire autorisée (`auditor`, `planner`, `debugger`)
- `docs/architecture/overview.fr.md` : principe 5 ("Lecture seule pour les agents non-développeurs") mis à jour — précise que l'écriture documentaire passe toujours par le `documentarian` via délégation explicite
- `docs/guides/workflows.fr.md` : ajout d'une étape "Enrichissement des documents vivants" dans le scénario audit (Phase 4) et dans le scénario debug (Phase 5) avec exemples de blocs de proposition

---

### Added

- **Workflows unifiés pour les agents coordinateurs** — 4 agents refactorisés avec workflows natifs en 5-7 phases (récaps systématiques, questions obligatoires via `question`, itérations contrôlées, phases de détection des cas particuliers, format handoff) :
  - **`planner`** : workflow unifié `planner-workflow.md` (7 phases : 0 prérequis → 1 exploration → 1.5 délégation design → 2 questions → 3 plan hiérarchique → 4 cas particuliers → 5 création Beads → 5.5 ai-delegated → 6 vérification)
  - **`onboarder`** : workflow unifié `onboarder-workflow.md` (6 phases : 0 prérequis → 1 exploration adaptative 7 profils → 2 questions → 3 rapport contexte → 4 cas particuliers → 5 production ONBOARDING.md + CONVENTIONS.md) — fusionne `project-discovery.md` et `project-conventions.md`
  - **`debugger`** : workflow unifié `debugger-workflow.md` (6 phases : 0 vérification artefacts → 1 exploration → 2 questions optionnel → 3 diagnostic 4 étapes → 4 cas particuliers → 5 rapport + ticket) — intègre la méthodologie `debug-protocol.md`
  - **`auditor`** : workflow unifié `auditor-workflow.md` (5 phases : 0 vérification prérequis → 1 chargement contexte → 2 sélection domaines avec compatibilité stack → 3 délégation sous-agents → 4 consolidation synthèse exécutive) — les 7 sous-agents `auditor-*` conservent leur workflow technique
- **Règle absolue inter-agents** : récap en texte clair dans la discussion AVANT tout appel à l'outil `question` — garantit la visibilité du contexte pour l'utilisateur et l'orchestrateur
- **Itérations contrôlées** : compteur max 3 par phase dans tous les nouveaux workflows — évite les boucles infinies, propose le passage forcé à la suite à la 3ème itération
- **Contexte d'invocation explicite** : détection du marqueur `[CONTEXTE] Invoqué depuis l'orchestrateur` dans tous les workflows — produit le bloc `## Retour vers orchestrator` en fin de workflow si détecté
- **Gouvernance des workflows** documentée (voir `CHANGELOG` ou `skills/` correspondants) : quand créer un workflow unifié (agents coordinateurs, phases itératives, validations utilisateur) vs workflow technique simple (agents spécialisés, exécution linéaire)

### Changed

- **Agent `planner`** (`agents/planning/planner.md`) : skills mis à jour — `planning/planner-workflow` remplace `planning/planner` + les 3 skills `analysis/*` ; `planning/planner-handoff-format` conservé
- **Agent `onboarder`** (`agents/planning/onboarder.md`) : skills mis à jour — `planning/onboarder-workflow` remplace `planning/project-discovery`, `planning/project-conventions` + les 3 skills `analysis/*` ; `planning/onboarder-handoff-format` conservé
- **Agent `debugger`** (`agents/quality/debugger.md`) : skills mis à jour — `quality/debugger-workflow` remplace `debugger/debug-protocol` ; `quality/debugger-handoff-format` conservé
- **Agent `auditor`** (`agents/auditor/auditor.md`) : skills mis à jour — `auditor/auditor-workflow` remplace `auditor/audit-protocol` + les 3 skills `analysis/*`
- **Agents `auditor-*`** (7 sous-agents) : les 3 skills `analysis/*` retirés du frontmatter — les sous-agents spécialisés n'en avaient pas besoin (workflow technique simple)

### Removed

- **Skills `analysis/*` supprimés** : `skills/analysis/analysis-workflow.md` (545 L), `skills/analysis/analysis-templates.md` (510 L), `skills/analysis/analysis-questions.md` (276 L) — répertoire `skills/analysis/` supprimé. Remplacés par les 4 workflows unifiés natifs.
- **Skills archivés** (renommés `*-legacy.md`) : `planning/planner.md` → `planner-legacy.md`, `planning/project-discovery.md` → `project-discovery-legacy.md`, `planning/project-conventions.md` → `project-conventions-legacy.md`, `debugger/debug-protocol.md` → `debug-protocol-legacy.md`, `auditor/audit-protocol.md` → `audit-protocol-legacy.md`

### Documentation

- `docs/architecture/skills.fr.md` et `skills.en.md` : domaines `planning/`, `debugger/`, `auditor/`, `quality/` mis à jour — nouveaux workflows unifiés, skills archivés, matrice de dépendances agents ↔ skills mise à jour
- `docs/architecture/agents.fr.md` et `agents.en.md` : skills injectés mis à jour pour `planner`, `onboarder`, `debugger`, `auditor`

---

### Added

- **Support des providers OAuth** — github-copilot et ollama peuvent être configurés sans clé API (authentification OAuth native pour github-copilot, pas de clé requise pour ollama)
- **Fix (adapter)** — correction du bug jq `false // true` dans la lecture de `requires_api_key` depuis `providers.json` — l'opérateur `//` traitait `false` comme `null`
- **`oc debug [PROJECT_ID]`** — lance l'agent debugger sur un projet pour diagnostiquer un bug (nouveau script `scripts/cmd-debug.sh`, intégration dans `oc.sh`, aide et i18n mis à jour)
- **`oc project rename <OLD_ID> <NEW_ID>`** — renomme un projet dans tous les fichiers registre (`projects.md`, `paths.local.md`, `api-keys.local.md`) de façon atomique
- **`oc project move <PROJECT_ID> <path>`** — change le chemin local d'un projet dans `paths.local.md`
- **`oc skills validate [name]`** — valide la cohérence des skills (frontmatter `name`/`description`, correspondance nom/fichier, sources externes)
- **`oc agent deploy <agent-id> [PROJECT_ID]`** — déploie un seul agent sans tout redéployer ; respecte les cibles du projet si fourni
- **`oc status --short`** (`-s`) — vue compacte tableau id/chemin/statut (remplace `oc list`)

### Changed

- **`oc list`** — conservé comme alias silencieux vers `oc status --short` (backward compat), retiré du `oc help`
- **`oc provider set <PROJECT_ID>`** et **`oc provider get <PROJECT_ID>`** — supprimés ; utiliser `oc config set/get <PROJECT_ID>` à la place (message d'erreur clair si l'ancienne forme est utilisée)
- **`oc config set`** — le sélecteur de provider est désormais un menu numéroté dynamique depuis `providers.json` (au lieu d'une liste statique codée en dur)
- **`oc update`** — description clarifiée : met à jour les outils installés (opencode, bd, skills externes)
- **`oc upgrade`** — description clarifiée : met à jour les sources du hub via git (git pull ou checkout tag)
- **`oc agent keytest`** — retiré du `oc help` (toujours utilisable, non documenté)
- **`lib/providers.sh`** — helpers `_build_provider_menu` et `_collect_provider_credentials` extraits et partagés (plus de duplication entre `cmd-config.sh` et `cmd-provider.sh`)

### Documentation

- `docs/reference/cli.fr.md` et `cli.en.md` : mise à jour complète — `oc list` → `oc status --short`, nouvelles sections `oc project`, `oc provider` (hub-level uniquement), `oc agent deploy`, `oc skills validate`, clarification `update`/`upgrade`

### Skill `documentarian/doc-slides`
  - 4 templates prêts à l'emploi : `tech-demo`, `product-pitch`, `retrospective`, `onboarding`
  - Directives Marp complètes : frontmatter (`marp: true`, `theme`, `paginate`, `size`), directives locales (`_class`, `_backgroundColor`, `_paginate: false`), séparateurs `---`
  - Bonnes pratiques intégrées : 1 idée par slide, max 5 bullets, titres actionnables, taille recommandée par type de présentation
  - Exploration obligatoire avant génération : slides existants, thème custom, `.marprc`
  - Détection automatique de Marp CLI post-génération (`which marp` / `npx @marp-team/marp-cli`) — proposition de compilation HTML/PDF via `question()` si disponible
  - Fallback si Marp absent : instructions claires (npx, installation globale, extension VS Code, web.marp.app)
  - Nommage normalisé (`kebab-case` + date ISO courte) et emplacement adaptatif (`docs/presentations/`, `docs/slides/`, ou racine)

### Changed

- **Agent `documentarian`** — frontmatter `skills:` enrichi avec `documentarian/doc-slides`
- **Agent `documentarian`** — section "Ce que tu fais" : ajout de la génération de présentations Marp
- **Agent `documentarian`** — table d'exemples : 2 nouveaux cas (`"Crée une présentation pour la démo v2.0"`, `"Slides de retrospective sprint 42"`)
- **Skill `documentarian/doc-protocol`** — tableau de routing : ajout de la ligne `slides, présentation, deck, pitch, diaporama, démo visuelle, retro, onboarding formation` → `doc-slides`

### Documentation

- `docs/architecture/skills.fr.md` et `skills.en.md` : ajout de `documentarian/doc-slides` dans le domaine `documentarian/`, mise à jour de la matrice de dépendances
- `docs/architecture/agents.fr.md` et `agents.en.md` : mise à jour des skills et de la description de l'agent `documentarian`

---

 — 9 skills de contrat de communication formalisés (voir [ADR-009](docs/architecture/adr/009-inter-agent-handoff-contracts.fr.md)) :
  - `auditor/audit-handoff-format` : bloc structuré `## Retour vers orchestrator` pour les 7 agents `auditor-*` — périmètre audité, tableau des vulnérabilités par sévérité, recommandations priorisées avec effort estimé, risque résiduel, statut (`corrections-requises` / `acceptable` / `bloquant`)
  - `design/design-handoff-format` : bloc structuré pour `ux-designer` et `ui-designer` — spec produite intégrale, contraintes d'implémentation, points ouverts, alternatives écartées, statut (`spec-complète` / `spec-partielle` / `bloqué`)
  - `developer/developer-handoff-format` : bloc structuré pour les 9 `developer-*` → `orchestrator-dev` — fichiers modifiés, tests écrits, critères d'acceptance cochés, points d'attention pour la review, blocages rencontrés, statut (`implémenté` / `partiellement-implémenté` / `bloqué`)
  - `planning/onboarder-handoff-format` : bloc structuré pour `onboarder` → `orchestrator` (Mode C) — stack technique détaillée, conventions identifiées, dette technique, zones d'incertitude, fichiers de contexte produits, statut (`contexte-établi` / `contexte-partiel` / `bloqué`)
  - `planning/planner-handoff-format` : bloc structuré pour `planner` → `orchestrator` — tableau complet des tickets créés avec agent prévu et dépendances, hypothèses et ambiguïtés, estimation, risques, statut (`planification-complète` / `planification-partielle` / `bloqué`)
  - `qa/qa-handoff-format` : bloc structuré pour `qa-engineer` → `orchestrator-dev` — tests écrits avec fichiers et cas couverts, critères d'acceptance cochés, zones non testables, statut (`couverture-complète` / `couverture-partielle` / `non-testable`)
  - `quality/debugger-handoff-format` : bloc structuré pour `debugger` → `orchestrator` (Mode D) — cause racine avec niveau de certitude + chaîne causale, hypothèses explorées, impact et régressions, tickets créés, actions d'urgence si bug en prod, statut (`diagnostiqué` / `partiellement-diagnostiqué` / `non-reproductible`)
  - `reviewer/reviewer-handoff-format` : bloc structuré pour `reviewer` → `orchestrator-dev` — verdict actionnable (`commit` / `corriger` / `corriger-sécurité`), synthèse des problèmes par sévérité, corrections requises verbatim, routing recommandé (`retour-initial` / `developer-security`), statut (`approuvé` / `corrections-requises` / `bloquant-sécurité`)

- Nouveau dossier `skills/design/` pour les skills des agents design
- Nouveau dossier `skills/quality/` pour les skills des agents qualité (hors `qa/` et `reviewer/`)

### Changed

- **Agents producteurs** — frontmatter `skills:` mis à jour pour inclure le skill de handoff correspondant :
  - `ux-designer`, `ui-designer` → `design/design-handoff-format`
  - `auditor-security`, `auditor-performance`, `auditor-accessibility`, `auditor-ecodesign`, `auditor-architecture`, `auditor-privacy`, `auditor-observability` → `auditor/audit-handoff-format`
  - `planner` → `planning/planner-handoff-format`
  - `onboarder` → `planning/onboarder-handoff-format`
  - `debugger` → `quality/debugger-handoff-format`
  - `reviewer` → `reviewer/reviewer-handoff-format`
  - `qa-engineer` → `qa/qa-handoff-format`
  - Tous les `developer-*` (×9) → `developer/developer-handoff-format`

- **Agent `orchestrator`** — frontmatter enrichi avec les 5 skills de handoff côté consommateur : `design/design-handoff-format`, `auditor/audit-handoff-format`, `planning/planner-handoff-format`, `planning/onboarder-handoff-format`, `quality/debugger-handoff-format`

- **Agent `orchestrator-dev`** — frontmatter enrichi avec les 3 skills de handoff côté consommateur : `developer/developer-handoff-format`, `reviewer/reviewer-handoff-format`, `qa/qa-handoff-format`

- **Skill `orchestrator/orchestrator-dev-protocol`** :
  - Étape 2 (délégation developer) : détection du bloc `## Retour vers orchestrator-dev` ; routing `bloqué` vers la gestion de blocage sans soumettre à review
  - Étape 3 (QA) : invocation qa-engineer enrichie avec les critères d'acceptance déjà couverts par le developer ; détection du statut QA avant de continuer ; transmission des critères non couverts au reviewer si `couverture-partielle`
  - Étape 4 (review) : invocation reviewer enrichie avec les `### Points d'attention pour la review` du developer
  - Étape 5 (CP-2) : routing de correction basé sur le `### Routing recommandé` du reviewer ; commentaire Beads rempli avec les `### Corrections requises` verbatim (plus de résumé manuel)
  - Étape 6 (compte rendu) : compte rendu enrichi avec fichiers modifiés, couverture des critères d'acceptance, points d'attention techniques agrégés depuis les sous-agents
  - Récap global : colonne `Critères couverts` ajoutée ; `### Points d'attention` alimentés par l'agrégation des retours de toute la chaîne

### Fixed

- **`orchestrator-dev` → `orchestrator` — remontée du bloc `## Retour vers orchestrator` manquante** : le bloc de retour n'était pas produit de manière fiable en fin de session d'`orchestrator-dev` quand invoqué depuis l'orchestrateur feature, empêchant la construction du CP-feature. Corrections apportées :
  - Ajout d'une règle absolue (`✅`) dans la section "Règles absolues" d'`orchestrator-dev-protocol` : le bloc est obligatoire sans exception, y compris en cas de stop, ticket bloqué ou session partielle
  - Transformation de la note conditionnelle de fin de section en une **Étape 2 numérotée et obligatoire** dans la section "Récap global", avec autocontrôle explicite avant clôture de session
  - Ajout dans la section "Ce que tu ne fais PAS" : interdiction de clore la session sans avoir produit le bloc
  - Renforcement du skill `orchestrator/orchestrator-handoff-format` côté producteur : rappel explicite que le bloc est requis même en cas de stop ou de session incomplète, avec autocontrôle

- **Skill `orchestrator/orchestrator-protocol`** :
  - Mode A : détection du bloc structuré `## Retour vers orchestrator` du planner ; présentation des hypothèses et risques au CP-0 avant de démarrer
  - Mode C : détection du bloc structuré de l'onboarder ; présentation des zones d'incertitude et dette technique au CP-onboard
  - Mode D : détection du bloc structuré du debugger ; présentation prioritaire des actions d'urgence si bug en prod
  - Tickets spec-ux/spec-ui : détection du bloc structuré des agents design ; transmission intégrale des `### Contraintes d'implémentation` à orchestrator-dev lors de l'implémentation
  - Tickets audit : détection du bloc structuré des auditors ; transmission intégrale des `### Recommandations priorisées` à orchestrator-dev

### Documentation

- `docs/architecture/skills.en.md` et `skills.fr.md` : ajout des 8 nouveaux skills de handoff dans leurs domaines respectifs, mise à jour de la matrice de dépendances agents ↔ skills
- `docs/architecture/agents.en.md` et `agents.fr.md` : mise à jour des skills injectés pour tous les agents concernés
- `docs/guides/workflows.en.md` et `workflows.fr.md` : ajout de notes sur les retours structurés dans les scénarios 1 et 3
- `docs/guides/contributing.en.md` et `contributing.fr.md` : ajout des nouveaux dossiers `skills/design/` et `skills/quality/`, règle sur les skills de handoff
- `docs/architecture/adr/009-inter-agent-handoff-contracts` (EN + FR) : décision architecturale de formaliser les contrats de communication inter-agents comme skills dédiés

---

## [1.5.0] — 2026-04-30

### Added

- **Skills spécifiques aux stacks** (`skills/developer/stacks/`) — 38 nouveaux skills atomiques organisés par catégorie :
  - **Langages** : `dev-standards-typescript`, `dev-standards-python`
  - **Frontend** : `dev-standards-react`, `dev-standards-nextjs`, `dev-standards-nuxtjs`, `dev-standards-angular`
  - **Backend** : `dev-standards-nestjs`, `dev-standards-express`, `dev-standards-django`, `dev-standards-fastapi`, `dev-standards-laravel`, `dev-standards-rails`, `dev-standards-springboot`
  - **ORMs / BDD** : `dev-standards-prisma`, `dev-standards-typeorm`, `dev-standards-sqlalchemy`, `dev-standards-mongodb`
  - **Spec API** : `dev-standards-openapi`
  - **Test** : `dev-standards-vitest`, `dev-standards-jest`, `dev-standards-playwright`, `dev-standards-cypress`
  - **Mobile** : `dev-standards-react-native`, `dev-standards-flutter`, `dev-standards-swift`, `dev-standards-kotlin`
  - **Data / ML** : `dev-standards-pandas`, `dev-standards-dbt`, `dev-standards-airflow`, `dev-standards-pyspark`
  - **DevOps / CI-CD** : `dev-standards-docker`, `dev-standards-github-actions`, `dev-standards-gitlab-ci`
  - **Platform / Infra** : `dev-standards-terraform`, `dev-standards-kubernetes`, `dev-standards-helm`, `dev-standards-argocd`

- **`config/stack-skills.json`** — table de mapping déclarative : stack détectée → skills à injecter, avec filtrage par type d'agent via `_agent_scope`

- **Détection de stack automatique à `oc deploy`** (`scripts/lib/prompt-builder.sh`) :
  - `detect_stack(project_path)` : détecte la stack depuis `package.json`, `pyproject.toml`, `requirements.txt`, `Gemfile`, `build.gradle`, `pom.xml`, `pubspec.yaml`, `Dockerfile`, `.github/workflows/`, `*.tf`, `Chart.yaml`, manifests K8s/ArgoCD
  - `resolve_stack_skills(agent_id, stacks, config)` : résout les skills à injecter par croisement stacks × `stack-skills.json` × scope d'agent — déduplique avec les skills déclarés en frontmatter
  - `build_agent_content()` : nouveau paramètre `$4 project_path` — si fourni, les stack skills sont injectés après les skills statiques

### Changed

- **Skills génériques purifiés** — suppression de toutes les références spécifiques à des outils ou frameworks dans les skills qui se veulent agnostiques :
  - `dev-standards-universal` : section TypeScript entière supprimée (extraite dans `stacks/dev-standards-typescript`)
  - `dev-standards-testing` : suppression des mentions Vitest, Jest, Playwright, Cypress, Vue, React, axios, SQLite, testcontainers — remplacées par des formulations agnostiques
  - `dev-standards-api` : section "Contrat OpenAPI" → "Contrat d'API (schema-first)", YAML OpenAPI et exemples TypeScript extraits dans `stacks/dev-standards-openapi`
  - `dev-standards-security` : `process.env.API_KEY` → `env("API_KEY")` agnostique, `npm/composer/pip audit` → "gestionnaire de paquets du projet"
  - `dev-standards-devops` : sections Docker, GitHub Actions, GitLab CI extraites dans leurs skills dédiés — garde scripts shell, secrets, registries, observabilité, IaC génériques

- **Skills multi-outils éclatés** en fichiers atomiques :
  - `dev-standards-mobile` → 4 skills (`react-native`, `flutter`, `swift`, `kotlin`) — fichier supprimé
  - `dev-standards-data` → 5 skills (`python`, `pandas`, `dbt`, `airflow`, `pyspark`) — fichier supprimé
  - `dev-standards-platform` → 4 skills (`terraform`, `kubernetes`, `helm`, `argocd`) — fichier supprimé
  - `dev-standards-vuejs` déplacé dans `stacks/`

- **Planner** (`skills/planning/planner.md`) :
  - Règle de granularité des tickets assouplie : un ticket unique est acceptable par défaut ; le découpage n'est suggéré que si **plusieurs** critères sont réunis simultanément (pas un seul)
  - PHASE 0.2 : nouvelle section "Recherche de logique existante" — le planner doit chercher dans **toutes les couches** (backend, frontend, partagé) si une logique similaire existe avant de planifier
  - PHASE 0.3 : section `### Logiques existantes réutilisables` ajoutée au template de résumé de contexte
  - Rappel final n°17 : signaler tout risque de duplication inter-couches dans le résumé de contexte

- **Agents developer** : `skills:` mis à jour pour pointer vers les nouveaux chemins `stacks/` (`developer-mobile`, `developer-data`, `developer-devops`, `developer-platform`, `developer-frontend`, `developer-fullstack`)

- **`oc deploy` / `--diff`** : passage de `deploy_dir` comme `project_path` à l'adapter OpenCode — déclenche la détection de stack automatique pour tout déploiement sur un projet enregistré

### Documentation

- `docs/architecture/skills.en.md` et `skills.fr.md` : refonte complète du domaine `developer/` — distinction skills génériques vs skills spécifiques aux stacks, tables par catégorie (langages, frontend, backend, ORMs, test, mobile, data, infra, platform), matrice de dépendances mise à jour avec mentions des catégories dynamiques
- `docs/guides/authoring.en.md` et `authoring.fr.md` : section "Skills à injecter selon le type d'agent" enrichie avec le mécanisme d'injection dynamique, scope par agent, et instructions pour ajouter une nouvelle stack
- `docs/reference/cli.en.md` et `cli.fr.md` : `oc deploy` documenté avec la détection de stack automatique et son comportement par `PROJECT_ID`

---

## [1.4.0] — 2026-04-22

### Added

- Skill `developer/dev-standards-simplicity` : KISS (solution la plus directe), YAGNI
  (n'implémenter que ce qui est dans le ticket actif), pas d'abstraction prématurée
  (3 cas concrets avant d'abstraire), limites mesurables (fonction ≤ 20 lignes,
  complexité cyclomatique ≤ 10, params ≤ 4, imbrication ≤ 3 niveaux)

### Changed

- Agent `orchestrator` : permissions techniques `bash: deny`, `edit: deny`, `write: deny`
  ajoutées dans le frontmatter — l'agent agit uniquement via `task` et `question` ;
  `task` restreint à une allowlist exhaustive (`planner`, `onboarder`, `ux-designer`,
  `ui-designer`, `auditor-*`, `orchestrator-dev`, `debugger`)
- Skill `orchestrator/orchestrator-protocol` :
  - Mode C conditionné à l'absence des fichiers `ONBOARDING.md` et `CONVENTIONS.md` sur
    disque — si l'un des deux est présent, le contexte est chargé directement sans
    proposer l'onboarder
  - Questions des sous-agents contextualisées : règle ajoutée pour qu'un sous-agent
    invoqué depuis un parent inclue toujours un bloc `[Agent — Phase | Feature]` en
    tête de son champ `question`
  - CP-0 : séparation explicite entre l'affichage du tableau des tickets (dans la
    discussion) et la demande de mode de workflow (outil `question` court, sans tableau)
  - Gestion des agents non déployés : nouvelle section avec table de substitution par
    domaine (`auditor-security → developer-security`, `auditor-accessibility →
    developer-frontend`, `auditor-architecture/performance → developer-fullstack`,
    `auditor-privacy/ecodesign/observability/ux-designer/ui-designer → aucun substitut`),
    question structurée avec option de déploiement via `!oc deploy opencode <PROJECT_ID>`
    sans quitter OpenCode
  - Annonces de délégation enrichies : chaque invocation de sous-agent (planner,
    ux-designer, ui-designer, auditor-*, orchestrator-dev) annonce explicitement que
    les questions remonteront avec leur contexte
  - Mode D — router les bugs vers `debugger` sans tentative de correction autonome
- Skill `posture/tool-question` : nouvelle section "Questions posées en tant que
  sous-agent" — format obligatoire `[Nom — Phase | Feature]` en tête du champ `question`
  quand l'agent est invoqué par un parent
- Skill `orchestrator/orchestrator-workflow-modes` : extrait en source de vérité
  autonome (précédemment intégré dans `orchestrator-dev-protocol`)
- Skill `orchestrator/orchestrator-handoff-format` : extrait en source de vérité
  autonome pour le format de retour `orchestrator-dev → orchestrator`
- `agents/planning/orchestrator.md` : skills mis à jour (`orchestrator-workflow-modes`,
  `orchestrator-handoff-format` ajoutés)
- `docs/architecture/agents.fr.md` / `agents.en.md` : section `orchestrator` enrichie
  (4 modes d'entrée D/C/A/B, permissions techniques, Mode C conditionnel, gestion des
  agents manquants)
- `docs/guides/workflows.fr.md` / `workflows.en.md` : CP-0 clarifié (tableau dans la
  discussion, question courte), notes sur les questions contextualisées des sous-agents
  et sur le comportement face aux agents manquants
- `tests/test_prompt_builder.bats` : 8 nouveaux tests d'intégrité couvrant les
  permissions du frontmatter, la table de substitution, le déploiement sans quitter
  OpenCode, la condition Mode C et la règle de contexte de `tool-question`

### Fixed

- `scripts/lib/prompt-builder.sh` : suppression de la variable `task_json` inutilisée
  (avertissement ShellCheck SC2034)
- Agent `orchestrator-dev` : délégation et outil `question` corrigés — alignement
  avec le protocole `orchestrator-dev-protocol`
- `orchestrator/orchestrator-protocol` et `orchestrator-dev-protocol` : alignement
  des deux protocoles (checkpoints, format handoff, modes de workflow)

---

## [1.3.0] — 2026-04-20

### Added

- Commande `oc review [PROJECT_ID] [--branch <branche>] [--agent <agent>]` : lance
  une review IA sur un projet en invoquant l'agent `reviewer` avec le diff injecté ;
  détecte automatiquement la branche courante si `--branch` absent ; vérifie la
  présence du reviewer dans `projects.md` ; injecte `CONVENTIONS.md` si présent
- `scripts/cmd-review.sh` : implémentation complète de la commande
- `scripts/lib/prompt-builder.sh` : `build_review_bootstrap_prompt` injecte le diff
  `git diff <branche>` et l'hint `CONVENTIONS.md` conditionnel
- `oc.sh` : case `review)` ajouté dans le dispatcher
- `docs/reference/cli.md` : section `oc review` ajoutée
- Skill `orchestrator/orchestrator-workflow-modes` : source de vérité unique pour
  les 3 modes (manuel/semi-auto/auto) — injecté dans `orchestrator` et
  `orchestrator-dev` pour garantir la cohérence
- Skill `orchestrator/orchestrator-handoff-format` : source de vérité unique pour
  le format de retour `orchestrator-dev → orchestrator`

### Changed

- Agent `orchestrator` : `onboarder` ajouté dans la table des agents disponibles,
  Mode C (projet inconnu) ajouté dans le workflow avec checkpoint `[CP-onboard]`
  optionnel et sautables — exemple d'invocation Mode C ajouté
- Skill `orchestrator/orchestrator-protocol` : Mode C documenté avec condition de
  déclenchement, proposition à l'utilisateur, format du `[CP-onboard]` et règle
  "toujours optionnel et sautables"
- Agent `planner` : invocation autonome optionnelle des agents `ux-designer` et
  `ui-designer` ajoutée (PHASE 1.5) — 3 options : invoquer directement (Option A),
  laisser l'utilisateur invoquer (Option B), continuer sans (Option C)
- Agent `orchestrator-dev` : création de branche dédiée par ticket avant implémentation —
  pause obligatoire à l'étape 1b dans tous les modes
- Agents (tous) : outil `question` OpenCode activé sur tous les agents — remplacement
  des pauses textuelles par des appels structurés à l'outil `question`
- `docs(beads)` : état review et cycle de feedback clarifiés
- `docs/architecture/agents.md` : total mis à jour, `onboarder` ajouté dans la
  famille Coordinateurs, nouvelle règle "Agents de découverte"
- `docs/architecture/skills.md` : `planning/project-discovery` ajouté, matrice
  de dépendances mise à jour pour `onboarder`
- `scripts/cmd-help.sh` : refonte avec `.cmd`/`.desc` séparés dans `i18n`,
  section `beads ui` et `tracker set-sync-mode` ajoutées

### Fixed

- `scripts/lib/prompt-builder.sh` : sauts de ligne dans les templates `bd update`
  pour le planner corrigés
- `scripts/cmd-help.sh` : commandes `agent select` et `mode` manquantes ajoutées
- Agent `planner` : sauts de ligne dans les templates `bd update` corrigés
- `fix(onboarding)` : ne pas proposer l'onboarding si `ONBOARDING.md` existe déjà
- `fix(release)` : bumper `hub.json.example` (tracké) au lieu de `hub.json` (ignoré)
- Agents `orchestrator`/`orchestrator-dev` : synchronisation de la permission
  `question` et du skill `tool-question`
- CI : avertissements ShellCheck corrigés dans `cmd-board` et `common`

---

## [1.2.0] — 2026-04-15

### Added

- Support natif AWS Bedrock (`amazon-bedrock`) : détection automatique du provider
  dans `opencode.adapter.sh`, sync `opencode.json` avec region et token
  `AWS_BEARER_TOKEN_BEDROCK` ; différencié du mode litellm
- Support région AWS pour le provider `amazon-bedrock` dans `providers.json`
- `feat(beads)` : ajout de `.beads/` au `.git/info/exclude` à l'init
- `feat(i18n)` : clés `beads.gitignore_added` et `beads.gitignore_exists` ajoutées
- `feat(beads-ui)` : intégration de `bdui` dans `oc install`, `oc update` et la
  documentation
- Import automatique des labels tracker (GitLab / Jira) à l'init Beads

### Changed

- `feat(deploy)` : utilisation de `.git/info/exclude` au lieu de `.gitignore` dans
  les projets cibles — évite de polluer le `.gitignore` versionné des projets
- `chore(config)` : `hub.json` et `opencode.json` retirés du tracking git, ajoutés
  à `.gitignore`
- `docs` : section prérequis retirée du README (EN + FR)

### Fixed

- `fix(beads)` : remplacement de `bd label add` par `bd label create` dans
  `cmd-init.sh` — alignement avec l'API Beads actuelle
- `fix(tests)` : stabilisation des tests BATS pour CI sans `hub.json`
- `test` : assertions BATS corrigées (`bd label add` → `bd label create`)

---

## [1.1.0] — 2026-04-13

### Added

- `feat(beads)` : champ `Sync mode` dans `projects.md` et commande
  `oc beads tracker set-sync-mode` pour configurer le mode de synchronisation
  du tracker
- Commande `oc init` : proposition d'ajout de `opencode.json` et `.opencode/` au
  `.gitignore` du projet à l'étape 5

### Fixed

- `fix(init)` : suppression des déclarations `local` invalides hors scope de fonction
- `fix(help)` : commandes `agent select` et `mode` manquantes ajoutées dans l'aide

---

## [1.0.0] — 2026-03-29

### Added

- Commande `oc upgrade` : met à jour les sources du hub via `git pull` (main) ou
  `git checkout <tag>` (`oc upgrade v1.1.0`). Propose `oc sync` après mise à jour réussie.
  Support du one-liner `VERSION=vX.Y.Z` dans `install.sh` pour installer une version épinglée.
- Agent `documentarian` (famille Documentation) avec 5 skills spécialisés :
  `doc-protocol`, `doc-standards`, `doc-adr`, `doc-api`, `doc-changelog`
- Skill `planning/planner.md` : Phase 0 (exploration adaptative de la codebase
  et des tickets existants, résumé de contexte), Phase 1 (questions contextualisées,
  priorités déduites et justifiées), Phase 2 (plan hiérarchique epics → tickets,
  règle >5 tickets pour création epics dans Beads), Phase 3 (`--parent`, `--deps`,
  `--estimate`), Phase 4 (`bd children`), section gestion des aléas
- `CHANGELOG.md` et `CONTRIBUTING.md` à la racine du dépôt

### Changed

- Restructuration de `agents/` en sous-dossiers par famille :
  `auditor/`, `developer/`, `documentation/`, `planning/`, `quality/`
- Migration `skills/planner.md` → `skills/planning/planner.md` — cohérence
  avec la convention de sous-dossiers par domaine
- Agent `planner` : frontmatter enrichi (skill `developer/dev-beads` ajouté),
  corps restructuré avec ce que l'agent lit, ce qu'il produit, tableau des aléas
- CI `validate-agents` : glob `agents/*.md` → `find agents/ -name "*.md"`
  pour couvrir la structure en sous-dossiers (le job était en faux positif permanent)

### Fixed

- `scripts/cmd-agent.sh` : `_find_agent_file` réécrit avec process substitution
  `< <(find ...)` — le `return 0` dans un pipe ne sortait pas de la fonction
- `scripts/cmd-skills.sh` : message d'aide corrigé (`agents/*.md` →
  `agents/<famille>/<id>.md`)
- `docs/guides/contributing.md` : chemins `agents/auditor.md`,
  `agents/developer-frontend.md` et `scripts/adapter-manager.sh` obsolètes corrigés
- `docs/architecture/skills.md` : matrice ASCII `developer-fullstack` complétée
  avec `dev-standards-frontend-a11y` et `dev-standards-vuejs`

---

## [0.1.0] — 2026-03-26

### Added

- Hub central multi-cible : OpenCode
- CLI `oc.sh` avec 13 commandes : `init`, `deploy`, `start`, `list`, `remove`,
  `agent`, `skills`, `beads`, `sync`, `update`, `install`, `version`, `help`
- 19 agents initiaux organisés en 5 familles :
  - Coordinateurs : `orchestrator`, `auditor`
  - Développeurs : `developer-frontend`, `developer-backend`, `developer-fullstack`,
    `developer-data`, `developer-devops`, `developer-mobile`, `developer-api`
  - Qualité : `reviewer`, `qa-engineer`, `debugger`
  - Audit : `auditor-security`, `auditor-performance`, `auditor-accessibility`,
    `auditor-ecodesign`, `auditor-architecture`, `auditor-privacy`
  - Planification : `planner`
- 27 skills organisés par domaine (`developer/`, `auditor/`, `orchestrator/`,
  `qa/`, `debugger/`, `reviewer/`)
- 1 adapter : `opencode.adapter.sh`
- Intégration Beads (`bd`) pour la gestion des tickets : `cmd-beads.sh`,
  workflow `bd claim → implémenter → bd close` dans tous les agents developers
- Commande `oc agent` : création interactive, édition, liste, info
- Commande `oc skills` : liste, ajout de sources externes, `used-by`
- Sélecteur de skills interactif avec navigation clavier (flèches + espace)
- Staleness detection : `oc deploy --check` pour détecter les agents obsolètes
- CI GitHub Actions : ShellCheck, validation frontmatter agents, staleness check
- Documentation complète : 5 ADR, guides (getting-started, workflows, contributing),
  référence CLI et config, architecture overview avec diagrammes Mermaid
- Support multi-projets via `projects.md` et `oc init` / `oc start`
- Config `hub.json` : targets actives, modèle IA, skills globaux VS Code
