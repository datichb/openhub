> 🇫🇷 [Lire en français](skills.fr.md)

# Skills Reference

Skills are Markdown blocks injected into agents at deploy time.
They contain detailed protocols, output formats, checklists, and rules
that agents apply.

---

## Skill format

```markdown
---
name: <skill-name>
description: <Short description — visible in oc agent edit and oc skills list>
---

# Skill — <Title>

<Skill body>
```

> The `name` key is documentary. Hub scripts only read `description`.
> The file path is the reference used in agent frontmatter.

---

## Domain — `developer/`

Development standards skills. Shared between developer agents and the reviewer.

### Generic skills (always loaded)

| File | Agents using it | Content |
|------|----------------|---------|
| `developer/beads-plan.md` | All developer-*, planner, onboarder, designers, documentarian | Reading and creating Beads tickets: `bd list`, `bd show`, `bd create`, `bd label list-all`, external links |
| `developer/beads-dev.md` | All developer-*, designers, documentarian | Beads executor workflow: `bd update --claim`, `bd close --suggest-next`, `ai-delegated` rules |
| `developer/dev-standards-universal.md` | All developer-*, reviewer | Clean Code, full SOLID, naming, structure — **language-agnostic** |
| `developer/dev-standards-security.md` | All developer-*, reviewer | Secrets/config, input validation, injections (SQL/shell/LDAP), auth/authorization, logs without sensitive data, dependency auditing — **tool-agnostic** |
| `developer/dev-standards-backend.md` | developer-backend, developer-fullstack, developer-api, reviewer | Layered architecture, DTOs, services, repositories, API security |
| `developer/dev-standards-frontend.md` | developer-frontend, developer-fullstack, reviewer | Logic/presentation separation, performance, bundle, lazy loading |
| `developer/dev-standards-frontend-a11y.md` | developer-frontend, developer-fullstack, reviewer | WCAG 2.1 A/AA, semantic HTML, ARIA, contrast |
| `developer/dev-standards-testing.md` | developer-frontend, developer-backend, developer-fullstack, developer-api, developer-data, qa-engineer | Testing strategy, pyramid, coverage, TDD — **tool-agnostic** |
| `developer/dev-standards-git.md` | All developer-*, reviewer | Conventional Commits, branches, PRs, commit messages |
| `developer/dev-standards-devops.md` | developer-devops | Shell scripts, secrets management, image registries, observability, IaC principles — **tool-agnostic** |
| `developer/dev-standards-api.md` | developer-api | API versioning, pagination, uniform response format, HTTP codes, idempotency, schema-first contract, breaking changes, webhooks, rate limiting |
| `developer/dev-standards-security-hardening.md` | developer-security | CORS, HTTP headers (CSP, HSTS, X-Frame-Options), bcrypt/argon2id, JWT (rotation, revocation), sessions (httpOnly/secure/sameSite), rate limiting, AES-256-GCM encryption |
| `developer/dev-standards-simplicity.md` | All developer-* | KISS, YAGNI, no premature abstraction, no premature optimisation, measurable complexity thresholds (function length, cyclomatic complexity, parameters, nesting depth, injected dependencies), over-engineering patterns to challenge |
| `developer/developer-handoff-format.md` | All developer-*, orchestrator-dev | **Handoff contract** — structured `## Return to orchestrator-dev` block: files modified, tests written, Beads status `review`, acceptance criteria checked one by one, points of attention for the reviewer, blockers encountered, status (`implemented` / `partially-implemented` / `blocked`) |

### Stack-specific skills — `developer/stacks/`

These skills are injected **dynamically at deploy time** when the corresponding stack is detected in the target project (`detect_stack()` in `prompt-builder.sh`). They are **additive** — they complement the generic skills above.

The mapping between detected stacks and skills to inject is declared in `config/stack-skills.json`. Each agent type (`developer-frontend`, `developer-backend`, etc.) has a defined scope that limits which categories of stack skills it receives.

#### Languages

| File | Stack detected | Content |
|------|---------------|---------|
| `developer/stacks/dev-standards-typescript.md` | `typescript` in dependencies | Strict config, interfaces vs types, enums, shared types, typed errors, type guards, generics |
| `developer/stacks/dev-standards-python.md` | `pyproject.toml` / `requirements.txt` present | Version, ruff, mypy/pyright, naming, custom exceptions, logging, pytest |

#### Frontend frameworks

