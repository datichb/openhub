> [Lire en français](043-session-bundle-deploy-removal.fr.md)

# ADR-043 — Session Bundle and Removal of Per-Project Deployment

## Status

Accepted

## Date

2026-10-05

## Context

Up to v4, `oh deploy` wrote the hub into every project: `.opencode/agents/`, `.opencode/skills/`, keys in `opencode.json`, `.opencode/.deploy-state`, `.opencode/context-manifest.json` and `.opencode/team.json`. `oh sync` resynchronized it, and every worktree received links to these files (`EnsureWorktreeConfig`). ADRs [008](./008-stack-skills-dynamic-injection.en.md), [010](./010-hybrid-skills-architecture.en.md), [011](./011-external-agents-per-project.en.md) and [016](./016-execution-path-skills.en.md) describe mechanisms applied at that time.

This model had several problems:

- **one world per directory**: every session of the project saw the hub's 20 agents and ~184 skills, and the choice of "workflow" relied on instructions in the prompts;
- **writes into the user's files**, drift between what was deployed and the hub, conflicts with the project's own agents (ADR-011);
- two different workflows could not run at the same time in the same directory;
- opencode V2 reads everything it finds in the project (`instructions` included), and deployment could no longer guarantee what the model sees.

Decisions D4 and D14: one bundle per session, built outside the project, and no deployment at all.

## Decision

### 1. Session bundle

`bundle.Build` (`internal/bundle`) compiles a bundle from a **resolved workflow** (mandatory: there is no bundle without a workflow anymore). The bundle contains:

- **the workflow's member agents**, assembled: inlined skills, then the project instructions merged into the agent body (`ONBOARDING.md`, `CONVENTIONS.md`, `.claude/CLAUDE.md`, `[deploy].instruction_files`). oh never writes `AGENTS.md`.
- **on-demand skills**, with their `requires:` closure (cycles, missing dependencies and duplicate IDs are build errors). Their annexes (`annexes:`) are copied next to `SKILL.md`.
- **the skills generated from the workflow YAML** (workflow map, checkpoints, mode rules) and the stack skills detected in the project.
- **neutral permissions**, the delegation graph and the maximum depth.
- **the project's MCP servers**, filtered by the workflow's `mcp:` field. oh's `workflow` MCP server is always added.
- **declared plugins, Code Mode and the default model**.

### 2. Location and hash

- `~/.oh/bundles/<hash>/`: `bundle.json`, `agents/<id>.md`, `skills/<id>/SKILL.md` and their annexes. Files are read-only (0444): an agent allowed to write cannot change the bundle without a prompt.
- Hash = SHA-256 of the canonical `bundle.json` (without paths) and of the file contents. Compilation is idempotent: same input, same directory. Machine paths are variables expanded when the server starts (`{{oh.bundle}}`, `{{oh.bin}}`, [ADR-038](./038-sessionspec-tool-adapters.en.md)).
- The rendered config is not written into the bundle: it is passed through `OPENCODE_CONFIG_CONTENT`. The oh plugin is installed in the group data directory (`~/.oh/servers/<group>/`). Session-specific data lives in `~/.oh/sessions/<id>/`.
- The server group key contains the bundle hash: two workflows (or two versions) never share a server.
- `oh bundle build|show <workflow> [--budget] [--json]` builds or describes a bundle; `oh skill budget` is an alias.

### 3. Removal of deployment

- `internal/deploy` is removed. What the bundle still needs moves to `internal/bricks`: agent frontmatter and assembly, permission bases, model cascade, stack skills, instruction files.
- `oh deploy` and `oh sync` remain during v5.x as commands that explain the migration (exit code 0).
- Also removed:
  - `Project.Agents` (migrations v36, then v37);
  - the Agents and Deployment steps of the wizards;
  - `[deploy].disable_native_agents` (the closed world is mandatory, see ADR-041);
  - `EnsureWorktreeConfig`;
  - the Deployment section of the TUI project mode.
- The `project.agents` view becomes the **Brick catalog**, read-only: agents and skills, origin, estimated cost, workflows that ship them.
- Agents specific to a team or a project go through the team-state brick catalog ([ADR-040](./040-workflows-team-state-governance.en.md)). `.opencode/agents` is no longer read (`OPENCODE_DISABLE_PROJECT_CONFIG=1`).

### 4. Project cleanup: `oh migrate deploy-cleanup`

`oh migrate deploy-cleanup [-p] [--dry-run] [--diff] [--yes] [--json]` (`internal/deploycleanup`) processes each registered project:

- it removes `.opencode/agents`, `.opencode/skills`, `.opencode/.deploy-state`, `.opencode/context-manifest.json` and `.opencode/team.json`;
- it removes from `opencode.json` **only** a key of a type that `oh deploy` used to write **and** whose value still matches the `config_snapshot` in `.deploy-state`. A key changed since then is kept and reported. Without `.deploy-state`, the file is not touched. Key order is preserved;
- it shows a diff and asks for confirmation (`--yes` without a terminal).

The cleanup is offered once at startup, in the CLI and the TUI (dedicated screen, `cleanup` command), and Doctor reports leftovers.

## Consequences

### Positive

- Closed world per session: `cadrage` presents 4 agents and 30 skills to the model, versus 20 agents and ~184 skills with the former full deployment.
- No more writes into the user's project, except `oh migrate deploy-cleanup` with a diff and confirmation.
- Several workflows and several sessions can run on the same project, each with its own bundle.
- Reproducible, immutable bundle: the container mounts it read-only, the remote job downloads it by hash (ADR-045).
- No more drift between the hub and what is deployed: a session always uses the bricks as they were at launch.

### Negative / Trade-offs

- No bundle purge is implemented (the plan called for 30 days without reference): `~/.oh/bundles/` keeps growing.
- Any change to a brick changes the hash: new bundle, new group and new server at the next launch. A running session keeps its bundle.
- Agents placed by hand in `.opencode/agents` are no longer loaded: they must be moved to the catalog of a team or of a solo space.
- The cleanup is cautious: an `opencode.json` key changed after deployment stays, and the `.opencode` links of worktrees created by the former `EnsureWorktreeConfig` are not visited.
- A bundle's token budget is an estimate (≈ 4 characters per token); the real measurement is in the backlog (BL-14).
- ADRs 008, 010, 011 and 016 describe their mechanism in the context of deployment; the current behavior is found in the bundle.

## Alternatives Considered

| Alternative | Rejected because |
|---|---|
| Keep `oh deploy` and filter by workflow | Still one world per directory, still writes into the project, and two sessions of different workflows would interfere. |
| Deploy into one worktree per session | Costly, still in the user's files, and does not cover the project's main directory. |
| Bundle inside the project (`.oh/` ignored by git) | Writes into the repository, risk of committing it, and opencode would read what it finds in the project. |
| Inject agents and skills through the per-session `instructions` API (S8) | Experimental, covers neither agents nor permissions: under study only (P1-T29). |
| Remove from `opencode.json` every key oh may have written | The snapshot also contains the user's keys: risk of deleting their configuration. |
| Remove `oh deploy` without a replacement command | Existing scripts and habits would fail without explanation. |
