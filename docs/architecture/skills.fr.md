# Référence des skills

Les skills sont des blocs Markdown injectés dans les agents au moment du déploiement.
Ils contiennent les protocoles détaillés, formats de sortie, checklists et règles
que les agents appliquent.

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

### Skills génériques (toujours chargés)

| Fichier | Agents qui l'utilisent | Contenu |
|---------|----------------------|---------|
| `developer/beads-plan.md` | Tous les developer-*, planner, onboarder, designers, documentarian | Lecture et création de tickets Beads : `bd list`, `bd show`, `bd create`, `bd label list-all`, liens externes |
| `developer/beads-dev.md` | Tous les developer-*, designers, documentarian | Workflow exécuteur Beads : `bd update --claim`, `bd close --suggest-next`, règles `ai-delegated` |
| `developer/dev-standards-universal.md` | Tous les developer-*, reviewer | Clean Code, SOLID complet, nommage, structure — **agnostique du langage** |
| `developer/dev-standards-security.md` | Tous les developer-*, reviewer | Secrets/config, validation des inputs, injections (SQL/shell/LDAP), auth/autorisation, logs sans données sensibles, audit des dépendances — **agnostique des outils** |
| `developer/dev-standards-backend.md` | developer-backend, developer-fullstack, developer-api, reviewer | Architecture en couches, DTOs, services, repositories, sécurité API |
| `developer/dev-standards-frontend.md` | developer-frontend, developer-fullstack, reviewer | Séparation logique/présentation, performance, bundle, lazy loading |
| `developer/dev-standards-frontend-a11y.md` | developer-frontend, developer-fullstack, reviewer | WCAG 2.1 A/AA, sémantique HTML, ARIA, contrastes |
| `developer/dev-standards-testing.md` | developer-frontend, developer-backend, developer-fullstack, developer-api, developer-data, qa-engineer | Stratégie de tests, pyramide, coverage, TDD — **agnostique des outils** |
| `developer/dev-standards-git.md` | Tous les developer-*, reviewer | Conventional Commits, branches, PR, messages de commit |
| `developer/dev-standards-devops.md` | developer-devops | Scripts shell, gestion des secrets, registries d'images, observabilité, principes IaC — **agnostique des outils** |
| `developer/dev-standards-api.md` | developer-api | Versioning d'API, pagination, format de réponse uniforme, codes HTTP, idempotence, contrat schema-first, breaking changes, webhooks, rate limiting |
| `developer/dev-standards-security-hardening.md` | developer-security | CORS, headers HTTP (CSP, HSTS, X-Frame-Options), bcrypt/argon2id, JWT (rotation, révocation), sessions (httpOnly/secure/sameSite), rate limiting, chiffrement AES-256-GCM |
| `developer/developer-handoff-format.md` | Tous les developer-*, orchestrator-dev | **Contrat de handoff** — bloc structuré `## Retour vers orchestrator-dev` : fichiers modifiés, tests écrits, statut Beads `review`, critères d'acceptance cochés un par un, points d'attention pour la review, blocages rencontrés, statut (`implémenté` / `partiellement-implémenté` / `bloqué`) |

### Skills spécifiques aux stacks — `developer/stacks/`

Ces skills sont injectés **dynamiquement au déploiement** quand la stack correspondante est détectée dans le projet cible (`detect_stack()` dans `prompt-builder.sh`). Ils sont **additifs** — ils complètent les skills génériques ci-dessus.

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

Skills d'audit. Tous les agents auditor-* injectent `audit-protocol` + leur skill de domaine.