| File | Stack detected | Content |
|------|---------------|---------|
| `developer/stacks/dev-standards-vuejs.md` | `vue` in deps | Composition API, `<script setup>`, Pinia, composables, Vue Router |
| `developer/stacks/dev-standards-react.md` | `react` in deps | Hooks, TanStack Query, memo/useCallback, RTL, conventions |
| `developer/stacks/dev-standards-nextjs.md` | `next` in deps | App Router, Server/Client Components, ISR, Server Actions, metadata |
| `developer/stacks/dev-standards-nuxtjs.md` | `nuxt` in deps | Auto-imports, useFetch, Nitro server routes, Pinia setup, routeRules |
| `developer/stacks/dev-standards-angular.md` | `@angular/core` in deps | Standalone components, Signals, inject(), RxJS, Reactive Forms, lazy routing |

#### Backend frameworks

| File | Stack detected | Content |
|------|---------------|---------|
| `developer/stacks/dev-standards-nestjs.md` | `@nestjs/core` in deps | Modules, DTOs + class-validator, guards, ConfigService + Joi, unit tests |
| `developer/stacks/dev-standards-express.md` | `express` or `fastify` in deps | Domain routing, zod middleware, AppError, helmet/cors, global error handler |
| `developer/stacks/dev-standards-django.md` | `django` in Python deps | BaseModel UUID, FormRequest, I/O serializers, services, migrations |
| `developer/stacks/dev-standards-fastapi.md` | `fastapi` in Python deps | pydantic-settings, Pydantic v2 schemas, inject(), async services, httpx tests |
| `developer/stacks/dev-standards-laravel.md` | `laravel` in Gemfile/composer | Eloquent, FormRequest, API Resources, service objects, queues/jobs |
| `developer/stacks/dev-standards-rails.md` | `rails` in Gemfile | MVC, service objects, query objects, scopes, RSpec request specs |
| `developer/stacks/dev-standards-springboot.md` | `spring-boot` in build.gradle/pom.xml | JPA entities, record DTOs + @Valid, @Transactional, ProblemDetail, MockMvc |

#### ORMs / Databases

| File | Stack detected | Content |
|------|---------------|---------|
| `developer/stacks/dev-standards-prisma.md` | `@prisma/client` in deps | Schema, singleton client, explicit select, transactions, migrate deploy |
| `developer/stacks/dev-standards-typeorm.md` | `typeorm` in deps | Entities select:false, custom repository, parameterised QueryBuilder, QueryRunner |
| `developer/stacks/dev-standards-sqlalchemy.md` | `sqlalchemy` in Python deps | Mapped v2, async sessions, Alembic, transaction context manager |
| `developer/stacks/dev-standards-mongodb.md` | `mongoose` in deps | Mongoose schemas, lean(), indexes, documented aggregations, transactions |

#### API spec

| File | Stack detected | Content |
|------|---------------|---------|
| `developer/stacks/dev-standards-openapi.md` | `openapi.yaml` / `swagger.yaml` present | `$ref`, reusable schemas/responses/params, writeOnly, JWT security, codegen |

#### Testing tools

| File | Stack detected | Content |
|------|---------------|---------|
| `developer/stacks/dev-standards-vitest.md` | `vitest` in deps | vi.mock, vi.fn, vi.spyOn, vi.useFakeTimers, Vue Test Utils |
| `developer/stacks/dev-standards-jest.md` | `jest` in deps | jest.mock, jest.fn, jest.spyOn, RTL behaviour testing, snapshots |
| `developer/stacks/dev-standards-playwright.md` | `@playwright/test` in deps | Semantic locators (getByRole), semantic waits, POM, session fixtures |
| `developer/stacks/dev-standards-cypress.md` | `cypress` in deps | data-cy, cy.intercept + alias, custom commands, cy.session |

#### Mobile

| File | Stack detected | Content |
|------|---------------|---------|
| `developer/stacks/dev-standards-react-native.md` | `react-native` in deps | Expo, React Navigation, Zustand/RTK, TanStack Query, Detox |
| `developer/stacks/dev-standards-flutter.md` | `flutter` in pubspec.yaml | BLoC/Riverpod, Clean Arch, freezed, flutter_test, mockito |
| `developer/stacks/dev-standards-swift.md` | Xcode project detected | SwiftUI, MVVM, Swift Concurrency, Keychain, XCTest async |
| `developer/stacks/dev-standards-kotlin.md` | `jetpack compose` in build.gradle | Jetpack Compose, MVVM+Clean, Hilt, Coroutines+Flow, JUnit5+Mockk+Turbine |

#### Data / ML

