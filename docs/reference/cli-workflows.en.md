> [Lire en français](cli-workflows.fr.md)

# CLI Reference — Workflows

Declarative workflows (`apiVersion: oh/v1`) describe a use case: entry agent, agents, checkpoints, inputs, resources and allowed execution environments. They are read by layers (hub, then the team and project layers of the team-state, see [Team workflows](../guides/team-workflows.en.md)); the most specific layer extends the one below.

## oh run

```
oh run [workflow] [-i key=value]… [--tickets a,b] [--one-session] [--mode <mode>] [--runtime local|container|remote]
                  [--location base|new|<worktree>] [--allow-dirty | --stash] [--attach <opening>] [--recap] [--draft] [-a <agent>]
                  [--headless [--output <file>] [--timeout <duration>]] [--parent <session>] [-p <project>] [-P <provider>]
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--input` | `-i` | string (repeatable) | | Workflow input `key=value` |
| `--tickets` | | list (commas) | | Beads tickets (one session per ticket when the workflow allows it) |
| `--one-session` | | bool | `false` | Every ticket in a single session |
| `--mode` | | string | workflow default mode | `manuel`, `semi-auto` or `auto` |
| `--runtime` | | string | see below | `local`, `container` or `remote` |
| `--location` | | string | `base` | `base`, `new` (new worktree) or the path of an existing worktree |
| `--allow-dirty` | | bool | `false` | Launch a writing session in a directory with uncommitted changes anyway |
| `--stash` | | bool | `false` | Stash (`git stash`) the uncommitted changes of the directory before the launch |
| `--attach` | | string | Settings preference | Opening: `auto`, `iterm`, `terminal`, `tmux`, `browser`, `suspend`, `none` |
| `--recap` | | bool | `false` | Recap and confirmation before the launch |
| `--draft` | | bool | `false` | Run your draft of the workflow (local only) |
| `--agent` | `-a` | string | | Entry agent (workflows with a selectable entry, e.g. `libre`) |
| `--headless` | | bool | `false` | Without interface: wait for the end of the turn, print the answer, stop the session |
| `--output` | | string | standard output | With `--headless`: file of the answer |
| `--timeout` | | duration | `30m` | With `--headless`: maximum wait (`0` = none) |
| `--parent` | | string | | Previous session (chaining) |
| `--project` | `-p` | string | project of the current folder | Project |
| `--provider` | `-P` | string | project provider | LLM provider |

Launches a workflow: layer resolution and validation, session bundle, session plan, initial prompt rendering, then start through the RunService (one server per group: bundle version, project, runtime).

