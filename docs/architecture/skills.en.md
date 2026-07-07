> 🇫🇷 [Lire en français](skills.fr.md)

# Skills Reference

Skills contain detailed protocols, output formats, checklists, and rules that agents apply.
The hub uses a **hybrid architecture** with two deployment paths — see [ADR-010](./adr/010-hybrid-skills-architecture.en.md).

## Deployment paths

| Path | Frontmatter field | Deployed to | When loaded |
|------|------------------|-------------|-------------|
| **Inline (Bucket A)** | `skills: [...]` | Assembled into the agent system prompt at deploy time | Always — from the first token |
| **Native (Bucket B)** | `native_skills: [...]` | `.opencode/skills/<name>/SKILL.md` | On-demand — the LLM loads them via the `skill` tool when the task requires it |

**Bucket A** — Workflow protocols, handoff formats, universal principles, posture skills, core execution skills (`beads-plan`, `beads-dev`, `quick-fix`). Must be active from the first token.

**Bucket B** — Domain standards, stack-specific skills, audit checklists, doc type skills, contextual research skills. Loaded only when the agent's task requires that specific domain context.

Agents that use native skills have `permission: skill: allow` in their frontmatter.
Coordinator/orchestrator agents that never need contextual skills have `permission: skill: deny`.

---

## Skill format

```markdown
---
name: <skill-name>
description: <Short description — used during deployment and in documentation>
---

# Skill — <Title>

<Skill body>
```

> The `name` key is documentary. Hub scripts only read `description`.
> The file path is the reference used in agent frontmatter.

---

## Domain — `developer/`

Development standards skills. Shared between developer agents and the reviewer.

### Generic skills

Skills marked **(A)** are Bucket A — always inline. Skills marked **(B)** are Bucket B — native, loaded on-demand.

| File | Bucket | Agents using it | Content |
|------|--------|----------------|---------|
| `developer/beads-plan.md` | **A** | All developer-*, planner, onboarder, designers, documentarian | Reading and creating Beads tickets: `bd list`, `bd show`, `bd create`, `bd label list-all`, external links |
| `developer/beads-dev.md` | **A** | All developer-*, designers, documentarian | Beads executor workflow: `bd update --claim`, `bd close --suggest-next`, `ai-delegated` rules |
| `developer/dev-standards-universal.md` | **A** | All developer-*, reviewer | Clean Code, full SOLID, naming, structure — **language-agnostic** |
| `developer/dev-standards-security.md` | **B** | All developer-*, reviewer | Secrets/config, input validation, injections (SQL/shell/LDAP), auth/authorization, logs without sensitive data, dependency auditing — **tool-agnostic** |
| `developer/dev-standards-backend.md` | **B** | developer-backend, developer-fullstack, developer-api, reviewer | Layered architecture, DTOs, services, repositories, API security |
| `developer/dev-standards-frontend.md` | **B** | developer-frontend, developer-fullstack, reviewer | Logic/presentation separation, performance, bundle, lazy loading |
| `developer/dev-standards-frontend-data.md` | **B** | developer-frontend, developer-fullstack, reviewer | Frontend data management — 5 characterization questions, decision matrix (local state, Context Provider, Store, Queries, Cookies, WebStorage, IndexedDB, Query String), detailed cards with trade-offs, golden rule "trim your data" |
| `developer/dev-standards-frontend-a11y.md` | **B** | developer-frontend, developer-fullstack, reviewer | WCAG 2.1 A/AA, semantic HTML, ARIA, contrast |
| `developer/dev-standards-testing.md` | **B** | developer-frontend, developer-backend, developer-fullstack, developer-api, developer-data | Testing strategy, pyramid, coverage, TDD — **tool-agnostic** |
| `developer/dev-standards-git.md` | **B** | All developer-*, reviewer | Conventional Commits, branches, PRs, commit messages |
| `developer/dev-standards-devops.md` | **B** | developer-devops | Shell scripts, secrets management, image registries, observability, IaC principles — **tool-agnostic** |
| `developer/dev-standards-api.md` | **B** | developer-api | API versioning, pagination, uniform response format, HTTP codes, idempotency, schema-first contract, breaking changes, webhooks, rate limiting |
| `developer/dev-standards-security-hardening.md` | **B** | developer-security | CORS, HTTP headers (CSP, HSTS, X-Frame-Options), bcrypt/argon2id, JWT (rotation, revocation), sessions (httpOnly/secure/sameSite), rate limiting, AES-256-GCM encryption |
| `developer/dev-standards-simplicity.md` | **A** | All developer-* | KISS, YAGNI, no premature abstraction, no premature optimisation, measurable complexity thresholds (function length, cyclomatic complexity, parameters, nesting depth, injected dependencies), over-engineering patterns to challenge |
| `developer/developer-handoff-format.md` | **A** | All developer-*, orchestrator-dev | **Handoff contract** — structured `## Return to orchestrator-dev` block: files modified, tests written, Beads status `review`, acceptance criteria checked one by one, points of attention for the reviewer, blockers encountered, status (`implemented` / `partially-implemented` / `blocked`) |