| File | Stack detected | Content |
|------|---------------|---------|
| `developer/stacks/dev-standards-pandas.md` | `pandas` in Python deps | Vectorisation, pandera, pipeline .pipe(), DataFrame tests |
| `developer/stacks/dev-standards-dbt.md` | `dbt-*` in Python deps | Layers staging/intermediate/mart, schema.yml, native + custom tests |
| `developer/stacks/dev-standards-airflow.md` | `apache-airflow` in Python deps | TaskFlow API, idempotence, Connections/Variables, DAG structure tests |
| `developer/stacks/dev-standards-pyspark.md` | `pyspark` in Python deps | DataFrame API, broadcast join, partitioning, ML lifecycle, MLflow, local tests |

#### DevOps / CI-CD

| File | Stack detected | Content |
|------|---------------|---------|
| `developer/stacks/dev-standards-docker.md` | `Dockerfile` present | Multi-stage, non-root, .dockerignore, Compose healthchecks, BuildKit secrets |
| `developer/stacks/dev-standards-github-actions.md` | `.github/workflows/` present | Minimal permissions, concurrency, SHA pinning, OIDC, environments with approval |
| `developer/stacks/dev-standards-gitlab-ci.md` | `.gitlab-ci.yml` present | rules (not only/except), YAML templates, masked variables, when:manual in prod |

#### Platform / Infrastructure

| File | Stack detected | Content |
|------|---------------|---------|
| `developer/stacks/dev-standards-terraform.md` | `*.tf` files present | Modules, variables + validation, remote state, lifecycle (plan → PR → apply via pipeline) |
| `developer/stacks/dev-standards-kubernetes.md` | K8s manifests present | Deployment, RBAC, NetworkPolicy, ResourceQuota, PDB, Kustomize |
| `developer/stacks/dev-standards-helm.md` | `Chart.yaml` present | Chart structure, values without secrets, ExternalSecret in templates, helm diff + --atomic |
| `developer/stacks/dev-standards-argocd.md` | ArgoCD manifests present | GitOps principles, sync policies by env (auto staging / manual prod), ESO, Vault |

---

## Domain — `auditor/`

Audit skills. All auditor-* agents inject `audit-protocol` + their domain skill.

| File | Agents using it | Content |
|------|----------------|---------|
| `auditor/audit-protocol.md` | auditor, all auditor-* | Common report format, 4 criticality levels (🔴/🟠/🟡/💡), /10 scoring, individual finding format |
| `auditor/audit-security.md` | auditor-security | OWASP Top 10, injections, exposed secrets, auth, CORS, CVE |
| `auditor/audit-performance.md` | auditor-performance | Core Web Vitals, LCP, CLS, TTI, N+1 queries, cache, bundle |
| `auditor/audit-accessibility.md` | auditor-accessibility | WCAG 2.1 AA, RGAA 4.1, semantics, ARIA, keyboard navigation, contrast |
| `auditor/audit-ecodesign.md` | auditor-ecodesign | RGESN, GreenIT, Écoindex, data transfer, resources, obsolescence |
| `auditor/audit-architecture.md` | auditor-architecture | SOLID, Clean Architecture, technical debt, coupling, cohesion |
| `auditor/audit-privacy.md` | auditor-privacy | GDPR articles 5/6/17/25/32, EDPB, CNIL, minimisation, consent |
| `auditor/audit-observability.md` | auditor-observability | RED method (Rate/Errors/Duration), structured logs, OpenTelemetry, SLOs/error budget, alerting (actionable, runbooks), dashboards, 5-question grid |
| `auditor/audit-handoff-format.md` | all auditor-*, orchestrator | **Handoff contract** — structured `## Return to orchestrator` block: audited scope, vulnerability table by severity, prioritized recommendations with effort estimate, residual risk, status (`corrections-required` / `acceptable` / `blocking`) |

---

## Domain — `orchestrator/`

