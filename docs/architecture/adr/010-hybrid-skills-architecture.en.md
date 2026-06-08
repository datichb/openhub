> 🇫🇷 [Lire en français](010-hybrid-skills-architecture.fr.md)

# ADR-010 — Hybrid Skills Architecture: Inline (Bucket A) vs Native (Bucket B)

## Status

Accepted

## Context

As the hub grew to 40+ skills, all skills were assembled inline into agents at deploy time. Every agent system prompt contained the full text of every skill declared in its frontmatter — including domain-specific standards (Vue.js conventions, security hardening, WCAG checklists) that are only relevant to a subset of tasks.

This created three problems:

1. **Token bloat**: Developer agents had system prompts exceeding 8 000 tokens before the first user message. Stack-specific skills (TypeScript, React, NestJS, Prisma…) were always included, even for tasks unrelated to those stacks.
2. **Cognitive noise**: The LLM receives all context upfront regardless of the actual task. A `developer-frontend` agent working on a pure CSS bug receives the full NestJS, Prisma, and OpenAPI standards for no benefit.
3. **Stack skills partially redundant with ADR-008**: Dynamic stack injection (ADR-008) was supposed to add _only_ the project's stack skills. But the inline model still embedded those skills permanently into the system prompt once injected, offering no way to load them conditionally at inference time.

OpenCode introduced a native `skill` tool that allows agents to load skill files on-demand at inference time, from `.opencode/skills/<name>/SKILL.md`. This enables a selective, task-driven loading model.

## Decision

Split skills into two buckets based on their loading guarantee requirement:

### Bucket A — Inline (mandatory, always-on)

Skills that **must be active from the first token** because they define the agent's fundamental behavior, output contracts, or workflow structure.

Declared in the agent frontmatter under `skills: [...]`. Assembled inline at deploy time by `prompt-builder.sh`. The LLM cannot skip them.

**Bucket A includes:**
- Workflow protocols (`*-protocol`, `*-workflow`)
- Handoff formats (`*-handoff-format`) — shared contracts between producer and consumer agents
- Core execution skills (`beads-plan`, `beads-dev`, `quick-fix`)
- Universal principles (`dev-standards-universal`, `dev-standards-simplicity`)
- Posture skills (`expert-posture`, `tool-question`, `coordination-only`, `retranscription-coordinateur`)
- Living docs skills (`shared/living-docs-enrichment` — all agents that produce analysis or implementation work)

### Bucket B — Native (contextual, on-demand)

Skills that **provide domain-specific context** and are only relevant to a subset of tasks. Loaded by the LLM on-demand using the `skill` tool when the task requires it.

Declared in the agent frontmatter under `native_skills: [...]`. Deployed by `deploy_native_skills()` in `opencode.adapter.sh` to `.opencode/skills/<name>/SKILL.md`. The hub guide in the agent body lists available native skills and when to load them.

**Bucket B includes:**
- Domain standards (`dev-standards-security`, `dev-standards-backend`, `dev-standards-frontend`, `dev-standards-testing`, `dev-standards-git`, etc.)
- Stack-specific skills (`developer/stacks/*`)
- Audit domain checklists (`audit-security`, `audit-performance`, `audit-accessibility`, etc.)
- Documentation type skills (`doc-standards`, `doc-adr`, `doc-api`, `doc-changelog`, `doc-slides`)
- Contextual research skills (`websearch-stack-research`, `websearch-design-patterns`, `websearch-cve-lookup`, `websearch-performance-research`)

### Permission model

- Agents that use native skills: `permission: skill: allow` in frontmatter
- Coordinator/orchestrator agents (no native skills needed): `permission: skill: deny`

### Deploy mechanism

`deploy_native_skills()` in `opencode.adapter.sh`:
1. Collects all `native_skills` entries from all agent frontmatters
2. Collects stack skills from `config/stack-skills.json` for the detected project stack
3. Deduplicates by basename
4. Wipes `.opencode/skills/` entirely, then recreates it with one `SKILL.md` per skill
5. Each generated `SKILL.md` has a valid opencode frontmatter (`name:`, `description:`)

The wipe-and-recreate approach guarantees that obsolete skills from previous deploys are never left behind.

## Consequences

### Positive

- System prompt size reduced significantly for all agents — domain standards are no longer injected unconditionally.
- The LLM receives domain context exactly when it is relevant to the task, not always.
- Adding a new domain standard skill does not increase the baseline system prompt size.
- Stack skills (ADR-008) now follow the same native path, completing the intent of ADR-008.
- The mandatory/optional distinction is explicit in the frontmatter, making agent authoring intent clear.

### Negative / trade-offs

- If an agent forgets to load a native skill before producing its output, domain standards are missing. Mitigated by the skills guide section in the agent body that lists available skills and their loading triggers.
- The `skill` tool must be available (`permission: skill: allow`) — coordinators that never need contextual skills explicitly set `skill: deny`.
- The `.opencode/skills/` directory is fully regenerated at every deploy (wipe + recreate). This is intentional for consistency but means skills cannot be manually patched in the target project.

## Relationship to other ADRs

- **ADR-001** (Agent/Skill Separation): evolved — the separation now has two deployment paths (inline vs native) rather than a single assembly at deploy time.
- **ADR-008** (Stack Skills Dynamic Injection): evolved — stack skills now deploy via `deploy_native_skills()` to `.opencode/skills/` and are loaded natively at inference time, rather than being assembled inline.
