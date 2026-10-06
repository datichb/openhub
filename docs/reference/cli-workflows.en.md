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

Lists the catalogue workflows, one per id (its most specific layer), with version, risk, execution environments and validity (`✓` valid, `!` warnings, `✗` errors). With a team-state: then **your drafts** (`✎`, error count, "new brick"), the files skipped by the integrity check and the publications waiting for the network (`⏳`). An unreadable team file is listed invalid in its own layer (team or project).

| Flag | Type | Description |
|------|------|-------------|
| `-p, --project` | string | Team-state and layer of the project (same context as the editing commands) |
| `--team` | string | Team or solo space, without a project |
| `--json` | bool | JSON output (array of summaries: id, layer, chain, risk, entry, modes, runtimes, Beads input, diagnostics; drafts are included with `"draft": true`, plus `queued`, `team_bricks`, `new_bricks` when relevant) |

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
oh workflow validate <file>|<id>|<layer>:<id> [--layer hub|team|project] [--project <project>] [--json]
oh workflow validate --all [--json]
```

Validates a file or a catalogue workflow: strict parsing, `extends` resolution, security rules, references to the brick catalogue. Non-zero exit on errors.

With `--project <project>`, the team and project layers of its team-state are loaded (else the active team); published files changed outside a publication are skipped with a warning. Brick references are checked against the catalogue **merged** with the team bricks (`catalog/`), as at launch.

## Editing team and project workflows

Commands of the draft → publication cycle (see [Team workflows](../guides/team-workflows.en.md)). All accept `-p, --project <project>`: team-state and layer of the project; by default, the project of the current folder (when it has a team or a solo space), else the active team (or, without a team, your only solo space); `--team <id>` picks a team or solo space without a project. A bare id means the project layer when it holds the workflow, else the team layer; `team:<id>` / `project:<id>` set it.

### oh workflow new

```
oh workflow new <id> [--layer team|project] [--extends <ref> | --copy <ref>] [--file <file>|-] [--no-edit]
```

Creates your draft: empty (valid) skeleton, patch of a workflow (`--extends hub:ticket`) or copy of a document under the new id (`--copy hub:review`, prompt template included). Opens `$VISUAL`/`$EDITOR` (else `vi`), then validates: on errors, the editor can be reopened, otherwise the edited file is kept.

### oh workflow edit

```
oh workflow edit <id> [--layer team|project] [--prompt] [--file <file>] [--prompt-file <file>]
```

Edits your draft (without a draft: a copy of the published version becomes your draft). `--prompt` opens the draft's own prompt template; `--file` / `--prompt-file` replace the content without an editor.

### oh workflow diff

```
oh workflow diff <id> [--against published|<version>] [--json]
```

Diff of the document and prompt template between your draft and the published version (or a history version), then the **impact summary** (⚠ = widening: risk, writing agents, remote, checkpoints, Beads, budget, MCP…) and new team bricks.

### oh workflow publish

```
oh workflow publish <id> -m "<message>" [--yes]
oh workflow publish --retry
```

Shows the next version and the impact, asks for confirmation when the workflow is widened (unless `--yes` or without a terminal), then publishes (sync, revalidation, version + 1, history, `workflows.lock`, commit + push, redone if another member published in the meantime). Offline: queued; `--retry` replays the pending publications (the TUI also replays them automatically at each team-state synchronization). Team members only (`[governance] publish`).

### oh workflow history

```
oh workflow history <id> [--json]
```

Published versions, most recent first: version, date, author, message.

### oh workflow restore

```
oh workflow restore <id> <version> [--yes]
```

Publishes the content of a version (document and template) again as a **new** version, after revalidation.

### oh workflow archive

```
oh workflow archive <id> [-m "<reason>"] [--yes]
```

Withdraws the published workflow; its last version stays in history (can be restored).

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