| Fichier | Agents qui l'utilisent | Contenu |
|---------|----------------------|---------|
| `auditor/audit-protocol.md` | auditor, tous les auditor-* | Format de rapport commun, 4 niveaux de criticité (🔴/🟠/🟡/💡), scoring /10, format des findings individuels |
| `auditor/audit-security.md` | auditor-security | OWASP Top 10, injections, secrets exposés, auth, CORS, CVE |
| `auditor/audit-performance.md` | auditor-performance | Core Web Vitals, LCP, CLS, TTI, requêtes N+1, cache, bundle |
| `auditor/audit-accessibility.md` | auditor-accessibility | WCAG 2.1 AA, RGAA 4.1, sémantique, ARIA, navigation clavier, contrastes |
| `auditor/audit-ecodesign.md` | auditor-ecodesign | RGESN, GreenIT, Écoindex, transfert de données, ressources, obsolescence |
| `auditor/audit-architecture.md` | auditor-architecture | SOLID, Clean Architecture, dette technique, couplage, cohésion |
| `auditor/audit-privacy.md` | auditor-privacy | RGPD articles 5/6/17/25/32, EDPB, CNIL, minimisation, consentement |
| `auditor/audit-observability.md` | auditor-observability | Méthode RED (Rate/Errors/Duration), logs structurés, OpenTelemetry, SLOs/error budget, alerting (actionnable, runbooks), dashboards, grille des 5 questions |
| `auditor/audit-handoff-format.md` | tous les auditor-*, orchestrator | **Contrat de handoff** — bloc structuré `## Retour vers orchestrator` : périmètre audité, tableau des vulnérabilités par sévérité, recommandations priorisées avec estimation d'effort, risque résiduel, statut (`corrections-requises` / `acceptable` / `bloquant`) |

---

## Domaine — `orchestrator/`

