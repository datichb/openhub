> [Lire en français](039-declarative-workflows-oh-v1.fr.md)

# ADR-039 — Declarative `oh/v1` Workflows

## Status

Accepted

## Date

2026-10-05

## Context

Up to v4, `oh` had **a single workflow**, hard-coded (`BaseWorkflow`, `WorkflowDefinition`), which the hub, the team and the project could only override (stacked overrides, [ADR-033](./033-config-cascade-enforcement.en.md)). Other use cases relied on "workarounds":

- **dedicated commands or options**: `oh start --dev`, `--onboard`, `oh audit`, `oh review`, `oh debug`, `--sweep`, `--parallel`;
- **modes A to E** described in the orchestrator prompt;
- **`manuel` / `semi-auto` / `auto` modes** of the dev orchestrator ([ADR-006](./006-orchestrator-configurable-mode.en.md));
- **`[CP-x]` checkpoints** written in the agents' prose ([ADR-003](./003-orchestrator-checkpoints.en.md));
- **map of agents and delegations** maintained by hand in a skill ([ADR-018](./018-hub-workflow-reference.en.md)).

None of this could be validated, versioned or shared, and every hub agent was visible in every session.

Decisions D5 (workflows in YAML), D6 (hybrid orchestration), O1 (computed depth), O7 (typed outputs), O9 (workflow level for models) and O11 (delimited inputs).

## Decision

### 1. Format and frozen schema

A workflow is a YAML document `apiVersion: oh/v1`, `kind: Workflow`. The schema is **frozen** (`cli/internal/workflow/schema.go`, `Spec` type); it only changes through additive changes (optional field, new enumeration value). Fields:

- **identity**: `id`, `version` (managed by publication), `category`, `label`, `description` (plain or per-language text);
- **inheritance**: `extends`, `enforce`;
- **security**: `risk` (`read` < `plan` < `write` < `publish`), `isolation` (`strict` | `standard`), `code_mode`;
- **flow**: `entry` (`agent`, `selectable`), `inputs`, `prompt` (`template` or `text`), `agents` (`role`, `mode`, `after`, `calls`), `checkpoints` (`label`, behavior per `mode`, `condition`, `remote`, `mandatory`, `disabled`), `modes`, `circuit_breaker`, `preconditions`;
- **resources**: `models`, `skills` (`extra`, `deny`), `plugins`, `mcp`, `beads.allow`;
- **execution and limits**: `runtime` (`default`, `allowed`), `outputs`, `limits` (`budget_usd`, `models`).

`inputs`, `agents`, `checkpoints` and `preconditions` are ordered maps: declaration order drives the launch form and the checkpoint order.

Additive changes made after the freeze: `risk: plan` and `preconditions` (shipped workflows), `enforce` (layer locks, [ADR-040](./040-workflows-team-state-governance.en.md)), `limits.models` (I6 restrictions), `entry.selectable` (`libre` workflow, [ADR-048](./048-opencode-v1-abandonment.en.md)).

### 2. Strict parsing and diagnostics

- Unknown fields, duplicate keys and mistyped values are rejected. Every error of the file is reported, with line and column.
- A diagnostic has the shape `{Severity, Code, Path, Source, Pos, Message, Hint}`, with translated messages (`workflow.diag.<code>`). It points to the document that set the value.
- Command: `oh workflow validate <file|id> [--all] [--json]`.

### 3. Layers and resolution

- Layers: `hub` < `team` < `project` < session options (never saved). Drafts are a source, not a layer ([ADR-040](./040-workflows-team-state-governance.en.md)).
- **Patch through `extends`**:
  - the presence of a field in the file decides the replacement, not its value;
  - maps are merged by key, lists and texts are replaced as a whole;
  - an agent is removed with `role: disabled`, a checkpoint with `disabled: true`.
- A workflow with the same ID in a more specific layer **must extend** the one of the nearest lower layer (`shadow_without_extends`, `must_extend_nearest`).
- **Security fields** (`risk`, `isolation`, `runtime.allowed`, mandatory checkpoints, `beads.allow`, `limits`): they can only be hardened. A loosening is a **blocking error**, never silently ignored.
- Fields locked by `enforce:`: a more specific layer that writes them is in error.
- Each resolved value keeps its **origin** (`oh workflow show <id> --origin`, TUI editor).

### 4. Validation

Besides the schema, validation checks in particular:

- kebab-case IDs;
- agents present in the catalog;
- acyclic graph, and `workflow` agents reachable from the entry;
- primary entry agent;
- valid `after` and `calls` targets;
- template variables ⊆ declared inputs;
- `risk: read` without any agent that writes nor Beads writes; `risk: plan` without file changes, with mandatory `beads.allow` and without `delete`;
- `remote` allowed ⇒ no checkpoint waiting for the user with `remote: forbid`;
- skill closure (`requires:`) resolvable, with unique IDs;
- `isolation: strict` ⇒ adapter with full isolation.

### 5. Graph and depth