### Stack-specific skills — `developer/stacks/` (Bucket B)

These skills are **Bucket B — native**. At deploy time, `deploy_native_skills()` deploys them to `.opencode/skills/` based on the project stack detected by `detect_stack()`. The LLM loads the relevant ones on-demand at inference time.

The mapping between detected stacks and skills is declared in `config/stack-skills.json`. Each agent type has a defined scope limiting which categories of stack skills it receives.

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

Audit skills. Skills marked **(A)** are Bucket A — inline. Skills marked **(B)** are Bucket B — native.

| File | Bucket | Agents using it | Content |
|------|--------|----------------|---------|
| `auditor/auditor-workflow.md` | **A** | auditor | **Unified coordinator workflow** — 5 phases (0 prerequisites check → 1 project context loading → 2 domain selection with stack compatibility → 3 subagent delegation → 4 consolidation executive summary) — systematic recaps, mandatory questions via `question`, backwards possible; orchestrator invocation marker detection for `## Return to orchestrator` block |
| `auditor/audit-protocol-light.md` | **A** | auditor-subagent | Common lightweight report format (subagents only): 4 criticality levels (🔴/🟠/🟡/💡), /10 scoring, individual finding format |
| `auditor/audit-security.md` | **B** | auditor-subagent | OWASP Top 10, injections, exposed secrets, auth, CORS, CVE |
| `auditor/audit-performance.md` | **B** | auditor-subagent | Core Web Vitals, LCP, CLS, TTI, N+1 queries, cache, bundle |
| `auditor/audit-accessibility.md` | **B** | auditor-subagent | WCAG 2.1 AA, RGAA 4.1, semantics, ARIA, keyboard navigation, contrast |
| `auditor/audit-ecodesign.md` | **B** | auditor-subagent | RGESN, GreenIT, Écoindex, data transfer, resources, obsolescence |
| `auditor/audit-architecture.md` | **B** | auditor-subagent | SOLID, Clean Architecture, technical debt, coupling, cohesion |
| `auditor/audit-privacy.md` | **B** | auditor-subagent | GDPR articles 5/6/17/25/32, EDPB, CNIL, minimisation, consent |
| `auditor/audit-observability.md` | **B** | auditor-subagent | RED method (Rate/Errors/Duration), structured logs, OpenTelemetry, SLOs/error budget, alerting (actionable, runbooks), dashboards, 5-question grid |
| `auditor/audit-handoff-format.md` | **A** | auditor-subagent, orchestrator | **Handoff contract** — structured `## Return to orchestrator` block: audited scope, vulnerability table by severity, prioritized recommendations with effort estimate, residual risk, status (`corrections-required` / `acceptable` / `blocking`) |
| `auditor/websearch-cve-lookup.md` | **B** | auditor-subagent | CVE/NVD lookup protocol via websearch |
| `auditor/websearch-performance-research.md` | **B** | auditor-subagent | Performance benchmark and regression research protocol via websearch |

---

## Domain — `orchestrator/`

