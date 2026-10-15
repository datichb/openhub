> 🇫🇷 [Lire en français](authoring.fr.md)

# Guide — Creating a Good Agent or Skill

This guide covers the design decisions for creating effective agents and skills,
consistent with the hub architecture.

---

## Agent or skill?

The first question to ask before creating anything.

| Criterion | Agent | Skill |
|-----------|-------|-------|
| Has a proper role, an invocable identity | ✅ | ❌ |
| Contains reusable rules or protocols | — | ✅ |
| Directly invoked by the user | ✅ | ❌ |
| Injected into multiple agents | ❌ | ✅ |
| Orchestrates other agents | ✅ | ❌ |
| Defines an output format or a checklist | — | ✅ |

**Decision rule:**
- If you answer "invoke [X] to do Y" → **agent**
- If you answer "apply these rules / this protocol when doing Y" → **skill**

**Example:** `auditor-subagent` is an agent (invocable), `audit-protocol` is a skill (format checklist injected into all auditors).

---

## Designing an agent

### Single responsibility

An agent has a clear and bounded responsibility. If it does "too many things", it is often a sign it should be split into two agents or that part of its logic belongs in a skill.

**Good signal:** the `description` fits in one sentence without a redundant "and".

**Bad signal:** "does X, Y, Z and also W depending on context" → split it.

### What an agent body must contain

1. **Identity** (1 paragraph) — who it is, what it does, its fundamental constraints
2. **What it does** — list of concrete responsibilities
3. **What it does NOT do** — explicit limits (as important as responsibilities)
4. **Workflow** — the steps in order, with Beads commands if applicable
5. **Technical focus** (optional) — patterns specific to its domain

### When to add a constraint in "What it does NOT do"

