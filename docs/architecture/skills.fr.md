# Référence des skills

Les skills contiennent des protocoles détaillés, des formats de sortie, des checklists et des règles que les agents appliquent.
Le hub utilise une **architecture hybride** avec deux chemins de déploiement — voir [ADR-010](./adr/010-hybrid-skills-architecture.fr.md).

## Chemins de déploiement

| Chemin | Champ frontmatter | Déployé vers | Quand chargé |
|--------|------------------|--------------|--------------|
| **Inline (Bucket A)** | `skills: [...]` | Assemblé dans le system prompt de l'agent au déploiement | Toujours — dès le premier token |
| **Natif (Bucket B)** | `native_skills: [...]` | `.opencode/skills/<name>/SKILL.md` | À la demande — le LLM les charge via l'outil `skill` quand la tâche le requiert |

**Bucket A** — Protocoles de workflow, formats de handoff, principes universels, skills de posture, skills d'exécution de base (`beads-plan`, `beads-dev`, `quick-fix`). Doit être actif dès le premier token.

**Bucket B** — Standards de domaine, skills spécifiques aux stacks, checklists d'audit, skills de type documentaire, skills de recherche contextuelle. Chargées uniquement quand la tâche de l'agent nécessite ce contexte de domaine spécifique.

Les agents qui utilisent des skills natives ont `permission: skill: allow` dans leur frontmatter.
Les agents coordinateurs/orchestrateurs qui n'ont jamais besoin de skills contextuelles ont `permission: skill: deny`.

---

## Format d'un skill

```markdown
---
name: <nom-du-skill>
description: <Description courte — visible dans oc agent edit et oc skills list>
---

# Skill — <Titre>

<Corps du skill>
```

> La clé `name` est documentaire. Les scripts hub lisent uniquement `description`.
> Le chemin du fichier est la référence utilisée dans le frontmatter des agents.

---

## Domaine — `developer/`

Skills de standards de développement. Partagés entre les agents développeurs et le reviewer.

### Skills génériques

Les skills marqués **(A)** sont Bucket A — toujours inline. Les skills marqués **(B)** sont Bucket B — natifs, chargés à la demande.

| Fichier | Bucket | Agents qui l'utilisent | Contenu |
|---------|--------|----------------------|---------|
| `developer/beads-plan.md` | **A** | Tous les developer-*, planner, onboarder, designers, documentarian | Lecture et création de tickets Beads : `bd list`, `bd show`, `bd create`, `bd label list-all`, liens externes |
| `developer/beads-dev.md` | **A** | Tous les developer-*, designers, documentarian | Workflow exécuteur Beads : `bd update --claim`, `bd close --suggest-next`, règles `ai-delegated` |
| `developer/dev-standards-universal.md` | **A** | Tous les developer-*, reviewer | Clean Code, SOLID complet, nommage, structure — **agnostique du langage** |
| `developer/dev-standards-security.md` | **B** | Tous les developer-*, reviewer | Secrets/config, validation des inputs, injections (SQL/shell/LDAP), auth/autorisation, logs sans données sensibles, audit des dépendances — **agnostique des outils** |
| `developer/dev-standards-backend.md` | **B** | developer-backend, developer-fullstack, developer-api, reviewer | Architecture en couches, DTOs, services, repositories, sécurité API |
| `developer/dev-standards-frontend.md` | **B** | developer-frontend, developer-fullstack, reviewer | Séparation logique/présentation, performance, bundle, lazy loading |
| `developer/dev-standards-frontend-data.md` | **B** | developer-frontend, developer-fullstack, reviewer | Gestion des données côté frontend — 5 questions de caractérisation, tableau de décision (état local, Context Provider, Store, Queries, Cookies, WebStorage, IndexedDB, Query String), fiches détaillées avec trade-offs, règle d'or "élaguer sa donnée" |
| `developer/dev-standards-frontend-a11y.md` | **B** | developer-frontend, developer-fullstack, reviewer | WCAG 2.1 A/AA, sémantique HTML, ARIA, contrastes |
| `developer/dev-standards-testing.md` | **B** | developer-frontend, developer-backend, developer-fullstack, developer-api, developer-data | Stratégie de tests, pyramide, coverage, TDD — **agnostique des outils** |
| `developer/dev-standards-git.md` | **B** | Tous les developer-*, reviewer | Conventional Commits, branches, PR, messages de commit |
| `developer/dev-standards-devops.md` | **B** | developer-devops | Scripts shell, gestion des secrets, registries d'images, observabilité, principes IaC — **agnostique des outils** |
| `developer/dev-standards-api.md` | **B** | developer-api | Versioning d'API, pagination, format de réponse uniforme, codes HTTP, idempotence, contrat schema-first, breaking changes, webhooks, rate limiting |
| `developer/dev-standards-security-hardening.md` | **B** | developer-security | CORS, headers HTTP (CSP, HSTS, X-Frame-Options), bcrypt/argon2id, JWT (rotation, révocation), sessions (httpOnly/secure/sameSite), rate limiting, chiffrement AES-256-GCM |
| `developer/dev-standards-simplicity.md` | **A** | Tous les developer-* | KISS, YAGNI, pas d'abstraction prématurée, pas d'optimisation prématurée, limites de complexité mesurables (longueur de fonction, cyclomatique, paramètres, imbrication, dépendances injectées), signaux d'over-engineering à challenger |
| `developer/developer-handoff-format.md` | **A** | Tous les developer-*, orchestrator-dev | **Contrat de handoff** — bloc structuré `## Retour vers orchestrator-dev` : fichiers modifiés, tests écrits, statut Beads `review`, critères d'acceptance cochés un par un, points d'attention pour la review, blocages rencontrés, statut (`implémenté` / `partiellement-implémenté` / `bloqué`) |