- The **delegation graph** comes from `calls` when it is written, otherwise from the agent's `task` permission, restricted to the workflow members.
- Self-delegation is possible if explicit (`calls: [<itself>]`, parallel `reviewer` sessions in `review`).
- The graph's **maximum depth** is rendered as `experimental.subagent_depth` (O1; opencode V2 limits delegation to 1 level by default).

### 6. Hybrid orchestration

- By default, the entry is **`conductor`**, a generic agent with no write or shell access. It follows the generated workflow map and delegates according to `after` and the checkpoints. A workflow can also name a dedicated entry agent (`entry.agent`).
- Flow skills are **generated from the YAML** into the bundle (`workflow/workflow-map`, `orchestrator-workflow-modes`, `hub-workflow-reference`): they are no longer maintained by hand.
- `orchestrator.md` no longer contains modes A to E.

### 7. Prompts

- Go `text/template` templates (`workflows/prompts/<id>.md.tmpl`), inputs at the top level, session context under `.oh`.
- Text inputs go through `data`: `<oh:data name="…">` tags, inner tags neutralized, truncation at `max_length` (20,000 characters by default, O11).
  - *Revision of 2026-10-08 (O11, anomaly A27)*: only the inputs computed from an outside source (`from:`) stay inside tags; an input typed by the user is their request and is written as it is (truncated, tags neutralized). Inside tags, a short imperative request ("Say only X") was taken for an injection.
- The prompt always contains the line `Mode de workflow : <mode>`. It ends with the list of checkpoints to report before each locked agent.

### 8. Modes and checkpoints

- Each checkpoint has a behavior per mode: `pause`, `auto`, `skip` or `conditional`. `mandatory` forbids a less strict behavior in a more specific layer.
- `modes.default` and `modes.allowed` bound the choice. The mode is fixed at launch and is no longer asked by the agent.
- Checkpoint execution is described in [ADR-042](./042-checkpoints-headless-decisions.en.md).

### 9. Shipped workflows and commands

- Shipped by the hub (`workflows/`, embedded): `feature`, `ticket`, `cadrage`, `onboarding`, `review`, `review-feedback`, `audit`, `debug`, `quick`, `sweep`, `brief-enrich`, `libre`.
- Launch: `oh run <workflow> [--input k=v] [--mode] [--runtime] [--location] [--tickets a,b] [--headless] [--recap] [--draft]`, a launch form generated in the TUI, a "Start" block (pinned, recent). The former commands are deprecated aliases of `oh run`.
- The **workflow level** is added to the model cascade: workflow·agent > workflow > project… (O9).
- **Typed outputs** (`branch`, `merge_request`, `beads-ids`, `path`), declared by the agent with the `workflow_outputs` MCP tool. They offer "Chain with…" at the end of a session (O7).
- Declarative **preconditions** (`path_exists`): `suggest` offers to run another workflow first and come back, `block` refuses the launch.

## Consequences

### Positive

- Each use case is a readable workflow, validated before launch, versioned and extensible per team or per project without touching the hub.
- The bundle only contains the workflow members: closed world per use case ([ADR-043](./043-session-bundle-deploy-removal.en.md)).
- Security cannot be loosened by a more specific layer; the origin of every value is visible.
- The workflow map seen by the agents is always the YAML one.
- Golden tests of the bundles and prompts of every shipped workflow.

### Negative / Trade-offs

- The frozen schema constrains changes: any incompatible change needs an explicit discussion. The addition of `entry.selectable` by track 3.E is still to be confirmed.
- Restricting an agent's writes goes no further than permissions: "`docs/wiki/` only" (`onboarding`) is a prompt instruction.
- The `condition` of a `conditional` checkpoint is not evaluated by oh: the checkpoint always asks the user.
- `enforce:` only covers top-level fields.
- Generated skills are in French, whatever the session language.
- Models may ignore the generated instructions: during acceptance testing, Haiku did not call `workflow_checkpoint`, hence the reminder at the end of the prompt. Some templates still cite inlined skills as "to load" (issue Q3-1).
- The former `hub.toml` overrides are archived but not loaded: there is no "local hub" layer.

## Alternatives Considered

| Alternative | Rejected because |
|---|---|
| Keep the single workflow and its stacked overrides (ADR-033) | Only one flow is possible; other use cases remain workarounds, and an override can neither add a workflow nor change the entry agent. |
| Describe workflows in Go inside the binary | Neither teams nor projects could create any; every change would need an oh release. |
| Keep the modes in the orchestrator prompt (modes A to E) | Neither validatable nor verifiable; every agent stays visible; the model chooses its own path. |
| A full execution engine in oh (each step launched by oh) | Loses the flexibility of orchestration by an agent; oh only controls the gates (checkpoints, `after` locks). |
| Layers free to loosen anything | A team or a project could remove a mandatory checkpoint or widen `beads.allow` without anyone noticing. |
| TOML or JSON | Less readable for templates and long texts; YAML keeps comments and order, which the editor relies on. |