| File | Agents using it | Content |
|------|----------------|---------|
| `orchestrator/orchestrator-protocol.md` | orchestrator | Full feature workflow, routing matrix (3 families: design, auditor, dev via orchestrator-dev), checkpoint format ([CP-0], [CP-spec], [CP-audit], [CP-feature]), edge case handling, structured return validation for each sub-agent type |
| `orchestrator/orchestrator-dev-protocol.md` | orchestrator-dev | Beads ticket-by-ticket workflow, developer-* routing matrix (9 signals → 9 agents), checkpoint format ([CP-1] to [CP-3]), 3 modes (manual/semi-auto/auto), `tdd` label detection (tests written by the developer in red/green/refactor), structured return exploitation (developer points-of-attention → reviewer, reviewer corrections verbatim → Beads comment), **mandatory two-step global recap**: (1) full narrative recap (text + ticket table: agent, review cycles, criteria covered, status) before the structured block, (2) `## Return to orchestrator` structured block — both required in this order |
| `orchestrator/orchestrator-handoff-format.md` | orchestrator-dev, orchestrator | **Handoff contract** — two formats: `## Return to orchestrator` (end of session: producer must emit condensed per-ticket summary first (status, key files, covered criteria, attention points + aggregated global attention points), then structured block with per-ticket detail table: agent, review cycles, criteria covered, status — plus attention points and global status `success`/`partial`/`blocked`; consumer must display this summary in its discussion thread before building [CP-feature]) and `## Question for the orchestrator` (high-stakes CPs: CP-2, 3-cycle blockage, unresolved dependency, blocked ticket — full context, waiting question, options, `task_id` for session resumption) |
| `orchestrator/orchestrator-workflow-modes.md` | orchestrator, orchestrator-dev | Single source of truth for the 3 workflow modes (manual/semi-auto/auto) — canonical question blocks, absolute rules per mode |

---

## Domain — `quality/`

Quality skills for agents that are not reviewer.

| File | Agents using it | Content |
|------|----------------|---------|
| `quality/debugger-workflow.md` | debugger | **Unified workflow** — 6 phases (0 artefact check → 1 contextual exploration → 2 complementary questions optional → 3 4-step diagnosis: reproduction/isolation/identification/graded hypothesis → 4 edge case detection: race condition, environment, data, configuration, dependencies, regression → 5 report + Beads ticket) — systematic recaps, graded hypotheses (high/medium/low probability), `## Return to orchestrator` block if invoked from orchestrator |
| `quality/debugger-handoff-format.md` | debugger, orchestrator | **Handoff contract** — structured `## Return to orchestrator` block: root cause with certainty level (confirmed/probable/uncertain) + causal chain, explored hypotheses, impact and potential regressions, correction tickets created, emergency actions if bug in production, status (`diagnosed` / `partially-diagnosed` / `non-reproducible`) |
| `quality/debugger-subagent.md` | debugger (conditional) | **Sub-agent execution path** — loaded by the debugger when `[SKILL:quality/debugger-subagent]` is detected in the prompt (orchestrator injection). Session interruption mechanism at each phase, `## Retour intermédiaire vers orchestrator` + `## Question pour l'orchestrator` blocks with `task_id`, `question` tool forbidden. Mirrors the `planner-subagent` pattern. |

---

## Domain — `reviewer/`

| File | Agents using it | Content |
|------|----------------|---------|
| `reviewer/review-protocol.md` | reviewer | Review report format (Critical/Major/Minor/Suggestion/Positive points/Out of scope), 4 severity levels, systematic 6-category checklist, individual comment format, "full audit" mode, raw output format specification for multi-mode fusion |
| `reviewer/reviewer-standalone.md` | **(B)** reviewer | **Standalone execution path** — interactive mode selection prompt (standard / adversarial / edge-case / combinations), parallel session orchestration for combined modes, result fusion via `review-merge`, living-docs enrichment, no orchestrator handoff block |
| `reviewer/reviewer-subagent.md` | **(B)** reviewer | **Sub-agent execution path** — supports standard mode (per-ticket from orchestrator-dev), adversarial mode (CP-feature from orchestrator), and combined adversarial+edge-case mode. Always produces full report + mandatory `## Return to orchestrator-dev` handoff block |
| `reviewer/reviewer-adversarial.md` | **(B)** reviewer | **Adversarial review mode** — maximum skepticism posture, min. 10 findings mandatory, 7 investigation categories (Architecture, Robustness, Performance, Security, Maintainability, Tests, Contracts), "dangerous assumptions" section, confidence score. Supports feature-scope (`git diff main..feature-branch`) for CP-feature. Context-isolated in multi-mode |
| `reviewer/reviewer-edge-case.md` | **(B)** reviewer | **Edge-case hunting mode** — exhaustive unhandled execution path analysis (control flow, value boundaries, implicit coercions, concurrency/timing, external dependencies, input security). Available everywhere as option. Context-isolated in multi-mode |
| `reviewer/review-merge.md` | **(B)** reviewer | **Report fusion skill** — receives N raw reports from parallel sessions, deduplicates findings (same file:line + same root cause → keep highest severity), tags provenance `[STD]`/`[ADV]`/`[EDGE]`, produces unified report with mode-specific annexes (dangerous assumptions, architecture issues, confidence score, missing paths by class) |
| `reviewer/reviewer-handoff-format.md` | reviewer, orchestrator-dev | **Handoff contract** — structured `## Return to orchestrator-dev` block: actionable verdict (`commit` / `fix` / `fix-security`), problem summary by severity, required corrections verbatim (pasted directly into Beads comment), recommended routing (`return-initial` / `developer-security`), status (`approved` / `corrections-required` / `blocking-security`) |