### Skills spécifiques aux stacks — `developer/stacks/` (Bucket B — natif)

Ces skills sont **Bucket B — natifs**. Au déploiement, `deploy_native_skills()` les déploie vers `.opencode/skills/` en fonction de la stack détectée dans le projet cible par `detect_stack()`. Le LLM charge ceux qui sont pertinents à la demande lors de l'inférence.

Le mapping entre les stacks détectées et les skills à injecter est déclaré dans `config/stack-skills.json`. Chaque type d'agent (`developer-frontend`, `developer-backend`, etc.) a un scope défini qui limite les catégories de stack skills qu'il reçoit.

#### Langages

| Fichier | Stack détectée | Contenu |
|---------|---------------|---------|
| `developer/stacks/dev-standards-typescript.md` | `typescript` dans les dépendances | Config stricte, interfaces vs types, enums, types partagés, erreurs typées, type guards, generics |
| `developer/stacks/dev-standards-python.md` | `pyproject.toml` / `requirements.txt` présent | Version, ruff, mypy/pyright, nommage, exceptions custom, logging, pytest |

#### Frameworks frontend

| Fichier | Stack détectée | Contenu |
|---------|---------------|---------|
| `developer/stacks/dev-standards-vuejs.md` | `vue` dans les dépendances | Composition API, `<script setup>`, Pinia, composables, Vue Router |
| `developer/stacks/dev-standards-react.md` | `react` dans les dépendances | Hooks, TanStack Query, memo/useCallback, RTL, conventions |
| `developer/stacks/dev-standards-nextjs.md` | `next` dans les dépendances | App Router, Server/Client Components, ISR, Server Actions, métadonnées |
| `developer/stacks/dev-standards-nuxtjs.md` | `nuxt` dans les dépendances | Auto-imports, useFetch, routes serveur Nitro, Pinia setup, routeRules |
| `developer/stacks/dev-standards-angular.md` | `@angular/core` dans les dépendances | Standalone components, Signals, inject(), RxJS, Reactive Forms, lazy routing |

#### Frameworks backend

| Fichier | Stack détectée | Contenu |
|---------|---------------|---------|
| `developer/stacks/dev-standards-nestjs.md` | `@nestjs/core` dans les dépendances | Modules, DTOs + class-validator, guards, ConfigService + Joi, tests unitaires |
| `developer/stacks/dev-standards-express.md` | `express` ou `fastify` dans les dépendances | Routing par domaine, middleware zod, AppError, helmet/cors, handler d'erreur global |
| `developer/stacks/dev-standards-django.md` | `django` dans les dépendances Python | BaseModel UUID, FormRequest, serializers I/O, services, migrations |
| `developer/stacks/dev-standards-fastapi.md` | `fastapi` dans les dépendances Python | pydantic-settings, schemas Pydantic v2, inject(), services async, tests httpx |
| `developer/stacks/dev-standards-laravel.md` | `laravel` dans Gemfile/composer | Eloquent, FormRequest, API Resources, service objects, queues/jobs |
| `developer/stacks/dev-standards-rails.md` | `rails` dans Gemfile | MVC, service objects, query objects, scopes, RSpec request specs |
| `developer/stacks/dev-standards-springboot.md` | `spring-boot` dans build.gradle/pom.xml | Entités JPA, record DTOs + @Valid, @Transactional, ProblemDetail, MockMvc |

#### ORMs / Bases de données

| Fichier | Stack détectée | Contenu |
|---------|---------------|---------|
| `developer/stacks/dev-standards-prisma.md` | `@prisma/client` dans les dépendances | Schema, client singleton, select explicite, transactions, migrate deploy |
| `developer/stacks/dev-standards-typeorm.md` | `typeorm` dans les dépendances | Entités select:false, repository custom, QueryBuilder paramétré, QueryRunner |
| `developer/stacks/dev-standards-sqlalchemy.md` | `sqlalchemy` dans les dépendances Python | Mapped v2, sessions async, Alembic, transactions context manager |
| `developer/stacks/dev-standards-mongodb.md` | `mongoose` dans les dépendances | Schemas Mongoose, lean(), indexes, agrégations documentées, transactions |

#### Spec API

| Fichier | Stack détectée | Contenu |
|---------|---------------|---------|
| `developer/stacks/dev-standards-openapi.md` | `openapi.yaml` / `swagger.yaml` présent | `$ref`, schemas/réponses/params réutilisables, writeOnly, sécurité JWT, codegen |

#### Outils de test

| Fichier | Stack détectée | Contenu |
|---------|---------------|---------|
| `developer/stacks/dev-standards-vitest.md` | `vitest` dans les dépendances | vi.mock, vi.fn, vi.spyOn, vi.useFakeTimers, Vue Test Utils |
| `developer/stacks/dev-standards-jest.md` | `jest` dans les dépendances | jest.mock, jest.fn, jest.spyOn, tests comportementaux RTL, snapshots |
| `developer/stacks/dev-standards-playwright.md` | `@playwright/test` dans les dépendances | Locators sémantiques (getByRole), waits sémantiques, POM, fixtures de session |
| `developer/stacks/dev-standards-cypress.md` | `cypress` dans les dépendances | data-cy, cy.intercept + alias, commandes custom, cy.session |

#### Mobile