| File | Agents using it | Content |
|------|----------------|---------|
| `orchestrator/orchestrator-protocol.md` | orchestrator | Full feature workflow, routing matrix (3 families: design, auditor, dev via orchestrator-dev), checkpoint format ([CP-0], [CP-spec], [CP-audit], [CP-feature]), edge case handling, structured return validation for each sub-agent type |
| `orchestrator/orchestrator-dev-protocol.md` | orchestrator-dev | Beads ticket-by-ticket workflow, developer-* routing matrix (9 signals → 9 agents), checkpoint format ([CP-1] to [CP-3] + [CP-QA]), 3 modes (manual/semi-auto/auto), `tdd` label detection (CP-QA automatically skipped — tests written by the developer in red/green/refactor), structured return exploitation (developer points-of-attention → reviewer, QA non-covered criteria → reviewer, reviewer corrections verbatim → Beads comment), **mandatory two-step global recap**: (1) full narrative recap (text + ticket table: agent, QA, review cycles, criteria covered, status) before the structured block, (2) `## Return to orchestrator` structured block — both required in this order |
| `orchestrator/orchestrator-handoff-format.md` | orchestrator-dev, orchestrator | **Handoff contract** — two formats: `## Return to orchestrator` (end of session: producer must emit full narrative recap first, then structured block with per-ticket detail table: agent, QA, review cycles, criteria covered, status — plus attention points and global status `success`/`partial`/`blocked`; consumer must display the full narrative recap in its discussion thread before building [CP-feature]) and `## Question for the orchestrator` (high-stakes CPs: CP-2, 3-cycle blockage, unresolved dependency, blocked ticket — full context, waiting question, options, `task_id` for session resumption) |
| `orchestrator/orchestrator-workflow-modes.md` | orchestrator, orchestrator-dev | Single source of truth for the 3 workflow modes (manual/semi-auto/auto) — canonical question blocks, absolute rules per mode |

---

## Domain — `qa/`

| File | Agents using it | Content |
|------|----------------|---------|
| `qa/qa-protocol.md` | qa-engineer | Test types (unit/integration/E2E/component), tools by stack, systematic checklist (nominal/error/edge cases/acceptance), coverage report format, AAA structure |
| `qa/qa-handoff-format.md` | qa-engineer, orchestrator-dev | **Handoff contract** — structured `## Return to orchestrator-dev` block: tests written with files and covered cases, acceptance criteria checked, non-testable zones, status (`complete-coverage` / `partial-coverage` / `non-testable`) |

---

## Domain — `debugger/`

| File | Agents using it | Content |
|------|----------------|---------|
| `debugger/debug-protocol.md` | debugger | 4-step methodology, reading stacktraces and logs, diagnostic report format with graduated hypotheses, Beads ticket creation protocol |

---

## Domain — `quality/`

Quality skills for agents that are not qa-engineer or reviewer.

| File | Agents using it | Content |
|------|----------------|---------|
| `quality/debugger-handoff-format.md` | debugger, orchestrator | **Handoff contract** — structured `## Return to orchestrator` block: root cause with certainty level (confirmed/probable/uncertain) + causal chain, explored hypotheses, impact and potential regressions, correction tickets created, emergency actions if bug in production, status (`diagnosed` / `partially-diagnosed` / `non-reproducible`) |

---

## Domain — `reviewer/`

| File | Agents using it | Content |
|------|----------------|---------|
| `reviewer/review-protocol.md` | reviewer | Review report format (Critical/Major/Minor/Suggestion/Positive points/Out of scope), 4 severity levels, systematic 6-category checklist, individual comment format, "full audit" mode |
| `reviewer/reviewer-handoff-format.md` | reviewer, orchestrator-dev | **Handoff contract** — structured `## Return to orchestrator-dev` block: actionable verdict (`commit` / `fix` / `fix-security`), problem summary by severity, required corrections verbatim (pasted directly into Beads comment), recommended routing (`return-initial` / `developer-security`), status (`approved` / `corrections-required` / `blocking-security`) |

---

## Domain — `documentarian/`

Documentation skills. Used by the `documentarian` agent.

| File | Agents using it | Content |
|------|----------------|---------|
| `documentarian/doc-protocol.md` | documentarian | Mandatory exploration before writing, 4-situation adaptation table (compliant format / improvable / absent / partial), routing by doc type, gap checklist, Beads and direct workflow |
| `documentarian/doc-standards.md` | documentarian | Diataxis framework (4 quadrants), readability principles, type-specific structures (README, how-to, reference), common anti-patterns, quality criteria, functional documentation |
| `documentarian/doc-adr.md` | documentarian | Existing format detection (Nygard / MADR / Y-Statements / house), reference MADR format, naming rules, statuses (proposed/accepted/deprecated/superseded), creation criteria |
| `documentarian/doc-api.md` | documentarian | OpenAPI 3.x (skeleton, endpoint, reusable schemas), HTTP codes, narrative documentation (usage guide, pagination, error handling), breaking change identification and documentation |
| `documentarian/doc-changelog.md` | documentarian | Keep a Changelog (6 sections), SemVer (MAJOR/MINOR/PATCH), Conventional Commits → changelog sections, generation from git log, release workflow, extended release notes format |
| `documentarian/doc-slides.md` | documentarian | Marp presentation generation (Markdown → HTML/PDF) — 4 templates (tech-demo, product-pitch, retrospective, onboarding), Marp directives (frontmatter, `---`, `_class`, `backgroundColor`), best practices (1 idea/slide, max 5 bullets, actionable titles), automatic Marp CLI detection post-generation with compilation proposal, fallback with installation options if absent |
| `documentarian/documentarian-handoff-format.md` | documentarian, orchestrator-dev | **Handoff contract** — structured `## Return to orchestrator-dev` block: type of documentation produced, modified files, entry summary, status (`documented` / `partially-documented` / `blocked`) |

