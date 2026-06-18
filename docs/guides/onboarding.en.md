> 🇫🇷 [Lire en français](onboarding.fr.md)

# Guide — Onboarding onto an Existing Project

This guide covers the use of the `onboarder` agent to quickly discover
an existing project: stack, architecture, risks, and agents to prioritise.

---

## When to invoke the onboarder

| Situation | Recommendation |
|-----------|---------------|
| You are joining a project you don't know | Invoke the onboarder first |
| You are resuming a project after a long absence | Invoke the onboarder to refresh context |
| You are about to start an important mission (feature, refactoring) | Invoke before handing anything to the orchestrator |
| You want to know which hub agents are relevant for this project | The onboarder produces the recommended agent map |
| The orchestrator detects an unknown project (Mode C) | The orchestrator offers to invoke the onboarder — you can accept or skip |

The onboarder is **read-only**. It writes no code, modifies no project
files (except `docs/wiki/`, minimal `ONBOARDING.md` and `projects.md` — only after explicit confirmation).

When `docs/wiki/index.md` **already exists** (enriched by other agents),
the onboarder proposes **incremental enrichment** by default rather than a full overwrite.
It applies the `living-docs-enrichment` skill and delegates to the `documentarian`.
Full overwrite remains available with an explicit warning about losing accumulated enrichments.

---

## Invoking the onboarder

```
"Onboard yourself onto this project"
"Discover this project and give me an overview"
"Before starting, explore the project"
"What is this project?"
```

The onboarder announces what it will read before reading it, then produces a
structured context report.

---

## Example session

### 1. Invocation

```
> "Onboard yourself onto this project"
```

### 2. The onboarder announces the exploration

```
I will read the configuration files at the root to identify the stack.

I read: package.json, .nvmrc, docker-compose.yml, .github/workflows/ci.yml

Stack detected: Node.js 20 + Vue 3 + PostgreSQL — fullstack profile.

I will now explore the structural files:
- src/router/index.ts — frontend routes
- src/stores/ — Pinia global state
- src/services/ — API calls
- server/routes/ — backend endpoints
- server/models/ — data models
- migrations/ — pending migrations
- .env.example — environment variables
```

### 3. Context report

```markdown
## Context Report — my-app — 2026-03-30

### Stack

| Category | Detected technologies |
|----------|-----------------------|
| Language(s) | TypeScript 5.3 |
| Framework(s) | Vue 3.4 + Nuxt 3, Fastify 4 |
| Database(s) | PostgreSQL 15 (Prisma ORM), Redis 7 |
| Infrastructure | Docker, GitHub Actions |
| Tests | Vitest, Playwright (E2E) |

### Architecture

Fullstack monorepo — SSR Nuxt frontend, Fastify backend exposing
a REST API. Frontend/backend communication via native fetch (no
generated client). Redis used for sessions and cache.

### Dominant patterns

- Composition API + `<script setup>` systematic on the Vue side
- Repository pattern on the backend (services → repositories → Prisma)
- Conventional Commits respected in the git log

### Points of attention

🔴 **Critical**
- `server/config/db.ts` line 12: database password
  hard-coded in the file (not read from environment variables)

🟠 **Important**
- 3 pending unapplied migrations (`migrations/20260318_*`,
  `20260322_*`, `20260325_*`) — risk of schema/code desync
- No test files in `server/services/` (untested business logic)

🟡 **Improvements**
- No ARIA attributes on forms (`src/components/forms/`)
- Bundle not split — a single 1.2MB JS chunk detected in `nuxt.config.ts`

### Blind spots

- OAuth authentication logic delegated to an external service —
  no visible code in this repository
- No README — setup is not documented

### Clarification questions

1. Are the 3 pending migrations intentionally unapplied
   (pending validation) or forgotten?
2. The external OAuth service — does it have a separate repository
   or available documentation?
3. The hard-coded database key in `db.ts` — is this a local dev file
   or is this file committed to the repository?

### Recommended agents

#### Priority — detected risk zones

| Agent | Why | Suggested invocation |
|-------|-----|----------------------|
| `auditor` (security) | Hard-coded secret in `server/config/db.ts:12` | `"Audit this project's security"` |
| `developer-security` | To invoke after the audit to fix vulnerabilities | `"Implement hardening following the security audit"` |
| `qa-engineer` | Business logic in `server/services/` entirely untested | `"Write missing tests for backend services"` |

#### Recommended — detected stack

| Agent | Why | Suggested invocation |
|-------|-----|----------------------|
| `developer-fullstack` | Vue 3 + Fastify in the same repo | `"Implement [feature]"` |
| `developer-api` | Exposed Fastify REST API | `"Implement endpoint [X]"` |

#### Optional — depending on project ambitions

| Agent | Why | Suggested invocation |
|-------|-----|----------------------|
| `auditor` (accessibility) | Forms without ARIA in `src/components/forms/` | `"Audit accessibility"` |
| `auditor` (performance) | 1.2MB unsplit JS bundle | `"Audit performance"` |
| `documentarian` | No README — setup not documented | `"Document the setup and architecture of this project"` |

---

> These invocations are suggestions — it is up to you to decide when and if you launch them.
```