| Fichier | Stack détectée | Contenu |
|---------|---------------|---------|
| `developer/stacks/dev-standards-react-native.md` | `react-native` dans les dépendances | Expo, React Navigation, Zustand/RTK, TanStack Query, Detox |
| `developer/stacks/dev-standards-flutter.md` | `flutter` dans pubspec.yaml | BLoC/Riverpod, Clean Arch, freezed, flutter_test, mockito |
| `developer/stacks/dev-standards-swift.md` | Projet Xcode détecté | SwiftUI, MVVM, Swift Concurrency, Keychain, XCTest async |
| `developer/stacks/dev-standards-kotlin.md` | `jetpack compose` dans build.gradle | Jetpack Compose, MVVM+Clean, Hilt, Coroutines+Flow, JUnit5+Mockk+Turbine |

#### Data / ML

| Fichier | Stack détectée | Contenu |
|---------|---------------|---------|
| `developer/stacks/dev-standards-pandas.md` | `pandas` dans les dépendances Python | Vectorisation, pandera, pipeline .pipe(), tests DataFrame |
| `developer/stacks/dev-standards-dbt.md` | `dbt-*` dans les dépendances Python | Layers staging/intermediate/mart, schema.yml, tests natifs + personnalisés |
| `developer/stacks/dev-standards-airflow.md` | `apache-airflow` dans les dépendances Python | TaskFlow API, idempotence, Connections/Variables, tests de structure DAG |
| `developer/stacks/dev-standards-pyspark.md` | `pyspark` dans les dépendances Python | DataFrame API, broadcast join, partitionnement, ML lifecycle, MLflow, tests locaux |

#### DevOps / CI-CD

| Fichier | Stack détectée | Contenu |
|---------|---------------|---------|
| `developer/stacks/dev-standards-docker.md` | `Dockerfile` présent | Multi-stage, non-root, .dockerignore, Compose healthchecks, BuildKit secrets |
| `developer/stacks/dev-standards-github-actions.md` | `.github/workflows/` présent | Permissions minimales, concurrency, SHA pinning, OIDC, environments avec approbation |
| `developer/stacks/dev-standards-gitlab-ci.md` | `.gitlab-ci.yml` présent | rules (pas only/except), templates YAML, variables masked, when:manual en prod |

#### Platform / Infrastructure

| Fichier | Stack détectée | Contenu |
|---------|---------------|---------|
| `developer/stacks/dev-standards-terraform.md` | Fichiers `*.tf` présents | Modules, variables + validation, state remote, cycle de vie (plan → PR → apply via pipeline) |
| `developer/stacks/dev-standards-kubernetes.md` | Manifests K8s présents | Deployment, RBAC, NetworkPolicy, ResourceQuota, PDB, Kustomize |
| `developer/stacks/dev-standards-helm.md` | `Chart.yaml` présent | Structure chart, values sans secrets, ExternalSecret dans les templates, helm diff + --atomic |
| `developer/stacks/dev-standards-argocd.md` | Manifests ArgoCD présents | Principes GitOps, sync policies par env (auto staging / manuel prod), ESO, Vault |

---

## Domaine — `auditor/`

Skills d'audit. Les skills marqués **(A)** sont Bucket A — inline. Les skills marqués **(B)** sont Bucket B — natifs.

| Fichier | Bucket | Agents qui l'utilisent | Contenu |
|---------|--------|----------------------|---------|
| `auditor/auditor-workflow.md` | **A** | auditor | **Workflow du coordinateur** — 5 phases (0 vérification prérequis → 1 chargement contexte projet → 2 sélection domaines avec compatibilité stack → 3 délégation sous-agents → 4 consolidation synthèse exécutive) — récaps systématiques, questions obligatoires. La logique standalone/sous-agent est extraite dans les skills de parcours dédiés. |
| `auditor/auditor-standalone.md` | **B** | auditor | **Parcours standalone** — récaps texte avant outil `question`, questions de validation par phase, synthèse finale sans bloc handoff |
| `auditor/auditor-subagent.md` | **B** | auditor | **Parcours sous-agent** — mécanisme d'interruption session à chaque phase (0-3), blocs structurés `## Retour intermédiaire` + `## Question pour l'orchestrateur`, `task_id` obligatoire |
| `auditor/audit-protocol-light.md` | **A** | tous les auditor-* | Format de rapport commun allégé (sous-agents uniquement) : 4 niveaux de criticité (🔴/🟠/🟡/💡), scoring /10, format des findings individuels |
| `auditor/audit-security.md` | **B** | auditor-security | OWASP Top 10, injections, secrets exposés, auth, CORS, CVE |
| `auditor/audit-performance.md` | **B** | auditor-performance | Core Web Vitals, LCP, CLS, TTI, requêtes N+1, cache, bundle |
| `auditor/audit-accessibility.md` | **B** | auditor-accessibility | WCAG 2.1 AA, RGAA 4.1, sémantique, ARIA, navigation clavier, contrastes |
| `auditor/audit-ecodesign.md` | **B** | auditor-ecodesign | RGESN, GreenIT, Écoindex, transfert de données, ressources, obsolescence |
| `auditor/audit-architecture.md` | **B** | auditor-architecture | SOLID, Clean Architecture, dette technique, couplage, cohésion |
| `auditor/audit-privacy.md` | **B** | auditor-privacy | RGPD articles 5/6/17/25/32, EDPB, CNIL, minimisation, consentement |
| `auditor/audit-observability.md` | **B** | auditor-observability | Méthode RED (Rate/Errors/Duration), logs structurés, OpenTelemetry, SLOs/error budget, alerting (actionnable, runbooks), dashboards, grille des 5 questions |
| `auditor/audit-handoff-format.md` | **A** | tous les auditor-*, orchestrator | **Contrat de handoff** — bloc structuré `## Retour vers orchestrator` : périmètre audité, tableau des vulnérabilités par sévérité, recommandations priorisées avec estimation d'effort, risque résiduel, statut (`corrections-requises` / `acceptable` / `bloquant`) |