---

## Domain — `planning/`

| File | Agents using it | Content |
|------|----------------|---------|
| `planning/planner.md` | planner | Phase 0 (codebase exploration + existing tickets + context summary), Phase 1 (contextualised questions + justified priority deduction), Phase 2 (hierarchical plan epics → tickets, >5 tickets rule), Phase 3 (creation with `--parent`, `--deps`, `--estimate`), Phase 4 (`bd children` verification), edge case handling (scope change, split, late dependency, duplicate) |
| `planning/project-discovery.md` | onboarder | Stack detection (manifests, CI, infra), adaptive exploration by profile (Vue, React, Node.js, Python, API, Data/ML, DevOps, Mobile), context report format (stack, architecture, patterns, 🔴/🟠/🟡, blind spots, questions, agent map), agent recommendation matrix (priority by risk + recommended by stack + optional), `projects.md` update protocol |
| `planning/project-conventions.md` | onboarder | Project-specific naming conventions and contribution standards, rules detected from the codebase (branches, commits, PRs, tickets) |
| `planning/planner-handoff-format.md` | planner, orchestrator | **Handoff contract** — structured `## Return to orchestrator` block: complete tickets table with planned agent and dependencies, hypotheses and ambiguities, global estimate, identified risks, status (`complete-planning` / `partial-planning` / `blocked`) |
| `planning/onboarder-handoff-format.md` | onboarder, orchestrator | **Handoff contract** — structured `## Return to orchestrator` block: detected tech stack (languages, frameworks, DB, infra, tools, key versions), identified conventions, technical debt (🔴/🟠/🟡), uncertainty zones, context files produced (`ONBOARDING.md`, `CONVENTIONS.md`), status (`context-established` / `partial-context` / `blocked`) |

---

## Domain — `designer/`

Design skills. Used by the `ux-designer` and `ui-designer` agents.