---

## Domain — `documentarian/`

Documentation skills. Skills marked **(A)** are Bucket A — inline. Skills marked **(B)** are Bucket B — native.

| File | Bucket | Agents using it | Content |
|------|--------|----------------|---------|
| `documentarian/doc-protocol.md` | **A** | documentarian | Mandatory exploration before writing, 4-situation adaptation table (compliant format / improvable / absent / partial), routing by doc type, gap checklist, Beads and direct workflow |
| `documentarian/doc-standards.md` | **B** | documentarian | Diataxis framework (4 quadrants), readability principles, type-specific structures (README, how-to, reference), common anti-patterns, quality criteria, functional documentation |
| `documentarian/doc-adr.md` | **B** | documentarian | Existing format detection (Nygard / MADR / Y-Statements / house), reference MADR format, naming rules, statuses (proposed/accepted/deprecated/superseded), creation criteria |
| `documentarian/doc-api.md` | **B** | documentarian | OpenAPI 3.x (skeleton, endpoint, reusable schemas), HTTP codes, narrative documentation (usage guide, pagination, error handling), breaking change identification and documentation |
| `documentarian/doc-changelog.md` | **B** | documentarian | Keep a Changelog (6 sections), SemVer (MAJOR/MINOR/PATCH), Conventional Commits → changelog sections, generation from git log, release workflow, extended release notes format |
| `documentarian/doc-slides.md` | **B** | documentarian | Marp presentation generation (Markdown → HTML/PDF) — 4 templates (tech-demo, product-pitch, retrospective, onboarding), Marp directives (frontmatter, `---`, `_class`, `backgroundColor`), best practices (1 idea/slide, max 5 bullets, actionable titles), automatic Marp CLI detection post-generation with compilation proposal, fallback with installation options if absent |
| `documentarian/documentarian-handoff-format.md` | **A** | documentarian, orchestrator-dev | **Handoff contract** — structured `## Return to orchestrator-dev` block: type of documentation produced, modified files, entry summary, status (`documented` / `partially-documented` / `blocked`) |

---

## Domain — `planning/`

Skills marked **(A)** are Bucket A — inline. Skills marked **(B)** are Bucket B — native.