---

## Domaine — `orchestrator/`

| Fichier | Agents qui l'utilisent | Contenu |
|---------|----------------------|---------|
| `orchestrator/orchestrator-protocol.md` | orchestrator | Workflow feature complet, matrice de routing (3 familles : design, auditor, dev via orchestrator-dev), format des checkpoints ([CP-0], [CP-spec], [CP-audit], [CP-feature]), gestion des cas particuliers, validation des retours structurés pour chaque type de sous-agent |
| `orchestrator/orchestrator-dev-protocol.md` | orchestrator-dev | Workflow Beads ticket par ticket, matrice de routing developer-* (9 signaux → 9 agents), format des checkpoints ([CP-1] à [CP-3] + [CP-QA]), 3 modes (manuel/semi-auto/auto), détection du label `tdd`, exploitation des retours structurés. La logique standalone/sous-agent est extraite dans les skills de parcours dédiés. |
| `orchestrator/orchestrator-dev-standalone.md` | **B** | orchestrator-dev | **Parcours standalone** — CP-0 demande le mode, tous les CPs via outil `question`, todo list visible et mise à jour avec labels de phase |
| `orchestrator/orchestrator-dev-subagent.md` | **B** | orchestrator-dev | **Parcours sous-agent** — CPs à enjeu fort produisent des blocs `## Question pour l'orchestrator` + `## Retour vers orchestrator` (partiel), session terminée après chaque CP à enjeu fort |
| `orchestrator/orchestrator-handoff-format.md` | orchestrator-dev, orchestrator | **Contrat de handoff** — deux formats : `## Retour vers orchestrator` (fin de session : le producteur émet d'abord la synthèse condensée par ticket (statut, fichiers clés, critères couverts, points d'attention + points d'attention globaux agrégés), puis le bloc structuré avec tableau de détail par ticket — agent, QA, cycles de review, critères couverts, statut — plus points d'attention et statut global `succès`/`partiel`/`bloqué` ; le consommateur affiche cette synthèse dans son fil de discussion avant de construire le [CP-feature]) et `## Question pour l'orchestrator` (CPs à enjeu fort : CP-2, blocage 3 cycles, dépendance non résolue, ticket bloqué — contexte complet, question en attente, options, `task_id` pour reprise de session) |
| `orchestrator/orchestrator-workflow-modes.md` | orchestrator, orchestrator-dev | Source de vérité unique pour les 3 modes de workflow (manuel/semi-auto/auto) — blocs question canoniques, règles absolues par mode |

---

## Domaine — `qa/`

| Fichier | Agents qui l'utilisent | Contenu |
|---------|----------------------|---------|
| `qa/qa-protocol.md` | qa-engineer | Protocole QA — typologie des tests (unit/integration/E2E/composants), outils par stack, checklist systématique, format du rapport. La logique standalone/sous-agent est dans les skills de parcours dédiés. |
| `qa/qa-standalone.md` | **B** | qa-engineer | **Parcours standalone** — rapport QA sans bloc handoff |
| `qa/qa-subagent.md` | **B** | qa-engineer | **Parcours sous-agent** — rapport QA + bloc `## Retour vers orchestrator-dev` obligatoire |
| `qa/qa-handoff-format.md` | qa-engineer, orchestrator-dev | **Contrat de handoff** — bloc structuré `## Retour vers orchestrator-dev` : tests écrits avec fichiers et cas couverts, critères d'acceptance cochés, zones non testables, statut (`couverture-complète` / `couverture-partielle` / `non-testable`) |

---

## Domaine — `quality/`

Skills de qualité pour les agents qui ne sont pas qa-engineer ni reviewer.

| Fichier | Agents qui l'utilisent | Contenu |
|---------|----------------------|---------|
| `quality/debugger-workflow.md` | debugger | **Workflow unifié** — 6 phases (0 vérification artefacts → 1 exploration contextuelle → 2 questions complémentaires optionnel → 3 diagnostic 4 étapes : reproduction/isolation/identification/hypothèse graduée → 4 détection cas particuliers : race condition, environnement, données, configuration, dépendances, régression → 5 rapport + ticket Beads) — récaps systématiques, hypothèses graduées (haute/moyenne/faible probabilité), bloc `## Retour vers orchestrator` si invoqué depuis orchestrateur |
| `quality/debugger-handoff-format.md` | debugger, orchestrator | **Contrat de handoff** — bloc structuré `## Retour vers orchestrator` : cause racine avec niveau de certitude (confirmé/probable/incertain) + chaîne causale, hypothèses explorées, impact et régressions potentielles, tickets de correction créés, actions d'urgence si bug en prod, statut (`diagnostiqué` / `partiellement-diagnostiqué` / `non-reproductible`) |

---

## Domaine — `reviewer/`

