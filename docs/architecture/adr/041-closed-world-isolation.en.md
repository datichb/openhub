> [Lire en français](041-closed-world-isolation.fr.md)

# ADR-041 — Closed World and Isolation Check

## Status

Accepted

## Date

2026-10-05

## Context

Up to v4, a session saw:

- every hub agent deployed into the project;
- opencode's native agents (`build`, `plan`, `general`, `explore`) — hiding them was an option (`[deploy].disable_native_agents`);
- the built-in skills (`opencode`, `report`);
- everything the user had in `~/.config/opencode`.

The model could call any agent, and nothing checked what it was actually shown.

Findings on opencode 2.0.20:

- an inline config that disables the natives and only declares our agents and skills closes the world, except for the two built-in skills, which must be refused explicitly (F5);
- `~/.config/opencode` is still loaded (F5);
- **session** permissions hide skills but **not sub-agents**; only agent rules do (F21);
- `GET /api/skill` lists every skill regardless of permissions (F28).

Decision D13: the closed world is **mandatory and not configurable**, implemented and **verified** by each adapter; if verification fails, the session does not start.

## Decision

### 1. Implementation (`opencodev2` adapter)

- **Native agents disabled**: their list is discovered at first startup (agents listed outside the bundle), cached per version, with a default of `build`, `plan`, `general`, `explore`. Hidden system agents (`compaction`, `title`, `summary`) are tolerated.
- **Skills**: `skill` denied on `*`, then allowed for the bundle skills only (this hides `opencode` and `report`).
- **Sub-agents**: for each agent, `subagent` denied on `*`, then allowed towards the targets of the workflow graph.
- **Project config and data excluded**: `OPENCODE_DISABLE_PROJECT_CONFIG=1`, and an `XDG_DATA_HOME` specific to the server group.
- **oh plugin**: `agent.transform` and `skill.transform` remove any agent or skill outside the bundle. This is a second barrier, on top of the rules.
- **Reading the bundle**: `external_directory` is allowed on the bundle's skills directory, whose files are read-only.
- **Code Mode**: `execute` is denied when `code_mode` is not enabled.

### 2. Verification: `Attest`

`Attest` runs after each server start and for each new directory, once agents and skills are loaded. It compares what the tool exposes with the bundle:

- **agents** listed by the tool;
- **skills visible to each agent**, judged on the **effective rules** returned by `/api/agent` (global, user and agent rules merged), falling back to the rendered rules;
- listed **MCP servers**;
- the user's global **plugins**: a simple, non-blocking warning.

Any unexpected item (`agent:<id>`, `skill:<id>@<agent>`, `mcp:<name>`) makes the launch fail. The server is then abandoned (stopped, token revoked) and a precise message is shown.

### 3. One server per workflow version

Since session permissions do not hide sub-agents (F21), a server cannot host sessions whose worlds differ. The server group key therefore contains the bundle hash ([ADR-047](./047-session-interaction-daemon.en.md)).

### 4. Strict isolation

- `isolation: strict` in a workflow requires an adapter with full isolation and forbids answering "always" to a permission.
- The `[execution] strict_isolation` setting additionally hides the user's opencode config, for the local runtime: `XDG_CONFIG_HOME` is replaced by a mirror without `opencode/`, so that git, gh and other tools keep their config.
- In a container and remotely, the user's config is never visible.

### 5. Not configurable

`[deploy].disable_native_agents` is removed. Native agent names only exist in the adapter.

## Consequences

### Positive

- The model only sees what the workflow declares: `cadrage` presents 4 agents and 30 skills instead of 20 agents and ~184 skills.
- A leak is detected at launch, not discovered during the session. The contract test adds an agent to `~/.config/opencode` and checks that the launch fails.
- The per-agent check also catches a user rule that would make a skill visible to one of our agents.
- Two independent barriers: the config rules and the plugin.

### Negative / Trade-offs

- Without strict isolation, locally, the user's opencode config is loaded. `Attest` refuses what adds visible agents, skills or MCP servers, but does not check other settings.
- The result depends on opencode's rule merge order: a user rule on one of our agents currently comes before ours. The contract test `TestContractAttestChecksSkillsPerAgentOnServer` breaks if this order changes.
- A declared plugin that does not load is silent (the case of `context-mode` under V2), because `Attest` does not check that it is active. A plugin whose ID differs from its package name produces an "outside the bundle" warning.
- The closed world is about what the model sees, not what the shell can do: locally, the agent keeps the user's rights ([ADR-044](./044-credential-proxy-session-limits.en.md), `SECURITY`).
- One server per workflow version costs memory (~340 to 440 MB per server).

## Alternatives Considered

| Alternative | Rejected because |
|---|---|
| Optional closed world (`disable_native_agents`) | Leaves natives and every agent visible by default; that was the starting point. |
| A shared server with per-workflow session permissions | Session permissions do not hide sub-agents (F21). |
| Only check the rendered config | Always true by construction; sees neither the user's config nor what the tool actually exposes. |
| Hide `XDG_CONFIG_HOME` by default | The agent's shell would lose the config of git, gh and other tools; hence a mirror without `opencode/`, as an option. |
| Rely on the plugin alone | If the plugin does not load, nothing closes the world; the config rules remain the main barrier. |