| File | Bucket | Agents using it | Content |
|------|--------|----------------|---------|
| `planning/planner-workflow.md` | **A** | planner | **Unified planner workflow** — 7 phases (0 prerequisites → 1 contextual exploration + UX/UI signals → 1.5 optional design delegation → 2 complementary questions → 3 hierarchical plan: epics/tickets/priorities → 4 edge case detection: duplicates, oversized tickets, circular dependencies → 5 Beads creation with full enrichment → 5.5 optional ai-delegated delegation → 6 verification + handoff) — systematic recaps, iterative phases with backwards possible (max 3), `## Return to orchestrator` block if invoked |
| `planning/onboarder-workflow.md` | **A** | onboarder | **Unified onboarder workflow** — 6 phases (0 prerequisites → 1 adaptive exploration 7 profiles → 2 questions → 3 context report: stack/architecture/patterns/attention points/prioritized agent map → 4 edge cases: inconsistencies, CVE, hidden debt, hybrid architecture → 5 ONBOARDING.md + CONVENTIONS.md + optional projects.md + handoff) — merges previous `project-discovery.md` and `project-conventions.md` |
| `planning/planner-handoff-format.md` | **A** | planner, orchestrator | **Handoff contract** — structured `## Return to orchestrator` block: complete tickets table with planned agent and dependencies, hypotheses and ambiguities, global estimate, identified risks, status (`complete-planning` / `partial-planning` / `blocked`) |
| `planning/onboarder-handoff-format.md` | **A** | onboarder, orchestrator | **Handoff contract** — structured `## Return to orchestrator` block: detected tech stack (languages, frameworks, DB, infra, tools, key versions), identified conventions, technical debt (🔴/🟠/🟡), uncertainty zones, context files produced (`ONBOARDING.md`, `CONVENTIONS.md`), status (`context-established` / `partial-context` / `blocked`) |
| `planning/websearch-stack-research.md` | **B** | planner, pathfinder, onboarder | Stack and library research protocol via websearch — how to find current best practices, changelogs, and library comparisons |
| `planning/pathfinder-handoff-format.md` | **A** | pathfinder, orchestrator | **Handoff contract** — structured `## Return to orchestrator` block |

---

## Domain — `designer/`

Design skills. Used by the `designer` agent.

| File | Bucket | Agents using it | Content |
|------|--------|----------------|---------|
| `designer/designer-protocol.md` | **A** | designer | **Unified designer protocol** — 4 invocation modes (recon/ux/ui/ux+ui), mode detection from prompt, routing logic, Figma access gate (only in recon mode), session interruption mechanism |
| `designer/ux-protocol.md` | **B** | designer | Nielsen heuristics (10 principles), 5-question UX grid, user flow format (nominal/alternatives/errors), UX spec format with acceptance criteria, friction audit protocol |
| `designer/ui-protocol.md` | **B** | designer | Design tokens (colours, typography, spacing, radius, shadows), component spec format (variants/states/tokens/do-don't), visual consistency rules, inconsistency audit protocol, typographic modular scale |
| `designer/figma-recon-protocol.md` | **B** | designer | Figma reconnaissance protocol — file search by feature name, component tree exploration, design system detection (DSFR, Material, Custom), token extraction (Figma Variables: colours, typography, spacing), output format for planning agents |
| `designer/figma-deep-protocol.md` | **B** | designer | Figma deep analysis — component variant enumeration, state mapping, responsive breakpoints, design-to-dev annotation extraction, handoff checklist |
| `designer/designer-execution-modes.md` | **B** | designer | **Execution paths** — standalone path (question tool active, no handoff block) and subagent path (session interruption mechanism, `## Question pour l'orchestrator` + `task_id` blocks) |

---

## Domain — `design/`

Handoff skills for the designer agent. Injected in `designer` (producer) and in `orchestrator` (consumer).

| File | Agents using it | Content |
|------|----------------|---------|
| `design/design-handoff-format.md` | designer, orchestrator | **Handoff contract** — structured `## Return to orchestrator` block: complete spec (never summarized), implementation constraints, open points, rejected alternatives, status (`complete-spec` / `partial-spec` / `blocked`) — produced only when invoked from the orchestrator, after explicit user validation |

---

## Domain — `posture/`

Cross-cutting posture skills. Injectable into any agent requiring an expert posture or structured interaction.

| File | Agents using it | Content |
|------|----------------|---------|
| `posture/expert-posture.md` | auditor-subagent, onboarder, designer, planner, documentarian | Systematic exploration before responding (announcing artefacts consulted, identifying uncertainty areas), argued counter-recommendation (⚠️ format with problem/alternative/why/trade-offs, first-person phrasing), confirmation pause before any high-risk action (🛑 format with explicit binary question) |
| `posture/tool-question.md` | orchestrator, orchestrator-dev, planner, onboarder, auditor, debugger, reviewer, documentarian, designer | Usage of OpenCode's `question` tool — `question({ questions: [{...}] })` syntax, multi-questions support in a single call, multi-selection (`multiple: true`), automatic "Type your own answer" option (don't duplicate), response format (array of labels), mandatory structure (`header` ≤ 30 chars, `question`, `options` with `label` + `description`), recommended option first with `(Recommended)`, mandatory context block when invoked as sub-agent |
| `posture/concision-posture.md` | orchestrator, orchestrator-dev, planner, pathfinder, developer, reviewer | **(A)** — Concision posture level `lite`: drops valueless intro phrases ("Sure!", "I'm going to...", "Here is..."), known-context restatements, redundant transitions between titled sections, closing formulas. Does not touch handoff blocks, mandatory narrative recaps, formal reports, or technical content. Tunable via `token_optimization.output_verbosity` in `hub.json`. See [ADR-015](./adr/015-concision-posture.en.md). |

