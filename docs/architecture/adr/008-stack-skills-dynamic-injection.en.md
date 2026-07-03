> 🇫🇷 [Lire en français](008-stack-skills-dynamic-injection.fr.md)

# ADR-008 — Dynamic injection of stack-specific skills at deploy time

## Status

Accepted — **Evolved by [ADR-010](./010-hybrid-skills-architecture.en.md)**

The stack detection mechanism (`detect_stack()`, `config/stack-skills.json`) remains valid. ADR-010 changes the deployment path: stack skills are no longer assembled inline into agent system prompts. They are now deployed to `.opencode/skills/` by `deploy_native_skills()` and loaded on-demand at inference time via the `skill` tool (Bucket B). This completes the intent of ADR-008 — agents only receive stack context when the task requires it.

## Context

Developer agents had stack-specific skills hardcoded in their frontmatter (e.g. `dev-standards-vuejs` always injected into `developer-frontend`, regardless of the project's stack). This forced all projects to receive Vue.js conventions even when working on a React or Angular codebase.

With the introduction of 38 stack-specific skills covering 9 categories (languages, frontend, backend, ORMs, test tools, mobile, data/ML, DevOps, platform), maintaining per-stack agent variants was not viable. A `developer-frontend-vue`, `developer-frontend-react`, etc. approach would have created N × M combinations that were unmaintainable and incompatible with the orchestrator routing matrix.

The generic skills (`dev-standards-universal`, `dev-standards-backend`, etc.) also contained framework-specific references that had leaked in over time, making them partial duplicates of stack-specific knowledge.

## Decision

- Create a `skills/developer/stacks/` directory for atomic stack-specific skills (one file per stack, one responsibility per file).
- Declare the mapping between detected stacks and skills to inject in `config/stack-skills.json`. Each agent type has a defined scope (`_agent_scope`) that limits which categories of stack skills it receives.
- Detect the project stack at every `oh deploy` via `detect_stack(project_path)`: reads `package.json`, `pyproject.toml`, `requirements.txt`, `Gemfile`, `build.gradle`, `pom.xml`, `pubspec.yaml`, and infrastructure files (`Dockerfile`, `.github/workflows/`, `*.tf`, `Chart.yaml`, ArgoCD manifests, etc.).
- Inject the corresponding skills dynamically via `resolve_stack_skills(agent_id, stacks, config)`, which filters by agent scope and deduplicates skills already declared in the frontmatter.
- Stack skills are **additive**: they are appended after the static skills declared in the agent frontmatter — never replacing them.
- Purge all framework-specific references from generic skills (`dev-standards-universal`, `dev-standards-testing`, `dev-standards-api`, `dev-standards-security`, `dev-standards-devops`) so that they remain truly tool-agnostic.
- Extend `oh deploy --check` to re-detect the stack and verify the mtimes of dynamically injected stack skills, so that modifications to `skills/developer/stacks/*.md` correctly trigger staleness for affected agents.

## Consequences

### Positive

- A single `developer-frontend` agent covers Vue, React, Angular, Next.js, Nuxt.js without duplication: each project receives only the conventions that match its actual stack.
- Adding a new stack requires only one skill file + one entry in `config/stack-skills.json` — no agent frontmatter changes needed.
- Standards are as precise as possible for the project's real stack.
- Generic skills are clean and truly agnostic: they focus on principles, not tools.
- `oh deploy --check` correctly detects staleness caused by stack skill changes.

### Negative / trade-offs

- `jq` is a runtime dependency for `resolve_stack_skills`. This was already required by other hub functions, so it does not introduce a new constraint.
- Agents deployed at hub level (without `PROJECT_ID`) do not benefit from stack skill injection — the deploy path lacks a project context for detection.
- The detection heuristics in `detect_stack()` are file-based and may occasionally produce false positives (e.g. detecting `docker` when a `Dockerfile` is present for an unrelated reason). These are low-risk false positives that add harmless context.

## Rejected Alternatives

**Per-stack agent variants** (`developer-frontend-vue`, `developer-frontend-react`, etc.): rejected because N agents × M stacks is unmaintainable, breaks the orchestrator routing matrix, and duplicates agent body logic.

**Stack declared in `hub.json`**: declare the project stack in the hub config and generate the corresponding frontmatters. Rejected because it creates tight coupling between the hub config and canonical agents, requires manual maintenance after each stack change, and does not support automatic detection.

**Skills declared per-project in `projects.md`**: extend the `projects.md` format with a `skills:` override field. Rejected because it moves configuration decisions from the hub (which manages canonical agents) to the project registration file, creating a dual source of truth.