### 4. Q&A — answers to clarification questions

The onboarder asks its questions and **waits for answers before writing anything**.
You can answer, or say `"skip"` to ignore the questions.

```
1. The migrations have been pending since last week —
   they are waiting for business validation, not forgotten.
2. The OAuth service is in a private repository: github.com/my-org/auth-service
3. db.ts is a local dev file not committed — .gitignore covers it.
```

### 5. Updated report

The onboarder integrates the answers and redisplays only the impacted sections:

```markdown
### Blind spots — updated

- ~~OAuth authentication logic~~ → separate repository: `github.com/my-org/auth-service`
- No README — setup is not documented

### Points of attention — updated

🔴 **Critical** — unchanged
🟠 **Important**
- 3 intentionally pending migrations (business validation) — to apply after decision
- ~~Hard-coded database key~~ → local dev file not committed, non-critical
- No test files in `server/services/` (untested business logic)
```

### 6. Confirmation before generation

```
Everything is clear — I am ready to generate the context files.
Generate? (Generate / Cancel)
```

```
→ Generate
```

### 7. Wiki generation

The onboarder produces the following files and adds them to `.git/info/exclude` (local exclusion, invisible to other devs):

- **`docs/wiki/index.md`** — global map: critical stack, architecture, god nodes (most connected concepts), business domain map, active critical points, dark zones
- **`docs/wiki/technical/architecture.md`** — detailed architecture, layering, structural decisions
- **`docs/wiki/technical/stack.md`** — full stack, versions, key libraries, environment variables
- **`docs/wiki/technical/tests.md`** — test strategy, frameworks, coverage thresholds, conventions
- **`docs/wiki/technical/conventions.md`** — linting, naming, git, config, team patterns
- **`docs/wiki/business/index.md`** — business domain map
- **`docs/wiki/business/<domain>.md`** — one file per detected business domain: business rules, flows, entities, risks
- **`ONBOARDING.md`** — minimal summary at the root (15-25 lines), redirects to `docs/wiki/index.md`

> The wiki replaces the old `CONVENTIONS.md`, `docs/context/technical.md` and `docs/context/business/` files. The `index.md` identifies "god nodes" — key concepts appearing in multiple pages — to guide agents to the essentials first.
> Each enrichment carries a confidence tag: `` `CONFIRMÉ` `` (direct code observation), `` `DÉDUIT` `` (contextual reasoning) or `` `INCERTAIN` `` (hypothesis to validate).

### 8. `projects.md` update proposal

If the `Stack` field is absent or generic in `projects.md`:

```
I detected the following stack: TypeScript 5.3, Vue 3 + Nuxt 3, Fastify 4,
PostgreSQL 15, Redis 7.

Would you like me to update the `Stack` field in projects.md? (yes / no)
```

---

## Interpreting the report

### Attention levels

| Level | Meaning | Recommended action |
|-------|---------|-------------------|
| 🔴 Critical | Direct impact on security, stability or data | Address before any new feature |
| 🟠 Important | Notable technical debt, medium-term risk | Plan for upcoming sprints |
| 🟡 Improvement | Quality, performance, accessibility opportunity | Prioritise according to project ambitions |

### The agent map

- **Priority** — directly triggered by detected 🔴/🟠. Handle first.
- **Recommended** — determined by the stack. These are the agents you'll use daily.
- **Optional** — relevant depending on project goals. Activate when the time comes.

