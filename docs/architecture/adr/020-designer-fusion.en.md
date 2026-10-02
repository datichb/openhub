> [Lire en francais](020-designer-fusion.fr.md)

# ADR-020 — Merge ux-designer and ui-designer agents into a unified designer agent

## Status

Accepted

## Date

2026-06-30

## Context

Before this decision, five hub agents had direct access to the Figma MCP:
`planner`, `pathfinder`, `onboarder`, `ux-designer`, and `ui-designer`. Each carried
a dedicated Figma adapter skill (`figma-planner-protocol`, `figma-pathfinder-protocol`,
`figma-onboarder-protocol`, `figma-ux-designer-protocol`, `figma-ui-designer-protocol`),
representing 1,285 lines of duplicated or adapted Figma content for each context.

Furthermore, the `ux-designer` and `ui-designer` agents were rarely invoked
independently: in more than 90% of cases, a feature required both perspectives
(UX and UI) or at least a preliminary exploration of existing mockups. This separation
into two distinct agents forced coordinators to orchestrate two sequential `task`
invocations for conceptually unitary work.

The dispersion of Figma access across five agents made integration governance
difficult: any MCP Figma update required coordinated modifications in five files,
with a risk of version drift between them.

## Decision

We decided to merge `ux-designer` and `ui-designer` into a single `designer` agent,
capable of operating in four distinct modes:

- **`recon` mode** — Figma exploration: mockup search, token extraction, design system
  detection. Replaces the 3 Figma adapter skills of planning agents (planner,
  pathfinder, onboarder).
- **`ux` mode** — UX specifications: user flows, Nielsen heuristics, acceptance
  criteria. Takes over the former `ux-designer` workflow.
- **`ui` mode** — UI specifications: design tokens, components, variants, guidelines.
  Takes over the former `ui-designer` workflow.
- **`ux+ui` mode** — Complete UX then UI processing in a single session.

The `designer` agent is now the **only hub agent with Figma MCP access**. The
`planner`, `pathfinder`, and `onboarder` agents delegate all their Figma needs to the
`designer` via `task` (`recon` mode), instead of calling the MCP directly.

The five dedicated Figma adapter skills are removed. Two new Figma protocol skills
are created in `designer/`: `figma-recon-protocol` (lightweight exploration) and
`figma-deep-protocol` (in-depth component structure analysis).

## Consequences

### Positive

- **-678 lines** of prompt on planning agents (removal of 3 Figma adapter skills
  and inline MCP logic).
- **-9 files** total: 2 agents removed (`ux-designer`, `ui-designer`), 5 Figma
  adapter skills removed, 2 execution skills merged into
  `designer-execution-modes`.
- Centralized Figma architecture: single source of truth for MCP access; MCP Figma
  updates only impact one agent.
- Simplified user experience: a single agent to know for all design needs, with an
  explicit `Mode:` parameter in the prompt.
- Consistent delegation: `planner`, `pathfinder`, `onboarder` use the same
  `task: designer` + `Mode: recon` pattern as for other delegations.

### Negative / Trade-offs

- **+30s latency** for Figma explorations from `planner`/`pathfinder`/`onboarder`:
  the addition of an intermediate `task` invocation introduces an extra session hop
  compared to direct MCP calls.
- **Breaking change**: any `opencode.json` configuration referencing `ux-designer` or
  `ui-designer` must be updated. No automatic alias is provided.
- The `task` permissions of `orchestrator`, `planner`, `pathfinder`, and `onboarder`
  agents must be updated to authorize `designer` instead of `ux-designer` and
  `ui-designer`.

## Migration

See the complete migration guide:
[docs/guides/migration-designer-fusion.md](../../guides/migration-designer-fusion.md)