- **Inputs** (`-i`, repeatable): values of the workflow `inputs` (`true`, `3`, `a,b` are converted to the input type). Defaults may depend on other inputs (`branch: feat/{{ .ticket }}`). In the prompt template, text inputs are wrapped with the `data` function in `<oh:data name="…">…</oh:data>` tags and truncated (`max_length`, else 20,000 characters); see [Workflow schema](workflow-schema.en.md).
- **Tickets** (`--tickets`): fill the first `beads-id` input; with `picker.multi`, **one session per ticket**, all in the same server group, each in its own worktree when the workflow writes. An **epic** stands for its open or in-progress children (one session each, or a single session with all of them with `--one-session`; the recap shows `Epic  <epic> → <children>`); an epic without such a child is refused. A `beads-ids` input receives the list in a single session. In a team project, each ticket is claimed for the active member when its session starts, or moves from « planned » to the work status (for any workflow, not only `oh start --dev`); a ticket claimed by another member is reported.
- **Computed inputs** (`from:`): an empty input that declares `from:` is computed by oh at launch (e.g. `review-feedback`: branch, target branch and discussions read on the MR of the `mr` input; `brief-enrich`: the takeover brief of the ticket). Sources: [workflow schema](workflow-schema.en.md#computed-inputs-from).
- **Location** (`--location`): `base` (default, project directory), `new` (a new worktree per session, branch = `branch` input or `oh/<workflow>-<ticket>`), or the path of an existing worktree. A writing session (`risk` other than `read`) **automatically gets a worktree** when another writing session (running or asleep) uses the same directory. Replaces `--worktree`.
- **Uncommitted changes**: a writing session never starts in a base directory with uncommitted changes (untracked files included) without a choice. By default it works in a **new worktree** of its branch and your changes stay untouched; `--stash` puts them aside first (`git stash apply <commit>` brings them back, the commit is printed), `--allow-dirty` launches there anyway. An existing worktree chosen with `--location <path>` is only reported. Whatever the choice, no agent may run the git commands that throw away work (`git checkout <…>` except `-b`, `git restore`, `git reset --hard|--merge|--keep`, `git clean`, `git stash` except `list`/`show`, `git switch -f|--discard-changes`, `git worktree remove`, `git branch -D`, `git rm` except `--cached`, also as `git -C <dir> …`): branch changes go through `git switch`.
- **Default workflow**: without argument, `oh run` launches the project default workflow (Project config › Execution).
- **Runtime** (`--runtime`): `local`, `container` or `remote` (must be in `runtime.allowed`; refused with the reason when the container engine is unavailable or the remote target is not set up); without `--runtime`, the project default runtime, then the Settings one, then the workflow one, when allowed. See [Container](../guides/container.en.md) and [Remote execution](../guides/remote-runners.en.md) (`oh remote setup`, then `oh session fetch` / `oh session resolve` on return).
- **Recap** (`--recap`): agents, first-turn budget, MCP, Code Mode, allowed Beads commands, checkpoints in the session mode (pause, auto, skipped, conditional; mandatory ones marked), isolation, sessions and locations, warnings (uncommitted changes, automatic worktree, pending decisions), then confirmation.
- **Single session** (`--one-session`): every ticket in the same session, instead of one session per ticket.
- **MCP**: without an `mcp:` field in the workflow, the session gets the project MCP servers; with `mcp:` (even empty), only the listed ones (a listed server missing from the project is reported).
- **Preconditions**: a blocking precondition refuses the launch; a suggestion (e.g. no wiki → `onboarding`) offers to run the suggested workflow first. With `resume: true`, the initial launch is remembered and offered again when that session ends ("Chain with…" in the TUI).
- **Without interface** (`--headless [--output <file>] [--timeout 30m]`): no window is opened; oh waits for the end of the turn, writes the answer (standard output or file, one file per session: `<file>.<ticket>`) then stops the session. Refused when a checkpoint waits for a validation in the chosen mode. A session asking for a decision (permission, question) stays open: `oh session inbox`, `oh session approve`. Beyond `--timeout`, the session is stopped and oh exits with an error (code 1). `oh takeover-brief enrich` goes through `oh run brief-enrich --headless`.
- **Chaining** (`--parent <session>`): links the new session to the previous one ("Chain with…" in the TUI).
- Two simultaneous launches in the same directory are refused ("launch in progress").
- **Draft** (`--draft`): runs the version being edited (your draft, team or project layer) instead of the published one. Refused on a remote runtime and when the draft **widens** the published version (risk, checkpoints, runtimes, Beads, budget…): publish it to apply these changes. See [Team workflows](../guides/team-workflows.en.md).
- **Entry agent** (`--agent`, `-a`): for a workflow with a selectable entry (`libre`), the starting agent; the members are this agent and those it may call. Refused on other workflows.
- Requires opencode V2 (minimum version in `oh doctor`).

**Examples:**

```bash
oh run                                              # project default workflow
oh run feature -i request="Add a /health endpoint"
oh run ticket --tickets bd-42,bd-43                 # one session per ticket
oh run ticket --tickets bd-42,bd-43 --one-session
oh run libre --agent debugger -i request="Test X fails"
oh run audit -i type=security --runtime container
oh run review --headless --output review.md --timeout 20m
oh run ticket --tickets bd-42 --runtime remote
```

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

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--all` | bool | `false` | Validates every hub workflow |
| `--layer` | string | `hub` | Layer of the validated file: `hub`, `team` or `project` |
| `--project` | string | | Project (id or name) whose layer is loaded with its team layer (no short form) |
| `--team` | string | | Team or solo space, without a project |
| `--json` | bool | `false` | JSON output |

With `--project <project>`, the team and project layers of its team-state are loaded (else the active team); published files changed outside a publication are skipped with a warning. Brick references are checked against the catalogue **merged** with the team bricks (`catalog/`), as at launch.

## Editing team and project workflows

Commands of the draft → publication cycle (see [Team workflows](../guides/team-workflows.en.md)). All accept `-p, --project <project>`: team-state and layer of the project; by default, the project of the current folder (when it has a team or a solo space), else the active team (or, without a team, your only solo space); `--team <id>` picks a team or solo space without a project. A bare id means the project layer when it holds the workflow, else the team layer; `team:<id>` / `project:<id>` set it. The `-p, --project` and `--team` flags are not repeated in the tables below.

### oh workflow new

```
oh workflow new <id> [--layer team|project] [--extends <ref> | --copy <ref>] [--file <file>|-] [--no-edit]
```

Creates your draft: empty (valid) skeleton, patch of a workflow (`--extends hub:ticket`) or copy of a document under the new id (`--copy hub:review`, prompt template included). Opens `$VISUAL`/`$EDITOR` (else `vi`), then validates: on errors, the editor can be reopened, otherwise the edited file is kept.

> Limitation: `--file` refuses a document whose `prompt.template` points to a template that does not exist yet. Create the draft without `prompt:`, then `oh workflow edit <id> --file <doc> --prompt-file <template>` (or the TUI editor, key `P`).

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--layer` | string | `team` | Draft layer: `team` or `project` |
| `--extends` | string | | Workflow to extend (e.g. `hub:ticket`): the draft is a patch |
| `--copy` | string | | Document to duplicate (e.g. `hub:review`) under the new id |
| `--file` | string | | Read the document from this file (`-`: standard input) instead of opening the editor |
| `--no-edit` | bool | `false` | Save the initial draft without opening the editor |

### oh workflow edit

```
oh workflow edit <id> [--layer team|project] [--prompt] [--file <file>] [--prompt-file <file>]
```

Edits your draft (without a draft: a copy of the published version becomes your draft). `--prompt` opens the draft's own prompt template; `--file` / `--prompt-file` replace the content without an editor.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--layer` | string | project when it holds the workflow, else team | `team` or `project` |
| `--prompt` | bool | `false` | Edit the prompt template instead of the document |
| `--file` | string | | Replace the document with this file (`-`: standard input) |
| `--prompt-file` | string | | Replace the draft's prompt template with this file |

### oh workflow diff

```
oh workflow diff <id> [--against published|<version>] [--json]
```

Diff of the document and prompt template between your draft and the published version (or a history version), then the **impact summary** (⚠ = widening: risk, writing agents, remote, checkpoints, Beads, budget, MCP…) and new team bricks.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--against` | string | `published` | `published` or a version number |
| `--layer` | string | project when it holds the workflow, else team | `team` or `project` |
| `--json` | bool | `false` | JSON output (diff, impact, next version) |

### oh workflow publish

```
oh workflow publish <id> -m "<message>" [--yes]
oh workflow publish --retry
```

Shows the next version and the impact, asks for confirmation when the workflow is widened (unless `--yes` or without a terminal), then publishes (sync, revalidation, version + 1, history, `workflows.lock`, commit + push, redone if another member published in the meantime). Offline: queued; `--retry` replays the pending publications (the TUI also replays them automatically at each team-state synchronization). Team members only (`[governance] publish`).

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--message` | `-m` | string | Publication message (required) |
| `--retry` | | bool | Replay the pending (offline) publications |
| `--yes` | `-y` | bool | Do not ask for confirmation |

### oh workflow history

```
oh workflow history <id> [--json]
```

Published versions, most recent first: version, date, author, message.

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | JSON output |

### oh workflow restore

```
oh workflow restore <id> <version> [--yes]
```

Publishes the content of a version (document and template) again as a **new** version, after revalidation.

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--yes` | `-y` | bool | Do not ask for confirmation |

### oh workflow archive

```
oh workflow archive <id> [-m "<reason>"] [--yes]
```

Withdraws the published workflow; its last version stays in history (can be restored).

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--message` | `-m` | string | Reason for archiving |
| `--yes` | `-y` | bool | Do not ask for confirmation |

## oh bundle build / show

```
oh bundle build <workflow> [-p <project>] [-P <provider>] [--json]
oh bundle show <workflow>|<hash> [-p <project>] [-P <provider>] [--budget] [--json]
```

Compiles (idempotently, by hash) the session bundle of a workflow into `~/.oh/bundles/<hash>/` and shows it: agents (entry agent first), delegations, on-demand skills, MCP, plugins, default model, delegation depth, isolation (`strict` when the workflow requires it), global permissions and the estimated cost of the first turn (entry agent + skill catalogue). Without `-p`, the project is the one of the current directory (otherwise a hub-only bundle, without project instructions or MCP).

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Project (instructions, models, MCP); default: detected from the current folder |
| `--provider` | `-P` | string | LLM provider (model normalization) |
| `--budget` | | bool | `show` only: details the estimated budget, tokens per agent and per skill (≈ 4 characters per token) |
| `--json` | | bool | JSON output (`bundle.Report`) |

`oh bundle show` accepts a workflow name or the hash of an already built bundle.

```bash
oh bundle build ticket -p my-app
oh bundle show ticket --budget
oh bundle show 3f9c2a… --json
```

`oh skill budget <workflow>` is a deprecated alias of `oh bundle show <workflow> --budget`; without a workflow, the former per-agent computation stays available with a warning.

## Environment variables

| Variable | Effect |
|----------|--------|
| `OH_WORKFLOWS_DIR` | Replaces `~/.oh/hub/workflows` as the hub layer (workflows under development, tests) |
