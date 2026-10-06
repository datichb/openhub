> [Lire en français](cli-workflows.fr.md)

# CLI Reference — Workflows

Declarative workflows (`apiVersion: oh/v1`) describe a use case: entry agent, agents, checkpoints, inputs, resources and allowed execution environments. They are read by layers (hub, then team and project in phase 2); the most specific layer extends the one below.

## oh workflow list

Lists the catalogue workflows, one per id (its most specific layer), with version, risk, execution environments and validity (`✓` valid, `!` warnings, `✗` errors).

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | JSON output (array of summaries: id, layer, chain, risk, entry, modes, runtimes, Beads input, diagnostics) |

## oh workflow show

```
oh workflow show <id>|<layer>:<id> [--origin] [--json]
```

Shows a workflow once its `extends` are resolved: header (chain, version, risk, isolation, entry agent, modes, runtimes, Code Mode), inputs, agents, checkpoints (behavior per mode), resources (skills, MCP, Beads, plugins, outputs, models) and diagnostics. The command fails when the workflow is invalid.

| Flag | Type | Description |
|------|------|-------------|
| `--origin` | bool | Shows the layer that set each value (`default` when no document writes it) |
| `--json` | bool | JSON output (resolved `spec`, `origins` by field path, `diagnostics`) |

## oh workflow validate

```
oh workflow validate <file>|<id>|<layer>:<id> [--layer hub|team|project] [--json]
oh workflow validate --all [--json]
```

Validates a file or a catalogue workflow: strict parsing, `extends` resolution, security rules, references to the brick catalogue. Non-zero exit on errors.

## oh bundle build / show

```
oh bundle build <workflow> [-p <project>] [-P <provider>] [--json]
oh bundle show <workflow>|<hash> [-p <project>] [--budget] [--json]
```

Compiles (idempotently, by hash) the session bundle of a workflow into `~/.oh/bundles/<hash>/` and shows it: agents (entry agent first), delegations, on-demand skills, MCP, plugins, default model, delegation depth, isolation (`strict` when the workflow requires it), global permissions and the estimated cost of the first turn (entry agent + skill catalogue). Without `-p`, the project is the one of the current directory (otherwise a hub-only bundle, without project instructions or MCP).

| Flag | Type | Description |
|------|------|-------------|
| `--budget` | bool | Details the estimated budget: tokens per agent and per skill (≈ 4 characters per token) |
| `--json` | bool | JSON output (`bundle.Report`) |

`oh skill budget <workflow>` is a deprecated alias of `oh bundle show <workflow> --budget`; without a workflow, the former per-agent computation stays available with a warning.

## Environment variables

| Variable | Effect |
|----------|--------|
| `OH_WORKFLOWS_DIR` | Replaces `~/.oh/hub/workflows` as the hub layer (workflows under development, tests) |
