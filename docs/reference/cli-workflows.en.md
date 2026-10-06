> [Lire en français](cli-workflows.fr.md)

# CLI Reference — Workflows

Declarative workflows (`apiVersion: oh/v1`) describe a use case: entry agent, agents, checkpoints, inputs, resources and allowed execution environments. They are read by layers (hub, then the team and project layers of the team-state, see [Team workflows](../guides/team-workflows.en.md)); the most specific layer extends the one below.

## oh run

```
oh run <workflow> [-i key=value]… [--tickets a,b] [--mode <mode>] [--runtime local|container]
                  [--location base|new|<worktree>] [--attach <opening>] [--recap] [-p <project>] [-P <provider>]
```

Launches a workflow: layer resolution and validation, session bundle, session plan, initial prompt rendering, then start through the RunService (one server per group: bundle version, project, runtime).

- **Inputs** (`-i`, repeatable): values of the workflow `inputs` (`true`, `3`, `a,b` are converted to the input type). Defaults may depend on other inputs (`branch: feat/{{ .ticket }}`). Free text is injected between `<oh:input name="…">…</oh:input>` tags and truncated (`max_length`, else 1,000 characters, 8,000 for `text`).
- **Tickets** (`--tickets`): fill the first `beads-id` input; with `picker.multi`, **one session per ticket**, all in the same server group, each in its own worktree when the workflow writes. A `beads-ids` input receives the list in a single session.
- **Location** (`--location`): `base` (default, project directory), `new` (a new worktree per session, branch = `branch` input or `oh/<workflow>-<ticket>`), or the path of an existing worktree. A writing session (`risk` other than `read`) **automatically gets a worktree** when another writing session is active in the same directory. Replaces `--worktree`.
- **Runtime** (`--runtime`): `local` or `container` (must be in `runtime.allowed`; refused with the reason when the container engine is unavailable).
- **Recap** (`--recap`): agents, first-turn budget, isolation, sessions and locations, warnings (uncommitted changes, automatic worktree, pending decisions), then confirmation.
- **Single session** (`--one-session`): every ticket in the same session, instead of one session per ticket.
- **MCP**: without an `mcp:` field in the workflow, the session gets the project MCP servers; with `mcp:` (even empty), only the listed ones (a listed server missing from the project is reported).
- **Preconditions**: a blocking precondition refuses the launch; a suggestion (e.g. no wiki → `onboarding`) offers to run the suggested workflow first. With `resume: true`, the initial launch is remembered and offered again when that session ends ("Chain with…" in the TUI).
- **Without interface** (`--headless [--output <file>] [--timeout 30m]`): no window is opened; oh waits for the end of the turn, writes the answer (standard output or file, one file per session: `<file>.<ticket>`) then stops the session. Refused when a checkpoint waits for a validation in the chosen mode. A session asking for a decision (permission, question) stays open: `oh session inbox`, `oh session approve`. `oh takeover-brief enrich` goes through `oh run brief-enrich --headless`.
- Two simultaneous launches in the same directory are refused ("launch in progress").
- **Draft** (`--draft`): runs the version being edited (your draft, team or project layer) instead of the published one. Refused on a remote runtime and when the draft **widens** the published version (risk, checkpoints, runtimes, Beads, budget…): publish it to apply these changes. See [Team workflows](../guides/team-workflows.en.md).
- Requires opencode V2.

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
