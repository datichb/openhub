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

**Example:** `auditor-security` is an agent (invocable), `audit-protocol` is a skill (format checklist injected into all auditors).

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

| Agent type | Recommended base skills |
|------------|------------------------|
| Developer | `dev-standards-universal`, `dev-standards-security`, `beads-plan`, `beads-dev` + domain skills |
| Auditor | `audit-protocol` + specific domain skill + `posture/expert-posture` |
| Coordinator (read-only) | Its own protocol — no `beads-dev` |
| Expert advisor agent | `posture/expert-posture` |
| Interactive primary agent | `posture/tool-question` (+ `permission: question: allow` in frontmatter) |
| Agent managing tickets | `beads-plan` (read + create), `beads-dev` (execution) |
| Agent producing code to test | `dev-standards-testing` |
| Agent that commits | `dev-standards-git` |

### Stack-specific skills (dynamic injection)

Developer agents automatically receive additional stack-specific skills at deploy time, based on what is detected in the target project. This dynamic injection is handled by `detect_stack()` and `resolve_stack_skills()` in `scripts/lib/prompt-builder.sh`, configured by `config/stack-skills.json`.

**You do not need to declare stack-specific skills in agent frontmatters.** They are injected automatically for the relevant agent types.

The scope of dynamic injection per agent type:

| Agent | Dynamic categories injected |
|---|---|
| `developer-frontend` | language, frontend, test, api-spec |
| `developer-backend` | language, backend, orm, test, api-spec |
| `developer-fullstack` | language, frontend, backend, orm, test, api-spec |
| `developer-mobile` | mobile, test |
| `developer-data` | language, data, test |
| `developer-devops` | infra |
| `developer-platform` | infra |

**Adding a new stack:** Add the detection signature in `detect_stack()` and the mapping entry in `config/stack-skills.json`. Create the skill file in `skills/developer/stacks/`. No agent frontmatter changes needed.

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

---

## Pre-creation checklist

### Agent

- [ ] The `description` fits in one sentence without an excessive "and"
- [ ] The family is correct (placed in the right sub-folder)
- [ ] The `mode:` field is defined: `primary` for a directly invocable agent, `subagent` for a delegated specialist
- [ ] Injected skills are consistent with the agent type (see table above)
- [ ] The body contains: identity + what it does + what it does NOT do + workflow
- [ ] Explicit limits point to the correct alternative agent if applicable
- [ ] `posture/expert-posture` is injected if the agent has an advisory or expert role
- [ ] `posture/tool-question` is injected **and** `permission: question: allow` is in the frontmatter if the `primary` agent needs to ask structured questions to the user
- [ ] `beads-plan` is injected if the agent reads or creates Beads tickets
- [ ] `beads-dev` is additionally injected if the agent executes (claims, implements, closes) tickets
- [ ] The dependency matrix in `docs/architecture/skills.en.md` is updated

### Skill

- [ ] The `description` in the frontmatter is filled in
- [ ] Content is operational (rules + examples) — not theoretical
- [ ] No duplication with existing skills
- [ ] The skill is added to the correct domain table in `docs/architecture/skills.en.md`
- [ ] **Generic skills** (`developer/`): agents that need it have it in their `skills` frontmatter; dependency matrix updated
- [ ] **Stack-specific skills** (`developer/stacks/`): detection added in `detect_stack()`, mapping added in `config/stack-skills.json` — no agent frontmatter changes needed

---

## Annotated example — Creating a `developer-security` agent

```markdown
---
id: developer-security                    # ← kebab-case, unique
label: DeveloperSecurity                  # ← PascalCase, displayed in the tool
description: Application security         # ← one sentence, usage-oriented
  development assistant — [...]
mode: subagent                            # ← subagent: invocable only via orchestrator-dev
targets: [opencode, claude-code]  # ← both supported targets
skills:                                   # ← from most generic to most specific
  - developer/dev-standards-universal     #   standards common to all devs
  - developer/dev-standards-security      #   preventive security
  - developer/dev-standards-security-hardening  # agent-specific domain
  - developer/dev-standards-backend       #   application context
  - developer/dev-standards-testing       #   it writes tests
  - developer/dev-standards-git           #   it commits
  - developer/beads-plan                  #   it reads and creates tickets
  - developer/beads-dev                   #   it executes tickets
---
```

**Recommended skill order:** universal → security → specific domain → context → tests → git → beads.

---

## Annotated example — Creating a `dev-standards-api` skill

```markdown
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
```

**What not to do:**
```markdown
## Introduction
In the world of modern APIs, it is crucial to...  # ← no lecture
```
