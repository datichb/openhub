> [Lire en français](058-primary-agent-called-is-all.fr.md)

# ADR-058 — A `primary` agent called by the workflow is shipped as `mode: all`

## Status

Accepted — complements [ADR-038](038-sessionspec-tool-adapters.en.md) (neutral session model and adapters) and [ADR-039](039-declarative-workflows-oh-v1.en.md) (declarative workflows)

## Date

2026-11-02

## Context

The session bundle ([ADR-043](043-session-bundle-deploy-removal.en.md)) ships each agent of the workflow with a `mode` (`primary`, `subagent`), taken from `agents.<id>.mode` of the workflow, otherwise from the agent frontmatter. opencode **refuses to launch a `primary` agent through `task`** ("Agent X cannot run as a subagent").

The flaw showed up in a real session of the `review` workflow: the `reviewer` (`mode: primary` in its frontmatter) delegates to itself for the combined modes (`standard+adversarial`, `all`), which launch independent sessions in parallel. opencode refused the self-delegation. The combined modes therefore never launched a parallel session, and `oh workflow validate` answered "valid". The same case existed for `documentarian` (`primary` in its frontmatter), launched by the `developer` in `ticket` and `implement-spec` according to the `task` permission.

Fixing each workflow by hand would not hold: the delegation graph of a workflow is composed through `extends` and layers (hub, team, project), and an agent changes role from one workflow to another (`developer` is `primary` in `quick`, called in `ticket`).

## Decision

**We decided that the shipped mode is computed from the delegation graph of the workflow, not only from the written mode.**

- `workflow.Spec.ResolveMode(id, base, graph)` gives the shipped mode: `agents.<id>.mode` if the workflow writes one, otherwise `base` (the agent frontmatter, `primary` by default). **A `primary` agent, written or not, that the graph lets a member of the workflow launch, itself included, becomes `all`.** `subagent` and `all` agents and those nobody calls keep their mode.
- The graph is `DelegationGraph`: the `calls:` of the workflow, or failing that the `task` permission of the agent restricted to the workflow. A self-delegation is only in it if `calls` names the agent itself (this is how `review` declares `calls: [reviewer]`). `bundle` applies `ResolveMode` when building the bundle (`workflowgen.go`, `applyWorkflowModes`): the generated workflow map and the agent file say the same thing.
- `all` is a valid value of `agents.<id>.mode` (`AgentMode.Valid`, offered by the TUI workflow editor): visible in opencode's picker and launchable through `task`. It does not exist in an agent frontmatter.
- **`calls_target_primary` warning** (`oh workflow validate`): a workflow that writes `mode: primary` on an agent it makes called is warned — the bundle ships it as `all` anyway; the warning suggests writing `mode: all` to say so, or `mode: subagent` if the user never picks it. A catalogue `primary` agent that is called without the workflow writing its mode is not reported: it is shipped as `all`, without surprise.
- opencode 2.0.20 accepts `all` (checked in the binary). The entry agent stays `primary` (`entry_not_primary`).

## Consequences

### Positive

- The `reviewer` self-delegation works: the combined modes launch their parallel sessions.
- No workflow to rewrite: the graph is enough, including for a team or project workflow that extends a hub workflow.
- The shipped mode is the same in the workflow map and in the agent file.

### Negative / trade-offs

- An agent may be `primary` in one workflow and `all` in another: the effective mode depends on the workflow (priority: workflow > frontmatter > `primary`).
- An `all` agent shows in opencode's picker although it is first of all a subagent of the workflow; this is the price of opencode's constraint.
- 18 reference files (goldens) of bundles changed, for the `mode` only: `reviewer` in `review`, `documentarian` in `ticket` and `implement-spec`, and the other called `primary` agents.
- The rule comes from an opencode constraint; it is computed in the neutral workflow model (`workflow`, `bundle`), not in the adapter: the bundle describes the mode, the adapter renders it as it is.

## Alternatives considered

| Alternative | Rejected because |
|-------------|------------------|
| Write `mode: all` by hand in each workflow | Workflows compose through layers and `extends`: an agent called by a team workflow would stay `primary`, and the error would only show at run time |
| Turn the agent into a `subagent` | The user could no longer pick it when it is the entry agent of another workflow |
| Make `validate` fail on a called `primary` | The bundle can fix it by itself; blocking would only force a rewrite with no benefit |
| Do not delegate to oneself (one session per mode in the prompt) | Loses the context isolation of parallel sessions, which is the point of the combined modes |