Suggested invocations are starting points — adapt them to the real context.

### Blind spots

Blind spots are what the onboarder **cannot determine** from the
codebase. This is not a failure — it is useful information. The clarification
questions that follow help fill these gaps.

After receiving your answers, the onboarder updates the report (impacted
sections only), then requests explicit confirmation before writing
the context files. The files thus reflect the enriched analysis, not the first draft.

---

## Structure of the generated wiki

The onboarder generates a two-level wiki structure optimising context loading in sessions:

```
docs/wiki/
├── index.md                    ← always read first — 40-80 lines
├── technical/
│   ├── architecture.md         ← loaded if task concerns architecture
│   ├── stack.md                ← loaded if task concerns dependencies
│   ├── tests.md                ← loaded if task concerns tests
│   └── conventions.md          ← loaded if task concerns code
└── business/
    ├── index.md                ← domain map
    └── <domain>.md             ← loaded if task concerns this domain
ONBOARDING.md                   ← minimal, redirects to docs/wiki/index.md
```

**`docs/wiki/index.md`** contains: critical stack (3-5 lines), architecture (2-3 lines),
god nodes table (critical concepts appearing in multiple pages), business domain map,
active critical points, dark zones.

**`docs/wiki/technical/conventions.md`** contains: linting, language & typing, naming,
Git conventions, config & secrets, team patterns, do-not-use.

The other `technical/` and `business/<domain>.md` pages are loaded on demand by
agents according to their current task — never all at once.

---

## Integration in an orchestrator workflow

The orchestrator can offer to invoke the onboarder automatically in **Mode C**
when it detects an unknown project. This mode is always optional.

### Full workflow with Mode C

```
1. You ask the orchestrator: "Implement the JWT authentication feature"
2. The orchestrator detects the project has not been explored in this session
3. The orchestrator proposes:
   "Unknown project. Invoke the onboarder first? (yes / no — skip if you already know the project)"
4. You answer "yes"
5. The onboarder explores the project and produces the report
6. [CP-onboard] The orchestrator presents the report summary and asks:
   "Sufficient context to start the feature? (yes / no — questions?)"
7. You validate → the orchestrator continues in Mode A (planner → routing)
```

### Skipping Mode C

If you already know the project or don't need the report:

```
> "No, skip — I know the project"
```

The orchestrator proceeds directly to Mode A or B.

---

## Advanced use cases

### Onboarder + planner in sequence

The onboarder identifies debt zones or risks → you can then ask
the planner to create tickets to address them:

```
1. "Onboard yourself onto this project"
   → Report: pending migrations, missing tests on services

2. "Create tickets for the identified points of attention"
   → The planner creates debt tickets with the right priorities
```

### Onboarder + auditor in sequence

The onboarder flags a priority security risk → you launch the targeted audit:

```
1. "Onboard yourself onto this project"
   → Report: hard-coded secret, missing CORS

2. "Audit this project's security"
   → The auditor deepens the analysis (security domain)

3. "Implement hardening following the security audit"
   → The developer-security fixes the vulnerabilities
```

### Re-onboarding after a long absence

The onboarder can be invoked at any time — not just the first time.
If you come back to a project after several weeks:

```
"Onboard yourself onto this project — I've been away 3 weeks"
```

The onboarder will read the current state of the codebase, recently closed tickets,
and give you an up-to-date overview.

### Re-onboarding — incremental mode

When `docs/wiki/index.md` already exists (generated by a previous onboarding
and progressively enriched by other agents: `developer-*`, `reviewer`, `auditor`, etc.),
the onboarder detects this in Phase 5 and proposes three options:

1. **Incremental enrichment (Recommended)** — wiki pages are enriched via the `living-docs-enrichment`
   skill which delegates to the `documentarian`; existing enrichments (with their confidence tags) are preserved.
2. **Full overwrite** — with an explicit warning that accumulated enrichments will be lost.
3. **Keep existing** — make no changes.

If a **new business domain** is discovered during re-onboarding, the onboarder proposes
creating a new `docs/wiki/business/<new-domain>.md` page and updating `docs/wiki/business/index.md`.

This preserves the continuous improvement loop: the wiki accumulates knowledge from all agents
over the entire project lifecycle, with each enrichment traceable by agent, date and confidence level.