| File | Agents using it | Content |
|------|----------------|---------|
| `designer/ux-protocol.md` | ux-designer | Nielsen heuristics (10 principles), 5-question UX grid, user flow format (nominal/alternatives/errors), UX spec format with acceptance criteria, friction audit protocol |
| `designer/ui-protocol.md` | ui-designer | Design tokens (colours, typography, spacing, radius, shadows), component spec format (variants/states/tokens/do-don't), visual consistency rules, inconsistency audit protocol, typographic modular scale |

---

## Domain — `design/`

Handoff skills for design agents. Injected in the design agent (producer) and in `orchestrator` (consumer).

| File | Agents using it | Content |
|------|----------------|---------|
| `design/design-handoff-format.md` | ux-designer, ui-designer, orchestrator | **Handoff contract** — structured `## Return to orchestrator` block: complete spec (never summarized), implementation constraints, open points, rejected alternatives, status (`complete-spec` / `partial-spec` / `blocked`) — produced only when invoked from the orchestrator, after explicit user validation |

---

## Domain — `posture/`

Cross-cutting posture skills. Injectable into any agent requiring an expert posture or structured interaction.

| File | Agents using it | Content |
|------|----------------|---------|
| `posture/expert-posture.md` | auditor-security, auditor-performance, auditor-accessibility, auditor-ecodesign, auditor-architecture, auditor-privacy, auditor-observability, onboarder, ux-designer, ui-designer, planner, documentarian, qa-engineer | Systematic exploration before responding (announcing artefacts consulted, identifying uncertainty areas), argued counter-recommendation (⚠️ format with problem/alternative/why/trade-offs, first-person phrasing), confirmation pause before any high-risk action (🛑 format with explicit binary question) |
| `posture/tool-question.md` | orchestrator, orchestrator-dev, planner, onboarder, auditor, debugger, reviewer, qa-engineer, documentarian, ux-designer, ui-designer | Usage of OpenCode's `question` tool — when to call it (blocking multi-choice decisions, risky confirmations, ambiguous instructions), when not to call it, mandatory structure for every call (`header` ≤ 30 chars, `question`, `options` with `label` + `description`), rules: `multiple: true` for multi-select, recommended option first with `(Recommended)`, never add an "Other" option |

---

## Agent ↔ skills dependency matrix

> **Note:** Stack-specific skills from `developer/stacks/` are injected **dynamically** at deploy time based on the target project's stack. Only the statically declared skills are listed below. See `config/stack-skills.json` for the complete mapping.
> **Handoff skills** are marked with `†` — injected in both the producing agent and the consuming agent to guarantee the shared contract.

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
                         qa/qa-handoff-format †,
                         documentarian/documentarian-handoff-format †
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
                         documentarian/doc-changelog, documentarian/doc-slides,
                         posture/expert-posture, posture/tool-question,
                         documentarian/documentarian-handoff-format †
developer-frontend    → dev-standards-universal, dev-standards-security,
                         dev-standards-frontend,
                         dev-standards-frontend-a11y, stacks/dev-standards-vuejs,
                         dev-standards-testing, dev-standards-git,
                         beads-plan, beads-dev,
                         developer/dev-standards-simplicity,
                         developer/developer-handoff-format †
                         + [stacks: language, frontend, test, api-spec] (dynamic)
developer-backend     → dev-standards-universal, dev-standards-security,
                         dev-standards-backend,
                         dev-standards-testing, dev-standards-git,
                         beads-plan, beads-dev,
                         developer/dev-standards-simplicity,
                         developer/developer-handoff-format †
                         + [stacks: language, backend, orm, test, api-spec] (dynamic)
developer-fullstack   → dev-standards-universal, dev-standards-security,
                         dev-standards-frontend,
                         dev-standards-frontend-a11y, stacks/dev-standards-vuejs,
                         dev-standards-backend, dev-standards-testing,
                         dev-standards-git, beads-plan, beads-dev,
                         developer/dev-standards-simplicity,
                         developer/developer-handoff-format †
                         + [stacks: language, frontend, backend, orm, test, api-spec] (dynamic)
developer-data        → dev-standards-universal, dev-standards-security,
                         stacks/dev-standards-python,
                         stacks/dev-standards-pandas, stacks/dev-standards-dbt,
                         stacks/dev-standards-airflow, stacks/dev-standards-pyspark,
                         dev-standards-testing, dev-standards-git, beads-plan, beads-dev,
                         developer/dev-standards-simplicity,
                         developer/developer-handoff-format †
                         + [stacks: language, data, test] (dynamic)
developer-devops      → dev-standards-universal, dev-standards-security,
                         dev-standards-devops,
                         stacks/dev-standards-docker, stacks/dev-standards-github-actions,
                         stacks/dev-standards-gitlab-ci,
                         dev-standards-git, beads-plan, beads-dev,
                         developer/dev-standards-simplicity,
                         developer/developer-handoff-format †
                         + [stacks: infra] (dynamic)
developer-mobile      → dev-standards-universal, dev-standards-security,
                         stacks/dev-standards-react-native, stacks/dev-standards-flutter,
                         stacks/dev-standards-swift, stacks/dev-standards-kotlin,
                         dev-standards-testing, dev-standards-git, beads-plan, beads-dev,
                         developer/dev-standards-simplicity,
                         developer/developer-handoff-format †
                         + [stacks: mobile, test] (dynamic)
developer-api         → dev-standards-universal, dev-standards-security,
                         dev-standards-backend, dev-standards-api,
                         dev-standards-testing, dev-standards-git,
                         beads-plan, beads-dev,
                         developer/dev-standards-simplicity,
                         developer/developer-handoff-format †
developer-platform    → dev-standards-universal, dev-standards-security,
                         dev-standards-devops,
                         stacks/dev-standards-terraform, stacks/dev-standards-kubernetes,
                         stacks/dev-standards-helm, stacks/dev-standards-argocd,
                         dev-standards-git, beads-plan, beads-dev,
                         developer/dev-standards-simplicity,
                         developer/developer-handoff-format †
                         + [stacks: infra] (dynamic)
developer-security    → dev-standards-universal, dev-standards-security,
                         dev-standards-security-hardening,
                         dev-standards-backend,
                         dev-standards-testing, dev-standards-git,
                         beads-plan, beads-dev,
                         developer/dev-standards-simplicity,
                         developer/developer-handoff-format †
```
