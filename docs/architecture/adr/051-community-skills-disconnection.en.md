> [Lire en français](051-community-skills-disconnection.fr.md)

# ADR-051 — Community Skills Registry Disconnected

## Status

Accepted

## Date

2026-10-09

## Context

[ADR-027](027-audit-extensibility.en.md) (July 2026) introduced a skill "marketplace": `oh skill add|list|remove|search`, a community index and installation into `~/.oh/skills/<name>/`. The `oh skill budget` computation (per-agent "Bucket A" budget, September 2026) dates from the same period. Both predate v5 and its session bundles.

The v5 audit (2026-10-08) showed they no longer make sense as they are:

- **Not working.** The `datichb/oh-skills-index` index does not exist (404): `oh skill search` and `oh skill add <name>` always fail. Installing from a GitHub repository, the documented case, writes no file (GitHub archives put the files at the root, which the code skipped) while the command prints "installed".
- **Risky.** Names and archive paths were not checked: an archive could write outside `~/.oh` ("zip-slip"), `oh skill remove ..` deleted the whole `~/.oh`, `skill_file` could point to a file outside the package. No hash, no signature, `main` branch followed, `http` accepted.
- **Against the v5 model.** A skill installed on one machine joined that machine's catalogue: the same team workflow produced different bundles depending on the computer, with no review and no trace in the team-state.
- **Duplicate.** `oh skill budget` ran its own computation on the former "Bucket A" model, while `oh bundle show <workflow> --budget` gives the budget of the real bundle.

The team catalogue (`catalog/{agents,skills}/` of the team-state, [ADR-040](040-workflows-team-state-governance.en.md)) already covers adding bricks, versioned and shared.

## Decision

These historical features are **disconnected**, until adding bricks is redesigned:

- The `oh skill add`, `list`, `remove`, `search` and `budget` commands are removed (cobra answers "unknown command"). `oh skill` keeps only `oh skill check`.
- The `internal/skillregistry` package and the former `internal/bricks/skill_budget.go` computation are removed (they remain in the git history).
- `~/.oh/skills` is no longer read: not when bundles are built, not when workflows are validated, not by `oh skill check`. A workflow listing a former community skill in `skills.extra` is invalid ("unknown skill").
- **Nothing is deleted** on the user's machine. `oh doctor` (and the TUI Doctor view) warns about the packages left in `~/.oh/skills` and explains how to keep one (team catalogue).

Unchanged: `oh skill check`, the team catalogue (validation, bundles, "new brick" badge), the TUI brick catalogue, `skills.extra` / `skills.deny` of workflows, `oh bundle show --budget` and the model cascade (`oh config model agent|family`).

## Consequences

### Positive

- No more attack surface from the registry (writes outside its folder, deletion of `~/.oh`, unchecked content in bundles).
- A session bundle no longer depends on the machine: it only comes from the hub, the team-state and the workflow.
- A single budget measurement, the one of the real bundle.

### Negative / Trade-offs

- No way left to add a skill without a team-state. A single user can create a solo space (`oh team init --solo`) and use its catalogue.
- The team catalogue is still edited by hand (commit in the team-state repository): no dedicated command in the CLI or the TUI.

## Leads for the redesign

To be taken up in a dedicated evolution (new ADR):

1. **A brick command** (`oh brick` or `oh catalog`), modelled on `oh workflow`: `list [--agents|--skills] [--origin]`, `show`, `new`, `edit`, `publish`, `history` for the team catalogue, with the same governance and integrity check.
2. **Importing external bricks into the team catalogue** (not onto the machine): version pinned by commit, verified hash, checked names and paths, `https` only, review before publication.
3. **TUI**: editable brick catalogue, a picker for `skills.extra` / `skills.deny` in the workflow editor, an "entry agent" field in the launch form when the workflow allows it (`selectable`), a "free session with this agent" action.
4. **Checks**: agent and family identifiers verified by `oh config model agent|family` (CLI and Models view).
5. Optionally a **shared index**, only if signed and versioned.

## Alternatives considered

| Alternative | Rejected because |
|-------------|------------------|
| Fix the registry (path checks, root layout, index) | Keeps a per-machine mechanism, against reproducible bundles; fixes a usage that does not exist (no index) |
| Keep the commands with a "disconnected" message | Dead commands to maintain; an unknown command is clear enough, the ADR and `oh doctor` document the migration |
| Keep `oh skill budget` until v6 | Different computation from the real bundle, misleading alias; `oh bundle show --budget` replaces it |
| Delete the packages of `~/.oh/skills` on upgrade | Destructive without the user's consent; a warning is enough |