| Fichier | Agents qui l'utilisent | Contenu |
|---------|----------------------|---------|
| `reviewer/review-protocol.md` | reviewer | Protocole de review — format du rapport, niveaux de sévérité, checklist, mode "audit complet". La logique standalone/sous-agent est dans les skills de parcours dédiés. |
| `reviewer/reviewer-standalone.md` | **B** | reviewer | **Parcours standalone** — rapport de review sans bloc handoff |
| `reviewer/reviewer-subagent.md` | **B** | reviewer | **Parcours sous-agent** — rapport de review + bloc `## Retour vers orchestrator-dev` obligatoire |
| `reviewer/reviewer-handoff-format.md` | reviewer, orchestrator-dev | **Contrat de handoff** — bloc structuré `## Retour vers orchestrator-dev` : verdict actionnable (`commit` / `corriger` / `corriger-sécurité`), synthèse des problèmes par sévérité, corrections requises verbatim (collées directement dans le commentaire Beads), routing recommandé (`retour-initial` / `developer-security`), statut (`approuvé` / `corrections-requises` / `bloquant-sécurité`) |

---

## Domaine — `documentarian/`

Skills de documentation. Les skills marqués **(A)** sont Bucket A — inline. Les skills marqués **(B)** sont Bucket B — natifs.