Add an explicit constraint if:
- The agent might naturally attempt to violate it (e.g. an auditor wanting to "fix" things itself)
- The limit is non-obvious to a user (e.g. a reviewer that doesn't close tickets)
- Another agent is responsible for that action (clarify who to delegate to)

### Families and placement

| Family | When to use it |
|--------|----------------|
| `auditor/` | Read-only agents that analyse and report |
| `design/` | UX/UI design agents — do not write code |
| `developer/` | Agents that implement code |
| `documentation/` | Agents that write documentation |
| `planning/` | Agents that orchestrate or plan — do not write code |
| `quality/` | Quality agents (review, QA, debug) |

An agent that writes code goes in `developer/`. An agent that orchestrates goes in `planning/`.
An agent that audits (read-only) goes in `auditor/`.

### Skills to inject by agent type

The hub uses a hybrid architecture — see [ADR-010](../architecture/adr/010-hybrid-skills-architecture.en.md):
- **Bucket A** (`skills: [...]`) — always-on, inline. Workflow protocols, handoff formats, universal principles.
- **Bucket B** (`native_skills: [...]`) — on-demand via the `skill` tool. Domain standards, stack skills, checklists.

Agents using native skills must have `permission: skill: allow`. Coordinators set `permission: skill: deny`.

| Agent type | Bucket A (always inline) | Bucket B (native, on-demand) |
|------------|--------------------------|------------------------------|
| Developer | `dev-standards-universal`, `dev-standards-simplicity`, `beads-plan`, `beads-dev` | `dev-standards-security`, domain skills, stack skills |
| Auditor subagent | `audit-protocol-light`, `auditor-subagent-payload`, `shared/handoff-envelope` | Domain audit checklist (`audit-security`, `audit-accessibility`, etc.) |
| Coordinator (read-only) | Its own protocol + `posture/coordinator-posture` (the statuses of the launched agents come from the session contract) — no `beads-dev` | none (`skill: deny`) |
| Expert advisor agent | `posture/expert-posture` | — |
| Interactive primary agent | `posture/user-facing-posture` (+ `permission: question: allow`) | — |
| Agent launched by another | `shared/handoff-envelope`, `<family>/<agent>-payload` (statuses in `statuses:`, 200 to 400 tokens, `when: role=subagent`) | — |
| Agent managing tickets | `beads-plan` (read + create), `beads-dev` (execution) | — |
| Agent that commits | Inline via `beads-dev` | `dev-standards-git` (native) |

### Stack-specific skills (Bucket B — native)

Stack skills are **always Bucket B**. When the session bundle is built (at launch, `oh deploy` removed in v5), oh detects the project stack (`DetectSignals()` in `cli/internal/bricks/stack_signals.go`) and adds the matching stack skills to the bundle's on-demand skills. The LLM loads the relevant ones on-demand at inference time. Nothing is written to the project's `.opencode/skills/`.

**You do not need to declare stack-specific skills in agent frontmatters.** The stack → skills mapping is the `stackSkillMapping` table in `cli/internal/bricks/stack_skills.go` (language, framework, test runner, Docker, CI).

**Adding a new stack:** add the detection in `DetectSignals()` and the entry in `stackSkillMapping`. Create the skill file in `skills/developer/stacks/`. No agent frontmatter changes needed — the skill will be automatically shipped in the bundle of the relevant projects (`oh bundle show <workflow>` to check).

### Project instructions

The `project_instructions` field decides what the agent gets of the project instructions (`ONBOARDING.md`, `CONVENTIONS.md`, `.claude/CLAUDE.md`, `[deploy] instruction_files`): `full` (default) for an agent that writes or judges code against the conventions, `index` (file titles, wiki index and god nodes) for an agent that scopes, plans or coordinates and reads the details on demand, `none` for a pure coordinator that reads no file. oh allows an `index` agent to read the listed files, on their exact paths; an explicit refusal of the agent on one of them is kept, and the line of the file tells it. Details: [Project instructions by level](../architecture/agents.en.md#project-instructions-by-level).

---

## Designing a skill

### A skill = a contract

A skill defines a contract that the agent commits to respecting. It is not a lecture — it is a set of operational rules, formats, and patterns that are directly applicable.

**A good skill answers:** "When you do X, here is exactly how you do it."

### Recommended skill structure

```markdown
---
name: skill-name
description: One sentence — what this skill brings to the agent that injects it.
---

# Skill — Title

## Role
This skill defines... It complements <other-skill> if applicable.

---

## [Thematic section 1]
<rules + code examples>

---

## [Thematic section N]
<rules + code examples>

---

## What this skill does not replace (optional)
<explicit limits — who to delegate to for going further>
```

### Content rules

- **Concrete before abstract**: start with rules, not philosophy
- **Code examples**: show a ✅ good example and a ❌ bad example for non-trivial rules
- **No duplication**: if a rule exists in `dev-standards-universal`, don't repeat it — reference it
- **Description in frontmatter**: short sentence, benefit-oriented for the consuming agent

### Granularity

**Too broad:** a skill covering "the entire backend" — impossible to inject selectively.
**Too narrow:** a skill covering only a single 3-line rule — doesn't justify a separate file.

**Good granularity:** a coherent domain that several agents could share, with 5 to 15 concrete rules.

### When to create a new skill vs. enrich an existing one

| Situation | Action |
|-----------|--------|
| New rules in the same domain | Enrich the existing skill |
| Rules used by a different subset of agents | New skill |
| Rules that would be injected in more than 3 distinct agents | New skill |
| Output format protocol specific to one agent | Dedicated new skill |
| Rules for a distinct technical domain (e.g. API vs backend) | New skill |

### Bucket A or Bucket B?

| Criterion | Bucket A (inline) | Bucket B (native) |
|-----------|------------------|------------------|
| Must be active from the first token | ✅ | ❌ |
| Defines a handoff contract between two agents | ✅ | ❌ |
| Universal workflow / protocol (always applies) | ✅ | ❌ |
| Domain-specific — only relevant to a subset of tasks | ❌ | ✅ |
| Stack-specific convention | ❌ | ✅ |
| Audit checklist for a specific domain | ❌ | ✅ |
| Documentation type standard (ADR, API, changelog…) | ❌ | ✅ |
| Contextual research protocol | ❌ | ✅ |

---

## Pre-creation checklist

### Agent

- [ ] The `description` fits in one sentence without an excessive "and"
- [ ] The family is correct (placed in the right sub-folder)
- [ ] The `mode:` field is defined: `primary` for a directly invocable agent, `subagent` for a delegated specialist
- [ ] `project_instructions` is chosen (`full` by default, `index` for an agent that reads the details on demand, `none` for a pure coordinator)
- [ ] `permission: skill: allow` is set if the agent uses native skills (Bucket B); `skill: deny` for coordinators
- [ ] ctx permissions are declared according to the agent type (ctx tools are NOT inherited — they must be explicit): orchestrators → `ctx_search`, `ctx_stats`, `ctx_batch_execute`; developers → add `ctx_execute`, `ctx_execute_file`, `ctx_fetch_and_index`, `ctx_index`; quality/audit → `ctx_search`, `ctx_execute`, `ctx_execute_file`, `ctx_batch_execute`; design/planning → `ctx_search`, `ctx_batch_execute`
- [ ] Bucket A skills (`skills:`) are consistent with the agent type: workflow protocol, handoff formats, universal principles
- [ ] Bucket B skills (`native_skills:`) are consistent with the agent type: domain standards, checklists, contextual skills
- [ ] The body contains: identity + what it does + what it does NOT do + workflow + native skills guide section
- [ ] Explicit limits point to the correct alternative agent if applicable
- [ ] `posture/expert-posture` is in `skills:` (Bucket A) if the agent has an advisory or expert role
- [ ] `posture/user-facing-posture` is in `skills:` (Bucket A) **and** `permission: question: allow` is in the frontmatter if the `primary` agent needs to ask structured questions to the user
- [ ] `beads-plan` is in `skills:` (Bucket A) if the agent reads or creates Beads tickets
- [ ] `beads-dev` is additionally in `skills:` (Bucket A) if the agent executes (claims, implements, closes) tickets
- [ ] The dependency matrix in `docs/architecture/skills.en.md` is updated

### Skill

- [ ] The `description` in the frontmatter is filled in
- [ ] Content is operational (rules + examples) — not theoretical
- [ ] No duplication with existing skills
- [ ] The skill is added to the correct domain table in `docs/architecture/skills.en.md` with its bucket marker (A) or (B)
- [ ] **Bucket A skills**: agents that need it have it in their `skills:` frontmatter; dependency matrix updated
- [ ] **Bucket B skills**: agents that need it have it in their `native_skills:` frontmatter; the agent body guide section lists it with a loading trigger description
- [ ] **Stack-specific skills** (`developer/stacks/`): detection added in `detect_stack()`, mapping added in `config/stack-skills.json` — no agent frontmatter changes needed

### Automatic content checks

`go test ./internal/bundle` checks the content shipped with every workflow:
- **one checkpoint protocol**: no question (`question`) whose header or text names a checkpoint (`cp-2`, « CP 2 », its label), no « end the session » to relay a checkpoint (`TestShippedCheckpointProtocol`); a skill generated from the workflow stays a « GÉNÉRÉ » stub, no skill is orphaned;
- **capabilities** (`TestCapabilities`): every command written for an agent (code spans, shell blocks, known first word: `bd`, `git`, `npm`, `go`…) must be allowed by its permissions; every `task(subagent_type: "x")` must target an agent it may call; an agent `after: cp-x` is not asked in the template before `cp-x`; a checkpoint that unlocks an operation has a member able to run it. A line that forbids (`❌`, « jamais »…) or relays an instruction to another agent (`> 1.`, « … ») is not counted. For a command meant for the user, an example or an optional agent, add `<!-- capability: user -->` (or `example`, `optional`) on the line, or alone on the line before a code block. The remaining exceptions are in `cli/internal/bundle/testdata/capability_allow.yaml`, the report in `testdata/capabilities.golden` (`-update`).

---

## Annotated example — Creating a `developer-security` agent

> **Hypothetical example.** The hub ships no `developer-security` agent: its single `developer` covers security through the `security` domain of the routing matrix (`orchestrator/orchestrator-dev-protocol`). The example shows the frontmatter of a specialised agent you could add to a team catalogue.

```markdown
---
id: developer-security                    # ← kebab-case, unique
label: DeveloperSecurity                  # ← PascalCase, displayed in the tool
description: Application security         # ← one sentence, usage-oriented
  development assistant — [...]
mode: subagent                            # ← subagent: invocable only via orchestrator-dev
permission:
  skill: allow                            # ← enables the native skill tool (Bucket B)
skills:                                   # ← Bucket A: always-on from the first token
  - developer/dev-standards-universal     #   principles common to all devs
  - developer/dev-standards-simplicity    #   KISS/YAGNI — always active
  - developer/beads-plan                  #   reads and creates tickets
  - developer/beads-dev                   #   executes tickets
  - developer/quick-fix                   #   minimal-impact fix protocol
  - shared/handoff-envelope               #   return envelope (Bucket A — mandatory as subagent)
  - developer/developer-payload           #   statuses and report sections (Bucket A — mandatory)
native_skills:                            # ← Bucket B: loaded on-demand via skill tool
  - developer/dev-standards-security      #   security principles
  - developer/dev-standards-security-hardening  # hardening specifics
  - developer/dev-standards-backend       #   application context
  - developer/dev-standards-testing       #   it writes tests
  - developer/dev-standards-git           #   it commits
---
```

**Recommended skill order for Bucket A:** postures → envelope → payload → universal → simplicity → beads-plan → beads-dev → quick-fix.

**Native skills guide section** (required in agent body when `native_skills` is non-empty):

```markdown
## Available skills

Load the relevant skill(s) via the `skill` tool before starting work:

| Skill | Load when |
|-------|-----------|
| `dev-standards-security` | Any task involving auth, input validation, secrets, injections |
| `dev-standards-security-hardening` | CORS, HTTP headers, JWT, sessions, rate limiting, encryption |
| `dev-standards-backend` | Layered architecture, DTOs, services, repositories |
| `dev-standards-testing` | Writing or reviewing tests |
| `dev-standards-git` | Committing or reviewing git history |
```

---

## Annotated example — Creating a `dev-standards-api` skill

````markdown
---
name: dev-standards-api                   # ← kebab-case, readable
description: Standards specific to        # ← concrete benefit for the agent
  APIs — versioning, pagination, [...]
---

# Skill — API Standards

## Role
This skill defines best practices for public APIs.
It complements `dev-standards-backend.md`.  # ← point to complements

## Versioning                             # ← one section = one theme
- Recommended URL prefix...

## Pagination                             # ← concrete code examples
```json
{ "data": [...], "pagination": { ... } }
```
````

**What not to do:**
```markdown
## Introduction
In the world of modern APIs, it is crucial to...  # ← no lecture
```
