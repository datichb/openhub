# ADR-001 — Agent / Skill Separation

## Status

Accepted — **Evolved by [ADR-010](./010-hybrid-skills-architecture.en.md)**

The core separation (agent identity vs. skill protocol) remains valid. ADR-010 extends the deployment model: skills are now split into two buckets — Bucket A (inline, always-on via `skills:`) and Bucket B (native, on-demand via `native_skills:`). The assembly-at-deploy-time principle still applies to Bucket A; Bucket B skills are deployed as separate files and loaded at inference time.

## Context

When designing the hub, two approaches were possible for defining AI agent behavior: putting all logic directly in the agent file, or separating the agent's identity (who it is, what it does) from its protocol (how it does it).

The first prototype concentrated everything in the agent file. This made files long (200-300 lines), hard to maintain, and prevented reuse of protocols across agents. For example, the review report format was duplicated in both the reviewer and the orchestrator.

## Decision

An agent's behavior is split into two layers:

- **Agent (`agents/<id>.md`)**: identity, role, what it does / doesn't do, condensed workflow, invocation examples. Short file (~40-80 lines).
- **Skill (`skills/<domain>/<name>.md`)**: detailed protocol, output formats, checklists, behavior rules, full examples. Reference file (~100-300 lines).

Skills are declared in the agent's frontmatter via the `skills: [...]` key.
The hub assembles agent + skills at deployment time.

## Consequences

### Positive

- A skill can be shared across multiple agents (e.g. `dev-standards-universal`
  is injected into all developer agents and the reviewer)
- Agents remain readable and quickly editable
- Protocols evolve independently of agents
- The identity / behavior separation facilitates composition

### Negative / trade-offs

- An agent without its skill is incomplete — both must always be deployed together
- Logic is spread across two files, which can be confusing at first
- Requires knowing the `skills/` structure to understand actual behavior

## Rejected Alternatives

**Everything in the agent**: files become too long, no reuse, hard to maintain.

**Skills as Markdown imports**: technically possible but not natively supported
by target tools (OpenCode, OpenCode) — hub-side assembly is the only
portable approach.