| Fichier | Bucket | Agents qui l'utilisent | Contenu |
|---------|--------|----------------------|---------|
| `documentarian/doc-protocol.md` | **A** | documentarian | Exploration obligatoire avant rédaction, tableau d'adaptation en 4 situations (format conforme / améliorable / absent / partiel), routing par type de doc, checklist de lacunes, workflow Beads et direct |
| `documentarian/doc-standards.md` | **B** | documentarian | Framework Diataxis (4 quadrants), principes de lisibilité, structures type par document (README, how-to, référence), anti-patterns courants, critères de qualité, documentation fonctionnelle |
| `documentarian/doc-adr.md` | **B** | documentarian | Détection du format existant (Nygard / MADR / Y-Statements / maison), format MADR de référence, règles de nommage, statuts (proposed/accepted/deprecated/superseded), critères de création |
| `documentarian/doc-api.md` | **B** | documentarian | OpenAPI 3.x (squelette, endpoint, schemas réutilisables), codes HTTP, documentation narrative (guide d'utilisation, pagination, gestion des erreurs), identification et documentation des breaking changes |
| `documentarian/doc-changelog.md` | **B** | documentarian | Keep a Changelog (6 sections), SemVer (MAJOR/MINOR/PATCH), Conventional Commits → sections changelog, génération depuis git log, workflow de release, release notes format étendu |
| `documentarian/doc-slides.md` | **B** | documentarian | Génération de présentations Marp (Markdown → HTML/PDF) — 4 templates (tech-demo, product-pitch, retrospective, onboarding), directives Marp (frontmatter, `---`, `_class`, `backgroundColor`), bonnes pratiques (1 idée/slide, max 5 bullets, titres actionnables), détection automatique de Marp CLI post-génération et proposition de compilation, fallback avec options d'installation si absent |
| `documentarian/documentarian-handoff-format.md` | **A** | documentarian, orchestrator-dev | **Contrat de handoff** — bloc structuré `## Retour vers orchestrator-dev` : type de documentation produite, fichiers modifiés, résumé de l'entrée, statut (`documenté` / `partiellement-documenté` / `bloqué`) |

---

## Domaine — `planning/`

Les skills marqués **(A)** sont Bucket A — inline. Les skills marqués **(B)** sont Bucket B — natifs.

| Fichier | Bucket | Agents qui l'utilisent | Contenu |
|---------|--------|----------------------|---------|
| `planning/planner-workflow.md` | **A** | planner | **Workflow planner** — 7 phases (0 prérequis → 1 exploration contextuelle + signaux UX/UI → 1.5 délégation design → 2 questions → 3 plan hiérarchique → 4 cas particuliers → 5 création Beads → 5.5 ai-delegated → 6 vérification) — récaps systématiques, phases itératives. La logique standalone/sous-agent est extraite dans les skills de parcours dédiés. |
| `planning/planner-standalone.md` | **B** | planner | **Parcours standalone** — récaps texte avant outil `question`, format des questions de validation par phase, sans bloc handoff orchestrateur |
| `planning/planner-subagent.md` | **B** | planner | **Parcours sous-agent** — mécanisme d'interruption session à chaque phase, blocs structurés, `task_id` obligatoire, terminaison de session après chaque checkpoint |
| `planning/onboarder-workflow.md` | **A** | onboarder | **Workflow onboarder** — 6 phases (0 prérequis → 1 exploration adaptative 7 profils → 2 questions → 3 rapport contexte → 4 cas particuliers → 5 production wiki + handoff). La logique standalone/sous-agent est extraite dans les skills de parcours dédiés. |
| `planning/onboarder-standalone.md` | **B** | onboarder | **Parcours standalone** — récaps texte avant outil `question`, sans bloc handoff orchestrateur |
| `planning/onboarder-subagent.md` | **B** | onboarder | **Parcours sous-agent** — mécanisme d'interruption session à chaque phase, blocs structurés, `task_id` obligatoire |
| `planning/pathfinder-protocol.md` | **A** | pathfinder | **Protocole pathfinder** — exploration rapide, estimation XS→XL, draft de plan, recommandation direct/escalade. La logique standalone/sous-agent est extraite dans les skills de parcours dédiés. |
| `planning/pathfinder-standalone.md` | **B** | pathfinder | **Parcours standalone** — outil `question` pour les pauses, rapport final sans bloc handoff |
| `planning/pathfinder-subagent.md` | **B** | pathfinder | **Parcours sous-agent** — session unique ou interruption si clarification critique, bloc handoff obligatoire |
| `planning/planner-handoff-format.md` | **A** | planner, orchestrator | **Contrat de handoff** — bloc structuré `## Retour vers orchestrator` : tableau complet des tickets créés avec agent prévu et dépendances, hypothèses et ambiguïtés, estimation globale, risques identifiés, statut (`planification-complète` / `planification-partielle` / `bloqué`) |
| `planning/onboarder-handoff-format.md` | **A** | onboarder, orchestrator | **Contrat de handoff** — bloc structuré `## Retour vers orchestrator` : stack technique détaillée (langages, frameworks, BDD, infra, outils, versions clés), conventions identifiées, dette technique (🔴/🟠/🟡), zones d'incertitude, fichiers de contexte produits (`ONBOARDING.md`, `CONVENTIONS.md`), statut (`contexte-établi` / `contexte-partiel` / `bloqué`) |

---

## Domaine — `designer/`

Skills de design. Utilisés par les agents `ux-designer` et `ui-designer`.

| Fichier | Agents qui l'utilisent | Contenu |
|---------|----------------------|---------|
| `designer/ux-protocol.md` | ux-designer | Heuristiques Nielsen (10 principes), grille des 5 questions UX, format user flow (nominal/alternatifs/erreurs), format spec UX avec critères d'acceptance, protocole d'audit friction |
| `designer/ui-protocol.md` | ui-designer | Tokens de design (couleurs, typographie, espacement, radius, ombres), format spec composant (variants/états/tokens/do-don't), règles de cohérence visuelle, protocole d'audit d'incohérences, échelle modulaire typographique |

---

## Domaine — `design/`

Skills de handoff pour les agents design. Injectés dans l'agent design (producteur) et dans l'`orchestrator` (consommateur).

| Fichier | Agents qui l'utilisent | Contenu |
|---------|----------------------|---------|
| `design/design-handoff-format.md` | ux-designer, ui-designer, orchestrator | **Contrat de handoff** — bloc structuré `## Retour vers orchestrator` : spec produite intégrale (jamais résumée), contraintes d'implémentation, points ouverts, alternatives écartées, statut (`spec-complète` / `spec-partielle` / `bloqué`) — produit uniquement quand invoqué depuis l'orchestrator, après validation explicite de l'utilisateur |

---

## Domaine — `adapters/`

Skills d'intégration avec des outils externes (Figma, GitLab, etc.). Ces skills sont chargés en fonction des `mcpServers` déclarés dans l'agent.

| Fichier | Agents qui l'utilisent | MCP Server | Contenu |
|---------|----------------------|------------|---------|
| `adapters/figma-pathfinder-protocol.md` | pathfinder | `figma` | Protocole d'enrichissement Figma pour Pathfinder — recherche automatique de maquettes par nom de feature, détection de signaux UX/UI (flow multi-étapes, composants, états visuels), ajustement d'estimation (+1 ticket UI / +1 niveau complexité), recommandation d'escalade au planner si complexité L/XL + signaux design forts |
| `adapters/figma-planner-protocol.md` | planner | `figma` | Protocole d'enrichissement Figma pour Planner — Phase 1.3 optionnelle (exploration Figma), recherche de maquettes par nom de feature, analyse de structure + signaux UX/UI, enrichissement récap Phase 1 avec URLs Figma et composants identifiés, déclenchement Phase 1.5 (délégation design) si signaux détectés, pré-remplissage champ `--design` des tickets avec données Figma |
| `adapters/figma-onboarder-protocol.md` | onboarder | `figma` | Protocole d'enrichissement Figma pour Onboarder — Phase 1.5 optionnelle (si frontend détecté), recherche de maquettes par nom de projet, analyse de 3 fichiers max, détection automatique du design system (DSFR, Material, Ant Design, Custom), extraction des design tokens depuis Figma Variables (couleurs, typographie, espacements, effets), intégration dans ONBOARDING.md et CONVENTIONS.md (section Design tokens) |
| `adapters/gitlab-pathfinder-protocol.md` | pathfinder | `gitlab` | Protocole d'enrichissement GitLab pour le Pathfinder — lecture d'un ticket pour affiner l'estimation de complexité (ACs détaillés, labels priorité, milestone, blockers dans les commentaires), détection de MR existantes sur le même périmètre, ajustement selon les contraintes temporelles du milestone |
| `adapters/gitlab-planner-protocol.md` | planner | `gitlab` | Protocole d'enrichissement GitLab pour le Planner — Phase 1.2bis optionnelle, lecture du ticket source comme cahier des charges, extraction des critères d'acceptation, exploitation des labels/milestone pour calibrer la priorité, détection des tickets liés pour identifier les dépendances, enrichissement du récap Phase 1 avec contexte GitLab |
| `adapters/gitlab-onboarder-protocol.md` | onboarder | `gitlab` | Protocole d'intégration GitLab pour l'Onboarder — Phase 1.4bis optionnelle (si projet GitLab détecté), cartographie des labels par catégorie (types, priorités, domaines, workflow), milestones actifs pour comprendre la cadence de livraison, aperçu du backlog, enrichissement de ONBOARDING.md (section Gestion de projet) et CONVENTIONS.md (conventions de labelling) |

---

## Domaine — `posture/`

Skills de posture transverse. Injectables dans tout agent nécessitant une posture d'expert ou une interaction structurée.

| Fichier | Agents qui l'utilisent | Contenu |
|---------|----------------------|---------|
| `posture/expert-posture.md` | auditor-security, auditor-performance, auditor-accessibility, auditor-ecodesign, auditor-architecture, auditor-privacy, auditor-observability, onboarder, ux-designer, ui-designer, planner, documentarian, qa-engineer | Exploration systématique avant de répondre (annonce des artefacts consultés, identification des zones d'incertitude), recommandation contraire argumentée (format ⚠️ avec problème/alternative/pourquoi/trade-offs, formulation à la première personne), pause de confirmation avant toute action à risque élevé (format 🛑 avec question binaire explicite) |
| `posture/tool-question.md` | orchestrator, orchestrator-dev, planner, onboarder, auditor, debugger, reviewer, qa-engineer, documentarian, ux-designer, ui-designer | Utilisation de l'outil `question` d'OpenCode — syntaxe `question({ questions: [{...}] })`, support multi-questions en un seul appel, multi-sélection (`multiple: true`), option "Type your own answer" automatique (ne pas dupliquer), format des réponses (tableau de labels), structure obligatoire (`header` ≤ 30 chars, `question`, `options` avec `label` + `description`), option recommandée en premier avec `(Recommandé)`, bloc de contexte obligatoire en tant que sous-agent |
| `posture/concision-posture.md` | orchestrator, orchestrator-dev, planner, pathfinder, developer, qa-engineer, reviewer | **(A)** — Posture de concision niveau `lite` : suppression des formules d'intro sans valeur ("Bien sûr !", "Je vais...", "Voici..."), reformulations du contexte déjà connu, transitions redondantes entre sections titrées, formules de clôture. Ne touche pas aux blocs handoff, récapitulatifs narratifs obligatoires, rapports formels ni au contenu technique. Calibré via `token_optimization.output_verbosity` dans `hub.json`. Voir [ADR-015](./adr/015-concision-posture.fr.md). |

---

## Domaine — `shared/`

Skills transverses partagés entre plusieurs familles d'agents. Les skills marqués **(A)** sont Bucket A — inline. Les skills marqués **(B)** sont Bucket B — natifs.

| Fichier | Bucket | Agents qui l'utilisent | Contenu |
|---------|--------|----------------------|---------|
| `shared/living-docs-enrichment.md` | **A** | auditor, planner, debugger, onboarder, pathfinder, reviewer, qa-engineer, developer-* (tous les 11) | **Skill partagé** — enrichissement incrémental de ONBOARDING.md et CONVENTIONS.md depuis les travaux de tout agent (audit, planification, debug, implémentation, review, QA, reconnaissance, re-onboarding) ; délègue l'écriture au documentarian après confirmation explicite de l'utilisateur |

---

## Matrice de dépendances agents ↔ skills

> **Note :** Les skills sont répartis en deux buckets (voir [ADR-010](./adr/010-hybrid-skills-architecture.fr.md)) :
> - **(A)** = Bucket A — inline, toujours actif (depuis le champ frontmatter `skills:`)
> - **(B)** = Bucket B — natif, chargé à la demande (depuis le champ frontmatter `native_skills:`, déployé vers `.opencode/skills/`)
>
> Les skills spécifiques aux stacks dans `developer/stacks/` sont toujours Bucket B. L'ensemble déployé dépend de la stack du projet cible. Voir `config/stack-skills.json` pour le mapping complet.
> **Les skills de handoff** sont marqués avec `†` — injectés à la fois dans l'agent producteur et dans l'agent consommateur pour garantir le contrat partagé. Tous les skills de handoff sont Bucket A.

```
orchestrator          → (A) orchestrator/orchestrator-protocol,
                             orchestrator/orchestrator-workflow-modes,
                             orchestrator/orchestrator-handoff-format,
                             developer/beads-plan, posture/tool-question,
                             design/design-handoff-format †,
                             auditor/audit-handoff-format †,
                             planning/planner-handoff-format †,
                             planning/onboarder-handoff-format †,
                             quality/debugger-handoff-format †
                        skill: deny
orchestrator-dev      → (A) orchestrator/orchestrator-dev-protocol,
                             orchestrator/orchestrator-handoff-format,
                             orchestrator/orchestrator-workflow-modes,
                             posture/tool-question,
                             developer/developer-handoff-format †,
                             reviewer/reviewer-handoff-format †,
                             qa/qa-handoff-format †,
                             documentarian/documentarian-handoff-format †
                        skill: deny
onboarder             → (A) planning/onboarder-workflow,
                             posture/expert-posture, posture/tool-question,
                             developer/beads-plan, developer/dev-standards-git,
                             shared/living-docs-enrichment,
                             planning/onboarder-handoff-format †
planner               → (A) developer/beads-plan, planning/planner-workflow,
                             posture/expert-posture, posture/tool-question,
                             shared/living-docs-enrichment,
                             planning/planner-handoff-format †
pathfinder                 → (A) shared/living-docs-enrichment,
                             planning/pathfinder-handoff-format †
                        (B) planning/websearch-stack-research
reviewer              → (A) dev-standards-universal, reviewer/review-protocol,
                             posture/tool-question,
                             shared/living-docs-enrichment,
                             reviewer/reviewer-handoff-format †
                        (B) dev-standards-security, dev-standards-backend,
                             dev-standards-frontend, dev-standards-frontend-a11y,
                             dev-standards-testing, dev-standards-git
qa-engineer           → (A) dev-standards-universal, posture/expert-posture,
                             posture/tool-question, qa/qa-protocol,
                             shared/living-docs-enrichment,
                             qa/qa-handoff-format †
                        (B) dev-standards-git
debugger              → (A) quality/debugger-workflow, posture/tool-question,
                             posture/expert-posture,
                             shared/living-docs-enrichment,
                             quality/debugger-handoff-format †
auditor               → (A) auditor/auditor-workflow, posture/tool-question,
                             shared/living-docs-enrichment,
                             auditor/audit-handoff-format †
                        skill: deny
auditor-security      → (A) auditor/audit-protocol-light, posture/expert-posture,
                             auditor/audit-handoff-format †
                        (B) auditor/audit-security
auditor-performance   → (A) auditor/audit-protocol-light, posture/expert-posture,
                             auditor/audit-handoff-format †
                        (B) auditor/audit-performance
auditor-accessibility → (A) auditor/audit-protocol-light, posture/expert-posture,
                             auditor/audit-handoff-format †
                        (B) auditor/audit-accessibility
auditor-ecodesign     → (A) auditor/audit-protocol-light, posture/expert-posture,
                             auditor/audit-handoff-format †
                        (B) auditor/audit-ecodesign
auditor-architecture  → (A) auditor/audit-protocol-light, posture/expert-posture,
                             auditor/audit-handoff-format †
                        (B) auditor/audit-architecture
auditor-privacy       → (A) auditor/audit-protocol-light, posture/expert-posture,
                             auditor/audit-handoff-format †
                        (B) auditor/audit-privacy
auditor-observability → (A) auditor/audit-protocol-light, posture/expert-posture,
                             auditor/audit-handoff-format †
                        (B) auditor/audit-observability
ux-designer           → (A) designer/ux-protocol, developer/beads-plan, developer/beads-dev,
                             posture/expert-posture, posture/tool-question,
                             design/design-handoff-format †
ui-designer           → (A) designer/ui-protocol, developer/beads-plan, developer/beads-dev,
                             posture/expert-posture, posture/tool-question,
                             design/design-handoff-format †
documentarian         → (A) dev-standards-git, beads-plan, beads-dev,
                             documentarian/doc-protocol, posture/expert-posture,
                             posture/tool-question,
                             documentarian/documentarian-handoff-format †
                        (B) documentarian/doc-standards, documentarian/doc-adr,
                             documentarian/doc-api, documentarian/doc-changelog,
                             documentarian/doc-slides
developer-frontend    → (A) dev-standards-universal, dev-standards-simplicity,
                             beads-plan, beads-dev,
                             shared/living-docs-enrichment,
                             developer/developer-handoff-format †
                        (B) dev-standards-security, dev-standards-frontend,
                             dev-standards-frontend-a11y, dev-standards-testing,
                             dev-standards-git
                             + [stacks: language, frontend, test, api-spec]
developer-backend     → (A) dev-standards-universal, dev-standards-simplicity,
                             beads-plan, beads-dev,
                             shared/living-docs-enrichment,
                             developer/developer-handoff-format †
                        (B) dev-standards-security, dev-standards-backend,
                             dev-standards-testing, dev-standards-git
                             + [stacks: language, backend, orm, test, api-spec]
developer-fullstack   → (A) dev-standards-universal, dev-standards-simplicity,
                             beads-plan, beads-dev,
                             shared/living-docs-enrichment,
                             developer/developer-handoff-format †
                        (B) dev-standards-security, dev-standards-frontend,
                             dev-standards-frontend-a11y, dev-standards-backend,
                             dev-standards-testing, dev-standards-git
                             + [stacks: language, frontend, backend, orm, test, api-spec]
developer-data        → (A) dev-standards-universal, dev-standards-simplicity,
                             beads-plan, beads-dev,
                             shared/living-docs-enrichment,
                             developer/developer-handoff-format †
                        (B) dev-standards-security, dev-standards-testing, dev-standards-git
                             + [stacks: language, data, test]
developer-devops      → (A) dev-standards-universal, dev-standards-simplicity,
                             beads-plan, beads-dev,
                             shared/living-docs-enrichment,
                             developer/developer-handoff-format †
                        (B) dev-standards-security, dev-standards-devops, dev-standards-git
                             + [stacks: infra]
developer-mobile      → (A) dev-standards-universal, dev-standards-simplicity,
                             beads-plan, beads-dev,
                             shared/living-docs-enrichment,
                             developer/developer-handoff-format †
                        (B) dev-standards-security, dev-standards-testing, dev-standards-git
                             + [stacks: mobile, test]
developer-api         → (A) dev-standards-universal, dev-standards-simplicity,
                             beads-plan, beads-dev,
                             shared/living-docs-enrichment,
                             developer/developer-handoff-format †
                        (B) dev-standards-security, dev-standards-backend, dev-standards-api,
                             dev-standards-testing, dev-standards-git
developer-platform    → (A) dev-standards-universal, dev-standards-simplicity,
                             beads-plan, beads-dev,
                             shared/living-docs-enrichment,
                             developer/developer-handoff-format †
                        (B) dev-standards-security, dev-standards-devops, dev-standards-git
                             + [stacks: infra]
developer-security    → (A) dev-standards-universal, dev-standards-simplicity,
                             beads-plan, beads-dev,
                             shared/living-docs-enrichment,
                             developer/developer-handoff-format †
                        (B) dev-standards-security, dev-standards-security-hardening,
                             dev-standards-backend, dev-standards-testing, dev-standards-git
developer-migrator    → (A) dev-standards-universal, dev-standards-simplicity,
                             beads-plan, beads-dev,
                             shared/living-docs-enrichment,
                             developer/developer-handoff-format †
                        (B) dev-standards-security, dev-standards-testing,
                             dev-standards-git, dev-standards-migration
developer-refactor    → (A) dev-standards-universal, dev-standards-simplicity,
                             beads-plan, beads-dev,
                             shared/living-docs-enrichment,
                             developer/developer-handoff-format †
                        (B) dev-standards-security, dev-standards-testing,
                             dev-standards-git, dev-standards-refactoring
```