---

## Domain — `shared/`

Cross-cutting skills shared across multiple agent families. Skills marked **(A)** are Bucket A — inline. Skills marked **(B)** are Bucket B — native.

| File | Bucket | Agents using it | Content |
|------|--------|----------------|---------|
| `shared/living-docs-enrichment.md` | **A** | auditor, planner, debugger, onboarder, pathfinder, reviewer, developer-* (all 11) | **Shared skill** — incremental enrichment of ONBOARDING.md and CONVENTIONS.md from any agent's work (audit, planning, debug, implementation, review, reconnaissance, re-onboarding); delegates writing to documentarian after explicit user confirmation |

---

## Agent ↔ skills dependency matrix

> **Note:** Skills are split into two buckets (see [ADR-010](./adr/010-hybrid-skills-architecture.en.md)):
> - **(A)** = Bucket A — inline, always active (from `skills:` frontmatter field)
> - **(B)** = Bucket B — native, loaded on-demand (from `native_skills:` frontmatter field, deployed to `.opencode/skills/`)
>
> Stack-specific skills from `developer/stacks/` are always Bucket B. The set deployed depends on the target project's stack. See `config/stack-skills.json` for the complete mapping.
> **Handoff skills** are marked with `†` — injected in both the producing agent and the consuming agent to guarantee the shared contract. All handoff skills are Bucket A.

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
                             documentarian/documentarian-handoff-format †
                        skill: deny
onboarder             → (A) planning/onboarder-workflow,
                             posture/expert-posture, posture/tool-question,
                             developer/beads-plan, developer/dev-standards-git,
                             shared/living-docs-enrichment,
                             planning/onboarder-handoff-format †
                        (B) planning/websearch-stack-research
planner               → (A) developer/beads-plan, planning/planner-workflow,
                             posture/expert-posture, posture/tool-question,
                             shared/living-docs-enrichment,
                             planning/planner-handoff-format †
                        (B) planning/websearch-stack-research
pathfinder                 → (A) shared/living-docs-enrichment,
                             planning/pathfinder-handoff-format †
                        (B) planning/websearch-stack-research
reviewer              → (A) dev-standards-universal, reviewer/review-protocol,
                             posture/tool-question,
                             shared/living-docs-enrichment,
                             reviewer/reviewer-handoff-format †
                        (B) reviewer/reviewer-standalone, reviewer/reviewer-subagent,
                             reviewer/reviewer-adversarial, reviewer/reviewer-edge-case,
                             reviewer/review-merge,
                             dev-standards-security, dev-standards-backend,
                             dev-standards-frontend, dev-standards-frontend-a11y,
                              dev-standards-testing, dev-standards-git
debugger              → (A) quality/debugger-workflow, posture/tool-question,
                             posture/expert-posture,
                             shared/living-docs-enrichment,
                             quality/debugger-handoff-format †
                        (C) quality/debugger-subagent [conditional — injected by orchestrator]
auditor               → (A) auditor/auditor-workflow, posture/tool-question,
                             shared/living-docs-enrichment,
                             auditor/audit-handoff-format †
                        skill: deny
auditor-subagent      → (A) auditor/audit-protocol-light, posture/expert-posture,
                             posture/subagent-concision-posture,
                             auditor/audit-handoff-format †
                        (B) auditor/audit-<domain>  ← injected by coordinator via [SKILL:...]
                             shared/websearch-usage
designer              → (A) designer/designer-protocol,
                             design/design-planner-format,
                             design/design-handoff-format †
                        (B) designer/ux-protocol, designer/ui-protocol,
                             designer/figma-recon-protocol, designer/figma-deep-protocol,
                             designer/designer-execution-modes,
                             design/websearch-design-patterns
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