| Fichier | Agents qui l'utilisent | Contenu |
|---------|----------------------|---------|
| `orchestrator/orchestrator-protocol.md` | orchestrator | Workflow feature complet, matrice de routing (3 familles : design, auditor, dev via orchestrator-dev), format des checkpoints ([CP-0], [CP-spec], [CP-audit], [CP-feature]), gestion des cas particuliers, validation des retours structurés pour chaque type de sous-agent |
| `orchestrator/orchestrator-dev-protocol.md` | orchestrator-dev | Workflow Beads ticket par ticket, matrice de routing developer-* (9 signaux → 9 agents), format des checkpoints ([CP-1] à [CP-3] + [CP-QA]), 3 modes (manuel/semi-auto/auto), détection du label `tdd`, exploitation des retours structurés (points d'attention developer → reviewer, critères QA non couverts → reviewer, corrections reviewer verbatim → commentaire Beads), format du compte rendu d'étape et du récap global |
| `orchestrator/orchestrator-handoff-format.md` | orchestrator-dev, orchestrator | **Contrat de handoff** — deux formats : `## Retour vers orchestrator` (fin de session : tickets traités, points d'attention, statut global `succès`/`partiel`/`bloqué`) et `## Question pour l'orchestrator` (CPs à enjeu fort : CP-2, blocage 3 cycles, dépendance non résolue, ticket bloqué — contexte complet, question en attente, options, `task_id` pour reprise de session) |
| `orchestrator/orchestrator-workflow-modes.md` | orchestrator, orchestrator-dev | Source de vérité unique pour les 3 modes de workflow (manuel/semi-auto/auto) — blocs question canoniques, règles absolues par mode |

---

## Domaine — `qa/`

| Fichier | Agents qui l'utilisent | Contenu |
|---------|----------------------|---------|
| `qa/qa-protocol.md` | qa-engineer | Typologie des tests (unit/integration/E2E/composants), outils par stack, checklist systématique (nominal/erreur/edge cases/acceptance), format du rapport de couverture, structure type AAA |
| `qa/qa-handoff-format.md` | qa-engineer, orchestrator-dev | **Contrat de handoff** — bloc structuré `## Retour vers orchestrator-dev` : tests écrits avec fichiers et cas couverts, critères d'acceptance cochés, zones non testables, statut (`couverture-complète` / `couverture-partielle` / `non-testable`) |

---

## Domaine — `debugger/`

| Fichier | Agents qui l'utilisent | Contenu |
|---------|----------------------|---------|
| `debugger/debug-protocol.md` | debugger | Méthodologie en 4 étapes, lecture de stacktraces et logs, format du rapport de diagnostic avec hypothèses graduées, protocole de création de ticket Beads |

---

## Domaine — `quality/`

Skills de qualité pour les agents qui ne sont pas qa-engineer ni reviewer.

| Fichier | Agents qui l'utilisent | Contenu |
|---------|----------------------|---------|
| `quality/debugger-handoff-format.md` | debugger, orchestrator | **Contrat de handoff** — bloc structuré `## Retour vers orchestrator` : cause racine avec niveau de certitude (confirmé/probable/incertain) + chaîne causale, hypothèses explorées, impact et régressions potentielles, tickets de correction créés, actions d'urgence si bug en prod, statut (`diagnostiqué` / `partiellement-diagnostiqué` / `non-reproductible`) |

---

## Domaine — `reviewer/`

| Fichier | Agents qui l'utilisent | Contenu |
|---------|----------------------|---------|
| `reviewer/review-protocol.md` | reviewer | Format du rapport de review (Critique/Majeur/Mineur/Suggestion/Points positifs/Hors scope), 4 niveaux de sévérité, checklist systématique en 6 catégories, format des commentaires individuels, mode "audit complet" |
| `reviewer/reviewer-handoff-format.md` | reviewer, orchestrator-dev | **Contrat de handoff** — bloc structuré `## Retour vers orchestrator-dev` : verdict actionnable (`commit` / `corriger` / `corriger-sécurité`), synthèse des problèmes par sévérité, corrections requises verbatim (collées directement dans le commentaire Beads), routing recommandé (`retour-initial` / `developer-security`), statut (`approuvé` / `corrections-requises` / `bloquant-sécurité`) |

---

## Domaine — `documentarian/`

Skills de documentation. Utilisés par l'agent `documentarian`.

| Fichier | Agents qui l'utilisent | Contenu |
|---------|----------------------|---------|
| `documentarian/doc-protocol.md` | documentarian | Exploration obligatoire avant rédaction, tableau d'adaptation en 4 situations (format conforme / améliorable / absent / partiel), routing par type de doc, checklist de lacunes, workflow Beads et direct |
| `documentarian/doc-standards.md` | documentarian | Framework Diataxis (4 quadrants), principes de lisibilité, structures type par document (README, how-to, référence), anti-patterns courants, critères de qualité, documentation fonctionnelle |
| `documentarian/doc-adr.md` | documentarian | Détection du format existant (Nygard / MADR / Y-Statements / maison), format MADR de référence, règles de nommage, statuts (proposed/accepted/deprecated/superseded), critères de création |
| `documentarian/doc-api.md` | documentarian | OpenAPI 3.x (squelette, endpoint, schemas réutilisables), codes HTTP, documentation narrative (guide d'utilisation, pagination, gestion des erreurs), identification et documentation des breaking changes |
| `documentarian/doc-changelog.md` | documentarian | Keep a Changelog (6 sections), SemVer (MAJOR/MINOR/PATCH), Conventional Commits → sections changelog, génération depuis git log, workflow de release, release notes format étendu |

---

## Domaine — `planning/`

| Fichier | Agents qui l'utilisent | Contenu |
|---------|----------------------|---------|
| `planning/planner.md` | planner | Phase 0 (exploration codebase + tickets existants + résumé de contexte), Phase 1 (questions contextualisées + déduction des priorités justifiées), Phase 2 (plan hiérarchique epics → tickets, règle >5 tickets), Phase 3 (création avec `--parent`, `--deps`, `--estimate`), Phase 4 (vérification `bd children`), gestion des aléas (scope change, scission, dépendance tardive, doublon) |
| `planning/project-discovery.md` | onboarder | Détection de stack (manifestes, CI, infra), exploration adaptative par profil (Vue, React, Node.js, Python, API, Data/ML, DevOps, Mobile), format du rapport de contexte (stack, architecture, patterns, 🔴/🟠/🟡, zones d'ombre, questions, carte agents), matrice de recommandation agents (prioritaires par risque + recommandés par stack + optionnels), protocole de mise à jour `projects.md` |
| `planning/project-conventions.md` | onboarder | Conventions de nommage et standards de contribution propres au projet, règles détectées depuis la codebase (branches, commits, PR, tickets) |
| `planning/planner-handoff-format.md` | planner, orchestrator | **Contrat de handoff** — bloc structuré `## Retour vers orchestrator` : tableau complet des tickets créés avec agent prévu et dépendances, hypothèses et ambiguïtés, estimation globale, risques identifiés, statut (`planification-complète` / `planification-partielle` / `bloqué`) |
| `planning/onboarder-handoff-format.md` | onboarder, orchestrator | **Contrat de handoff** — bloc structuré `## Retour vers orchestrator` : stack technique détaillée (langages, frameworks, BDD, infra, outils, versions clés), conventions identifiées, dette technique (🔴/🟠/🟡), zones d'incertitude, fichiers de contexte produits (`ONBOARDING.md`, `CONVENTIONS.md`), statut (`contexte-établi` / `contexte-partiel` / `bloqué`) |

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

## Domaine — `posture/`

Skills de posture transverse. Injectables dans tout agent nécessitant une posture d'expert ou une interaction structurée.

| Fichier | Agents qui l'utilisent | Contenu |
|---------|----------------------|---------|
| `posture/expert-posture.md` | auditor-security, auditor-performance, auditor-accessibility, auditor-ecodesign, auditor-architecture, auditor-privacy, auditor-observability, onboarder, ux-designer, ui-designer, planner, documentarian, qa-engineer | Exploration systématique avant de répondre (annonce des artefacts consultés, identification des zones d'incertitude), recommandation contraire argumentée (format ⚠️ avec problème/alternative/pourquoi/trade-offs, formulation à la première personne), pause de confirmation avant toute action à risque élevé (format 🛑 avec question binaire explicite) |
| `posture/tool-question.md` | orchestrator, orchestrator-dev, planner, onboarder, auditor, debugger, reviewer, qa-engineer, documentarian, ux-designer, ui-designer | Utilisation de l'outil `question` d'OpenCode — quand l'appeler (décisions bloquantes multi-choix, confirmations risquées, instructions ambiguës), quand ne pas l'appeler, structure obligatoire de chaque appel (`header` ≤ 30 chars, `question`, `options` avec `label` + `description`), règles : `multiple: true` pour multi-sélection, option recommandée en premier avec `(Recommandé)`, ne pas ajouter d'option "Autre" |

---

## Matrice de dépendances agents ↔ skills

> **Note :** Les skills spécifiques aux stacks dans `developer/stacks/` sont injectés **dynamiquement** au déploiement selon la stack du projet cible. Seuls les skills déclarés statiquement sont listés ci-dessous. Voir `config/stack-skills.json` pour le mapping complet.
> **Les skills de handoff** sont marqués avec `†` — injectés à la fois dans l'agent producteur et dans l'agent consommateur pour garantir le contrat partagé.

```
orchestrator          → orchestrator/orchestrator-protocol,
                         orchestrator/orchestrator-workflow-modes,
                         orchestrator/orchestrator-handoff-format,
                         developer/beads-plan, posture/tool-question,
                         design/design-handoff-format †,
                         auditor/audit-handoff-format †,
                         planning/planner-handoff-format †,
                         planning/onboarder-handoff-format †,
                         quality/debugger-handoff-format †
orchestrator-dev      → orchestrator/orchestrator-dev-protocol,
                         orchestrator/orchestrator-handoff-format,
                         orchestrator/orchestrator-workflow-modes,
                         posture/tool-question,
                         developer/developer-handoff-format †,
                         reviewer/reviewer-handoff-format †,
                         qa/qa-handoff-format †
onboarder             → planning/project-discovery, planning/project-conventions,
                         posture/expert-posture, posture/tool-question,
                         developer/beads-plan, developer/dev-standards-git,
                         planning/onboarder-handoff-format †
planner               → developer/beads-plan, planning/planner,
                         posture/expert-posture, posture/tool-question,
                         planning/planner-handoff-format †
reviewer              → dev-standards-universal, dev-standards-security,
                         dev-standards-backend,
                         dev-standards-frontend, dev-standards-frontend-a11y,
                         dev-standards-testing,
                         dev-standards-git, reviewer/review-protocol,
                         posture/tool-question,
                         reviewer/reviewer-handoff-format †
qa-engineer           → dev-standards-universal, dev-standards-testing,
                         dev-standards-git, posture/expert-posture,
                         posture/tool-question, qa/qa-protocol,
                         qa/qa-handoff-format †
debugger              → debugger/debug-protocol, posture/tool-question,
                         quality/debugger-handoff-format †
auditor               → auditor/audit-protocol, posture/tool-question
auditor-security      → auditor/audit-protocol-light, auditor/audit-security,
                         posture/expert-posture,
                         auditor/audit-handoff-format †
auditor-performance   → auditor/audit-protocol-light, auditor/audit-performance,
                         posture/expert-posture,
                         auditor/audit-handoff-format †
auditor-accessibility → auditor/audit-protocol-light, auditor/audit-accessibility,
                         posture/expert-posture,
                         auditor/audit-handoff-format †
auditor-ecodesign     → auditor/audit-protocol-light, auditor/audit-ecodesign,
                         posture/expert-posture,
                         auditor/audit-handoff-format †
auditor-architecture  → auditor/audit-protocol-light, auditor/audit-architecture,
                         posture/expert-posture,
                         auditor/audit-handoff-format †
auditor-privacy       → auditor/audit-protocol-light, auditor/audit-privacy,
                         posture/expert-posture,
                         auditor/audit-handoff-format †
auditor-observability → auditor/audit-protocol-light, auditor/audit-observability,
                         posture/expert-posture,
                         auditor/audit-handoff-format †
ux-designer           → designer/ux-protocol, developer/beads-plan, developer/beads-dev,
                         posture/expert-posture, posture/tool-question,
                         design/design-handoff-format †
ui-designer           → designer/ui-protocol, developer/beads-plan, developer/beads-dev,
                         posture/expert-posture, posture/tool-question,
                         design/design-handoff-format †
documentarian         → dev-standards-git, beads-plan, beads-dev,
                         documentarian/doc-protocol, documentarian/doc-standards,
                         documentarian/doc-adr, documentarian/doc-api,
                         documentarian/doc-changelog, posture/expert-posture,
                         posture/tool-question
developer-frontend    → dev-standards-universal, dev-standards-security,
                         dev-standards-frontend,
                         dev-standards-frontend-a11y, stacks/dev-standards-vuejs,
                         dev-standards-testing, dev-standards-git,
                         beads-plan, beads-dev,
                         developer/developer-handoff-format †
                         + [stacks: language, frontend, test, api-spec] (dynamique)
developer-backend     → dev-standards-universal, dev-standards-security,
                         dev-standards-backend,
                         dev-standards-testing, dev-standards-git,
                         beads-plan, beads-dev,
                         developer/developer-handoff-format †
                         + [stacks: language, backend, orm, test, api-spec] (dynamique)
developer-fullstack   → dev-standards-universal, dev-standards-security,
                         dev-standards-frontend,
                         dev-standards-frontend-a11y, stacks/dev-standards-vuejs,
                         dev-standards-backend, dev-standards-testing,
                         dev-standards-git, beads-plan, beads-dev,
                         developer/developer-handoff-format †
                         + [stacks: language, frontend, backend, orm, test, api-spec] (dynamique)
developer-data        → dev-standards-universal, dev-standards-security,
                         stacks/dev-standards-python,
                         stacks/dev-standards-pandas, stacks/dev-standards-dbt,
                         stacks/dev-standards-airflow, stacks/dev-standards-pyspark,
                         dev-standards-testing, dev-standards-git, beads-plan, beads-dev,
                         developer/developer-handoff-format †
                         + [stacks: language, data, test] (dynamique)
developer-devops      → dev-standards-universal, dev-standards-security,
                         dev-standards-devops,
                         stacks/dev-standards-docker, stacks/dev-standards-github-actions,
                         stacks/dev-standards-gitlab-ci,
                         dev-standards-git, beads-plan, beads-dev,
                         developer/developer-handoff-format †
                         + [stacks: infra] (dynamique)
developer-mobile      → dev-standards-universal, dev-standards-security,
                         stacks/dev-standards-react-native, stacks/dev-standards-flutter,
                         stacks/dev-standards-swift, stacks/dev-standards-kotlin,
                         dev-standards-testing, dev-standards-git, beads-plan, beads-dev,
                         developer/developer-handoff-format †
                         + [stacks: mobile, test] (dynamique)
developer-api         → dev-standards-universal, dev-standards-security,
                         dev-standards-backend, dev-standards-api,
                         dev-standards-testing, dev-standards-git,
                         beads-plan, beads-dev,
                         developer/developer-handoff-format †
developer-platform    → dev-standards-universal, dev-standards-security,
                         dev-standards-devops,
                         stacks/dev-standards-terraform, stacks/dev-standards-kubernetes,
                         stacks/dev-standards-helm, stacks/dev-standards-argocd,
                         dev-standards-git, beads-plan, beads-dev,
                         developer/developer-handoff-format †
                         + [stacks: infra] (dynamique)
developer-security    → dev-standards-universal, dev-standards-security,
                         dev-standards-security-hardening,
                         dev-standards-backend,
                         dev-standards-testing, dev-standards-git,
                         beads-plan, beads-dev,
                         developer/developer-handoff-format †
```
